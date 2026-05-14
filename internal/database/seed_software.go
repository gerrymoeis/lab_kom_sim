package database

import "fmt"

func seedRequiredSoftware(db *DB) error {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM software_catalog WHERE category = 'required'`).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check existing software seeds: %w", err)
	}
	if count > 0 {
		return nil
	}

	required := []struct {
		Name        string
		Description string
	}{
		{"Visual Studio Code", "Code editor untuk berbagai bahasa pemrograman"},
		{"PHP + Xampp", "Web server lokal dengan PHP dan MySQL"},
		{"Python", "Bahasa pemrograman serbaguna"},
		{"Unity", "Game engine multiplatform"},
		{"Blender", "Software modeling dan animasi 3D"},
		{"Cisco Packet Tracer", "Simulator jaringan Cisco"},
		{"Wireshark", "Network protocol analyzer"},
		{"Postman", "API testing dan development tool"},
		{"Composer", "Dependency manager untuk PHP"},
		{"Android Studio", "IDE untuk pengembangan Android"},
		{"Git", "Version control system"},
		{"Figma", "Design dan prototyping kolaboratif"},
		{"Node.js", "JavaScript runtime environment"},
		{"SQL Server", "Microsoft SQL Server database"},
	}

	for _, sw := range required {
		_, err := db.Exec(`INSERT INTO software_catalog (name, category, description, created_at, updated_at) VALUES (?, 'required', ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`, sw.Name, sw.Description)
		if err != nil {
			return fmt.Errorf("failed to seed software %s: %w", sw.Name, err)
		}
	}

	fmt.Printf("Seeded %d required software to catalog\n", len(required))
	return nil
}
