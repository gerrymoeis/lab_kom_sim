package main

import (
	"log"

	"inventaris-lab-kom/internal/config"
	"inventaris-lab-kom/internal/database"
	"inventaris-lab-kom/internal/services"
	"inventaris-lab-kom/internal/timeutil"
)

func main() {
	cfg := config.Load()
	timeutil.SetTimezone(cfg.Timezone)

	db, err := database.InitDB(cfg.DatabasePath, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("DB init: %v", err)
	}
	defer db.Close()

	if err := database.RunMigrations(db, cfg.DatabaseURL != ""); err != nil {
		log.Fatalf("migrations: %v", err)
	}

	if err := services.RunPublicBuild(db, cfg.PublicBuild); err != nil {
		log.Fatalf("build failed: %v", err)
	}
}
