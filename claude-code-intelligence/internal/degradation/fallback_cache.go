package degradation

import (
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// FallbackCache provides caching for fallback responses
type FallbackCache struct {
	mu          sync.RWMutex
	cache       map[string]*CacheEntry
	maxSize     int
	defaultTTL  time.Duration
	logger      *logrus.Logger
	accessOrder []string // For LRU eviction
}

// CacheEntry represents a cached response
type CacheEntry struct {
	Key       string      `json:"key"`
	Data      interface{} `json:"data"`
	CachedAt  time.Time   `json:"cached_at"`
	ExpiresAt time.Time   `json:"expires_at"`
	AccessCount int       `json:"access_count"`
	LastAccess  time.Time `json:"last_access"`
}

// NewFallbackCache creates a new fallback cache
func NewFallbackCache(maxSize int, defaultTTL time.Duration, logger *logrus.Logger) *FallbackCache {
	fc := &FallbackCache{
		cache:       make(map[string]*CacheEntry),
		maxSize:     maxSize,
		defaultTTL:  defaultTTL,
		logger:      logger,
		accessOrder: make([]string, 0),
	}

	// Start cleanup routine
	go fc.startCleanup()

	logger.WithFields(logrus.Fields{
		"max_size":    maxSize,
		"default_ttl": defaultTTL,
	}).Info("Fallback cache initialized")

	return fc
}

// Set stores a response in the cache
func (fc *FallbackCache) Set(serviceName, operation string, data interface{}) {
	fc.SetWithTTL(serviceName, operation, data, fc.defaultTTL)
}

// SetWithTTL stores a response in the cache with a specific TTL
func (fc *FallbackCache) SetWithTTL(serviceName, operation string, data interface{}, ttl time.Duration) {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	key := fc.buildKey(serviceName, operation)
	now := time.Now()

	// Check if we need to evict entries
	if len(fc.cache) >= fc.maxSize {
		fc.evictLRU()
	}

	entry := &CacheEntry{
		Key:       key,
		Data:      data,
		CachedAt:  now,
		ExpiresAt: now.Add(ttl),
		AccessCount: 0,
		LastAccess:  now,
	}

	fc.cache[key] = entry
	fc.updateAccessOrder(key)

	fc.logger.WithFields(logrus.Fields{
		"key":        key,
		"expires_at": entry.ExpiresAt.Format(time.RFC3339),
		"cache_size": len(fc.cache),
	}).Debug("Cached fallback response")
}

// Get retrieves a response from the cache
func (fc *FallbackCache) Get(serviceName, operation string) *CacheEntry {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	key := fc.buildKey(serviceName, operation)
	entry, exists := fc.cache[key]
	
	if !exists {
		return nil
	}

	// Check if entry has expired
	if time.Now().After(entry.ExpiresAt) {
		delete(fc.cache, key)
		fc.removeFromAccessOrder(key)
		return nil
	}

	// Update access statistics
	entry.AccessCount++
	entry.LastAccess = time.Now()
	fc.updateAccessOrder(key)

	fc.logger.WithFields(logrus.Fields{
		"key":          key,
		"access_count": entry.AccessCount,
	}).Debug("Retrieved cached fallback response")

	// Return a copy to prevent external modification
	entryCopy := *entry
	return &entryCopy
}

// Delete removes an entry from the cache
func (fc *FallbackCache) Delete(serviceName, operation string) {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	key := fc.buildKey(serviceName, operation)
	delete(fc.cache, key)
	fc.removeFromAccessOrder(key)

	fc.logger.WithField("key", key).Debug("Deleted cached fallback response")
}

// Clear removes all entries from the cache
func (fc *FallbackCache) Clear() {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	fc.cache = make(map[string]*CacheEntry)
	fc.accessOrder = make([]string, 0)

	fc.logger.Info("Cleared fallback cache")
}

// GetStats returns cache statistics
func (fc *FallbackCache) GetStats() map[string]interface{} {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	now := time.Now()
	totalSize := len(fc.cache)
	expiredCount := 0
	totalAccesses := 0

	oldestCachedAt := now
	newestCachedAt := time.Time{}

	for _, entry := range fc.cache {
		if now.After(entry.ExpiresAt) {
			expiredCount++
		}
		totalAccesses += entry.AccessCount

		if entry.CachedAt.Before(oldestCachedAt) {
			oldestCachedAt = entry.CachedAt
		}
		if entry.CachedAt.After(newestCachedAt) {
			newestCachedAt = entry.CachedAt
		}
	}

	hitRate := 0.0
	if totalAccesses > 0 {
		hitRate = float64(totalAccesses) / float64(totalAccesses+expiredCount) * 100
	}

	stats := map[string]interface{}{
		"total_entries":    totalSize,
		"expired_entries":  expiredCount,
		"valid_entries":    totalSize - expiredCount,
		"max_size":         fc.maxSize,
		"utilization":      float64(totalSize) / float64(fc.maxSize) * 100,
		"total_accesses":   totalAccesses,
		"hit_rate":         hitRate,
		"default_ttl":      fc.defaultTTL.String(),
	}

	if totalSize > 0 {
		stats["oldest_entry"] = oldestCachedAt.Format(time.RFC3339)
		stats["newest_entry"] = newestCachedAt.Format(time.RFC3339)
	}

	return stats
}

// GetAllEntries returns all cache entries (for debugging)
func (fc *FallbackCache) GetAllEntries() map[string]*CacheEntry {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	entries := make(map[string]*CacheEntry)
	for key, entry := range fc.cache {
		// Return copies to prevent external modification
		entryCopy := *entry
		entries[key] = &entryCopy
	}

	return entries
}

// buildKey creates a cache key from service name and operation
func (fc *FallbackCache) buildKey(serviceName, operation string) string {
	return serviceName + ":" + operation
}

// evictLRU evicts the least recently used entry
func (fc *FallbackCache) evictLRU() {
	if len(fc.accessOrder) == 0 {
		return
	}

	// Remove the first (oldest) entry
	keyToEvict := fc.accessOrder[0]
	delete(fc.cache, keyToEvict)
	fc.accessOrder = fc.accessOrder[1:]

	fc.logger.WithFields(logrus.Fields{
		"evicted_key":  keyToEvict,
		"cache_size":   len(fc.cache),
	}).Debug("Evicted LRU cache entry")
}

// updateAccessOrder updates the access order for LRU tracking
func (fc *FallbackCache) updateAccessOrder(key string) {
	// Remove key from current position
	fc.removeFromAccessOrder(key)
	
	// Add to end (most recently used)
	fc.accessOrder = append(fc.accessOrder, key)
}

// removeFromAccessOrder removes a key from the access order list
func (fc *FallbackCache) removeFromAccessOrder(key string) {
	for i, k := range fc.accessOrder {
		if k == key {
			fc.accessOrder = append(fc.accessOrder[:i], fc.accessOrder[i+1:]...)
			break
		}
	}
}

// startCleanup starts the periodic cleanup routine
func (fc *FallbackCache) startCleanup() {
	ticker := time.NewTicker(5 * time.Minute) // Cleanup every 5 minutes
	defer ticker.Stop()

	for range ticker.C {
		fc.cleanupExpired()
	}
}

// cleanupExpired removes expired entries from the cache
func (fc *FallbackCache) cleanupExpired() {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	now := time.Now()
	expired := []string{}

	for key, entry := range fc.cache {
		if now.After(entry.ExpiresAt) {
			expired = append(expired, key)
		}
	}

	// Remove expired entries
	for _, key := range expired {
		delete(fc.cache, key)
		fc.removeFromAccessOrder(key)
	}

	if len(expired) > 0 {
		fc.logger.WithFields(logrus.Fields{
			"expired_count": len(expired),
			"cache_size":    len(fc.cache),
		}).Debug("Cleaned up expired cache entries")
	}
}

// Prewarm preloads the cache with known good responses
func (fc *FallbackCache) Prewarm(serviceName, operation string, data interface{}) {
	fc.logger.WithFields(logrus.Fields{
		"service":   serviceName,
		"operation": operation,
	}).Info("Prewarming cache with known good response")

	fc.Set(serviceName, operation, data)
}

// GetExpiredEntries returns entries that have expired (for diagnostics)
func (fc *FallbackCache) GetExpiredEntries() []string {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	now := time.Now()
	expired := []string{}

	for key, entry := range fc.cache {
		if now.After(entry.ExpiresAt) {
			expired = append(expired, key)
		}
	}

	return expired
}