package handlers

import (
	"database/sql"
	"inventaris-lab-kom/internal/config"
)

// Handler holds dependencies for all handlers
type Handler struct {
	db  *sql.DB
	cfg *config.Config
}

// NewHandler creates a new handler instance
func NewHandler(db *sql.DB, cfg *config.Config) *Handler {
	return &Handler{
		db:  db,
		cfg: cfg,
	}
}
