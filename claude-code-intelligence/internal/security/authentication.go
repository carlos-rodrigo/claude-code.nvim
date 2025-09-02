package security

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// AuthenticationManager handles API key based authentication
type AuthenticationManager struct {
	apiKeys map[string]*APIKey
	logger  *logrus.Logger
}

// APIKey represents an API key with metadata
type APIKey struct {
	Key         string    `json:"key"`
	Name        string    `json:"name"`
	CreatedAt   time.Time `json:"created_at"`
	LastUsed    time.Time `json:"last_used"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	Permissions []string  `json:"permissions"`
	Enabled     bool      `json:"enabled"`
	RateLimit   int       `json:"rate_limit"` // requests per minute
}

// AuthContext contains authentication information
type AuthContext struct {
	APIKey      *APIKey `json:"api_key"`
	Permissions []string `json:"permissions"`
}

// NewAuthenticationManager creates a new authentication manager
func NewAuthenticationManager(logger *logrus.Logger) *AuthenticationManager {
	am := &AuthenticationManager{
		apiKeys: make(map[string]*APIKey),
		logger:  logger,
	}

	// Create a default admin API key for development
	adminKey, err := am.generateAPIKey()
	if err != nil {
		logger.WithError(err).Error("Failed to generate admin API key")
	} else {
		am.apiKeys[adminKey] = &APIKey{
			Key:         adminKey,
			Name:        "admin",
			CreatedAt:   time.Now(),
			LastUsed:    time.Now(),
			ExpiresAt:   nil, // Never expires
			Permissions: []string{"*"}, // All permissions
			Enabled:     true,
			RateLimit:   1000, // 1000 requests per minute
		}
		logger.WithField("api_key", adminKey[:8]+"...").Info("Created default admin API key")
	}

	return am
}

// AuthenticationMiddleware creates middleware for API key authentication
func (am *AuthenticationManager) AuthenticationMiddleware() gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		// Skip authentication for health checks and public endpoints
		if am.isPublicEndpoint(c.Request.URL.Path) {
			c.Next()
			return
		}

		// Extract API key from header
		apiKey := c.GetHeader("X-API-Key")
		if apiKey == "" {
			// Try Authorization header with Bearer token
			auth := c.GetHeader("Authorization")
			if strings.HasPrefix(auth, "Bearer ") {
				apiKey = strings.TrimPrefix(auth, "Bearer ")
			}
		}

		if apiKey == "" {
			am.logger.WithFields(logrus.Fields{
				"path":      c.Request.URL.Path,
				"client_ip": c.ClientIP(),
			}).Warn("Missing API key")

			c.JSON(401, gin.H{
				"error":     "Authentication required",
				"message":   "API key is required. Provide it via X-API-Key header or Authorization: Bearer header",
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			})
			c.Abort()
			return
		}

		// Validate API key
		key, valid := am.validateAPIKey(apiKey)
		if !valid {
			am.logger.WithFields(logrus.Fields{
				"api_key_prefix": apiKey[:8] + "...",
				"path":           c.Request.URL.Path,
				"client_ip":      c.ClientIP(),
			}).Warn("Invalid API key")

			c.JSON(401, gin.H{
				"error":     "Invalid API key",
				"message":   "The provided API key is not valid or has been disabled",
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			})
			c.Abort()
			return
		}

		// Update last used time
		key.LastUsed = time.Now()

		// Set authentication context
		authCtx := &AuthContext{
			APIKey:      key,
			Permissions: key.Permissions,
		}
		c.Set("auth_context", authCtx)

		am.logger.WithFields(logrus.Fields{
			"api_key_name": key.Name,
			"path":         c.Request.URL.Path,
			"client_ip":    c.ClientIP(),
		}).Debug("Request authenticated")

		c.Next()
	})
}

// AuthorizationMiddleware creates middleware for permission-based authorization
func (am *AuthenticationManager) AuthorizationMiddleware(requiredPermission string) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		// Skip authorization for public endpoints
		if am.isPublicEndpoint(c.Request.URL.Path) {
			c.Next()
			return
		}

		authCtx, exists := c.Get("auth_context")
		if !exists {
			c.JSON(403, gin.H{
				"error":     "Authorization failed",
				"message":   "No authentication context found",
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			})
			c.Abort()
			return
		}

		auth := authCtx.(*AuthContext)

		// Check if user has required permission
		if !am.hasPermission(auth.Permissions, requiredPermission) {
			am.logger.WithFields(logrus.Fields{
				"api_key_name":        auth.APIKey.Name,
				"required_permission": requiredPermission,
				"user_permissions":    auth.Permissions,
				"path":                c.Request.URL.Path,
			}).Warn("Access denied - insufficient permissions")

			c.JSON(403, gin.H{
				"error":               "Access denied",
				"message":             "Insufficient permissions for this operation",
				"required_permission": requiredPermission,
				"timestamp":           time.Now().UTC().Format(time.RFC3339),
			})
			c.Abort()
			return
		}

		c.Next()
	})
}

// generateAPIKey generates a new API key
func (am *AuthenticationManager) generateAPIKey() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// CreateAPIKey creates a new API key
func (am *AuthenticationManager) CreateAPIKey(name string, permissions []string, expiresIn *time.Duration, rateLimit int) (*APIKey, error) {
	key, err := am.generateAPIKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate API key: %w", err)
	}

	var expiresAt *time.Time
	if expiresIn != nil {
		expTime := time.Now().Add(*expiresIn)
		expiresAt = &expTime
	}

	apiKey := &APIKey{
		Key:         key,
		Name:        name,
		CreatedAt:   time.Now(),
		LastUsed:    time.Now(),
		ExpiresAt:   expiresAt,
		Permissions: permissions,
		Enabled:     true,
		RateLimit:   rateLimit,
	}

	am.apiKeys[key] = apiKey

	am.logger.WithFields(logrus.Fields{
		"name":        name,
		"permissions": permissions,
		"rate_limit":  rateLimit,
		"expires_at":  expiresAt,
	}).Info("Created new API key")

	return apiKey, nil
}

// validateAPIKey validates an API key
func (am *AuthenticationManager) validateAPIKey(key string) (*APIKey, bool) {
	apiKey, exists := am.apiKeys[key]
	if !exists {
		return nil, false
	}

	// Check if key is enabled
	if !apiKey.Enabled {
		return nil, false
	}

	// Check if key is expired
	if apiKey.ExpiresAt != nil && time.Now().After(*apiKey.ExpiresAt) {
		return nil, false
	}

	return apiKey, true
}

// hasPermission checks if the user has the required permission
func (am *AuthenticationManager) hasPermission(userPermissions []string, requiredPermission string) bool {
	for _, perm := range userPermissions {
		if perm == "*" || perm == requiredPermission {
			return true
		}
		
		// Check for wildcard permissions (e.g., "read:*" matches "read:sessions")
		if strings.HasSuffix(perm, ":*") {
			prefix := strings.TrimSuffix(perm, ":*")
			if strings.HasPrefix(requiredPermission, prefix+":") {
				return true
			}
		}
	}
	return false
}

// isPublicEndpoint checks if an endpoint is public (doesn't require authentication)
func (am *AuthenticationManager) isPublicEndpoint(path string) bool {
	publicPaths := []string{
		"/health",
		"/ready",
		"/live",
		"/metrics", // Prometheus metrics should be accessible for monitoring
	}

	for _, publicPath := range publicPaths {
		if path == publicPath || strings.HasPrefix(path, publicPath+"/") {
			return true
		}
	}
	return false
}

// RevokeAPIKey revokes an API key
func (am *AuthenticationManager) RevokeAPIKey(key string) error {
	apiKey, exists := am.apiKeys[key]
	if !exists {
		return fmt.Errorf("API key not found")
	}

	apiKey.Enabled = false

	am.logger.WithFields(logrus.Fields{
		"name": apiKey.Name,
		"key":  key[:8] + "...",
	}).Info("API key revoked")

	return nil
}

// ListAPIKeys returns a list of all API keys (without the actual key values)
func (am *AuthenticationManager) ListAPIKeys() []*APIKey {
	var keys []*APIKey
	for _, key := range am.apiKeys {
		// Create a copy without the actual key value for security
		keyCopy := *key
		keyCopy.Key = key.Key[:8] + "..." // Show only first 8 characters
		keys = append(keys, &keyCopy)
	}
	return keys
}

// GetAPIKeyStats returns statistics about API key usage
func (am *AuthenticationManager) GetAPIKeyStats() map[string]interface{} {
	total := len(am.apiKeys)
	enabled := 0
	expired := 0
	neverExpire := 0

	for _, key := range am.apiKeys {
		if key.Enabled {
			enabled++
		}
		if key.ExpiresAt == nil {
			neverExpire++
		} else if time.Now().After(*key.ExpiresAt) {
			expired++
		}
	}

	return map[string]interface{}{
		"total_keys":        total,
		"enabled_keys":      enabled,
		"expired_keys":      expired,
		"never_expire_keys": neverExpire,
		"disabled_keys":     total - enabled,
	}
}

// GetAuthContext retrieves authentication context from Gin context
func GetAuthContext(c *gin.Context) (*AuthContext, bool) {
	authCtx, exists := c.Get("auth_context")
	if !exists {
		return nil, false
	}
	return authCtx.(*AuthContext), true
}

// RequirePermission is a helper function to check permissions in handlers
func RequirePermission(c *gin.Context, permission string) bool {
	authCtx, exists := GetAuthContext(c)
	if !exists {
		return false
	}

	am := &AuthenticationManager{} // This would need proper initialization in real use
	return am.hasPermission(authCtx.Permissions, permission)
}