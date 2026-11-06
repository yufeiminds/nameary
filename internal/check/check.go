// Package check defines the contract between the availability engine and the
// individual registry / domain adapters.
package check

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Target is the stable identifier for a check source.
type Target string

const (
	TargetDomains    Target = "domains"
	TargetNPM        Target = "npm"
	TargetPyPI       Target = "pypi"
	TargetCrates     Target = "crates"
	TargetRubyGems   Target = "rubygems"
	TargetNuGet      Target = "nuget"
	TargetDocker     Target = "dockerhub"
	TargetHomebrew   Target = "homebrew"
	TargetPackagist  Target = "packagist"
	TargetHex        Target = "hex"
	TargetPub        Target = "pub"
	TargetGitHub     Target = "github"
	TargetMaven      Target = "maven"
	TargetConda      Target = "conda"
	TargetCocoaPods  Target = "cocoapods"
	TargetAUR        Target = "aur"
	TargetSnap       Target = "snap"
	TargetChocolatey Target = "chocolatey"
	TargetHackage    Target = "hackage"
	TargetJSR        Target = "jsr"
)

// Category groups rows in the UI.
type Category string

const (
	CategoryDomains  Category = "DOMAINS"
	CategoryPackages Category = "PACKAGES"
	CategoryHandles  Category = "HANDLES"
)

// Status is the lifecycle state of a single check.
type Status int

const (
	StatusQueued Status = iota
	StatusChecking
	StatusAvailable
	StatusTaken
	StatusUnknown
	StatusError
	StatusSkipped
	StatusCanceled
	StatusTimeout
	StatusRateLimited
)

func (s Status) String() string {
	switch s {
	case StatusQueued:
		return "queued"
	case StatusChecking:
		return "checking"
	case StatusAvailable:
		return "available"
	case StatusTaken:
		return "taken"
	case StatusUnknown:
		return "unknown"
	case StatusError:
		return "error"
	case StatusSkipped:
		return "skipped"
	case StatusCanceled:
		return "canceled"
	case StatusTimeout:
		return "timeout"
	case StatusRateLimited:
		return "rate_limit"
	default:
		return "unknown"
	}
}

// IsFinal reports whether the status will not change without a rerun.
func (s Status) IsFinal() bool {
	switch s {
	case StatusAvailable, StatusTaken, StatusUnknown, StatusError, StatusSkipped, StatusCanceled, StatusTimeout, StatusRateLimited:
		return true
	}
	return false
}

// Confidence says how much weight to put on an "available" result.
type Confidence int

const (
	Certain Confidence = iota
	Likely
	Low
)

func (c Confidence) String() string {
	switch c {
	case Certain:
		return "certain"
	case Likely:
		return "likely"
	case Low:
		return "low"
	default:
		return "unknown"
	}
}

// Field is an ordered key/value pair for the detail view and exports.
type Field struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Result is the outcome of one check. It is deliberately richer than a bool:
// a timeout, a rate limit and a 404 are three different things.
type Result struct {
	Status     Status        `json:"status"`
	Confidence Confidence    `json:"confidence"`
	Query      string        `json:"query"`
	Summary    string        `json:"summary,omitempty"`
	Fields     []Field       `json:"fields,omitempty"`
	SourceURL  string        `json:"source_url,omitempty"`
	FetchedAt  time.Time     `json:"fetched_at"`
	Latency    time.Duration `json:"latency"`
	Cached     bool          `json:"cached,omitempty"`
	Err        error         `json:"-"`
}

// Field returns the value of a detail field, or "".
func (r Result) Field(key string) string {
	for _, f := range r.Fields {
		if f.Key == key {
			return f.Value
		}
	}
	return ""
}

// Policy controls scheduling and caching for a checker.
type Policy struct {
	// Priority is lower-is-earlier. Domains start first because they are the
	// slowest and most visible category.
	Priority int
	// Concurrency is the maximum number of in-flight requests for this target.
	Concurrency int
	// Timeout is the per-attempt deadline.
	Timeout time.Duration
	// CacheTTL is how long a definitive result stays fresh.
	CacheTTL time.Duration
}

// Checker is implemented by every registry and by the domain checker.
type Checker interface {
	Target() Target
	Label() string
	Category() Category
	Normalize(raw string) (string, error)
	Check(ctx context.Context, query string) (Result, error)
	Homepage(query string) string
	Policy() Policy
}

// HTTPStatusError is returned by adapters for non-2xx responses. The engine
// uses it to decide whether a retry is worth it.
type HTTPStatusError struct {
	Code       int
	URL        string
	RetryAfter time.Duration
}

func (e *HTTPStatusError) Error() string {
	if e.RetryAfter > 0 {
		return fmt.Sprintf("http %d from %s (retry after %s)", e.Code, e.URL, e.RetryAfter)
	}
	return fmt.Sprintf("http %d from %s", e.Code, e.URL)
}

// Retryable reports whether the status is transient.
func (e *HTTPStatusError) Retryable() bool {
	switch e.Code {
	case http.StatusTooManyRequests, http.StatusInternalServerError,
		http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	}
	return false
}

// Retryable reports whether err is worth retrying.
func Retryable(err error) bool {
	if err == nil {
		return false
	}
	var r interface{ Retryable() bool }
	if errors.As(err, &r) {
		return r.Retryable()
	}
	// Timeouts are not retried: they usually mean a slow or unavailable
	// source, and a second attempt would double the worst-case latency.
	return false
}

// IsRateLimited reports whether err is a 429 or a GitHub quota error.
func IsRateLimited(err error) bool {
	var he *HTTPStatusError
	if errors.As(err, &he) && he.Code == http.StatusTooManyRequests {
		return true
	}
	return false
}

// StatusCode extracts an HTTP status code from err, or 0.
func StatusCode(err error) int {
	var he *HTTPStatusError
	if errors.As(err, &he) {
		return he.Code
	}
	return 0
}

// ErrorResult converts an error into a conservative result. It never returns
// StatusAvailable: a network problem is not a free name.
func ErrorResult(err error) Result {
	if err == nil {
		return Result{Status: StatusUnknown}
	}
	status := StatusError
	switch {
	case errors.Is(err, context.Canceled):
		status = StatusCanceled
	case errors.Is(err, context.DeadlineExceeded):
		status = StatusTimeout
	case IsRateLimited(err):
		status = StatusRateLimited
	default:
		var ne net.Error
		if errors.As(err, &ne) && ne.Timeout() {
			status = StatusTimeout
		}
		var he *HTTPStatusError
		if errors.As(err, &he) && he.Code >= 500 {
			status = StatusUnknown
		}
	}
	return Result{
		Status:     status,
		Confidence: Low,
		Summary:    ShortError(err),
		Err:        err,
	}
}

// ShortError makes transport errors fit in a table cell.
func ShortError(err error) string {
	if err == nil {
		return ""
	}
	var he *HTTPStatusError
	if errors.As(err, &he) {
		switch he.Code {
		case http.StatusTooManyRequests:
			return "rate limited"
		case http.StatusForbidden:
			return "forbidden"
		case http.StatusNotFound:
			return "not found"
		default:
			if he.Code >= 500 {
				return fmt.Sprintf("server %d", he.Code)
			}
		}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	msg := strings.TrimSpace(err.Error())
	if len(msg) > 80 {
		msg = msg[:77] + "..."
	}
	return msg
}

// RetryAfter parses a Retry-After header (seconds or HTTP date), capped.
func RetryAfter(v string) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(v); err == nil && seconds > 0 {
		d := time.Duration(seconds) * time.Second
		if d > 5*time.Second {
			d = 5 * time.Second
		}
		return d
	}
	if t, err := http.ParseTime(v); err == nil {
		d := time.Until(t)
		if d < 0 {
			return 0
		}
		if d > 5*time.Second {
			d = 5 * time.Second
		}
		return d
	}
	return 0
}
