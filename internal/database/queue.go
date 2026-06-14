package database

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"inventaris-lab-kom/internal/queue"
)

type trackedResult struct {
	insertID int64
}

func (r trackedResult) LastInsertId() (int64, error) { return r.insertID, nil }
func (r trackedResult) RowsAffected() (int64, error) { return 1, nil }

type fakeUpdateResult struct{}

func (fakeUpdateResult) LastInsertId() (int64, error) { return 0, nil }
func (fakeUpdateResult) RowsAffected() (int64, error) { return 1, nil }

type insertTracker struct {
	mu       sync.Mutex
	counters map[string]int64
}

func newInsertTracker(db *DB) *insertTracker {
	t := &insertTracker{counters: make(map[string]int64)}
	tables := []string{
		"users", "pcs", "device_types", "devices",
		"device_loans", "device_usages", "software_catalog",
		"course_schedules", "logbook_entries",
	}
	for _, table := range tables {
		var maxID sql.NullInt64
		if err := db.reader.QueryRow(fmt.Sprintf("SELECT MAX(id) FROM %s", table)).Scan(&maxID); err == nil {
			if maxID.Valid {
				t.counters[table] = maxID.Int64
			} else {
				// Initialize with 0 for empty tables to ensure tracking works
				t.counters[table] = 0
			}
		}
	}
	return t
}

func (t *insertTracker) nextID(table string) (int64, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	_, ok := t.counters[table]
	if !ok {
		return 0, false
	}
	t.counters[table]++
	return t.counters[table], true
}

var extractTableRE = regexp.MustCompile(`(?i)(?:INSERT\s+(?:OR\s+\w+\s+)?INTO|UPDATE|DELETE\s+FROM)\s+(\w+)`)

func extractTableName(query string) string {
	m := extractTableRE.FindStringSubmatch(query)
	if len(m) < 2 {
		return ""
	}
	return strings.ToLower(m[1])
}

func (db *DB) NewWriteQueue(bufferSize, batchSize int, flushEvery time.Duration) *queue.Queue {
	q := queue.New(bufferSize, batchSize, flushEvery)
	db.queue = q
	tracker := newInsertTracker(db)

	db.execInt = func(query string, args ...any) (sql.Result, error) {
		if strings.Contains(query, "session_token") {
			return db.writer.Exec(query, args...)
		}

		// Bypass async for constraints and critical tables
		tbl := extractTableName(query)
		switch tbl {
		case "devices", "categories", "device_types", "users":
			return db.writer.Exec(query, args...)
		}

		q.Enqueue(queue.Task{
			Label: query,
			Execute: func() error {
				_, err := db.writer.Exec(query, args...)
				return err
			},
		})
		if tbl != "" {
			if id, ok := tracker.nextID(tbl); ok {
				return trackedResult{insertID: id}, nil
			}
			// For UPDATE/DELETE queries, return fake success result
			return fakeUpdateResult{}, nil
		}
		return noopResult{}, nil
	}
	return q
}
