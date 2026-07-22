package db

// rowScanner is satisfied by both *sql.Row and *sql.Rows, letting scan
// helpers work with either a single-row QueryRow or a Query loop.
type rowScanner interface {
	Scan(dest ...any) error
}

// nullIfEmpty turns an empty string into a SQL NULL argument. Used for the
// optional "viewer" screenname parameter: comparing a column to NULL never
// matches, which reproduces the JS code's `likes.screenname = undefined`
// behavior for anonymous callers.
func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
