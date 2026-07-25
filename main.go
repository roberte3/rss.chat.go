package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"rss.chat.go/api"
	"rss.chat.go/db"
	"rss.chat.go/feed"
	"rss.chat.go/publish"
	"rss.chat.go/setup"
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

func main() {
	setupFlag := flag.Bool("setup", false, "initialize the database and exit")
	flag.Parse()

	conn, err := db.Open("rss.chat.db")
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer conn.Close()

	if *setupFlag {
		if err := setup.CreateDatabase(conn); err != nil {
			log.Fatalf("setup failed: %v", err)
		}
		if err := setup.CreateSettings("settings.json"); err != nil {
			log.Fatalf("setup failed: %v", err)
		}

		log.Println("setup complete")
		return
	}

	log.Println("connected to database")
	runHttpSvr(conn)
}

func runHttpSvr(conn *sql.DB) {
	mux := http.NewServeMux()

	// Create feed configuration
	feedConfig := feed.BuilderConfig{
		BaseURL:      "localhost:8081",
		ProductName:  "rss.chat",
		MaxFeedItems: 100,
		Language:     "en",
		DocsURL:      "http://www.rssboard.org/rss-specification",
	}

	// Initialize publisher
	if err := os.MkdirAll("feeds", 0755); err != nil {
		log.Fatalf("failed to create feeds directory: %v", err)
	}
	pub := publish.NewPublisher("feeds", feedConfig)
	if err := pub.EnsureDir(); err != nil {
		log.Fatalf("failed to ensure feed directories: %v", err)
	}

	// Create API handler
	handler := api.NewHandler(conn, pub, feedConfig)
	handler.RegisterRoutes(mux)

	fmt.Println("Server is running on :8081...")
	if err := http.ListenAndServe(":8081", mux); err != nil {
		panic(err)
	}
}
