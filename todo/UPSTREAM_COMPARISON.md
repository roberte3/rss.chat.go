# Upstream Comparison: RSS.Chat Go vs Dave's JavaScript Implementation

**Date:** September 12, 2026  
**Go Repo Last Sync:** August 26, 2026 (commit 14408cb)  
**Upstream Latest:** August 5, 2026 (commit 0a77f7b)  
**Time Since Sync:** ~17 days

## Status: No Recent Changes in Last 7 Days

The upstream JavaScript repository has **not been updated in the last 7 days** (since 2026-09-05). The most recent commit was on 2026-08-05 (over a month ago).

## Changes Since Last Go Sync (2026-08-26)

The Go implementation last synced the upstream client on 2026-08-01 (commit 15df560), but the most recent documentation update was 2026-08-26 (commit 14408cb, which archived API changes). Since then, **no new changes have been made to the upstream repository**.

However, there were significant changes made **between the last Go sync point and the latest upstream**:

### 1. **WebSub (Web Push) Support** — CRITICAL ADDITION
**Date:** August 5, 2026 (CC - likely Claude/contributor)  
**Version:** 0.6.14  
**Files Changed:** `server/code/rssnetwork.js`, `server/code/package.json`

#### What was added:
- **New Config Options:**
  ```javascript
  flWebsubEnabled: true,                           // Enable/disable WebSub
  urlWebsubHub: "https://rpc.rsscloud.io/websub"  // WebSub hub URL
  ```

- **New Function:** `pingWebsubHub(feedUrl)`
  ```javascript
  function pingWebsubHub (feedUrl) {
      if (config.flWebsubEnabled) {
          const theParams = {
              "hub.mode": "publish",
              "hub.url": feedUrl
          };
          request.post ({url: config.urlWebsubHub, form: theParams}, function (err) {
              if (err) {
                  console.log ("pingWebsubHub error: " + err.message);
              }
          });
      }
  }
  ```

- **WebSub Hub Announcements:** Feed responses now include WebSub hub link headers:
  ```javascript
  if ((config.flWebsubEnabled) && (fileRec.type === "text/xml")) {
      const selfUrl = config.urlServerForClient + theRequest.lowerpath.substring(1);
      theHeaders = {
          link: "<" + config.urlWebsubHub + ">; rel=\"hub\", <" + selfUrl + ">; rel=\"self\""
      };
  }
  ```

- **Activation:** Called in `updateFeedsOnS3()` alongside existing `rss.cloudPing()` calls:
  - When user feed is rebuilt
  - When "everyone" feed is rebuilt
  - Applies to: user feeds, everyone feed, and OPML subscription lists

#### Impact on Go Implementation:
**HIGH IMPACT** — This is a new feature that should be implemented to maintain parity with upstream. It allows readers using WebSub protocol to follow RSS.Chat feeds in real-time, complementing the existing RSSCloud support.

**Work Required:**
- [ ] Add config options for WebSub (enabled flag and hub URL)
- [ ] Implement `pingWebsubHub()` function
- [ ] Add WebSub link headers to feed responses
- [ ] Integrate with feed publishing workflow

---

### 2. **Feed URL Construction Fix** — BUG FIX
**Date:** August 2, 2026  
**Severity:** Medium  
**Version:** 0.6.12

#### What changed:
When building the `<link>` element in RSS feeds, the upstream code was using hardcoded URL construction:
```javascript
// OLD (WRONG):
headElements.link = "http://" + config.myDomain + "/";

// NEW (CORRECT):
headElements.link = config.urlServerForClient;  // Already includes scheme and domain
```

Applied in two places:
1. `buildFeedForUser()` - User's personal feed link
2. `buildFeedForEveryone()` - Global "everyone" feed link
3. `pingCloud()` - RSS Cloud ping URL

#### Why it matters:
- If `config.myDomain` includes HTTPS, the hardcoded `http://` would be wrong
- The existing `config.urlServerForClient` is already properly configured with scheme
- This matches the CLAUDE.md documentation in the Go repo

#### Impact on Go Implementation:
**MINIMAL** — The Go implementation likely already does this correctly, but should verify the feed URL construction matches this pattern.

**Verification Needed:**
- [ ] Check `feed/builder.go` to ensure URLs use proper base URL (likely already correct)
- [ ] Confirm feed responses use configured scheme (http/https) properly

---

### 3. **Security Enhancement: /readhttpfile Authorization** — SECURITY FIX
**Date:** August 1, 2026  
**Severity:** HIGH  
**Version:** 0.6.11

#### What changed:
The `/readhttpfile` endpoint was refactored to include authorization checks:

```javascript
// NEW: handleReadHttpFile function
function handleReadHttpFile (url, callback) {
    if (url == config.urlMenuOpml) {
        httpRequest (url, undefined, undefined, function (err, filetext) {
            if (err) {
                callback (err);
            } else {
                callback (undefined, {filetext});
            }
        });
    } else {
        const message = "Can't read the file because it is not authorized.";
        callback ({message});
    }
}
```

The endpoint now:
- Only allows reading files from the URL specified in `config.urlMenuOpml`
- Rejects all other URLs with an authorization error
- Prevents SSRF (Server-Side Request Forgery) attacks

#### Impact on Go Implementation:
**CRITICAL** — The Go implementation documents this as not implemented due to SSRF concerns. If it is ever implemented, it **MUST** include this authorization check.

From CLAUDE.md:
> Upstream's version takes a `url` parameter and returns whatever it fetches, with no authentication, scheme check, host check or size limit — an unauthenticated SSRF that would let any caller use the server to reach cloud metadata endpoints and private-network hosts.

The upstream version has now fixed this. If you implement `/readhttpfile` in Go:
- [ ] Only allow URLs matching `config.urlMenuOpml`
- [ ] Validate scheme is http/https only
- [ ] Check resolved addresses against loopback/private/link-local ranges
- [ ] Validate size limits and timeouts
- [ ] Require this authorization before implementing

---

### 4. **Version Updates**
- **Version:** 0.6.10 → 0.6.14
- **npm package.json:** Updated to reflect new version

---

## Summary Table

| Change | Date | Severity | Status | Impact on Go |
|--------|------|----------|--------|------------|
| WebSub Support | 8/5/26 | HIGH | New Feature | Needs Implementation |
| Feed URL Fix | 8/2/26 | MEDIUM | Bug Fix | Verify Correct |
| /readhttpfile Authorization | 8/1/26 | HIGH | Security | Document/Plan |
| Version Bump | 8/5/26 | LOW | Administrative | Update Docs |

---

## Recommendations

### Immediate (This Sprint)
1. ✅ **Document WebSub changes** — Created this comparison
2. **Verify feed URL construction** — Ensure Go implementation uses configured base URL correctly
3. **Update implementation notes** — Document that upstream has fixed /readhttpfile with authorization

### Near-term (Next Sprint)
1. **Plan WebSub implementation** — Design how to integrate with Go's feed publishing
2. **Test feed URL behavior** — Ensure HTTP/HTTPS handling matches upstream

### Future
1. **Implement WebSub support** — Add this feature to match upstream capability
2. **Consider /readhttpfile** — If ever planned, build with authorization from the start

---

## Files to Review in Upstream

- `/tmp/rss.chat/server/code/rssnetwork.js` — Main changes
- `/tmp/rss.chat/server/code/worknotes.md` — Change documentation
- `/tmp/rss.chat/server/code/source.opml` — Architecture notes in outline format

## Next Steps

1. Create implementation plan for WebSub support
2. Verify feed URL construction in Go implementation
3. Document in project notes for future reference
