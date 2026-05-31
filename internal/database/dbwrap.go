package database

import (
	"database/sql"
	"runtime"
	"strconv"
	"strings"
	"time"

	"inventaris-lab-kom/internal/queue"
)

type ExecInterceptor func(query string, args ...any) (sql.Result, error)

type DB struct {
	writer  *sql.DB
	reader  *sql.DB
	rewrite bool
	execInt ExecInterceptor
	queue   *queue.Queue
}

func (db *DB) SetExecInterceptor(int ExecInterceptor) {
	db.execInt = int
}

type noopResult struct{}

func (noopResult) LastInsertId() (int64, error) { return 0, nil }
func (noopResult) RowsAffected() (int64, error) { return 0, nil }

func wrapPG(db *sql.DB) *DB {
	return &DB{writer: db, reader: db, rewrite: true}
}

func wrapSQLite(reader, writer *sql.DB) *DB {
	reader.SetMaxOpenConns(runtime.GOMAXPROCS(0))
	reader.SetMaxIdleConns(4)
	reader.SetConnMaxLifetime(5 * time.Minute)
	reader.SetConnMaxIdleTime(1 * time.Minute)

	writer.SetMaxOpenConns(1)
	writer.SetMaxIdleConns(1)
	writer.SetConnMaxLifetime(5 * time.Minute)
	writer.SetConnMaxIdleTime(1 * time.Minute)

	go func() {
		for range time.NewTicker(30 * time.Second).C {
			if _, err := writer.Exec("PRAGMA wal_checkpoint(PASSIVE)"); err != nil {
				_ = err
			}
		}
	}()

	return &DB{writer: writer, reader: reader, rewrite: false}
}

func (db *DB) IsPostgres() bool {
	return db.rewrite
}

func (db *DB) Close() error {
	if db.writer != db.reader {
		if err := db.reader.Close(); err != nil { return err }
	}
	return db.writer.Close()
}

func (db *DB) Ping() error {
	return db.reader.Ping()
}

// Flush blocks until all pending writes in the async queue complete.
// Safe to call even when write mode is sync (no-op).
func (db *DB) Flush() {
	if db.queue != nil {
		db.queue.Flush()
	}
}

func (db *DB) maybeRewrite(query string) string {
	if db.rewrite { return rewriteQM(query) }
	return query
}

func (db *DB) Query(query string, args ...any) (*sql.Rows, error) {
	return db.reader.Query(db.maybeRewrite(query), args...)
}
func (db *DB) QueryRow(query string, args ...any) *sql.Row {
	return db.reader.QueryRow(db.maybeRewrite(query), args...)
}
func (db *DB) Exec(query string, args ...any) (sql.Result, error) {
	q := db.maybeRewrite(query)
	if db.execInt != nil {
		return db.execInt(q, args...)
	}
	return db.writer.Exec(q, args...)
}
func (db *DB) RawWriter() *sql.DB {
	return db.writer
}

func (db *DB) Prepare(query string) (*sql.Stmt, error) {
	return db.writer.Prepare(db.maybeRewrite(query))
}

type Tx struct {
	*sql.Tx
	rewrite bool
}

func (db *DB) Begin() (*Tx, error) {
	tx, err := db.writer.Begin()
	if err != nil { return nil, err }
	return &Tx{Tx: tx, rewrite: db.rewrite}, nil
}

func (tx *Tx) maybeRewrite(query string) string {
	if tx.rewrite { return rewriteQM(query) }
	return query
}

func (tx *Tx) Exec(query string, args ...any) (sql.Result, error) {
	return tx.Tx.Exec(tx.maybeRewrite(query), args...)
}
func (tx *Tx) Query(query string, args ...any) (*sql.Rows, error) {
	return tx.Tx.Query(tx.maybeRewrite(query), args...)
}
func (tx *Tx) QueryRow(query string, args ...any) *sql.Row {
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
