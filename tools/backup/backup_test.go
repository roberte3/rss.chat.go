package main

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func createTestDatabase(t *testing.T, path string) *sql.DB {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	// Create tables
	schema := `
		CREATE TABLE users (
			screenname TEXT PRIMARY KEY,
			emailAddress TEXT,
			emailSecret TEXT,
			imageUrl TEXT,
			prefs TEXT,
			ctHits INTEGER DEFAULT 0,
			ctHitsToday INTEGER DEFAULT 0,
			whenLastHit DATETIME,
			whenCreated TEXT DEFAULT CURRENT_TIMESTAMP,
			whenUpdated TEXT DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			screenname TEXT NOT NULL,
			text TEXT NOT NULL,
			guidHash TEXT UNIQUE,
			idParent INTEGER,
			whenCreated TEXT DEFAULT CURRENT_TIMESTAMP,
			whenUpdated TEXT DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (screenname) REFERENCES users(screenname)
		);

		CREATE TABLE likes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			itemId INTEGER NOT NULL,
			screenname TEXT NOT NULL,
			whenCreated TEXT DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (itemId) REFERENCES items(id),
			FOREIGN KEY (screenname) REFERENCES users(screenname)
		);
	`

	if _, err := conn.Exec(schema); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	return conn
}

func createTestMediaDatabase(t *testing.T, path string) *sql.DB {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("failed to open media database: %v", err)
	}

	schema := `
		CREATE TABLE media (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			screenname TEXT NOT NULL COLLATE NOCASE,
			contentType TEXT NOT NULL,
			mediabytes BLOB NOT NULL,
			size INTEGER NOT NULL,
			whenCreated TEXT DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_media_screenname ON media(screenname);
	`

	if _, err := conn.Exec(schema); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	return conn
}

func TestExportRoundtrip(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "backup_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	mediaDBPath := filepath.Join(tmpDir, "media.db")
	backupPath := filepath.Join(tmpDir, "backup.json")

	// Create and populate databases
	mainDB := createTestDatabase(t, dbPath)
	defer mainDB.Close()

	mediaDB := createTestMediaDatabase(t, mediaDBPath)
	defer mediaDB.Close()

	// Insert test data
	_, err = mainDB.Exec(
		"INSERT INTO users (screenname, emailAddress, emailSecret, whenCreated, whenUpdated) VALUES (?, ?, ?, ?, ?)",
		"alice", "alice@example.com", "secret123", "2024-01-01T00:00:00Z", "2024-01-01T00:00:00Z",
	)
	if err != nil {
		t.Fatalf("failed to insert user: %v", err)
	}

	// Insert item with explicit ID
	_, err = mainDB.Exec(
		"INSERT INTO items (id, screenname, text, guidHash, idParent, whenCreated, whenUpdated) VALUES (?, ?, ?, ?, ?, ?, ?)",
		1, "alice", "Hello world", "guid123", 0, "2024-01-01T01:00:00Z", "2024-01-01T01:00:00Z",
	)
	if err != nil {
		t.Fatalf("failed to insert item: %v", err)
	}

	// Insert like with explicit ID
	_, err = mainDB.Exec(
		"INSERT INTO likes (id, itemId, screenname, whenCreated) VALUES (?, ?, ?, ?)",
		1, 1, "alice", "2024-01-01T02:00:00Z",
	)
	if err != nil {
		t.Fatalf("failed to insert like: %v", err)
	}

	// Insert media
	testData := []byte{0xFF, 0xD8, 0xFF, 0xE0}
	_, err = mediaDB.Exec(
		"INSERT INTO media (id, screenname, contentType, mediabytes, size, whenCreated) VALUES (?, ?, ?, ?, ?, ?)",
		1, "alice", "image/jpeg", testData, int64(len(testData)), "2024-01-01T03:00:00Z",
	)
	if err != nil {
		t.Fatalf("failed to insert media: %v", err)
	}

	// Export
	users, err := exportUsers(mainDB)
	if err != nil {
		t.Fatalf("failed to export users: %v", err)
	}

	items, err := exportItems(mainDB)
	if err != nil {
		t.Fatalf("failed to export items: %v", err)
	}

	likes, err := exportLikes(mainDB)
	if err != nil {
		t.Fatalf("failed to export likes: %v", err)
	}

	media, err := exportMedia(mediaDB)
	if err != nil {
		t.Fatalf("failed to export media: %v", err)
	}

	// Verify exports
	if len(users) != 1 {
		t.Errorf("expected 1 user, got %d", len(users))
	}
	if users[0].Screenname != "alice" {
		t.Errorf("expected screenname 'alice', got %s", users[0].Screenname)
	}

	if len(items) != 1 {
		t.Errorf("expected 1 item, got %d", len(items))
	}
	if items[0].ID != 1 || items[0].Screenname != "alice" {
		t.Errorf("item data mismatch: ID=%d, Screenname=%s", items[0].ID, items[0].Screenname)
	}

	if len(likes) != 1 {
		t.Errorf("expected 1 like, got %d", len(likes))
	}
	if likes[0].ID != 1 || likes[0].ItemID != 1 {
		t.Errorf("like data mismatch: ID=%d, ItemID=%d", likes[0].ID, likes[0].ItemID)
	}

	if len(media) != 1 {
		t.Errorf("expected 1 media, got %d", len(media))
	}
	if media[0].ID != 1 || media[0].Screenname != "alice" {
		t.Errorf("media data mismatch: ID=%d, Screenname=%s", media[0].ID, media[0].Screenname)
	}
	if media[0].Size != int64(len(testData)) {
		t.Errorf("media size mismatch: expected %d, got %d", len(testData), media[0].Size)
	}

	// Create backup structure
	backup := BackupData{
		ExportedAt: "2024-01-01T00:00:00Z",
		Users:      users,
		Items:      items,
		Likes:      likes,
		Media:      media,
	}

	// Write backup
	data, err := json.MarshalIndent(backup, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal backup: %v", err)
	}

	if err := os.WriteFile(backupPath, data, 0600); err != nil {
		t.Fatalf("failed to write backup: %v", err)
	}

	// Verify backup file exists and has content
	if _, err := os.Stat(backupPath); err != nil {
		t.Errorf("backup file not found: %v", err)
	}

	// Verify JSON is valid
	var parsed BackupData
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Errorf("backup JSON is invalid: %v", err)
	}

	if len(parsed.Users) != 1 || len(parsed.Items) != 1 || len(parsed.Likes) != 1 || len(parsed.Media) != 1 {
		t.Error("backup JSON has wrong counts")
	}
}

func TestMediaDataPreservesIDs(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "backup_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	mainDB := createTestDatabase(t, dbPath)
	defer mainDB.Close()

	// Insert items with specific IDs
	for i := 1; i <= 5; i++ {
		_, err := mainDB.Exec(
			"INSERT INTO items (id, screenname, text, guidHash, idParent, whenCreated, whenUpdated) VALUES (?, ?, ?, ?, ?, ?, ?)",
			int64(i), "alice", "post", "guid"+string(rune('0'+i)), 0, "2024-01-01T00:00:00Z", "2024-01-01T00:00:00Z",
		)
		if err != nil {
			t.Fatalf("failed to insert item: %v", err)
		}
	}

	items, err := exportItems(mainDB)
	if err != nil {
		t.Fatalf("failed to export items: %v", err)
	}

	// Verify IDs are preserved in order
	for i, item := range items {
		if item.ID != int64(i+1) {
			t.Errorf("item ID mismatch at index %d: expected %d, got %d", i, i+1, item.ID)
		}
	}
}
