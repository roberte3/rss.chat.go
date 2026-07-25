package api

import (
	"testing"
)

func TestSanitizePostHTML(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "preserves plain text",
			input:    "Hello world",
			expected: "Hello world",
		},
		{
			name:     "preserves paragraphs",
			input:    "<p>Hello</p><p>World</p>",
			expected: "<p>Hello</p><p>World</p>",
		},
		{
			name:     "preserves links",
			input:    `<a href="https://example.com">click here</a>`,
			expected: `<a href="https://example.com" rel="nofollow">click here</a>`,
		},
		{
			name:     "preserves bold and italic",
			input:    "<p>This is <b>bold</b> and <i>italic</i></p>",
			expected: "<p>This is <b>bold</b> and <i>italic</i></p>",
		},
		{
			name:     "preserves strong and em",
			input:    "<p>This is <strong>strong</strong> and <em>emphasized</em></p>",
			expected: "<p>This is <strong>strong</strong> and <em>emphasized</em></p>",
		},
		{
			name:     "preserves lists",
			input:    "<ul><li>Item 1</li><li>Item 2</li></ul>",
			expected: "<ul><li>Item 1</li><li>Item 2</li></ul>",
		},
		{
			name:     "preserves ordered lists",
			input:    "<ol><li>First</li><li>Second</li></ol>",
			expected: "<ol><li>First</li><li>Second</li></ol>",
		},
		{
			name:     "preserves blockquotes",
			input:    "<blockquote><p>A quote</p></blockquote>",
			expected: "<blockquote><p>A quote</p></blockquote>",
		},
		{
			name:     "preserves images with safe attributes",
			input:    `<img src="/media/123" alt="photo">`,
			expected: `<img src="/media/123" alt="photo">`,
		},
		{
			name:     "removes script tags",
			input:    `<p>Hello</p><script>alert('xss')</script><p>World</p>`,
			expected: "<p>Hello</p><p>World</p>",
		},
		{
			name:     "removes event handlers",
			input:    `<img src="x" onerror="alert('xss')">`,
			expected: `<img src="x">`,
		},
		{
			name:     "removes multiple event handlers",
			input:    `<p onclick="steal()" onmouseover="alert()">Click me</p>`,
			expected: "<p>Click me</p>",
		},
		{
			name:     "removes iframes",
			input:    `<p>Content</p><iframe src="evil.com"></iframe><p>More</p>`,
			expected: "<p>Content</p><p>More</p>",
		},
		{
			name:     "removes style tags",
			input:    `<style>body { display: none; }</style><p>Hello</p>`,
			expected: "<p>Hello</p>",
		},
		{
			name:     "removes style attributes",
			input:    `<p style="color: red; background: url(javascript:alert())">Text</p>`,
			expected: "<p>Text</p>",
		},
		{
			name:     "blocks javascript protocol",
			input:    `<a href="javascript:alert('xss')">Click</a>`,
			expected: "Click",
		},
		{
			name:     "removes data URIs",
			input:    `<img src="data:text/html,<script>alert('xss')</script>">`,
			expected: ``,
		},
		{
			name:     "removes unknown tags",
			input:    `<p>Hello</p><custom-tag>should be removed</custom-tag><p>World</p>`,
			expected: "<p>Hello</p>should be removed<p>World</p>",
		},
		{
			name:     "allows headings",
			input:    "<h3>Heading</h3>",
			expected: "<h3>Heading</h3>",
		},
		{
			name:     "allows line breaks",
			input:    "<p>Line 1<br>Line 2</p>",
			expected: "<p>Line 1<br>Line 2</p>",
		},
		{
			name:     "complex legitimate content",
			input: `<p>Here's a <b>great</b> example:</p>
<blockquote>
<p>This is <strong>important</strong></p>
<ul>
<li>Point 1</li>
<li>Point 2</li>
</ul>
</blockquote>
<p>Check <a href="https://example.com">this link</a>.</p>
<p><img src="/media/123" alt="photo"></p>`,
			expected: `<p>Here&#39;s a <b>great</b> example:</p>
<blockquote>
<p>This is <strong>important</strong></p>
<ul>
<li>Point 1</li>
<li>Point 2</li>
</ul>
</blockquote>
<p>Check <a href="https://example.com" rel="nofollow">this link</a>.</p>
<p><img src="/media/123" alt="photo"></p>`,
		},
		{
			name:     "complex attack attempt",
			input: `<p>Click <a href="javascript:alert('xss')">here</a></p>
<img src=x onerror="alert('xss')">
<script>steal_data()</script>
<iframe src="https://evil.com"></iframe>
<p>Normal text</p>`,
			expected: `<p>Click here</p>
<img src="x">


<p>Normal text</p>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizePostHTML(tt.input)
			if result != tt.expected {
				t.Errorf("got:\n%q\n\nwant:\n%q", result, tt.expected)
			}
		})
	}
}
