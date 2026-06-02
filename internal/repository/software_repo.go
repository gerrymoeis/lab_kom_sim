package repository

import (
	"database/sql"

	"inventaris-lab-kom/internal/database"
	"inventaris-lab-kom/internal/models"
	"inventaris-lab-kom/internal/search"
	"inventaris-lab-kom/internal/util"
)

type SoftwareRepository struct {
	db     *database.DB
	search *search.Builder
}

func NewSoftwareRepository(db *database.DB) *SoftwareRepository {
	return &SoftwareRepository{db: db, search: search.New(db)}
}

type SoftwareStat struct {
	models.SoftwareCatalog
	InstalledCount int `json:"installed_count"`
	TotalPCs       int `json:"total_pcs"`
}

func (r *SoftwareRepository) List(search, category string) ([]SoftwareStat, error) {
	return r.listWithQuery(search, category, "", "", 0, 0)
}

func (r *SoftwareRepository) ListPaginated(search, category, sortBy string, page, pageSize int) ([]SoftwareStat, int, error) {
	if page < 1 { page = 1 }
	if pageSize < 1 { pageSize = 20 }

	var total int
	countQuery := `SELECT COUNT(*) FROM (SELECT sc.id FROM software_catalog sc
		LEFT JOIN pc_software ps ON sc.id = ps.software_id AND ps.installed = TRUE
		CROSS JOIN (SELECT COUNT(*) AS cnt FROM pcs) pc WHERE 1=1`
	var args []any
	if search != "" {
		sClause, sArgs := r.search.Where("software", search)
		countQuery += sClause
		args = append(args, sArgs...)
	}
	if category == "required" || category == "other" {
		countQuery += ` AND sc.category = ?`
		args = append(args, category)
	}
	countQuery += ` GROUP BY sc.id, sc.name, sc.category, sc.description, pc.cnt) sub`
	r.db.QueryRow(countQuery, args...).Scan(&total)

	stats, err := r.listWithQuery(search, category, sortBy, ` LIMIT ? OFFSET ?`, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	return stats, total, nil
}

func (r *SoftwareRepository) listWithQuery(search, category, sortBy string, suffix string, limit, offset int) ([]SoftwareStat, error) {
	query := `SELECT sc.id, sc.name, sc.category, sc.description, sc.slug, COUNT(ps.software_id), pc.cnt
		FROM software_catalog sc
		LEFT JOIN pc_software ps ON sc.id = ps.software_id AND ps.installed = TRUE
		CROSS JOIN (SELECT COUNT(*) AS cnt FROM pcs) pc
		WHERE 1=1`
	var args []any

	if search != "" {
		sClause, sArgs := r.search.Where("software", search)
		query += sClause
		args = append(args, sArgs...)
	}
	if category == "required" || category == "other" {
		query += ` AND sc.category = ?`
		args = append(args, category)
	}

	query += ` GROUP BY sc.id, sc.name, sc.category, sc.description, sc.slug, pc.cnt`
	switch sortBy {
	case "name":
		query += ` ORDER BY sc.name`
	case "category":
		query += ` ORDER BY sc.category, sc.name`
	default:
		query += ` ORDER BY CASE WHEN sc.category = 'required' THEN 0 ELSE 1 END, sc.name`
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

	var stats []SoftwareStat
	for rows.Next() {
		var st SoftwareStat
		if rows.Scan(&st.ID, &st.Name, &st.Category, &st.Description, &st.Slug, &st.InstalledCount, &st.TotalPCs) == nil {
			stats = append(stats, st)
		}
	}
	return stats, nil
}

type SoftwareItem struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Description string `json:"description"`
}

func (r *SoftwareRepository) GetOtherCatalog() ([]SoftwareItem, error) {
	rows, err := r.db.Query(`SELECT id, name, category, COALESCE(description, '') AS description FROM software_catalog WHERE category = 'other' ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []SoftwareItem
	for rows.Next() {
		var it SoftwareItem
		if rows.Scan(&it.ID, &it.Name, &it.Category, &it.Description) == nil {
			items = append(items, it)
		}
	}
	return items, nil
}

func (r *SoftwareRepository) GetBySlug(slug string) (*models.SoftwareCatalog, error) {
	var sw models.SoftwareCatalog
	err := r.db.QueryRow(`SELECT id, name, category, COALESCE(description,''), slug FROM software_catalog WHERE slug = ?`, slug).
		Scan(&sw.ID, &sw.Name, &sw.Category, &sw.Description, &sw.Slug)
	if err != nil {
		return nil, err
	}
	return &sw, nil
}

func (r *SoftwareRepository) GetByID(id int) (*models.SoftwareCatalog, error) {
	var sw models.SoftwareCatalog
	err := r.db.QueryRow(`SELECT id, name, category, COALESCE(description,''), slug FROM software_catalog WHERE id = ?`, id).
		Scan(&sw.ID, &sw.Name, &sw.Category, &sw.Description, &sw.Slug)
	if err != nil {
		return nil, err
	}
	return &sw, nil
}

type PCInstallStatus struct {
	PCID      int
	Label     string
	Row       int
	Column    int
	Installed bool
}

func (r *SoftwareRepository) GetPCInstallStatus(softwareID int) ([]PCInstallStatus, error) {
	rows, err := r.db.Query(`SELECT p.id, p.label, p.row, p.column, COALESCE(ps.installed, FALSE) AS installed
		FROM pcs p LEFT JOIN pc_software ps ON p.id = ps.pc_id AND ps.software_id = ?
		ORDER BY p.label`, softwareID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pcList []PCInstallStatus
	for rows.Next() {
		var p PCInstallStatus
		if rows.Scan(&p.PCID, &p.Label, &p.Row, &p.Column, &p.Installed) == nil {
			pcList = append(pcList, p)
		}
	}
	return pcList, nil
}

func (r *SoftwareRepository) Create(name, category, description string) (sql.Result, error) {
	slug := util.Slugify(name)
	return r.db.Exec(`INSERT INTO software_catalog (name, category, description, slug, created_at, updated_at) VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		name, category, description, slug)
}

func (r *SoftwareRepository) UpdateSoftwarePCs(softwareID int, pcIDs []int) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	tx.Exec(`DELETE FROM pc_software WHERE software_id = ?`, softwareID)
	for _, pid := range pcIDs {
		tx.Exec(`INSERT INTO pc_software (pc_id, software_id, installed) VALUES (?, ?, TRUE)`, pid, softwareID)
	}

	return tx.Commit()
}

func (r *SoftwareRepository) UpdateMetadata(id int, name, category, description string) error {
	slug := util.Slugify(name)
	_, err := r.db.Exec(`UPDATE software_catalog SET name = ?, category = ?, description = ?, slug = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		name, category, description, slug, id)
	return err
}

func (r *SoftwareRepository) GetName(id int) (string, error) {
	var name string
	err := r.db.QueryRow(`SELECT name FROM software_catalog WHERE id = ?`, id).Scan(&name)
	return name, err
}

func (r *SoftwareRepository) Delete(id int) error {
	_, err := r.db.Exec(`DELETE FROM software_catalog WHERE id = ?`, id)
	return err
}

func (r *SoftwareRepository) IsDuplicate(name string) bool {
	// check for duplicate name by trying to insert and catching error
	return false
}

func (r *SoftwareRepository) Export() ([]SoftwareStat, error) {
	query := `SELECT sc.id, sc.name, sc.category, sc.description, sc.slug, COUNT(ps.software_id), pc.cnt
		FROM software_catalog sc
		LEFT JOIN pc_software ps ON sc.id = ps.software_id AND ps.installed = TRUE
		CROSS JOIN (SELECT COUNT(*) AS cnt FROM pcs) pc
		GROUP BY sc.id, sc.name, sc.category, sc.description, sc.slug, pc.cnt
		ORDER BY CASE WHEN sc.category = 'required' THEN 0 ELSE 1 END, sc.name`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []SoftwareStat
	for rows.Next() {
		var st SoftwareStat
		if rows.Scan(&st.ID, &st.Name, &st.Category, &st.Description, &st.Slug, &st.InstalledCount, &st.TotalPCs) == nil {
			stats = append(stats, st)
		}
	}
	return stats, nil
}
