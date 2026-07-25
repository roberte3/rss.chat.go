package feed

import (
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestBuildSubscriptionList(t *testing.T) {
	// This is a minimal test that verifies the function can build OPML
	config := BuilderConfig{
		BaseURL:     "localhost:8081",
		ProductName: "rss.chat",
	}

	// Create a test database connection
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

	// Add a test user
	_, err = db.Exec(`insert into users (screenname, emailAddress, emailSecret, imageUrl, prefs, ctHits, ctHitsToday, whenLastHit, whenCreated, whenUpdated)
		values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"testuser", "test@example.com", "secret", "", "{}", 0, 0, time.Now(), time.Now(), time.Now())
	if err != nil {
		t.Fatalf("failed to insert test user: %v", err)
	}

	// Build subscription list
	opml, err := BuildSubscriptionList(db, config.BaseURL, config)
	if err != nil {
		t.Fatalf("failed to build subscription list: %v", err)
	}

	if opml == "" {
		t.Errorf("expected non-empty OPML, got empty string")
	}

	if !contains(opml, "testuser") {
		t.Errorf("expected OPML to contain 'testuser', got:\n%s", opml)
	}

	if !contains(opml, "localhost:8081") {
		t.Errorf("expected OPML to contain base URL")
	}
}

func TestBuildFeedForUser(t *testing.T) {
	config := BuilderConfig{
		BaseURL:      "localhost:8081",
		ProductName:  "rss.chat",
		MaxFeedItems: 100,
		Language:     "en",
		DocsURL:      "http://www.rssboard.org/rss-specification",
	}

	// Create a test database connection
	testDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer testDB.Close()

	// Initialize schema
	err = initTestDB(testDB)
	if err != nil {
		t.Fatalf("failed to init test db: %v", err)
	}

	feedURL := "http://localhost:8081/feed?screenname=dave"
	now := time.Now()

	// Add a test user
	_, err = testDB.Exec(`insert into users (screenname, emailAddress, emailSecret, imageUrl, prefs, ctHits, ctHitsToday, whenLastHit, whenCreated, whenUpdated)
		values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"dave", "dave@example.com", "secret", "", `{"myFeedTitle":"Dave's Blog"}`, 0, 0, now, now, now)
	if err != nil {
		t.Fatalf("failed to insert test user: %v", err)
	}

	// Add a test item for the user
	_, err = testDB.Exec(`insert into items (feedUrl, author, title, description, pubDate, whenCreated, whenUpdated)
		values (?, ?, ?, ?, ?, ?, ?)`,
		feedURL, "dave", "Test Post", "This is a test post", now, now, now)
	if err != nil {
		t.Fatalf("failed to insert test item: %v", err)
	}

	// Build feed for user
	rss, err := BuildFeedForUser(testDB, "dave", config.BaseURL, config)
	if err != nil {
		t.Fatalf("failed to build feed for user: %v", err)
	}

	if rss == "" {
		t.Errorf("expected non-empty RSS, got empty string")
	}

	if !contains(rss, "Dave") || !contains(rss, "Blog") {
		t.Errorf("expected RSS to contain feed title from prefs")
	}

	if !contains(rss, "<?xml") {
		t.Errorf("expected RSS to start with XML declaration")
	}

	if !contains(rss, "Test Post") {
		t.Errorf("expected RSS to contain item title")
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
