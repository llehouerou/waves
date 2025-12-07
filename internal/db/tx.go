package db

import (
	"database/sql"
)

// WithTx executes fn within a transaction.
// It handles Begin, Rollback on error, and Commit on success.
func WithTx(db *sql.DB, fn func(tx *sql.Tx) error) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // rollback on error is intentional

	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

// NullInt64ToPtr converts a sql.NullInt64 to *int64.
// Returns nil if the value is not valid.
func NullInt64ToPtr(n sql.NullInt64) *int64 {
	if !n.Valid {
		return nil
	}
	return &n.Int64
}

// NullInt64Value returns the int64 value or 0 if not valid.
func NullInt64Value(n sql.NullInt64) int64 {
	if !n.Valid {
		return 0
	}
	return n.Int64
}

// NullStringValue returns the string value or empty string if not valid.
func NullStringValue(n sql.NullString) string {
	if !n.Valid {
		return ""
	}
	return n.String
}
