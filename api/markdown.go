package api

import (
	"bytes"
	"golang.org/x/net/html"
	"strings"
)

// HtmlToMarkdown converts HTML to Markdown.
// Handles common HTML elements: paragraphs, headings, links, bold, italic, lists, code.
func HtmlToMarkdown(htmlText string) (string, error) {
	if strings.TrimSpace(htmlText) == "" {
		return "", nil
	}

	doc, err := html.Parse(strings.NewReader(htmlText))
	if err != nil {
		return "", err
	}

	// Find the body element (html.Parse always creates a full document)
	var body *html.Node
	var findBody func(*html.Node)
	findBody = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "body" {
			body = n
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findBody(c)
		}
	}
	findBody(doc)

	if body == nil {
		body = doc // Fallback if no body found
	}

	var buf bytes.Buffer
	// Process body's children
	for c := body.FirstChild; c != nil; c = c.NextSibling {
		nodeToMarkdown(c, &buf, &mdContext{})
	}
	result := buf.String()

	// Clean up excessive whitespace
	result = strings.TrimSpace(result)
	// Normalize multiple newlines to max 2
	for strings.Contains(result, "\n\n\n") {
		result = strings.ReplaceAll(result, "\n\n\n", "\n\n")
	}

	return result, nil
}

// mdContext tracks state during markdown conversion
type mdContext struct {
	inList      bool
	listLevel   int
	inCode      bool
	inPre       bool
	inLink      bool
	lastWasText bool
}

// nodeToMarkdown recursively converts HTML nodes to Markdown.
func nodeToMarkdown(n *html.Node, buf *bytes.Buffer, ctx *mdContext) {
	if n == nil {
		return
	}

	switch n.Type {
	case html.TextNode:
		text := n.Data
		if !ctx.inPre {
			// Normalize whitespace: replace newlines/tabs with spaces, collapse multiple spaces
			text = strings.NewReplacer("\n", " ", "\t", " ", "\r", "").Replace(text)
			// Collapse multiple spaces but preserve single spaces
			for strings.Contains(text, "  ") {
				text = strings.ReplaceAll(text, "  ", " ")
			}
		}
		if text != "" {
			buf.WriteString(text)
			ctx.lastWasText = true
		}

	case html.ElementNode:
		switch n.Data {
		// Headings
		case "h1":
			ensureNewline(buf)
			buf.WriteString("# ")
			nodeChildrenToMarkdown(n, buf, ctx)
			ensureNewline(buf)
			ctx.lastWasText = false

		case "h2":
			ensureNewline(buf)
			buf.WriteString("## ")
			nodeChildrenToMarkdown(n, buf, ctx)
			ensureNewline(buf)
			ctx.lastWasText = false

		case "h3":
			ensureNewline(buf)
			buf.WriteString("### ")
			nodeChildrenToMarkdown(n, buf, ctx)
			ensureNewline(buf)
			ctx.lastWasText = false

		// Paragraphs
		case "p":
			if buf.Len() > 0 {
				// Add spacing before paragraph if not at start
				buf.WriteString("\n\n")
			}
			nodeChildrenToMarkdown(n, buf, ctx)
			ctx.lastWasText = false

		// Links
		case "a":
			href := getAttribute(n, "href")
			buf.WriteString("[")
			nodeChildrenToMarkdown(n, buf, ctx)
			buf.WriteString("](")
			buf.WriteString(href)
			buf.WriteString(")")
			ctx.lastWasText = false

		// Emphasis
		case "em", "i":
			buf.WriteString("*")
			nodeChildrenToMarkdown(n, buf, ctx)
			buf.WriteString("*")
			ctx.lastWasText = false

		case "strong", "b":
			buf.WriteString("**")
			nodeChildrenToMarkdown(n, buf, ctx)
			buf.WriteString("**")
			ctx.lastWasText = false

		// Code (skip formatting if already in pre)
		case "code":
			if !ctx.inPre {
				buf.WriteString("`")
				nodeChildrenToMarkdown(n, buf, ctx)
				buf.WriteString("`")
			} else {
				nodeChildrenToMarkdown(n, buf, ctx)
			}
			ctx.lastWasText = false

		case "pre":
			oldInPre := ctx.inPre
			ctx.inPre = true
			ensureNewline(buf)
			buf.WriteString("```\n")
			nodeChildrenToMarkdown(n, buf, ctx)
			ensureNewline(buf)
			buf.WriteString("```\n")
			ctx.inPre = oldInPre
			ctx.lastWasText = false

		// Line breaks
		case "br":
			buf.WriteString("  \n")
			ctx.lastWasText = false

		// Horizontal rule
		case "hr":
			ensureNewline(buf)
			buf.WriteString("---\n")
			ctx.lastWasText = false

		// Lists
		case "ul":
			ensureNewline(buf)
			oldInList := ctx.inList
			oldListLevel := ctx.listLevel
			ctx.inList = true
			ctx.listLevel = 0
			nodeChildrenToMarkdown(n, buf, ctx)
			ctx.inList = oldInList
			ctx.listLevel = oldListLevel
			ensureNewline(buf)
			ctx.lastWasText = false

		case "ol":
			ensureNewline(buf)
			oldInList := ctx.inList
			oldListLevel := ctx.listLevel
			ctx.inList = true
			ctx.listLevel = 0
			nodeChildrenToMarkdown(n, buf, ctx)
			ctx.inList = oldInList
			ctx.listLevel = oldListLevel
			ensureNewline(buf)
			ctx.lastWasText = false

		case "li":
			ensureNewline(buf)
			// Add indentation for nested lists
			for i := 0; i < ctx.listLevel; i++ {
				buf.WriteString("  ")
			}
			buf.WriteString("- ")
			ctx.listLevel++
			nodeChildrenToMarkdown(n, buf, ctx)
			ctx.listLevel--
			ctx.lastWasText = false

		// Blockquote
		case "blockquote":
			ensureNewline(buf)
			oldInList := ctx.inList
			ctx.inList = false
			nodeChildrenToMarkdown(n, buf, ctx)
			ctx.inList = oldInList
			ensureNewline(buf)
			ctx.lastWasText = false

		// Ignore these tags but process children
		case "div", "span", "section", "article":
			nodeChildrenToMarkdown(n, buf, ctx)

		// Skip script and style
		case "script", "style":
			// Don't process children

		// Default: process children
		default:
			nodeChildrenToMarkdown(n, buf, ctx)
		}
	}
}

// nodeChildrenToMarkdown processes all children of a node.
func nodeChildrenToMarkdown(n *html.Node, buf *bytes.Buffer, ctx *mdContext) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		nodeToMarkdown(c, buf, ctx)
	}
}

// getAttribute gets an attribute value from a node, or empty string if not found.
func getAttribute(n *html.Node, name string) string {
	for _, attr := range n.Attr {
		if attr.Key == name {
			return attr.Val
		}
	}
	return ""
}

// ensureNewline ensures we end with a newline.
func ensureNewline(buf *bytes.Buffer) {
	content := buf.String()
	if len(content) == 0 || content[len(content)-1] != '\n' {
		buf.WriteString("\n")
	}
}
