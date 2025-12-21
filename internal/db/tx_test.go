package db

import (
	"database/sql"
	"errors"
	"testing"

	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	_, err = db.Exec(`CREATE TABLE test_table (id INTEGER PRIMARY KEY, value TEXT)`)
	if err != nil {
		db.Close()
		t.Fatalf("failed to create table: %v", err)
	}

	return db
}

func TestWithTx_Success(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	err := WithTx(db, func(tx *sql.Tx) error {
		_, err := tx.Exec(`INSERT INTO test_table (value) VALUES (?)`, "test")
		return err
	})

	if err != nil {
		t.Fatalf("WithTx failed: %v", err)
	}

	// Verify the insert was committed
	var count int
	err = db.QueryRow(`SELECT COUNT(*) FROM test_table`).Scan(&count)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}
}

func TestWithTx_Rollback(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	testErr := errors.New("test error")

	err := WithTx(db, func(tx *sql.Tx) error {
		_, err := tx.Exec(`INSERT INTO test_table (value) VALUES (?)`, "test")
		if err != nil {
			return err
		}
		return testErr // Return error to trigger rollback
	})

	if !errors.Is(err, testErr) {
		t.Fatalf("WithTx should return the error: got %v, want %v", err, testErr)
	}

	// Verify the insert was rolled back
	var count int
	err = db.QueryRow(`SELECT COUNT(*) FROM test_table`).Scan(&count)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if count != 0 {
		t.Errorf("count = %d, want 0 (rolled back)", count)
	}
}

func TestWithTx_MultipleOperations(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	err := WithTx(db, func(tx *sql.Tx) error {
		if _, err := tx.Exec(`INSERT INTO test_table (value) VALUES (?)`, "first"); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO test_table (value) VALUES (?)`, "second"); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO test_table (value) VALUES (?)`, "third"); err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		t.Fatalf("WithTx failed: %v", err)
	}

	var count int
	err = db.QueryRow(`SELECT COUNT(*) FROM test_table`).Scan(&count)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if count != 3 {
		t.Errorf("count = %d, want 3", count)
	}
}

func TestWithTx_PartialRollback(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	err := WithTx(db, func(tx *sql.Tx) error {
		if _, err := tx.Exec(`INSERT INTO test_table (value) VALUES (?)`, "first"); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO test_table (value) VALUES (?)`, "second"); err != nil {
			return err
		}
		// Return error after some operations
		return errors.New("abort")
	})

	if err == nil {
		t.Fatal("WithTx should return error")
	}

	// All operations should be rolled back
	var count int
	err = db.QueryRow(`SELECT COUNT(*) FROM test_table`).Scan(&count)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if count != 0 {
		t.Errorf("count = %d, want 0 (all rolled back)", count)
	}
}

func TestNullInt64ToPtr_Valid(t *testing.T) {
	n := sql.NullInt64{Int64: 42, Valid: true}

	ptr := NullInt64ToPtr(n)

	if ptr == nil {
		t.Fatal("expected non-nil pointer")
	}
	if *ptr != 42 {
		t.Errorf("*ptr = %d, want 42", *ptr)
	}
}

func TestNullInt64ToPtr_Invalid(t *testing.T) {
	n := sql.NullInt64{Int64: 42, Valid: false}

	ptr := NullInt64ToPtr(n)

	if ptr != nil {
		t.Errorf("expected nil pointer, got %d", *ptr)
	}
}

func TestNullInt64ToPtr_Zero(t *testing.T) {
	n := sql.NullInt64{Int64: 0, Valid: true}

	ptr := NullInt64ToPtr(n)

	if ptr == nil {
		t.Fatal("expected non-nil pointer for valid zero")
	}
	if *ptr != 0 {
		t.Errorf("*ptr = %d, want 0", *ptr)
	}
}

func TestNullInt64Value_Valid(t *testing.T) {
	n := sql.NullInt64{Int64: 123, Valid: true}

	result := NullInt64Value(n)

	if result != 123 {
		t.Errorf("result = %d, want 123", result)
	}
}

func TestNullInt64Value_Invalid(t *testing.T) {
	n := sql.NullInt64{Int64: 123, Valid: false}

	result := NullInt64Value(n)

	if result != 0 {
		t.Errorf("result = %d, want 0", result)
	}
}

func TestNullInt64Value_Zero(t *testing.T) {
	n := sql.NullInt64{Int64: 0, Valid: true}

	result := NullInt64Value(n)

	if result != 0 {
		t.Errorf("result = %d, want 0", result)
	}
}

func TestNullStringValue_Valid(t *testing.T) {
	n := sql.NullString{String: "hello", Valid: true}

	result := NullStringValue(n)

	if result != "hello" {
		t.Errorf("result = %q, want \"hello\"", result)
	}
}

func TestNullStringValue_Invalid(t *testing.T) {
	n := sql.NullString{String: "hello", Valid: false}

	result := NullStringValue(n)

	if result != "" {
		t.Errorf("result = %q, want empty string", result)
	}
}

func TestNullStringValue_Empty(t *testing.T) {
	n := sql.NullString{String: "", Valid: true}

	result := NullStringValue(n)

	if result != "" {
		t.Errorf("result = %q, want empty string", result)
	}
}

func TestNullInt64Value_Negative(t *testing.T) {
	n := sql.NullInt64{Int64: -42, Valid: true}

	result := NullInt64Value(n)

	if result != -42 {
		t.Errorf("result = %d, want -42", result)
	}
}

func TestNullInt64ToPtr_Negative(t *testing.T) {
	n := sql.NullInt64{Int64: -100, Valid: true}

	ptr := NullInt64ToPtr(n)

	if ptr == nil {
		t.Fatal("expected non-nil pointer")
	}
	if *ptr != -100 {
		t.Errorf("*ptr = %d, want -100", *ptr)
	}
}
