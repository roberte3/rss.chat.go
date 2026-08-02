// Command bluesky-subscribe backfills a curated list of Bluesky accounts
// over a bounded time window and forwards their top-level posts into
// rss.chat as real posts, one synthetic rss.chat user per tracked handle.
// See notes/feature-bluesky-bridge-utility.md for the design.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type config struct {
	handlesPath  string
	statePath    string
	rssChatURL   string
	resolver     string
	plcDirectory string
	appview      string
	since        time.Duration
	timeout      time.Duration
	dryRun       bool
}

func main() {
	var cfg config
	flag.StringVar(&cfg.handlesPath, "handles", "handles.json", "path to the JSON list of Bluesky handles to track")
	flag.StringVar(&cfg.statePath, "state", "bluesky-subscribe.db", "path to this tool's local sqlite state (dedup index + item cache)")
	flag.StringVar(&cfg.rssChatURL, "rss-chat-url", "http://localhost:8081", "base URL of the rss.chat server -- /localnewuser requires this tool to run on the same host")
	flag.StringVar(&cfg.resolver, "resolver", "https://bsky.social", "AT Protocol entryway used to resolve handles to DIDs")
	flag.StringVar(&cfg.plcDirectory, "plc-directory", "https://plc.directory", "PLC directory used to resolve did:plc documents")
	flag.StringVar(&cfg.appview, "appview", "https://public.api.bsky.app", "AppView used to fetch profile info (avatar) for tracked handles")
	flag.DurationVar(&cfg.since, "since", 24*time.Hour, "how far back to backfill a handle the first time it's tracked")
	flag.DurationVar(&cfg.timeout, "timeout", 30*time.Second, "per-request HTTP timeout")
	flag.BoolVar(&cfg.dryRun, "dry-run", false, "fetch and log without provisioning users or posting")
	flag.Parse()

	if err := run(cfg); err != nil {
		log.Fatal(err)
	}
}

func run(cfg config) error {
	handles, err := loadHandles(cfg.handlesPath)
	if err != nil {
		return fmt.Errorf("loading handles: %w", err)
	}

	state, err := openState(cfg.statePath)
	if err != nil {
		return fmt.Errorf("opening state db: %w", err)
	}
	defer state.Close()

	client := &http.Client{Timeout: cfg.timeout}
	ctx := context.Background()

	for _, entry := range handles {
		if err := backfillHandle(ctx, client, state, cfg, entry); err != nil {
			// One handle's PDS or DID hiccup shouldn't stop the rest of the
			// list from being processed.
			log.Printf("warning: backfill failed for %s: %v", entry.Handle, err)
			continue
		}
	}

	return forwardPending(ctx, client, state, cfg)
}

// handleEntry is one entry of handles.json. Accepts either a bare handle
// string (use the -since default for its initial backfill) or an object
// with an "initialBackfillDays" override for accounts that need a deeper or
// shallower first pull than the rest of the list -- e.g. a very active
// account you only want a day of, or a quiet one you want a month from.
// The two forms can be mixed freely in the same array.
type handleEntry struct {
	Handle              string
	InitialBackfillDays int // 0 means "use the -since default"
}

func (h *handleEntry) UnmarshalJSON(data []byte) error {
	var handle string
	if err := json.Unmarshal(data, &handle); err == nil {
		h.Handle = handle
		return nil
	}

	var obj struct {
		Handle              string `json:"handle"`
		InitialBackfillDays int    `json:"initialBackfillDays"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	if obj.Handle == "" {
		return fmt.Errorf(`handles.json entry is missing "handle": %s`, data)
	}
	h.Handle = obj.Handle
	h.InitialBackfillDays = obj.InitialBackfillDays
	return nil
}

func loadHandles(path string) ([]handleEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var handles []handleEntry
	if err := json.Unmarshal(data, &handles); err != nil {
		return nil, err
	}
	return handles, nil
}

func backfillHandle(ctx context.Context, client *http.Client, state *sql.DB, cfg config, entry handleEntry) error {
	handle := entry.Handle

	th, err := ensureTrackedHandle(ctx, client, state, cfg, handle)
	if err != nil {
		return err
	}

	// Best-effort: a stale or missing avatar is cosmetic, not worth aborting
	// the rest of this handle's backfill over.
	if err := syncHandleAvatar(ctx, client, state, cfg, th); err != nil {
		log.Printf("warning: avatar sync failed for %s: %v", handle, err)
	}

	if th.DID == "" {
		did, err := resolveHandle(ctx, client, cfg.resolver, handle)
		if err != nil {
			return fmt.Errorf("resolving DID: %w", err)
		}
		if !cfg.dryRun {
			if err := updateHandleDID(state, handle, did); err != nil {
				return err
			}
		}
		th.DID = did
	}

	pds, err := resolvePDSEndpoint(ctx, client, cfg.plcDirectory, th.DID)
	if err != nil {
		return fmt.Errorf("resolving PDS: %w", err)
	}

	// entry.InitialBackfillDays only ever applies before a watermark exists --
	// once a handle has a recorded lastBackfillAt, every run uses that
	// instead, same as the global -since default (see
	// TestBackfillHandleUsesStoredWatermarkNotSinceFlag).
	initialWindow := cfg.since
	if entry.InitialBackfillDays > 0 {
		initialWindow = time.Duration(entry.InitialBackfillDays) * 24 * time.Hour
	}
	since := time.Now().Add(-initialWindow)
	if th.LastBackfillAt.Valid {
		since = th.LastBackfillAt.Time
	}

	now := time.Now()
	items, stats, err := fetchTopLevelPostsSince(ctx, client, pds, handle, th.DID, since)
	if err != nil {
		return fmt.Errorf("fetching posts: %w", err)
	}
	// Printed every run, not just when something looks wrong: "only one post
	// came through" is ambiguous between a narrow window, a reply-heavy
	// account, and silently unparseable records without these counts.
	log.Printf("%s: %d records since %s (%d replies skipped, %d unparseable, %d top-level)",
		handle, stats.RecordsSeen, since.Format(time.RFC3339), stats.RepliesSkipped, stats.MalformedSkipped, len(items))

	for _, item := range items {
		if cfg.dryRun {
			log.Printf("dry-run: would cache %s (%s)", item.AtURI, truncateForLog(item.Text))
			continue
		}
		if _, err := insertItemIfNew(state, item); err != nil {
			return fmt.Errorf("storing %s: %w", item.AtURI, err)
		}
	}

	if !cfg.dryRun {
		if err := updateLastBackfillAt(state, handle, now); err != nil {
			return err
		}
	}

	return nil
}

// syncHandleAvatar fetches th's current Bluesky avatar and, if it differs
// from what was last synced, merges it into the synthetic rss.chat user's
// prefs as myAvatarImageUrl -- the same key feed/builder.go and
// db/items.go already read avatars from. Skipped entirely when Bluesky
// reports no avatar at all; an avatar that's since been removed on Bluesky
// is left stale on rss.chat rather than cleared, matching this tool's
// general no-takebacks-on-delete posture for cross-posted content.
func syncHandleAvatar(ctx context.Context, client *http.Client, state *sql.DB, cfg config, th *trackedHandle) error {
	avatarURL, err := fetchAvatarURL(ctx, client, cfg.appview, th.Handle)
	if err != nil {
		return fmt.Errorf("fetching avatar: %w", err)
	}
	if avatarURL == "" || avatarURL == th.AvatarURL {
		return nil
	}

	if cfg.dryRun {
		log.Printf("dry-run: would set avatar for %s to %s", th.RSSScreenname, avatarURL)
		return nil
	}

	if err := syncAvatar(ctx, client, cfg.rssChatURL, th.RSSScreenname, th.RSSEmail, th.RSSEmailSecret, avatarURL); err != nil {
		return err
	}
	if err := updateHandleAvatarURL(state, th.Handle, avatarURL); err != nil {
		return err
	}
	th.AvatarURL = avatarURL
	return nil
}

// ensureTrackedHandle returns the handle's tracked_handles row, provisioning
// a new synthetic rss.chat user via /localnewuser the first time a handle is
// seen. Idempotent across reruns: an already-tracked handle just reads its
// existing row back.
func ensureTrackedHandle(ctx context.Context, client *http.Client, state *sql.DB, cfg config, handle string) (*trackedHandle, error) {
	existing, err := getTrackedHandle(state, handle)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}

	screenname := screennameForHandle(handle)
	// .invalid is reserved by RFC 2606 for addresses that are never meant to
	// be resolved or delivered to -- this account authenticates entirely via
	// the emailSecret /localnewuser hands back, never via mail.
	email := screenname + "@bsky.invalid"

	if cfg.dryRun {
		log.Printf("dry-run: would provision rss.chat user %s for %s", screenname, handle)
		return &trackedHandle{Handle: handle, RSSScreenname: screenname, RSSEmail: email}, nil
	}

	provisioned, err := provisionLocalUser(ctx, client, cfg.rssChatURL, screenname, email)
	if err != nil {
		return nil, fmt.Errorf("provisioning rss.chat user: %w", err)
	}

	t := trackedHandle{
		Handle:         handle,
		RSSScreenname:  provisioned.Screenname,
		RSSEmail:       provisioned.Email,
		RSSEmailSecret: provisioned.EmailSecret,
	}
	if err := insertTrackedHandle(state, t); err != nil {
		return nil, fmt.Errorf("recording provisioned user: %w", err)
	}
	return &t, nil
}

func forwardPending(ctx context.Context, client *http.Client, state *sql.DB, cfg config) error {
	pending, err := pendingItems(state)
	if err != nil {
		return err
	}

	for _, item := range pending {
		th, err := getTrackedHandle(state, item.Handle)
		if err != nil {
			return err
		}
		if th == nil {
			log.Printf("warning: no tracked handle record for %s, skipping %s", item.Handle, item.AtURI)
			continue
		}

		if cfg.dryRun {
			log.Printf("dry-run: would post %s as %s", item.AtURI, th.RSSScreenname)
			continue
		}

		id, err := forwardPost(ctx, client, cfg.rssChatURL, th.RSSEmail, th.RSSEmailSecret, blueskyItem{
			AtURI:  item.AtURI,
			Handle: item.Handle,
			Rkey:   item.Rkey,
			Text:   item.Text,
		})
		if err != nil {
			log.Printf("warning: failed to forward %s: %v", item.AtURI, err)
			if markErr := markForwardFailed(state, item.AtURI, err); markErr != nil {
				log.Printf("warning: failed to record forward failure for %s: %v", item.AtURI, markErr)
			}
			continue
		}

		if err := markForwarded(state, item.AtURI, id); err != nil {
			log.Printf("warning: failed to mark %s forwarded: %v", item.AtURI, err)
		}
	}

	return nil
}

// screennameForHandle derives a synthetic rss.chat screenname from a
// Bluesky handle's first label (e.g. "wario64.bsky.social" -> "bsky_wario64"),
// sanitized to the character set isValidScreenname in api/auth_endpoints.go
// allows: letters, digits, underscore -- notably no dots or hyphens. Using
// only the first label keeps names readable; two different custom-domain
// handles sharing a first label would collide, an accepted tradeoff for a
// curated, operator-controlled handle list.
func screennameForHandle(handle string) string {
	label := handle
	if idx := strings.Index(handle, "."); idx != -1 {
		label = handle[:idx]
	}

	var b strings.Builder
	b.WriteString("bsky_")
	for _, r := range strings.ToLower(label) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}

	name := b.String()
	if len(name) > 64 {
		name = name[:64]
	}
	return name
}

func truncateForLog(s string) string {
	const max = 60
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
