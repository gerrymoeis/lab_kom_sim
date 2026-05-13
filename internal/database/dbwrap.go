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

type Tx struct {
	*sql.Tx
	rewrite bool
}

func (db *DB) Begin() (*Tx, error) {
	tx, err := db.DB.Begin()
	if err != nil {
		return nil, err
	}
	return &Tx{Tx: tx, rewrite: db.rewrite}, nil
}

func (tx *Tx) Exec(query string, args ...interface{}) (sql.Result, error) {
	if tx.rewrite {
		query = rewriteQM(query)
	}
	return tx.Tx.Exec(query, args...)
}

func (tx *Tx) Query(query string, args ...interface{}) (*sql.Rows, error) {
	if tx.rewrite {
		query = rewriteQM(query)
	}
	return tx.Tx.Query(query, args...)
}

func (tx *Tx) QueryRow(query string, args ...interface{}) *sql.Row {
	if tx.rewrite {
		query = rewriteQM(query)
	}
	return tx.Tx.QueryRow(query, args...)
}

func (tx *Tx) Prepare(query string) (*sql.Stmt, error) {
	if tx.rewrite {
		query = rewriteQM(query)
	}
	return tx.Tx.Prepare(query)
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
