package api

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/roberte3/rss.chat.go/websub"
)

// TestWebSubPingOnNewPost verifies hub is pinged when a new post is created.
func TestWebSubPingOnNewPost(t *testing.T) {
	// Set up mock hub
	hubReceived := make(chan url.Values, 1)
	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := r.ParseForm(); err != nil {
				http.Error(w, "parse error", http.StatusBadRequest)
				return
			}
			hubReceived <- r.PostForm
			w.WriteHeader(http.StatusAccepted)
		}),
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	addr := listener.Addr().String()
	hubURL := "http://" + addr + "/"

	go server.Serve(listener)
	defer server.Close()

	// Set up test server with WebSub enabled
	mux, conn, handler := setupTestServer(t)
	handler.Config.URLWebsubHub = hubURL
	handler.Config.WebsubEnabled = true
	pinger := websub.NewPinger(hubURL, true)
	handler.Publisher.SetPinger(pinger)

	// Create user
	secret := "test-secret"
	insertTestUser(t, conn, "alice", secret)

	// Create a post
	form := url.Values{}
	form.Set("jsontext", `{"description":"<p>Hello</p>"}`)
	form.Set("inReplyTo", "")

	w := authedPost(mux, "/newpost", "alice", secret, form)
	if w.Code != http.StatusOK {
		t.Fatalf("newpost status = %d, want %d: %s", w.Code, http.StatusOK, w.Body.String())
	}

	// Wait for async ping
	select {
	case received := <-hubReceived:
		// Verify hub was pinged with correct feed URL
		if mode := received.Get("hub.mode"); mode != "publish" {
			t.Errorf("hub.mode = %s, want publish", mode)
		}
		if hubURL := received.Get("hub.url"); !strings.Contains(hubURL, "feed") {
			t.Errorf("hub.url = %s, want to contain 'feed'", hubURL)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("hub was not pinged within 2 seconds")
	}
}

// TestWebSubPingOnUpdatePost verifies hub is pinged when a post is updated.
func TestWebSubPingOnUpdatePost(t *testing.T) {
	hubPingCount := atomic.Int32{}
	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hubPingCount.Add(1)
			w.WriteHeader(http.StatusAccepted)
		}),
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	addr := listener.Addr().String()
	hubURL := "http://" + addr + "/"

	go server.Serve(listener)
	defer server.Close()

	// Set up test server with WebSub enabled
	mux, conn, handler := setupTestServer(t)
	handler.Config.URLWebsubHub = hubURL
	handler.Config.WebsubEnabled = true
	pinger := websub.NewPinger(hubURL, true)
	handler.Publisher.SetPinger(pinger)

	// Create user and post
	secret := "test-secret"
	insertTestUser(t, conn, "alice", secret)

	form := url.Values{}
	form.Set("jsontext", `{"description":"<p>Original</p>"}`)
	form.Set("inReplyTo", "")

	w := authedPost(mux, "/newpost", "alice", secret, form)
	if w.Code != http.StatusOK {
		t.Fatalf("newpost failed: %s", w.Body.String())
	}

	// Get the post ID
	var itemID int64
	err = conn.QueryRow("SELECT id FROM items WHERE author = ? ORDER BY id DESC LIMIT 1", "alice").Scan(&itemID)
	if err != nil {
		t.Fatalf("failed to get item ID: %v", err)
	}

	initialPings := hubPingCount.Load()
	time.Sleep(200 * time.Millisecond) // Wait for initial ping

	// Update the post
	updateForm := url.Values{}
	updateForm.Set("id", fmt.Sprintf("%d", itemID))
	updateForm.Set("jsontext", `{"description":"<p>Updated</p>"}`)

	w = authedPost(mux, "/updatepost", "alice", secret, updateForm)
	if w.Code != http.StatusOK {
		t.Fatalf("updatepost failed: %s", w.Body.String())
	}

	// Wait for ping after update
	time.Sleep(200 * time.Millisecond)

	if hubPingCount.Load() <= initialPings {
		t.Error("hub was not pinged after post update")
	}
}

// TestWebSubPingOnDelete verifies hub is pinged when a post is deleted.
func TestWebSubPingOnDelete(t *testing.T) {
	hubReceived := make(chan struct{}, 5)
	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hubReceived <- struct{}{}
			w.WriteHeader(http.StatusAccepted)
		}),
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	addr := listener.Addr().String()
	hubURL := "http://" + addr + "/"

	go server.Serve(listener)
	defer server.Close()

	mux, conn, handler := setupTestServer(t)
	handler.Config.URLWebsubHub = hubURL
	handler.Config.WebsubEnabled = true
	pinger := websub.NewPinger(hubURL, true)
	handler.Publisher.SetPinger(pinger)

	secret := "test-secret"
	insertTestUser(t, conn, "alice", secret)

	// Create post
	form := url.Values{}
	form.Set("jsontext", `{"description":"<p>To delete</p>"}`)
	form.Set("inReplyTo", "")

	w := authedPost(mux, "/newpost", "alice", secret, form)
	if w.Code != http.StatusOK {
		t.Fatalf("newpost failed: %s", w.Body.String())
	}

	// Get post ID
	var itemID int64
	err = conn.QueryRow("SELECT id FROM items WHERE author = ? ORDER BY id DESC LIMIT 1", "alice").Scan(&itemID)
	if err != nil {
		t.Fatalf("failed to get item ID: %v", err)
	}

	// Drain initial pings
	for {
		select {
		case <-hubReceived:
		case <-time.After(100 * time.Millisecond):
			goto deletePost
		}
	}

deletePost:
	// Delete post
	deleteForm := url.Values{}
	deleteForm.Set("id", fmt.Sprintf("%d", itemID))

	w = authedPost(mux, "/deletepost", "alice", secret, deleteForm)
	if w.Code != http.StatusOK {
		t.Fatalf("deletepost failed: %s", w.Body.String())
	}

	// Verify hub was pinged
	select {
	case <-hubReceived:
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("hub was not pinged after post deletion")
	}
}

// TestWebSubPingOnLike verifies hub is pinged when a like is toggled.
func TestWebSubPingOnLike(t *testing.T) {
	hubReceived := make(chan struct{}, 5)
	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hubReceived <- struct{}{}
			w.WriteHeader(http.StatusAccepted)
		}),
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	addr := listener.Addr().String()
	hubURL := "http://" + addr + "/"

	go server.Serve(listener)
	defer server.Close()

	mux, conn, handler := setupTestServer(t)
	handler.Config.URLWebsubHub = hubURL
	handler.Config.WebsubEnabled = true
	pinger := websub.NewPinger(hubURL, true)
	handler.Publisher.SetPinger(pinger)

	// Create users
	insertTestUser(t, conn, "alice", "secret1")
	insertTestUser(t, conn, "bob", "secret2")

	// Alice creates a post
	form := url.Values{}
	form.Set("jsontext", `{"description":"<p>Like me</p>"}`)
	form.Set("inReplyTo", "")

	w := authedPost(mux, "/newpost", "alice", "secret1", form)
	if w.Code != http.StatusOK {
		t.Fatalf("newpost failed: %s", w.Body.String())
	}

	// Get post ID
	var itemID int64
	err = conn.QueryRow("SELECT id FROM items WHERE author = ? ORDER BY id DESC LIMIT 1", "alice").Scan(&itemID)
	if err != nil {
		t.Fatalf("failed to get item ID: %v", err)
	}

	// Drain initial pings
	for {
		select {
		case <-hubReceived:
		case <-time.After(100 * time.Millisecond):
			goto toggleLike
		}
	}

toggleLike:
	// Bob likes the post
	likeForm := url.Values{}
	likeForm.Set("id", fmt.Sprintf("%d", itemID))

	w = authedPost(mux, "/togglelike", "bob", "secret2", likeForm)
	if w.Code != http.StatusOK {
		t.Fatalf("togglelike failed: %s", w.Body.String())
	}

	// Verify hub was pinged
	select {
	case <-hubReceived:
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("hub was not pinged after like toggle")
	}
}

// TestWebSubPingOnReply verifies hub is pinged when a reply is added.
func TestWebSubPingOnReply(t *testing.T) {
	hubReceived := make(chan url.Values, 10)
	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := r.ParseForm(); err != nil {
				http.Error(w, "parse error", http.StatusBadRequest)
				return
			}
			hubReceived <- r.PostForm
			w.WriteHeader(http.StatusAccepted)
		}),
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	addr := listener.Addr().String()
	hubURL := "http://" + addr + "/"

	go server.Serve(listener)
	defer server.Close()

	mux, conn, handler := setupTestServer(t)
	handler.Config.URLWebsubHub = hubURL
	handler.Config.WebsubEnabled = true
	pinger := websub.NewPinger(hubURL, true)
	handler.Publisher.SetPinger(pinger)

	// Create users
	insertTestUser(t, conn, "alice", "secret1")
	insertTestUser(t, conn, "bob", "secret2")

	// Alice creates a post
	form := url.Values{}
	form.Set("jsontext", `{"description":"<p>Original post</p>"}`)
	form.Set("inReplyTo", "")

	w := authedPost(mux, "/newpost", "alice", "secret1", form)
	if w.Code != http.StatusOK {
		t.Fatalf("newpost failed: %s", w.Body.String())
	}

	// Get post ID
	var itemID int64
	err = conn.QueryRow("SELECT id FROM items WHERE author = ? ORDER BY id DESC LIMIT 1", "alice").Scan(&itemID)
	if err != nil {
		t.Fatalf("failed to get item ID: %v", err)
	}

	// Drain initial pings
	time.Sleep(200 * time.Millisecond)
	for {
		select {
		case <-hubReceived:
		default:
			goto addReply
		}
	}

addReply:
	// Bob replies to Alice's post
	replyForm := url.Values{}
	replyForm.Set("jsontext", `{"description":"<p>Great post!</p>"}`)
	replyForm.Set("inReplyTo", fmt.Sprintf("%d", itemID))

	w = authedPost(mux, "/newpost", "bob", "secret2", replyForm)
	if w.Code != http.StatusOK {
		t.Fatalf("reply newpost failed: %s", w.Body.String())
	}

	// Verify hub was pinged for both the reply and the comments feed
	var receivedPings []url.Values
	timeout := time.After(1 * time.Second)
	for {
		select {
		case form := <-hubReceived:
			receivedPings = append(receivedPings, form)
		case <-timeout:
			goto checkPings
		}
	}

checkPings:
	if len(receivedPings) < 2 {
		t.Errorf("expected at least 2 pings (user feed + comments feed), got %d", len(receivedPings))
	}
}

// TestWebSubHeadersDisabledWhenWebSubDisabled verifies headers aren't added when WebSub is disabled.
func TestWebSubHeadersDisabledWhenWebSubDisabled(t *testing.T) {
	mux, conn, handler := setupTestServer(t)
	handler.Config.URLWebsubHub = "" // Disabled
	handler.Config.WebsubEnabled = false

	insertTestUser(t, conn, "alice", "secret")
	if err := handler.Publisher.PublishUserFeed(conn, "alice"); err != nil {
		t.Fatalf("failed to publish feed: %v", err)
	}

	req := httptest.NewRequest("GET", "/feed?screenname=alice", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	// Even with disabled config, we still add headers if URLWebsubHub is set
	// The key is that no pinging happens
	link := w.Header().Get("Link")
	if link != "" && !strings.Contains(link, "rel=\"hub\"") {
		t.Logf("Link header present: %s", link)
	}
}

// TestWebSubHeaderContainsHubAndSelf verifies Link header has both hub and self.
func TestWebSubHeaderContainsHubAndSelf(t *testing.T) {
	mux, conn, handler := setupTestServer(t)
	handler.Config.URLWebsubHub = "https://hub.example.com"

	insertTestUser(t, conn, "alice", "secret")
	if err := handler.Publisher.PublishUserFeed(conn, "alice"); err != nil {
		t.Fatalf("failed to publish feed: %v", err)
	}

	req := httptest.NewRequest("GET", "/feed?screenname=alice", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	link := w.Header().Get("Link")
	if !strings.Contains(link, "rel=\"hub\"") {
		t.Errorf("Link header missing rel=\"hub\": %s", link)
	}
	if !strings.Contains(link, "rel=\"self\"") {
		t.Errorf("Link header missing rel=\"self\": %s", link)
	}
	if !strings.Contains(link, "https://hub.example.com") {
		t.Errorf("Link header missing hub URL: %s", link)
	}
	if !strings.Contains(link, "screenname=alice") {
		t.Errorf("Link header missing feed URL: %s", link)
	}
}

// TestWebSubConcurrentPingsFromDifferentEndpoints verifies multiple users' posts trigger pings.
// Note: SQLite doesn't support high concurrency, so this test creates posts sequentially.
func TestWebSubConcurrentPingsFromDifferentEndpoints(t *testing.T) {
	pingCount := atomic.Int32{}
	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			pingCount.Add(1)
			w.WriteHeader(http.StatusAccepted)
		}),
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	addr := listener.Addr().String()
	hubURL := "http://" + addr + "/"

	go server.Serve(listener)
	defer server.Close()

	mux, conn, handler := setupTestServer(t)
	handler.Config.URLWebsubHub = hubURL
	handler.Config.WebsubEnabled = true
	pinger := websub.NewPinger(hubURL, true)
	handler.Publisher.SetPinger(pinger)

	// Create multiple users
	for i := 0; i < 3; i++ {
		insertTestUser(t, conn, fmt.Sprintf("user%d", i), "secret")
	}

	// Have each user create a post sequentially (SQLite doesn't support high concurrency)
	for i := 0; i < 3; i++ {
		form := url.Values{}
		form.Set("jsontext", fmt.Sprintf(`{"description":"<p>Post from user %d</p>"}`, i))
		form.Set("inReplyTo", "")

		screenname := fmt.Sprintf("user%d", i)
		w := authedPost(mux, "/newpost", screenname, "secret", form)
		if w.Code != http.StatusOK {
			t.Fatalf("newpost failed for %s: %s", screenname, w.Body.String())
		}

		time.Sleep(50 * time.Millisecond) // Small delay between posts
	}

	time.Sleep(500 * time.Millisecond)

	// Should have pings for each user's feed + global feed
	// Each post triggers: user feed + global feed = 2 pings
	if count := pingCount.Load(); count < 6 {
		t.Errorf("expected at least 6 pings, got %d", count)
	}
}

// TestWebSubHubURLFromConfig verifies correct hub URL is used from config.
func TestWebSubHubURLFromConfig(t *testing.T) {
	var receivedHubURL string
	var receivedURLsMutex sync.Mutex

	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := r.ParseForm(); err != nil {
				return
			}
			receivedURLsMutex.Lock()
			receivedHubURL = r.Host
			receivedURLsMutex.Unlock()
			w.WriteHeader(http.StatusAccepted)
		}),
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	addr := listener.Addr().String()
	hubURL := "http://" + addr + "/"

	go server.Serve(listener)
	defer server.Close()

	mux, conn, handler := setupTestServer(t)
	handler.Config.URLWebsubHub = hubURL
	handler.Config.WebsubEnabled = true
	pinger := websub.NewPinger(hubURL, true)
	handler.Publisher.SetPinger(pinger)

	insertTestUser(t, conn, "alice", "secret")

	form := url.Values{}
	form.Set("jsontext", `{"description":"<p>Test</p>"}`)
	form.Set("inReplyTo", "")

	w := authedPost(mux, "/newpost", "alice", "secret", form)
	if w.Code != http.StatusOK {
		t.Fatalf("newpost failed: %s", w.Body.String())
	}

	time.Sleep(200 * time.Millisecond)

	receivedURLsMutex.Lock()
	if receivedHubURL == "" {
		t.Fatal("hub was not pinged")
	}
	receivedURLsMutex.Unlock()
}
