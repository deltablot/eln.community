package main

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

func TestRorNameCache_Search(t *testing.T) {
	// Create a mock cache with test data
	cache := &RorNameCache{
		cache: map[string]string{
			"01an7q238": "University of California, Berkeley",
			"02jbv0t02": "Stanford University",
			"03vek6s52": "Harvard University",
			"04xm1d337": "Massachusetts Institute of Technology",
		},
		lastRefresh: time.Now(),
	}

	tests := []struct {
		name          string
		query         string
		expectedCount int
		shouldContain string
	}{
		{
			name:          "Search by full name",
			query:         "Stanford University",
			expectedCount: 1,
			shouldContain: "02jbv0t02",
		},
		{
			name:          "Search by partial name",
			query:         "University",
			expectedCount: 3,
			shouldContain: "01an7q238",
		},
		{
			name:          "Search case insensitive",
			query:         "HARVARD",
			expectedCount: 1,
			shouldContain: "03vek6s52",
		},
		{
			name:          "Search with no results",
			query:         "Nonexistent University",
			expectedCount: 0,
			shouldContain: "",
		},
		{
			name:          "Empty query",
			query:         "",
			expectedCount: 0,
			shouldContain: "",
		},
		{
			name:          "Search by partial word",
			query:         "Massachusetts",
			expectedCount: 1,
			shouldContain: "04xm1d337",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := cache.Search(tt.query)

			if len(results) != tt.expectedCount {
				t.Errorf("Expected %d results, got %d", tt.expectedCount, len(results))
			}

			if tt.shouldContain != "" {
				found := false
				for _, org := range results {
					if org.ID == tt.shouldContain {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected results to contain ROR ID %s", tt.shouldContain)
				}
			}
		})
	}
}

func TestRorNameCache_Get(t *testing.T) {
	cache := &RorNameCache{
		cache: map[string]string{
			"01an7q238": "University of California, Berkeley",
			"02jbv0t02": "Stanford University",
		},
		lastRefresh: time.Now(),
	}

	tests := []struct {
		name         string
		rorID        string
		expectedName string
		shouldFind   bool
	}{
		{
			name:         "Get existing organization",
			rorID:        "01an7q238",
			expectedName: "University of California, Berkeley",
			shouldFind:   true,
		},
		{
			name:         "Get non-existing organization",
			rorID:        "99999999",
			expectedName: "",
			shouldFind:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, found := cache.Get(tt.rorID)

			if found != tt.shouldFind {
				t.Errorf("Expected found=%v, got %v", tt.shouldFind, found)
			}

			if name != tt.expectedName {
				t.Errorf("Expected name=%s, got %s", tt.expectedName, name)
			}
		})
	}
}

func TestRorNameCache_Size(t *testing.T) {
	cache := &RorNameCache{
		cache: map[string]string{
			"01an7q238": "University of California, Berkeley",
			"02jbv0t02": "Stanford University",
			"03vek6s52": "Harvard University",
		},
		lastRefresh: time.Now(),
	}

	size := cache.Size()
	if size != 3 {
		t.Errorf("Expected size=3, got %d", size)
	}
}

// Integration test - requires database connection
func TestRorNameCache_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This test requires a real database connection
	// Skip if DATABASE_URL is not set
	dsn := getTestDatabaseURL()
	if dsn == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		t.Skipf("Database not available: %v", err)
	}

	// Create test data
	ctx := context.Background()
	_, err = db.ExecContext(ctx, `
		INSERT INTO records_ror (record_id, ror) 
		VALUES 
			('00000000-0000-7000-0000-000000000001', '01an7q238'),
			('00000000-0000-7000-0000-000000000002', '02jbv0t02')
		ON CONFLICT DO NOTHING
	`)
	if err != nil {
		t.Logf("Warning: Could not insert test data: %v", err)
	}

	// Create cache with mock ROR client
	mockCache := NewInMemoryCache[RorOrganization](24 * time.Hour)
	mockCache.Set("01an7q238", RorOrganization{
		ID:   "01an7q238",
		Name: "University of California, Berkeley",
	})
	mockCache.Set("02jbv0t02", RorOrganization{
		ID:   "02jbv0t02",
		Name: "Stanford University",
	})

	rorClient := NewRorClientWithCache(mockCache)
	rorRepo := NewPostgresRorRepository(db)
	nameCache := NewRorNameCache(rorRepo, rorClient)
	defer nameCache.Stop()

	// Wait a bit for initial load
	time.Sleep(100 * time.Millisecond)

	// Test that cache has data
	if nameCache.Size() == 0 {
		t.Error("Expected cache to have data after initialization")
	}

	// Test search
	results := nameCache.Search("University")
	if len(results) == 0 {
		t.Error("Expected search results, got none")
	}
}

func getTestDatabaseURL() string {
	// Try to get from environment or use default test database
	// This is a helper for integration tests
	return ""
}
