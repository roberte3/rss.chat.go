package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"rss.chat.go/db"
	"rss.chat.go/feed"
)

// getRecentItems fetches the most recent posts on the network.
func getRecentItems(conn *sql.DB, viewerScreenname string, maxCt int, baseURL string) ([]db.Item, error) {
	return db.GetRecentItems(conn, viewerScreenname, maxCt, baseURL)
}

// getRecentUserItems fetches a user's recent posts.
func getRecentUserItems(conn *sql.DB, screenname, viewerScreenname string, maxCt int, baseURL string) ([]db.Item, error) {
	feedURL := fmt.Sprintf("%s/feed?screenname=%s", baseURL, screenname)
	return db.GetRecentUserItems(conn, viewerScreenname, feedURL, maxCt, baseURL)
}

// getItemByGuid fetches a post by its GUID.
func getItemByGuid(conn *sql.DB, guid, viewerScreenname, baseURL string) (*db.Item, error) {
	// Parse the guid to get the item ID
	// GUID format is like "http://rss.chat/?id=123"
	var itemID int64
	_, err := fmt.Sscanf(guid, "%*[^?]?id=%d", &itemID)
	if err != nil {
		return nil, fmt.Errorf("invalid guid format")
	}

	item, err := db.GetItemByID(conn, viewerScreenname, itemID, baseURL)
	if err != nil {
		return nil, err
	}

	if item.FlDeleted {
		return nil, fmt.Errorf("the post has been deleted")
	}

	return item, nil
}

// getItemAndReplies fetches a post and its direct replies.
func getItemAndReplies(conn *sql.DB, viewerScreenname string, idParent int64, baseURL string) ([]db.Item, error) {
	items, err := db.GetItemAndReplies(conn, viewerScreenname, idParent, baseURL)
	if err != nil {
		return nil, err
	}

	if len(items) == 0 {
		return nil, fmt.Errorf("post not found")
	}

	// Filter out deleted items from the result (but keep the parent)
	var result []db.Item
	for i, item := range items {
		if i == 0 || !item.FlDeleted {
			result = append(result, item)
		}
	}

	return result, nil
}

// getItemInfo fetches an item by GUID or ID.
func getItemInfo(conn *sql.DB, guid, idStr, viewerScreenname, baseURL string) (*db.Item, error) {
	if guid != "" {
		return getItemByGuid(conn, guid, viewerScreenname, baseURL)
	}

	if idStr == "" {
		return nil, fmt.Errorf("guid or id is required")
	}

	var itemID int64
	_, err := fmt.Sscanf(idStr, "%d", &itemID)
	if err != nil {
		return nil, fmt.Errorf("invalid id format")
	}

	item, err := db.GetItemByID(conn, viewerScreenname, itemID, baseURL)
	if err != nil {
		return nil, err
	}

	if item.FlDeleted {
		return nil, fmt.Errorf("the post has been deleted")
	}

	return item, nil
}

// getUserData returns server and user information.
type UserDataResponse struct {
	// Server info
	EveryoneFeedUrl string `json:"everyoneFeedUrl,omitempty"`
	BaseUrl         string `json:"baseUrl,omitempty"`
	SubsUrl         string `json:"subsUrl,omitempty"`
	FlHasWhitelist  bool   `json:"flHasWhitelist,omitempty"`
	// User info (if screenname provided)
	Screenname  string          `json:"screenname,omitempty"`
	FeedUrl     string          `json:"feedUrl,omitempty"`
	ImageUrl    string          `json:"imageUrl,omitempty"`
	Prefs       json.RawMessage `json:"prefs,omitempty"`
	WhenCreated time.Time       `json:"whenCreated,omitempty"`
	WhenUpdated time.Time       `json:"whenUpdated,omitempty"`
}

func getUserData(conn *sql.DB, screenname string, config feed.BuilderConfig) (*UserDataResponse, error) {
	resp := &UserDataResponse{
		EveryoneFeedUrl: fmt.Sprintf("%s/feed", config.BaseURL),
		BaseUrl:         config.BaseURL,
		SubsUrl:         fmt.Sprintf("%s/getsubscriptionlist", config.BaseURL),
		FlHasWhitelist:  false, // TODO: Set from config
	}

	if screenname != "" {
		user, err := db.GetUserInfoByScreenname(conn, screenname)
		if err != nil {
			return nil, err
		}

		if user == nil {
			return nil, fmt.Errorf("user not found")
		}

		resp.Screenname = user.Screenname
		resp.FeedUrl = fmt.Sprintf("%s/feed?screenname=%s", config.BaseURL, screenname)
		resp.ImageUrl = user.ImageURL
		resp.Prefs = user.Prefs
		resp.WhenCreated = user.WhenCreated
		resp.WhenUpdated = user.WhenUpdated
	}

	return resp, nil
}

// getLikersList returns screennames of everyone who liked a post.
func getLikersList(conn *sql.DB, itemID int64) ([]string, error) {
	return db.GetLikersList(conn, itemID)
}

// getMostActiveToday returns the most active users today.
func getMostActiveToday(conn *sql.DB) ([]db.ActiveUser, error) {
	return db.GetMostActiveToday(conn)
}

// isUserInDatabase checks if a user exists.
func isUserInDatabase(conn *sql.DB, screenname string) (bool, error) {
	user, err := db.GetUserInfoByScreenname(conn, screenname)
	if err != nil {
		return false, err
	}
	return user != nil, nil
}

// isEmailInDatabase checks if an email exists.
func isEmailInDatabase(conn *sql.DB, email string) (bool, error) {
	user, err := db.GetUserInfoByEmail(conn, email)
	if err != nil {
		return false, err
	}
	return user != nil, nil
}
