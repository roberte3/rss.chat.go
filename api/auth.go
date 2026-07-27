package api

import (
	"crypto/subtle"
	"database/sql"
	"fmt"

	"github.com/roberte3/rss.chat.go/db"
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

	// Compared in constant time. == returns as soon as two bytes differ, so how
	// long a rejection takes depends on how long a prefix the caller got right,
	// which is a signal an attacker can walk the secret out of one byte at a
	// time. The margin is tiny next to a network round trip and the database
	// read above, but the fix costs nothing.
	//
	// Length still leaks: ConstantTimeCompare reports 0 immediately for
	// mismatched lengths. Secrets here are a fixed 32 bytes from
	// generateEmailSecret, so that reveals nothing.
	if subtle.ConstantTimeCompare([]byte(user.EmailSecret), []byte(code)) != 1 {
		return nil, fmt.Errorf("invalid authentication code")
	}

	return user, nil
}

// IsUserAdmin checks if a user is an admin.
// Currently a stub—the JS original also returns false.
// TODO: Implement admin logic if needed
func IsUserAdmin(conn *sql.DB, screenname string) (bool, error) {
	// For v1, no admins
	return false, nil
}
