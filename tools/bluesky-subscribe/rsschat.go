package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type localNewUserResponse struct {
	Screenname  string `json:"screenname"`
	Email       string `json:"email"`
	EmailSecret string `json:"emailSecret"`
}

// provisionLocalUser calls rss.chat's /localnewuser -- loopback-only by
// design, see api/localnewuser.go -- to create or fetch the synthetic
// account a tracked handle posts as, without an email confirmation round
// trip. Idempotent: an already-provisioned screenname just gets its stored
// secret back.
func provisionLocalUser(ctx context.Context, client *http.Client, rssChatURL, screenname, email string) (localNewUserResponse, error) {
	q := url.Values{}
	q.Set("screenname", screenname)
	q.Set("email", email)
	reqURL := strings.TrimRight(rssChatURL, "/") + "/localnewuser?" + q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return localNewUserResponse{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return localNewUserResponse{}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return localNewUserResponse{}, fmt.Errorf("localnewuser(%s): status %d: %s",
			screenname, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var out localNewUserResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return localNewUserResponse{}, err
	}
	if out.EmailSecret == "" {
		return localNewUserResponse{}, fmt.Errorf(
			"localnewuser(%s): response had no emailSecret -- is this tool running on the same host as the rss.chat server?", screenname)
	}
	return out, nil
}

// forwardPost posts item to rss.chat as the given synthetic account via the
// existing authenticated /newpost endpoint. Description is the Bluesky
// text plus a permalink back to the original post, for provenance -- rss.chat
// stays a discovery/aggregation surface, not a silo for the source content.
func forwardPost(ctx context.Context, client *http.Client, rssChatURL, email, secret string, item blueskyItem) (int64, error) {
	permalink := fmt.Sprintf("https://bsky.app/profile/%s/post/%s", item.Handle, item.Rkey)
	description := item.Text
	if description != "" {
		description += "\n\n"
	}
	description += permalink

	jsontext, err := json.Marshal(map[string]string{"description": description})
	if err != nil {
		return 0, err
	}

	form := url.Values{}
	form.Set("emailaddress", email)
	form.Set("emailcode", secret)
	form.Set("jsontext", string(jsontext))

	reqURL := strings.TrimRight(rssChatURL, "/") + "/newpost"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, strings.NewReader(form.Encode()))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("newpost(%s): status %d: %s", item.AtURI, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var out struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return 0, err
	}
	return out.ID, nil
}

// fetchUserPrefs reads a screenname's current prefs via the public
// /getuserdata endpoint (no auth needed for a read). Returns an empty map
// for a user with no prefs set yet, rather than nil, so callers can add keys
// to it directly.
func fetchUserPrefs(ctx context.Context, client *http.Client, rssChatURL, screenname string) (map[string]interface{}, error) {
	reqURL := strings.TrimRight(rssChatURL, "/") + "/getuserdata?screenname=" + url.QueryEscape(screenname)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("getuserdata(%s): status %d: %s", screenname, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var out struct {
		Prefs json.RawMessage `json:"prefs"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}

	prefs := map[string]interface{}{}
	if len(out.Prefs) > 0 {
		if err := json.Unmarshal(out.Prefs, &prefs); err != nil {
			return nil, err
		}
	}
	return prefs, nil
}

// saveUserPrefs writes prefs back via /saveprefs. UpdateUserPrefs on the
// server side replaces the whole prefs blob rather than merging it, so
// callers must pass the full map -- see fetchUserPrefs, which is meant to be
// read, modified, and passed straight to this function.
func saveUserPrefs(ctx context.Context, client *http.Client, rssChatURL, email, secret string, prefs map[string]interface{}) error {
	jsontext, err := json.Marshal(prefs)
	if err != nil {
		return err
	}

	form := url.Values{}
	form.Set("emailaddress", email)
	form.Set("emailcode", secret)
	form.Set("jsontext", string(jsontext))

	reqURL := strings.TrimRight(rssChatURL, "/") + "/saveprefs"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("saveprefs(%s): status %d: %s", email, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

// syncAvatar sets myAvatarImageUrl in screenname's prefs to avatarURL,
// merging onto whatever prefs already exist there rather than clobbering
// them. This is the same key feed/builder.go and db/items.go already read
// avatars from, so no server-side change was needed.
func syncAvatar(ctx context.Context, client *http.Client, rssChatURL, screenname, email, secret, avatarURL string) error {
	prefs, err := fetchUserPrefs(ctx, client, rssChatURL, screenname)
	if err != nil {
		return fmt.Errorf("fetching current prefs: %w", err)
	}
	prefs["myAvatarImageUrl"] = avatarURL
	if err := saveUserPrefs(ctx, client, rssChatURL, email, secret, prefs); err != nil {
		return fmt.Errorf("saving prefs: %w", err)
	}
	return nil
}
