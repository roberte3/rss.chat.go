package api

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/roberte3/rss.chat.go/config"
)

// TestBlocklistReloadFromConfigJSON tests using blocklist from config.json.
func TestBlocklistReloadFromConfigJSON(t *testing.T) {
	// Create handler
	_, conn, handler := setupTestServer(t)
	defer conn.Close()

	// Create config with blocklist in BlockedUsersList
	cfg := &config.Config{
		ProductName:        "test",
		ProductNameForDisplay: "Test",
		MyDomain:           "http://localhost:8080",
		URLServerForClient: "http://localhost:8080",
		DatabasePath:       ":memory:",
		BlocklistPath:      "",
		BlockedUsersList:   []string{"blocked@example.com", "spam@example.com"},
	}
	handler.Config = cfg

	// Get merged blocklist
	emails, err := handler.getMergedBlocklist()
	if err != nil {
		t.Fatalf("Failed to get merged blocklist: %v", err)
	}

	// Check that blocklist includes emails from config
	if len(emails) != 2 {
		t.Errorf("Expected 2 blocked emails, got %d", len(emails))
	}

	found := make(map[string]bool)
	for _, email := range emails {
		found[email] = true
	}

	if !found["blocked@example.com"] {
		t.Error("blocked@example.com not in blocklist")
	}
	if !found["spam@example.com"] {
		t.Error("spam@example.com not in blocklist")
	}
}

// TestBlocklistReloadFromSeparateFile tests loading blocklist from separate blocklist.json file.
func TestBlocklistReloadFromSeparateFile(t *testing.T) {
	// Create a temporary blocklist.json file
	blocklistPath := "test_blocklist.json"
	blocklist := map[string]interface{}{
		"blockedEmails": []string{"file-blocked@example.com", "file-spam@example.com"},
	}

	data, err := json.Marshal(blocklist)
	if err != nil {
		t.Fatalf("Failed to marshal blocklist: %v", err)
	}

	if err := ioutil.WriteFile(blocklistPath, data, 0644); err != nil {
		t.Fatalf("Failed to write blocklist file: %v", err)
	}
	defer os.Remove(blocklistPath)

	// Create handler
	_, conn, handler := setupTestServer(t)
	defer conn.Close()

	// Create config with blocklist path
	cfg := &config.Config{
		MyDomain:      "http://localhost:8080",
		BlocklistPath: blocklistPath,
	}
	handler.Config = cfg

	// Get merged blocklist
	emails, err := handler.getMergedBlocklist()
	if err != nil {
		t.Fatalf("Failed to get merged blocklist: %v", err)
	}

	// Check that blocklist includes emails from file
	if len(emails) != 2 {
		t.Errorf("Expected 2 blocked emails, got %d: %v", len(emails), emails)
	}

	found := make(map[string]bool)
	for _, email := range emails {
		found[email] = true
	}

	if !found["file-blocked@example.com"] {
		t.Error("file-blocked@example.com not in blocklist")
	}
	if !found["file-spam@example.com"] {
		t.Error("file-spam@example.com not in blocklist")
	}
}

// TestBlocklistReloadMergesBothSources tests that blocklist merges from both config.json and blocklist.json.
func TestBlocklistReloadMergesBothSources(t *testing.T) {
	// Create blocklist.json
	blocklistPath := "test_merged_blocklist.json"
	blocklist := map[string]interface{}{
		"blockedEmails": []string{"file-blocked@example.com"},
	}

	data, err := json.Marshal(blocklist)
	if err != nil {
		t.Fatalf("Failed to marshal blocklist: %v", err)
	}

	if err := ioutil.WriteFile(blocklistPath, data, 0644); err != nil {
		t.Fatalf("Failed to write blocklist file: %v", err)
	}
	defer os.Remove(blocklistPath)

	// Create handler
	_, conn, handler := setupTestServer(t)
	defer conn.Close()

	// Create config with both sources
	cfg := &config.Config{
		MyDomain:         "http://localhost:8080",
		BlocklistPath:    blocklistPath,
		BlockedUsersList: []string{"config-blocked@example.com"},
	}
	handler.Config = cfg

	// Get merged blocklist
	emails, err := handler.getMergedBlocklist()
	if err != nil {
		t.Fatalf("Failed to get merged blocklist: %v", err)
	}

	// Should have 2 unique emails from both sources
	if len(emails) != 2 {
		t.Errorf("Expected 2 blocked emails, got %d: %v", len(emails), emails)
	}

	found := make(map[string]bool)
	for _, email := range emails {
		found[email] = true
	}

	if !found["config-blocked@example.com"] {
		t.Error("config-blocked@example.com not in merged blocklist")
	}
	if !found["file-blocked@example.com"] {
		t.Error("file-blocked@example.com not in merged blocklist")
	}
}

// TestBlocklistReloadMtimeCache tests that blocklist is only reloaded when file mtime changes.
func TestBlocklistReloadMtimeCache(t *testing.T) {
	// Create blocklist.json
	blocklistPath := "test_mtime_blocklist.json"
	blocklist := map[string]interface{}{
		"blockedEmails": []string{"initial@example.com"},
	}

	data, err := json.Marshal(blocklist)
	if err != nil {
		t.Fatalf("Failed to marshal blocklist: %v", err)
	}

	if err := ioutil.WriteFile(blocklistPath, data, 0644); err != nil {
		t.Fatalf("Failed to write blocklist file: %v", err)
	}
	defer os.Remove(blocklistPath)

	// Create handler
	_, conn, handler := setupTestServer(t)
	defer conn.Close()

	cfg := &config.Config{
		MyDomain:      "http://localhost:8080",
		BlocklistPath: blocklistPath,
	}
	handler.Config = cfg

	// First load
	emails1, err := handler.getMergedBlocklist()
	if err != nil {
		t.Fatalf("Failed first load: %v", err)
	}

	if len(emails1) != 1 || emails1[0] != "initial@example.com" {
		t.Errorf("First load failed: got %v", emails1)
	}

	// Second load (file unchanged, should use cache)
	emails2, err := handler.getMergedBlocklist()
	if err != nil {
		t.Fatalf("Failed second load: %v", err)
	}

	if len(emails2) != 1 || emails2[0] != "initial@example.com" {
		t.Errorf("Second load failed: got %v", emails2)
	}

	// Wait for mtime to change (Unix mtime has 1-second granularity)
	time.Sleep(1100 * time.Millisecond)

	// Update blocklist file
	newBlocklist := map[string]interface{}{
		"blockedEmails": []string{"initial@example.com", "new@example.com"},
	}

	newData, err := json.Marshal(newBlocklist)
	if err != nil {
		t.Fatalf("Failed to marshal updated blocklist: %v", err)
	}

	if err := ioutil.WriteFile(blocklistPath, newData, 0644); err != nil {
		t.Fatalf("Failed to write updated blocklist file: %v", err)
	}

	// Third load (file changed, should reload)
	emails3, err := handler.getMergedBlocklist()
	if err != nil {
		t.Fatalf("Failed third load: %v", err)
	}

	if len(emails3) != 2 {
		t.Errorf("Third load failed, expected 2 emails, got %d: %v", len(emails3), emails3)
	}

	found := make(map[string]bool)
	for _, email := range emails3 {
		found[email] = true
	}

	if !found["new@example.com"] {
		t.Error("new@example.com not in reloaded blocklist")
	}
}

// TestBlocklistReloadNormalizeEmail tests that email normalization works correctly.
func TestBlocklistReloadNormalizeEmail(t *testing.T) {
	// Create handler
	_, conn, handler := setupTestServer(t)
	defer conn.Close()

	// Create config with mixed-case emails
	cfg := &config.Config{
		MyDomain:         "http://localhost:8080",
		BlocklistPath:    "",
		BlockedUsersList: []string{"Blocked@EXAMPLE.COM", "  SPAM@example.com  "},
	}
	handler.Config = cfg

	// Get blocklist
	emails, err := handler.getMergedBlocklist()
	if err != nil {
		t.Fatalf("Failed to get merged blocklist: %v", err)
	}

	// Check normalization
	if len(emails) != 2 {
		t.Errorf("Expected 2 emails, got %d", len(emails))
	}

	found := make(map[string]bool)
	for _, email := range emails {
		found[email] = true
	}

	// All should be lowercase
	if !found["blocked@example.com"] {
		t.Error("blocked@example.com not found (normalization failed)")
	}
	if !found["spam@example.com"] {
		t.Error("spam@example.com not found (normalization failed)")
	}
}

// TestCheckBlocklistWithReload tests the full checkBlocklist flow with reload.
func TestCheckBlocklistWithReload(t *testing.T) {
	// Create blocklist.json
	blocklistPath := "test_check_blocklist.json"
	blocklist := map[string]interface{}{
		"blockedEmails": []string{"blocked@example.com"},
	}

	data, err := json.Marshal(blocklist)
	if err != nil {
		t.Fatalf("Failed to marshal blocklist: %v", err)
	}

	if err := ioutil.WriteFile(blocklistPath, data, 0644); err != nil {
		t.Fatalf("Failed to write blocklist file: %v", err)
	}
	defer os.Remove(blocklistPath)

	// Create handler
	_, conn, handler := setupTestServer(t)
	defer conn.Close()

	cfg := &config.Config{
		MyDomain:      "http://localhost:8080",
		BlocklistPath: blocklistPath,
	}
	handler.Config = cfg

	// Check blocked email - should return false (not allowed)
	isAllowed := handler.checkBlocklist("blocked@example.com")
	if isAllowed {
		t.Error("blocked@example.com should not be allowed")
	}

	// Check unblocked email - should return true (allowed)
	isAllowed = handler.checkBlocklist("allowed@example.com")
	if !isAllowed {
		t.Error("allowed@example.com should be allowed")
	}
}
