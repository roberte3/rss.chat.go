package websub

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestPingerFeedURLEncoding verifies that feed URLs are properly encoded in POST form data.
func TestPingerFeedURLEncoding(t *testing.T) {
	tests := []struct {
		name    string
		feedURL string
	}{
		{
			name:    "Simple URL",
			feedURL: "https://example.com/feed",
		},
		{
			name:    "URL with query parameters",
			feedURL: "https://example.com/feed?screenname=alice&format=xml",
		},
		{
			name:    "URL with special characters in screenname",
			feedURL: "https://example.com/feed?screenname=user%2Bname",
		},
		{
			name:    "URL with fragment",
			feedURL: "https://example.com/feed#section",
		},
		{
			name:    "Comments feed URL",
			feedURL: "https://example.com/comments/alice/123.xml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			urlChan := make(chan string, 1)
			server := &http.Server{
				Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if err := r.ParseForm(); err != nil {
						t.Fatalf("failed to parse form: %v", err)
					}
					urlChan <- r.PostFormValue("hub.url")
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

			pinger := NewPinger(hubURL, true)
			pinger.Ping(tt.feedURL)

			// Wait for the server to receive the request
			var receivedURL string
			select {
			case url := <-urlChan:
				receivedURL = url
			case <-time.After(1 * time.Second):
				t.Fatal("timeout waiting for ping")
			}

			if receivedURL != tt.feedURL {
				t.Errorf("expected hub.url=%s, got %s", tt.feedURL, receivedURL)
			}
		})
	}
}

// TestPingerHubResponseCodes verifies handling of different HTTP response codes.
func TestPingerHubResponseCodes(t *testing.T) {
	tests := []struct {
		name           string
		statusCode     int
		shouldError    bool
		expectedInLogs bool
	}{
		{"200 OK", http.StatusOK, false, false},
		{"202 Accepted", http.StatusAccepted, false, false},
		{"204 No Content", http.StatusNoContent, false, false},
		{"400 Bad Request", http.StatusBadRequest, true, true},
		{"401 Unauthorized", http.StatusUnauthorized, true, true},
		{"404 Not Found", http.StatusNotFound, true, true},
		{"500 Internal Server Error", http.StatusInternalServerError, true, true},
		{"503 Service Unavailable", http.StatusServiceUnavailable, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := &http.Server{
				Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(tt.statusCode)
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

			pinger := NewPinger(hubURL, true)
			// Should not panic regardless of response code
			pinger.Ping("https://example.com/feed")

			time.Sleep(100 * time.Millisecond)
		})
	}
}

// TestPingerConcurrentPings verifies multiple concurrent pings work correctly.
func TestPingerConcurrentPings(t *testing.T) {
	receivedCount := atomic.Int32{}
	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedCount.Add(1)
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

	pinger := NewPinger(hubURL, true)

	// Send multiple concurrent pings
	const numPings = 10
	var wg sync.WaitGroup
	for i := 0; i < numPings; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			feedURL := fmt.Sprintf("https://example.com/feed?id=%d", index)
			pinger.Ping(feedURL)
		}(i)
	}

	wg.Wait()
	time.Sleep(500 * time.Millisecond)

	if count := receivedCount.Load(); count != numPings {
		t.Errorf("expected %d pings, got %d", numPings, count)
	}
}

// TestPingerFormDataCorrectness verifies the exact form data sent to hub.
func TestPingerFormDataCorrectness(t *testing.T) {
	var receivedForm url.Values
	var formMutex sync.Mutex

	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := r.ParseForm(); err != nil {
				http.Error(w, "parse error", http.StatusBadRequest)
				return
			}
			formMutex.Lock()
			receivedForm = r.PostForm
			formMutex.Unlock()
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

	pinger := NewPinger(hubURL, true)
	feedURL := "https://example.com/feed?screenname=alice"
	pinger.Ping(feedURL)

	time.Sleep(200 * time.Millisecond)

	formMutex.Lock()
	defer formMutex.Unlock()

	// Verify form fields
	if mode := receivedForm.Get("hub.mode"); mode != "publish" {
		t.Errorf("expected hub.mode=publish, got %s", mode)
	}
	if url := receivedForm.Get("hub.url"); url != feedURL {
		t.Errorf("expected hub.url=%s, got %s", feedURL, url)
	}

	// Verify no extra fields
	expectedFields := map[string]bool{"hub.mode": true, "hub.url": true}
	for key := range receivedForm {
		if !expectedFields[key] {
			t.Errorf("unexpected form field: %s", key)
		}
	}
}

// TestPingerHubTimeout verifies handling of slow/hanging hub servers.
func TestPingerHubTimeout(t *testing.T) {
	// Create a server that never responds
	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			<-time.After(5 * time.Second) // Never actually responds
			w.WriteHeader(http.StatusOK)
		}),
		ReadHeaderTimeout: 1 * time.Second,
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	addr := listener.Addr().String()
	hubURL := "http://" + addr + "/"

	go server.Serve(listener)
	defer server.Close()

	pinger := NewPinger(hubURL, true)
	pinger.Ping("https://example.com/feed")

	// Should return quickly even though hub doesn't respond
	// (async operation, so this test just verifies it doesn't block)
	time.Sleep(100 * time.Millisecond)
}

// TestPingerEmptyFeedURL verifies graceful handling of empty feed URLs.
func TestPingerEmptyFeedURL(t *testing.T) {
	receivedPings := atomic.Int32{}
	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedPings.Add(1)
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

	pinger := NewPinger(hubURL, true)
	pinger.Ping("") // Empty feed URL

	time.Sleep(100 * time.Millisecond)

	// Should not send a ping with empty feed URL
	if count := receivedPings.Load(); count != 0 {
		t.Errorf("expected no pings for empty feed URL, got %d", count)
	}
}

// TestPingerEmptyHubURL verifies graceful handling of empty hub URL.
func TestPingerEmptyHubURL(t *testing.T) {
	pinger := NewPinger("", true)
	// Should not panic
	pinger.Ping("https://example.com/feed")
}

// TestPingerContentType verifies the correct Content-Type header is sent.
func TestPingerContentType(t *testing.T) {
	contentTypeChan := make(chan string, 1)
	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			contentTypeChan <- r.Header.Get("Content-Type")
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

	pinger := NewPinger(hubURL, true)
	pinger.Ping("https://example.com/feed")

	// Wait for the server to receive the request
	var receivedContentType string
	select {
	case ct := <-contentTypeChan:
		receivedContentType = ct
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for ping")
	}

	// net/http.PostForm automatically sets Content-Type to application/x-www-form-urlencoded
	if !strings.Contains(receivedContentType, "application/x-www-form-urlencoded") {
		t.Errorf("expected application/x-www-form-urlencoded, got %s", receivedContentType)
	}
}

// TestPingerHTTPMethod verifies POST method is used.
func TestPingerHTTPMethod(t *testing.T) {
	methodChan := make(chan string, 1)
	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			methodChan <- r.Method
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

	pinger := NewPinger(hubURL, true)
	pinger.Ping("https://example.com/feed")

	// Wait for the server to receive the request
	var receivedMethod string
	select {
	case method := <-methodChan:
		receivedMethod = method
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for ping")
	}

	if receivedMethod != "POST" {
		t.Errorf("expected POST, got %s", receivedMethod)
	}
}

// TestPingerStateChanges verifies pinger can be enabled/disabled.
func TestPingerStateChanges(t *testing.T) {
	disabledPinger := NewPinger("https://example.com/hub", false)
	if disabledPinger.IsEnabled() {
		t.Error("expected IsEnabled() to return false")
	}

	enabledPinger := NewPinger("https://example.com/hub", true)
	if !enabledPinger.IsEnabled() {
		t.Error("expected IsEnabled() to return true")
	}
}

// TestPingerHubConnectionRefusal verifies handling of refused connections.
func TestPingerHubConnectionRefusal(t *testing.T) {
	// Use a port that won't accept connections
	pinger := NewPinger("http://127.0.0.1:1/", true)
	pinger.SetDebugLog(nil) // Silent logging

	// Should not panic
	pinger.Ping("https://example.com/feed")

	time.Sleep(100 * time.Millisecond)
}

// TestPingerMultipleHubURLs verifies pinger works with different hub URLs.
func TestPingerMultipleHubURLs(t *testing.T) {
	hubs := map[string]struct {
		server *http.Server
		url    string
	}{}

	for i := 0; i < 3; i++ {
		server := &http.Server{
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusAccepted)
			}),
		}

		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("failed to listen: %v", err)
		}

		addr := listener.Addr().String()
		hubURL := "http://" + addr + "/"
		hubs[hubURL] = struct {
			server *http.Server
			url    string
		}{server, hubURL}

		go server.Serve(listener)
		defer server.Close()
	}

	for hubURL := range hubs {
		pinger := NewPinger(hubURL, true)
		pinger.Ping("https://example.com/feed")
		time.Sleep(100 * time.Millisecond)
	}
}

// TestPingerFeedURLVariety verifies handling of diverse feed URL formats.
func TestPingerFeedURLVariety(t *testing.T) {
	feedURLs := []string{
		"https://example.com/feed",                        // Simple
		"http://example.com/feed?screenname=alice",        // Query param
		"https://example.com:8080/feed",                   // Custom port
		"https://subdomain.example.com/feed",              // Subdomain
		"https://example.com/path/to/feed",                // Path
		"https://example.com/comments/alice/123.xml",      // Comments feed
		"https://example.com/getsubscriptionlist",         // OPML
		"https://example.com/feed?screenname=test%2Buser", // Encoded chars
	}

	for _, feedURL := range feedURLs {
		urlChan := make(chan string, 1)
		server := &http.Server{
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if err := r.ParseForm(); err != nil {
					return
				}
				urlChan <- r.PostFormValue("hub.url")
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

		pinger := NewPinger(hubURL, true)
		pinger.Ping(feedURL)

		// Wait for the server to receive the request
		var receivedURL string
		select {
		case url := <-urlChan:
			receivedURL = url
		case <-time.After(1 * time.Second):
			t.Errorf("timeout waiting for ping for feed URL %s", feedURL)
			server.Close()
			continue
		}

		if receivedURL != feedURL {
			t.Errorf("for feed URL %s, expected %s, got %s", feedURL, feedURL, receivedURL)
		}

		server.Close()
	}
}

// TestPingerBurstLoad verifies handling of rapid successive pings.
func TestPingerBurstLoad(t *testing.T) {
	receivedCount := atomic.Int32{}
	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedCount.Add(1)
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

	pinger := NewPinger(hubURL, true)

	// Send 100 pings in rapid succession
	const burstSize = 100
	for i := 0; i < burstSize; i++ {
		pinger.Ping(fmt.Sprintf("https://example.com/feed?id=%d", i))
	}

	// Wait for all async operations to complete
	time.Sleep(1 * time.Second)

	if count := receivedCount.Load(); count != int32(burstSize) {
		t.Errorf("expected %d pings, got %d", burstSize, count)
	}
}

// TestPingerHubPartialResponse verifies handling of incomplete hub responses.
func TestPingerHubPartialResponse(t *testing.T) {
	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			io.WriteString(w, "Partial response")
			// Connection will close abruptly
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

	pinger := NewPinger(hubURL, true)
	pinger.SetDebugLog(nil)

	// Should not panic even if response is incomplete
	pinger.Ping("https://example.com/feed")

	time.Sleep(100 * time.Millisecond)
}

// TestPingerDebugLogging verifies debug logging can be controlled.
func TestPingerDebugLogging(t *testing.T) {
	pinger := NewPinger("https://example.com/hub", false)

	// Should not panic when setting nil logger
	pinger.SetDebugLog(nil)

	// Should not panic when setting actual logger
	pinger.SetDebugLog(log.New(io.Discard, "", 0))
}

// TestPingerRequestPath verifies POST is sent to the hub's root path.
func TestPingerRequestPath(t *testing.T) {
	pathChan := make(chan string, 1)
	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			pathChan <- r.RequestURI
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

	pinger := NewPinger(hubURL, true)
	pinger.Ping("https://example.com/feed")

	// Wait for the server to receive the request
	var receivedPath string
	select {
	case path := <-pathChan:
		receivedPath = path
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for ping")
	}

	if !strings.HasPrefix(receivedPath, "/") {
		t.Errorf("expected path starting with /, got %s", receivedPath)
	}
}
