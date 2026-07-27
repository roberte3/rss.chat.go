package setup

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"rss.chat.go/config"
	"rss.chat.go/db"
)

type Settings struct {
	Note        string `json:"note"`
	ProductName string `json:"productName"`
}

type Blocklist struct {
	Note          string   `json:"note"`
	BlockedEmails []string `json:"blockedEmails"`
}

func CreateDatabase(conn *sql.DB) error {
	fmt.Printf("CreateDatabase...\n")
	if err := db.CreateTables(conn); err != nil {
		return fmt.Errorf("setup failed: %w", err)
	}
	return nil
}

func CreateSettings(path string, productName string) error {
	fmt.Printf("Create Settings files...\n")
	return generateSettings(path, productName)
}

func CreateBlocklist(path string) error {
	fmt.Printf("Create Blocklist file...\n")
	return generateBlocklist(path)
}

// CreateConfig generates a config.json file interactively if it doesn't exist.
// If config.json already exists, skips creation (idempotent).
// Reads prompts from the provided io.Reader (os.Stdin for interactive use, or a string reader for tests).
func CreateConfig(path string, in io.Reader) error {
	// Check if config already exists (idempotent)
	if _, err := os.Stat(path); err == nil {
		fmt.Printf("Config file already exists at %s, skipping creation\n", path)
		return nil
	}

	fmt.Printf("Creating configuration file at %s...\n", path)
	fmt.Printf("Press Enter to accept defaults (shown in [brackets])\n\n")

	reader := bufio.NewReader(in)

	// 1. Product name for display
	fmt.Printf("Product name for display [rss.chat]: ")
	productDisplay, _ := reader.ReadString('\n')
	productDisplay = strings.TrimSpace(productDisplay)
	if productDisplay == "" {
		productDisplay = "rss.chat"
	}

	// 2. My domain
	fmt.Printf("My domain (e.g. http://localhost:8081) [http://localhost:8081]: ")
	myDomain, _ := reader.ReadString('\n')
	myDomain = strings.TrimSpace(myDomain)
	if myDomain == "" {
		myDomain = "http://localhost:8081"
	}

	// 3. Mail sender address
	fmt.Printf("Mail sender address (from email) [admin@localhost]: ")
	mailSender, _ := reader.ReadString('\n')
	mailSender = strings.TrimSpace(mailSender)
	if mailSender == "" {
		mailSender = "admin@localhost"
	}

	// 4. Enable email sending
	fmt.Printf("\nEnable email sending for confirmation links? (y/n) [y]: ")
	emailEnabledStr, _ := reader.ReadString('\n')
	emailEnabledStr = strings.TrimSpace(emailEnabledStr)
	if emailEnabledStr == "" {
		emailEnabledStr = "y"
	}
	emailEnabled := strings.ToLower(emailEnabledStr) == "y"

	var smtpHost string
	var smtpPort int
	var smtpUsername string
	var smtpPassword string

	if emailEnabled {
		fmt.Printf("\n--- Email Configuration (SMTP) ---\n")
		fmt.Printf("For Gmail: Use smtp.gmail.com:587 and an App Password (not your regular password)\n")
		fmt.Printf("  Get App Password: Google Account > Security > App passwords\n\n")

		// 5. SMTP host
		fmt.Printf("SMTP host [smtp.gmail.com]: ")
		smtpHostInput, _ := reader.ReadString('\n')
		smtpHost = strings.TrimSpace(smtpHostInput)
		if smtpHost == "" {
			smtpHost = "smtp.gmail.com"
		}

		// 6. SMTP port
		fmt.Printf("SMTP port [587]: ")
		smtpPortStr, _ := reader.ReadString('\n')
		smtpPortStr = strings.TrimSpace(smtpPortStr)
		if smtpPortStr == "" {
			smtpPortStr = "587"
		}
		if _, err := fmt.Sscanf(smtpPortStr, "%d", &smtpPort); err != nil {
			smtpPort = 587
		}

		// 7. SMTP username (default to mail sender if blank)
		fmt.Printf("SMTP username [%s]: ", mailSender)
		smtpUsernameInput, _ := reader.ReadString('\n')
		smtpUsername = strings.TrimSpace(smtpUsernameInput)
		if smtpUsername == "" {
			smtpUsername = mailSender
		}

		// 8. SMTP password
		fmt.Printf("SMTP password (for Gmail, use App Password) []: ")
		smtpPasswordInput, _ := reader.ReadString('\n')
		smtpPassword = strings.TrimSpace(smtpPasswordInput)
	}

	// 9. Enable WebSocket
	fmt.Printf("\nEnable WebSocket for real-time updates? (y/n) [y]: ")
	wsEnabledStr, _ := reader.ReadString('\n')
	wsEnabledStr = strings.TrimSpace(wsEnabledStr)
	if wsEnabledStr == "" {
		wsEnabledStr = "y"
	}
	websocketEnabled := strings.ToLower(wsEnabledStr) == "y"

	// Derive URLServerForClient and URLServerForEmail
	urlServerForClient := myDomain + "/api"
	urlServerForEmail := myDomain

	// Derive WebSocket URL if enabled
	urlWebsocketServerForClient := ""
	if websocketEnabled {
		urlWebsocketServerForClient = deriveWebsocketURL(myDomain, 1462)
	}

	// Build the config
	cfg := config.Config{
		ProductNameForDisplay:       productDisplay,
		MyDomain:                    myDomain,
		URLServerForClient:          urlServerForClient,
		URLServerForEmail:           urlServerForEmail,
		MailSender:                  mailSender,
		SMTPHost:                    smtpHost,
		SMTPPort:                    smtpPort,
		SMTPUsername:                smtpUsername,
		SMTPPassword:                smtpPassword,
		WebsocketEnabled:            websocketEnabled,
		URLWebsocketServerForClient: urlWebsocketServerForClient,
	}

	// Apply defaults for optional fields
	cfg.ApplyDefaults()

	// Validate before writing
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	// Marshal to JSON with nice formatting
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config to JSON: %w", err)
	}

	// Write to file
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}

	fmt.Printf("Configuration file created successfully\n")
	return nil
}

// deriveWebsocketURL converts http/https domain to ws/wss with the given port.
func deriveWebsocketURL(myDomain string, port int) string {
	u, err := url.Parse(myDomain)
	if err != nil {
		// Fallback if parsing fails
		return fmt.Sprintf("ws://localhost:%d", port)
	}

	scheme := "ws"
	if u.Scheme == "https" {
		scheme = "wss"
	}

	// Extract just the host (without port if present)
	host := u.Hostname()
	if host == "" {
		host = "localhost"
	}

	return fmt.Sprintf("%s://%s:%d", scheme, host, port)
}

func generateSettings(path string, productName string) error {
	s := Settings{
		Note:        "Example settings file for rss.chat.go. Edit values as needed.",
		ProductName: productName,
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
