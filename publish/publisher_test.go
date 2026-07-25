package publish

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"rss.chat.go/feed"

	_ "modernc.org/sqlite"
)

func TestPublishUserFeed(t *testing.T) {
	// Create temporary directory for feeds
	tmpDir, err := os.MkdirTemp("", "feeds")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test database
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	// Initialize schema
	err = initTestDB(db)
	if err != nil {
		t.Fatalf("failed to init test db: %v", err)
	}

	// Add test user
	now := time.Now()
	_, err = db.Exec(`insert into users (screenname, emailAddress, emailSecret, imageUrl, prefs, ctHits, ctHitsToday, whenLastHit, whenCreated, whenUpdated)
		values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"alice", "alice@example.com", "secret", "", `{}`, 0, 0, now, now, now)
	if err != nil {
		t.Fatalf("failed to insert user: %v", err)
	}

	// Add test item
	feedURL := "http://localhost:8081/feed?screenname=alice"
	_, err = db.Exec(`insert into items (feedUrl, author, title, description, pubDate, whenCreated, whenUpdated)
		values (?, ?, ?, ?, ?, ?, ?)`,
		feedURL, "alice", "Hello World", "My first post", now, now, now)
	if err != nil {
		t.Fatalf("failed to insert item: %v", err)
	}

	// Create publisher
	config := feed.BuilderConfig{
		BaseURL:      "localhost:8081",
		ProductName:  "rss.chat",
		MaxFeedItems: 100,
		Language:     "en",
		DocsURL:      "http://www.rssboard.org/rss-specification",
	}
	pub := NewPublisher(tmpDir, config)

	// Publish user feed
	err = pub.PublishUserFeed(db, "alice")
	if err != nil {
		t.Fatalf("failed to publish user feed: %v", err)
	}

	// Verify file exists
	feedPath := filepath.Join(tmpDir, "alice", "rss.xml")
	if _, err := os.Stat(feedPath); err != nil {
		t.Fatalf("feed file not created: %v", err)
	}

	// Verify file contains expected content
	content, err := os.ReadFile(feedPath)
	if err != nil {
		t.Fatalf("failed to read feed file: %v", err)
	}

	if len(content) == 0 {
		t.Errorf("feed file is empty")
	}

	if !contains(string(content), "Hello World") {
		t.Errorf("feed does not contain item title")
	}
}

func TestPublishEveryoneFeed(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "feeds")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	err = initTestDB(db)
	if err != nil {
		t.Fatalf("failed to init test db: %v", err)
	}

	now := time.Now()
	_, err = db.Exec(`insert into users (screenname, emailAddress, emailSecret, imageUrl, prefs, ctHits, ctHitsToday, whenLastHit, whenCreated, whenUpdated)
		values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"bob", "bob@example.com", "secret", "", `{}`, 0, 0, now, now, now)
	if err != nil {
		t.Fatalf("failed to insert user: %v", err)
	}

	feedURL := "http://localhost:8081/feed?screenname=bob"
	_, err = db.Exec(`insert into items (feedUrl, author, title, description, pubDate, whenCreated, whenUpdated)
		values (?, ?, ?, ?, ?, ?, ?)`,
		feedURL, "bob", "Network Post", "Posted by bob", now, now, now)
	if err != nil {
		t.Fatalf("failed to insert item: %v", err)
	}

	config := feed.BuilderConfig{
		BaseURL:      "localhost:8081",
		ProductName:  "rss.chat",
		MaxFeedItems: 100,
		Language:     "en",
		DocsURL:      "http://www.rssboard.org/rss-specification",
	}
	pub := NewPublisher(tmpDir, config)

	err = pub.PublishEveryoneFeed(db)
	if err != nil {
		t.Fatalf("failed to publish everyone feed: %v", err)
	}

	feedPath := filepath.Join(tmpDir, "rss.xml")
	if _, err := os.Stat(feedPath); err != nil {
		t.Fatalf("everyone feed file not created: %v", err)
	}

	content, err := os.ReadFile(feedPath)
	if err != nil {
		t.Fatalf("failed to read everyone feed: %v", err)
	}

	if !contains(string(content), "Network Post") {
		t.Errorf("everyone feed does not contain item")
	}
}

func TestPublishSubscriptionList(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "feeds")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	err = initTestDB(db)
	if err != nil {
		t.Fatalf("failed to init test db: %v", err)
	}

	now := time.Now()
	_, err = db.Exec(`insert into users (screenname, emailAddress, emailSecret, imageUrl, prefs, ctHits, ctHitsToday, whenLastHit, whenCreated, whenUpdated)
		values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"charlie", "charlie@example.com", "secret", "", `{}`, 0, 0, now, now, now)
	if err != nil {
		t.Fatalf("failed to insert user: %v", err)
	}

	config := feed.BuilderConfig{
		BaseURL:      "localhost:8081",
		ProductName:  "rss.chat",
		MaxFeedItems: 100,
		Language:     "en",
		DocsURL:      "http://www.rssboard.org/rss-specification",
	}
	pub := NewPublisher(tmpDir, config)

	err = pub.PublishSubscriptionList(db)
	if err != nil {
		t.Fatalf("failed to publish subscription list: %v", err)
	}

	opmlPath := filepath.Join(tmpDir, "subs.opml")
	if _, err := os.Stat(opmlPath); err != nil {
		t.Fatalf("subscription list file not created: %v", err)
	}

	content, err := os.ReadFile(opmlPath)
	if err != nil {
		t.Fatalf("failed to read subscription list: %v", err)
	}

	if !contains(string(content), "charlie") {
		t.Errorf("subscription list does not contain user")
	}
}

func initTestDB(db *sql.DB) error {
	schema := `
	create table users (
		screenname text primary key,
		emailAddress text unique,
		emailSecret text,
		imageUrl text,
		prefs text,
		ctHits integer default 0,
		ctHitsToday integer default 0,
		whenLastHit datetime,
		whenCreated datetime,
		whenUpdated datetime
	);

	create table items (
		id integer primary key autoincrement,
		feedUrl text,
		author text,
		inReplyTo integer,
		title text,
		link text,
		description text,
		pubDate datetime,
		enclosureUrl text,
		enclosureType text,
		enclosureLength integer,
		whenCreated datetime,
		whenUpdated datetime,
		markdowntext text,
		outlineJsontext text,
		flDeleted integer default 0
	);

	create table likes (
		screenname text,
		itemId integer,
		whenCreated datetime,
		primary key (screenname, itemId),
		foreign key (itemId) references items(id)
	);
	`

	_, err := db.Exec(schema)
	return err
}

func contains(s, substr string) bool {
	for i := 0; i < len(s)-len(substr)+1; i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
