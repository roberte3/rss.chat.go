package websub

import (
	"io"
	"log"
	"net/http"
	"net/url"
)

// Pinger sends WebSub notifications to a hub when feeds are updated.
type Pinger struct {
	hubURL    string
	client    *http.Client
	enabled   bool
	debugLog  *log.Logger
}

// NewPinger creates a new WebSub pinger.
// hubURL should be the full URL of the WebSub hub (e.g., https://rpc.rsscloud.io/websub).
// enabled controls whether pings are actually sent.
func NewPinger(hubURL string, enabled bool) *Pinger {
	return &Pinger{
		hubURL:   hubURL,
		client:   &http.Client{},
		enabled:  enabled,
		debugLog: log.New(io.Discard, "", 0),
	}
}

// SetDebugLog sets a logger for debug output. Pass nil to disable.
func (p *Pinger) SetDebugLog(logger *log.Logger) {
	if logger == nil {
		p.debugLog = log.New(io.Discard, "", 0)
	} else {
		p.debugLog = logger
	}
}

// Ping notifies the WebSub hub that a feed has been updated.
// feedURL should be the full URL of the feed (e.g., https://example.com/feed?screenname=alice).
// This call does not block on hub response; failures are logged but do not error.
func (p *Pinger) Ping(feedURL string) {
	if !p.enabled {
		return
	}

	if p.hubURL == "" || feedURL == "" {
		p.debugLog.Printf("websub: skipping ping (hub=%q, feed=%q)", p.hubURL, feedURL)
		return
	}

	go p.pingAsync(feedURL)
}

// pingAsync performs the actual HTTP POST to the hub in the background.
func (p *Pinger) pingAsync(feedURL string) {
	data := url.Values{}
	data.Set("hub.mode", "publish")
	data.Set("hub.url", feedURL)

	resp, err := p.client.PostForm(p.hubURL, data)
	if err != nil {
		p.debugLog.Printf("websub: ping failed for %s: %v", feedURL, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		p.debugLog.Printf("websub: ping returned %d for %s: %s", resp.StatusCode, feedURL, string(body))
		return
	}

	p.debugLog.Printf("websub: pinged hub for %s (status %d)", feedURL, resp.StatusCode)
}

// IsEnabled returns whether WebSub pinging is enabled.
func (p *Pinger) IsEnabled() bool {
	return p.enabled
}
