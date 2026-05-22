package database

import (
	"database/sql"
	"time"

	"inventaris-lab-kom/internal/queue"
)

func (db *DB) NewWriteQueue(bufferSize, batchSize int, flushEvery time.Duration) *queue.Queue {
	q := queue.New(bufferSize, batchSize, flushEvery)
	db.execInt = func(query string, args ...any) (sql.Result, error) {
		q.Enqueue(queue.Task{
			Label: query,
			Execute: func() error {
				_, err := db.writer.Exec(query, args...)
				return err
			},
		})
		return noopResult{}, nil
	}
	return q
}
