package db

import (
	"database/sql"
	"fmt"
)

// StoreMentions deletes any existing mentions for an item and stores new ones.
// screennames should be a list of canonical screennames (as stored in database).
func StoreMentions(db *sql.DB, itemID int, screennames []string) error {
	// Delete existing mentions for this item
	if _, err := db.Exec("DELETE FROM mentions WHERE itemId = ?", itemID); err != nil {
		return fmt.Errorf("delete existing mentions: %w", err)
	}

	if len(screennames) == 0 {
		return nil
	}

	// Insert new mentions
	stmt, err := db.Prepare("INSERT INTO mentions (itemId, screenname) VALUES (?, ?)")
	if err != nil {
		return fmt.Errorf("prepare insert mentions: %w", err)
	}
	defer stmt.Close()

	for _, screenname := range screennames {
		if _, err := stmt.Exec(itemID, screenname); err != nil {
			return fmt.Errorf("insert mention for %q: %w", screenname, err)
		}
	}

	return nil
}

// GetMentionsForItem returns all screennames mentioned in an item.
func GetMentionsForItem(db *sql.DB, itemID int) ([]string, error) {
	rows, err := db.Query("SELECT screenname FROM mentions WHERE itemId = ?", itemID)
	if err != nil {
		return nil, fmt.Errorf("query mentions: %w", err)
	}
	defer rows.Close()

	var screennames []string
	for rows.Next() {
		var screenname string
		if err := rows.Scan(&screenname); err != nil {
			return nil, fmt.Errorf("scan mention: %w", err)
		}
		screennames = append(screennames, screenname)
	}

	return screennames, rows.Err()
}

// GetItemsForMention returns items that mention a specific screenname.
// Results are ordered by publication date descending.
// ct (continuation token) is used for pagination - if non-empty, only items
// created before this timestamp are returned.
func GetItemsForMention(db *sql.DB, screenname string, ct string, limit int) ([]Item, error) {
	query := `
		SELECT DISTINCT i.id, i.feedUrl, i.author, i.inReplyTo, i.title, i.link,
				i.description, i.pubDate, i.enclosureUrl, i.enclosureType, i.enclosureLength,
				i.whenCreated, i.whenUpdated, i.markdowntext, i.outlineJsontext
		FROM items i
		INNER JOIN mentions m ON i.id = m.itemId
		WHERE m.screenname = ? AND (i.flDeleted IS NULL OR i.flDeleted = 0)
	`
	args := []interface{}{screenname}

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
		return nil, fmt.Errorf("query items for mention: %w", err)
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
