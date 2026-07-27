package api

import (
	"database/sql"
	"fmt"
	"net/http"
	"rss.chat.go/config"
	"rss.chat.go/db"
	"rss.chat.go/feed"
	"rss.chat.go/publish"
	"rss.chat.go/websocket"
)

// Handler holds dependencies for HTTP request handling.
type Handler struct {
	DB                  *sql.DB
	Publisher           *publish.Publisher
	FeedConfig          feed.BuilderConfig
	Config              *config.Config // Application configuration
	WebsocketHub        *websocket.Hub
	MediaDB             *sql.DB
	FeedsDB             *sql.DB // Nil if feeds are served from filesystem
	MaxMediaUploadBytes int
	TempMediaPath       string
	RobotsContent       string
	blocklistMtime      int64 // Last modification time of blocklist file
	EmailSender         interface {
		SendConfirmationEmail(string, string, string) error
	} // Email sender interface
}

// NewHandler creates a new API handler.
func NewHandler(db *sql.DB, pub *publish.Publisher, cfg feed.BuilderConfig) *Handler {
	return &Handler{
		DB:         db,
		Publisher:  pub,
		FeedConfig: cfg,
	}
}

// SetWebsocketHub sets the websocket hub for broadcasting updates.
func (h *Handler) SetWebsocketHub(hub *websocket.Hub) {
	h.WebsocketHub = hub
}

// SetEmailSender sets the email sender for sending confirmation emails.
func (h *Handler) SetEmailSender(sender interface {
	SendConfirmationEmail(string, string, string) error
}) {
	h.EmailSender = sender
}

// UploadMediaAuth wraps HandleUploadMedia with authentication
func (h *Handler) UploadMediaAuth(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("emailaddress")
	code := r.FormValue("emailcode")

	user, err := AuthenticateUser(h.DB, email, code)
	if err != nil {
		RespondError(w, "Can't upload media because "+err.Error())
		return
	}

	h.HandleUploadMedia(w, r, user)
}

// RegisterRoutes registers all API endpoints with the mux at both root and /api/ paths.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// Helper to register each route at both /route and /api/route
	register := func(method, path string, handler http.HandlerFunc) {
		mux.HandleFunc(method+" "+path, handler)
		mux.HandleFunc(method+" /api"+path, handler)
	}

	// Read endpoints (no auth)
	register("GET", "/health", h.Health)
	register("GET", "/feed", h.Feed)
	register("GET", "/getrecentitems", h.GetRecentItems)
	register("GET", "/getrecentuseritems", h.GetRecentUserItems)
	register("GET", "/getitembyguid", h.GetItemByGuid)
	register("GET", "/getitemandreplies", h.GetItemAndReplies)
	register("GET", "/getiteminfo", h.GetItemInfo)
	register("GET", "/getuserdata", h.GetUserData)
	register("GET", "/getlikerslist", h.GetLikersList)
	register("GET", "/getmostactivetoday", h.GetMostActiveToday)
	register("GET", "/getsubscriptionlist", h.GetSubscriptionList)
	register("GET", "/isuserindatabase", h.IsUserInDatabase)
	register("GET", "/isemailindatabase", h.IsEmailInDatabase)
	register("GET", "/checkwhitelist", h.CheckWhitelist)
	register("GET", "/robots.txt", h.RobotsTxt)

	// Auth endpoints (no auth required, but rate-limited in production)
	register("GET", "/sendconfirmingemail", h.SendConfirmingEmail)
	register("GET", "/createnewuser", h.CreateNewUser)

	// Write endpoints (authenticated)
	register("POST", "/newpost", h.NewPost)
	register("POST", "/updatepost", h.UpdatePost)
	register("POST", "/deletepost", h.DeletePost)
	register("POST", "/togglelike", h.ToggleLike)
	register("POST", "/saveprefs", h.SavePrefs)
	register("POST", "/uploadmedia", h.UploadMediaAuth)

	// Media serving (public)
	register("GET", "/media/{id}", h.HandleGetMedia)

	// Websocket
	mux.HandleFunc("GET /ws", h.WebSocket)
}

// Health is a simple health check endpoint.
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	RespondText(w, "OK")
}

// Feed serves an RSS feed: either user's feed or everyone's feed.
// Query params: screenname (optional), format (optional, default "xml", can be "json")
func (h *Handler) Feed(w http.ResponseWriter, r *http.Request) {
	screenname := r.URL.Query().Get("screenname")
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "xml"
	}

	// Validate format
	if format != "xml" && format != "json" {
		RespondError(w, fmt.Sprintf("Invalid format: %s (must be 'xml' or 'json')", format))
		return
	}

	// Handle JSON format - generate on demand
	if format == "json" {
		var jsonContent string
		var err error
		if screenname == "" {
			jsonContent, err = feed.BuildFeedForEveryoneJSON(h.DB, h.FeedConfig.BaseURL, h.FeedConfig)
		} else {
			jsonContent, err = feed.BuildFeedForUserJSON(h.DB, screenname, h.FeedConfig.BaseURL, h.FeedConfig)
		}

		if err != nil {
			RespondError(w, fmt.Sprintf("Can't build feed because %s", err.Error()))
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Write([]byte(jsonContent))
		return
	}

	// Handle XML format (default)
	// If feeds are stored in database, serve from there
	if h.FeedsDB != nil {
		var feedType string
		if screenname == "" {
			feedType = "global"
		} else {
			feedType = "user"
		}

		content, err := db.GetFeed(h.FeedsDB, feedType, screenname)
		if err == nil && content != nil {
			w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
			w.Write(content)
			return
		}
	}

	// Fallback to filesystem mode
	if screenname == "" {
		// Serve everyone feed
		h.Publisher.ServeEveryoneFeed(w, r)
	} else {
		// Serve user feed
		h.Publisher.ServeUserFeed(w, r, screenname)
	}
}

// GetRecentItems returns the most recent posts on the network.
// Query params: ct (optional, default 100)
func (h *Handler) GetRecentItems(w http.ResponseWriter, r *http.Request) {
	ct := parseIntParam(r, "ct", 100)
	screenname := r.URL.Query().Get("screenname")

	items, err := getRecentItems(h.DB, screenname, ct, h.FeedConfig.BaseURL)
	if err != nil {
		RespondError(w, "Can't get recent items because "+err.Error())
		return
	}

	RespondJSON(w, items)
}

// GetRecentUserItems returns a user's recent posts.
// Query params: name (required), ct (optional)
func (h *Handler) GetRecentUserItems(w http.ResponseWriter, r *http.Request) {
	screenname := r.URL.Query().Get("name")
	if screenname == "" {
		RespondError(w, "Can't get recent user items because screenname is required")
		return
	}

	ct := parseIntParam(r, "ct", 100)
	viewerScreenname := r.URL.Query().Get("screenname")

	items, err := getRecentUserItems(h.DB, screenname, viewerScreenname, ct, h.FeedConfig.BaseURL)
	if err != nil {
		RespondError(w, "Can't get recent user items because "+err.Error())
		return
	}

	RespondJSON(w, items)
}

// GetItemByGuid returns a post by its guid.
// Query params: guid (required)
func (h *Handler) GetItemByGuid(w http.ResponseWriter, r *http.Request) {
	guid := r.URL.Query().Get("guid")
	if guid == "" {
		RespondError(w, "Can't get item by guid because guid is required")
		return
	}

	viewerScreenname := r.URL.Query().Get("screenname")

	item, err := getItemByGuid(h.DB, guid, viewerScreenname, h.FeedConfig.BaseURL)
	if err != nil {
		RespondError(w, "Can't get item by guid because "+err.Error())
		return
	}

	if item == nil {
		RespondError(w, "Can't get item by guid because item not found")
		return
	}

	RespondJSON(w, item)
}

// GetItemAndReplies returns a post and its direct replies.
// Query params: idparent (required)
func (h *Handler) GetItemAndReplies(w http.ResponseWriter, r *http.Request) {
	idParent := int64(parseIntParam(r, "idparent", 0))
	if idParent == 0 {
		RespondError(w, "Can't get item and replies because idparent is required")
		return
	}

	viewerScreenname := r.URL.Query().Get("screenname")

	items, err := getItemAndReplies(h.DB, viewerScreenname, idParent, h.FeedConfig.BaseURL)
	if err != nil {
		RespondError(w, "Can't get item and replies because "+err.Error())
		return
	}

	RespondJSON(w, items)
}

// GetItemInfo returns an item in RSS or feedland format.
// Query params: guid or id (required), format (optional, default "rss")
func (h *Handler) GetItemInfo(w http.ResponseWriter, r *http.Request) {
	guid := r.URL.Query().Get("guid")
	idStr := r.URL.Query().Get("id")
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "rss"
	}

	viewerScreenname := r.URL.Query().Get("screenname")

	item, err := getItemInfo(h.DB, guid, idStr, viewerScreenname, h.FeedConfig.BaseURL)
	if err != nil {
		RespondError(w, "Can't get item info because "+err.Error())
		return
	}

	if item == nil {
		RespondError(w, "Can't get item info because item not found")
		return
	}

	if format == "feedland" {
		RespondJSON(w, item)
	} else if format == "rss" {
		RespondJSONString(w, item.Description) // Should return RSS-formatted item
	} else {
		RespondError(w, "Can't get item info because invalid format "+format)
	}
}

// GetUserData returns server and user information.
// Query params: screenname (optional)
func (h *Handler) GetUserData(w http.ResponseWriter, r *http.Request) {
	screenname := r.URL.Query().Get("screenname")

	data, err := getUserData(h.DB, screenname, h.FeedConfig)
	if err != nil {
		RespondError(w, "Can't get user data because "+err.Error())
		return
	}

	RespondJSON(w, data)
}

// GetLikersList returns screennames of everyone who liked a post.
// Query params: id (required)
func (h *Handler) GetLikersList(w http.ResponseWriter, r *http.Request) {
	itemID := int64(parseIntParam(r, "id", 0))
	if itemID == 0 {
		RespondError(w, "Can't get likers list because id is required")
		return
	}

	likers, err := getLikersList(h.DB, itemID)
	if err != nil {
		RespondError(w, "Can't get likers list because "+err.Error())
		return
	}

	RespondJSON(w, likers)
}

// GetMostActiveToday returns the most active users today.
// No query params.
func (h *Handler) GetMostActiveToday(w http.ResponseWriter, r *http.Request) {
	users, err := getMostActiveToday(h.DB)
	if err != nil {
		RespondError(w, "Can't get most active today because "+err.Error())
		return
	}

	RespondJSON(w, users)
}

// GetSubscriptionList returns the OPML subscription list.
// No query params.
//
// The list is built from the database on every request, matching
// getSubscriptionList in rssnetwork.js. It is cheap — one query for the
// screennames — and it means the endpoint can never 404 or go stale because a
// published artifact is missing. The on-disk/in-database copy that
// PublishSubscriptionList maintains is the mirror for external consumers,
// analogous to the original's S3 copy, and is deliberately not what is served
// here.
func (h *Handler) GetSubscriptionList(w http.ResponseWriter, r *http.Request) {
	opml, err := feed.BuildSubscriptionList(h.DB, h.FeedConfig.BaseURL, h.FeedConfig)
	if err != nil {
		RespondError(w, "Can't get the subscription list because "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Write([]byte(opml))
}

// IsUserInDatabase checks if a user exists.
// Query params: screenname (required)
func (h *Handler) IsUserInDatabase(w http.ResponseWriter, r *http.Request) {
	screenname := r.URL.Query().Get("screenname")
	if screenname == "" {
		RespondError(w, "Can't check user because screenname is required")
		return
	}

	exists, err := isUserInDatabase(h.DB, screenname)
	if err != nil {
		RespondError(w, "Can't check user because "+err.Error())
		return
	}

	RespondJSON(w, map[string]bool{"flInDatabase": exists})
}

// IsEmailInDatabase checks if an email exists.
// Query params: email (required)
func (h *Handler) IsEmailInDatabase(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")
	if email == "" {
		RespondError(w, "Can't check email because email is required")
		return
	}

	exists, err := isEmailInDatabase(h.DB, email)
	if err != nil {
		RespondError(w, "Can't check email because "+err.Error())
		return
	}

	RespondJSON(w, map[string]bool{"flInDatabase": exists})
}

// CheckWhitelist checks if an email is whitelisted.
// Query params: emailaddress (required)
func (h *Handler) CheckWhitelist(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("emailaddress")
	if email == "" {
		RespondError(w, "Can't check whitelist because email is required")
		return
	}

	// TODO: Check against actual whitelist from config
	// For now, always allow (no whitelist)
	RespondJSON(w, map[string]bool{"flWhitelisted": true})
}

// RobotsTxt returns the robots.txt file from config.
func (h *Handler) RobotsTxt(w http.ResponseWriter, r *http.Request) {
	RespondText(w, h.RobotsContent)
}

// NewPost creates a new post.
// Auth required. POST params: jsontext, emailaddress, emailcode
func (h *Handler) NewPost(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("emailaddress")
	code := r.FormValue("emailcode")

	user, err := AuthenticateUser(h.DB, email, code)
	if err != nil {
		RespondError(w, "Can't create post because "+err.Error())
		return
	}

	h.HandleNewPost(w, r, user)
}

// UpdatePost updates an existing post.
// Auth required. POST params: jsontext, id, emailaddress, emailcode
func (h *Handler) UpdatePost(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("emailaddress")
	code := r.FormValue("emailcode")

	user, err := AuthenticateUser(h.DB, email, code)
	if err != nil {
		RespondError(w, "Can't update post because "+err.Error())
		return
	}

	h.HandleUpdatePost(w, r, user)
}

// DeletePost deletes a post.
// Auth required. POST params: id, emailaddress, emailcode
func (h *Handler) DeletePost(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("emailaddress")
	code := r.FormValue("emailcode")

	user, err := AuthenticateUser(h.DB, email, code)
	if err != nil {
		RespondError(w, "Can't delete post because "+err.Error())
		return
	}

	h.HandleDeletePost(w, r, user)
}

// ToggleLike toggles a like on a post.
// Auth required. POST params: id, emailaddress, emailcode
func (h *Handler) ToggleLike(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("emailaddress")
	code := r.FormValue("emailcode")

	user, err := AuthenticateUser(h.DB, email, code)
	if err != nil {
		RespondError(w, "Can't toggle like because "+err.Error())
		return
	}

	h.HandleToggleLike(w, r, user)
}

// SavePrefs saves user preferences.
// Auth required. POST params: jsontext, emailaddress, emailcode
func (h *Handler) SavePrefs(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("emailaddress")
	code := r.FormValue("emailcode")

	user, err := AuthenticateUser(h.DB, email, code)
	if err != nil {
		RespondError(w, "Can't save prefs because "+err.Error())
		return
	}

	h.HandleSavePrefs(w, r, user)
}

// WebSocket handles websocket connections.
func (h *Handler) WebSocket(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement websocket handler
}

// Helper to parse integer query parameters
func parseIntParam(r *http.Request, name string, defaultValue int) int {
	str := r.URL.Query().Get(name)
	if str == "" {
		return defaultValue
	}
	var val int
	if _, err := fmt.Sscanf(str, "%d", &val); err != nil {
		return defaultValue
	}
	return val
}
