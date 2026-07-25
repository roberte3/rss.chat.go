package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigLoad(t *testing.T) {
	// Create temporary config file
	tmpDir, err := os.MkdirTemp("", "config_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.json")
	configContent := `{
		"productNameForDisplay": "Test Server",
		"myDomain": "localhost:8081",
		"urlServerForClient": "http://localhost:8081",
		"urlServerForEmail": "http://localhost:8081",
		"mailSender": "test@localhost",
		"whitelist": ["user1@example.com", "user2@example.com"]
	}`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Verify required fields
	if cfg.ProductNameForDisplay != "Test Server" {
		t.Errorf("ProductNameForDisplay = %q, want %q", cfg.ProductNameForDisplay, "Test Server")
	}

	if cfg.MyDomain != "localhost:8081" {
		t.Errorf("MyDomain = %q, want %q", cfg.MyDomain, "localhost:8081")
	}

	// Verify defaults
	if cfg.ProductName != "rssChat" {
		t.Errorf("ProductName = %q, want %q (default)", cfg.ProductName, "rssChat")
	}

	if cfg.DatabasePath != "rss.chat.db" {
		t.Errorf("DatabasePath = %q, want %q (default)", cfg.DatabasePath, "rss.chat.db")
	}

	if cfg.HTTPPort != 8081 {
		t.Errorf("HTTPPort = %d, want %d (default)", cfg.HTTPPort, 8081)
	}

	if cfg.WebsocketPort != 1462 {
		t.Errorf("WebsocketPort = %d, want %d (default)", cfg.WebsocketPort, 1462)
	}

	// Verify trailing slashes on URLs
	if !hasSuffix(cfg.URLServerForClient, "/") {
		t.Errorf("URLServerForClient should have trailing slash")
	}

	// Verify whitelist
	if len(cfg.Whitelist) != 2 {
		t.Errorf("whitelist length = %d, want 2", len(cfg.Whitelist))
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantError bool
	}{
		{
			name: "valid config",
			content: `{
				"productNameForDisplay": "Test",
				"myDomain": "test.local",
				"urlServerForClient": "http://test.local",
				"urlServerForEmail": "http://test.local",
				"mailSender": "admin@test.local"
			}`,
			wantError: false,
		},
		{
			name: "missing required field",
			content: `{
				"productNameForDisplay": "Test",
				"myDomain": "test.local"
			}`,
			wantError: true,
		},
		{
			name: "websocket enabled without URL",
			content: `{
				"productNameForDisplay": "Test",
				"myDomain": "test.local",
				"urlServerForClient": "http://test.local",
				"urlServerForEmail": "http://test.local",
				"mailSender": "admin@test.local",
				"flWebsocketEnabled": true
			}`,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("", "config_test")
			if err != nil {
				t.Fatalf("failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			configPath := filepath.Join(tmpDir, "config.json")
			if err := os.WriteFile(configPath, []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to write config: %v", err)
			}

			_, err = Load(configPath)
			if (err != nil) != tt.wantError {
				t.Errorf("Load() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestIsEmailWhitelisted(t *testing.T) {
	tests := []struct {
		name       string
		whitelist  []string
		email      string
		wantResult bool
	}{
		{
			name:       "empty whitelist allows all",
			whitelist:  []string{},
			email:      "anyone@example.com",
			wantResult: true,
		},
		{
			name:       "email on whitelist",
			whitelist:  []string{"user@example.com"},
			email:      "user@example.com",
			wantResult: true,
		},
		{
			name:       "email not on whitelist",
			whitelist:  []string{"user@example.com"},
			email:      "other@example.com",
			wantResult: false,
		},
		{
			name:       "case insensitive match",
			whitelist:  []string{"User@Example.com"},
			email:      "user@example.com",
			wantResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Whitelist: tt.whitelist}
			result := cfg.IsEmailWhitelisted(tt.email)
			if result != tt.wantResult {
				t.Errorf("IsEmailWhitelisted(%q) = %v, want %v", tt.email, result, tt.wantResult)
			}
		})
	}
}

func TestIsEmailBlocked(t *testing.T) {
	cfg := &Config{BlockedUsersList: []string{"spam@example.com", "blocked@test.com"}}

	if cfg.IsEmailBlocked("spam@example.com") != true {
		t.Errorf("IsEmailBlocked should return true for blocked email")
	}

	if cfg.IsEmailBlocked("allowed@example.com") != false {
		t.Errorf("IsEmailBlocked should return false for allowed email")
	}

	// Test case insensitivity
	if cfg.IsEmailBlocked("SPAM@EXAMPLE.COM") != true {
		t.Errorf("IsEmailBlocked should be case insensitive")
	}
}

func TestConfigurablePorts(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "config_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.json")
	configContent := `{
		"productNameForDisplay": "Test Server",
		"myDomain": "localhost",
		"urlServerForClient": "http://localhost:9000",
		"urlServerForEmail": "http://localhost:9000",
		"mailSender": "test@localhost",
		"httpPort": 9000,
		"websocketPort": 9001
	}`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.HTTPPort != 9000 {
		t.Errorf("HTTPPort = %d, want 9000", cfg.HTTPPort)
	}

	if cfg.WebsocketPort != 9001 {
		t.Errorf("WebsocketPort = %d, want 9001", cfg.WebsocketPort)
	}
}

func hasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}
