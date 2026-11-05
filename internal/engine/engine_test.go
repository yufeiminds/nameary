package engine

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yufeiminds/nameary/internal/cache"
	"github.com/yufeiminds/nameary/internal/check"
)

type fakeChecker struct {
	target check.Target
	policy check.Policy
	calls  atomic.Int32
	fn     func(ctx context.Context, q string) (check.Result, error)
}

func (f *fakeChecker) Target() check.Target                 { return f.target }
func (f *fakeChecker) Label() string                        { return string(f.target) }
func (f *fakeChecker) Category() check.Category             { return check.CategoryPackages }
func (f *fakeChecker) Normalize(raw string) (string, error) { return raw, nil }
func (f *fakeChecker) Homepage(q string) string             { return "https://example.test/" + q }
func (f *fakeChecker) Policy() check.Policy                 { return f.policy }
func (f *fakeChecker) Check(ctx context.Context, q string) (check.Result, error) {
	f.calls.Add(1)
	return f.fn(ctx, q)
}

func job(id string, c check.Checker, q string) Job {
	return Job{ID: id, Target: c.Target(), TargetLabel: c.Label(), Category: c.Category(), Raw: q, Query: q, Label: q, Checker: c}
}

func TestRunStreams(t *testing.T) {
	c := &fakeChecker{target: "fake", policy: check.Policy{Concurrency: 2, Timeout: time.Second}, fn: func(ctx context.Context, q string) (check.Result, error) {
		time.Sleep(5 * time.Millisecond)
		return check.Result{Status: check.StatusTaken, Confidence: check.Certain, Query: q}, nil
	}}
	e := New(Options{Concurrency: 3})
	ch := e.Run(context.Background(), 7, []Job{job("a", c, "a"), job("b", c, "b"), job("c", c, "c")})

	started, settled := 0, 0
	for ev := range ch {
		if ev.RunID != 7 {
			t.Fatalf("runID=%d", ev.RunID)
		}
		switch ev.Kind {
		case EventStarted:
			started++
		case EventSettled:
			settled++
			if ev.Result.Status != check.StatusTaken {
				t.Fatalf("status=%v", ev.Result.Status)
			}
		}
	}
	if started != 3 || settled != 3 {
		t.Fatalf("started=%d settled=%d", started, settled)
	}
}

func TestRetry(t *testing.T) {
	var c *fakeChecker
	c = &fakeChecker{target: "fake", policy: check.Policy{Concurrency: 1, Timeout: time.Second}, fn: func(ctx context.Context, q string) (check.Result, error) {
		if c.calls.Load() == 1 {
			return check.Result{}, &check.HTTPStatusError{Code: 429, URL: "https://example.test", RetryAfter: time.Millisecond}
		}
		return check.Result{Status: check.StatusAvailable, Confidence: check.Certain, Query: q}, nil
	}}
	e := New(Options{Concurrency: 1, MaxRetries: 2})
	ch := e.Run(context.Background(), 1, []Job{job("a", c, "a")})
	var res check.Result
	for ev := range ch {
		if ev.Kind == EventSettled {
			res = ev.Result
		}
	}
	if c.calls.Load() != 2 {
		t.Fatalf("calls=%d, want 2", c.calls.Load())
	}
	if res.Status != check.StatusAvailable {
		t.Fatalf("status=%v", res.Status)
	}
}

func TestCache(t *testing.T) {
	store, err := cache.Open(t.TempDir() + "/cache.db")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	c := &fakeChecker{target: "fake", policy: check.Policy{Concurrency: 1, Timeout: time.Second, CacheTTL: time.Hour}, fn: func(ctx context.Context, q string) (check.Result, error) {
		return check.Result{Status: check.StatusTaken, Confidence: check.Certain, Query: q}, nil
	}}
	e := New(Options{Concurrency: 1, Cache: store})
	for range e.Run(context.Background(), 1, []Job{job("a", c, "a")}) {
	}
	if c.calls.Load() != 1 {
		t.Fatalf("calls=%d", c.calls.Load())
	}
	for ev := range e.Run(context.Background(), 2, []Job{job("a", c, "a")}) {
		if ev.Kind == EventSettled && !ev.Result.Cached {
			t.Fatalf("expected cached result: %+v", ev.Result)
		}
	}
	if c.calls.Load() != 1 {
		t.Fatalf("cache miss, calls=%d", c.calls.Load())
	}
}

func TestCancel(t *testing.T) {
	blocked := make(chan struct{})
	c := &fakeChecker{target: "fake", policy: check.Policy{Concurrency: 1, Timeout: time.Minute}, fn: func(ctx context.Context, q string) (check.Result, error) {
		close(blocked)
		<-ctx.Done()
		return check.Result{}, ctx.Err()
	}}
	e := New(Options{Concurrency: 1})
	ctx, cancel := context.WithCancel(context.Background())
	ch := e.Run(ctx, 1, []Job{job("a", c, "a")})
	select {
	case <-blocked:
	case <-time.After(time.Second):
		t.Fatal("checker never started")
	}
	cancel()
	done := make(chan struct{})
	go func() {
		for range ch {
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("engine did not stop after cancel")
	}
}
