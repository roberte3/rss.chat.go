package api

import (
	"golang.org/x/net/html"
	"regexp"
	"testing"
)

func TestTransformTextNodesEmpty(t *testing.T) {
	result, err := TransformTextNodes("", []Matcher{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestTransformTextNodesWhitespaceOnly(t *testing.T) {
	result, err := TransformTextNodes("   ", []Matcher{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestTransformTextNodesNoMatches(t *testing.T) {
	input := "<p>hello world</p>"
	matcher := Matcher{
		Pattern: regexp.MustCompile(`@\w+`),
		Replace: func(text string, start, end int) (*html.Node, string) {
			return nil, ""
		},
	}

	result, err := TransformTextNodes(input, []Matcher{matcher})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != input {
		t.Errorf("expected %q, got %q", input, result)
	}
}

func TestTransformTextNodesSingleMatcher(t *testing.T) {
	input := "<p>hello @bob world</p>"
	matcher := Matcher{
		Pattern: regexp.MustCompile(`@(\w+)`),
		Replace: func(text string, start, end int) (*html.Node, string) {
			match := text[start:end]
			link := &html.Node{
				Type: html.ElementNode,
				Data: "strong",
			}
			link.AppendChild(&html.Node{
				Type: html.TextNode,
				Data: match,
			})
			return link, ""
		},
	}

	result, err := TransformTextNodes(input, []Matcher{matcher})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check that @bob is wrapped in <strong>
	if !containsSubstring(result, "<strong>@bob</strong>") {
		t.Errorf("expected <strong>@bob</strong> in result, got %q", result)
	}
}

func TestTransformTextNodesSkipsCode(t *testing.T) {
	input := "<p>hello @bob</p><code>@alice</code>"
	matcher := Matcher{
		Pattern: regexp.MustCompile(`@(\w+)`),
		Replace: func(text string, start, end int) (*html.Node, string) {
			match := text[start:end]
			link := &html.Node{
				Type: html.ElementNode,
				Data: "strong",
			}
			link.AppendChild(&html.Node{
				Type: html.TextNode,
				Data: match,
			})
			return link, ""
		},
	}

	result, err := TransformTextNodes(input, []Matcher{matcher})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// @bob should be transformed
	if !containsSubstring(result, "<strong>@bob</strong>") {
		t.Errorf("expected <strong>@bob</strong> in result, got %q", result)
	}

	// @alice in code should NOT be transformed
	if containsSubstring(result, "<strong>@alice</strong>") {
		t.Errorf("expected @alice to remain plain in code, got %q", result)
	}
	if !containsSubstring(result, "<code>@alice</code>") {
		t.Errorf("expected <code>@alice</code> unchanged, got %q", result)
	}
}

func TestTransformTextNodesSkipsPre(t *testing.T) {
	input := "<p>hello @bob</p><pre>@alice</pre>"
	matcher := Matcher{
		Pattern: regexp.MustCompile(`@(\w+)`),
		Replace: func(text string, start, end int) (*html.Node, string) {
			match := text[start:end]
			link := &html.Node{
				Type: html.ElementNode,
				Data: "em",
			}
			link.AppendChild(&html.Node{
				Type: html.TextNode,
				Data: match,
			})
			return link, ""
		},
	}

	result, err := TransformTextNodes(input, []Matcher{matcher})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// @bob should be transformed
	if !containsSubstring(result, "<em>@bob</em>") {
		t.Errorf("expected <em>@bob</em> in result, got %q", result)
	}

	// @alice in pre should NOT be transformed
	if containsSubstring(result, "<em>@alice</em>") {
		t.Errorf("expected @alice to remain plain in pre, got %q", result)
	}
	if !containsSubstring(result, "<pre>@alice</pre>") {
		t.Errorf("expected <pre>@alice</pre> unchanged, got %q", result)
	}
}

func TestTransformTextNodesSkipsAnchor(t *testing.T) {
	input := "<p>hello @bob</p><a href='#'>@alice</a>"
	matcher := Matcher{
		Pattern: regexp.MustCompile(`@(\w+)`),
		Replace: func(text string, start, end int) (*html.Node, string) {
			match := text[start:end]
			link := &html.Node{
				Type: html.ElementNode,
				Data: "b",
			}
			link.AppendChild(&html.Node{
				Type: html.TextNode,
				Data: match,
			})
			return link, ""
		},
	}

	result, err := TransformTextNodes(input, []Matcher{matcher})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// @bob should be transformed
	if !containsSubstring(result, "<b>@bob</b>") {
		t.Errorf("expected <b>@bob</b> in result, got %q", result)
	}

	// @alice in anchor should NOT be transformed
	if containsSubstring(result, "<b>@alice</b>") {
		t.Errorf("expected @alice to remain plain in anchor, got %q", result)
	}
}

func TestTransformTextNodesMultipleMatches(t *testing.T) {
	input := "<p>hello @bob and @alice</p>"
	matcher := Matcher{
		Pattern: regexp.MustCompile(`@(\w+)`),
		Replace: func(text string, start, end int) (*html.Node, string) {
			match := text[start:end]
			link := &html.Node{
				Type: html.ElementNode,
				Data: "strong",
			}
			link.AppendChild(&html.Node{
				Type: html.TextNode,
				Data: match,
			})
			return link, ""
		},
	}

	result, err := TransformTextNodes(input, []Matcher{matcher})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Both should be transformed
	if !containsSubstring(result, "<strong>@bob</strong>") {
		t.Errorf("expected <strong>@bob</strong> in result, got %q", result)
	}
	if !containsSubstring(result, "<strong>@alice</strong>") {
		t.Errorf("expected <strong>@alice</strong> in result, got %q", result)
	}
}

func TestTransformTextNodesMultipleMatchers(t *testing.T) {
	input := "<p>hello @bob and #golang</p>"

	matchers := []Matcher{
		{
			Pattern: regexp.MustCompile(`@(\w+)`),
			Replace: func(text string, start, end int) (*html.Node, string) {
				match := text[start:end]
				link := &html.Node{
					Type: html.ElementNode,
					Data: "strong",
				}
				link.AppendChild(&html.Node{
					Type: html.TextNode,
					Data: match,
				})
				return link, ""
			},
		},
		{
			Pattern: regexp.MustCompile(`#(\w+)`),
			Replace: func(text string, start, end int) (*html.Node, string) {
				match := text[start:end]
				link := &html.Node{
					Type: html.ElementNode,
					Data: "em",
				}
				link.AppendChild(&html.Node{
					Type: html.TextNode,
					Data: match,
				})
				return link, ""
			},
		},
	}

	result, err := TransformTextNodes(input, matchers)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Both should be transformed
	if !containsSubstring(result, "<strong>@bob</strong>") {
		t.Errorf("expected <strong>@bob</strong> in result, got %q", result)
	}
	if !containsSubstring(result, "<em>#golang</em>") {
		t.Errorf("expected <em>#golang</em> in result, got %q", result)
	}
}

func TestTransformTextNodesMatcherReturnsNil(t *testing.T) {
	input := "<p>hello @bob world</p>"
	matcher := Matcher{
		Pattern: regexp.MustCompile(`@(\w+)`),
		Replace: func(text string, start, end int) (*html.Node, string) {
			return nil, "" // Skip transforming
		},
	}

	result, err := TransformTextNodes(input, []Matcher{matcher})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Match should remain as plain text
	if !containsSubstring(result, "@bob") {
		t.Errorf("expected @bob to remain as plain text, got %q", result)
	}
}

func containsSubstring(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle || len(needle) == 0 ||
		(len(haystack) > 0 && len(needle) > 0 &&
			func() bool {
				for i := 0; i <= len(haystack)-len(needle); i++ {
					if haystack[i:i+len(needle)] == needle {
						return true
					}
				}
				return false
			}()))
}
