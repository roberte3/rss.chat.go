package api

import (
	"bytes"
	"fmt"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
	"regexp"
	"strings"
)

// LinkifyURLs finds bare URLs in HTML text and wraps them in <a> tags.
// Only linkifies URLs in text nodes, not in existing attributes or tags.
// Handles http://, https://, and www. URLs.
func LinkifyURLs(htmlText string) (string, error) {
	if strings.TrimSpace(htmlText) == "" {
		return "", nil
	}

	// Create a wrapper div to parse into
	wrapper := fmt.Sprintf("<div>%s</div>", htmlText)
	doc, err := html.Parse(strings.NewReader(wrapper))
	if err != nil {
		return "", fmt.Errorf("parse html: %w", err)
	}

	// Find the div we created
	var divNode *html.Node
	var findDiv func(*html.Node)
	findDiv = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "div" && divNode == nil {
			divNode = n
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findDiv(c)
		}
	}
	findDiv(doc)

	if divNode == nil {
		return "", fmt.Errorf("could not find wrapper div")
	}

	// Linkify contents of the div
	linkifyNode(divNode)

	// Render div's children back to HTML
	var buf bytes.Buffer
	for c := divNode.FirstChild; c != nil; c = c.NextSibling {
		err = html.Render(&buf, c)
		if err != nil {
			return "", fmt.Errorf("render html: %w", err)
		}
	}

	return buf.String(), nil
}

// linkifyNode recursively traverses the DOM and linkifies text in text nodes.
// It modifies the tree in-place by replacing text nodes with a mix of text and link nodes.
func linkifyNode(n *html.Node) {
	if n == nil {
		return
	}

	// Skip script and style tags (don't descend into them)
	if n.Type == html.ElementNode && (n.Data == "script" || n.Data == "style") {
		return
	}

	// Skip pre, code tags (preserve formatting, don't linkify inside)
	if n.Type == html.ElementNode && (n.Data == "pre" || n.Data == "code") {
		return
	}

	// Skip anchor tags (already has links)
	if n.Type == html.ElementNode && n.Data == "a" {
		return
	}

	// Process children in forward order, but save next before modifying
	c := n.FirstChild
	for c != nil {
		next := c.NextSibling // Save next before potentially modifying tree

		if c.Type == html.TextNode {
			linkifyTextNode(c)
		} else {
			linkifyNode(c)
		}

		c = next
	}
}

// linkifyTextNode finds URLs in a text node and replaces it with a mix of text and link nodes.
func linkifyTextNode(textNode *html.Node) {
	if textNode.Type != html.TextNode {
		return
	}

	text := textNode.Data

	// Regex to find URLs: http(s)://, www.
	urlPattern := regexp.MustCompile(
		`(?:https?://|www\.)[^\s<>"{}|\\^~\[\]` + "`" + `]+`,
	)

	matches := urlPattern.FindAllStringIndex(text, -1)
	if len(matches) == 0 {
		// No URLs found
		return
	}

	parent := textNode.Parent
	if parent == nil {
		return
	}

	// Build new nodes to replace the text node
	var newNodes []*html.Node
	lastEnd := 0

	for _, match := range matches {
		start, end := match[0], match[1]
		rawURL := text[start:end]

		// Trim trailing punctuation
		trimmedURL := strings.TrimRight(rawURL, ".,;:!?)'\"]")
		trailingPunct := rawURL[len(trimmedURL):]

		// Add text before the URL
		if start > lastEnd {
			beforeNode := &html.Node{
				Type: html.TextNode,
				Data: text[lastEnd:start],
			}
			newNodes = append(newNodes, beforeNode)
		}

		// Skip linkifying .md and .zip files
		if strings.HasSuffix(strings.ToLower(trimmedURL), ".md") ||
			strings.HasSuffix(strings.ToLower(trimmedURL), ".zip") {
			// Add as plain text instead of a link
			urlNode := &html.Node{
				Type: html.TextNode,
				Data: trimmedURL,
			}
			newNodes = append(newNodes, urlNode)
		} else {
			// Create the link with trimmed URL
			linkNode := createLinkNode(trimmedURL)
			newNodes = append(newNodes, linkNode)
		}

		// Add trailing punctuation as text if any
		if trailingPunct != "" {
			punctNode := &html.Node{
				Type: html.TextNode,
				Data: trailingPunct,
			}
			newNodes = append(newNodes, punctNode)
		}

		lastEnd = end
	}

	// Add remaining text after last URL
	if lastEnd < len(text) {
		afterNode := &html.Node{
			Type: html.TextNode,
			Data: text[lastEnd:],
		}
		newNodes = append(newNodes, afterNode)
	}

	// Insert all new nodes before the text node
	for _, newNode := range newNodes {
		parent.InsertBefore(newNode, textNode)
	}

	// Remove the original text node
	parent.RemoveChild(textNode)
}

// createLinkNode creates an <a> element for a URL.
func createLinkNode(urlStr string) *html.Node {
	// Ensure the URL has a protocol
	href := urlStr
	if strings.HasPrefix(urlStr, "www.") {
		href = "http://" + urlStr
	}

	// The link node
	linkNode := &html.Node{
		Type:     html.ElementNode,
		Data:     "a",
		DataAtom: atom.A,
	}

	// Add href attribute
	linkNode.Attr = []html.Attribute{
		{
			Key: "href",
			Val: href,
		},
	}

	// Add text content (don't set Parent, AppendChild does that)
	textNode := &html.Node{
		Type: html.TextNode,
		Data: urlStr,
	}
	linkNode.AppendChild(textNode)

	return linkNode
}
