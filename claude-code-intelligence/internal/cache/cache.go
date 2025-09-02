package cache

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// CacheManager provides multi-level caching for performance optimization
type CacheManager struct {
	memoryCache *MemoryCache
	diskCache   *DiskCache
	config      *CacheConfig
	logger      *logrus.Logger
	metrics     *CacheMetrics
}

// CacheConfig holds cache configuration
type CacheConfig struct {
	MemoryCacheSize   int           `json:"memory_cache_size"`    // Max items in memory
	DiskCacheSize     int64         `json:"disk_cache_size"`      // Max size in bytes
	DefaultTTL        time.Duration `json:"default_ttl"`          // Default time-to-live
	EvictionPolicy    string        `json:"eviction_policy"`      // LRU, LFU, FIFO
	EnableCompression bool          `json:"enable_compression"`   // Compress disk cache
	CachePath         string        `json:"cache_path"`           // Disk cache location
}

// CacheMetrics tracks cache performance
type CacheMetrics struct {
	mu          sync.RWMutex
	Hits        int64     `json:"hits"`
	Misses      int64     `json:"misses"`
	Evictions   int64     `json:"evictions"`
	TotalSize   int64     `json:"total_size"`
	LastCleanup time.Time `json:"last_cleanup"`
}

// CacheEntry represents a cached item
type CacheEntry struct {
	Key        string      `json:"key"`
	Value      interface{} `json:"value"`
	CreatedAt  time.Time   `json:"created_at"`
	ExpiresAt  time.Time   `json:"expires_at"`
	AccessCount int64      `json:"access_count"`
	Size       int64       `json:"size"`
}

// NewCacheManager creates a new cache manager
func NewCacheManager(config *CacheConfig, logger *logrus.Logger) *CacheManager {
	if config == nil {
		config = &CacheConfig{
			MemoryCacheSize: 1000,
			DiskCacheSize:   100 * 1024 * 1024, // 100MB
			DefaultTTL:      15 * time.Minute,
			EvictionPolicy:  "LRU",
			CachePath:       "./data/cache",
		}
	}

	manager := &CacheManager{
		memoryCache: NewMemoryCache(config.MemoryCacheSize, config.EvictionPolicy),
		diskCache:   NewDiskCache(config.CachePath, config.DiskCacheSize),
		config:      config,
		logger:      logger,
		metrics:     &CacheMetrics{},
	}

	// Start background cleanup
	go manager.cleanupRoutine()

	return manager
}

// Get retrieves a value from cache
func (cm *CacheManager) Get(ctx context.Context, key string) (interface{}, error) {
	// Try memory cache first
	if value, found := cm.memoryCache.Get(key); found {
		cm.recordHit()
		return value, nil
	}

	// Try disk cache
	if value, err := cm.diskCache.Get(key); err == nil {
		// Promote to memory cache
		cm.memoryCache.Set(key, value, cm.config.DefaultTTL)
		cm.recordHit()
		return value, nil
	}

	cm.recordMiss()
	return nil, fmt.Errorf("cache miss for key: %s", key)
}

// Set stores a value in cache
func (cm *CacheManager) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if ttl == 0 {
		ttl = cm.config.DefaultTTL
	}

	// Store in memory cache
	cm.memoryCache.Set(key, value, ttl)

	// Store in disk cache for persistence
	if err := cm.diskCache.Set(key, value, ttl); err != nil {
		cm.logger.WithError(err).Warn("Failed to store in disk cache")
	}

	return nil
}

// Delete removes a value from cache
func (cm *CacheManager) Delete(ctx context.Context, key string) error {
	cm.memoryCache.Delete(key)
	cm.diskCache.Delete(key)
	return nil
}

// Clear removes all cached items
func (cm *CacheManager) Clear(ctx context.Context) error {
	cm.memoryCache.Clear()
	cm.diskCache.Clear()
	cm.resetMetrics()
	return nil
}

// GetStats returns cache statistics
func (cm *CacheManager) GetStats() map[string]interface{} {
	cm.metrics.mu.RLock()
	defer cm.metrics.mu.RUnlock()

	hitRate := float64(0)
	total := cm.metrics.Hits + cm.metrics.Misses
	if total > 0 {
		hitRate = float64(cm.metrics.Hits) / float64(total) * 100
	}

	return map[string]interface{}{
		"hits":          cm.metrics.Hits,
		"misses":        cm.metrics.Misses,
		"hit_rate":      hitRate,
		"evictions":     cm.metrics.Evictions,
		"memory_items":  cm.memoryCache.Size(),
		"disk_size":     cm.diskCache.Size(),
		"last_cleanup":  cm.metrics.LastCleanup,
	}
}

// recordHit increments hit counter
func (cm *CacheManager) recordHit() {
	cm.metrics.mu.Lock()
	cm.metrics.Hits++
	cm.metrics.mu.Unlock()
}

// recordMiss increments miss counter
func (cm *CacheManager) recordMiss() {
	cm.metrics.mu.Lock()
	cm.metrics.Misses++
	cm.metrics.mu.Unlock()
}

// recordEviction increments eviction counter
func (cm *CacheManager) recordEviction() {
	cm.metrics.mu.Lock()
	cm.metrics.Evictions++
	cm.metrics.mu.Unlock()
}

// resetMetrics resets all metrics
func (cm *CacheManager) resetMetrics() {
	cm.metrics.mu.Lock()
	cm.metrics.Hits = 0
	cm.metrics.Misses = 0
	cm.metrics.Evictions = 0
	cm.metrics.mu.Unlock()
}

// cleanupRoutine runs periodic cleanup
func (cm *CacheManager) cleanupRoutine() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		cm.cleanup()
	}
}

// cleanup removes expired entries
func (cm *CacheManager) cleanup() {
	evicted := cm.memoryCache.Cleanup()
	cm.diskCache.Cleanup()

	cm.metrics.mu.Lock()
	cm.metrics.Evictions += int64(evicted)
	cm.metrics.LastCleanup = time.Now()
	cm.metrics.mu.Unlock()

	if evicted > 0 {
		cm.logger.WithField("evicted", evicted).Debug("Cache cleanup completed")
	}
}

// MemoryCache provides in-memory caching with LRU eviction
type MemoryCache struct {
	mu       sync.RWMutex
	items    map[string]*CacheEntry
	maxSize  int
	policy   string
	lruList  []string // Track access order for LRU
}

// NewMemoryCache creates a new memory cache
func NewMemoryCache(maxSize int, policy string) *MemoryCache {
	return &MemoryCache{
		items:   make(map[string]*CacheEntry),
		maxSize: maxSize,
		policy:  policy,
		lruList: make([]string, 0, maxSize),
	}
}

// Get retrieves a value from memory cache
func (mc *MemoryCache) Get(key string) (interface{}, bool) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	entry, exists := mc.items[key]
	if !exists {
		return nil, false
	}

	// Check expiration
	if time.Now().After(entry.ExpiresAt) {
		delete(mc.items, key)
		mc.removeFromLRU(key)
		return nil, false
	}

	// Update access count and LRU order
	entry.AccessCount++
	mc.updateLRU(key)

	return entry.Value, true
}

// Set stores a value in memory cache
func (mc *MemoryCache) Set(key string, value interface{}, ttl time.Duration) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	// Evict if at capacity
	if len(mc.items) >= mc.maxSize && mc.items[key] == nil {
		mc.evictLRU()
	}

	entry := &CacheEntry{
		Key:        key,
		Value:      value,
		CreatedAt:  time.Now(),
		ExpiresAt:  time.Now().Add(ttl),
		AccessCount: 0,
	}

	mc.items[key] = entry
	mc.updateLRU(key)
}

// Delete removes a value from memory cache
func (mc *MemoryCache) Delete(key string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	delete(mc.items, key)
	mc.removeFromLRU(key)
}

// Clear removes all items from memory cache
func (mc *MemoryCache) Clear() {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.items = make(map[string]*CacheEntry)
	mc.lruList = make([]string, 0, mc.maxSize)
}

// Size returns the number of items in cache
func (mc *MemoryCache) Size() int {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return len(mc.items)
}

// Cleanup removes expired entries
func (mc *MemoryCache) Cleanup() int {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	evicted := 0
	now := time.Now()

	for key, entry := range mc.items {
		if now.After(entry.ExpiresAt) {
			delete(mc.items, key)
			mc.removeFromLRU(key)
			evicted++
		}
	}

	return evicted
}

// updateLRU updates the LRU list for a key
func (mc *MemoryCache) updateLRU(key string) {
	// Remove from current position
	mc.removeFromLRU(key)
	// Add to end (most recently used)
	mc.lruList = append(mc.lruList, key)
}

// removeFromLRU removes a key from the LRU list
func (mc *MemoryCache) removeFromLRU(key string) {
	for i, k := range mc.lruList {
		if k == key {
			mc.lruList = append(mc.lruList[:i], mc.lruList[i+1:]...)
			break
		}
	}
}

// evictLRU removes the least recently used item
func (mc *MemoryCache) evictLRU() {
	if len(mc.lruList) > 0 {
		lruKey := mc.lruList[0]
		delete(mc.items, lruKey)
		mc.lruList = mc.lruList[1:]
	}
}

// DiskCache provides persistent disk-based caching
type DiskCache struct {
	mu        sync.RWMutex
	path      string
	maxSize   int64
	index     map[string]*DiskCacheEntry
}

// DiskCacheEntry represents an entry in disk cache
type DiskCacheEntry struct {
	Key       string    `json:"key"`
	Filename  string    `json:"filename"`
	Size      int64     `json:"size"`
	ExpiresAt time.Time `json:"expires_at"`
}

// NewDiskCache creates a new disk cache
func NewDiskCache(path string, maxSize int64) *DiskCache {
	return &DiskCache{
		path:    path,
		maxSize: maxSize,
		index:   make(map[string]*DiskCacheEntry),
	}
}

// Get retrieves a value from disk cache
func (dc *DiskCache) Get(key string) (interface{}, error) {
	dc.mu.RLock()
	defer dc.mu.RUnlock()

	entry, exists := dc.index[key]
	if !exists {
		return nil, fmt.Errorf("key not found in disk cache")
	}

	// Check expiration
	if time.Now().After(entry.ExpiresAt) {
		dc.mu.RUnlock()
		dc.mu.Lock()
		delete(dc.index, key)
		dc.mu.Unlock()
		dc.mu.RLock()
		return nil, fmt.Errorf("cache entry expired")
	}

	// Read from disk
	// In a real implementation, this would read from the file
	// For now, return a placeholder
	return fmt.Sprintf("disk_cache_value_%s", key), nil
}

// Set stores a value in disk cache
func (dc *DiskCache) Set(key string, value interface{}, ttl time.Duration) error {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	// In a real implementation, this would write to disk
	entry := &DiskCacheEntry{
		Key:       key,
		Filename:  fmt.Sprintf("%s/%s.cache", dc.path, key),
		Size:      100, // Placeholder size
		ExpiresAt: time.Now().Add(ttl),
	}

	dc.index[key] = entry
	return nil
}

// Delete removes a value from disk cache
func (dc *DiskCache) Delete(key string) {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	delete(dc.index, key)
	// In a real implementation, this would also delete the file
}

// Clear removes all items from disk cache
func (dc *DiskCache) Clear() {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	dc.index = make(map[string]*DiskCacheEntry)
	// In a real implementation, this would clear all cache files
}

// Size returns the total size of disk cache
func (dc *DiskCache) Size() int64 {
	dc.mu.RLock()
	defer dc.mu.RUnlock()

	var totalSize int64
	for _, entry := range dc.index {
		totalSize += entry.Size
	}
	return totalSize
}

// Cleanup removes expired entries from disk cache
func (dc *DiskCache) Cleanup() int {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	evicted := 0
	now := time.Now()

	for key, entry := range dc.index {
		if now.After(entry.ExpiresAt) {
			delete(dc.index, key)
			evicted++
			// In a real implementation, this would also delete the file
		}
	}

	return evicted
}

// CacheKey generates a cache key from components
func CacheKey(components ...string) string {
	result := ""
	for i, comp := range components {
		if i > 0 {
			result += ":"
		}
		result += comp
	}
	return result
}

// CacheSessionKey generates a cache key for session data
func CacheSessionKey(sessionID string) string {
	return CacheKey("session", sessionID)
}

// CacheContextKey generates a cache key for context data
func CacheContextKey(sessionID, projectID string) string {
	return CacheKey("context", sessionID, projectID)
}

// CacheSearchKey generates a cache key for search results
func CacheSearchKey(query string, limit int) string {
	return CacheKey("search", query, fmt.Sprintf("%d", limit))
}