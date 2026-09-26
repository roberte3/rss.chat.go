package api

import (
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
	"regexp"
	"strings"
)

// HashtagMatcher creates a matcher for rendering #hashtags as links.
// The urlTemplate should contain {tag} placeholder (e.g., "https://example.com/?tag={tag}").
func HashtagMatcher(urlTemplate string) Matcher {
	// Match #hashtag - ASCII alphanumeric and underscore only, 1-50 chars
	hashtagPattern := regexp.MustCompile(`#([A-Za-z0-9_]{1,50})`)

	return Matcher{
		Pattern: hashtagPattern,
		Replace: func(text string, start, end int) (*html.Node, string) {
			// Extract the full match
			fullMatch := text[start:end]

			// The pattern is just #tag, so extract the tag (skip the #)
			tag := fullMatch[1:]

			// Create the hashtag link
			href := strings.ReplaceAll(urlTemplate, "{tag}", strings.ToLower(tag))
			linkNode := &html.Node{
				Type:     html.ElementNode,
				Data:     "a",
				DataAtom: atom.A,
				Attr: []html.Attribute{
					{Key: "href", Val: href},
					{Key: "class", Val: "hashtag"},
				},
			}

			// Add the hashtag text inside the link (preserve display casing)
			linkNode.AppendChild(&html.Node{
				Type: html.TextNode,
				Data: fullMatch, // Include the # in display
			})

			return linkNode, ""
		},
	}
}

// ExtractHashtags finds all valid #hashtags in HTML text and returns unique tags.
// Tags are normalized to lowercase for storage.
func ExtractHashtags(htmlText string) []string {
	hashtagPattern := regexp.MustCompile(`#([A-Za-z0-9_]{1,50})`)
	matches := hashtagPattern.FindAllStringSubmatch(htmlText, -1)

	if len(matches) == 0 {
		return []string{}
	}

	// Collect unique tags (normalized to lowercase)
	tagMap := make(map[string]bool)
	for _, match := range matches {
		if len(match) >= 2 {
			tagMap[strings.ToLower(match[1])] = true
		}
	}

	// Build result list
	var tags []string
	for tag := range tagMap {
		tags = append(tags, tag)
	}

	return tags
}
