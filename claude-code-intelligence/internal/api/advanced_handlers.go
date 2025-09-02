package api

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"claude-code-intelligence/internal/ai"
	"claude-code-intelligence/internal/cache"
	"claude-code-intelligence/internal/types"

	"github.com/gin-gonic/gin"
)

// AdvancedHandlers contains handlers for Phase 2 advanced features
type AdvancedHandlers struct {
	*Handlers
	contextBuilder *ai.ContextBuilder
	memorySystem   *ai.MemorySystem
	cacheManager   *cache.CacheManager
}

// NewAdvancedHandlers creates handlers with advanced features
func NewAdvancedHandlers(base *Handlers, contextBuilder *ai.ContextBuilder, memorySystem *ai.MemorySystem, cacheManager *cache.CacheManager) *AdvancedHandlers {
	return &AdvancedHandlers{
		Handlers:       base,
		contextBuilder: contextBuilder,
		memorySystem:   memorySystem,
		cacheManager:   cacheManager,
	}
}

// BuildContext builds smart context from multiple sessions
func (ah *AdvancedHandlers) BuildContext(c *gin.Context) {
	var req ai.ContextRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ah.errorResponse(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	ctx := c.Request.Context()

	// Check cache first
	cacheKey := cache.CacheContextKey(req.SessionID, req.ProjectID)
	if cached, err := ah.cacheManager.Get(ctx, cacheKey); err == nil {
		ah.logger.Debug("Context served from cache")
		c.JSON(http.StatusOK, cached)
		return
	}

	// Build context
	result, err := ah.contextBuilder.BuildContext(ctx, req)
	if err != nil {
		ah.errorResponse(c, http.StatusInternalServerError, "Failed to build context", err)
		return
	}

	// Cache the result
	if err := ah.cacheManager.Set(ctx, cacheKey, result, 10*time.Minute); err != nil {
		ah.logger.WithError(err).Warn("Failed to cache context result")
	}

	c.JSON(http.StatusOK, result)
}

// RestoreSession restores a session with enriched context
func (ah *AdvancedHandlers) RestoreSession(c *gin.Context) {
	sessionID := c.Param("id")
	if sessionID == "" {
		ah.errorResponse(c, http.StatusBadRequest, "Session ID is required", nil)
		return
	}

	ctx := c.Request.Context()

	// Build context for the session
	result, err := ah.contextBuilder.RestoreContext(ctx, sessionID)
	if err != nil {
		ah.errorResponse(c, http.StatusInternalServerError, "Failed to restore session context", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"session_id": sessionID,
		"context":    result,
		"restored_at": time.Now(),
	})
}

// ConsolidateProjectMemory consolidates memory for a project
func (ah *AdvancedHandlers) ConsolidateProjectMemory(c *gin.Context) {
	projectID := c.Param("id")
	if projectID == "" {
		projectID = c.Query("project_id")
	}

	if projectID == "" {
		ah.errorResponse(c, http.StatusBadRequest, "Project ID is required", nil)
		return
	}

	ctx := c.Request.Context()

	// Start consolidation
	ah.logger.WithField("project_id", projectID).Info("Starting memory consolidation")

	memory, err := ah.memorySystem.ConsolidateProjectMemory(ctx, projectID)
	if err != nil {
		ah.errorResponse(c, http.StatusInternalServerError, "Failed to consolidate project memory", err)
		return
	}

	c.JSON(http.StatusOK, memory)
}

// GetProjectMemory retrieves consolidated project memory
func (ah *AdvancedHandlers) GetProjectMemory(c *gin.Context) {
	projectID := c.Param("id")
	if projectID == "" {
		ah.errorResponse(c, http.StatusBadRequest, "Project ID is required", nil)
		return
	}

	ctx := c.Request.Context()

	memory, err := ah.memorySystem.GetProjectMemory(ctx, projectID)
	if err != nil {
		ah.errorResponse(c, http.StatusNotFound, "Project memory not found", err)
		return
	}

	c.JSON(http.StatusOK, memory)
}

// AdvancedSearch performs semantic search with filters
func (ah *AdvancedHandlers) AdvancedSearch(c *gin.Context) {
	var req struct {
		Query       string            `json:"query" binding:"required"`
		Filters     map[string]string `json:"filters"`
		TimeRange   *ai.TimeRange     `json:"time_range"`
		ProjectID   string            `json:"project_id"`
		Topics      []string          `json:"topics"`
		Limit       int               `json:"limit"`
		Offset      int               `json:"offset"`
		SortBy      string            `json:"sort_by"` // relevance, date, size
		SortOrder   string            `json:"sort_order"` // asc, desc
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		ah.errorResponse(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	// Set defaults
	if req.Limit == 0 {
		req.Limit = 20
	}
	if req.Limit > 100 {
		req.Limit = 100
	}
	if req.SortBy == "" {
		req.SortBy = "relevance"
	}
	if req.SortOrder == "" {
		req.SortOrder = "desc"
	}

	ctx := c.Request.Context()

	// Check cache
	cacheKey := cache.CacheSearchKey(req.Query, req.Limit)
	if cached, err := ah.cacheManager.Get(ctx, cacheKey); err == nil {
		ah.logger.Debug("Search results served from cache")
		c.JSON(http.StatusOK, cached)
		return
	}

	// Build context request for advanced search
	contextReq := ai.ContextRequest{
		Query:     req.Query,
		ProjectID: req.ProjectID,
		Topics:    req.Topics,
		TimeRange: req.TimeRange,
		MaxTokens: 1000, // Limit for search results
	}

	// Use context builder to find related sessions
	contextResult, err := ah.contextBuilder.BuildContext(ctx, contextReq)
	if err != nil {
		ah.errorResponse(c, http.StatusInternalServerError, "Search failed", err)
		return
	}

	// Format as search results
	searchResults := gin.H{
		"query":       req.Query,
		"results":     contextResult.Sessions,
		"topics":      contextResult.Topics,
		"count":       len(contextResult.Sessions),
		"total_tokens": contextResult.TokenCount,
		"filters":     req.Filters,
	}

	// Cache results
	if err := ah.cacheManager.Set(ctx, cacheKey, searchResults, 5*time.Minute); err != nil {
		ah.logger.WithError(err).Warn("Failed to cache search results")
	}

	c.JSON(http.StatusOK, searchResults)
}

// GetSessionAnalytics returns comprehensive analytics for sessions
func (ah *AdvancedHandlers) GetSessionAnalytics(c *gin.Context) {
	projectID := c.Query("project_id")
	days := c.DefaultQuery("days", "30")
	granularity := c.DefaultQuery("granularity", "day") // day, week, month

	daysInt, err := strconv.Atoi(days)
	if err != nil {
		daysInt = 30
	}

	ctx := c.Request.Context()

	// Get comprehensive analytics
	analytics, err := ah.buildComprehensiveAnalytics(ctx, projectID, daysInt, granularity)
	if err != nil {
		ah.errorResponse(c, http.StatusInternalServerError, "Failed to get analytics", err)
		return
	}

	c.JSON(http.StatusOK, analytics)
}

// buildComprehensiveAnalytics builds detailed analytics data
func (ah *AdvancedHandlers) buildComprehensiveAnalytics(ctx context.Context, projectID string, days int, granularity string) (gin.H, error) {
	timeRange := &ai.TimeRange{
		Start: time.Now().AddDate(0, 0, -days),
		End:   time.Now(),
	}

	contextReq := ai.ContextRequest{
		ProjectID: projectID,
		TimeRange: timeRange,
		MaxTokens: 2000,
	}

	contextResult, err := ah.contextBuilder.BuildContext(ctx, contextReq)
	if err != nil {
		return nil, err
	}

	// Get actual sessions from session references
	sessions, err := ah.getActualSessions(ctx, contextResult.Sessions)
	if err != nil {
		return nil, err
	}
	
	// Time series data
	timeSeries := ah.buildTimeSeries(sessions, granularity, days)
	
	// Topic analysis
	topicAnalysis := ah.analyzeTopics(contextResult.Topics)
	
	// Session patterns
	sessionPatterns := ah.analyzeSessionPatterns(sessions)
	
	// Decision analysis
	decisionAnalysis := ah.analyzeDecisions(contextResult.Decisions)
	
	// Performance metrics
	performanceMetrics := ah.calculatePerformanceMetrics(sessions)

	return gin.H{
		"period": gin.H{
			"start":       timeRange.Start,
			"end":         timeRange.End,
			"days":        days,
			"granularity": granularity,
		},
		"overview": gin.H{
			"total_sessions":    len(contextResult.Sessions),
			"total_topics":      len(contextResult.Topics),
			"total_decisions":   len(contextResult.Decisions),
			"average_quality":   contextResult.QualityScore,
			"total_tokens":      contextResult.TokenCount,
		},
		"time_series":         timeSeries,
		"topic_analysis":      topicAnalysis,
		"session_patterns":    sessionPatterns,
		"decision_analysis":   decisionAnalysis,
		"performance_metrics": performanceMetrics,
		"generated_at":        time.Now(),
	}, nil
}

// GetCacheStats returns cache statistics
func (ah *AdvancedHandlers) GetCacheStats(c *gin.Context) {
	stats := ah.cacheManager.GetStats()
	c.JSON(http.StatusOK, stats)
}

// ClearCache clears the cache
func (ah *AdvancedHandlers) ClearCache(c *gin.Context) {
	ctx := c.Request.Context()
	
	if err := ah.cacheManager.Clear(ctx); err != nil {
		ah.errorResponse(c, http.StatusInternalServerError, "Failed to clear cache", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Cache cleared successfully",
		"cleared_at": time.Now(),
	})
}

// GetSessionRelationships returns related sessions
func (ah *AdvancedHandlers) GetSessionRelationships(c *gin.Context) {
	sessionID := c.Param("id")
	if sessionID == "" {
		ah.errorResponse(c, http.StatusBadRequest, "Session ID is required", nil)
		return
	}

	ctx := c.Request.Context()

	// Build context to find related sessions
	contextReq := ai.ContextRequest{
		SessionID: sessionID,
		MaxTokens: 2000,
	}

	result, err := ah.contextBuilder.BuildContext(ctx, contextReq)
	if err != nil {
		ah.errorResponse(c, http.StatusInternalServerError, "Failed to find relationships", err)
		return
	}

	// Format relationships
	relationships := gin.H{
		"session_id":      sessionID,
		"related_sessions": result.Sessions,
		"shared_topics":   result.Topics,
		"related_decisions": result.Decisions,
		"relationship_quality": result.QualityScore,
	}

	c.JSON(http.StatusOK, relationships)
}

// GetProjectTimeline returns project timeline
func (ah *AdvancedHandlers) GetProjectTimeline(c *gin.Context) {
	projectID := c.Param("id")
	if projectID == "" {
		ah.errorResponse(c, http.StatusBadRequest, "Project ID is required", nil)
		return
	}

	ctx := c.Request.Context()

	// Get project memory which includes timeline
	memory, err := ah.memorySystem.GetProjectMemory(ctx, projectID)
	if err != nil {
		// If no cached memory, try to build it
		memory, err = ah.memorySystem.ConsolidateProjectMemory(ctx, projectID)
		if err != nil {
			ah.errorResponse(c, http.StatusInternalServerError, "Failed to get project timeline", err)
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"project_id": projectID,
		"timeline":   memory.Timeline,
		"insights":   memory.KeyInsights,
		"patterns":   memory.Patterns,
	})
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// getActualSessions retrieves full session objects from references
func (ah *AdvancedHandlers) getActualSessions(ctx context.Context, sessionRefs []ai.SessionReference) ([]*types.Session, error) {
	sessions := make([]*types.Session, 0, len(sessionRefs))
	
	for _, ref := range sessionRefs {
		session, err := ah.db.GetSession(ctx, ref.SessionID)
		if err == nil {
			sessions = append(sessions, session)
		}
	}
	
	return sessions, nil
}

// buildTimeSeries creates time series data for sessions
func (ah *AdvancedHandlers) buildTimeSeries(sessions []*types.Session, granularity string, days int) []gin.H {
	timeSlots := make(map[string]int)
	
	for _, session := range sessions {
		var timeKey string
		switch granularity {
		case "hour":
			timeKey = session.CreatedAt.Format("2006-01-02T15")
		case "day":
			timeKey = session.CreatedAt.Format("2006-01-02")
		case "week":
			year, week := session.CreatedAt.ISOWeek()
			timeKey = fmt.Sprintf("%d-W%02d", year, week)
		case "month":
			timeKey = session.CreatedAt.Format("2006-01")
		default:
			timeKey = session.CreatedAt.Format("2006-01-02")
		}
		timeSlots[timeKey]++
	}
	
	// Convert to array format for charts
	timeSeries := make([]gin.H, 0, len(timeSlots))
	for timeKey, count := range timeSlots {
		timeSeries = append(timeSeries, gin.H{
			"time":  timeKey,
			"count": count,
		})
	}
	
	return timeSeries
}

// analyzeTopics provides detailed topic analysis
func (ah *AdvancedHandlers) analyzeTopics(topics []string) gin.H {
	if len(topics) == 0 {
		return gin.H{
			"total": 0,
			"trending": []gin.H{},
			"categories": gin.H{},
		}
	}
	
	// Categorize topics
	categories := make(map[string]int)
	trending := make([]gin.H, 0)
	topicFreq := make(map[string]int)
	
	// Count frequencies
	for _, topic := range topics {
		topicFreq[topic]++
	}
	
	// Create trending topics
	topicList := make([]struct{
		topic string
		freq int
	}, 0, len(topicFreq))
	
	for topic, freq := range topicFreq {
		topicList = append(topicList, struct{
			topic string
			freq int
		}{topic, freq})
		
		category := ah.categorizeTopicByKeywords(topic)
		categories[category]++
	}
	
	// Sort by frequency (simple approach)
	for i := 0; i < len(topicList) && i < 10; i++ {
		trending = append(trending, gin.H{
			"topic":     topicList[i].topic,
			"frequency": topicList[i].freq,
			"category":  ah.categorizeTopicByKeywords(topicList[i].topic),
		})
	}
	
	return gin.H{
		"total":      len(topics),
		"unique":     len(topicFreq),
		"trending":   trending,
		"categories": categories,
	}
}

// categorizeTopicByKeywords categorizes topics based on keywords
func (ah *AdvancedHandlers) categorizeTopicByKeywords(topic string) string {
	topicLower := strings.ToLower(topic)
	
	if strings.Contains(topicLower, "error") || strings.Contains(topicLower, "bug") || strings.Contains(topicLower, "issue") {
		return "errors"
	} else if strings.Contains(topicLower, "feature") || strings.Contains(topicLower, "implement") {
		return "features"
	} else if strings.Contains(topicLower, "refactor") || strings.Contains(topicLower, "optimize") {
		return "improvements"
	} else if strings.Contains(topicLower, "test") || strings.Contains(topicLower, "spec") {
		return "testing"
	} else if strings.Contains(topicLower, "config") || strings.Contains(topicLower, "setup") {
		return "configuration"
	}
	return "general"
}

// analyzeSessionPatterns analyzes patterns in sessions
func (ah *AdvancedHandlers) analyzeSessionPatterns(sessions []*types.Session) gin.H {
	if len(sessions) == 0 {
		return gin.H{}
	}
	
	// Session size distribution
	sizeDistribution := make(map[string]int)
	compressionDistribution := make(map[string]int)
	modelUsage := make(map[string]int)
	
	totalSize := int64(0)
	totalCompressed := int64(0)
	
	for _, session := range sessions {
		// Size categories
		if session.OriginalSize < 10000 {
			sizeDistribution["small"]++
		} else if session.OriginalSize < 100000 {
			sizeDistribution["medium"]++
		} else {
			sizeDistribution["large"]++
		}
		
		// Compression quality
		if session.CompressionRatio == 0 {
			compressionDistribution["none"]++
		} else if session.CompressionRatio < 0.3 {
			compressionDistribution["high"]++
		} else if session.CompressionRatio < 0.7 {
			compressionDistribution["medium"]++
		} else {
			compressionDistribution["low"]++
		}
		
		// Model usage
		if session.CompressionModel != nil {
			modelUsage[*session.CompressionModel]++
		}
		
		totalSize += session.OriginalSize
		totalCompressed += session.CompressedSize
	}
	
	avgCompressionRatio := float64(0)
	if totalSize > 0 {
		avgCompressionRatio = float64(totalCompressed) / float64(totalSize)
	}
	
	return gin.H{
		"size_distribution":        sizeDistribution,
		"compression_distribution": compressionDistribution,
		"model_usage":             modelUsage,
		"average_compression":     avgCompressionRatio,
		"total_original_size":     totalSize,
		"total_compressed_size":   totalCompressed,
	}
}

// analyzeDecisions provides decision impact analysis
func (ah *AdvancedHandlers) analyzeDecisions(decisions []string) gin.H {
	if len(decisions) == 0 {
		return gin.H{
			"total": 0,
			"recent": []gin.H{},
			"categories": gin.H{},
		}
	}
	
	categories := make(map[string]int)
	recent := make([]gin.H, 0)
	
	for i, decision := range decisions {
		// Simple categorization based on keywords
		category := ah.categorizeDecisionByKeywords(decision)
		categories[category]++
		
		// Top 5 recent decisions
		if i < 5 {
			recent = append(recent, gin.H{
				"decision": decision,
				"category": category,
			})
		}
	}
	
	return gin.H{
		"total":      len(decisions),
		"recent":     recent,
		"categories": categories,
	}
}

// categorizeDecisionByKeywords categorizes decisions based on keywords
func (ah *AdvancedHandlers) categorizeDecisionByKeywords(decision string) string {
	decisionLower := strings.ToLower(decision)
	
	if strings.Contains(decisionLower, "implement") || strings.Contains(decisionLower, "add") {
		return "implementation"
	} else if strings.Contains(decisionLower, "fix") || strings.Contains(decisionLower, "resolve") {
		return "bugfix"
	} else if strings.Contains(decisionLower, "refactor") || strings.Contains(decisionLower, "improve") {
		return "improvement"
	} else if strings.Contains(decisionLower, "remove") || strings.Contains(decisionLower, "delete") {
		return "removal"
	} else if strings.Contains(decisionLower, "change") || strings.Contains(decisionLower, "update") {
		return "modification"
	}
	return "other"
}

// calculatePerformanceMetrics calculates performance metrics
func (ah *AdvancedHandlers) calculatePerformanceMetrics(sessions []*types.Session) gin.H {
	if len(sessions) == 0 {
		return gin.H{}
	}
	
	totalProcessingTime := int64(0)
	successfulSessions := 0
	
	for _, session := range sessions {
		if session.ProcessingTimeMs != nil {
			totalProcessingTime += *session.ProcessingTimeMs
		}
		if session.Status == "compressed" {
			successfulSessions++
		}
	}
	
	avgProcessingTime := float64(totalProcessingTime) / float64(len(sessions))
	successRate := float64(successfulSessions) / float64(len(sessions)) * 100
	
	return gin.H{
		"average_processing_time_ms": avgProcessingTime,
		"success_rate":              successRate,
		"total_sessions":            len(sessions),
		"successful_sessions":       successfulSessions,
	}
}

// GetSessionVisualization returns visualization data for a single session
func (ah *AdvancedHandlers) GetSessionVisualization(c *gin.Context) {
	sessionID := c.Param("id")
	if sessionID == "" {
		ah.errorResponse(c, http.StatusBadRequest, "Session ID is required", nil)
		return
	}

	ctx := c.Request.Context()

	// Get session details
	session, err := ah.db.GetSession(ctx, sessionID)
	if err != nil {
		ah.errorResponse(c, http.StatusNotFound, "Session not found", err)
		return
	}

	// Get topics for visualization
	topics, err := ah.db.GetSessionTopics(ctx, sessionID)
	if err != nil {
		topics = []types.Topic{} // Empty if not found
	}

	// Get decisions for visualization
	decisions, err := ah.db.GetSessionDecisions(ctx, sessionID)
	if err != nil {
		decisions = []types.Decision{} // Empty if not found
	}

	// Build visualization data
	visualization := gin.H{
		"session": gin.H{
			"id":                session.ID,
			"name":              session.Name,
			"created_at":        session.CreatedAt,
			"status":            session.Status,
			"original_size":     session.OriginalSize,
			"compressed_size":   session.CompressedSize,
			"compression_ratio": session.CompressionRatio,
		},
		"topic_network": ah.buildTopicNetwork(topics),
		"decision_flow":  ah.buildDecisionFlow(decisions),
		"metrics": gin.H{
			"topic_count":    len(topics),
			"decision_count": len(decisions),
			"complexity":     ah.calculateSessionComplexity(topics, decisions),
		},
		"timeline": ah.buildSessionTimeline(session, topics, decisions),
	}

	c.JSON(http.StatusOK, visualization)
}

// GetProjectGraph returns project-level graph data for visualization
func (ah *AdvancedHandlers) GetProjectGraph(c *gin.Context) {
	projectID := c.Param("id")
	if projectID == "" {
		ah.errorResponse(c, http.StatusBadRequest, "Project ID is required", nil)
		return
	}

	days := c.DefaultQuery("days", "30")
	daysInt, _ := strconv.Atoi(days)

	ctx := c.Request.Context()

	// Get project sessions
	sessions, err := ah.db.ListSessions(ctx, 100, 0, &projectID)
	if err != nil {
		ah.errorResponse(c, http.StatusInternalServerError, "Failed to get project sessions", err)
		return
	}

	// Filter by time if needed
	if daysInt > 0 {
		cutoff := time.Now().AddDate(0, 0, -daysInt)
		filteredSessions := make([]*types.Session, 0)
		for _, session := range sessions {
			if session.CreatedAt.After(cutoff) {
				filteredSessions = append(filteredSessions, session)
			}
		}
		sessions = filteredSessions
	}

	// Build graph data
	graph := ah.buildProjectGraph(ctx, sessions)

	c.JSON(http.StatusOK, graph)
}

// GetProjectHeatmap returns heatmap data for project activity
func (ah *AdvancedHandlers) GetProjectHeatmap(c *gin.Context) {
	projectID := c.Param("id")
	if projectID == "" {
		ah.errorResponse(c, http.StatusBadRequest, "Project ID is required", nil)
		return
	}

	days := c.DefaultQuery("days", "90")
	daysInt, _ := strconv.Atoi(days)

	ctx := c.Request.Context()

	// Get project sessions for heatmap
	sessions, err := ah.db.ListSessions(ctx, 1000, 0, &projectID)
	if err != nil {
		ah.errorResponse(c, http.StatusInternalServerError, "Failed to get project sessions", err)
		return
	}

	// Build heatmap data
	heatmap := ah.buildProjectHeatmap(sessions, daysInt)

	c.JSON(http.StatusOK, heatmap)
}

// GetWorkflowFlow returns workflow visualization data
func (ah *AdvancedHandlers) GetWorkflowFlow(c *gin.Context) {
	projectID := c.Param("id")
	if projectID == "" {
		ah.errorResponse(c, http.StatusBadRequest, "Project ID is required", nil)
		return
	}

	ctx := c.Request.Context()

	// Get workflow patterns from memory system
	memory, err := ah.memorySystem.GetProjectMemory(ctx, projectID)
	if err != nil {
		// Try to build it
		memory, err = ah.memorySystem.ConsolidateProjectMemory(ctx, projectID)
		if err != nil {
			ah.errorResponse(c, http.StatusInternalServerError, "Failed to get workflow data", err)
			return
		}
	}

	// Build workflow flow
	workflowFlow := gin.H{
		"patterns": memory.Patterns,
		"timeline": memory.Timeline,
		"flow":     ah.buildWorkflowFlow(memory.Patterns, memory.Timeline),
	}

	c.JSON(http.StatusOK, workflowFlow)
}

// buildTopicNetwork creates a network visualization of topics
func (ah *AdvancedHandlers) buildTopicNetwork(topics []types.Topic) gin.H {
	if len(topics) == 0 {
		return gin.H{
			"nodes": []gin.H{},
			"edges": []gin.H{},
		}
	}

	nodes := make([]gin.H, 0, len(topics))
	edges := make([]gin.H, 0)

	// Create nodes for topics
	for i, topic := range topics {
		size := int(topic.RelevanceScore * 50) + 10 // Base size + relevance
		nodes = append(nodes, gin.H{
			"id":         fmt.Sprintf("topic-%d", i),
			"label":      topic.Topic,
			"size":       size,
			"relevance":  topic.RelevanceScore,
			"frequency":  topic.Frequency,
			"category":   ah.categorizeTopicByKeywords(topic.Topic),
		})
	}

	// Create edges based on topic co-occurrence or similarity
	for i := 0; i < len(topics); i++ {
		for j := i + 1; j < len(topics); j++ {
			if ah.areTopicsRelated(topics[i].Topic, topics[j].Topic) {
				edges = append(edges, gin.H{
					"source": fmt.Sprintf("topic-%d", i),
					"target": fmt.Sprintf("topic-%d", j),
					"weight": 1,
				})
			}
		}
	}

	return gin.H{
		"nodes": nodes,
		"edges": edges,
	}
}

// areTopicsRelated checks if two topics are related (simple word overlap)
func (ah *AdvancedHandlers) areTopicsRelated(topic1, topic2 string) bool {
	words1 := strings.Fields(strings.ToLower(topic1))
	words2 := strings.Fields(strings.ToLower(topic2))
	
	commonWords := 0
	for _, w1 := range words1 {
		for _, w2 := range words2 {
			if w1 == w2 && len(w1) > 3 { // Ignore short words
				commonWords++
			}
		}
	}
	
	return commonWords >= 1
}

// buildDecisionFlow creates a flow diagram for decisions
func (ah *AdvancedHandlers) buildDecisionFlow(decisions []types.Decision) gin.H {
	if len(decisions) == 0 {
		return gin.H{
			"steps": []gin.H{},
		}
	}

	steps := make([]gin.H, 0, len(decisions))

	for i, decision := range decisions {
		step := gin.H{
			"id":          fmt.Sprintf("decision-%d", i),
			"title":       decision.DecisionText,
			"importance":  decision.ImportanceScore,
			"timestamp":   decision.CreatedAt,
			"category":    ah.categorizeDecisionByKeywords(decision.DecisionText),
		}

		if decision.Reasoning != nil {
			step["reasoning"] = *decision.Reasoning
		}
		if decision.Outcome != nil {
			step["outcome"] = *decision.Outcome
		}

		steps = append(steps, step)
	}

	return gin.H{
		"steps": steps,
	}
}

// calculateSessionComplexity calculates a complexity score for the session
func (ah *AdvancedHandlers) calculateSessionComplexity(topics []types.Topic, decisions []types.Decision) float64 {
	if len(topics) == 0 && len(decisions) == 0 {
		return 0.0
	}

	// Base complexity from counts
	complexity := float64(len(topics))*0.3 + float64(len(decisions))*0.5

	// Add relevance-based complexity
	for _, topic := range topics {
		complexity += topic.RelevanceScore * 0.2
	}

	// Add importance-based complexity
	for _, decision := range decisions {
		complexity += decision.ImportanceScore * 0.3
	}

	// Normalize to 0-10 scale
	return math.Min(complexity/5.0*10, 10.0)
}

// buildSessionTimeline creates a timeline for session events
func (ah *AdvancedHandlers) buildSessionTimeline(session *types.Session, topics []types.Topic, decisions []types.Decision) gin.H {
	events := make([]gin.H, 0)

	// Add session creation
	events = append(events, gin.H{
		"timestamp": session.CreatedAt,
		"type":      "session_created",
		"title":     "Session Created",
		"details":   session.Name,
	})

	// Add topic events
	for _, topic := range topics {
		if topic.FirstMentionedAt != nil {
			events = append(events, gin.H{
				"timestamp": *topic.FirstMentionedAt,
				"type":      "topic_introduced",
				"title":     "Topic: " + topic.Topic,
				"details":   fmt.Sprintf("Relevance: %.2f", topic.RelevanceScore),
			})
		}
	}

	// Add decision events
	for _, decision := range decisions {
		events = append(events, gin.H{
			"timestamp": decision.CreatedAt,
			"type":      "decision_made",
			"title":     decision.DecisionText,
			"details":   fmt.Sprintf("Importance: %.2f", decision.ImportanceScore),
		})
	}

	return gin.H{
		"events": events,
	}
}

// buildProjectGraph creates a graph representation of project sessions
func (ah *AdvancedHandlers) buildProjectGraph(ctx context.Context, sessions []*types.Session) gin.H {
	nodes := make([]gin.H, 0, len(sessions))
	edges := make([]gin.H, 0)

	// Create session nodes
	for _, session := range sessions {
		size := int(session.CompressionRatio*30) + 10
		if session.CompressionRatio == 0 {
			size = 10
		}

		nodes = append(nodes, gin.H{
			"id":                session.ID,
			"label":             session.Name,
			"size":              size,
			"created_at":        session.CreatedAt,
			"compression_ratio": session.CompressionRatio,
			"status":            session.Status,
			"category":          ah.categorizeSessionByStatus(session.Status),
		})
	}

	// Create edges based on temporal proximity and topic similarity
	for i := 0; i < len(sessions); i++ {
		for j := i + 1; j < len(sessions); j++ {
			if ah.areSessionsRelated(ctx, sessions[i], sessions[j]) {
				edges = append(edges, gin.H{
					"source": sessions[i].ID,
					"target": sessions[j].ID,
					"weight": 1,
				})
			}
		}
	}

	return gin.H{
		"nodes": nodes,
		"edges": edges,
	}
}

// categorizeSessionByStatus categorizes sessions by their status
func (ah *AdvancedHandlers) categorizeSessionByStatus(status string) string {
	switch status {
	case "compressed":
		return "completed"
	case "processing":
		return "active"
	case "error":
		return "failed"
	default:
		return "unknown"
	}
}

// areSessionsRelated checks if two sessions are related
func (ah *AdvancedHandlers) areSessionsRelated(ctx context.Context, session1, session2 *types.Session) bool {
	// Check temporal proximity (within 24 hours)
	timeDiff := session1.CreatedAt.Sub(session2.CreatedAt)
	if timeDiff < 0 {
		timeDiff = -timeDiff
	}
	
	return timeDiff < 24*time.Hour
}

// buildProjectHeatmap creates heatmap data for project activity
func (ah *AdvancedHandlers) buildProjectHeatmap(sessions []*types.Session, days int) gin.H {
	if len(sessions) == 0 {
		return gin.H{
			"data": []gin.H{},
		}
	}

	// Create date buckets
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -days)
	
	dailyActivity := make(map[string]int)
	hourlyActivity := make(map[int]int)
	weeklyActivity := make(map[string]int)

	for _, session := range sessions {
		if session.CreatedAt.After(startDate) && session.CreatedAt.Before(endDate) {
			// Daily activity
			dateKey := session.CreatedAt.Format("2006-01-02")
			dailyActivity[dateKey]++
			
			// Hourly activity
			hour := session.CreatedAt.Hour()
			hourlyActivity[hour]++
			
			// Weekly activity
			weekday := session.CreatedAt.Weekday().String()
			weeklyActivity[weekday]++
		}
	}

	// Convert to heatmap format
	heatmapData := make([]gin.H, 0)
	for dateStr, count := range dailyActivity {
		heatmapData = append(heatmapData, gin.H{
			"date":  dateStr,
			"count": count,
		})
	}

	return gin.H{
		"daily":   heatmapData,
		"hourly":  hourlyActivity,
		"weekly":  weeklyActivity,
		"period": gin.H{
			"start": startDate,
			"end":   endDate,
			"days":  days,
		},
	}
}

// buildWorkflowFlow creates workflow flow visualization
func (ah *AdvancedHandlers) buildWorkflowFlow(patterns []ai.Pattern, timeline []ai.TimelineEvent) gin.H {
	flows := make([]gin.H, 0)
	
	// Extract workflow patterns
	for _, pattern := range patterns {
		if pattern.Type == "workflow_pattern" {
			flows = append(flows, gin.H{
				"pattern":    pattern.Description,
				"frequency":  pattern.Occurrences,
				"examples":   pattern.Examples,
				"recommendation": pattern.Recommendation,
			})
		}
	}

	// Build flow steps from timeline
	steps := make([]gin.H, 0)
	for _, event := range timeline {
		steps = append(steps, gin.H{
			"timestamp": event.Timestamp,
			"type":      event.Type,
			"title":     event.Description,
			"impact":    event.Impact,
		})
	}

	return gin.H{
		"patterns": flows,
		"steps":    steps,
	}
}