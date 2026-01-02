package main

import (
	"fmt"
	"html/template"

	"github.com/microcosm-cc/bluemonday"
)

// ContentSandbox renders user HTML in a sandboxed iframe
type ContentSandbox struct {
	policy *bluemonday.Policy
}

// NewContentSandbox creates a new content sandbox with a strict sanitization policy
func NewContentSandbox() *ContentSandbox {
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

	return &ContentSandbox{policy: p}
}

// Sanitize cleans HTML content
func (s *ContentSandbox) Sanitize(html string) string {
	return s.policy.Sanitize(html)
}

// ProcessRoCrateMetadata processes RO-Crate metadata and returns a map of entity IDs to sandboxed HTML
func (s *ContentSandbox) ProcessRoCrateMetadata(metadata map[string]interface{}) map[string]template.HTML {
	result := make(map[string]template.HTML)

	// Get the @graph array
	graph, ok := metadata["@graph"].([]interface{})
	if !ok {
		return result
	}

	// Process each entity in the graph
	for _, item := range graph {
		entity, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		// Get entity ID
		entityID, _ := entity["@id"].(string)
		if entityID == "" {
			continue
		}

		// Check if this entity has HTML content
		encodingFormat, _ := entity["encodingFormat"].(string)
		if encodingFormat != "text/html" {
			continue
		}

		// Look for HTML content in text, description, or content fields
		var htmlContent string
		for _, key := range []string{"text", "description", "content"} {
			if val, ok := entity[key].(string); ok && val != "" {
				htmlContent = val
				break
			}
		}

		if htmlContent != "" {
			result[entityID] = s.RenderSandboxedHTML(htmlContent)
		}
	}

	return result
}

// RenderSandboxedHTML returns HTML for a sandboxed iframe containing user content
// The iframe uses srcdoc to inject content without a separate request
func (s *ContentSandbox) RenderSandboxedHTML(userHTML string) template.HTML {
	// First sanitize the HTML
	sanitizedHTML := s.Sanitize(userHTML)

	// Wrap with minimal safe styles for readability
	styledHTML := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            font-size: 14px;
            line-height: 1.5;
            color: #333;
            padding: 16px;
            margin: 0;
        }
        table { border-collapse: collapse; width: 100%%; }
        th, td { border: 1px solid #ddd; padding: 8px; text-align: left; }
        th { background-color: #f5f5f5; }
        img { max-width: 100%%; height: auto; }
        a { color: #0066cc; text-decoration: none; }
        a:hover { text-decoration: underline; }
        pre, code { background: #f5f5f5; padding: 2px 4px; border-radius: 3px; font-family: monospace; }
        pre { padding: 12px; overflow-x: auto; }
        blockquote { border-left: 3px solid #ddd; margin-left: 0; padding-left: 16px; color: #666; }
    </style>
</head>
<body>%s</body>
</html>`, sanitizedHTML)

	// Escape for srcdoc attribute (needs HTML entity encoding)
	escapedHTML := template.HTMLEscapeString(styledHTML)

	// Build the sandboxed iframe
	// sandbox="" means NO permissions at all - most restrictive
	// We explicitly do NOT add:
	// - allow-scripts (no JavaScript execution)
	// - allow-same-origin (prevents access to parent page)
	// - allow-top-navigation (prevents redirecting parent)
	// - allow-forms (prevents form submission)
	// - allow-popups (prevents opening new windows)
	iframeHTML := fmt.Sprintf(`
<div class="user-content-container" style="contain: strict; overflow: auto; max-width: 100%%; max-height: 600px; border: 1px solid #ddd; border-radius: 4px; background: white; margin-top: 8px;">
    <iframe 
        sandbox=""
        srcdoc="%s"
        style="width: 100%%; height: 100%%; min-height: 400px; border: none; display: block;"
        title="User-submitted content"
        loading="lazy"
    ></iframe>
</div>`, escapedHTML)

	return template.HTML(iframeHTML)
}
