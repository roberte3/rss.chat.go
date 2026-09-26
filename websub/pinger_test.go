package websub

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"
)

// TestPingerDisabled verifies that pinging does nothing when disabled.
func TestPingerDisabled(t *testing.T) {
	pinger := NewPinger("https://example.com/hub", false)

	// Should not panic or error even when calling with empty values
	pinger.Ping("")
	pinger.Ping("https://example.com/feed")
}

// TestPingerNotInitialized verifies that pinging does nothing when pinger has no hub URL.
func TestPingerNotInitialized(t *testing.T) {
	pinger := NewPinger("", true)

	// Should not panic or error
	pinger.Ping("https://example.com/feed")
}

// TestPingerSendsCorrectRequest verifies the pinger sends proper HTTP POST requests.
func TestPingerSendsCorrectRequest(t *testing.T) {
	// Start a mock hub server
	hubReceived := make(chan url.Values, 1)
	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "POST" {
				t.Errorf("expected POST, got %s", r.Method)
			}
			if err := r.ParseForm(); err != nil {
				t.Fatalf("failed to parse form: %v", err)
			}
			hubReceived <- r.PostForm
			w.WriteHeader(http.StatusAccepted)
		}),
	}

	// Find an available port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	addr := listener.Addr().String()
	hubURL := "http://" + addr + "/"

	go func() {
		server.Serve(listener)
	}()
	defer server.Close()

	// Create pinger and send a ping
	pinger := NewPinger(hubURL, true)
	feedURL := "https://example.com/feed?screenname=alice"

	pinger.Ping(feedURL)

	// Wait for the ping to arrive (async, so give it some time)
	select {
	case form := <-hubReceived:
		// Verify the form data
		if form.Get("hub.mode") != "publish" {
			t.Errorf("expected hub.mode=publish, got %s", form.Get("hub.mode"))
		}
		if form.Get("hub.url") != feedURL {
			t.Errorf("expected hub.url=%s, got %s", feedURL, form.Get("hub.url"))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ping did not arrive at hub within 2 seconds")
	}
}

// TestPingerWithHubError verifies the pinger handles hub errors gracefully.
func TestPingerWithHubError(t *testing.T) {
	// Start a server that returns an error
	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, "hub error")
		}),
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	addr := listener.Addr().String()
	hubURL := "http://" + addr + "/"

	go func() {
		server.Serve(listener)
	}()
	defer server.Close()

	// Create pinger with debug logging to catch errors
	pinger := NewPinger(hubURL, true)
	pinger.SetDebugLog(log.New(os.Stderr, "websub_test: ", log.LstdFlags))

	// Should not panic
	pinger.Ping("https://example.com/feed")

	// Wait a bit for async ping to complete
	time.Sleep(100 * time.Millisecond)
}

// TestPingerWithUnreachableHub verifies the pinger handles connection errors gracefully.
func TestPingerWithUnreachableHub(t *testing.T) {
	// Use an IP:port that won't accept connections
	pinger := NewPinger("http://127.0.0.1:1/", true)
	pinger.SetDebugLog(log.New(os.Stderr, "websub_test: ", log.LstdFlags))

	// Should not panic
	pinger.Ping("https://example.com/feed")

	// Wait a bit for async ping to complete
	time.Sleep(100 * time.Millisecond)
}

// TestPingerIsEnabled verifies the IsEnabled method returns the correct state.
func TestPingerIsEnabled(t *testing.T) {
	tests := []struct {
		enabled bool
	}{
		{true},
		{false},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("enabled=%v", tt.enabled), func(t *testing.T) {
			pinger := NewPinger("https://example.com/hub", tt.enabled)
			if pinger.IsEnabled() != tt.enabled {
				t.Errorf("expected IsEnabled() to return %v", tt.enabled)
			}
		})
	}
}

