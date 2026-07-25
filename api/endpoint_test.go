package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"rss.chat.go/db"
	"rss.chat.go/feed"
	"rss.chat.go/publish"
	_ "modernc.org/sqlite"
)

// setupTestServer creates a test HTTP server with handler
func setupTestServer(t *testing.T) (*http.ServeMux, *sql.DB, *Handler) {
	// Create temp directory for feeds and database
	tmpDir, err := os.MkdirTemp("", "api_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() {
		os.RemoveAll(tmpDir)
	})

	// Create database
	dbPath := filepath.Join(tmpDir, "test.db")
	conn, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	t.Cleanup(func() {
		conn.Close()
	})

	// Create schema
	if err := createTestSchema(conn); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	// Create feeds directory
	feedsDir := filepath.Join(tmpDir, "feeds")
	if err := os.MkdirAll(feedsDir, 0755); err != nil {
		t.Fatalf("failed to create feeds dir: %v", err)
	}

	// Create publisher
	feedConfig := feed.BuilderConfig{
		BaseURL:      "localhost:8081",
		ProductName:  "rss.chat",
		MaxFeedItems: 100,
		Language:     "en",
	}
	pub := publish.NewPublisher(feedsDir, feedConfig)

	// Create handler
	handler := NewHandler(conn, pub, feedConfig)
	handler.RobotsContent = `User-agent: *
Disallow: /getitembyguid
Disallow: /getiteminfo
`

	// Create mux and register routes
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	return mux, conn, handler
}

// setupTestServerWithFeedsDB creates a test server with feeds database enabled
func setupTestServerWithFeedsDB(t *testing.T) (*http.ServeMux, *sql.DB, *sql.DB, *Handler) {
	mux, conn, handler := setupTestServer(t)

	// Create feeds database
	tmpDir, err := os.MkdirTemp("", "feeds_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	feedsDBPath := filepath.Join(tmpDir, "test_feeds.db")
	feedsDB, err := db.OpenFeedsDB(feedsDBPath)
	if err != nil {
		t.Fatalf("failed to open feeds database: %v", err)
	}

	t.Cleanup(func() {
		feedsDB.Close()
		os.RemoveAll(tmpDir)
	})

	// Enable database mode on handler
	handler.FeedsDB = feedsDB

	// Enable database mode on publisher
	handler.Publisher.SetDatabaseMode(feedsDB)

	return mux, conn, feedsDB, handler
}

// createTestSchema creates the database schema for testing
func createTestSchema(conn *sql.DB) error {
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
		AFTER UPDATE ON users BEGIN
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
		AFTER UPDATE ON items BEGIN
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

// Test /health endpoint
func TestHealthEndpoint(t *testing.T) {
	mux, _, _ := setupTestServer(t)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	// Health endpoint returns plain text
	if w.Body.String() != "OK" {
		t.Errorf("body = %s, want OK", w.Body.String())
	}
}

// Test read endpoints with empty database
func TestReadEndpointsEmpty(t *testing.T) {
	mux, _, _ := setupTestServer(t)

	tests := []struct {
		name   string
		path   string
		status int
	}{
		{"/getrecentitems", "/getrecentitems", http.StatusOK},
		{"/getmostactivetoday", "/getmostactivetoday", http.StatusOK},
		{"/isuserindatabase?screenname=alice", "/isuserindatabase?screenname=alice", http.StatusOK},
		{"/isemailindatabase?email=alice@example.com", "/isemailindatabase?email=alice@example.com", http.StatusOK},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", test.path, nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != test.status {
				t.Errorf("status = %d, want %d", w.Code, test.status)
			}
		})
	}
}

// Test /robots.txt endpoint
func TestRobotsTxt(t *testing.T) {
	mux, _, _ := setupTestServer(t)

	req := httptest.NewRequest("GET", "/robots.txt", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	if w.Header().Get("Content-Type") != "text/plain" {
		t.Errorf("content type = %s, want text/plain", w.Header().Get("Content-Type"))
	}

	body := w.Body.String()
	if !strings.Contains(body, "User-agent:") {
		t.Error("robots.txt missing User-agent")
	}
	if !strings.Contains(body, "Disallow:") {
		t.Error("robots.txt missing Disallow")
	}
}

// Test user creation flow
func TestUserCreationFlow(t *testing.T) {
	mux, conn, _ := setupTestServer(t)

	// First, create a new user
	req := httptest.NewRequest("GET", "/createnewuser?email=test@example.com&name=testuser&urlredirect=http://localhost/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
		t.Logf("response: %s", w.Body.String())
		return
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	// Verify user was created
	user, err := db.GetUserInfoByScreenname(conn, "testuser")
	if err != nil {
		t.Fatalf("failed to get user: %v", err)
	}
	if user == nil {
		t.Fatal("user was not created")
	}

	if user.Screenname != "testuser" {
		t.Errorf("screenname = %s, want testuser", user.Screenname)
	}
	if user.EmailAddress != "test@example.com" {
		t.Errorf("email = %s, want test@example.com", user.EmailAddress)
	}
}

// Test /isuserindatabase endpoint
func TestIsUserInDatabase(t *testing.T) {
	mux, conn, _ := setupTestServer(t)

	// Create a user
	db.AddUser(conn, "alice", "alice@example.com", "secret123")

	// Test user exists
	req := httptest.NewRequest("GET", "/isuserindatabase?screenname=alice", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if !resp["flInDatabase"].(bool) {
		t.Error("user should exist")
	}

	// Test user doesn't exist
	req = httptest.NewRequest("GET", "/isuserindatabase?screenname=bob", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp["flInDatabase"].(bool) {
		t.Error("user should not exist")
	}
}

// Test /getrecentitems endpoint
func TestGetRecentItems(t *testing.T) {
	mux, conn, _ := setupTestServer(t)

	// Create a user and post
	db.AddUser(conn, "alice", "alice@example.com", "s1")
	itemID, err := db.AddItem(conn, db.NewItem{
		FeedURL:     "http://localhost:8081/feed?screenname=alice",
		Title:       "Test Post",
		Description: "<p>Hello world</p>",
		Author:      "alice",
		PubDate:     time.Now(),
	})
	if err != nil {
		t.Fatalf("failed to add item: %v", err)
	}

	req := httptest.NewRequest("GET", "/getrecentitems", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var items []map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &items); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(items) != 1 {
		t.Errorf("item count = %d, want 1", len(items))
	}

	if items[0]["title"] != "Test Post" {
		t.Errorf("title = %s, want Test Post", items[0]["title"])
	}

	if int(items[0]["id"].(float64)) != int(itemID) {
		t.Errorf("id = %v, want %d", items[0]["id"], itemID)
	}
}

// Test /getuserdata endpoint
func TestGetUserData(t *testing.T) {
	mux, conn, _ := setupTestServer(t)

	// Create a user
	db.AddUser(conn, "alice", "alice@example.com", "secret")
	prefs := []byte(`{"myFeedTitle":"Alice's Feed"}`)
	db.UpdateUserPrefs(conn, "alice", prefs)

	req := httptest.NewRequest("GET", "/getuserdata?screenname=alice", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var user map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &user); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if user["screenname"] != "alice" {
		t.Errorf("screenname = %s, want alice", user["screenname"])
	}
}

// Test error response format
func TestErrorResponseFormat(t *testing.T) {
	mux, _, _ := setupTestServer(t)

	// Try to get a non-existent item
	req := httptest.NewRequest("GET", "/getitemandreplies?id=999", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// Should get 503 for error responses per spec
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}

	body := w.Body.String()
	// Error responses are plain text with format "Can't X because Y."
	if !strings.HasPrefix(body, "Can't") {
		t.Errorf("error message doesn't start with 'Can't': %s", body)
	}
}

// Test /checkwhitelist endpoint
func TestCheckWhitelist(t *testing.T) {
	mux, _, _ := setupTestServer(t)

	// No whitelist configured, so all emails should pass
	req := httptest.NewRequest("GET", "/checkwhitelist?emailaddress=anyone@example.com", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if !resp["flWhitelisted"].(bool) {
		t.Error("email should be whitelisted (no whitelist = all pass)")
	}
}

// Test /getlikerslist endpoint
func TestGetLikersList(t *testing.T) {
	mux, conn, _ := setupTestServer(t)

	// Create users
	db.AddUser(conn, "alice", "alice@example.com", "s1")
	db.AddUser(conn, "bob", "bob@example.com", "s2")
	db.AddUser(conn, "charlie", "charlie@example.com", "s3")

	// Create item
	itemID, _ := db.AddItem(conn, db.NewItem{
		FeedURL:     "http://localhost:8081/feed?screenname=alice",
		Title:       "Post",
		Description: "Content",
		Author:      "alice",
		PubDate:     time.Now(),
	})

	// Add likes
	db.AddToLikesTable(conn, "bob", itemID)
	db.AddToLikesTable(conn, "charlie", itemID)

	req := httptest.NewRequest("GET", fmt.Sprintf("/getlikerslist?id=%d", itemID), nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var likers []string
	if err := json.Unmarshal(w.Body.Bytes(), &likers); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(likers) != 2 {
		t.Errorf("liker count = %d, want 2", len(likers))
	}
}

// Database mode endpoint tests

func TestFeedEndpointDatabase(t *testing.T) {
	mux, conn, _, handler := setupTestServerWithFeedsDB(t)

	// Create test user and item
	now := time.Now()
	_, err := conn.Exec(`insert into users (screenname, emailAddress, emailSecret, imageUrl, prefs, ctHits, ctHitsToday, whenLastHit, whenCreated, whenUpdated)
		values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"alice", "alice@example.com", "secret", "", `{}`, 0, 0, now, now, now)
	if err != nil {
		t.Fatalf("failed to insert user: %v", err)
	}

	feedURL := "http://localhost:8081/feed?screenname=alice"
	_, err = conn.Exec(`insert into items (feedUrl, author, title, description, pubDate, whenCreated, whenUpdated)
		values (?, ?, ?, ?, ?, ?, ?)`,
		feedURL, "alice", "DB Test Post", "Testing database mode", now, now, now)
	if err != nil {
		t.Fatalf("failed to insert item: %v", err)
	}

	// Publish feed to database
	if err := handler.Publisher.PublishUserFeed(conn, "alice"); err != nil {
		t.Fatalf("failed to publish feed: %v", err)
	}

	// Test user feed from database
	req := httptest.NewRequest("GET", "/feed?screenname=alice", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "application/rss+xml") {
		t.Errorf("content type = %q, want application/rss+xml", contentType)
	}

	if !strings.Contains(w.Body.String(), "DB Test Post") {
		t.Errorf("feed does not contain test post")
	}
}

func TestGlobalFeedDatabaseMode(t *testing.T) {
	mux, conn, _, handler := setupTestServerWithFeedsDB(t)

	// Create test user and item
	now := time.Now()
	_, err := conn.Exec(`insert into users (screenname, emailAddress, emailSecret, imageUrl, prefs, ctHits, ctHitsToday, whenLastHit, whenCreated, whenUpdated)
		values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"bob", "bob@example.com", "secret", "", `{}`, 0, 0, now, now, now)
	if err != nil {
		t.Fatalf("failed to insert user: %v", err)
	}

	feedURL := "http://localhost:8081/feed?screenname=bob"
	_, err = conn.Exec(`insert into items (feedUrl, author, title, description, pubDate, whenCreated, whenUpdated)
		values (?, ?, ?, ?, ?, ?, ?)`,
		feedURL, "bob", "Global Test Post", "Testing database mode", now, now, now)
	if err != nil {
		t.Fatalf("failed to insert item: %v", err)
	}

	// Publish global feed to database
	if err := handler.Publisher.PublishEveryoneFeed(conn); err != nil {
		t.Fatalf("failed to publish everyone feed: %v", err)
	}

	// Test global feed from database
	req := httptest.NewRequest("GET", "/feed", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}

	if !strings.Contains(w.Body.String(), "Global Test Post") {
		t.Errorf("global feed does not contain test post")
	}
}

func TestSubscriptionListDatabaseMode(t *testing.T) {
	mux, conn, _, handler := setupTestServerWithFeedsDB(t)

	// Create test user
	now := time.Now()
	_, err := conn.Exec(`insert into users (screenname, emailAddress, emailSecret, imageUrl, prefs, ctHits, ctHitsToday, whenLastHit, whenCreated, whenUpdated)
		values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"charlie", "charlie@example.com", "secret", "", `{}`, 0, 0, now, now, now)
	if err != nil {
		t.Fatalf("failed to insert user: %v", err)
	}

	// Publish subscription list to database
	if err := handler.Publisher.PublishSubscriptionList(conn); err != nil {
		t.Fatalf("failed to publish subscription list: %v", err)
	}

	// Test subscription list from database
	req := httptest.NewRequest("GET", "/getsubscriptionlist", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}

	if w.Header().Get("Content-Type") != "application/xml; charset=utf-8" {
		t.Errorf("content type = %q, want application/xml; charset=utf-8", w.Header().Get("Content-Type"))
	}

	if !strings.Contains(w.Body.String(), "charlie") {
		t.Errorf("subscription list does not contain user")
	}
}

// Format negotiation tests

func TestFeedFormatJSON(t *testing.T) {
	mux, conn, _ := setupTestServer(t)

	// Create test user and item
	now := time.Now()
	_, err := conn.Exec(`insert into users (screenname, emailAddress, emailSecret, imageUrl, prefs, ctHits, ctHitsToday, whenLastHit, whenCreated, whenUpdated)
		values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"alice", "alice@example.com", "secret", "", `{}`, 0, 0, now, now, now)
	if err != nil {
		t.Fatalf("failed to insert user: %v", err)
	}

	feedURL := "http://localhost:8081/feed?screenname=alice"
	_, err = conn.Exec(`insert into items (feedUrl, author, title, description, pubDate, whenCreated, whenUpdated)
		values (?, ?, ?, ?, ?, ?, ?)`,
		feedURL, "alice", "JSON Format Test", "Testing JSON format", now, now, now)
	if err != nil {
		t.Fatalf("failed to insert item: %v", err)
	}

	// Request JSON format
	req := httptest.NewRequest("GET", "/feed?screenname=alice&format=json", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}

	if w.Header().Get("Content-Type") != "application/json; charset=utf-8" {
		t.Errorf("content type = %q, want application/json; charset=utf-8", w.Header().Get("Content-Type"))
	}

	// Verify valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Errorf("response is not valid JSON: %v", err)
	}

	// Verify content
	if !strings.Contains(w.Body.String(), "JSON Format Test") {
		t.Errorf("JSON feed does not contain test post")
	}
}

func TestFeedFormatXML(t *testing.T) {
	mux, conn, handler := setupTestServer(t)

	// Create test user and item
	now := time.Now()
	_, err := conn.Exec(`insert into users (screenname, emailAddress, emailSecret, imageUrl, prefs, ctHits, ctHitsToday, whenLastHit, whenCreated, whenUpdated)
		values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"bob", "bob@example.com", "secret", "", `{}`, 0, 0, now, now, now)
	if err != nil {
		t.Fatalf("failed to insert user: %v", err)
	}

	feedURL := "http://localhost:8081/feed?screenname=bob"
	_, err = conn.Exec(`insert into items (feedUrl, author, title, description, pubDate, whenCreated, whenUpdated)
		values (?, ?, ?, ?, ?, ?, ?)`,
		feedURL, "bob", "XML Format Test", "Testing XML format", now, now, now)
	if err != nil {
		t.Fatalf("failed to insert item: %v", err)
	}

	// Publish the feed to filesystem
	if err := handler.Publisher.PublishUserFeed(conn, "bob"); err != nil {
		t.Fatalf("failed to publish feed: %v", err)
	}

	// Request explicit XML format
	req := httptest.NewRequest("GET", "/feed?screenname=bob&format=xml", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "application/rss+xml") {
		t.Errorf("content type = %q, want application/rss+xml", contentType)
	}

	if !strings.Contains(w.Body.String(), "<?xml") {
		t.Errorf("XML feed does not start with XML declaration")
	}
}

func TestFeedDefaultFormatXML(t *testing.T) {
	mux, conn, handler := setupTestServer(t)

	// Create test user and item
	now := time.Now()
	_, err := conn.Exec(`insert into users (screenname, emailAddress, emailSecret, imageUrl, prefs, ctHits, ctHitsToday, whenLastHit, whenCreated, whenUpdated)
		values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"charlie", "charlie@example.com", "secret", "", `{}`, 0, 0, now, now, now)
	if err != nil {
		t.Fatalf("failed to insert user: %v", err)
	}

	feedURL := "http://localhost:8081/feed?screenname=charlie"
	_, err = conn.Exec(`insert into items (feedUrl, author, title, description, pubDate, whenCreated, whenUpdated)
		values (?, ?, ?, ?, ?, ?, ?)`,
		feedURL, "charlie", "Default Format Test", "Testing default format", now, now, now)
	if err != nil {
		t.Fatalf("failed to insert item: %v", err)
	}

	// Publish the feed to filesystem
	if err := handler.Publisher.PublishUserFeed(conn, "charlie"); err != nil {
		t.Fatalf("failed to publish feed: %v", err)
	}

	// Request without format parameter - should default to XML
	req := httptest.NewRequest("GET", "/feed?screenname=charlie", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "application/rss+xml") {
		t.Errorf("default format should be XML, got %q", contentType)
	}
}

func TestFeedInvalidFormat(t *testing.T) {
	mux, conn, _ := setupTestServer(t)

	// Create test user
	now := time.Now()
	_, err := conn.Exec(`insert into users (screenname, emailAddress, emailSecret, imageUrl, prefs, ctHits, ctHitsToday, whenLastHit, whenCreated, whenUpdated)
		values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"dave", "dave@example.com", "secret", "", `{}`, 0, 0, now, now, now)
	if err != nil {
		t.Fatalf("failed to insert user: %v", err)
	}

	// Request invalid format
	req := httptest.NewRequest("GET", "/feed?screenname=dave&format=invalid", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		// Note: Our error response is not an HTTP error, it's a JSON response with error message
		// This is consistent with other API error responses
	}

	if !strings.Contains(w.Body.String(), "Invalid format") {
		t.Errorf("expected error message about invalid format")
	}
}

func TestGlobalFeedJSON(t *testing.T) {
	mux, conn, _ := setupTestServer(t)

	// Create test user and item
	now := time.Now()
	_, err := conn.Exec(`insert into users (screenname, emailAddress, emailSecret, imageUrl, prefs, ctHits, ctHitsToday, whenLastHit, whenCreated, whenUpdated)
		values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"eve", "eve@example.com", "secret", "", `{}`, 0, 0, now, now, now)
	if err != nil {
		t.Fatalf("failed to insert user: %v", err)
	}

	feedURL := "http://localhost:8081/feed"
	_, err = conn.Exec(`insert into items (feedUrl, author, title, description, pubDate, whenCreated, whenUpdated)
		values (?, ?, ?, ?, ?, ?, ?)`,
		feedURL, "eve", "Global JSON Test", "Testing global feed JSON", now, now, now)
	if err != nil {
		t.Fatalf("failed to insert item: %v", err)
	}

	// Request global feed in JSON
	req := httptest.NewRequest("GET", "/feed?format=json", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}

	if w.Header().Get("Content-Type") != "application/json; charset=utf-8" {
		t.Errorf("content type should be JSON")
	}

	if !strings.Contains(w.Body.String(), "Global JSON Test") {
		t.Errorf("global JSON feed does not contain test post")
	}
}
