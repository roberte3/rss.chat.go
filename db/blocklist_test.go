package db

import (
	"testing"
)

func TestSyncBlocklistToDB(t *testing.T) {
	conn := setupTestDB(t)

	emails := []string{"blocked@example.com", "spam@test.com", "bad@domain.com"}
	err := SyncBlocklistToDB(conn, emails)
	if err != nil {
		t.Fatalf("SyncBlocklistToDB failed: %v", err)
	}

	// Verify emails were inserted
	result, err := GetBlocklistEmails(conn)
	if err != nil {
		t.Fatalf("GetBlocklistEmails failed: %v", err)
	}

	if len(result) != 3 {
		t.Errorf("expected 3 emails, got %d", len(result))
	}

	// Check specific emails
	emailMap := make(map[string]bool)
	for _, email := range result {
		emailMap[email] = true
	}

	if !emailMap["blocked@example.com"] {
		t.Errorf("blocked@example.com not in blocklist")
	}
	if !emailMap["spam@test.com"] {
		t.Errorf("spam@test.com not in blocklist")
	}
	if !emailMap["bad@domain.com"] {
		t.Errorf("bad@domain.com not in blocklist")
	}
}

func TestSyncBlocklistReplaces(t *testing.T) {
	conn := setupTestDB(t)

	// First sync
	emails1 := []string{"old@example.com", "remove@test.com"}
	if err := SyncBlocklistToDB(conn, emails1); err != nil {
		t.Fatalf("first sync failed: %v", err)
	}

	// Second sync should replace
	emails2 := []string{"new@example.com"}
	if err := SyncBlocklistToDB(conn, emails2); err != nil {
		t.Fatalf("second sync failed: %v", err)
	}

	result, err := GetBlocklistEmails(conn)
	if err != nil {
		t.Fatalf("GetBlocklistEmails failed: %v", err)
	}

	if len(result) != 1 {
		t.Errorf("expected 1 email after replacement, got %d", len(result))
	}

	if result[0] != "new@example.com" {
		t.Errorf("expected new@example.com, got %s", result[0])
	}
}

func TestIsEmailBlocked(t *testing.T) {
	conn := setupTestDB(t)

	emails := []string{"blocked@example.com", "spam@test.com"}
	if err := SyncBlocklistToDB(conn, emails); err != nil {
		t.Fatalf("SyncBlocklistToDB failed: %v", err)
	}

	tests := []struct {
		email    string
		expected bool
	}{
		{"blocked@example.com", true},
		{"BLOCKED@EXAMPLE.COM", true}, // Case-insensitive
		{"spam@test.com", true},
		{"allowed@example.com", false},
		{"", false},
	}

	for _, tt := range tests {
		blocked, err := IsEmailBlocked(conn, tt.email)
		if err != nil {
			t.Errorf("IsEmailBlocked(%q) failed: %v", tt.email, err)
			continue
		}
		if blocked != tt.expected {
			t.Errorf("IsEmailBlocked(%q) = %v, expected %v", tt.email, blocked, tt.expected)
		}
	}
}

func TestClearBlocklist(t *testing.T) {
	conn := setupTestDB(t)

	emails := []string{"blocked@example.com", "spam@test.com"}
	if err := SyncBlocklistToDB(conn, emails); err != nil {
		t.Fatalf("SyncBlocklistToDB failed: %v", err)
	}

	if err := ClearBlocklist(conn); err != nil {
		t.Fatalf("ClearBlocklist failed: %v", err)
	}

	result, err := GetBlocklistEmails(conn)
	if err != nil {
		t.Fatalf("GetBlocklistEmails failed: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("expected 0 emails after clear, got %d", len(result))
	}
}

func TestGetBlocklistEmails(t *testing.T) {
	conn := setupTestDB(t)

	// Empty blocklist
	result, err := GetBlocklistEmails(conn)
	if err != nil {
		t.Fatalf("GetBlocklistEmails failed: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty blocklist, got %d emails", len(result))
	}

	// Add emails
	emails := []string{"test@example.com"}
	if err := SyncBlocklistToDB(conn, emails); err != nil {
		t.Fatalf("SyncBlocklistToDB failed: %v", err)
	}

	result, err = GetBlocklistEmails(conn)
	if err != nil {
		t.Fatalf("GetBlocklistEmails failed: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 email, got %d", len(result))
	}
}
