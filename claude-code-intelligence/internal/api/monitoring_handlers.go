package api

import (
	"net/http"
	"strconv"
	"time"

	"claude-code-intelligence/internal/monitoring"

	"github.com/gin-gonic/gin"
)

// MonitoringHandlers contains handlers for monitoring endpoints
type MonitoringHandlers struct {
	metricsCollector *monitoring.MetricsCollector
	healthChecker    *monitoring.HealthChecker
	logger           loggerInterface
}

// NewMonitoringHandlers creates monitoring handlers
func NewMonitoringHandlers(
	metricsCollector *monitoring.MetricsCollector,
	healthChecker *monitoring.HealthChecker,
	logger loggerInterface,
) *MonitoringHandlers {
	return &MonitoringHandlers{
		metricsCollector: metricsCollector,
		healthChecker:    healthChecker,
		logger:          logger,
	}
}

// GetMetrics returns system metrics
func (mh *MonitoringHandlers) GetMetrics(c *gin.Context) {
	metrics := mh.metricsCollector.GetMetrics()
	c.JSON(http.StatusOK, gin.H{
		"metrics": metrics,
		"collected_at": time.Now().UTC().Format(time.RFC3339),
	})
}

// GetHealth returns overall system health
func (mh *MonitoringHandlers) GetHealth(c *gin.Context) {
	health := mh.healthChecker.GetHealth()
	
	// Determine HTTP status based on health
	statusCode := http.StatusOK
	if health.Status == "unhealthy" {
		statusCode = http.StatusServiceUnavailable
	} else if health.Status == "warning" {
		statusCode = http.StatusPartialContent // 206
	}
	
	c.JSON(statusCode, health)
}

// GetDetailedHealth returns detailed health information
func (mh *MonitoringHandlers) GetDetailedHealth(c *gin.Context) {
	health := mh.healthChecker.GetHealth()
	metrics := mh.metricsCollector.GetMetrics()
	
	detailed := gin.H{
		"health":  health,
		"metrics": metrics,
		"system": gin.H{
			"timestamp":    time.Now().UTC().Format(time.RFC3339),
			"version":      "1.0.0", // You might want to get this from build info
			"environment":  "development", // From config
		},
	}
	
	// Determine HTTP status
	statusCode := http.StatusOK
	if health.Status == "unhealthy" {
		statusCode = http.StatusServiceUnavailable
	}
	
	c.JSON(statusCode, detailed)
}

// GetComponentHealth returns health for a specific component
func (mh *MonitoringHandlers) GetComponentHealth(c *gin.Context) {
	component := c.Param("component")
	if component == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Component name is required",
		})
		return
	}
	
	health, exists := mh.healthChecker.GetComponentHealth(component)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Component not found",
			"component": component,
		})
		return
	}
	
	// Determine status code
	statusCode := http.StatusOK
	if health.Status == "unhealthy" {
		statusCode = http.StatusServiceUnavailable
	}
	
	c.JSON(statusCode, gin.H{
		"component": component,
		"health":    health,
	})
}

// GetReadiness returns readiness status (simpler than health)
func (mh *MonitoringHandlers) GetReadiness(c *gin.Context) {
	// Check if critical components are ready
	health := mh.healthChecker.GetHealth()
	
	ready := true
	message := "Service is ready"
	
	// Check critical components
	for name, result := range health.Components {
		if result.Status == "unhealthy" {
			// You might want to check if this component is critical
			ready = false
			message = "Service not ready - critical component unhealthy: " + name
			break
		}
	}
	
	statusCode := http.StatusOK
	if !ready {
		statusCode = http.StatusServiceUnavailable
	}
	
	c.JSON(statusCode, gin.H{
		"ready":     ready,
		"message":   message,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// GetLiveness returns liveness status (basic server responsiveness)
func (mh *MonitoringHandlers) GetLiveness(c *gin.Context) {
	// Simple liveness check - if we can respond, we're alive
	c.JSON(http.StatusOK, gin.H{
		"alive":     true,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"uptime":    time.Since(time.Now()).Seconds(), // This would be actual start time
	})
}

// GetPrometheusMetrics returns metrics in Prometheus format
func (mh *MonitoringHandlers) GetPrometheusMetrics(c *gin.Context) {
	metrics := mh.metricsCollector.GetMetrics()
	
	// Convert to Prometheus format
	prometheusMetrics := convertToPrometheusFormat(metrics)
	
	c.Header("Content-Type", "text/plain; version=0.0.4")
	c.String(http.StatusOK, prometheusMetrics)
}

// convertToPrometheusFormat converts metrics to Prometheus format
func convertToPrometheusFormat(metrics *monitoring.SystemMetrics) string {
	var result string
	
	// Helper function to add metric
	addMetric := func(name, help, metricType string, value interface{}, labels map[string]string) {
		result += "# HELP " + name + " " + help + "\n"
		result += "# TYPE " + name + " " + metricType + "\n"
		
		labelStr := ""
		if len(labels) > 0 {
			labelStr = "{"
			first := true
			for k, v := range labels {
				if !first {
					labelStr += ","
				}
				labelStr += k + `="` + v + `"`
				first = false
			}
			labelStr += "}"
		}
		
		result += name + labelStr + " " + formatValue(value) + "\n\n"
	}
	
	// Add metrics
	addMetric("claude_code_uptime_seconds", "Service uptime in seconds", "gauge", metrics.Uptime, nil)
	addMetric("claude_code_requests_total", "Total number of requests", "counter", metrics.RequestCount, nil)
	addMetric("claude_code_errors_total", "Total number of errors", "counter", metrics.ErrorCount, nil)
	addMetric("claude_code_response_time_ms", "Average response time in milliseconds", "gauge", metrics.ResponseTime, nil)
	
	addMetric("claude_code_db_queries_total", "Total database queries", "counter", metrics.DBQueryCount, nil)
	addMetric("claude_code_db_query_time_ms", "Average database query time in milliseconds", "gauge", metrics.DBAvgQueryTime, nil)
	addMetric("claude_code_db_healthy", "Database health status", "gauge", boolToInt(metrics.DBHealthy), nil)
	
	addMetric("claude_code_ollama_requests_total", "Total Ollama requests", "counter", metrics.OllamaRequests, nil)
	addMetric("claude_code_ollama_errors_total", "Total Ollama errors", "counter", metrics.OllamaErrors, nil)
	addMetric("claude_code_ollama_time_ms", "Average Ollama response time in milliseconds", "gauge", metrics.OllamaAvgTime, nil)
	addMetric("claude_code_ollama_healthy", "Ollama service health status", "gauge", boolToInt(metrics.OllamaHealthy), nil)
	
	addMetric("claude_code_cache_hits_total", "Total cache hits", "counter", metrics.CacheHits, nil)
	addMetric("claude_code_cache_misses_total", "Total cache misses", "counter", metrics.CacheMisses, nil)
	addMetric("claude_code_cache_hit_rate", "Cache hit rate percentage", "gauge", metrics.CacheHitRate, nil)
	addMetric("claude_code_cache_size_bytes", "Cache size in bytes", "gauge", metrics.CacheSize, nil)
	
	addMetric("claude_code_memory_usage_bytes", "Memory usage in bytes", "gauge", metrics.MemoryUsage, nil)
	addMetric("claude_code_memory_usage_percent", "Memory usage percentage", "gauge", metrics.MemoryPercent, nil)
	addMetric("claude_code_goroutines", "Number of goroutines", "gauge", metrics.GoroutineCount, nil)
	
	addMetric("claude_code_sessions_total", "Total number of sessions", "gauge", metrics.SessionsTotal, nil)
	addMetric("claude_code_sessions_compressed", "Number of compressed sessions", "gauge", metrics.SessionsCompressed, nil)
	addMetric("claude_code_compression_ratio", "Average compression ratio", "gauge", metrics.AvgCompressionRatio, nil)
	addMetric("claude_code_compression_errors_total", "Total compression errors", "counter", metrics.CompressionErrors, nil)
	
	return result
}

// Helper functions
func formatValue(value interface{}) string {
	switch v := value.(type) {
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case uint64:
		return strconv.FormatUint(v, 10)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		if v {
			return "1"
		}
		return "0"
	default:
		return "0"
	}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// SetLogLevel allows dynamic log level changes
func (mh *MonitoringHandlers) SetLogLevel(c *gin.Context) {
	var request struct {
		Level string `json:"level" binding:"required"`
	}
	
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"details": err.Error(),
		})
		return
	}
	
	// This would require access to the logger configuration
	// For now, just acknowledge the request
	mh.logger.WithFields(map[string]interface{}{
		"new_level": request.Level,
		"endpoint":  "set_log_level",
	}).Info("Log level change requested")
	
	c.JSON(http.StatusOK, gin.H{
		"message":   "Log level change requested",
		"new_level": request.Level,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// GetConfiguration returns current service configuration (non-sensitive)
func (mh *MonitoringHandlers) GetConfiguration(c *gin.Context) {
	// Return non-sensitive configuration information
	config := gin.H{
		"service": gin.H{
			"name":    "claude-code-intelligence",
			"version": "1.0.0",
		},
		"features": gin.H{
			"monitoring_enabled": true,
			"cache_enabled":      true,
			"ai_features":        true,
		},
		"limits": gin.H{
			"max_request_size":   "10MB",
			"request_timeout":    "30s",
			"max_sessions":       1000,
		},
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	
	c.JSON(http.StatusOK, config)
}