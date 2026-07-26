package feed

import (
	"database/sql"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"rss.chat.go/db"
)

// BuilderConfig holds configuration for feed generation.
type BuilderConfig struct {
	BaseURL                  string
	ProductName              string
	MaxFeedItems             int
	Language                 string
	DocsURL                  string
	RSSCloudEnabled          bool
	RSSCloudDomain           string
	RSSCloudPort             string
	RSSCloudPath             string
	RSSCloudReg              string
	RSSCloudProto            string
	TitleForSubscriptionList string // Custom OPML title (optional)
}

// BuildFeedForUser generates an RSS feed for a user's posts.
func BuildFeedForUser(conn *sql.DB, userScreenname string, baseURL string, config BuilderConfig) (string, error) {
	user, err := db.GetUserInfoByScreenname(conn, userScreenname)
	if err != nil {
		return "", fmt.Errorf("failed to fetch user: %w", err)
	}
	if user == nil {
		return "", fmt.Errorf("user not found: %s", userScreenname)
	}

	feedURL := getFeedURL(baseURL, userScreenname)
	items, err := db.GetRecentUserItems(conn, "", feedURL, config.MaxFeedItems, baseURL)
	if err != nil {
		return "", fmt.Errorf("failed to fetch user items: %w", err)
	}

	itemPtrs := make([]*db.Item, len(items))
	for i := range items {
		itemPtrs[i] = &items[i]
	}

	return buildRSSFeed(user, itemPtrs, feedURL, config, false)
}

// BuildFeedForEveryone generates an RSS feed of all posts on the network.
func BuildFeedForEveryone(conn *sql.DB, baseURL string, config BuilderConfig) (string, error) {
	items, err := db.GetRecentItems(conn, "", config.MaxFeedItems, baseURL)
	if err != nil {
		return "", fmt.Errorf("failed to fetch recent items: %w", err)
	}

	itemPtrs := make([]*db.Item, len(items))
	for i := range items {
		itemPtrs[i] = &items[i]
	}

	return buildRSSFeedForEveryone(itemPtrs, config)
}

// BuildCommentsFeed generates an RSS feed of replies to a post.
func BuildCommentsFeed(conn *sql.DB, itemID int64, baseURL string, config BuilderConfig) (string, error) {
	items, err := db.GetItemAndReplies(conn, "", itemID, baseURL)
	if err != nil {
		return "", fmt.Errorf("failed to fetch item and replies: %w", err)
	}

	if len(items) == 0 {
		return "", fmt.Errorf("no post found with id %d", itemID)
	}

	itemPtrs := make([]*db.Item, len(items))
	for i := range items {
		itemPtrs[i] = &items[i]
	}

	parentItem := itemPtrs[0]
	return buildRSSFeedForComments(itemPtrs, parentItem, config)
}

// BuildSubscriptionList generates an OPML subscription list of all users.
func BuildSubscriptionList(conn *sql.DB, baseURL string, config BuilderConfig) (string, error) {
	screennames, err := db.GetAllScreennames(conn)
	if err != nil {
		return "", fmt.Errorf("failed to fetch screennames: %w", err)
	}

	// Use custom title if provided, otherwise use default
	title := config.TitleForSubscriptionList
	if title == "" {
		title = fmt.Sprintf("Subscription list for %s running on %s", config.ProductName, baseURL)
	}

	opml := &OPMLFeed{
		Version: "2.0",
		Head: &Head{
			Title:        title,
			DateModified: time.Now().UTC().Format("Mon, 02 Jan 2006 15:04:05 GMT"),
		},
		Body: &Body{
			Outlines: make([]*Outline, 0, len(screennames)),
		},
	}

	for _, screenname := range screennames {
		opml.Body.Outlines = append(opml.Body.Outlines, &Outline{
			Type:   "rss",
			Text:   screenname,
			XMLURL: getFeedURL(baseURL, screenname),
		})
	}

	data, err := xml.MarshalIndent(opml, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal OPML: %w", err)
	}

	// Prepend XML declaration
	return xml.Header + string(data), nil
}

// buildRSSFeed creates an RSS feed for a user.
func buildRSSFeed(user *db.User, items []*db.Item, feedURL string, config BuilderConfig, sourceAttribution bool) (string, error) {
	channel := &Channel{
		Title:       user.Screenname + " on rss.network",
		Link:        "http://" + config.BaseURL + "/",
		Description: "Posts by " + user.Screenname + " on rss.network",
		Language:    config.Language,
		Docs:        config.DocsURL,
		Generator:   fmt.Sprintf("%s v1", config.ProductName),
		SelfLink: &Link{
			Rel:  "self",
			Type: "application/rss+xml",
			Href: feedURL,
		},
	}

	// Override with user prefs if available
	if user.Prefs != nil {
		var prefs map[string]interface{}
		if err := json.Unmarshal(user.Prefs, &prefs); err == nil {
			if title, ok := prefs["myFeedTitle"].(string); ok && title != "" {
				channel.Title = title
			}
			if link, ok := prefs["myFeedLink"].(string); ok && link != "" {
				channel.Link = link
			}
			if desc, ok := prefs["myFeedDescription"].(string); ok && desc != "" {
				channel.Description = desc
			}
			if imageURL, ok := prefs["myAvatarImageUrl"].(string); ok && imageURL != "" {
				channel.Image = &Image{
					URL:         imageURL,
					Title:       channel.Title,
					Link:        channel.Link,
					Description: channel.Description,
				}
			}
		}
	}

	// Add RSS Cloud if enabled
	if config.RSSCloudEnabled {
		channel.Cloud = &Cloud{
			Domain:            config.RSSCloudDomain,
			Port:              config.RSSCloudPort,
			Path:              config.RSSCloudPath,
			RegisterProcedure: config.RSSCloudReg,
			Protocol:          config.RSSCloudProto,
		}
	}

	// Build items
	channel.Items = buildFeedItems(items, config, sourceAttribution)

	feed := &RSSFeed{
		Version: "2.0",
		Channel: channel,
		NS:      "http://www.opml.org/spec2",
	}

	data, err := xml.MarshalIndent(feed, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal RSS: %w", err)
	}

	return xml.Header + string(data), nil
}

// buildRSSFeedForEveryone creates an RSS feed for the entire network.
func buildRSSFeedForEveryone(items []*db.Item, config BuilderConfig) (string, error) {
	everyoneFeedURL := getEveryoneFeedURL(config.BaseURL)

	channel := &Channel{
		Title:       "Everyone on rss.network",
		Link:        "http://" + config.BaseURL + "/",
		Description: "All posts on rss.network",
		Language:    config.Language,
		Docs:        config.DocsURL,
		Generator:   fmt.Sprintf("%s v1", config.ProductName),
		SelfLink: &Link{
			Rel:  "self",
			Type: "application/rss+xml",
			Href: everyoneFeedURL,
		},
	}

	if config.RSSCloudEnabled {
		channel.Cloud = &Cloud{
			Domain:            config.RSSCloudDomain,
			Port:              config.RSSCloudPort,
			Path:              config.RSSCloudPath,
			RegisterProcedure: config.RSSCloudReg,
			Protocol:          config.RSSCloudProto,
		}
	}

	// Build items with source attribution
	channel.Items = buildFeedItems(items, config, true)

	feed := &RSSFeed{
		Version: "2.0",
		Channel: channel,
		NS:      "http://www.opml.org/spec2",
	}

	data, err := xml.MarshalIndent(feed, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal RSS: %w", err)
	}

	return xml.Header + string(data), nil
}

// buildRSSFeedForComments creates an RSS feed for replies to a post.
func buildRSSFeedForComments(allItems []*db.Item, parentItem *db.Item, config BuilderConfig) (string, error) {
	channel := &Channel{
		Title:       fmt.Sprintf("Comments on post %d", parentItem.ID),
		Link:        parentItem.Guid,
		Description: fmt.Sprintf("Replies to post by %s", parentItem.Author),
		Language:    config.Language,
		Docs:        config.DocsURL,
		Generator:   fmt.Sprintf("%s v1", config.ProductName),
		SelfLink: &Link{
			Rel:  "self",
			Type: "application/rss+xml",
			Href: getCommentsFeedURL(config.BaseURL, parentItem.Screenname, parentItem.ID),
		},
	}

	// Build items with source attribution
	channel.Items = buildFeedItems(allItems, config, true)

	feed := &RSSFeed{
		Version: "2.0",
		Channel: channel,
		NS:      "http://www.opml.org/spec2",
	}

	data, err := xml.MarshalIndent(feed, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal RSS: %w", err)
	}

	return xml.Header + string(data), nil
}

// buildFeedItems transforms database items into RSS feed items.
func buildFeedItems(items []*db.Item, config BuilderConfig, sourceAttribution bool) []*Item {
	feedItems := make([]*Item, 0, len(items))

	for _, dbItem := range items {
		if dbItem.FlDeleted {
			continue
		}

		feedItem := &Item{
			Title:       dbItem.Title,
			Link:        dbItem.Link,
			Description: dbItem.Description,
			PubDate:     dbItem.PubDate.Format("Mon, 02 Jan 2006 15:04:05 GMT"),
			Guid: &Guid{
				IsPermalink: "true",
				Value:       dbItem.Guid,
			},
			MarkdownText: dbItem.MarkdownText,
			Account: &Account{
				Service: config.BaseURL,
				Name:    dbItem.Screenname,
			},
		}

		if dbItem.EnclosureURL != "" {
			feedItem.Enclosure = &Enclosure{
				URL:    dbItem.EnclosureURL,
				Type:   dbItem.EnclosureType,
				Length: formatEnclosureLength(dbItem.EnclosureLength),
			}
		}

		if dbItem.InReplyToURL != "" {
			feedItem.InReplyTo = &InReplyTo{
				IsPermalink: "true",
				Value:       dbItem.InReplyToURL,
			}
		}

		if dbItem.CtReplies > 0 {
			feedItem.Comments = &Comments{
				Count:   dbItem.CtReplies,
				FeedURL: getCommentsFeedURL(config.BaseURL, dbItem.Screenname, dbItem.ID),
			}
		}

		if sourceAttribution {
			feedItem.Source = &Source{
				URL:   dbItem.FeedURL,
				Title: dbItem.Author,
			}
		}

		feedItems = append(feedItems, feedItem)
	}

	return feedItems
}

// Helper functions for URLs

func getFeedURL(baseURL, screenname string) string {
	return fmt.Sprintf("%s/feed?screenname=%s", baseURL, url.QueryEscape(screenname))
}

func getEveryoneFeedURL(baseURL string) string {
	return fmt.Sprintf("%s/feed", baseURL)
}

func getCommentsFeedURL(baseURL, screenname string, itemID int64) string {
	return fmt.Sprintf("%s/comments/%s/%d.xml", baseURL, url.QueryEscape(screenname), itemID)
}

func formatEnclosureLength(length *int64) string {
	if length == nil || *length == 0 {
		return ""
	}
	return strconv.FormatInt(*length, 10)
}

// Link represents an RSS link element.
type Link struct {
	Rel  string `xml:"rel,attr,omitempty"`
	Type string `xml:"type,attr,omitempty"`
	Href string `xml:"href,attr"`
}

// JSONFeed represents an RSS feed in JSON format (mirrors RSS 2.0 structure).
type JSONFeed struct {
	Version string      `json:"version"`
	Channel JSONChannel `json:"channel"`
}

// JSONChannel represents the channel element in JSON format.
type JSONChannel struct {
	Title         string     `json:"title"`
	Link          string     `json:"link"`
	Description   string     `json:"description"`
	Language      string     `json:"language,omitempty"`
	LastBuildDate string     `json:"lastBuildDate,omitempty"`
	Items         []JSONItem `json:"item,omitempty"`
}

// JSONItem represents an item element in JSON format.
type JSONItem struct {
	Title       string `json:"title,omitempty"`
	Link        string `json:"link,omitempty"`
	Description string `json:"description,omitempty"`
	GUID        string `json:"guid,omitempty"`
	PubDate     string `json:"pubDate,omitempty"`
	Author      string `json:"author,omitempty"`
}

// BuildFeedForUserJSON generates a JSON feed for a user's posts.
func BuildFeedForUserJSON(conn *sql.DB, userScreenname string, baseURL string, config BuilderConfig) (string, error) {
	rssXML, err := BuildFeedForUser(conn, userScreenname, baseURL, config)
	if err != nil {
		return "", err
	}
	return convertRSSToJSON(rssXML)
}

// BuildFeedForEveryoneJSON generates a JSON feed of all posts on the network.
func BuildFeedForEveryoneJSON(conn *sql.DB, baseURL string, config BuilderConfig) (string, error) {
	rssXML, err := BuildFeedForEveryone(conn, baseURL, config)
	if err != nil {
		return "", err
	}
	return convertRSSToJSON(rssXML)
}

// BuildCommentsFeedJSON generates a JSON feed of replies to a post.
func BuildCommentsFeedJSON(conn *sql.DB, itemID int64, baseURL string, config BuilderConfig) (string, error) {
	rssXML, err := BuildCommentsFeed(conn, itemID, baseURL, config)
	if err != nil {
		return "", err
	}
	return convertRSSToJSON(rssXML)
}

// convertRSSToJSON converts an RSS XML string to JSON format.
func convertRSSToJSON(rssXML string) (string, error) {
	// Parse the RSS XML
	var rssFeed RSSFeed
	if err := xml.Unmarshal([]byte(rssXML), &rssFeed); err != nil {
		return "", fmt.Errorf("failed to parse RSS XML: %w", err)
	}

	if rssFeed.Channel == nil {
		return "", fmt.Errorf("RSS feed has no channel")
	}

	// Convert to JSON structure
	jsonFeed := JSONFeed{
		Version: rssFeed.Version,
		Channel: JSONChannel{
			Title:       rssFeed.Channel.Title,
			Link:        rssFeed.Channel.Link,
			Description: rssFeed.Channel.Description,
			Language:    rssFeed.Channel.Language,
		},
	}

	// Convert items
	if len(rssFeed.Channel.Items) > 0 {
		jsonFeed.Channel.Items = make([]JSONItem, len(rssFeed.Channel.Items))
		for i, item := range rssFeed.Channel.Items {
			if item != nil {
				guid := ""
				if item.Guid != nil {
					guid = item.Guid.Value
				}
				jsonFeed.Channel.Items[i] = JSONItem{
					Title:       item.Title,
					Link:        item.Link,
					Description: item.Description,
					GUID:        guid,
					PubDate:     item.PubDate,
				}
			}
		}
	}

	// Marshal to JSON with indentation
	jsonBytes, err := json.MarshalIndent(jsonFeed, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return string(jsonBytes), nil
}
