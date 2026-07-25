package api

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"rss.chat.go/db"
	"strings"
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

	// Check blocklist
	if IsEmailBlocked(email) {
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

	// Validate screenname
	if !isValidScreenname(screenname) {
		RespondError(w, "Can't create new user because screenname is invalid")
		return
	}

	// Check blocklist
	if IsEmailBlocked(email) {
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
// For now, this is stubbed and will be called with a configured sender.
func (h *Handler) sendConfirmationEmail(recipient string, confirmationURL string, operationType string) error {
	// TODO: Wire up email.Sender from config in Phase 7
	// For now, we'll just log and return success
	fmt.Printf("Would send confirmation email to %s for %s\n", recipient, operationType)
	fmt.Printf("Confirmation URL: %s\n", confirmationURL)
	return nil
}

// checkWhitelist checks if an email is on the whitelist.
// TODO: Load from config in Phase 7
func (h *Handler) checkWhitelist(email string) bool {
	// For v1, no whitelist means allow all
	return true
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
