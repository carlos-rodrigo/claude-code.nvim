package monitoring

import (
	"context"
	"fmt"
	"net/http"
	"runtime"
	"sync"
	"time"

	"claude-code-intelligence/internal/database"

	"github.com/sirupsen/logrus"
)

// HealthChecker manages health checks for system components
type HealthChecker struct {
	mu       sync.RWMutex
	logger   *logrus.Logger
	checks   map[string]HealthCheck
	results  map[string]HealthResult
	interval time.Duration
	stopCh   chan struct{}
}

// HealthCheck represents a health check function
type HealthCheck struct {
	Name        string
	Description string
	CheckFunc   func(ctx context.Context) HealthResult
	Critical    bool // If true, failure causes overall health to be unhealthy
	Timeout     time.Duration
}

// HealthResult represents the result of a health check
type HealthResult struct {
	Status      string            `json:"status"`      // healthy, unhealthy, warning
	Message     string            `json:"message"`
	LastCheck   time.Time         `json:"last_check"`
	Duration    time.Duration     `json:"duration"`
	Details     map[string]interface{} `json:"details,omitempty"`
}

// OverallHealth represents the overall system health
type OverallHealth struct {
	Status     string                   `json:"status"`
	Timestamp  time.Time               `json:"timestamp"`
	Uptime     time.Duration           `json:"uptime"`
	Components map[string]HealthResult `json:"components"`
	Summary    HealthSummary           `json:"summary"`
}

// HealthSummary provides aggregated health information
type HealthSummary struct {
	Total      int `json:"total"`
	Healthy    int `json:"healthy"`
	Unhealthy  int `json:"unhealthy"`
	Warning    int `json:"warning"`
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(logger *logrus.Logger, interval time.Duration) *HealthChecker {
	return &HealthChecker{
		logger:   logger,
		checks:   make(map[string]HealthCheck),
		results:  make(map[string]HealthResult),
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// RegisterCheck registers a new health check
func (hc *HealthChecker) RegisterCheck(check HealthCheck) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	
	if check.Timeout == 0 {
		check.Timeout = 5 * time.Second // Default timeout
	}
	
	hc.checks[check.Name] = check
	hc.logger.WithFields(logrus.Fields{
		"name":        check.Name,
		"description": check.Description,
		"critical":    check.Critical,
		"timeout":     check.Timeout,
	}).Info("Health check registered")
}

// Start begins health check monitoring
func (hc *HealthChecker) Start(ctx context.Context) {
	hc.logger.WithField("interval", hc.interval).Info("Starting health check monitoring")
	
	// Run initial checks
	hc.runAllChecks(ctx)
	
	ticker := time.NewTicker(hc.interval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			hc.logger.Info("Stopping health checks due to context cancellation")
			return
		case <-hc.stopCh:
			hc.logger.Info("Stopping health checks")
			return
		case <-ticker.C:
			hc.runAllChecks(ctx)
		}
	}
}

// Stop stops health check monitoring
func (hc *HealthChecker) Stop() {
	close(hc.stopCh)
}

// GetHealth returns the current health status
func (hc *HealthChecker) GetHealth() OverallHealth {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	
	// Calculate overall status
	overallStatus := "healthy"
	summary := HealthSummary{}
	
	for _, result := range hc.results {
		summary.Total++
		
		switch result.Status {
		case "healthy":
			summary.Healthy++
		case "unhealthy":
			summary.Unhealthy++
			// If any critical check is unhealthy, overall is unhealthy
			if check, exists := hc.checks[getCheckNameFromResult(result)]; exists && check.Critical {
				overallStatus = "unhealthy"
			}
		case "warning":
			summary.Warning++
			// If overall is still healthy, set to warning
			if overallStatus == "healthy" {
				overallStatus = "warning"
			}
		}
	}
	
	return OverallHealth{
		Status:     overallStatus,
		Timestamp:  time.Now(),
		Components: hc.results,
		Summary:    summary,
	}
}

// GetComponentHealth returns health for a specific component
func (hc *HealthChecker) GetComponentHealth(name string) (HealthResult, bool) {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	
	result, exists := hc.results[name]
	return result, exists
}

// runAllChecks runs all registered health checks
func (hc *HealthChecker) runAllChecks(ctx context.Context) {
	hc.mu.RLock()
	checks := make(map[string]HealthCheck)
	for k, v := range hc.checks {
		checks[k] = v
	}
	hc.mu.RUnlock()
	
	// Run checks concurrently
	var wg sync.WaitGroup
	resultsChan := make(chan struct {
		name   string
		result HealthResult
	}, len(checks))
	
	for name, check := range checks {
		wg.Add(1)
		go func(n string, c HealthCheck) {
			defer wg.Done()
			result := hc.runSingleCheck(ctx, c)
			resultsChan <- struct {
				name   string
				result HealthResult
			}{n, result}
		}(name, check)
	}
	
	go func() {
		wg.Wait()
		close(resultsChan)
	}()
	
	// Collect results
	hc.mu.Lock()
	for res := range resultsChan {
		hc.results[res.name] = res.result
	}
	hc.mu.Unlock()
}

// runSingleCheck runs a single health check with timeout
func (hc *HealthChecker) runSingleCheck(ctx context.Context, check HealthCheck) HealthResult {
	start := time.Now()
	
	// Create context with timeout
	checkCtx, cancel := context.WithTimeout(ctx, check.Timeout)
	defer cancel()
	
	// Run the check
	result := check.CheckFunc(checkCtx)
	result.Duration = time.Since(start)
	result.LastCheck = time.Now()
	
	// Log unhealthy checks
	if result.Status != "healthy" {
		hc.logger.WithFields(logrus.Fields{
			"check":    check.Name,
			"status":   result.Status,
			"message":  result.Message,
			"duration": result.Duration,
		}).Warn("Health check failed")
	}
	
	return result
}

// getCheckNameFromResult is a helper to find check name from result
func getCheckNameFromResult(result HealthResult) string {
	// This is a simple implementation - in practice, you might want to
	// store the check name in the result or maintain a reverse mapping
	return ""
}

// Common health check functions

// DatabaseHealthCheck creates a database health check
func DatabaseHealthCheck(db *database.Manager) HealthCheck {
	return HealthCheck{
		Name:        "database",
		Description: "Database connectivity and basic operations",
		Critical:    true,
		Timeout:     5 * time.Second,
		CheckFunc: func(ctx context.Context) HealthResult {
			health := db.HealthCheck(ctx)
			
			status := "healthy"
			if health.Status != "healthy" {
				status = "unhealthy"
			}
			
			return HealthResult{
				Status:  status,
				Message: health.Message,
				Details: map[string]interface{}{
					"status": health.Status,
				},
			}
		},
	}
}

// OllamaHealthCheck creates an Ollama service health check
func OllamaHealthCheck(url string) HealthCheck {
	return HealthCheck{
		Name:        "ollama",
		Description: "Ollama service connectivity",
		Critical:    false, // Non-critical - system can work without AI features
		Timeout:     10 * time.Second,
		CheckFunc: func(ctx context.Context) HealthResult {
			client := &http.Client{Timeout: 5 * time.Second}
			
			req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/api/version", url), nil)
			if err != nil {
				return HealthResult{
					Status:  "unhealthy",
					Message: fmt.Sprintf("Failed to create request: %v", err),
				}
			}
			
			resp, err := client.Do(req)
			if err != nil {
				return HealthResult{
					Status:  "unhealthy",
					Message: fmt.Sprintf("Connection failed: %v", err),
				}
			}
			defer resp.Body.Close()
			
			if resp.StatusCode != http.StatusOK {
				return HealthResult{
					Status:  "unhealthy",
					Message: fmt.Sprintf("HTTP %d", resp.StatusCode),
				}
			}
			
			return HealthResult{
				Status:  "healthy",
				Message: "Ollama service is responding",
				Details: map[string]interface{}{
					"status_code": resp.StatusCode,
				},
			}
		},
	}
}

// MemoryHealthCheck creates a memory usage health check
func MemoryHealthCheck(warningThresholdMB, criticalThresholdMB float64) HealthCheck {
	return HealthCheck{
		Name:        "memory",
		Description: "System memory usage monitoring",
		Critical:    false,
		Timeout:     2 * time.Second,
		CheckFunc: func(ctx context.Context) HealthResult {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			
			usageMB := float64(m.Alloc) / 1024 / 1024
			
			status := "healthy"
			message := fmt.Sprintf("Memory usage: %.2f MB", usageMB)
			
			if usageMB > criticalThresholdMB {
				status = "unhealthy"
				message = fmt.Sprintf("Memory usage critical: %.2f MB (threshold: %.2f MB)", usageMB, criticalThresholdMB)
			} else if usageMB > warningThresholdMB {
				status = "warning"
				message = fmt.Sprintf("Memory usage high: %.2f MB (warning threshold: %.2f MB)", usageMB, warningThresholdMB)
			}
			
			return HealthResult{
				Status:  status,
				Message: message,
				Details: map[string]interface{}{
					"usage_mb":           usageMB,
					"warning_threshold":  warningThresholdMB,
					"critical_threshold": criticalThresholdMB,
					"goroutines":        runtime.NumGoroutine(),
				},
			}
		},
	}
}

// DiskSpaceHealthCheck creates a disk space health check
func DiskSpaceHealthCheck(path string, warningThresholdPercent, criticalThresholdPercent float64) HealthCheck {
	return HealthCheck{
		Name:        "disk_space",
		Description: fmt.Sprintf("Disk space monitoring for %s", path),
		Critical:    false,
		Timeout:     3 * time.Second,
		CheckFunc: func(ctx context.Context) HealthResult {
			// This is a simplified version - in production you'd use syscalls
			// to get actual disk usage statistics
			
			return HealthResult{
				Status:  "healthy",
				Message: "Disk space check not implemented",
				Details: map[string]interface{}{
					"path":               path,
					"warning_threshold":  warningThresholdPercent,
					"critical_threshold": criticalThresholdPercent,
				},
			}
		},
	}
}