package db

import "database/sql"

// AddToLikesTable ports addToLikesTable. Idempotent: liking an
// already-liked item just refreshes whenCreated, same as the JS
// "replace into likes" did.
func AddToLikesTable(conn *sql.DB, screenname string, itemID int64) error {
	_, err := conn.Exec(`
		insert into likes (screenname, itemId) values (?, ?)
		on conflict (screenname, itemId) do update set whenCreated = current_timestamp`,
		screenname, itemID)
	return err
}

// RemoveFromLikesTable ports removeFromLikesTable.
func RemoveFromLikesTable(conn *sql.DB, screenname string, itemID int64) error {
	_, err := conn.Exec(`delete from likes where screenname = ? and itemId = ?`, screenname, itemID)
	return err
}

// IsLiked ports isLiked.
func IsLiked(conn *sql.DB, screenname string, itemID int64) (bool, error) {
	var ct int64
	err := conn.QueryRow(`select count(*) from likes where screenname = ? and itemId = ?`, screenname, itemID).Scan(&ct)
	if err != nil {
		return false, err
	}
	return ct > 0, nil
}

// GetLikersList ports getLikersList: screennames of everyone who liked an
// item, in the order they liked it.
func GetLikersList(conn *sql.DB, itemID int64) ([]string, error) {
	rows, err := conn.Query(`select screenname from likes where itemId = ? order by whenCreated`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []string
	for rows.Next() {
		var screenname string
		if err := rows.Scan(&screenname); err != nil {
			return nil, err
		}
		list = append(list, screenname)
	}
	return list, rows.Err()
}
