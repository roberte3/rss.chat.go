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

**In Progress:**
- Phase 4 (final): Websocket broadcasting

**Deferred to later phases:**
- Phase 5: Email auth (/sendconfirmingemail, /createnewuser)
- Phase 6: Static client hosting
- Phase 7: Config loading
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

- [ ] `/sendconfirmingemail?email=...&urlredirect=...` — generate/reuse a per-user
      `emailSecret` (mint only once — see the 5/10/26 worknote about email-scanner
      pre-fetch breaking a naive "always regenerate" approach), email a confirmation
      link.
- [ ] `/createnewuser?email=...&name=...&urlredirect=...` — same flow for new
      screennames, gated by whitelist (`checkWhitelist`) and blocklist.
- [ ] Email sending: pick a Go mail path (SMTP, or an SES/Sendgrid API client). Render
      `server/code/emailtemplate.html`'s `[%operationToConfirm%]` /
      `[%confirmationUrl%]` macros.
- [ ] Confirmation redirect: append `emailconfirmed=true&email=...&code=...&screenname=...`
      to `urlredirect` and 302.
- [ ] `isUserAdmin` — currently a stub (`callback(false)`) in the JS too; port as a stub.

## Phase 6 — client hosting

- [ ] Serve `archive/rss.chat/client/code/*` as static assets (or fetch/cache the home
      page from `urlServerHomePageSource` the way `daveappserver` does — decide which
      model fits a self-hosted Go binary better).
- [ ] Macro substitution in the home page HTML (`[%title%]` etc. — same mechanism as
      the email template, generalized) if config-driven values need to reach the client
      without hardcoding.

## Phase 7 — config & ops

- [ ] `Settings`/config loading: reconcile the existing `Settings` struct in `main.go`
      (`note`, `productName`) with the much larger `config.json` shape documented in
      `config.md` (domain, S3 paths, email sender, websocket, whitelist, blockedUsersList).
      Decide what's required vs. defaulted for a first Go release.
- [ ] `setup/setup.go`'s `CreateSettings` currently writes a 2-field settings file —
      expand it once the real config shape is settled, or split "app settings" from
      "server config" if that separation is worth keeping.
- [ ] `robots.txt` content from config, disallowing `/getitembyguid` and `/getiteminfo`.
- [ ] Startup sequence (`startup()` in the JS): load config, connect DB, republish
      subscription list, start any periodic tasks, start the HTTP (and websocket) server.

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