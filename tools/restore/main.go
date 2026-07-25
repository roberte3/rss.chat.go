package main

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	_ "modernc.org/sqlite"
)

// BackupData represents the complete database state from backup
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
	ID          int64  `json:"id"`
	ItemID      int64  `json:"itemId"`
	Screenname  string `json:"screenname"`
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
	input := flag.String("f", "backup.json", "backup file path")
	dbPath := flag.String("db", "rss.chat.db", "path to main database")
	mediaDBPath := flag.String("mediadb", "rss.chat.media.db", "path to media database")
	force := flag.Bool("force", false, "force restore even if database is not empty")
	flag.Parse()

	// Read backup file
	fmt.Printf("Reading backup from %s...\n", *input)
	data, err := os.ReadFile(*input)
	if err != nil {
		log.Fatalf("Failed to read backup file: %v", err)
	}

	var backup BackupData
	if err := json.Unmarshal(data, &backup); err != nil {
		log.Fatalf("Failed to parse backup file: %v", err)
	}

	fmt.Printf("Backup exported at: %s\n", backup.ExportedAt)
	fmt.Printf("  Users: %d\n", len(backup.Users))
	fmt.Printf("  Items: %d\n", len(backup.Items))
	fmt.Printf("  Likes: %d\n", len(backup.Likes))
	fmt.Printf("  Media: %d\n", len(backup.Media))

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

	// Check if database has data
	if !*force {
		if hasData(mainDB) || hasMediaData(mediaDB) {
			log.Fatal("Database is not empty. Use -force to restore anyway.")
		}
	}

	// Restore data
	fmt.Println("\nRestoring users...")
	if err := restoreUsers(mainDB, backup.Users); err != nil {
		log.Fatalf("Failed to restore users: %v", err)
	}
	fmt.Printf("  Restored %d users\n", len(backup.Users))

	fmt.Println("Restoring items...")
	if err := restoreItems(mainDB, backup.Items); err != nil {
		log.Fatalf("Failed to restore items: %v", err)
	}
	fmt.Printf("  Restored %d items\n", len(backup.Items))

	fmt.Println("Restoring likes...")
	if err := restoreLikes(mainDB, backup.Likes); err != nil {
		log.Fatalf("Failed to restore likes: %v", err)
	}
	fmt.Printf("  Restored %d likes\n", len(backup.Likes))

	fmt.Println("Restoring media...")
	if err := restoreMedia(mediaDB, backup.Media); err != nil {
		log.Fatalf("Failed to restore media: %v", err)
	}
	fmt.Printf("  Restored %d media files\n", len(backup.Media))

	fmt.Println("\nRestore complete!")
}

func hasData(db *sql.DB) bool {
	var count int
	db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if count > 0 {
		return true
	}
	db.QueryRow("SELECT COUNT(*) FROM items").Scan(&count)
	if count > 0 {
		return true
	}
	db.QueryRow("SELECT COUNT(*) FROM likes").Scan(&count)
	if count > 0 {
		return true
	}
	return false
}

func hasMediaData(db *sql.DB) bool {
	var count int
	db.QueryRow("SELECT COUNT(*) FROM media").Scan(&count)
	return count > 0
}

func restoreUsers(db *sql.DB, users []UserData) error {
	stmt, err := db.Prepare(`
		INSERT INTO users (screenname, emailAddress, emailSecret, imageUrl, prefs, whenCreated, whenUpdated)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, u := range users {
		var imageURL, prefs interface{} = nil, nil
		if u.ImageURL != "" {
			imageURL = u.ImageURL
		}
		if u.Prefs != "" {
			prefs = u.Prefs
		}
		if _, err := stmt.Exec(u.Screenname, u.EmailAddress, u.EmailSecret, imageURL, prefs, u.WhenCreated, u.WhenUpdated); err != nil {
			return fmt.Errorf("insert user %s: %w", u.Screenname, err)
		}
	}

	return nil
}

func restoreItems(db *sql.DB, items []ItemData) error {
	stmt, err := db.Prepare(`
		INSERT INTO items (id, screenname, text, guidHash, idParent, whenCreated, whenUpdated)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("prepare statement: %w", err)
	}
	defer stmt.Close()

	// Disable autoincrement to allow explicit ID insertion
	if _, err := db.Exec("PRAGMA foreign_keys = OFF"); err != nil {
		return fmt.Errorf("disable foreign keys: %w", err)
	}
	defer db.Exec("PRAGMA foreign_keys = ON")

	for _, i := range items {
		if _, err := stmt.Exec(i.ID, i.Screenname, i.Text, i.GuidHash, i.IDParent, i.WhenCreated, i.WhenUpdated); err != nil {
			return fmt.Errorf("insert item %d: %w", i.ID, err)
		}
	}

	return nil
}

func restoreLikes(db *sql.DB, likes []LikeData) error {
	stmt, err := db.Prepare(`
		INSERT INTO likes (id, itemId, screenname, whenCreated)
		VALUES (?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("prepare statement: %w", err)
	}
	defer stmt.Close()

	// Disable autoincrement to allow explicit ID insertion
	if _, err := db.Exec("PRAGMA foreign_keys = OFF"); err != nil {
		return fmt.Errorf("disable foreign keys: %w", err)
	}
	defer db.Exec("PRAGMA foreign_keys = ON")

	for _, l := range likes {
		if _, err := stmt.Exec(l.ID, l.ItemID, l.Screenname, l.WhenCreated); err != nil {
			return fmt.Errorf("insert like %d: %w", l.ID, err)
		}
	}

	return nil
}

func restoreMedia(db *sql.DB, media []MediaData) error {
	stmt, err := db.Prepare(`
		INSERT INTO media (id, screenname, contentType, mediabytes, size, whenCreated)
		VALUES (?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("prepare statement: %w", err)
	}
	defer stmt.Close()

	// Disable autoincrement to allow explicit ID insertion
	if _, err := db.Exec("PRAGMA foreign_keys = OFF"); err != nil {
		return fmt.Errorf("disable foreign keys: %w", err)
	}
	defer db.Exec("PRAGMA foreign_keys = ON")

	for _, m := range media {
		// Decode base64 data
		data, err := base64.StdEncoding.DecodeString(m.Data)
		if err != nil {
			return fmt.Errorf("decode media %d: %w", m.ID, err)
		}

		if _, err := stmt.Exec(m.ID, m.Screenname, m.ContentType, data, m.Size, m.WhenCreated); err != nil {
			return fmt.Errorf("insert media %d: %w", m.ID, err)
		}
	}

	return nil
}
