package db

import (
	"encoding/json"
	"time"
)

// User mirrors the users table, plus prefs decoded from its JSON column.
// Ported from convertUser in archive/rss.chat/server/code/rssnetwork.js.
type User struct {
	Screenname   string          `json:"screenname"`
	EmailAddress string          `json:"emailAddress,omitempty"`
	EmailSecret  string          `json:"emailSecret,omitempty"`
	ImageURL     string          `json:"imageUrl,omitempty"`
	WhenCreated  time.Time       `json:"whenCreated"`
	WhenUpdated  time.Time       `json:"whenUpdated"`
	Prefs        json.RawMessage `json:"prefs,omitempty"`
}

// ActiveUser is one row of GetMostActiveToday's ranked-by-activity result.
type ActiveUser struct {
	Screenname  string    `json:"screenname"`
	Name        string    `json:"name"`
	ImageURL    string    `json:"imageUrl,omitempty"`
	CtHits      int64     `json:"ctHits"`
	CtHitsToday int64     `json:"ctHitsToday"`
	WhenLastHit time.Time `json:"whenLastHit"`
}

// Item mirrors the items table, plus the computed fields (guid, author,
// inReplyToUrl, like/reply counts) that convertItem builds up around it.
// Ported from convertItem in archive/rss.chat/server/code/rssnetwork.js.
type Item struct {
	ID              int64     `json:"id"`
	FeedURL         string    `json:"feedUrl,omitempty"`
	Guid            string    `json:"guid,omitempty"`
	Title           string    `json:"title,omitempty"`
	InReplyToNum    *int64    `json:"inReplyToNum,omitempty"`
	InReplyToURL    string    `json:"inReplyToUrl,omitempty"`
	Link            string    `json:"link,omitempty"`
	Description     string    `json:"description,omitempty"`
	PubDate         time.Time `json:"pubDate"`
	EnclosureURL    string    `json:"enclosureUrl,omitempty"`
	EnclosureType   string    `json:"enclosureType,omitempty"`
	EnclosureLength *int64    `json:"enclosureLength,omitempty"`
	WhenCreated     time.Time `json:"whenCreated"`
	WhenUpdated     time.Time `json:"whenUpdated"`
	MarkdownText    string    `json:"markdowntext,omitempty"`
	OutlineJSONText string    `json:"outlineJsontext,omitempty"`
	ImageURL        string    `json:"imageUrl,omitempty"`
	Author          string    `json:"author,omitempty"`
	Screenname      string    `json:"screenname,omitempty"`
	FeedLink        string    `json:"feedLink,omitempty"`
	FeedDescription string    `json:"feedDescription,omitempty"`
	FlDeleted       bool      `json:"flDeleted,omitempty"`
	CtLikes         int64     `json:"ctLikes,omitempty"`
	FlLiked         bool      `json:"flLiked,omitempty"`
	InReplyToAuthor string    `json:"inReplyToAuthor,omitempty"`
	CtReplies       int64     `json:"ctReplies,omitempty"`
}

// ItemPatch carries the fields updateItem is allowed to change; a nil field
// means "leave as is", mirroring the JS updateItem's "only set fields the
// caller provided" behavior.
type ItemPatch struct {
	ID              int64
	FeedURL         *string
	Title           *string
	Link            *string
	Description     *string
	InReplyTo       *int64
	PubDate         *time.Time
	EnclosureURL    *string
	EnclosureType   *string
	EnclosureLength *int64
	MarkdownText    *string
	OutlineJSONText *string
	Author          *string
}
