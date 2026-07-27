# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

Project is a golang port of Dave Winer's RSS.Chat project: https://github.com/scripting/rss.chat/tree/main written in Go. 

Goal of the project is to be an opensource release, and run as a backend service on glurpglurp.app (site has not been written or launched yet). 

A copy of the RSS.Chat app is in the subfolder (archive/rss.chat)

## Commands

- Build: `go build ./...`
- Run: `go run .`
- Tidy dependencies: `go mod tidy`
- Build backup tool: `go build -o backup ./tools/backup`
- Build restore tool: `go build -o restore ./tools/restore`

## Architecture

- Module: `github.com/roberte3/rss.chat.go` (Go 1.25).
- `db/db.go`: opens the SQLite connection via `db.Open(path)`. Uses `modernc.org/sqlite`, a pure-Go driver (no CGO/C toolchain needed). Applies default pragmas on open: WAL journal mode, foreign keys enforced, 5s busy timeout.
- `db/media.go`: manages separate media database (`rss.chat.media.db`) with CRUD operations for uploaded media files. Uses base64 encoding for safe binary data storage.
- `main.go`: opens `rss.chat.db` and `rss.chat.media.db` on startup, creates temp media directory for validation.
- SQLite database files (`*.db`, `*.db-wal`, `*.db-shm`) are gitignored and created on demand.
- `config/config.go`: loads configuration from `config.json`, includes media database settings (path, temp directory, max upload size).
- `api/uploadmedia.go`: handles authenticated media uploads with validation pipeline: size check → content-type whitelist → magic byte verification → temp file storage → database insertion.

## Database Structure

**Main Database (rss.chat.db):**
- `users`: screenname (PK), emailAddress, emailSecret, imageUrl, prefs, ctHits, ctHitsToday, whenLastHit, whenCreated, whenUpdated
- `items`: id (PK), screenname (FK), text, guidHash (unique), idParent (self-FK), whenCreated, whenUpdated
- `likes`: id (PK), itemId (FK), screenname (FK), whenCreated

**Media Database (rss.chat.media.db):**
- `media`: id (PK), screenname (indexed, case-insensitive), contentType, mediabytes (blob), size, whenCreated

## Tools

Located in `tools/`:

- **backup**: Exports users, items, likes, and media to JSON with base64-encoded binary data
- **restore**: Imports JSON backup into empty databases, preserving all IDs for permalink continuity
- **websocket-status**: Monitors real-time WebSocket events

See `tools/TOOLS.md` for detailed usage instructions.

## Server Configuration

- **HTTP Port**: Configurable via `httpPort` in config.json (default: 8081)
- **WebSocket Port**: Configurable via `websocketPort` in config.json (default: 1462)
- **Email/SMTP**: Configurable host and port for email sending
- All ports support environment-variable or config-file overrides
- Backward compatible with hardcoded defaults

## Media Handling

- Uploaded media stored in separate SQLite database for scalability
- Content type validation (image/jpeg, image/png, image/gif, image/webp, image/svg+xml)
- Magic byte verification prevents spoofed content types
- Temporary filesystem storage during validation (not in HTTP server routes)
- Base64 encoding for JSON export/import
- Stateless handlers allow horizontal scaling
