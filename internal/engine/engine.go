// Package engine schedules checks, applies per-target limits, retries
// transient failures, caches results, and streams events to any consumer.
package engine

import (
	"context"
	crand "crypto/rand"
	"encoding/binary"
	"errors"
	"sort"
	"sync"
	"time"

	"charm.land/log/v2"
	"golang.org/x/sync/semaphore"

	"github.com/yufeiminds/nameary/internal/cache"
	"github.com/yufeiminds/nameary/internal/check"
	"github.com/yufeiminds/nameary/internal/textutil"
)

// Job is one name × one target.
type Job struct {
	ID          string
	Target      check.Target
	TargetLabel string
	Category    check.Category
	Raw         string
	Query       string
	Label       string
	Priority    int
	Checker     check.Checker
	// Err marks an invalid name so the UI can still show a "skipped" row.
	Err error
}

// EventKind distinguishes the two stream events.
type EventKind int

const (
	EventStarted EventKind = iota
	EventSettled
)

// Event is a small, immutable message suitable for a TUI or a writer.
type Event struct {
	RunID  uint64
	Job    Job
	Kind   EventKind
	Result check.Result
	At     time.Time
}

// Options configures an Engine.
type Options struct {
	Concurrency int
	Cache       *cache.Store
	Refresh     bool
	MaxRetries  int
	// Timeout overrides every checker's policy timeout when non-zero.
	Timeout time.Duration
	// CacheTTL overrides every checker's policy TTL when non-zero.
	CacheTTL time.Duration
}

// Engine is safe for concurrent use. Each Run owns its own worker set.
type Engine struct {
	concurrency int
	cache       *cache.Store
	refresh     bool
	maxRetries  int
	timeout     time.Duration
	cacheTTL    time.Duration
	runs        sync.WaitGroup
}

// New returns an engine with sensible defaults.
func New(opts Options) *Engine {
	if opts.Concurrency <= 0 {
		opts.Concurrency = 16
	}
	if opts.MaxRetries < 0 {
		opts.MaxRetries = 0
	} else if opts.MaxRetries == 0 {
		opts.MaxRetries = 2
	}
	return &Engine{
		concurrency: opts.Concurrency,
		cache:       opts.Cache,
		refresh:     opts.Refresh,
		maxRetries:  opts.MaxRetries,
		timeout:     opts.Timeout,
		cacheTTL:    opts.CacheTTL,
	}
}

// Run starts the checks and returns a channel that closes when every worker
// has stopped. The channel is buffered; consumers should drain it until close.
func (e *Engine) Run(ctx context.Context, runID uint64, jobs []Job) <-chan Event {
	out := make(chan Event, 256)

	ordered := make([]Job, len(jobs))
	copy(ordered, jobs)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].Priority < ordered[j].Priority
	})

	limiters := make(map[check.Target]*semaphore.Weighted)
	for _, j := range ordered {
		if _, ok := limiters[j.Target]; ok || j.Checker == nil {
			continue
		}
		n := j.Checker.Policy().Concurrency
		if n <= 0 {
			n = 4
		}
		limiters[j.Target] = semaphore.NewWeighted(int64(n))
	}

	work := make(chan Job)
	var wg sync.WaitGroup
	for i := 0; i < e.concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range work {
				e.runOne(ctx, runID, j, limiters[j.Target], out)
			}
		}()
	}

	go func() {
		defer close(work)
		for _, j := range ordered {
			select {
			case work <- j:
			case <-ctx.Done():
				return
			}
		}
	}()

	e.runs.Add(1)
	go func() {
		defer e.runs.Done()
		wg.Wait()
		close(out)
	}()

	return out
}

func (e *Engine) runOne(ctx context.Context, runID uint64, j Job, limiter *semaphore.Weighted, out chan<- Event) {
	emit(ctx, out, Event{RunID: runID, Job: j, Kind: EventStarted, At: time.Now()})

	if j.Err != nil {
		emit(ctx, out, Event{
			RunID: runID, Job: j, Kind: EventSettled, At: time.Now(),
			Result: check.Result{Status: check.StatusSkipped, Confidence: check.Low, Query: j.Query, Summary: check.ShortError(j.Err), Err: j.Err},
		})
		return
	}
	if j.Checker == nil {
		err := errors.New("missing checker")
		emit(ctx, out, Event{
			RunID: runID, Job: j, Kind: EventSettled, At: time.Now(),
			Result: check.Result{Status: check.StatusError, Confidence: check.Low, Query: j.Query, Summary: err.Error(), Err: err},
		})
		return
	}

	if e.cache != nil && !e.refresh {
		if r, ok := e.cache.Get(j.Target, j.Query); ok {
			log.Debug("cache hit", "target", j.Target, "query", textutil.Clean(j.Query), "status", r.Status)
			emit(ctx, out, Event{RunID: runID, Job: j, Kind: EventSettled, At: time.Now(), Result: r})
			return
		}
	}

	if limiter != nil {
		if err := limiter.Acquire(ctx, 1); err != nil {
			return
		}
		defer limiter.Release(1)
	}

	start := time.Now()
	res, err := e.checkWithRetry(ctx, j)
	res.Latency = time.Since(start)
	res.FetchedAt = time.Now()
	if res.Query == "" {
		res.Query = j.Query
	}
	if err != nil {
		if res.Status == check.StatusQueued || res.Status == check.StatusChecking || res.Status == check.StatusAvailable {
			errRes := check.ErrorResult(err)
			errRes.Query = j.Query
			errRes.Latency = res.Latency
			errRes.FetchedAt = res.FetchedAt
			res = errRes
		} else {
			res.Err = err
			if res.Summary == "" {
				res.Summary = check.ShortError(err)
			}
		}
	}

	if err == nil && e.cache != nil {
		ttl := j.Checker.Policy().CacheTTL
		if e.cacheTTL > 0 {
			ttl = e.cacheTTL
		}
		e.cache.Put(j.Target, j.Query, res, ttl)
	}

	log.Debug("check done", "target", j.Target, "query", textutil.Clean(j.Query), "status", res.Status, "latency", res.Latency, "cached", res.Cached)
	emit(ctx, out, Event{RunID: runID, Job: j, Kind: EventSettled, At: time.Now(), Result: res})
}

func (e *Engine) checkWithRetry(ctx context.Context, j Job) (check.Result, error) {
	timeout := j.Checker.Policy().Timeout
	if e.timeout > 0 {
		timeout = e.timeout
	}
	if timeout <= 0 {
		timeout = 8 * time.Second
	}

	var last check.Result
	var lastErr error
	for attempt := 0; attempt <= e.maxRetries; attempt++ {
		attemptCtx, cancel := context.WithTimeout(ctx, timeout)
		res, err := j.Checker.Check(attemptCtx, j.Query)
		cancel()
		last, lastErr = res, err
		if err == nil {
			return res, nil
		}
		if ctx.Err() != nil {
			return res, ctx.Err()
		}
		if !check.Retryable(err) || attempt == e.maxRetries {
			return res, err
		}
		delay := retryDelay(attempt, err)
		timer := time.NewTimer(delay)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return last, ctx.Err()
		}
	}
	return last, lastErr
}

func retryDelay(attempt int, err error) time.Duration {
	var he *check.HTTPStatusError
	if errors.As(err, &he) && he.RetryAfter > 0 {
		return he.RetryAfter
	}
	base := 300 * time.Millisecond * time.Duration(1<<attempt)
	jitter := time.Duration(0)
	var buf [8]byte
	if _, err := crand.Read(buf[:]); err == nil {
		jitter = time.Duration(binary.LittleEndian.Uint64(buf[:]) % uint64(200*time.Millisecond))
	}
	d := base + jitter
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}

// Wait blocks until every run started by this engine has closed its event
// channel. Call it before closing shared resources such as the cache.
func (e *Engine) Wait() {
	e.runs.Wait()
}

// WaitTimeout waits up to d for all runs to finish. It returns false when a
// checker is still stuck (for example, a WHOIS socket that ignores context
// cancellation). Callers can then skip closing shared resources and let the
// process exit clean up.
func (e *Engine) WaitTimeout(d time.Duration) bool {
	done := make(chan struct{})
	go func() {
		e.runs.Wait()
		close(done)
	}()
	select {
	case <-done:
		return true
	case <-time.After(d):
		return false
	}
}

func emit(ctx context.Context, out chan<- Event, ev Event) {
	select {
	case out <- ev:
	case <-ctx.Done():
	}
}
