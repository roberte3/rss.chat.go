package api

import (
	"bytes"
	"fmt"
	"golang.org/x/net/html"
	"regexp"
	"strings"
)

// Matcher defines a pattern and how to transform matched text into nodes.
type Matcher struct {
	Pattern *regexp.Regexp
	// Replace is called for each match with the full text and match positions.
	// It should return: a replacement node to splice in, optionally including part of
	// the original text inside the node; and any remaining text from the match that
	// should be kept as plain text (e.g., trimmed trailing punctuation).
	Replace func(text string, start, end int) (*html.Node, string)
}

// TransformTextNodes applies one or more matchers to HTML text, transforming matched
// portions in text nodes. Matchers run in a single parse pass. Only text nodes are
// searched; content in <a>, <pre>, <code>, <script>, <style> tags is untouched.
func TransformTextNodes(htmlText string, matchers []Matcher) (string, error) {
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

	// Transform contents of the div
	transformNode(divNode, matchers)

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

// transformNode recursively traverses the DOM and applies matchers to text nodes.
// It modifies the tree in-place.
func transformNode(n *html.Node, matchers []Matcher) {
	if n == nil {
		return
	}

	// Skip script and style tags
	if n.Type == html.ElementNode && (n.Data == "script" || n.Data == "style") {
		return
	}

	// Skip pre, code tags
	if n.Type == html.ElementNode && (n.Data == "pre" || n.Data == "code") {
		return
	}

	// Skip anchor tags
	if n.Type == html.ElementNode && n.Data == "a" {
		return
	}

	// Process children in forward order, but save next before modifying
	c := n.FirstChild
	for c != nil {
		next := c.NextSibling

		if c.Type == html.TextNode {
			transformTextNode(c, matchers)
		} else {
			transformNode(c, matchers)
		}

		c = next
	}
}

// transformTextNode applies all matchers to a text node and replaces it with a mix
// of text and transformed nodes.
func transformTextNode(textNode *html.Node, matchers []Matcher) {
	if textNode.Type != html.TextNode {
		return
	}

	text := textNode.Data
	parent := textNode.Parent
	if parent == nil {
		return
	}

	// Apply matchers in sequence, building a list of replacement nodes
	var replacements []interface{} // mix of string segments and *html.Node
	replacements = append(replacements, text)

	for _, matcher := range matchers {
		var newReplacements []interface{}

		for _, seg := range replacements {
			// Only process string segments; leave nodes alone
			str, ok := seg.(string)
			if !ok {
				newReplacements = append(newReplacements, seg)
				continue
			}

			// Apply the matcher to this string segment
			matches := matcher.Pattern.FindAllStringIndex(str, -1)
			if len(matches) == 0 {
				newReplacements = append(newReplacements, str)
				continue
			}

			// Build new segments from matches
			lastEnd := 0
			for _, match := range matches {
				start, end := match[0], match[1]

				// Add text before the match
				if start > lastEnd {
					newReplacements = append(newReplacements, str[lastEnd:start])
				}

				// Call the matcher's Replace callback
				node, remaining := matcher.Replace(str, start, end)
				if node != nil {
					newReplacements = append(newReplacements, node)
				} else {
					// If Replace returns nil, leave the match as plain text
					newReplacements = append(newReplacements, str[start:end])
				}

				// Include any remaining text from the match (like trimmed punctuation)
				if remaining != "" {
					newReplacements = append(newReplacements, remaining)
				}

				lastEnd = end
			}

			// Add remaining text after last match
			if lastEnd < len(str) {
				newReplacements = append(newReplacements, str[lastEnd:])
			}
		}

		replacements = newReplacements
	}

	// Convert replacements to nodes and insert before the original text node
	for _, item := range replacements {
		switch v := item.(type) {
		case string:
			if v != "" {
				newNode := &html.Node{
					Type: html.TextNode,
					Data: v,
				}
				parent.InsertBefore(newNode, textNode)
			}
		case *html.Node:
			parent.InsertBefore(v, textNode)
		}
	}

	// Remove the original text node
	parent.RemoveChild(textNode)
}
