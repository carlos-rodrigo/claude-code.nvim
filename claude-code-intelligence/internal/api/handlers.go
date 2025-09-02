package api

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"claude-code-intelligence/internal/ai"
	"claude-code-intelligence/internal/config"
	"claude-code-intelligence/internal/database"
	"claude-code-intelligence/internal/types"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// loggerInterface defines the interface for logger implementations
type loggerInterface interface {
	WithFields(fields map[string]interface{}) loggerInterface
	WithError(err error) loggerInterface
	WithField(key string, value interface{}) loggerInterface
	Debug(args ...interface{})
	Info(args ...interface{})
	Warn(args ...interface{})
	Error(args ...interface{})
	Fatal(args ...interface{})
	Panic(args ...interface{})
}

// logrusWrapper wraps logrus.Logger to implement loggerInterface
type logrusWrapper struct {
	*logrus.Logger
}

func (l *logrusWrapper) WithFields(fields map[string]interface{}) loggerInterface {
	return &logrusWrapper{l.Logger.WithFields(fields).Logger}
}

func (l *logrusWrapper) WithError(err error) loggerInterface {
	return &logrusWrapper{l.Logger.WithError(err).Logger}
}

func (l *logrusWrapper) WithField(key string, value interface{}) loggerInterface {
	return &logrusWrapper{l.Logger.WithField(key, value).Logger}
}

// newLoggerWrapper creates a new logger wrapper
func newLoggerWrapper(logger *logrus.Logger) loggerInterface {
	return &logrusWrapper{logger}
}

// Handlers contains all HTTP handlers
type Handlers struct {
	db       *database.Manager
	ollama   *ai.OllamaClient
	config   *config.Config
	logger   *logrus.Logger
	startTime time.Time
}

// NewHandlers creates a new handlers instance
func NewHandlers(db *database.Manager, ollama *ai.OllamaClient, cfg *config.Config, logger *logrus.Logger) *Handlers {
	return &Handlers{
		db:        db,
		ollama:    ollama,
		config:    cfg,
		logger:    logger,
		startTime: time.Now(),
	}
}

// Health Check Handlers

// HealthCheck returns the overall health status
func (h *Handlers) HealthCheck(c *gin.Context) {
	ctx := c.Request.Context()
	
	status := &types.HealthStatus{
		Status:    "healthy",
		Timestamp: time.Now(),
		Uptime:    time.Since(h.startTime),
		Version:   "1.0.0",
		Components: map[string]types.ComponentHealth{
			"database": h.db.HealthCheck(ctx),
			"ollama":   h.ollama.HealthCheck(ctx),
		},
	}

	// Check if any component is unhealthy
	overallHealthy := true
	for _, component := range status.Components {
		if component.Status != "healthy" {
			overallHealthy = false
			break
		}
	}

	if !overallHealthy {
		status.Status = "degraded"
		c.JSON(http.StatusServiceUnavailable, status)
		return
	}

	c.JSON(http.StatusOK, status)
}

// Session Management Handlers

// CreateSession creates a new session
func (h *Handlers) CreateSession(c *gin.Context) {
	var session types.Session
	if err := c.ShouldBindJSON(&session); err != nil {
		h.errorResponse(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	ctx := c.Request.Context()
	if err := h.db.CreateSession(ctx, &session); err != nil {
		h.errorResponse(c, http.StatusInternalServerError, "Failed to create session", err)
		return
	}

	c.JSON(http.StatusCreated, session)
}

// GetSession retrieves a session by ID
func (h *Handlers) GetSession(c *gin.Context) {
	sessionID := c.Param("id")
	if sessionID == "" {
		h.errorResponse(c, http.StatusBadRequest, "Session ID is required", nil)
		return
	}

	ctx := c.Request.Context()
	session, err := h.db.GetSession(ctx, sessionID)
	if err != nil {
		h.errorResponse(c, http.StatusNotFound, "Session not found", err)
		return
	}

	c.JSON(http.StatusOK, session)
}

// ListSessions lists sessions with pagination
func (h *Handlers) ListSessions(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	projectID := c.Query("project_id")

	if limit > 100 {
		limit = 100 // Prevent abuse
	}

	ctx := c.Request.Context()
	var projectIDPtr *string
	if projectID != "" {
		projectIDPtr = &projectID
	}

	sessions, err := h.db.ListSessions(ctx, limit, offset, projectIDPtr)
	if err != nil {
		h.errorResponse(c, http.StatusInternalServerError, "Failed to list sessions", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"sessions": sessions,
		"limit":    limit,
		"offset":   offset,
		"count":    len(sessions),
	})
}

// AI Operations Handlers

// CompressSession compresses session content using AI
func (h *Handlers) CompressSession(c *gin.Context) {
	var req types.CompressionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.errorResponse(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	if req.Content == "" {
		h.errorResponse(c, http.StatusBadRequest, "Content is required", nil)
		return
	}

	ctx := c.Request.Context()

	// Set defaults
	if req.Options.Style == "" {
		req.Options.Style = "balanced"
	}
	if req.Options.MaxLength == 0 {
		req.Options.MaxLength = 2000
	}
	if req.Options.Priority == "" {
		req.Options.Priority = "balanced"
	}
	req.Options.AllowFallback = true // Always allow fallback for API requests

	h.logger.WithFields(logrus.Fields{
		"session_id":   req.SessionID,
		"content_size": len(req.Content),
		"model":        req.Options.Model,
		"style":        req.Options.Style,
	}).Info("Starting session compression")

	result, err := h.ollama.CompressSession(ctx, req.Content, req.Options)
	if err != nil {
		h.errorResponse(c, http.StatusInternalServerError, "Compression failed", err)
		return
	}

	// Update session in database if session ID provided
	if req.SessionID != "" {
		session, getErr := h.db.GetSession(ctx, req.SessionID)
		if getErr == nil {
			session.Status = string(types.StatusCompressed)
			session.CompressedSize = int64(result.CompressedSize)
			session.CompressionRatio = result.CompressionRatio
			session.CompressionModel = &result.Model
			session.Summary = &result.Summary
			processingTimeMs := int64(result.ProcessingTime.Nanoseconds() / 1e6)
			session.ProcessingTimeMs = &processingTimeMs

			if updateErr := h.db.UpdateSession(ctx, session); updateErr != nil {
				h.logger.WithError(updateErr).Warn("Failed to update session after compression")
			}
		}
	}

	// Track model performance
	go func() {
		bgCtx := context.Background()
		_ = h.db.TrackModelPerformance(bgCtx, result.Model, "compression", 
			true, result.ProcessingTime, result.QualityScore)
	}()

	c.JSON(http.StatusOK, result)
}

// ExtractTopics extracts topics from session content
func (h *Handlers) ExtractTopics(c *gin.Context) {
	var req struct {
		Content   string `json:"content" binding:"required"`
		MaxTopics int    `json:"max_topics"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.errorResponse(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	if req.MaxTopics == 0 {
		req.MaxTopics = 10
	}

	ctx := c.Request.Context()
	topics, err := h.ollama.ExtractTopics(ctx, req.Content, req.MaxTopics)
	if err != nil {
		h.errorResponse(c, http.StatusInternalServerError, "Topic extraction failed", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"topics": topics,
		"count":  len(topics),
	})
}

// TestModels tests multiple models with sample content
func (h *Handlers) TestModels(c *gin.Context) {
	if !h.config.Features.ModelTesting {
		h.errorResponse(c, http.StatusForbidden, "Model testing is disabled", nil)
		return
	}

	var req struct {
		Content string   `json:"content" binding:"required"`
		Models  []string `json:"models"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.errorResponse(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	ctx := c.Request.Context()
	results, err := h.ollama.TestModels(ctx, req.Content, req.Models)
	if err != nil {
		h.errorResponse(c, http.StatusInternalServerError, "Model testing failed", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"results": results,
		"count":   len(results),
	})
}

// Search Handlers

// SearchSessions performs semantic search on sessions
func (h *Handlers) SearchSessions(c *gin.Context) {
	var req types.SearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.errorResponse(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	if req.Query == "" {
		h.errorResponse(c, http.StatusBadRequest, "Search query is required", nil)
		return
	}

	if req.Limit == 0 {
		req.Limit = 10
	}
	if req.Limit > 50 {
		req.Limit = 50 // Prevent abuse
	}

	ctx := c.Request.Context()
	
	// For now, use simple text search until we implement embeddings
	results, err := h.db.SearchSessions(ctx, req.Query, req.Limit)
	if err != nil {
		h.errorResponse(c, http.StatusInternalServerError, "Search failed", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"results": results,
		"query":   req.Query,
		"count":   len(results),
	})
}

// Configuration and Status Handlers

// GetConfig returns the current configuration (sanitized)
func (h *Handlers) GetConfig(c *gin.Context) {
	// Return sanitized config without sensitive information
	config := map[string]interface{}{
		"server": map[string]interface{}{
			"env": h.config.Server.Env,
		},
		"ollama": map[string]interface{}{
			"url":            h.config.Ollama.URL,
			"primary_model":  h.config.Ollama.PrimaryModel,
			"fallback_model": h.config.Ollama.FallbackModel,
		},
		"features": h.config.Features,
		"model_presets": h.config.ModelPresets,
	}

	c.JSON(http.StatusOK, config)
}

// GetStats returns database and service statistics
func (h *Handlers) GetStats(c *gin.Context) {
	ctx := c.Request.Context()
	
	dbStats, err := h.db.GetStats(ctx)
	if err != nil {
		h.errorResponse(c, http.StatusInternalServerError, "Failed to get database stats", err)
		return
	}

	modelPerformance, err := h.db.GetModelPerformance(ctx)
	if err != nil {
		h.errorResponse(c, http.StatusInternalServerError, "Failed to get model performance", err)
		return
	}

	availableModels := h.ollama.GetAvailableModels()
	modelNames := make([]string, len(availableModels))
	for i, model := range availableModels {
		modelNames[i] = model.Name
	}

	stats := map[string]interface{}{
		"service": map[string]interface{}{
			"uptime":           time.Since(h.startTime).String(),
			"version":          "1.0.0",
			"available_models": modelNames,
		},
		"database":         dbStats,
		"model_performance": modelPerformance,
	}

	c.JSON(http.StatusOK, stats)
}

// GetAvailableModels returns the list of available Ollama models
func (h *Handlers) GetAvailableModels(c *gin.Context) {
	models := h.ollama.GetAvailableModels()
	
	// Format response
	response := make([]map[string]interface{}, len(models))
	for i, model := range models {
		response[i] = map[string]interface{}{
			"name":        model.Name,
			"size":        model.Size,
			"digest":      model.Digest,
			"modified_at": model.ModifiedAt,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"models": response,
		"count":  len(response),
	})
}

// Model Management Handlers

// InstallModel installs a specific model
func (h *Handlers) InstallModel(c *gin.Context) {
	modelName := c.Param("model")
	if modelName == "" {
		h.errorResponse(c, http.StatusBadRequest, "Model name is required", nil)
		return
	}

	ctx := c.Request.Context()

	// This will install the model if it's not available
	if _, err := h.ollama.CompressSession(ctx, "test", types.CompressionOptions{
		Model:         &modelName,
		Style:         "concise",
		MaxLength:     100,
		AllowFallback: false,
	}); err != nil {
		h.errorResponse(c, http.StatusInternalServerError, "Failed to install model", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("Model %s installed successfully", modelName),
		"model":   modelName,
	})
}

// Utility methods

func (h *Handlers) errorResponse(c *gin.Context, code int, message string, err error) {
	response := types.APIError{
		Code:    code,
		Message: message,
	}

	if err != nil {
		response.Details = err.Error()
		h.logger.WithError(err).Error(message)
	}

	c.JSON(code, response)
}