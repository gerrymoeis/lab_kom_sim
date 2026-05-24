package services

import (
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"database/sql"

	"inventaris-lab-kom/internal/config"
	"inventaris-lab-kom/internal/database"
)

type BackupService struct {
	db         *database.DB
	cfg        config.BackupConfig
	mu         sync.Mutex
	running    bool
	lastBackup time.Time
	lastSize   int64
	stop       chan struct{}
	done       chan struct{}
}

func NewBackupService(db *database.DB, cfg config.BackupConfig) *BackupService {
	return &BackupService{
		db:   db,
		cfg:  cfg,
		stop: make(chan struct{}),
		done: make(chan struct{}),
	}
}

func (s *BackupService) Start() {
	if !s.cfg.Enabled {
		log.Println("Backup: disabled via BACKUP_ENABLED=false")
		return
	}
	if s.db.IsPostgres() {
		log.Println("Backup: PostgreSQL detected — auto-backup skipped (use Neon built-in backup)")
		return
	}
	if err := os.MkdirAll(s.cfg.Dir, 0755); err != nil {
		log.Printf("Backup: failed to create directory %s: %v", s.cfg.Dir, err)
		return
	}
	log.Printf("Backup: started — interval=%ds dir=%s retention=%d compress=%v",
		s.cfg.Interval, s.cfg.Dir, s.cfg.Retention, s.cfg.Compress)

	go func() {
		defer close(s.done)
		s.backup()
		ticker := time.NewTicker(time.Duration(s.cfg.Interval) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.backup()
			case <-s.stop:
				return
			}
		}
	}()
}

func (s *BackupService) Stop() {
	if !s.cfg.Enabled || s.db.IsPostgres() {
		return
	}
	close(s.stop)
	<-s.done
	log.Println("Backup: stopped")
}

func (s *BackupService) BackupNow() error {
	if !s.cfg.Enabled {
		return fmt.Errorf("backup is disabled")
	}
	if s.db.IsPostgres() {
		return fmt.Errorf("backup not available for PostgreSQL")
	}
	if !s.mu.TryLock() {
		return fmt.Errorf("backup already running")
	}
	defer s.mu.Unlock()
	return s.runBackup()
}

func (s *BackupService) Stats() (time.Time, int64) {
	return s.lastBackup, s.lastSize
}

func (s *BackupService) backup() {
	if !s.mu.TryLock() {
		log.Println("Backup: previous backup still running, skipping this cycle")
		return
	}
	defer s.mu.Unlock()
	if err := s.runBackup(); err != nil {
		log.Printf("Backup: FAILED — %v", err)
	}
}

func (s *BackupService) runBackup() error {
	if err := os.MkdirAll(s.cfg.Dir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	now := time.Now()
	filename := fmt.Sprintf("backup_%s.db", now.Format("20060102_150405"))
	tmpPath := filepath.Join(s.cfg.Dir, filename+".tmp")
	finalPath := filepath.Join(s.cfg.Dir, filename)

	escaped := strings.ReplaceAll(tmpPath, "'", "''")
	_, err := s.db.RawWriter().Exec("VACUUM INTO '" + escaped + "'")
	if err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("vacuum into: %w", err)
	}

	if ok, msg := verifyIntegrity(tmpPath); !ok {
		os.Remove(tmpPath)
		return fmt.Errorf("integrity check: %s", msg)
	}

	if err := os.Rename(tmpPath, finalPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename: %w", err)
	}

	if s.cfg.Compress {
		if err := gzipFile(finalPath); err != nil {
			log.Printf("Backup: compress warning — %v (keeping uncompressed)", err)
		}
	}

	s.enforceRetention()

	s.lastBackup = now
	fi, err := os.Stat(finalPath)
	if err != nil {
		if fi, err := os.Stat(finalPath + ".gz"); err == nil {
			s.lastSize = fi.Size()
		}
	} else {
		s.lastSize = fi.Size()
	}
	_ = fi

	log.Printf("Backup: SUCCESS — %s (%s)", filename, formatSize(s.lastSize))
	return nil
}

func verifyIntegrity(path string) (bool, string) {
	drivers := []string{"sqlite3", "sqlite"}
	var lastErr error
	for _, d := range drivers {
		conn, err := sql.Open(d, path)
		if err != nil {
			lastErr = err
			continue
		}
		var result string
		if err := conn.QueryRow("PRAGMA integrity_check").Scan(&result); err != nil {
			conn.Close()
			lastErr = err
			continue
		}
		conn.Close()
		if result != "ok" {
			return false, result
		}
		return true, ""
	}
	return false, fmt.Sprintf("no sqlite driver available: %v", lastErr)
}

func gzipFile(path string) error {
	src, err := os.Open(path)
	if err != nil {
		return err
	}
	defer src.Close()

	gzPath := path + ".gz"
	dst, err := os.Create(gzPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	gw := gzip.NewWriter(dst)
	_, err = io.Copy(gw, src)
	gw.Close()
	if err != nil {
		os.Remove(gzPath)
		return fmt.Errorf("compress: %w", err)
	}

	os.Remove(path)
	return nil
}

func (s *BackupService) enforceRetention() {
	pattern := filepath.Join(s.cfg.Dir, "backup_*")
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) <= s.cfg.Retention {
		return
	}

	sort.Slice(matches, func(i, j int) bool {
		fi, errI := os.Stat(matches[i])
		fj, errJ := os.Stat(matches[j])
		if errI != nil || errJ != nil {
			return false
		}
		return fi.ModTime().Before(fj.ModTime())
	})

	for _, f := range matches[:len(matches)-s.cfg.Retention] {
		os.Remove(f)
		log.Printf("Backup: removed old backup — %s", filepath.Base(f))
	}
}

func formatSize(bytes int64) string {
	switch {
	case bytes >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(bytes)/(1<<20))
	case bytes >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(bytes)/(1<<10))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
