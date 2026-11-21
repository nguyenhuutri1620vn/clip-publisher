// Package jobs runs background work one job at a time, with cancel and queue positions.
package jobs

import (
	"context"
	"sync"
)

type item struct {
	key    string
	fn     func(ctx context.Context) error
	cancel context.CancelFunc
	ctx    context.Context
}

// Queue runs jobs serially in FIFO order.
type Queue struct {
	mu      sync.Mutex
	running *item
	waiting []*item
	idle    *sync.Cond
}

// NewQueue creates an empty queue.
func NewQueue() *Queue {
	q := &Queue{}
	q.idle = sync.NewCond(&q.mu)
	return q
}

// Enqueue adds a job; returns 0 if it starts now, else its 1-based waiting position.
func (q *Queue) Enqueue(key string, fn func(ctx context.Context) error) int {
	ctx, cancel := context.WithCancel(context.Background())
	it := &item{key: key, fn: fn, ctx: ctx, cancel: cancel}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.running == nil {
		q.start(it)
		return 0
	}
	q.waiting = append(q.waiting, it)
	return len(q.waiting)
}

// start runs it; q.mu must be held.
func (q *Queue) start(it *item) {
	q.running = it
	go func() {
		_ = it.fn(it.ctx)
		it.cancel()
		q.mu.Lock()
		q.running = nil
		if len(q.waiting) > 0 {
			next := q.waiting[0]
			q.waiting = q.waiting[1:]
			q.start(next)
		} else {
			q.idle.Broadcast()
		}
		q.mu.Unlock()
	}()
}

// Cancel stops the running job with key, or removes it from the waiting list.
func (q *Queue) Cancel(key string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.running != nil && q.running.key == key {
		q.running.cancel()
		return true
	}
	for i, it := range q.waiting {
		if it.key == key {
			it.cancel()
			q.waiting = append(q.waiting[:i], q.waiting[i+1:]...)
			return true
		}
	}
	return false
}

// CancelAll drops every waiting job and cancels the running one.
func (q *Queue) CancelAll() {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, it := range q.waiting {
		it.cancel()
	}
	q.waiting = nil
	if q.running != nil {
		q.running.cancel()
	}
}

// Position is 0 when running, 1.. when waiting, -1 when absent.
func (q *Queue) Position(key string) int {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.running != nil && q.running.key == key {
		return 0
	}
	for i, it := range q.waiting {
		if it.key == key {
			return i + 1
		}
	}
	return -1
}

// Busy reports whether a job is running.
func (q *Queue) Busy() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.running != nil
}

// Wait blocks until the queue is idle.
func (q *Queue) Wait() {
	q.mu.Lock()
	defer q.mu.Unlock()
	for q.running != nil || len(q.waiting) > 0 {
		q.idle.Wait()
	}
}
