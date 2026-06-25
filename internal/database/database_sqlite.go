package database

import _ "modernc.org/sqlite"

func sqliteDriverName() string { return "sqlite" }

func sqliteDSNSuffix() string {
	return "_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)&loc=UTC"
}
