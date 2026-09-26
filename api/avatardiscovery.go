package api

import (
	"fmt"
	"net/http"

	"github.com/roberte3/rss.chat.go/db"
)

// AvatarInfo holds information about a user's avatar
type AvatarInfo struct {
	Screenname string `json:"screenname"`
	AvatarURL  string `json:"avatarUrl"`
}

// GetUserAvatar returns a specific user's avatar URL.
// Query params: screenname (required)
func (h *Handler) GetUserAvatar(w http.ResponseWriter, r *http.Request) {
	screenname := r.URL.Query().Get("screenname")
	if screenname == "" {
		RespondError(w, "Can't get avatar because screenname is required")
		return
	}

	user, err := db.GetUserInfoByScreenname(h.DB, screenname)
	if err != nil {
		RespondError(w, "Can't get avatar because "+err.Error())
		return
	}

	if user == nil {
		RespondError(w, fmt.Sprintf("Can't get avatar because user %q not found", screenname))
		return
	}

	if user.ImageURL == "" {
		RespondError(w, fmt.Sprintf("User %q does not have an avatar", screenname))
		return
	}

	response := AvatarInfo{
		Screenname: user.Screenname,
		AvatarURL:  user.ImageURL,
	}

	RespondJSON(w, response)
}

// GetUsersWithAvatars returns users who have avatars.
// Query params: ct (optional, continuation token), maxct (optional, default 50)
func (h *Handler) GetUsersWithAvatars(w http.ResponseWriter, r *http.Request) {
	ct := r.URL.Query().Get("ct")
	maxCt := parseIntParam(r, "maxct", 50)

	users, err := db.GetUsersWithAvatars(h.DB, ct, maxCt)
	if err != nil {
		RespondError(w, "Can't get users with avatars because "+err.Error())
		return
	}

	// Convert to avatar info format
	avatars := make([]AvatarInfo, 0, len(users))
	for _, user := range users {
		if user.ImageURL != "" {
			avatars = append(avatars, AvatarInfo{
				Screenname: user.Screenname,
				AvatarURL:  user.ImageURL,
			})
		}
	}

	RespondJSON(w, avatars)
}
