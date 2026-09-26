package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

const userSelectColumns = `select screenname, emailAddress, emailSecret, imageUrl, prefs, whenCreated, whenUpdated from users`

func scanUser(s rowScanner) (*User, error) {
	var u User
	var emailAddress, emailSecret, imageURL, prefs sql.NullString
	if err := s.Scan(&u.Screenname, &emailAddress, &emailSecret, &imageURL, &prefs, &u.WhenCreated, &u.WhenUpdated); err != nil {
		return nil, err
	}
	u.EmailAddress = emailAddress.String
	u.EmailSecret = emailSecret.String
	u.ImageURL = imageURL.String
	if prefs.Valid {
		u.Prefs = json.RawMessage(prefs.String)
	}
	return &u, nil
}

// GetUserInfoByScreenname ports getUserInfoByScreenname. Returns (nil, nil)
// when there is no such user.
func GetUserInfoByScreenname(conn *sql.DB, screenname string) (*User, error) {
	u, err := scanUser(conn.QueryRow(userSelectColumns+` where screenname = ?`, screenname))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return u, nil
}

// GetUserInfoByEmail ports getUserInfoByEmail. Returns (nil, nil) when there
// is no such user.
func GetUserInfoByEmail(conn *sql.DB, email string) (*User, error) {
	u, err := scanUser(conn.QueryRow(userSelectColumns+` where emailAddress = ?`, email))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return u, nil
}

// AddUser ports addUser (minus the S3 subscription-list republish, which
// belongs to the Phase 3 publishing layer).
func AddUser(conn *sql.DB, screenname, emailAddress, emailSecret string) error {
	_, err := conn.Exec(`insert into users (screenname, emailAddress, emailSecret) values (?, ?, ?)`, screenname, emailAddress, emailSecret)
	return err
}

// UpdateUser ports updateUser.
func UpdateUser(conn *sql.DB, screenname, emailAddress, emailSecret string) error {
	result, err := conn.Exec(`update users set emailAddress = ?, emailSecret = ? where screenname = ?`, emailAddress, emailSecret, screenname)
	if err != nil {
		return err
	}
	ct, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if ct == 0 {
		return fmt.Errorf("Can't update the user because there is no user with screenname %q.", screenname)
	}
	return nil
}

// UpdateUserPrefs updates a user's preferences.
func UpdateUserPrefs(conn *sql.DB, screenname string, prefs []byte) error {
	result, err := conn.Exec(`update users set prefs = ? where screenname = ?`, string(prefs), screenname)
	if err != nil {
		return err
	}
	ct, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if ct == 0 {
		return fmt.Errorf("Can't update prefs because there is no user with screenname %q.", screenname)
	}
	return nil
}

// UpdateUserImageURL updates a user's avatar URL.
func UpdateUserImageURL(conn *sql.DB, screenname, imageURL string) error {
	result, err := conn.Exec(`update users set imageUrl = ? where screenname = ?`, imageURL, screenname)
	if err != nil {
		return err
	}
	ct, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if ct == 0 {
		return fmt.Errorf("Can't update imageUrl because there is no user with screenname %q.", screenname)
	}
	return nil
}

// GetUsersWithAvatars returns users who have avatars, ordered by screenname.
// ct (continuation token) is used for pagination - if non-empty, only users
// with screenname > ct are returned.
func GetUsersWithAvatars(conn *sql.DB, ct string, limit int) ([]*User, error) {
	query := `select screenname, emailAddress, emailSecret, imageUrl, prefs, whenCreated, whenUpdated
		from users where imageUrl is not null and imageUrl != ''`
	args := []interface{}{}

	if ct != "" {
		query += ` and screenname > ?`
		args = append(args, ct)
	}

	query += ` order by screenname asc`

	if limit > 0 {
		query += ` limit ?`
		args = append(args, limit)
	}

	rows, err := conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query users with avatars: %w", err)
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, user)
	}

	return users, rows.Err()
}

// GetAllScreennames ports getAllScreennames.
func GetAllScreennames(conn *sql.DB) ([]string, error) {
	rows, err := conn.Query(`select screenname from users`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var screennames []string
	for rows.Next() {
		var screenname string
		if err := rows.Scan(&screenname); err != nil {
			return nil, err
		}
		screennames = append(screennames, screenname)
	}
	return screennames, rows.Err()
}

// BumpUserHits ports bumpUserHits: increments the lifetime hit count, rolls
// ctHitsToday over to 1 when whenLastHit wasn't today, and stamps
// whenLastHit with now. See install.md's note on bumpUserHits.
func BumpUserHits(conn *sql.DB, screenname string) error {
	_, err := conn.Exec(`
		update users set
			ctHits = ctHits + 1,
			ctHitsToday = case when date (whenLastHit) = date ('now') then ctHitsToday + 1 else 1 end,
			whenLastHit = current_timestamp
		where screenname = ?;
	`, screenname)
	return err
}

// GetMostActiveToday ports getMostActiveToday.
func GetMostActiveToday(conn *sql.DB) ([]ActiveUser, error) {
	rows, err := conn.Query(`
		select
			screenname,
			coalesce (nullif (json_extract (prefs, '$.myFeedTitle'), ''), screenname) as name,
			json_extract (prefs, '$.myAvatarImageUrl') as imageUrl,
			ctHits, ctHitsToday, whenLastHit
		from users
		order by ctHitsToday desc, ctHits desc
		limit 100;
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []ActiveUser
	for rows.Next() {
		var a ActiveUser
		var imageURL sql.NullString
		var whenLastHit sql.NullTime
		if err := rows.Scan(&a.Screenname, &a.Name, &imageURL, &a.CtHits, &a.CtHitsToday, &whenLastHit); err != nil {
			return nil, err
		}
		a.ImageURL = imageURL.String
		a.WhenLastHit = whenLastHit.Time
		list = append(list, a)
	}
	return list, rows.Err()
}
