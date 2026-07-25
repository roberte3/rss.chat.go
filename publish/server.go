package publish

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
)

// ServeFeed serves a feed file (user RSS, everyone RSS, or comments feed).
func (p *Publisher) ServeFeed(w http.ResponseWriter, r *http.Request, feedPath string) error {
	// Prevent directory traversal
	if strings.Contains(feedPath, "..") {
		return fmt.Errorf("invalid feed path")
	}

	path := filepath.Join(p.baseDir, feedPath)

	// Verify the file is within baseDir
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	absBase, err := filepath.Abs(p.baseDir)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(abs, absBase) {
		return fmt.Errorf("feed path outside base directory")
	}

	w.Header().Set("Content-Type", "application/rss+xml")
	http.ServeFile(w, r, path)
	return nil
}

// ServeOPML serves the subscription list.
func (p *Publisher) ServeOPML(w http.ResponseWriter, r *http.Request) error {
	path := filepath.Join(p.baseDir, "subs.opml")

	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	absBase, err := filepath.Abs(p.baseDir)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(abs, absBase) {
		return fmt.Errorf("opml path outside base directory")
	}

	w.Header().Set("Content-Type", "application/xml")
	http.ServeFile(w, r, path)
	return nil
}

// ServeUserFeed serves a user's RSS feed.
// Expects feedPath like "dave/rss.xml"
func (p *Publisher) ServeUserFeed(w http.ResponseWriter, r *http.Request, screenname string) error {
	feedPath := filepath.Join(screenname, "rss.xml")
	return p.ServeFeed(w, r, feedPath)
}

// ServeEveryoneFeed serves the network-wide RSS feed.
func (p *Publisher) ServeEveryoneFeed(w http.ResponseWriter, r *http.Request) error {
	return p.ServeFeed(w, r, "rss.xml")
}

// ServeCommentsFeed serves a comments feed.
// Expects feedPath like "comments/dave-123.xml"
func (p *Publisher) ServeCommentsFeed(w http.ResponseWriter, r *http.Request, screenname string, itemID int64) error {
	filename := fmt.Sprintf("%s-%d.xml", screenname, itemID)
	feedPath := filepath.Join("comments", filename)
	return p.ServeFeed(w, r, feedPath)
}
