package publish

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/roberte3/rss.chat.go/db"
	"github.com/roberte3/rss.chat.go/feed"
	"github.com/roberte3/rss.chat.go/websub"
)

// Publisher handles writing feeds to disk or database.
type Publisher struct {
	baseDir     string
	config      feed.BuilderConfig
	FeedsDB     *sql.DB // Nil if filesystem mode
	StorageMode string  // "filesystem" or "database"
	pinger      *websub.Pinger
}

// NewPublisher creates a new feed publisher (filesystem mode by default).
func NewPublisher(baseDir string, config feed.BuilderConfig) *Publisher {
	return &Publisher{
		baseDir:     baseDir,
		config:      config,
		StorageMode: "filesystem",
		pinger:      websub.NewPinger("", false), // Disabled by default
	}
}

// SetDatabaseMode configures the publisher to store feeds in a database.
func (p *Publisher) SetDatabaseMode(feedsDB *sql.DB) {
	p.FeedsDB = feedsDB
	p.StorageMode = "database"
}

// SetPinger configures the WebSub pinger for this publisher.
func (p *Publisher) SetPinger(pinger *websub.Pinger) {
	if pinger != nil {
		p.pinger = pinger
	}
}

// BaseDir returns the directory feeds are written to in filesystem mode.
func (p *Publisher) BaseDir() string {
	return p.baseDir
}

// EnsureDir ensures the feeds directory structure exists.
func (p *Publisher) EnsureDir() error {
	dirs := []string{
		p.baseDir,
		filepath.Join(p.baseDir, "comments"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create directory %s: %w", dir, err)
		}
	}

	return nil
}

// PublishUserFeed generates and writes a user's feed to disk or database.
// File (filesystem): feeds/<screenname>/rss.xml
func (p *Publisher) PublishUserFeed(conn *sql.DB, screenname string) error {
	rss, err := feed.BuildFeedForUser(conn, screenname, p.config.BaseURL, p.config)
	if err != nil {
		return fmt.Errorf("build feed for user %s: %w", screenname, err)
	}

	if p.StorageMode == "database" {
		if err := db.StoreFeed(p.FeedsDB, "user", screenname, "text/xml", []byte(rss)); err != nil {
			return err
		}
	} else {
		userDir := filepath.Join(p.baseDir, screenname)
		if err := os.MkdirAll(userDir, 0755); err != nil {
			return fmt.Errorf("create user directory: %w", err)
		}

		path := filepath.Join(userDir, "rss.xml")
		if err := os.WriteFile(path, []byte(rss), 0644); err != nil {
			return fmt.Errorf("write feed file: %w", err)
		}
	}

	// Ping WebSub hub
	feedURL := p.config.BaseURL + "feed?screenname=" + screenname
	p.pinger.Ping(feedURL)

	return nil
}

// PublishEveryoneFeed generates and writes the network-wide feed to disk or database.
// File (filesystem): feeds/rss.xml
func (p *Publisher) PublishEveryoneFeed(conn *sql.DB) error {
	rss, err := feed.BuildFeedForEveryone(conn, p.config.BaseURL, p.config)
	if err != nil {
		return fmt.Errorf("build everyone feed: %w", err)
	}

	if p.StorageMode == "database" {
		if err := db.StoreFeed(p.FeedsDB, "global", "", "text/xml", []byte(rss)); err != nil {
			return err
		}
	} else {
		path := filepath.Join(p.baseDir, "rss.xml")
		if err := os.WriteFile(path, []byte(rss), 0644); err != nil {
			return fmt.Errorf("write everyone feed file: %w", err)
		}
	}

	// Ping WebSub hub
	feedURL := p.config.BaseURL + "feed"
	p.pinger.Ping(feedURL)

	return nil
}

// PublishCommentsFeed generates and writes a comments feed to disk.
// File: feeds/comments/<screenname>-<itemId>.xml
func (p *Publisher) PublishCommentsFeed(conn *sql.DB, screenname string, itemID int64) error {
	rss, err := feed.BuildCommentsFeed(conn, itemID, p.config.BaseURL, p.config)
	if err != nil {
		return fmt.Errorf("build comments feed for item %d: %w", itemID, err)
	}

	commentsDir := filepath.Join(p.baseDir, "comments")
	if err := os.MkdirAll(commentsDir, 0755); err != nil {
		return fmt.Errorf("create comments directory: %w", err)
	}

	filename := fmt.Sprintf("%s-%d.xml", screenname, itemID)
	path := filepath.Join(commentsDir, filename)
	if err := os.WriteFile(path, []byte(rss), 0644); err != nil {
		return fmt.Errorf("write comments feed file: %w", err)
	}

	// Ping WebSub hub for comments feed
	feedURL := fmt.Sprintf("%scomments/%s/%d.xml", p.config.BaseURL, screenname, itemID)
	p.pinger.Ping(feedURL)

	return nil
}

// PublishSubscriptionList generates and writes the subscription list to disk or database.
// File (filesystem): feeds/subs.opml
func (p *Publisher) PublishSubscriptionList(conn *sql.DB) error {
	opml, err := feed.BuildSubscriptionList(conn, p.config.BaseURL, p.config)
	if err != nil {
		return fmt.Errorf("build subscription list: %w", err)
	}

	if p.StorageMode == "database" {
		return db.StoreFeed(p.FeedsDB, "opml", "", "application/xml", []byte(opml))
	}

	path := filepath.Join(p.baseDir, "subs.opml")
	if err := os.WriteFile(path, []byte(opml), 0644); err != nil {
		return fmt.Errorf("write subscription list file: %w", err)
	}

	return nil
}

// UpdateFeedsOnPostWrite is called after a post is created, updated, or deleted.
// It republishes the author's feed and the everyone feed.
func (p *Publisher) UpdateFeedsOnPostWrite(conn *sql.DB, screenname string) error {
	if err := p.PublishUserFeed(conn, screenname); err != nil {
		return fmt.Errorf("publish user feed: %w", err)
	}

	if err := p.PublishEveryoneFeed(conn); err != nil {
		return fmt.Errorf("publish everyone feed: %w", err)
	}

	return nil
}

// UpdateFeedsOnReply is called after a reply is added, updated, or deleted.
// It republishes:
// - the comments feed for the parent post
// - the parent author's feed
// - recursively up the reply chain (one level up)
// - the everyone feed
func (p *Publisher) UpdateFeedsOnReply(conn *sql.DB, parentScreenname string, parentItemID int64) error {
	// Republish comments feed for the parent
	if err := p.PublishCommentsFeed(conn, parentScreenname, parentItemID); err != nil {
		return fmt.Errorf("publish comments feed for parent: %w", err)
	}

	// Republish parent author's feed
	if err := p.PublishUserFeed(conn, parentScreenname); err != nil {
		return fmt.Errorf("publish parent author feed: %w", err)
	}

	// Republish everyone feed
	if err := p.PublishEveryoneFeed(conn); err != nil {
		return fmt.Errorf("publish everyone feed: %w", err)
	}

	return nil
}

// UpdateFeedsOnLike is called when a like is toggled.
// Only the everyone feed needs republishing (like counts may be displayed there).
func (p *Publisher) UpdateFeedsOnLike(conn *sql.DB) error {
	return p.PublishEveryoneFeed(conn)
}

// BackfillCommentFeeds generates comments feeds for all existing threaded posts.
// This is a one-time operation, not called on every startup.
func (p *Publisher) BackfillCommentFeeds(conn *sql.DB) (int, error) {
	// Get all items that have replies
	query := `
		select distinct i.id, i.author
		from items i
		where exists (
			select 1 from items r where r.inReplyTo = i.id and (r.flDeleted is null or r.flDeleted = 0)
		) and (i.flDeleted is null or i.flDeleted = 0)
	`

	rows, err := conn.Query(query)
	if err != nil {
		return 0, fmt.Errorf("query items with replies: %w", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var itemID int64
		var screenname string
		if err := rows.Scan(&itemID, &screenname); err != nil {
			return count, fmt.Errorf("scan row: %w", err)
		}

		if err := p.PublishCommentsFeed(conn, screenname, itemID); err != nil {
			return count, fmt.Errorf("publish comments feed for item %d: %w", itemID, err)
		}

		count++
	}

	if err := rows.Err(); err != nil {
		return count, fmt.Errorf("rows error: %w", err)
	}

	return count, nil
}

// UserFeedExists reports whether a user's feed has already been published,
// looking wherever the current storage mode puts it.
func (p *Publisher) UserFeedExists(screenname string) (bool, error) {
	if p.StorageMode == "database" {
		if p.FeedsDB == nil {
			return false, nil
		}
		return db.FeedExists(p.FeedsDB, "user", screenname)
	}

	_, err := os.Stat(filepath.Join(p.baseDir, screenname, "rss.xml"))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("stat feed for %s: %w", screenname, err)
}

// BackfillMissingFeeds publishes a feed for every user who does not have one
// yet, so that a user is a feed from the day the account is created rather
// than from their first post. Called on startup in both storage modes — the
// filesystem is the default, and skipping it there was leaving every
// pre-existing account with a 404 feed.
//
// Users whose feed already exists are left alone, matching
// backfillMissingFeeds in rssnetwork.js; the everyone feed and the
// subscription list are always regenerated, as upstream does alongside it.
// The count returned is the number of user feeds filled in.
func (p *Publisher) BackfillMissingFeeds(conn *sql.DB) (int, error) {
	count := 0

	// Always generate global feed and OPML
	if err := p.PublishEveryoneFeed(conn); err != nil {
		return count, fmt.Errorf("publish everyone feed: %w", err)
	}

	if err := p.PublishSubscriptionList(conn); err != nil {
		return count, fmt.Errorf("publish subscription list: %w", err)
	}

	// Get all user screennames
	query := `SELECT screenname FROM users WHERE screenname IS NOT NULL ORDER BY screenname`
	rows, err := conn.Query(query)
	if err != nil {
		return count, fmt.Errorf("query users: %w", err)
	}
	defer rows.Close()

	var screennames []string
	for rows.Next() {
		var screenname string
		if err := rows.Scan(&screenname); err != nil {
			return count, fmt.Errorf("scan screenname: %w", err)
		}
		screennames = append(screennames, screenname)
	}

	if err := rows.Err(); err != nil {
		return count, fmt.Errorf("rows error: %w", err)
	}

	// Publish a feed for anyone missing one, leaving existing feeds untouched
	for _, screenname := range screennames {
		exists, err := p.UserFeedExists(screenname)
		if err != nil {
			return count, err
		}
		if exists {
			continue
		}
		if err := p.PublishUserFeed(conn, screenname); err != nil {
			return count, fmt.Errorf("publish user feed for %s: %w", screenname, err)
		}
		count++
	}

	return count, nil
}
