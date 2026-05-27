package search

import (
	"strings"

	"inventaris-lab-kom/internal/database"
)

type Config struct {
	Alias   string
	Columns []string
}

type Builder struct {
	db      *database.DB
	configs map[string]Config
}

func New(db *database.DB) *Builder {
	return &Builder{
		db:      db,
		configs: defaultConfigs(),
	}
}

func defaultConfigs() map[string]Config {
	return map[string]Config{
		"pc": {
			Alias: "", Columns: []string{
				"CAST(pc_number AS TEXT)", "serial_number", "brand_model",
				"operating_system", "processor", "ram", "storage",
				"pc_type", "accessories", "notes", "label",
			},
		},
		"device": {
			Alias: "d", Columns: []string{
				"d.name", "d.asset_code", "d.serial_number",
				"d.brand", "d.model", "d.location", "d.notes",
			},
		},
		"software": {
			Alias: "sc", Columns: []string{
				"sc.name", "sc.description",
			},
		},
		"schedule": {
			Alias: "", Columns: []string{
				"course_name", "lecturer", "class", "notes",
			},
		},
		"device_type": {
			Alias: "", Columns: []string{
				"name", "category", "brand", "model", "notes_template",
			},
		},
		"user": {
			Alias: "", Columns: []string{
				"username", "full_name",
			},
		},
		"logbook": {
			Alias: "", Columns: []string{
				"student_name", "nim", "purpose",
			},
		},
		"activity_log": {
			Alias: "", Columns: []string{
				"username", "action", "entity_type", "description",
				"ip_address", "user_agent",
			},
		},
		"device_loan": {
			Alias: "l", Columns: []string{
				"l.borrower_name", "d.name", "d.asset_code",
			},
		},
		"device_usage": {
			Alias: "u", Columns: []string{
				"u.user_name", "d.name",
			},
		},
	}
}

func (b *Builder) Where(entity, term string) (string, []any) {
	if term == "" {
		return "", nil
	}

	cfg, ok := b.configs[entity]
	if !ok {
		return "", nil
	}

	parts := make([]string, len(cfg.Columns))
	for i, col := range cfg.Columns {
		parts[i] = col + " LIKE ?"
	}
	s := "%" + term + "%"
	args := make([]any, len(cfg.Columns))
	for i := range args {
		args[i] = s
	}
	return " AND (" + strings.Join(parts, " OR ") + ")", args
}
