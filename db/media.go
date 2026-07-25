package db

import (
	"database/sql"
	"fmt"
)

// Media represents an uploaded media file
type Media struct {
	ID          int64
	Screenname  string
	ContentType string
	Size        int64
	WhenCreated string
	// Data is not loaded by default (it's binary blob in DB)
}

// OpenMediaDB opens or creates the media database
func OpenMediaDB(path string) (*sql.DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open media database: %w", err)
	}

	// Set pragmas for performance and safety
	if _, err := conn.Exec(`
		PRAGMA journal_mode = WAL;
		PRAGMA foreign_keys = ON;
		PRAGMA busy_timeout = 5000;
	`); err != nil {
		conn.Close()
		return nil, fmt.Errorf("set media database pragmas: %w", err)
	}

	// Create media table if it doesn't exist
	if _, err := conn.Exec(`
		CREATE TABLE IF NOT EXISTS media (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			screenname TEXT NOT NULL COLLATE NOCASE,
			contentType TEXT NOT NULL,
			mediabytes BLOB NOT NULL,
			size INTEGER NOT NULL,
			whenCreated TEXT DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_media_screenname ON media(screenname);
	`); err != nil {
		conn.Close()
		return nil, fmt.Errorf("create media table: %w", err)
	}

	return conn, nil
}

// StoreMedia stores a media file and returns its ID
func StoreMedia(db *sql.DB, screenname, contentType string, data []byte) (int64, error) {
	result, err := db.Exec(
		`INSERT INTO media (screenname, contentType, mediabytes, size) VALUES (?, ?, ?, ?)`,
		screenname, contentType, data, len(data),
	)
	if err != nil {
		return 0, fmt.Errorf("insert media: %w", err)
	}

	mediaID, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("get media id: %w", err)
	}

	return mediaID, nil
}

// GetMedia retrieves media by ID
func GetMedia(db *sql.DB, mediaID int64) (contentType string, data []byte, err error) {
	row := db.QueryRow(
		`SELECT contentType, mediabytes FROM media WHERE id = ?`,
		mediaID,
	)
	err = row.Scan(&contentType, &data)
	if err == sql.ErrNoRows {
		return "", nil, fmt.Errorf("media not found")
	}
	if err != nil {
		return "", nil, fmt.Errorf("query media: %w", err)
	}
	return contentType, data, nil
}

// GetMediaInfo retrieves media metadata without the binary data
func GetMediaInfo(db *sql.DB, mediaID int64) (*Media, error) {
	var m Media
	row := db.QueryRow(
		`SELECT id, screenname, contentType, size, whenCreated FROM media WHERE id = ?`,
		mediaID,
	)
	err := row.Scan(&m.ID, &m.Screenname, &m.ContentType, &m.Size, &m.WhenCreated)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("media not found")
	}
	if err != nil {
		return nil, fmt.Errorf("query media info: %w", err)
	}
	return &m, nil
}

// DeleteMedia removes a media file
func DeleteMedia(db *sql.DB, mediaID int64) error {
	result, err := db.Exec(`DELETE FROM media WHERE id = ?`, mediaID)
	if err != nil {
		return fmt.Errorf("delete media: %w", err)
	}

	ct, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}

	if ct == 0 {
		return fmt.Errorf("media not found")
	}

	return nil
}

// GetUserMedia returns all media uploaded by a user
func GetUserMedia(db *sql.DB, screenname string, limit int) ([]*Media, error) {
	rows, err := db.Query(
		`SELECT id, screenname, contentType, size, whenCreated FROM media
		 WHERE screenname = ?
		 ORDER BY whenCreated DESC
		 LIMIT ?`,
		screenname, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query user media: %w", err)
	}
	defer rows.Close()

	var media []*Media
	for rows.Next() {
		var m Media
		if err := rows.Scan(&m.ID, &m.Screenname, &m.ContentType, &m.Size, &m.WhenCreated); err != nil {
			return nil, fmt.Errorf("scan media: %w", err)
		}
		media = append(media, &m)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate media rows: %w", err)
	}

	return media, nil
}
