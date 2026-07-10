package main

import (
	"strings"
)

// sanitizeCommentContent strips HTML and returns plain text
func sanitizeCommentContent(content string) string {
	// HTML escape to prevent any HTML rendering
//	content = html.EscapeString(content)
	// Trim whitespace
	content = strings.TrimSpace(content)
	return content
}
