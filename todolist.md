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

**In Progress:**
- Phase 4 (final): Websocket broadcasting

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

- [ ] Websocket broadcast: `notifySocketSubscribers` for newItem/updatedItem after
      publish/update/like-toggle. Stubbed (TODO). `github.com/coder/websocket` available.
- [x] Integrated into `main.go`: publisher and API handler wired up, routes registered,
      feeds directory created on startup. All endpoints functional.

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

- [ ] Unit tests for the data layer queries against a temp SQLite DB (schema from
      Phase 0).
- [ ] Endpoint tests against `api.md`'s documented request/response shapes, including
      the error-message format (`"Can't X because Y."`) and status codes (200/503).
- [ ] End-to-end smoke test: create a user (or seed one), post, reply, like, edit,
      delete, confirm feed XML and websocket broadcasts all update as `worknotes.md`
      describes.

## Explicitly out of scope for v1 (flag if the user wants them later)

- `getExtrasList` (pulling an external "extras" OPML into the network) — no endpoint
  calls it directly in `api.md`; used internally, low priority.
- rssCloud ping — only meaningful once feeds are publicly hosted and cloud-notify is
  wanted.
- Full `daveappserver` feature parity (admin tools, etc.) beyond what `rssnetwork.js`
  actually calls into.


## NOTES: 
Key things worth knowing about the plan:
- rssnetwork.js itself is thin glue around private npm packages not included in the archive (daveappserver, daverss, davesql, daves3, opml, turndown, autolinker) — each needs a Go equivalent, so the todo list treats those as their own phases, not just a straight port of the one file.
- Phase 0 flags that your current SQLite schema (db/db.go) only has a bare users table, while the archive's MySQL schema has users/items/likes with a lot more columns — that needs porting first, including translating MySQL's ->> JSON operator to SQLite's json_extract.
- Phase 5 (auth/magic-link email) is the trickiest piece since daveappserver isn't in the archive at all — I derived its contract from api.md and a worknotes entry about a real bug (email-scanner pre-fetch breaking naive secret regeneration), not from source code.
- The end has an explicit "out of scope for v1" section (extras-list OPML, rssCloud ping, full daveappserver admin parity) so it doesn't read as mandatory.

Note: I left main.go alone since it looked like you were mid-edit there (currently has a stray mux.HandlerFunc("") causing a syntax error at line 66) — let me know if you want that cleaned up.