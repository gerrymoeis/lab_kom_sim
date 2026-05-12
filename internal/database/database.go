package database

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/mattn/go-sqlite3"
)

func InitDB(dbPath, dbURL string) (*sql.DB, error) {
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
		return db, nil
	}

	log.Println("Using SQLite (local)")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite: %w", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}
	return db, nil
}

func RunMigrations(db *sql.DB, isPostgres bool) error {
	if isPostgres {
		return runPostgresMigrations(db)
	}
	return runSQLiteMigrations(db)
}
