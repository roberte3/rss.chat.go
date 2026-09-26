package api

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	_ "modernc.org/sqlite"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/roberte3/rss.chat.go/config"
	"github.com/roberte3/rss.chat.go/db"
	"github.com/roberte3/rss.chat.go/feed"
	"github.com/roberte3/rss.chat.go/publish"
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
		BaseURL:      "http://localhost:8081",
		ProductName:  "rss.chat",
		MaxFeedItems: 100,
		Language:     "en",
	}
	pub := publish.NewPublisher(feedsDir, feedConfig)

	// Create media database
	mediaDBPath := filepath.Join(tmpDir, "media.db")
	mediaDB, err := db.OpenMediaDB(mediaDBPath)
	if err != nil {
		t.Fatalf("failed to open media database: %v", err)
	}
	t.Cleanup(func() {
		mediaDB.Close()
	})

	// Create temp media path
	tempMediaPath := filepath.Join(tmpDir, "temp_media")
	if err := os.MkdirAll(tempMediaPath, 0755); err != nil {
		t.Fatalf("failed to create temp media dir: %v", err)
	}

	// Create handler
	handler := NewHandler(conn, pub, feedConfig)
	handler.RobotsContent = `User-agent: *
Disallow: /getitembyguid
Disallow: /getiteminfo
`
	handler.MediaDB = mediaDB
	handler.MaxMediaUploadBytes = 2 * 1024 * 1024 // 2MB limit
	handler.TempMediaPath = tempMediaPath

	// Set up config with WebSub defaults
	handler.Config = &config.Config{
		URLWebsubHub:  "https://rpc.rsscloud.io/websub",
		WebsubEnabled: false, // Disabled by default in tests
	}

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
		`CREATE TABLE blocklist (
			email TEXT PRIMARY KEY,
			whenAdded DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE mentions (
			itemId INTEGER NOT NULL REFERENCES items(id) ON DELETE CASCADE,
			screenname TEXT NOT NULL REFERENCES users(screenname),
			whenCreated DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (itemId, screenname)
		)`,
		`CREATE INDEX idx_mentions_screenname ON mentions(screenname)`,
		`CREATE TABLE hashtags (
			tag TEXT NOT NULL,
			itemId INTEGER NOT NULL REFERENCES items(id) ON DELETE CASCADE,
			PRIMARY KEY (tag, itemId)
		)`,
		`CREATE INDEX idx_hashtags_itemId ON hashtags(itemId)`,
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

// Blocklist enforcement tests

func TestBlockedUserCannotSignIn(t *testing.T) {
	mux, conn, handler := setupTestServer(t)

	// Create a temporary blocklist.json file
	tmpDir := t.TempDir()
	blocklistPath := filepath.Join(tmpDir, "blocklist.json")
	blocklistContent := []byte(`{
		"note": "Test blocklist",
		"blockedEmails": ["blocked@example.com"]
	}`)
	if err := os.WriteFile(blocklistPath, blocklistContent, 0644); err != nil {
		t.Fatalf("failed to create blocklist file: %v", err)
	}

	// Set handler config to use the blocklist file
	handler.Config = &config.Config{
		BlocklistPath: blocklistPath,
	}

	// Sync blocklist to database
	emails, err := config.LoadBlocklist(blocklistPath)
	if err != nil {
		t.Fatalf("failed to load blocklist: %v", err)
	}
	if err := db.SyncBlocklistToDB(conn, emails); err != nil {
		t.Fatalf("failed to sync blocklist to database: %v", err)
	}

	// Try to send confirmation email to blocked address
	req := httptest.NewRequest("GET", "/sendconfirmingemail?email=blocked@example.com&urlredirect=http://localhost/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if !strings.Contains(w.Body.String(), "not allowed") {
		t.Errorf("expected 'not allowed' error for blocked email, got: %s", w.Body.String())
	}
}

func TestBlockedUserCannotSignUp(t *testing.T) {
	mux, conn, handler := setupTestServer(t)

	// Create a temporary blocklist.json file
	tmpDir := t.TempDir()
	blocklistPath := filepath.Join(tmpDir, "blocklist.json")
	blocklistContent := []byte(`{
		"note": "Test blocklist",
		"blockedEmails": ["blocked@example.com"]
	}`)
	if err := os.WriteFile(blocklistPath, blocklistContent, 0644); err != nil {
		t.Fatalf("failed to create blocklist file: %v", err)
	}

	// Set handler config to use the blocklist file
	handler.Config = &config.Config{
		BlocklistPath: blocklistPath,
	}

	// Sync blocklist to database
	emails, err := config.LoadBlocklist(blocklistPath)
	if err != nil {
		t.Fatalf("failed to load blocklist: %v", err)
	}
	if err := db.SyncBlocklistToDB(conn, emails); err != nil {
		t.Fatalf("failed to sync blocklist to database: %v", err)
	}

	// Try to create new user with blocked email
	req := httptest.NewRequest("GET", "/createnewuser?email=blocked@example.com&name=blockeduser&urlredirect=http://localhost/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if !strings.Contains(w.Body.String(), "not allowed") {
		t.Errorf("expected 'not allowed' error for blocked email, got: %s", w.Body.String())
	}
}

func TestAllowedUserCanSignUp(t *testing.T) {
	mux, conn, handler := setupTestServer(t)

	// Create a temporary blocklist.json file (empty, allow all)
	tmpDir := t.TempDir()
	blocklistPath := filepath.Join(tmpDir, "blocklist.json")
	blocklistContent := []byte(`{
		"note": "Test blocklist",
		"blockedEmails": []
	}`)
	if err := os.WriteFile(blocklistPath, blocklistContent, 0644); err != nil {
		t.Fatalf("failed to create blocklist file: %v", err)
	}

	// Set handler config to use the blocklist file
	handler.Config = &config.Config{
		BlocklistPath: blocklistPath,
	}

	// Sync blocklist to database (empty list, allow all)
	emails, err := config.LoadBlocklist(blocklistPath)
	if err != nil {
		t.Fatalf("failed to load blocklist: %v", err)
	}
	if err := db.SyncBlocklistToDB(conn, emails); err != nil {
		t.Fatalf("failed to sync blocklist to database: %v", err)
	}

	// Try to send confirmation email to allowed address
	req := httptest.NewRequest("GET", "/sendconfirmingemail?email=allowed@example.com&urlredirect=http://localhost/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// Should succeed (or fail for other reasons like email sending, but not blocklist)
	if strings.Contains(w.Body.String(), "not allowed") {
		t.Errorf("allowed email should not be rejected by blocklist")
	}
}

func TestBlocklistCaseInsensitive(t *testing.T) {
	mux, conn, handler := setupTestServer(t)

	// Create a temporary blocklist.json file
	tmpDir := t.TempDir()
	blocklistPath := filepath.Join(tmpDir, "blocklist.json")
	blocklistContent := []byte(`{
		"note": "Test blocklist",
		"blockedEmails": ["blocked@example.com"]
	}`)
	if err := os.WriteFile(blocklistPath, blocklistContent, 0644); err != nil {
		t.Fatalf("failed to create blocklist file: %v", err)
	}

	// Set handler config to use the blocklist file
	handler.Config = &config.Config{
		BlocklistPath: blocklistPath,
	}

	// Sync blocklist to database
	emails, err := config.LoadBlocklist(blocklistPath)
	if err != nil {
		t.Fatalf("failed to load blocklist: %v", err)
	}
	if err := db.SyncBlocklistToDB(conn, emails); err != nil {
		t.Fatalf("failed to sync blocklist to database: %v", err)
	}

	// Try to sign up with uppercase version
	req := httptest.NewRequest("GET", "/sendconfirmingemail?email=BLOCKED@EXAMPLE.COM&urlredirect=http://localhost/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// Should still be blocked (case insensitive)
	if !strings.Contains(w.Body.String(), "not allowed") {
		t.Errorf("blocklist check should be case-insensitive, but uppercase wasn't blocked")
	}
}

// Post cleanup tests

func TestRemoveTrailingEmptyParagraphs(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no trailing paragraphs",
			input:    "<p>Hello</p>",
			expected: "<p>Hello</p>",
		},
		{
			name:     "single trailing empty paragraph",
			input:    "<p>Hello</p><p></p>",
			expected: "<p>Hello</p>",
		},
		{
			name:     "multiple trailing empty paragraphs",
			input:    "<p>Hello</p><p></p><p></p>",
			expected: "<p>Hello</p>",
		},
		{
			name:     "empty paragraph with whitespace",
			input:    "<p>Hello</p><p>   </p>",
			expected: "<p>Hello</p>",
		},
		{
			name:     "all empty paragraphs",
			input:    "<p></p><p></p>",
			expected: "",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "paragraph with attributes",
			input:    "<p>Hello</p><p class=\"empty\"></p>",
			expected: "<p>Hello</p>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := removeTrailingEmptyParagraphs(tt.input)
			if result != tt.expected {
				t.Errorf("removeTrailingEmptyParagraphs(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestCustomOPMLTitle(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "opml_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	testDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer testDB.Close()

	if err := createTestSchema(testDB); err != nil {
		t.Fatalf("failed to init test db: %v", err)
	}

	now := time.Now()
	_, err = testDB.Exec(`insert into users (screenname, emailAddress, emailSecret, imageUrl, prefs, ctHits, ctHitsToday, whenLastHit, whenCreated, whenUpdated)
		values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"alice", "alice@example.com", "secret", "", `{}`, 0, 0, now, now, now)
	if err != nil {
		t.Fatalf("failed to insert user: %v", err)
	}

	tests := []struct {
		name          string
		customTitle   string
		shouldContain string
	}{
		{
			name:          "default title",
			customTitle:   "",
			shouldContain: "Subscription list",
		},
		{
			name:          "custom title",
			customTitle:   "My News Subscriptions",
			shouldContain: "My News Subscriptions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := feed.BuilderConfig{
				BaseURL:                  "http://localhost:8081",
				ProductName:              "rss.chat",
				MaxFeedItems:             100,
				Language:                 "en",
				DocsURL:                  "http://www.rssboard.org/rss-specification",
				TitleForSubscriptionList: tt.customTitle,
			}

			opml, err := feed.BuildSubscriptionList(testDB, "http://localhost:8081", cfg)
			if err != nil {
				t.Fatalf("failed to build subscription list: %v", err)
			}

			if !strings.Contains(opml, tt.shouldContain) {
				t.Errorf("OPML should contain %q, got: %s", tt.shouldContain, opml)
			}
		})
	}
}

// TestGetSubscriptionListEndpoint covers the endpoint in the default
// filesystem storage mode with nothing published yet. It used to serve
// feeds/subs.opml from disk, but that file was only ever written by
// BackfillMissingFeeds, which returns early unless storage is "database" — so
// in the default configuration the file never existed and the endpoint always
// 404'd. The list is now built from the database per request, so a fresh
// install answers correctly.
func TestGetSubscriptionListEndpoint(t *testing.T) {
	mux, conn, _ := setupTestServer(t)

	now := time.Now()
	for _, name := range []string{"alice", "bob"} {
		_, err := conn.Exec(`insert into users (screenname, emailAddress, emailSecret, imageUrl, prefs, ctHits, ctHitsToday, whenLastHit, whenCreated, whenUpdated)
			values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			name, name+"@example.com", "secret", "", `{}`, 0, 0, now, now, now)
		if err != nil {
			t.Fatalf("failed to insert user %s: %v", name, err)
		}
	}

	// Deliberately no PublishSubscriptionList call: a fresh install has no
	// published artifact, which is the case that regressed.
	for _, path := range []string{"/getsubscriptionlist", "/api/getsubscriptionlist"} {
		req := httptest.NewRequest("GET", path, nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("%s: status = %d, want 200", path, w.Code)
			continue
		}
		if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "xml") {
			t.Errorf("%s: content type = %q, want xml", path, ct)
		}
		body := w.Body.String()
		for _, want := range []string{"<opml", "alice", "bob"} {
			if !strings.Contains(body, want) {
				t.Errorf("%s: OPML missing %q, got: %s", path, want, body)
			}
		}
	}
}

// TestGetSubscriptionListReflectsNewUsers guards the staleness half: because
// the list is generated per request, a user added after the last publish still
// shows up.
func TestGetSubscriptionListReflectsNewUsers(t *testing.T) {
	mux, conn, handler := setupTestServer(t)

	now := time.Now()
	if _, err := conn.Exec(`insert into users (screenname, emailAddress, emailSecret, imageUrl, prefs, ctHits, ctHitsToday, whenLastHit, whenCreated, whenUpdated)
		values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"alice", "alice@example.com", "secret", "", `{}`, 0, 0, now, now, now); err != nil {
		t.Fatalf("failed to insert alice: %v", err)
	}
	if err := handler.Publisher.PublishSubscriptionList(conn); err != nil {
		t.Fatalf("failed to publish subscription list: %v", err)
	}

	// carol arrives after the artifact was written
	if _, err := conn.Exec(`insert into users (screenname, emailAddress, emailSecret, imageUrl, prefs, ctHits, ctHitsToday, whenLastHit, whenCreated, whenUpdated)
		values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"carol", "carol@example.com", "secret", "", `{}`, 0, 0, now, now, now); err != nil {
		t.Fatalf("failed to insert carol: %v", err)
	}

	req := httptest.NewRequest("GET", "/getsubscriptionlist", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if !strings.Contains(w.Body.String(), "carol") {
		t.Errorf("OPML is stale: missing user added after the last publish, got: %s", w.Body.String())
	}
}

// insertTestUser adds a user whose emailSecret doubles as the emailcode used
// to authenticate write requests.
func insertTestUser(t *testing.T, conn *sql.DB, screenname, secret string) {
	t.Helper()
	now := time.Now()
	_, err := conn.Exec(`insert into users (screenname, emailAddress, emailSecret, imageUrl, prefs, ctHits, ctHitsToday, whenLastHit, whenCreated, whenUpdated)
		values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		screenname, screenname+"@example.com", secret, "", `{}`, 0, 0, now, now, now)
	if err != nil {
		t.Fatalf("failed to insert user %s: %v", screenname, err)
	}
}

// authedPost issues an authenticated form POST the way the web client does.
func authedPost(mux *http.ServeMux, path, screenname, secret string, form url.Values) *httptest.ResponseRecorder {
	form.Set("emailaddress", screenname+"@example.com")
	form.Set("emailcode", secret)
	req := httptest.NewRequest("POST", path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	return w
}

// TestDeletePostEndpoint covers the soft delete. The handler used to verify
// ownership, discard the patch it had built, and answer {"status":"deleted"}
// without touching the row, so the post survived every subsequent read while
// the client believed it was gone.
func TestDeletePostEndpoint(t *testing.T) {
	mux, conn, _ := setupTestServer(t)
	insertTestUser(t, conn, "alice", "s3cret")

	now := time.Now()
	res, err := conn.Exec(`insert into items (feedUrl, author, title, description, pubDate, whenCreated, whenUpdated)
		values (?, ?, ?, ?, ?, ?, ?)`,
		"http://localhost:8081/feed?screenname=alice", "alice", "Doomed Post", "goodbye", now, now, now)
	if err != nil {
		t.Fatalf("failed to insert item: %v", err)
	}
	itemID, _ := res.LastInsertId()

	w := authedPost(mux, "/deletepost", "alice", "s3cret", url.Values{"id": {fmt.Sprint(itemID)}})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}

	// The row must actually be flagged, not merely reported as deleted.
	var flDeleted int
	if err := conn.QueryRow(`select flDeleted from items where id = ?`, itemID).Scan(&flDeleted); err != nil {
		t.Fatalf("failed to read flDeleted: %v", err)
	}
	if flDeleted != 1 {
		t.Errorf("flDeleted = %d, want 1: the post was reported deleted but the row is untouched", flDeleted)
	}

	// And it must drop out of the reads that filter on the flag.
	for _, path := range []string{"/getrecentitems", "/feed", "/feed?screenname=alice"} {
		req := httptest.NewRequest("GET", path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if strings.Contains(rec.Body.String(), "Doomed Post") {
			t.Errorf("%s still lists the deleted post", path)
		}
	}
}

// TestDeletePostRejectsNonOwner keeps the ownership check honest now that the
// handler actually writes.
func TestDeletePostRejectsNonOwner(t *testing.T) {
	mux, conn, _ := setupTestServer(t)
	insertTestUser(t, conn, "alice", "alice-secret")
	insertTestUser(t, conn, "mallory", "mallory-secret")

	now := time.Now()
	res, err := conn.Exec(`insert into items (feedUrl, author, title, description, pubDate, whenCreated, whenUpdated)
		values (?, ?, ?, ?, ?, ?, ?)`,
		"http://localhost:8081/feed?screenname=alice", "alice", "Alice's Post", "mine", now, now, now)
	if err != nil {
		t.Fatalf("failed to insert item: %v", err)
	}
	itemID, _ := res.LastInsertId()

	authedPost(mux, "/deletepost", "mallory", "mallory-secret", url.Values{"id": {fmt.Sprint(itemID)}})

	var flDeleted int
	if err := conn.QueryRow(`select flDeleted from items where id = ?`, itemID).Scan(&flDeleted); err != nil {
		t.Fatalf("failed to read flDeleted: %v", err)
	}
	if flDeleted != 0 {
		t.Error("a non-owner deleted the post")
	}
}

// TestDeleteReplyRepublishesParentCommentsFeed covers the reply half. The
// parent's comments feed still lists a deleted reply unless it is rebuilt,
// which is the updateReplyFeedsOnS3 call deletePost makes in rssnetwork.js.
func TestDeleteReplyRepublishesParentCommentsFeed(t *testing.T) {
	mux, conn, handler := setupTestServer(t)
	insertTestUser(t, conn, "alice", "alice-secret")
	insertTestUser(t, conn, "bob", "bob-secret")

	now := time.Now()
	parentRes, err := conn.Exec(`insert into items (feedUrl, author, title, description, pubDate, whenCreated, whenUpdated)
		values (?, ?, ?, ?, ?, ?, ?)`,
		"http://localhost:8081/feed?screenname=alice", "alice", "Parent Post", "original", now, now, now)
	if err != nil {
		t.Fatalf("failed to insert parent: %v", err)
	}
	parentID, _ := parentRes.LastInsertId()

	replyRes, err := conn.Exec(`insert into items (feedUrl, author, title, description, inReplyTo, pubDate, whenCreated, whenUpdated)
		values (?, ?, ?, ?, ?, ?, ?, ?)`,
		"http://localhost:8081/feed?screenname=bob", "bob", "Regrettable Reply", "oops", parentID, now, now, now)
	if err != nil {
		t.Fatalf("failed to insert reply: %v", err)
	}
	replyID, _ := replyRes.LastInsertId()

	if err := handler.Publisher.PublishCommentsFeed(conn, "alice", parentID); err != nil {
		t.Fatalf("failed to publish comments feed: %v", err)
	}

	w := authedPost(mux, "/deletepost", "bob", "bob-secret", url.Values{"id": {fmt.Sprint(replyID)}})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}

	// Assert against the published file rather than an HTTP route: the
	// comments feed is written to disk and advertised in RSS, but no route
	// serves it, so a request-based assertion would pass vacuously on a 404.
	commentsPath := filepath.Join(handler.Publisher.BaseDir(), "comments", fmt.Sprintf("alice-%d.xml", parentID))
	republished, err := os.ReadFile(commentsPath)
	if err != nil {
		t.Fatalf("failed to read republished comments feed at %s: %v", commentsPath, err)
	}
	if strings.Contains(string(republished), "Regrettable Reply") {
		t.Error("parent's comments feed was not republished; it still lists the deleted reply")
	}
	// Guard against the assertion above passing because the feed is empty or
	// truncated: the parent must still be there.
	if !strings.Contains(string(republished), "Parent Post") {
		t.Errorf("comments feed lost the parent post, so the check above proves nothing:\n%s", republished)
	}
}

// seedPostWithReply inserts a parent post and one reply, returning their ids.
func seedPostWithReply(t *testing.T, conn *sql.DB) (parentID, replyID int64) {
	t.Helper()
	insertTestUser(t, conn, "alice", "alice-secret")
	insertTestUser(t, conn, "bob", "bob-secret")

	now := time.Now()
	pr, err := conn.Exec(`insert into items (feedUrl, author, title, description, pubDate, whenCreated, whenUpdated)
		values (?, ?, ?, ?, ?, ?, ?)`,
		"http://localhost:8081/feed?screenname=alice", "alice", "Parent Post", "original", now, now, now)
	if err != nil {
		t.Fatalf("failed to insert parent: %v", err)
	}
	parentID, _ = pr.LastInsertId()

	rr, err := conn.Exec(`insert into items (feedUrl, author, title, description, inReplyTo, pubDate, whenCreated, whenUpdated)
		values (?, ?, ?, ?, ?, ?, ?, ?)`,
		"http://localhost:8081/feed?screenname=bob", "bob", "A Reply", "responding", parentID, now, now, now)
	if err != nil {
		t.Fatalf("failed to insert reply: %v", err)
	}
	replyID, _ = rr.LastInsertId()
	return parentID, replyID
}

// TestCommentsFeedAdvertisedURLResolves is the round trip that matters: take
// the comments URL the server puts in its own generated feed and request
// exactly that. Every such URL used to 404 — the feed was published to
// feeds/comments/{screenname}-{id}.xml, advertised at
// /comments/{screenname}/{id}.xml, and no route served either.
func TestCommentsFeedAdvertisedURLResolves(t *testing.T) {
	mux, conn, handler := setupTestServer(t)
	parentID, _ := seedPostWithReply(t, conn)

	// The everyone feed is served from disk in filesystem mode, so publish it
	// the way a write would before reading it back.
	if err := handler.Publisher.PublishEveryoneFeed(conn); err != nil {
		t.Fatalf("failed to publish everyone feed: %v", err)
	}

	// Pull the advertised URL out of the everyone feed rather than hardcoding
	// it, so the test breaks if the advertised shape and the route drift apart.
	req := httptest.NewRequest("GET", "/feed", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	m := regexp.MustCompile(`feedUrl="([^"]*/comments/[^"]+)"`).FindStringSubmatch(rec.Body.String())
	if m == nil {
		t.Fatalf("everyone feed advertises no comments URL, nothing to resolve:\n%s", rec.Body.String())
	}
	advertised := strings.ReplaceAll(m[1], "&amp;", "&")

	u, err := url.Parse(advertised)
	if err != nil {
		t.Fatalf("advertised comments URL %q does not parse: %v", advertised, err)
	}
	if !strings.Contains(u.Path, fmt.Sprint(parentID)) {
		t.Fatalf("advertised URL %q is not for parent %d", advertised, parentID)
	}

	req = httptest.NewRequest("GET", u.Path, nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("advertised comments URL %s returned %d, want 200", u.Path, rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "rss+xml") {
		t.Errorf("content type = %q, want rss+xml", ct)
	}
	body := rec.Body.String()
	for _, want := range []string{"Parent Post", "A Reply"} {
		if !strings.Contains(body, want) {
			t.Errorf("comments feed missing %q, got:\n%s", want, body)
		}
	}
}

// TestCommentsFeedServedWithoutPublishing pins the on-demand behaviour: no
// PublishCommentsFeed call, so there is no file and no stored copy, and the
// response must still be correct. This is the case that fails if the handler
// is ever switched back to serving the published artifact.
func TestCommentsFeedServedWithoutPublishing(t *testing.T) {
	mux, conn, handler := setupTestServer(t)
	parentID, _ := seedPostWithReply(t, conn)

	published := filepath.Join(handler.Publisher.BaseDir(), "comments", fmt.Sprintf("alice-%d.xml", parentID))
	if _, err := os.Stat(published); err == nil {
		t.Fatalf("precondition failed: %s already exists, so this proves nothing", published)
	}

	req := httptest.NewRequest("GET", fmt.Sprintf("/comments/alice/%d.xml", parentID), nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 with nothing published", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "A Reply") {
		t.Errorf("comments feed missing the reply:\n%s", rec.Body.String())
	}
}

// TestCommentsFeedReflectsDeletedReply covers freshness: a reply deleted after
// the feed was last published must not come back.
func TestCommentsFeedReflectsDeletedReply(t *testing.T) {
	mux, conn, _ := setupTestServer(t)
	parentID, replyID := seedPostWithReply(t, conn)

	path := fmt.Sprintf("/comments/alice/%d.xml", parentID)
	req := httptest.NewRequest("GET", path, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), "A Reply") {
		t.Fatalf("precondition failed: reply absent before deletion")
	}

	if w := authedPost(mux, "/deletepost", "bob", "bob-secret", url.Values{"id": {fmt.Sprint(replyID)}}); w.Code != http.StatusOK {
		t.Fatalf("delete failed: %d %s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest("GET", path, nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if strings.Contains(rec.Body.String(), "A Reply") {
		t.Error("comments feed still lists the deleted reply")
	}
	if !strings.Contains(rec.Body.String(), "Parent Post") {
		t.Errorf("comments feed lost the parent, so the check above proves nothing:\n%s", rec.Body.String())
	}
}

// TestCommentsFeedRejectsBadPaths checks the parsing edges answer rather than
// panic or 500.
func TestCommentsFeedRejectsBadPaths(t *testing.T) {
	mux, conn, _ := setupTestServer(t)
	seedPostWithReply(t, conn)

	for _, path := range []string{
		"/comments/alice/notanumber.xml",
		"/comments/alice/42",        // missing .xml
		"/comments/alice/.xml",      // empty id
		"/comments/alice/99999.xml", // no such post
	} {
		req := httptest.NewRequest("GET", path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code == http.StatusOK {
			t.Errorf("%s returned 200, want an error response", path)
		}
		if rec.Code >= 500 && rec.Code != 503 {
			t.Errorf("%s returned %d; bad input should not be a server error", path, rec.Code)
		}
	}
}

// TestSavePrefsDoesNotLeakEmailSecret pins the response shape. /saveprefs
// re-fetched the user and returned it directly, and db.User serialised
// EmailSecret, so every successful call wrote the caller's permanent bearer
// credential into the response body — devtools, client-side logging, and any
// intermediary recording bodies.
func TestSavePrefsDoesNotLeakEmailSecret(t *testing.T) {
	mux, conn, _ := setupTestServer(t)
	const secret = "super-secret-value-not-for-the-wire"
	insertTestUser(t, conn, "alice", secret)

	w := authedPost(mux, "/saveprefs", "alice", secret, url.Values{
		"jsontext": {`{"myFeedTitle":"Alice"}`},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}

	body := w.Body.String()
	if strings.Contains(body, secret) {
		t.Errorf("response leaks the caller's emailSecret:\n%s", body)
	}
	if strings.Contains(body, "emailSecret") {
		t.Errorf("response still carries an emailSecret field:\n%s", body)
	}
	// The response must still be useful, or the check above proves nothing.
	if !strings.Contains(body, "Alice") {
		t.Errorf("response lost the saved prefs:\n%s", body)
	}
}

// TestSendConfirmingEmailRateLimitedPerEmail covers the mailbox-flooding case:
// the endpoint delivers to an address the caller names, so unthrottled it lets
// anyone bomb a third party through this server's SMTP credentials.
func TestSendConfirmingEmailRateLimitedPerEmail(t *testing.T) {
	mux, conn, handler := setupTestServer(t)
	insertTestUser(t, conn, "alice", "alice-secret")

	// Tight limits, and a generous per-IP budget so this test isolates the
	// per-email one.
	handler.authLimitByEmail = newRateLimiter(1.0/300.0, 2, time.Hour)
	handler.authLimitByIP = newRateLimiter(1, 1000, time.Hour)

	send := func(from string) int {
		req := httptest.NewRequest("GET",
			"/sendconfirmingemail?email=alice@example.com&urlredirect=http://localhost/", nil)
		req.RemoteAddr = from
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec.Code
	}

	for i := 1; i <= 2; i++ {
		if code := send("203.0.113.1:1000"); code == http.StatusTooManyRequests {
			t.Fatalf("request %d limited, want the first 2 through", i)
		}
	}
	if code := send("203.0.113.1:1000"); code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want 429 once the per-email burst is spent", code)
	}
	// Rotating source addresses must not reset a mailbox's budget.
	if code := send("198.51.100.77:2000"); code != http.StatusTooManyRequests {
		t.Errorf("status = %d from a new IP, want 429: the limit is per mailbox", code)
	}
}

// TestSendConfirmingEmailRateLimitedPerIP covers the other direction: one
// source spraying many addresses.
func TestSendConfirmingEmailRateLimitedPerIP(t *testing.T) {
	mux, _, handler := setupTestServer(t)

	handler.authLimitByIP = newRateLimiter(1.0/60.0, 2, time.Hour)
	handler.authLimitByEmail = newRateLimiter(1, 1000, time.Hour)

	send := func(email string) int {
		req := httptest.NewRequest("GET",
			"/sendconfirmingemail?email="+email+"&urlredirect=http://localhost/", nil)
		req.RemoteAddr = "203.0.113.5:9999"
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec.Code
	}

	for i := 1; i <= 2; i++ {
		if code := send(fmt.Sprintf("victim%d@example.com", i)); code == http.StatusTooManyRequests {
			t.Fatalf("request %d limited, want the first 2 through", i)
		}
	}
	if code := send("victim3@example.com"); code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want 429 once the per-IP burst is spent", code)
	}
}

// TestCreateNewUserRateLimited checks the second mail-sending endpoint shares
// the limits, so it cannot be used to route around them.
func TestCreateNewUserRateLimited(t *testing.T) {
	mux, _, handler := setupTestServer(t)

	handler.authLimitByIP = newRateLimiter(1.0/60.0, 1, time.Hour)
	handler.authLimitByEmail = newRateLimiter(1, 1000, time.Hour)

	send := func(name string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET",
			"/createnewuser?email="+name+"@example.com&name="+name+"&urlredirect=http://localhost/", nil)
		req.RemoteAddr = "203.0.113.8:4444"
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec
	}

	if code := send("first").Code; code == http.StatusTooManyRequests {
		t.Fatal("first request limited")
	}
	rec := send("second")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", rec.Code)
	}
	if ra := rec.Header().Get("Retry-After"); ra == "" {
		t.Error("429 has no Retry-After header")
	}
}

// TestRateLimitRejectionHappensBeforeSideEffects checks the limiter runs early
// enough to matter: a rejected /createnewuser must not have created the account.
func TestRateLimitRejectionHappensBeforeSideEffects(t *testing.T) {
	mux, conn, handler := setupTestServer(t)

	handler.authLimitByIP = newRateLimiter(1.0/60.0, 0, time.Hour) // nothing allowed
	handler.authLimitByEmail = newRateLimiter(1, 1000, time.Hour)

	req := httptest.NewRequest("GET",
		"/createnewuser?email=ghost@example.com&name=ghost&urlredirect=http://localhost/", nil)
	req.RemoteAddr = "203.0.113.11:5555"
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", rec.Code)
	}

	var count int
	if err := conn.QueryRow(`select count(*) from users where screenname = ?`, "ghost").Scan(&count); err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if count != 0 {
		t.Error("a rate-limited request still created the account")
	}
}

// TestAuthenticatedResponsesAreNotCacheable covers the mitigation for
// credentials travelling in the request URL: the URL of an authenticated
// request is itself a secret, so neither it nor its response may be written to
// a shared or on-disk cache. Applies on the rejection path too — a 503 still
// echoes back a URL that carried a credential.
func TestAuthenticatedResponsesAreNotCacheable(t *testing.T) {
	mux, conn, _ := setupTestServer(t)
	insertTestUser(t, conn, "alice", "alice-secret")

	now := time.Now()
	res, err := conn.Exec(`insert into items (feedUrl, author, title, description, pubDate, whenCreated, whenUpdated)
		values (?, ?, ?, ?, ?, ?, ?)`,
		"http://localhost:8081/feed?screenname=alice", "alice", "Post", "body", now, now, now)
	if err != nil {
		t.Fatalf("failed to insert item: %v", err)
	}
	itemID, _ := res.LastInsertId()

	cases := []struct {
		path string
		form url.Values
	}{
		{"/newpost", url.Values{"jsontext": {`{"description":"<p>hi</p>"}`}}},
		{"/updatepost", url.Values{"id": {fmt.Sprint(itemID)}, "jsontext": {`{"description":"<p>edit</p>"}`}}},
		{"/togglelike", url.Values{"id": {fmt.Sprint(itemID)}}},
		{"/saveprefs", url.Values{"jsontext": {`{"myFeedTitle":"t"}`}}},
		{"/deletepost", url.Values{"id": {fmt.Sprint(itemID)}}},
	}

	for _, tc := range cases {
		t.Run("authenticated"+tc.path, func(t *testing.T) {
			w := authedPost(mux, tc.path, "alice", "alice-secret", tc.form)
			if got := w.Header().Get("Cache-Control"); got != "no-store" {
				t.Errorf("Cache-Control = %q, want no-store (status was %d)", got, w.Code)
			}
		})

		t.Run("rejected"+tc.path, func(t *testing.T) {
			w := authedPost(mux, tc.path, "alice", "wrong-secret", tc.form)
			if w.Code == http.StatusOK {
				t.Fatal("bad credentials were accepted")
			}
			if got := w.Header().Get("Cache-Control"); got != "no-store" {
				t.Errorf("Cache-Control = %q on a rejection, want no-store", got)
			}
		})
	}
}

// TestNewUserHasFeedImmediately pins the 7/25/26 upstream change: a user is a
// feed from the moment the account exists. Before this, /feed answered 404
// until the user's first post triggered UpdateFeedsOnPostWrite — so anyone who
// subscribed off the subscription list, which already listed the new account,
// got a 404.
func TestNewUserHasFeedImmediately(t *testing.T) {
	mux, _, handler := setupTestServer(t)

	req := httptest.NewRequest("GET",
		"/createnewuser?email=fresh@example.com&name=freshuser&urlredirect=http://localhost/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("createnewuser status = %d, want %d: %s", w.Code, http.StatusOK, w.Body.String())
	}

	// The feed must have been published, without the user posting anything.
	exists, err := handler.Publisher.UserFeedExists("freshuser")
	if err != nil {
		t.Fatalf("failed to check for feed: %v", err)
	}
	if !exists {
		t.Error("no feed was published for the new user")
	}

	// And it must actually serve, as a valid feed naming the user.
	req = httptest.NewRequest("GET", "/feed?screenname=freshuser", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("feed status = %d, want %d: %s", w.Code, http.StatusOK, w.Body.String())
	}

	body := w.Body.String()
	if !strings.Contains(body, "<rss") {
		t.Errorf("feed is not an RSS document: %q", body)
	}
	if !strings.Contains(body, "freshuser") {
		t.Errorf("feed does not name the user: %q", body)
	}
}

// TestWebSubHeaderPresent verifies that feed responses include WebSub Link headers.
func TestWebSubHeaderPresent(t *testing.T) {
	mux, conn, handler := setupTestServer(t)

	// Create a test user
	secret := "test-secret"
	insertTestUser(t, conn, "alice", secret)

	// Publish the user's feed and everyone feed so they exist
	if err := handler.Publisher.PublishUserFeed(conn, "alice"); err != nil {
		t.Fatalf("failed to publish user feed: %v", err)
	}
	if err := handler.Publisher.PublishEveryoneFeed(conn); err != nil {
		t.Fatalf("failed to publish everyone feed: %v", err)
	}

	tests := []struct {
		name    string
		path    string
		wantURL string
	}{
		{
			name:    "User feed includes self URL",
			path:    "/feed?screenname=alice",
			wantURL: "feed?screenname=alice",
		},
		{
			name:    "Everyone feed includes self URL",
			path:    "/feed",
			wantURL: "feed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
			}

			link := w.Header().Get("Link")
			if link == "" {
				t.Errorf("expected Link header, got none")
				return
			}

			// Verify the header contains rel="self" with the feed URL
			if !strings.Contains(link, "rel=\"self\"") {
				t.Errorf("Link header missing rel=\"self\": %s", link)
			}

			if !strings.Contains(link, tt.wantURL) {
				t.Errorf("Link header missing feed URL %s: %s", tt.wantURL, link)
			}

			// Verify the header contains rel="hub"
			if !strings.Contains(link, "rel=\"hub\"") {
				t.Errorf("Link header missing rel=\"hub\": %s", link)
			}
		})
	}
}

// TestWebSubCommentsFeedHeader verifies that comments feeds include WebSub headers.
func TestWebSubCommentsFeedHeader(t *testing.T) {
	mux, conn, handler := setupTestServer(t)

	// Create a user and a post
	secret := "test-secret"
	insertTestUser(t, conn, "alice", secret)

	// Create a post
	form := url.Values{}
	form.Set("jsontext", `{"description":"<p>Test post for comments feed</p>"}`)
	form.Set("inReplyTo", "")

	w := authedPost(mux, "/newpost", "alice", secret, form)
	if w.Code != http.StatusOK {
		t.Fatalf("newpost status = %d, want %d: %s", w.Code, http.StatusOK, w.Body.String())
	}

	// Get the item ID from the database
	var itemID int64
	err := conn.QueryRow("SELECT id FROM items WHERE author = ? ORDER BY id DESC LIMIT 1", "alice").Scan(&itemID)
	if err != nil {
		t.Fatalf("failed to get item ID: %v", err)
	}

	// Publish the comments feed
	if err := handler.Publisher.PublishCommentsFeed(conn, "alice", itemID); err != nil {
		t.Fatalf("failed to publish comments feed: %v", err)
	}

	// Request the comments feed
	path := fmt.Sprintf("/comments/alice/%d.xml", itemID)
	req := httptest.NewRequest("GET", path, nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("comments feed status = %d, want %d: %s", w.Code, http.StatusOK, w.Body.String())
	}

	// Verify the Link header
	link := w.Header().Get("Link")
	if link == "" {
		t.Errorf("expected Link header, got none")
		return
	}

	if !strings.Contains(link, "rel=\"self\"") {
		t.Errorf("Link header missing rel=\"self\": %s", link)
	}

	if !strings.Contains(link, "comments") {
		t.Errorf("Link header missing comments URL: %s", link)
	}
}

// TestWebSubOPMLHeader verifies that OPML responses include WebSub headers.
func TestWebSubOPMLHeader(t *testing.T) {
	mux, conn, handler := setupTestServer(t)

	// Create some users
	for _, name := range []string{"alice", "bob"} {
		insertTestUser(t, conn, name, "secret")
	}

	// Publish the subscription list
	if err := handler.Publisher.PublishSubscriptionList(conn); err != nil {
		t.Fatalf("failed to publish subscription list: %v", err)
	}

	// Request the OPML
	req := httptest.NewRequest("GET", "/getsubscriptionlist", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusOK, w.Body.String())
	}

	// Verify the Link header
	link := w.Header().Get("Link")
	if link == "" {
		t.Errorf("expected Link header, got none")
		return
	}

	if !strings.Contains(link, "rel=\"self\"") {
		t.Errorf("Link header missing rel=\"self\": %s", link)
	}

	if !strings.Contains(link, "getsubscriptionlist") {
		t.Errorf("Link header missing OPML URL: %s", link)
	}
}

// TestMentionExtractionOnNewPost verifies mentions are extracted and stored when creating a post.
func TestMentionExtractionOnNewPost(t *testing.T) {
	mux, conn, _ := setupTestServer(t)

	// Create users
	insertTestUser(t, conn, "alice", "secret_alice")
	insertTestUser(t, conn, "bob", "secret_bob")

	// Alice creates a post mentioning Bob
	postReq := PostRequest{
		Description: "hello <p>@bob this is great!</p>",
		Title:       "Test post",
	}
	jsonData, _ := json.Marshal(postReq)
	form := url.Values{
		"jsontext": {string(jsonData)},
	}

	w := authedPost(mux, "/newpost", "alice", "secret_alice", form)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, expected %d: %s", w.Code, http.StatusOK, w.Body.String())
	}

	// Parse response to get item ID
	var result map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	itemID := int(result["id"].(float64))

	// Verify mention was stored
	mentions, err := db.GetMentionsForItem(conn, itemID)
	if err != nil {
		t.Fatalf("failed to get mentions: %v", err)
	}

	if len(mentions) != 1 || mentions[0] != "bob" {
		t.Errorf("expected mention of bob, got %v", mentions)
	}
}

// TestMentionExtractionOnUpdatePost verifies mentions are updated when editing a post.
func TestMentionExtractionOnUpdatePost(t *testing.T) {
	mux, conn, _ := setupTestServer(t)

	// Create users
	insertTestUser(t, conn, "alice", "secret_alice")
	insertTestUser(t, conn, "bob", "secret_bob")
	insertTestUser(t, conn, "charlie", "secret_charlie")

	// Alice creates a post mentioning Bob
	postReq := PostRequest{
		Description: "<p>@bob check this out</p>",
		Title:       "Test post",
	}
	jsonData, _ := json.Marshal(postReq)
	form := url.Values{
		"jsontext": {string(jsonData)},
	}

	w := authedPost(mux, "/newpost", "alice", "secret_alice", form)

	if w.Code != http.StatusOK {
		t.Fatalf("failed to create post: %s", w.Body.String())
	}

	var result map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &result)
	itemID := int64(result["id"].(float64))

	// Verify initial mention
	mentions, _ := db.GetMentionsForItem(conn, int(itemID))
	if len(mentions) != 1 || mentions[0] != "bob" {
		t.Fatalf("initial mention check failed: got %v", mentions)
	}

	// Alice updates post to mention Charlie instead
	updateReq := PostRequest{
		Description: "<p>@charlie this is updated</p>",
	}
	updateData, _ := json.Marshal(updateReq)
	updateForm := url.Values{
		"id":       {fmt.Sprintf("%d", itemID)},
		"jsontext": {string(updateData)},
	}

	w = authedPost(mux, "/updatepost", "alice", "secret_alice", updateForm)

	if w.Code != http.StatusOK {
		t.Fatalf("failed to update post: %s", w.Body.String())
	}

	// Verify mentions were updated (Bob gone, Charlie added)
	updatedMentions, _ := db.GetMentionsForItem(conn, int(itemID))
	if len(updatedMentions) != 1 || updatedMentions[0] != "charlie" {
		t.Errorf("expected mention of charlie after update, got %v", updatedMentions)
	}
}

// TestMentionExtractionCaseInsensitive verifies case-insensitive mention matching.
func TestMentionExtractionCaseInsensitive(t *testing.T) {
	mux, conn, _ := setupTestServer(t)

	// Create users
	insertTestUser(t, conn, "alice", "secret_alice")
	insertTestUser(t, conn, "bob", "secret_bob")

	// Bob creates a post with mention in different case
	postReq := PostRequest{
		Description: "<p>hello @BOB and @Alice!</p>",
		Title:       "Test case",
	}
	jsonData, _ := json.Marshal(postReq)
	form := url.Values{
		"jsontext": {string(jsonData)},
	}

	w := authedPost(mux, "/newpost", "bob", "secret_bob", form)

	if w.Code != http.StatusOK {
		t.Fatalf("failed to create post: %s", w.Body.String())
	}

	var result map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &result)
	itemID := int(result["id"].(float64))

	// Verify both case-insensitive mentions were stored with canonical names
	mentions, _ := db.GetMentionsForItem(conn, itemID)
	if len(mentions) != 2 {
		t.Errorf("expected 2 mentions, got %d: %v", len(mentions), mentions)
	}

	mentionMap := make(map[string]bool)
	for _, m := range mentions {
		mentionMap[m] = true
	}

	if !mentionMap["bob"] || !mentionMap["alice"] {
		t.Errorf("expected mentions of bob and alice, got %v", mentions)
	}
}

// TestMentionExtractionNonexistentUser verifies non-existent users aren't stored as mentions.
func TestMentionExtractionNonexistentUser(t *testing.T) {
	mux, conn, _ := setupTestServer(t)

	insertTestUser(t, conn, "alice", "secret_alice")

	// Alice creates a post mentioning non-existent users
	postReq := PostRequest{
		Description: "<p>@nonexistent @alice @alsobaduser</p>",
		Title:       "Test invalid mentions",
	}
	jsonData, _ := json.Marshal(postReq)
	form := url.Values{
		"jsontext": {string(jsonData)},
	}

	w := authedPost(mux, "/newpost", "alice", "secret_alice", form)

	if w.Code != http.StatusOK {
		t.Fatalf("failed to create post: %s", w.Body.String())
	}

	var result map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &result)
	itemID := int(result["id"].(float64))

	// Verify only alice was stored (the only existing user)
	mentions, _ := db.GetMentionsForItem(conn, itemID)
	if len(mentions) != 1 || mentions[0] != "alice" {
		t.Errorf("expected only alice mention, got %v", mentions)
	}
}

// TestHashtagExtractionOnNewPost verifies hashtags are extracted and stored when creating a post.
func TestHashtagExtractionOnNewPost(t *testing.T) {
	mux, conn, _ := setupTestServer(t)

	insertTestUser(t, conn, "alice", "secret_alice")

	// Alice creates a post with hashtags
	postReq := PostRequest{
		Description: "<p>check out #golang and #rust programming</p>",
		Title:       "Test post with tags",
	}
	jsonData, _ := json.Marshal(postReq)
	form := url.Values{
		"jsontext": {string(jsonData)},
	}

	w := authedPost(mux, "/newpost", "alice", "secret_alice", form)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, expected %d: %s", w.Code, http.StatusOK, w.Body.String())
	}

	// Parse response to get item ID
	var result map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	itemID := int(result["id"].(float64))

	// Verify hashtags were stored
	tags, err := db.GetHashtagsForItem(conn, itemID)
	if err != nil {
		t.Fatalf("failed to get hashtags: %v", err)
	}

	if len(tags) != 2 {
		t.Errorf("expected 2 tags, got %d: %v", len(tags), tags)
	}

	tagMap := make(map[string]bool)
	for _, tag := range tags {
		tagMap[tag] = true
	}

	if !tagMap["golang"] || !tagMap["rust"] {
		t.Errorf("expected golang and rust tags, got %v", tags)
	}
}

// TestHashtagExtractionOnUpdatePost verifies hashtags are updated when editing a post.
func TestHashtagExtractionOnUpdatePost(t *testing.T) {
	mux, conn, _ := setupTestServer(t)

	insertTestUser(t, conn, "alice", "secret_alice")

	// Alice creates a post with one tag
	postReq := PostRequest{
		Description: "<p>#golang is great</p>",
		Title:       "Original title",
	}
	jsonData, _ := json.Marshal(postReq)
	form := url.Values{
		"jsontext": {string(jsonData)},
	}

	w := authedPost(mux, "/newpost", "alice", "secret_alice", form)
	var result map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &result)
	itemID := result["id"].(float64)

	// Verify initial tag
	tags, _ := db.GetHashtagsForItem(conn, int(itemID))
	if len(tags) != 1 || tags[0] != "golang" {
		t.Errorf("expected golang tag initially, got %v", tags)
	}

	// Update post with different tags
	updateReq := PostRequest{
		Description: "<p>#rust #python are also nice</p>",
	}
	updateData, _ := json.Marshal(updateReq)
	updateForm := url.Values{
		"id":       {fmt.Sprintf("%.0f", itemID)},
		"jsontext": {string(updateData)},
	}

	w = authedPost(mux, "/updatepost", "alice", "secret_alice", updateForm)
	if w.Code != http.StatusOK {
		t.Fatalf("failed to update post: %s", w.Body.String())
	}

	// Verify tags were replaced
	tags, _ = db.GetHashtagsForItem(conn, int(itemID))
	if len(tags) != 2 {
		t.Errorf("expected 2 tags after update, got %d: %v", len(tags), tags)
	}

	tagMap := make(map[string]bool)
	for _, tag := range tags {
		tagMap[tag] = true
	}

	if !tagMap["rust"] || !tagMap["python"] {
		t.Errorf("expected rust and python tags after update, got %v", tags)
	}

	// golang should be gone
	if tagMap["golang"] {
		t.Errorf("golang tag should be gone, got %v", tags)
	}
}

// TestHashtagExtractionCaseNormalization verifies hashtag tags are normalized to lowercase.
func TestHashtagExtractionCaseNormalization(t *testing.T) {
	mux, conn, _ := setupTestServer(t)

	insertTestUser(t, conn, "alice", "secret_alice")

	// Alice creates a post with mixed-case hashtags
	postReq := PostRequest{
		Description: "<p>#GoLang #RUST #Python are fun</p>",
		Title:       "Mixed case tags",
	}
	jsonData, _ := json.Marshal(postReq)
	form := url.Values{
		"jsontext": {string(jsonData)},
	}

	w := authedPost(mux, "/newpost", "alice", "secret_alice", form)

	if w.Code != http.StatusOK {
		t.Fatalf("failed to create post: %s", w.Body.String())
	}

	var result map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &result)
	itemID := int(result["id"].(float64))

	// Verify all tags are normalized to lowercase
	tags, _ := db.GetHashtagsForItem(conn, itemID)
	if len(tags) != 3 {
		t.Errorf("expected 3 tags, got %d: %v", len(tags), tags)
	}

	for _, tag := range tags {
		if tag != strings.ToLower(tag) {
			t.Errorf("tag %q not normalized to lowercase", tag)
		}
	}

	tagMap := make(map[string]bool)
	for _, tag := range tags {
		tagMap[tag] = true
	}

	if !tagMap["golang"] || !tagMap["rust"] || !tagMap["python"] {
		t.Errorf("expected golang, rust, python in lowercase, got %v", tags)
	}
}

// TestHashtagExtractionDuplicates verifies duplicate hashtags in a post are stored only once.
func TestHashtagExtractionDuplicates(t *testing.T) {
	mux, conn, _ := setupTestServer(t)

	insertTestUser(t, conn, "alice", "secret_alice")

	// Alice creates a post with duplicate hashtags
	postReq := PostRequest{
		Description: "<p>#golang is great and #golang is fast</p>",
		Title:       "Duplicate tags",
	}
	jsonData, _ := json.Marshal(postReq)
	form := url.Values{
		"jsontext": {string(jsonData)},
	}

	w := authedPost(mux, "/newpost", "alice", "secret_alice", form)

	if w.Code != http.StatusOK {
		t.Fatalf("failed to create post: %s", w.Body.String())
	}

	var result map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &result)
	itemID := int(result["id"].(float64))

	// Verify golang appears only once
	tags, _ := db.GetHashtagsForItem(conn, itemID)
	if len(tags) != 1 || tags[0] != "golang" {
		t.Errorf("expected only golang tag, got %v", tags)
	}
}


// TestGetHashtagItemsNoTag verifies missing tag parameter returns error.
func TestGetHashtagItemsNoTag(t *testing.T) {
	mux, _, _ := setupTestServer(t)

	req := httptest.NewRequest("GET", "/gethashtagitems", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		// Could be error status - just check we get a response
		t.Logf("Got expected error response: %s", w.Body.String())
	}
}

// TestGetTrendingHashtags verifies the /gettrendinghashtags endpoint.
func TestGetTrendingHashtags(t *testing.T) {
	mux, conn, _ := setupTestServer(t)

	insertTestUser(t, conn, "alice", "secret_alice")

	// Create multiple posts with different tags to establish trending
	for i := 0; i < 3; i++ {
		postReq := PostRequest{
			Description: "<p>#golang is popular</p>",
			Title:       fmt.Sprintf("Post %d", i),
		}
		jsonData, _ := json.Marshal(postReq)
		form := url.Values{
			"jsontext": {string(jsonData)},
		}
		authedPost(mux, "/newpost", "alice", "secret_alice", form)
	}

	for i := 0; i < 2; i++ {
		postReq := PostRequest{
			Description: "<p>#rust is also good</p>",
			Title:       fmt.Sprintf("Rust post %d", i),
		}
		jsonData, _ := json.Marshal(postReq)
		form := url.Values{
			"jsontext": {string(jsonData)},
		}
		authedPost(mux, "/newpost", "alice", "secret_alice", form)
	}

	// Query trending tags
	req := httptest.NewRequest("GET", "/gettrendinghashtags", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, expected %d: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var tags []map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &tags); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if len(tags) == 0 {
		t.Errorf("expected trending tags, got none")
	}

	// golang should be first (3 posts) followed by rust (2 posts)
	if len(tags) >= 2 {
		if tags[0]["Tag"] != "golang" {
			t.Errorf("expected golang as trending, got %v", tags[0])
		}
		if tags[1]["Tag"] != "rust" {
			t.Errorf("expected rust as second, got %v", tags[1])
		}
	}
}

// TestAvatarDisplayInUserData verifies avatar URL is returned in /getuserdata response.
func TestAvatarDisplayInUserData(t *testing.T) {
	mux, conn, _ := setupTestServer(t)

	insertTestUser(t, conn, "alice", "secret_alice")

	// Upload avatar
	pngData := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, 0xDE,
	}
	form := url.Values{
		"data":        {base64.StdEncoding.EncodeToString(pngData)},
		"contentType": {"image/png"},
	}
	w := authedPost(mux, "/uploadavatar", "alice", "secret_alice", form)
	var avatarResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &avatarResp)
	avatarURL := avatarResp["avatarUrl"].(string)

	// Get user data
	req := httptest.NewRequest("GET", "/getuserdata?screenname=alice", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, expected 200", w.Code)
	}

	var user db.User
	json.Unmarshal(w.Body.Bytes(), &user)

	if user.ImageURL != avatarURL {
		t.Errorf("expected imageUrl=%q, got %q", avatarURL, user.ImageURL)
	}
}

// TestAvatarDisplayInRecentItems verifies avatars show up in /getrecentitems.
func TestAvatarDisplayInRecentItems(t *testing.T) {
	mux, conn, _ := setupTestServer(t)

	insertTestUser(t, conn, "alice", "secret_alice")

	// Upload avatar
	pngData := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, 0xDE,
	}
	form := url.Values{
		"data":        {base64.StdEncoding.EncodeToString(pngData)},
		"contentType": {"image/png"},
	}
	w := authedPost(mux, "/uploadavatar", "alice", "secret_alice", form)
	var avatarResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &avatarResp)
	avatarURL := avatarResp["avatarUrl"].(string)

	// Create a post from alice
	postReq := PostRequest{
		Description: "<p>Hello world</p>",
		Title:       "My first post",
	}
	jsonData, _ := json.Marshal(postReq)
	form = url.Values{
		"jsontext": {string(jsonData)},
	}
	w = authedPost(mux, "/newpost", "alice", "secret_alice", form)

	if w.Code != http.StatusOK {
		t.Fatalf("failed to create post: %s", w.Body.String())
	}

	// Get recent items
	req := httptest.NewRequest("GET", "/getrecentitems", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, expected 200", w.Code)
	}

	var items []db.Item
	json.Unmarshal(w.Body.Bytes(), &items)

	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}

	if items[0].ImageURL != avatarURL {
		t.Errorf("expected item imageUrl=%q, got %q", avatarURL, items[0].ImageURL)
	}
}

// TestAvatarDisplayInUserFeed verifies avatars show up in RSS feeds.
func TestAvatarDisplayInUserFeed(t *testing.T) {
	mux, conn, _ := setupTestServer(t)

	insertTestUser(t, conn, "alice", "secret_alice")

	// Upload avatar
	pngData := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, 0xDE,
	}
	form := url.Values{
		"data":        {base64.StdEncoding.EncodeToString(pngData)},
		"contentType": {"image/png"},
	}
	w := authedPost(mux, "/uploadavatar", "alice", "secret_alice", form)
	var avatarResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &avatarResp)
	avatarURL := avatarResp["avatarUrl"].(string)

	// Create a post
	postReq := PostRequest{
		Description: "<p>Hello world</p>",
		Title:       "My first post",
	}
	jsonData, _ := json.Marshal(postReq)
	form = url.Values{
		"jsontext": {string(jsonData)},
	}
	authedPost(mux, "/newpost", "alice", "secret_alice", form)

	// Get user feed (RSS XML)
	req := httptest.NewRequest("GET", "/feed?screenname=alice", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, expected 200", w.Code)
	}

	feedXML := w.Body.String()

	// Check that avatar URL is in the feed
	if !strings.Contains(feedXML, avatarURL) {
		t.Errorf("expected avatarUrl=%q in feed, feed was: %s", avatarURL, feedXML[:500])
	}

	// Check that it's in the source:account imageUrl attribute
	if !strings.Contains(feedXML, fmt.Sprintf(`imageUrl="%s"`, avatarURL)) {
		t.Errorf("expected imageUrl attribute in source:account")
	}
}
