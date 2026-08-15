// Package verify menyediakan verifikasi read-only terhadap database hasil deploy
// (global + per-lab) dan struktur uploads. Dipakai oleh subcommand `app-simlab -verify`
// (production tools) dan oleh tool E2E — AMAN dijalankan kapan pun karena tidak
// pernah menulis data (mode read-only), tidak mengubah DB maupun filesystem.
package verify

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"inventaris-lab-kom/internal/config"

	_ "modernc.org/sqlite"
)

// LabReport berisi hasil verifikasi satu lab (semua read-only).
type LabReport struct {
	URLPath  string
	Counts   map[string]int64
	Checks   []string
	Uploads  []string
	SeedDone bool
}

// Report adalah hasil verifikasi menyeluruh; Failed=true bila ada pelanggaran.
type Report struct {
	GlobalChecks []string
	GlobalCounts map[string]int64
	Labs         []LabReport
	Failed       bool
	Errors       []string
}

// openReadOnly membuka DB sqlite mode read-only (tidak membuat/mengubah file).
func openReadOnly(path string) (*sql.DB, error) {
	dsn := "file:" + filepath.ToSlash(path) + "?mode=ro&_pragma=foreign_keys(1)"
	return sql.Open("sqlite", dsn)
}

func closeAll(dbs ...*sql.DB) {
	for _, db := range dbs {
		if db != nil {
			_ = db.Close()
		}
	}
}

// integrityCheck menjalankan PRAGMA integrity_check dan wajib 'ok'.
func integrityCheck(db *sql.DB) error {
	var r string
	if err := db.QueryRow("PRAGMA integrity_check").Scan(&r); err != nil {
		return fmt.Errorf("integrity_check: %w", err)
	}
	if r != "ok" {
		return fmt.Errorf("integrity_check gagal: %s", r)
	}
	return nil
}

// countRows mengembalikan jumlah baris tabel; tabel yang belum ada dihitung 0.
func countRows(db *sql.DB, table string) (int64, error) {
	var n int64
	err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count %s: %w", table, err)
	}
	return n, nil
}

// orphanChecks memastikan tidak ada FK mengambang di DB lab.
func orphanChecks(db *sql.DB) error {
	checks := []string{
		"devices WHERE device_type_id NOT IN (SELECT id FROM device_types)",
		"device_loans WHERE device_id NOT IN (SELECT id FROM devices)",
		"device_usages WHERE device_id NOT IN (SELECT id FROM devices)",
		"loan_extensions WHERE loan_id NOT IN (SELECT id FROM device_loans)",
		"device_installations WHERE device_id NOT IN (SELECT id FROM devices)",
		"pc_software WHERE pc_id NOT IN (SELECT id FROM pcs)",
		"pc_software WHERE software_id NOT IN (SELECT id FROM software_catalog)",
	}
	for _, c := range checks {
		var n int64
		if err := db.QueryRow("SELECT COUNT(*) FROM " + c).Scan(&n); err != nil {
			return fmt.Errorf("orphan check %s: %w", c, err)
		}
		if n > 0 {
			return fmt.Errorf("FK mengambang terdeteksi: %s (%d baris)", c, n)
		}
	}
	return nil
}

// verifyActivityMapping memastikan setiap activity_logs.user_id ada di global_users
// (bukti remap mapping saat ETL lengkap).
func verifyActivityMapping(db, gdb *sql.DB) error {
	valid := map[int64]bool{}
	rows, err := gdb.Query("SELECT id FROM global_users")
	if err != nil {
		return fmt.Errorf("baca id global_users: %w", err)
	}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		valid[id] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	var orphans []int64
	rows, err = db.Query("SELECT DISTINCT user_id FROM activity_logs WHERE user_id IS NOT NULL")
	if err != nil {
		return fmt.Errorf("baca activity_logs.user_id: %w", err)
	}
	for rows.Next() {
		var uid int64
		if err := rows.Scan(&uid); err != nil {
			rows.Close()
			return err
		}
		if !valid[uid] {
			orphans = append(orphans, uid)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	if len(orphans) > 0 {
		return fmt.Errorf("activity_logs.user_id mengambang (tidak ada di global_users): %v", orphans)
	}
	return nil
}

// labTables adalah daftar tabel yang diharapkan ada di setiap DB lab hasil deploy.
var labTables = []string{
	"pcs", "categories", "device_types", "devices", "device_loans", "device_usages",
	"loan_extensions", "device_installations", "software_catalog", "pc_software",
	"course_schedules", "logbook_entries", "sticker_templates", "activity_logs",
}

// globalTables adalah daftar tabel global yang diharapkan ada.
var globalTables = []string{"global_users", "lab_permissions", "grid_layouts"}

// Run memverifikasi global DB + semua DB lab + struktur uploads, read-only.
// Mengembalikan report (selalu non-nil) dan error fatal (mis. file DB tidak ada).
func Run(cfg *config.Config) (*Report, error) {
	rep := &Report{GlobalCounts: map[string]int64{}}
	if cfg == nil {
		return rep, fmt.Errorf("config kosong")
	}

	if _, err := os.Stat(cfg.GlobalDBPath); err != nil {
		return rep, fmt.Errorf("global DB tidak ditemukan %s: %w", cfg.GlobalDBPath, err)
	}

	gdb, err := openReadOnly(cfg.GlobalDBPath)
	if err != nil {
		rep.Failed = true
		rep.Errors = append(rep.Errors, fmt.Sprintf("buka global DB: %v", err))
		return rep, err
	}
	defer closeAll(gdb)

	if err := integrityCheck(gdb); err != nil {
		rep.Failed = true
		rep.Errors = append(rep.Errors, "global: "+err.Error())
	} else {
		rep.GlobalChecks = append(rep.GlobalChecks, "global integrity_check ok")
	}

	for _, t := range globalTables {
		n, err := countRows(gdb, t)
		if err != nil {
			rep.Failed = true
			rep.Errors = append(rep.Errors, "global: "+err.Error())
		} else {
			rep.GlobalCounts[t] = n
		}
	}

	// Wajib ada super admin (>=1) untuk akses panel admin global.
	var superAdmin int64
	if err := gdb.QueryRow("SELECT COUNT(*) FROM global_users WHERE is_super_admin = 1").Scan(&superAdmin); err != nil {
		rep.Failed = true
		rep.Errors = append(rep.Errors, "global super admin count: "+err.Error())
	} else {
		rep.GlobalCounts["super_admin"] = superAdmin
		if superAdmin < 1 {
			rep.Failed = true
			rep.Errors = append(rep.Errors, fmt.Sprintf("tidak ada super admin (is_super_admin=1): count=%d", superAdmin))
		} else {
			rep.GlobalChecks = append(rep.GlobalChecks, fmt.Sprintf("global super_admin=%d (>=1) ok", superAdmin))
		}
	}

	// Orphan FK global: lab_permissions harus mengacu ke global_users yang ada.
	var orphanPerms int64
	if err := gdb.QueryRow("SELECT COUNT(*) FROM lab_permissions WHERE user_id NOT IN (SELECT id FROM global_users)").Scan(&orphanPerms); err != nil {
		rep.Failed = true
		rep.Errors = append(rep.Errors, "global orphan lab_permissions: "+err.Error())
	} else if orphanPerms > 0 {
		rep.Failed = true
		rep.Errors = append(rep.Errors, fmt.Sprintf("global lab_permissions mengambang: %d", orphanPerms))
	} else {
		rep.GlobalChecks = append(rep.GlobalChecks, "global lab_permissions orphan=0 ok")
	}

	// Verifikasi per-lab.
	for _, lab := range cfg.Labs {
		lr := LabReport{URLPath: lab.URLPath, Counts: map[string]int64{}}
		ldb, err := openReadOnly(lab.DBPath)
		if err != nil {
			lr.Checks = append(lr.Checks, fmt.Sprintf("FAIL: buka DB: %v", err))
			rep.Failed = true
			rep.Errors = append(rep.Errors, lab.URLPath+": "+err.Error())
			rep.Labs = append(rep.Labs, lr)
			continue
		}
		if err := integrityCheck(ldb); err != nil {
			lr.Checks = append(lr.Checks, "FAIL integrity_check")
			rep.Failed = true
			rep.Errors = append(rep.Errors, lab.URLPath+": integrity "+err.Error())
		} else {
			lr.Checks = append(lr.Checks, "integrity_check ok")
		}

		if err := orphanChecks(ldb); err != nil {
			lr.Checks = append(lr.Checks, "FAIL orphan FK")
			rep.Failed = true
			rep.Errors = append(rep.Errors, lab.URLPath+": orphan "+err.Error())
		} else {
			lr.Checks = append(lr.Checks, "orphan FK=0 ok")
		}

		if err := verifyActivityMapping(ldb, gdb); err != nil {
			lr.Checks = append(lr.Checks, "FAIL activity mapping")
			rep.Failed = true
			rep.Errors = append(rep.Errors, lab.URLPath+": activity "+err.Error())
		} else {
			lr.Checks = append(lr.Checks, "activity_logs remap ok")
		}

		for _, t := range labTables {
			n, err := countRows(ldb, t)
			if err != nil {
				lr.Checks = append(lr.Checks, fmt.Sprintf("count %s: %v", t, err))
				rep.Failed = true
				rep.Errors = append(rep.Errors, lab.URLPath+": "+err.Error())
			} else {
				lr.Counts[t] = n
			}
		}
		_ = ldb.Close()

		// Struktur uploads per-lab.
		for _, sub := range []string{"pc", "device_types", "device_installations", "logbook", "temp"} {
			dir := filepath.Join(lab.UploadDir, sub)
			if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
				lr.Uploads = append(lr.Uploads, fmt.Sprintf("FAIL: folder upload %s tidak ada", sub))
				rep.Failed = true
				rep.Errors = append(rep.Errors, lab.URLPath+": upload subdir "+sub+" tidak ada")
			} else {
				lr.Uploads = append(lr.Uploads, sub+"/ ok")
			}
		}

		// Marker seed per-lab (menandakan seed folder selesai untuk lab source).
		marker := filepath.Join(lab.UploadDir, ".seed_done")
		_, err = os.Stat(marker)
		lr.SeedDone = err == nil
		lr.Checks = append(lr.Checks, fmt.Sprintf("marker .seed_done=%v", lr.SeedDone))
		rep.Labs = append(rep.Labs, lr)
	}

	if len(cfg.Labs) == 0 {
		rep.Failed = true
		rep.Errors = append(rep.Errors, "tidak ada lab terkonfigurasi")
	}
	return rep, nil
}