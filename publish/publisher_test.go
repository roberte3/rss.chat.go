package publish

import (
	"database/sql"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/roberte3/rss.chat.go/db"
	"github.com/roberte3/rss.chat.go/feed"
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
		BaseURL:      "http://localhost:8081",
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
		BaseURL:      "http://localhost:8081",
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
		BaseURL:      "http://localhost:8081",
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

// Database mode tests

func TestPublishUserFeedDatabase(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "feeds_db_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create main database
	mainDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to create main db: %v", err)
	}
	defer mainDB.Close()

	if err := initTestDB(mainDB); err != nil {
		t.Fatalf("failed to init main db: %v", err)
	}

	// Create feeds database
	feedsDBPath := filepath.Join(tmpDir, "test_feeds.db")
	feedsDB, err := db.OpenFeedsDB(feedsDBPath)
	if err != nil {
		t.Fatalf("failed to create feeds db: %v", err)
	}
	defer feedsDB.Close()

	// Add test user and item
	now := time.Now()
	_, err = mainDB.Exec(`insert into users (screenname, emailAddress, emailSecret, imageUrl, prefs, ctHits, ctHitsToday, whenLastHit, whenCreated, whenUpdated)
		values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"alice", "alice@example.com", "secret", "", `{}`, 0, 0, now, now, now)
	if err != nil {
		t.Fatalf("failed to insert user: %v", err)
	}

	feedURL := "http://localhost:8081/feed?screenname=alice"
	_, err = mainDB.Exec(`insert into items (feedUrl, author, title, description, pubDate, whenCreated, whenUpdated)
		values (?, ?, ?, ?, ?, ?, ?)`,
		feedURL, "alice", "Hello World", "My first post", now, now, now)
	if err != nil {
		t.Fatalf("failed to insert item: %v", err)
	}

	// Create publisher in database mode
	config := feed.BuilderConfig{
		BaseURL:      "http://localhost:8081",
		ProductName:  "rss.chat",
		MaxFeedItems: 100,
		Language:     "en",
		DocsURL:      "http://www.rssboard.org/rss-specification",
	}
	pub := NewPublisher(tmpDir, config)
	pub.SetDatabaseMode(feedsDB)

	// Publish user feed
	err = pub.PublishUserFeed(mainDB, "alice")
	if err != nil {
		t.Fatalf("failed to publish user feed: %v", err)
	}

	// Verify feed is in database
	content, err := db.GetFeed(feedsDB, "user", "alice")
	if err != nil {
		t.Fatalf("failed to get feed from database: %v", err)
	}

	if len(content) == 0 {
		t.Errorf("feed content is empty")
	}

	if !contains(string(content), "Hello World") {
		t.Errorf("feed does not contain item title")
	}
}

func TestPublishEveryoneFeedDatabase(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "feeds_db_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mainDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to create main db: %v", err)
	}
	defer mainDB.Close()

	if err := initTestDB(mainDB); err != nil {
		t.Fatalf("failed to init main db: %v", err)
	}

	feedsDBPath := filepath.Join(tmpDir, "test_feeds.db")
	feedsDB, err := db.OpenFeedsDB(feedsDBPath)
	if err != nil {
		t.Fatalf("failed to create feeds db: %v", err)
	}
	defer feedsDB.Close()

	now := time.Now()
	_, err = mainDB.Exec(`insert into users (screenname, emailAddress, emailSecret, imageUrl, prefs, ctHits, ctHitsToday, whenLastHit, whenCreated, whenUpdated)
		values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"bob", "bob@example.com", "secret", "", `{}`, 0, 0, now, now, now)
	if err != nil {
		t.Fatalf("failed to insert user: %v", err)
	}

	feedURL := "http://localhost:8081/feed?screenname=bob"
	_, err = mainDB.Exec(`insert into items (feedUrl, author, title, description, pubDate, whenCreated, whenUpdated)
		values (?, ?, ?, ?, ?, ?, ?)`,
		feedURL, "bob", "Network Post", "Posted by bob", now, now, now)
	if err != nil {
		t.Fatalf("failed to insert item: %v", err)
	}

	config := feed.BuilderConfig{
		BaseURL:      "http://localhost:8081",
		ProductName:  "rss.chat",
		MaxFeedItems: 100,
		Language:     "en",
		DocsURL:      "http://www.rssboard.org/rss-specification",
	}
	pub := NewPublisher(tmpDir, config)
	pub.SetDatabaseMode(feedsDB)

	err = pub.PublishEveryoneFeed(mainDB)
	if err != nil {
		t.Fatalf("failed to publish everyone feed: %v", err)
	}

	content, err := db.GetFeed(feedsDB, "global", "")
	if err != nil {
		t.Fatalf("failed to get global feed: %v", err)
	}

	if !contains(string(content), "Network Post") {
		t.Errorf("global feed does not contain item")
	}
}

func TestPublishSubscriptionListDatabase(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "feeds_db_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mainDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to create main db: %v", err)
	}
	defer mainDB.Close()

	if err := initTestDB(mainDB); err != nil {
		t.Fatalf("failed to init main db: %v", err)
	}

	feedsDBPath := filepath.Join(tmpDir, "test_feeds.db")
	feedsDB, err := db.OpenFeedsDB(feedsDBPath)
	if err != nil {
		t.Fatalf("failed to create feeds db: %v", err)
	}
	defer feedsDB.Close()

	now := time.Now()
	_, err = mainDB.Exec(`insert into users (screenname, emailAddress, emailSecret, imageUrl, prefs, ctHits, ctHitsToday, whenLastHit, whenCreated, whenUpdated)
		values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"charlie", "charlie@example.com", "secret", "", `{}`, 0, 0, now, now, now)
	if err != nil {
		t.Fatalf("failed to insert user: %v", err)
	}

	config := feed.BuilderConfig{
		BaseURL:      "http://localhost:8081",
		ProductName:  "rss.chat",
		MaxFeedItems: 100,
		Language:     "en",
		DocsURL:      "http://www.rssboard.org/rss-specification",
	}
	pub := NewPublisher(tmpDir, config)
	pub.SetDatabaseMode(feedsDB)

	err = pub.PublishSubscriptionList(mainDB)
	if err != nil {
		t.Fatalf("failed to publish subscription list: %v", err)
	}

	content, err := db.GetFeed(feedsDB, "opml", "")
	if err != nil {
		t.Fatalf("failed to get OPML from database: %v", err)
	}

	if !contains(string(content), "charlie") {
		t.Errorf("OPML does not contain user")
	}
}

func TestBackfillMissingFeeds(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "feeds_db_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mainDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to create main db: %v", err)
	}
	defer mainDB.Close()

	if err := initTestDB(mainDB); err != nil {
		t.Fatalf("failed to init main db: %v", err)
	}

	feedsDBPath := filepath.Join(tmpDir, "test_feeds.db")
	feedsDB, err := db.OpenFeedsDB(feedsDBPath)
	if err != nil {
		t.Fatalf("failed to create feeds db: %v", err)
	}
	defer feedsDB.Close()

	// Add multiple users
	now := time.Now()
	for _, name := range []string{"alice", "bob", "charlie"} {
		_, err = mainDB.Exec(`insert into users (screenname, emailAddress, emailSecret, imageUrl, prefs, ctHits, ctHitsToday, whenLastHit, whenCreated, whenUpdated)
			values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			name, name+"@example.com", "secret", "", `{}`, 0, 0, now, now, now)
		if err != nil {
			t.Fatalf("failed to insert user %s: %v", name, err)
		}
	}

	config := feed.BuilderConfig{
		BaseURL:      "http://localhost:8081",
		ProductName:  "rss.chat",
		MaxFeedItems: 100,
		Language:     "en",
		DocsURL:      "http://www.rssboard.org/rss-specification",
	}
	pub := NewPublisher(tmpDir, config)
	pub.SetDatabaseMode(feedsDB)

	// None of the three users has a feed yet, so all three get filled in.
	// The count is user feeds only; global and OPML are always regenerated.
	count, err := pub.BackfillMissingFeeds(mainDB)
	if err != nil {
		t.Fatalf("failed to backfill feeds: %v", err)
	}

	if count != 3 {
		t.Errorf("expected 3 user feeds, got %d", count)
	}

	// Running again must be a no-op: feeds that already exist are left alone,
	// rather than every user feed being rebuilt on every startup.
	count, err = pub.BackfillMissingFeeds(mainDB)
	if err != nil {
		t.Fatalf("failed to re-run backfill: %v", err)
	}

	if count != 0 {
		t.Errorf("expected 0 user feeds on second run, got %d", count)
	}

	// Verify all feeds exist
	for _, name := range []string{"alice", "bob", "charlie"} {
		content, err := db.GetFeed(feedsDB, "user", name)
		if err != nil {
			t.Fatalf("failed to get feed for %s: %v", name, err)
		}
		if len(content) == 0 {
			t.Errorf("feed for %s is empty", name)
		}
	}

	// Verify global and OPML
	globalContent, err := db.GetFeed(feedsDB, "global", "")
	if err != nil {
		t.Fatalf("failed to get global feed: %v", err)
	}
	if len(globalContent) == 0 {
		t.Error("global feed is empty")
	}

	opmlContent, err := db.GetFeed(feedsDB, "opml", "")
	if err != nil {
		t.Fatalf("failed to get OPML: %v", err)
	}
	if len(opmlContent) == 0 {
		t.Error("OPML is empty")
	}
}

func TestBothModesSameContent(t *testing.T) {
	tmpFSDir, err := os.MkdirTemp("", "feeds_fs")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpFSDir)

	tmpDBDir, err := os.MkdirTemp("", "feeds_db")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDBDir)

	// Create main database
	mainDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to create main db: %v", err)
	}
	defer mainDB.Close()

	if err := initTestDB(mainDB); err != nil {
		t.Fatalf("failed to init main db: %v", err)
	}

	// Add test data
	now := time.Now()
	_, err = mainDB.Exec(`insert into users (screenname, emailAddress, emailSecret, imageUrl, prefs, ctHits, ctHitsToday, whenLastHit, whenCreated, whenUpdated)
		values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"alice", "alice@example.com", "secret", "", `{}`, 0, 0, now, now, now)
	if err != nil {
		t.Fatalf("failed to insert user: %v", err)
	}

	feedURL := "http://localhost:8081/feed?screenname=alice"
	_, err = mainDB.Exec(`insert into items (feedUrl, author, title, description, pubDate, whenCreated, whenUpdated)
		values (?, ?, ?, ?, ?, ?, ?)`,
		feedURL, "alice", "Test Post", "Test content", now, now, now)
	if err != nil {
		t.Fatalf("failed to insert item: %v", err)
	}

	config := feed.BuilderConfig{
		BaseURL:      "http://localhost:8081",
		ProductName:  "rss.chat",
		MaxFeedItems: 100,
		Language:     "en",
		DocsURL:      "http://www.rssboard.org/rss-specification",
	}

	// Filesystem mode
	pubFS := NewPublisher(tmpFSDir, config)
	if err := pubFS.EnsureDir(); err != nil {
		t.Fatalf("failed to ensure dir: %v", err)
	}
	if err := pubFS.PublishUserFeed(mainDB, "alice"); err != nil {
		t.Fatalf("failed to publish user feed (fs): %v", err)
	}

	fsContent, err := os.ReadFile(filepath.Join(tmpFSDir, "alice", "rss.xml"))
	if err != nil {
		t.Fatalf("failed to read filesystem feed: %v", err)
	}

	// Database mode
	feedsDBPath := filepath.Join(tmpDBDir, "test_feeds.db")
	feedsDB, err := db.OpenFeedsDB(feedsDBPath)
	if err != nil {
		t.Fatalf("failed to create feeds db: %v", err)
	}
	defer feedsDB.Close()

	pubDB := NewPublisher(tmpDBDir, config)
	pubDB.SetDatabaseMode(feedsDB)
	if err := pubDB.PublishUserFeed(mainDB, "alice"); err != nil {
		t.Fatalf("failed to publish user feed (db): %v", err)
	}

	dbContent, err := db.GetFeed(feedsDB, "user", "alice")
	if err != nil {
		t.Fatalf("failed to get feed from database: %v", err)
	}

	// Both should be identical
	if string(fsContent) != string(dbContent) {
		t.Error("filesystem and database feed content differs")
	}
}

// TestBackfillMissingFeedsFilesystemMode covers the default storage mode.
// Backfill used to return early unless the publisher was in database mode, so
// on a filesystem server every account that had not yet posted kept serving a
// 404 for its feed, no matter how many times the server restarted.
func TestBackfillMissingFeedsFilesystemMode(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "feeds_fs_backfill")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mainDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to create main db: %v", err)
	}
	defer mainDB.Close()

	if err := initTestDB(mainDB); err != nil {
		t.Fatalf("failed to init main db: %v", err)
	}

	now := time.Now()
	for _, name := range []string{"alice", "bob"} {
		_, err = mainDB.Exec(`insert into users (screenname, emailAddress, emailSecret, imageUrl, prefs, ctHits, ctHitsToday, whenLastHit, whenCreated, whenUpdated)
			values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			name, name+"@example.com", "secret", "", `{}`, 0, 0, now, now, now)
		if err != nil {
			t.Fatalf("failed to insert user %s: %v", name, err)
		}
	}

	config := feed.BuilderConfig{
		BaseURL:      "http://localhost:8081",
		ProductName:  "rss.chat",
		MaxFeedItems: 100,
		Language:     "en",
		DocsURL:      "http://www.rssboard.org/rss-specification",
	}
	pub := NewPublisher(tmpDir, config) // filesystem mode, the default
	if err := pub.EnsureDir(); err != nil {
		t.Fatalf("failed to ensure dirs: %v", err)
	}

	count, err := pub.BackfillMissingFeeds(mainDB)
	if err != nil {
		t.Fatalf("failed to backfill feeds: %v", err)
	}

	if count != 2 {
		t.Errorf("expected 2 user feeds, got %d", count)
	}

	// Both users must now have a feed file on disk, with real content in it.
	for _, name := range []string{"alice", "bob"} {
		path := filepath.Join(tmpDir, name, "rss.xml")
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("no feed on disk for %s: %v", name, err)
		}
		if !contains(string(content), "<rss") {
			t.Errorf("feed for %s is not an RSS document: %q", name, string(content))
		}
	}

	// Second run leaves the existing files alone.
	count, err = pub.BackfillMissingFeeds(mainDB)
	if err != nil {
		t.Fatalf("failed to re-run backfill: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 user feeds on second run, got %d", count)
	}
}
