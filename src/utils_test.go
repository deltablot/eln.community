package main

import "testing"

func TestValidateAndNormalizeRorId(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		valid    bool
	}{
		// Valid cases
		{"", "", true}, // empty is valid
		{"0abcdef12", "https://ror.org/0abcdef12", true},                 // just ID
		{"https://ror.org/0abcdef12", "https://ror.org/0abcdef12", true}, // full URL
		{"ror.org/0abcdef12", "https://ror.org/0abcdef12", true},         // without https
		{"  0abcdef12  ", "https://ror.org/0abcdef12", true},             // with whitespace

		// Invalid cases
		{"abcdef12", "", false},                      // missing leading 0
		{"0abcdef1", "", false},                      // too short
		{"0abcdef123", "", false},                    // too long
		{"0ABCDEF12", "", false},                     // uppercase not allowed
		{"0abcdefg2", "", false},                     // invalid character
		{"https://ror.org/abcdef12", "", false},      // invalid ID in URL
		{"https://example.com/0abcdef12", "", false}, // wrong domain
	}

	for _, test := range tests {
		result, valid := validateAndNormalizeRorId(test.input)
		if valid != test.valid {
			t.Errorf("validateAndNormalizeRorId(%q) validity = %v, want %v", test.input, valid, test.valid)
		}
		if result != test.expected {
			t.Errorf("validateAndNormalizeRorId(%q) result = %q, want %q", test.input, result, test.expected)
		}
	}
}
