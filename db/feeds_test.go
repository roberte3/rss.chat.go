package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenFeedsDB(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "feeds_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test_feeds.db")
	db, err := OpenFeedsDB(dbPath)
	if err != nil {
		t.Fatalf("failed to open feeds database: %v", err)
	}
	defer db.Close()

	// Verify file was created
	if _, err := os.Stat(dbPath); err != nil {
		t.Errorf("database file not created: %v", err)
	}

	// Verify we can query the schema
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM feeds").Scan(&count)
	if err != nil {
		t.Errorf("failed to query feeds table: %v", err)
	}
}

func TestStoreFeed(t *testing.T) {
	db := setupFeedsDB(t)
	defer db.Close()

	tests := []struct {
		name        string
		feedType    string
		screenname  string
		contentType string
		content     string
	}{
		{
			name:        "store global feed",
			feedType:    "global",
			screenname:  "",
			contentType: "text/xml",
			content:     `<?xml version="1.0"?><rss><channel>Global</channel></rss>`,
		},
		{
			name:        "store user feed",
			feedType:    "user",
			screenname:  "alice",
			contentType: "text/xml",
			content:     `<?xml version="1.0"?><rss><channel>Alice's Feed</channel></rss>`,
		},
		{
			name:        "store opml",
			feedType:    "opml",
			screenname:  "",
			contentType: "application/xml",
			content:     `<?xml version="1.0"?><opml><body><outline/></body></opml>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := StoreFeed(db, tt.feedType, tt.screenname, tt.contentType, []byte(tt.content))
			if err != nil {
				t.Errorf("StoreFeed failed: %v", err)
			}

			// Verify feed was stored
			retrieved, err := GetFeed(db, tt.feedType, tt.screenname)
			if err != nil {
				t.Errorf("GetFeed failed: %v", err)
			}
			if string(retrieved) != tt.content {
				t.Errorf("content mismatch: got %q, want %q", string(retrieved), tt.content)
			}
		})
	}
}

func TestStoreFeedUpdate(t *testing.T) {
	db := setupFeedsDB(t)
	defer db.Close()

	feedType := "user"
	screenname := "bob"
	contentType := "text/xml"
	oldContent := `<?xml version="1.0"?><rss><channel>Old</channel></rss>`
	newContent := `<?xml version="1.0"?><rss><channel>New</channel></rss>`

	// Store initial feed
	err := StoreFeed(db, feedType, screenname, contentType, []byte(oldContent))
	if err != nil {
		t.Fatalf("StoreFeed (initial) failed: %v", err)
	}

	// Update the feed
	err = StoreFeed(db, feedType, screenname, contentType, []byte(newContent))
	if err != nil {
		t.Fatalf("StoreFeed (update) failed: %v", err)
	}

	// Verify new content
	retrieved, err := GetFeed(db, feedType, screenname)
	if err != nil {
		t.Fatalf("GetFeed failed: %v", err)
	}

	if string(retrieved) != newContent {
		t.Errorf("content not updated: got %q, want %q", string(retrieved), newContent)
	}

	// Verify only one feed exists
	record, err := GetFeedRecord(db, feedType, screenname)
	if err != nil {
		t.Fatalf("GetFeedRecord failed: %v", err)
	}
	if record == nil {
		t.Fatal("feed record not found")
	}
}

func TestGetFeedNotFound(t *testing.T) {
	db := setupFeedsDB(t)
	defer db.Close()

	// Try to get non-existent feed
	content, err := GetFeed(db, "user", "nonexistent")
	if err != nil {
		t.Errorf("GetFeed should not error for missing feed: %v", err)
	}
	if content != nil {
		t.Errorf("GetFeed should return nil for missing feed, got %q", string(content))
	}
}

func TestGetFeedRecord(t *testing.T) {
	db := setupFeedsDB(t)
	defer db.Close()

	feedType := "user"
	screenname := "charlie"
	contentType := "text/xml"
	content := `<?xml version="1.0"?><rss><channel>Charlie</channel></rss>`

	// Store feed
	err := StoreFeed(db, feedType, screenname, contentType, []byte(content))
	if err != nil {
		t.Fatalf("StoreFeed failed: %v", err)
	}

	// Retrieve record
	record, err := GetFeedRecord(db, feedType, screenname)
	if err != nil {
		t.Fatalf("GetFeedRecord failed: %v", err)
	}

	if record == nil {
		t.Fatal("feed record is nil")
	}

	if record.FeedType != feedType {
		t.Errorf("feedType mismatch: got %q, want %q", record.FeedType, feedType)
	}

	if record.Screenname == nil || *record.Screenname != screenname {
		t.Errorf("screenname mismatch: got %v, want %q", record.Screenname, screenname)
	}

	if record.ContentType != contentType {
		t.Errorf("contentType mismatch: got %q, want %q", record.ContentType, contentType)
	}

	if string(record.Content) != content {
		t.Errorf("content mismatch: got %q, want %q", string(record.Content), content)
	}

	if record.ID == 0 {
		t.Error("record ID should be set")
	}
}

func TestDeleteFeed(t *testing.T) {
	db := setupFeedsDB(t)
	defer db.Close()

	feedType := "user"
	screenname := "dave"
	contentType := "text/xml"
	content := `<?xml version="1.0"?><rss><channel>Dave</channel></rss>`

	// Store feed
	err := StoreFeed(db, feedType, screenname, contentType, []byte(content))
	if err != nil {
		t.Fatalf("StoreFeed failed: %v", err)
	}

	// Verify it exists
	retrieved, err := GetFeed(db, feedType, screenname)
	if err != nil || retrieved == nil {
		t.Fatalf("GetFeed failed: %v", err)
	}

	// Delete it
	err = DeleteFeed(db, feedType, screenname)
	if err != nil {
		t.Fatalf("DeleteFeed failed: %v", err)
	}

	// Verify it's gone
	retrieved, err = GetFeed(db, feedType, screenname)
	if err != nil {
		t.Fatalf("GetFeed after delete failed: %v", err)
	}
	if retrieved != nil {
		t.Error("feed should be deleted")
	}
}

func TestDeleteFeedNotFound(t *testing.T) {
	db := setupFeedsDB(t)
	defer db.Close()

	// Try to delete non-existent feed
	err := DeleteFeed(db, "user", "nonexistent")
	if err == nil {
		t.Error("DeleteFeed should error for missing feed")
	}
}

func TestListFeeds(t *testing.T) {
	db := setupFeedsDB(t)
	defer db.Close()

	// Store multiple feeds
	feeds := []struct {
		feedType    string
		screenname  string
		contentType string
		content     string
	}{
		{"global", "", "text/xml", "global"},
		{"user", "alice", "text/xml", "alice"},
		{"user", "bob", "text/xml", "bob"},
		{"opml", "", "application/xml", "opml"},
	}

	for _, f := range feeds {
		err := StoreFeed(db, f.feedType, f.screenname, f.contentType, []byte(f.content))
		if err != nil {
			t.Fatalf("StoreFeed failed: %v", err)
		}
	}

	// List all feeds
	list, err := ListFeeds(db)
	if err != nil {
		t.Fatalf("ListFeeds failed: %v", err)
	}

	if len(list) != len(feeds) {
		t.Errorf("feed count mismatch: got %d, want %d", len(list), len(feeds))
	}

	// Verify all feeds are present (create map for order-independent verification)
	foundFeeds := make(map[string]bool)
	for _, record := range list {
		key := record.FeedType
		if record.Screenname != nil {
			key += ":" + *record.Screenname
		}
		foundFeeds[key] = true
	}

	for _, f := range feeds {
		key := f.feedType
		if f.screenname != "" {
			key += ":" + f.screenname
		}
		if !foundFeeds[key] {
			t.Errorf("feed not found: %s", key)
		}
	}
}

func TestListUserFeeds(t *testing.T) {
	db := setupFeedsDB(t)
	defer db.Close()

	// Store feeds for multiple users
	err := StoreFeed(db, "user", "alice", "text/xml", []byte("alice-feed"))
	if err != nil {
		t.Fatalf("StoreFeed failed: %v", err)
	}

	err = StoreFeed(db, "user", "bob", "text/xml", []byte("bob-feed"))
	if err != nil {
		t.Fatalf("StoreFeed failed: %v", err)
	}

	// List feeds for alice
	list, err := ListUserFeeds(db, "alice")
	if err != nil {
		t.Fatalf("ListUserFeeds failed: %v", err)
	}

	if len(list) != 1 {
		t.Errorf("expected 1 feed for alice, got %d", len(list))
	}

	if list[0].Screenname == nil || *list[0].Screenname != "alice" {
		t.Errorf("screenname mismatch: got %v, want alice", list[0].Screenname)
	}
}

func TestListUserFeedsCaseInsensitive(t *testing.T) {
	db := setupFeedsDB(t)
	defer db.Close()

	// Store feed with lowercase screenname
	err := StoreFeed(db, "user", "alice", "text/xml", []byte("alice-feed"))
	if err != nil {
		t.Fatalf("StoreFeed failed: %v", err)
	}

	// Query with uppercase
	list, err := ListUserFeeds(db, "ALICE")
	if err != nil {
		t.Fatalf("ListUserFeeds failed: %v", err)
	}

	if len(list) != 1 {
		t.Errorf("case-insensitive lookup failed: expected 1 feed, got %d", len(list))
	}
}

func TestFeedExists(t *testing.T) {
	db := setupFeedsDB(t)
	defer db.Close()

	feedType := "user"
	screenname := "eve"

	// Feed shouldn't exist yet
	exists, err := FeedExists(db, feedType, screenname)
	if err != nil {
		t.Fatalf("FeedExists failed: %v", err)
	}
	if exists {
		t.Error("feed should not exist yet")
	}

	// Store feed
	err = StoreFeed(db, feedType, screenname, "text/xml", []byte("eve-feed"))
	if err != nil {
		t.Fatalf("StoreFeed failed: %v", err)
	}

	// Now it should exist
	exists, err = FeedExists(db, feedType, screenname)
	if err != nil {
		t.Fatalf("FeedExists failed: %v", err)
	}
	if !exists {
		t.Error("feed should exist")
	}
}

func TestGlobalFeedNormalization(t *testing.T) {
	db := setupFeedsDB(t)
	defer db.Close()

	tests := []struct {
		name       string
		screenname string
	}{
		{"empty string", ""},
		{"global keyword", "global"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := StoreFeed(db, "global", tt.screenname, "text/xml", []byte("global-feed"))
			if err != nil {
				t.Fatalf("StoreFeed failed: %v", err)
			}

			// Both should retrieve the same feed (because screenname is normalized to NULL)
			for _, queryName := range []string{"", "global"} {
				content, err := GetFeed(db, "global", queryName)
				if err != nil {
					t.Errorf("GetFeed(%q) failed: %v", queryName, err)
				}
				if string(content) != "global-feed" {
					t.Errorf("GetFeed(%q) returned wrong content", queryName)
				}
			}
		})
	}
}

func TestUniqueConstraint(t *testing.T) {
	db := setupFeedsDB(t)
	defer db.Close()

	feedType := "user"
	screenname := "frank"
	contentType := "text/xml"

	// Store first feed
	err := StoreFeed(db, feedType, screenname, contentType, []byte("content1"))
	if err != nil {
		t.Fatalf("StoreFeed (first) failed: %v", err)
	}

	// Store second feed with same type/screenname (should update, not error)
	err = StoreFeed(db, feedType, screenname, contentType, []byte("content2"))
	if err != nil {
		t.Fatalf("StoreFeed (second) failed: %v", err)
	}

	// Verify only one feed exists with the new content
	content, err := GetFeed(db, feedType, screenname)
	if err != nil {
		t.Fatalf("GetFeed failed: %v", err)
	}

	if string(content) != "content2" {
		t.Errorf("expected second content, got %q", string(content))
	}

	// Verify record count
	list, err := ListFeeds(db)
	if err != nil {
		t.Fatalf("ListFeeds failed: %v", err)
	}

	if len(list) != 1 {
		t.Errorf("expected 1 feed, got %d", len(list))
	}
}

func TestLargeContentHandling(t *testing.T) {
	db := setupFeedsDB(t)
	defer db.Close()

	feedType := "user"
	screenname := "grace"
	contentType := "text/xml"

	// Create large content (1MB)
	largeContent := make([]byte, 1024*1024)
	for i := range largeContent {
		largeContent[i] = byte(i % 256)
	}

	err := StoreFeed(db, feedType, screenname, contentType, largeContent)
	if err != nil {
		t.Fatalf("StoreFeed failed: %v", err)
	}

	retrieved, err := GetFeed(db, feedType, screenname)
	if err != nil {
		t.Fatalf("GetFeed failed: %v", err)
	}

	if len(retrieved) != len(largeContent) {
		t.Errorf("content size mismatch: got %d bytes, want %d bytes", len(retrieved), len(largeContent))
	}

	// Verify first and last bytes match
	if retrieved[0] != largeContent[0] || retrieved[len(retrieved)-1] != largeContent[len(largeContent)-1] {
		t.Error("large content corrupted")
	}
}

// Helper function to set up a test database
func setupFeedsDB(t *testing.T) *sql.DB {
	tmpDir, err := os.MkdirTemp("", "feeds_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tmpDir, "test_feeds.db")
	db, err := OpenFeedsDB(dbPath)
	if err != nil {
		t.Fatalf("failed to open feeds database: %v", err)
	}

	t.Cleanup(func() {
		db.Close()
		os.RemoveAll(tmpDir)
	})

	return db
}
