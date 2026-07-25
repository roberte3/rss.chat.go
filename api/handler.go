package api

import (
	"database/sql"
	"fmt"
	"net/http"
	"rss.chat.go/feed"
	"rss.chat.go/publish"
	"rss.chat.go/websocket"
)

// Handler holds dependencies for HTTP request handling.
type Handler struct {
	DB        *sql.DB
	Publisher *publish.Publisher
	FeedConfig feed.BuilderConfig
	WebsocketHub *websocket.Hub
	MediaDB *sql.DB
	MaxMediaUploadBytes int
	TempMediaPath string
}

// NewHandler creates a new API handler.
func NewHandler(db *sql.DB, pub *publish.Publisher, cfg feed.BuilderConfig) *Handler {
	return &Handler{
		DB:        db,
		Publisher: pub,
		FeedConfig: cfg,
	}
}

// SetWebsocketHub sets the websocket hub for broadcasting updates.
func (h *Handler) SetWebsocketHub(hub *websocket.Hub) {
	h.WebsocketHub = hub
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

// RegisterRoutes registers all API endpoints with the mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// Read endpoints (no auth)
	mux.HandleFunc("GET /health", h.Health)
	mux.HandleFunc("GET /feed", h.Feed)
	mux.HandleFunc("GET /getrecentitems", h.GetRecentItems)
	mux.HandleFunc("GET /getrecentuseritems", h.GetRecentUserItems)
	mux.HandleFunc("GET /getitembyguid", h.GetItemByGuid)
	mux.HandleFunc("GET /getitemandreplies", h.GetItemAndReplies)
	mux.HandleFunc("GET /getiteminfo", h.GetItemInfo)
	mux.HandleFunc("GET /getuserdata", h.GetUserData)
	mux.HandleFunc("GET /getlikerslist", h.GetLikersList)
	mux.HandleFunc("GET /getmostactivetoday", h.GetMostActiveToday)
	mux.HandleFunc("GET /getsubscriptionlist", h.GetSubscriptionList)
	mux.HandleFunc("GET /isuserindatabase", h.IsUserInDatabase)
	mux.HandleFunc("GET /isemailindatabase", h.IsEmailInDatabase)
	mux.HandleFunc("GET /checkwhitelist", h.CheckWhitelist)
	mux.HandleFunc("GET /robots.txt", h.RobotsTxt)

	// Auth endpoints (no auth required, but rate-limited in production)
	mux.HandleFunc("GET /sendconfirmingemail", h.SendConfirmingEmail)
	mux.HandleFunc("GET /createnewuser", h.CreateNewUser)

	// Write endpoints (authenticated)
	mux.HandleFunc("POST /newpost", h.NewPost)
	mux.HandleFunc("POST /updatepost", h.UpdatePost)
	mux.HandleFunc("POST /deletepost", h.DeletePost)
	mux.HandleFunc("POST /togglelike", h.ToggleLike)
	mux.HandleFunc("POST /saveprefs", h.SavePrefs)
	mux.HandleFunc("POST /uploadmedia", h.UploadMediaAuth)

	// Media serving (public)
	mux.HandleFunc("GET /media/{id}", h.HandleGetMedia)

	// Websocket
	mux.HandleFunc("GET /ws", h.WebSocket)
}

// Health is a simple health check endpoint.
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	RespondText(w, "OK")
}

// Feed serves an RSS feed: either user's feed or everyone's feed.
// Query params: screenname (optional)
func (h *Handler) Feed(w http.ResponseWriter, r *http.Request) {
	screenname := r.URL.Query().Get("screenname")

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
func (h *Handler) GetSubscriptionList(w http.ResponseWriter, r *http.Request) {
	h.Publisher.ServeOPML(w, r)
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

// RobotsTxt returns the robots.txt file.
func (h *Handler) RobotsTxt(w http.ResponseWriter, r *http.Request) {
	robotsTxt := `User-agent: *
Disallow: /getitembyguid
Disallow: /getiteminfo
`
	RespondText(w, robotsTxt)
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
