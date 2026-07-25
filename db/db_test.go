package db

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// setupTestDB creates a temporary database for testing
func setupTestDB(t *testing.T) *sql.DB {
	tmpDir, err := os.MkdirTemp("", "db_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() {
		os.RemoveAll(tmpDir)
	})

	dbPath := filepath.Join(tmpDir, "test.db")
	conn, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}
	t.Cleanup(func() {
		conn.Close()
	})

	// Create schema
	if err := createSchema(conn); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	return conn
}

// createSchema initializes the database schema
func createSchema(conn *sql.DB) error {
	queries := []string{
		`CREATE TABLE users (
			screenname TEXT PRIMARY KEY,
			emailAddress TEXT,
			emailSecret TEXT,
			imageUrl TEXT,
			prefs TEXT,
			ctHits INTEGER DEFAULT 0,
			ctHitsToday INTEGER DEFAULT 0,
			whenLastHit DATETIME,
			whenCreated DATETIME DEFAULT CURRENT_TIMESTAMP,
			whenUpdated DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TRIGGER trg_users_whenUpdated
		AFTER UPDATE ON users
		BEGIN
			UPDATE users SET whenUpdated = CURRENT_TIMESTAMP WHERE screenname = NEW.screenname;
		END`,
		`CREATE TABLE items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			feedUrl TEXT,
			author TEXT,
			inReplyTo INTEGER,
			title TEXT,
			link TEXT,
			description TEXT,
			pubDate DATETIME,
			enclosureUrl TEXT,
			enclosureType TEXT,
			enclosureLength INTEGER,
			markdowntext TEXT,
			outlineJsontext TEXT,
			flDeleted INTEGER DEFAULT 0,
			whenCreated DATETIME DEFAULT CURRENT_TIMESTAMP,
			whenUpdated DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX idx_items_feedUrl ON items(feedUrl)`,
		`CREATE INDEX idx_items_author ON items(author)`,
		`CREATE TRIGGER trg_items_whenUpdated
		AFTER UPDATE ON items
		BEGIN
			UPDATE items SET whenUpdated = CURRENT_TIMESTAMP WHERE id = NEW.id;
		END`,
		`CREATE TABLE likes (
			screenname TEXT,
			itemId INTEGER,
			whenCreated DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (screenname, itemId),
			FOREIGN KEY (itemId) REFERENCES items(id)
		)`,
		`CREATE INDEX idx_likes_itemId ON likes(itemId)`,
	}

	for _, query := range queries {
		if _, err := conn.Exec(query); err != nil {
			return err
		}
	}
	return nil
}

// Test user operations
func TestUserCRUD(t *testing.T) {
	db := setupTestDB(t)

	// Test AddUser
	err := AddUser(db, "alice", "alice@example.com", "secret123")
	if err != nil {
		t.Fatalf("AddUser failed: %v", err)
	}

	// Test GetUserInfoByScreenname
	retrieved, err := GetUserInfoByScreenname(db, "alice")
	if err != nil {
		t.Fatalf("GetUserInfoByScreenname failed: %v", err)
	}
	if retrieved == nil {
		t.Fatal("user not found")
	}
	if retrieved.Screenname != "alice" {
		t.Errorf("screenname = %s, want alice", retrieved.Screenname)
	}
	if retrieved.EmailAddress != "alice@example.com" {
		t.Errorf("email = %s, want alice@example.com", retrieved.EmailAddress)
	}

	// Test GetUserInfoByEmail
	byEmail, err := GetUserInfoByEmail(db, "alice@example.com")
	if err != nil {
		t.Fatalf("GetUserInfoByEmail failed: %v", err)
	}
	if byEmail == nil {
		t.Fatal("user not found by email")
	}
	if byEmail.Screenname != "alice" {
		t.Errorf("screenname = %s, want alice", byEmail.Screenname)
	}

	// Test UpdateUser
	err = UpdateUser(db, "alice", "alice@example.com", "newSecret")
	if err != nil {
		t.Fatalf("UpdateUser failed: %v", err)
	}

	updated, err := GetUserInfoByScreenname(db, "alice")
	if err != nil {
		t.Fatalf("GetUserInfoByScreenname failed after update: %v", err)
	}
	if updated.EmailSecret != "newSecret" {
		t.Errorf("emailSecret = %s, want newSecret", updated.EmailSecret)
	}

	// Test UpdateUserPrefs
	prefs := []byte(`{"theme":"dark"}`)
	err = UpdateUserPrefs(db, "alice", prefs)
	if err != nil {
		t.Fatalf("UpdateUserPrefs failed: %v", err)
	}

	updated, err = GetUserInfoByScreenname(db, "alice")
	if err != nil {
		t.Fatalf("GetUserInfoByScreenname failed after prefs update: %v", err)
	}
	var prefMap map[string]string
	if err := json.Unmarshal(updated.Prefs, &prefMap); err != nil {
		t.Fatalf("failed to unmarshal prefs: %v", err)
	}
	if prefMap["theme"] != "dark" {
		t.Errorf("theme pref = %s, want dark", prefMap["theme"])
	}

	// Test GetAllScreennames
	AddUser(db, "bob", "bob@example.com", "s2")
	AddUser(db, "charlie", "charlie@example.com", "s3")

	names, err := GetAllScreennames(db)
	if err != nil {
		t.Fatalf("GetAllScreennames failed: %v", err)
	}
	if len(names) != 3 {
		t.Errorf("count = %d, want 3", len(names))
	}
}

// Test item operations
func TestItemCRUD(t *testing.T) {
	db := setupTestDB(t)

	// Add a user first
	AddUser(db, "alice", "alice@example.com", "s1")

	// Test AddItem
	now := time.Now()
	newItem := NewItem{
		FeedURL:      "http://example.com/feed?screenname=alice",
		Title:        "My First Post",
		Description:  "<p>Hello world</p>",
		Author:       "alice",
		PubDate:      now,
		MarkdownText: "Hello world",
	}

	itemID, err := AddItem(db, newItem)
	if err != nil {
		t.Fatalf("AddItem failed: %v", err)
	}
	if itemID <= 0 {
		t.Errorf("itemID = %d, want > 0", itemID)
	}

	// Test GetItemByID
	item, err := GetItemByID(db, "alice", itemID, "http://example.com")
	if err != nil {
		t.Fatalf("GetItemByID failed: %v", err)
	}
	if item == nil {
		t.Fatal("item not found")
	}
	if item.Title != "My First Post" {
		t.Errorf("title = %s, want My First Post", item.Title)
	}
	if item.Author != "alice" {
		t.Errorf("author = %s, want alice", item.Author)
	}

	// Test UpdateItem
	patch := ItemPatch{
		ID:    itemID,
		Title: strPtr("My Updated Post"),
	}
	err = UpdateItem(db, patch)
	if err != nil {
		t.Fatalf("UpdateItem failed: %v", err)
	}

	updated, err := GetItemByID(db, "alice", itemID, "http://example.com")
	if err != nil {
		t.Fatalf("GetItemByID failed after update: %v", err)
	}
	if updated.Title != "My Updated Post" {
		t.Errorf("title = %s, want My Updated Post", updated.Title)
	}

	// Test GetRecentItems
	items, err := GetRecentItems(db, "alice", 10, "http://example.com")
	if err != nil {
		t.Fatalf("GetRecentItems failed: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("count = %d, want 1", len(items))
	}
}

// Test likes operations
func TestLikes(t *testing.T) {
	db := setupTestDB(t)

	// Add users
	AddUser(db, "alice", "alice@example.com", "s1")
	AddUser(db, "bob", "bob@example.com", "s2")

	// Add an item
	itemID, err := AddItem(db, NewItem{
		FeedURL:     "http://example.com/feed?screenname=alice",
		Title:       "Post",
		Description: "Content",
		Author:      "alice",
		PubDate:     time.Now(),
	})
	if err != nil {
		t.Fatalf("AddItem failed: %v", err)
	}

	// Test IsLiked (should be false initially)
	liked, err := IsLiked(db, "bob", itemID)
	if err != nil {
		t.Fatalf("IsLiked failed: %v", err)
	}
	if liked {
		t.Error("item should not be liked initially")
	}

	// Test AddToLikesTable
	err = AddToLikesTable(db, "bob", itemID)
	if err != nil {
		t.Fatalf("AddToLikesTable failed: %v", err)
	}

	// Check again
	liked, err = IsLiked(db, "bob", itemID)
	if err != nil {
		t.Fatalf("IsLiked failed: %v", err)
	}
	if !liked {
		t.Error("item should be liked after adding like")
	}

	// Test GetLikersList
	likers, err := GetLikersList(db, itemID)
	if err != nil {
		t.Fatalf("GetLikersList failed: %v", err)
	}
	if len(likers) != 1 {
		t.Errorf("likers count = %d, want 1", len(likers))
	}
	if likers[0] != "bob" {
		t.Errorf("liker = %s, want bob", likers[0])
	}

	// Test RemoveFromLikesTable
	err = RemoveFromLikesTable(db, "bob", itemID)
	if err != nil {
		t.Fatalf("RemoveFromLikesTable failed: %v", err)
	}

	liked, err = IsLiked(db, "bob", itemID)
	if err != nil {
		t.Fatalf("IsLiked failed: %v", err)
	}
	if liked {
		t.Error("item should not be liked after removing like")
	}
}

// Test reply threading
func TestReplies(t *testing.T) {
	db := setupTestDB(t)

	// Add users
	AddUser(db, "alice", "alice@example.com", "s1")
	AddUser(db, "bob", "bob@example.com", "s2")

	// Add parent item
	parentID, err := AddItem(db, NewItem{
		FeedURL:     "http://example.com/feed?screenname=alice",
		Title:       "Parent Post",
		Description: "Original content",
		Author:      "alice",
		PubDate:     time.Now(),
	})
	if err != nil {
		t.Fatalf("AddItem failed: %v", err)
	}

	// Add reply
	replyID, err := AddItem(db, NewItem{
		FeedURL:     "http://example.com/feed?screenname=bob",
		Title:       "Reply",
		Description: "Reply content",
		Author:      "bob",
		InReplyTo:   &parentID,
		PubDate:     time.Now(),
	})
	if err != nil {
		t.Fatalf("AddItem failed: %v", err)
	}

	// Test GetItemAndReplies
	parentWithReplies, err := GetItemAndReplies(db, "alice", parentID, "http://example.com")
	if err != nil {
		t.Fatalf("GetItemAndReplies failed: %v", err)
	}

	if len(parentWithReplies) != 2 {
		t.Errorf("items count = %d, want 2 (parent + reply)", len(parentWithReplies))
	}

	// First item should be parent, second should be reply
	if parentWithReplies[0].ID != parentID {
		t.Errorf("first item id = %d, want %d", parentWithReplies[0].ID, parentID)
	}
	if parentWithReplies[1].ID != replyID {
		t.Errorf("second item id = %d, want %d", parentWithReplies[1].ID, replyID)
	}
}

// Helper function to create string pointer
func strPtr(s string) *string {
	return &s
}
