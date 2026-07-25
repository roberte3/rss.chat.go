package db

import (
	"os"
	"path/filepath"
	"testing"
)


func TestStoreAndGetMedia(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "media_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "media.db")
	mediaDB, err := OpenMediaDB(dbPath)
	if err != nil {
		t.Fatalf("failed to open media database: %v", err)
	}
	defer mediaDB.Close()

	// Test data (fake JPEG bytes)
	testData := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46}
	contentType := "image/jpeg"
	screenname := "alice"

	// Store media
	mediaID, err := StoreMedia(mediaDB, screenname, contentType, testData)
	if err != nil {
		t.Fatalf("failed to store media: %v", err)
	}

	if mediaID <= 0 {
		t.Errorf("mediaID = %d, want > 0", mediaID)
	}

	// Retrieve media
	retrievedType, retrievedData, err := GetMedia(mediaDB, mediaID)
	if err != nil {
		t.Fatalf("failed to get media: %v", err)
	}

	if retrievedType != contentType {
		t.Errorf("contentType = %s, want %s", retrievedType, contentType)
	}

	if len(retrievedData) != len(testData) {
		t.Errorf("data length = %d, want %d", len(retrievedData), len(testData))
	}

	for i, b := range retrievedData {
		if b != testData[i] {
			t.Errorf("data[%d] = %x, want %x", i, b, testData[i])
		}
	}
}

func TestGetMediaInfo(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "media_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "media.db")
	mediaDB, err := OpenMediaDB(dbPath)
	if err != nil {
		t.Fatalf("failed to open media database: %v", err)
	}
	defer mediaDB.Close()

	testData := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10}
	contentType := "image/jpeg"
	screenname := "bob"

	mediaID, err := StoreMedia(mediaDB, screenname, contentType, testData)
	if err != nil {
		t.Fatalf("failed to store media: %v", err)
	}

	// Get media info
	info, err := GetMediaInfo(mediaDB, mediaID)
	if err != nil {
		t.Fatalf("failed to get media info: %v", err)
	}

	if info.ID != mediaID {
		t.Errorf("ID = %d, want %d", info.ID, mediaID)
	}

	if info.Screenname != screenname {
		t.Errorf("Screenname = %s, want %s", info.Screenname, screenname)
	}

	if info.ContentType != contentType {
		t.Errorf("ContentType = %s, want %s", info.ContentType, contentType)
	}

	if info.Size != int64(len(testData)) {
		t.Errorf("Size = %d, want %d", info.Size, len(testData))
	}
}

func TestDeleteMedia(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "media_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "media.db")
	mediaDB, err := OpenMediaDB(dbPath)
	if err != nil {
		t.Fatalf("failed to open media database: %v", err)
	}
	defer mediaDB.Close()

	testData := []byte{0xFF, 0xD8, 0xFF}
	mediaID, _ := StoreMedia(mediaDB, "alice", "image/jpeg", testData)

	// Delete media
	err = DeleteMedia(mediaDB, mediaID)
	if err != nil {
		t.Fatalf("failed to delete media: %v", err)
	}

	// Verify it's gone
	_, _, err = GetMedia(mediaDB, mediaID)
	if err == nil {
		t.Error("media should be deleted but was found")
	}
}

func TestGetUserMedia(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "media_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "media.db")
	mediaDB, err := OpenMediaDB(dbPath)
	if err != nil {
		t.Fatalf("failed to open media database: %v", err)
	}
	defer mediaDB.Close()

	// Store multiple media items from same user
	testData := []byte{0xFF, 0xD8, 0xFF}
	StoreMedia(mediaDB, "alice", "image/jpeg", testData)
	StoreMedia(mediaDB, "alice", "image/jpeg", testData)
	StoreMedia(mediaDB, "bob", "image/jpeg", testData)

	// Get user media
	aliceMedia, err := GetUserMedia(mediaDB, "alice", 10)
	if err != nil {
		t.Fatalf("failed to get user media: %v", err)
	}

	if len(aliceMedia) != 2 {
		t.Errorf("alice media count = %d, want 2", len(aliceMedia))
	}

	for _, m := range aliceMedia {
		if m.Screenname != "alice" {
			t.Errorf("screenname = %s, want alice", m.Screenname)
		}
	}

	// Get bob media
	bobMedia, err := GetUserMedia(mediaDB, "bob", 10)
	if err != nil {
		t.Fatalf("failed to get user media: %v", err)
	}

	if len(bobMedia) != 1 {
		t.Errorf("bob media count = %d, want 1", len(bobMedia))
	}
}

func TestGetMediaNotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "media_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "media.db")
	mediaDB, err := OpenMediaDB(dbPath)
	if err != nil {
		t.Fatalf("failed to open media database: %v", err)
	}
	defer mediaDB.Close()

	// Try to get non-existent media
	_, _, err = GetMedia(mediaDB, 999)
	if err == nil {
		t.Error("should get error for non-existent media")
	}

	// Try to get non-existent media info
	_, err = GetMediaInfo(mediaDB, 999)
	if err == nil {
		t.Error("should get error for non-existent media info")
	}
}
