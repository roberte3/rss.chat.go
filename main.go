package main

import (
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
)


func main() {
	setupFlag := flag.Bool("setup", false, "initialize the database and exit")
	configPath := flag.String("config", "config.json", "path to config file")
	flag.Parse()

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
		if err := setup.CreateSettings("settings.json"); err != nil {
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

	// Create feed configuration from app config
	feedConfig := feed.BuilderConfig{
		BaseURL:         cfg.MyDomain,
		ProductName:     cfg.ProductName,
		MaxFeedItems:    100,
		Language:        "en",
		DocsURL:         "http://www.rssboard.org/rss-specification",
		RSSCloudEnabled: false, // TODO: enable if rssCloud ping is implemented
	}

	// Initialize publisher
	if err := os.MkdirAll(cfg.FeedsPath, 0755); err != nil {
		log.Fatalf("failed to create feeds directory: %v", err)
	}
	pub := publish.NewPublisher(cfg.FeedsPath, feedConfig)
	if err := pub.EnsureDir(); err != nil {
		log.Fatalf("failed to ensure feed directories: %v", err)
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

	// Create API handler
	handler := api.NewHandler(conn, pub, feedConfig)
	handler.RegisterRoutes(mux)

	// Register client server (serves at root, must be last)
	clientConfig := client.Config{
		ProductName:              cfg.ProductName,
		ProductNameForDisplay:    cfg.ProductNameForDisplay,
		Version:                  "1.0",
		EnableLogin:              true,
		URLServerForClient:       cfg.URLServerForClient,
		URLWebsocketServerForClient: cfg.URLWebsocketServerForClient,
		WebsocketEnabled:         cfg.WebsocketEnabled,
	}

	clientServer := client.NewServer("archive/rss.chat/client/code", clientConfig)
	mux.Handle("/", clientServer)

	fmt.Println("Server is running on :" + cfg.MyDomain)
	fmt.Println("API Base:", cfg.URLServerForClient)
	if err := http.ListenAndServe(":8081", mux); err != nil {
		panic(err)
	}
}
