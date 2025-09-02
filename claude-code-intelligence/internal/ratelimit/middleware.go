package ratelimit

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// RateLimitMiddleware creates middleware for rate limiting
func RateLimitMiddleware(limiter *RateLimiter, logger *logrus.Logger) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		// Skip rate limiting for health checks and metrics
		if isExemptEndpoint(c.Request.URL.Path) {
			c.Next()
			return
		}

		// Get client ID (use API key if available, otherwise IP)
		clientID := getClientID(c)
		endpoint := getEndpointKey(c.Request.URL.Path, c.Request.Method)
		method := c.Request.Method

		// Get custom rate limits from API key if available
		customRateLimit, customBurstLimit := getCustomLimits(c)

		// Check rate limit
		result := limiter.CheckLimit(clientID, endpoint, method, customRateLimit, customBurstLimit)

		// Set rate limit headers
		setRateLimitHeaders(c, result)

		if !result.Allowed {
			// Log rate limit violation
			logger.WithFields(logrus.Fields{
				"client_id":        clientID,
				"endpoint":         endpoint,
				"method":           method,
				"reason":           result.Reason,
				"retry_after_seconds": result.RetryAfter.Seconds(),
				"client_ip":        c.ClientIP(),
				"user_agent":       c.Request.UserAgent(),
			}).Warn("Rate limit exceeded")

			// Return rate limit error
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "Rate limit exceeded",
				"message":     result.Reason,
				"retry_after": result.RetryAfter.Seconds(),
				"reset_time":  result.ResetTime.UTC().Format(time.RFC3339),
				"timestamp":   time.Now().UTC().Format(time.RFC3339),
			})
			c.Abort()
			return
		}

		// Log successful rate limit check (debug level)
		logger.WithFields(logrus.Fields{
			"client_id":         clientID,
			"endpoint":          endpoint,
			"remaining_tokens":  result.RemainingTokens,
			"reset_time":        result.ResetTime.Format(time.RFC3339),
		}).Debug("Rate limit check passed")

		c.Next()
	})
}

// getClientID extracts client ID for rate limiting
func getClientID(c *gin.Context) string {
	// Try to get API key from authentication context
	if authCtx, exists := c.Get("auth_context"); exists {
		if auth, ok := authCtx.(interface{ GetAPIKey() interface{ GetName() string } }); ok {
			return "api_key:" + auth.GetAPIKey().GetName()
		}
	}

	// Try API key from header
	if apiKey := c.GetHeader("X-API-Key"); apiKey != "" {
		return "api_key:" + apiKey[:8] + "..." // Truncated for privacy
	}

	// Fall back to IP address
	return "ip:" + c.ClientIP()
}

// getEndpointKey creates a standardized endpoint key for rate limiting
func getEndpointKey(path, method string) string {
	// Normalize path by removing IDs and parameters
	normalizedPath := normalizePath(path)
	return method + ":" + normalizedPath
}

// normalizePath normalizes URL paths for rate limiting
func normalizePath(path string) string {
	// Simple normalization - replace common ID patterns
	// In production, you might want more sophisticated path normalization
	
	// Remove trailing slash
	if len(path) > 1 && path[len(path)-1] == '/' {
		path = path[:len(path)-1]
	}

	// Replace UUIDs and common ID patterns with placeholders
	// This is a simplified version - you might want regex-based replacement
	normalizedPath := path
	
	// Replace session IDs, backup filenames, etc.
	if len(path) > 4 {
		// Simple heuristic: if path segment is longer than 8 chars and alphanumeric, treat as ID
		segments := splitPath(path)
		for i, segment := range segments {
			if len(segment) > 8 && isAlphanumeric(segment) {
				segments[i] = "{id}"
			}
		}
		normalizedPath = "/" + joinPath(segments)
	}
	
	return normalizedPath
}

// Helper functions for path normalization
func splitPath(path string) []string {
	if path == "/" || path == "" {
		return []string{}
	}
	
	if path[0] == '/' {
		path = path[1:]
	}
	
	segments := []string{}
	current := ""
	
	for _, char := range path {
		if char == '/' {
			if current != "" {
				segments = append(segments, current)
				current = ""
			}
		} else {
			current += string(char)
		}
	}
	
	if current != "" {
		segments = append(segments, current)
	}
	
	return segments
}

func joinPath(segments []string) string {
	result := ""
	for i, segment := range segments {
		if i > 0 {
			result += "/"
		}
		result += segment
	}
	return result
}

func isAlphanumeric(s string) bool {
	for _, char := range s {
		if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '-' || char == '_') {
			return false
		}
	}
	return true
}

// getCustomLimits extracts custom rate limits from API key
func getCustomLimits(c *gin.Context) (rateLimit, burstLimit int) {
	// Try to get custom limits from authentication context
	if authCtx, exists := c.Get("auth_context"); exists {
		// This is a simplified interface - adjust based on your auth context structure
		if auth, ok := authCtx.(interface{ 
			GetAPIKey() interface{ 
				GetRateLimit() int 
				GetBurstLimit() int 
			} 
		}); ok {
			apiKey := auth.GetAPIKey()
			return apiKey.GetRateLimit(), apiKey.GetBurstLimit()
		}
	}

	return 0, 0 // Use default limits
}

// setRateLimitHeaders sets standard rate limit headers
func setRateLimitHeaders(c *gin.Context, result *RateLimitResult) {
	// Set standard rate limit headers
	c.Header("X-RateLimit-Remaining", strconv.Itoa(result.RemainingTokens))
	c.Header("X-RateLimit-Reset", strconv.FormatInt(result.ResetTime.Unix(), 10))
	
	if !result.Allowed {
		c.Header("Retry-After", strconv.FormatInt(int64(result.RetryAfter.Seconds()), 10))
	}
}

// isExemptEndpoint checks if an endpoint is exempt from rate limiting
func isExemptEndpoint(path string) bool {
	exemptPaths := []string{
		"/health",
		"/ready", 
		"/live",
		"/metrics",
	}

	for _, exemptPath := range exemptPaths {
		if path == exemptPath {
			return true
		}
	}
	
	return false
}

// AdaptiveRateLimitMiddleware creates middleware with adaptive rate limiting
func AdaptiveRateLimitMiddleware(limiter *RateLimiter, logger *logrus.Logger) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		start := time.Now()
		
		// Apply normal rate limiting first
		RateLimitMiddleware(limiter, logger)(c)
		
		if c.IsAborted() {
			return
		}

		// Continue with request processing
		c.Next()

		// Measure response time for adaptive adjustment
		duration := time.Since(start)
		
		// Simple adaptive logic: if response time is high, temporarily reduce limits
		if duration > 5*time.Second {
			clientID := getClientID(c)
			logger.WithFields(logrus.Fields{
				"client_id":     clientID,
				"response_time": duration.Milliseconds(),
				"path":          c.Request.URL.Path,
			}).Info("High response time detected - consider adaptive rate limiting")
			
			// In a more sophisticated implementation, you would:
			// 1. Temporarily reduce rate limits for this client
			// 2. Implement exponential backoff
			// 3. Consider system load metrics
		}
	})
}

// BurstProtectionMiddleware provides additional protection against traffic bursts
func BurstProtectionMiddleware(limiter *RateLimiter, logger *logrus.Logger, maxBurstClients int) gin.HandlerFunc {
	burstTracker := &BurstTracker{
		clients:           make(map[string]*BurstInfo),
		maxBurstClients:   maxBurstClients,
		burstWindow:      time.Minute,
		burstThreshold:   50, // requests per minute to be considered a burst
	}

	return gin.HandlerFunc(func(c *gin.Context) {
		clientID := getClientID(c)
		
		if burstTracker.IsBursting(clientID) {
			logger.WithFields(logrus.Fields{
				"client_id": clientID,
				"endpoint":  c.Request.URL.Path,
			}).Warn("Burst protection activated")

			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":     "Burst protection activated",
				"message":   "Too many requests in a short time period",
				"retry_after": 60,
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			})
			c.Abort()
			return
		}

		burstTracker.RecordRequest(clientID)
		c.Next()
	})
}

// BurstTracker tracks request bursts for clients
type BurstTracker struct {
	clients         map[string]*BurstInfo
	maxBurstClients int
	burstWindow     time.Duration
	burstThreshold  int
}

// BurstInfo tracks burst information for a client
type BurstInfo struct {
	requestTimes []time.Time
	lastRequest  time.Time
	burstCount   int
}

// IsBursting checks if a client is currently bursting
func (bt *BurstTracker) IsBursting(clientID string) bool {
	info, exists := bt.clients[clientID]
	if !exists {
		return false
	}

	now := time.Now()
	windowStart := now.Add(-bt.burstWindow)

	// Count requests in the current window
	validRequests := []time.Time{}
	for _, reqTime := range info.requestTimes {
		if reqTime.After(windowStart) {
			validRequests = append(validRequests, reqTime)
		}
	}

	info.requestTimes = validRequests
	return len(validRequests) >= bt.burstThreshold
}

// RecordRequest records a request for burst tracking
func (bt *BurstTracker) RecordRequest(clientID string) {
	now := time.Now()
	
	info, exists := bt.clients[clientID]
	if !exists {
		info = &BurstInfo{
			requestTimes: []time.Time{},
		}
		bt.clients[clientID] = info
	}

	info.requestTimes = append(info.requestTimes, now)
	info.lastRequest = now

	// Clean up old request times
	windowStart := now.Add(-bt.burstWindow)
	validRequests := []time.Time{}
	for _, reqTime := range info.requestTimes {
		if reqTime.After(windowStart) {
			validRequests = append(validRequests, reqTime)
		}
	}
	info.requestTimes = validRequests
}