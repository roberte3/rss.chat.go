# RSS Chat Go — Project Status & Roadmap

A Go implementation of Dave Winer's RSS Chat, aiming for feature parity with v0.6.3.

**Reference**: Original RSS.Chat on GitHub: https://github.com/scripting/rss.chat  
**Current Build**: Passing all tests (170+ tests across 12 packages)  
**Latest Feature**: @Mentions rendering & storage (Phase 1-2); TextNodes refactor for single-pass transforms

---

## 📋 Remaining Work (Prioritized by Tier)

### Tier 1: Feature Parity with Upstream (Dave Winer's v0.6.14) — ✅ COMPLETE

#### 1. WebSub (Web Push) Support ✅ DONE
**Feature**: Real-time feed notifications via WebSub protocol
- [x] Add config fields: `flWebsubEnabled` and `urlWebsubHub`
- [x] Create `websub/pinger.go` with HTTP POST logic to notify hub on feed updates
- [x] Add WebSub `Link` headers to feed responses (hub + self URLs)
- [x] Integrate pinger into `publish/publisher.go` for user and global feed updates
- [x] Comprehensive test coverage (38 protocol tests + integration tests)
- [x] Update config.example.json with defaults
- [x] Update README and CLAUDE.md documentation
- **Completed**: September 25, 2026
- **Commits**: `16ad2b1` (implementation), `385dd16` (completion marker)

#### 2. Feed URL Construction Verification ✅ DONE
**Feature**: Verify feed URLs respect configured scheme and domain
- [x] Reviewed `feed/builder.go` — all URLs correctly use `config.BaseURL`
- [x] Confirmed scheme handling works for HTTP and HTTPS
- [x] Verified per CLAUDE.md architecture guidance
- **Status**: Confirmed correct; no changes needed

#### 3. Update Version & Sync Docs ✅ DONE
**Feature**: Align project version and documentation with upstream
- [x] Updated CLAUDE.md with WebSub architecture notes
- [x] Documented `/readhttpfile` security fix with authorization check requirement
- [x] Synced vendored client (already up-to-date at commit)
- **Completed**: September 25, 2026

### Tier 2: Important Enhancements

#### 7. Autolinker Refinements (1 day) — ✅ DONE
**Feature**: Smarter URL detection
- [x] Don't linkify filenames with extensions (.md, .zip)
- [x] Comprehensive tests covering skip behavior
- **Tests**: All 16 test cases passing
- **Impact**: Better content quality

#### 6. Blocklist Persistence — Reload on demand (OPTIONAL ENHANCEMENT)
**Feature**: Re-read blocklist from config.json without restart
- [ ] Implement re-read mechanism (currently done via blocklist.json with hot-reload)
- **Note**: Hot-reload already works via separate blocklist.json file; config.json reload may be follow-up
- **Impact**: Operational flexibility for config changes
- **Status**: Blocklist persistence via JSON file DONE; config persistence could be future work

### Tier 3: Nice-to-Have & Future

#### 9. End-to-End Smoke Test (2-3 days) — IN PROGRESS
**Feature**: Full workflow validation
- [x] Individual component tests verify workflow steps (user CRUD, post/reply, likes, feeds, OPML, XML format, WebSocket)
- [ ] Full HTTP-integrated E2E test (requires comprehensive test framework)
- **Note**: Core workflows verified independently; full E2E needs architectural refactoring for cleaner setup
- **Impact**: Confidence in full deployment flow

#### 10. Autolinker Refinements (1 day) — BACKLOG
See Tier 2 item above.

#### 11. Email Sender Integration — BACKLOG
**Status**: Built but unused
**Issue**: emailSender is instantiated in main.go (line 130-137) but discarded with `_ = emailSender` (line 138)
- Reason: sendConfirmationEmail in api/auth_endpoints.go is stubbed (line 203) — just logs instead of sending
- Current flow: `/sendconfirmingemail` and `/createnewuser` endpoints call sendConfirmationEmail but it's a no-op
- Impact: Account creation works, but users don't receive confirmation emails
- To fix: 
  - Wire emailSender into Handler struct
  - Replace printf stub with actual email.Sender.Send() call
  - Consider whether to require SMTP in production vs. development mode
  - Update Phase 7 completion criterion
- **Note**: Feature gaps for production deployment, but not critical for basic testing

---

## ✅ Completed Features (v1.0 Core - Production Ready)

### Core Data Layer
- [x] **SQLite schema** with users, items, likes, blocklist tables
- [x] **CRUD operations** for all entities (users, posts, replies, likes)
- [x] **User hit tracking** with daily rollover
- [x] **Reply threading** with parent-child relationships
- [x] **Email-based authentication** with email secrets and confirmation flow
- [x] **Blocklist & whitelist** enforcement with case-insensitive matching
- [x] **User preferences** stored as JSON in database

### HTTP API (All Endpoints)
- [x] **Read endpoints** (public, no auth required):
  - `/feed` - RSS feed for user or everyone
  - `/getrecentitems` - Recent posts across network
  - `/getrecentuseritems` - User's recent posts
  - `/getitembyguid` - Get post by GUID
  - `/getitemandreplies` - Get post and replies
  - `/getiteminfo` - Post metadata (RSS or JSON format)
  - `/getuserdata` - User profile data
  - `/getlikerslist` - List of users who liked a post
  - `/getmostactivetoday` - Most active users in last 24h
  - `/getsubscriptionlist` - OPML subscription list
  - `/isuserindatabase` - Check if user exists
  - `/isemailindatabase` - Check if email exists
  - `/checkwhitelist` - Email whitelist status
  - `/robots.txt` - Robots exclusion file
- [x] **Auth endpoints** (public):
  - `/sendconfirmingemail` - Send confirmation email
  - `/createnewuser` - Create new user account
- [x] **Write endpoints** (authenticated):
  - `/newpost` - Create new post
  - `/updatepost` - Update existing post
  - `/deletepost` - Delete post
  - `/togglelike` - Like/unlike a post
  - `/saveprefs` - Save user preferences
- [x] **Media endpoints**:
  - `/uploadmedia` - Upload media (base64-encoded)
  - `/media/{id}` - Serve uploaded media

### Content Processing
- [x] **HTML linkification** - Auto-link URLs in post text
- [x] **HTML→Markdown conversion** - Generate markdown from HTML
- [x] **HTML sanitization** - Remove scripts/malicious content (bluemonday)
- [x] **Media validation** - Size limits, content-type whitelist, magic byte verification

### Feed Generation
- [x] **RSS 2.0 feed generation** with proper XML structure
- [x] **User feeds** - Posts by individual users
- [x] **Global feed** - Posts from all users
- [x] **Comment feeds** - Threaded replies with source attribution
- [x] **OPML subscription list** - Browser-importable feed list
- [x] **Local filesystem publishing** - Feeds saved to disk for serving
- [x] **Database-driven feeds** - Optional SQLite storage mode (dual-mode: filesystem or DB)
- [x] **Feed format negotiation** - JSON and XML output formats (query param: `?format=json|xml`)

### Real-time Features
- [x] **WebSocket support** with `/subscribe` endpoint
- [x] **Event broadcasting** for posts, updates, and likes
- [x] **WebSocket monitoring tool** (`tools/websocket-status`)

### Configuration & Deployment
- [x] **Config loading** from `config.json` with validation
- [x] **Configurable ports** - HTTP and WebSocket
- [x] **Environment-specific settings** - SMTP, database, URLs
- [x] **Client macro substitution** - Dynamic HTML templating
- [x] **Static asset serving** - Client files bundled with server
- [x] **Interactive setup** - Binary-only deployment support with `binary -setup`
- [x] **robots.txt from config** - Configurable robots.txt content

### Database & Backup
- [x] **Media database** - Separate SQLite file for scalability
- [x] **Blocklist database** - Separate blocklist table with hot-reload from JSON
- [x] **Feeds database** - Optional separate SQLite file for feed storage
- [x] **Backup tool** - Export users, items, likes, media to JSON
- [x] **Restore tool** - Import from backup (preserves IDs)
- [x] **Temporary file handling** - Safe media upload validation

### Security
- [x] **Email blocklist** - Hot-reload from separate blocklist.json file
- [x] **Email whitelist** - Optional whitelist enforcement
- [x] **Case-insensitive blocking** - Robust email matching
- [x] **Feed autodiscovery** - HTML `<link rel="alternate">` tag for RSS readers

### Testing
- [x] **Unit tests** - Data layer, database operations, blocklist operations
- [x] **Endpoint tests** - HTTP API, feed generation, format negotiation
- [x] **Config tests** - Loading, validation, defaults, interactive generation
- [x] **Database tests** - CRUD operations, transactions, blocklist sync
- [x] **Media tests** - Upload, retrieval, data integrity
- [x] **Linkify tests** - URL auto-linking edge cases
- [x] **Markdown tests** - HTML→Markdown conversion
- [x] **Backup/restore tests** - Roundtrip export/import
- [x] **Setup tests** - Config generation, settings, blocklist

---

## 🚀 Yesterday's Progress (2026-07-26)

### Security & Auth Hardening
- ✅ Rewritten CLAUDE.md focusing on non-discoverable info (architecture, storage modes, auth design)
- ✅ Documented credentials travelling in URL (query parameters on POSTs)
- ✅ Implemented constant-time secret comparison (crypto/subtle)
- ✅ Stopped leaking emailSecret in logging/responses
- ✅ Rate-limited mail-sending endpoints per mailbox and source address
- ✅ Added Referrer-Policy and Cache-Control headers for authenticated responses

### Feature Fixes & Improvements
- ✅ Actually delete posts in /deletepost (was soft-delete only)
- ✅ Serve comments feeds at advertised URL (fixed 404s)
- ✅ Fix /getsubscriptionlist 404 in default configuration
- ✅ Fix WebSocket subscriber registration race condition
- ✅ Fix WebSocket event delivery to all subscribers
- ✅ Enhance websocket-status tool to show title and description preview

### Repository & Build
- ✅ Move module to github.com/roberte3/rss.chat.go
- ✅ Add MIT LICENSE and CI workflow
- ✅ Vendor web client so fresh clone serves working site
- ✅ Apply gofmt across tree
- ✅ Clean .gitignore

### Previous Sessions
#### Setup: Interactive Config Generation
- ✅ Fixed chicken-and-egg bug: `-setup` now creates config.json *before* config.Load()
- ✅ Interactive prompts for 8 fields with sensible defaults
- ✅ Auto-derives URLs: URLServerForClient, URLServerForEmail, URLWebsocketServerForClient
- ✅ Idempotent: re-running `-setup` never overwrites existing config.json
- ✅ Full workflow: binary-only user can bootstrap with `binary -setup` + defaults

#### Blocklist Persistence
- ✅ Separate blocklist.json file (operational data, not config)
- ✅ SQLite blocklist table for backup/restore
- ✅ Hot-reload: changes take effect on next auth check without restart
- ✅ Case-insensitive email matching

---

## 📊 Project Statistics

**Codebase**:
- 10 test packages (110 total tests passing)
- ~10 API endpoints
- ~25 data layer functions
- ~2500 lines of Go code (core logic)

**Dependencies**:
- Zero external dependencies for core (uses stdlib)
- Pure Go (no CGO required)
- SQLite for persistence (modernc.org/sqlite, pure Go driver)

**Documentation**:
- README.md — Comprehensive project guide
- CLAUDE.md — Development guidelines
- tools/TOOLS.md — CLI tools documentation
- config.example.json — Configuration template

---

## 🎯 Version Roadmap

### v1.0 ✅ Complete (Core Features)
- Core social networking
- RSS/OPML feed generation
- Real-time WebSocket updates
- Email-based auth
- Media uploads with validation
- Database backup/restore

### v1.1 (In Progress - Tier 1/2 Features)
- [x] Database-driven feeds (`flFeedsInDatabase` option)
- [x] Feed format negotiation (`?format=json|xml`)
- [x] Blocklist enforcement in auth endpoints
- [x] Post cleanup options (trailing paragraph trimming, custom OPML title)
- [x] Feed autodiscovery HTML link tags
- [x] Blocklist persistence with hot-reload from JSON
- [x] Interactive setup for binary-only deployments
- [x] Autolinker refinements (skip extensions like .md, .zip)
- [x] Blocklist mtime caching (optimization for signup traffic)

### v1.2 (In Progress - Tier 3)
- **@Mentions** — Render and store mentions with discovery API (Phase 1-2 ✅, Phase 3 🔄)
  - Generic text transformer for single-pass processing (Phase 0 ✅)
  - Mention rendering with case-insensitive lookup (Phase 1 ✅)
  - Database storage and query API (Phase 2 ✅)
  - Discovery endpoint `/getmentions` and websocket notifications (Phase 3 🔄)
- **#Hashtags** — Tag-based discovery with RSS feeds per hashtag (Planned)
  - Extraction and storage (planned)
  - Rendering as links (planned)
  - Discovery API `/gethashtagitems`, `/gettrendinghashtags`, `/feed?tag=` (planned)
- **User Avatars** — Profile pictures with upload support (Planned)
- End-to-end integration testing (partial ✅, E2E suite added)
- Performance optimizations
- Operational improvements

### v2.0 (Frontend Clients & Integrations)
- **HTMX-based front end** - Server-rendered HTML with dynamic interactions (no JS framework)
- **Mobile-optimized front end** - iOS/Android-ready UI for publishing into wider community
- **Bluesky (ATProtocol) Bridge** - Federation with Bluesky network
- **Advanced user features** (avatars, mentions, hashtags)
- **Email notifications** - User activity summaries and mentions
- **rssCloud support** - Legacy feed notification protocol

### v2.1+ (Future)
- Federation/interoperability (ActivityPub)
- Admin dashboard & moderation
- Horizontal scaling support
- Docker/Kubernetes deployment
- External storage (S3) support

---

## 🔗 Key Files & Locations

| Component | Location | Status |
|-----------|----------|--------|
| Database Layer | `db/` | ✅ Complete |
| HTTP API | `api/` | ✅ Complete |
| RSS/OPML Generation | `feed/` | ✅ Complete |
| Feed Publishing | `publish/` | ✅ Complete |
| WebSocket | `websocket/` | ✅ Complete |
| Configuration | `config/` | ✅ Complete |
| Setup/Bootstrap | `setup/` | ✅ Complete |
| Client Hosting | `client/` | ✅ Complete |
| Email | `email/` | ✅ Complete |
| CLI Tools | `tools/` | ✅ Complete (backup/restore/ws-monitor) |
| Tests | `*_test.go` | ✅ 110 tests passing

---

## 📝 Developer Quick Start

**Build**: `go build ./...`  
**Test**: `go test ./... -v`  
**Run**: `go run . -config config.json`  
**Setup**: `go run . -setup` (interactive config generation)

**Key Implementation Details**:
- WAL mode for SQLite (concurrent access)
- Parameterized queries (SQL injection protection)
- Base64 encoding for media in backups
- Magic byte verification for uploads
- bluemonday for HTML sanitization
- modernc.org/sqlite (pure Go, no CGO)
- github.com/coder/websocket for real-time
- bufio for interactive setup prompts

**Out of Scope for v1**:
- rssCloud pinging
- External OPML feed imports
- Full admin dashboard
- S3 storage (using filesystem/SQLite instead)
- Static web client bundling (currently gitignored; separate deployment concern)

---

## 🎓 Reference Material

- **Upstream Project**: https://github.com/scripting/rss.chat
- **Upstream v0.6.3**: Feature target for parity
- **API Spec**: `archive/rss.chat/server/docs/api.md`
- **Config Reference**: `archive/rss.chat/server/docs/config.md`

---

## 📋 Recent API Changes from Dave Winer's RSS.Chat (as of 2026-08-05)

Based on analysis of changes in the upstream repository, the following API and configuration changes have been made by Dave Winer since our last update:

### New Features

1. **WebSub Support**
   - Added `flWebsubEnabled` configuration flag
   - Added `urlWebsubHub` configuration for WebSub hub URL
   - Added `pingWebsubHub` function to notify WebSub hubs when feeds are updated
   - Modified feed update logic to ping both RSS Cloud and WebSub hubs
   - Added WebSub headers to feed responses when enabled

2. **New API Endpoint: `/readhttpfile`**
   - Added a new endpoint that allows reading HTTP files (used for Scripts menu functionality)
   - This endpoint was previously implemented inline but now uses a dedicated handler function
   - Designed to fetch OPML files for the Scripts menu functionality

### Configuration Changes

1. Updated RSS documentation URL from HTTP to HTTPS
2. Changed feed link construction to use `config.urlServerForClient` instead of hardcoded domain
3. Added new configuration parameters for WebSub support
4. Improved feed URL construction consistency

### Impact on Go Implementation

The Go implementation remains largely unaffected by these changes since:
- The Go port doesn't currently implement the `/readhttpfile` endpoint
- The Go port doesn't implement WebSub support (it only has RSS Cloud support)
- The core API endpoints (newpost, getitembyguid, etc.) remain unchanged
- The changes are mostly related to feed publishing and discovery mechanisms

The most significant change is the addition of WebSub support, which is a new feature that would require additional implementation in the Go version if desired, but doesn't break existing functionality.

This update does not affect the Go port's compatibility or existing functionality.