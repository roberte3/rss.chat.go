package feed

import "encoding/xml"

// RSSFeed represents an RSS 2.0 feed.
type RSSFeed struct {
	XMLName xml.Name `xml:"rss"`
	Version string   `xml:"version,attr"`
	Channel *Channel `xml:"channel"`
	NS      string   `xml:"xmlns:source,attr"`
}

// Channel represents the RSS channel element.
type Channel struct {
	Title       string  `xml:"title"`
	Link        string  `xml:"link"`
	Description string  `xml:"description"`
	Language    string  `xml:"language,omitempty"`
	Docs        string  `xml:"docs,omitempty"`
	Generator   string  `xml:"generator,omitempty"`
	Image       *Image  `xml:"image,omitempty"`
	Cloud       *Cloud  `xml:"cloud,omitempty"`
	SelfLink    *Link   `xml:"source:self,omitempty"`
	Items       []*Item `xml:"item"`
}

// Image represents the RSS image element.
type Image struct {
	URL         string `xml:"url"`
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description,omitempty"`
}

// Cloud represents the RSS cloud element for notifications.
type Cloud struct {
	Domain            string `xml:"domain,attr"`
	Port              string `xml:"port,attr"`
	Path              string `xml:"path,attr"`
	RegisterProcedure string `xml:"registerProcedure,attr"`
	Protocol          string `xml:"protocol,attr"`
}

// Item represents an RSS item.
type Item struct {
	Title        string     `xml:"title,omitempty"`
	Link         string     `xml:"link,omitempty"`
	Description  string     `xml:"description,omitempty"`
	Guid         *Guid      `xml:"guid,omitempty"`
	PubDate      string     `xml:"pubDate,omitempty"`
	Enclosure    *Enclosure `xml:"enclosure,omitempty"`
	Comments     *Comments  `xml:"source:comments,omitempty"`
	InReplyTo    *InReplyTo `xml:"source:inReplyTo,omitempty"`
	Source       *Source    `xml:"source,omitempty"`
	MarkdownText string     `xml:"source:markdown,omitempty"`
	Account      *Account   `xml:"source:account,omitempty"`
}

// Guid represents the guid element with isPermalink attribute.
type Guid struct {
	IsPermalink string `xml:"isPermalink,attr"`
	Value       string `xml:",chardata"`
}

// Enclosure represents the enclosure element.
type Enclosure struct {
	URL    string `xml:"url,attr"`
	Type   string `xml:"type,attr"`
	Length string `xml:"length,attr,omitempty"`
}

// Comments represents the source:comments element.
type Comments struct {
	Count   int64  `xml:"count,attr"`
	FeedURL string `xml:"feedUrl,attr"`
}

// InReplyTo represents the source:inReplyTo element.
type InReplyTo struct {
	IsPermalink string `xml:"isPermalink,attr"`
	Value       string `xml:",chardata"`
}

// Source represents the source element for attribution.
type Source struct {
	URL   string `xml:"url"`
	Title string `xml:"title"`
}

// Account represents the source:account element.
type Account struct {
	Service string `xml:"service,attr"`
	Name    string `xml:"name,attr"`
}

// OPMLFeed represents an OPML document for subscription lists.
type OPMLFeed struct {
	XMLName xml.Name `xml:"opml"`
	Version string   `xml:"version,attr"`
	Head    *Head    `xml:"head"`
	Body    *Body    `xml:"body"`
}

// Head represents the OPML head.
type Head struct {
	Title        string `xml:"title"`
	DateModified string `xml:"dateModified"`
}

// Body represents the OPML body.
type Body struct {
	Outlines []*Outline `xml:"outline"`
}

// Outline represents an OPML outline (subscription).
type Outline struct {
	Type   string `xml:"type,attr"`
	Text   string `xml:"text,attr"`
	XMLURL string `xml:"xmlUrl,attr"`
}
