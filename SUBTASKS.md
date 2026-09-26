# Detailed Subtasks for RSS.Chat Go — All Tiers

**Last Updated**: 2026-09-25  
**Format**: Agent-ready subtasks with acceptance criteria and success metrics

---

## TIER 1: Feature Parity with Upstream (v0.6.14)

These tasks are blocking for feature parity with Dave Winer's implementation.

### T1-1: WebSub Configuration & Config Parsing

**Epic**: WebSub (Web Push) Support  
**Effort**: 1-2 hours  
**Prerequisites**: None  
**Blocks**: T1-2, T1-3, T1-5, T1-6

**Description**:
Add configuration fields for WebSub protocol support to config.go and ensure they load/validate correctly.

**Subtasks**:

1. **T1-1a**: Add config struct fields
   - [ ] Open `config/config.go`
   - [ ] Add two new fields to `Config` struct:
     - `FlWebsubEnabled bool` (matches upstream camelCase `flWebsubEnabled`)
     - `URLWebsubHub string` (matches upstream `urlWebsubHub`)
   - [ ] Ensure JSON tags match: `json:"flWebsubEnabled"`, `json:"urlWebsubHub"`
   - **Acceptance**: Fields exist, compile succeeds, JSON tags are correct

2. **T1-1b**: Update config validation
   - [ ] Open `config/config.go` and find existing validation logic
   - [ ] Add validation for `URLWebsubHub`:
     - Must be valid HTTPS URL if `FlWebsubEnabled` is true
     - Should use `url.Parse()` and check scheme is `https`
     - Return error with message "invalid urlWebsubHub, must be HTTPS" if invalid
   - **Acceptance**: Validation rejects invalid URLs, allows valid ones

3. **T1-1c**: Update config defaults
   - [ ] Find `config.example.json` in root
   - [ ] Add two fields with upstream defaults:
     ```json
     "flWebsubEnabled": true,
     "urlWebsubHub": "https://rpc.rsscloud.io/websub"
     ```
   - [ ] Update config.go's Load() to set these defaults if missing
   - **Acceptance**: example.json has both fields; -setup populates them

4. **T1-1d**: Add config tests
   - [ ] Open `config/config_test.go`
   - [ ] Add test: WebSub config loads with defaults
   - [ ] Add test: Validation rejects invalid hub URL (e.g., http://, malformed)
   - [ ] Add test: Both fields are JSON marshaled/unmarshaled correctly
   - **Acceptance**: All 3 tests pass, coverage includes WebSub paths

---

### T1-2: WebSub Pinger Core Implementation

**Epic**: WebSub (Web Push) Support  
**Effort**: 2-3 hours  
**Prerequisites**: T1-1 (config)  
**Blocks**: T1-4, T1-5, T1-6

**Description**:
Implement the WebSub pinger that sends HTTP POST notifications to the hub when feeds are published.

**Subtasks**:

1. **T1-2a**: Create websub package structure
   - [ ] Create directory `websub/` in project root
   - [ ] Create file `websub/pinger.go`
   - [ ] Add package declaration and imports (net/http, net/url, log)
   - **Acceptance**: Directory and file exist; no compiler errors

2. **T1-2b**: Implement Pinger struct and constructor
   - [ ] In `websub/pinger.go`:
     ```go
     type Pinger struct {
         Enabled bool
         HubURL  string
     }
     
     func NewPinger(enabled bool, hubURL string) *Pinger {
         return &Pinger{
             Enabled: enabled,
             HubURL:  hubURL,
         }
     }
     ```
   - **Acceptance**: Code compiles, struct has Enabled and HubURL fields

3. **T1-2c**: Implement Ping method
   - [ ] Add `Ping(feedURL string) error` method to Pinger:
     - Return nil immediately if `!p.Enabled`
     - Create URL form data: `hub.mode=publish` and `hub.url={feedURL}`
     - Use `http.PostForm()` to POST to `p.HubURL`
     - If POST fails, log the error but don't return error (fire-and-forget)
     - Return nil in all cases
   - [ ] Reference: `/tmp/rss.chat/server/code/rssnetwork.js` lines 1134-1268
   - **Acceptance**: 
     - Disabled pinger returns nil immediately
     - POST called with correct params when enabled
     - Errors logged but not returned
     - Test manually: curl shows POST data format matches spec

4. **T1-2d**: Add pinger tests
   - [ ] Create `websub/pinger_test.go`
   - [ ] Test 1: Disabled pinger returns nil without calling POST
   - [ ] Test 2: Enabled pinger calls http.PostForm with correct params
   - [ ] Test 3: Network error is logged but returns nil
   - [ ] Test 4: Verify params are sent as form data (not JSON)
   - [ ] Use httptest.Server to mock the hub for testing
   - **Acceptance**: All 4 tests pass; coverage > 90%

---

### T1-3: WebSub Headers in Feed Responses

**Epic**: WebSub (Web Push) Support  
**Effort**: 1-2 hours  
**Prerequisites**: T1-1 (config)  
**Blocks**: T1-5, T1-6

**Description**:
Add WebSub `Link` headers to feed responses so feed readers can discover the hub.

**Subtasks**:

1. **T1-3a**: Identify feed response locations
   - [ ] Search codebase for where feed responses are sent
   - [ ] Find all HTTP handlers that return XML/RSS feeds:
     - User feeds (`/feed` endpoint for individual users)
     - Global feed (`/feed` endpoint for everyone)
     - Comments feeds
     - OPML feeds (`/getsubscriptionlist`)
   - [ ] Note which files contain these handlers (likely in `api/` or `feed/`)
   - **Acceptance**: List of 4+ feed response points identified

2. **T1-3b**: Create header builder function
   - [ ] In `websub/pinger.go` or new `websub/headers.go`:
     ```go
     func BuildWebsubHeader(feedURL, hubURL string, enabled bool) (string, bool) {
         if !enabled {
             return "", false
         }
         return fmt.Sprintf(
             "<%s>; rel=\"hub\", <%s>; rel=\"self\"",
             hubURL, feedURL,
         ), true
     }
     ```
   - **Acceptance**: Compiles; returns correct Link header format

3. **T1-3c**: Add headers to each feed response
   - [ ] For each feed response point identified in T1-3a:
     - Get `config.FlWebsubEnabled` and `config.URLWebsubHub`
     - Get the feed URL being served
     - Call `BuildWebsubHeader()` to create header string
     - Add to response: `w.Header().Set("Link", headerValue)`
   - [ ] Ensure header only added for XML/RSS responses (not JSON)
   - **Acceptance**: 
     - All 4+ feed types have header added
     - Header format matches WebSub spec
     - Only XML responses have it

4. **T1-3d**: Add response header tests
   - [ ] In existing endpoint tests (likely `api/endpoint_test.go`):
   - [ ] Test 1: Feed response includes Link header with hub and self
   - [ ] Test 2: Header format matches WebSub spec exactly
   - [ ] Test 3: Header absent when WebSub disabled
   - [ ] Test 4: Header present for all feed types (user, global, comments, OPML)
   - **Acceptance**: All 4 tests pass

---

### T1-4: WebSub Integration with Feed Publishing

**Epic**: WebSub (Web Push) Support  
**Effort**: 2-3 hours  
**Prerequisites**: T1-2 (pinger implementation)  
**Blocks**: T1-5, T1-6

**Description**:
Wire the WebSub pinger into the feed publishing workflow so hubs are notified when feeds update.

**Subtasks**:

1. **T1-4a**: Identify feed publishing points
   - [ ] Open `publish/publisher.go` (or equivalent)
   - [ ] Find all places where feeds are published:
     - User feeds
     - Global/everyone feed
     - Comments feeds (if published, not just built on-demand)
   - [ ] Note the function names and file locations
   - **Acceptance**: 2+ publishing points identified

2. **T1-4b**: Add Pinger to Publisher struct
   - [ ] Find `Publisher` struct definition
   - [ ] Add field: `WebsubPinger *websub.Pinger`
   - [ ] Update Publisher constructor/factory to accept pinger
   - [ ] In `main.go`, pass pinger when creating Publisher:
     ```go
     pinger := websub.NewPinger(cfg.FlWebsubEnabled, cfg.URLWebsubHub)
     // pass to publisher
     ```
   - **Acceptance**: Publisher has Pinger field; main.go wires it

3. **T1-4c**: Call pinger on user feed publish
   - [ ] Find the function that publishes user feeds
   - [ ] After successful feed publish:
     - Build the feed URL (e.g., `config.URLServerForClient + "/feed?screenname=...&format=xml"`)
     - Call `p.WebsubPinger.Ping(feedURL)`
   - [ ] Don't block on ping result (fire-and-forget already handled in Ping())
   - **Acceptance**: Pinger called with correct feed URL after publish

4. **T1-4d**: Call pinger on global feed publish
   - [ ] Find the function that publishes the "everyone" feed
   - [ ] After successful publish:
     - Build feed URL (e.g., `config.URLServerForClient + "/feed?format=xml"`)
     - Call `p.WebsubPinger.Ping(feedURL)`
   - **Acceptance**: Global feed ping called with correct URL

5. **T1-4e**: Add publishing tests
   - [ ] In `publish/publisher_test.go`:
   - [ ] Test 1: Publishing user feed triggers WebSub ping
   - [ ] Test 2: Ping called with correct feed URL
   - [ ] Test 3: Publish succeeds even if ping fails
   - [ ] Test 4: Ping not called when WebSub disabled
   - **Acceptance**: All 4 tests pass

---

### T1-5: WebSub Integration Tests (End-to-End)

**Epic**: WebSub (Web Push) Support  
**Effort**: 2-3 hours  
**Prerequisites**: T1-2, T1-3, T1-4  
**Blocks**: T1-7

**Description**:
Comprehensive integration tests ensuring WebSub works end-to-end from config to feed responses to hub notifications.

**Subtasks**:

1. **T1-5a**: Create integration test helpers
   - [ ] In test file (new `api/websub_integration_test.go` or add to `endpoint_test.go`):
   - [ ] Helper 1: Mock hub server for testing
     ```go
     func createMockWebsubHub(t *testing.T) (hubURL string, received *[]string) {
         // Return URL and pointer to slice of feed URLs pings
     }
     ```
   - [ ] Helper 2: Create server with WebSub enabled
     ```go
     func setupTestServerWithWebsub(t *testing.T, hubURL string) (*mux, *sql.DB, *http.Handler) {
         // Setup similar to setupTestServer but with WebSub enabled
     }
     ```
   - **Acceptance**: Helpers compile; can mock WebSub hub

2. **T1-5b**: Test full workflow
   - [ ] Test: User creation → post → feed publish → WebSub ping
     - Create user
     - Create post
     - Trigger feed publish
     - Verify mock hub received ping
   - [ ] Verify ping contains correct feed URL
   - **Acceptance**: Test passes; verifies full workflow

3. **T1-5c**: Test feed response headers
   - [ ] Test: Request to feed endpoint includes WebSub headers
     - GET `/feed?screenname=user&format=xml`
     - Verify `Link` header present
     - Verify header includes hub URL and feed URL
   - **Acceptance**: Test passes; header format correct

4. **T1-5d**: Test configuration variations
   - [ ] Test 1: WebSub disabled → no pings, no headers
   - [ ] Test 2: WebSub enabled with custom hub → pings go to custom hub
   - [ ] Test 3: Multiple feed updates → multiple pings sent
   - **Acceptance**: All 3 tests pass

---

### T1-6: Documentation & Examples

**Epic**: WebSub (Web Push) Support  
**Effort**: 1 hour  
**Prerequisites**: T1-1, T1-2, T1-3, T1-4, T1-5  
**Blocks**: None

**Description**:
Update project documentation to explain WebSub feature.

**Subtasks**:

1. **T1-6a**: Update README.md
   - [ ] Open `README.md`
   - [ ] Find "Features" or "Capabilities" section
   - [ ] Add bullet point under real-time/feed features:
     - "WebSub (Web Push) support for instant feed notifications"
   - [ ] Add to configuration section:
     - Explain `flWebsubEnabled` and `urlWebsubHub`
     - Default hub URL is Andrew Shell's unified hub
     - Can be disabled by setting `flWebsubEnabled: false`
   - **Acceptance**: README updated; WebSub mentioned in features

2. **T1-6b**: Update CLAUDE.md
   - [ ] Open `CLAUDE.md`
   - [ ] Add section under "Feed Generation" or similar:
     ```markdown
     ### WebSub Support
     
     Feeds include WebSub hub discovery headers (`Link` rel=hub/self)
     and notify the configured hub when feeds are published.
     
     - Config: `flWebsubEnabled`, `urlWebsubHub`
     - Published via `websub.Pinger` in publishing workflow
     - Complements RSSCloud support for real-time notifications
     ```
   - **Acceptance**: CLAUDE.md has WebSub section

3. **T1-6c**: Update config.example.json comments
   - [ ] Open `config.example.json`
   - [ ] Add inline comment above `flWebsubEnabled`:
     - Explain what WebSub is (real-time push notifications)
     - Note it requires the feed URL to be accessible to hub
   - **Acceptance**: Comments explain purpose

---

### T1-7: Feed URL Construction Verification

**Epic**: Fix HTTPS deployment issues  
**Effort**: 1-2 hours  
**Prerequisites**: None  
**Blocks**: None

**Description**:
Verify that feed URLs use the configured base URL (which includes scheme), not hardcoded `http://`.

**Subtasks**:

1. **T1-7a**: Audit feed URL construction
   - [ ] Open `feed/builder.go` (or wherever URLs are built)
   - [ ] Search for all places URLs are constructed for feeds
   - [ ] Check for hardcoded `http://` or `https://` prefixes
   - [ ] List all locations that build URLs:
     - Feed URLs in XML `<link>` elements
     - Feed URLs in response headers
     - Self URLs in WebSub headers
   - **Acceptance**: Complete list of URL construction points

2. **T1-7b**: Verify BaseURL usage
   - [ ] For each location found in T1-7a:
     - Verify it uses `config.BaseURL` or `config.URLServerForClient`
     - These should already include scheme (http/https)
     - Should NOT prepend scheme manually
   - [ ] If any hardcoded schemes found, replace with config-driven URLs
   - **Acceptance**: 
     - No hardcoded `http://` or `https://` found in URL construction
     - All URLs built from config.BaseURL or URLServerForClient

3. **T1-7c**: Add URL construction tests
   - [ ] In `feed/builder_test.go`:
   - [ ] Test 1: HTTPS BaseURL → feed URLs use https
   - [ ] Test 2: HTTP BaseURL → feed URLs use http
   - [ ] Test 3: URLs in XML `<link>` elements match constructed URLs
   - [ ] Test 4: WebSub self URL matches feed URL
   - **Acceptance**: All 4 tests pass

4. **T1-7d**: Document in CLAUDE.md
   - [ ] Open `CLAUDE.md`
   - [ ] Find section about BaseURL
   - [ ] Add note: "Feed URL construction verified to respect BaseURL scheme (HTTP/HTTPS only, no hardcoded prefixes)"
   - **Acceptance**: CLAUDE.md updated

---

### T1-8: Update Version & Final Documentation

**Epic**: Version sync with upstream  
**Effort**: 1 hour  
**Prerequisites**: All other T1 tasks  
**Blocks**: None

**Description**:
Update project version and finalize documentation to reflect upstream v0.6.14 feature parity.

**Subtasks**:

1. **T1-8a**: Update version strings
   - [ ] Find all version references:
     - `README.md` (top, anywhere it mentions version)
     - `go.mod` module version comment (if any)
     - `CHANGELOG.md` (if exists)
     - Version constant in `main.go` (if exists)
   - [ ] Change to: "v0.6.14" to match upstream
   - **Acceptance**: All version strings updated to v0.6.14

2. **T1-8b**: Add /readhttpfile security notes to CLAUDE.md
   - [ ] Open `CLAUDE.md`
   - [ ] Find "Known gaps" or "Security" section
   - [ ] Add note about `/readhttpfile`:
     ```markdown
     ### /readhttpfile Security Fix (Upstream v0.6.11+)
     
     Dave fixed upstream's `/readhttpfile` endpoint to only read from
     `config.urlMenuOpml` (prevents SSRF). The Go implementation
     intentionally does not implement this endpoint due to SSRF risk.
     If implemented in future, must include authorization check.
     ```
   - **Acceptance**: CLAUDE.md has security note

3. **T1-8c**: Update project status
   - [ ] Open `todolist.md`
   - [ ] Update top section:
     - Change "aiming for feature parity with v0.6.3" → "v0.6.14"
   - [ ] Mark Tier 1 tasks as complete
   - **Acceptance**: todolist.md shows updated version and completion

4. **T1-8d**: Create CHANGELOG entry
   - [ ] Create or update `CHANGELOG.md`
   - [ ] Add entry for v0.6.14 (or current date):
     ```markdown
     ## [0.6.14] - 2026-09-25
     
     ### Added
     - WebSub (Web Push) support for real-time feed notifications
     - Configuration options: flWebsubEnabled, urlWebsubHub
     - WebSub hub discovery headers (Link rel=hub/self)
     
     ### Verified
     - Feed URL construction respects configured scheme (HTTP/HTTPS)
     - All feed responses include proper base URLs
     
     ### Security
     - Documented upstream's /readhttpfile authorization fix
     ```
   - **Acceptance**: CHANGELOG.md created/updated

---

## TIER 2: Important Enhancements

### T2-1: Email Sender Integration (Wire-up)

**Epic**: Email Sender Integration  
**Effort**: 2-3 hours  
**Prerequisites**: None  
**Blocks**: T2-2

**Description**:
Wire the already-built emailSender into the API handlers so confirmation emails are actually sent (currently stubbed).

**Subtasks**:

1. **T2-1a**: Audit current email flow
   - [ ] Open `main.go` around line 130-138
   - [ ] Note how emailSender is created
   - [ ] Open `api/auth_endpoints.go` around line 203
   - [ ] Find `sendConfirmationEmail` function
   - [ ] Verify it's currently stubbed (just logs printf)
   - **Acceptance**: Current flow documented

2. **T2-1b**: Update api.Handler struct
   - [ ] Open `api/handler.go` (or find Handler struct definition)
   - [ ] Add field: `EmailSender *email.Sender`
   - [ ] Update Handler constructor to accept emailSender parameter
   - **Acceptance**: Handler has EmailSender field; compiles

3. **T2-1c**: Update main.go wiring
   - [ ] Open `main.go`
   - [ ] Find where API Handler is created
   - [ ] Change from `_ = emailSender` to pass it:
     ```go
     handler.EmailSender = emailSender
     ```
   - [ ] Ensure no compiler errors
   - **Acceptance**: emailSender passed to handler; main.go compiles

4. **T2-1d**: Implement sendConfirmationEmail
   - [ ] Open `api/auth_endpoints.go` at sendConfirmationEmail function
   - [ ] Replace printf stub with actual send:
     ```go
     if h.EmailSender == nil {
         log.Printf("email sender not configured")
         return nil
     }
     
     subject := "Confirm your email for RSS Chat"
     body := fmt.Sprintf("Click here to confirm: %s/...", confirmURL)
     
     return h.EmailSender.Send(email.Message{
         To:      toEmail,
         Subject: subject,
         Body:    body,
     })
     ```
   - [ ] Reference upstream implementation for email template format
   - [ ] Ensure error handling: if send fails, return error (don't swallow it)
   - **Acceptance**: 
     - emailSender.Send() called with proper params
     - Error returned to caller
     - Subject and body reasonable

---

### T2-2: Email Sender Integration (Testing)

**Epic**: Email Sender Integration  
**Effort**: 1-2 hours  
**Prerequisites**: T2-1  
**Blocks**: None

**Description**:
Test that confirmation emails are sent through the proper flow.

**Subtasks**:

1. **T2-2a**: Create mock email sender for tests
   - [ ] In `api/endpoint_test.go` or new `email_test.go`:
     ```go
     type MockEmailSender struct {
         SentMessages []email.Message
         ShouldFail   bool
     }
     
     func (m *MockEmailSender) Send(msg email.Message) error {
         if m.ShouldFail {
             return errors.New("mock send failed")
         }
         m.SentMessages = append(m.SentMessages, msg)
         return nil
     }
     ```
   - **Acceptance**: Mock compiles; implements email.Sender interface

2. **T2-2b**: Add email send tests
   - [ ] Test 1: `/sendconfirmingemail` endpoint sends email
     - Call endpoint
     - Verify mock received message
     - Verify To, Subject contain expected values
   - [ ] Test 2: `/createnewuser` sends confirmation email
     - Create user
     - Verify confirmation email sent to user's email
   - [ ] Test 3: Email send failure returns error
     - Mock configured to fail
     - Endpoint returns error status
   - [ ] Test 4: Missing emailSender handled gracefully
     - Handler with nil EmailSender
     - Logs but doesn't crash
   - **Acceptance**: All 4 tests pass

---

### T2-3: Blocklist Reload on Demand (Research)

**Epic**: Config Reload Enhancement  
**Effort**: 1-2 hours  
**Prerequisites**: None  
**Blocks**: None (optional)

**Description**:
Research whether blocklist should support live reload from config.json (currently only hot-reloads from blocklist.json).

**Subtasks**:

1. **T2-3a**: Verify current blocklist hot-reload
   - [ ] Open `db/blocklist.go` (or similar)
   - [ ] Confirm hot-reload mechanism:
     - Watches/reloads blocklist.json on changes
     - Checks mtime before re-reading
     - Applied on next auth check
   - [ ] Confirm blocklist.json is separate from config.json
   - **Acceptance**: Current behavior documented

2. **T2-3b**: Assess config.json blocklist reload feasibility
   - [ ] Determine: Can config.json include blocklist?
     - Current design separates config (permanent) from blocklist (operational)
     - Reloading config.json requires full server restart (architecture)
     - Separate blocklist.json avoids restart
   - [ ] Decision: Keep separate blocklist.json or merge into config.json?
   - [ ] Document trade-offs:
     - Separate: No restart needed, simpler mtime logic ✓ (current)
     - Merged: Everything in one file, requires restart ✗
   - **Acceptance**: Decision documented; trade-offs clear

3. **T2-3c**: If proceeding: Design config reload
   - [ ] Only if decided to implement:
   - [ ] Design: HTTP endpoint to reload blocklist from config.json
     - `/admin/reload-config` (requires auth)
     - Parses config.json
     - Syncs new blocklist to database
     - Doesn't restart server
   - **Acceptance**: Design doc written; reviewed with maintainer

---

## TIER 3: Nice-to-Have & Future

### T3-1: End-to-End HTTP Integration Test

**Epic**: End-to-End Smoke Test  
**Effort**: 3-4 hours  
**Prerequisites**: All other tests passing  
**Blocks**: None

**Description**:
Comprehensive HTTP-integrated E2E test covering full workflow from user creation to feed publishing.

**Subtasks**:

1. **T3-1a**: Create E2E test framework
   - [ ] Create `api/e2e_test.go`
   - [ ] Helper function to start full server with all components:
     ```go
     func setupE2ETestServer(t *testing.T) (serverURL string, cleanup func()) {
         // Start real HTTP server on random port
         // Create temp databases
         // Return URL and cleanup function
     }
     ```
   - [ ] Helper to make authenticated HTTP requests:
     ```go
     func authedHTTPRequest(t *testing.T, method, path, email, code string) *http.Response {
         // Make request with credentials
     }
     ```
   - **Acceptance**: E2E framework compiles; server starts/stops cleanly

2. **T3-1b**: Test full user workflow
   - [ ] E2E Test 1: Complete user lifecycle
     ```
     1. Create user via /createnewuser
     2. Verify user exists via /isuserindatabase
     3. Create post via /newpost
     4. Fetch post via /getrecentitems
     5. Verify post appears in /feed
     ```
   - [ ] Verify each step's response status and content
   - **Acceptance**: Test passes; all 5 steps work end-to-end

3. **T3-1c**: Test reply workflow
   - [ ] E2E Test 2: Post and reply
     ```
     1. User A creates post
     2. User B replies to post
     3. Fetch replies via /getitemandreplies
     4. Verify reply appears in comments feed
     ```
   - [ ] Verify reply threading correct
   - **Acceptance**: Test passes; reply chain correct

4. **T3-1d**: Test like workflow
   - [ ] E2E Test 3: Post and likes
     ```
     1. User A creates post
     2. Users B, C like post via /togglelike
     3. Fetch likers via /getlikerslist
     4. Verify count and list correct
     ```
   - **Acceptance**: Test passes; likes counted correctly

5. **T3-1e**: Test feed formats
   - [ ] E2E Test 4: Feed generation and formats
     ```
     1. Create posts
     2. Request /feed as XML (RSS)
     3. Request /feed as JSON
     4. Request /getsubscriptionlist (OPML)
     5. Verify each format valid/parseable
     ```
   - [ ] Parse each format to verify structure
   - **Acceptance**: All 3 formats valid and parseable

6. **T3-1f**: Test WebSocket real-time
   - [ ] E2E Test 5: WebSocket event flow
     ```
     1. Connect via /subscribe
     2. Create post in separate goroutine
     3. Receive update event on WebSocket
     4. Verify event contains correct post data
     ```
   - [ ] Use channels to coordinate async WebSocket receives
   - **Acceptance**: Test passes; WebSocket receives events

---

### T3-2: Performance Optimization (Research & Baseline)

**Epic**: Performance Optimization  
**Effort**: 2-3 hours  
**Prerequisites**: T3-1 (E2E tests)  
**Blocks**: None

**Description**:
Profile codebase and establish performance baseline for future optimizations.

**Subtasks**:

1. **T3-2a**: Run existing benchmarks
   - [ ] Execute: `go test ./... -bench=. -benchmem`
   - [ ] Capture output → save to `docs/benchmarks-baseline.txt`
   - [ ] Note memory allocations and op/sec for each benchmark
   - **Acceptance**: Baseline captured; benchmarks documented

2. **T3-2b**: Profile with pprof
   - [ ] Run E2E tests with CPU profiling:
     ```bash
     go test -cpuprofile=cpu.prof -memprofile=mem.prof ./api -run TestE2E
     go tool pprof cpu.prof
     ```
   - [ ] Identify hot functions (>5% CPU time)
   - [ ] Document findings in `docs/performance-profile.md`:
     - Top 5 functions by CPU
     - Memory allocations by package
     - Opportunities for optimization
   - **Acceptance**: Profile results documented

3. **T3-2c**: Identify quick wins
   - [ ] Review profile findings
   - [ ] List potential optimizations (don't implement yet):
     - SQL query optimization (indexing, caching)
     - Unnecessary allocations (strings, slices)
     - Connection pooling opportunities
     - Concurrency improvements
   - [ ] Prioritize by impact (CPU or memory saved)
   - **Acceptance**: List of 3-5 potential optimizations documented

---

## Summary Table

| ID | Task | Tier | Effort | Status | Owner |
|----|----|------|--------|--------|-------|
| T1-1 | WebSub Config | 1 | 1-2h | Open | — |
| T1-2 | WebSub Pinger | 1 | 2-3h | Open | — |
| T1-3 | WebSub Headers | 1 | 1-2h | Open | — |
| T1-4 | WebSub Publishing | 1 | 2-3h | Open | — |
| T1-5 | WebSub Integration Tests | 1 | 2-3h | Open | — |
| T1-6 | WebSub Documentation | 1 | 1h | Open | — |
| T1-7 | Feed URL Verification | 1 | 1-2h | Open | — |
| T1-8 | Version & Final Docs | 1 | 1h | Open | — |
| T2-1 | Email Sender Wire-up | 2 | 2-3h | Open | — |
| T2-2 | Email Sender Tests | 2 | 1-2h | Open | — |
| T2-3 | Blocklist Reload Research | 2 | 1-2h | Open | — |
| T3-1 | E2E Integration Tests | 3 | 3-4h | Open | — |
| T3-2 | Performance Baseline | 3 | 2-3h | Open | — |

**Total Tier 1**: ~14-20 hours (blocking parity)  
**Total Tier 2**: ~5-7 hours (important features)  
**Total Tier 3**: ~5-7 hours (nice-to-have)  
**Project Total**: ~24-34 hours

---

## Agent Handoff Guidelines

Each subtask above is designed to be:

- **Self-contained**: Can be completed independently (given prerequisites)
- **Testable**: Has clear acceptance criteria
- **Bounded**: Estimated effort is specific and achievable in one session
- **Documented**: References to upstream code, existing patterns, and expected outcomes

### How to Hand to an Agent

When giving a subtask to an agent:

1. **Provide context**:
   - Link to this file
   - Link to upstream reference: `/tmp/rss.chat/server/code/rssnetwork.js`
   - Link to CLAUDE.md for architecture notes

2. **Specify scope**:
   - "Please complete T1-2b: Implement Pinger struct and constructor"
   - Agent will read prerequisites and understand what's needed

3. **Verify completion**:
   - Agent runs acceptance criteria from subtask
   - Tests pass
   - No compiler errors
   - Code reviewed for style (gofmt, standard Go patterns)

4. **Track dependencies**:
   - Don't start T1-4 until T1-2 is done
   - Tier 1 → Tier 2 → Tier 3 (in general)
   - Use "Blocks" field to understand blocking relationships
