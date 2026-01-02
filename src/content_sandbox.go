package main

import (
	"github.com/microcosm-cc/bluemonday"
)

// HTMLSanitizer provides server-side HTML sanitization using bluemonday
// This can be used during upload to sanitize HTML content before storage
type HTMLSanitizer struct {
	policy *bluemonday.Policy
}

// NewHTMLSanitizer creates a new sanitizer with a strict policy
func NewHTMLSanitizer() *HTMLSanitizer {
	p := bluemonday.NewPolicy()

	// Allow safe structural elements
	p.AllowElements("div", "span", "p", "br", "hr")
	p.AllowElements("h1", "h2", "h3", "h4", "h5", "h6")
	p.AllowElements("ul", "ol", "li", "dl", "dt", "dd")
	p.AllowElements("table", "thead", "tbody", "tfoot", "tr", "th", "td", "caption")
	p.AllowElements("blockquote", "pre", "code", "em", "strong", "i", "b", "u", "s")
	p.AllowElements("sub", "sup", "small", "mark", "abbr", "cite", "q")
	p.AllowElements("figure", "figcaption", "details", "summary")

	// Allow images with safe src (http, https, and safe data URIs)
	p.AllowImages()
	p.AllowDataURIImages()

	// Allow links with safe protocols only
	p.AllowAttrs("href").OnElements("a")
	p.AllowURLSchemes("http", "https", "mailto")
	p.AllowRelativeURLs(true)
	p.RequireNoReferrerOnLinks(true)
	p.AddTargetBlankToFullyQualifiedLinks(true)

	// Allow common safe attributes
	p.AllowAttrs("id", "class", "title", "lang", "dir").Globally()
	p.AllowAttrs("colspan", "rowspan", "headers", "scope").OnElements("th", "td")
	p.AllowAttrs("alt", "width", "height").OnElements("img")

	return &HTMLSanitizer{policy: p}
}

// Sanitize cleans HTML content by removing dangerous elements and attributes
func (s *HTMLSanitizer) Sanitize(html string) string {
	return s.policy.Sanitize(html)
}

// SanitizeRoCrateMetadata processes RO-Crate metadata and sanitizes any HTML content
// Returns the modified metadata with sanitized HTML
func (s *HTMLSanitizer) SanitizeRoCrateMetadata(metadata map[string]interface{}) map[string]interface{} {
	// Get the @graph array
	graph, ok := metadata["@graph"].([]interface{})
	if !ok {
		return metadata
	}

	// Process each entity in the graph
	for i, item := range graph {
		entity, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		// Check if this entity has HTML content
		encodingFormat, _ := entity["encodingFormat"].(string)
		if encodingFormat != "text/html" {
			continue
		}

		// Sanitize HTML content in text, description, or content fields
		for _, key := range []string{"text", "description", "content"} {
			if val, ok := entity[key].(string); ok && val != "" {
				entity[key] = s.Sanitize(val)
			}
		}

		graph[i] = entity
	}

	metadata["@graph"] = graph
	return metadata
}
