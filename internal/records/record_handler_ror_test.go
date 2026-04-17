package records

import (
	"testing"
)

// Test the logic for determining if input is ROR ID or organization name
func TestRorInputProcessing(t *testing.T) {
	// Create a mock name cache with French and Vietnamese institutions
	mockCache := &RorNameCache{
		cache: map[string]string{
			"02feahw73": "Centre National de la Recherche Scientifique",
			"051escj72": "Université Paris Cité",
			"03rnk6m14": "Vietnam National University, Hanoi",
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
			input:          "02feahw73",
			wantRorID:      "02feahw73",
			wantOrgName:    "",
			wantEmptyMatch: false,
		},
		{
			name:           "Organization name - exact match (French)",
			input:          "Université Paris Cité",
			wantRorID:      "051escj72",
			wantOrgName:    "Université Paris Cité",
			wantEmptyMatch: false,
		},
		{
			name:           "Organization name - partial match (Vietnamese)",
			input:          "Vietnam National",
			wantRorID:      "03rnk6m14",
			wantOrgName:    "Vietnam National University, Hanoi",
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
	// Create a mock name cache with French and Vietnamese institutions
	// Both contain "de" in their names
	mockCache := &RorNameCache{
		cache: map[string]string{
			"02feahw73": "Centre National de la Recherche Scientifique",
			"051escj72": "Université Paris Cité",
			"03rnk6m14": "Vietnam National University, Hanoi",
			"05qghxh33": "Vietnam National University, Ho Chi Minh City",
			"01ggx4157": "École Polytechnique",
		},
	}

	// Search for "de" should match French institutions with "de"
	matchingOrgs := mockCache.Search("de")

	if len(matchingOrgs) != 1 {
		t.Errorf("Expected has 1 match for 'de', got %d", len(matchingOrgs))
	}

	// Verify that we get multiple ROR IDs
	rorIDs := make([]string, len(matchingOrgs))
	for i, org := range matchingOrgs {
		rorIDs[i] = org.ID
	}

	t.Logf("Found %d organizations matching 'de': %v", len(matchingOrgs), rorIDs)

	// Verify expected IDs are in the results
	foundCNRS := false
	for _, org := range matchingOrgs {
		if org.ID == "02feahw73" {
			foundCNRS = true
		}
	}

	if !foundCNRS {
		t.Error("Expected to find Centre National de la Recherche Scientifique in results")
	}

	// Test Vietnamese institutions - search for "Vietnam"
	vietnamOrgs := mockCache.Search("Vietnam")
	if len(vietnamOrgs) != 2 {
		t.Errorf("Expected 2 matches for 'Vietnam', got %d", len(vietnamOrgs))
	}

	foundHanoi := false
	foundHCMC := false
	for _, org := range vietnamOrgs {
		if org.ID == "03rnk6m14" {
			foundHanoi = true
		}
		if org.ID == "05qghxh33" {
			foundHCMC = true
		}
	}

	if !foundHanoi {
		t.Error("Expected to find Vietnam National University, Hanoi in results")
	}
	if !foundHCMC {
		t.Error("Expected to find Vietnam National University, Ho Chi Minh City in results")
	}
}
