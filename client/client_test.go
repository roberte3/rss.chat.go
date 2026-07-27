package client

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestClientServer(t *testing.T) {
	// Create temporary directory with test files
	tmpDir, err := os.MkdirTemp("", "client_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create index.html with template variables
	indexHTML := `<!DOCTYPE html>
<html>
<head>
	<title>Test App</title>
	<script>
		const config = {
			productName: "[%productName%]",
			version: "[%version%]",
			urlServer: "[%urlServerForClient%]",
			flWebsocketEnabled: "[%flWebsocketEnabled%]"
		};
	</script>
</head>
<body>
	<h1>Welcome</h1>
</body>
</html>`

	if err := os.WriteFile(filepath.Join(tmpDir, "index.html"), []byte(indexHTML), 0644); err != nil {
		t.Fatalf("failed to write index.html: %v", err)
	}

	// Create a static JS file
	if err := os.WriteFile(filepath.Join(tmpDir, "app.js"), []byte("console.log('test');"), 0644); err != nil {
		t.Fatalf("failed to write app.js: %v", err)
	}

	// Create client server with config
	config := Config{
		ProductName:                 "rss.chat",
		ProductNameForDisplay:       "rss.chat",
		Version:                     "1.0",
		EnableLogin:                 true,
		URLServerForClient:          "http://localhost:8081",
		URLWebsocketServerForClient: "ws://localhost:8081",
		WebsocketEnabled:            true,
	}

	server := NewServer(tmpDir, config)

	// Test index.html with macro substitution
	t.Run("index.html macro substitution", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/", nil)
		if err != nil {
			t.Fatalf("failed to create request: %v", err)
		}

		w := httptest.NewRecorder()
		server.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		body := w.Body.String()

		// Check that macros were substituted
		if !strings.Contains(body, "rss.chat") {
			t.Errorf("productName not substituted")
		}

		if !strings.Contains(body, "1.0") {
			t.Errorf("version not substituted")
		}

		if !strings.Contains(body, "http://localhost:8081") {
			t.Errorf("urlServer not substituted")
		}

		if !strings.Contains(body, "true") {
			t.Errorf("websocket enabled not substituted")
		}

		// Check that template variables are NOT in the output
		if strings.Contains(body, "[%") {
			t.Errorf("template variables not fully substituted")
		}
	})

	// Test static file serving
	t.Run("serve static file", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/app.js", nil)
		if err != nil {
			t.Fatalf("failed to create request: %v", err)
		}

		w := httptest.NewRecorder()
		server.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		if !strings.Contains(w.Body.String(), "console.log") {
			t.Errorf("static file not served correctly")
		}
	})

	// Test 404 for missing file
	t.Run("404 for missing file", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/nonexistent.js", nil)
		if err != nil {
			t.Fatalf("failed to create request: %v", err)
		}

		w := httptest.NewRecorder()
		server.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", w.Code)
		}
	})

	// Test directory traversal prevention
	t.Run("prevent directory traversal", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/../../../etc/passwd", nil)
		if err != nil {
			t.Fatalf("failed to create request: %v", err)
		}

		w := httptest.NewRecorder()
		server.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status 404 for traversal attempt, got %d", w.Code)
		}
	})

	// Test root path handling
	t.Run("root path redirects to index.html", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/", nil)
		if err != nil {
			t.Fatalf("failed to create request: %v", err)
		}

		w := httptest.NewRecorder()
		server.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		if !strings.Contains(w.Body.String(), "Welcome") {
			t.Errorf("index.html not served at root")
		}
	})
}

func TestBoolToString(t *testing.T) {
	tests := []struct {
		input    bool
		expected string
	}{
		{true, "true"},
		{false, "false"},
	}

	for _, tt := range tests {
		result := boolToString(tt.input)
		if result != tt.expected {
			t.Errorf("boolToString(%v) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestSubstituteConfig(t *testing.T) {
	config := Config{
		ProductName:                 "test-app",
		ProductNameForDisplay:       "Test App",
		Version:                     "2.0",
		EnableLogin:                 true,
		URLServerForClient:          "http://api.test.com",
		URLWebsocketServerForClient: "ws://api.test.com",
		WebsocketEnabled:            false,
	}

	server := NewServer("", config)

	html := `productName: [%productName%], version: [%version%], enabled: [%flWebsocketEnabled%]`
	result := server.substituteConfig(html)

	if !strings.Contains(result, "test-app") {
		t.Errorf("productName not substituted")
	}

	if !strings.Contains(result, "2.0") {
		t.Errorf("version not substituted")
	}

	if !strings.Contains(result, "false") {
		t.Errorf("websocket enabled not substituted correctly")
	}

	if strings.Contains(result, "[%") {
		t.Errorf("template variables remain in output")
	}
}

func TestFeedAutodiscoveryLink(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "feed_autodiscovery_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create index.html with feed autodiscovery link
	indexHTML := `<!DOCTYPE html>
<html>
<head>
	<title>rss.chat</title>
	<link rel="alternate" type="application/rss+xml" href="[%feedUrlEveryone%]">
	<script>
		const config = {
			urlServer: "[%urlServerForClient%]"
		};
	</script>
</head>
<body>
	<h1>Welcome</h1>
</body>
</html>`

	if err := os.WriteFile(filepath.Join(tmpDir, "index.html"), []byte(indexHTML), 0644); err != nil {
		t.Fatalf("failed to write index.html: %v", err)
	}

	feedURL := "http://example.com/feed"
	config := Config{
		ProductName:                 "rss.chat",
		ProductNameForDisplay:       "rss.chat",
		Version:                     "1.0",
		EnableLogin:                 true,
		URLServerForClient:          "http://api.example.com",
		URLWebsocketServerForClient: "ws://api.example.com",
		WebsocketEnabled:            true,
		FeedURLEveryone:             feedURL,
	}

	server := NewServer(tmpDir, config)

	t.Run("feed autodiscovery link present", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/", nil)
		if err != nil {
			t.Fatalf("failed to create request: %v", err)
		}

		w := httptest.NewRecorder()
		server.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		body := w.Body.String()

		// Verify link tag is present
		if !strings.Contains(body, `<link rel="alternate" type="application/rss+xml"`) {
			t.Errorf("feed autodiscovery link tag not found")
		}

		// Verify feed URL is substituted correctly
		if !strings.Contains(body, feedURL) {
			t.Errorf("feed URL not substituted, expected %q in body", feedURL)
		}

		// Verify macro is not in output
		if strings.Contains(body, "[%feedUrlEveryone%]") {
			t.Errorf("feed URL macro not substituted")
		}
	})

	t.Run("feed URL macro substitution", func(t *testing.T) {
		html := `<link rel="alternate" type="application/rss+xml" href="[%feedUrlEveryone%]">`
		result := server.substituteConfig(html)

		if !strings.Contains(result, feedURL) {
			t.Errorf("feed URL not substituted, got: %s", result)
		}

		if strings.Contains(result, "[%feedUrlEveryone%]") {
			t.Errorf("macro not substituted")
		}
	})
}

// TestVendoredClientMacrosAreSatisfied checks the real vendored index.html
// against the server's substitution table. The vendored client is upstream
// code that gets re-synced from github.com/scripting/rss.chat, and a sync can
// introduce a new [%macro%] the server knows nothing about. That failure is
// silent at runtime — the placeholder is served verbatim into the page — so
// catch it here instead.
func TestVendoredClientMacrosAreSatisfied(t *testing.T) {
	indexPath := filepath.Join("code", "index.html")
	data, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("cannot read vendored %s: %v", indexPath, err)
	}

	// A fully-populated server: anything it still fails to substitute is a
	// macro it genuinely does not know about, not one left blank by config.
	srv := NewServer("code", Config{
		ProductName:                 "p",
		ProductNameForDisplay:       "d",
		Version:                     "v",
		EnableLogin:                 true,
		URLServerForClient:          "u",
		URLWebsocketServerForClient: "w",
		WebsocketEnabled:            true,
		FeedURLEveryone:             "f",
	})

	macro := regexp.MustCompile(`\[%[a-zA-Z]+%\]`)
	if leftover := macro.FindAllString(srv.substituteConfig(string(data)), -1); len(leftover) > 0 {
		seen := map[string]bool{}
		var uniq []string
		for _, m := range leftover {
			if !seen[m] {
				seen[m] = true
				uniq = append(uniq, m)
			}
		}
		t.Errorf("vendored index.html uses macros the server does not substitute: %s\n"+
			"Add them to substituteConfig (and Config) or re-check the upstream sync.",
			strings.Join(uniq, ", "))
	}
}

// TestVendoredClientAssetsPresent guards against a partial vendor drop: the
// server serves these from disk, so a missing file is a 404 in the browser
// rather than a build error.
func TestVendoredClientAssetsPresent(t *testing.T) {
	for _, f := range []string{
		"index.html", "api.js", "code.js", "globals.js", "misc.js", "chat.js",
		"chat.css", "styles.css",
		filepath.Join("themes", "classic", "theme.js"),
		filepath.Join("themes", "classic", "theme.css"),
		"LICENSE",
	} {
		if _, err := os.Stat(filepath.Join("code", f)); err != nil {
			t.Errorf("vendored client asset missing: %s (%v)", f, err)
		}
	}
}
