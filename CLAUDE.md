# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

Project is a golang port of Dave Winer's RSS.Chat project: https://github.com/scripting/rss.chat/tree/main written in Go. 

Goal of the project is to be an opensource release, and run as a backend service on glurpglurp.app (site has not been written or launched yet). 

A copy of the RSS.Chat app is in the subfolder (archive/rss.chat)

## Commands

- Build: `go build ./...`
- Run: `go run .`
- Tidy dependencies: `go mod tidy`

## Architecture

- Module: `rss.chat.go` (Go 1.25).
- `db/db.go`: opens the SQLite connection via `db.Open(path)`. Uses `modernc.org/sqlite`, a pure-Go driver (no CGO/C toolchain needed). Applies default pragmas on open: WAL journal mode, foreign keys enforced, 5s busy timeout.
- `main.go`: opens `rss.chat.db` in the working directory on startup.
- SQLite database files (`*.db`, `*.db-wal`, `*.db-shm`) are gitignored and created on demand — no seed/migration step exists yet.

Beyond this, the codebase has no further structure yet (no RSS fetching or chat logic implemented). Update this file as real features are added.
