package api

import (
	"github.com/microcosm-cc/bluemonday"
)

// sanitizer mirrors the allowlist rssnetwork.js hands to sanitize-html as
// config.legalTags, added upstream on 7/23/26 (server v0.6.3). Keeping the two
// in step matters because the sanitized text is what lands in the database and
// therefore in every feed: a tag we keep and upstream drops is a permanent
// difference in the bytes a subscriber sees, not just a rendering detail.
//
// Deliberate divergence: rel="nofollow" on links. sanitize-html does not add
// it; bluemonday's UGCPolicy did, this port has always emitted it, and it is
// anti-spam rather than formatting. Drop RequireNoFollowOnLinks to match
// upstream byte for byte.
var sanitizer = newPostPolicy()

func newPostPolicy() *bluemonday.Policy {
	p := bluemonday.NewPolicy()

	// config.legalTags.allowedTags
	p.AllowElements("p", "br", "a", "b", "i", "strong", "em", "img",
		"blockquote", "ul", "ol", "li", "h3")

	// config.legalTags.allowedAttributes
	p.AllowAttrs("href").OnElements("a")
	p.AllowAttrs("src", "alt").OnElements("img")

	// sanitize-html's default allowedSchemes, applied to href and src.
	p.AllowURLSchemes("http", "https", "ftp", "mailto", "tel")

	// sanitize-html permits relative URLs, which is how uploaded media is
	// referenced: /media/123.
	p.RequireParseableURLs(true)
	p.AllowRelativeURLs(true)

	// sanitize-html's default nonTextTags: the tag and its text both go.
	// Every other disallowed tag is unwrapped, keeping its text.
	p.SkipElementsContent("script", "style", "textarea", "option")

	p.RequireNoFollowOnLinks(true)

	return p
}

// SanitizePostHTML strips anything outside the allowlist from post content on
// its way into the database, so a post cannot carry markup that runs in a
// reader's browser. Ordinary writing—links, bold, italic, quotes, lists and
// pasted images—passes through untouched.
func SanitizePostHTML(html string) string {
	if html == "" {
		return ""
	}
	return sanitizer.Sanitize(html)
}
