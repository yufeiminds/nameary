package engine

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/yufeiminds/nameary/internal/cache"
	"github.com/yufeiminds/nameary/internal/check"
)

func TestMissingCheckerAndJobError(t *testing.T) {
	e := New(Options{Concurrency: 2})
	jobs := []Job{
		{ID: "missing", Target: "fake", Query: "x", Label: "x"},
		{ID: "bad", Target: "fake", Query: "x", Label: "x", Err: fmt.Errorf("bad name")},
	}
	var statuses []check.Status
	for ev := range e.Run(context.Background(), 1, jobs) {
		if ev.Kind == EventSettled {
			statuses = append(statuses, ev.Result.Status)
		}
	}
	if len(statuses) != 2 {
		t.Fatalf("statuses=%v", statuses)
	}
	seen := map[check.Status]bool{}
	for _, st := range statuses {
		seen[st] = true
	}
	if !seen[check.StatusError] || !seen[check.StatusSkipped] {
		t.Fatalf("statuses=%v", statuses)
	}
}

func TestTimeoutIsNotRetried(t *testing.T) {
	var calls int
	c := &fakeChecker{target: "fake", policy: check.Policy{Concurrency: 1, Timeout: 20 * time.Millisecond}, fn: func(ctx context.Context, q string) (check.Result, error) {
		calls++
		<-ctx.Done()
		return check.Result{}, ctx.Err()
	}}
	e := New(Options{Concurrency: 1, MaxRetries: 2, Timeout: 20 * time.Millisecond})
	var res check.Result
	for ev := range e.Run(context.Background(), 1, []Job{job("a", c, "a")}) {
		if ev.Kind == EventSettled {
			res = ev.Result
		}
	}
	if calls != 1 {
		t.Fatalf("calls=%d", calls)
	}
	if res.Status != check.StatusTimeout {
		t.Fatalf("status=%v", res.Status)
	}
}

func TestRetryServerError(t *testing.T) {
	var c *fakeChecker
	c = &fakeChecker{target: "fake", policy: check.Policy{Concurrency: 1, Timeout: time.Second}, fn: func(ctx context.Context, q string) (check.Result, error) {
		if c.calls.Load() == 1 {
			return check.Result{}, &check.HTTPStatusError{Code: 500, URL: "https://x", RetryAfter: time.Millisecond}
		}
		return check.Result{Status: check.StatusTaken, Confidence: check.Certain, Query: q}, nil
	}}
	e := New(Options{Concurrency: 1, MaxRetries: 2})
	var res check.Result
	for ev := range e.Run(context.Background(), 1, []Job{job("a", c, "a")}) {
		if ev.Kind == EventSettled {
			res = ev.Result
		}
	}
	if c.calls.Load() != 2 || res.Status != check.StatusTaken {
		t.Fatalf("calls=%d res=%+v", c.calls.Load(), res)
	}
}

func TestRefreshSkipsCache(t *testing.T) {
	store, err := cache.Open(t.TempDir() + "/cache.db")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	store.Put("fake", "a", check.Result{Status: check.StatusTaken, Confidence: check.Certain}, time.Hour)

	var calls int
	c := &fakeChecker{target: "fake", policy: check.Policy{Concurrency: 1, Timeout: time.Second, CacheTTL: time.Hour}, fn: func(ctx context.Context, q string) (check.Result, error) {
		calls++
		return check.Result{Status: check.StatusAvailable, Confidence: check.Certain, Query: q}, nil
	}}
	e := New(Options{Concurrency: 1, Cache: store, Refresh: true})
	var res check.Result
	for ev := range e.Run(context.Background(), 1, []Job{job("a", c, "a")}) {
		if ev.Kind == EventSettled {
			res = ev.Result
		}
	}
	if calls != 1 || res.Cached || res.Status != check.StatusAvailable {
		t.Fatalf("calls=%d res=%+v", calls, res)
	}
}

func TestEmptyRunAndWait(t *testing.T) {
	e := New(Options{Concurrency: 1})
	ch := e.Run(context.Background(), 1, nil)
	if _, ok := <-ch; ok {
		t.Fatal("expected closed channel")
	}
	e.Wait()
}

func TestWaitTimeout(t *testing.T) {
	blocked := make(chan struct{})
	c := &fakeChecker{target: "fake", policy: check.Policy{Concurrency: 1, Timeout: time.Minute}, fn: func(ctx context.Context, q string) (check.Result, error) {
		close(blocked)
		<-ctx.Done()
		return check.Result{}, ctx.Err()
	}}
	e := New(Options{Concurrency: 1})
	ctx, cancel := context.WithCancel(context.Background())
	e.Run(ctx, 1, []Job{job("a", c, "a")})
	<-blocked
	// Drain is not required for Wait; the checker is blocked until cancel.
	if e.WaitTimeout(10 * time.Millisecond) {
		t.Fatal("expected timeout")
	}
	cancel()
	if !e.WaitTimeout(time.Second) {
		t.Fatal("expected runs to finish")
	}
}

func TestRetryDelay(t *testing.T) {
	if d := retryDelay(0, &check.HTTPStatusError{Code: 429, RetryAfter: 2 * time.Second}); d != 2*time.Second {
		t.Fatalf("d=%v", d)
	}
	d := retryDelay(1, &check.HTTPStatusError{Code: 500})
	if d <= 0 || d > 5*time.Second {
		t.Fatalf("d=%v", d)
	}
}
