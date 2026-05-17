package repository

import "database/sql"

type DBTX interface {
	Query(string, ...any) (*sql.Rows, error)
	QueryRow(string, ...any) *sql.Row
	Exec(string, ...any) (sql.Result, error)
	Prepare(string) (*sql.Stmt, error)
}
