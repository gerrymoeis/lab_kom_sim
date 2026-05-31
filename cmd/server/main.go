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
	db, err := database.InitDB(cfg.DatabasePath, cfg.DatabaseURL)
	if err != nil { log.Fatalf("Failed to initialize database: %v", err) }
	defer db.Close()

	if err := database.RunMigrations(db, isPostgres); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}
	if err := database.SeedDefaultUser(db); err != nil {
		log.Printf("Warning: Failed to seed default user: %v", err)
	}

	var wq *queue.Queue
	if cfg.WriteMode == "async" {
		wq = db.NewWriteQueue(50000, 200, 200*time.Millisecond)
		wq.Start()
		log.Println("⚡ Write mode: async (queue-based batch writer)")
	} else {
		log.Println("🔒 Write mode: sync (direct writer)")
	}
	if wq != nil {
		defer wq.Stop()
	}

	backupSvc := services.NewBackupService(db, cfg.Backup)
	publicBuildSvc := services.NewPublicBuildService(db, cfg.PublicBuild)

	notifier := services.NewMultiNotifier(backupSvc, publicBuildSvc)

	if cfg.Environment == "production" { gin.SetMode(gin.ReleaseMode) }

	router := server.SetupRouter(db, cfg, notifier)

	if err := os.MkdirAll("uploads", 0755); err != nil {
		log.Printf("Warning: Failed to create uploads directory: %v", err)
	}
	if err := os.MkdirAll("uploads/temp", 0755); err != nil {
		log.Printf("Warning: Failed to create temp directory: %v", err)
	}

	go func() {
		for { time.Sleep(30 * time.Minute); server.CleanupTempFiles() }
	}()

	backupSvc.Start()
	defer backupSvc.Stop()
	publicBuildSvc.Start()
	defer publicBuildSvc.Stop()

	addr := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)
	srv := &http.Server{Addr: addr, Handler: router}

	go func() {
		log.Printf("🚀 Server starting on http://%s", addr)
		log.Printf("📊 Environment: %s", cfg.Environment)
		log.Printf("💾 Database: %s", cfg.DatabasePath)
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
