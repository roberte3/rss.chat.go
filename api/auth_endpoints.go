package api

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/roberte3/rss.chat.go/config"
	"github.com/roberte3/rss.chat.go/db"
)

// SendConfirmingEmail handles the /sendconfirmingemail endpoint.
// Generates or reuses a per-user emailSecret and sends a confirmation link.
// Query params: email, urlredirect
func (h *Handler) SendConfirmingEmail(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")
	urlRedirect := r.URL.Query().Get("urlredirect")

	if email == "" {
		RespondError(w, "Can't send confirmation email because email is required")
		return
	}

	if urlRedirect == "" {
		RespondError(w, "Can't send confirmation email because urlredirect is required")
		return
	}

	// Before any mail is sent: this endpoint will deliver to an address the
	// caller names, so unthrottled it is a way to flood someone else's mailbox
	// and burn this server's sending reputation.
	if !h.allowAuthRequest(w, r, email, "send confirmation email") {
		return
	}

	// Check blocklist
	if !h.checkBlocklist(email) {
		RespondError(w, "Can't send confirmation email because this email is not allowed")
		return
	}

	// Find or create the user
	user, err := db.GetUserInfoByEmail(h.DB, email)
	if err != nil {
		RespondError(w, "Can't send confirmation email because of database error")
		return
	}

	// For existing user, reuse their emailSecret
	// For new user, we'll create one during /createnewuser
	if user == nil {
		// New user: check whitelist
		if !h.checkWhitelist(email) {
			RespondError(w, "Can't send confirmation email because this email is not whitelisted")
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
			RespondError(w, "Can't send confirmation email because email sending failed")
			return
		}

		RespondJSON(w, map[string]string{"status": "confirmation email sent"})
		return
	}

	// Existing user: reuse their emailSecret
	emailSecret := user.EmailSecret
	if emailSecret == "" {
		// If no secret exists, generate one and update the user
		emailSecret = generateEmailSecret()
		if err := db.UpdateUser(h.DB, user.Screenname, user.EmailAddress, emailSecret); err != nil {
			RespondError(w, "Can't send confirmation email because of database error")
			return
		}
	}

	// Build confirmation URL
	confirmationURL := buildConfirmationURL(urlRedirect, email, user.Screenname, emailSecret)

	// Send email
	if err := h.sendConfirmationEmail(email, confirmationURL, "signin"); err != nil {
		RespondError(w, "Can't send confirmation email because email sending failed")
		return
	}

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
		RespondError(w, "Can't create new user because email and name are required")
		return
	}

	if urlRedirect == "" {
		RespondError(w, "Can't create new user because urlredirect is required")
		return
	}

	// Creates an account and sends mail, so it carries the same abuse potential
	// as /sendconfirmingemail and shares its limits.
	if !h.allowAuthRequest(w, r, email, "create new user") {
		return
	}

	// Validate screenname
	if !isValidScreenname(screenname) {
		RespondError(w, "Can't create new user because screenname is invalid")
		return
	}

	// Check blocklist
	if !h.checkBlocklist(email) {
		RespondError(w, "Can't create new user because this email is not allowed")
		return
	}

	// Check whitelist
	if !h.checkWhitelist(email) {
		RespondError(w, "Can't create new user because this email is not whitelisted")
		return
	}

	// Check if email already exists
	existingUser, err := db.GetUserInfoByEmail(h.DB, email)
	if err != nil {
		RespondError(w, "Can't create new user because of database error")
		return
	}
	if existingUser != nil {
		RespondError(w, "Can't create new user because this email is already registered")
		return
	}

	// Check if screenname already exists
	existingScreenname, err := db.GetUserInfoByScreenname(h.DB, screenname)
	if err != nil {
		RespondError(w, "Can't create new user because of database error")
		return
	}
	if existingScreenname != nil {
		RespondError(w, "Can't create new user because this screenname is already taken")
		return
	}

	// Generate email secret (this is the code the user will confirm)
	emailSecret := generateEmailSecret()

	// Create the user
	if err := db.AddUser(h.DB, screenname, email, emailSecret); err != nil {
		RespondError(w, "Can't create new user because "+err.Error())
		return
	}

	// A new user changes the subscription list, so refresh the published copy.
	// Ports the updateSubscriptionListOnS3 call addUser makes in
	// rssnetwork.js. The user exists either way, so a failure here is logged
	// rather than surfaced.
	if err := h.Publisher.PublishSubscriptionList(h.DB); err != nil {
		log.Printf("warning: failed to publish subscription list after creating %s: %v", screenname, err)
	}

	// Build confirmation URL
	confirmationURL := buildConfirmationURL(urlRedirect, email, screenname, emailSecret)

	// Send confirmation email
	if err := h.sendConfirmationEmail(email, confirmationURL, "signup"); err != nil {
		// User was created but email failed—log but don't fail the request
		// User can resend via /sendconfirmingemail
		fmt.Printf("Warning: failed to send confirmation email to %s: %v\n", email, err)
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

// checkBlocklist checks if an email is NOT blocked.
// Returns true if email is allowed, false if blocked.
// Uses mtime cache to avoid expensive reloads when file hasn't changed.
func (h *Handler) checkBlocklist(email string) bool {
	if h.Config == nil || h.DB == nil {
		return true // No config, allow all
	}

	// Check if blocklist file has been modified
	fi, err := os.Stat(h.Config.BlocklistPath)
	if err != nil {
		// File doesn't exist or can't be read, allow all
		return true
	}

	currentMtime := fi.ModTime().Unix()

	// Only reload if file has been modified since last check
	if currentMtime != h.blocklistMtime {
		h.blocklistMtime = currentMtime

		// Reload blocklist from JSON file
		emails, err := config.LoadBlocklist(h.Config.BlocklistPath)
		if err != nil {
			// Log error but allow access if file can't be read
			fmt.Printf("Warning: failed to load blocklist: %v\n", err)
			return true
		}

		// Sync to database for backup
		if err := db.SyncBlocklistToDB(h.DB, emails); err != nil {
			fmt.Printf("Warning: failed to sync blocklist to database: %v\n", err)
			// Continue even if sync fails
		}
	}

	// Check if email is blocked
	blocked, err := db.IsEmailBlocked(h.DB, email)
	if err != nil {
		// Log error but allow access if database check fails
		fmt.Printf("Warning: failed to check blocklist: %v\n", err)
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
