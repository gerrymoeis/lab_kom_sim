package database

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"inventaris-lab-kom/internal/config"
	"inventaris-lab-kom/internal/util"
)

type jsonPCSpec struct {
	Number     int      `json:"number"`
	SN         string   `json:"sn,omitempty"`
	OS         string   `json:"os,omitempty"`
	RequiredSW []string `json:"requiredSW,omitempty"`
	OtherSW    []string `json:"otherSW,omitempty"`
	Status     string   `json:"status,omitempty"`
	Notes      string   `json:"notes,omitempty"`
}

type jsonRequiredSW struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type jsonSchedule struct {
	Day        string `json:"day"`
	TimeStart  string `json:"timeStart"`
	TimeEnd    string `json:"timeEnd"`
	CourseName string `json:"courseName"`
	Class      string `json:"class"`
	Lecturer   string `json:"lecturer"`
}

func RunSeedFolder(db *DB, labID string, urlPath string, useDefaultFallback bool) error {
	folder := filepath.Join("seeds", strings.ToLower(labID))
	if _, err := os.Stat(folder); os.IsNotExist(err) {
		if !useDefaultFallback {
			return nil
		}
		defaultFolder := filepath.Join("seeds", "default")
		if _, err := os.Stat(defaultFolder); os.IsNotExist(err) {
			return nil
		}
		folder = defaultFolder
	}
	if err := seedRequiredSWFromJSON(db, folder); err != nil {
		return fmt.Errorf("seed required_software for %s: %w", labID, err)
	}
	if err := seedPCsFromJSON(db, folder, urlPath); err != nil {
		return fmt.Errorf("seed pcs for %s: %w", labID, err)
	}
	if err := seedSchedulesFromJSON(db, folder); err != nil {
		return fmt.Errorf("seed schedules for %s: %w", labID, err)
	}
	return nil
}

func readJSONFile(folder, name string, v any) (bool, error) {
	path := filepath.Join(folder, name)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if err := json.Unmarshal(data, v); err != nil {
		return false, fmt.Errorf("parse %s: %w", name, err)
	}
	return true, nil
}

func seedRequiredSWFromJSON(db *DB, folder string) error {
	var entries []jsonRequiredSW
	ok, err := readJSONFile(folder, "required_software.json", &entries)
	if err != nil || !ok {
		return err
	}

	for _, sw := range entries {
		slug := util.Slugify(sw.Name)
		var exists int
		db.QueryRow(`SELECT COUNT(*) FROM software_catalog WHERE slug = ?`, slug).Scan(&exists)
		if exists > 0 {
			continue
		}
		_, err := db.Exec(`INSERT INTO software_catalog (name, category, description, slug, created_at, updated_at) VALUES (?, 'required', ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
			sw.Name, sw.Description, slug)
		if err != nil {
			return fmt.Errorf("failed to seed software %s: %w", sw.Name, err)
		}
	}

	return nil
}

func seedPCsFromJSON(db *DB, folder string, urlPath string) error {
	var pcs []jsonPCSpec
	ok, err := readJSONFile(folder, "pcs.json", &pcs)
	if err != nil || !ok {
		return err
	}

	layout := config.GetGridLayout(urlPath)
	colsPerRow := layout.ColsPerRow

	const (
		defPCType      = "PC All-in-one"
		defBrandModel  = "Axioo Mypc One Pro K7-24 (16N9)"
		defProcessor   = "Intel Core i7"
		defRAM         = "16GB DDR4"
		defStorage     = "1TB NVMe"
		defAccessories = "Keyboard & Mouse Axioo (Wired Set)"
		defStatus      = "normal"
		defPlacement   = "dipakai"
	)

	rowFor, colFor := gridPositionFunc(colsPerRow)
	labelFor := func(n int) string {
		switch n {
		case 41:
			return "pc-dosen"
		case 42:
			return "pc-laboran"
		case 43:
			return "pc-cctv"
		default:
			return fmt.Sprintf("pc-%d", n)
		}
	}

	existing := map[string]bool{}
	if rows, err := db.Query(`SELECT label FROM pcs`); err == nil {
		defer rows.Close()
		for rows.Next() {
			var lbl string
			if rows.Scan(&lbl) == nil {
				existing[lbl] = true
			}
		}
	}

	missing := map[string]bool{}
	for _, pc := range pcs {
		lbl := labelFor(pc.Number)
		if !existing[lbl] {
			missing[lbl] = true
		}
	}
	if len(missing) == 0 {
		return nil
	}

	swByName := map[string]int{}
	catalogRows, _ := db.Query(`SELECT id, name FROM software_catalog`)
	if catalogRows != nil {
		defer catalogRows.Close()
		for catalogRows.Next() {
			var id int
			var name string
			catalogRows.Scan(&id, &name)
			swByName[name] = id
		}
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin tx: %w", err)
	}
	defer tx.Rollback()

	insertSW := func(tx *Tx, pcID int, names []string, skipMissing bool) {
		for _, name := range names {
			swID, ok := swByName[name]
			if !ok {
				if skipMissing {
					continue
				}
				slug := util.Slugify(name)
				pgErr := tx.QueryRow(`INSERT INTO software_catalog (name, category, description, slug) VALUES (?, 'other', '', ?) RETURNING id`, name, slug).Scan(&swID)
				if pgErr != nil {
					tx.Exec(`INSERT INTO software_catalog (name, category, description, slug) VALUES (?, 'other', '', ?)`, name, slug)
					tx.QueryRow(`SELECT id FROM software_catalog WHERE name = ?`, name).Scan(&swID)
				}
				if swID > 0 {
					swByName[name] = swID
				}
			}
			if swID > 0 {
				var exists int
				tx.QueryRow(`SELECT COUNT(*) FROM pc_software WHERE pc_id = ? AND software_id = ?`, pcID, swID).Scan(&exists)
				if exists == 0 {
					tx.Exec(`INSERT INTO pc_software (pc_id, software_id, installed) VALUES (?, ?, TRUE)`, pcID, swID)
				}
			}
		}
	}

	for _, pc := range pcs {
		pcStatus := pc.Status
		if pcStatus == "" {
			pcStatus = defStatus
		}

		label := labelFor(pc.Number)
		if !missing[label] {
			continue
		}
		pcType := defPCType
		brandModel := defBrandModel
		if pc.Number >= 41 {
			pcType = label
			brandModel = ""
		}
		placement := defPlacement
		if pcStatus == "broken" && pc.SN == "" {
			placement = "cadangan"
		}
		_, execErr := tx.Exec(`INSERT INTO pcs ("row", "column", status, processor, ram, storage,
			serial_number, operating_system, pc_type, brand_model, accessories,
			notes, label, placement, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
			CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
			rowFor(pc.Number), colFor(pc.Number),
			pcStatus, defProcessor, defRAM, defStorage,
			pc.SN, pc.OS, pcType, brandModel, defAccessories, pc.Notes, label, placement)
		if execErr != nil {
			tx.Rollback()
			return fmt.Errorf("failed to seed PC-%d: %w", pc.Number, execErr)
		}

		var pcID int
		tx.QueryRow(`SELECT id FROM pcs WHERE label = ?`, label).Scan(&pcID)
		if pcID == 0 {
			continue
		}

		insertSW(tx, pcID, pc.RequiredSW, true)
		insertSW(tx, pcID, pc.OtherSW, false)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}

	fmt.Printf("Seeded %d PCs with software data\n", len(pcs))
	return nil
}

func seedSchedulesFromJSON(db *DB, folder string) error {
	var entries []jsonSchedule
	ok, err := readJSONFile(folder, "course_schedules.json", &entries)
	if err != nil || !ok {
		return err
	}

	for _, s := range entries {
		var exists int
		db.QueryRow(`SELECT COUNT(*) FROM course_schedules WHERE course_name = ? AND day = ? AND class = ? AND time_start = ? AND time_end = ?`,
			s.CourseName, s.Day, s.Class, s.TimeStart, s.TimeEnd).Scan(&exists)
		if exists > 0 {
			continue
		}
		_, err := db.Exec(`INSERT INTO course_schedules (course_name, lecturer, day, class, time_start, time_end, notes, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
			s.CourseName, s.Lecturer, s.Day, s.Class, s.TimeStart, s.TimeEnd)
		if err != nil {
			return fmt.Errorf("failed to seed schedule (day=%s time=%s-%s course=%s): %w",
				s.Day, s.TimeStart, s.TimeEnd, s.CourseName, err)
		}
	}

	return nil
}

func gridPositionFunc(colsPerRow []int) (func(int) int, func(int) int) {
	rowStarts := []int{1}
	for _, cols := range colsPerRow {
		rowStarts = append(rowStarts, rowStarts[len(rowStarts)-1]+cols)
	}
	totalGrid := rowStarts[len(rowStarts)-1] - 1

	rowFor := func(n int) int {
		if n > totalGrid || n < 1 {
			return 0
		}
		for r := 0; r < len(colsPerRow); r++ {
			if n >= rowStarts[r] && n < rowStarts[r+1] {
				return r + 1
			}
		}
		return 0
	}
	colFor := func(n int) int {
		if n > totalGrid || n < 1 {
			return n - totalGrid
		}
		for r := 0; r < len(colsPerRow); r++ {
			if n >= rowStarts[r] && n < rowStarts[r+1] {
				return n - rowStarts[r] + 1
			}
		}
		return n - totalGrid
	}
	return rowFor, colFor
}


