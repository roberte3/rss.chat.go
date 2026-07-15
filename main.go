package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"

	"rss.chat.go/db"
)

type Settings struct {
	Note        string `json:"note"`
	ProductName string `json:"productName"`
}

func readSettings(path string) (*Settings, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s Settings
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
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

func main() {
	setup := flag.Bool("setup", false, "initialize the database and exit")
	flag.Parse()

	conn, err := db.Open("rss.chat.db")
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer conn.Close()

	if *setup {
		if err := db.CreateTables(conn); err != nil {
			log.Fatalf("setup failed: %v", err)
		}
		log.Println("setup complete")
		return
	}

	log.Println("connected to database")
}
