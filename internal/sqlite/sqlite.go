// Package sqlite holds the storage mechanics that every feature package
// shares: opening the file, running migrations, adding a missing column,
// reading rows, and retrying a slug collision. It also carries the driver
// import, so no other package needs it.
package sqlite

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"

	"jump-pad/internal/api"

	_ "modernc.org/sqlite"
)

// maxCollisionSuffix is the last candidate: slug, slug-2, up to slug-20.
const maxCollisionSuffix = 20

// Open opens the database file in WAL mode. The path ":memory:" opens a
// private database for a test.
func Open(path string) (*sql.DB, error) {
	dsn := path
	if path != ":memory:" {
		dsn = path + "?_pragma=journal_mode(WAL)"
	}
	return sql.Open("sqlite", dsn)
}

// Migrate runs each feature migration in order. It is safe on every
// startup, because each step is idempotent.
func Migrate(db *sql.DB, steps ...func(*sql.DB) error) error {
	for _, step := range steps {
		if err := step(db); err != nil {
			return err
		}
	}
	return nil
}

// EnsureColumn adds a column when a table does not have it yet. CREATE
// TABLE IF NOT EXISTS does nothing for a table that already exists, so
// every new column needs this.
func EnsureColumn(db *sql.DB, table, column, ddl string) error {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var id, notNull, primaryKey int
		var name, columnType string
		var defaultValue sql.NullString
		if err := rows.Scan(&id, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return err
		}
		if name == column {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	_, err = db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, ddl))
	return err
}
// InsertWithCollisionRetry tries base, then base-2 up to base-20, and
// stops at the first candidate that does not hit a UNIQUE collision.
func InsertWithCollisionRetry(base string, insert func(candidate string) error) (string, error) {
	for attempt := 1; attempt <= maxCollisionSuffix; attempt++ {
		candidate := base
		if attempt > 1 {
			candidate = fmt.Sprintf("%s-%d", base, attempt)
		}
		err := insert(candidate)
		if err == nil {
			return candidate, nil
		}
		if !IsUniqueViolation(err) {
			return "", err
		}
	}
	return "", fmt.Errorf("%w: %q and its %d suffixes are all taken", api.ErrSlugTaken, base, maxCollisionSuffix-1)
}

// IsUniqueViolation says whether err is a primary key or unique collision.
func IsUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint")
}

// RandomID returns a 12 character hex id from 6 random bytes.
func RandomID() string {
	raw := make([]byte, 6)
	rand.Read(raw)
	return hex.EncodeToString(raw)
}
