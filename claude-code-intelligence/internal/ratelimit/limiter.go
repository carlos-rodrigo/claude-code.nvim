package ratelimit

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// RateLimiter provides advanced rate limiting functionality
type RateLimiter struct {
	mu           sync.RWMutex
	clients      map[string]*ClientLimiter
	globalConfig *GlobalConfig
	logger       *logrus.Logger
	cleanupTicker *time.Ticker
	stopCleanup  chan struct{}
}

// GlobalConfig contains global rate limiting configuration
type GlobalConfig struct {
	DefaultRateLimit  int           `json:"default_rate_limit"`  // requests per minute
	DefaultBurstLimit int           `json:"default_burst_limit"` // max burst requests
	CleanupInterval   time.Duration `json:"cleanup_interval"`
	ClientTTL         time.Duration `json:"client_ttl"`
	MaxClients        int           `json:"max_clients"`
	
	// Per-endpoint limits
	EndpointLimits map[string]EndpointLimit `json:"endpoint_limits"`
	
	// Global limits
	GlobalRequestsPerSecond int `json:"global_requests_per_second"`
	GlobalBurstLimit       int `json:"global_burst_limit"`
}

// EndpointLimit defines rate limits for specific endpoints
type EndpointLimit struct {
	RequestsPerMinute int      `json:"requests_per_minute"`
	BurstLimit        int      `json:"burst_limit"`
	Methods           []string `json:"methods"` // HTTP methods this applies to
}

// ClientLimiter tracks rate limiting for a specific client
type ClientLimiter struct {
	ID               string
	RequestsPerMinute int
	BurstLimit       int
	
	// Token bucket implementation
	tokens     int
	lastRefill time.Time
	
	// Statistics
	totalRequests   int64
	blockedRequests int64
	lastRequest     time.Time
	
	// Per-endpoint tracking
	endpointLimits map[string]*EndpointTracker
}

// EndpointTracker tracks requests for a specific endpoint
type EndpointTracker struct {
	tokens         int
	lastRefill     time.Time
	requestsPerMinute int
	burstLimit     int
	totalRequests  int64
	blockedRequests int64
}

// RateLimitResult represents the result of a rate limit check
type RateLimitResult struct {
	Allowed         bool          `json:"allowed"`
	Reason          string        `json:"reason"`
	RemainingTokens int           `json:"remaining_tokens"`
	RetryAfter      time.Duration `json:"retry_after"`
	ResetTime       time.Time     `json:"reset_time"`
}

// RateLimitStats contains statistics about rate limiting
type RateLimitStats struct {
	TotalClients       int                    `json:"total_clients"`
	ActiveClients      int                    `json:"active_clients"`
	GlobalStats        *GlobalStats           `json:"global_stats"`
	ClientStats        []*ClientStats         `json:"client_stats"`
	EndpointStats      map[string]*EndpointStats `json:"endpoint_stats"`
}

// GlobalStats contains global rate limiting statistics
type GlobalStats struct {
	TotalRequests       int64     `json:"total_requests"`
	BlockedRequests     int64     `json:"blocked_requests"`
	RequestsPerSecond   float64   `json:"requests_per_second"`
	AverageResponseTime time.Duration `json:"average_response_time"`
	LastRequest         time.Time `json:"last_request"`
}

// ClientStats contains per-client statistics
type ClientStats struct {
	ID              string    `json:"id"`
	TotalRequests   int64     `json:"total_requests"`
	BlockedRequests int64     `json:"blocked_requests"`
	LastRequest     time.Time `json:"last_request"`
	RemainingTokens int       `json:"remaining_tokens"`
	IsActive        bool      `json:"is_active"`
}

// EndpointStats contains per-endpoint statistics
type EndpointStats struct {
	Endpoint        string  `json:"endpoint"`
	TotalRequests   int64   `json:"total_requests"`
	BlockedRequests int64   `json:"blocked_requests"`
	BlockRate       float64 `json:"block_rate"`
	AverageRPS      float64 `json:"average_rps"`
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(config *GlobalConfig, logger *logrus.Logger) *RateLimiter {
	if config == nil {
		config = &GlobalConfig{
			DefaultRateLimit:        100,
			DefaultBurstLimit:       150,
			CleanupInterval:         5 * time.Minute,
			ClientTTL:               1 * time.Hour,
			MaxClients:             10000,
			EndpointLimits:         make(map[string]EndpointLimit),
			GlobalRequestsPerSecond: 1000,
			GlobalBurstLimit:       1500,
		}
	}

	rl := &RateLimiter{
		clients:      make(map[string]*ClientLimiter),
		globalConfig: config,
		logger:       logger,
		stopCleanup:  make(chan struct{}),
	}

	// Start cleanup routine
	rl.startCleanup()

	logger.WithFields(logrus.Fields{
		"default_rate_limit":  config.DefaultRateLimit,
		"default_burst_limit": config.DefaultBurstLimit,
		"max_clients":        config.MaxClients,
	}).Info("Rate limiter initialized")

	return rl
}

// CheckLimit checks if a request should be allowed
func (rl *RateLimiter) CheckLimit(clientID, endpoint, method string, customRateLimit, customBurstLimit int) *RateLimitResult {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	// Get or create client limiter
	client, exists := rl.clients[clientID]
	if !exists {
		// Check max clients limit
		if len(rl.clients) >= rl.globalConfig.MaxClients {
			return &RateLimitResult{
				Allowed:    false,
				Reason:     "Maximum number of clients exceeded",
				RetryAfter: time.Minute,
				ResetTime:  now.Add(time.Minute),
			}
		}

		// Create new client limiter
		rateLimit := rl.globalConfig.DefaultRateLimit
		burstLimit := rl.globalConfig.DefaultBurstLimit
		
		if customRateLimit > 0 {
			rateLimit = customRateLimit
		}
		if customBurstLimit > 0 {
			burstLimit = customBurstLimit
		}

		client = &ClientLimiter{
			ID:               clientID,
			RequestsPerMinute: rateLimit,
			BurstLimit:       burstLimit,
			tokens:           burstLimit,
			lastRefill:       now,
			endpointLimits:   make(map[string]*EndpointTracker),
		}
		rl.clients[clientID] = client
	}

	// Update last request time
	client.lastRequest = now

	// Check global rate limit first
	globalResult := rl.checkGlobalLimit(now)
	if !globalResult.Allowed {
		client.blockedRequests++
		return globalResult
	}

	// Check client-level rate limit
	clientResult := rl.checkClientLimit(client, now)
	if !clientResult.Allowed {
		client.blockedRequests++
		return clientResult
	}

	// Check endpoint-specific rate limit
	endpointResult := rl.checkEndpointLimit(client, endpoint, method, now)
	if !endpointResult.Allowed {
		client.blockedRequests++
		return endpointResult
	}

	// All checks passed - consume tokens and allow request
	client.tokens--
	client.totalRequests++

	return &RateLimitResult{
		Allowed:         true,
		RemainingTokens: client.tokens,
		ResetTime:       rl.calculateResetTime(now),
	}
}

// checkGlobalLimit checks global rate limiting
func (rl *RateLimiter) checkGlobalLimit(now time.Time) *RateLimitResult {
	// This is a simplified implementation
	// In production, you might use Redis or another distributed store
	return &RateLimitResult{
		Allowed: true, // For now, always allow global requests
	}
}

// checkClientLimit checks per-client rate limiting using token bucket
func (rl *RateLimiter) checkClientLimit(client *ClientLimiter, now time.Time) *RateLimitResult {
	// Refill tokens based on time elapsed
	elapsed := now.Sub(client.lastRefill)
	if elapsed > 0 {
		// Calculate tokens to add (requests per minute converted to per second)
		tokensToAdd := int(elapsed.Seconds() * float64(client.RequestsPerMinute) / 60.0)
		if tokensToAdd > 0 {
			client.tokens += tokensToAdd
			if client.tokens > client.BurstLimit {
				client.tokens = client.BurstLimit
			}
			client.lastRefill = now
		}
	}

	// Check if we have tokens available
	if client.tokens <= 0 {
		// Calculate retry after time
		tokensNeeded := 1
		secondsPerToken := 60.0 / float64(client.RequestsPerMinute)
		retryAfter := time.Duration(float64(tokensNeeded) * secondsPerToken * float64(time.Second))

		return &RateLimitResult{
			Allowed:         false,
			Reason:          "Client rate limit exceeded",
			RemainingTokens: 0,
			RetryAfter:      retryAfter,
			ResetTime:       now.Add(retryAfter),
		}
	}

	return &RateLimitResult{
		Allowed:         true,
		RemainingTokens: client.tokens - 1, // -1 because we'll consume one
	}
}

// checkEndpointLimit checks endpoint-specific rate limiting
func (rl *RateLimiter) checkEndpointLimit(client *ClientLimiter, endpoint, method string, now time.Time) *RateLimitResult {
	// Check if there are endpoint-specific limits
	endpointLimit, hasEndpointLimit := rl.globalConfig.EndpointLimits[endpoint]
	if !hasEndpointLimit {
		return &RateLimitResult{Allowed: true}
	}

	// Check if the method is covered by this limit
	if len(endpointLimit.Methods) > 0 {
		methodAllowed := false
		for _, allowedMethod := range endpointLimit.Methods {
			if allowedMethod == method || allowedMethod == "*" {
				methodAllowed = true
				break
			}
		}
		if !methodAllowed {
			return &RateLimitResult{Allowed: true}
		}
	}

	// Get or create endpoint tracker
	tracker, exists := client.endpointLimits[endpoint]
	if !exists {
		tracker = &EndpointTracker{
			tokens:            endpointLimit.BurstLimit,
			lastRefill:        now,
			requestsPerMinute: endpointLimit.RequestsPerMinute,
			burstLimit:        endpointLimit.BurstLimit,
		}
		client.endpointLimits[endpoint] = tracker
	}

	// Refill tokens for endpoint
	elapsed := now.Sub(tracker.lastRefill)
	if elapsed > 0 {
		tokensToAdd := int(elapsed.Seconds() * float64(tracker.requestsPerMinute) / 60.0)
		if tokensToAdd > 0 {
			tracker.tokens += tokensToAdd
			if tracker.tokens > tracker.burstLimit {
				tracker.tokens = tracker.burstLimit
			}
			tracker.lastRefill = now
		}
	}

	// Check endpoint tokens
	if tracker.tokens <= 0 {
		tracker.blockedRequests++
		secondsPerToken := 60.0 / float64(tracker.requestsPerMinute)
		retryAfter := time.Duration(secondsPerToken * float64(time.Second))

		return &RateLimitResult{
			Allowed:    false,
			Reason:     fmt.Sprintf("Endpoint rate limit exceeded for %s", endpoint),
			RetryAfter: retryAfter,
			ResetTime:  now.Add(retryAfter),
		}
	}

	// Consume endpoint token
	tracker.tokens--
	tracker.totalRequests++

	return &RateLimitResult{Allowed: true}
}

// calculateResetTime calculates when the rate limit will reset
func (rl *RateLimiter) calculateResetTime(now time.Time) time.Time {
	// Tokens refill continuously, but for API purposes, show next minute
	return now.Add(time.Minute)
}

// GetStats returns rate limiting statistics
func (rl *RateLimiter) GetStats() *RateLimitStats {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	stats := &RateLimitStats{
		TotalClients:  len(rl.clients),
		ClientStats:   make([]*ClientStats, 0),
		EndpointStats: make(map[string]*EndpointStats),
		GlobalStats: &GlobalStats{
			LastRequest: time.Now(),
		},
	}

	activeClients := 0
	now := time.Now()

	// Collect client stats
	for _, client := range rl.clients {
		isActive := now.Sub(client.lastRequest) < time.Hour

		if isActive {
			activeClients++
		}

		clientStat := &ClientStats{
			ID:              client.ID,
			TotalRequests:   client.totalRequests,
			BlockedRequests: client.blockedRequests,
			LastRequest:     client.lastRequest,
			RemainingTokens: client.tokens,
			IsActive:        isActive,
		}

		stats.ClientStats = append(stats.ClientStats, clientStat)

		// Add to global stats
		stats.GlobalStats.TotalRequests += client.totalRequests
		stats.GlobalStats.BlockedRequests += client.blockedRequests
	}

	stats.ActiveClients = activeClients

	// Collect endpoint stats
	for endpoint := range rl.globalConfig.EndpointLimits {
		endpointStat := &EndpointStats{
			Endpoint: endpoint,
		}

		// Aggregate stats from all clients for this endpoint
		for _, client := range rl.clients {
			if tracker, exists := client.endpointLimits[endpoint]; exists {
				endpointStat.TotalRequests += tracker.totalRequests
				endpointStat.BlockedRequests += tracker.blockedRequests
			}
		}

		if endpointStat.TotalRequests > 0 {
			endpointStat.BlockRate = float64(endpointStat.BlockedRequests) / float64(endpointStat.TotalRequests) * 100
		}

		stats.EndpointStats[endpoint] = endpointStat
	}

	return stats
}

// UpdateClientLimit updates rate limiting for a specific client
func (rl *RateLimiter) UpdateClientLimit(clientID string, rateLimit, burstLimit int) error {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	client, exists := rl.clients[clientID]
	if !exists {
		return fmt.Errorf("client not found: %s", clientID)
	}

	oldRateLimit := client.RequestsPerMinute
	oldBurstLimit := client.BurstLimit

	client.RequestsPerMinute = rateLimit
	client.BurstLimit = burstLimit

	// Adjust current tokens if burst limit changed
	if client.tokens > burstLimit {
		client.tokens = burstLimit
	}

	rl.logger.WithFields(logrus.Fields{
		"client_id":        clientID,
		"old_rate_limit":   oldRateLimit,
		"new_rate_limit":   rateLimit,
		"old_burst_limit":  oldBurstLimit,
		"new_burst_limit":  burstLimit,
	}).Info("Updated client rate limits")

	return nil
}

// ResetClient resets rate limiting for a specific client
func (rl *RateLimiter) ResetClient(clientID string) error {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	client, exists := rl.clients[clientID]
	if !exists {
		return fmt.Errorf("client not found: %s", clientID)
	}

	// Reset tokens to maximum
	client.tokens = client.BurstLimit
	client.lastRefill = time.Now()

	// Reset endpoint trackers
	for _, tracker := range client.endpointLimits {
		tracker.tokens = tracker.burstLimit
		tracker.lastRefill = time.Now()
	}

	rl.logger.WithField("client_id", clientID).Info("Reset client rate limits")

	return nil
}

// RemoveClient removes a client from rate limiting
func (rl *RateLimiter) RemoveClient(clientID string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	delete(rl.clients, clientID)
	rl.logger.WithField("client_id", clientID).Info("Removed client from rate limiter")
}

// startCleanup starts the cleanup routine for inactive clients
func (rl *RateLimiter) startCleanup() {
	rl.cleanupTicker = time.NewTicker(rl.globalConfig.CleanupInterval)

	go func() {
		for {
			select {
			case <-rl.cleanupTicker.C:
				rl.cleanupInactiveClients()
			case <-rl.stopCleanup:
				rl.cleanupTicker.Stop()
				return
			}
		}
	}()
}

// cleanupInactiveClients removes clients that haven't made requests recently
func (rl *RateLimiter) cleanupInactiveClients() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cleaned := 0

	for clientID, client := range rl.clients {
		if now.Sub(client.lastRequest) > rl.globalConfig.ClientTTL {
			delete(rl.clients, clientID)
			cleaned++
		}
	}

	if cleaned > 0 {
		rl.logger.WithFields(logrus.Fields{
			"cleaned_clients": cleaned,
			"total_clients":   len(rl.clients),
		}).Debug("Cleaned up inactive clients")
	}
}

// Stop stops the rate limiter and cleanup routines
func (rl *RateLimiter) Stop() {
	close(rl.stopCleanup)
	rl.logger.Info("Rate limiter stopped")
}

// AddEndpointLimit adds or updates an endpoint-specific rate limit
func (rl *RateLimiter) AddEndpointLimit(endpoint string, limit EndpointLimit) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.globalConfig.EndpointLimits[endpoint] = limit

	rl.logger.WithFields(logrus.Fields{
		"endpoint":           endpoint,
		"requests_per_minute": limit.RequestsPerMinute,
		"burst_limit":        limit.BurstLimit,
		"methods":            limit.Methods,
	}).Info("Added endpoint rate limit")
}