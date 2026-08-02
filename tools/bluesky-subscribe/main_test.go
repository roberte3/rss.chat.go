package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"
)

// validScreenname mirrors isValidScreenname in api/auth_endpoints.go: letters,
// digits, underscore only. That function is unexported, so it can't be
// called directly from here -- if it ever changes, this regex needs to be
// updated to match, or screennameForHandle's output could start failing
// /localnewuser's validation at runtime instead of in a test.
var validScreenname = regexp.MustCompile(`^[a-zA-Z0-9_]{1,64}$`)

func TestScreennameForHandle(t *testing.T) {
	tests := []struct {
		handle string
		want   string
	}{
		{"wario64.bsky.social", "bsky_wario64"},
		{"roberte3-dev.bsky.social", "bsky_roberte3_dev"},
		{"ALLCAPS.bsky.social", "bsky_allcaps"},
		{"no-tld-handle", "bsky_no_tld_handle"},
	}

	for _, tt := range tests {
		t.Run(tt.handle, func(t *testing.T) {
			got := screennameForHandle(tt.handle)
			if got != tt.want {
				t.Errorf("screennameForHandle(%q) = %q, want %q", tt.handle, got, tt.want)
			}
			if !validScreenname.MatchString(got) {
				t.Errorf("screennameForHandle(%q) = %q, does not satisfy isValidScreenname's character set", tt.handle, got)
			}
		})
	}
}

// TestRunEndToEnd wires fake Bluesky identity/PDS servers and a fake
// rss.chat server together and drives a full run(): provision, backfill,
// forward. Confirms the reply in the fixture is skipped and the top-level
// post makes it all the way through to a forwarded, non-pending item.
func TestRunEndToEnd(t *testing.T) {
	now := time.Now().UTC()

	pds := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/xrpc/com.atproto.repo.listRecords" {
			t.Fatalf("unexpected PDS path %s", r.URL.Path)
		}
		records := []map[string]interface{}{
			fakePost("post1", now, "Big sale happening now, link below", false),
			fakePost("reply1", now, "a reply that should be skipped", true),
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"records": records})
	}))
	defer pds.Close()

	plc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/did:plc:wario64" {
			t.Fatalf("unexpected plc path %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"service": []map[string]string{
				{"id": "#atproto_pds", "type": "AtprotoPersonalDataServer", "serviceEndpoint": pds.URL},
			},
		})
	}))
	defer plc.Close()

	resolver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"did": "did:plc:wario64"})
	}))
	defer resolver.Close()

	appview := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"avatar": "https://cdn.bsky.app/img/avatar/plain/did:plc:wario64/bafyavatar@jpeg"})
	}))
	defer appview.Close()

	var newPostCalls, provisionCalls, saveprefsCalls int
	var lastSavedPrefs map[string]string
	rssChat := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/localnewuser":
			provisionCalls++
			json.NewEncoder(w).Encode(map[string]string{
				"screenname":  r.URL.Query().Get("screenname"),
				"email":       r.URL.Query().Get("email"),
				"emailSecret": "s3cret",
			})
		case "/getuserdata":
			json.NewEncoder(w).Encode(map[string]interface{}{"screenname": r.URL.Query().Get("screenname")})
		case "/saveprefs":
			saveprefsCalls++
			r.ParseForm()
			json.Unmarshal([]byte(r.PostFormValue("jsontext")), &lastSavedPrefs)
			json.NewEncoder(w).Encode(map[string]interface{}{"screenname": "bsky_wario64"})
		case "/newpost":
			newPostCalls++
			json.NewEncoder(w).Encode(map[string]interface{}{"id": 100 + newPostCalls})
		default:
			t.Fatalf("unexpected rss.chat path %s", r.URL.Path)
		}
	}))
	defer rssChat.Close()

	dir := t.TempDir()
	handlesPath := filepath.Join(dir, "handles.json")
	if err := os.WriteFile(handlesPath, []byte(`["wario64.bsky.social"]`), 0644); err != nil {
		t.Fatalf("writing handles.json: %v", err)
	}
	statePath := filepath.Join(dir, "state.db")

	cfg := config{
		handlesPath:  handlesPath,
		statePath:    statePath,
		rssChatURL:   rssChat.URL,
		resolver:     resolver.URL,
		plcDirectory: plc.URL,
		appview:      appview.URL,
		since:        24 * time.Hour,
		timeout:      5 * time.Second,
	}

	if err := run(cfg); err != nil {
		t.Fatalf("run: %v", err)
	}

	if provisionCalls != 1 {
		t.Errorf("provisionCalls = %d, want 1", provisionCalls)
	}
	if newPostCalls != 1 {
		t.Errorf("newPostCalls = %d, want 1 -- the reply in the fixture should have been skipped", newPostCalls)
	}
	if saveprefsCalls != 1 {
		t.Errorf("saveprefsCalls = %d, want 1 -- the avatar should have been synced", saveprefsCalls)
	}
	if lastSavedPrefs["myAvatarImageUrl"] != "https://cdn.bsky.app/img/avatar/plain/did:plc:wario64/bafyavatar@jpeg" {
		t.Errorf("saved prefs = %v, missing the expected avatar URL", lastSavedPrefs)
	}

	state, err := openState(statePath)
	if err != nil {
		t.Fatalf("openState: %v", err)
	}
	defer state.Close()

	pending, err := pendingItems(state)
	if err != nil {
		t.Fatalf("pendingItems: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("pendingItems = %d, want 0 after a successful run", len(pending))
	}

	// Rerun against the same state: provisioning must not repeat, the
	// already-forwarded post must not be forwarded a second time, and since
	// the fake AppView returns the same avatar, the prefs round trip should
	// be skipped too.
	if err := run(cfg); err != nil {
		t.Fatalf("second run: %v", err)
	}
	if provisionCalls != 1 {
		t.Errorf("provisionCalls after rerun = %d, want still 1 (provisioning must be idempotent)", provisionCalls)
	}
	if newPostCalls != 1 {
		t.Errorf("newPostCalls after rerun = %d, want still 1 (must not double-post)", newPostCalls)
	}
	if saveprefsCalls != 1 {
		t.Errorf("saveprefsCalls after rerun = %d, want still 1 (unchanged avatar should not re-sync)", saveprefsCalls)
	}
}

// TestRunContinuesPastAHandleFailure covers the multi-handle resilience the
// plan doc calls for: one handle's PDS/identity failure must not prevent the
// rest of the list from being processed.
func TestRunContinuesPastAHandleFailure(t *testing.T) {
	now := time.Now().UTC()

	pds := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		records := []map[string]interface{}{fakePost("post1", now, "hello from bob", false)}
		json.NewEncoder(w).Encode(map[string]interface{}{"records": records})
	}))
	defer pds.Close()

	plc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/did:plc:bob":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"service": []map[string]string{
					{"id": "#atproto_pds", "type": "AtprotoPersonalDataServer", "serviceEndpoint": pds.URL},
				},
			})
		default:
			w.WriteHeader(http.StatusInternalServerError) // simulates alice's PDS lookup failing
		}
	}))
	defer plc.Close()

	resolver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("handle") {
		case "alice.bsky.social":
			json.NewEncoder(w).Encode(map[string]string{"did": "did:plc:alice"})
		case "bob.bsky.social":
			json.NewEncoder(w).Encode(map[string]string{"did": "did:plc:bob"})
		}
	}))
	defer resolver.Close()

	// No avatar in the fixture -- exercises fetchAvatarURL's "nothing to
	// sync" path alongside the PDS-failure path this test is really about.
	appview := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{})
	}))
	defer appview.Close()

	var newPostCalls int
	rssChat := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/localnewuser":
			json.NewEncoder(w).Encode(map[string]string{
				"screenname":  r.URL.Query().Get("screenname"),
				"email":       r.URL.Query().Get("email"),
				"emailSecret": "s3cret",
			})
		case "/newpost":
			newPostCalls++
			json.NewEncoder(w).Encode(map[string]interface{}{"id": 200 + newPostCalls})
		}
	}))
	defer rssChat.Close()

	dir := t.TempDir()
	handlesPath := filepath.Join(dir, "handles.json")
	os.WriteFile(handlesPath, []byte(`["alice.bsky.social", "bob.bsky.social"]`), 0644)

	cfg := config{
		handlesPath:  handlesPath,
		statePath:    filepath.Join(dir, "state.db"),
		rssChatURL:   rssChat.URL,
		resolver:     resolver.URL,
		plcDirectory: plc.URL,
		appview:      appview.URL,
		since:        24 * time.Hour,
		timeout:      5 * time.Second,
	}

	if err := run(cfg); err != nil {
		t.Fatalf("run should not return an error just because one handle failed: %v", err)
	}
	if newPostCalls != 1 {
		t.Errorf("newPostCalls = %d, want 1 -- bob's post should still make it through despite alice's PDS lookup failing", newPostCalls)
	}
}

// TestBackfillHandleUsesStoredWatermarkNotSinceFlag proves something the
// rerun assertions in TestRunEndToEnd can't distinguish from plain
// atUri-based dedup: a handle with a previously recorded lastBackfillAt must
// use it -- not -since -- as the fetch window's start. A post older than the
// watermark must be excluded even though it would fall inside a fresh
// -since window, and a post newer than the watermark must come through.
func TestBackfillHandleUsesStoredWatermarkNotSinceFlag(t *testing.T) {
	watermark := time.Now().Add(-1 * time.Hour).UTC()
	older := watermark.Add(-30 * time.Minute) // before the watermark: must be excluded
	newer := watermark.Add(30 * time.Minute)  // after the watermark: must be included

	pds := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		records := []map[string]interface{}{
			fakePost("newpost", newer, "posted after the last backfill", false),
			fakePost("oldpost", older, "posted before the last backfill", false),
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"records": records})
	}))
	defer pds.Close()

	plc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"service": []map[string]string{
				{"id": "#atproto_pds", "type": "AtprotoPersonalDataServer", "serviceEndpoint": pds.URL},
			},
		})
	}))
	defer plc.Close()

	appview := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{}) // no avatar; not what this test is about
	}))
	defer appview.Close()

	state := newTestState(t)
	if err := insertTrackedHandle(state, trackedHandle{
		Handle: "alice.bsky.social", DID: "did:plc:alice",
		RSSScreenname: "bsky_alice", RSSEmail: "bsky_alice@bsky.invalid", RSSEmailSecret: "s3cret",
	}); err != nil {
		t.Fatalf("insertTrackedHandle: %v", err)
	}
	if err := updateLastBackfillAt(state, "alice.bsky.social", watermark); err != nil {
		t.Fatalf("updateLastBackfillAt: %v", err)
	}

	cfg := config{
		plcDirectory: plc.URL,
		appview:      appview.URL,
		since:        24 * time.Hour, // would wrongly include "older" too, if -since were used instead of the watermark
		timeout:      5 * time.Second,
	}

	err := backfillHandle(context.Background(), &http.Client{Timeout: cfg.timeout}, state, cfg, handleEntry{Handle: "alice.bsky.social"})
	if err != nil {
		t.Fatalf("backfillHandle: %v", err)
	}

	pending, err := pendingItems(state)
	if err != nil {
		t.Fatalf("pendingItems: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("pendingItems = %d, want exactly 1 (only the post after the watermark); got %+v", len(pending), pending)
	}
	if pending[0].Rkey != "newpost" {
		t.Errorf("stored item = %q, want the post after the watermark; the pre-watermark post should have been excluded", pending[0].Rkey)
	}
}

func TestLoadHandlesParsesMixedStringsAndObjects(t *testing.T) {
	path := filepath.Join(t.TempDir(), "handles.json")
	content := `[
		"plain.bsky.social",
		{"handle": "override.bsky.social", "initialBackfillDays": 14}
	]`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing handles.json: %v", err)
	}

	got, err := loadHandles(path)
	if err != nil {
		t.Fatalf("loadHandles: %v", err)
	}
	want := []handleEntry{
		{Handle: "plain.bsky.social"},
		{Handle: "override.bsky.social", InitialBackfillDays: 14},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d entries, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestLoadHandlesRejectsObjectMissingHandle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "handles.json")
	if err := os.WriteFile(path, []byte(`[{"initialBackfillDays": 7}]`), 0644); err != nil {
		t.Fatalf("writing handles.json: %v", err)
	}

	if _, err := loadHandles(path); err == nil {
		t.Fatal("expected an error for an entry missing \"handle\"")
	}
}

// TestBackfillHandleUsesInitialBackfillDaysOnFirstRun covers the actual
// feature: a per-handle initialBackfillDays override must widen (or narrow)
// the window on a handle's first-ever backfill, beyond what -since alone
// would allow.
func TestBackfillHandleUsesInitialBackfillDaysOnFirstRun(t *testing.T) {
	now := time.Now().UTC()
	tenDaysAgo := now.Add(-10 * 24 * time.Hour) // outside a 24h -since, inside a 14-day override

	pds := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		records := []map[string]interface{}{fakePost("old-post", tenDaysAgo, "ten days old", false)}
		json.NewEncoder(w).Encode(map[string]interface{}{"records": records})
	}))
	defer pds.Close()

	plc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"service": []map[string]string{
				{"id": "#atproto_pds", "type": "AtprotoPersonalDataServer", "serviceEndpoint": pds.URL},
			},
		})
	}))
	defer plc.Close()

	appview := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{})
	}))
	defer appview.Close()

	state := newTestState(t)
	if err := insertTrackedHandle(state, trackedHandle{
		Handle: "alice.bsky.social", DID: "did:plc:alice",
		RSSScreenname: "bsky_alice", RSSEmail: "bsky_alice@bsky.invalid", RSSEmailSecret: "s3cret",
	}); err != nil {
		t.Fatalf("insertTrackedHandle: %v", err)
	}
	// No lastBackfillAt set: this is a first run, so the override applies.

	cfg := config{
		plcDirectory: plc.URL,
		appview:      appview.URL,
		since:        24 * time.Hour, // would exclude the 10-day-old post on its own
		timeout:      5 * time.Second,
	}
	entry := handleEntry{Handle: "alice.bsky.social", InitialBackfillDays: 14}

	if err := backfillHandle(context.Background(), &http.Client{Timeout: cfg.timeout}, state, cfg, entry); err != nil {
		t.Fatalf("backfillHandle: %v", err)
	}

	pending, err := pendingItems(state)
	if err != nil {
		t.Fatalf("pendingItems: %v", err)
	}
	if len(pending) != 1 || pending[0].Rkey != "old-post" {
		t.Fatalf("pending = %+v, want the 10-day-old post included via the 14-day override", pending)
	}
}
