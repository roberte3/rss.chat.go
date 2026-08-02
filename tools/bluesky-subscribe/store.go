package main

import (
	"database/sql"
	"time"
)

// stateSchema creates the tool's local sqlite state. It doubles as a dedup
// index (atUri is Bluesky's own natural key) and a durable cache of what was
// pulled from Bluesky, per notes/feature-bluesky-bridge-utility.md.
const stateSchema = `
CREATE TABLE IF NOT EXISTS tracked_handles (
	handle          TEXT PRIMARY KEY,
	did             TEXT NOT NULL DEFAULT '',
	rssScreenname   TEXT NOT NULL UNIQUE,
	rssEmail        TEXT NOT NULL DEFAULT '',
	rssEmailSecret  TEXT NOT NULL DEFAULT '',
	avatarUrl       TEXT NOT NULL DEFAULT '',
	lastBackfillAt  DATETIME,
	createdAt       DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS bluesky_items (
	atUri           TEXT PRIMARY KEY,
	handle          TEXT NOT NULL REFERENCES tracked_handles(handle),
	did             TEXT NOT NULL,
	rkey            TEXT NOT NULL,
	cid             TEXT NOT NULL,
	createdAt       DATETIME NOT NULL,
	text            TEXT NOT NULL,
	rawJson         TEXT NOT NULL,
	fetchedAt       DATETIME DEFAULT CURRENT_TIMESTAMP,
	forwardedAt     DATETIME,
	rssItemId       INTEGER,
	forwardAttempts INTEGER NOT NULL DEFAULT 0,
	forwardError    TEXT
);
`

func openState(path string) (*sql.DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := conn.Exec(stateSchema); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

type trackedHandle struct {
	Handle         string
	DID            string
	RSSScreenname  string
	RSSEmail       string
	RSSEmailSecret string
	AvatarURL      string
	LastBackfillAt sql.NullTime
}

// getTrackedHandle returns (nil, nil) when handle isn't tracked yet.
func getTrackedHandle(conn *sql.DB, handle string) (*trackedHandle, error) {
	row := conn.QueryRow(`select handle, did, rssScreenname, rssEmail, rssEmailSecret, avatarUrl, lastBackfillAt
		from tracked_handles where handle = ?`, handle)
	var t trackedHandle
	err := row.Scan(&t.Handle, &t.DID, &t.RSSScreenname, &t.RSSEmail, &t.RSSEmailSecret, &t.AvatarURL, &t.LastBackfillAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func insertTrackedHandle(conn *sql.DB, t trackedHandle) error {
	_, err := conn.Exec(`insert into tracked_handles (handle, did, rssScreenname, rssEmail, rssEmailSecret)
		values (?, ?, ?, ?, ?)`,
		t.Handle, t.DID, t.RSSScreenname, t.RSSEmail, t.RSSEmailSecret)
	return err
}

func updateHandleDID(conn *sql.DB, handle, did string) error {
	_, err := conn.Exec(`update tracked_handles set did = ? where handle = ?`, did, handle)
	return err
}

// updateHandleAvatarURL records the avatar URL last synced to rss.chat for
// handle, so a rerun can skip the getuserdata/saveprefs round trip when
// Bluesky's avatar hasn't changed.
func updateHandleAvatarURL(conn *sql.DB, handle, avatarURL string) error {
	_, err := conn.Exec(`update tracked_handles set avatarUrl = ? where handle = ?`, avatarURL, handle)
	return err
}

func updateLastBackfillAt(conn *sql.DB, handle string, when time.Time) error {
	_, err := conn.Exec(`update tracked_handles set lastBackfillAt = ? where handle = ?`, when, handle)
	return err
}

// insertItemIfNew reports whether the item was new. atUri is Bluesky's own
// key, so a rerun over an overlapping window is naturally a no-op rather than
// needing a separate seen-set.
func insertItemIfNew(conn *sql.DB, item blueskyItem) (bool, error) {
	res, err := conn.Exec(`insert or ignore into bluesky_items
		(atUri, handle, did, rkey, cid, createdAt, text, rawJson)
		values (?, ?, ?, ?, ?, ?, ?, ?)`,
		item.AtURI, item.Handle, item.DID, item.Rkey, item.CID, item.CreatedAt, item.Text, item.RawJSON)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

type pendingItem struct {
	AtURI  string
	Handle string
	Rkey   string
	Text   string
}

func pendingItems(conn *sql.DB) ([]pendingItem, error) {
	rows, err := conn.Query(`select atUri, handle, rkey, text from bluesky_items
		where forwardedAt is null order by createdAt`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []pendingItem
	for rows.Next() {
		var p pendingItem
		if err := rows.Scan(&p.AtURI, &p.Handle, &p.Rkey, &p.Text); err != nil {
			return nil, err
		}
		items = append(items, p)
	}
	return items, rows.Err()
}

func markForwarded(conn *sql.DB, atURI string, rssItemID int64) error {
	_, err := conn.Exec(`update bluesky_items set forwardedAt = current_timestamp, rssItemId = ?
		where atUri = ?`, rssItemID, atURI)
	return err
}

func markForwardFailed(conn *sql.DB, atURI string, forwardErr error) error {
	_, err := conn.Exec(`update bluesky_items set forwardAttempts = forwardAttempts + 1, forwardError = ?
		where atUri = ?`, forwardErr.Error(), atURI)
	return err
}
