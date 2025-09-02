package degradation

import (
	"context"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// DegradationManager handles graceful service degradation
type DegradationManager struct {
	mu           sync.RWMutex
	logger       *logrus.Logger
	services     map[string]*ServiceStatus
	config       *DegradationConfig
	circuitBreaker *CircuitBreaker
	fallbackCache  *FallbackCache
}

// DegradationConfig contains configuration for graceful degradation
type DegradationConfig struct {
	// Circuit breaker settings
	FailureThreshold    int           `json:"failure_threshold"`
	RecoveryTimeout     time.Duration `json:"recovery_timeout"`
	HalfOpenMaxCalls    int           `json:"half_open_max_calls"`
	
	// Fallback settings
	EnableFallbacks     bool          `json:"enable_fallbacks"`
	CacheExpiry        time.Duration `json:"cache_expiry"`
	MaxCacheSize       int           `json:"max_cache_size"`
	
	// Service monitoring
	HealthCheckInterval time.Duration `json:"health_check_interval"`
	ServiceTimeout     time.Duration `json:"service_timeout"`
}

// ServiceStatus represents the current status of a service
type ServiceStatus struct {
	Name           string                 `json:"name"`
	Status         ServiceHealthStatus    `json:"status"`
	LastCheck      time.Time              `json:"last_check"`
	FailureCount   int                    `json:"failure_count"`
	LastError      string                 `json:"last_error,omitempty"`
	ResponseTime   time.Duration          `json:"response_time"`
	DegradationLevel DegradationLevel     `json:"degradation_level"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// ServiceHealthStatus represents the health status of a service
type ServiceHealthStatus string

const (
	ServiceHealthy     ServiceHealthStatus = "healthy"
	ServiceDegraded    ServiceHealthStatus = "degraded"
	ServiceUnhealthy   ServiceHealthStatus = "unhealthy"
	ServiceUnavailable ServiceHealthStatus = "unavailable"
)

// DegradationLevel represents the level of service degradation
type DegradationLevel int

const (
	DegradationNone     DegradationLevel = iota // Full functionality
	DegradationMinimal                          // Minor features disabled
	DegradationPartial                          // Some features disabled
	DegradationMajor                            // Most features disabled
	DegradationCritical                         // Only core features available
)

// ServiceResponse represents a response from a service call
type ServiceResponse struct {
	Success     bool                   `json:"success"`
	Data        interface{}            `json:"data,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Source      ResponseSource         `json:"source"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Duration    time.Duration          `json:"duration"`
}

// ResponseSource indicates where the response came from
type ResponseSource string

const (
	SourceService  ResponseSource = "service"
	SourceFallback ResponseSource = "fallback"
	SourceCache    ResponseSource = "cache"
)

// NewDegradationManager creates a new degradation manager
func NewDegradationManager(config *DegradationConfig, logger *logrus.Logger) *DegradationManager {
	if config == nil {
		config = &DegradationConfig{
			FailureThreshold:    5,
			RecoveryTimeout:     30 * time.Second,
			HalfOpenMaxCalls:    3,
			EnableFallbacks:     true,
			CacheExpiry:         15 * time.Minute,
			MaxCacheSize:        1000,
			HealthCheckInterval: 30 * time.Second,
			ServiceTimeout:      10 * time.Second,
		}
	}

	dm := &DegradationManager{
		logger:   logger,
		services: make(map[string]*ServiceStatus),
		config:   config,
	}

	// Initialize circuit breaker
	dm.circuitBreaker = NewCircuitBreaker(&CircuitBreakerConfig{
		FailureThreshold: config.FailureThreshold,
		RecoveryTimeout:  config.RecoveryTimeout,
		HalfOpenMaxCalls: config.HalfOpenMaxCalls,
	}, logger)

	// Initialize fallback cache
	dm.fallbackCache = NewFallbackCache(config.MaxCacheSize, config.CacheExpiry, logger)

	logger.WithFields(logrus.Fields{
		"failure_threshold":     config.FailureThreshold,
		"recovery_timeout":      config.RecoveryTimeout,
		"enable_fallbacks":      config.EnableFallbacks,
		"health_check_interval": config.HealthCheckInterval,
	}).Info("Degradation manager initialized")

	return dm
}

// RegisterService registers a service for monitoring
func (dm *DegradationManager) RegisterService(name string) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	dm.services[name] = &ServiceStatus{
		Name:             name,
		Status:           ServiceHealthy,
		LastCheck:        time.Now(),
		FailureCount:     0,
		DegradationLevel: DegradationNone,
		Metadata:         make(map[string]interface{}),
	}

	dm.logger.WithField("service", name).Info("Service registered for degradation monitoring")
}

// CallService makes a call to a service with degradation handling
func (dm *DegradationManager) CallService(ctx context.Context, serviceName string, operation string, fn func(ctx context.Context) (interface{}, error)) *ServiceResponse {
	start := time.Now()

	// Check if service is registered
	dm.mu.RLock()
	service, exists := dm.services[serviceName]
	dm.mu.RUnlock()

	if !exists {
		dm.RegisterService(serviceName)
		service = dm.services[serviceName]
	}

	// Check circuit breaker
	if !dm.circuitBreaker.CanCall(serviceName) {
		dm.logger.WithFields(logrus.Fields{
			"service":   serviceName,
			"operation": operation,
		}).Warn("Circuit breaker open - using fallback")

		return dm.handleFallback(serviceName, operation, "circuit_breaker_open")
	}

	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, dm.config.ServiceTimeout)
	defer cancel()

	// Make the service call
	result, err := fn(timeoutCtx)
	duration := time.Since(start)

	// Update service status
	dm.updateServiceStatus(serviceName, err, duration)

	if err != nil {
		// Record failure in circuit breaker
		dm.circuitBreaker.RecordFailure(serviceName)

		dm.logger.WithFields(logrus.Fields{
			"service":     serviceName,
			"operation":   operation,
			"error":       err.Error(),
			"duration_ms": duration.Milliseconds(),
		}).Error("Service call failed")

		// Try fallback
		return dm.handleFallback(serviceName, operation, err.Error())
	}

	// Record success
	dm.circuitBreaker.RecordSuccess(serviceName)

	// Cache successful response for fallback
	if dm.config.EnableFallbacks {
		dm.fallbackCache.Set(serviceName, operation, result)
	}

	return &ServiceResponse{
		Success:  true,
		Data:     result,
		Source:   SourceService,
		Duration: duration,
		Metadata: map[string]interface{}{
			"service":   serviceName,
			"operation": operation,
		},
	}
}

// updateServiceStatus updates the status of a service
func (dm *DegradationManager) updateServiceStatus(serviceName string, err error, duration time.Duration) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	service, exists := dm.services[serviceName]
	if !exists {
		return
	}

	service.LastCheck = time.Now()
	service.ResponseTime = duration

	if err != nil {
		service.FailureCount++
		service.LastError = err.Error()
		
		// Update status based on failure count
		if service.FailureCount >= dm.config.FailureThreshold {
			service.Status = ServiceUnhealthy
			service.DegradationLevel = DegradationCritical
		} else if service.FailureCount >= dm.config.FailureThreshold/2 {
			service.Status = ServiceDegraded
			service.DegradationLevel = DegradationPartial
		}
	} else {
		// Reset on success
		if service.FailureCount > 0 {
			dm.logger.WithField("service", serviceName).Info("Service recovered")
		}
		service.FailureCount = 0
		service.LastError = ""
		service.Status = ServiceHealthy
		service.DegradationLevel = DegradationNone
	}

	// Log significant status changes
	if err != nil && service.FailureCount == 1 {
		dm.logger.WithFields(logrus.Fields{
			"service": serviceName,
			"error":   err.Error(),
		}).Warn("Service failure detected")
	}
}

// handleFallback attempts to handle a service failure with fallbacks
func (dm *DegradationManager) handleFallback(serviceName, operation, reason string) *ServiceResponse {
	if !dm.config.EnableFallbacks {
		return &ServiceResponse{
			Success: false,
			Error:   reason,
			Source:  SourceService,
		}
	}

	// Try cache fallback
	if cached := dm.fallbackCache.Get(serviceName, operation); cached != nil {
		dm.logger.WithFields(logrus.Fields{
			"service":   serviceName,
			"operation": operation,
			"reason":    reason,
		}).Info("Using cached fallback response")

		return &ServiceResponse{
			Success: true,
			Data:    cached.Data,
			Source:  SourceCache,
			Metadata: map[string]interface{}{
				"service":    serviceName,
				"operation":  operation,
				"cached_at":  cached.CachedAt,
				"reason":     reason,
			},
		}
	}

	// Try service-specific fallback
	fallback := dm.getServiceFallback(serviceName, operation)
	if fallback != nil {
		dm.logger.WithFields(logrus.Fields{
			"service":   serviceName,
			"operation": operation,
			"reason":    reason,
		}).Info("Using service fallback response")

		return &ServiceResponse{
			Success: true,
			Data:    fallback,
			Source:  SourceFallback,
			Metadata: map[string]interface{}{
				"service":   serviceName,
				"operation": operation,
				"reason":    reason,
			},
		}
	}

	// No fallback available
	return &ServiceResponse{
		Success: false,
		Error:   "Service unavailable and no fallback available: " + reason,
		Source:  SourceService,
		Metadata: map[string]interface{}{
			"service":   serviceName,
			"operation": operation,
			"reason":    reason,
		},
	}
}

// getServiceFallback returns a fallback response for a service operation
func (dm *DegradationManager) getServiceFallback(serviceName, operation string) interface{} {
	// Define fallbacks for known services and operations
	fallbacks := map[string]map[string]interface{}{
		"ollama": {
			"compress": map[string]interface{}{
				"compressed_content": "Service temporarily unavailable - content preserved as-is",
				"compression_ratio":  1.0,
				"success":           true,
				"fallback":          true,
			},
			"embed": []float64{}, // Empty embedding
			"chat": map[string]interface{}{
				"response": "I apologize, but I'm temporarily unable to process your request. Please try again later.",
				"fallback": true,
			},
		},
		"database": {
			"search": []interface{}{}, // Empty search results
			"list":   []interface{}{}, // Empty list
		},
	}

	if serviceMap, exists := fallbacks[serviceName]; exists {
		if fallback, exists := serviceMap[operation]; exists {
			return fallback
		}
	}

	return nil
}

// GetServiceStatus returns the current status of a service
func (dm *DegradationManager) GetServiceStatus(serviceName string) (*ServiceStatus, bool) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	status, exists := dm.services[serviceName]
	if !exists {
		return nil, false
	}

	// Return a copy to prevent external modification
	statusCopy := *status
	return &statusCopy, true
}

// GetAllServiceStatuses returns the status of all registered services
func (dm *DegradationManager) GetAllServiceStatuses() map[string]*ServiceStatus {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	statuses := make(map[string]*ServiceStatus)
	for name, status := range dm.services {
		// Return copies
		statusCopy := *status
		statuses[name] = &statusCopy
	}

	return statuses
}

// GetSystemDegradationLevel returns the overall system degradation level
func (dm *DegradationManager) GetSystemDegradationLevel() DegradationLevel {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	maxDegradation := DegradationNone
	for _, service := range dm.services {
		if service.DegradationLevel > maxDegradation {
			maxDegradation = service.DegradationLevel
		}
	}

	return maxDegradation
}

// IsFeatureAvailable checks if a feature is available given current degradation
func (dm *DegradationManager) IsFeatureAvailable(featureName string) bool {
	degradationLevel := dm.GetSystemDegradationLevel()
	
	// Define feature availability based on degradation level
	featureRequirements := map[string]DegradationLevel{
		"compression":     DegradationMajor,    // Available unless critically degraded
		"search":          DegradationPartial,   // Available unless majorly degraded
		"analytics":       DegradationMinimal,   // Disabled with any degradation
		"backup":          DegradationCritical,  // Always available
		"advanced_ai":     DegradationNone,      // Only available when fully healthy
		"basic_storage":   DegradationCritical,  // Always available
	}

	requiredLevel, exists := featureRequirements[featureName]
	if !exists {
		// Unknown feature - assume it needs full health
		return degradationLevel == DegradationNone
	}

	return degradationLevel <= requiredLevel
}

// SetServiceMetadata sets metadata for a service
func (dm *DegradationManager) SetServiceMetadata(serviceName string, key string, value interface{}) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if service, exists := dm.services[serviceName]; exists {
		if service.Metadata == nil {
			service.Metadata = make(map[string]interface{})
		}
		service.Metadata[key] = value
	}
}

// StartHealthChecks starts periodic health checks for registered services
func (dm *DegradationManager) StartHealthChecks(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(dm.config.HealthCheckInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				dm.performHealthChecks(ctx)
			}
		}
	}()

	dm.logger.Info("Health checks started")
}

// performHealthChecks performs health checks on all registered services
func (dm *DegradationManager) performHealthChecks(ctx context.Context) {
	dm.mu.RLock()
	serviceNames := make([]string, 0, len(dm.services))
	for name := range dm.services {
		serviceNames = append(serviceNames, name)
	}
	dm.mu.RUnlock()

	for _, serviceName := range serviceNames {
		go dm.performServiceHealthCheck(ctx, serviceName)
	}
}

// performServiceHealthCheck performs a health check for a specific service
func (dm *DegradationManager) performServiceHealthCheck(ctx context.Context, serviceName string) {
	// This is a simplified health check
	// In a real implementation, you would ping the actual service
	
	start := time.Now()
	healthy := true
	var err error

	// Simulate health check based on service type
	switch serviceName {
	case "ollama":
		// Check if Ollama is responding
		healthy = dm.checkOllamaHealth(ctx)
	case "database":
		// Check database connectivity
		healthy = dm.checkDatabaseHealth(ctx)
	default:
		// Generic health check
		healthy = true
	}

	duration := time.Since(start)

	if !healthy {
		err = fmt.Errorf("health check failed for %s", serviceName)
	}

	dm.updateServiceStatus(serviceName, err, duration)
}

// checkOllamaHealth checks Ollama service health
func (dm *DegradationManager) checkOllamaHealth(ctx context.Context) bool {
	// This would typically make a call to Ollama's health endpoint
	// For now, return true as a placeholder
	return true
}

// checkDatabaseHealth checks database health
func (dm *DegradationManager) checkDatabaseHealth(ctx context.Context) bool {
	// This would typically check database connectivity
	// For now, return true as a placeholder
	return true
}

// GetDegradationStats returns statistics about degradation
func (dm *DegradationManager) GetDegradationStats() map[string]interface{} {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	stats := map[string]interface{}{
		"system_degradation_level": dm.GetSystemDegradationLevel(),
		"total_services":          len(dm.services),
		"service_breakdown":       make(map[ServiceHealthStatus]int),
		"degradation_breakdown":   make(map[DegradationLevel]int),
		"circuit_breaker_stats":   dm.circuitBreaker.GetStats(),
		"fallback_cache_stats":    dm.fallbackCache.GetStats(),
	}

	serviceBreakdown := stats["service_breakdown"].(map[ServiceHealthStatus]int)
	degradationBreakdown := stats["degradation_breakdown"].(map[DegradationLevel]int)

	for _, service := range dm.services {
		serviceBreakdown[service.Status]++
		degradationBreakdown[service.DegradationLevel]++
	}

	return stats
}