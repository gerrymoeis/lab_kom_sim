package search

import (
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	b := New(nil)
	if b == nil {
		t.Fatal("New() returned nil")
	}
	if len(b.configs) == 0 {
		t.Error("New() should have configs populated")
	}
}

func TestWhere_EmptyTerm(t *testing.T) {
	b := New(nil)
	sql, args := b.Where("pc", "")
	if sql != "" || args != nil {
		t.Errorf("expected empty, got sql=%q args=%v", sql, args)
	}
}

func TestWhere_InvalidEntity(t *testing.T) {
	b := New(nil)
	sql, args := b.Where("nonexistent", "searchterm")
	if sql != "" || args != nil {
		t.Errorf("expected empty, got sql=%q args=%v", sql, args)
	}
}

func TestWhere_AllEntities(t *testing.T) {
	b := New(nil)

	entities := []string{
		"pc", "device", "software", "schedule",
		"device_type", "user", "logbook", "activity_log",
		"device_loan", "device_usage", "device_installation",
	}

	for _, entity := range entities {
		t.Run(entity, func(t *testing.T) {
			sql, args := b.Where(entity, "searchterm")
			if sql == "" {
				t.Errorf("expected non-empty SQL for %q", entity)
			}
			if !strings.HasPrefix(sql, " AND (") || !strings.HasSuffix(sql, ")") {
				t.Errorf("expected SQL wrapped in AND (...), got %q", sql)
			}
			if len(args) == 0 {
				t.Errorf("expected non-empty args for %q", entity)
			}
			for _, arg := range args {
				if arg != "%searchterm%" {
					t.Errorf("expected arg to be '%%searchterm%%', got %v", arg)
				}
			}
			// Verify each column appears in the SQL
			cfg, ok := b.configs[entity]
			if !ok {
				t.Fatalf("no config for %q", entity)
			}
			for _, col := range cfg.Columns {
				if !strings.Contains(sql, col+" LIKE ?") {
					t.Errorf("SQL missing column %q: %s", col, sql)
				}
			}
		})
	}
}
