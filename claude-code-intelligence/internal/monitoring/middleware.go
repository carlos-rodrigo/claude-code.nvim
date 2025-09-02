package monitoring

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// HTTPMetricsMiddleware creates middleware for collecting HTTP metrics
func HTTPMetricsMiddleware(metricsCollector *MetricsCollector, logger *logrus.Logger) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		start := time.Now()
		
		// Increment request count
		metricsCollector.IncrementRequests()
		
		// Process request
		c.Next()
		
		// Record metrics after request
		duration := time.Since(start)
		statusCode := c.Writer.Status()
		
		// Record response time
		metricsCollector.RecordResponseTime(duration)
		
		// Increment error count if status indicates error
		if statusCode >= 400 {
			metricsCollector.IncrementErrors()
		}
		
		// Log request with metrics
		logger.WithFields(logrus.Fields{
			"method":      c.Request.Method,
			"path":        c.Request.URL.Path,
			"status":      statusCode,
			"duration_ms": float64(duration.Nanoseconds()) / 1e6,
			"client_ip":   c.ClientIP(),
			"user_agent":  c.Request.UserAgent(),
		}).Info("HTTP request processed")
	})
}

// DatabaseMetricsWrapper wraps database operations to collect metrics
type DatabaseMetricsWrapper struct {
	metricsCollector *MetricsCollector
	logger           *logrus.Logger
}

// NewDatabaseMetricsWrapper creates a new database metrics wrapper
func NewDatabaseMetricsWrapper(metricsCollector *MetricsCollector, logger *logrus.Logger) *DatabaseMetricsWrapper {
	return &DatabaseMetricsWrapper{
		metricsCollector: metricsCollector,
		logger:          logger,
	}
}

// WrapQuery wraps a database query operation
func (dmw *DatabaseMetricsWrapper) WrapQuery(operation string, fn func() error) error {
	start := time.Now()
	err := fn()
	duration := time.Since(start)
	
	// Record metrics
	dmw.metricsCollector.IncrementDBQueries(duration)
	
	if err != nil {
		dmw.metricsCollector.SetDBHealth(false)
		dmw.logger.WithFields(logrus.Fields{
			"operation":   operation,
			"duration_ms": float64(duration.Nanoseconds()) / 1e6,
			"error":       err.Error(),
		}).Error("Database operation failed")
	} else {
		dmw.metricsCollector.SetDBHealth(true)
		dmw.logger.WithFields(logrus.Fields{
			"operation":   operation,
			"duration_ms": float64(duration.Nanoseconds()) / 1e6,
		}).Debug("Database operation completed")
	}
	
	return err
}

// OllamaMetricsWrapper wraps Ollama API calls to collect metrics
type OllamaMetricsWrapper struct {
	metricsCollector *MetricsCollector
	logger           *logrus.Logger
}

// NewOllamaMetricsWrapper creates a new Ollama metrics wrapper
func NewOllamaMetricsWrapper(metricsCollector *MetricsCollector, logger *logrus.Logger) *OllamaMetricsWrapper {
	return &OllamaMetricsWrapper{
		metricsCollector: metricsCollector,
		logger:          logger,
	}
}

// WrapOllamaCall wraps an Ollama API call
func (omw *OllamaMetricsWrapper) WrapOllamaCall(operation string, fn func() error) error {
	start := time.Now()
	err := fn()
	duration := time.Since(start)
	
	// Record metrics
	omw.metricsCollector.IncrementOllamaRequests(duration)
	
	if err != nil {
		omw.metricsCollector.IncrementOllamaErrors()
		omw.metricsCollector.SetOllamaHealth(false)
		omw.logger.WithFields(logrus.Fields{
			"operation":   operation,
			"duration_ms": float64(duration.Nanoseconds()) / 1e6,
			"error":       err.Error(),
		}).Error("Ollama operation failed")
	} else {
		omw.metricsCollector.SetOllamaHealth(true)
		omw.logger.WithFields(logrus.Fields{
			"operation":   operation,
			"duration_ms": float64(duration.Nanoseconds()) / 1e6,
		}).Debug("Ollama operation completed")
	}
	
	return err
}

// ErrorHandlerMiddleware provides enhanced error handling with metrics
func ErrorHandlerMiddleware(metricsCollector *MetricsCollector, logger *logrus.Logger) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				metricsCollector.IncrementErrors()
				logger.WithFields(logrus.Fields{
					"error":  err,
					"path":   c.Request.URL.Path,
					"method": c.Request.Method,
				}).Error("Panic recovered in HTTP handler")
				
				c.JSON(http.StatusInternalServerError, gin.H{
					"error":     "Internal server error",
					"timestamp": time.Now().UTC().Format(time.RFC3339),
					"path":      c.Request.URL.Path,
				})
				c.Abort()
			}
		}()
		c.Next()
	})
}

// CORSMiddleware provides CORS support with monitoring
func CORSMiddleware(logger *logrus.Logger) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Header("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")
		
		if c.Request.Method == "OPTIONS" {
			logger.WithFields(logrus.Fields{
				"origin": origin,
				"path":   c.Request.URL.Path,
			}).Debug("CORS preflight request")
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		
		c.Next()
	})
}

// SecurityHeadersMiddleware adds security headers
func SecurityHeadersMiddleware() gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Content-Security-Policy", "default-src 'self'")
		c.Next()
	})
}

// RequestIDMiddleware adds a unique request ID to each request
func RequestIDMiddleware(logger *logrus.Logger) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}
		
		c.Header("X-Request-ID", requestID)
		c.Set("request_id", requestID)
		
		// Add request ID to logger context
logger := logger.WithField("request_id", requestID)
		c.Set("logger", logger)
		
		c.Next()
	})
}

// generateRequestID generates a simple request ID
func generateRequestID() string {
	return strconv.FormatInt(time.Now().UnixNano(), 36)
}

// RateLimitMiddleware provides simple rate limiting
type RateLimiter struct {
	requests map[string][]time.Time
	maxRPS   int
	window   time.Duration
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(maxRPS int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		requests: make(map[string][]time.Time),
		maxRPS:   maxRPS,
		window:   window,
	}
}

// RateLimitMiddleware creates rate limiting middleware
func (rl *RateLimiter) RateLimitMiddleware(logger *logrus.Logger) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		if rl.maxRPS <= 0 {
			c.Next()
			return
		}
		
		clientIP := c.ClientIP()
		now := time.Now()
		windowStart := now.Add(-rl.window)
		
		// Clean old requests
		if requests, exists := rl.requests[clientIP]; exists {
			var validRequests []time.Time
			for _, reqTime := range requests {
				if reqTime.After(windowStart) {
					validRequests = append(validRequests, reqTime)
				}
			}
			rl.requests[clientIP] = validRequests
		}
		
		// Check rate limit
		if len(rl.requests[clientIP]) >= rl.maxRPS {
			logger.WithFields(logrus.Fields{
				"client_ip":    clientIP,
				"current_rps":  len(rl.requests[clientIP]),
				"max_rps":      rl.maxRPS,
				"path":         c.Request.URL.Path,
			}).Warn("Rate limit exceeded")
			
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":     "Rate limit exceeded",
				"max_rps":   rl.maxRPS,
				"window":    rl.window.String(),
				"timestamp": now.UTC().Format(time.RFC3339),
			})
			c.Abort()
			return
		}
		
		// Add current request
		rl.requests[clientIP] = append(rl.requests[clientIP], now)
		c.Next()
	})
}