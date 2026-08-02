package api

import (
	"database/sql"
	"log"
	"net"
	"net/http"

	"github.com/roberte3/rss.chat.go/db"
)

// isLoopbackRequest reports whether r arrived straight from the machine the
// server runs on. Ports requestIsFromThisMachine from rssnetwork.js: any
// request carrying X-Forwarded-For is rejected outright rather than trusted,
// since behind a proxy the socket address is the proxy's and the header
// itself is attacker-controlled -- same posture clientIP takes for rate
// limiting, applied here as an access gate instead.
func isLoopbackRequest(r *http.Request) bool {
	if r.Header.Get("X-Forwarded-For") != "" {
		return false
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// findOrCreateLocalUser looks up screenname, creating it with a fresh
// emailSecret if it doesn't exist yet. Mirrors addEmailToUserInDatabase: an
// existing screenname's stored email and secret win over whatever was passed
// in, so a repeat call is a safe, idempotent no-op rather than a drift risk.
func findOrCreateLocalUser(conn *sql.DB, screenname, email string) (*db.User, bool, error) {
	existing, err := db.GetUserInfoByScreenname(conn, screenname)
	if err != nil {
		return nil, false, err
	}
	if existing != nil {
		return existing, false, nil
	}

	emailSecret := generateEmailSecret()
	if err := db.AddUser(conn, screenname, email, emailSecret); err != nil {
		return nil, false, err
	}

	created, err := db.GetUserInfoByScreenname(conn, screenname)
	if err != nil {
		return nil, false, err
	}
	return created, true, nil
}

// LocalNewUser handles the /localnewuser endpoint.
//
// Ports rssnetwork.js's /localnewuser (added 7/29/26, issue #205): a
// loopback-only account bootstrap that skips the email confirmation round
// trip and, unlike /createnewuser, returns the emailSecret directly in the
// response. That's only safe because isLoopbackRequest gates it -- there is
// deliberately no whitelist/blocklist/rate-limit check here, matching the
// reference implementation, since those exist to protect the public signup
// path this endpoint doesn't use.
//
// Query params: screenname, email
func (h *Handler) LocalNewUser(w http.ResponseWriter, r *http.Request) {
	denyCaching(w)

	if !isLoopbackRequest(r) {
		RespondError(w, "Can't create the user because localnewuser only works from the machine the server is running on")
		return
	}

	screenname := r.URL.Query().Get("screenname")
	email := r.URL.Query().Get("email")

	if screenname == "" {
		RespondError(w, "Can't create the user because no screenname was specified")
		return
	}
	if email == "" {
		RespondError(w, "Can't create the user "+screenname+" because no email address was specified")
		return
	}
	if !isValidScreenname(screenname) {
		RespondError(w, "Can't create the user because screenname is invalid")
		return
	}

	user, isNewUser, err := findOrCreateLocalUser(h.DB, screenname, email)
	if err != nil {
		RespondError(w, "Can't create the user because "+err.Error())
		return
	}

	if isNewUser {
		// Same tail as CreateNewUser: a new user changes the subscription
		// list, and needs its own feed publishing straight away so it
		// answers with a valid empty feed instead of a 404. Both are
		// best-effort -- the user exists either way.
		if err := h.Publisher.PublishSubscriptionList(h.DB); err != nil {
			log.Printf("warning: failed to publish subscription list after creating %s: %v", screenname, err)
		}
		if err := h.Publisher.PublishUserFeed(h.DB, screenname); err != nil {
			log.Printf("warning: failed to publish feed for new user %s: %v", screenname, err)
		}
	}

	RespondJSON(w, map[string]interface{}{
		"screenname":  user.Screenname,
		"email":       user.EmailAddress,
		"emailSecret": user.EmailSecret,
	})
}
