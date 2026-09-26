package api

import (
	"database/sql"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/roberte3/rss.chat.go/config"
	"github.com/roberte3/rss.chat.go/db"
)

func writeBlocklistFile(t *testing.T, path string, mtime time.Time, emails ...string) {
	t.Helper()
	body := `{"blockedEmails": [`
	for i, e := range emails {
		if i > 0 {
			body += ","
		}
		body += `"` + e + `"`
	}
	body += `]}`
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatalf("write blocklist: %v", err)
	}
	// Set mtime explicitly: filesystem timestamp granularity varies, and a
	// rewrite within the same tick would otherwise look unchanged.
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
}

func blocklistRows(t *testing.T, conn *sql.DB) map[string]bool {
	t.Helper()
	emails, err := db.GetBlocklistEmails(conn)
	if err != nil {
		t.Fatalf("GetBlocklistEmails: %v", err)
	}
	m := make(map[string]bool, len(emails))
	for _, e := range emails {
		m[e] = true
	}
	return m
}

func assertAllowed(t *testing.T, h *Handler, email string, want bool) {
	t.Helper()
	if got := h.checkBlocklist(email); got != want {
		t.Errorf("checkBlocklist(%q) = %v, want %v", email, got, want)
	}
}

func TestBlocklistConfigListEnforcedWithoutFile(t *testing.T) {
	_, _, handler := setupTestServer(t)
	handler.Config = &config.Config{
		BlocklistPath:    filepath.Join(t.TempDir(), "missing.json"),
		BlockedUsersList: []string{"Blocked@EXAMPLE.com", "  spam@example.com  "},
	}

	assertAllowed(t, handler, "blocked@example.com", false)
	assertAllowed(t, handler, "SPAM@example.com", false)
	assertAllowed(t, handler, "fine@example.com", true)
}

func TestBlocklistMergesConfigAndFile(t *testing.T) {
	_, conn, handler := setupTestServer(t)
	path := filepath.Join(t.TempDir(), "blocklist.json")
	writeBlocklistFile(t, path, time.Now(), "file@example.com", "both@example.com")
	handler.Config = &config.Config{
		BlocklistPath:    path,
		BlockedUsersList: []string{"config@example.com", "BOTH@example.com"},
	}

	assertAllowed(t, handler, "file@example.com", false)
	assertAllowed(t, handler, "config@example.com", false)
	assertAllowed(t, handler, "both@example.com", false)
	assertAllowed(t, handler, "fine@example.com", true)

	if rows := blocklistRows(t, conn); len(rows) != 3 {
		t.Errorf("expected 3 de-duplicated rows, got %v", rows)
	}
}

func TestBlocklistHotReloadsOnFileChange(t *testing.T) {
	_, _, handler := setupTestServer(t)
	path := filepath.Join(t.TempDir(), "blocklist.json")
	base := time.Now().Add(-time.Hour)
	writeBlocklistFile(t, path, base, "old@example.com")
	handler.Config = &config.Config{BlocklistPath: path}

	assertAllowed(t, handler, "old@example.com", false)
	assertAllowed(t, handler, "new@example.com", true)

	writeBlocklistFile(t, path, base.Add(time.Minute), "new@example.com")

	assertAllowed(t, handler, "old@example.com", true)
	assertAllowed(t, handler, "new@example.com", false)
}

func TestBlocklistFileRemovalFallsBackToConfigList(t *testing.T) {
	_, _, handler := setupTestServer(t)
	path := filepath.Join(t.TempDir(), "blocklist.json")
	writeBlocklistFile(t, path, time.Now(), "file@example.com")
	handler.Config = &config.Config{
		BlocklistPath:    path,
		BlockedUsersList: []string{"config@example.com"},
	}

	assertAllowed(t, handler, "file@example.com", false)

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}

	assertAllowed(t, handler, "file@example.com", true)
	assertAllowed(t, handler, "config@example.com", false)
}

// The table is only rewritten when blocklist.json changes, not per request.
func TestBlocklistDoesNotResyncWhenUnchanged(t *testing.T) {
	_, conn, handler := setupTestServer(t)
	path := filepath.Join(t.TempDir(), "blocklist.json")
	writeBlocklistFile(t, path, time.Now(), "file@example.com")
	handler.Config = &config.Config{BlocklistPath: path}

	assertAllowed(t, handler, "file@example.com", false)

	// Overwrite the table behind the handler's back. A per-request resync
	// would wipe this sentinel out.
	if err := db.SyncBlocklistToDB(conn, []string{"sentinel@example.com"}); err != nil {
		t.Fatal(err)
	}

	assertAllowed(t, handler, "anyone@example.com", true)

	if rows := blocklistRows(t, conn); !rows["sentinel@example.com"] || len(rows) != 1 {
		t.Errorf("blocklist was resynced despite unchanged file: %v", rows)
	}
}

func TestBlocklistConcurrentChecks(t *testing.T) {
	_, _, handler := setupTestServer(t)
	path := filepath.Join(t.TempDir(), "blocklist.json")
	writeBlocklistFile(t, path, time.Now(), "blocked@example.com")
	handler.Config = &config.Config{BlocklistPath: path}

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if handler.checkBlocklist("blocked@example.com") {
				t.Error("blocked address was allowed")
			}
		}()
	}
	wg.Wait()
}
