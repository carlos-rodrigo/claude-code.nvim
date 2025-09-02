package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"claude-code-intelligence/internal/config"
	"claude-code-intelligence/internal/types"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	_ "modernc.org/sqlite" // SQLite driver
)

// Manager handles all database operations
type Manager struct {
	db     *sql.DB
	config *config.Config
	logger *logrus.Logger
}

// NewManager creates a new database manager
func NewManager(cfg *config.Config, logger *logrus.Logger) *Manager {
	return &Manager{
		config: cfg,
		logger: logger,
	}
}

// Initialize opens the database connection and runs migrations
func (m *Manager) Initialize(ctx context.Context) error {
	// Ensure data directory exists
	dbDir := filepath.Dir(m.config.Database.Path)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return fmt.Errorf("failed to create database directory: %w", err)
	}

	// Open database connection
	db, err := sql.Open("sqlite", m.config.Database.Path)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	m.db = db

	// Configure SQLite
	if err := m.configureSQLite(); err != nil {
		return fmt.Errorf("failed to configure SQLite: %w", err)
	}

	// Run migrations
	if err := m.migrate(ctx); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	m.logger.WithField("db_path", m.config.Database.Path).Info("Database initialized successfully")
	return nil
}

// configureSQLite sets up SQLite pragmas for optimal performance
func (m *Manager) configureSQLite() error {
	pragmas := []string{
		"PRAGMA journal_mode = WAL",
		"PRAGMA synchronous = NORMAL",
		"PRAGMA cache_size = 10000",
		"PRAGMA temp_store = MEMORY",
		"PRAGMA foreign_keys = ON",
	}

	for _, pragma := range pragmas {
		if _, err := m.db.Exec(pragma); err != nil {
			return fmt.Errorf("failed to execute pragma %s: %w", pragma, err)
		}
	}

	return nil
}

// migrate runs database migrations
func (m *Manager) migrate(ctx context.Context) error {
	// Read schema file
	schemaBytes, err := os.ReadFile("internal/database/schema.sql")
	if err != nil {
		return fmt.Errorf("failed to read schema file: %w", err)
	}

	// Execute schema
	if _, err := m.db.ExecContext(ctx, string(schemaBytes)); err != nil {
		return fmt.Errorf("failed to execute schema: %w", err)
	}

	m.logger.Debug("Database migration completed")
	return nil
}

// Session operations

// CreateSession creates a new session record
func (m *Manager) CreateSession(ctx context.Context, session *types.Session) error {
	if session.ID == "" {
		session.ID = uuid.New().String()
	}

	// Serialize metadata
	metadataJSON := "{}"
	if session.Metadata != "" {
		metadataJSON = session.Metadata
	}

	query := `
		INSERT INTO sessions (
			id, project_id, name, original_path, compressed_path,
			original_size, compressed_size, compression_ratio,
			compression_model, status, summary, metadata, processing_time_ms
		) VALUES (
			?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
		)
	`

	_, err := m.db.ExecContext(ctx, query,
		session.ID, session.ProjectID, session.Name, session.OriginalPath,
		session.CompressedPath, session.OriginalSize, session.CompressedSize,
		session.CompressionRatio, session.CompressionModel, session.Status,
		session.Summary, metadataJSON, session.ProcessingTimeMs,
	)

	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	m.logger.WithField("session_id", session.ID).Debug("Session created")
	return nil
}

// GetSession retrieves a session by ID
func (m *Manager) GetSession(ctx context.Context, id string) (*types.Session, error) {
	query := `SELECT * FROM sessions WHERE id = ?`
	
	row := m.db.QueryRowContext(ctx, query, id)
	
	session := &types.Session{}
	err := row.Scan(
		&session.ID, &session.ProjectID, &session.Name, &session.OriginalPath,
		&session.CompressedPath, &session.CreatedAt, &session.UpdatedAt,
		&session.OriginalSize, &session.CompressedSize, &session.CompressionRatio,
		&session.CompressionModel, &session.Status, &session.ErrorMessage,
		&session.Metadata, &session.Summary, &session.ProcessingTimeMs,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("session not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	return session, nil
}

// UpdateSession updates an existing session
func (m *Manager) UpdateSession(ctx context.Context, session *types.Session) error {
	query := `
		UPDATE sessions SET
			project_id = ?, name = ?, compressed_path = ?, compressed_size = ?,
			compression_ratio = ?, compression_model = ?, status = ?,
			error_message = ?, summary = ?, processing_time_ms = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`

	result, err := m.db.ExecContext(ctx, query,
		session.ProjectID, session.Name, session.CompressedPath,
		session.CompressedSize, session.CompressionRatio, session.CompressionModel,
		session.Status, session.ErrorMessage, session.Summary,
		session.ProcessingTimeMs, session.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("session not found: %s", session.ID)
	}

	m.logger.WithField("session_id", session.ID).Debug("Session updated")
	return nil
}

// ListSessions lists sessions with optional filtering
func (m *Manager) ListSessions(ctx context.Context, limit, offset int, projectID *string) ([]*types.Session, error) {
	query := `SELECT * FROM sessions`
	args := []interface{}{}

	if projectID != nil {
		query += ` WHERE project_id = ?`
		args = append(args, *projectID)
	}

	query += ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*types.Session
	for rows.Next() {
		session := &types.Session{}
		err := rows.Scan(
			&session.ID, &session.ProjectID, &session.Name, &session.OriginalPath,
			&session.CompressedPath, &session.CreatedAt, &session.UpdatedAt,
			&session.OriginalSize, &session.CompressedSize, &session.CompressionRatio,
			&session.CompressionModel, &session.Status, &session.ErrorMessage,
			&session.Metadata, &session.Summary, &session.ProcessingTimeMs,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}
		sessions = append(sessions, session)
	}

	return sessions, nil
}

// SearchSessions performs text-based search on sessions
func (m *Manager) SearchSessions(ctx context.Context, query string, limit int) ([]*types.SearchResult, error) {
	sqlQuery := `
		SELECT s.id, s.name, s.summary, s.created_at,
			   CASE 
				   WHEN s.name LIKE ? THEN 1.0
				   WHEN s.summary LIKE ? THEN 0.8
				   ELSE 0.5
			   END as similarity
		FROM sessions s
		WHERE s.name LIKE ? OR s.summary LIKE ?
		ORDER BY similarity DESC, s.created_at DESC
		LIMIT ?
	`

	searchPattern := "%" + query + "%"
	rows, err := m.db.QueryContext(ctx, sqlQuery, 
		searchPattern, searchPattern, searchPattern, searchPattern, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search sessions: %w", err)
	}
	defer rows.Close()

	var results []*types.SearchResult
	for rows.Next() {
		result := &types.SearchResult{}
		err := rows.Scan(
			&result.SessionID, &result.SessionName, &result.Summary,
			&result.CreatedAt, &result.Similarity,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan search result: %w", err)
		}

		// Set content preview from summary
		if result.Summary != nil {
			preview := *result.Summary
			if len(preview) > 200 {
				preview = preview[:200] + "..."
			}
			result.ContentPreview = preview
		}

		results = append(results, result)
	}

	return results, nil
}

// Embedding operations

// SaveEmbedding stores an embedding in the database
func (m *Manager) SaveEmbedding(ctx context.Context, embedding *types.Embedding) error {
	if embedding.ID == "" {
		embedding.ID = uuid.New().String()
	}

	query := `
		INSERT INTO embeddings (
			id, session_id, chunk_index, content_hash, embedding,
			content_preview, chunk_size, model_used
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := m.db.ExecContext(ctx, query,
		embedding.ID, embedding.SessionID, embedding.ChunkIndex,
		embedding.ContentHash, embedding.Embedding, embedding.ContentPreview,
		embedding.ChunkSize, embedding.ModelUsed,
	)

	if err != nil {
		return fmt.Errorf("failed to save embedding: %w", err)
	}

	return nil
}

// Topic operations

// SaveTopics saves multiple topics for a session
func (m *Manager) SaveTopics(ctx context.Context, sessionID string, topics []types.Topic) error {
	if len(topics) == 0 {
		return nil
	}

	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	query := `
		INSERT INTO topics (
			id, session_id, topic, relevance_score, frequency,
			first_mentioned_at, context, extracted_by_model
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, topic := range topics {
		if topic.ID == "" {
			topic.ID = uuid.New().String()
		}

		_, err := stmt.ExecContext(ctx,
			topic.ID, sessionID, topic.Topic, topic.RelevanceScore,
			topic.Frequency, topic.FirstMentionedAt, topic.Context,
			topic.ExtractedBy,
		)
		if err != nil {
			return fmt.Errorf("failed to save topic: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	m.logger.WithFields(logrus.Fields{
		"session_id":   sessionID,
		"topic_count": len(topics),
	}).Debug("Topics saved")

	return nil
}

// Model performance tracking

// TrackModelPerformance records model performance metrics
func (m *Manager) TrackModelPerformance(ctx context.Context, model, operation string, success bool, processingTime time.Duration, qualityScore float64) error {
	query := `
		INSERT INTO model_performance (
			id, model_name, operation_type, success_count, failure_count,
			avg_processing_time_ms, avg_quality_score, total_tokens_used
		) VALUES (?, ?, ?, ?, ?, ?, ?, 0)
		ON CONFLICT(model_name, operation_type) DO UPDATE SET
			success_count = success_count + ?,
			failure_count = failure_count + ?,
			avg_processing_time_ms = (avg_processing_time_ms + ?) / 2,
			avg_quality_score = (avg_quality_score + ?) / 2,
			last_used = CURRENT_TIMESTAMP
	`

	id := fmt.Sprintf("%s_%s", model, operation)
	successCount := 0
	failureCount := 0
	
	if success {
		successCount = 1
	} else {
		failureCount = 1
	}

	processingTimeMs := float64(processingTime.Nanoseconds()) / 1e6

	_, err := m.db.ExecContext(ctx, query,
		id, model, operation, successCount, failureCount,
		processingTimeMs, qualityScore,
		successCount, failureCount, processingTimeMs, qualityScore,
	)

	return err
}

// GetModelPerformance retrieves model performance statistics
func (m *Manager) GetModelPerformance(ctx context.Context) ([]map[string]interface{}, error) {
	query := `SELECT * FROM v_model_usage ORDER BY last_used DESC`

	rows, err := m.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get model performance: %w", err)
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var modelName, operationType string
		var totalOps int
		var successRate, avgProcessingTime, avgQualityScore float64
		var lastUsed time.Time

		err := rows.Scan(&modelName, &operationType, &totalOps, &successRate,
			&avgProcessingTime, &avgQualityScore, &lastUsed)
		if err != nil {
			return nil, fmt.Errorf("failed to scan performance data: %w", err)
		}

		results = append(results, map[string]interface{}{
			"model_name":             modelName,
			"operation_type":         operationType,
			"total_operations":       totalOps,
			"success_rate":          successRate,
			"avg_processing_time_ms": avgProcessingTime,
			"avg_quality_score":     avgQualityScore,
			"last_used":             lastUsed,
		})
	}

	return results, nil
}

// Utility operations

// GetSessionTopics retrieves all topics for a session
func (m *Manager) GetSessionTopics(ctx context.Context, sessionID string) ([]types.Topic, error) {
	query := `SELECT * FROM topics WHERE session_id = ? ORDER BY relevance_score DESC`
	
	rows, err := m.db.QueryContext(ctx, query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session topics: %w", err)
	}
	defer rows.Close()

	var topics []types.Topic
	for rows.Next() {
		var topic types.Topic
		err := rows.Scan(
			&topic.ID, &topic.SessionID, &topic.Topic, &topic.RelevanceScore,
			&topic.Frequency, &topic.FirstMentionedAt, &topic.Context, &topic.ExtractedBy,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan topic: %w", err)
		}
		topics = append(topics, topic)
	}

	return topics, nil
}

// GetSessionDecisions retrieves all decisions for a session
func (m *Manager) GetSessionDecisions(ctx context.Context, sessionID string) ([]types.Decision, error) {
	query := `SELECT * FROM decisions WHERE session_id = ? ORDER BY importance_score DESC`
	
	rows, err := m.db.QueryContext(ctx, query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session decisions: %w", err)
	}
	defer rows.Close()

	var decisions []types.Decision
	for rows.Next() {
		var decision types.Decision
		err := rows.Scan(
			&decision.ID, &decision.SessionID, &decision.DecisionText, &decision.Reasoning,
			&decision.Outcome, &decision.ImportanceScore, &decision.CreatedAt, &decision.Tags,
			&decision.ExtractedBy,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan decision: %w", err)
		}
		decisions = append(decisions, decision)
	}

	return decisions, nil
}

// GetStats returns database statistics
func (m *Manager) GetStats(ctx context.Context) (map[string]interface{}, error) {
	query := `
		SELECT 
			(SELECT COUNT(*) FROM sessions) as total_sessions,
			(SELECT COUNT(*) FROM sessions WHERE status = 'compressed') as compressed_sessions,
			(SELECT AVG(compression_ratio) FROM sessions WHERE compression_ratio > 0) as avg_compression_ratio,
			(SELECT COUNT(*) FROM topics) as total_topics,
			(SELECT COUNT(*) FROM embeddings) as total_embeddings,
			(SELECT COUNT(DISTINCT model_name) FROM model_performance) as models_used
	`

	row := m.db.QueryRowContext(ctx, query)

	var totalSessions, compressedSessions, totalTopics, totalEmbeddings, modelsUsed int
	var avgCompressionRatio sql.NullFloat64

	err := row.Scan(&totalSessions, &compressedSessions, &avgCompressionRatio,
		&totalTopics, &totalEmbeddings, &modelsUsed)
	if err != nil {
		return nil, fmt.Errorf("failed to get stats: %w", err)
	}

	stats := map[string]interface{}{
		"total_sessions":       totalSessions,
		"compressed_sessions":  compressedSessions,
		"total_topics":        totalTopics,
		"total_embeddings":    totalEmbeddings,
		"models_used":         modelsUsed,
	}

	if avgCompressionRatio.Valid {
		stats["avg_compression_ratio"] = avgCompressionRatio.Float64
	}

	return stats, nil
}

// Backup creates a backup of the database
func (m *Manager) Backup(ctx context.Context) (string, error) {
	backupDir := m.config.Database.BackupPath
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create backup directory: %w", err)
	}

	timestamp := time.Now().Format("20060102_150405")
	backupPath := filepath.Join(backupDir, fmt.Sprintf("backup_%s.db", timestamp))

	// SQLite backup command
	query := fmt.Sprintf("VACUUM INTO '%s'", backupPath)
	_, err := m.db.ExecContext(ctx, query)
	if err != nil {
		return "", fmt.Errorf("failed to backup database: %w", err)
	}

	m.logger.WithField("backup_path", backupPath).Info("Database backup created")
	return backupPath, nil
}

// ExecContext executes a query without returning any rows
func (m *Manager) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return m.db.ExecContext(ctx, query, args...)
}

// QueryRowContext executes a query that returns at most one row
func (m *Manager) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return m.db.QueryRowContext(ctx, query, args...)
}

// Close closes the database connection
func (m *Manager) Close() error {
	if m.db != nil {
		return m.db.Close()
	}
	return nil
}

// Health check
func (m *Manager) HealthCheck(ctx context.Context) types.ComponentHealth {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := m.db.PingContext(ctx); err != nil {
		return types.ComponentHealth{
			Status:    "unhealthy",
			Message:   fmt.Sprintf("Database connection failed: %v", err),
			LastCheck: time.Now(),
		}
	}

	// Test a simple query
	var count int
	err := m.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sessions").Scan(&count)
	if err != nil {
		return types.ComponentHealth{
			Status:    "unhealthy",
			Message:   fmt.Sprintf("Database query failed: %v", err),
			LastCheck: time.Now(),
		}
	}

	return types.ComponentHealth{
		Status:    "healthy",
		Message:   fmt.Sprintf("Database operational with %d sessions", count),
		LastCheck: time.Now(),
	}
}