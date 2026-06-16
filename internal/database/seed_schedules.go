package database

import "fmt"

func SeedSchedules(db *DB, labName string) error {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM course_schedules`).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check existing schedule seeds: %w", err)
	}
	if count > 0 {
		return nil
	}

	type scheduleSeed struct {
		Day        string
		TimeStart  string
		TimeEnd    string
		CourseName string
		Class      string
		Lecturer   string
	}

	schedules := []scheduleSeed{
		{"Senin", "07:00", "08:40", "Praktek Pemrograman Web Lanjut", "2024 D", "Pak Kurniawan"},
		{"Senin", "08:40", "10:20", "Praktek Pemrograman Web Lanjut", "2024 E", "Pak Kurniawan"},
		{"Senin", "10:20", "12:00", "Praktek Pemrograman Web Lanjut", "2024 I", "Pak Kurniawan"},
		{"Selasa", "07:00", "08:40", "Praktek Pemrograman Mobile", "2024 I", "Pak Dimas"},
		{"Selasa", "08:40", "10:20", "Praktek Pemrograman Mobile", "2024 E", "Pak Dimas"},
		{"Selasa", "10:20", "12:00", "Praktek Pemrograman Mobile", "2024 D", "Pak Dimas"},
		{"Rabu", "07:00", "08:40", "Praktek Pemrograman Web Lanjut", "2024 B", "Pak Faris"},
		{"Rabu", "08:40", "10:20", "Praktek Pemrograman Web Lanjut", "2024 C", "Pak Faris"},
		{"Rabu", "10:20", "12:00", "Praktek Pemrograman Web Lanjut", "2024 A", "Pak Faris"},
		{"Kamis", "07:00", "08:40", "Praktek Pemrograman API", "2024 C", "Pak Faris"},
		{"Kamis", "08:40", "10:20", "Praktek Pemrograman API", "2024 D", "Pak Faris"},
		{"Kamis", "10:20", "12:00", "Praktek Pemrograman API", "2024 I", "Pak Faris"},
		{"Jumat", "07:00", "08:40", "Praktek Pemrograman Mobile", "2024 A", "Pak I Gde Agung"},
		{"Jumat", "08:40", "10:20", "Praktek Pemrograman Mobile", "2024 B", "Pak I Gde Agung"},
		{"Jumat", "10:20", "12:00", "Praktek Pemrograman Mobile", "2024 C", "Pak I Gde Agung"},
	}

	for _, s := range schedules {
		_, err := db.Exec(`INSERT INTO course_schedules (course_name, lecturer, day, class, time_start, time_end, notes, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
			s.CourseName, s.Lecturer, s.Day, s.Class, s.TimeStart, s.TimeEnd)
		if err != nil {
			return fmt.Errorf("failed to seed schedule (day=%s time=%s-%s course=%s): %w",
				s.Day, s.TimeStart, s.TimeEnd, s.CourseName, err)
		}
	}

	fmt.Printf("✅ Seeded %d course schedules\n", len(schedules))
	return nil
}
