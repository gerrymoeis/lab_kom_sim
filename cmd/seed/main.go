package main

import (
	"fmt"
	"log"

	"inventaris-lab-kom/internal/config"
	"inventaris-lab-kom/internal/database"
)

func main() {
	fmt.Println("🌱 Seeding sample data...")

	// Load configuration
	cfg := config.Load()

	// Initialize database
	db, err := database.InitDB(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Run migrations
	if err := database.RunMigrations(db); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Seed default admin user
	if err := database.SeedDefaultUser(db); err != nil {
		log.Printf("Warning: Failed to seed default user: %v", err)
	}

	// Seed sample data
	if err := database.SeedSampleData(db); err != nil {
		log.Fatalf("Failed to seed sample data: %v", err)
	}

	fmt.Println("✅ Seeding completed successfully!")
	fmt.Println("\n📊 Sample data created:")
	fmt.Println("   - 40 PCs (8 columns × 5 rows)")
	fmt.Println("   - 5 devices (printer, router, speaker, etc.)")
	fmt.Println("   - 1 admin user (username: admin, password: admin123)")
	fmt.Println("\n🚀 You can now run the server with: go run cmd/server/main.go")
}
