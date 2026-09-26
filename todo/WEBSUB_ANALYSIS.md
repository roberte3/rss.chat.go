# WebSub Implementation Analysis for RSS.Chat Go

## Overview

Dave Winer's upstream JavaScript implementation added **WebSub (Web Push) support** on August 5, 2026. This is a significant new feature that enables real-time push notifications to feed readers using the WebSub protocol.

## What is WebSub?

WebSub (formerly PubSubHubbub) is a push-based notification protocol that allows:
- Subscribers to be notified immediately when a feed is updated
- No need for polling/checking feeds periodically
- Real-time updates (complementary to RSSCloud)
- Works with RSS, Atom, and OPML feeds

## How WebSub Works

```
1. Subscriber discovers hub via <link> header or feed element
2. Subscriber sends subscription request to hub
3. Hub verifies subscription ownership
4. When feed updates, server notifies hub via HTTP POST
5. Hub pushes update to all subscribers
```

## Upstream Implementation Details

### Configuration Options

```javascript
flWebsubEnabled: true,                              // Master on/off switch
urlWebsubHub: "https://rpc.rsscloud.io/websub"     // Hub endpoint (Andrew Shell's)
```

### Core Function: `pingWebsubHub(feedUrl)`

```javascript
function pingWebsubHub (feedUrl) {
    if (config.flWebsubEnabled) {
        const theParams = {
            "hub.mode": "publish",        // WebSub standard mode
            "hub.url": feedUrl            // Feed URL that was updated
        };
        request.post ({
            url: config.urlWebsubHub, 
            form: theParams
        }, function (err) {
            if (err) {
                console.log ("pingWebsubHub error: " + err.message);
            }
        });
    }
}
```

**Parameters Sent to Hub:**
- `hub.mode=publish` — Standard WebSub notification
- `hub.url=<feed-url>` — Which feed was updated

### Feed Response Headers

When serving feed responses, the server includes WebSub hub information:

```javascript
if ((config.flWebsubEnabled) && (fileRec.type === "text/xml")) {
    const selfUrl = config.urlServerForClient + theRequest.lowerpath.substring(1);
    theHeaders = {
        link: "<" + config.urlWebsubHub + ">; rel=\"hub\", <" + selfUrl + ">; rel=\"self\""
    };
}
```

**Header Format:**
```
Link: <https://rpc.rsscloud.io/websub>; rel="hub", <https://example.com/feed.xml>; rel="self"
```

This header tells subscribers:
- Where the hub is located (`rel="hub"`)
- What the feed's canonical URL is (`rel="self"`)

### Notification Timing

The `pingWebsubHub()` is called when:

1. **User feeds are published** (via `updateFeedsOnS3()`)
   ```javascript
   rss.cloudPing(undefined, feedUrl);
   pingWebsubHub(feedUrl);  // Right after RSSCloud ping
   ```

2. **"Everyone" feed is published**
   ```javascript
   rss.cloudPing(undefined, everyoneFeedUrl);
   pingWebsubHub(everyoneFeedUrl);  // Notification sent
   ```

3. **Any XML feed is served** (headers included)
   - User feeds
   - Global feed
   - Comments feeds
   - OPML subscription lists

## Comparison with RSSCloud

| Feature | RSSCloud | WebSub | RSS.Chat Uses |
|---------|----------|--------|---------------|
| Protocol | Custom XML-RPC | HTTP POST | Both |
| Hub URL | rpc.rsscloud.io | Configurable | Separate config |
| Real-time | Yes | Yes | Yes |
| Standards | Older | Modern (W3C) | Both |
| Configuration | Hardcoded | Configurable | Pluggable |
| Default Hub | Andrew Shell's | Andrew Shell's | Same (unified) |

**Key Insight:** Andrew Shell's hub (rpc.rsscloud.io) speaks both protocols over one unified subscriber list, so adding WebSub doesn't break existing RSSCloud readers.

## Go Implementation Strategy

### Phase 1: Core Infrastructure

**1. Config Extensions** (`config/config.go`)
```go
type Config struct {
    // ... existing fields ...
    FlWebsubEnabled bool
    URLWebsubHub    string
}
```

**2. Configuration Loading** (in config loading logic)
```json
{
    "flWebsubEnabled": true,
    "urlWebsubHub": "https://rpc.rsscloud.io/websub"
}
```

### Phase 2: Core Functionality

**3. WebSub Pinger** (new file: `websub/pinger.go`)
```go
package websub

import "net/http"

type Pinger struct {
    Enabled bool
    HubURL  string
}

func (p *Pinger) Ping(feedURL string) error {
    if !p.Enabled {
        return nil
    }
    
    params := map[string][]string{
        "hub.mode": {"publish"},
        "hub.url":  {feedURL},
    }
    
    resp, err := http.PostForm(p.HubURL, params)
    if err != nil {
        // Log error, don't fail
        return err
    }
    defer resp.Body.Close()
    
    return nil
}
```

**4. Header Generation** (in feed serving code)
```go
func buildWebsubHeaders(feedURL string, hubURL string, enabled bool) map[string]string {
    if !enabled {
        return nil
    }
    
    return map[string]string{
        "Link": fmt.Sprintf(
            "<%s>; rel=\"hub\", <%s>; rel=\"self\"",
            hubURL,
            feedURL,
        ),
    }
}
```

### Phase 3: Integration Points

**5. Feed Publishing** (modify `publish/publisher.go`)
- Call WebSub pinger when publishing user feeds
- Call WebSub pinger when publishing global feed
- Handle errors gracefully (don't block publishing if WebSub fails)

**6. Feed Serving** (modify `feed/builder.go` or feed handler)
- Add WebSub headers to XML feed responses
- Apply to: user feeds, global feed, comments feeds, OPML

### Phase 4: Testing

**7. Unit Tests** (`websub/pinger_test.go`)
- Test ping succeeds with correct parameters
- Test ping handles disabled config gracefully
- Test error handling (network failures)

**8. Integration Tests** (`api/endpoint_test.go`)
- Verify WebSub headers in feed responses
- Verify pings are called on feed updates
- Verify configuration options work

## Implementation Checklist

### Code Changes Required
- [ ] Add config fields (`flWebsubEnabled`, `urlWebsubHub`)
- [ ] Create `websub/pinger.go` with core ping logic
- [ ] Update feed builder to include WebSub headers
- [ ] Integrate pinger into feed publishing flow
- [ ] Add tests for WebSub functionality

### Configuration
- [ ] Add default values to config.json template
- [ ] Document in README
- [ ] Update CLAUDE.md with WebSub info

### Testing
- [ ] Unit tests for pinger
- [ ] Integration tests for feed headers
- [ ] Test with real hub (optional, for validation)

### Documentation
- [ ] Update CLAUDE.md with WebSub architecture
- [ ] Document configuration options
- [ ] Add to implementation status notes

## Risk Assessment

### Low Risk
- Optional feature (disabled/configured separately)
- Doesn't affect existing RSSCloud support
- Errors don't break feed publishing
- Can be added without schema changes

### Implementation Notes
- **Error Handling:** WebSub pings are "fire and forget" — errors should log but not fail
- **Concurrency:** Pings should be non-blocking (goroutine or async)
- **Configuration:** Should support runtime updates if config reloads
- **Backward Compatibility:** Not breaking — existing servers work unchanged

## Default Configuration

Recommend matching upstream defaults:
```json
{
    "flWebsubEnabled": true,
    "urlWebsubHub": "https://rpc.rsscloud.io/websub"
}
```

This allows immediate interoperability with Andrew Shell's hub, which is the reference implementation and widely used.

## Performance Impact

- **Ping requests:** One HTTP POST per feed update (negligible overhead)
- **Response headers:** Small string added to feed responses (negligible impact)
- **CPU:** Essentially zero
- **Network:** Minimal (one small POST per update, async)
- **Scalability:** Non-blocking, can scale with goroutines

## Security Considerations

✅ **Already built-in to upstream design:**
- Hub URL is configurable (can be internal/trusted)
- Only sends public feed URLs to hub
- No credentials or secrets in ping payload
- Hub is HTTPS by default
- Authorization handled by hub, not this server

## Standards Compliance

WebSub (WebSubHubbub) Specification:
- RFC/Specification: [W3C Community Group Note](https://www.w3.org/TR/websub/)
- Protocol: HTTP POST with hub.mode=publish
- Response codes: Any 2xx is success
- Verification: Not needed for "publish" mode

## Upstream Reference

**Files to reference:**
- `server/code/rssnetwork.js` — Lines 1134-1268 (pingWebsubHub, updateFeedsOnS3)
- `server/code/rssnetwork.js` — Lines 2458-2475 (feed response headers)

**Key commits:**
- 0a77f7b (2026-08-05) — WebSub addition with docs
- 87d8d8a (2026-08-01) — Earlier related changes

## Timeline Estimate

- **Phase 1 (Config):** 30 minutes
- **Phase 2 (Core):** 1-2 hours
- **Phase 3 (Integration):** 1-2 hours
- **Phase 4 (Testing):** 2-3 hours
- **Total:** ~5-7 hours for complete implementation

Can be done incrementally or in one sprint.

## Related Issues

- Connects to: [[Phase 8 Testing Complete]] for test infrastructure
- Related to: Feed publishing and generation system
- Complements: Existing RSSCloud support in `feed/` and `publish/` packages

## Next Steps

1. Review this analysis with team
2. Decide on priority (could defer or implement now)
3. If implementing: Start with Phase 1 (config) and Phase 2 (core)
4. Add comprehensive tests before merging
