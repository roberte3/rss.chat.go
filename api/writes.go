package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/roberte3/rss.chat.go/db"
	"github.com/roberte3/rss.chat.go/websocket"
)

// PostRequest holds fields for creating or updating a post.
type PostRequest struct {
	Title        string `json:"title,omitempty"`
	Description  string `json:"description,omitempty"`
	MarkdownText string `json:"markdowntext,omitempty"`
	InReplyTo    *int64 `json:"inReplyTo,omitempty"`
	ID           *int64 `json:"id,omitempty"` // For updates
}

// HandleNewPost creates a new post.
func (h *Handler) HandleNewPost(w http.ResponseWriter, r *http.Request, user *db.User) {
	LogOperationStart(r.Context(), "create_post", map[string]interface{}{"screenname": user.Screenname})

	if err := r.ParseForm(); err != nil {
		LogValidationError(r, "form", err.Error())
		RespondValidationError(w, r, "form", err.Error())
		return
	}

	jsonText := r.FormValue("jsontext")
	if jsonText == "" {
		LogValidationError(r, "jsontext", "required parameter missing")
		RespondValidationError(w, r, "jsontext", "required parameter missing")
		return
	}

	var req PostRequest
	if err := json.Unmarshal([]byte(jsonText), &req); err != nil {
		LogValidationError(r, "jsontext", "invalid JSON: "+err.Error())
		RespondValidationError(w, r, "jsontext", "invalid JSON")
		return
	}

	if req.Description == "" {
		LogValidationError(r, "description", "required field missing")
		RespondValidationError(w, r, "description", "required field missing")
		return
	}

	// Linkify bare URLs in description
	linkified, err := LinkifyURLs(req.Description)
	if err != nil {
		RespondErrorWithIDAndCode(w, r, "Failed to process URL links", "URL_PROCESSING_ERROR", err.Error())
		return
	}

	// Sanitize HTML to prevent XSS attacks
	sanitized := SanitizePostHTML(linkified)

	// Remove trailing empty paragraphs if configured
	if h.Config != nil && h.Config.RemoveBlanksAtEnd {
		sanitized = removeTrailingEmptyParagraphs(sanitized)
	}

	// Generate markdown from HTML if not provided
	markdownText := req.MarkdownText
	if markdownText == "" {
		markdownText, err = HtmlToMarkdown(sanitized)
		if err != nil {
			// If markdown conversion fails, just use empty (not critical)
			markdownText = ""
		}
	}

	now := time.Now()
	feedURL := fmt.Sprintf("%s/feed?screenname=%s", h.FeedConfig.BaseURL, user.Screenname)

	newItem := db.NewItem{
		FeedURL:      feedURL,
		Title:        req.Title,
		Link:         "",
		Description:  sanitized,
		InReplyTo:    req.InReplyTo,
		PubDate:      now,
		MarkdownText: markdownText,
		Author:       user.Screenname,
	}

	itemID, err := db.AddItem(h.DB, newItem)
	if err != nil {
		LogOperationError(r.Context(), "add_item", err, map[string]interface{}{})
		RespondErrorWithIDAndCode(w, r, "Failed to create post", "POST_CREATION_ERROR", err.Error())
		return
	}

	// Extract and store mentions from the post
	if mentions, err := ExtractMentions(h.DB, sanitized); err == nil && len(mentions) > 0 {
		if err := db.StoreMentions(h.DB, int(itemID), mentions); err != nil {
			// Log but don't fail the request - mentions are secondary feature
			LogWarning(r.Context(), "failed to store mentions", "err", err)
		} else {
			LogFeatureUsage(r.Context(), "mentions_extracted", len(mentions))
		}
	}

	// Extract and store hashtags from the post
	if hashtags := ExtractHashtags(sanitized); len(hashtags) > 0 {
		if err := db.StoreHashtags(h.DB, int(itemID), hashtags); err != nil {
			// Log but don't fail the request - hashtags are secondary feature
			LogWarning(r.Context(), "failed to store hashtags", "err", err)
		} else {
			LogFeatureUsage(r.Context(), "hashtags_extracted", len(hashtags))
		}
	}

	// Fetch the created item
	item, err := db.GetItemByID(h.DB, user.Screenname, itemID, h.FeedConfig.BaseURL)
	if err != nil {
		LogOperationError(r.Context(), "get_item_by_id", err, map[string]interface{}{})
		RespondError(w, "Can't create post because "+err.Error())
		return
	}

	// Republish feeds
	if err := h.Publisher.UpdateFeedsOnPostWrite(h.DB, user.Screenname); err != nil {
		// Log but don't fail the request
		LogWarning(r.Context(), "failed to republish feeds", "err", err)
	}

	// If this is a reply, also update parent feeds
	if req.InReplyTo != nil {
		parent, err := db.GetItemByID(h.DB, "", *req.InReplyTo, h.FeedConfig.BaseURL)
		if err == nil && parent != nil {
			if err := h.Publisher.UpdateFeedsOnReply(h.DB, parent.Screenname, *req.InReplyTo); err != nil {
				LogWarning(r.Context(), "failed to republish reply feeds", "err", err)
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
				"title":       item.Title,
				"description": item.Description,
				"inReplyTo":   req.InReplyTo,
			},
		})
	}

	LogOperationComplete(r.Context(), "create_post", map[string]interface{}{"itemID": itemID})
	RespondJSON(w, item)
}

// HandleUpdatePost updates an existing post.
func (h *Handler) HandleUpdatePost(w http.ResponseWriter, r *http.Request, user *db.User) {
	LogOperationStart(r.Context(), "update_post", map[string]interface{}{"screenname": user.Screenname})

	if err := r.ParseForm(); err != nil {
		LogValidationError(r, "form", err.Error())
		RespondError(w, "Can't update post because "+err.Error())
		return
	}

	idStr := r.FormValue("id")
	if idStr == "" {
		LogValidationError(r, "id", "required parameter missing")
		RespondValidationError(w, r, "id", "required parameter missing")
		return
	}

	var itemID int64
	if _, err := fmt.Sscanf(idStr, "%d", &itemID); err != nil {
		LogValidationError(r, "id", "invalid format")
		RespondValidationError(w, r, "id", "invalid format")
		return
	}

	// Verify user owns the post
	existing, err := db.GetItemByID(h.DB, "", itemID, h.FeedConfig.BaseURL)
	if err != nil || existing == nil || existing.Screenname != user.Screenname {
		LogAuthFailure(r, "user does not own post")
		RespondAuthError(w, r, "You don't own this post")
		return
	}

	jsonText := r.FormValue("jsontext")
	if jsonText == "" {
		LogValidationError(r, "jsontext", "required parameter missing")
		RespondValidationError(w, r, "jsontext", "required parameter missing")
		return
	}

	var req PostRequest
	if err := json.Unmarshal([]byte(jsonText), &req); err != nil {
		LogValidationError(r, "jsontext", "invalid JSON: "+err.Error())
		RespondValidationError(w, r, "jsontext", "invalid JSON")
		return
	}

	// Linkify bare URLs in description if provided
	description := req.Description
	markdownText := req.MarkdownText

	if description != "" {
		linkified, err := LinkifyURLs(description)
		if err != nil {
			RespondErrorWithIDAndCode(w, r, "Failed to process URL links", "URL_PROCESSING_ERROR", err.Error())
			return
		}

		// Sanitize HTML to prevent XSS attacks
		description = SanitizePostHTML(linkified)

		// Generate markdown if not provided
		if markdownText == "" {
			markdownText, err = HtmlToMarkdown(description)
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
		LogOperationError(r.Context(), "update_item", err, map[string]interface{}{"itemID": itemID})
		RespondErrorWithIDAndCode(w, r, "Failed to update post", "POST_UPDATE_ERROR", err.Error())
		return
	}

	// Extract and store mentions from the updated post (if description was updated)
	if description != "" {
		if mentions, err := ExtractMentions(h.DB, description); err == nil && len(mentions) > 0 {
			if err := db.StoreMentions(h.DB, int(itemID), mentions); err != nil {
				// Log but don't fail the request
				LogWarning(r.Context(), "failed to update mentions", "err", err)
			} else {
				LogFeatureUsage(r.Context(), "mentions_updated", len(mentions))
			}
		}
	}

	// Extract and store hashtags from the updated post (if description was updated)
	if description != "" {
		if hashtags := ExtractHashtags(description); len(hashtags) > 0 {
			if err := db.StoreHashtags(h.DB, int(itemID), hashtags); err != nil {
				// Log but don't fail the request
				LogWarning(r.Context(), "failed to update hashtags", "err", err)
			} else {
				LogFeatureUsage(r.Context(), "hashtags_updated", len(hashtags))
			}
		}
	}

	// Fetch the updated item
	updatedItem, err := db.GetItemByID(h.DB, user.Screenname, itemID, h.FeedConfig.BaseURL)
	if err != nil {
		LogOperationError(r.Context(), "get_item_by_id", err, map[string]interface{}{})
		RespondError(w, "Can't update post because "+err.Error())
		return
	}

	// Republish feeds
	if err := h.Publisher.UpdateFeedsOnPostWrite(h.DB, user.Screenname); err != nil {
		LogWarning(r.Context(), "failed to republish feeds", "err", err)
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

	LogOperationComplete(r.Context(), "update_post", map[string]interface{}{"itemID": itemID})
	RespondJSON(w, updatedItem)
}

// HandleDeletePost deletes a post (soft delete).
func (h *Handler) HandleDeletePost(w http.ResponseWriter, r *http.Request, user *db.User) {
	LogOperationStart(r.Context(), "delete_post", map[string]interface{}{"screenname": user.Screenname})

	if err := r.ParseForm(); err != nil {
		LogValidationError(r, "form", err.Error())
		RespondError(w, "Can't delete post because "+err.Error())
		return
	}

	idStr := r.FormValue("id")
	if idStr == "" {
		LogValidationError(r, "id", "required parameter missing")
		RespondValidationError(w, r, "id", "required parameter missing")
		return
	}

	var itemID int64
	if _, err := fmt.Sscanf(idStr, "%d", &itemID); err != nil {
		LogValidationError(r, "id", "invalid format")
		RespondValidationError(w, r, "id", "invalid format")
		return
	}

	// Verify user owns the post
	existing, err := db.GetItemByID(h.DB, "", itemID, h.FeedConfig.BaseURL)
	if err != nil || existing == nil || existing.Screenname != user.Screenname {
		LogAuthFailure(r, "user does not own post")
		RespondAuthError(w, r, "You don't own this post")
		return
	}

	deleted := true
	if err := db.UpdateItem(h.DB, db.ItemPatch{ID: itemID, FlDeleted: &deleted}); err != nil {
		LogOperationError(r.Context(), "update_item_delete", err, map[string]interface{}{"itemID": itemID})
		RespondErrorWithIDAndCode(w, r, "Failed to delete post", "POST_DELETE_ERROR", err.Error())
		return
	}

	// Republish feeds
	if err := h.Publisher.UpdateFeedsOnPostWrite(h.DB, user.Screenname); err != nil {
		LogWarning(r.Context(), "failed to republish feeds after deleting", "itemID", itemID, "err", err)
	}

	// If the deleted post was a reply, the parent's comments feed still lists
	// it, so republish that too. Ports the updateReplyFeedsOnS3 call deletePost
	// makes in rssnetwork.js.
	if existing.InReplyToNum != nil {
		parent, err := db.GetItemByID(h.DB, "", *existing.InReplyToNum, h.FeedConfig.BaseURL)
		if err == nil && parent != nil {
			if err := h.Publisher.UpdateFeedsOnReply(h.DB, parent.Screenname, *existing.InReplyToNum); err != nil {
				LogWarning(r.Context(), "failed to republish reply feeds after deleting", "itemID", itemID, "err", err)
			}
		}
	}

	// Return the deleted item, matching deletePost in rssnetwork.js.
	existing.FlDeleted = true
	LogOperationComplete(r.Context(), "delete_post", map[string]interface{}{"itemID": itemID})
	RespondJSON(w, existing)
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
		LogWarning(r.Context(), "failed to republish feeds on like", "err", err)
	}

	// Broadcast like toggle event to websocket subscribers
	if h.WebsocketHub != nil {
		h.WebsocketHub.Broadcast(&websocket.Event{
			Type:   websocket.TypeToggledLike,
			ItemID: itemID,
			Author: user.Screenname,
			Data: map[string]interface{}{
				"liked":   !isLiked,
				"ctLikes": item.CtLikes,
			},
		})
	}

	RespondJSON(w, item)
}

// HandleSavePrefs saves user preferences.
func (h *Handler) HandleSavePrefs(w http.ResponseWriter, r *http.Request, user *db.User) {
	if err := r.ParseForm(); err != nil {
		RespondValidationError(w, r, "form", err.Error())
		return
	}

	jsonText := r.FormValue("jsontext")
	if jsonText == "" {
		RespondValidationError(w, r, "jsontext", "required parameter missing")
		return
	}

	// Update user prefs
	if err := db.UpdateUserPrefs(h.DB, user.Screenname, []byte(jsonText)); err != nil {
		RespondErrorWithIDAndCode(w, r, "Failed to save preferences", "PREFS_UPDATE_ERROR", err.Error())
		return
	}

	// Re-fetch user to return updated data
	updated, err := db.GetUserInfoByScreenname(h.DB, user.Screenname)
	if err != nil {
		RespondErrorWithIDAndCode(w, r, "Failed to fetch updated user data", "DB_ERROR", err.Error())
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

// removeTrailingEmptyParagraphs strips trailing empty <p></p> tags from HTML.
// This helps clean up post content when posts end with empty paragraphs.
func removeTrailingEmptyParagraphs(html string) string {
	if html == "" {
		return html
	}

	// Repeatedly remove trailing empty paragraphs until none remain
	for {
		trimmed := strings.TrimSpace(html)
		// Check if it ends with </p>
		if !strings.HasSuffix(trimmed, "</p>") {
			return trimmed
		}

		// Find the last <p> tag
		lastOpenTag := strings.LastIndex(trimmed, "<p")
		if lastOpenTag == -1 {
			return trimmed
		}

		// Extract the paragraph
		para := trimmed[lastOpenTag:]
		// Remove any attributes from the opening tag
		endOfTag := strings.Index(para, ">")
		if endOfTag == -1 {
			return trimmed
		}

		// Get the content between tags
		content := para[endOfTag+1 : len(para)-4] // -4 for "</p>"
		content = strings.TrimSpace(content)

		// If paragraph is empty, remove it and continue
		if content == "" {
			html = strings.TrimSpace(trimmed[:lastOpenTag])
			continue
		}

		// Otherwise we're done
		return trimmed
	}
}
