package api

import (
	"github.com/microcosm-cc/bluemonday"
)

// sanitizer is a shared HTML sanitizer configured for user-generated content
var sanitizer = bluemonday.UGCPolicy()

// SanitizePostHTML removes potentially dangerous HTML from post content while
// preserving safe formatting like links, bold, italic, lists, images, etc.
// This prevents XSS attacks by stripping scripts, event handlers, and other
// malicious markup before storing posts in the database.
func SanitizePostHTML(html string) string {
	if html == "" {
		return ""
	}
	return sanitizer.Sanitize(html)
}
