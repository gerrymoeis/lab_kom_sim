package database

import (
	"database/sql"
	"fmt"
	"strings"
)

type DB struct {
	*sql.DB
	rewrite bool
}

func wrapPG(db *sql.DB) *DB {
	return &DB{DB: db, rewrite: true}
}

func wrapSQLite(db *sql.DB) *DB {
	return &DB{DB: db, rewrite: false}
}

func (db *DB) Query(query string, args ...interface{}) (*sql.Rows, error) {
	if db.rewrite {
		query = rewriteQM(query)
	}
	return db.DB.Query(query, args...)
}

func (db *DB) QueryRow(query string, args ...interface{}) *sql.Row {
	if db.rewrite {
		query = rewriteQM(query)
	}
	return db.DB.QueryRow(query, args...)
}

func (db *DB) Exec(query string, args ...interface{}) (sql.Result, error) {
	if db.rewrite {
		query = rewriteQM(query)
	}
	return db.DB.Exec(query, args...)
}

func (db *DB) Prepare(query string) (*sql.Stmt, error) {
	if db.rewrite {
		query = rewriteQM(query)
	}
	return db.DB.Prepare(query)
}

func rewriteQM(query string) string {
	if !strings.Contains(query, "?") {
		return query
	}
	var buf strings.Builder
	n := 0
	for _, ch := range query {
		if ch == '?' {
			n++
			buf.WriteString(fmt.Sprintf("$%d", n))
		} else {
			buf.WriteRune(ch)
		}
	}
	return buf.String()
}
