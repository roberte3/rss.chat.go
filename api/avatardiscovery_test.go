package api

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// TestGetUserAvatarSuccess verifies successful avatar retrieval
func TestGetUserAvatarSuccess(t *testing.T) {
	mux, conn, _ := setupTestServer(t)

	insertTestUser(t, conn, "alice", "secret_alice")

	// Upload avatar for alice
	pngData := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, 0xDE,
	}
	form := url.Values{
		"data":        {base64.StdEncoding.EncodeToString(pngData)},
		"contentType": {"image/png"},
	}
	w := authedPost(mux, "/uploadavatar", "alice", "secret_alice", form)
	var uploadResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &uploadResp)
	avatarURL := uploadResp["avatarUrl"].(string)

	// Get avatar
	req := httptest.NewRequest("GET", "/getuseravatar?screenname=alice", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, expected 200: %s", w.Code, w.Body.String())
	}

	var resp AvatarInfo
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Screenname != "alice" {
		t.Errorf("expected screenname=alice, got %q", resp.Screenname)
	}

	if resp.AvatarURL != avatarURL {
		t.Errorf("expected avatarUrl=%q, got %q", avatarURL, resp.AvatarURL)
	}
}

// TestGetUserAvatarNotFound verifies 404 for non-existent user
func TestGetUserAvatarNotFound(t *testing.T) {
	mux, _, _ := setupTestServer(t)

	req := httptest.NewRequest("GET", "/getuseravatar?screenname=nonexistent", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code == http.StatusOK {
		t.Errorf("expected error for non-existent user, got status %d", w.Code)
	}

	if !containsSubstring(w.Body.String(), "not found") {
		t.Errorf("expected 'not found' error message, got: %s", w.Body.String())
	}
}

// TestGetUserAvatarNoAvatar verifies error when user has no avatar
func TestGetUserAvatarNoAvatar(t *testing.T) {
	mux, conn, _ := setupTestServer(t)

	insertTestUser(t, conn, "alice", "secret_alice")

	// Don't upload avatar, just try to get it
	req := httptest.NewRequest("GET", "/getuseravatar?screenname=alice", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code == http.StatusOK {
		t.Errorf("expected error when user has no avatar, got status %d", w.Code)
	}

	if !containsSubstring(w.Body.String(), "does not have an avatar") {
		t.Errorf("expected 'does not have an avatar' message, got: %s", w.Body.String())
	}
}

// TestGetUserAvatarMissingScreenname verifies error when screenname is missing
func TestGetUserAvatarMissingScreenname(t *testing.T) {
	mux, _, _ := setupTestServer(t)

	req := httptest.NewRequest("GET", "/getuseravatar", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code == http.StatusOK {
		t.Errorf("expected error for missing screenname, got status %d", w.Code)
	}

	if !containsSubstring(w.Body.String(), "screenname is required") {
		t.Errorf("expected 'screenname is required', got: %s", w.Body.String())
	}
}

// TestGetUsersWithAvatarsEmpty verifies empty result when no users have avatars
func TestGetUsersWithAvatarsEmpty(t *testing.T) {
	mux, conn, _ := setupTestServer(t)

	// Create users without avatars
	insertTestUser(t, conn, "alice", "secret_alice")
	insertTestUser(t, conn, "bob", "secret_bob")

	req := httptest.NewRequest("GET", "/getuserswithavars", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, expected 200", w.Code)
	}

	var avatars []AvatarInfo
	json.Unmarshal(w.Body.Bytes(), &avatars)

	if len(avatars) != 0 {
		t.Errorf("expected no avatars, got %d", len(avatars))
	}
}

// TestGetUsersWithAvatarsList verifies listing users with avatars
func TestGetUsersWithAvatarsList(t *testing.T) {
	mux, conn, _ := setupTestServer(t)

	// Create users
	insertTestUser(t, conn, "alice", "secret_alice")
	insertTestUser(t, conn, "bob", "secret_bob")
	insertTestUser(t, conn, "charlie", "secret_charlie")

	pngData := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, 0xDE,
	}

	// Upload avatars for alice and charlie
	for _, user := range []string{"alice", "charlie"} {
		form := url.Values{
			"data":        {base64.StdEncoding.EncodeToString(pngData)},
			"contentType": {"image/png"},
		}
		w := authedPost(mux, "/uploadavatar", user, "secret_"+user, form)
		if w.Code != http.StatusOK {
			t.Fatalf("failed to upload avatar for %s: %s", user, w.Body.String())
		}
	}

	// Get users with avatars
	req := httptest.NewRequest("GET", "/getuserswithavars", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, expected 200: %s", w.Code, w.Body.String())
	}

	var avatars []AvatarInfo
	json.Unmarshal(w.Body.Bytes(), &avatars)

	if len(avatars) != 2 {
		t.Errorf("expected 2 users with avatars, got %d", len(avatars))
	}

	// Should be sorted by screenname
	if len(avatars) >= 2 {
		if avatars[0].Screenname != "alice" || avatars[1].Screenname != "charlie" {
			t.Errorf("expected sorted order [alice, charlie], got [%s, %s]", avatars[0].Screenname, avatars[1].Screenname)
		}
	}

	// Verify all have avatar URLs
	for _, avatar := range avatars {
		if avatar.AvatarURL == "" {
			t.Errorf("user %s has empty avatarUrl", avatar.Screenname)
		}
	}
}

// TestGetUsersWithAvatarsPagination verifies pagination with continuation token
func TestGetUsersWithAvatarsPagination(t *testing.T) {
	mux, conn, _ := setupTestServer(t)

	// Create users a, b, c and upload avatars for all
	users := []string{"a", "b", "c"}
	pngData := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, 0xDE,
	}

	for _, user := range users {
		insertTestUser(t, conn, user, "secret_"+user)
		form := url.Values{
			"data":        {base64.StdEncoding.EncodeToString(pngData)},
			"contentType": {"image/png"},
		}
		w := authedPost(mux, "/uploadavatar", user, "secret_"+user, form)
		if w.Code != http.StatusOK {
			t.Fatalf("failed to upload avatar for %s: %s", user, w.Body.String())
		}
	}

	// Get first page with limit=1
	req := httptest.NewRequest("GET", "/getuserswithavars?maxct=1", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var page1 []AvatarInfo
	json.Unmarshal(w.Body.Bytes(), &page1)

	if len(page1) != 1 {
		t.Errorf("expected 1 item on page 1, got %d", len(page1))
	}

	if page1[0].Screenname != "a" {
		t.Errorf("expected first page to have user 'a', got %q", page1[0].Screenname)
	}

	// Get next page using continuation token
	ct := page1[0].Screenname
	req = httptest.NewRequest("GET", fmt.Sprintf("/getuserswithavars?ct=%s&maxct=1", ct), nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var page2 []AvatarInfo
	json.Unmarshal(w.Body.Bytes(), &page2)

	if len(page2) != 1 {
		t.Errorf("expected 1 item on page 2, got %d", len(page2))
	}

	if page2[0].Screenname != "b" {
		t.Errorf("expected second page to have user 'b', got %q", page2[0].Screenname)
	}
}

// TestGetUsersWithAvatarsDefaultLimit verifies default limit is applied
func TestGetUsersWithAvatarsDefaultLimit(t *testing.T) {
	mux, conn, _ := setupTestServer(t)

	pngData := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, 0xDE,
	}

	// Create 60 users with avatars
	for i := 0; i < 60; i++ {
		screenname := fmt.Sprintf("user%d", i)
		insertTestUser(t, conn, screenname, "secret_"+screenname)
		form := url.Values{
			"data":        {base64.StdEncoding.EncodeToString(pngData)},
			"contentType": {"image/png"},
		}
		w := authedPost(mux, "/uploadavatar", screenname, "secret_"+screenname, form)
		if w.Code != http.StatusOK {
			t.Fatalf("failed to upload avatar for %s: %s", screenname, w.Body.String())
		}
	}

	// Query without maxct (should use default of 50)
	req := httptest.NewRequest("GET", "/getuserswithavars", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var avatars []AvatarInfo
	json.Unmarshal(w.Body.Bytes(), &avatars)

	if len(avatars) != 50 {
		t.Errorf("expected default limit of 50, got %d", len(avatars))
	}
}
