package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type blueskyItem struct {
	AtURI     string
	Handle    string
	DID       string
	Rkey      string
	CID       string
	CreatedAt time.Time
	Text      string
	RawJSON   string
}

type listRecordsResponse struct {
	Records []struct {
		URI   string          `json:"uri"`
		CID   string          `json:"cid"`
		Value json.RawMessage `json:"value"`
	} `json:"records"`
	Cursor string `json:"cursor"`
}

type postRecordValue struct {
	Type      string          `json:"$type"`
	Text      string          `json:"text"`
	CreatedAt time.Time       `json:"createdAt"`
	Reply     json.RawMessage `json:"reply,omitempty"`
}

// fetchStats reports what happened to every record examined during a
// fetchTopLevelPostsSince call, so a caller (and whoever's reading its logs)
// can tell "the window was narrow" apart from "most of this was replies"
// apart from "records were silently unparseable" -- three very different
// reasons for coming back with fewer posts than expected.
type fetchStats struct {
	RecordsSeen      int
	MalformedSkipped int
	RepliesSkipped   int
}

// fetchTopLevelPostsSince pages com.atproto.repo.listRecords for did's
// app.bsky.feed.post collection -- newest first -- stopping once records
// fall before since. Replies are dropped rather than stored: v1 policy is
// top-level posts only (see notes/feature-bluesky-bridge-utility.md); a
// reply needs its parent's rss.chat item ID to thread correctly, which is
// deferred.
func fetchTopLevelPostsSince(ctx context.Context, client *http.Client, pdsEndpoint, handle, did string, since time.Time) ([]blueskyItem, fetchStats, error) {
	var out []blueskyItem
	var stats fetchStats
	cursor := ""

	for {
		q := url.Values{}
		q.Set("repo", did)
		q.Set("collection", "app.bsky.feed.post")
		q.Set("limit", "100")
		if cursor != "" {
			q.Set("cursor", cursor)
		}
		reqURL := strings.TrimRight(pdsEndpoint, "/") + "/xrpc/com.atproto.repo.listRecords?" + q.Encode()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			return nil, stats, err
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, stats, err
		}

		var page listRecordsResponse
		decodeErr := json.NewDecoder(resp.Body).Decode(&page)
		statusCode := resp.StatusCode
		resp.Body.Close()

		if statusCode != http.StatusOK {
			return nil, stats, fmt.Errorf("listRecords(%s): status %d", did, statusCode)
		}
		if decodeErr != nil {
			return nil, stats, decodeErr
		}

		stop := false
		for _, rec := range page.Records {
			stats.RecordsSeen++

			var val postRecordValue
			if err := json.Unmarshal(rec.Value, &val); err != nil {
				stats.MalformedSkipped++
				log.Printf("warning: skipping unparseable record %s for %s: %v", rec.URI, handle, err)
				continue
			}
			if val.CreatedAt.Before(since) {
				stop = true
				break
			}
			if len(val.Reply) > 0 {
				stats.RepliesSkipped++
				continue
			}
			out = append(out, blueskyItem{
				AtURI:     rec.URI,
				Handle:    handle,
				DID:       did,
				Rkey:      rkeyFromURI(rec.URI),
				CID:       rec.CID,
				CreatedAt: val.CreatedAt,
				Text:      val.Text,
				RawJSON:   string(rec.Value),
			})
		}

		if stop || page.Cursor == "" || len(page.Records) == 0 {
			break
		}
		cursor = page.Cursor
	}

	return out, stats, nil
}

func rkeyFromURI(atURI string) string {
	idx := strings.LastIndex(atURI, "/")
	if idx == -1 {
		return atURI
	}
	return atURI[idx+1:]
}
