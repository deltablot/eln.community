package main

import (
	"sync"
	"time"
)

// CacheEntry represents a cached item with expiration
type CacheEntry[T any] struct {
	Value     T
	ExpiresAt time.Time
}

// InMemoryCache is a generic thread-safe cache with TTL support
type InMemoryCache[T any] struct {
	items      map[string]*CacheEntry[T]
	mutex      sync.RWMutex
	ttl        time.Duration
	cleanupTTL time.Duration
	stopChan   chan struct{}
}

// NewInMemoryCache creates a new cache with the specified TTL
func NewInMemoryCache[T any](ttl time.Duration) *InMemoryCache[T] {
	cache := &InMemoryCache[T]{
		items:      make(map[string]*CacheEntry[T]),
		ttl:        ttl,
		cleanupTTL: 1 * time.Hour, // Default cleanup interval
		stopChan:   make(chan struct{}),
	}

	// Start background cleanup
	go cache.startCleanup()

	return cache
}

// NewInMemoryCacheWithCleanup creates a new cache with custom TTL and cleanup interval
func NewInMemoryCacheWithCleanup[T any](ttl, cleanupInterval time.Duration) *InMemoryCache[T] {
	cache := &InMemoryCache[T]{
		items:      make(map[string]*CacheEntry[T]),
		ttl:        ttl,
		cleanupTTL: cleanupInterval,
		stopChan:   make(chan struct{}),
	}

	// Start background cleanup
	go cache.startCleanup()

	return cache
}

// Get retrieves a value from the cache
// Returns the value and true if found and not expired, otherwise returns zero value and false
func (c *InMemoryCache[T]) Get(key string) (T, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	entry, exists := c.items[key]
	if !exists {
		var zero T
		return zero, false
	}

	// Check if expired
	if time.Now().After(entry.ExpiresAt) {
		var zero T
		return zero, false
	}

	return entry.Value, true
}

// Set stores a value in the cache with the default TTL
func (c *InMemoryCache[T]) Set(key string, value T) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.items[key] = &CacheEntry[T]{
		Value:     value,
		ExpiresAt: time.Now().Add(c.ttl),
	}
}

// Clear removes all items from the cache
func (c *InMemoryCache[T]) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.items = make(map[string]*CacheEntry[T])
}

// Size returns the current number of items in the cache
func (c *InMemoryCache[T]) Size() int {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return len(c.items)
}

// Keys returns all keys currently in the cache (including expired ones)
func (c *InMemoryCache[T]) Keys() []string {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	keys := make([]string, 0, len(c.items))
	for key := range c.items {
		keys = append(keys, key)
	}
	return keys
}

// GetMultiple retrieves multiple values from the cache
// Returns a map of found values and a slice of missing keys
func (c *InMemoryCache[T]) GetMultiple(keys []string) (map[string]T, []string) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	found := make(map[string]T)
	missing := make([]string, 0)
	now := time.Now()

	for _, key := range keys {
		entry, exists := c.items[key]
		if !exists || now.After(entry.ExpiresAt) {
			missing = append(missing, key)
			continue
		}
		found[key] = entry.Value
	}

	return found, missing
}

// SetMultiple stores multiple values in the cache
func (c *InMemoryCache[T]) SetMultiple(items map[string]T) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	expiresAt := time.Now().Add(c.ttl)
	for key, value := range items {
		c.items[key] = &CacheEntry[T]{
			Value:     value,
			ExpiresAt: expiresAt,
		}
	}
}

// startCleanup runs periodic cleanup of expired entries
func (c *InMemoryCache[T]) startCleanup() {
	ticker := time.NewTicker(c.cleanupTTL)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.cleanup()
		case <-c.stopChan:
			return
		}
	}
}

// cleanup removes expired entries from the cache
func (c *InMemoryCache[T]) cleanup() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	now := time.Now()
	removed := 0

	for key, entry := range c.items {
		if now.After(entry.ExpiresAt) {
			delete(c.items, key)
			removed++
		}
	}

	if removed > 0 {
		infoLogger.Printf("Cache cleanup: removed %d expired entries, %d remaining", removed, len(c.items))
	}
}

// Stop stops the background cleanup goroutine
func (c *InMemoryCache[T]) Stop() {
	close(c.stopChan)
}

// Has checks if a key exists in the cache and is not expired
func (c *InMemoryCache[T]) Has(key string) bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	entry, exists := c.items[key]
	if !exists {
		return false
	}

	return time.Now().Before(entry.ExpiresAt)
}
