package performance

import (
	"context"
	"runtime"
	"runtime/debug"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// PerformanceOptimizer manages system performance optimizations
type PerformanceOptimizer struct {
	mu              sync.RWMutex
	logger          *logrus.Logger
	config          *OptimizerConfig
	metrics         *PerformanceMetrics
	connectionPool  *ConnectionPool
	rateLimiter     *AdaptiveRateLimiter
	gcOptimizer     *GCOptimizer
	memoryManager   *MemoryManager
	enabled         bool
}

// OptimizerConfig holds performance optimization configuration
type OptimizerConfig struct {
	// Memory management
	MemoryLimitMB       int           `json:"memory_limit_mb"`
	GCTargetPercent     int           `json:"gc_target_percent"`
	GCInterval          time.Duration `json:"gc_interval"`
	
	// Connection pooling
	MaxConnections      int           `json:"max_connections"`
	ConnectionTimeout   time.Duration `json:"connection_timeout"`
	IdleTimeout         time.Duration `json:"idle_timeout"`
	
	// Rate limiting
	BaseRateLimit       int           `json:"base_rate_limit"`
	BurstLimit          int           `json:"burst_limit"`
	AdaptiveEnabled     bool          `json:"adaptive_enabled"`
	
	// General
	OptimizationInterval time.Duration `json:"optimization_interval"`
	MetricsInterval      time.Duration `json:"metrics_interval"`
}

// PerformanceMetrics tracks performance statistics
type PerformanceMetrics struct {
	mu sync.RWMutex
	
	// Memory metrics
	MemoryUsageMB      float64 `json:"memory_usage_mb"`
	MemoryLimitMB      float64 `json:"memory_limit_mb"`
	MemoryPressure     float64 `json:"memory_pressure"`
	GCCount            int64   `json:"gc_count"`
	GCPauseTime        time.Duration `json:"gc_pause_time"`
	
	// Performance metrics
	ResponseTimeP50    time.Duration `json:"response_time_p50"`
	ResponseTimeP95    time.Duration `json:"response_time_p95"`
	ResponseTimeP99    time.Duration `json:"response_time_p99"`
	ThroughputRPS      float64       `json:"throughput_rps"`
	
	// Resource metrics
	GoroutineCount     int           `json:"goroutine_count"`
	ActiveConnections  int           `json:"active_connections"`
	QueuedRequests     int           `json:"queued_requests"`
	
	// Optimization metrics
	OptimizationsRun   int64         `json:"optimizations_run"`
	LastOptimization   time.Time     `json:"last_optimization"`
}

// NewPerformanceOptimizer creates a new performance optimizer
func NewPerformanceOptimizer(config *OptimizerConfig, logger *logrus.Logger) *PerformanceOptimizer {
	if config == nil {
		config = &OptimizerConfig{
			MemoryLimitMB:        500,
			GCTargetPercent:      100,
			GCInterval:           30 * time.Second,
			MaxConnections:       100,
			ConnectionTimeout:    30 * time.Second,
			IdleTimeout:          5 * time.Minute,
			BaseRateLimit:        100,
			BurstLimit:          200,
			AdaptiveEnabled:     true,
			OptimizationInterval: 1 * time.Minute,
			MetricsInterval:     10 * time.Second,
		}
	}

	po := &PerformanceOptimizer{
		logger:  logger,
		config:  config,
		metrics: &PerformanceMetrics{},
		enabled: true,
	}

	// Initialize sub-components
	po.connectionPool = NewConnectionPool(config.MaxConnections, config.ConnectionTimeout, config.IdleTimeout, logger)
	po.rateLimiter = NewAdaptiveRateLimiter(config.BaseRateLimit, config.BurstLimit, config.AdaptiveEnabled, logger)
	po.gcOptimizer = NewGCOptimizer(config.GCTargetPercent, config.GCInterval, logger)
	po.memoryManager = NewMemoryManager(config.MemoryLimitMB, logger)

	return po
}

// Start begins performance optimization
func (po *PerformanceOptimizer) Start(ctx context.Context) {
	if !po.enabled {
		return
	}

	po.logger.Info("Starting performance optimizer")

	// Start sub-components
	go po.connectionPool.Start(ctx)
	go po.rateLimiter.Start(ctx)
	go po.gcOptimizer.Start(ctx)
	go po.memoryManager.Start(ctx)

	// Start optimization loop
	go po.optimizationLoop(ctx)
	
	// Start metrics collection
	go po.metricsLoop(ctx)
}

// Stop stops the performance optimizer
func (po *PerformanceOptimizer) Stop() {
	po.mu.Lock()
	defer po.mu.Unlock()
	
	po.enabled = false
	po.logger.Info("Performance optimizer stopped")
}

// GetMetrics returns current performance metrics
func (po *PerformanceOptimizer) GetMetrics() *PerformanceMetrics {
	po.metrics.mu.RLock()
	defer po.metrics.mu.RUnlock()
	
	// Create a copy
	metrics := *po.metrics
	return &metrics
}

// optimizationLoop runs periodic optimizations
func (po *PerformanceOptimizer) optimizationLoop(ctx context.Context) {
	ticker := time.NewTicker(po.config.OptimizationInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if po.enabled {
				po.runOptimizations()
			}
		}
	}
}

// metricsLoop collects performance metrics
func (po *PerformanceOptimizer) metricsLoop(ctx context.Context) {
	ticker := time.NewTicker(po.config.MetricsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if po.enabled {
				po.collectMetrics()
			}
		}
	}
}

// runOptimizations performs various performance optimizations
func (po *PerformanceOptimizer) runOptimizations() {
	start := time.Now()
	
	po.logger.Debug("Running performance optimizations")

	// Memory optimization
	po.optimizeMemory()
	
	// Connection pool optimization
	po.optimizeConnections()
	
	// Rate limiting optimization
	po.optimizeRateLimit()

	// Update metrics
	po.metrics.mu.Lock()
	po.metrics.OptimizationsRun++
	po.metrics.LastOptimization = time.Now()
	po.metrics.mu.Unlock()

	duration := time.Since(start)
	po.logger.WithField("duration_ms", duration.Milliseconds()).Debug("Performance optimizations completed")
}

// collectMetrics collects current system metrics
func (po *PerformanceOptimizer) collectMetrics() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	po.metrics.mu.Lock()
	defer po.metrics.mu.Unlock()

	// Memory metrics
	po.metrics.MemoryUsageMB = float64(m.Alloc) / 1024 / 1024
	po.metrics.MemoryLimitMB = float64(po.config.MemoryLimitMB)
	po.metrics.MemoryPressure = po.metrics.MemoryUsageMB / po.metrics.MemoryLimitMB
	po.metrics.GCCount = int64(m.NumGC)
	
	// Runtime metrics
	po.metrics.GoroutineCount = runtime.NumGoroutine()
	
	// Component metrics
	po.metrics.ActiveConnections = po.connectionPool.GetActiveCount()
	po.metrics.QueuedRequests = po.rateLimiter.GetQueuedCount()
}

// optimizeMemory performs memory optimizations
func (po *PerformanceOptimizer) optimizeMemory() {
	metrics := po.GetMetrics()
	
	// Trigger GC if memory pressure is high
	if metrics.MemoryPressure > 0.8 {
		po.logger.WithField("memory_pressure", metrics.MemoryPressure).Info("High memory pressure detected, triggering GC")
		runtime.GC()
		
		// Set lower GC target to be more aggressive
		debug.SetGCPercent(50)
	} else if metrics.MemoryPressure < 0.4 {
		// Reset to normal GC behavior
		debug.SetGCPercent(po.config.GCTargetPercent)
	}
}

// optimizeConnections optimizes connection pool
func (po *PerformanceOptimizer) optimizeConnections() {
	po.connectionPool.Optimize()
}

// optimizeRateLimit adjusts rate limiting based on current load
func (po *PerformanceOptimizer) optimizeRateLimit() {
	if po.config.AdaptiveEnabled {
		po.rateLimiter.AdaptToLoad()
	}
}

// ConnectionPool manages database connections
type ConnectionPool struct {
	mu            sync.RWMutex
	maxConns      int
	activeConns   int
	timeout       time.Duration
	idleTimeout   time.Duration
	logger        *logrus.Logger
}

// NewConnectionPool creates a new connection pool
func NewConnectionPool(maxConns int, timeout, idleTimeout time.Duration, logger *logrus.Logger) *ConnectionPool {
	return &ConnectionPool{
		maxConns:    maxConns,
		timeout:     timeout,
		idleTimeout: idleTimeout,
		logger:      logger,
	}
}

// Start starts the connection pool
func (cp *ConnectionPool) Start(ctx context.Context) {
	cp.logger.Info("Connection pool started")
}

// GetActiveCount returns the number of active connections
func (cp *ConnectionPool) GetActiveCount() int {
	cp.mu.RLock()
	defer cp.mu.RUnlock()
	return cp.activeConns
}

// Optimize optimizes the connection pool
func (cp *ConnectionPool) Optimize() {
	// Connection pool optimization logic
	cp.logger.Debug("Optimizing connection pool")
}

// AdaptiveRateLimiter provides adaptive rate limiting
type AdaptiveRateLimiter struct {
	mu           sync.RWMutex
	baseLimit    int
	currentLimit int
	burstLimit   int
	queuedCount  int
	adaptive     bool
	logger       *logrus.Logger
}

// NewAdaptiveRateLimiter creates a new adaptive rate limiter
func NewAdaptiveRateLimiter(baseLimit, burstLimit int, adaptive bool, logger *logrus.Logger) *AdaptiveRateLimiter {
	return &AdaptiveRateLimiter{
		baseLimit:    baseLimit,
		currentLimit: baseLimit,
		burstLimit:   burstLimit,
		adaptive:     adaptive,
		logger:       logger,
	}
}

// Start starts the rate limiter
func (arl *AdaptiveRateLimiter) Start(ctx context.Context) {
	arl.logger.Info("Adaptive rate limiter started")
}

// GetQueuedCount returns the number of queued requests
func (arl *AdaptiveRateLimiter) GetQueuedCount() int {
	arl.mu.RLock()
	defer arl.mu.RUnlock()
	return arl.queuedCount
}

// AdaptToLoad adjusts rate limit based on current load
func (arl *AdaptiveRateLimiter) AdaptToLoad() {
	arl.mu.Lock()
	defer arl.mu.Unlock()
	
	// Adaptive rate limiting logic
	if arl.queuedCount > arl.currentLimit {
		// Increase limit if we have queued requests
		newLimit := int(float64(arl.currentLimit) * 1.1)
		if newLimit <= arl.burstLimit {
			arl.currentLimit = newLimit
			arl.logger.WithFields(logrus.Fields{
				"old_limit": arl.currentLimit,
				"new_limit": newLimit,
			}).Debug("Increased rate limit")
		}
	} else if arl.queuedCount < arl.currentLimit/2 {
		// Decrease limit if we have low load
		newLimit := int(float64(arl.currentLimit) * 0.9)
		if newLimit >= arl.baseLimit {
			arl.currentLimit = newLimit
			arl.logger.WithFields(logrus.Fields{
				"old_limit": arl.currentLimit,
				"new_limit": newLimit,
			}).Debug("Decreased rate limit")
		}
	}
}

// GCOptimizer optimizes garbage collection
type GCOptimizer struct {
	targetPercent int
	interval      time.Duration
	logger        *logrus.Logger
}

// NewGCOptimizer creates a new GC optimizer
func NewGCOptimizer(targetPercent int, interval time.Duration, logger *logrus.Logger) *GCOptimizer {
	return &GCOptimizer{
		targetPercent: targetPercent,
		interval:      interval,
		logger:        logger,
	}
}

// Start starts the GC optimizer
func (gco *GCOptimizer) Start(ctx context.Context) {
	// Set initial GC target
	debug.SetGCPercent(gco.targetPercent)
	
	gco.logger.WithField("gc_target_percent", gco.targetPercent).Info("GC optimizer started")
	
	// Periodic GC optimization
	ticker := time.NewTicker(gco.interval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			gco.optimize()
		}
	}
}

// optimize performs GC optimization
func (gco *GCOptimizer) optimize() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	// Log GC stats periodically
	gco.logger.WithFields(logrus.Fields{
		"gc_count":        m.NumGC,
		"pause_total_ms":  float64(m.PauseTotalNs) / 1e6,
		"heap_alloc_mb":   float64(m.Alloc) / 1024 / 1024,
		"heap_sys_mb":     float64(m.HeapSys) / 1024 / 1024,
	}).Debug("GC statistics")
}

// MemoryManager manages memory usage
type MemoryManager struct {
	limitMB int
	logger  *logrus.Logger
}

// NewMemoryManager creates a new memory manager
func NewMemoryManager(limitMB int, logger *logrus.Logger) *MemoryManager {
	return &MemoryManager{
		limitMB: limitMB,
		logger:  logger,
	}
}

// Start starts the memory manager
func (mm *MemoryManager) Start(ctx context.Context) {
	mm.logger.WithField("memory_limit_mb", mm.limitMB).Info("Memory manager started")
	
	// Periodic memory checks
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			mm.checkMemoryUsage()
		}
	}
}

// checkMemoryUsage monitors memory usage
func (mm *MemoryManager) checkMemoryUsage() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	usageMB := float64(m.Alloc) / 1024 / 1024
	usagePercent := usageMB / float64(mm.limitMB) * 100
	
	if usagePercent > 90 {
		mm.logger.WithFields(logrus.Fields{
			"usage_mb":      usageMB,
			"limit_mb":      mm.limitMB,
			"usage_percent": usagePercent,
		}).Warn("High memory usage detected")
		
		// Trigger aggressive GC
		runtime.GC()
		debug.FreeOSMemory()
	} else if usagePercent > 80 {
		mm.logger.WithFields(logrus.Fields{
			"usage_mb":      usageMB,
			"usage_percent": usagePercent,
		}).Info("Memory usage warning")
	}
}