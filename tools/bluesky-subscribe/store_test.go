package main

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

var errBoom = errors.New("boom")

func newTestState(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "state.db")
	conn, err := openState(path)
	if err != nil {
		t.Fatalf("openState: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func TestGetTrackedHandleMissingReturnsNil(t *testing.T) {
	conn := newTestState(t)

	got, err := getTrackedHandle(conn, "nobody.bsky.social")
	if err != nil {
		t.Fatalf("getTrackedHandle: %v", err)
	}
	if got != nil {
		t.Fatalf("got = %+v, want nil for an untracked handle", got)
	}
}

func TestInsertAndGetTrackedHandleRoundTrip(t *testing.T) {
	conn := newTestState(t)

	want := trackedHandle{
		Handle:         "wario64.bsky.social",
		DID:            "did:plc:abc123",
		RSSScreenname:  "bsky_wario64",
		RSSEmail:       "bsky_wario64@bsky.invalid",
		RSSEmailSecret: "s3cret",
	}
	if err := insertTrackedHandle(conn, want); err != nil {
		t.Fatalf("insertTrackedHandle: %v", err)
	}

	got, err := getTrackedHandle(conn, want.Handle)
	if err != nil {
		t.Fatalf("getTrackedHandle: %v", err)
	}
	if got == nil {
		t.Fatal("got nil, want the row just inserted")
	}
	if got.DID != want.DID || got.RSSScreenname != want.RSSScreenname ||
		got.RSSEmail != want.RSSEmail || got.RSSEmailSecret != want.RSSEmailSecret {
		t.Errorf("got = %+v, want %+v", *got, want)
	}
	if got.LastBackfillAt.Valid {
		t.Error("LastBackfillAt should be unset on a freshly provisioned handle")
	}
}

func TestUpdateHandleDIDAndLastBackfillAt(t *testing.T) {
	conn := newTestState(t)
	insertTrackedHandle(conn, trackedHandle{Handle: "alice.bsky.social", RSSScreenname: "bsky_alice"})

	if err := updateHandleDID(conn, "alice.bsky.social", "did:plc:xyz"); err != nil {
		t.Fatalf("updateHandleDID: %v", err)
	}

	when := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	if err := updateLastBackfillAt(conn, "alice.bsky.social", when); err != nil {
		t.Fatalf("updateLastBackfillAt: %v", err)
	}

	got, err := getTrackedHandle(conn, "alice.bsky.social")
	if err != nil || got == nil {
		t.Fatalf("getTrackedHandle: %v, %v", got, err)
	}
	if got.DID != "did:plc:xyz" {
		t.Errorf("DID = %q, want did:plc:xyz", got.DID)
	}
	if !got.LastBackfillAt.Valid {
		t.Fatal("LastBackfillAt should be set")
	}
	if !got.LastBackfillAt.Time.Equal(when) {
		t.Errorf("LastBackfillAt = %v, want %v", got.LastBackfillAt.Time, when)
	}
}

// TestInsertItemIfNewDedupsOnAtURI is the core dedup guarantee: rerunning a
// backfill over an overlapping window must not create duplicate rows or
// re-queue an already-forwarded item.
func TestInsertItemIfNewDedupsOnAtURI(t *testing.T) {
	conn := newTestState(t)
	insertTrackedHandle(conn, trackedHandle{Handle: "alice.bsky.social", RSSScreenname: "bsky_alice"})

	item := blueskyItem{
		AtURI:     "at://did:plc:xyz/app.bsky.feed.post/abc",
		Handle:    "alice.bsky.social",
		DID:       "did:plc:xyz",
		Rkey:      "abc",
		CID:       "bafyabc",
		CreatedAt: time.Now(),
		Text:      "hello",
		RawJSON:   `{"text":"hello"}`,
	}

	inserted, err := insertItemIfNew(conn, item)
	if err != nil {
		t.Fatalf("first insert: %v", err)
	}
	if !inserted {
		t.Fatal("first insert should report inserted=true")
	}

	inserted, err = insertItemIfNew(conn, item)
	if err != nil {
		t.Fatalf("second insert: %v", err)
	}
	if inserted {
		t.Fatal("second insert of the same atUri should report inserted=false")
	}

	pending, err := pendingItems(conn)
	if err != nil {
		t.Fatalf("pendingItems: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("pendingItems returned %d rows, want exactly 1 (no duplicate)", len(pending))
	}
}

func TestMarkForwardedRemovesItemFromPending(t *testing.T) {
	conn := newTestState(t)
	insertTrackedHandle(conn, trackedHandle{Handle: "alice.bsky.social", RSSScreenname: "bsky_alice"})
	insertItemIfNew(conn, blueskyItem{
		AtURI: "at://did:plc:xyz/app.bsky.feed.post/abc", Handle: "alice.bsky.social",
		DID: "did:plc:xyz", Rkey: "abc", CID: "c1", CreatedAt: time.Now(), Text: "hi", RawJSON: "{}",
	})

	if err := markForwarded(conn, "at://did:plc:xyz/app.bsky.feed.post/abc", 42); err != nil {
		t.Fatalf("markForwarded: %v", err)
	}

	pending, err := pendingItems(conn)
	if err != nil {
		t.Fatalf("pendingItems: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("pendingItems returned %d rows, want 0 after forwarding", len(pending))
	}
}

func TestMarkForwardFailedKeepsItemPendingForRetry(t *testing.T) {
	conn := newTestState(t)
	insertTrackedHandle(conn, trackedHandle{Handle: "alice.bsky.social", RSSScreenname: "bsky_alice"})
	insertItemIfNew(conn, blueskyItem{
		AtURI: "at://did:plc:xyz/app.bsky.feed.post/abc", Handle: "alice.bsky.social",
		DID: "did:plc:xyz", Rkey: "abc", CID: "c1", CreatedAt: time.Now(), Text: "hi", RawJSON: "{}",
	})

	if err := markForwardFailed(conn, "at://did:plc:xyz/app.bsky.feed.post/abc", errBoom); err != nil {
		t.Fatalf("markForwardFailed: %v", err)
	}

	pending, err := pendingItems(conn)
	if err != nil {
		t.Fatalf("pendingItems: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("pendingItems returned %d rows, want 1 -- a failed forward must stay pending for retry", len(pending))
	}
}
