package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// fakePost builds a minimal listRecords record for a given rkey/time/text,
// optionally marked as a reply.
func fakePost(rkey string, when time.Time, text string, isReply bool) map[string]interface{} {
	value := map[string]interface{}{
		"$type":     "app.bsky.feed.post",
		"text":      text,
		"createdAt": when.Format(time.RFC3339),
	}
	if isReply {
		value["reply"] = map[string]interface{}{
			"parent": map[string]string{"uri": "at://did:plc:parent/app.bsky.feed.post/xyz", "cid": "bafyparent"},
			"root":   map[string]string{"uri": "at://did:plc:parent/app.bsky.feed.post/xyz", "cid": "bafyparent"},
		}
	}
	return map[string]interface{}{
		"uri":   "at://did:plc:abc123/app.bsky.feed.post/" + rkey,
		"cid":   "bafy" + rkey,
		"value": value,
	}
}

func TestFetchTopLevelPostsSinceFiltersRepliesAndWindow(t *testing.T) {
	now := time.Now().UTC()
	records := []map[string]interface{}{
		fakePost("new1", now, "brand new top-level post", false),
		fakePost("new2reply", now.Add(-time.Minute), "a reply, should be skipped", true),
		fakePost("instant-window-edge", now.Add(-2*time.Hour), "inside the window", false),
		fakePost("too-old", now.Add(-48*time.Hour), "outside the window", false),
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("repo"); got != "did:plc:abc123" {
			t.Errorf("repo param = %q, want did:plc:abc123", got)
		}
		if got := r.URL.Query().Get("collection"); got != "app.bsky.feed.post" {
			t.Errorf("collection param = %q, want app.bsky.feed.post", got)
		}
		resp := map[string]interface{}{"records": records}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	since := now.Add(-24 * time.Hour)
	items, stats, err := fetchTopLevelPostsSince(context.Background(), srv.Client(), srv.URL, "alice.bsky.social", "did:plc:abc123", since)
	if err != nil {
		t.Fatalf("fetchTopLevelPostsSince: %v", err)
	}

	if len(items) != 2 {
		t.Fatalf("got %d items, want 2 (reply and too-old must be excluded); items: %+v", len(items), items)
	}
	for _, item := range items {
		if item.Rkey == "new2reply" || item.Rkey == "too-old" {
			t.Errorf("item %s should have been filtered out", item.Rkey)
		}
		if item.Handle != "alice.bsky.social" {
			t.Errorf("Handle = %q, want alice.bsky.social", item.Handle)
		}
	}

	if stats.RepliesSkipped != 1 {
		t.Errorf("RepliesSkipped = %d, want 1", stats.RepliesSkipped)
	}
	// "too-old" is where the stop condition fires, so it's counted as seen
	// even though it's excluded from the window.
	if stats.RecordsSeen != 4 {
		t.Errorf("RecordsSeen = %d, want 4", stats.RecordsSeen)
	}
}

// TestFetchTopLevelPostsSinceCountsUnparseableRecordsWithoutAborting is the
// regression test for the silent-drop gap: an account with a record whose
// createdAt won't parse (a known real-world occurrence from nonstandard
// third-party posting clients) must not lose every subsequent post over it,
// and the drop must be visible in stats rather than invisible.
func TestFetchTopLevelPostsSinceCountsUnparseableRecordsWithoutAborting(t *testing.T) {
	now := time.Now().UTC()
	good := fakePost("good", now, "a normal post", false)
	bad := map[string]interface{}{
		"uri": "at://did:plc:abc123/app.bsky.feed.post/bad",
		"cid": "bafybad",
		"value": map[string]interface{}{
			"$type":     "app.bsky.feed.post",
			"text":      "posted from something with a broken clock",
			"createdAt": "not-a-real-timestamp",
		},
	}
	olderGood := fakePost("older-good", now.Add(-time.Minute), "another normal post", false)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{"records": []map[string]interface{}{good, bad, olderGood}}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	items, stats, err := fetchTopLevelPostsSince(context.Background(), srv.Client(), srv.URL, "alice.bsky.social", "did:plc:abc123", now.Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("fetchTopLevelPostsSince: %v", err)
	}

	if len(items) != 2 {
		t.Fatalf("got %d items, want 2 -- the malformed record should be skipped, not the whole fetch aborted; items: %+v", len(items), items)
	}
	if stats.MalformedSkipped != 1 {
		t.Errorf("MalformedSkipped = %d, want 1", stats.MalformedSkipped)
	}
	if stats.RecordsSeen != 3 {
		t.Errorf("RecordsSeen = %d, want 3", stats.RecordsSeen)
	}
}

func TestFetchTopLevelPostsSincePaginates(t *testing.T) {
	now := time.Now().UTC()
	pages := [][]map[string]interface{}{
		{fakePost("page1-a", now, "first page a", false)},
		{fakePost("page2-a", now.Add(-time.Minute), "second page a", false)},
	}

	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cursor := r.URL.Query().Get("cursor")
		var resp map[string]interface{}
		if cursor == "" {
			resp = map[string]interface{}{"records": pages[0], "cursor": "page2"}
		} else if cursor == "page2" {
			resp = map[string]interface{}{"records": pages[1]} // no cursor: last page
		} else {
			t.Fatalf("unexpected cursor %q", cursor)
		}
		calls++
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	items, _, err := fetchTopLevelPostsSince(context.Background(), srv.Client(), srv.URL, "alice.bsky.social", "did:plc:abc123", now.Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("fetchTopLevelPostsSince: %v", err)
	}
	if calls != 2 {
		t.Fatalf("made %d requests, want 2 (one per page)", calls)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2 across both pages", len(items))
	}
}

func TestFetchTopLevelPostsSinceErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, _, err := fetchTopLevelPostsSince(context.Background(), srv.Client(), srv.URL, "alice.bsky.social", "did:plc:abc123", time.Now().Add(-time.Hour))
	if err == nil {
		t.Fatal("expected an error for a 500 response")
	}
}

func TestRkeyFromURI(t *testing.T) {
	got := rkeyFromURI("at://did:plc:abc123/app.bsky.feed.post/3kb3fge5lm32x")
	if got != "3kb3fge5lm32x" {
		t.Errorf("rkeyFromURI = %q, want 3kb3fge5lm32x", got)
	}
}
