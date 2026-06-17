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

	var labs []config.LabConfig
	for _, lab := range cfg.Labs {
		db, err := database.InitDB(lab.DBPath, cfg.DatabaseURL)
		if err != nil {
			log.Fatalf("DB init for lab %s: %v", lab.Name, err)
		}
		defer db.Close()

		if err := services.RunPublicBuild(db, cfg.PublicBuild, lab.Name, lab.Title); err != nil {
			log.Fatalf("build for lab %s: %v", lab.Name, err)
		}
		labs = append(labs, lab)
	}

	if err := services.GenerateLabSelector(labs, cfg.PublicBuild); err != nil {
		log.Fatalf("generate lab selector: %v", err)
	}
}
