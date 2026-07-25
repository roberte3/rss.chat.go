package main

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	_ "modernc.org/sqlite"
)

// BackupData represents the complete database state
type BackupData struct {
	ExportedAt string          `json:"exportedAt"`
	Users      []UserData      `json:"users"`
	Items      []ItemData      `json:"items"`
	Likes      []LikeData      `json:"likes"`
	Media      []MediaData     `json:"media"`
}

// UserData represents a user in the backup
type UserData struct {
	Screenname   string `json:"screenname"`
	EmailAddress string `json:"emailAddress"`
	EmailSecret  string `json:"emailSecret"`
	ImageURL     string `json:"imageUrl,omitempty"`
	Prefs        string `json:"prefs,omitempty"`
	WhenCreated  string `json:"whenCreated"`
	WhenUpdated  string `json:"whenUpdated"`
}

// ItemData represents a post/item in the backup
type ItemData struct {
	ID          int64  `json:"id"`
	Screenname  string `json:"screenname"`
	Text        string `json:"text"`
	GuidHash    string `json:"guidHash"`
	IDParent    int64  `json:"idParent"`
	WhenCreated string `json:"whenCreated"`
	WhenUpdated string `json:"whenUpdated"`
}

// LikeData represents a like in the backup
type LikeData struct {
	ID         int64  `json:"id"`
	ItemID     int64  `json:"itemId"`
	Screenname string `json:"screenname"`
	WhenCreated string `json:"whenCreated"`
}

// MediaData represents media in the backup with base64-encoded data
type MediaData struct {
	ID          int64  `json:"id"`
	Screenname  string `json:"screenname"`
	ContentType string `json:"contentType"`
	Data        string `json:"data"` // Base64-encoded
	Size        int64  `json:"size"`
	WhenCreated string `json:"whenCreated"`
}

func main() {
	dbPath := flag.String("db", "rss.chat.db", "path to main database")
	mediaDBPath := flag.String("mediadb", "rss.chat.media.db", "path to media database")
	output := flag.String("o", "backup.json", "output file path")
	flag.Parse()

	// Open main database
	mainDB, err := sql.Open("sqlite", *dbPath)
	if err != nil {
		log.Fatalf("Failed to open main database: %v", err)
	}
	defer mainDB.Close()

	// Open media database
	mediaDB, err := sql.Open("sqlite", *mediaDBPath)
	if err != nil {
		log.Fatalf("Failed to open media database: %v", err)
	}
	defer mediaDB.Close()

	// Export data
	fmt.Println("Exporting users...")
	users, err := exportUsers(mainDB)
	if err != nil {
		log.Fatalf("Failed to export users: %v", err)
	}
	fmt.Printf("  Exported %d users\n", len(users))

	fmt.Println("Exporting items...")
	items, err := exportItems(mainDB)
	if err != nil {
		log.Fatalf("Failed to export items: %v", err)
	}
	fmt.Printf("  Exported %d items\n", len(items))

	fmt.Println("Exporting likes...")
	likes, err := exportLikes(mainDB)
	if err != nil {
		log.Fatalf("Failed to export likes: %v", err)
	}
	fmt.Printf("  Exported %d likes\n", len(likes))

	fmt.Println("Exporting media...")
	media, err := exportMedia(mediaDB)
	if err != nil {
		log.Fatalf("Failed to export media: %v", err)
	}
	fmt.Printf("  Exported %d media files\n", len(media))

	// Create backup data
	backup := BackupData{
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
		Users:      users,
		Items:      items,
		Likes:      likes,
		Media:      media,
	}

	// Write to file
	fmt.Printf("\nWriting backup to %s...\n", *output)
	data, err := json.MarshalIndent(backup, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal backup data: %v", err)
	}

	if err := os.WriteFile(*output, data, 0600); err != nil {
		log.Fatalf("Failed to write backup file: %v", err)
	}

	fmt.Printf("Backup complete! File size: %d bytes\n", len(data))
}

func exportUsers(db *sql.DB) ([]UserData, error) {
	rows, err := db.Query(`
		SELECT screenname, emailAddress, emailSecret, imageUrl, prefs, whenCreated, whenUpdated
		FROM users
		ORDER BY screenname
	`)
	if err != nil {
		return nil, fmt.Errorf("query users: %w", err)
	}
	defer rows.Close()

	var users []UserData
	for rows.Next() {
		var u UserData
		var imageURL, prefs sql.NullString
		if err := rows.Scan(&u.Screenname, &u.EmailAddress, &u.EmailSecret, &imageURL, &prefs, &u.WhenCreated, &u.WhenUpdated); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		if imageURL.Valid {
			u.ImageURL = imageURL.String
		}
		if prefs.Valid {
			u.Prefs = prefs.String
		}
		users = append(users, u)
	}

	return users, rows.Err()
}

func exportItems(db *sql.DB) ([]ItemData, error) {
	rows, err := db.Query(`
		SELECT id, screenname, text, guidHash, idParent, whenCreated, whenUpdated
		FROM items
		ORDER BY id
	`)
	if err != nil {
		return nil, fmt.Errorf("query items: %w", err)
	}
	defer rows.Close()

	var items []ItemData
	for rows.Next() {
		var i ItemData
		if err := rows.Scan(&i.ID, &i.Screenname, &i.Text, &i.GuidHash, &i.IDParent, &i.WhenCreated, &i.WhenUpdated); err != nil {
			return nil, fmt.Errorf("scan item: %w", err)
		}
		items = append(items, i)
	}

	return items, rows.Err()
}

func exportLikes(db *sql.DB) ([]LikeData, error) {
	rows, err := db.Query(`
		SELECT id, itemId, screenname, whenCreated
		FROM likes
		ORDER BY id
	`)
	if err != nil {
		return nil, fmt.Errorf("query likes: %w", err)
	}
	defer rows.Close()

	var likes []LikeData
	for rows.Next() {
		var l LikeData
		if err := rows.Scan(&l.ID, &l.ItemID, &l.Screenname, &l.WhenCreated); err != nil {
			return nil, fmt.Errorf("scan like: %w", err)
		}
		likes = append(likes, l)
	}

	return likes, rows.Err()
}

func exportMedia(db *sql.DB) ([]MediaData, error) {
	rows, err := db.Query(`
		SELECT id, screenname, contentType, mediabytes, size, whenCreated
		FROM media
		ORDER BY id
	`)
	if err != nil {
		return nil, fmt.Errorf("query media: %w", err)
	}
	defer rows.Close()

	var media []MediaData
	for rows.Next() {
		var m MediaData
		var data []byte
		if err := rows.Scan(&m.ID, &m.Screenname, &m.ContentType, &data, &m.Size, &m.WhenCreated); err != nil {
			return nil, fmt.Errorf("scan media: %w", err)
		}
		m.Data = base64.StdEncoding.EncodeToString(data)
		media = append(media, m)
	}

	return media, rows.Err()
}
