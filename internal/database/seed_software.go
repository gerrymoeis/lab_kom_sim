package database

import "fmt"

func seedRequiredSoftware(db *DB) error {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM software WHERE category = 'required'`).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check existing software seeds: %w", err)
	}
	if count > 0 {
		return nil
	}

	// Get all PC IDs
	rows, err := db.Query(`SELECT id FROM pcs ORDER BY id`)
	if err != nil {
		return fmt.Errorf("failed to query PCs: %w", err)
	}
	defer rows.Close()

	type pcEntry struct {
		id int
	}
	var pcs []int
	for rows.Next() {
		var pc pcEntry
		rows.Scan(&pc.id)
		pcs = append(pcs, pc.id)
	}
	rows.Close()

	if len(pcs) == 0 {
		return nil
	}

	required := []struct {
		Name    string
		Version string
	}{
		{"Visual Studio Code", ""},
		{"PHP + Xampp", ""},
		{"Python", ""},
		{"Unity", ""},
		{"Blender", ""},
		{"Cisco Packet Tracer", ""},
		{"Wireshark", ""},
		{"Postman", ""},
		{"Composer", ""},
		{"Android Studio", ""},
		{"Git", ""},
		{"Figma", ""},
		{"Node.js", ""},
		{"SQL Server", ""},
	}

	for _, pcID := range pcs {
		for _, sw := range required {
			_, err := db.Exec(`INSERT INTO software (pc_id, name, version, category, created_at, updated_at) VALUES (?, ?, ?, 'required', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`, pcID, sw.Name, sw.Version)
			if err != nil {
				return fmt.Errorf("failed to seed software for PC-%d: %w", pcID, err)
			}
		}
	}

	fmt.Printf("Seeded %d required software entries for %d PCs\n", len(required)*len(pcs), len(pcs))
	return nil
}
