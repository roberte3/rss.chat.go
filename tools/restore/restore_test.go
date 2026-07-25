package main

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func createEmptyMainDB(t *testing.T, path string) *sql.DB {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

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

func createEmptyMediaDB(t *testing.T, path string) *sql.DB {
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

func TestRestoreRoundtrip(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "restore_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create source databases
	srcDBPath := filepath.Join(tmpDir, "source.db")
	srcMediaDBPath := filepath.Join(tmpDir, "source_media.db")
	srcMainDB := createEmptyMainDB(t, srcDBPath)
	srcMediaDB := createEmptyMediaDB(t, srcMediaDBPath)

	// Insert test data in source
	_, err = srcMainDB.Exec(
		"INSERT INTO users (screenname, emailAddress, emailSecret, whenCreated, whenUpdated) VALUES (?, ?, ?, ?, ?)",
		"alice", "alice@example.com", "secret123", "2024-01-01T00:00:00Z", "2024-01-01T00:00:00Z",
	)
	if err != nil {
		t.Fatalf("failed to insert user: %v", err)
	}

	_, err = srcMainDB.Exec(
		"INSERT INTO items (id, screenname, text, guidHash, idParent, whenCreated, whenUpdated) VALUES (?, ?, ?, ?, ?, ?, ?)",
		1, "alice", "Hello world", "guid123", 0, "2024-01-01T01:00:00Z", "2024-01-01T01:00:00Z",
	)
	if err != nil {
		t.Fatalf("failed to insert item: %v", err)
	}

	_, err = srcMainDB.Exec(
		"INSERT INTO likes (id, itemId, screenname, whenCreated) VALUES (?, ?, ?, ?)",
		1, 1, "alice", "2024-01-01T02:00:00Z",
	)
	if err != nil {
		t.Fatalf("failed to insert like: %v", err)
	}

	testData := []byte{0xFF, 0xD8, 0xFF, 0xE0}
	_, err = srcMediaDB.Exec(
		"INSERT INTO media (id, screenname, contentType, mediabytes, size, whenCreated) VALUES (?, ?, ?, ?, ?, ?)",
		1, "alice", "image/jpeg", testData, int64(len(testData)), "2024-01-01T03:00:00Z",
	)
	if err != nil {
		t.Fatalf("failed to insert media: %v", err)
	}

	// Create backup data
	backup := BackupData{
		ExportedAt: "2024-01-01T00:00:00Z",
	}

	// Query users for backup
	rows, err := srcMainDB.Query("SELECT screenname, emailAddress, emailSecret, imageUrl, prefs, whenCreated, whenUpdated FROM users")
	if err != nil {
		t.Fatalf("failed to query users: %v", err)
	}
	for rows.Next() {
		var u UserData
		var imageURL, prefs sql.NullString
		if err := rows.Scan(&u.Screenname, &u.EmailAddress, &u.EmailSecret, &imageURL, &prefs, &u.WhenCreated, &u.WhenUpdated); err != nil {
			t.Fatalf("failed to scan user: %v", err)
		}
		if imageURL.Valid {
			u.ImageURL = imageURL.String
		}
		if prefs.Valid {
			u.Prefs = prefs.String
		}
		backup.Users = append(backup.Users, u)
	}
	rows.Close()

	// Query items for backup
	rows, err = srcMainDB.Query("SELECT id, screenname, text, guidHash, idParent, whenCreated, whenUpdated FROM items")
	if err != nil {
		t.Fatalf("failed to query items: %v", err)
	}
	for rows.Next() {
		var i ItemData
		if err := rows.Scan(&i.ID, &i.Screenname, &i.Text, &i.GuidHash, &i.IDParent, &i.WhenCreated, &i.WhenUpdated); err != nil {
			t.Fatalf("failed to scan item: %v", err)
		}
		backup.Items = append(backup.Items, i)
	}
	rows.Close()

	// Query likes for backup
	rows, err = srcMainDB.Query("SELECT id, itemId, screenname, whenCreated FROM likes")
	if err != nil {
		t.Fatalf("failed to query likes: %v", err)
	}
	for rows.Next() {
		var l LikeData
		if err := rows.Scan(&l.ID, &l.ItemID, &l.Screenname, &l.WhenCreated); err != nil {
			t.Fatalf("failed to scan like: %v", err)
		}
		backup.Likes = append(backup.Likes, l)
	}
	rows.Close()

	// Query media for backup
	rows, err = srcMediaDB.Query("SELECT id, screenname, contentType, mediabytes, size, whenCreated FROM media")
	if err != nil {
		t.Fatalf("failed to query media: %v", err)
	}
	for rows.Next() {
		var m MediaData
		var data []byte
		if err := rows.Scan(&m.ID, &m.Screenname, &m.ContentType, &data, &m.Size, &m.WhenCreated); err != nil {
			t.Fatalf("failed to scan media: %v", err)
		}
		m.Data = base64.StdEncoding.EncodeToString(data)
		backup.Media = append(backup.Media, m)
	}
	rows.Close()

	srcMainDB.Close()
	srcMediaDB.Close()

	// Create backup file
	backupPath := filepath.Join(tmpDir, "backup.json")
	backupJSON, err := json.MarshalIndent(backup, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal backup: %v", err)
	}
	if err := os.WriteFile(backupPath, backupJSON, 0600); err != nil {
		t.Fatalf("failed to write backup file: %v", err)
	}

	// Create destination databases
	dstDBPath := filepath.Join(tmpDir, "destination.db")
	dstMediaDBPath := filepath.Join(tmpDir, "destination_media.db")
	dstMainDB := createEmptyMainDB(t, dstDBPath)
	dstMediaDB := createEmptyMediaDB(t, dstMediaDBPath)

	// Restore users
	if err := restoreUsers(dstMainDB, backup.Users); err != nil {
		t.Fatalf("failed to restore users: %v", err)
	}

	// Restore items
	if err := restoreItems(dstMainDB, backup.Items); err != nil {
		t.Fatalf("failed to restore items: %v", err)
	}

	// Restore likes
	if err := restoreLikes(dstMainDB, backup.Likes); err != nil {
		t.Fatalf("failed to restore likes: %v", err)
	}

	// Restore media
	if err := restoreMedia(dstMediaDB, backup.Media); err != nil {
		t.Fatalf("failed to restore media: %v", err)
	}

	// Verify restored data
	var count int

	// Check users
	dstMainDB.QueryRow("SELECT COUNT(*) FROM users WHERE screenname = ?", "alice").Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 user, got %d", count)
	}

	// Check items
	dstMainDB.QueryRow("SELECT COUNT(*) FROM items WHERE id = ?", 1).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 item, got %d", count)
	}

	// Check item data integrity
	var itemText string
	dstMainDB.QueryRow("SELECT text FROM items WHERE id = ?", 1).Scan(&itemText)
	if itemText != "Hello world" {
		t.Errorf("item text mismatch: expected 'Hello world', got '%s'", itemText)
	}

	// Check likes
	dstMainDB.QueryRow("SELECT COUNT(*) FROM likes WHERE id = ?", 1).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 like, got %d", count)
	}

	// Check media
	dstMediaDB.QueryRow("SELECT COUNT(*) FROM media WHERE id = ?", 1).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 media, got %d", count)
	}

	// Check media data integrity
	var mediaContentType string
	var mediaSize int64
	dstMediaDB.QueryRow("SELECT contentType, size FROM media WHERE id = ?", 1).Scan(&mediaContentType, &mediaSize)
	if mediaContentType != "image/jpeg" {
		t.Errorf("media content type mismatch: expected 'image/jpeg', got '%s'", mediaContentType)
	}
	if mediaSize != int64(len(testData)) {
		t.Errorf("media size mismatch: expected %d, got %d", len(testData), mediaSize)
	}

	// Verify media bytes are intact
	var mediaBytes []byte
	dstMediaDB.QueryRow("SELECT mediabytes FROM media WHERE id = ?", 1).Scan(&mediaBytes)
	if len(mediaBytes) != len(testData) {
		t.Errorf("media bytes length mismatch: expected %d, got %d", len(testData), len(mediaBytes))
	}
	for i, b := range mediaBytes {
		if b != testData[i] {
			t.Errorf("media byte mismatch at index %d: expected %x, got %x", i, testData[i], b)
		}
	}

	dstMainDB.Close()
	dstMediaDB.Close()
}

func TestIDPreservationOnRestore(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "restore_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	mainDB := createEmptyMainDB(t, dbPath)
	defer mainDB.Close()

	// Create backup with specific IDs
	backup := BackupData{
		Items: []ItemData{
			{ID: 100, Screenname: "alice", Text: "post1", GuidHash: "guid1"},
			{ID: 101, Screenname: "alice", Text: "post2", GuidHash: "guid2"},
			{ID: 200, Screenname: "bob", Text: "post3", GuidHash: "guid3"},
		},
	}

	if err := restoreItems(mainDB, backup.Items); err != nil {
		t.Fatalf("failed to restore items: %v", err)
	}

	// Verify IDs are preserved
	var id1, id2, id3 int64
	mainDB.QueryRow("SELECT id FROM items WHERE guidHash = ?", "guid1").Scan(&id1)
	mainDB.QueryRow("SELECT id FROM items WHERE guidHash = ?", "guid2").Scan(&id2)
	mainDB.QueryRow("SELECT id FROM items WHERE guidHash = ?", "guid3").Scan(&id3)

	if id1 != 100 || id2 != 101 || id3 != 200 {
		t.Errorf("ID preservation failed: got %d, %d, %d, want 100, 101, 200", id1, id2, id3)
	}
}
