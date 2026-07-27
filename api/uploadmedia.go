package api

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/roberte3/rss.chat.go/db"
)

// UploadMediaRequest holds the media upload data
type UploadMediaRequest struct {
	Data        string `json:"data"`        // Base64-encoded media data
	ContentType string `json:"contentType"` // MIME type (image/jpeg, image/png, etc)
}

// MediaResponse is the response after successful upload
type MediaResponse struct {
	MediaID     int64  `json:"mediaId"`
	URL         string `json:"url"`
	ContentType string `json:"contentType"`
	Size        int64  `json:"size"`
}

// HandleUploadMedia handles media uploads from authenticated users
func (h *Handler) HandleUploadMedia(w http.ResponseWriter, r *http.Request, user *db.User) {
	// Parse form data
	if err := r.ParseMultipartForm(10 * 1024 * 1024); err != nil {
		RespondError(w, "Can't upload media because "+err.Error())
		return
	}

	// Get media data (base64-encoded)
	dataStr := r.FormValue("data")
	if dataStr == "" {
		RespondError(w, "Can't upload media because data is required")
		return
	}

	// Get content type
	contentType := r.FormValue("contentType")
	if contentType == "" {
		RespondError(w, "Can't upload media because contentType is required")
		return
	}

	// Decode base64 data
	data, err := base64.StdEncoding.DecodeString(dataStr)
	if err != nil {
		RespondError(w, "Can't upload media because data is not valid base64")
		return
	}

	// Validate media
	if err := validateMedia(data, contentType, h.MaxMediaUploadBytes); err != nil {
		RespondError(w, "Can't upload media because "+err.Error())
		return
	}

	// Store to temporary file for safe handling
	tempFile, err := storeToTempFile(data, h.TempMediaPath)
	if err != nil {
		RespondError(w, "Can't upload media because "+err.Error())
		return
	}
	defer os.Remove(tempFile) // Clean up temp file after DB insert

	// Store in media database
	mediaID, err := db.StoreMedia(h.MediaDB, user.Screenname, contentType, data)
	if err != nil {
		RespondError(w, "Can't upload media because "+err.Error())
		return
	}

	// Return success response
	response := MediaResponse{
		MediaID:     mediaID,
		URL:         fmt.Sprintf("/media/%d", mediaID),
		ContentType: contentType,
		Size:        int64(len(data)),
	}

	RespondJSON(w, response)
}

// HandleGetMedia serves stored media files
func (h *Handler) HandleGetMedia(w http.ResponseWriter, r *http.Request) {
	// Extract media ID from URL path
	// Assumes route like /media/{id}
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 3 {
		RespondError(w, "Can't get media because id is required")
		return
	}

	var mediaID int64
	if _, err := fmt.Sscanf(pathParts[2], "%d", &mediaID); err != nil {
		RespondError(w, "Can't get media because id is invalid")
		return
	}

	// Retrieve from media database
	contentType, data, err := db.GetMedia(h.MediaDB, mediaID)
	if err != nil {
		RespondError(w, "Can't get media because "+err.Error())
		return
	}

	// Set content type and serve data
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

// validateMedia checks if media is acceptable
func validateMedia(data []byte, contentType string, maxBytes int) error {
	// Check size
	if len(data) == 0 {
		return fmt.Errorf("media is empty")
	}

	if len(data) > maxBytes {
		return fmt.Errorf("media exceeds maximum size of %d bytes", maxBytes)
	}

	// Validate content type is image
	validTypes := map[string]bool{
		"image/jpeg":    true,
		"image/png":     true,
		"image/gif":     true,
		"image/webp":    true,
		"image/svg+xml": true,
	}

	if !validTypes[contentType] {
		return fmt.Errorf("content type %q is not allowed (must be image/*)", contentType)
	}

	// Check magic bytes to prevent spoofed content types
	if !verifyMediaSignature(data, contentType) {
		return fmt.Errorf("media signature does not match content type")
	}

	return nil
}

// verifyMediaSignature checks that the file header matches the claimed content type
func verifyMediaSignature(data []byte, contentType string) bool {
	if len(data) < 4 {
		return false
	}

	// Check magic bytes
	switch contentType {
	case "image/jpeg":
		// JPEG: FF D8 FF
		return data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF
	case "image/png":
		// PNG: 89 50 4E 47
		return data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47
	case "image/gif":
		// GIF: 47 49 46 38 (GIF8)
		return len(data) >= 3 && data[0] == 0x47 && data[1] == 0x49 && data[2] == 0x46
	case "image/webp":
		// WebP: RIFF ... WEBP
		return len(data) >= 12 && data[0] == 0x52 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x46 &&
			data[8] == 0x57 && data[9] == 0x45 && data[10] == 0x42 && data[11] == 0x50
	case "image/svg+xml":
		// SVG: XML declaration or <svg
		str := string(data[:min(100, len(data))])
		return strings.Contains(str, "<?xml") || strings.Contains(str, "<svg")
	default:
		return false
	}
}

// storeToTempFile writes media data to a temporary file for validation
func storeToTempFile(data []byte, tempMediaPath string) (string, error) {
	// Create temp directory if needed
	if err := os.MkdirAll(tempMediaPath, 0755); err != nil {
		return "", fmt.Errorf("create temp media directory: %w", err)
	}

	// Create temporary file with unique name
	timestamp := time.Now().UnixNano()
	filename := fmt.Sprintf("upload_%d.tmp", timestamp)
	filepath := filepath.Join(tempMediaPath, filename)

	// Write to temporary file
	if err := os.WriteFile(filepath, data, 0600); err != nil {
		return "", fmt.Errorf("write temp file: %w", err)
	}

	return filepath, nil
}

// Helper function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
