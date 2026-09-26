package api

import (
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

	// Regex to find URLs: http(s)://, www.
	urlPattern := regexp.MustCompile(
		`(?:https?://|www\.)[^\s<>"{}|\\^~\[\]` + "`" + `]+`,
	)

	matcher := Matcher{
		Pattern: urlPattern,
		Replace: func(text string, start, end int) (*html.Node, string) {
			rawURL := text[start:end]

			// Trim trailing punctuation
			trimmedURL := strings.TrimRight(rawURL, ".,;:!?)'\"]")
			trailingPunct := rawURL[len(trimmedURL):]

			// Skip linkifying .md and .zip files
			if strings.HasSuffix(strings.ToLower(trimmedURL), ".md") ||
				strings.HasSuffix(strings.ToLower(trimmedURL), ".zip") {
				return nil, trailingPunct // Leave as plain text
			}

			// Create the link with trimmed URL, return trailing punctuation as plain text
			return createLinkNode(trimmedURL), trailingPunct
		},
	}

	return TransformTextNodes(htmlText, []Matcher{matcher})
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

	// Add text content
	textNode := &html.Node{
		Type: html.TextNode,
		Data: urlStr,
	}
	linkNode.AppendChild(textNode)

	return linkNode
}
