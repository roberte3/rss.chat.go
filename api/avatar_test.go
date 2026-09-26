package api

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/roberte3/rss.chat.go/db"
)

// Test image data with proper magic bytes
var (
	// PNG magic bytes (0x89 0x50 0x4E 0x47) followed by IHDR chunk header
	pngImageData = []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, // PNG signature
		0x00, 0x00, 0x00, 0x0D, // IHDR chunk length
		0x49, 0x48, 0x44, 0x52, // "IHDR"
		0x00, 0x00, 0x00, 0x01, // width: 1
		0x00, 0x00, 0x00, 0x01, // height: 1
		0x08, 0x02, 0x00, 0x00, 0x00, // bit depth, color type, compression, filter, interlace
		0x90, 0x77, 0x53, 0xDE, // CRC
	}

	// JPEG magic bytes (0xFF 0xD8 0xFF) followed by JFIF marker
	jpegImageData = []byte{
		0xFF, 0xD8, 0xFF, 0xE0, // SOI + APP0
		0x00, 0x10, // length
		0x4A, 0x46, 0x49, 0x46, 0x00, // "JFIF\0"
		0x01, 0x01, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00,
	}
)

// TestUploadAvatarValidation verifies avatar upload validation
func TestUploadAvatarValidation(t *testing.T) {
	mux, conn, _ := setupTestServer(t)

	insertTestUser(t, conn, "alice", "secret_alice")

	tests := []struct {
		name        string
		data        string
		contentType string
		expectError bool
	}{
		{
			name:        "Missing data",
			data:        "",
			contentType: "image/png",
			expectError: true,
		},
		{
			name:        "Missing content type",
			data:        base64.StdEncoding.EncodeToString(pngImageData),
			contentType: "",
			expectError: true,
		},
		{
			name:        "Invalid base64",
			data:        "not-base64!!!",
			contentType: "image/png",
			expectError: true,
		},
		{
			name:        "Non-image content type",
			data:        base64.StdEncoding.EncodeToString([]byte("text content")),
			contentType: "text/plain",
			expectError: true,
		},
		{
			name:        "Valid PNG",
			data:        base64.StdEncoding.EncodeToString(pngImageData),
			contentType: "image/png",
			expectError: false,
		},
		{
			name:        "Valid JPEG",
			data:        base64.StdEncoding.EncodeToString(jpegImageData),
			contentType: "image/jpeg",
			expectError: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			form := url.Values{
				"data":        {test.data},
				"contentType": {test.contentType},
			}

			w := authedPost(mux, "/uploadavatar", "alice", "secret_alice", form)

			hasError := w.Code != http.StatusOK
			if hasError != test.expectError {
				t.Errorf("expectError=%v, got code=%d, body=%s", test.expectError, w.Code, w.Body.String())
			}
		})
	}
}

// TestUploadAvatarSuccess verifies successful avatar upload
func TestUploadAvatarSuccess(t *testing.T) {
	mux, conn, _ := setupTestServer(t)

	insertTestUser(t, conn, "alice", "secret_alice")

	// Upload avatar
	form := url.Values{
		"data":        {base64.StdEncoding.EncodeToString(pngImageData)},
		"contentType": {"image/png"},
	}

	w := authedPost(mux, "/uploadavatar", "alice", "secret_alice", form)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, expected %d: %s", w.Code, http.StatusOK, w.Body.String())
	}

	// Parse response
	var response AvatarResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response.AvatarURL == "" {
		t.Errorf("expected avatarUrl in response, got empty")
	}

	if response.Size != int64(len(pngImageData)) {
		t.Errorf("expected size %d, got %d", len(pngImageData), response.Size)
	}

	// Verify user's imageUrl was updated
	user, err := db.GetUserInfoByScreenname(conn, "alice")
	if err != nil {
		t.Fatalf("failed to get user: %v", err)
	}

	if user.ImageURL != response.AvatarURL {
		t.Errorf("expected user.ImageURL=%q, got %q", response.AvatarURL, user.ImageURL)
	}
}

// TestUploadAvatarMultiple verifies replacing an existing avatar
func TestUploadAvatarMultiple(t *testing.T) {
	mux, conn, _ := setupTestServer(t)

	insertTestUser(t, conn, "alice", "secret_alice")

	// Upload first avatar
	form := url.Values{
		"data":        {base64.StdEncoding.EncodeToString(pngImageData)},
		"contentType": {"image/png"},
	}

	w := authedPost(mux, "/uploadavatar", "alice", "secret_alice", form)
	var firstResponse AvatarResponse
	json.Unmarshal(w.Body.Bytes(), &firstResponse)

	// Upload second avatar
	form = url.Values{
		"data":        {base64.StdEncoding.EncodeToString(jpegImageData)},
		"contentType": {"image/jpeg"},
	}

	w = authedPost(mux, "/uploadavatar", "alice", "secret_alice", form)
	var secondResponse AvatarResponse
	json.Unmarshal(w.Body.Bytes(), &secondResponse)

	if firstResponse.AvatarURL == secondResponse.AvatarURL {
		t.Errorf("expected different URLs for second upload, both got %q", firstResponse.AvatarURL)
	}

	// Verify user's imageUrl is the new one
	user, err := db.GetUserInfoByScreenname(conn, "alice")
	if err != nil {
		t.Fatalf("failed to get user: %v", err)
	}

	if user.ImageURL != secondResponse.AvatarURL {
		t.Errorf("expected user.ImageURL to be updated to %q, got %q", secondResponse.AvatarURL, user.ImageURL)
	}
}

// TestUploadAvatarFormatNegotiation verifies different image formats are accepted
func TestUploadAvatarFormatNegotiation(t *testing.T) {
	mux, conn, _ := setupTestServer(t)

	insertTestUser(t, conn, "alice", "secret_alice")

	formats := []struct {
		name        string
		contentType string
		data        []byte
	}{
		{"PNG", "image/png", pngImageData},
		{"JPEG", "image/jpeg", jpegImageData},
	}

	for _, format := range formats {
		t.Run(format.name, func(t *testing.T) {
			form := url.Values{
				"data":        {base64.StdEncoding.EncodeToString(format.data)},
				"contentType": {format.contentType},
			}

			w := authedPost(mux, "/uploadavatar", "alice", "secret_alice", form)

			if w.Code != http.StatusOK {
				t.Errorf("status = %d for %s, expected %d", w.Code, format.name, http.StatusOK)
			}

			var response AvatarResponse
			if err := json.Unmarshal(w.Body.Bytes(), &response); err == nil {
				// Verify the avatar is stored in media database
				if response.AvatarURL == "" {
					t.Errorf("expected avatarUrl for %s format", format.name)
				}
			}
		})
	}
}

// TestUploadAvatarUnauthorized verifies unauthenticated upload is rejected
func TestUploadAvatarUnauthorized(t *testing.T) {
	mux, _, _ := setupTestServer(t)

	form := url.Values{
		"data":        {base64.StdEncoding.EncodeToString(pngImageData)},
		"contentType": {"image/png"},
	}

	// No auth
	req := httptest.NewRequest("POST", "/uploadavatar", nil)
	req.Form = form
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code == http.StatusOK {
		t.Errorf("status = %d, expected error for unauthenticated request", w.Code)
	}
}

// TestGetUserDataIncludesAvatar verifies avatar URL is included in user data
func TestGetUserDataIncludesAvatar(t *testing.T) {
	mux, conn, _ := setupTestServer(t)

	insertTestUser(t, conn, "alice", "secret_alice")

	// Upload avatar
	form := url.Values{
		"data":        {base64.StdEncoding.EncodeToString(pngImageData)},
		"contentType": {"image/png"},
	}

	w := authedPost(mux, "/uploadavatar", "alice", "secret_alice", form)
	var avatarResponse AvatarResponse
	json.Unmarshal(w.Body.Bytes(), &avatarResponse)

	// Get user data
	req := httptest.NewRequest("GET", "/getuserdata?screenname=alice", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, expected %d: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var userData db.User
	if err := json.Unmarshal(w.Body.Bytes(), &userData); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if userData.ImageURL != avatarResponse.AvatarURL {
		t.Errorf("expected imageUrl=%q, got %q", avatarResponse.AvatarURL, userData.ImageURL)
	}
}
