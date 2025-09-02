package monitoring

import (
	"context"
	"runtime"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// MetricsCollector collects and manages system metrics
type MetricsCollector struct {
	mu       sync.RWMutex
	logger   *logrus.Logger
	metrics  *SystemMetrics
	enabled  bool
	interval time.Duration
	stopCh   chan struct{}
}

// SystemMetrics holds comprehensive system metrics
type SystemMetrics struct {
	// Service metrics
	StartTime      time.Time `json:"start_time"`
	Uptime         float64   `json:"uptime_seconds"`
	RequestCount   int64     `json:"total_requests"`
	ErrorCount     int64     `json:"total_errors"`
	ResponseTime   float64   `json:"avg_response_time_ms"`
	
	// Database metrics
	DBConnections     int   `json:"db_connections"`
	DBQueryCount      int64 `json:"db_query_count"`
	DBAvgQueryTime    float64 `json:"db_avg_query_time_ms"`
	DBHealthy         bool  `json:"db_healthy"`
	
	// AI/Ollama metrics
	OllamaRequests    int64   `json:"ollama_requests"`
	OllamaErrors      int64   `json:"ollama_errors"`
	OllamaAvgTime     float64 `json:"ollama_avg_time_ms"`
	OllamaHealthy     bool    `json:"ollama_healthy"`
	
	// Cache metrics  
	CacheHits         int64   `json:"cache_hits"`
	CacheMisses       int64   `json:"cache_misses"`
	CacheHitRate      float64 `json:"cache_hit_rate"`
	CacheSize         int64   `json:"cache_size_bytes"`
	
	// System metrics
	MemoryUsage       uint64  `json:"memory_usage_bytes"`
	MemoryPercent     float64 `json:"memory_usage_percent"`
	CPUPercent        float64 `json:"cpu_usage_percent"`
	DiskUsage         uint64  `json:"disk_usage_bytes"`
	GoroutineCount    int     `json:"goroutine_count"`
	
	// Session metrics
	SessionsTotal     int64   `json:"sessions_total"`
	SessionsCompressed int64  `json:"sessions_compressed"`
	AvgCompressionRatio float64 `json:"avg_compression_ratio"`
	CompressionErrors int64   `json:"compression_errors"`
	
	// Performance metrics
	P50ResponseTime   float64 `json:"p50_response_time_ms"`
	P95ResponseTime   float64 `json:"p95_response_time_ms"`
	P99ResponseTime   float64 `json:"p99_response_time_ms"`
	
	// Last updated
	LastUpdated       time.Time `json:"last_updated"`
}

// ResponseTimeTracker tracks response times for percentile calculation
type ResponseTimeTracker struct {
	mu    sync.RWMutex
	times []float64
	maxSamples int
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector(logger *logrus.Logger, interval time.Duration) *MetricsCollector {
	return &MetricsCollector{
		logger:   logger,
		enabled:  true,
		interval: interval,
		stopCh:   make(chan struct{}),
		metrics: &SystemMetrics{
			StartTime:   time.Now(),
			DBHealthy:   true,
			OllamaHealthy: true,
			LastUpdated: time.Now(),
		},
	}
}

// Start begins metrics collection
func (mc *MetricsCollector) Start(ctx context.Context) {
	if !mc.enabled {
		return
	}
	
	mc.logger.WithField("interval", mc.interval).Info("Starting metrics collection")
	
	ticker := time.NewTicker(mc.interval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			mc.logger.Info("Stopping metrics collection due to context cancellation")
			return
		case <-mc.stopCh:
			mc.logger.Info("Stopping metrics collection")
			return
		case <-ticker.C:
			mc.collectSystemMetrics()
		}
	}
}

// Stop stops metrics collection
func (mc *MetricsCollector) Stop() {
	close(mc.stopCh)
}

// GetMetrics returns current system metrics
func (mc *MetricsCollector) GetMetrics() *SystemMetrics {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	
	// Create a copy to avoid race conditions
	metrics := *mc.metrics
	metrics.Uptime = time.Since(mc.metrics.StartTime).Seconds()
	metrics.LastUpdated = time.Now()
	
	return &metrics
}

// IncrementRequests increments the request counter
func (mc *MetricsCollector) IncrementRequests() {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.metrics.RequestCount++
}

// IncrementErrors increments the error counter
func (mc *MetricsCollector) IncrementErrors() {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.metrics.ErrorCount++
}

// RecordResponseTime records a response time
func (mc *MetricsCollector) RecordResponseTime(duration time.Duration) {
	ms := float64(duration.Nanoseconds()) / 1e6
	
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	// Update running average
	if mc.metrics.RequestCount > 0 {
		mc.metrics.ResponseTime = (mc.metrics.ResponseTime*float64(mc.metrics.RequestCount-1) + ms) / float64(mc.metrics.RequestCount)
	} else {
		mc.metrics.ResponseTime = ms
	}
}

// IncrementDBQueries increments database query counter
func (mc *MetricsCollector) IncrementDBQueries(duration time.Duration) {
	ms := float64(duration.Nanoseconds()) / 1e6
	
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	mc.metrics.DBQueryCount++
	if mc.metrics.DBQueryCount > 0 {
		mc.metrics.DBAvgQueryTime = (mc.metrics.DBAvgQueryTime*float64(mc.metrics.DBQueryCount-1) + ms) / float64(mc.metrics.DBQueryCount)
	} else {
		mc.metrics.DBAvgQueryTime = ms
	}
}

// SetDBHealth sets database health status
func (mc *MetricsCollector) SetDBHealth(healthy bool) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.metrics.DBHealthy = healthy
}

// IncrementOllamaRequests increments Ollama request counter
func (mc *MetricsCollector) IncrementOllamaRequests(duration time.Duration) {
	ms := float64(duration.Nanoseconds()) / 1e6
	
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	mc.metrics.OllamaRequests++
	if mc.metrics.OllamaRequests > 0 {
		mc.metrics.OllamaAvgTime = (mc.metrics.OllamaAvgTime*float64(mc.metrics.OllamaRequests-1) + ms) / float64(mc.metrics.OllamaRequests)
	} else {
		mc.metrics.OllamaAvgTime = ms
	}
}

// IncrementOllamaErrors increments Ollama error counter
func (mc *MetricsCollector) IncrementOllamaErrors() {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.metrics.OllamaErrors++
}

// SetOllamaHealth sets Ollama health status
func (mc *MetricsCollector) SetOllamaHealth(healthy bool) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.metrics.OllamaHealthy = healthy
}

// UpdateCacheMetrics updates cache-related metrics
func (mc *MetricsCollector) UpdateCacheMetrics(hits, misses, size int64) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	mc.metrics.CacheHits = hits
	mc.metrics.CacheMisses = misses
	mc.metrics.CacheSize = size
	
	total := hits + misses
	if total > 0 {
		mc.metrics.CacheHitRate = float64(hits) / float64(total) * 100
	}
}

// UpdateSessionMetrics updates session-related metrics
func (mc *MetricsCollector) UpdateSessionMetrics(total, compressed int64, avgCompression float64, errors int64) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	mc.metrics.SessionsTotal = total
	mc.metrics.SessionsCompressed = compressed
	mc.metrics.AvgCompressionRatio = avgCompression
	mc.metrics.CompressionErrors = errors
}

// collectSystemMetrics collects runtime system metrics
func (mc *MetricsCollector) collectSystemMetrics() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	// Memory metrics
	mc.metrics.MemoryUsage = m.Alloc
	mc.metrics.GoroutineCount = runtime.NumGoroutine()
	
	// Calculate memory percentage (approximation)
	totalMem := m.Sys
	if totalMem > 0 {
		mc.metrics.MemoryPercent = float64(m.Alloc) / float64(totalMem) * 100
	}
	
	mc.metrics.LastUpdated = time.Now()
	
	// Log metrics periodically (every 10th collection)
	if mc.metrics.RequestCount%10 == 0 {
		mc.logger.WithFields(logrus.Fields{
			"memory_mb":      float64(mc.metrics.MemoryUsage) / 1024 / 1024,
			"goroutines":     mc.metrics.GoroutineCount,
			"requests":       mc.metrics.RequestCount,
			"errors":         mc.metrics.ErrorCount,
			"avg_response":   mc.metrics.ResponseTime,
			"db_healthy":     mc.metrics.DBHealthy,
			"ollama_healthy": mc.metrics.OllamaHealthy,
		}).Debug("System metrics collected")
	}
}

// GetHealthStatus returns overall system health
func (mc *MetricsCollector) GetHealthStatus() map[string]interface{} {
	metrics := mc.GetMetrics()
	
	// Determine overall health
	healthy := metrics.DBHealthy && metrics.OllamaHealthy
	status := "healthy"
	if !healthy {
		status = "unhealthy"
	}
	
	// Calculate error rate
	errorRate := float64(0)
	if metrics.RequestCount > 0 {
		errorRate = float64(metrics.ErrorCount) / float64(metrics.RequestCount) * 100
	}
	
	// Memory health check
	memoryHealthy := metrics.MemoryPercent < 80 // Alert if memory usage > 80%
	if !memoryHealthy && status == "healthy" {
		status = "warning"
	}
	
	return map[string]interface{}{
		"status":          status,
		"uptime_seconds":  metrics.Uptime,
		"healthy_components": map[string]bool{
			"database": metrics.DBHealthy,
			"ollama":   metrics.OllamaHealthy,
			"memory":   memoryHealthy,
		},
		"error_rate_percent": errorRate,
		"memory_usage_mb":    float64(metrics.MemoryUsage) / 1024 / 1024,
		"total_requests":     metrics.RequestCount,
		"last_updated":       metrics.LastUpdated,
	}
}