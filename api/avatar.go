package api

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"os"

	"github.com/roberte3/rss.chat.go/db"
)

// AvatarUploadRequest holds avatar data from the client
type AvatarUploadRequest struct {
	Data        string `json:"data"`        // Base64-encoded image data
	ContentType string `json:"contentType"` // MIME type (image/jpeg, image/png, etc)
}

// AvatarResponse is the response after successful avatar upload
type AvatarResponse struct {
	AvatarURL string `json:"avatarUrl"`
	Size      int64  `json:"size"`
}

// HandleUploadAvatar handles avatar uploads from authenticated users.
// Stores the image and updates the user's imageUrl field.
func (h *Handler) HandleUploadAvatar(w http.ResponseWriter, r *http.Request, user *db.User) {
	// Parse form data
	if err := r.ParseForm(); err != nil {
		RespondError(w, "Can't upload avatar because "+err.Error())
		return
	}

	// Get image data (base64-encoded)
	dataStr := r.FormValue("data")
	if dataStr == "" {
		RespondError(w, "Can't upload avatar because data is required")
		return
	}

	// Get content type
	contentType := r.FormValue("contentType")
	if contentType == "" {
		RespondError(w, "Can't upload avatar because contentType is required")
		return
	}

	// Validate content type is an image
	if !isImageContentType(contentType) {
		RespondError(w, "Can't upload avatar because contentType must be an image type")
		return
	}

	// Decode base64 data
	data, err := base64.StdEncoding.DecodeString(dataStr)
	if err != nil {
		RespondError(w, "Can't upload avatar because data is not valid base64")
		return
	}

	// Validate media (size, magic bytes, content type)
	if err := validateMedia(data, contentType, h.MaxMediaUploadBytes); err != nil {
		RespondError(w, "Can't upload avatar because "+err.Error())
		return
	}

	// Store to temporary file for safe handling
	tempFile, err := storeToTempFile(data, h.TempMediaPath)
	if err != nil {
		RespondError(w, "Can't upload avatar because "+err.Error())
		return
	}
	defer os.Remove(tempFile)

	// Store in media database
	mediaID, err := db.StoreMedia(h.MediaDB, user.Screenname, contentType, data)
	if err != nil {
		RespondError(w, "Can't upload avatar because "+err.Error())
		return
	}

	// Build URL to the stored media
	avatarURL := fmt.Sprintf("%s/media/%d", h.FeedConfig.BaseURL, mediaID)

	// Update user's imageUrl field
	if err := db.UpdateUserImageURL(h.DB, user.Screenname, avatarURL); err != nil {
		RespondError(w, "Can't upload avatar because "+err.Error())
		return
	}

	// Return success response
	response := AvatarResponse{
		AvatarURL: avatarURL,
		Size:      int64(len(data)),
	}

	RespondJSON(w, response)
}

// isImageContentType validates that the content type is a supported image format
func isImageContentType(contentType string) bool {
	switch contentType {
	case "image/jpeg", "image/png", "image/gif", "image/webp", "image/svg+xml":
		return true
	default:
		return false
	}
}
