package database

import "os"

// SeedDefaultUser is deprecated — per-lab users table no longer exists.
// Default admin user is seeded via global migration/seed in global database.
func SeedDefaultUser(db *DB) error {
	return nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
