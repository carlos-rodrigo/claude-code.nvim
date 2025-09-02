package ai

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"claude-code-intelligence/internal/database"
	"claude-code-intelligence/internal/types"

	"github.com/sirupsen/logrus"
)

// ContextBuilder assembles smart context from multiple sessions
type ContextBuilder struct {
	db       *database.Manager
	ollama   *OllamaClient
	logger   *logrus.Logger
	maxTokens int
}

// NewContextBuilder creates a new context builder
func NewContextBuilder(db *database.Manager, ollama *OllamaClient, logger *logrus.Logger) *ContextBuilder {
	return &ContextBuilder{
		db:        db,
		ollama:    ollama,
		logger:    logger,
		maxTokens: 4000, // Default max context size
	}
}

// ContextRequest represents a request to build context
type ContextRequest struct {
	SessionID      string            `json:"session_id,omitempty"`
	ProjectID      string            `json:"project_id,omitempty"`
	Query          string            `json:"query,omitempty"`
	Topics         []string          `json:"topics,omitempty"`
	MaxTokens      int               `json:"max_tokens,omitempty"`
	TimeRange      *TimeRange        `json:"time_range,omitempty"`
	IncludeTypes   []string          `json:"include_types,omitempty"` // decisions, topics, code, discussions
	Filters        map[string]string `json:"filters,omitempty"`        // status, model, importance
	MinRelevance   float64           `json:"min_relevance,omitempty"`  // Minimum relevance score
	SortBy         string            `json:"sort_by,omitempty"`        // relevance, date, size, importance
	SortOrder      string            `json:"sort_order,omitempty"`     // asc, desc
	ExcludeSessionIDs []string       `json:"exclude_sessions,omitempty"` // Sessions to exclude
}

// TimeRange for filtering sessions
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// ContextResult represents the assembled context
type ContextResult struct {
	Context          string                 `json:"context"`
	Sessions         []SessionReference     `json:"sessions"`
	Topics           []string               `json:"topics"`
	Decisions        []string               `json:"decisions"`
	TokenCount       int                    `json:"token_count"`
	QualityScore     float64                `json:"quality_score"`
	AssemblyTime     time.Duration          `json:"assembly_time"`
	TruncationNeeded bool                   `json:"truncation_needed"`
}

// SessionReference tracks which sessions contributed to context
type SessionReference struct {
	SessionID   string    `json:"session_id"`
	SessionName string    `json:"session_name"`
	Relevance   float64   `json:"relevance"`
	CreatedAt   time.Time `json:"created_at"`
}

// BuildContext assembles smart context from multiple sessions
func (cb *ContextBuilder) BuildContext(ctx context.Context, req ContextRequest) (*ContextResult, error) {
	startTime := time.Now()
	
	// Set defaults
	if req.MaxTokens == 0 {
		req.MaxTokens = cb.maxTokens
	}

	cb.logger.WithFields(logrus.Fields{
		"session_id": req.SessionID,
		"project_id": req.ProjectID,
		"query":      req.Query,
		"max_tokens": req.MaxTokens,
	}).Info("Building context")

	// Find related sessions
	relatedSessions, err := cb.findRelatedSessions(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to find related sessions: %w", err)
	}

	if len(relatedSessions) == 0 {
		return &ContextResult{
			Context:      "No related sessions found",
			Sessions:     []SessionReference{},
			Topics:       []string{},
			Decisions:    []string{},
			TokenCount:   0,
			AssemblyTime: time.Since(startTime),
		}, nil
	}

	// Sort sessions by requested criteria
	cb.sortSessions(relatedSessions, req)

	// Extract key information from sessions
	extractedInfo := cb.extractKeyInformation(ctx, relatedSessions, req)

	// Optimize for token limit
	optimizedContext := cb.optimizeForTokenLimit(extractedInfo, req.MaxTokens)

	// Calculate quality score
	qualityScore := cb.calculateQualityScore(optimizedContext, relatedSessions)

	// Build session references
	sessionRefs := make([]SessionReference, 0, len(relatedSessions))
	for _, rs := range relatedSessions {
		if rs.included {
			sessionRefs = append(sessionRefs, SessionReference{
				SessionID:   rs.session.ID,
				SessionName: rs.session.Name,
				Relevance:   rs.relevanceScore,
				CreatedAt:   rs.session.CreatedAt,
			})
		}
	}

	result := &ContextResult{
		Context:          optimizedContext.content,
		Sessions:         sessionRefs,
		Topics:           optimizedContext.topics,
		Decisions:        optimizedContext.decisions,
		TokenCount:       optimizedContext.tokenCount,
		QualityScore:     qualityScore,
		AssemblyTime:     time.Since(startTime),
		TruncationNeeded: optimizedContext.truncated,
	}

	cb.logger.WithFields(logrus.Fields{
		"sessions_found":    len(relatedSessions),
		"sessions_included": len(sessionRefs),
		"token_count":       result.TokenCount,
		"quality_score":     result.QualityScore,
		"assembly_time":     result.AssemblyTime,
	}).Info("Context built successfully")

	return result, nil
}

// relatedSession holds session data with relevance scoring
type relatedSession struct {
	session        *types.Session
	topics         []types.Topic
	decisions      []types.Decision
	relevanceScore float64
	included       bool
}

// findRelatedSessions discovers sessions related to the request
func (cb *ContextBuilder) findRelatedSessions(ctx context.Context, req ContextRequest) ([]*relatedSession, error) {
	var sessions []*types.Session
	var err error

	// Get sessions based on request type
	if req.SessionID != "" {
		// Find sessions related to a specific session
		sessions, err = cb.findSessionsByRelationship(ctx, req.SessionID)
	} else if req.ProjectID != "" {
		// Get all sessions for a project
		sessions, err = cb.db.ListSessions(ctx, 100, 0, &req.ProjectID)
	} else if req.Query != "" {
		// Search sessions by query
		searchResults, searchErr := cb.db.SearchSessions(ctx, req.Query, 50)
		if searchErr != nil {
			return nil, searchErr
		}
		// Convert search results to sessions
		sessions = make([]*types.Session, 0, len(searchResults))
		for _, result := range searchResults {
			session, getErr := cb.db.GetSession(ctx, result.SessionID)
			if getErr == nil {
				sessions = append(sessions, session)
			}
		}
	} else {
		// Get recent sessions
		sessions, err = cb.db.ListSessions(ctx, 20, 0, nil)
	}

	if err != nil {
		return nil, err
	}

	// Apply filters
	sessions = cb.applyFilters(sessions, req)
	
	// Apply exclusions
	if len(req.ExcludeSessionIDs) > 0 {
		excludeMap := make(map[string]bool)
		for _, id := range req.ExcludeSessionIDs {
			excludeMap[id] = true
		}
		filtered := make([]*types.Session, 0)
		for _, session := range sessions {
			if !excludeMap[session.ID] {
				filtered = append(filtered, session)
			}
		}
		sessions = filtered
	}

	// Score and enrich sessions
	relatedSessions := make([]*relatedSession, 0, len(sessions))
	for _, session := range sessions {
		rs := &relatedSession{
			session: session,
		}

		// Load topics for this session
		topics, err := cb.db.GetSessionTopics(ctx, session.ID)
		if err == nil {
			rs.topics = topics
		}

		// Load decisions for this session
		decisions, err := cb.db.GetSessionDecisions(ctx, session.ID)
		if err == nil {
			rs.decisions = decisions
		}

		// Calculate relevance score
		rs.relevanceScore = cb.calculateRelevance(rs, req)

		// Only include if relevance is above threshold
		minRelevance := req.MinRelevance
		if minRelevance == 0 {
			minRelevance = 0.1 // Default threshold
		}
		if rs.relevanceScore > minRelevance {
			relatedSessions = append(relatedSessions, rs)
		}
	}

	return relatedSessions, nil
}

// findSessionsByRelationship finds sessions related to a given session
func (cb *ContextBuilder) findSessionsByRelationship(ctx context.Context, sessionID string) ([]*types.Session, error) {
	// This would query the session_relationships table
	// For now, return recent sessions from the same project
	
	baseSession, err := cb.db.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	if baseSession.ProjectID != nil {
		return cb.db.ListSessions(ctx, 20, 0, baseSession.ProjectID)
	}

	return cb.db.ListSessions(ctx, 10, 0, nil)
}

// calculateRelevance scores how relevant a session is to the request
func (cb *ContextBuilder) calculateRelevance(rs *relatedSession, req ContextRequest) float64 {
	score := 0.0
	factors := 0

	// Topic matching
	if len(req.Topics) > 0 && len(rs.topics) > 0 {
		topicScore := 0.0
		for _, reqTopic := range req.Topics {
			for _, sessionTopic := range rs.topics {
				if strings.Contains(strings.ToLower(sessionTopic.Topic), strings.ToLower(reqTopic)) {
					topicScore += sessionTopic.RelevanceScore
				}
			}
		}
		score += topicScore
		factors++
	}

	// Query matching in summary
	if req.Query != "" && rs.session.Summary != nil {
		if strings.Contains(strings.ToLower(*rs.session.Summary), strings.ToLower(req.Query)) {
			score += 1.0
			factors++
		}
	}

	// Time proximity (more recent = more relevant)
	age := time.Since(rs.session.CreatedAt)
	if age < 24*time.Hour {
		score += 1.0
	} else if age < 7*24*time.Hour {
		score += 0.5
	} else if age < 30*24*time.Hour {
		score += 0.2
	}
	factors++

	// Compression quality (better compressed = better content)
	if rs.session.CompressionRatio > 0 {
		score += (1 - rs.session.CompressionRatio) * 0.5
		factors++
	}

	// Decision importance
	if len(rs.decisions) > 0 {
		decisionScore := 0.0
		for _, decision := range rs.decisions {
			decisionScore += decision.ImportanceScore
		}
		score += decisionScore / float64(len(rs.decisions))
		factors++
	}

	if factors == 0 {
		return 0.1 // Base relevance
	}

	return score / float64(factors)
}

// extractedInformation holds information extracted from sessions
type extractedInformation struct {
	content    string
	topics     []string
	decisions  []string
	tokenCount int
	truncated  bool
}

// extractKeyInformation extracts key information from related sessions
func (cb *ContextBuilder) extractKeyInformation(ctx context.Context, sessions []*relatedSession, req ContextRequest) *extractedInformation {
	info := &extractedInformation{
		topics:    make([]string, 0),
		decisions: make([]string, 0),
	}

	var contentParts []string
	topicMap := make(map[string]bool)
	decisionMap := make(map[string]bool)

	// Process sessions in order of relevance
	for _, rs := range sessions {
		// Add session header
		header := fmt.Sprintf("=== Session: %s (Relevance: %.2f) ===\n", rs.session.Name, rs.relevanceScore)
		contentParts = append(contentParts, header)

		// Add summary if available
		if rs.session.Summary != nil && *rs.session.Summary != "" {
			contentParts = append(contentParts, fmt.Sprintf("Summary: %s\n", *rs.session.Summary))
		}

		// Add topics
		if len(rs.topics) > 0 {
			contentParts = append(contentParts, "\nKey Topics:")
			for _, topic := range rs.topics {
				if !topicMap[topic.Topic] {
					topicMap[topic.Topic] = true
					info.topics = append(info.topics, topic.Topic)
					contentParts = append(contentParts, fmt.Sprintf("- %s (relevance: %.2f)", topic.Topic, topic.RelevanceScore))
				}
			}
		}

		// Add decisions
		if len(rs.decisions) > 0 {
			contentParts = append(contentParts, "\nKey Decisions:")
			for _, decision := range rs.decisions {
				if !decisionMap[decision.DecisionText] {
					decisionMap[decision.DecisionText] = true
					info.decisions = append(info.decisions, decision.DecisionText)
					contentParts = append(contentParts, fmt.Sprintf("- %s", decision.DecisionText))
					if decision.Reasoning != nil && *decision.Reasoning != "" {
						contentParts = append(contentParts, fmt.Sprintf("  Reasoning: %s", *decision.Reasoning))
					}
				}
			}
		}

		contentParts = append(contentParts, "\n")
		rs.included = true
	}

	info.content = strings.Join(contentParts, "\n")
	info.tokenCount = cb.estimateTokenCount(info.content)

	return info
}

// optimizeForTokenLimit optimizes content to fit within token limit
func (cb *ContextBuilder) optimizeForTokenLimit(info *extractedInformation, maxTokens int) *extractedInformation {
	if info.tokenCount <= maxTokens {
		return info
	}

	cb.logger.WithFields(logrus.Fields{
		"original_tokens": info.tokenCount,
		"max_tokens":      maxTokens,
	}).Debug("Optimizing context for token limit")

	// Strategy: Progressively remove less important content
	// 1. Remove duplicate information
	// 2. Summarize long sections
	// 3. Remove older/less relevant sessions

	// For now, simple truncation with ellipsis
	targetLength := maxTokens * 3 // Rough estimate: 1 token ≈ 3 characters
	if len(info.content) > targetLength {
		info.content = info.content[:targetLength] + "\n\n[Context truncated to fit token limit]"
		info.truncated = true
		info.tokenCount = cb.estimateTokenCount(info.content)
	}

	return info
}

// estimateTokenCount estimates the token count for a text
func (cb *ContextBuilder) estimateTokenCount(text string) int {
	// Simple estimation: 1 token ≈ 4 characters
	// This is a rough approximation; actual tokenization depends on the model
	return len(text) / 4
}

// calculateQualityScore calculates the quality of the assembled context
func (cb *ContextBuilder) calculateQualityScore(info *extractedInformation, sessions []*relatedSession) float64 {
	score := 0.0
	factors := 0.0

	// Factor 1: Coverage (how many relevant sessions were included)
	includedCount := 0
	totalRelevance := 0.0
	for _, rs := range sessions {
		if rs.included {
			includedCount++
			totalRelevance += rs.relevanceScore
		}
	}
	
	if len(sessions) > 0 {
		coverageScore := float64(includedCount) / float64(len(sessions))
		score += coverageScore * 2
		factors += 2
	}

	// Factor 2: Relevance (average relevance of included sessions)
	if includedCount > 0 {
		avgRelevance := totalRelevance / float64(includedCount)
		score += avgRelevance * 3
		factors += 3
	}

	// Factor 3: Information density (topics and decisions per token)
	if info.tokenCount > 0 {
		infoDensity := float64(len(info.topics)+len(info.decisions)) / float64(info.tokenCount) * 100
		score += math.Min(infoDensity, 1.0) * 2
		factors += 2
	}

	// Factor 4: Truncation penalty
	if !info.truncated {
		score += 1.0
	}
	factors += 1

	if factors == 0 {
		return 0
	}

	// Normalize to 0-10 scale
	return (score / factors) * 10
}

// RestoreContext restores a session with enriched context
func (cb *ContextBuilder) RestoreContext(ctx context.Context, sessionID string) (*ContextResult, error) {
	req := ContextRequest{
		SessionID: sessionID,
		MaxTokens: cb.maxTokens,
	}

	return cb.BuildContext(ctx, req)
}

// GetProjectContext builds context for an entire project
func (cb *ContextBuilder) GetProjectContext(ctx context.Context, projectID string, maxTokens int) (*ContextResult, error) {
	req := ContextRequest{
		ProjectID: projectID,
		MaxTokens: maxTokens,
	}

	return cb.BuildContext(ctx, req)
}

// applyFilters applies various filters to the session list
func (cb *ContextBuilder) applyFilters(sessions []*types.Session, req ContextRequest) []*types.Session {
	filtered := make([]*types.Session, 0, len(sessions))
	
	for _, session := range sessions {
		// Time range filter
		if req.TimeRange != nil {
			if session.CreatedAt.Before(req.TimeRange.Start) || session.CreatedAt.After(req.TimeRange.End) {
				continue
			}
		}
		
		// Status filter
		if statusFilter, ok := req.Filters["status"]; ok && statusFilter != "" {
			if session.Status != statusFilter {
				continue
			}
		}
		
		// Model filter
		if modelFilter, ok := req.Filters["model"]; ok && modelFilter != "" {
			if session.CompressionModel == nil || *session.CompressionModel != modelFilter {
				continue
			}
		}
		
		// Size filter (small, medium, large)
		if sizeFilter, ok := req.Filters["size"]; ok && sizeFilter != "" {
			size := cb.categorizeSessionSize(session)
			if size != sizeFilter {
				continue
			}
		}
		
		// Compression quality filter
		if qualityFilter, ok := req.Filters["quality"]; ok && qualityFilter != "" {
			quality := cb.categorizeCompressionQuality(session)
			if quality != qualityFilter {
				continue
			}
		}
		
		filtered = append(filtered, session)
	}
	
	return filtered
}

// categorizeSessionSize categorizes session size as small, medium, or large
func (cb *ContextBuilder) categorizeSessionSize(session *types.Session) string {
	if session.OriginalSize < 10000 {
		return "small"
	} else if session.OriginalSize < 100000 {
		return "medium"
	}
	return "large"
}

// categorizeCompressionQuality categorizes compression quality as low, medium, or high
func (cb *ContextBuilder) categorizeCompressionQuality(session *types.Session) string {
	if session.CompressionRatio == 0 {
		return "none"
	} else if session.CompressionRatio < 0.3 {
		return "high"  // Low ratio means high compression
	} else if session.CompressionRatio < 0.7 {
		return "medium"
	}
	return "low"
}

// sortSessions sorts sessions based on the request criteria
func (cb *ContextBuilder) sortSessions(sessions []*relatedSession, req ContextRequest) {
	sortBy := req.SortBy
	if sortBy == "" {
		sortBy = "relevance" // Default
	}
	
	sortOrder := req.SortOrder
	if sortOrder == "" {
		sortOrder = "desc" // Default
	}
	
	sort.Slice(sessions, func(i, j int) bool {
		var less bool
		
		switch sortBy {
		case "relevance":
			less = sessions[i].relevanceScore < sessions[j].relevanceScore
		case "date", "created_at":
			less = sessions[i].session.CreatedAt.Before(sessions[j].session.CreatedAt)
		case "size", "original_size":
			less = sessions[i].session.OriginalSize < sessions[j].session.OriginalSize
		case "compression":
			less = sessions[i].session.CompressionRatio < sessions[j].session.CompressionRatio
		case "importance":
			// Calculate average importance score from decisions
			avgI := cb.calculateAverageImportance(sessions[i])
			avgJ := cb.calculateAverageImportance(sessions[j])
			less = avgI < avgJ
		default:
			// Default to relevance
			less = sessions[i].relevanceScore < sessions[j].relevanceScore
		}
		
		// Apply sort order
		if sortOrder == "asc" {
			return less
		}
		return !less
	})
}

// calculateAverageImportance calculates average importance score from decisions
func (cb *ContextBuilder) calculateAverageImportance(rs *relatedSession) float64 {
	if len(rs.decisions) == 0 {
		return 0.0
	}
	
	total := 0.0
	for _, decision := range rs.decisions {
		total += decision.ImportanceScore
	}
	
	return total / float64(len(rs.decisions))
}