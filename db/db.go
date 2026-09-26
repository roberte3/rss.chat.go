// Package db provides the SQLite database connection for the application.
package db

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// Open opens a SQLite database at path, applying sensible default pragmas
// (WAL journaling, foreign keys enforced, busy timeout to reduce lock errors).
func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	pragmas := []string{
		"PRAGMA journal_mode = WAL;",
		"PRAGMA foreign_keys = ON;",
		"PRAGMA busy_timeout = 5000;",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			db.Close()
			return nil, fmt.Errorf("apply pragma %q: %w", p, err)
		}
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	return db, nil
}

// CreateTables creates the initial schema if it doesn't already exist.
func CreateTables(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			screenname TEXT PRIMARY KEY,
			emailAddress TEXT,
			emailSecret TEXT,
			imageUrl TEXT,
			prefs TEXT,
			ctHits INTEGER NOT NULL DEFAULT 0,
			ctHitsToday INTEGER NOT NULL DEFAULT 0,
			whenLastHit DATETIME,
			whenCreated DATETIME DEFAULT CURRENT_TIMESTAMP,
			whenUpdated DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_users_emailAddress ON users (emailAddress);
		CREATE TRIGGER IF NOT EXISTS trg_users_whenUpdated
			AFTER UPDATE ON users FOR EACH ROW
			BEGIN
				UPDATE users SET whenUpdated = CURRENT_TIMESTAMP WHERE screenname = NEW.screenname;
			END;

		CREATE TABLE IF NOT EXISTS items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			feedUrl TEXT,
			author TEXT,
			inReplyTo INTEGER,
			title TEXT,
			link TEXT,
			description TEXT,
			pubDate DATETIME,
			enclosureUrl TEXT,
			enclosureType TEXT,
			enclosureLength INTEGER,
			whenCreated DATETIME DEFAULT CURRENT_TIMESTAMP,
			whenUpdated DATETIME DEFAULT CURRENT_TIMESTAMP,
			markdowntext TEXT,
			outlineJsontext TEXT,
			flDeleted INTEGER NOT NULL DEFAULT 0
		);
		CREATE INDEX IF NOT EXISTS idx_items_feedUrl ON items (feedUrl);
		CREATE INDEX IF NOT EXISTS idx_items_author ON items (author);
		CREATE TRIGGER IF NOT EXISTS trg_items_whenUpdated
			AFTER UPDATE ON items FOR EACH ROW
			BEGIN
				UPDATE items SET whenUpdated = CURRENT_TIMESTAMP WHERE id = NEW.id;
			END;

		CREATE TABLE IF NOT EXISTS likes (
			screenname TEXT NOT NULL,
			itemId INTEGER NOT NULL,
			whenCreated DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (screenname, itemId)
		);
		CREATE INDEX IF NOT EXISTS idx_likes_itemId ON likes (itemId);

		CREATE TABLE IF NOT EXISTS blocklist (
			email TEXT PRIMARY KEY,
			whenAdded DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS mentions (
			itemId INTEGER NOT NULL REFERENCES items(id) ON DELETE CASCADE,
			screenname TEXT NOT NULL REFERENCES users(screenname),
			whenCreated DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (itemId, screenname)
		);
		CREATE INDEX IF NOT EXISTS idx_mentions_screenname ON mentions(screenname);

		CREATE TABLE IF NOT EXISTS hashtags (
			tag TEXT NOT NULL,
			itemId INTEGER NOT NULL REFERENCES items(id) ON DELETE CASCADE,
			PRIMARY KEY (tag, itemId)
		);
		CREATE INDEX IF NOT EXISTS idx_hashtags_itemId ON hashtags(itemId);
	`)
	if err != nil {
		return fmt.Errorf("create tables: %w", err)
	}
	return nil
}
