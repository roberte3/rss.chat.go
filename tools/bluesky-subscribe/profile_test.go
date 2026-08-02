package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchAvatarURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/xrpc/app.bsky.actor.getProfile" {
			t.Errorf("path = %q, want /xrpc/app.bsky.actor.getProfile", r.URL.Path)
		}
		if got := r.URL.Query().Get("actor"); got != "wario64.bsky.social" {
			t.Errorf("actor param = %q, want wario64.bsky.social", got)
		}
		fmt.Fprint(w, `{"did":"did:plc:abc123","handle":"wario64.bsky.social","avatar":"https://cdn.bsky.app/img/avatar/plain/did:plc:abc123/bafyavatar@jpeg"}`)
	}))
	defer srv.Close()

	got, err := fetchAvatarURL(context.Background(), srv.Client(), srv.URL, "wario64.bsky.social")
	if err != nil {
		t.Fatalf("fetchAvatarURL: %v", err)
	}
	want := "https://cdn.bsky.app/img/avatar/plain/did:plc:abc123/bafyavatar@jpeg"
	if got != want {
		t.Errorf("got = %q, want %q", got, want)
	}
}

// TestFetchAvatarURLNoAvatarSet covers the common case of an account with no
// avatar: this must not be treated as an error.
func TestFetchAvatarURLNoAvatarSet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"did":"did:plc:abc123","handle":"nobody.bsky.social"}`)
	}))
	defer srv.Close()

	got, err := fetchAvatarURL(context.Background(), srv.Client(), srv.URL, "nobody.bsky.social")
	if err != nil {
		t.Fatalf("fetchAvatarURL: %v", err)
	}
	if got != "" {
		t.Errorf("got = %q, want empty string for an account with no avatar", got)
	}
}

func TestFetchAvatarURLErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := fetchAvatarURL(context.Background(), srv.Client(), srv.URL, "nobody.bsky.social")
	if err == nil {
		t.Fatal("expected an error for a 404 response")
	}
}
