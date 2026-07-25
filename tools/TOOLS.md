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

## Building the Tools

Build all tools:

```bash
go build ./tools/...
```

Build a specific tool:

```bash
go build ./tools/backup
go build ./tools/restore
go build ./tools/websocket-status
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
