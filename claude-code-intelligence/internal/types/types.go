package types

import (
	"time"
)

// Session represents a claude-code session
type Session struct {
	ID                string     `json:"id" db:"id"`
	ProjectID         *string    `json:"project_id" db:"project_id"`
	Name              string     `json:"name" db:"name"`
	OriginalPath      string     `json:"original_path" db:"original_path"`
	CompressedPath    *string    `json:"compressed_path" db:"compressed_path"`
	CreatedAt         time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at" db:"updated_at"`
	OriginalSize      int64      `json:"original_size" db:"original_size"`
	CompressedSize    int64      `json:"compressed_size" db:"compressed_size"`
	CompressionRatio  float64    `json:"compression_ratio" db:"compression_ratio"`
	CompressionModel  *string    `json:"compression_model" db:"compression_model"`
	Status            string     `json:"status" db:"status"`
	ErrorMessage      *string    `json:"error_message" db:"error_message"`
	Metadata          string     `json:"metadata" db:"metadata"` // JSON string
	Summary           *string    `json:"summary" db:"summary"`
	ProcessingTimeMs  *int64     `json:"processing_time_ms" db:"processing_time_ms"`
}

// SessionStatus represents the possible states of a session
type SessionStatus string

const (
	StatusPending    SessionStatus = "pending"
	StatusProcessing SessionStatus = "processing"
	StatusCompressed SessionStatus = "compressed"
	StatusFailed     SessionStatus = "failed"
	StatusArchived   SessionStatus = "archived"
)

// Topic represents an extracted topic from a session
type Topic struct {
	ID               string     `json:"id" db:"id"`
	SessionID        string     `json:"session_id" db:"session_id"`
	Topic            string     `json:"topic" db:"topic"`
	RelevanceScore   float64    `json:"relevance_score" db:"relevance_score"`
	Frequency        int        `json:"frequency" db:"frequency"`
	FirstMentionedAt *time.Time `json:"first_mentioned_at" db:"first_mentioned_at"`
	Context          *string    `json:"context" db:"context"`
	ExtractedBy      *string    `json:"extracted_by_model" db:"extracted_by_model"`
}

// Decision represents an important decision tracked in a session
type Decision struct {
	ID              string     `json:"id" db:"id"`
	SessionID       string     `json:"session_id" db:"session_id"`
	DecisionText    string     `json:"decision_text" db:"decision_text"`
	Reasoning       *string    `json:"reasoning" db:"reasoning"`
	Outcome         *string    `json:"outcome" db:"outcome"`
	ImportanceScore float64    `json:"importance_score" db:"importance_score"`
	CreatedAt       time.Time  `json:"created_at" db:"created_at"`
	Tags            string     `json:"tags" db:"tags"` // JSON array
	ExtractedBy     *string    `json:"extracted_by_model" db:"extracted_by_model"`
}

// Embedding represents a vector embedding for semantic search
type Embedding struct {
	ID             string    `json:"id" db:"id"`
	SessionID      string    `json:"session_id" db:"session_id"`
	ChunkIndex     int       `json:"chunk_index" db:"chunk_index"`
	ContentHash    string    `json:"content_hash" db:"content_hash"`
	Embedding      []byte    `json:"embedding" db:"embedding"` // Vector data as bytes
	ContentPreview string    `json:"content_preview" db:"content_preview"`
	ChunkSize      int       `json:"chunk_size" db:"chunk_size"`
	ModelUsed      string    `json:"model_used" db:"model_used"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
}

// Project represents a project grouping sessions
type Project struct {
	ID           string     `json:"id" db:"id"`
	Name         string     `json:"name" db:"name"`
	Path         string     `json:"path" db:"path"`
	Description  *string    `json:"description" db:"description"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
	LastActive   *time.Time `json:"last_active" db:"last_active"`
	SessionCount int        `json:"session_count" db:"session_count"`
	TotalSize    int64      `json:"total_size" db:"total_size"`
	Metadata     string     `json:"metadata" db:"metadata"` // JSON string
}

// ModelPreset represents a predefined model configuration
type ModelPreset struct {
	Name        string  `json:"name"`
	Model       string  `json:"model"`
	Temperature float64 `json:"temperature"`
	MaxTokens   int     `json:"max_tokens"`
	Description string  `json:"description"`
}

// CompressionRequest represents a request to compress a session
type CompressionRequest struct {
	SessionID string                 `json:"session_id"`
	Content   string                 `json:"content"`
	Options   CompressionOptions     `json:"options"`
}

// CompressionOptions represents options for session compression
type CompressionOptions struct {
	Model       *string `json:"model,omitempty"`
	Preset      *string `json:"preset,omitempty"`
	Style       string  `json:"style"`         // concise, balanced, detailed
	MaxLength   int     `json:"max_length"`
	Priority    string  `json:"priority"`      // speed, balanced, quality
	Type        string  `json:"type"`          // general, code, discussion
	AllowFallback bool  `json:"allow_fallback"`
}

// CompressionResult represents the result of session compression
type CompressionResult struct {
	Summary          string        `json:"summary"`
	Model            string        `json:"model"`
	ProcessingTime   time.Duration `json:"processing_time"`
	OriginalSize     int           `json:"original_size"`
	CompressedSize   int           `json:"compressed_size"`
	CompressionRatio float64       `json:"compression_ratio"`
	Topics           []Topic       `json:"topics,omitempty"`
	QualityScore     float64       `json:"quality_score"`
}

// SearchRequest represents a semantic search request
type SearchRequest struct {
	Query     string `json:"query"`
	Limit     int    `json:"limit"`
	Threshold float64 `json:"threshold"`
	ProjectID *string `json:"project_id,omitempty"`
}

// SearchResult represents a search result
type SearchResult struct {
	SessionID      string  `json:"session_id"`
	SessionName    string  `json:"session_name"`
	Similarity     float64 `json:"similarity"`
	ContentPreview string  `json:"content_preview"`
	Summary        *string `json:"summary,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

// HealthStatus represents the health status of the service
type HealthStatus struct {
	Status      string                 `json:"status"`
	Timestamp   time.Time              `json:"timestamp"`
	Uptime      time.Duration          `json:"uptime"`
	Version     string                 `json:"version"`
	Components  map[string]ComponentHealth `json:"components"`
}

// ComponentHealth represents the health of a service component
type ComponentHealth struct {
	Status  string    `json:"status"`
	Message string    `json:"message,omitempty"`
	LastCheck time.Time `json:"last_check"`
}

// ModelTestResult represents the result of testing a model
type ModelTestResult struct {
	Model            string        `json:"model"`
	Success          bool          `json:"success"`
	ProcessingTime   time.Duration `json:"processing_time"`
	CompressionRatio float64       `json:"compression_ratio"`
	OutputLength     int           `json:"output_length"`
	QualityScore     float64       `json:"quality_score"`
	Error            *string       `json:"error,omitempty"`
}

// APIError represents a structured API error response
type APIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}