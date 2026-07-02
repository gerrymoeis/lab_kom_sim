package tests

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func getPC(t *testing.T, lab *testLab, label string) map[string]any {
	var row struct {
		Label     string `json:"label"`
		Row       int    `json:"row"`
		Column    int    `json:"column"`
		Placement string `json:"placement"`
	}
	err := lab.db.QueryRow("SELECT label, row, column, placement FROM pcs WHERE label = ?", label).
		Scan(&row.Label, &row.Row, &row.Column, &row.Placement)
	if err != nil {
		return nil
	}
	return map[string]any{
		"label":     row.Label,
		"row":       row.Row,
		"column":    row.Column,
		"placement": row.Placement,
	}
}

func getPCByPosition(t *testing.T, lab *testLab, row, col int) map[string]any {
	var label, placement string
	err := lab.db.QueryRow("SELECT label, placement FROM pcs WHERE row=? AND column=?", row, col).
		Scan(&label, &placement)
	if err != nil {
		return nil
	}
	return map[string]any{
		"label":     label,
		"row":       row,
		"column":    col,
		"placement": placement,
	}
}

func countSoftware(t *testing.T, lab *testLab, pcLabel string) int {
	var n int
	lab.db.QueryRow("SELECT COUNT(*) FROM pc_software WHERE pc_id = (SELECT id FROM pcs WHERE label = ?)", pcLabel).Scan(&n)
	return n
}

func requirePCExists(t *testing.T, lab *testLab, label string, expectedRow, expectedCol int, expectedPlacement string) {
	pc := getPC(t, lab, label)
	if pc == nil {
		t.Fatalf("PC %s not found in DB", label)
	}
	if pc["row"].(int) != expectedRow {
		t.Errorf("PC %s: expected row=%d, got %d", label, expectedRow, pc["row"])
	}
	if pc["column"].(int) != expectedCol {
		t.Errorf("PC %s: expected column=%d, got %d", label, expectedCol, pc["column"])
	}
	if pc["placement"].(string) != expectedPlacement {
		t.Errorf("PC %s: expected placement=%s, got %s", label, expectedPlacement, pc["placement"])
	}
}

func postGridOp(t *testing.T, lab *testLab, path, body string) (int, map[string]any) {
	if !lab.refreshCSRF() {
		t.Fatal("failed to refresh CSRF")
	}
	resp, err := lab.postJSON(path, body)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	defer resp.Body.Close()
	var res map[string]any
	json.NewDecoder(resp.Body).Decode(&res)
	return resp.StatusCode, res
}

func moveToCadangan(t *testing.T, lab *testLab, label string) string {
	code, res := postGridOp(t, lab, "/api/pc/move-to-cadangan", fmt.Sprintf(`{"label":"%s"}`, label))
	if code != 200 {
		t.Fatalf("move-to-cadangan %s: expected 200, got %d", label, code)
	}
	changesRaw, _ := res["changes"].([]any)
	if len(changesRaw) == 0 {
		t.Fatal("no changes in move-to-cadangan response")
	}
	return changesRaw[0].(map[string]any)["new_label"].(string)
}

func TestPCGridOperations(t *testing.T) {
	env := setupTestEnvironment(t)
	lab := env.LabA
	db := env.DB_A

	if !loginAndRefresh(lab, "labA_only", "test123") {
		t.Fatal("login failed")
	}

	// ============================================
	// 2D.1: PC Move — verifikasi row/col/label di DB
	// ============================================
	t.Run("2D.1_pc_move", func(t *testing.T) {
		// Free target position (1,8) by moving pc-8 to cadangan
		moveToCadangan(t, lab, "pc-8")

		// Move pc-3 to (1,8) → should become pc-8
		code, res := postGridOp(t, lab, "/api/pc/move", `{"label":"pc-3","row":1,"col":8}`)
		if code != 200 {
			t.Fatalf("expected 200, got %d", code)
		}
		success, _ := res["success"].(bool)
		if !success {
			t.Fatal("move success=false")
		}

		// pc-3 should now be pc-8 with row=1, col=8
		requirePCExists(t, lab, "pc-8", 1, 8, "dipakai")
		// pc-3 should no longer exist
		if getPC(t, lab, "pc-3") != nil {
			t.Error("pc-3 should no longer exist after move")
		}
	})

	// ============================================
	// 2D.2: PC Swap — verifikasi label di posisi tertukar
	// ============================================
	t.Run("2D.2_pc_swap", func(t *testing.T) {
		// SwapLabels swaps labels AND positions simultaneously.
		// Entity A (id) takes B's label AND B's position, Entity B takes A's label AND A's position.
		// Verified: labels stay at the same grid positions, but the underlying entities swap.
		// We verify by checking entity IDs moved.

		// Get entity IDs before swap
		var id5, id6 int
		var row5, col5, row6, col6 int
		lab.db.QueryRow("SELECT id, row, column FROM pcs WHERE label='pc-5'").Scan(&id5, &row5, &col5)
		lab.db.QueryRow("SELECT id, row, column FROM pcs WHERE label='pc-6'").Scan(&id6, &row6, &col6)
		if id5 == 0 || id6 == 0 {
			t.Fatal("pc-5 or pc-6 not found")
		}

		code, res := postGridOp(t, lab, "/api/pc/swap", `{"a":"pc-5","b":"pc-6"}`)
		if code != 200 {
			t.Fatalf("expected 200, got %d", code)
		}
		success, _ := res["success"].(bool)
		if !success {
			t.Fatal("swap success=false")
		}

		// Entity id=5 (old pc-5) should now be at pc-6's old position with label "pc-6"
		var newRow5, newCol5 int
		lab.db.QueryRow("SELECT row, column FROM pcs WHERE id=?", id5).Scan(&newRow5, &newCol5)
		if newRow5 != row6 || newCol5 != col6 {
			t.Errorf("entity id=%d: expected at (%d,%d), got (%d,%d)", id5, row6, col6, newRow5, newCol5)
		}

		// Entity id=6 (old pc-6) should now be at pc-5's old position with label "pc-5"
		var newRow6, newCol6 int
		lab.db.QueryRow("SELECT row, column FROM pcs WHERE id=?", id6).Scan(&newRow6, &newCol6)
		if newRow6 != row5 || newCol6 != col5 {
			t.Errorf("entity id=%d: expected at (%d,%d), got (%d,%d)", id6, row5, col5, newRow6, newCol6)
		}
	})

	// ============================================
	// 2D.3: PC Replace — verifikasi spare→dipakai, old→cadangan, software di-seed
	// ============================================
	t.Run("2D.3_pc_replace", func(t *testing.T) {
		// Move pc-7 to cadangan to get a spare
		cadanganLabel := moveToCadangan(t, lab, "pc-7")
		requirePCExists(t, lab, cadanganLabel, 0, 0, "cadangan")

		// Record pc-1's original position and software count
		pc1Before := getPC(t, lab, "pc-1")
		targetSWBefore := countSoftware(t, lab, "pc-1")
		cadanganSWBefore := countSoftware(t, lab, cadanganLabel)

		// Replace pc-1 with the cadangan PC
		code, res := postGridOp(t, lab, "/api/pc/replace",
			fmt.Sprintf(`{"target":"pc-1","spare":"%s"}`, cadanganLabel))
		if code != 200 {
			t.Fatalf("replace: expected 200, got %d", code)
		}
		success, _ := res["success"].(bool)
		if !success {
			t.Fatal("replace success=false")
		}

		// ReplaceWithSpare: spare takes target's label & position, target becomes cadangan
		// - The former spare (cadanganLabel entity) now has label "pc-1" @ target's old pos
		// - The former pc-1 entity now has cadanganLabel @ (0,0) with placement=cadangan
		requirePCExists(t, lab, "pc-1", pc1Before["row"].(int), pc1Before["column"].(int), "dipakai")
		requirePCExists(t, lab, cadanganLabel, 0, 0, "cadangan")

		// Software: new pc-1 (former spare) should have its original software + required seeded
		targetSWAfter := countSoftware(t, lab, "pc-1")
		if targetSWAfter < cadanganSWBefore {
			t.Errorf("pc-1 software after replace: expected >=%d (former spare), got %d",
				cadanganSWBefore, targetSWAfter)
		}
		// Cadangan (former pc-1) should retain its software
		cadanganSWAfter := countSoftware(t, lab, cadanganLabel)
		if cadanganSWAfter != targetSWBefore {
			t.Errorf("cadangan software after replace: expected %d (former pc-1), got %d",
				targetSWBefore, cadanganSWAfter)
		}
	})

	// ============================================
	// 2D.4: PC Place — verifikasi cadangan→dipakai, label sesuai (row,col)
	// ============================================
	t.Run("2D.4_pc_place", func(t *testing.T) {
		// Free target position (2,1) by moving pc-9 to cadangan
		cadanganLabel := moveToCadangan(t, lab, "pc-9")

		// Place cadangan at (2,1) → pc-9
		code, res := postGridOp(t, lab, "/api/pc/place",
			fmt.Sprintf(`{"label":"%s","row":2,"col":1}`, cadanganLabel))
		if code != 200 {
			t.Fatalf("place: expected 200, got %d", code)
		}
		success, _ := res["success"].(bool)
		if !success {
			t.Fatal("place success=false")
		}

		// Should now be at row=2, col=1 with label=pc-9, placement=dipakai
		requirePCExists(t, lab, "pc-9", 2, 1, "dipakai")

		// Software: place calls SeedMissingRequiredSoftware
		sw := countSoftware(t, lab, "pc-9")
		if sw == 0 {
			t.Error("pc-9 should have required software after place")
		}
	})

	// ============================================
	// 2D.5: MoveToCadangan — verifikasi label jadi pc-cadangan-<N>
	// ============================================
	t.Run("2D.5_move_to_cadangan", func(t *testing.T) {
		cadanganLabel := moveToCadangan(t, lab, "pc-12")

		if !strings.HasPrefix(cadanganLabel, "pc-cadangan-") {
			t.Errorf("expected label to start with pc-cadangan-, got %s", cadanganLabel)
		}
		requirePCExists(t, lab, cadanganLabel, 0, 0, "cadangan")
		if getPC(t, lab, "pc-12") != nil {
			t.Error("pc-12 should no longer exist")
		}
	})

	// ============================================
	// 2D.6: MoveRowToCadangan — semua PC di row jadi cadangan
	// ============================================
	t.Run("2D.6_move_row_to_cadangan", func(t *testing.T) {
		// Create a fresh PC in row 4
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		formData := "row=4&column=1&status=normal&placement=dipakai&is_mahasiswa=true" +
			"&serial_number=SN-ROW-TEST&operating_system=Win11&pc_type=PC" +
			"&brand_model=Dell&accessories=KB&processor=i5&ram=8GB&storage=256GB" +
			"&_csrf=" + lab.csrf
		resp, err := lab.post("/pc/create", formData)
		if err != nil {
			t.Fatalf("POST /pc/create: %v", err)
		}
		resp.Body.Close()

		code, res := postGridOp(t, lab, "/api/pc/move-row", `{"row":4}`)
		if code != 200 {
			t.Fatalf("expected 200, got %d", code)
		}
		success, _ := res["success"].(bool)
		if !success {
			t.Fatal("move-row success=false")
		}

		changesRaw, _ := res["changes"].([]any)
		for _, c := range changesRaw {
			ch := c.(map[string]any)
			newLabel := ch["new_label"].(string)
			pc := getPC(t, lab, newLabel)
			if pc == nil {
				t.Errorf("cadangan PC %s not found", newLabel)
				continue
			}
			if pc["placement"].(string) != "cadangan" {
				t.Errorf("PC %s: expected placement=cadangan, got %s", newLabel, pc["placement"])
			}
			if pc["row"].(int) != 0 || pc["column"].(int) != 0 {
				t.Errorf("PC %s: expected row=0,col=0, got row=%d,col=%d", newLabel, pc["row"], pc["column"])
			}
		}
	})

	// ============================================
	// 2D.7: Software consistency after Move/Swap
	// ============================================
	t.Run("2D.7_software_consistency_move_swap", func(t *testing.T) {
		// Free a position by moving pc-13 to cadangan
		moveToCadangan(t, lab, "pc-13")

		// Move pc-14 to (2,6) → becomes pc-14 stays same label since its position is (2,6)
		// Actually (2,6) = position 14 → pc-14. It's already pc-14 at (2,6).
		// Let me use a different position: move pc-14 to (2,2) which is free now (pc-13 was at (2,5)? No...
		// Let me check: pc-13 was at row 2, col 5 → position 13
		// pc-14 is at row 2, col 6 → position 14
		// I moved pc-13 to cadangan, freeing (2,5).
		// Move pc-14 to (2,5) → becomes pc-13 (position 13)
		swBefore := countSoftware(t, lab, "pc-14")

		code, _ := postGridOp(t, lab, "/api/pc/move", `{"label":"pc-14","row":2,"col":5}`)
		if code != 200 {
			t.Fatalf("move pc-14: expected 200, got %d", code)
		}

		swAfter := countSoftware(t, lab, "pc-13") // pc-13 is new label at (2,5)
		if swAfter != swBefore {
			t.Errorf("software count changed after move: before=%d, after=%d", swBefore, swAfter)
		}

		// Swap pc-1 and pc-5
		swPC1Before := countSoftware(t, lab, "pc-1")
		swPC5Before := countSoftware(t, lab, "pc-5")

		code, _ = postGridOp(t, lab, "/api/pc/swap", `{"a":"pc-1","b":"pc-5"}`)
		if code != 200 {
			t.Fatalf("swap: expected 200, got %d", code)
		}

		// SwapLabels: entities (by id) swap labels AND positions
		// After swap: entity id=5 (was pc-5) now has label "pc-1"
		//             entity id=1 (was pc-1) now has label "pc-5"
		// Software is tied to entity (id), not label
		// So "pc-1" after swap = former pc-5 entity = has pc-5's software
		//    "pc-5" after swap = former pc-1 entity = has pc-1's software
		swPC1After := countSoftware(t, lab, "pc-1")
		swPC5After := countSoftware(t, lab, "pc-5")

		if swPC1After != swPC5Before {
			t.Errorf("pc-1 software after swap: expected %d (old pc-5), got %d", swPC5Before, swPC1After)
		}
		if swPC5After != swPC1Before {
			t.Errorf("pc-5 software after swap: expected %d (old pc-1), got %d", swPC1Before, swPC5After)
		}
	})

	// ============================================
	// 2D.8: Software consistency after Replace/Place
	// ============================================
	t.Run("2D.8_software_consistency_replace_place", func(t *testing.T) {
		pc21Before := getPC(t, lab, "pc-21")
		if pc21Before == nil {
			t.Skip("pc-21 not found")
		}

		// Move pc-21 to cadangan → get spare
		cadanganLabel := moveToCadangan(t, lab, "pc-21")

		targetSWBefore := countSoftware(t, lab, "pc-1")
		cadanganSWBefore := countSoftware(t, lab, cadanganLabel)

		// Replace pc-1 with the cadangan
		code, _ := postGridOp(t, lab, "/api/pc/replace",
			fmt.Sprintf(`{"target":"pc-1","spare":"%s"}`, cadanganLabel))
		if code != 200 {
			t.Fatalf("replace: expected 200, got %d", code)
		}

		// After replace: spare (cadangan) takes pc-1's label & position
		// pc-1 entity becomes cadangan with cadanganLabel
		targetSWAfter := countSoftware(t, lab, "pc-1")   // former spare → had spare's software
		cadanganSWAfter := countSoftware(t, lab, cadanganLabel) // former pc-1 → has pc-1's software

		if targetSWAfter < cadanganSWBefore {
			t.Errorf("pc-1 software after replace: expected >=%d (former spare), got %d",
				cadanganSWBefore, targetSWAfter)
		}
		if cadanganSWAfter != targetSWBefore {
			t.Errorf("cadangan software after replace: expected %d (former pc-1), got %d",
				targetSWBefore, cadanganSWAfter)
		}

		// Place: move pc-22 to cadangan, then place it back
		cadanganLabel2 := moveToCadangan(t, lab, "pc-22")
		// Free target position (3,1) by moving occupant
		moveToCadangan(t, lab, "pc-17")

		code, _ = postGridOp(t, lab, "/api/pc/place",
			fmt.Sprintf(`{"label":"%s","row":3,"col":1}`, cadanganLabel2))
		if code != 200 {
			t.Fatalf("place: expected 200, got %d", code)
		}

		// Place calls SeedMissingRequiredSoftware
		pc17SW := countSoftware(t, lab, "pc-17")
		if pc17SW == 0 {
			t.Error("pc-17 should have required software after place")
		}
	})

	// ============================================
	// 2D.9: Label collision — move ke slot terisi → error
	// ============================================
	t.Run("2D.9_label_collision", func(t *testing.T) {
		pc18 := getPC(t, lab, "pc-18")
		pc19 := getPC(t, lab, "pc-19")
		if pc18 == nil || pc19 == nil {
			t.Skip("pc-18 or pc-19 not available")
		}

		// Try to move pc-18 to pc-19's position (which is occupied)
		body := fmt.Sprintf(`{"label":"pc-18","row":%d,"col":%d}`,
			pc19["row"].(int), pc19["column"].(int))
		code, _ := postGridOp(t, lab, "/api/pc/move", body)
		if code != 500 {
			t.Errorf("expected 500 for label collision, got %d", code)
		}

		// pc-18 should NOT have moved (rollback)
		requirePCExists(t, lab, "pc-18", pc18["row"].(int), pc18["column"].(int), "dipakai")
		// pc-19 should be unchanged
		requirePCExists(t, lab, "pc-19", pc19["row"].(int), pc19["column"].(int), "dipakai")
	})

	// ============================================
	// 2D.10: Batch delete — success + empty IDs
	// ============================================
	t.Run("2D.10_batch_delete", func(t *testing.T) {
		code, res := postGridOp(t, lab, "/pc/batch-delete", `{"ids":["pc-18","pc-19"]}`)
		if code != 200 {
			t.Fatalf("batch-delete: expected 200, got %d", code)
		}
		success, _ := res["success"].(bool)
		if !success {
			t.Fatal("batch-delete success=false")
		}

		if getPC(t, lab, "pc-18") != nil {
			t.Error("pc-18 should be deleted")
		}
		if getPC(t, lab, "pc-19") != nil {
			t.Error("pc-19 should be deleted")
		}

		// Verify pc_software cascade deleted
		var swCount int
		db.QueryRow("SELECT COUNT(*) FROM pc_software WHERE pc_id IN (SELECT id FROM pcs WHERE label IN ('pc-18','pc-19'))").Scan(&swCount)
		if swCount != 0 {
			t.Errorf("expected 0 pc_software for deleted PCs, got %d", swCount)
		}

		// Test empty IDs → 400
		code, _ = postGridOp(t, lab, "/pc/batch-delete", `{"ids":[]}`)
		if code != 400 {
			t.Errorf("expected 400 for empty ids, got %d", code)
		}
	})

	// ============================================
	// 2D.11: Batch delete partial rollback — valid + invalid label → 500 + rollback
	// ============================================
	t.Run("2D.11_batch_delete_rollback", func(t *testing.T) {
		pc20Before := getPC(t, lab, "pc-20")
		if pc20Before == nil {
			t.Skip("pc-20 not available")
		}

		code, _ := postGridOp(t, lab, "/pc/batch-delete", `{"ids":["pc-20","pc-NONEXISTENT"]}`)
		if code != 500 {
			t.Errorf("expected 500 for partial rollback, got %d", code)
		}

		// pc-20 should still exist (transaction rolled back)
		requirePCExists(t, lab, "pc-20", pc20Before["row"].(int), pc20Before["column"].(int), "dipakai")
	})

	// ============================================
	// 2D.12: Swap A == B — same label → 400
	// ============================================
	t.Run("2D.12_swap_same_label", func(t *testing.T) {
		pc1Before := getPC(t, lab, "pc-1")
		if pc1Before == nil {
			t.Skip("pc-1 not available")
		}

		code, _ := postGridOp(t, lab, "/api/pc/swap", `{"a":"pc-1","b":"pc-1"}`)
		if code != 400 {
			t.Errorf("expected 400 for same label swap, got %d", code)
		}

		// pc-1 should be unchanged
		requirePCExists(t, lab, "pc-1", pc1Before["row"].(int), pc1Before["column"].(int), "dipakai")
	})
}
