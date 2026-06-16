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

	for _, lab := range cfg.Labs {
		db, err := database.InitDB(lab.DBPath, cfg.DatabaseURL)
		if err != nil {
			log.Fatalf("DB init for lab %s: %v", lab.Name, err)
		}
		defer db.Close()

		if err := database.RunMigrations(db, cfg.DatabaseURL != "", lab.Name); err != nil {
			log.Fatalf("migrations for lab %s: %v", lab.Name, err)
		}

		if err := services.RunPublicBuild(db, cfg.PublicBuild); err != nil {
			log.Fatalf("build for lab %s: %v", lab.Name, err)
		}
	}
}
