package setup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
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
	err = CreateSettings(settingsPath)
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

	if s.ProductName == "" {
		t.Errorf("settings productName is empty")
	}
}
