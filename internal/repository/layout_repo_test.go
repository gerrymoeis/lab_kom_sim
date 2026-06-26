package repository

import (
	"testing"

	"inventaris-lab-kom/internal/database"
)

func setupLayoutDB(t *testing.T) *database.DB {
	t.Helper()
	db, err := database.InitDB(t.TempDir()+"/layout_test.db", "")
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS grid_layouts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		lab_url_path TEXT NOT NULL UNIQUE,
		cols_per_row TEXT NOT NULL DEFAULT '[8,8,8,8,8]',
		has_gap INTEGER NOT NULL DEFAULT 0,
		gap_pos INTEGER NOT NULL DEFAULT 0,
		row_gaps TEXT DEFAULT '',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		t.Fatalf("create grid_layouts: %v", err)
	}
	return db
}

func TestLayoutRepoUpsert(t *testing.T) {
	db := setupLayoutDB(t)
	repo := NewLayoutRepository(db)

	err := repo.Upsert("test-lab", []int{8, 8, 8}, true, 3, [][]int{{}, {}, {3}})
	if err != nil {
		t.Fatalf("Upsert insert: %v", err)
	}

	var colsJSON string
	var hasGap int
	db.QueryRow("SELECT cols_per_row, has_gap FROM grid_layouts WHERE lab_url_path='test-lab'").Scan(&colsJSON, &hasGap)
	if colsJSON != "[8,8,8]" {
		t.Errorf("expected [8,8,8], got %s", colsJSON)
	}
	if hasGap != 1 {
		t.Errorf("expected has_gap=1, got %d", hasGap)
	}

	err = repo.Upsert("test-lab", []int{10, 10}, false, 0, [][]int{{}, {}})
	if err != nil {
		t.Fatalf("Upsert update: %v", err)
	}
	db.QueryRow("SELECT cols_per_row FROM grid_layouts WHERE lab_url_path='test-lab'").Scan(&colsJSON)
	if colsJSON != "[10,10]" {
		t.Errorf("expected [10,10], got %s", colsJSON)
	}
}

func TestLayoutRepoGetByLab(t *testing.T) {
	db := setupLayoutDB(t)
	repo := NewLayoutRepository(db)

	db.Exec("INSERT INTO grid_layouts (lab_url_path, cols_per_row, has_gap, gap_pos, row_gaps) VALUES ('lab-a', '[6,6,6]', 0, 0, '[]')")

	layout, err := repo.GetByLab("lab-a")
	if err != nil {
		t.Fatalf("GetByLab: %v", err)
	}
	if len(layout.ColsPerRow) != 3 || layout.ColsPerRow[0] != 6 {
		t.Errorf("unexpected cols: %v", layout.ColsPerRow)
	}
	if layout.HasGap {
		t.Error("expected has_gap=false")
	}

	_, err = repo.GetByLab("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent lab")
	}
}

func TestLayoutRepoList(t *testing.T) {
	db := setupLayoutDB(t)
	repo := NewLayoutRepository(db)

	layouts, err := repo.List()
	if err != nil {
		t.Fatalf("List empty: %v", err)
	}
	if len(layouts) != 0 {
		t.Errorf("expected 0 layouts, got %d", len(layouts))
	}

	repo.Upsert("lab-a", []int{8, 8}, false, 0, [][]int{{}, {}})
	repo.Upsert("lab-b", []int{10, 10, 10}, true, 2, [][]int{{}, {}, {2}})

	layouts, err = repo.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(layouts) != 2 {
		t.Errorf("expected 2 layouts, got %d", len(layouts))
	}
}
