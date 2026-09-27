package api

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
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

	// Guards the blocklist sync state below; checkBlocklist runs on
	// concurrent request goroutines.
	blocklistMu     sync.Mutex
	blocklistMtime  int64 // blocklist.json mtime (UnixNano) at last sync; -1 if absent
	blocklistSynced bool

	EmailSender interface {
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
		RespondAuthError(w, r, "Authentication failed")
		return
	}

	h.HandleUploadMedia(w, r, user)
}

// UploadAvatarAuth wraps HandleUploadAvatar with authentication
func (h *Handler) UploadAvatarAuth(w http.ResponseWriter, r *http.Request) {
	denyCaching(w)

	email := r.FormValue("emailaddress")
	code := r.FormValue("emailcode")

	user, err := AuthenticateUser(h.DB, email, code)
	if err != nil {
		RespondAuthError(w, r, "Authentication failed")
		return
	}

	h.HandleUploadAvatar(w, r, user)
}

// RegisterRoutes registers all API endpoints with the mux at both root and /api/ paths.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// Helper to register each route at both /route and /api/route
	register := func(method, path string, handler http.HandlerFunc) {
		mux.HandleFunc(method+" "+path, handler)
		mux.HandleFunc(method+" /api"+path, handler)
	}

	// Health check endpoints
	register("GET", "/health", h.Health)
	register("GET", "/ready", h.Ready)
	register("GET", "/metrics", h.Metrics)

	// Read endpoints (no auth)
	register("GET", "/feed", h.Feed)
	// Matches the URL getCommentsFeedURL advertises. The trailing ".xml" is
	// part of {file}, not the pattern; see CommentsFeed.
	register("GET", "/comments/{screenname}/{file}", h.CommentsFeed)
	register("GET", "/getrecentitems", h.GetRecentItems)
	register("GET", "/getrecentuseritems", h.GetRecentUserItems)
	register("GET", "/getmentions", h.GetMentions)
	register("GET", "/gethashtagitems", h.GetHashtagItems)
	register("GET", "/gettrendinghashtags", h.GetTrendingHashtags)
	register("GET", "/getuseravatar", h.GetUserAvatar)
	register("GET", "/getuserswithavars", h.GetUsersWithAvatars)
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
	register("POST", "/uploadavatar", h.UploadAvatarAuth)

	// Media serving (public)
	register("GET", "/media/{id}", h.HandleGetMedia)

	// Websocket
	mux.HandleFunc("GET /ws", h.WebSocket)
}

// Health is a simple health check endpoint indicating the server is alive
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	HandleHealth(w, r)
}

// Ready checks if the server is ready to handle traffic
func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	// Create a context value with the database for readiness checks
	ctxWithDB := context.WithValue(r.Context(), "db", h.DB)
	newReq := r.WithContext(ctxWithDB)
	HandleReady(w, newReq)
}

// Metrics exposes Prometheus metrics
func (h *Handler) Metrics(w http.ResponseWriter, r *http.Request) {
	HandleMetrics(w, r)
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
		RespondValidationError(w, r, "format", "must be 'xml' or 'json'")
		return
	}

	// Build feed URL for WebSub headers
	var feedURL string
	if screenname == "" {
		feedURL = h.FeedConfig.BaseURL + "feed"
	} else {
		feedURL = h.FeedConfig.BaseURL + "feed?screenname=" + screenname
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
			RespondErrorWithIDAndCode(w, r, "Failed to build feed", "FEED_BUILD_ERROR", err.Error())
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
			addWebsubHeader(w, h.Config.URLWebsubHub, feedURL)
			w.Write(content)
			return
		}
	}

	// Fallback to filesystem mode
	if screenname == "" {
		// Serve everyone feed
		h.Publisher.ServeEveryoneFeed(w, r, h.Config.URLWebsubHub, feedURL)
	} else {
		// Serve user feed
		h.Publisher.ServeUserFeed(w, r, screenname, h.Config.URLWebsubHub, feedURL)
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
		RespondValidationError(w, r, "file", "path must end with .xml")
		return
	}

	itemID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		RespondValidationError(w, r, "id", "invalid post ID format")
		return
	}

	rss, err := feed.BuildCommentsFeed(h.DB, itemID, h.FeedConfig.BaseURL, h.FeedConfig)
	if err != nil {
		RespondErrorWithIDAndCode(w, r, "Failed to build comments feed", "FEED_BUILD_ERROR", err.Error())
		return
	}

	// Build comments feed URL for WebSub header
	screenname := r.PathValue("screenname")
	commentsFeedURL := h.FeedConfig.BaseURL + fmt.Sprintf("comments/%s/%s.xml", screenname, idStr)

	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	addWebsubHeader(w, h.Config.URLWebsubHub, commentsFeedURL)
	w.Write([]byte(rss))
}

// GetRecentItems returns the most recent posts on the network.
// Query params: ct (optional, default 100)
func (h *Handler) GetRecentItems(w http.ResponseWriter, r *http.Request) {
	ct := parseIntParam(r, "ct", 100)
	screenname := r.URL.Query().Get("screenname")

	items, err := getRecentItems(h.DB, screenname, ct, h.FeedConfig.BaseURL)
	if err != nil {
		RespondErrorWithIDAndCode(w, r, "Failed to get recent items", "RECENT_ITEMS_ERROR", err.Error())
		return
	}

	RespondJSON(w, items)
}

// GetRecentUserItems returns a user's recent posts.
// Query params: name (required), ct (optional)
func (h *Handler) GetRecentUserItems(w http.ResponseWriter, r *http.Request) {
	screenname := r.URL.Query().Get("name")
	if screenname == "" {
		RespondValidationError(w, r, "name", "required parameter missing")
		return
	}

	ct := parseIntParam(r, "ct", 100)
	viewerScreenname := r.URL.Query().Get("screenname")

	items, err := getRecentUserItems(h.DB, screenname, viewerScreenname, ct, h.FeedConfig.BaseURL)
	if err != nil {
		RespondErrorWithIDAndCode(w, r, "Failed to get user items", "USER_ITEMS_ERROR", err.Error())
		return
	}

	RespondJSON(w, items)
}

// GetMentions returns posts that mention a specific user.
// Query params: screenname (required), ct (optional continuation token)
func (h *Handler) GetMentions(w http.ResponseWriter, r *http.Request) {
	screenname := r.URL.Query().Get("screenname")
	if screenname == "" {
		RespondValidationError(w, r, "screenname", "required parameter missing")
		return
	}

	ct := r.URL.Query().Get("ct")
	maxCt := parseIntParam(r, "maxct", 100)

	items, err := getMentions(h.DB, screenname, ct, maxCt, h.FeedConfig.BaseURL)
	if err != nil {
		RespondErrorWithIDAndCode(w, r, "Failed to get mentions", "MENTIONS_ERROR", err.Error())
		return
	}

	RespondJSON(w, items)
}

// GetHashtagItems returns items tagged with a specific hashtag.
// Query params: tag (required), ct (optional), maxct (optional, default 100)
func (h *Handler) GetHashtagItems(w http.ResponseWriter, r *http.Request) {
	tag := r.URL.Query().Get("tag")
	if tag == "" {
		RespondValidationError(w, r, "tag", "required parameter missing")
		return
	}

	ct := r.URL.Query().Get("ct")
	maxCt := parseIntParam(r, "maxct", 100)

	items, err := getHashtagItems(h.DB, tag, ct, maxCt, h.FeedConfig.BaseURL)
	if err != nil {
		RespondErrorWithIDAndCode(w, r, "Failed to get hashtag items", "HASHTAGS_ERROR", err.Error())
		return
	}

	RespondJSON(w, items)
}

// GetTrendingHashtags returns trending hashtags.
// Query params: days (optional, default 7), limit (optional, default 50)
func (h *Handler) GetTrendingHashtags(w http.ResponseWriter, r *http.Request) {
	days := parseIntParam(r, "days", 7)
	limit := parseIntParam(r, "limit", 50)

	tags, err := db.GetTrendingHashtags(h.DB, days, limit)
	if err != nil {
		RespondErrorWithIDAndCode(w, r, "Failed to get trending hashtags", "TRENDING_HASHTAGS_ERROR", err.Error())
		return
	}

	if tags == nil {
		tags = []struct {
			Tag   string
			Count int
		}{}
	}

	RespondJSON(w, tags)
}

// GetItemByGuid returns a post by its guid.
// Query params: guid (required)
func (h *Handler) GetItemByGuid(w http.ResponseWriter, r *http.Request) {
	guid := r.URL.Query().Get("guid")
	if guid == "" {
		RespondValidationError(w, r, "guid", "required parameter missing")
		return
	}

	viewerScreenname := r.URL.Query().Get("screenname")

	item, err := getItemByGuid(h.DB, guid, viewerScreenname, h.FeedConfig.BaseURL)
	if err != nil {
		RespondErrorWithIDAndCode(w, r, "Failed to get item", "ITEM_FETCH_ERROR", err.Error())
		return
	}

	if item == nil {
		RespondNotFound(w, r, "Item not found")
		return
	}

	RespondJSON(w, item)
}

// GetItemAndReplies returns a post and its direct replies.
// Query params: idparent (required)
func (h *Handler) GetItemAndReplies(w http.ResponseWriter, r *http.Request) {
	idParent := int64(parseIntParam(r, "idparent", 0))
	if idParent == 0 {
		RespondValidationError(w, r, "idparent", "required parameter missing")
		return
	}

	viewerScreenname := r.URL.Query().Get("screenname")

	items, err := getItemAndReplies(h.DB, viewerScreenname, idParent, h.FeedConfig.BaseURL)
	if err != nil {
		RespondErrorWithIDAndCode(w, r, "Failed to get item and replies", "ITEM_REPLIES_ERROR", err.Error())
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
		RespondErrorWithIDAndCode(w, r, "Failed to get item info", "ITEM_INFO_ERROR", err.Error())
		return
	}

	if item == nil {
		RespondNotFound(w, r, "Item not found")
		return
	}

	if format == "feedland" {
		RespondJSON(w, item)
	} else if format == "rss" {
		RespondJSONString(w, item.Description) // Should return RSS-formatted item
	} else {
		RespondValidationError(w, r, "format", "invalid format")
	}
}

// GetUserData returns server and user information.
// Query params: screenname (optional)
func (h *Handler) GetUserData(w http.ResponseWriter, r *http.Request) {
	screenname := r.URL.Query().Get("screenname")

	data, err := getUserData(h.DB, screenname, h.FeedConfig)
	if err != nil {
		RespondErrorWithIDAndCode(w, r, "Can't get user data: "+err.Error(), "USER_DATA_ERROR", err.Error())
		return
	}

	RespondJSON(w, data)
}

// GetLikersList returns screennames of everyone who liked a post.
// Query params: id (required)
func (h *Handler) GetLikersList(w http.ResponseWriter, r *http.Request) {
	itemID := int64(parseIntParam(r, "id", 0))
	if itemID == 0 {
		RespondValidationError(w, r, "id", "required parameter missing")
		return
	}

	likers, err := getLikersList(h.DB, itemID)
	if err != nil {
		RespondErrorWithIDAndCode(w, r, "Can't get likers list: "+err.Error(), "LIKERS_LIST_ERROR", err.Error())
		return
	}

	RespondJSON(w, likers)
}

// GetMostActiveToday returns the most active users today.
// No query params.
func (h *Handler) GetMostActiveToday(w http.ResponseWriter, r *http.Request) {
	users, err := getMostActiveToday(h.DB)
	if err != nil {
		RespondErrorWithIDAndCode(w, r, "Failed to get most active users", "ACTIVE_USERS_ERROR", err.Error())
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
		RespondErrorWithIDAndCode(w, r, "Failed to build subscription list", "SUBSCRIPTION_LIST_ERROR", err.Error())
		return
	}

	// Build subscription list URL for WebSub header
	opmlFeedURL := h.FeedConfig.BaseURL + "getsubscriptionlist"

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	addWebsubHeader(w, h.Config.URLWebsubHub, opmlFeedURL)
	w.Write([]byte(opml))
}

// IsUserInDatabase checks if a user exists.
// Query params: screenname (required)
func (h *Handler) IsUserInDatabase(w http.ResponseWriter, r *http.Request) {
	screenname := r.URL.Query().Get("screenname")
	if screenname == "" {
		RespondValidationError(w, r, "screenname", "required parameter missing")
		return
	}

	exists, err := isUserInDatabase(h.DB, screenname)
	if err != nil {
		RespondErrorWithIDAndCode(w, r, "Failed to check user", "DB_LOOKUP_ERROR", err.Error())
		return
	}

	RespondJSON(w, map[string]bool{"flInDatabase": exists})
}

// IsEmailInDatabase checks if an email exists.
// Query params: email (required)
func (h *Handler) IsEmailInDatabase(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")
	if email == "" {
		RespondValidationError(w, r, "email", "required parameter missing")
		return
	}

	exists, err := isEmailInDatabase(h.DB, email)
	if err != nil {
		RespondErrorWithIDAndCode(w, r, "Failed to check email", "DB_LOOKUP_ERROR", err.Error())
		return
	}

	RespondJSON(w, map[string]bool{"flInDatabase": exists})
}

// CheckWhitelist checks if an email is whitelisted.
// Query params: emailaddress (required)
func (h *Handler) CheckWhitelist(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("emailaddress")
	if email == "" {
		RespondValidationError(w, r, "emailaddress", "required parameter missing")
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
		RespondAuthError(w, r, "Authentication failed")
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
		RespondAuthError(w, r, "Authentication failed")
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
		RespondAuthError(w, r, "Authentication failed")
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
		RespondAuthError(w, r, "Authentication failed")
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
		RespondAuthError(w, r, "Authentication failed")
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
