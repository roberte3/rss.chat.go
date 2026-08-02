package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// fetchAvatarURL asks appviewBase's app.bsky.actor.getProfile for actor's
// current avatar. actor may be a handle or a DID; the handle is enough, so
// callers don't need to wait on DID resolution first. Returns "" (no error)
// when the account has no avatar set -- that's a normal, common case, not a
// failure.
func fetchAvatarURL(ctx context.Context, client *http.Client, appviewBase, actor string) (string, error) {
	reqURL := strings.TrimRight(appviewBase, "/") +
		"/xrpc/app.bsky.actor.getProfile?actor=" + url.QueryEscape(actor)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("getProfile(%s): status %d", actor, resp.StatusCode)
	}

	var out struct {
		Avatar string `json:"avatar"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return out.Avatar, nil
}
