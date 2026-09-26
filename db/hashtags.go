package db

import (
	"database/sql"
	"fmt"
	"strings"
)

// StoreHashtags deletes any existing hashtags for an item and stores new ones.
// tags should be normalized to lowercase.
func StoreHashtags(db *sql.DB, itemID int, tags []string) error {
	// Delete existing hashtags for this item
	if _, err := db.Exec("DELETE FROM hashtags WHERE itemId = ?", itemID); err != nil {
		return fmt.Errorf("delete existing hashtags: %w", err)
	}

	if len(tags) == 0 {
		return nil
	}

	// Insert new hashtags
	stmt, err := db.Prepare("INSERT INTO hashtags (tag, itemId) VALUES (?, ?)")
	if err != nil {
		return fmt.Errorf("prepare insert hashtags: %w", err)
	}
	defer stmt.Close()

	for _, tag := range tags {
		if _, err := stmt.Exec(tag, itemID); err != nil {
			return fmt.Errorf("insert hashtag %q: %w", tag, err)
		}
	}

	return nil
}

// GetHashtagsForItem returns all tags for a given item (normalized to lowercase).
func GetHashtagsForItem(db *sql.DB, itemID int) ([]string, error) {
	rows, err := db.Query("SELECT tag FROM hashtags WHERE itemId = ?", itemID)
	if err != nil {
		return nil, fmt.Errorf("query hashtags: %w", err)
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, fmt.Errorf("scan hashtag: %w", err)
		}
		tags = append(tags, tag)
	}

	return tags, rows.Err()
}

// GetItemsForHashtag returns items tagged with a specific hashtag.
// Results are ordered by publication date descending.
// ct (continuation token) is used for pagination - if non-empty, only items
// created before this timestamp are returned.
func GetItemsForHashtag(db *sql.DB, tag string, ct string, limit int) ([]Item, error) {
	// Normalize tag to lowercase for case-insensitive search
	tag = strings.ToLower(tag)

	query := `
		SELECT DISTINCT i.id, i.feedUrl, i.author, i.inReplyTo, i.title, i.link,
				i.description, i.pubDate, i.enclosureUrl, i.enclosureType, i.enclosureLength,
				i.whenCreated, i.whenUpdated, i.markdowntext, i.outlineJsontext
		FROM items i
		INNER JOIN hashtags h ON i.id = h.itemId
		WHERE h.tag = ? AND (i.flDeleted IS NULL OR i.flDeleted = 0)
	`
	args := []interface{}{tag}

	if ct != "" {
		query += " AND i.whenCreated < ?"
		args = append(args, ct)
	}

	query += " ORDER BY i.whenCreated DESC"

	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query items for hashtag: %w", err)
	}
	defer rows.Close()

	var items []Item
	for rows.Next() {
		var item Item
		if err := rows.Scan(&item.ID, &item.FeedURL, &item.Author, &item.InReplyToNum,
			&item.Title, &item.Link, &item.Description, &item.PubDate,
			&item.EnclosureURL, &item.EnclosureType, &item.EnclosureLength,
			&item.WhenCreated, &item.WhenUpdated, &item.MarkdownText, &item.OutlineJSONText); err != nil {
			return nil, fmt.Errorf("scan item: %w", err)
		}
		items = append(items, item)
	}

	return items, rows.Err()
}

// GetTrendingHashtags returns the most frequently used hashtags in the last N days.
// Results are ordered by count descending, then by tag alphabetically.
func GetTrendingHashtags(db *sql.DB, daysSince int, limit int) ([]struct {
	Tag   string
	Count int
}, error) {
	query := `
		SELECT h.tag, COUNT(*) as count
		FROM hashtags h
		INNER JOIN items i ON h.itemId = i.id
		WHERE i.whenCreated > datetime('now', '-' || ? || ' days')
			AND (i.flDeleted IS NULL OR i.flDeleted = 0)
		GROUP BY h.tag
		ORDER BY count DESC, h.tag ASC
	`
	args := []interface{}{daysSince}

	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query trending hashtags: %w", err)
	}
	defer rows.Close()

	var results []struct {
		Tag   string
		Count int
	}
	for rows.Next() {
		var tag string
		var count int
		if err := rows.Scan(&tag, &count); err != nil {
			return nil, fmt.Errorf("scan hashtag: %w", err)
		}
		results = append(results, struct {
			Tag   string
			Count int
		}{tag, count})
	}

	return results, rows.Err()
}
