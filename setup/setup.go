package setup

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"

	"rss.chat.go/db"
)

type Settings struct {
	Note        string `json:"note"`
	ProductName string `json:"productName"`
}

type Blocklist struct {
	Note           string   `json:"note"`
	BlockedEmails  []string `json:"blockedEmails"`
}

func CreateDatabase(conn *sql.DB) error {
	fmt.Printf("CreateDatabase...\n")
	if err := db.CreateTables(conn); err != nil {
		return fmt.Errorf("setup failed: %w", err)
	}
	return nil
}

func CreateSettings(path string) error {
	fmt.Printf("Create Settings files...\n")
	return generateSettings(path)
}

func CreateBlocklist(path string) error {
	fmt.Printf("Create Blocklist file...\n")
	return generateBlocklist(path)
}

func generateSettings(path string) error {
	s := Settings{
		Note:        "Example settings file for rss.chat.go. Edit values as needed.",
		ProductName: "rssChat",
	}
	data, err := json.MarshalIndent(s, "", "\t")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func generateBlocklist(path string) error {
	b := Blocklist{
		Note:          "Email addresses to block from creating accounts or signing in. Add email addresses to the blockedEmails array.",
		BlockedEmails: []string{},
	}
	data, err := json.MarshalIndent(b, "", "\t")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
