# RSS Chat Go

A modern Go implementation of [Dave Winer's RSS Chat](https://github.com/scripting/rss.chat) — a distributed social networking platform built on RSS feeds with real-time updates via WebSocket.

![Go Version](https://img.shields.io/badge/Go-1.25+-blue)
![License](https://img.shields.io/badge/license-MIT-green)
[![Tests](https://img.shields.io/badge/tests-passing-brightgreen)](#testing)

## Overview

RSS Chat Go is a backend service that enables real-time social conversations powered by RSS. Users can post messages, reply to conversations, like posts, and share media — all synchronized in real-time across clients. The system is designed for scalability and self-hosting.

The overall longer term goals of this fork are to make a backend that is easily spun up on a wide varity of systems (including docker/ Kubernetes containers), so a wide variety of communities can easily build communities. Also this backend will make it easier to build mobile applications that publish into the wider community. 

**Key Characteristics:**
- ✅ Pure Go implementation (no CGO required)
- ✅ SQLite database with WAL mode for concurrent access
- ✅ Separate media database for scalability
- ✅ Real-time WebSocket updates
- ✅ RSS feed generation for subscriptions
- ✅ HTML sanitization against XSS attacks
- ✅ RESTful API with JSON responses
- ✅ Backup/restore tools for data migration

## Features

### Core Functionality
- **User Management**: Screenname-based profiles with email authentication
- **Posts**: Create, update, delete posts with text and media attachments
- **Conversations**: Thread replies with parent-child relationships
- **Likes**: Like/unlike posts with aggregated counts
- **Media Upload**: Image support (JPEG, PNG, GIF, WebP, SVG) with validation
- **Real-time Updates**: WebSocket broadcasts for likes, posts, and deletions

### Security
- Email-based authentication with confirmation codes
- HTML sanitization (bluemonday UGC policy)
- Magic byte verification for uploaded media
- Content-type whitelist enforcement
- Parameterized queries (SQL injection protection)
- Rate limiting on the endpoints that send mail

See [Security model](#security-model) for what the authentication scheme does
and does not protect against. Read it before running this on a public domain.

### Scalability
- Separate media database for independent scaling
- Stateless API handlers (horizontal scaling ready)
- Connection pooling and WAL mode
- Temporary file storage during media validation

## Quick Start

### Prerequisites

- Go 1.25 or later
- SQLite 3 (included in modernc.org/sqlite pure-Go driver)

### Installation

```bash
# Clone the repository
git clone https://github.com/roberte3/rss.chat.go.git
cd rss.chat.go

# Download dependencies
go mod download

# Build the server
go build ./...

# (Optional) Build CLI tools
go build -o backup ./tools/backup
go build -o restore ./tools/restore
```

### Running the Server

```bash
# Create default configuration
go run . -setup

# Start the server
go run .
```

The server will:
1. Load configuration from `config.json`
2. Create SQLite databases if they don't exist
3. Listen on port 8081 (configurable)
4. Serve the web client and API

Visit `http://localhost:8081` in your browser.

## Configuration

Configuration is stored in `config.json`. Create or modify this file to customize:

```json
{
  "myDomain": "http://localhost:8081",
  "productName": "RSS Chat",
  "productNameForDisplay": "RSS Chat",
  "httpPort": 8081,
  "websocketPort": 1462,
  "databasePath": "rss.chat.db",
  "mediaDBPath": "rss.chat.media.db",
  "tempMediaPath": "temp_media",
  "maxMediaUploadBytes": 2097152,
  "feedsPath": "feeds",
  "smtpHost": "localhost",
  "smtpPort": 25,
  "smtpUsername": "",
  "smtpPassword": "",
  "mailSender": "noreply@localhost",
  "urlServerForClient": "http://localhost:8081/api",
  "urlWebsocketServerForClient": "ws://localhost:8081",
  "flWebsocketEnabled": true,
  "whitelist": [],
  "blockedUsersList": []
}
```

### Configuration Options

| Option | Description | Default |
|--------|-------------|---------|
| `myDomain` | Public domain for RSS generation | `http://localhost:8081` |
| `productName` | Internal product name | `RSS Chat` |
| `httpPort` | HTTP server port | `8081` |
| `websocketPort` | WebSocket server port | `1462` |
| `databasePath` | Main SQLite database file | `rss.chat.db` |
| `mediaDBPath` | Media SQLite database file | `rss.chat.media.db` |
| `tempMediaPath` | Temporary storage directory for uploads | `temp_media` |
| `maxMediaUploadBytes` | Maximum file size for uploads (bytes) | `2097152` (2MB) |
| `smtpHost` | SMTP server for email | `localhost` |
| `smtpPort` | SMTP server port | `25` |
| `flWebsocketEnabled` | Enable real-time WebSocket updates | `true` |

## API Endpoints

### Read Endpoints (No Authentication)

```
GET /health                    # Health check
GET /feed                      # RSS feed (all users or specific user)
GET /getrecentitems            # Recent posts across network
GET /getrecentuseritems        # User's recent posts
GET /getitembyguid             # Get post by GUID
GET /getitemandreplies         # Get post and its replies
GET /getiteminfo               # Get post metadata
GET /getuserdata               # Get user profile
GET /getlikerslist             # List users who liked a post
GET /getmostactivetoday        # Most active users (24h)
GET /getsubscriptionlist       # OPML subscription list
GET /isuserindatabase          # Check if user exists
GET /isemailindatabase         # Check if email exists
GET /checkwhitelist            # Check email whitelist status
```

### Auth Endpoints

```
GET /sendconfirmingemail       # Send confirmation email
GET /createnewuser             # Create new user account
```

### Write Endpoints (Authenticated)

```
POST /newpost                  # Create new post
POST /updatepost               # Update post
POST /deletepost               # Delete post
POST /togglelike               # Like/unlike a post
POST /saveprefs                # Save user preferences
POST /uploadmedia              # Upload media file
```

### Media Endpoint

```
GET /media/{id}                # Retrieve uploaded media by ID
```

### WebSocket

```
GET /subscribe                 # Subscribe to real-time updates
GET /ws                        # Alternative WebSocket endpoint
```

## Usage Examples

### Create a User

```bash
curl "http://localhost:8081/createnewuser?email=user@example.com&screenname=alice"
```

### Create a Post

```bash
curl -X POST http://localhost:8081/newpost \
  -d "emailaddress=user@example.com" \
  -d "emailcode=<confirmation-code>" \
  -d "jsontext={\"text\":\"Hello, World!\"}"
```

### Like a Post

```bash
curl -X POST http://localhost:8081/togglelike \
  -d "emailaddress=user@example.com" \
  -d "emailcode=<confirmation-code>" \
  -d "id=<post-id>"
```

### Upload Media

```bash
# Read image and convert to base64
base64 -i image.jpg > /tmp/image.b64

curl -X POST http://localhost:8081/uploadmedia \
  -F "emailaddress=user@example.com" \
  -F "emailcode=<confirmation-code>" \
  -F "data=<base64-image-data>" \
  -F "contentType=image/jpeg"
```

## CLI Tools

### Backup Tool

Export complete database state to JSON:

```bash
./backup -db rss.chat.db -mediadb rss.chat.media.db -o backup.json
```

### Restore Tool

Import database from backup (into empty databases):

```bash
./restore -f backup.json -db rss.chat.db -mediadb rss.chat.media.db
```

### WebSocket Status Monitor

Monitor real-time events:

```bash
./websocket-status -server ws://localhost:8081 -v
```

See [tools/TOOLS.md](tools/TOOLS.md) for detailed documentation.

## Project Structure

```
rss.chat.go/
├── main.go                 # Entry point and HTTP server setup
├── config/                 # Configuration loading and validation
├── db/                     # Database operations
│   ├── db.go              # Main database (users, items, likes)
│   └── media.go           # Media database operations
├── api/                    # HTTP API handlers
│   ├── handler.go         # Route registration
│   ├── uploadmedia.go     # Media upload with validation
│   ├── newpost.go         # Post creation
│   └── ...other endpoints
├── feed/                   # RSS feed generation
├── publish/                # Feed publishing to disk
├── websocket/              # Real-time updates
├── client/                 # Web client server
│   ├── client.go          # Serves client/code, substitutes [%macros%]
│   └── code/              # Vendored upstream web client (MIT, Dave Winer)
├── setup/                  # Database initialization
└── tools/                  # CLI tools
    ├── backup/            # Database export
    ├── restore/           # Database import
    ├── reset/             # Database/config reset
    ├── testdata/          # Sample data generator
    └── websocket-status/  # Event monitor
```

## Architecture

### Database Design

**Main Database (rss.chat.db):**
- `users`: Screenname-based profiles
- `items`: Posts with parent-child threading
- `likes`: Like relationships
- Uses SQLite WAL mode for concurrent reads

**Media Database (rss.chat.media.db):**
- `media`: Uploaded files with binary blob storage
- Separate database enables:
  - Independent backups
  - Future migration to external storage
  - Easier cleanup/archival

### Media Validation Pipeline

1. **Size Check**: Validate against `maxMediaUploadBytes`
2. **Content-Type Whitelist**: Restrict to image/* types
3. **Magic Byte Verification**: Prevent spoofed file types
4. **Temporary Storage**: Write to filesystem for safety
5. **Database Insertion**: Commit to media database
6. **Cleanup**: Remove temporary file

### Real-time Architecture

WebSocket hub broadcasts events to connected clients:
- `newpost`: Post created
- `like`: Post liked
- `deletepost`: Post deleted
- Connection IDs allow targeted updates

## Testing

Run all tests:

```bash
go test ./...
```

Run tests with verbose output:

```bash
go test ./... -v
```

Run tests for a specific package:

```bash
go test ./api -v
go test ./db -v
go test ./tools/backup -v
```

### Test Coverage

- **Database Layer**: CRUD operations, media handling, ID preservation
- **API Layer**: Upload validation, handler authentication, error responses
- **Integration**: Full backup/restore roundtrips with data integrity
- **Media**: Magic byte verification, signature detection, base64 encoding

**Current Status:** 168 tests passing

## Development

### Building from Source

```bash
# Build all packages
go build ./...

# Build with output file
go build -o server .

# Build with version info
go build -ldflags="-X main.Version=1.0.0" .
```

### Running Tests During Development

```bash
# Run tests with race detector
go test -race ./...

# Run tests on code changes (requires entr)
find . -name '*.go' | entr go test ./...
```

### Debugging

Enable verbose logging (configure in code):

```go
log.SetFlags(log.LstdFlags | log.Lshortfile)
```

Monitor WebSocket events:

```bash
./websocket-status -v
```

## Feature Parity

This implementation targets feature parity with [RSS.Chat v0.6.3](https://github.com/scripting/rss.chat/releases/tag/v0.6.3):

- ✅ User management and authentication
- ✅ Post creation, updates, deletion
- ✅ Threaded conversations
- ✅ Like/unlike functionality
- ✅ RSS feed generation
- ✅ Media uploads with validation
- ✅ Real-time WebSocket updates
- ✅ Backup/restore tools
- 🔄 Email notifications (in progress)
- 🔄 rssCloud ping support
- 🔄 Advanced preferences/profiles

See [todolist.md](todolist.md) for detailed roadmap.

## Security Considerations

### Implemented
- Email-based access codes (sent via SMTP)
- HTML sanitization via bluemonday (prevents XSS)
- Magic byte verification for uploads
- Content-type whitelist
- Parameterized SQL queries
- CORS headers for API endpoints
- WAL mode with foreign key constraints

### Recommended Deployment
- Use HTTPS in production (reverse proxy)
- Enable email whitelist for private instances
- Regular backups to separate storage
- Monitor disk usage (media database growth)
- Configure SMTP for real email delivery
- Rate limit authentication endpoints

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes
4. Write tests for new functionality
5. Ensure all tests pass (`go test ./...`)
6. Commit with clear messages
7. Push to your branch
8. Open a Pull Request

## Security model

Authentication is inherited from the original rss.chat and is worth
understanding before you run this on a public domain. None of what follows is a
bug report — it is the design, written down so you can decide whether it suits
your deployment.

### How it works

`emailSecret` is a **bearer token**: 32 bytes from `crypto/rand`, mailed to the
user as a sign-in link. Whoever holds it is the user. There are no sessions and
no cookies.

Two properties follow, one good and one not:

- **CSRF does not apply.** The credential is an explicit parameter, never
  ambient like a cookie, so a hostile page cannot make a browser spend it. Do
  not "fix" this toward cookie sessions without adding CSRF protection.
- **The token never expires and is never rotated.** The link in the signup
  email is the permanent credential. An account is as secure as its mailbox and
  as any copy of that link that still exists. There is no revocation; the only
  recovery is to change `emailSecret` in the database by hand.

### Credentials travel in the URL

The web client sends `emailaddress` and `emailcode` as query parameters on every
request, including POSTs. So a credential appears in anything that records
request URLs: reverse-proxy and CDN access logs, browser history, and the
address bar during sign-in confirmation.

The server accepts credentials from the query string because its own client
sends them that way; requiring a request body would break every write from the
bundled UI. Two mitigations are in place:

- `Referrer-Policy: no-referrer` on client responses, so the confirmation URL
  cannot leak to a third party through a `Referer` header — which matters
  because avatar URLs are user-supplied and load cross-origin.
- `Cache-Control: no-store` on authenticated responses, so neither the URL nor
  its response is written to a shared or on-disk cache.

**If you deploy this, configure your reverse proxy not to log query strings.**
That is the remaining exposure and it is outside the server's control.

### What is protected

- Confirmation-mail endpoints are rate-limited per mailbox and per source
  address, so neither a third party's inbox nor this server's sending
  reputation can be flooded. Defaults are in `api/handler.go`.
- The secret is compared in constant time.
- The secret is never serialized into a response.
- Post ownership is checked before update and delete.

### What is not

- **No transport security of its own.** Terminate TLS in front of this. Over
  plain HTTP every request hands the credential to the network.
- **Account existence is public**, deliberately: `/isuserindatabase` and
  `/isemailindatabase` are unauthenticated because the signup flow uses them.
  Authentication errors also distinguish an unknown address from a wrong code.
- **The client stores the token in `localStorage`**, so any XSS is a permanent
  account compromise rather than a session hijack. This is why the HTML
  sanitization matters more here than the feature list suggests.
- **No admin roles.** `IsUserAdmin` always returns false, as in the original.

## License

The Go server is licensed under the MIT License — see [LICENSE](LICENSE).

### Third-party code

`client/code/` is **not** part of the Go server. It is the web client from
[Dave Winer's rss.chat](https://github.com/scripting/rss.chat), vendored so
that a clone serves a working site with no extra setup. It is byte-for-byte
upstream, with no local modifications.

That directory is covered by its own copy of Dave Winer's MIT license at
[client/code/LICENSE](client/code/LICENSE), not by the license above.
[client/code/README-VENDORED.md](client/code/README-VENDORED.md) records the
exact upstream commit and how to re-sync it.

## Acknowledgments

- [Dave Winer](https://en.wikipedia.org/wiki/Dave_Winer) — creator of RSS Chat,
  and author of the web client vendored in `client/code/`
- [Original RSS.Chat Repository](https://github.com/scripting/rss.chat)
- Go community and excellent standard library

## Support

For issues, questions, or suggestions:

1. Check [todolist.md](todolist.md) for known issues and roadmap
2. Review [tools/TOOLS.md](tools/TOOLS.md) for CLI tool questions
3. Check existing GitHub issues
4. Open a new issue with:
   - Go version (`go version`)
   - Steps to reproduce
   - Expected vs actual behavior
   - Configuration (redacted)

## Roadmap

### Phase 1 (Complete)
- ✅ Core user and post management
- ✅ WebSocket real-time updates
- ✅ Media database and uploads
- ✅ HTML sanitization
- ✅ Backup/restore tools

### Phase 2 (In Progress)
- 🔄 HTMX based front end
- 🔄 Mobile based front end
- 🔄 Bluesky (ATProtocol) Bridge 
- 🔄 Email notifications
- 🔄 Feed format negotiation
- 🔄 Configurable ports and domains

### Phase 3 (Planned)
- ⏳ Post cleanup/archival
- ⏳ Advanced blocking/filtering
- ⏳ rssCloud support
- ⏳ Kubernetes/container deployment

See [todolist.md](todolist.md) for detailed feature breakdown by tier.

---

**Built with ❤️ in Go**
