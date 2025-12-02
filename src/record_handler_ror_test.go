package main

import (
	"testing"
)

// Test the logic for determining if input is ROR ID or organization name
func TestRorInputProcessing(t *testing.T) {
	// Create a mock name cache
	mockCache := &RorNameCache{
		cache: map[string]string{
			"01an7q238": "University of California, Berkeley",
			"02jbv0t02": "Stanford University",
			"03vek6s52": "Harvard University",
		},
	}

	tests := []struct {
		name           string
		input          string
		wantRorID      string
		wantOrgName    string
		wantEmptyMatch bool
	}{
		{
			name:           "Valid ROR ID",
			input:          "01an7q238",
			wantRorID:      "01an7q238",
			wantOrgName:    "",
			wantEmptyMatch: false,
		},
		{
			name:           "Organization name - exact match",
			input:          "Stanford University",
			wantRorID:      "02jbv0t02",
			wantOrgName:    "Stanford University",
			wantEmptyMatch: false,
		},
		{
			name:           "Organization name - partial match",
			input:          "Harvard",
			wantRorID:      "03vek6s52",
			wantOrgName:    "Harvard University",
			wantEmptyMatch: false,
		},
		{
			name:           "No match found",
			input:          "Nonexistent University",
			wantRorID:      "",
			wantOrgName:    "",
			wantEmptyMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the logic from GetBrowsePage
			var rorID string
			var rorOrgName string
			var emptyMatch bool

			normalizedRorId, isValid := validateAndNormalizeRorId(tt.input)
			if isValid && normalizedRorId != "" {
				// It's a valid ROR ID
				rorID = normalizedRorId
			} else {
				// It's not a valid ROR ID, treat it as organization name search
				matchingOrgs := mockCache.Search(tt.input)
				if len(matchingOrgs) > 0 {
					// Use the first matching organization's ROR ID
					rorID = matchingOrgs[0].ID
					rorOrgName = matchingOrgs[0].Name
				} else {
					// No matching organizations found
					emptyMatch = true
				}
			}

			if rorID != tt.wantRorID {
				t.Errorf("rorID = %v, want %v", rorID, tt.wantRorID)
			}
			if rorOrgName != tt.wantOrgName {
				t.Errorf("rorOrgName = %v, want %v", rorOrgName, tt.wantOrgName)
			}
			if emptyMatch != tt.wantEmptyMatch {
				t.Errorf("emptyMatch = %v, want %v", emptyMatch, tt.wantEmptyMatch)
			}
		})
	}
}

// Test multiple matching organizations
func TestRorInputProcessing_MultipleMatches(t *testing.T) {
	// Create a mock name cache with organizations containing "de"
	mockCache := &RorNameCache{
		cache: map[string]string{
			"01an7q238": "University of California, Berkeley",
			"02jbv0t02": "Stanford University",
			"03vek6s52": "Harvard University",
			"04abc1234": "Technical University of Denmark",
			"05def5678": "University of Delaware",
		},
	}

	// Search for "de" should match both Denmark and Delaware
	matchingOrgs := mockCache.Search("de")

	if len(matchingOrgs) < 2 {
		t.Errorf("Expected at least 2 matches for 'de', got %d", len(matchingOrgs))
	}

	// Verify that we get multiple ROR IDs
	rorIDs := make([]string, len(matchingOrgs))
	for i, org := range matchingOrgs {
		rorIDs[i] = org.ID
	}

	t.Logf("Found %d organizations matching 'de': %v", len(matchingOrgs), rorIDs)

	// Verify both expected IDs are in the results
	foundDenmark := false
	foundDelaware := false
	for _, org := range matchingOrgs {
		if org.ID == "04abc1234" {
			foundDenmark = true
		}
		if org.ID == "05def5678" {
			foundDelaware = true
		}
	}

	if !foundDenmark {
		t.Error("Expected to find Technical University of Denmark in results")
	}
	if !foundDelaware {
		t.Error("Expected to find University of Delaware in results")
	}
}
