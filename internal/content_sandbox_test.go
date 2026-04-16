package app

import (
	"strings"
	"testing"
)

func TestSanitizeEncodingFormat(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "valid text/html",
			input:    "text/html",
			expected: "text/html",
		},
		{
			name:     "text/html with charset",
			input:    "text/html; charset=utf-8",
			expected: "text/html",
		},
		{
			name:     "uppercase normalization",
			input:    "TEXT/HTML",
			expected: "text/html",
		},
		{
			name:     "mixed case normalization",
			input:    "Text/Html",
			expected: "text/html",
		},
		{
			name:     "valid text/plain",
			input:    "text/plain",
			expected: "text/plain",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "invalid mime type",
			input:    "not-a-mime-type",
			expected: "",
		},
		{
			name:     "injection attempt with semicolon",
			input:    "text/html; <script>alert(1)</script>",
			expected: "",
		},
		{
			name:     "whitespace padding",
			input:    "  text/html  ",
			expected: "text/html",
		},
		{
			name:     "application/json",
			input:    "application/json",
			expected: "application/json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeEncodingFormat(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeEncodingFormat(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestHTMLSanitizer_Sanitize(t *testing.T) {
	sanitizer := NewHTMLSanitizer()

	tests := []struct {
		name        string
		input       string
		contains    []string
		notContains []string
	}{
		{
			name:        "removes script tags",
			input:       "<p>Hello</p><script>alert('xss')</script>",
			contains:    []string{"<p>Hello</p>"},
			notContains: []string{"<script>", "alert"},
		},
		{
			name:        "removes event handlers",
			input:       `<div onclick="alert('xss')">Click me</div>`,
			contains:    []string{"<div>", "Click me"},
			notContains: []string{"onclick", "alert"},
		},
		{
			name:        "removes style tags and attributes",
			input:       `<style>body{background:red}</style><p style="color:red">Text</p>`,
			contains:    []string{"<p>", "Text"},
			notContains: []string{"<style>", "style=", "background", "color:red"},
		},
		{
			name:        "preserves safe HTML",
			input:       `<h1>Title</h1><p>Paragraph with <strong>bold</strong> and <em>italic</em></p>`,
			contains:    []string{"<h1>Title</h1>", "<p>", "<strong>bold</strong>", "<em>italic</em>"},
			notContains: []string{},
		},
		{
			name:        "allows safe links",
			input:       `<a href="https://example.com">Link</a>`,
			contains:    []string{"<a", "href=", "https://example.com", "Link", "</a>"},
			notContains: []string{},
		},
		{
			name:        "removes javascript: URLs",
			input:       `<a href="javascript:alert('xss')">Bad Link</a>`,
			contains:    []string{"Bad Link"},
			notContains: []string{"javascript:", "alert"},
		},
		{
			name:        "removes iframe tags",
			input:       `<p>Text</p><iframe src="evil.com"></iframe>`,
			contains:    []string{"<p>Text</p>"},
			notContains: []string{"<iframe>", "evil.com"},
		},
		{
			name:        "removes form elements",
			input:       `<form action="/steal"><input type="text"><button>Submit</button></form>`,
			contains:    []string{"Submit"}, // Text content is preserved
			notContains: []string{"<form", "<input", "<button", "action="},
		},
		{
			name:        "preserves tables",
			input:       `<table><tr><th>Header</th></tr><tr><td>Data</td></tr></table>`,
			contains:    []string{"<table>", "<tr>", "<th>Header</th>", "<td>Data</td>", "</table>"},
			notContains: []string{},
		},
		{
			name:        "preserves lists",
			input:       `<ul><li>Item 1</li><li>Item 2</li></ul>`,
			contains:    []string{"<ul>", "<li>Item 1</li>", "<li>Item 2</li>", "</ul>"},
			notContains: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizer.Sanitize(tt.input)

			for _, expected := range tt.contains {
				if !strings.Contains(result, expected) {
					t.Errorf("Expected result to contain %q, but it didn't. Result: %s", expected, result)
				}
			}

			for _, notExpected := range tt.notContains {
				if strings.Contains(result, notExpected) {
					t.Errorf("Expected result NOT to contain %q, but it did. Result: %s", notExpected, result)
				}
			}
		})
	}
}

func TestHTMLSanitizer_SanitizeRoCrateMetadata(t *testing.T) {
	sanitizer := NewHTMLSanitizer()

	metadata := map[string]interface{}{
		"@context": "https://w3id.org/ro/crate/1.1/context",
		"@graph": []interface{}{
			map[string]interface{}{
				"@id":   "./",
				"@type": "Dataset",
				"name":  "Test Dataset",
			},
			map[string]interface{}{
				"@id":            "file1.html",
				"@type":          "File",
				"encodingFormat": "text/html",
				"text":           "<p>Safe content</p><script>alert('xss')</script>",
			},
			map[string]interface{}{
				"@id":            "file2.txt",
				"@type":          "File",
				"encodingFormat": "text/plain",
				"text":           "This is plain text with <script>tags</script>",
			},
		},
	}

	result := sanitizer.SanitizeRoCrateMetadata(metadata)

	// Get the graph
	graph, ok := result["@graph"].([]interface{})
	if !ok {
		t.Fatal("Expected @graph to be an array")
	}

	// Check file1.html was sanitized
	file1 := graph[1].(map[string]interface{})
	text1 := file1["text"].(string)

	if !strings.Contains(text1, "<p>Safe content</p>") {
		t.Errorf("Expected safe content to be preserved, got: %s", text1)
	}

	if strings.Contains(text1, "<script>") || strings.Contains(text1, "alert") {
		t.Errorf("Expected script to be removed, got: %s", text1)
	}

	// Check file2.txt was NOT sanitized (not HTML)
	file2 := graph[2].(map[string]interface{})
	text2 := file2["text"].(string)

	if text2 != "This is plain text with <script>tags</script>" {
		t.Errorf("Expected plain text to be unchanged, got: %s", text2)
	}
}
