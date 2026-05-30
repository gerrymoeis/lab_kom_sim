package database

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/jackc/pgx/v5/stdlib"
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
	dsn := dbPath + "?" + sqliteDSNSuffix()

	reader, err := sql.Open(sqliteDriverName(), dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite reader: %w", err)
	}
	writer, err := sql.Open(sqliteDriverName(), dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite writer: %w", err)
	}

	for _, db := range []*sql.DB{reader, writer} {
		if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
			return nil, fmt.Errorf("failed to set journal_mode: %w", err)
		}
		if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
			return nil, fmt.Errorf("failed to set busy_timeout: %w", err)
		}
		if _, err := db.Exec("PRAGMA temp_store=MEMORY"); err != nil {
			return nil, fmt.Errorf("failed to set temp_store: %w", err)
		}
		if _, err := db.Exec("PRAGMA cache_size=-64000"); err != nil {
			return nil, fmt.Errorf("failed to set cache_size: %w", err)
		}
	}
	return wrapSQLite(reader, writer), nil
}

func RunMigrations(db *DB, isPostgres bool) error {
	if err := runMigrations(db, isPostgres); err != nil {
		return err
	}
	if err := seedRequiredSoftware(db); err != nil {
		return err
	}
	if err := seedPCs(db); err != nil {
		return err
	}
	return seedPCPhotos(db)
}
