package security

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// SecurityHandlers contains handlers for security management
type SecurityHandlers struct {
	authManager   *AuthenticationManager
	validator     *InputValidator
	logger        *logrus.Logger
}

// NewSecurityHandlers creates new security handlers
func NewSecurityHandlers(authManager *AuthenticationManager, validator *InputValidator, logger *logrus.Logger) *SecurityHandlers {
	return &SecurityHandlers{
		authManager: authManager,
		validator:   validator,
		logger:      logger,
	}
}

// CreateAPIKey creates a new API key
func (sh *SecurityHandlers) CreateAPIKey(c *gin.Context) {
	var request struct {
		Name           string   `json:"name" binding:"required"`
		Permissions    []string `json:"permissions" binding:"required"`
		RateLimit      int      `json:"rate_limit"`
		ExpiresInDays  *int     `json:"expires_in_days"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		sh.errorResponse(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	// Validate input
	rules := sh.validator.ValidateAPIKeyRequest()
	data := map[string]interface{}{
		"name":             request.Name,
		"permissions":      request.Permissions,
		"rate_limit":       request.RateLimit,
		"expires_in_days":  request.ExpiresInDays,
	}
	
	if result := sh.validator.ValidateData(data, rules); !result.Valid {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":     "Validation failed",
			"errors":    result.Errors,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	// Set default rate limit
	if request.RateLimit <= 0 {
		request.RateLimit = 100 // Default 100 requests per minute
	}

	// Calculate expiration
	var expiresIn *time.Duration
	if request.ExpiresInDays != nil {
		duration := time.Duration(*request.ExpiresInDays) * 24 * time.Hour
		expiresIn = &duration
	}

	// Check authorization
	authCtx, exists := GetAuthContext(c)
	if !exists {
		sh.errorResponse(c, http.StatusUnauthorized, "Authentication required", nil)
		return
	}

	if !sh.authManager.hasPermission(authCtx.Permissions, "admin:api_keys") {
		sh.errorResponse(c, http.StatusForbidden, "Insufficient permissions", nil)
		return
	}

	sh.logger.WithFields(logrus.Fields{
		"name":        request.Name,
		"permissions": request.Permissions,
		"rate_limit":  request.RateLimit,
		"created_by":  authCtx.APIKey.Name,
		"endpoint":    "create_api_key",
	}).Info("Creating new API key")

	// Create API key
	apiKey, err := sh.authManager.CreateAPIKey(request.Name, request.Permissions, expiresIn, request.RateLimit)
	if err != nil {
		sh.errorResponse(c, http.StatusInternalServerError, "Failed to create API key", err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success":    true,
		"message":    "API key created successfully",
		"api_key":    apiKey,
		"created_at": time.Now().UTC().Format(time.RFC3339),
		"warning":    "Store this API key securely. It will not be shown again.",
	})
}

// ListAPIKeys lists all API keys
func (sh *SecurityHandlers) ListAPIKeys(c *gin.Context) {
	// Check authorization
	authCtx, exists := GetAuthContext(c)
	if !exists {
		sh.errorResponse(c, http.StatusUnauthorized, "Authentication required", nil)
		return
	}

	if !sh.authManager.hasPermission(authCtx.Permissions, "admin:api_keys") {
		sh.errorResponse(c, http.StatusForbidden, "Insufficient permissions", nil)
		return
	}

	keys := sh.authManager.ListAPIKeys()
	stats := sh.authManager.GetAPIKeyStats()

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"api_keys":   keys,
		"statistics": stats,
		"retrieved_at": time.Now().UTC().Format(time.RFC3339),
	})
}

// RevokeAPIKey revokes an API key
func (sh *SecurityHandlers) RevokeAPIKey(c *gin.Context) {
	apiKey := c.Param("key")
	if apiKey == "" {
		sh.errorResponse(c, http.StatusBadRequest, "API key is required", nil)
		return
	}

	// Check authorization
	authCtx, exists := GetAuthContext(c)
	if !exists {
		sh.errorResponse(c, http.StatusUnauthorized, "Authentication required", nil)
		return
	}

	if !sh.authManager.hasPermission(authCtx.Permissions, "admin:api_keys") {
		sh.errorResponse(c, http.StatusForbidden, "Insufficient permissions", nil)
		return
	}

	// Prevent self-revocation
	if apiKey == authCtx.APIKey.Key {
		sh.errorResponse(c, http.StatusBadRequest, "Cannot revoke your own API key", nil)
		return
	}

	sh.logger.WithFields(logrus.Fields{
		"api_key":    apiKey[:8] + "...",
		"revoked_by": authCtx.APIKey.Name,
		"endpoint":   "revoke_api_key",
	}).Info("Revoking API key")

	if err := sh.authManager.RevokeAPIKey(apiKey); err != nil {
		sh.errorResponse(c, http.StatusInternalServerError, "Failed to revoke API key", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"message":    "API key revoked successfully",
		"revoked_at": time.Now().UTC().Format(time.RFC3339),
	})
}

// GetAPIKeyStats returns API key statistics
func (sh *SecurityHandlers) GetAPIKeyStats(c *gin.Context) {
	// Check authorization
	authCtx, exists := GetAuthContext(c)
	if !exists {
		sh.errorResponse(c, http.StatusUnauthorized, "Authentication required", nil)
		return
	}

	if !sh.authManager.hasPermission(authCtx.Permissions, "read:api_keys") {
		sh.errorResponse(c, http.StatusForbidden, "Insufficient permissions", nil)
		return
	}

	stats := sh.authManager.GetAPIKeyStats()

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"statistics": stats,
		"retrieved_at": time.Now().UTC().Format(time.RFC3339),
	})
}

// ValidateToken validates an API token
func (sh *SecurityHandlers) ValidateToken(c *gin.Context) {
	token := c.GetHeader("X-API-Key")
	if token == "" {
		token = c.Query("token")
	}

	if token == "" {
		sh.errorResponse(c, http.StatusBadRequest, "Token is required", nil)
		return
	}

	apiKey, valid := sh.authManager.validateAPIKey(token)
	if !valid {
		c.JSON(http.StatusUnauthorized, gin.H{
			"valid":     false,
			"message":   "Invalid or expired token",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"valid":       true,
		"name":        apiKey.Name,
		"permissions": apiKey.Permissions,
		"rate_limit":  apiKey.RateLimit,
		"expires_at":  apiKey.ExpiresAt,
		"last_used":   apiKey.LastUsed,
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
	})
}

// GetSecurityConfig returns security configuration (non-sensitive)
func (sh *SecurityHandlers) GetSecurityConfig(c *gin.Context) {
	config := gin.H{
		"authentication": gin.H{
			"enabled":    true,
			"type":       "api_key",
			"header":     "X-API-Key",
			"alt_header": "Authorization: Bearer",
		},
		"authorization": gin.H{
			"enabled":           true,
			"permission_based":  true,
			"wildcard_support":  true,
		},
		"validation": gin.H{
			"enabled":          true,
			"input_sanitization": true,
			"max_request_size": "10MB",
		},
		"security_headers": gin.H{
			"enabled": true,
			"headers": []string{
				"X-Content-Type-Options",
				"X-Frame-Options", 
				"X-XSS-Protection",
				"Referrer-Policy",
				"Content-Security-Policy",
			},
		},
		"rate_limiting": gin.H{
			"enabled":      true,
			"per_api_key":  true,
			"default_limit": 100,
		},
		"public_endpoints": []string{
			"/health",
			"/ready", 
			"/live",
			"/metrics",
		},
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"config":  config,
	})
}

// GetSecurityEvents returns security-related events/logs
func (sh *SecurityHandlers) GetSecurityEvents(c *gin.Context) {
	// Check authorization
	authCtx, exists := GetAuthContext(c)
	if !exists {
		sh.errorResponse(c, http.StatusUnauthorized, "Authentication required", nil)
		return
	}

	if !sh.authManager.hasPermission(authCtx.Permissions, "read:security_events") {
		sh.errorResponse(c, http.StatusForbidden, "Insufficient permissions", nil)
		return
	}

	// Get query parameters
	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 1000 {
		limit = 50
	}

	eventType := c.Query("type") // authentication, authorization, validation, etc.

	// In a real implementation, you would fetch events from a database or log store
	events := []gin.H{
		{
			"id":        "evt_001",
			"type":      "authentication_failure",
			"timestamp": time.Now().Add(-1 * time.Hour).UTC().Format(time.RFC3339),
			"details": gin.H{
				"ip":     "192.168.1.100",
				"path":   "/api/sessions",
				"reason": "invalid_api_key",
			},
		},
		{
			"id":        "evt_002", 
			"type":      "authorization_denied",
			"timestamp": time.Now().Add(-2 * time.Hour).UTC().Format(time.RFC3339),
			"details": gin.H{
				"ip":                 "192.168.1.101",
				"path":               "/api/admin/keys",
				"api_key":            "admin_key",
				"required_permission": "admin:api_keys",
			},
		},
	}

	// Filter by type if specified
	if eventType != "" {
		var filteredEvents []gin.H
		for _, event := range events {
			if event["type"] == eventType {
				filteredEvents = append(filteredEvents, event)
			}
		}
		events = filteredEvents
	}

	// Apply limit
	if len(events) > limit {
		events = events[:limit]
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"events":  events,
		"count":   len(events),
		"filters": gin.H{
			"type":  eventType,
			"limit": limit,
		},
		"retrieved_at": time.Now().UTC().Format(time.RFC3339),
	})
}

// errorResponse sends a standardized error response
func (sh *SecurityHandlers) errorResponse(c *gin.Context, statusCode int, message string, err error) {
	response := gin.H{
		"success":   false,
		"message":   message,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	if err != nil {
		response["error"] = err.Error()
		sh.logger.WithFields(logrus.Fields{
			"error":       err.Error(),
			"message":     message,
			"status_code": statusCode,
			"path":        c.Request.URL.Path,
			"method":      c.Request.Method,
		}).Error("Security operation failed")
	}

	c.JSON(statusCode, response)
}