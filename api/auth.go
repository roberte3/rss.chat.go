package api

import (
	"database/sql"
	"fmt"
	"rss.chat.go/db"
)

// AuthenticateUser verifies email and code credentials.
func AuthenticateUser(conn *sql.DB, email, code string) (*db.User, error) {
	if email == "" || code == "" {
		return nil, fmt.Errorf("authentication credentials required")
	}

	user, err := db.GetUserInfoByEmail(conn, email)
	if err != nil {
		return nil, fmt.Errorf("authentication failed")
	}

	if user == nil {
		return nil, fmt.Errorf("user not found")
	}

	// Verify the email code matches the user's email secret
	if user.EmailSecret != code {
		return nil, fmt.Errorf("invalid authentication code")
	}

	return user, nil
}

// IsEmailBlocked checks if an email is on the blocklist.
// TODO: Load from config instead of hardcoding
func IsEmailBlocked(email string) bool {
	// For now, allow all emails (no blocklist)
	return false
}

// IsUserAdmin checks if a user is an admin.
// Currently a stub—the JS original also returns false.
// TODO: Implement admin logic if needed
func IsUserAdmin(conn *sql.DB, screenname string) (bool, error) {
	// For v1, no admins
	return false, nil
}
