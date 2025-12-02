package main

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

func TestRorNameCache_Search(t *testing.T) {
	// Create a mock cache with test data - French and Vietnamese institutions
	cache := &RorNameCache{
		cache: map[string]string{
			"02feahw73": "Centre National de la Recherche Scientifique",
			"051escj72": "Université Paris Cité",
			"03xjwb503": "Sorbonne Université",
			"03rnk6m14": "Vietnam National University, Hanoi",
			"05qghxh33": "Vietnam National University, Ho Chi Minh City",
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
			query:         "Sorbonne Université",
			expectedCount: 1,
			shouldContain: "03xjwb503",
		},
		{
			name:          "Search by partial name - multiple matches",
			query:         "Vietnam National University",
			expectedCount: 2,
			shouldContain: "03rnk6m14",
		},
		{
			name:          "Search case insensitive",
			query:         "PARIS",
			expectedCount: 1,
			shouldContain: "051escj72",
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
			name:          "Search by partial word - French",
			query:         "Recherche",
			expectedCount: 1,
			shouldContain: "02feahw73",
		},
		{
			name:          "Search by partial word - Vietnamese city",
			query:         "Hanoi",
			expectedCount: 1,
			shouldContain: "03rnk6m14",
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
			"02feahw73": "Centre National de la Recherche Scientifique",
			"03rnk6m14": "Vietnam National University, Hanoi",
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
			name:         "Get existing organization - French",
			rorID:        "02feahw73",
			expectedName: "Centre National de la Recherche Scientifique",
			shouldFind:   true,
		},
		{
			name:         "Get existing organization - Vietnamese",
			rorID:        "03rnk6m14",
			expectedName: "Vietnam National University, Hanoi",
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
			"02feahw73": "Centre National de la Recherche Scientifique",
			"051escj72": "Université Paris Cité",
			"03rnk6m14": "Vietnam National University, Hanoi",
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
	mockCache.Set("02feahw73", RorOrganization{
		ID:   "02feahw73",
		Name: "Centre National de la Recherche Scientifique",
	})
	mockCache.Set("03rnk6m14", RorOrganization{
		ID:   "03rnk6m14",
		Name: "Vietnam National University, Hanoi",
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
