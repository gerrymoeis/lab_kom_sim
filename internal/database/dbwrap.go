package database

import (
	"database/sql"
	"strconv"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/jmoiron/sqlx/reflectx"
)

type DB struct {
	*sql.DB
	X *sqlx.DB
	rewrite bool
}

func wrapPG(db *sql.DB) *DB {
	xdb := sqlx.NewDb(db, "pgx")
	xdb.Mapper = reflectx.NewMapperFunc("json", strings.ToLower)
	return &DB{DB: db, X: xdb, rewrite: true}
}

func wrapSQLite(db *sql.DB) *DB {
	xdb := sqlx.NewDb(db, "sqlite3")
	xdb.Mapper = reflectx.NewMapperFunc("json", strings.ToLower)
	return &DB{DB: db, X: xdb, rewrite: false}
}

func (db *DB) maybeRewrite(query string) string {
	if db.rewrite { return rewriteQM(query) }
	return query
}

func (db *DB) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return db.DB.Query(db.maybeRewrite(query), args...)
}
func (db *DB) QueryRow(query string, args ...interface{}) *sql.Row {
	return db.DB.QueryRow(db.maybeRewrite(query), args...)
}
func (db *DB) Exec(query string, args ...interface{}) (sql.Result, error) {
	return db.DB.Exec(db.maybeRewrite(query), args...)
}
func (db *DB) Prepare(query string) (*sql.Stmt, error) {
	return db.DB.Prepare(db.maybeRewrite(query))
}

type Tx struct {
	*sql.Tx
	rewrite bool
}

func (db *DB) Begin() (*Tx, error) {
	tx, err := db.DB.Begin()
	if err != nil { return nil, err }
	return &Tx{Tx: tx, rewrite: db.rewrite}, nil
}

func (tx *Tx) maybeRewrite(query string) string {
	if tx.rewrite { return rewriteQM(query) }
	return query
}

func (tx *Tx) Exec(query string, args ...interface{}) (sql.Result, error) {
	return tx.Tx.Exec(tx.maybeRewrite(query), args...)
}
func (tx *Tx) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return tx.Tx.Query(tx.maybeRewrite(query), args...)
}
func (tx *Tx) QueryRow(query string, args ...interface{}) *sql.Row {
	return tx.Tx.QueryRow(tx.maybeRewrite(query), args...)
}
func (tx *Tx) Prepare(query string) (*sql.Stmt, error) {
	return tx.Tx.Prepare(tx.maybeRewrite(query))
}

func rewriteQM(query string) string {
	if !strings.Contains(query, "?") { return query }
	var buf strings.Builder
	n := 0
	for _, ch := range query {
		if ch == '?' { n++; buf.WriteString("$"); buf.WriteString(strconv.Itoa(n)) } else { buf.WriteRune(ch) }
	}
	return buf.String()
}
