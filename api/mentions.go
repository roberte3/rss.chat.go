package api

import (
	"database/sql"
	"fmt"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
	"regexp"
	"strings"
)

// MentionMatcher creates a matcher for rendering @mentions as links to users.
// The urlTemplate should contain {screenname} placeholder (e.g., "https://example.com/?screenname={screenname}").
// Only screennames that exist in the database are linkified; non-existent mentions remain plain text.
func MentionMatcher(conn *sql.DB, urlTemplate string) Matcher {
	// Match @screenname but don't consume the preceding boundary
	mentionPattern := regexp.MustCompile(`@([A-Za-z0-9_]{1,64})`)

	return Matcher{
		Pattern: mentionPattern,
		Replace: func(text string, start, end int) (*html.Node, string) {
			// Check if there's a valid boundary before the @
			if start > 0 {
				charBefore := text[start-1]
				// Valid boundaries: space, paren, angle bracket, or start of line
				if charBefore != ' ' && charBefore != '(' && charBefore != '>' && start != 0 {
					// Previous character is part of email or other text, not a boundary
					return nil, ""
				}
			}

			// Extract screenname from the match
			screenname := text[start+1 : end] // Skip the @ and get the rest

			// Check if user exists (case-insensitive query)
			var canonicalScreenname string
			err := conn.QueryRow(
				"SELECT screenname FROM users WHERE LOWER(screenname) = LOWER(?)",
				screenname,
			).Scan(&canonicalScreenname)

			if err == sql.ErrNoRows {
				// User doesn't exist, leave as plain text
				return nil, ""
			}
			if err != nil {
				// DB error, also leave as plain text (safer than crashing)
				return nil, ""
			}

			// Create the mention link
			href := strings.ReplaceAll(urlTemplate, "{screenname}", canonicalScreenname)
			linkNode := &html.Node{
				Type:     html.ElementNode,
				Data:     "a",
				DataAtom: atom.A,
				Attr: []html.Attribute{
					{Key: "href", Val: href},
					{Key: "class", Val: "mention"},
				},
			}

			// Add the mention text inside the link (using original casing from text)
			mentionText := fmt.Sprintf("@%s", screenname)
			linkNode.AppendChild(&html.Node{
				Type: html.TextNode,
				Data: mentionText,
			})

			return linkNode, ""
		},
	}
}

// ExtractMentions finds all valid @mentions in HTML text and returns unique screennames.
// Only returns screennames that exist in the database.
func ExtractMentions(conn *sql.DB, htmlText string) ([]string, error) {
	mentionPattern := regexp.MustCompile(`(^|[\s(>])@([A-Za-z0-9_]{1,64})`)
	matches := mentionPattern.FindAllStringSubmatch(htmlText, -1)

	if len(matches) == 0 {
		return []string{}, nil
	}

	// Collect unique screennames (case-insensitive)
	screennameMap := make(map[string]bool)
	for _, match := range matches {
		if len(match) >= 3 {
			screennameMap[strings.ToLower(match[2])] = true
		}
	}

	// Build a list of screennames to query
	var screennamesToCheck []interface{}
	for screenname := range screennameMap {
		screennamesToCheck = append(screennamesToCheck, screenname)
	}

	if len(screennamesToCheck) == 0 {
		return []string{}, nil
	}

	// Query for existing users (case-insensitive)
	placeholders := make([]string, len(screennamesToCheck))
	for i := range screennamesToCheck {
		placeholders[i] = "LOWER(screenname) = ?"
	}

	query := "SELECT DISTINCT screenname FROM users WHERE " + strings.Join(placeholders, " OR ")
	rows, err := conn.Query(query, screennamesToCheck...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var mentions []string
	for rows.Next() {
		var screenname string
		if err := rows.Scan(&screenname); err != nil {
			return nil, err
		}
		mentions = append(mentions, screenname)
	}

	return mentions, rows.Err()
}
