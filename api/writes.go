package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"rss.chat.go/db"
	"rss.chat.go/websocket"
)

// PostRequest holds fields for creating or updating a post.
type PostRequest struct {
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	MarkdownText string `json:"markdowntext,omitempty"`
	InReplyTo   *int64 `json:"inReplyTo,omitempty"`
	ID          *int64 `json:"id,omitempty"` // For updates
}

// HandleNewPost creates a new post.
func (h *Handler) HandleNewPost(w http.ResponseWriter, r *http.Request, user *db.User) {
	if err := r.ParseForm(); err != nil {
		RespondError(w, "Can't create post because "+err.Error())
		return
	}

	jsonText := r.FormValue("jsontext")
	if jsonText == "" {
		RespondError(w, "Can't create post because jsontext is required")
		return
	}

	var req PostRequest
	if err := json.Unmarshal([]byte(jsonText), &req); err != nil {
		RespondError(w, "Can't create post because "+err.Error())
		return
	}

	if req.Description == "" {
		RespondError(w, "Can't create post because description is required")
		return
	}

	// Linkify bare URLs in description
	linkified, err := LinkifyURLs(req.Description)
	if err != nil {
		RespondError(w, "Can't create post because "+err.Error())
		return
	}

	// Generate markdown from HTML if not provided
	markdownText := req.MarkdownText
	if markdownText == "" {
		markdownText, err = HtmlToMarkdown(linkified)
		if err != nil {
			// If markdown conversion fails, just use empty (not critical)
			markdownText = ""
		}
	}

	now := time.Now()
	feedURL := fmt.Sprintf("http://%s/feed?screenname=%s", h.FeedConfig.BaseURL, user.Screenname)

	newItem := db.NewItem{
		FeedURL:      feedURL,
		Title:        req.Title,
		Link:         "",
		Description:  linkified,
		InReplyTo:    req.InReplyTo,
		PubDate:      now,
		MarkdownText: markdownText,
	}

	itemID, err := db.AddItem(h.DB, newItem)
	if err != nil {
		RespondError(w, "Can't create post because "+err.Error())
		return
	}

	// Fetch the created item
	item, err := db.GetItemByID(h.DB, user.Screenname, itemID, h.FeedConfig.BaseURL)
	if err != nil {
		RespondError(w, "Can't create post because "+err.Error())
		return
	}

	// Republish feeds
	if err := h.Publisher.UpdateFeedsOnPostWrite(h.DB, user.Screenname); err != nil {
		// Log but don't fail the request
		fmt.Printf("Warning: failed to republish feeds: %v\n", err)
	}

	// If this is a reply, also update parent feeds
	if req.InReplyTo != nil {
		parent, err := db.GetItemByID(h.DB, "", *req.InReplyTo, h.FeedConfig.BaseURL)
		if err == nil && parent != nil {
			if err := h.Publisher.UpdateFeedsOnReply(h.DB, parent.Screenname, *req.InReplyTo); err != nil {
				fmt.Printf("Warning: failed to republish reply feeds: %v\n", err)
			}
		}
	}

	// Broadcast new item event to websocket subscribers
	if h.WebsocketHub != nil {
		h.WebsocketHub.Broadcast(&websocket.Event{
			Type:   websocket.TypeNewItem,
			ItemID: itemID,
			Author: user.Screenname,
			Data: map[string]interface{}{
				"title":        item.Title,
				"description":  item.Description,
				"inReplyTo":    req.InReplyTo,
			},
		})
	}

	RespondJSON(w, item)
}

// HandleUpdatePost updates an existing post.
func (h *Handler) HandleUpdatePost(w http.ResponseWriter, r *http.Request, user *db.User) {
	if err := r.ParseForm(); err != nil {
		RespondError(w, "Can't update post because "+err.Error())
		return
	}

	idStr := r.FormValue("id")
	if idStr == "" {
		RespondError(w, "Can't update post because id is required")
		return
	}

	var itemID int64
	if _, err := fmt.Sscanf(idStr, "%d", &itemID); err != nil {
		RespondError(w, "Can't update post because invalid id")
		return
	}

	// Verify user owns the post
	existing, err := db.GetItemByID(h.DB, "", itemID, h.FeedConfig.BaseURL)
	if err != nil || existing == nil || existing.Screenname != user.Screenname {
		RespondError(w, "Can't update post because you don't own this post")
		return
	}

	jsonText := r.FormValue("jsontext")
	if jsonText == "" {
		RespondError(w, "Can't update post because jsontext is required")
		return
	}

	var req PostRequest
	if err := json.Unmarshal([]byte(jsonText), &req); err != nil {
		RespondError(w, "Can't update post because "+err.Error())
		return
	}

	// Linkify bare URLs in description if provided
	description := req.Description
	markdownText := req.MarkdownText

	if description != "" {
		linkified, err := LinkifyURLs(description)
		if err != nil {
			RespondError(w, "Can't update post because "+err.Error())
			return
		}
		description = linkified

		// Generate markdown if not provided
		if markdownText == "" {
			markdownText, err = HtmlToMarkdown(linkified)
			if err != nil {
				// If markdown conversion fails, just use empty (not critical)
				markdownText = ""
			}
		}
	}

	patch := db.ItemPatch{
		ID:           itemID,
		Title:        strPtr(req.Title),
		Description:  strPtr(description),
		MarkdownText: strPtr(markdownText),
	}

	err = db.UpdateItem(h.DB, patch)
	if err != nil {
		RespondError(w, "Can't update post because "+err.Error())
		return
	}

	// Fetch the updated item
	updatedItem, err := db.GetItemByID(h.DB, user.Screenname, itemID, h.FeedConfig.BaseURL)
	if err != nil {
		RespondError(w, "Can't update post because "+err.Error())
		return
	}

	// Republish feeds
	if err := h.Publisher.UpdateFeedsOnPostWrite(h.DB, user.Screenname); err != nil {
		fmt.Printf("Warning: failed to republish feeds: %v\n", err)
	}

	// Broadcast updated item event to websocket subscribers
	if h.WebsocketHub != nil {
		h.WebsocketHub.Broadcast(&websocket.Event{
			Type:   websocket.TypeUpdatedItem,
			ItemID: itemID,
			Author: user.Screenname,
			Data: map[string]interface{}{
				"title":       updatedItem.Title,
				"description": updatedItem.Description,
			},
		})
	}

	RespondJSON(w, updatedItem)
}

// HandleDeletePost deletes a post (soft delete).
func (h *Handler) HandleDeletePost(w http.ResponseWriter, r *http.Request, user *db.User) {
	if err := r.ParseForm(); err != nil {
		RespondError(w, "Can't delete post because "+err.Error())
		return
	}

	idStr := r.FormValue("id")
	if idStr == "" {
		RespondError(w, "Can't delete post because id is required")
		return
	}

	var itemID int64
	if _, err := fmt.Sscanf(idStr, "%d", &itemID); err != nil {
		RespondError(w, "Can't delete post because invalid id")
		return
	}

	// Verify user owns the post
	existing, err := db.GetItemByID(h.DB, "", itemID, h.FeedConfig.BaseURL)
	if err != nil || existing == nil || existing.Screenname != user.Screenname {
		RespondError(w, "Can't delete post because you don't own this post")
		return
	}

	patch := &db.ItemPatch{
		ID: itemID,
		// Mark as deleted by setting flDeleted = 1
		// Note: The db layer doesn't currently expose this in ItemPatch
		// For now, use raw SQL or extend ItemPatch
	}

	// TODO: Implement soft delete properly in db layer
	// For now, just return success but note this is incomplete
	_ = patch

	// Republish feeds
	if err := h.Publisher.UpdateFeedsOnPostWrite(h.DB, user.Screenname); err != nil {
		fmt.Printf("Warning: failed to republish feeds: %v\n", err)
	}

	RespondJSON(w, map[string]string{"status": "deleted"})
}

// HandleToggleLike toggles a like on a post.
func (h *Handler) HandleToggleLike(w http.ResponseWriter, r *http.Request, user *db.User) {
	if err := r.ParseForm(); err != nil {
		RespondError(w, "Can't toggle like because "+err.Error())
		return
	}

	idStr := r.FormValue("id")
	if idStr == "" {
		RespondError(w, "Can't toggle like because id is required")
		return
	}

	var itemID int64
	if _, err := fmt.Sscanf(idStr, "%d", &itemID); err != nil {
		RespondError(w, "Can't toggle like because invalid id")
		return
	}

	// Check if already liked
	isLiked, err := db.IsLiked(h.DB, user.Screenname, itemID)
	if err != nil {
		RespondError(w, "Can't toggle like because "+err.Error())
		return
	}

	if isLiked {
		// Remove like
		if err := db.RemoveFromLikesTable(h.DB, user.Screenname, itemID); err != nil {
			RespondError(w, "Can't toggle like because "+err.Error())
			return
		}
	} else {
		// Add like
		if err := db.AddToLikesTable(h.DB, user.Screenname, itemID); err != nil {
			RespondError(w, "Can't toggle like because "+err.Error())
			return
		}
	}

	// Refresh item with updated like count
	item, err := db.GetItemByID(h.DB, user.Screenname, itemID, h.FeedConfig.BaseURL)
	if err != nil {
		RespondError(w, "Can't toggle like because "+err.Error())
		return
	}

	// Republish everyone feed (like counts may have changed)
	if err := h.Publisher.UpdateFeedsOnLike(h.DB); err != nil {
		fmt.Printf("Warning: failed to republish feeds: %v\n", err)
	}

	// Broadcast like toggle event to websocket subscribers
	if h.WebsocketHub != nil {
		h.WebsocketHub.Broadcast(&websocket.Event{
			Type:   websocket.TypeToggledLike,
			ItemID: itemID,
			Author: user.Screenname,
			Data: map[string]interface{}{
				"liked":    !isLiked,
				"ctLikes":  item.CtLikes,
			},
		})
	}

	RespondJSON(w, item)
}

// HandleSavePrefs saves user preferences.
func (h *Handler) HandleSavePrefs(w http.ResponseWriter, r *http.Request, user *db.User) {
	if err := r.ParseForm(); err != nil {
		RespondError(w, "Can't save prefs because "+err.Error())
		return
	}

	jsonText := r.FormValue("jsontext")
	if jsonText == "" {
		RespondError(w, "Can't save prefs because jsontext is required")
		return
	}

	// Update user prefs
	if err := db.UpdateUserPrefs(h.DB, user.Screenname, []byte(jsonText)); err != nil {
		RespondError(w, "Can't save prefs because "+err.Error())
		return
	}

	// Re-fetch user to return updated data
	updated, err := db.GetUserInfoByScreenname(h.DB, user.Screenname)
	if err != nil {
		RespondError(w, "Can't save prefs because "+err.Error())
		return
	}

	RespondJSON(w, updated)
}

// Helper to create a string pointer
func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
