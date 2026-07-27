package api

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/roberte3/rss.chat.go/db"
)

func TestUploadMediaValidation(t *testing.T) {
	tests := []struct {
		name        string
		data        []byte
		contentType string
		shouldPass  bool
	}{
		{
			name:        "valid JPEG",
			data:        []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10},
			contentType: "image/jpeg",
			shouldPass:  true,
		},
		{
			name:        "valid PNG",
			data:        []byte{0x89, 0x50, 0x4E, 0x47},
			contentType: "image/png",
			shouldPass:  true,
		},
		{
			name:        "empty data",
			data:        []byte{},
			contentType: "image/jpeg",
			shouldPass:  false,
		},
		{
			name:        "invalid content type",
			data:        []byte{0xFF, 0xD8, 0xFF},
			contentType: "application/pdf",
			shouldPass:  false,
		},
		{
			name:        "signature mismatch (claim JPEG, send PNG)",
			data:        []byte{0x89, 0x50, 0x4E, 0x47},
			contentType: "image/jpeg",
			shouldPass:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMedia(tt.data, tt.contentType, 2*1024*1024)
			if (err == nil) != tt.shouldPass {
				t.Errorf("validateMedia error = %v, shouldPass = %v", err, tt.shouldPass)
			}
		})
	}
}

func TestVerifyMediaSignature(t *testing.T) {
	tests := []struct {
		name        string
		data        []byte
		contentType string
		shouldPass  bool
	}{
		{
			name:        "JPEG signature valid",
			data:        []byte{0xFF, 0xD8, 0xFF, 0xE0},
			contentType: "image/jpeg",
			shouldPass:  true,
		},
		{
			name:        "PNG signature valid",
			data:        []byte{0x89, 0x50, 0x4E, 0x47},
			contentType: "image/png",
			shouldPass:  true,
		},
		{
			name:        "GIF signature valid",
			data:        []byte{0x47, 0x49, 0x46, 0x38},
			contentType: "image/gif",
			shouldPass:  true,
		},
		{
			name:        "WebP signature valid",
			data:        []byte{0x52, 0x49, 0x46, 0x46, 0x00, 0x00, 0x00, 0x00, 0x57, 0x45, 0x42, 0x50},
			contentType: "image/webp",
			shouldPass:  true,
		},
		{
			name:        "SVG signature valid",
			data:        []byte{'<', 's', 'v', 'g'},
			contentType: "image/svg+xml",
			shouldPass:  true,
		},
		{
			name:        "JPEG signature invalid",
			data:        []byte{0x00, 0xD8, 0xFF},
			contentType: "image/jpeg",
			shouldPass:  false,
		},
		{
			name:        "PNG signature invalid",
			data:        []byte{0x00, 0x50, 0x4E, 0x47},
			contentType: "image/png",
			shouldPass:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := verifyMediaSignature(tt.data, tt.contentType)
			if result != tt.shouldPass {
				t.Errorf("verifyMediaSignature = %v, want %v", result, tt.shouldPass)
			}
		})
	}
}

func TestStoreToTempFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "temp_media_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	testData := []byte{0xFF, 0xD8, 0xFF}

	// Store to temp file
	filepath, err := storeToTempFile(testData, tmpDir)
	if err != nil {
		t.Fatalf("failed to store temp file: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(filepath); err != nil {
		t.Errorf("temp file does not exist: %v", err)
	}

	// Verify content
	readData, err := os.ReadFile(filepath)
	if err != nil {
		t.Fatalf("failed to read temp file: %v", err)
	}

	if len(readData) != len(testData) {
		t.Errorf("file size = %d, want %d", len(readData), len(testData))
	}

	// Clean up
	os.Remove(filepath)
}

func TestUploadMediaHandler(t *testing.T) {
	// Create test setup
	tmpDir, err := os.MkdirTemp("", "upload_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create databases
	dbPath := filepath.Join(tmpDir, "main.db")
	mainDB, err := setupTestDatabase(t, dbPath)
	if err != nil {
		t.Fatalf("failed to setup main database: %v", err)
	}
	defer mainDB.Close()

	mediaDBPath := filepath.Join(tmpDir, "media.db")
	mediaDB, err := db.OpenMediaDB(mediaDBPath)
	if err != nil {
		t.Fatalf("failed to open media database: %v", err)
	}
	defer mediaDB.Close()

	// Create temp media path
	tempMediaPath := filepath.Join(tmpDir, "temp_media")

	// Create test user
	CreateTestUser(t, mainDB, "alice", "alice@example.com", "secret123")

	// Create handler
	handler := &Handler{
		DB:                  mainDB,
		MediaDB:             mediaDB,
		MaxMediaUploadBytes: 2 * 1024 * 1024,
		TempMediaPath:       tempMediaPath,
	}

	// Create test data (fake JPEG)
	testData := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10}
	encodedData := base64.StdEncoding.EncodeToString(testData)

	// Create multipart form request
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add form fields
	writer.WriteField("data", encodedData)
	writer.WriteField("contentType", "image/jpeg")
	writer.Close()

	req := httptest.NewRequest("POST", "/uploadmedia", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	// Get user
	user := &db.User{
		Screenname:   "alice",
		EmailAddress: "alice@example.com",
		EmailSecret:  "secret123",
	}

	// Call handler
	handler.HandleUploadMedia(w, req, user)

	// Verify response
	if w.Code != http.StatusOK {
		t.Errorf("status code = %d, want %d. Response: %s", w.Code, http.StatusOK, w.Body.String())
	}

	// Verify response is valid JSON
	var response MediaResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v. Body: %s", err, w.Body.String())
	}

	if response.MediaID == 0 {
		t.Error("mediaID should be set")
	}

	if !strings.Contains(response.URL, "/media/") {
		t.Errorf("URL = %s, want /media/ prefix", response.URL)
	}

	if response.Size != int64(len(testData)) {
		t.Errorf("size = %d, want %d", response.Size, len(testData))
	}
}

// Helper functions
func setupTestDatabase(t *testing.T, path string) (*sql.DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	if _, err := conn.Exec(`
		CREATE TABLE IF NOT EXISTS users (
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
	`); err != nil {
		conn.Close()
		return nil, err
	}

	return conn, nil
}

func CreateTestUser(t *testing.T, conn *sql.DB, screenname, email, secret string) {
	err := db.AddUser(conn, screenname, email, secret)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}
}
