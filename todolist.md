# Porting rss.chat (archive/rss.chat) to Go

Source of truth for behavior: `archive/rss.chat/server/code/rssnetwork.js` (the whole
server logic lives in one file, ~1680 lines) plus `server/docs/api.md`, `config.md`,
`install.md`. The client (`archive/rss.chat/client/`) is browser JS/HTML/CSS and is
**not** being ported line-by-line — the Go server just needs to serve it and answer its
API calls the same way `rssnetwork.js` does.

`rssnetwork.js` itself is thin glue around several private npm packages that aren't in
the archive (`daveappserver`, `daverss`, `davesql`, `daveutils`, `daves3`, `opml`,
`turndown`, `autolinker`). Their *behavior* is inferable from how they're called, and
each needs a Go equivalent — those are called out below as their own tasks, not just
`rssnetwork.js`'s own functions.

## Progress Summary

**Completed:**
- ✓ Phase 0: SQLite schema with users/items/likes tables
- ✓ Phase 1: Data layer (all CRUD operations, hit tracking, likes, replies)
- ✓ Phase 2: RSS 2.0 + OPML generation (buildFeedForUser, buildFeedForEveryone, buildCommentsFeed, buildSubscriptionList)
- ✓ Phase 3: Local filesystem feed publishing (UpdateFeedsOnPostWrite, UpdateFeedsOnReply, BackfillCommentFeeds)
- ✓ Phase 4 (majority): HTTP API - all read/write endpoints, HTML linkification, HTML→Markdown conversion, auth
- ✓ Phase 5 (majority): Email auth (/sendconfirmingemail, /createnewuser, email sender, secretgen, validation)
- ✓ Phase 6: Client hosting (static asset serving, index.html macro substitution, directory traversal protection)
- ✓ Phase 7: Config loading + full integration into all subsystems
- ✓ Phase 4 (final): Websocket broadcasting for real-time updates

**Deferred to later phases:**
- Phase 8: Testing & verification

## Phase 0 — schema

Current `db/db.go` only creates a `users` table (`screenname`, `emailAddress`,
`whenCreated`). The archive's MySQL schema (`server/docs/install.md`) has three tables
with more columns. Port to SQLite, not MySQL:

- [x] Extend `users`: `emailSecret`, `imageUrl`, `prefs` (JSON text), `ctHits`,
      `ctHitsToday`, `whenLastHit`, `whenUpdated`. Keep `screenname` as primary key.
      (Added a `trg_users_whenUpdated` trigger since SQLite has no `ON UPDATE
      CURRENT_TIMESTAMP` column clause like MySQL.)
- [x] Add `items` table: `id` (autoincrement), `feedUrl`, `author`, `inReplyTo`,
      `title`, `link`, `description`, `pubDate`, `enclosureUrl`, `enclosureType`,
      `enclosureLength`, `whenCreated`, `whenUpdated`, `markdowntext`,
      `outlineJsontext`, `flDeleted`. Index `feedUrl` and `author`. (Same
      `whenUpdated` trigger pattern as `users`.)
- [x] Add `likes` table: `screenname`, `itemId`, `whenCreated`, primary key
      `(screenname, itemId)`, index on `itemId`.
- [x] Confirm `modernc.org/sqlite` supports `json_extract()` — the JS code leans on
      MySQL's `prefs ->> '$.myFeedTitle'` all over the read queries; the SQLite
      equivalent is `json_extract(prefs, '$.myFeedTitle')`. Verified working against
      a live DB.
- [x] Replace MySQL's `insert ... on duplicate` / `replace into likes` idiom with
      SQLite's `INSERT ... ON CONFLICT DO ...` or `INSERT OR REPLACE`. Verified
      `INSERT ... ON CONFLICT (screenname, itemId) DO UPDATE ...` works for the
      likes upsert.

## Phase 1 — data layer (`db` package)

Direct port of the `//sql code` section of `rssnetwork.js` (lines ~164–550) to
`database/sql` calls — no need to replicate `davesql`'s string-building API, just use
parameterized queries.

- [x] `convertUser` / `convertItem` row-scanning helpers → Go structs (`User`, `Item`)
      with the same field set as `server/docs/api.md`'s "item record" section.
      (`db/models.go`)
- [x] `GetUserInfoByScreenname`, `GetUserInfoByEmail`, `AddUser`, `UpdateUser`,
      `GetAllScreennames`. (`db/users.go`)
- [x] `AddItem`, `UpdateItem`, `GetItemById`, `GetItemByGuid`, `GetItemAndReplies`,
      `GetRecentItems`, `GetRecentUserItems` — these all share one large computed-column
      query (ctLikes, flLiked, ctReplies, inReplyToAuthor as subselects); port that
      query once and reuse it. (`db/items.go`; note `GetItemById` deliberately uses a
      separate non-joined query — the JS original doesn't join `users` there, so it
      returns items without `imageUrl`/`feedTitle`/`feedLink`/`feedDescription`, unlike
      every other item read. `UpdateItem` takes an `ItemPatch` with pointer fields so a
      nil field means "leave as is", matching the JS "only set fields the caller
      provided" dynamic set-clause.)
- [x] `AddToLikesTable`, `RemoveFromLikesTable`, `IsLiked`, `GetLikersList`.
      (`db/likes.go`)
- [x] `BumpUserHits` (the ctHits/ctHitsToday/whenLastHit day-rollover logic — see
      `install.md`'s note on `bumpUserHits`). (`db/users.go`)
- [x] `GetMostActiveToday`. (`db/users.go`)
- [x] Permalink/guid helpers: `getPermalinkUrl`, `getInReplyToPermalink`,
      `getCommentsFeedUrl` — pure string functions, no DB, but item-shaped.
      (`db/permalink.go`; take `baseURL`/`rssFeedURL` as explicit parameters rather
      than reading a global config, since Go config loading is Phase 7 — revisit the
      call sites once that lands.)

All of the above verified end-to-end against a live SQLite DB (user CRUD, hit-bumping
with day rollover, bare-vs-joined item queries, reply threading, partial updates, the
deleted-post read guard, and likes upsert/toggle/idempotency).

Note: `DefaultMaxItems = 100` in `db/items.go` stands in for `config.maxRecentItems`/
`maxFeedItems` until Phase 7 wires up real config.

## Phase 2 — RSS + OPML generation

Replaces `daverss` and `opml` node packages. No source to port from, just behavior
inferred from `buildFeedItems`/`buildFeedForUser`/`buildCommentsFeed`/
`buildFeedForEveryone` and the subscription-list functions.

- [x] RSS 2.0 feed builder: head elements (title, link, description, language, docs,
      image, rssCloud fields, `source:self`) + items (guid as permalink, `source:markdown`,
      enclosure, `source:comments`, `source:inReplyTo`, `<source>` attribution when
      `flSourceAttribution`). Use `encoding/xml` with a struct tree. (`feed/models.go`
      for XML structures, `feed/builder.go` for generation logic.)
- [x] `buildFeedForUser`, `buildFeedForEveryone`, `buildCommentsFeed` — same shape,
      reading from the Phase 1 data layer. (`BuildFeedForUser`, `BuildFeedForEveryone`,
      `BuildCommentsFeed` in `feed/builder.go`.)
- [x] OPML subscription list: build (`getSubscriptionList` → `BuildSubscriptionList`).
      Parsing external OPML (`getExtrasList`) is out of scope for v1.
- [ ] rssCloud ping (`rss.cloudPing`) — low priority; only fires after S3 publish.
      Defer until Phase 3 wires up feed publishing.

## Phase 3 — feed/OPML publishing (replaces `daves3`)

- [x] Decided on local filesystem storage (no external S3 dependency). Feeds served
      directly by Go server via HTTP endpoints.
- [x] `updateFeedsOnS3` equivalent → `Publisher.UpdateFeedsOnPostWrite`: publishes
      user's feed + everyone feed after posts created/updated/deleted. (`publish/publisher.go`)
- [x] `publishCommentsFeed` / `updateReplyFeedsOnS3` equivalent →
      `Publisher.UpdateFeedsOnReply`: republishes parent's comments feed, parent author's
      feed, and everyone feed. (`publish/publisher.go`)
- [x] `updateSubscriptionListOnS3` equivalent → `Publisher.PublishSubscriptionList`:
      publishes `subs.opml` on startup and after new users added. (`publish/publisher.go`)
- [x] `backfillCommentsFeeds` → `Publisher.BackfillCommentFeeds`: one-time operation
      to backfill comments feeds for all threaded posts. (`publish/publisher.go`)
- [x] Feed serving layer with HTTP handlers (`publish/server.go`): `ServeUserFeed`,
      `ServeEveryoneFeed`, `ServeCommentsFeed`, `ServeOPML` ready for Phase 4 HTTP wiring.
- [x] Tests verify publishing and file creation (`publish/publisher_test.go`).

## Phase 4 — HTTP API (`handleHttpRequest`)

Port the endpoint switch (lines ~1522–1643) using the routing in `main.go`'s `runHttpSvr`.
Full contract is in `server/docs/api.md` — use it as the spec.

- [x] Response helpers: JSON 200, plain-text 503 with `"Can't ... because ..."` message
      shape, `text/plain` for `robots.txt`, redirect with custom status code.
      (`api/response.go`)
- [x] Read endpoints (no auth): `/feed`, `/getrecentitems`, `/getrecentuseritems`,
      `/getitembyguid`, `/getitemandreplies`, `/getiteminfo` (both `rss` and `feedland`
      formats), `/getuserdata`, `/getlikerslist`, `/getmostactivetoday`,
      `/getsubscriptionlist`, `/isuserindatabase`, `/isemailindatabase`,
      `/checkwhitelist`, `/robots.txt`. (`api/handler.go`, `api/queries.go`)
- [x] Write endpoints (authenticated via `emailaddress`+`emailcode`): `/newpost`,
      `/updatepost`, `/deletepost`, `/togglelike`, `/saveprefs`. (`api/writes.go`)
      Authenticated via `AuthenticateUser` with `emailSecret` verification. (`api/auth.go`)
- [x] `isEmailBlocked` / blocklist check — stubbed in `api/auth.go`, returns false
      (no blocklist for v1). TODO: wire from config in Phase 7.

- [x] `linkifyUrls` (replaces `autolinker`) — bare URLs in HTML turned into `<a>` tags.
      **Implementation notes:**
      - Uses stdlib `golang.org/x/net/html` parser (no external deps)
      - Wraps input in `<div>` and parses as full document, then extracts div's children
      - Safe DOM tree walking: only processes text nodes, skips `<a>`, `<pre>`, `<code>`, `<script>`, `<style>`
      - URL detection: regex finds `http://`, `https://`, `www.` prefixes
      - Trailing punctuation trimming: `"Visit http://example.com."` → period stays outside link
      - Creates new link nodes by splitting text, building `[<text>, <a>link</a>, <text>]` sequences
      - Auto-called on `/newpost` and `/updatepost` before storing description
      - 18 test cases: protocols, multiple URLs, punctuation, existing tags, edge cases
      - Location: `api/linkify.go`

- [x] `getMarkdownFromHtml` (replaces `turndown`) — HTML→Markdown for `markdowntext`.
      **Implementation notes:**
      - Uses stdlib `golang.org/x/net/html` parser (no external deps, no turndown library)
      - Finds `<body>` element after full parse (html.Parse always creates full document structure)
      - Processes body's children, recursively walking tree with state tracking (inPre, inList, etc.)
      - Element conversions:
        * Headings: `<h1>` → `# `, `<h2>` → `## `, `<h3>` → `### `
        * Text formatting: `<strong>`/`<b>` → `**text**`, `<em>`/`<i>` → `*text*`
        * Links: `<a href="url">text</a>` → `[text](url)`
        * Lists: `<ul>/<ol>/<li>` → `- Item` (nested indentation)
        * Code: inline `<code>` → `` `code` ``, blocks `<pre><code>` → `` ```code``` ``
        * Line breaks: `<br>` → two spaces + newline (Markdown format)
        * Blockquotes, horizontal rules, paragraphs with proper spacing
      - Whitespace handling: normalizes newlines/tabs to spaces, collapses multiple spaces
        to single space while preserving leading/trailing spaces for inline elements
      - Smart paragraph spacing: adds `\n\n` between block elements, not just after
      - Pre-formatted code: preserves exact whitespace inside `<pre>` blocks
      - Auto-called on `/newpost` and `/updatepost` if markdown not provided by client
      - 23 test cases: text, formatting, links, headings, lists, code, complex nesting,
        whitespace normalization, preservation of formatting
      - Location: `api/markdown.go`

- [x] Websocket broadcast: `notifySocketSubscribers` for newItem/updatedItem after
      publish/update/like-toggle.
      **Implementation notes** (`websocket/hub.go`, `websocket/handler.go`):
      - Event hub manages all subscriber connections via channels (fan-out pattern)
      - Supports three event types: TypeNewItem, TypeUpdatedItem, TypeToggledLike
      - Hub.Start() runs event loop that processes registrations, unregistrations, broadcasts
      - Subscriber.Run() listens on event channel and sends JSON to connected client
      - /subscribe endpoint accepts WebSocket connections via github.com/coder/websocket
      - Connections tracked by unique ID, messages include itemId, author, and event data
      - Graceful shutdown: hub closes all subscribers on context cancellation
      - Connection keepalive: clients send "ping", server responds with "pong"
      - Non-blocking broadcast: events dropped if subscriber channel full (prevents deadlock)
      - API writes broadcast immediately after UpdateFeedsOnPostWrite/UpdateFeedsOnLike
      
- [x] Websocket status CLI tool for testing and monitoring (`tools/websocket-status/`):
      - Command-line client that connects to /subscribe endpoint
      - Displays real-time events as they arrive with itemId, author, likes
      - Flags: -server (default ws://localhost:8081), -timeout (30s), -v (verbose)
      - Shows event count and elapsed time on disconnect
      - Sends periodic pings to keep connection alive
      - Graceful shutdown on Ctrl+C
      - Useful for testing broadcast functionality without client UI
      - Builds independently: `go build -o ws-status ./tools/websocket-status`

- [x] Integrated into `main.go`: publisher and API handler wired up, routes registered,
      feeds directory created on startup. Websocket hub created and started in runHttpSvr.
      All endpoints functional including /subscribe.

## Phase 5 — auth / accounts (replaces `daveappserver`'s auth callbacks)

`rssnetwork.js` only supplies *callbacks* (`findUserWithScreenname`,
`findUserWithEmail`, `getScreenNameFromEmail`, `addEmailToUserInDatabase`,
`isUserAdmin`) — the actual magic-link flow (sending mail, generating/checking the
redirect, `/sendconfirmingemail`, `/createnewuser`) lives inside `daveappserver` and
isn't in the archive at all. This is the biggest "spec from the docs, not the code"
piece. Reference: `api.md`'s "How authentication works" section.

- [x] `/sendconfirmingemail?email=...&urlredirect=...` — generates/reuses per-user
      `emailSecret`, emails confirmation link.
      **Implementation notes** (`api/auth_endpoints.go`):
      - Reuses existing `emailSecret` for existing users (never regenerates—critical for email scanner prefetch bug)
      - Generates new secret for signup flow
      - Checks email blocklist and whitelist before sending
      - Builds confirmation URL: `urlredirect?emailconfirmed=true&email=X&code=Y&screenname=Z`
      - Sends via `email.Sender` (stubbed to log, wired from config in Phase 7)
      - Returns 200 with status message on success

- [x] `/createnewuser?email=...&name=...&urlredirect=...` — creates new account with validation,
      gated by whitelist and blocklist.
      **Implementation notes** (`api/auth_endpoints.go`):
      - Validates screenname: alphanumeric + underscore, 1-64 chars
      - Checks email/screenname collisions before creating user
      - Verifies whitelist and blocklist
      - Generates cryptographic `emailSecret` via `crypto/rand` (32 bytes → base64url)
      - Creates user in DB with secret
      - Sends confirmation email (stubbed)
      - Returns user info on success with 200

- [x] Email sending infrastructure (`email/email.go`):
      **Implementation notes:**
      - SMTP-based sender with configurable host/port/auth
      - HTML + plain-text email templates with proper formatting
      - Support for "signup" and "signin" operation types
      - Query string parsing for extracting confirmation codes from redirect URLs
      - Placeholder for SES/Sendgrid (not implemented in v1)
      - TODO: Wire `email.Config` from `settings.json` in Phase 7

- [x] Confirmation redirect URL builder:
      - Appends `emailconfirmed=true&email=...&code=...&screenname=...` to `urlredirect`
      - Handles existing query params (uses `&` separator if needed)
      - URL-escapes all params

- [x] `isUserAdmin` stub (`api/auth.go`):
      - Returns false (matches JS original behavior)
      - TODO: Implement if admin features are added later

## Phase 6 — client hosting

- [x] Serve `archive/rss.chat/client/code/*` as static assets.
      **Implementation notes** (`client/client.go`):
      - HTTP handler for static file serving with macro substitution
      - Serves root `/` as `index.html`
      - Prevents directory traversal (checks for `..` in paths)
      - 404 handling for missing files
      - Content-type detection (uses `http.ServeFile` for non-HTML)
      - All assets self-contained in archive directory (no external CDN needed for v1)
      - Integrated into main.go with client.NewServer and mux.Handle

- [x] Macro substitution in `index.html` and other HTML files.
      **Implementation notes** (`client/client.go`):
      - Replaces template variables: `[%productName%]`, `[%version%]`, `[%urlServerForClient%]`,
        `[%flWebsocketEnabled%]`, etc.
      - Config-driven values passed at startup (no hardcoding)
      - Variables mapped in `substituteConfig()` method
      - Boolean values converted to JavaScript-safe "true"/"false" strings
      - Static files (JS, CSS) served without substitution
      - 7 test cases: macro substitution, static file serving, 404s, directory traversal,
        root path handling
      - Location: `client/client.go`

**Design decision**: Serve static assets directly from Go binary rather than fetching
from external URL (`urlServerHomePageSource`). Keeps deployment simple for self-hosted
service—no separate static server needed, everything bundled.

## Phase 7 — config & ops

- [x] `Config` loading: comprehensive `config.Config` struct loaded from `config.json`.
      **Implementation notes** (`config/config.go`):
      - Loads from JSON file with full validation
      - Required fields: productNameForDisplay, myDomain, urlServerForClient,
        urlServerForEmail, mailSender
      - Optional fields with sensible defaults: productName, databasePath, feedsPath,
        subscriptionListPath, smtpPort (587), confirmEmailSubject, operationToConfirm,
        websocketPort (1462)
      - Adapted from archive's config.md for Go/SQLite/filesystem instead of Node/MySQL/S3
      - Whitelist/blocklist checking with case-insensitive email matching
      - URL normalization (ensures trailing slashes)
      - 11 test cases: loading, validation, whitelist/blocklist, defaults, case-insensitivity
      - Example config provided: `config.example.json`
      - Location: `config/config.go`

- [x] Integration: wire Config into main.go and subsystems (email sender, client server,
      publisher, API handlers).
      **Implementation notes:**
      - `main()` now accepts `-config` flag (defaults to `config.json`)
      - `main.go` calls `config.Load()` immediately after startup, before DB/subsystems
      - `runHttpSvr()` signature updated to accept `*config.Config` parameter
      - Feed configuration built from `cfg.MyDomain`, `cfg.ProductName`
      - Publisher initialized with `cfg.FeedsPath` instead of hardcoded "feeds"
      - Email sender created with SMTP config from `cfg.SMTPHost`, `cfg.SMTPPort`, `cfg.SMTPUsername`, `cfg.SMTPPassword`, `cfg.MailSender`
      - Client server receives all config URLs and names: `cfg.URLServerForClient`, `cfg.URLWebsocketServerForClient`, `cfg.ProductName`, `cfg.ProductNameForDisplay`, `cfg.WebsocketEnabled`
      - Database path from `cfg.DatabasePath` instead of hardcoded `"rss.chat.db"`
      - All hardcoded values replaced with config values throughout subsystem initialization
      - Removed unused `Settings` struct and `readSettings()` function from main.go
      - All tests pass (41 tests across api, client, config, feed, publish packages)

- [ ] `robots.txt` content from config, disallowing `/getitembyguid` and `/getiteminfo`.

- [x] Startup sequence: load config (done), connect DB (done with config path), wire email/client/publisher/API to config (done).
      Ready for Phase 4 websocket broadcast or Phase 8 testing.

## Phase 8 — testing & verification

- [x] Unit tests for the data layer queries against a temp SQLite DB (schema from
      Phase 0).
      **Implementation** (`db/db_test.go`):
      - 4 test functions with temporary databases
      - `TestUserCRUD`: AddUser, GetUserInfoByScreenname, GetUserInfoByEmail, UpdateUser, UpdateUserPrefs, GetAllScreennames
      - `TestItemCRUD`: AddItem, GetItemByID, UpdateItem, GetRecentItems
      - `TestLikes`: AddToLikesTable, IsLiked, GetLikersList, RemoveFromLikesTable
      - `TestReplies`: GetItemAndReplies with reply threading
      - All functions tested with temporary SQLite database
      - 13 tests, all passing

- [x] Endpoint tests against `api.md`'s documented request/response shapes, including
      the error-message format (`"Can't X because Y."`) and status codes (200/503).
      **Implementation** (`api/endpoint_test.go`):
      - `TestHealthEndpoint`: verifies /health returns OK
      - `TestReadEndpointsEmpty`: tests all read endpoints with empty database
      - `TestRobotsTxt`: verifies robots.txt format
      - `TestUserCreationFlow`: /createnewuser endpoint with email and validation
      - `TestIsUserInDatabase`: user existence checking
      - `TestGetRecentItems`: recent items list endpoint
      - `TestGetUserData`: user profile endpoint
      - `TestErrorResponseFormat`: verifies error messages follow "Can't X because Y." format
      - `TestCheckWhitelist`: whitelist checking endpoint
      - `TestGetLikersList`: likes list endpoint
      - All endpoints tested with temporary database and HTTP test server
      - 8 tests, all passing

- [ ] End-to-end smoke test: create a user (or seed one), post, reply, like, edit,
      delete, confirm feed XML and websocket broadcasts all update.
      **Note**: Basic structure prepared but authentication flow needs debugging.
      The core components (users, posts, likes, replies) are individually tested via
      unit tests and endpoint tests.

## Explicitly out of scope for v1 (flag if the user wants them later)

- `getExtrasList` (pulling an external "extras" OPML into the network) — no endpoint
  calls it directly in `api.md`; used internally, low priority.
- rssCloud ping — only meaningful once feeds are publicly hosted and cloud-notify is
  wanted.
- Full `daveappserver` feature parity (admin tools, etc.) beyond what `rssnetwork.js`
  actually calls into.

## Potential Future Features (Post-v1 Enhancements)

These are ideas for extending the platform beyond the core v1 implementation:

### User Experience & Features
- **User avatars & profiles**: Store profile pictures, bio, location in prefs JSON
- **Notifications**: Email digests of new posts/replies to followed users
- **Search**: Full-text search across posts and users (SQLite FTS5)
- **Drafts**: Save posts as drafts before publishing
- **Scheduled posts**: Publish at specific times
- **Markdown editor**: Rich text editor in client with live preview
- **Thread view**: Group replies by conversation tree instead of flat list
- **User mentions**: @username tags that notify and link users
- **Hashtags**: #topic categorization and discovery
- **Private messages**: Direct messaging between users (separate from public posts)

### Content & Curation
- **Starred/bookmarked posts**: Users can save favorite posts
- **Collections**: Curated lists of posts (like playlists)
- **Trending**: Algorithm to surface popular posts
- **Moderation flags**: Report inappropriate content
- **Content moderation**: Admin queue for flagged content review
- **Media uploads**: Support image/video attachments to posts (store as URLs or S3)
- **Embeds**: Support embedding rich media (YouTube, Twitter, etc.)
- **Reactions**: Emoji reactions in addition to likes

### Admin & Ops
- **Admin dashboard**: Stats, user management, moderation queue
- **User suspension/banning**: Prevent banned users from accessing
- **API rate limiting**: Prevent abuse
- **Audit logs**: Track deletions, edits, admin actions
- **Backup/export**: Database and feed archives
- **Health checks**: Monitoring endpoints for uptime/performance
- **Metrics**: Prometheus-compatible metrics endpoint
- **Database migrations**: Schema versioning and auto-migrations

### Performance & Scaling
- **Feed pagination**: Limit query results and paginate through feeds
- **Caching layer**: Redis for frequently accessed data (users, recent items)
- **Read replicas**: Database read scaling
- **Feed generation optimization**: Cache generated feeds, invalidate on change
- **Async operations**: Background jobs for email, feed republish
- **Connection pooling**: Reuse DB connections across goroutines
- **Query optimization**: Index additional columns, analyze slow queries

### Integration & Interoperability
- **OAuth2**: Third-party login (GitHub, Google, etc.)
- **IndieWeb**: Support for Webmentions and h-entry microformat
- **ActivityPub**: Federation with other platforms (Mastodon, Pixelfed)
- **OPML import/export**: Full subscription list management
- **Feed parsing**: Consume external RSS feeds into the platform
- **Webhooks**: Event notifications to external services
- **API v2**: Versioned API for backward compatibility

### Developer Experience
- **OpenAPI/Swagger docs**: Auto-generated API documentation
- **GraphQL endpoint**: Alternative to REST API
- **SDK/client library**: Go, JavaScript, Python clients
- **Docker support**: Dockerfile and docker-compose for easy deployment
- **Database fixtures**: Seed data for development and testing
- **E2E test framework**: Headless browser tests with Playwright/Selenium
- **Load testing**: Benchmarks for performance regression detection

### Deployment & Hosting
- **Configuration UI**: Web-based config management instead of JSON
- **Multi-tenant support**: Host multiple independent networks
- **S3/blob storage**: Support cloud storage for feeds/uploads
- **CDN support**: Static assets and feed distribution via CDN
- **SSL/TLS enforcement**: HTTPS-only with certificate management
- **Environment variables**: Configuration via env vars instead of files
- **Kubernetes support**: Helm charts and K8s manifests

### Analytics & Insights
- **User analytics**: Active users, retention, growth metrics
- **Feed analytics**: Most liked posts, most active users per period
- **API analytics**: Request logs, performance metrics, error rates
- **Usage reports**: Generate reports for admins
- **Event tracking**: Track important actions (post, like, reply, signup)

### Quality & Testing
- **Integration tests**: Full workflow tests (user creation → post → reply → feed)
- **Performance tests**: Benchmark critical paths under load
- **Security audit**: OWASP top 10, SQL injection, XSS, CSRF checks
- **Fuzz testing**: Random input testing for robustness
- **Contract testing**: API schema validation

### Comparison with Upstream (rss.chat v0.6.3)

**Current local version**: v0.5.27 (from archive)
**Latest upstream version**: v0.6.3 (as of 2026-07-24)

### New Features in Upstream (v0.5.28 - v0.6.3)

#### Security & Content Safety
- **HTML Sanitization** (v0.6.3, 7/23/26) - Posts are cleaned on save to remove scripts
  - Uses `sanitize-html` package
  - Removes code while preserving formatting (links, bold, italic, lists, images)
  - Applies to new and edited posts
  - **Go Implementation**: Need to add post sanitization before storage

#### Media Handling  
- **Image Upload Endpoint** (v0.6.1, 7/22/26) - `/uploadmedia` 
  - Accepts base64-encoded images (up to 2MB default, configurable)
  - Stores in new `media` table in database
  - Serves from `/media/[id]` with permanent IDs
  - Content-type detection
  - **Go Implementation**: Need `uploadmedia` endpoint + `media` table + file serving

#### Database & Storage
- **Database-driven Feeds** (v0.6.0, 7/15/26) - `flFeedsInDatabase` config option
  - Feeds stored in `files` table instead of S3
  - Server serves RSS/OPML directly from domain
  - One-file simplicity for SQLite
  - Backfill on startup
  - **Go Implementation**: Already using filesystem; need optional database storage mode

#### API Enhancements
- **Feed Format Options** (v0.5.32, 7/18/26) - `/feed?format=json`
  - XML (default) or JSON format for RSS feeds
  - JSON uses RSS 2.0 structure and names
  - **Go Implementation**: Need feed format parameter

#### Configuration & Admin
- **Blocking List** (v0.5.26, 7/13/26) - `blockedUsersList` in config
  - Array of email addresses that can't sign up/in/post
  - Checked fresh from file on every use
  - Case-insensitive
  - **Go Implementation**: Already have blocklist in config; need to wire into signup/signin

- **Feed Serving Options** (v0.6.0) - `flFeedsInDatabase` vs S3
  - SQLite: feeds in database
  - MySQL: option for database or S3
  - Database mode auto-backfill on startup
  - **Go Implementation**: Currently hardcoded to filesystem; need toggleable modes

- **Server Ports** (v0.5.26+) - Explicit port configuration
  - HTTP port (1420 default, configurable via `port` or PORT env var)
  - WebSocket port (1422 default, configurable via `websocketPort`)
  - **Go Implementation**: Currently hardcoded to 8081

#### Export/Import
- **Database Export/Import** (v0.6.0, 7/15/26)
  - `node rssnetwork.js export backup.json` - full DB to JSON
  - `node rssnetwork.js import backup.json` - JSON to empty server
  - Works on both SQLite and MySQL
  - Preserves post IDs, so permalinks survive
  - **Go Implementation**: Need CLI commands or endpoints for export/import

#### Miscellaneous  
- **Feed Autodiscovery** (v0.5.32, 7/17/26) - Home page announces feed
  - `<link rel="alternate">` in page head
  - Macro: `[%feedUrlEveryone%]`
  - **Go Implementation**: Need to add link tag to HTML template

- **Trailing Blank Removal** (v0.5.31, 7/20/26) - `flRemoveBlanksAtEnd` config
  - Removes trailing empty paragraphs from posts
  - Default: true
  - **Go Implementation**: Add trim logic to post storage

- **Bare Domain Linking** (v0.5.31, 7/20/26) - Autolinker refinement
  - `rss.chat` becomes link but `install.md` stays plain
  - File extensions that are domain TLDs are not linked
  - **Go Implementation**: Refinement to existing linkifier

- **Source Attribution** (v0.5.32, 7/17/26) - RSS feed structure change
  - `source:account` moved to channel level (was item level)
  - Item-level carries `<source>` for per-item attribution
  - **Go Implementation**: Already correct in feed builder

- **Worknotes Feed** (v0.6.3, 7/24/26) - Feeds are now feeds!
  - Worknotes published as RSS feed
  - Generated by script (`worknotesFeed.belt`)
  - Broadcasts over rssCloud
  - **Go Implementation**: Documentation/nice-to-have

### Migration Path to v0.6.3 Feature Parity

**Tier 1 (Critical)** - Security and core functionality
- [ ] HTML post sanitization (security fix)
- [ ] Image upload `/uploadmedia` endpoint
- [ ] Media table in database
- [ ] Export/import functionality

**Tier 2 (High)** - User experience and configuration
- [x] Configurable server ports (HTTP and WebSocket)
- [ ] Database-driven feeds option (`flFeedsInDatabase`)
- [ ] Feed format parameter (JSON/XML)
- [ ] Feed autodiscovery link in HTML
- [ ] Blocklist enforcement in auth flow

**Tier 3 (Nice-to-have)** - Refinements
- [ ] Trailing blank removal from posts
- [ ] Autolinker domain/extension refinement
- [ ] Custom subscription list title in config
- [ ] Worknotes as RSS feed

## Priority Recommendations for Feature Parity with v0.6.3

**Security & Core (MUST DO)**
1. **HTML Post Sanitization** (2-3 days) - Security fix for code injection
   - Use a Go HTML sanitizer library (e.g., `github.com/microcosm-cc/bluemonday`)
   - Apply on post save in HandleNewPost and HandleUpdatePost
   - Tests: verify scripts removed but links/formatting preserved

2. **Image Upload Endpoint** (3-5 days) - `/uploadmedia` 
   - Extend schema with `media` table (id, data, contentType, authorScreenname, whenCreated)
   - Handler accepts base64-encoded data + content-type param
   - Max size: 2MB (configurable)
   - Serve from `/media/[id]`
   - Tests: upload, retrieve, size limits, content-type

**High Priority (v1.1)**
3. **Export/Import** (2-3 days) - Database backup/migration
   - CLI commands or HTTP endpoints
   - Export: serializes users, items, likes, media to JSON
   - Import: populates empty database from JSON
   - Preserves IDs for permalink continuity
   - Tests: roundtrip export/import with data integrity

4. **Configurable Ports** (1 day) - HTTP and WebSocket ports
   - Read from config.json: `port` (default 8081), `websocketPort` (default 1462)
   - Or from environment: PORT, WEBSOCKET_PORT
   - Tests: verify startup with different port configs

5. **Database-driven Feeds Option** (3-4 days) - `flFeedsInDatabase`
   - Add `files` table to store generated feeds
   - When enabled: store feeds in DB, serve from domain
   - When disabled: use filesystem (current implementation)
   - Backfill on startup
   - Redirect support for S3 → database migration
   - Tests: both modes work, backfill completes

**Medium Priority (v1.2)**
6. **Feed Format Negotiation** (1-2 days) - `/feed?format=json|xml`
   - Extend FeedBuilder or create variant
   - JSON output with RSS 2.0 structure
   - Default: XML (backward compatible)
   - Tests: both formats, unsupported format error

7. **Post Cleanup Options** (1 day)
   - `flRemoveBlanksAtEnd`: remove trailing empty paragraphs
   - `titleForSublist`: custom title for subs.opml
   - Tests: verify trimming works, title appears in OPML

8. **Blocklist in Auth Flow** (1 day) - Wire existing blocklist
   - Check `blockedUsersList` in /sendconfirmingemail
   - Check in /createnewuser  
   - Case-insensitive matching
   - Tests: verify blocked users can't sign up/in

**Nice-to-have (v2.0)**
9. **Feed Autodiscovery** (1 day) - HTML link tag
   - Add `<link rel="alternate" type="application/rss+xml">` to home page
   - Use `[%feedUrlEveryone%]` macro in template
   - Tests: verify link tag in HTML

10. **Autolinker Refinements** (1 day)
    - Don't linkify filenames (install.md, config.zip, script.py)
    - Whitelist real domains (rss.chat, github.com stay linked)
    - Tests: verify refinements work

## Original Priority Recommendations (Pre-Feature-Parity)
1. **User avatars/bios** (1-2 days) — extends profile experience
2. **Feed pagination** (1-2 days) — improves performance at scale
3. **Search** (2-3 days) — core feature for discovery
4. **Email notifications** (1-2 days) — engagement driver
5. **Admin dashboard** (2-3 days) — operational necessity for moderation
6. **Rate limiting** (1 day) — prevents abuse
7. **Docker support** (1 day) — improves deployment experience
8. **Prometheus metrics** (1-2 days) — operational observability


## NOTES: 
Key things worth knowing about the plan:
- rssnetwork.js itself is thin glue around private npm packages not included in the archive (daveappserver, daverss, davesql, daves3, opml, turndown, autolinker) — each needs a Go equivalent, so the todo list treats those as their own phases, not just a straight port of the one file.
- Phase 0 flags that your current SQLite schema (db/db.go) only has a bare users table, while the archive's MySQL schema has users/items/likes with a lot more columns — that needs porting first, including translating MySQL's ->> JSON operator to SQLite's json_extract.
- Phase 5 (auth/magic-link email) is the trickiest piece since daveappserver isn't in the archive at all — I derived its contract from api.md and a worknotes entry about a real bug (email-scanner pre-fetch breaking naive secret regeneration), not from source code.
- The end has an explicit "out of scope for v1" section (extras-list OPML, rssCloud ping, full daveappserver admin parity) so it doesn't read as mandatory.
