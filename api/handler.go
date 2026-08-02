package api

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/roberte3/rss.chat.go/config"
	"github.com/roberte3/rss.chat.go/db"
	"github.com/roberte3/rss.chat.go/feed"
	"github.com/roberte3/rss.chat.go/publish"
	"github.com/roberte3/rss.chat.go/websocket"
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

	// Limiters for the two unauthenticated endpoints that send mail. Both are
	// consulted: the per-email one keeps a single mailbox from being flooded
	// however many addresses the sender comes from, and the per-IP one caps
	// what any one source can spend of the server's SMTP quota and sending
	// reputation.
	authLimitByEmail *rateLimiter
	authLimitByIP    *rateLimiter
}

// Defaults for the mail-sending endpoints. Sized to be invisible to someone
// clicking "sign in" a few times in a row, while capping sustained abuse:
// 3 immediately per mailbox then one per 5 minutes, and 10 immediately per
// source address then one per minute — enough headroom for an office behind
// one NAT. Buckets idle for an hour are forgotten.
const (
	authBurstPerEmail  = 3
	authRefillPerEmail = 1.0 / 300.0 // one per 5 minutes
	authBurstPerIP     = 10
	authRefillPerIP    = 1.0 / 60.0 // one per minute
	authLimiterIdleTTL = time.Hour
	authRetryAfterHint = 5 * time.Minute
)

// NewHandler creates a new API handler.
func NewHandler(db *sql.DB, pub *publish.Publisher, cfg feed.BuilderConfig) *Handler {
	return &Handler{
		DB:               db,
		Publisher:        pub,
		FeedConfig:       cfg,
		authLimitByEmail: newRateLimiter(authRefillPerEmail, authBurstPerEmail, authLimiterIdleTTL),
		authLimitByIP:    newRateLimiter(authRefillPerIP, authBurstPerIP, authLimiterIdleTTL),
	}
}

// allowAuthRequest applies both limiters to a mail-sending request, writing a
// 429 and reporting false when either is exhausted. Limiters may be nil on a
// Handler built as a bare struct literal, in which case nothing is limited.
func (h *Handler) allowAuthRequest(w http.ResponseWriter, r *http.Request, email, operation string) bool {
	if h.authLimitByIP != nil && !h.authLimitByIP.allow(clientIP(r)) {
		RespondTooManyRequests(w, authRetryAfterHint,
			"Can't "+operation+" because too many requests have come from your address recently, please wait a few minutes")
		return false
	}
	// Checked second so a flood aimed at one mailbox from many sources still
	// trips, and so the per-IP budget is spent first by the noisier case.
	if h.authLimitByEmail != nil && !h.authLimitByEmail.allow(limitKeyForEmail(email)) {
		RespondTooManyRequests(w, authRetryAfterHint,
			"Can't "+operation+" because too many requests have been made for this email address recently, please wait a few minutes")
		return false
	}
	return true
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
	denyCaching(w)

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
	// Matches the URL getCommentsFeedURL advertises. The trailing ".xml" is
	// part of {file}, not the pattern; see CommentsFeed.
	register("GET", "/comments/{screenname}/{file}", h.CommentsFeed)
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

	// Auth endpoints. Unauthenticated by nature — they are how a caller gets a
	// credential — so both are rate-limited per email and per source address;
	// see allowAuthRequest.
	register("GET", "/sendconfirmingemail", h.SendConfirmingEmail)
	register("GET", "/createnewuser", h.CreateNewUser)

	// Local-only account bootstrap. No rate limiting or whitelist/blocklist
	// check -- access is gated by isLoopbackRequest instead, see LocalNewUser.
	register("GET", "/localnewuser", h.LocalNewUser)

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

// CommentsFeed serves a post's comments feed: the post plus its replies. The
// URL shape is /comments/{screenname}/{id}.xml, which is what
// getCommentsFeedURL advertises in every feed the server generates.
//
// The ".xml" cannot be part of the route pattern — net/http requires a wildcard
// to span a whole path segment, and "{id}.xml" panics at registration — so the
// segment is matched whole and the extension is stripped here.
//
// The feed is built from the database per request, like the subscription list.
// PublishCommentsFeed only ever writes to the filesystem, so serving the
// published copy would fail outright in database mode, and in filesystem mode
// until the first republish.
//
// {screenname} is not checked against the post's author. The id determines the
// content, and the original treats the path as a storage location rather than
// an assertion about ownership.
func (h *Handler) CommentsFeed(w http.ResponseWriter, r *http.Request) {
	idStr, ok := strings.CutSuffix(r.PathValue("file"), ".xml")
	if !ok {
		RespondError(w, "Can't get the comments feed because the path must end in .xml")
		return
	}

	itemID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		RespondError(w, "Can't get the comments feed because "+idStr+" is not a post id")
		return
	}

	rss, err := feed.BuildCommentsFeed(h.DB, itemID, h.FeedConfig.BaseURL, h.FeedConfig)
	if err != nil {
		RespondError(w, "Can't get the comments feed because "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	w.Write([]byte(rss))
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
	denyCaching(w)

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
	denyCaching(w)

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
	denyCaching(w)

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
	denyCaching(w)

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
	denyCaching(w)

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
