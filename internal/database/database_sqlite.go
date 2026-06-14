//go:build mattn

package database

import _ "github.com/mattn/go-sqlite3"

func sqliteDriverName() string { return "sqlite3" }

func sqliteDSNSuffix() string {
	return "_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL&_foreign_keys=ON&_loc=UTC"
}
