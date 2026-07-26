package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"rss.chat.go/api"
	"rss.chat.go/client"
	"rss.chat.go/config"
	"rss.chat.go/db"
	"rss.chat.go/email"
	"rss.chat.go/feed"
	"rss.chat.go/publish"
	"rss.chat.go/setup"
	"rss.chat.go/websocket"
)


func main() {
	setupFlag := flag.Bool("setup", false, "initialize the database and exit")
	configPath := flag.String("config", "config.json", "path to config file")
	flag.Parse()

	// If setup flag is set, create config file first (if missing)
	if *setupFlag {
		if err := setup.CreateConfig(*configPath, os.Stdin); err != nil {
			log.Fatalf("setup failed: %v", err)
		}
	}

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Open database
	conn, err := db.Open(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer conn.Close()

	if *setupFlag {
		if err := setup.CreateDatabase(conn); err != nil {
			log.Fatalf("setup failed: %v", err)
		}
		if err := setup.CreateSettings("settings.json", cfg.ProductName); err != nil {
			log.Fatalf("setup failed: %v", err)
		}
		if err := setup.CreateBlocklist(cfg.BlocklistPath); err != nil {
			log.Fatalf("setup failed: %v", err)
		}

		log.Println("setup complete")
		return
	}

	log.Println("connected to database")
	log.Println("server:", cfg.ProductNameForDisplay)
	log.Println("domain:", cfg.MyDomain)

	runHttpSvr(conn, cfg)
}

func runHttpSvr(conn *sql.DB, cfg *config.Config) {
	mux := http.NewServeMux()
	ctx := context.Background()

	// Create feed configuration from app config
	feedConfig := feed.BuilderConfig{
		BaseURL:                  cfg.MyDomain,
		ProductName:              cfg.ProductName,
		MaxFeedItems:             100,
		Language:                 "en",
		DocsURL:                  "http://www.rssboard.org/rss-specification",
		RSSCloudEnabled:          false, // TODO: enable if rssCloud ping is implemented
		TitleForSubscriptionList: cfg.TitleForSubscriptionList,
	}

	// Initialize media database
	mediaDB, err := db.OpenMediaDB(cfg.MediaDBPath)
	if err != nil {
		log.Fatalf("failed to open media database: %v", err)
	}
	defer mediaDB.Close()

	// Initialize feeds database (if database-driven feeds are enabled)
	var feedsDB *sql.DB
	if cfg.FeedsInDatabase {
		feedsDB, err = db.OpenFeedsDB(cfg.FeedsDBPath)
		if err != nil {
			log.Fatalf("failed to open feeds database: %v", err)
		}
		defer feedsDB.Close()
	}

	// Initialize publisher
	if !cfg.FeedsInDatabase {
		if err := os.MkdirAll(cfg.FeedsPath, 0755); err != nil {
			log.Fatalf("failed to create feeds directory: %v", err)
		}
	}
	pub := publish.NewPublisher(cfg.FeedsPath, feedConfig)
	if cfg.FeedsInDatabase && feedsDB != nil {
		pub.SetDatabaseMode(feedsDB)
		if count, err := pub.BackfillMissingFeeds(conn); err != nil {
			log.Fatalf("failed to backfill feeds: %v", err)
		} else {
			log.Printf("backfilled %d feeds in database", count)
		}
	} else if !cfg.FeedsInDatabase {
		if err := pub.EnsureDir(); err != nil {
			log.Fatalf("failed to ensure feed directories: %v", err)
		}
	}

	// Create temporary media directory
	if err := os.MkdirAll(cfg.TempMediaPath, 0755); err != nil {
		log.Fatalf("failed to create temp media directory: %v", err)
	}

	// Create email sender from config
	emailSender := email.NewSender(email.Config{
		SMTPHost:     cfg.SMTPHost,
		SMTPPort:     cfg.SMTPPort,
		SMTPUsername: cfg.SMTPUsername,
		SMTPPassword: cfg.SMTPPassword,
		FromAddress:  cfg.MailSender,
		FromName:     cfg.ProductNameForDisplay,
		Provider:     "smtp",
	})
	_ = emailSender // TODO: wire into API endpoints

	// Create and start websocket hub
	wsHub := websocket.NewHub()
	go wsHub.Start(ctx)

	// Create API handler with websocket hub and media database
	handler := api.NewHandler(conn, pub, feedConfig)
	handler.SetWebsocketHub(wsHub)
	handler.Config = cfg // Wire config for auth checks
	handler.MediaDB = mediaDB
	handler.FeedsDB = feedsDB // Set feeds database if available
	handler.MaxMediaUploadBytes = cfg.MaxMediaUploadBytes
	handler.TempMediaPath = cfg.TempMediaPath
	handler.RobotsContent = cfg.RobotsTxt
	handler.RegisterRoutes(mux)

	// Register websocket endpoint
	wsHandler := websocket.NewHandler(wsHub)
	mux.Handle("/subscribe", wsHandler)

	// Register client server (serves at root, must be last)
	clientConfig := client.Config{
		ProductName:              cfg.ProductName,
		ProductNameForDisplay:    cfg.ProductNameForDisplay,
		Version:                  "1.0",
		EnableLogin:              true,
		URLServerForClient:       cfg.URLServerForClient,
		URLWebsocketServerForClient: cfg.URLWebsocketServerForClient,
		WebsocketEnabled:         cfg.WebsocketEnabled,
		FeedURLEveryone:          fmt.Sprintf("%s/feed", cfg.MyDomain),
	}

	clientServer := client.NewServer("archive/rss.chat/client/code", clientConfig)
	mux.Handle("/", clientServer)

	fmt.Printf("Server is running on http://localhost:%d\n", cfg.HTTPPort)
	fmt.Println("API Base:", cfg.URLServerForClient)
	if cfg.WebsocketEnabled {
		fmt.Println("WebSocket enabled at:", cfg.URLWebsocketServerForClient)
	}
	addr := fmt.Sprintf(":%d", cfg.HTTPPort)
	if err := http.ListenAndServe(addr, mux); err != nil {
		panic(err)
	}
}
