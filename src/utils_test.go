package main

import (
	"strings"
	"testing"
)

func TestValidateAndNormalizeRorId(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		valid    bool
	}{
		// Valid cases real ROR https://ror.org/024mw5h28
		{"", "", true},                       // empty is valid
		{"024mw5h28", "024mw5h28", true},     // just ID
		{"  024mw5h28  ", "024mw5h28", true}, // with whitespace

		// Invalid cases
		{"abcdef12", "abcdef12", false},     // missing leading 0
		{"0abcdef1", "0abcdef1", false},     // too short
		{"0abcdef123", "0abcdef123", false}, // too long
		{"0ABCDEF12", "0ABCDEF12", false},   // uppercase not allowed
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

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Protocol for X", "Protocol for X"},
		{"Protocol/for\\X", "Protocol_for_X"},
		{"Protocol<for>X", "Protocol_for_X"},
		{"Protocol:for|X", "Protocol_for_X"},
		{"Protocol\"for*X", "Protocol_for_X"},
		{"Protocol?for X", "Protocol_for X"},
		{"", "unnamed"},
		{"   ", "unnamed"},
		{"...", "unnamed"},
		{" Protocol for X ", "Protocol for X"},
		{" Protocol for X. ", "Protocol for X"},
		// Test very long filename (300 'a' characters should be truncated to 250)
		{strings.Repeat("a", 300), strings.Repeat("a", 250)},
	}

	for _, test := range tests {
		result := sanitizeFilename(test.input)
		if result != test.expected {
			t.Errorf("sanitizeFilename(%q) = %q, expected %q", test.input, result, test.expected)
		}
	}
}
