package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/roberte3/rss.chat.go/db"
)

// localRequest builds a /localnewuser request that looks like it came from
// the machine the server runs on.
func localRequest(path string) *http.Request {
	req := httptest.NewRequest("GET", path, nil)
	req.RemoteAddr = "127.0.0.1:54321"
	return req
}

// TestLocalNewUserCreatesUserAndReturnsSecret is the core case this endpoint
// exists for: unlike /createnewuser, the emailSecret comes back in the
// response body instead of only ever reaching a real mailbox.
func TestLocalNewUserCreatesUserAndReturnsSecret(t *testing.T) {
	mux, conn, _ := setupTestServer(t)

	req := localRequest("/localnewuser?screenname=bsky_wario64&email=bsky_wario64@example.com")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp["screenname"] != "bsky_wario64" {
		t.Errorf("screenname = %q, want bsky_wario64", resp["screenname"])
	}
	if resp["email"] != "bsky_wario64@example.com" {
		t.Errorf("email = %q, want bsky_wario64@example.com", resp["email"])
	}
	if resp["emailSecret"] == "" {
		t.Error("emailSecret was empty in the response")
	}

	// The returned secret must actually authenticate as this user.
	user, err := AuthenticateUser(conn, resp["email"], resp["emailSecret"])
	if err != nil {
		t.Fatalf("returned emailSecret did not authenticate: %v", err)
	}
	if user.Screenname != "bsky_wario64" {
		t.Errorf("authenticated screenname = %q, want bsky_wario64", user.Screenname)
	}

	// A user created this way must have a feed from the moment it exists,
	// same guarantee CreateNewUser gives -- see CLAUDE.md's storage-modes note.
	stored, err := db.GetUserInfoByScreenname(conn, "bsky_wario64")
	if err != nil || stored == nil {
		t.Fatalf("user was not persisted: err=%v, stored=%v", err, stored)
	}
}

// TestLocalNewUserRejectsRemoteRequest is the reason this endpoint is safe to
// return a bearer credential in plain JSON: anything not on loopback must be
// refused, full stop.
func TestLocalNewUserRejectsRemoteRequest(t *testing.T) {
	mux, conn, _ := setupTestServer(t)

	req := httptest.NewRequest("GET", "/localnewuser?screenname=intruder&email=intruder@example.com", nil)
	req.RemoteAddr = "203.0.113.9:54321"
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code == http.StatusOK {
		t.Fatalf("remote request was allowed to create a user; body: %s", w.Body.String())
	}

	user, err := db.GetUserInfoByScreenname(conn, "intruder")
	if err != nil {
		t.Fatalf("lookup failed: %v", err)
	}
	if user != nil {
		t.Fatal("user was created despite failing the loopback check")
	}
}

// TestLocalNewUserRejectsForwardedRequest covers the proxy case: a loopback
// RemoteAddr behind X-Forwarded-For is the proxy talking, not a local caller,
// and must not be trusted just because the socket address looks local.
func TestLocalNewUserRejectsForwardedRequest(t *testing.T) {
	mux, conn, _ := setupTestServer(t)

	req := httptest.NewRequest("GET", "/localnewuser?screenname=proxied&email=proxied@example.com", nil)
	req.RemoteAddr = "127.0.0.1:54321"
	req.Header.Set("X-Forwarded-For", "203.0.113.9")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code == http.StatusOK {
		t.Fatalf("forwarded request was allowed to create a user; body: %s", w.Body.String())
	}

	user, err := db.GetUserInfoByScreenname(conn, "proxied")
	if err != nil {
		t.Fatalf("lookup failed: %v", err)
	}
	if user != nil {
		t.Fatal("user was created despite arriving via X-Forwarded-For")
	}
}

// TestLocalNewUserIsIdempotent covers the property the bluesky-bridge utility
// depends on: rerunning provisioning for an already-created screenname must
// return the same secret rather than erroring or minting a second one.
func TestLocalNewUserIsIdempotent(t *testing.T) {
	mux, _, _ := setupTestServer(t)

	first := httptest.NewRecorder()
	mux.ServeHTTP(first, localRequest("/localnewuser?screenname=bsky_wario64&email=bsky_wario64@example.com"))
	if first.Code != http.StatusOK {
		t.Fatalf("first call: status = %d, body: %s", first.Code, first.Body.String())
	}
	var firstResp map[string]string
	if err := json.Unmarshal(first.Body.Bytes(), &firstResp); err != nil {
		t.Fatalf("failed to unmarshal first response: %v", err)
	}

	// Second call passes a different email; the existing record must win.
	second := httptest.NewRecorder()
	mux.ServeHTTP(second, localRequest("/localnewuser?screenname=bsky_wario64&email=someone-else@example.com"))
	if second.Code != http.StatusOK {
		t.Fatalf("second call: status = %d, body: %s", second.Code, second.Body.String())
	}
	var secondResp map[string]string
	if err := json.Unmarshal(second.Body.Bytes(), &secondResp); err != nil {
		t.Fatalf("failed to unmarshal second response: %v", err)
	}

	if secondResp["emailSecret"] != firstResp["emailSecret"] {
		t.Errorf("emailSecret changed on rerun: first %q, second %q", firstResp["emailSecret"], secondResp["emailSecret"])
	}
	if secondResp["email"] != firstResp["email"] {
		t.Errorf("email changed on rerun: first %q, second %q -- the stored record should win over the new param", firstResp["email"], secondResp["email"])
	}
}

// TestLocalNewUserRequiresScreennameAndEmail covers the validation errors,
// mirroring the messages localNewUser in rssnetwork.js returns.
func TestLocalNewUserRequiresScreennameAndEmail(t *testing.T) {
	mux, _, _ := setupTestServer(t)

	tests := []struct {
		name string
		path string
	}{
		{"missing screenname", "/localnewuser?email=nobody@example.com"},
		{"missing email", "/localnewuser?screenname=nobody"},
		{"invalid screenname", "/localnewuser?screenname=has.a.dot&email=nobody@example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, localRequest(tt.path))
			if w.Code == http.StatusOK {
				t.Fatalf("expected an error status, got %d; body: %s", w.Code, w.Body.String())
			}
		})
	}
}
