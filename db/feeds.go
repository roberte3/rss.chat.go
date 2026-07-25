package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// FeedRecord represents a stored feed in the database.
type FeedRecord struct {
	ID          int64
	FeedType    string    // 'global', 'user', 'opml'
	Screenname  *string   // NULL for global feed
	ContentType string    // 'text/xml', 'application/xml'
	Content     []byte    // XML/OPML blob
	WhenCreated time.Time
	WhenUpdated time.Time
}

// OpenFeedsDB opens or creates the feeds database.
func OpenFeedsDB(path string) (*sql.DB, error) {
	connStr := fmt.Sprintf("file:%s?mode=rwc", path)
	db, err := sql.Open("sqlite", connStr)
	if err != nil {
		return nil, fmt.Errorf("open feeds database: %w", err)
	}

	// Apply pragmas for performance and safety
	pragmas := []string{
		"PRAGMA journal_mode = WAL",
		"PRAGMA foreign_keys = ON",
		"PRAGMA busy_timeout = 5000",
	}
	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("apply pragma: %w", err)
		}
	}

	// Create schema if it doesn't exist
	if err := createFeedsSchema(db); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

// createFeedsSchema creates the feeds table and indexes.
func createFeedsSchema(db *sql.DB) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS feeds (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			feedType TEXT NOT NULL,
			screenname TEXT,
			contentType TEXT NOT NULL,
			content BLOB NOT NULL,
			whenCreated DATETIME DEFAULT CURRENT_TIMESTAMP,
			whenUpdated DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_feeds_type_screenname
			ON feeds(feedType, screenname)`,
		`CREATE INDEX IF NOT EXISTS idx_feeds_screenname_ci
			ON feeds(LOWER(screenname))`,
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			return fmt.Errorf("create feeds schema: %w", err)
		}
	}

	return nil
}

// StoreFeed stores or updates a feed in the database.
func StoreFeed(db *sql.DB, feedType, screenname, contentType string, content []byte) error {
	// Normalize screenname to NULL for global feeds
	var screennamePtr *string
	if screenname != "" && screenname != "global" {
		screennamePtr = &screenname
	}

	query := `
		INSERT INTO feeds (feedType, screenname, contentType, content, whenCreated, whenUpdated)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(feedType, screenname) DO UPDATE SET
			content = excluded.content,
			contentType = excluded.contentType,
			whenUpdated = CURRENT_TIMESTAMP
	`

	_, err := db.Exec(query, feedType, screennamePtr, contentType, content)
	if err != nil {
		return fmt.Errorf("store feed: %w", err)
	}

	return nil
}

// GetFeed retrieves a feed from the database.
func GetFeed(db *sql.DB, feedType, screenname string) ([]byte, error) {
	// Normalize screenname to NULL for global feeds
	var screennamePtr *string
	if screenname != "" && screenname != "global" {
		screennamePtr = &screenname
	}

	query := `SELECT content FROM feeds WHERE feedType = ? AND screenname IS ?`
	var content []byte
	err := db.QueryRow(query, feedType, screennamePtr).Scan(&content)
	if err == sql.ErrNoRows {
		return nil, nil // Feed not found, return nil (not an error)
	}
	if err != nil {
		return nil, fmt.Errorf("get feed: %w", err)
	}

	return content, nil
}

// GetFeedRecord retrieves full feed metadata from the database.
func GetFeedRecord(db *sql.DB, feedType, screenname string) (*FeedRecord, error) {
	// Normalize screenname to NULL for global feeds
	var screennamePtr *string
	if screenname != "" && screenname != "global" {
		screennamePtr = &screenname
	}

	query := `
		SELECT id, feedType, screenname, contentType, content, whenCreated, whenUpdated
		FROM feeds
		WHERE feedType = ? AND screenname IS ?
	`

	var record FeedRecord
	err := db.QueryRow(query, feedType, screennamePtr).Scan(
		&record.ID,
		&record.FeedType,
		&record.Screenname,
		&record.ContentType,
		&record.Content,
		&record.WhenCreated,
		&record.WhenUpdated,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get feed record: %w", err)
	}

	return &record, nil
}

// DeleteFeed removes a feed from the database.
func DeleteFeed(db *sql.DB, feedType, screenname string) error {
	// Normalize screenname to NULL for global feeds
	var screennamePtr *string
	if screenname != "" && screenname != "global" {
		screennamePtr = &screenname
	}

	query := `DELETE FROM feeds WHERE feedType = ? AND screenname IS ?`
	result, err := db.Exec(query, feedType, screennamePtr)
	if err != nil {
		return fmt.Errorf("delete feed: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}

	if affected == 0 {
		return fmt.Errorf("feed not found: %s/%s", feedType, screenname)
	}

	return nil
}

// ListFeeds returns all feeds in the database.
func ListFeeds(db *sql.DB) ([]FeedRecord, error) {
	query := `
		SELECT id, feedType, screenname, contentType, content, whenCreated, whenUpdated
		FROM feeds
		ORDER BY feedType, screenname
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("list feeds: %w", err)
	}
	defer rows.Close()

	var feeds []FeedRecord
	for rows.Next() {
		var record FeedRecord
		err := rows.Scan(
			&record.ID,
			&record.FeedType,
			&record.Screenname,
			&record.ContentType,
			&record.Content,
			&record.WhenCreated,
			&record.WhenUpdated,
		)
		if err != nil {
			return nil, fmt.Errorf("scan feed: %w", err)
		}
		feeds = append(feeds, record)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate feeds: %w", err)
	}

	return feeds, nil
}

// ListUserFeeds returns all feeds for a specific user (case-insensitive).
func ListUserFeeds(db *sql.DB, screenname string) ([]FeedRecord, error) {
	query := `
		SELECT id, feedType, screenname, contentType, content, whenCreated, whenUpdated
		FROM feeds
		WHERE LOWER(screenname) = LOWER(?)
		ORDER BY feedType
	`

	rows, err := db.Query(query, screenname)
	if err != nil {
		return nil, fmt.Errorf("list user feeds: %w", err)
	}
	defer rows.Close()

	var feeds []FeedRecord
	for rows.Next() {
		var record FeedRecord
		err := rows.Scan(
			&record.ID,
			&record.FeedType,
			&record.Screenname,
			&record.ContentType,
			&record.Content,
			&record.WhenCreated,
			&record.WhenUpdated,
		)
		if err != nil {
			return nil, fmt.Errorf("scan feed: %w", err)
		}
		feeds = append(feeds, record)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate user feeds: %w", err)
	}

	return feeds, nil
}

// FeedExists checks if a feed exists in the database.
func FeedExists(db *sql.DB, feedType, screenname string) (bool, error) {
	// Normalize screenname to NULL for global feeds
	var screennamePtr *string
	if screenname != "" && screenname != "global" {
		screennamePtr = &screenname
	}

	query := `SELECT 1 FROM feeds WHERE feedType = ? AND screenname IS ? LIMIT 1`
	var exists int
	err := db.QueryRow(query, feedType, screennamePtr).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check feed exists: %w", err)
	}

	return true, nil
}

// GetMissingFeeds returns all users who need feeds regenerated.
// Returns a list of screennames that don't have user feeds.
func GetMissingFeeds(db *sql.DB, allUserScreennames []string) ([]string, error) {
	if len(allUserScreennames) == 0 {
		return []string{}, nil
	}

	// Build placeholder list
	placeholders := make([]string, len(allUserScreennames))
	args := make([]interface{}, len(allUserScreennames))
	for i, name := range allUserScreennames {
		placeholders[i] = "?"
		args[i] = name
	}

	query := fmt.Sprintf(`
		SELECT screenname FROM (
			SELECT ? as screenname UNION ALL
			%s
		) all_users
		WHERE screenname NOT IN (
			SELECT screenname FROM feeds
			WHERE feedType = 'user' AND screenname IS NOT NULL
		)
	`, strings.Join(placeholders, " UNION ALL SELECT ?"))

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("get missing feeds: %w", err)
	}
	defer rows.Close()

	var missing []string
	for rows.Next() {
		var screenname string
		if err := rows.Scan(&screenname); err != nil {
			return nil, fmt.Errorf("scan missing screenname: %w", err)
		}
		missing = append(missing, screenname)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate missing feeds: %w", err)
	}

	return missing, nil
}
