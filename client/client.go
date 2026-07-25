package client

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Config holds configuration for the client server.
type Config struct {
	ProductName              string // "rss.chat"
	ProductNameForDisplay    string // "rss.chat"
	Version                  string // "1.0"
	EnableLogin              bool
	URLServerForClient       string // "http://localhost:8081"
	URLWebsocketServerForClient string // "ws://localhost:8081"
	WebsocketEnabled         bool
	FeedURLEveryone          string // Feed URL for autodiscovery
}

// Server serves static client assets.
type Server struct {
	assetPath string
	config    Config
}

// NewServer creates a new client server.
func NewServer(assetPath string, config Config) *Server {
	return &Server{
		assetPath: assetPath,
		config:    config,
	}
}

// ServeHTTP serves static files and handles macro substitution for index.html.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Normalize path
	path := r.URL.Path
	if path == "/" || path == "" {
		path = "/index.html"
	}

	// Remove leading slash
	if strings.HasPrefix(path, "/") {
		path = path[1:]
	}

	// Prevent directory traversal
	if strings.Contains(path, "..") {
		http.NotFound(w, r)
		return
	}

	filePath := filepath.Join(s.assetPath, path)

	// Check if file exists
	_, err := os.Stat(filePath)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Handle index.html with macro substitution
	if strings.HasSuffix(path, "index.html") {
		s.serveIndexHTML(w, filePath)
		return
	}

	// Serve other files directly
	http.ServeFile(w, r, filePath)
}

// serveIndexHTML serves index.html with macro substitution.
func (s *Server) serveIndexHTML(w http.ResponseWriter, filePath string) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Substitute macros
	substituted := s.substituteConfig(string(data))

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, substituted)
}

// substituteConfig replaces template variables in HTML with config values.
func (s *Server) substituteConfig(html string) string {
	// Build substitution map
	subs := map[string]string{
		"[%productName%]":                  s.config.ProductName,
		"[%productNameForDisplay%]":        s.config.ProductNameForDisplay,
		"[%version%]":                      s.config.Version,
		"[%flEnableLogin%]":                boolToString(s.config.EnableLogin),
		"[%urlServerForClient%]":           s.config.URLServerForClient,
		"[%urlSocketServer%]":              s.config.URLWebsocketServerForClient,
		"[%urlWebsocketServerForClient%]": s.config.URLWebsocketServerForClient,
		"[%flWebsocketEnabled%]":           boolToString(s.config.WebsocketEnabled),
		"[%feedUrlEveryone%]":              s.config.FeedURLEveryone,
	}

	result := html
	for placeholder, value := range subs {
		result = strings.ReplaceAll(result, placeholder, value)
	}

	return result
}

// boolToString converts a bool to "true" or "false" for JavaScript.
func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// RegisterHandler registers the client server handler with an http.ServeMux.
func RegisterHandler(mux *http.ServeMux, assetPath string, config Config) {
	server := NewServer(assetPath, config)
	mux.Handle("/", server)
}
