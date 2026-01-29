package main

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"
)

// RorNameCache manages an in-memory cache of ROR ID to organization name mappings
// It periodically refreshes the cache from the database and ROR API
type RorNameCache struct {
	rorRepo     RorRepository
	rorClient   *RorClient
	cache       map[string]string // rorID -> organization name
	mutex       sync.RWMutex
	refreshTTL  time.Duration
	stopChan    chan struct{}
	lastRefresh time.Time
}

// NewRorNameCache creates a new ROR name cache with 24-hour refresh interval
func NewRorNameCache(rorRepo RorRepository, rorClient *RorClient) *RorNameCache {
	cache := &RorNameCache{
		rorRepo:    rorRepo,
		rorClient:  rorClient,
		cache:      make(map[string]string),
		refreshTTL: 24 * time.Hour,
		stopChan:   make(chan struct{}),
	}

	// Initial load
	if err := cache.refresh(); err != nil {
		log.Printf("Warning: Initial ROR name cache load failed: %v", err)
	}

	// Start background refresh job
	go cache.startRefreshJob()

	return cache
}

// startRefreshJob runs periodic refresh of the cache
func (c *RorNameCache) startRefreshJob() {
	ticker := time.NewTicker(c.refreshTTL)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := c.refresh(); err != nil {
				log.Printf("Error refreshing ROR name cache: %v", err)
			}
		case <-c.stopChan:
			return
		}
	}
}

// refresh fetches all ROR IDs from database and their names from ROR API
func (c *RorNameCache) refresh() error {
	startTime := time.Now()
	log.Printf("Starting ROR name cache refresh...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Get all unique ROR IDs from database
	rorIDs, err := c.getAllRorIdsFromDB(ctx)
	if err != nil {
		return err
	}

	if len(rorIDs) == 0 {
		log.Printf("No ROR IDs found in database")
		return nil
	}

	log.Printf("Found %d unique ROR IDs in database", len(rorIDs))

	// Fetch organization names from ROR API
	organizations := c.rorClient.GetOrganizations(rorIDs)

	// Build new cache map
	newCache := make(map[string]string, len(organizations))
	for _, org := range organizations {
		newCache[org.ID] = org.Name
	}

	// Update cache atomically
	c.mutex.Lock()
	c.cache = newCache
	c.lastRefresh = time.Now()
	c.mutex.Unlock()

	duration := time.Since(startTime)
	log.Printf("ROR name cache refreshed: %d organizations cached in %v", len(newCache), duration)

	return nil
}

// getAllRorIdsFromDB retrieves all unique ROR IDs from the database
func (c *RorNameCache) getAllRorIdsFromDB(ctx context.Context) ([]string, error) {
	return c.rorRepo.GetAllUniqueRorIds(ctx)
}

// Search searches for organizations by name with wildcard support
// Returns a list of matching ROR IDs and their names
func (c *RorNameCache) Search(query string) []RorOrganization {
	if query == "" {
		return []RorOrganization{}
	}

	query = strings.ToLower(strings.TrimSpace(query))

	c.mutex.RLock()
	defer c.mutex.RUnlock()

	var results []RorOrganization
	for rorID, name := range c.cache {
		nameLower := strings.ToLower(name)
		if strings.Contains(nameLower, query) {
			results = append(results, RorOrganization{
				ID:   rorID,
				Name: name,
			})
		}
	}

	return results
}

// Get retrieves an organization name by ROR ID from cache
func (c *RorNameCache) Get(rorID string) (string, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	name, found := c.cache[rorID]
	return name, found
}

// Set adds or updates an organization name in the cache
func (c *RorNameCache) Set(rorID string, name string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.cache[rorID] = name
}

// AddRorIds fetches and caches organization names for the given ROR IDs
// This is useful for immediately caching newly added ROR IDs
func (c *RorNameCache) AddRorIds(rorIDs []string) {
	if len(rorIDs) == 0 {
		return
	}

	// Check which IDs are not already in cache
	c.mutex.RLock()
	missingIDs := make([]string, 0)
	for _, rorID := range rorIDs {
		if _, found := c.cache[rorID]; !found {
			missingIDs = append(missingIDs, rorID)
		}
	}
	c.mutex.RUnlock()

	if len(missingIDs) == 0 {
		return // All IDs already cached
	}

	// Fetch organization names from ROR API
	organizations := c.rorClient.GetOrganizations(missingIDs)

	// Add to cache
	c.mutex.Lock()
	for _, org := range organizations {
		c.cache[org.ID] = org.Name
	}
	c.mutex.Unlock()

	log.Printf("Added %d new ROR organizations to cache", len(organizations))
}

// Size returns the current number of cached organizations
func (c *RorNameCache) Size() int {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return len(c.cache)
}

// LastRefresh returns the timestamp of the last successful refresh
func (c *RorNameCache) LastRefresh() time.Time {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.lastRefresh
}

// Stop stops the background refresh job
func (c *RorNameCache) Stop() {
	close(c.stopChan)
}

// ForceRefresh triggers an immediate cache refresh
func (c *RorNameCache) ForceRefresh() error {
	return c.refresh()
}
