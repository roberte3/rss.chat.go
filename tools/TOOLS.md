# RSS Chat Tools

This directory contains command-line tools for managing the RSS Chat application.

## Backup Tool

The backup tool exports the complete database state to a JSON file for data migration, archival, or disaster recovery.

### Usage

```bash
./backup [options]
```

### Options

- `-db string` - Path to main database (default: "rss.chat.db")
- `-mediadb string` - Path to media database (default: "rss.chat.media.db")
- `-o string` - Output file path (default: "backup.json")

### Example

```bash
./backup -db rss.chat.db -mediadb rss.chat.media.db -o my-backup.json
```

### Output Format

The backup file is a JSON document containing:

- **exportedAt**: RFC3339 timestamp of export time
- **users**: Array of user records with screenname, email, secret, preferences
- **items**: Array of posts/items with IDs, text, parent relationships
- **likes**: Array of like records with item and user references
- **media**: Array of uploaded media with base64-encoded binary data

All binary media data is base64-encoded for JSON compatibility.

## Restore Tool

The restore tool imports data from a backup JSON file into empty databases, preserving all IDs and relationships.

### Usage

```bash
./restore [options]
```

### Options

- `-f string` - Backup file path (default: "backup.json")
- `-db string` - Path to main database (default: "rss.chat.db")
- `-mediadb string` - Path to media database (default: "rss.chat.media.db")
- `-force` - Force restore even if database is not empty (default: false)

### Example

```bash
./restore -f my-backup.json -db rss.chat.db -mediadb rss.chat.media.db
```

### Safety

By default, the restore tool will refuse to restore into a database that already contains data. Use `-force` to override this safety check.

### ID Preservation

The restore tool preserves all original IDs from the backup, maintaining permalink continuity and referential integrity:

- User screennames are preserved
- Item IDs remain the same (ensuring GUIDs/permalinks work)
- Like IDs are preserved
- Media IDs are preserved

## WebSocket Status Tool

Monitors real-time events from the WebSocket server.

### Usage

```bash
./websocket-status [options]
```

### Options

- `-server string` - WebSocket server URL (default: "ws://localhost:8081")
- `-timeout duration` - Connection timeout (default: 30s)
- `-v` - Verbose output

### Example

```bash
./websocket-status -server ws://localhost:8081 -v
```

## TestData Tool

Generates sample data for testing and development: creates a test user and populates the database with 10 sample posts.

### Usage

```bash
./testdata [options]
```

### Options

- `-db string` - Path to main database (default: "rss.chat.db")

### Example

```bash
./testdata -db rss.chat.db
or 
go run tools/testdata/main.go -db rss.chat.db
from project root. 
```

### Generated Data

Creates:
- **Test User**: screenname `testuser`, email `testuser@example.com`
- **10 Posts**: with sequential timestamps, sample HTML content, and markdown versions

The posts span 10 hours and are immediately available through:
- `/api/getrecentitems` — appears in network feed
- `/feed?screenname=testuser` — user's personal RSS feed
- `/api/getrecentuseritems?name=testuser` — user's recent posts

## Bluesky Subscribe Tool

Backfills a curated list of Bluesky accounts and forwards their top-level
posts into rss.chat as real posts, one synthetic rss.chat user per tracked
handle. Design details: `notes/feature-bluesky-bridge-utility.md` (gitignored
locally).

### Usage

```bash
cp tools/bluesky-subscribe/handles.example.json handles.json
# edit handles.json to the accounts you want to track

./bluesky-subscribe -handles handles.json -rss-chat-url http://localhost:8081
```

Run it again (e.g. from cron) to pick up new posts — it only backfills since
the last run per handle, and both provisioning and forwarding are safe to
repeat.

### Options

- `-handles string` — path to the JSON array of Bluesky handles to track
  (default: `handles.json`). Each entry is either a bare handle string or an
  object with an `initialBackfillDays` override, and the two forms can be
  mixed freely:
  ```json
  [
      "wario64.bsky.social",
      { "handle": "roberte3-dev.bsky.social", "initialBackfillDays": 30 }
  ]
  ```
  The override only affects a handle's *first* backfill (before it has a
  stored watermark) — see `-since` below for what applies without one, and
  every run after the first.
- `-state string` — path to this tool's own sqlite state: dedup index and a
  cache of everything pulled from Bluesky (default: `bluesky-subscribe.db`)
- `-rss-chat-url string` — base URL of the rss.chat server (default: `http://localhost:8081`)
- `-since duration` — how far back to backfill a handle the *first* time it's
  tracked, unless overridden per-handle in `-handles` (default: `24h`)
- `-resolver string` — AT Protocol entryway used to resolve handles to DIDs (default: `https://bsky.social`)
- `-plc-directory string` — PLC directory used to resolve `did:plc` documents (default: `https://plc.directory`)
- `-appview string` — AppView used to fetch profile info (avatar) for tracked handles (default: `https://public.api.bsky.app`)
- `-timeout duration` — per-request HTTP timeout (default: `30s`)
- `-dry-run` — fetch and log without provisioning users or posting

### Requires running on the same host as the rss.chat server

User provisioning calls `/localnewuser`, which is gated to loopback requests
only (see the README's Security model section). Run this tool on the same
machine as the rss.chat server, or through an SSH tunnel/loopback forward if
not — pointing `-rss-chat-url` at a remote server directly will fail
provisioning with a 503.

### What it does

1. **Provision**: for each handle not yet tracked, derives a screenname
   (`wario64.bsky.social` → `bsky_wario64`) and calls `/localnewuser` to
   create or fetch that account's credential.
2. **Backfill**: resolves each handle to a DID and PDS, then pages
   `com.atproto.repo.listRecords` for posts newer than the last run (or
   `-since`/`initialBackfillDays` on a handle's first run). Replies are
   skipped — v1 only forwards top-level posts. Each run logs one summary
   line per handle (`N records since <window> (R replies skipped, M
   unparseable, K top-level)`) — read that first if a handle seems to be
   fetching fewer posts than expected.
3. **Forward**: posts new items to rss.chat via `/newpost`, as the tracked
   handle's synthetic account. Each post's text is followed by a permalink
   back to the original on bsky.app.
4. **Avatar sync**: each run, fetches the handle's current Bluesky avatar
   (`app.bsky.actor.getProfile`) and, if it's changed since last synced,
   merges it into the account's `prefs.myAvatarImageUrl` via `/getuserdata` +
   `/saveprefs` — the same key rss.chat's feed/item rendering already reads
   avatars from. Skipped when the account has no avatar; left stale (not
   cleared) if an avatar is later removed on Bluesky.

## Reset Tool

Clears all databases and settings files for a fresh start. Useful for testing, resetting development state, or preparing for a clean deployment.

### Usage

```bash
./reset [options]
```

### Options

- `-db string` - Path to main database (default: "rss.chat.db")
- `-mediadb string` - Path to media database (default: "rss.chat.media.db")
- `-feedsdb string` - Path to feeds database (default: "rss.chat.feeds.db")
- `-settings string` - Path to settings file (default: "settings.json")
- `-blocklist string` - Path to blocklist file (default: "blocklist.json")
- `-config string` - Path to config file (default: "config.json")
- `-feedsdir string` - Path to feeds directory (default: "feeds")
- `-tempmedia string` - Path to temp media directory (default: "temp_media")
- `-keep-config` - Preserve config.json (don't delete)
- `-keep-feeds` - Preserve feeds directory (default: true)
- `-force` - Skip confirmation prompt
- `-v` - Verbose output

### Example

```bash
# Interactive reset (asks for confirmation)
./reset

# Force reset without prompt, preserving config
./reset -force -keep-config

# Delete everything including config
./reset -force -keep-config=false

# Delete everything including feeds directory
./reset -force -keep-feeds=false
```

### Safety

By default, the tool asks for confirmation before deleting. Config.json and feeds/ directory are preserved by default to avoid data loss. Use `-keep-config=false` and `-keep-feeds=false` to delete them.

## Building the Tools

Build all tools:

```bash
go build ./tools/...
```

Build a specific tool:

```bash
go build -o backup ./tools/backup
go build -o restore ./tools/restore
go build -o websocket-status ./tools/websocket-status
go build -o testdata ./tools/testdata
go build -o reset ./tools/reset
go build -o bluesky-subscribe ./tools/bluesky-subscribe
```

## Testing

Test all tools:

```bash
go test ./tools/...
```

Test a specific tool:

```bash
go test ./tools/backup -v
go test ./tools/restore -v
```

The tests verify:

- Complete roundtrip export/import with data integrity
- ID preservation for permalink continuity
- Media data integrity (binary data is preserved)
- Nullable field handling
- Error cases and safety checks
