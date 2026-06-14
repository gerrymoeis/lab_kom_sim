package repository

import (
	"database/sql"

	"inventaris-lab-kom/internal/database"
	"inventaris-lab-kom/internal/models"
	"inventaris-lab-kom/internal/search"
)

type ScheduleRepository struct {
	db     DBTX
	search *search.Builder
}

func NewScheduleRepository(db *database.DB) *ScheduleRepository {
	return &ScheduleRepository{db: db, search: search.New(db)}
}

var dayOrder = `CASE day WHEN 'Senin' THEN 1 WHEN 'Selasa' THEN 2 WHEN 'Rabu' THEN 3 WHEN 'Kamis' THEN 4 WHEN 'Jumat' THEN 5 WHEN 'Sabtu' THEN 6 ELSE 7 END`

func (r *ScheduleRepository) List(search, dayFilter string) ([]models.CourseSchedule, error) {
	return r.listWithQuery(search, dayFilter, "", "", 0, 0)
}

func (r *ScheduleRepository) ListPaginated(search, dayFilter, sortBy string, page, pageSize int) ([]models.CourseSchedule, int, error) {
	if page < 1 { page = 1 }
	if pageSize < 1 { pageSize = 20 }

	var total int
	query := `SELECT COUNT(*) FROM course_schedules WHERE 1=1`
	var args []any
	if search != "" {
		sClause, sArgs := r.search.Where("schedule", search)
		query += sClause
		args = append(args, sArgs...)
	}
	if dayFilter != "" {
		query += ` AND day = ?`
		args = append(args, dayFilter)
	}
	r.db.QueryRow(query, args...).Scan(&total)

	schedules, err := r.listWithQuery(search, dayFilter, sortBy, ` LIMIT ? OFFSET ?`, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	return schedules, total, nil
}

func (r *ScheduleRepository) listWithQuery(search, dayFilter, sortBy string, suffix string, limit, offset int) ([]models.CourseSchedule, error) {
	query := `SELECT id, course_name, lecturer, day, class, time_start, time_end, notes FROM course_schedules WHERE 1=1`
	var args []any
	if search != "" {
		sClause, sArgs := r.search.Where("schedule", search)
		query += sClause
		args = append(args, sArgs...)
	}
	if dayFilter != "" {
		query += ` AND day = ?`
		args = append(args, dayFilter)
	}
	switch sortBy {
	case "course_name":
		query += ` ORDER BY course_name`
	case "time_start":
		query += ` ORDER BY time_start`
	default:
		query += ` ORDER BY ` + dayOrder + `, course_name, time_start`
	}
	query += suffix
	if suffix != "" {
		args = append(args, limit, offset)
	}

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var schedules []models.CourseSchedule
	for rows.Next() {
		var s models.CourseSchedule
		var notes sql.NullString
		if err := rows.Scan(&s.ID, &s.CourseName, &s.Lecturer, &s.Day, &s.Class, &s.TimeStart, &s.TimeEnd, &notes); err != nil {
			return nil, err
		}
		s.Notes = valStr(notes)
		schedules = append(schedules, s)
	}
	return schedules, nil
}

func (r *ScheduleRepository) GetByID(id int) (*models.CourseSchedule, error) {
	var s models.CourseSchedule
	var notes sql.NullString
	err := r.db.QueryRow(`SELECT id, course_name, lecturer, day, class, time_start, time_end, notes FROM course_schedules WHERE id = ?`, id).
		Scan(&s.ID, &s.CourseName, &s.Lecturer, &s.Day, &s.Class, &s.TimeStart, &s.TimeEnd, &notes)
	if err != nil {
		return nil, err
	}
	s.Notes = valStr(notes)
	return &s, nil
}

func (r *ScheduleRepository) GetCourseName(id int) (string, error) {
	var name string
	err := r.db.QueryRow(`SELECT course_name FROM course_schedules WHERE id = ?`, id).Scan(&name)
	return name, err
}

func (r *ScheduleRepository) Create(courseName, lecturer, day, class, timeStart, timeEnd, notes string) (sql.Result, error) {
	return r.db.Exec(`INSERT INTO course_schedules (course_name, lecturer, day, class, time_start, time_end, notes) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		courseName, lecturer, day, class, timeStart, timeEnd, notes)
}

func (r *ScheduleRepository) Update(id int, courseName, lecturer, day, class, timeStart, timeEnd, notes string) error {
	_, err := r.db.Exec(`UPDATE course_schedules SET course_name=?, lecturer=?, day=?, class=?, time_start=?, time_end=?, notes=? WHERE id=?`,
		courseName, lecturer, day, class, timeStart, timeEnd, notes, id)
	return err
}

func (r *ScheduleRepository) Delete(id int) error {
	_, err := r.db.Exec(`DELETE FROM course_schedules WHERE id = ?`, id)
	return err
}
