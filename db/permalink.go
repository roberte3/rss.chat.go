package db

import "strconv"

// PermalinkURL builds an item's guid/permalink. Ported from getPermalinkUrl
// in rssnetwork.js, which builds it from config.urlServerForClient rather
// than storing it — baseURL is that value, passed in by the caller.
func PermalinkURL(baseURL string, id int64) string {
	return baseURL + "?id=" + strconv.FormatInt(id, 10)
}

// InReplyToPermalink builds the permalink of the post an item replies to,
// or "" if it isn't a reply. Ported from getInReplyToPermalink.
func InReplyToPermalink(baseURL string, inReplyTo *int64) string {
	if inReplyTo == nil {
		return ""
	}
	return PermalinkURL(baseURL, *inReplyTo)
}

// CommentsFeedURL builds the address of a post's comments feed. Ported from
// getCommentsFeedUrl, which builds it from config.rssFeedUrl — rssFeedURL is
// that value, passed in by the caller.
func CommentsFeedURL(rssFeedURL, screenname string, idPost int64) string {
	return rssFeedURL + screenname + "/comments/" + strconv.FormatInt(idPost, 10) + ".xml"
}
