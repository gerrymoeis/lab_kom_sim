package repository

import (
	"database/sql"

	"inventaris-lab-kom/internal/database"
	"inventaris-lab-kom/internal/models"
)

type StickerTemplateRepository struct {
	db DBTX
}

func NewStickerTemplateRepository(db *database.DB) *StickerTemplateRepository {
	return &StickerTemplateRepository{db: db}
}

var stickerTemplateCols = []string{"id", "name", "sticker_type", "font_size_cm", "padding_h_cm", "padding_v_cm", "created_at", "updated_at"}

func (r *StickerTemplateRepository) ListByType(stickerType string) ([]models.StickerTemplate, error) {
	return getAll[models.StickerTemplate](r.db, "sticker_templates", stickerTemplateCols, "sticker_type = ? ORDER BY name", stickerType)
}

func (r *StickerTemplateRepository) GetByID(id int) (*models.StickerTemplate, error) {
	return getOne[models.StickerTemplate](r.db, "sticker_templates", stickerTemplateCols, "id = ?", id)
}

func (r *StickerTemplateRepository) Create(name, stickerType string, fontSize, padH, padV float64) (sql.Result, error) {
	return r.db.Exec("INSERT INTO sticker_templates (name, sticker_type, font_size_cm, padding_h_cm, padding_v_cm) VALUES (?, ?, ?, ?, ?)",
		name, stickerType, fontSize, padH, padV)
}

func (r *StickerTemplateRepository) Update(id int, name string, fontSize, padH, padV float64) error {
	_, err := r.db.Exec("UPDATE sticker_templates SET name=?, font_size_cm=?, padding_h_cm=?, padding_v_cm=?, updated_at=CURRENT_TIMESTAMP WHERE id=?",
		name, fontSize, padH, padV, id)
	return err
}

func (r *StickerTemplateRepository) Delete(id int) error {
	_, err := r.db.Exec("DELETE FROM sticker_templates WHERE id=?", id)
	return err
}
