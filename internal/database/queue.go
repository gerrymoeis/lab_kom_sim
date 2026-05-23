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

type insertTracker struct {
	mu       sync.Mutex
	counters map[string]int64
}

func newInsertTracker(db *DB) *insertTracker {
	t := &insertTracker{counters: make(map[string]int64)}
	tables := []string{
		"users", "pcs", "device_types", "devices",
		"device_loans", "device_usages", "software_catalog",
		"course_schedules", "logbook_entries", "lost_items",
	}
	for _, table := range tables {
		var maxID sql.NullInt64
		if err := db.reader.QueryRow(fmt.Sprintf("SELECT MAX(id) FROM %s", table)).Scan(&maxID); err == nil && maxID.Valid {
			t.counters[table] = maxID.Int64
		}
	}
	return t
}

func (t *insertTracker) nextID(table string) int64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.counters[table]++
	return t.counters[table]
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
	tracker := newInsertTracker(db)

	db.execInt = func(query string, args ...any) (sql.Result, error) {
		if strings.Contains(query, "session_token") {
			return db.writer.Exec(query, args...)
		}

		q.Enqueue(queue.Task{
			Label: query,
			Execute: func() error {
				_, err := db.writer.Exec(query, args...)
				return err
			},
		})
		if tbl := extractTableName(query); tbl != "" {
			if _, ok := tracker.counters[tbl]; ok {
				return trackedResult{insertID: tracker.nextID(tbl)}, nil
			}
		}
		return noopResult{}, nil
	}
	return q
}
