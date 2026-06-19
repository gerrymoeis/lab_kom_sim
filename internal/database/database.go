package database

import (
	"database/sql"
	"fmt"
	"log"

	"inventaris-lab-kom/internal/config"

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
		if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
			return nil, fmt.Errorf("failed to set foreign_keys: %w", err)
		}
		if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
			return nil, fmt.Errorf("failed to set busy_timeout: %w", err)
		}
		if _, err := db.Exec("PRAGMA synchronous=NORMAL"); err != nil {
			return nil, fmt.Errorf("failed to set synchronous: %w", err)
		}
		if _, err := db.Exec("PRAGMA temp_store=MEMORY"); err != nil {
			return nil, fmt.Errorf("failed to set temp_store: %w", err)
		}
		if _, err := db.Exec("PRAGMA cache_size=-64000"); err != nil {
			return nil, fmt.Errorf("failed to set cache_size: %w", err)
		}
		var mode string
		db.QueryRow("PRAGMA journal_mode").Scan(&mode)
		log.Printf("SQLite journal_mode: %s", mode)
		if mode != "wal" {
			log.Fatalf("Expected WAL journal_mode, got %s. WAL mode is required for proper concurrent access.", mode)
		}
	}

	// Startup WAL checkpoint: truncate WAL file on service restart
	if _, err := writer.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		log.Printf("Warning: startup WAL checkpoint failed: %v", err)
	}

	return wrapSQLite(reader, writer), nil
}

func RunMigrations(db *DB, isPostgres bool, labID, urlPath, uploadPath string, useDefaultFallback bool) error {
	if err := runMigrations(db, isPostgres); err != nil {
		return err
	}
	// RunSeedFolder first to create PCs, then seedPCPhotos can match photos to existing PCs
	if err := RunSeedFolder(db, labID, urlPath, useDefaultFallback); err != nil {
		return err
	}
	if err := seedPCPhotos(db, uploadPath, urlPath); err != nil {
		return err
	}
	return nil
}

func SetupGlobalDB(db *DB, labs []config.LabConfig) error {
	if err := RunGlobalMigrations(db); err != nil {
		return fmt.Errorf("global migration: %w", err)
	}
	if err := SeedGlobalUsers(db, labs); err != nil {
		return fmt.Errorf("global seed: %w", err)
	}
	return nil
}
