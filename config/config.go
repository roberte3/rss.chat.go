package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Config holds all application configuration.
type Config struct {
	// Required settings
	ProductName           string `json:"productName"`
	ProductNameForDisplay string `json:"productNameForDisplay"`
	MyDomain              string `json:"myDomain"`
	URLServerForClient    string `json:"urlServerForClient"`
	URLServerForEmail     string `json:"urlServerForEmail"`

	// Database (SQLite in our implementation, not MySQL)
	DatabasePath string `json:"databasePath"` // Path to SQLite file

	// Feed publishing (filesystem in our implementation, not S3)
	FeedsPath            string `json:"feedsPath"`            // Local folder to publish feeds
	SubscriptionListPath string `json:"subscriptionListPath"` // Path for subs.opml

	// Email configuration
	MailSender          string `json:"mailSender"`
	SMTPHost            string `json:"smtpHost"`
	SMTPPort            int    `json:"smtpPort"`
	SMTPUsername        string `json:"smtpUsername"`
	SMTPPassword        string `json:"smtpPassword"`
	ConfirmEmailSubject string `json:"confirmEmailSubject"`
	OperationToConfirm  string `json:"operationToConfirm"`

	// Optional settings with defaults
	// (ProductName above is already set with sensible default)

	// Server ports
	HTTPPort int `json:"httpPort"` // HTTP server port (default: 8081)

	// WebSocket configuration
	WebsocketEnabled            bool   `json:"flWebsocketEnabled"`
	WebsocketPort               int    `json:"websocketPort"`
	SecureWebsocket             bool   `json:"flSecureWebsocket"`
	URLWebsocketServerForClient string `json:"urlWebsocketServerForClient"`

	// Whitelist/blocklist
	Whitelist        []string `json:"whitelist"`
	BlockedUsersList []string `json:"blockedUsersList"`
	BlocklistPath    string   `json:"blocklistPath"` // Path to separate blocklist.json file

	// Media handling
	MediaDBPath         string `json:"mediaDBPath"`         // Path to separate media database
	TempMediaPath       string `json:"tempMediaPath"`       // Temporary storage for uploads
	MaxMediaUploadBytes int    `json:"maxMediaUploadBytes"` // Max upload size in bytes

	// Feed storage
	FeedsInDatabase bool   `json:"flFeedsInDatabase"` // Store feeds in database (default: false)
	FeedsDBPath     string `json:"feedsDBPath"`       // Path to feeds database

	// WebSub (Web Push) configuration
	WebsubEnabled bool   `json:"flWebsubEnabled"` // Enable WebSub protocol support (default: false)
	URLWebsubHub  string `json:"urlWebsubHub"`    // WebSub hub URL to ping (e.g., https://rpc.rsscloud.io/websub)

	// Post cleanup
	RemoveBlanksAtEnd        bool   `json:"flRemoveBlanksAtEnd"`      // Strip trailing empty paragraphs (default: false)
	TitleForSubscriptionList string `json:"titleForSubscriptionList"` // Custom OPML title

	// Client menus
	// URLMenuOpml points at an OPML outline the client turns into extra
	// menubar menus. Empty — the default — means the client shows no Scripts
	// menu, which is how upstream signals the feature is off.
	URLMenuOpml string `json:"urlMenuOpml"`

	// SEO and robots
	RobotsTxt string `json:"robotsTxt"` // Content for robots.txt file

	// Optional metadata
	Note string `json:"note"`
}

// Load reads and parses a config.json file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	// Apply defaults
	cfg.applyDefaults()

	// Validate required fields
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// applyDefaults sets sensible defaults for optional fields.
func (c *Config) applyDefaults() {
	if c.ProductName == "" {
		c.ProductName = "rssChat"
	}

	if c.ProductNameForDisplay == "" {
		c.ProductNameForDisplay = c.MyDomain
	}

	if c.DatabasePath == "" {
		c.DatabasePath = "rss.chat.db"
	}

	if c.FeedsPath == "" {
		c.FeedsPath = "feeds"
	}

	if c.SubscriptionListPath == "" {
		c.SubscriptionListPath = "feeds/subs.opml"
	}

	if c.SMTPPort == 0 {
		c.SMTPPort = 587
	}

	if c.ConfirmEmailSubject == "" {
		c.ConfirmEmailSubject = c.ProductNameForDisplay + " confirmation"
	}

	if c.OperationToConfirm == "" {
		c.OperationToConfirm = "sign in to " + c.ProductNameForDisplay
	}

	if c.HTTPPort == 0 {
		c.HTTPPort = 8081
	}

	if c.WebsocketPort == 0 {
		c.WebsocketPort = 1462
	}

	if c.MediaDBPath == "" {
		c.MediaDBPath = "rss.chat.media.db"
	}

	if c.TempMediaPath == "" {
		c.TempMediaPath = "temp_media"
	}

	if c.MaxMediaUploadBytes == 0 {
		c.MaxMediaUploadBytes = 2 * 1024 * 1024 // 2MB default
	}

	if c.FeedsDBPath == "" {
		c.FeedsDBPath = "rss.chat.feeds.db"
	}

	if c.URLWebsubHub == "" {
		c.URLWebsubHub = "https://rpc.rsscloud.io/websub"
	}

	if c.BlocklistPath == "" {
		c.BlocklistPath = "blocklist.json"
	}

	if c.TitleForSubscriptionList == "" {
		c.TitleForSubscriptionList = fmt.Sprintf("Subscription list for %s running on %s", c.ProductName, c.MyDomain)
	}

	if c.RobotsTxt == "" {
		c.RobotsTxt = `User-agent: *
Disallow: /getitembyguid
Disallow: /getiteminfo
`
	}

	// Normalize URLs to have trailing slashes
	c.URLServerForClient = ensureTrailingSlash(c.URLServerForClient)
	c.URLServerForEmail = ensureTrailingSlash(c.URLServerForEmail)
	if c.URLWebsocketServerForClient != "" {
		c.URLWebsocketServerForClient = ensureTrailingSlash(c.URLWebsocketServerForClient)
	}
}

// validate checks that all required fields are set.
func (c *Config) validate() error {
	required := map[string]string{
		"productNameForDisplay": c.ProductNameForDisplay,
		"myDomain":              c.MyDomain,
		"urlServerForClient":    c.URLServerForClient,
		"urlServerForEmail":     c.URLServerForEmail,
		"mailSender":            c.MailSender,
	}

	for field, value := range required {
		if value == "" {
			return fmt.Errorf("required config field missing: %s", field)
		}
	}

	// If websockets are enabled, validate websocket config
	if c.WebsocketEnabled {
		if c.URLWebsocketServerForClient == "" {
			return fmt.Errorf("urlWebsocketServerForClient required when websockets enabled")
		}
	}

	return nil
}

// ApplyDefaults sets sensible defaults for optional fields.
// Exported for use by setup.go during config generation.
func (c *Config) ApplyDefaults() {
	c.applyDefaults()
}

// Validate checks that all required fields are set.
// Exported for use by setup.go during config generation.
func (c *Config) Validate() error {
	return c.validate()
}

// ensureTrailingSlash adds a trailing slash to a URL if it doesn't have one.
func ensureTrailingSlash(url string) string {
	if url != "" && !strings.HasSuffix(url, "/") {
		url += "/"
	}
	return url
}

// Blocklist represents the structure of blocklist.json
type Blocklist struct {
	Note          string   `json:"note"`
	BlockedEmails []string `json:"blockedEmails"`
}

// LoadBlocklist reads the blocklist from a JSON file.
// Returns an empty list if file doesn't exist.
func LoadBlocklist(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("read blocklist file: %w", err)
	}

	var bl Blocklist
	if err := json.Unmarshal(data, &bl); err != nil {
		return nil, fmt.Errorf("parse blocklist file: %w", err)
	}

	// Normalize emails to lowercase
	normalized := make([]string, 0, len(bl.BlockedEmails))
	for _, email := range bl.BlockedEmails {
		email = strings.ToLower(strings.TrimSpace(email))
		if email != "" {
			normalized = append(normalized, email)
		}
	}

	return normalized, nil
}

// IsEmailWhitelisted checks if an email is on the whitelist.
// Returns true if no whitelist is configured (allow all).
func (c *Config) IsEmailWhitelisted(email string) bool {
	if len(c.Whitelist) == 0 {
		return true // No whitelist means allow all
	}

	email = strings.ToLower(email)
	for _, whitelisted := range c.Whitelist {
		if strings.ToLower(whitelisted) == email {
			return true
		}
	}
	return false
}

// IsEmailBlocked checks if an email is on the blocklist.
func (c *Config) IsEmailBlocked(email string) bool {
	email = strings.ToLower(email)
	for _, blocked := range c.BlockedUsersList {
		if strings.ToLower(blocked) == email {
			return true
		}
	}
	return false
}
