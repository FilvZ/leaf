package pgsql

import (
	"database/sql"
	"fmt"
	"github.com/name5566/leaf/log"
	"time"

	// PostgreSQL driver
	_ "github.com/lib/pq"
)

// DialContext wraps a *sql.DB connection pool for PostgreSQL
type DialContext struct {
	db *sql.DB
}

// goroutine safe
func Dial(dsn string, maxOpenConns int) (*DialContext, error) {
	return DialWithTimeout(dsn, maxOpenConns, 5*time.Minute)
}

// goroutine safe
func DialWithTimeout(dsn string, maxOpenConns int, maxLifetime time.Duration) (*DialContext, error) {
	if maxOpenConns <= 0 {
		maxOpenConns = 100
		log.Release("invalid maxOpenConns, reset to %v", maxOpenConns)
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(maxOpenConns / 4)
	db.SetConnMaxLifetime(maxLifetime)

	// verify connection
	if err = db.Ping(); err != nil {
		db.Close()
		return nil, err
	}

	c := new(DialContext)
	c.db = db

	return c, nil
}

// goroutine safe
func (c *DialContext) Close() {
	if c.db != nil {
		if err := c.db.Close(); err != nil {
			log.Error("pgsql close error: %v", err)
		}
	}
}

// goroutine safe
func (c *DialContext) DB() *sql.DB {
	return c.db
}

// EnsureSequence creates a sequence for auto-increment if it does not exist.
// goroutine safe
func (c *DialContext) EnsureSequence(name string) error {
	query := fmt.Sprintf("CREATE SEQUENCE IF NOT EXISTS %s", quoteIdent(name))
	_, err := c.db.Exec(query)
	if err != nil {
		log.Error("ensure sequence %v error: %v", name, err)
	}
	return err
}

// NextSeq returns the next value from a sequence.
// goroutine safe
func (c *DialContext) NextSeq(name string) (int64, error) {
	query := fmt.Sprintf("SELECT nextval('%s')", quoteIdent(name))
	var seq int64
	err := c.db.QueryRow(query).Scan(&seq)
	if err != nil {
		log.Error("next sequence %v error: %v", name, err)
		return 0, err
	}
	return seq, nil
}

// DropSequence drops a sequence.
// goroutine safe
func (c *DialContext) DropSequence(name string) error {
	query := fmt.Sprintf("DROP SEQUENCE IF EXISTS %s", quoteIdent(name))
	_, err := c.db.Exec(query)
	return err
}

// EnsureIndex creates a non-unique index on the specified table and columns.
// goroutine safe
func (c *DialContext) EnsureIndex(table string, columns []string) error {
	query := fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_%s ON %s (%s)",
		table, columns[0], quoteIdent(table), quoteIdentList(columns))
	_, err := c.db.Exec(query)
	if err != nil {
		log.Error("ensure index error: %v", err)
	}
	return err
}

// EnsureUniqueIndex creates a unique index on the specified table and columns.
// goroutine safe
func (c *DialContext) EnsureUniqueIndex(table string, columns []string) error {
	query := fmt.Sprintf("CREATE UNIQUE INDEX IF NOT EXISTS idx_%s_%s ON %s (%s)",
		table, columns[0], quoteIdent(table), quoteIdentList(columns))
	_, err := c.db.Exec(query)
	if err != nil {
		log.Error("ensure unique index error: %v", err)
	}
	return err
}

// EnsureTable creates a table if it does not exist.
// goroutine safe
func (c *DialContext) EnsureTable(table string, schema string) error {
	query := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (%s)", quoteIdent(table), schema)
	_, err := c.db.Exec(query)
	if err != nil {
		log.Error("ensure table %v error: %v", table, err)
	}
	return err
}

// DropTable drops a table if it exists.
// goroutine safe
func (c *DialContext) DropTable(table string) error {
	query := fmt.Sprintf("DROP TABLE IF EXISTS %s", quoteIdent(table))
	_, err := c.db.Exec(query)
	return err
}

// Exec executes a query without returning any rows.
// goroutine safe
func (c *DialContext) Exec(query string, args ...interface{}) (sql.Result, error) {
	return c.db.Exec(query, args...)
}

// Query executes a query that returns rows.
// goroutine safe
func (c *DialContext) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return c.db.Query(query, args...)
}

// QueryRow executes a query that is expected to return at most one row.
// goroutine safe
func (c *DialContext) QueryRow(query string, args ...interface{}) *sql.Row {
	return c.db.QueryRow(query, args...)
}

// quoteIdent quotes a PostgreSQL identifier.
func quoteIdent(name string) string {
	return `"` + name + `"`
}

// quoteIdentList quotes a list of PostgreSQL identifiers.
func quoteIdentList(names []string) string {
	result := ""
	for i, name := range names {
		if i > 0 {
			result += ", "
		}
		result += quoteIdent(name)
	}
	return result
}