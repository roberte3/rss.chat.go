package setup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"rss.chat.go/config"
)

func TestCreateBlocklist(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "setup_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	blocklistPath := filepath.Join(tmpDir, "blocklist.json")
	err = CreateBlocklist(blocklistPath)
	if err != nil {
		t.Fatalf("CreateBlocklist failed: %v", err)
	}

	// Verify file exists
	_, err = os.Stat(blocklistPath)
	if err != nil {
		t.Fatalf("blocklist.json not created: %v", err)
	}

	// Verify file contents
	data, err := os.ReadFile(blocklistPath)
	if err != nil {
		t.Fatalf("failed to read blocklist.json: %v", err)
	}

	var bl Blocklist
	err = json.Unmarshal(data, &bl)
	if err != nil {
		t.Fatalf("failed to parse blocklist.json: %v", err)
	}

	// Verify structure
	if bl.Note == "" {
		t.Errorf("blocklist note is empty")
	}

	if bl.BlockedEmails == nil {
		t.Errorf("blockedEmails should be a slice, not nil")
	}

	if len(bl.BlockedEmails) != 0 {
		t.Errorf("blockedEmails should be empty by default, got %v", bl.BlockedEmails)
	}
}

func TestCreateSettings(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "setup_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	settingsPath := filepath.Join(tmpDir, "settings.json")
	err = CreateSettings(settingsPath, "test-app")
	if err != nil {
		t.Fatalf("CreateSettings failed: %v", err)
	}

	// Verify file exists
	_, err = os.Stat(settingsPath)
	if err != nil {
		t.Fatalf("settings.json not created: %v", err)
	}

	// Verify file contents
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("failed to read settings.json: %v", err)
	}

	var s Settings
	err = json.Unmarshal(data, &s)
	if err != nil {
		t.Fatalf("failed to parse settings.json: %v", err)
	}

	// Verify structure
	if s.Note == "" {
		t.Errorf("settings note is empty")
	}

	if s.ProductName != "test-app" {
		t.Errorf("settings productName = %q, expected test-app", s.ProductName)
	}
}

func TestCreateConfig(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "setup_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.json")

	// Simulate user input with custom values. The blank-looking line after the
	// mail sender is the "enable email sending?" answer.
	input := `test-app
http://example.com:8080
sender@example.com
y
smtp.example.com
465
custom-smtp-user
password123
y`

	reader := strings.NewReader(input)
	err = CreateConfig(configPath, reader)
	if err != nil {
		t.Fatalf("CreateConfig failed: %v", err)
	}

	// Verify file exists
	_, err = os.Stat(configPath)
	if err != nil {
		t.Fatalf("config.json not created: %v", err)
	}

	// Verify file is parseable by config.Load
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("config.Load failed: %v", err)
	}

	// Verify prompted values were set
	if cfg.ProductNameForDisplay != "test-app" {
		t.Errorf("productNameForDisplay = %q, expected test-app", cfg.ProductNameForDisplay)
	}

	if cfg.MyDomain != "http://example.com:8080" {
		t.Errorf("myDomain = %q, expected http://example.com:8080", cfg.MyDomain)
	}

	if cfg.MailSender != "sender@example.com" {
		t.Errorf("mailSender = %q, expected sender@example.com", cfg.MailSender)
	}

	if cfg.SMTPHost != "smtp.example.com" {
		t.Errorf("smtpHost = %q, expected smtp.example.com", cfg.SMTPHost)
	}

	if cfg.SMTPPort != 465 {
		t.Errorf("smtpPort = %d, expected 465", cfg.SMTPPort)
	}

	if cfg.SMTPUsername != "custom-smtp-user" {
		t.Errorf("smtpUsername = %q, expected custom-smtp-user", cfg.SMTPUsername)
	}

	if cfg.SMTPPassword != "password123" {
		t.Errorf("smtpPassword = %q, expected password123", cfg.SMTPPassword)
	}

	if !cfg.WebsocketEnabled {
		t.Errorf("websocketEnabled should be true")
	}

	// Verify derived values (note: applyDefaults adds trailing slashes to URLs)
	if cfg.URLServerForClient != "http://example.com:8080/api/" {
		t.Errorf("urlServerForClient = %q, expected http://example.com:8080/api/", cfg.URLServerForClient)
	}

	if cfg.URLServerForEmail != "http://example.com:8080/" {
		t.Errorf("urlServerForEmail = %q, expected http://example.com:8080/", cfg.URLServerForEmail)
	}

	if cfg.URLWebsocketServerForClient != "ws://example.com:1462/" {
		t.Errorf("urlWebsocketServerForClient = %q, expected ws://example.com:1462/", cfg.URLWebsocketServerForClient)
	}
}

func TestCreateConfigDefaults(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "setup_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.json")

	// Simulate user pressing Enter for all prompts (all defaults)
	input := "\n\n\n\n\n\n\n\n"
	reader := strings.NewReader(input)
	err = CreateConfig(configPath, reader)
	if err != nil {
		t.Fatalf("CreateConfig with defaults failed: %v", err)
	}

	// Verify file is parseable
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("config.Load failed: %v", err)
	}

	// Verify defaults were used
	if cfg.ProductNameForDisplay != "rss.chat" {
		t.Errorf("productNameForDisplay = %q, expected rss.chat (default)", cfg.ProductNameForDisplay)
	}

	if cfg.MyDomain != "http://localhost:8081" {
		t.Errorf("myDomain = %q, expected http://localhost:8081 (default)", cfg.MyDomain)
	}

	if cfg.MailSender != "admin@localhost" {
		t.Errorf("mailSender = %q, expected admin@localhost (default)", cfg.MailSender)
	}

	if cfg.SMTPPort != 587 {
		t.Errorf("smtpPort = %d, expected 587 (default)", cfg.SMTPPort)
	}
}

// TestCreateConfigEmailDisabled covers answering "n" to the email prompt: the
// SMTP questions are skipped entirely and no SMTP settings are written, which
// is what makes the server start up with email sending switched off.
func TestCreateConfigEmailDisabled(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "setup_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.json")

	// product name, domain, mail sender, "n" to email, then websocket answer.
	// If the SMTP prompts were still being read, "y" would be consumed as the
	// SMTP host and websocket would end up disabled.
	input := "test-app\nhttp://example.com:8080\nsender@example.com\nn\ny\n"
	reader := strings.NewReader(input)
	if err := CreateConfig(configPath, reader); err != nil {
		t.Fatalf("CreateConfig failed: %v", err)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("config.Load failed: %v", err)
	}

	if cfg.SMTPHost != "" {
		t.Errorf("smtpHost = %q, expected empty when email is disabled", cfg.SMTPHost)
	}
	if cfg.SMTPUsername != "" {
		t.Errorf("smtpUsername = %q, expected empty when email is disabled", cfg.SMTPUsername)
	}
	if cfg.SMTPPassword != "" {
		t.Errorf("smtpPassword = %q, expected empty when email is disabled", cfg.SMTPPassword)
	}

	// The prompts after the email block must still line up.
	if !cfg.WebsocketEnabled {
		t.Error("websocketEnabled should be true; the SMTP prompts likely consumed the wrong input")
	}
	if cfg.MailSender != "sender@example.com" {
		t.Errorf("mailSender = %q, expected sender@example.com", cfg.MailSender)
	}
}

func TestCreateConfigSkipsExisting(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "setup_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.json")

	// Write a sentinel config
	sentinelContent := []byte(`{"productNameForDisplay": "SENTINEL"}`)
	if err := os.WriteFile(configPath, sentinelContent, 0644); err != nil {
		t.Fatalf("failed to write sentinel config: %v", err)
	}

	// Try to create config again (should skip)
	reader := strings.NewReader("\n\n\n\n\n\n\n\n")
	err = CreateConfig(configPath, reader)
	if err != nil {
		t.Fatalf("CreateConfig (skip) failed: %v", err)
	}

	// Verify file wasn't overwritten
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read config.json: %v", err)
	}

	if string(data) != string(sentinelContent) {
		t.Errorf("config.json was overwritten when it should have been skipped")
	}
}
