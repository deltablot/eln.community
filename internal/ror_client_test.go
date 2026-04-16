package app

import (
	"testing"
	"time"
)

func TestRorClient_GetOrganization_Cache(t *testing.T) {
	client := NewRorClient()
	defer client.cache.Stop()

	// First call should fetch from API (we can't test actual API without mocking)
	// But we can test the cache behavior

	// Manually add to cache
	testOrg := RorOrganization{
		ID:    "042nb2s44",
		Name:  "Massachusetts Institute of Technology",
		Types: []string{"education"},
	}
	client.cache.Set("042nb2s44", testOrg)

	// Second call should hit cache
	org, err := client.GetOrganization("042nb2s44")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if org.ID != "042nb2s44" {
		t.Errorf("Expected ID 042nb2s44, got %s", org.ID)
	}

	if org.Name != "Massachusetts Institute of Technology" {
		t.Errorf("Expected MIT, got %s", org.Name)
	}
}

func TestRorClient_InvalidRorID(t *testing.T) {
	client := NewRorClient()
	defer client.cache.Stop()

	// Test with invalid ROR ID
	_, err := client.GetOrganization("invalid-id")
	if err == nil {
		t.Error("Expected error for invalid ROR ID, got nil")
	}

	if !contains(err.Error(), "invalid") {
		t.Errorf("Expected error message to contain 'invalid', got: %s", err.Error())
	}
}

func TestExtractRorID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"https://ror.org/042nb2s44", "042nb2s44"},
		{"042nb2s44", "042nb2s44"},
		{"https://ror.org/013meh722", "013meh722"},
		{"", ""},
	}

	for _, tt := range tests {
		result := extractRorID(tt.input)
		if result != tt.expected {
			t.Errorf("extractRorID(%s) = %s, expected %s", tt.input, result, tt.expected)
		}
	}
}

func TestGetDisplayName(t *testing.T) {
	// Test with ror_display type
	names1 := []RorName{
		{Lang: nil, Types: []string{"label"}, Value: "Label Name"},
		{Lang: nil, Types: []string{"ror_display"}, Value: "Display Name"},
	}
	result1 := getDisplayName(names1)
	if result1 != "Display Name" {
		t.Errorf("Expected 'Display Name', got '%s'", result1)
	}

	// Test with only label
	names2 := []RorName{
		{Lang: nil, Types: []string{"label"}, Value: "Label Name"},
	}
	result2 := getDisplayName(names2)
	if result2 != "Label Name" {
		t.Errorf("Expected 'Label Name', got '%s'", result2)
	}

	// Test with only first name
	names3 := []RorName{
		{Lang: nil, Types: []string{"other"}, Value: "First Name"},
	}
	result3 := getDisplayName(names3)
	if result3 != "First Name" {
		t.Errorf("Expected 'First Name', got '%s'", result3)
	}
}

func TestRorClient_CustomCache(t *testing.T) {
	// Create a custom cache with short TTL
	customCache := NewInMemoryCache[RorOrganization](100 * time.Millisecond)
	defer customCache.Stop()

	client := NewRorClientWithCache(customCache)

	// Add organization to cache
	testOrg := RorOrganization{
		ID:    "042nb2s44",
		Name:  "MIT",
		Types: []string{"education"},
	}
	client.cache.Set("042nb2s44", testOrg)

	// Should be in cache immediately
	if !client.cache.Has("042nb2s44") {
		t.Error("Expected organization to be in cache")
	}

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Should be expired
	if client.cache.Has("042nb2s44") {
		t.Error("Expected organization to be expired")
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
