package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	_ "modernc.org/sqlite"
	"rss.chat.go/db"
)

const (
	testScreenname = "testmonkey"
	testEmail      = "testmonkey@example.com"
	testSecret     = "test-secret-12345"
	numItems       = 10
)

var (
	dbPath = flag.String("db", "rss.chat.db", "path to the database")
)

func main() {
	flag.Parse()

	conn, err := db.Open(*dbPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer conn.Close()

	// Create test user
	err = db.AddUser(conn, testScreenname, testEmail, testSecret)
	if err != nil {
		log.Fatalf("failed to create user: %v", err)
	}
	fmt.Printf("Created user: %s (%s)\n", testScreenname, testEmail)

	// Create 10 test items
	feedURL := fmt.Sprintf("http://localhost:8081/feed?screenname=%s", testScreenname)

	for i := 1; i <= numItems; i++ {
		item := db.NewItem{
			FeedURL:      feedURL,
			Title:        fmt.Sprintf("Test Post #%d", i),
			Link:         "",
			Description:  fmt.Sprintf("<p>This is test post number %d. It contains some sample content to demonstrate the rss.chat.go application.</p><p>You can create, edit, and delete posts. Each post gets a unique GUID and can be referenced by other posts as replies.</p>", i),
			InReplyTo:    nil,
			PubDate:      time.Now().Add(-time.Duration(numItems-i) * time.Hour),
			MarkdownText: fmt.Sprintf("This is test post number %d. It contains some sample content to demonstrate the rss.chat.go application.\n\nYou can create, edit, and delete posts. Each post gets a unique GUID and can be referenced by other posts as replies.", i),
			Author:       testScreenname,
		}

		itemID, err := db.AddItem(conn, item)
		if err != nil {
			log.Fatalf("failed to create item %d: %v", i, err)
		}
		fmt.Printf("  Created post #%d (id=%d, pubDate=%s)\n", i, itemID, item.PubDate.Format(time.RFC3339))
	}

	fmt.Printf("\nSuccess! Created %d test posts for user '%s'\n", numItems, testScreenname)
	fmt.Printf("Visit http://localhost:8081/feed?screenname=%s to view the feed\n", testScreenname)
}
