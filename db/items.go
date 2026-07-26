package db

import (
	"database/sql"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// DefaultMaxItems mirrors config.maxRecentItems/maxFeedItems's default of
// 100 in the JS server. Once Phase 7 wires up real config loading this
// should come from there instead of being a constant.
const DefaultMaxItems = 100

const itemColumns = `items.id, items.feedUrl, items.author, items.inReplyTo, items.title, items.link, items.description, items.pubDate, items.enclosureUrl, items.enclosureType, items.enclosureLength, items.whenCreated, items.whenUpdated, items.markdowntext, items.outlineJsontext, items.flDeleted`

const itemAuthorColumns = `json_extract (users.prefs, '$.myAvatarImageUrl') as imageUrl, json_extract (users.prefs, '$.myFeedTitle') as feedTitle, json_extract (users.prefs, '$.myFeedLink') as feedLink, json_extract (users.prefs, '$.myFeedDescription') as feedDescription`

// itemComputedColumns's first "?" binds the viewer's screenname (for
// flLiked); callers must pass it as the first query argument.
const itemComputedColumns = `
	(select count(*) from likes where likes.itemId = items.id) as ctLikes,
	(select count(*) from likes where likes.itemId = items.id and likes.screenname = ?) as flLiked,
	(select count(*) from items c where c.inReplyTo = items.id and (c.flDeleted is null or c.flDeleted = 0)) as ctReplies,
	(select coalesce (nullif (json_extract (u2.prefs, '$.myFeedTitle'), ''), i2.author)
		from items i2 left join users u2 on u2.screenname = i2.author
		where i2.id = items.inReplyTo) as inReplyToAuthor`

// bareItemQuery mirrors getItemById's query, which (unlike every other item
// read in rssnetwork.js) does not join users, so it has no imageUrl/
// feedTitle/feedLink/feedDescription.
const bareItemQuery = `select ` + itemColumns + `, ` + itemComputedColumns + ` from items`

// joinedItemQuery mirrors getItemByGuid/getRecentItems/getRecentUserItems/
// getItemAndReplies's shared query shape.
const joinedItemQuery = `select ` + itemColumns + `, ` + itemAuthorColumns + `, ` + itemComputedColumns + ` from items left join users on users.screenname = items.author`

type rawItem struct {
	id                                             int64
	feedURL, author, title, link, description      sql.NullString
	inReplyTo                                      sql.NullInt64
	pubDate                                        sql.NullTime
	enclosureURL, enclosureType                    sql.NullString
	enclosureLength                                sql.NullInt64
	whenCreated, whenUpdated                       time.Time
	markdownText, outlineJSONText                  sql.NullString
	flDeleted                                      int64
	imageURL, feedTitle, feedLink, feedDescription sql.NullString
	ctLikes, flLiked, ctReplies                    int64
	inReplyToAuthor                                sql.NullString
}

func scanBareItemRow(s rowScanner) (*rawItem, error) {
	var r rawItem
	err := s.Scan(&r.id, &r.feedURL, &r.author, &r.inReplyTo, &r.title, &r.link, &r.description, &r.pubDate,
		&r.enclosureURL, &r.enclosureType, &r.enclosureLength, &r.whenCreated, &r.whenUpdated,
		&r.markdownText, &r.outlineJSONText, &r.flDeleted, &r.ctLikes, &r.flLiked, &r.ctReplies, &r.inReplyToAuthor)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func scanJoinedItemRow(s rowScanner) (*rawItem, error) {
	var r rawItem
	err := s.Scan(&r.id, &r.feedURL, &r.author, &r.inReplyTo, &r.title, &r.link, &r.description, &r.pubDate,
		&r.enclosureURL, &r.enclosureType, &r.enclosureLength, &r.whenCreated, &r.whenUpdated,
		&r.markdownText, &r.outlineJSONText, &r.flDeleted,
		&r.imageURL, &r.feedTitle, &r.feedLink, &r.feedDescription,
		&r.ctLikes, &r.flLiked, &r.ctReplies, &r.inReplyToAuthor)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// rawItemToItem ports convertItem: it computes guid, inReplyToUrl, and the
// display-name-vs-account-id split from the raw row.
func rawItemToItem(r *rawItem, baseURL string) *Item {
	item := &Item{
		ID:              r.id,
		FeedURL:         r.feedURL.String,
		Guid:            PermalinkURL(baseURL, r.id),
		Title:           r.title.String,
		Link:            r.link.String,
		Description:     r.description.String,
		EnclosureURL:    r.enclosureURL.String,
		EnclosureType:   r.enclosureType.String,
		WhenCreated:     r.whenCreated,
		WhenUpdated:     r.whenUpdated,
		MarkdownText:    r.markdownText.String,
		OutlineJSONText: r.outlineJSONText.String,
		ImageURL:        r.imageURL.String,
		Screenname:      r.author.String,
		FeedLink:        r.feedLink.String,
		FeedDescription: r.feedDescription.String,
		FlDeleted:       r.flDeleted != 0,
		CtLikes:         r.ctLikes,
		FlLiked:         r.flLiked > 0,
		InReplyToAuthor: r.inReplyToAuthor.String,
		CtReplies:       r.ctReplies,
	}
	if r.pubDate.Valid {
		item.PubDate = r.pubDate.Time
	}
	if r.inReplyTo.Valid {
		v := r.inReplyTo.Int64
		item.InReplyToNum = &v
		item.InReplyToURL = InReplyToPermalink(baseURL, &v)
	}
	if r.enclosureLength.Valid {
		v := r.enclosureLength.Int64
		item.EnclosureLength = &v
	}
	if r.feedTitle.Valid && r.feedTitle.String != "" {
		item.Author = r.feedTitle.String
	} else {
		item.Author = r.author.String
	}
	return item
}

func collectItems(rows *sql.Rows, baseURL string) ([]Item, error) {
	defer rows.Close()
	var items []Item
	for rows.Next() {
		raw, err := scanJoinedItemRow(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *rawItemToItem(raw, baseURL))
	}
	return items, rows.Err()
}

// idFromGuid extracts the numeric id from a permalink guid, e.g.
// "https://x/?id=204" -> 204. Ports the utils.stringNthField(guid, "=", 2)
// call in getItemByGuid.
func idFromGuid(guid string) (int64, error) {
	if u, err := url.Parse(guid); err == nil {
		if idStr := u.Query().Get("id"); idStr != "" {
			return strconv.ParseInt(idStr, 10, 64)
		}
	}
	if i := strings.LastIndex(guid, "="); i >= 0 {
		if id, err := strconv.ParseInt(guid[i+1:], 10, 64); err == nil {
			return id, nil
		}
	}
	return 0, fmt.Errorf("Can't parse an id out of guid %q.", guid)
}

// GetItemByID ports getItemById. viewerScreenname may be "" for an
// anonymous caller. Returns (nil, nil) when there is no such item.
// Uses joinedItemQuery to include author info so parent posts can be
// republished when used in reply operations.
func GetItemByID(conn *sql.DB, viewerScreenname string, id int64, baseURL string) (*Item, error) {
	raw, err := scanJoinedItemRow(conn.QueryRow(joinedItemQuery+` where items.id = ?`, nullIfEmpty(viewerScreenname), id))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return rawItemToItem(raw, baseURL), nil
}

// GetItemByGuid ports getItemByGuid, including the "can't view a deleted
// post" check. Returns (nil, nil) when there is no such item.
func GetItemByGuid(conn *sql.DB, viewerScreenname, guid, baseURL string) (*Item, error) {
	if guid == "" {
		return nil, fmt.Errorf("Can't get the item record because the GUID param is undefined.")
	}
	id, err := idFromGuid(guid)
	if err != nil {
		return nil, err
	}
	raw, err := scanJoinedItemRow(conn.QueryRow(joinedItemQuery+` where items.id = ?`, nullIfEmpty(viewerScreenname), id))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	item := rawItemToItem(raw, baseURL)
	if item.FlDeleted {
		return nil, fmt.Errorf("Can't view the post because it has been deleted.")
	}
	return item, nil
}

// GetItemAndReplies ports getItemAndReplies: a post and its direct replies,
// oldest first, as one flat slice.
func GetItemAndReplies(conn *sql.DB, viewerScreenname string, idParent int64, baseURL string) ([]Item, error) {
	query := joinedItemQuery + ` where (items.id = ? or items.inReplyTo = ?) and (items.flDeleted is null or items.flDeleted = 0) order by items.pubDate asc`
	rows, err := conn.Query(query, nullIfEmpty(viewerScreenname), idParent, idParent)
	if err != nil {
		return nil, err
	}
	return collectItems(rows, baseURL)
}

// GetRecentItems ports getRecentItems. maxCt <= 0 or greater than
// DefaultMaxItems is clamped to DefaultMaxItems, matching the JS default/cap
// behavior.
func GetRecentItems(conn *sql.DB, viewerScreenname string, maxCt int, baseURL string) ([]Item, error) {
	if maxCt <= 0 || maxCt > DefaultMaxItems {
		maxCt = DefaultMaxItems
	}
	query := joinedItemQuery + ` where (items.flDeleted is null or items.flDeleted = 0) order by items.pubDate desc limit ?`
	rows, err := conn.Query(query, nullIfEmpty(viewerScreenname), maxCt)
	if err != nil {
		return nil, err
	}
	return collectItems(rows, baseURL)
}

// GetRecentUserItems ports getRecentUserItems: one user's posts, newest
// first.
func GetRecentUserItems(conn *sql.DB, viewerScreenname, feedURL string, maxCt int, baseURL string) ([]Item, error) {
	if maxCt <= 0 {
		maxCt = DefaultMaxItems
	}
	query := joinedItemQuery + ` where items.feedUrl = ? and (items.flDeleted is null or items.flDeleted = 0) order by items.pubDate desc limit ?`
	rows, err := conn.Query(query, nullIfEmpty(viewerScreenname), feedURL, maxCt)
	if err != nil {
		return nil, err
	}
	return collectItems(rows, baseURL)
}

// NewItem carries the fields addItem is allowed to set on insert.
type NewItem struct {
	FeedURL         string
	Title           string
	Link            string
	Description     string
	InReplyTo       *int64
	PubDate         time.Time
	EnclosureURL    string
	EnclosureType   string
	EnclosureLength *int64
	MarkdownText    string
	OutlineJSONText string
	Author          string
}

// AddItem ports addItem (minus the S3 republish and websocket notify, which
// belong to later phases) and returns the new row's id.
func AddItem(conn *sql.DB, item NewItem) (int64, error) {
	result, err := conn.Exec(`
		insert into items (feedUrl, title, link, description, inReplyTo, pubDate, enclosureUrl, enclosureType, enclosureLength, markdowntext, outlineJsontext, author)
		values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		nullIfEmpty(item.FeedURL), nullIfEmpty(item.Title), nullIfEmpty(item.Link), nullIfEmpty(item.Description),
		item.InReplyTo, item.PubDate, nullIfEmpty(item.EnclosureURL), nullIfEmpty(item.EnclosureType),
		item.EnclosureLength, nullIfEmpty(item.MarkdownText), nullIfEmpty(item.OutlineJSONText), nullIfEmpty(item.Author))
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// UpdateItem ports updateItem's dynamic set-clause: only non-nil fields are
// written, matching the JS "add if defined" behavior.
func UpdateItem(conn *sql.DB, patch ItemPatch) error {
	if patch.ID == 0 {
		return fmt.Errorf("Can't update the item because no id was provided.")
	}

	var setParts []string
	var args []any
	add := func(col string, val any) {
		setParts = append(setParts, col+" = ?")
		args = append(args, val)
	}
	if patch.FeedURL != nil {
		add("feedUrl", *patch.FeedURL)
	}
	if patch.Title != nil {
		add("title", *patch.Title)
	}
	if patch.Link != nil {
		add("link", *patch.Link)
	}
	if patch.Description != nil {
		add("description", *patch.Description)
	}
	if patch.InReplyTo != nil {
		add("inReplyTo", *patch.InReplyTo)
	}
	if patch.PubDate != nil {
		add("pubDate", *patch.PubDate)
	}
	if patch.EnclosureURL != nil {
		add("enclosureUrl", *patch.EnclosureURL)
	}
	if patch.EnclosureType != nil {
		add("enclosureType", *patch.EnclosureType)
	}
	if patch.EnclosureLength != nil {
		add("enclosureLength", *patch.EnclosureLength)
	}
	if patch.MarkdownText != nil {
		add("markdowntext", *patch.MarkdownText)
	}
	if patch.OutlineJSONText != nil {
		add("outlineJsontext", *patch.OutlineJSONText)
	}
	if patch.Author != nil {
		add("author", *patch.Author)
	}
	if len(setParts) == 0 {
		return fmt.Errorf("Can't update the item because no fields were provided.")
	}

	args = append(args, patch.ID)
	query := "update items set " + strings.Join(setParts, ", ") + " where id = ?"
	result, err := conn.Exec(query, args...)
	if err != nil {
		return err
	}
	ct, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if ct == 0 {
		return fmt.Errorf("Can't update the item because there is no item with id %d.", patch.ID)
	}
	return nil
}
