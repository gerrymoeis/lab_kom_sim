package database

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

func InitDB(dbPath, dbURL string) (*DB, error) {
	if dbURL != "" {
		log.Println("Using PostgreSQL (Neon DB)")
		db, err := sql.Open("pgx", dbURL)
		if err != nil {
			return nil, fmt.Errorf("failed to open postgres: %w", err)
		}
		db.SetMaxOpenConns(10)
		db.SetMaxIdleConns(5)
		if err := db.Ping(); err != nil {
			return nil, fmt.Errorf("failed to ping postgres: %w", err)
		}
		return wrapPG(db), nil
	}

	log.Println("Using SQLite (local)")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite: %w", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		return nil, fmt.Errorf("failed to set busy timeout: %w", err)
	}
	return wrapSQLite(db), nil
}

func RunMigrations(db *DB, isPostgres bool) error {
	if err := runMigrations(db, isPostgres); err != nil {
		return err
	}
	if err := seedRequiredSoftware(db); err != nil {
		return err
	}
	return seedPCs(db)
}
