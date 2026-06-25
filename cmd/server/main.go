package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"path/filepath"

	"inventaris-lab-kom/internal/config"
	"inventaris-lab-kom/internal/database"
	"inventaris-lab-kom/internal/queue"
	"inventaris-lab-kom/internal/server"
	"inventaris-lab-kom/internal/services"
	"inventaris-lab-kom/internal/timeutil"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Load()

	timeutil.SetTimezone(cfg.Timezone)
	locName := timeutil.Code()
	if locName == "" { locName = timeutil.Location().String() }
	log.Printf("🌍 Timezone: %s (%s)", timeutil.Location(), locName)

	isPostgres := cfg.DatabaseURL != ""
	dbs := make(map[string]*database.DB)
	for _, lab := range cfg.Labs {
		if dir := filepath.Dir(lab.DBPath); dir != "." {
			if err := os.MkdirAll(dir, 0755); err != nil {
				log.Fatalf("Failed to create database directory %s for lab %s: %v", dir, lab.URLPath, err)
			}
		}
		db, err := database.InitDB(lab.DBPath, cfg.DatabaseURL)
		if err != nil {
			log.Fatalf("Failed to initialize database for lab %s: %v", lab.URLPath, err)
		}
		defer db.Close()

		useDefaultFallback := !cfg.MultiLabMode
		if err := database.RunMigrations(db, isPostgres, lab.ID, lab.URLPath, cfg.UploadPath, useDefaultFallback); err != nil {
			log.Fatalf("Failed to run migrations for lab %s: %v", lab.URLPath, err)
		}
		if err := database.SeedDefaultUser(db); err != nil {
			log.Printf("Warning: Failed to seed default user for lab %s: %v", lab.URLPath, err)
		}

		dbs[lab.URLPath] = db
	}

	if len(dbs) == 0 {
		log.Fatal("No databases initialized")
	}

	// Init global database (users, permissions, layouts, configs)
	globalDBDir := filepath.Dir(cfg.GlobalDBPath)
	if globalDBDir != "." {
		if err := os.MkdirAll(globalDBDir, 0755); err != nil {
			log.Fatalf("Failed to create global db directory %s: %v", globalDBDir, err)
		}
	}
	globalDB, err := database.InitDB(cfg.GlobalDBPath, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to initialize global database: %v", err)
	}
	defer globalDB.Close()

	if err := database.SetupGlobalDB(globalDB, cfg.Labs); err != nil {
		log.Fatalf("Failed to setup global database: %v", err)
	}
	config.SetGlobalDB(globalDB)
	log.Printf("🌐 Global database: %s", cfg.GlobalDBPath)

	var (
		wqs           []*queue.Queue
		backupSvcs    []*services.BackupService
		publicBuildSvcs []*services.PublicBuildService
		notifiers     []services.CUDNotifier
	)

	if cfg.WriteMode == "async" {
		for _, lab := range cfg.Labs {
			db := dbs[lab.URLPath]
			wq := db.NewWriteQueue(50000, 200, 200*time.Millisecond)
			wq.Start()
			wqs = append(wqs, wq)
		}
		log.Printf("⚡ Write mode: async — %d queue(s) started", len(wqs))
	} else {
		log.Println("🔒 Write mode: sync (direct writer)")
	}

	for _, lab := range cfg.Labs {
		db := dbs[lab.URLPath]

		labBackupCfg := cfg.Backup
		labBackupDir := filepath.Join("backups", lab.URLPath)
		labBackupCfg.Dir = []string{labBackupDir}

		backupSvc := services.NewBackupService(db, labBackupCfg)
		backupSvcs = append(backupSvcs, backupSvc)
		notifiers = append(notifiers, backupSvc)

		pubSvc := services.NewPublicBuildService(db, cfg.PublicBuild, lab.URLPath, lab.Title, cfg.UploadPath)
		publicBuildSvcs = append(publicBuildSvcs, pubSvc)
		notifiers = append(notifiers, pubSvc)
	}
	notifier := services.NewMultiNotifier(notifiers...)

	if cfg.Environment == "production" { gin.SetMode(gin.ReleaseMode) }

	router, cleanup, _ := server.SetupRouter(dbs, globalDB, cfg, notifier)
	defer cleanup()

	for _, lab := range cfg.Labs {
		for _, sub := range []string{"pc", "device_types", "temp", "logbook", "device_installations"} {
			dir := filepath.Join(lab.UploadDir, sub)
			if err := os.MkdirAll(dir, 0755); err != nil {
				log.Printf("Warning: Failed to create upload subdir for lab %s: %v", lab.URLPath, err)
			}
		}
	}

	go func() {
		for { time.Sleep(30 * time.Minute); server.CleanupTempFiles(cfg) }
	}()

	for _, wq := range wqs {
		defer wq.Stop()
	}
	for _, s := range backupSvcs {
		s.Start()
		defer s.Stop()
	}
	for _, s := range publicBuildSvcs {
		s.Start()
		defer s.Stop()
	}

	addr := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)
	srv := &http.Server{Addr: addr, Handler: router}

	go func() {
		log.Printf("🚀 Server starting on http://%s", addr)
		log.Printf("📊 Environment: %s", cfg.Environment)
		for _, lab := range cfg.Labs {
			log.Printf("💾 Database [%s]: %s", lab.URLPath, lab.DBPath)
		}
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("🛑 Shutting down server gracefully...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	log.Println("✅ Server exited gracefully")
}
