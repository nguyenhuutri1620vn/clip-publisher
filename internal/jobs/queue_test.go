package jobs

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestSequentialAndPositions(t *testing.T) {
	q := NewQueue()
	var mu sync.Mutex
	var order []string
	release := make(chan struct{})
	started := make(chan struct{})
	if pos := q.Enqueue("a", func(ctx context.Context) error {
		close(started)
		<-release
		mu.Lock()
		order = append(order, "a")
		mu.Unlock()
		return nil
	}); pos != 0 {
		t.Fatalf("first pos %d", pos)
	}
	<-started
	if pos := q.Enqueue("b", func(ctx context.Context) error {
		mu.Lock()
		order = append(order, "b")
		mu.Unlock()
		return nil
	}); pos != 1 {
		t.Fatalf("second pos %d", pos)
	}
	if q.Position("a") != 0 || q.Position("b") != 1 || q.Position("zz") != -1 {
		t.Fatal("positions")
	}
	close(release)
	q.Wait()
	if len(order) != 2 || order[0] != "a" || order[1] != "b" {
		t.Fatalf("order %v", order)
	}
	if q.Position("a") != -1 {
		t.Fatal("finished job still present")
	}
}

func TestCancelWaiting(t *testing.T) {
	q := NewQueue()
	release := make(chan struct{})
	q.Enqueue("a", func(ctx context.Context) error { <-release; return nil })
	ran := false
	q.Enqueue("b", func(ctx context.Context) error { ran = true; return nil })
	if !q.Cancel("b") {
		t.Fatal("cancel waiting returned false")
	}
	close(release)
	q.Wait()
	if ran {
		t.Fatal("canceled job ran")
	}
}

func TestCancelRunning(t *testing.T) {
	q := NewQueue()
	done := make(chan error, 1)
	started := make(chan struct{})
	q.Enqueue("a", func(ctx context.Context) error {
		close(started)
		select {
		case <-ctx.Done():
			done <- ctx.Err()
		case <-time.After(5 * time.Second):
			done <- nil
		}
		return nil
	})
	<-started
	if !q.Cancel("a") {
		t.Fatal("cancel running returned false")
	}
	if err := <-done; err == nil {
		t.Fatal("ctx not canceled")
	}
	q.Wait()
	if q.Cancel("missing") {
		t.Fatal("cancel of missing key returned true")
	}
}

func TestCancelAll(t *testing.T) {
	q := NewQueue()
	started := make(chan struct{})
	q.Enqueue("a", func(ctx context.Context) error { close(started); <-ctx.Done(); return nil })
	q.Enqueue("b", func(ctx context.Context) error { t.Error("b ran"); return nil })
	<-started
	if !q.Busy() {
		t.Fatal("not busy")
	}
	q.CancelAll()
	q.Wait()
	if q.Busy() {
		t.Fatal("still busy")
	}
}
