package queue

import (
	"context"
	"log"
	"sync"
	"time"
)

type Task struct {
	Label   string
	Execute func() error
}

type Queue struct {
	tasks      chan Task
	flushReq   chan chan struct{}
	batchSize  int
	flushEvery time.Duration
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

func New(bufferSize, batchSize int, flushEvery time.Duration) *Queue {
	ctx, cancel := context.WithCancel(context.Background())
	return &Queue{
		tasks:      make(chan Task, bufferSize),
		flushReq:   make(chan chan struct{}),
		batchSize:  batchSize,
		flushEvery: flushEvery,
		ctx:        ctx,
		cancel:     cancel,
	}
}

func (q *Queue) Enqueue(t Task) {
	select {
	case q.tasks <- t:
	case <-q.ctx.Done():
	}
}

// Flush blocks until all pending tasks have been executed.
func (q *Queue) Flush() {
	done := make(chan struct{})
	select {
	case q.flushReq <- done:
		<-done
	case <-q.ctx.Done():
	}
}

func (q *Queue) Start() {
	q.wg.Add(1)
	go q.run()
}

func (q *Queue) run() {
	defer q.wg.Done()
	var batch []Task
	ticker := time.NewTicker(q.flushEvery)
	defer ticker.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}
		for _, t := range batch {
			if err := t.Execute(); err != nil {
				log.Printf("write queue: task %q failed: %v", t.Label, err)
			}
		}
		batch = batch[:0]
	}

	for {
		select {
		case <-q.ctx.Done():
			flush()
			return
		case t := <-q.tasks:
			batch = append(batch, t)
			if len(batch) >= q.batchSize {
				flush()
			}
		case done := <-q.flushReq:
			flush()
			close(done)
		case <-ticker.C:
			flush()
		}
	}
}

func (q *Queue) Stop() {
	q.cancel()
	q.wg.Wait()
}
