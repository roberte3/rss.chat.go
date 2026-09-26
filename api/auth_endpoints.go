package api

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/roberte3/rss.chat.go/config"
	"github.com/roberte3/rss.chat.go/db"
	applog "github.com/roberte3/rss.chat.go/log"
)

// SendConfirmingEmail handles the /sendconfirmingemail endpoint.
// Generates or reuses a per-user emailSecret and sends a confirmation link.
// Query params: email, urlredirect
func (h *Handler) SendConfirmingEmail(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")
	urlRedirect := r.URL.Query().Get("urlredirect")

	if email == "" {
		LogValidationError(r, "email", "required parameter missing")
		RespondValidationError(w, r, "email", "required parameter missing")
		return
	}

	if urlRedirect == "" {
		LogValidationError(r, "urlredirect", "required parameter missing")
		RespondValidationError(w, r, "urlredirect", "required parameter missing")
		return
	}

	// Before any mail is sent: this endpoint will deliver to an address the
	// caller names, so unthrottled it is a way to flood someone else's mailbox
	// and burn this server's sending reputation.
	if !h.allowAuthRequest(w, r, email, "send confirmation email") {
		LogAuthFailure(r, "rate limit exceeded")
		return
	}

	// Check blocklist
	if !h.checkBlocklist(email) {
		LogAuthFailure(r, "email is blocklisted")
		RespondErrorWithIDAndCode(w, r, "This email is not allowed", "EMAIL_BLOCKLISTED", "Email address is on the blocklist")
		return
	}

	// Find or create the user
	user, err := db.GetUserInfoByEmail(h.DB, email)
	if err != nil {
		LogOperationError(r.Context(), "get_user_by_email", err, map[string]interface{}{})
		RespondErrorWithIDAndCode(w, r, "Database error looking up user", "DB_ERROR", err.Error())
		return
	}

	// For existing user, reuse their emailSecret
	// For new user, we'll create one during /createnewuser
	if user == nil {
		// New user: check whitelist
		if !h.checkWhitelist(email) {
			LogAuthFailure(r, "email is not whitelisted")
			RespondErrorWithIDAndCode(w, r, "This email is not whitelisted", "EMAIL_NOT_WHITELISTED", "Email address is not on the whitelist")
			return
		}

		// Create a temporary emailSecret for the confirmation link
		// (actual user will be created in /createnewuser)
		emailSecret := generateEmailSecret()

		// Store temporarily (note: in a real system, we might use a temp table or cache)
		// For now, we'll just generate it fresh each time /createnewuser is called
		// This is handled in the createnewuser endpoint
		_ = emailSecret

		// Build confirmation URL
		confirmationURL := buildConfirmationURL(urlRedirect, email, "", emailSecret)

		// Send email
		if err := h.sendConfirmationEmail(email, confirmationURL, "signup"); err != nil {
			LogOperationError(r.Context(), "send_confirmation_email", err, map[string]interface{}{"type": "signup"})
			RespondErrorWithIDAndCode(w, r, "Failed to send confirmation email", "EMAIL_SEND_FAILED", err.Error())
			return
		}

		LogOperationComplete(r.Context(), "send_confirmation_email", map[string]interface{}{"type": "signup"})
		RespondJSON(w, map[string]string{"status": "confirmation email sent"})
		return
	}

	// Existing user: reuse their emailSecret
	emailSecret := user.EmailSecret
	if emailSecret == "" {
		// If no secret exists, generate one and update the user
		emailSecret = generateEmailSecret()
		if err := db.UpdateUser(h.DB, user.Screenname, user.EmailAddress, emailSecret); err != nil {
			LogOperationError(r.Context(), "update_user_secret", err, map[string]interface{}{})
			RespondErrorWithIDAndCode(w, r, "Database error updating user", "DB_ERROR", err.Error())
			return
		}
	}

	// Build confirmation URL
	confirmationURL := buildConfirmationURL(urlRedirect, email, user.Screenname, emailSecret)

	// Send email
	if err := h.sendConfirmationEmail(email, confirmationURL, "signin"); err != nil {
		LogOperationError(r.Context(), "send_confirmation_email", err, map[string]interface{}{"type": "signin"})
		RespondErrorWithIDAndCode(w, r, "Failed to send confirmation email", "EMAIL_SEND_FAILED", err.Error())
		return
	}

	LogOperationComplete(r.Context(), "send_confirmation_email", map[string]interface{}{"type": "signin"})
	RespondJSON(w, map[string]string{"status": "confirmation email sent"})
}

// CreateNewUser handles the /createnewuser endpoint.
// Creates a new user account after email confirmation.
// Query params: email, name (screenname), urlredirect
func (h *Handler) CreateNewUser(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")
	screenname := r.URL.Query().Get("name")
	urlRedirect := r.URL.Query().Get("urlredirect")

	if email == "" || screenname == "" {
		RespondValidationError(w, r, "email/name", "email and name are required")
		return
	}

	if urlRedirect == "" {
		RespondValidationError(w, r, "urlredirect", "required parameter missing")
		return
	}

	// Creates an account and sends mail, so it carries the same abuse potential
	// as /sendconfirmingemail and shares its limits.
	if !h.allowAuthRequest(w, r, email, "create new user") {
		return
	}

	// Validate screenname
	if !isValidScreenname(screenname) {
		RespondValidationError(w, r, "name", "screenname is invalid")
		return
	}

	// Check blocklist
	if !h.checkBlocklist(email) {
		RespondErrorWithIDAndCode(w, r, "This email is not allowed", "EMAIL_BLOCKLISTED", "Email address is on the blocklist")
		return
	}

	// Check whitelist
	if !h.checkWhitelist(email) {
		RespondErrorWithIDAndCode(w, r, "This email is not whitelisted", "EMAIL_NOT_WHITELISTED", "Email address is not on the whitelist")
		return
	}

	// Check if email already exists
	existingUser, err := db.GetUserInfoByEmail(h.DB, email)
	if err != nil {
		RespondErrorWithIDAndCode(w, r, "Database error looking up user", "DB_ERROR", err.Error())
		return
	}
	if existingUser != nil {
		RespondErrorWithIDAndCode(w, r, "This email is already registered", "EMAIL_ALREADY_REGISTERED", "An account with this email already exists")
		return
	}

	// Check if screenname already exists
	existingScreenname, err := db.GetUserInfoByScreenname(h.DB, screenname)
	if err != nil {
		RespondErrorWithIDAndCode(w, r, "Database error looking up screenname", "DB_ERROR", err.Error())
		return
	}
	if existingScreenname != nil {
		RespondErrorWithIDAndCode(w, r, "This screenname is already taken", "SCREENNAME_TAKEN", "The requested screenname is already in use")
		return
	}

	// Generate email secret (this is the code the user will confirm)
	emailSecret := generateEmailSecret()

	// Create the user
	if err := db.AddUser(h.DB, screenname, email, emailSecret); err != nil {
		RespondErrorWithIDAndCode(w, r, "Failed to create user", "DB_ERROR", err.Error())
		return
	}

	// A new user changes the subscription list, so refresh the published copy.
	// Ports the updateSubscriptionListOnS3 call addUser makes in
	// rssnetwork.js. The user exists either way, so a failure here is logged
	// rather than surfaced.
	if err := h.Publisher.PublishSubscriptionList(h.DB); err != nil {
		LogWarning(r.Context(), "failed to publish subscription list after creating user", "screenname", screenname, "err", err)
	}

	// Publish the new user's feed straight away, so it answers with a valid
	// empty feed instead of a 404 from the moment the account exists. Without
	// this, anyone subscribing off the subscription list above gets a 404
	// until the user's first post triggers UpdateFeedsOnPostWrite. Ports the
	// addEmailToUserInDatabase change of 7/25/26 (server v0.6.5).
	if err := h.Publisher.PublishUserFeed(h.DB, screenname); err != nil {
		LogWarning(r.Context(), "failed to publish feed for new user", "screenname", screenname, "err", err)
	}

	// Build confirmation URL
	confirmationURL := buildConfirmationURL(urlRedirect, email, screenname, emailSecret)

	// Send confirmation email
	if err := h.sendConfirmationEmail(email, confirmationURL, "signup"); err != nil {
		// User was created but email failed—log but don't fail the request
		// User can resend via /sendconfirmingemail
		LogWarning(r.Context(), "failed to send confirmation email to new user", "screenname", screenname, "err", err)
	}

	RespondJSON(w, map[string]interface{}{
		"status":     "user created",
		"screenname": screenname,
		"email":      email,
	})
}

// buildConfirmationURL builds the confirmation URL to send in the email.
func buildConfirmationURL(urlRedirect string, userEmail string, screenname string, emailSecret string) string {
	// Append confirmation params to the redirect URL
	separator := "?"
	if strings.Contains(urlRedirect, "?") {
		separator = "&"
	}

	return fmt.Sprintf(
		"%s%semailconfirmed=true&email=%s&code=%s&screenname=%s",
		urlRedirect,
		separator,
		url.QueryEscape(userEmail),
		url.QueryEscape(emailSecret),
		url.QueryEscape(screenname),
	)
}

// sendConfirmationEmail sends a confirmation email via the email sender.
func (h *Handler) sendConfirmationEmail(recipient string, confirmationURL string, operationType string) error {
	if h.EmailSender == nil {
		// Email sender not configured, just log
		fmt.Printf("Email sender not configured - would send to %s for %s\n", recipient, operationType)
		return nil
	}

	err := h.EmailSender.SendConfirmationEmail(recipient, confirmationURL, operationType)
	if err != nil {
		fmt.Printf("Error sending confirmation email to %s: %v\n", recipient, err)
		return err
	}

	fmt.Printf("Sent confirmation email to %s for %s\n", recipient, operationType)
	return nil
}

// checkWhitelist checks if an email is on the whitelist.
// Returns true if whitelist is empty (allow all) or email is on the list.
func (h *Handler) checkWhitelist(email string) bool {
	if h.Config == nil {
		return true // No config, allow all
	}
	return h.Config.IsEmailWhitelisted(email)
}

// refreshBlocklist rewrites the database blocklist from config.BlockedUsersList
// plus blocklist.json, but only on the first call and whenever blocklist.json's
// mtime changes (including it appearing or disappearing). SyncBlocklistToDB
// replaces the whole table, so running it per request would be a full rewrite
// on every signup. On a read or sync error the previous table is left in place
// and the next call retries.
func (h *Handler) refreshBlocklist() {
	h.blocklistMu.Lock()
	defer h.blocklistMu.Unlock()

	mtime := int64(-1)
	if fi, err := os.Stat(h.Config.BlocklistPath); err == nil {
		mtime = fi.ModTime().UnixNano()
	}
	if h.blocklistSynced && mtime == h.blocklistMtime {
		return
	}

	fileEmails, err := config.LoadBlocklist(h.Config.BlocklistPath)
	if err != nil {
		ctx := context.Background()
		logger := applog.WithContext(ctx)
		logger.WarnContext(ctx, "failed to load blocklist", "err", err)
		return
	}

	merged := make([]string, 0, len(h.Config.BlockedUsersList)+len(fileEmails))
	merged = append(merged, h.Config.BlockedUsersList...)
	merged = append(merged, fileEmails...)
	if err := db.SyncBlocklistToDB(h.DB, dedupeEmails(merged)); err != nil {
		ctx := context.Background()
		logger := applog.WithContext(ctx)
		logger.WarnContext(ctx, "failed to sync blocklist to database", "err", err)
		return
	}

	h.blocklistMtime = mtime
	h.blocklistSynced = true
}

// dedupeEmails lowercases, trims and de-duplicates, dropping blanks.
// SyncBlocklistToDB inserts row by row, so duplicates would hit the key.
func dedupeEmails(emails []string) []string {
	seen := make(map[string]bool, len(emails))
	out := make([]string, 0, len(emails))
	for _, e := range emails {
		e = strings.ToLower(strings.TrimSpace(e))
		if e == "" || seen[e] {
			continue
		}
		seen[e] = true
		out = append(out, e)
	}
	return out
}

// checkBlocklist reports whether email is allowed (not blocked). Both
// config.BlockedUsersList and blocklist.json are enforced; edits to
// blocklist.json take effect without a restart.
func (h *Handler) checkBlocklist(email string) bool {
	if h.Config == nil || h.DB == nil {
		return true
	}

	h.refreshBlocklist()

	blocked, err := db.IsEmailBlocked(h.DB, email)
	if err != nil {
		ctx := context.Background()
		logger := applog.WithContext(ctx)
		logger.WarnContext(ctx, "failed to check blocklist", "err", err)
		return true
	}
	return !blocked
}

// generateEmailSecret generates a random email confirmation code.
// Uses crypto/rand for cryptographic randomness.
func generateEmailSecret() string {
	// Generate 32 random bytes
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		panic(fmt.Sprintf("failed to generate random secret: %v", err))
	}

	// Encode as base64url for safe URL transport
	encoded := base64.RawURLEncoding.EncodeToString(secret)
	return encoded
}

// isValidScreenname validates a screenname.
// Screennames should be alphanumeric, no spaces or special chars.
func isValidScreenname(screenname string) bool {
	if len(screenname) == 0 || len(screenname) > 64 {
		return false
	}

	for _, r := range screenname {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
			return false
		}
	}

	return true
}
