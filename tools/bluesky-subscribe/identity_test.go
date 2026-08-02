package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResolveHandle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/xrpc/com.atproto.identity.resolveHandle" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("handle"); got != "wario64.bsky.social" {
			t.Errorf("handle param = %q, want wario64.bsky.social", got)
		}
		fmt.Fprint(w, `{"did":"did:plc:abc123"}`)
	}))
	defer srv.Close()

	did, err := resolveHandle(context.Background(), srv.Client(), srv.URL, "wario64.bsky.social")
	if err != nil {
		t.Fatalf("resolveHandle: %v", err)
	}
	if did != "did:plc:abc123" {
		t.Errorf("did = %q, want did:plc:abc123", did)
	}
}

func TestResolveHandleErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := resolveHandle(context.Background(), srv.Client(), srv.URL, "nobody.bsky.social")
	if err == nil {
		t.Fatal("expected an error for a 404 response")
	}
}

func TestResolvePDSEndpointDidPlc(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/did:plc:abc123" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		fmt.Fprint(w, `{
			"service": [
				{"id": "#atproto_pds", "type": "AtprotoPersonalDataServer", "serviceEndpoint": "https://pds.example.com"}
			]
		}`)
	}))
	defer srv.Close()

	pds, err := resolvePDSEndpoint(context.Background(), srv.Client(), srv.URL, "did:plc:abc123")
	if err != nil {
		t.Fatalf("resolvePDSEndpoint: %v", err)
	}
	if pds != "https://pds.example.com" {
		t.Errorf("pds = %q, want https://pds.example.com", pds)
	}
}

func TestResolvePDSEndpointNoServiceFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"service": []}`)
	}))
	defer srv.Close()

	_, err := resolvePDSEndpoint(context.Background(), srv.Client(), srv.URL, "did:plc:abc123")
	if err == nil {
		t.Fatal("expected an error when no AtprotoPersonalDataServer entry exists")
	}
}
