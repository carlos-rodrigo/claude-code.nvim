package degradation

import (
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	mu       sync.RWMutex
	services map[string]*CircuitState
	config   *CircuitBreakerConfig
	logger   *logrus.Logger
}

// CircuitBreakerConfig contains circuit breaker configuration
type CircuitBreakerConfig struct {
	FailureThreshold int           `json:"failure_threshold"`
	RecoveryTimeout  time.Duration `json:"recovery_timeout"`
	HalfOpenMaxCalls int           `json:"half_open_max_calls"`
}

// CircuitState represents the state of a circuit for a specific service
type CircuitState struct {
	State         CircuitStateType `json:"state"`
	FailureCount  int              `json:"failure_count"`
	LastFailure   time.Time        `json:"last_failure"`
	NextAttempt   time.Time        `json:"next_attempt"`
	HalfOpenCalls int              `json:"half_open_calls"`
	TotalCalls    int64            `json:"total_calls"`
	SuccessCalls  int64            `json:"success_calls"`
}

// CircuitStateType represents the state of a circuit breaker
type CircuitStateType string

const (
	CircuitClosed   CircuitStateType = "closed"   // Normal operation
	CircuitOpen     CircuitStateType = "open"     // Failing fast
	CircuitHalfOpen CircuitStateType = "half_open" // Testing recovery
)

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(config *CircuitBreakerConfig, logger *logrus.Logger) *CircuitBreaker {
	return &CircuitBreaker{
		services: make(map[string]*CircuitState),
		config:   config,
		logger:   logger,
	}
}

// CanCall checks if a call to the service is allowed
func (cb *CircuitBreaker) CanCall(serviceName string) bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	state := cb.getOrCreateState(serviceName)
	now := time.Now()

	switch state.State {
	case CircuitClosed:
		return true

	case CircuitOpen:
		// Check if it's time to try recovery
		if now.After(state.NextAttempt) {
			state.State = CircuitHalfOpen
			state.HalfOpenCalls = 0
			cb.logger.WithField("service", serviceName).Info("Circuit breaker transitioning to half-open")
			return true
		}
		return false

	case CircuitHalfOpen:
		// Allow limited calls to test recovery
		if state.HalfOpenCalls < cb.config.HalfOpenMaxCalls {
			state.HalfOpenCalls++
			return true
		}
		return false

	default:
		return false
	}
}

// RecordSuccess records a successful call
func (cb *CircuitBreaker) RecordSuccess(serviceName string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	state := cb.getOrCreateState(serviceName)
	state.TotalCalls++
	state.SuccessCalls++

	switch state.State {
	case CircuitClosed:
		// Reset failure count on success
		state.FailureCount = 0

	case CircuitHalfOpen:
		// Check if we've had enough successful calls to close the circuit
		if state.HalfOpenCalls >= cb.config.HalfOpenMaxCalls {
			state.State = CircuitClosed
			state.FailureCount = 0
			state.HalfOpenCalls = 0
			cb.logger.WithField("service", serviceName).Info("Circuit breaker closed - service recovered")
		}
	}
}

// RecordFailure records a failed call
func (cb *CircuitBreaker) RecordFailure(serviceName string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	state := cb.getOrCreateState(serviceName)
	state.TotalCalls++
	state.FailureCount++
	state.LastFailure = time.Now()

	switch state.State {
	case CircuitClosed:
		// Check if we should open the circuit
		if state.FailureCount >= cb.config.FailureThreshold {
			state.State = CircuitOpen
			state.NextAttempt = time.Now().Add(cb.config.RecoveryTimeout)
			cb.logger.WithFields(logrus.Fields{
				"service":         serviceName,
				"failure_count":   state.FailureCount,
				"next_attempt":    state.NextAttempt.Format(time.RFC3339),
			}).Warn("Circuit breaker opened - service failing")
		}

	case CircuitHalfOpen:
		// Failure during half-open means we go back to open
		state.State = CircuitOpen
		state.NextAttempt = time.Now().Add(cb.config.RecoveryTimeout)
		state.HalfOpenCalls = 0
		cb.logger.WithField("service", serviceName).Warn("Circuit breaker re-opened after half-open failure")
	}
}

// getOrCreateState gets or creates a circuit state for a service
func (cb *CircuitBreaker) getOrCreateState(serviceName string) *CircuitState {
	state, exists := cb.services[serviceName]
	if !exists {
		state = &CircuitState{
			State:        CircuitClosed,
			FailureCount: 0,
		}
		cb.services[serviceName] = state
	}
	return state
}

// GetState returns the current state of a circuit
func (cb *CircuitBreaker) GetState(serviceName string) *CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	state, exists := cb.services[serviceName]
	if !exists {
		return &CircuitState{State: CircuitClosed}
	}

	// Return a copy to prevent external modification
	stateCopy := *state
	return &stateCopy
}

// GetStats returns circuit breaker statistics
func (cb *CircuitBreaker) GetStats() map[string]interface{} {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	stats := map[string]interface{}{
		"total_circuits": len(cb.services),
		"states":         make(map[CircuitStateType]int),
		"circuits":       make(map[string]*CircuitState),
	}

	stateBreakdown := stats["states"].(map[CircuitStateType]int)
	circuits := stats["circuits"].(map[string]*CircuitState)

	for serviceName, state := range cb.services {
		stateBreakdown[state.State]++
		// Store a copy
		stateCopy := *state
		circuits[serviceName] = &stateCopy
	}

	return stats
}

// Reset resets the circuit breaker for a specific service
func (cb *CircuitBreaker) Reset(serviceName string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if state, exists := cb.services[serviceName]; exists {
		state.State = CircuitClosed
		state.FailureCount = 0
		state.HalfOpenCalls = 0
		cb.logger.WithField("service", serviceName).Info("Circuit breaker reset")
	}
}