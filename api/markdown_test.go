package api

import (
	"strings"
	"testing"
)

func TestHtmlToMarkdown(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "plain text",
			input:    "Hello world",
			expected: "Hello world",
		},
		{
			name:     "paragraph",
			input:    "<p>Hello world</p>",
			expected: "Hello world",
		},
		{
			name:     "multiple paragraphs",
			input:    "<p>First</p><p>Second</p>",
			expected: "First\n\nSecond",
		},
		{
			name:     "bold",
			input:    "<p>This is <strong>bold</strong> text</p>",
			expected: "This is **bold** text",
		},
		{
			name:     "italic",
			input:    "<p>This is <em>italic</em> text</p>",
			expected: "This is *italic* text",
		},
		{
			name:     "link",
			input:    `<p><a href="http://example.com">Click here</a></p>`,
			expected: "[Click here](http://example.com)",
		},
		{
			name:     "heading",
			input:    "<h1>Title</h1>",
			expected: "# Title",
		},
		{
			name:     "h2 heading",
			input:    "<h2>Subtitle</h2>",
			expected: "## Subtitle",
		},
		{
			name:     "unordered list",
			input:    "<ul><li>Item 1</li><li>Item 2</li></ul>",
			expected: "- Item 1\n- Item 2",
		},
		{
			name:     "code block",
			input:    "<pre><code>var x = 1;</code></pre>",
			expected: "```\nvar x = 1;\n```",
		},
		{
			name:     "inline code",
			input:    "<p>Use <code>console.log()</code> to debug</p>",
			expected: "Use `console.log()` to debug",
		},
		{
			name:     "line break",
			input:    "<p>Line 1<br>Line 2</p>",
			expected: "Line 1  \nLine 2",
		},
		{
			name:     "nested tags",
			input:    "<p>This is <strong><em>bold italic</em></strong> text</p>",
			expected: "This is ***bold italic*** text",
		},
		{
			name:     "link with formatting",
			input:    `<p><a href="http://example.com"><strong>Bold Link</strong></a></p>`,
			expected: "[**Bold Link**](http://example.com)",
		},
		{
			name:     "empty input",
			input:    "",
			expected: "",
		},
		{
			name:     "whitespace only",
			input:    "   \n\n   ",
			expected: "",
		},
		{
			name:     "multiple spaces collapse",
			input:    "<p>Multiple   spaces   here</p>",
			expected: "Multiple spaces here",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := HtmlToMarkdown(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			result = strings.TrimSpace(result)
			expected := strings.TrimSpace(tc.expected)

			if result != expected {
				t.Errorf("mismatch\nExpected: %q\nGot:      %q", expected, result)
			}
		})
	}
}

func TestHtmlToMarkdownComplex(t *testing.T) {
	html := `
	<h1>My Article</h1>
	<p>This is a paragraph with <strong>bold</strong> and <em>italic</em> text.</p>
	<p>Here's a <a href="http://example.com">link to example</a>.</p>
	<h2>Subsection</h2>
	<ul>
		<li>First item</li>
		<li>Second item</li>
		<li>Third item</li>
	</ul>
	<pre><code>function test() {
  return true;
}</code></pre>
	`

	result, err := HtmlToMarkdown(html)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Just verify it doesn't crash and produces markdown
	if !strings.Contains(result, "# My Article") {
		t.Errorf("expected heading in output")
	}

	if !strings.Contains(result, "**bold**") {
		t.Errorf("expected bold in output")
	}

	if !strings.Contains(result, "[link to example]") {
		t.Errorf("expected link in output")
	}

	if !strings.Contains(result, "## Subsection") {
		t.Errorf("expected h2 in output")
	}

	if !strings.Contains(result, "- First item") {
		t.Errorf("expected list in output")
	}

	if !strings.Contains(result, "```") {
		t.Errorf("expected code block in output")
	}
}

func TestHtmlToMarkdownPreservesCodeFormatting(t *testing.T) {
	// Inside <pre>, whitespace should be preserved
	html := `<pre>Line 1
Line 2
  Indented</pre>`

	result, err := HtmlToMarkdown(html)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should preserve the line breaks and indentation
	if !strings.Contains(result, "Line 1\nLine 2") {
		t.Errorf("expected preserved line breaks in pre block")
	}
}
