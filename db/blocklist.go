package db

import (
	"database/sql"
	"fmt"
	"strings"
)

// SyncBlocklistToDB clears the blocklist table and inserts the given emails.
func SyncBlocklistToDB(conn *sql.DB, emails []string) error {
	tx, err := conn.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Clear existing blocklist
	if _, err := tx.Exec("DELETE FROM blocklist"); err != nil {
		return fmt.Errorf("clear blocklist: %w", err)
	}

	// Insert new blocklist entries
	for _, email := range emails {
		email = strings.ToLower(strings.TrimSpace(email))
		if email == "" {
			continue
		}
		if _, err := tx.Exec("INSERT INTO blocklist (email) VALUES (?)", email); err != nil {
			return fmt.Errorf("insert email %q: %w", email, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// GetBlocklistEmails returns all blocked emails from the database.
func GetBlocklistEmails(conn *sql.DB) ([]string, error) {
	rows, err := conn.Query("SELECT email FROM blocklist ORDER BY email")
	if err != nil {
		return nil, fmt.Errorf("query blocklist: %w", err)
	}
	defer rows.Close()

	var emails []string
	for rows.Next() {
		var email string
		if err := rows.Scan(&email); err != nil {
			return nil, fmt.Errorf("scan email: %w", err)
		}
		emails = append(emails, email)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rows: %w", err)
	}

	return emails, nil
}

// IsEmailBlocked checks if an email is in the blocklist (case-insensitive).
func IsEmailBlocked(conn *sql.DB, email string) (bool, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	var count int
	err := conn.QueryRow("SELECT COUNT(*) FROM blocklist WHERE email = ?", email).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("query blocklist: %w", err)
	}
	return count > 0, nil
}

// ClearBlocklist removes all entries from the blocklist.
func ClearBlocklist(conn *sql.DB) error {
	_, err := conn.Exec("DELETE FROM blocklist")
	if err != nil {
		return fmt.Errorf("clear blocklist: %w", err)
	}
	return nil
}
