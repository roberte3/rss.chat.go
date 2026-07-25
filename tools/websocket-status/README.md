# WebSocket Status Tool

A command-line utility for testing and monitoring WebSocket connections to the rss.chat Go server.

## Build

From the project root:

```bash
go build -o tools/websocket-status/ws-status ./tools/websocket-status
```

Or from the tools/websocket-status directory:

```bash
go build -o ws-status
```

## Usage

```bash
./ws-status [flags]
```

### Flags

- `-server` (default: `ws://localhost:8081`) - WebSocket server URL
- `-timeout` (default: `30s`) - Connection timeout
- `-v` - Verbose output (shows full message JSON)

## Examples

### Connect to default local server
```bash
./ws-status
```

### Connect to production server
```bash
./ws-status -server wss://example.com
```

### Verbose mode
```bash
./ws-status -v
```

### With longer timeout
```bash
./ws-status -timeout 60s
```

## Output

The tool displays real-time events from the server:

```
Connected!
[CONNECTED] Connection ID: conn_1
[newItem] ItemID=42 Author=bob Title=Hello Likes=0
[toggledLike] ItemID=42 Author=alice Liked=true Likes=1
[updatedItem] ItemID=42 Author=bob Title=Hello World
```

Press Ctrl+C to disconnect. The tool will show a summary of events received.

## Testing

To test websocket broadcasts:

1. Start the server: `go run .`
2. In another terminal, start the status tool: `./ws-status`
3. In another terminal, post an item via API:
   ```bash
   curl -X POST "http://localhost:8081/newpost?emailaddress=test%40example.com&emailcode=CODE" \
     -d 'jsontext={"description":"Test post"}'
   ```
4. Observe the `[newItem]` event appear in the status tool output

## Architecture

The tool:
- Connects to `/subscribe` endpoint via WebSocket
- Receives JSON-encoded events for items changes (new, updated, liked)
- Maintains connection with periodic ping messages
- Gracefully handles disconnection and Ctrl+C
- Counts and displays event statistics
