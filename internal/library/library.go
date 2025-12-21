package library

import (
	"database/sql"
)

// executor is an interface satisfied by both *sql.DB and *sql.Tx.
// Used to allow FTS operations to run within transactions.
type executor interface {
	Exec(query string, args ...any) (sql.Result, error)
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

// Track represents a music track in the library.
type Track struct {
	ID           int64
	Path         string
	Mtime        int64
	Artist       string
	AlbumArtist  string
	Album        string
	Title        string
	DiscNumber   int
	TrackNumber  int
	Year         int
	Genre        string
	OriginalDate string // YYYY-MM-DD, YYYY-MM, or YYYY
	ReleaseDate  string // YYYY-MM-DD, YYYY-MM, or YYYY
	Label        string // Record label/publisher
}

// Album represents an album in the library.
type Album struct {
	Name string
	Year int
}

// Library manages the music library database.
type Library struct {
	db *sql.DB
}

// New creates a new Library instance.
func New(db *sql.DB) *Library {
	return &Library{db: db}
}
