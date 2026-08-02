package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProvisionLocalUser(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/localnewuser" {
			t.Errorf("path = %q, want /localnewuser", r.URL.Path)
		}
		if got := r.URL.Query().Get("screenname"); got != "bsky_wario64" {
			t.Errorf("screenname param = %q, want bsky_wario64", got)
		}
		json.NewEncoder(w).Encode(map[string]string{
			"screenname":  "bsky_wario64",
			"email":       "bsky_wario64@bsky.invalid",
			"emailSecret": "s3cret",
		})
	}))
	defer srv.Close()

	got, err := provisionLocalUser(context.Background(), srv.Client(), srv.URL, "bsky_wario64", "bsky_wario64@bsky.invalid")
	if err != nil {
		t.Fatalf("provisionLocalUser: %v", err)
	}
	if got.EmailSecret != "s3cret" {
		t.Errorf("EmailSecret = %q, want s3cret", got.EmailSecret)
	}
}

// TestProvisionLocalUserRejectsEmptySecret guards against silently accepting
// a malformed or unexpected response -- a missing secret here means every
// later forward for this handle would fail authentication anyway, so it's
// better to fail loudly at provisioning time.
func TestProvisionLocalUserRejectsEmptySecret(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"screenname": "bsky_wario64", "email": "x@bsky.invalid"})
	}))
	defer srv.Close()

	_, err := provisionLocalUser(context.Background(), srv.Client(), srv.URL, "bsky_wario64", "x@bsky.invalid")
	if err == nil {
		t.Fatal("expected an error when emailSecret is missing from the response")
	}
}

func TestProvisionLocalUserErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("Can't create the user because localnewuser only works from the machine the server is running on."))
	}))
	defer srv.Close()

	_, err := provisionLocalUser(context.Background(), srv.Client(), srv.URL, "bsky_wario64", "x@bsky.invalid")
	if err == nil {
		t.Fatal("expected an error for a non-200 response")
	}
	if !strings.Contains(err.Error(), "only works from the machine") {
		t.Errorf("error = %q, want it to surface the server's message", err)
	}
}

func TestForwardPostSendsExpectedFormAndParsesID(t *testing.T) {
	var gotForm map[string][]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/newpost" {
			t.Errorf("path = %q, want /newpost", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm: %v", err)
		}
		gotForm = map[string][]string(r.PostForm)
		json.NewEncoder(w).Encode(map[string]interface{}{"id": 42})
	}))
	defer srv.Close()

	item := blueskyItem{
		AtURI:  "at://did:plc:abc123/app.bsky.feed.post/xyz",
		Handle: "wario64.bsky.social",
		Rkey:   "xyz",
		Text:   "Cyber Monday deals incoming",
	}

	id, err := forwardPost(context.Background(), srv.Client(), srv.URL, "bsky_wario64@bsky.invalid", "s3cret", item)
	if err != nil {
		t.Fatalf("forwardPost: %v", err)
	}
	if id != 42 {
		t.Errorf("id = %d, want 42", id)
	}

	if gotForm["emailaddress"][0] != "bsky_wario64@bsky.invalid" {
		t.Errorf("emailaddress = %v, want bsky_wario64@bsky.invalid", gotForm["emailaddress"])
	}
	if gotForm["emailcode"][0] != "s3cret" {
		t.Errorf("emailcode = %v, want s3cret", gotForm["emailcode"])
	}

	var payload struct {
		Description string `json:"description"`
	}
	if err := json.Unmarshal([]byte(gotForm["jsontext"][0]), &payload); err != nil {
		t.Fatalf("failed to unmarshal jsontext: %v", err)
	}
	if !strings.Contains(payload.Description, "Cyber Monday deals incoming") {
		t.Errorf("description = %q, missing the post text", payload.Description)
	}
	if !strings.Contains(payload.Description, "https://bsky.app/profile/wario64.bsky.social/post/xyz") {
		t.Errorf("description = %q, missing the permalink", payload.Description)
	}
}

func TestForwardPostErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("Can't create post because description is required."))
	}))
	defer srv.Close()

	_, err := forwardPost(context.Background(), srv.Client(), srv.URL, "x@bsky.invalid", "s3cret", blueskyItem{AtURI: "at://x", Handle: "x.bsky.social", Rkey: "x"})
	if err == nil {
		t.Fatal("expected an error for a non-200 response")
	}
}

func TestFetchUserPrefsEmptyForNewUser(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"screenname": "bsky_wario64"}) // no prefs key at all
	}))
	defer srv.Close()

	prefs, err := fetchUserPrefs(context.Background(), srv.Client(), srv.URL, "bsky_wario64")
	if err != nil {
		t.Fatalf("fetchUserPrefs: %v", err)
	}
	if prefs == nil {
		t.Fatal("prefs should be an empty map, not nil, so callers can add keys directly")
	}
	if len(prefs) != 0 {
		t.Errorf("prefs = %v, want empty", prefs)
	}
}

func TestFetchUserPrefsParsesExisting(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"screenname": "bsky_wario64",
			"prefs":      map[string]string{"myFeedTitle": "Wario64 Deals"},
		})
	}))
	defer srv.Close()

	prefs, err := fetchUserPrefs(context.Background(), srv.Client(), srv.URL, "bsky_wario64")
	if err != nil {
		t.Fatalf("fetchUserPrefs: %v", err)
	}
	if prefs["myFeedTitle"] != "Wario64 Deals" {
		t.Errorf("prefs = %v, want myFeedTitle preserved", prefs)
	}
}

func TestSaveUserPrefsSendsFullBlob(t *testing.T) {
	var gotForm map[string][]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/saveprefs" {
			t.Errorf("path = %q, want /saveprefs", r.URL.Path)
		}
		r.ParseForm()
		gotForm = map[string][]string(r.PostForm)
		json.NewEncoder(w).Encode(map[string]interface{}{"screenname": "bsky_wario64"})
	}))
	defer srv.Close()

	err := saveUserPrefs(context.Background(), srv.Client(), srv.URL, "bsky_wario64@bsky.invalid", "s3cret",
		map[string]interface{}{"myFeedTitle": "Wario64 Deals", "myAvatarImageUrl": "https://cdn.example/a.jpg"})
	if err != nil {
		t.Fatalf("saveUserPrefs: %v", err)
	}

	var payload map[string]string
	if err := json.Unmarshal([]byte(gotForm["jsontext"][0]), &payload); err != nil {
		t.Fatalf("failed to unmarshal jsontext: %v", err)
	}
	if payload["myFeedTitle"] != "Wario64 Deals" || payload["myAvatarImageUrl"] != "https://cdn.example/a.jpg" {
		t.Errorf("payload = %v, missing an expected key", payload)
	}
}

// TestSyncAvatarMergesRatherThanClobbers is the property that matters most
// here: /saveprefs replaces the whole prefs blob server-side (see
// db.UpdateUserPrefs), so syncAvatar must read-modify-write rather than
// sending just {"myAvatarImageUrl": ...} and silently wiping out any other
// prefs the account already has.
func TestSyncAvatarMergesRatherThanClobbers(t *testing.T) {
	var savedPrefs map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/getuserdata":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"screenname": "bsky_wario64",
				"prefs":      map[string]string{"myFeedTitle": "Wario64 Deals"},
			})
		case "/saveprefs":
			r.ParseForm()
			json.Unmarshal([]byte(r.PostFormValue("jsontext")), &savedPrefs)
			json.NewEncoder(w).Encode(map[string]interface{}{"screenname": "bsky_wario64"})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	err := syncAvatar(context.Background(), srv.Client(), srv.URL, "bsky_wario64", "bsky_wario64@bsky.invalid", "s3cret", "https://cdn.example/a.jpg")
	if err != nil {
		t.Fatalf("syncAvatar: %v", err)
	}

	if savedPrefs["myFeedTitle"] != "Wario64 Deals" {
		t.Errorf("myFeedTitle was lost, saved prefs = %v", savedPrefs)
	}
	if savedPrefs["myAvatarImageUrl"] != "https://cdn.example/a.jpg" {
		t.Errorf("myAvatarImageUrl not set correctly, saved prefs = %v", savedPrefs)
	}
}
