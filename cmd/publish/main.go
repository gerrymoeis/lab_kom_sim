package main

import (
	"log"
	"os"
	"path/filepath"

	"inventaris-lab-kom/internal/config"
	"inventaris-lab-kom/internal/database"
	"inventaris-lab-kom/internal/services"
	"inventaris-lab-kom/internal/timeutil"
)

func main() {
	cfg := config.Load()
	timeutil.SetTimezone(cfg.Timezone)

	var labs []config.LabConfig
	for _, lab := range cfg.Labs {
		db, err := database.InitDB(lab.DBPath, cfg.DatabaseURL)
		if err != nil {
			log.Fatalf("DB init for lab %s: %v", lab.URLPath, err)
		}
		defer db.Close()

		// Ensure upload directories exist
		for _, sub := range []string{"pc", "device_types", "temp", "logbook", "device_installations"} {
			dir := filepath.Join(cfg.UploadPath, lab.URLPath, sub)
			if err := os.MkdirAll(dir, 0755); err != nil {
				log.Printf("Warning: mkdir %s: %v", dir, err)
			}
		}

		isPostgres := cfg.DatabaseURL != ""
		if err := database.RunMigrations(db, isPostgres, lab.ID, lab.URLPath, cfg.UploadPath, !cfg.MultiLabMode); err != nil {
			log.Fatalf("migrations for lab %s: %v", lab.URLPath, err)
		}
		if err := database.SeedDefaultUser(db); err != nil {
			log.Printf("Warning: seed default user for lab %s: %v", lab.URLPath, err)
		}

		if err := services.RunPublicBuild(db, cfg.PublicBuild, lab.URLPath, lab.Title, cfg.UploadPath); err != nil {
			log.Fatalf("build for lab %s: %v", lab.URLPath, err)
		}
		labs = append(labs, lab)
	}

	if err := services.GenerateLabSelector(labs, cfg.PublicBuild); err != nil {
		log.Fatalf("generate lab selector: %v", err)
	}
}
