package api

import (
	"strings"
	"testing"
)

func TestLinkifyURLs(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "http URL in text",
			input:    "Check out http://example.com for more",
			expected: `Check out <a href="http://example.com">http://example.com</a> for more`,
		},
		{
			name:     "https URL in text",
			input:    "Visit https://secure.example.com today",
			expected: `Visit <a href="https://secure.example.com">https://secure.example.com</a> today`,
		},
		{
			name:     "www URL",
			input:    "Go to www.example.com now",
			expected: `Go to <a href="http://www.example.com">www.example.com</a> now`,
		},
		{
			name:     "multiple URLs",
			input:    "See http://one.com and https://two.com both",
			expected: `See <a href="http://one.com">http://one.com</a> and <a href="https://two.com">https://two.com</a> both`,
		},
		{
			name:     "URL with trailing punctuation",
			input:    "Visit http://example.com.",
			expected: `Visit <a href="http://example.com">http://example.com</a>.`,
		},
		{
			name:     "URL in parentheses",
			input:    "(see http://example.com)",
			expected: `(see <a href="http://example.com">http://example.com</a>)`,
		},
		{
			name:     "don't linkify URLs already in href",
			input:    `<a href="http://example.com">example</a>`,
			expected: `<a href="http://example.com">example</a>`,
		},
		{
			name:     "don't linkify URLs in img src",
			input:    `<img src="http://example.com/image.jpg" alt="test">`,
			expected: `<img src="http://example.com/image.jpg" alt="test"/>`, // HTML parser renders self-closing tags with /
		},
		{
			name:     "linkify text before existing link",
			input:    `Text with http://example.com <a href="http://other.com">link</a>`,
			expected: `Text with <a href="http://example.com">http://example.com</a> <a href="http://other.com">link</a>`,
		},
		{
			name:     "empty input",
			input:    "",
			expected: "",
		},
		{
			name:     "no URLs",
			input:    "Just plain text",
			expected: "Just plain text",
		},
		{
			name:     "skip .md files",
			input:    "See http://example.com/readme.md for docs",
			expected: `See http://example.com/readme.md for docs`,
		},
		{
			name:     "skip .zip files",
			input:    "Download http://example.com/archive.zip here",
			expected: `Download http://example.com/archive.zip here`,
		},
		{
			name:     "skip .MD uppercase",
			input:    "Read http://example.com/README.MD carefully",
			expected: `Read http://example.com/README.MD carefully`,
		},
		{
			name:     "linkify regular URLs alongside skipped files",
			input:    "Visit http://example.com and download http://files.com/data.zip",
			expected: `Visit <a href="http://example.com">http://example.com</a> and download http://files.com/data.zip`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := LinkifyURLs(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Normalize whitespace for comparison
			result = strings.TrimSpace(result)
			expected := strings.TrimSpace(tc.expected)

			if result != expected {
				t.Errorf("mismatch\nExpected: %s\nGot:      %s", expected, result)
			}
		})
	}
}

func TestLinkifyURLsPreservesFormatting(t *testing.T) {
	// Test that we don't linkify inside <pre> or <code> tags
	input := `<pre>Visit http://example.com</pre>`
	result, err := LinkifyURLs(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should NOT contain a linkified URL inside pre
	if strings.Contains(result, `<a href="http://example.com">`) {
		t.Errorf("should not linkify URLs in <pre> tags")
	}
}

func TestLinkifyURLsEdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "URL with path and query",
			input: "Check http://example.com/path?query=value&other=123",
		},
		{
			name:  "URL with fragment",
			input: "See http://example.com/page#section",
		},
		{
			name:  "multiple URLs in same sentence",
			input: "http://a.com http://b.com http://c.com",
		},
		{
			name:  "URL with hyphen in domain",
			input: "Visit http://my-domain.com today",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := LinkifyURLs(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Just verify it doesn't crash and produces valid HTML
			if result == "" {
				t.Errorf("expected non-empty result")
			}

			// Count that we have closing tags for all opening tags
			openCount := strings.Count(result, "<a href=")
			closeCount := strings.Count(result, "</a>")
			if openCount != closeCount {
				t.Errorf("unbalanced <a> tags: %d open, %d close", openCount, closeCount)
			}
		})
	}
}
