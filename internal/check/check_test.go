package check

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"
)

func TestErrorResult(t *testing.T) {
	if got := ErrorResult(context.DeadlineExceeded); got.Status != StatusUnknown {
		t.Fatalf("deadline status=%v", got.Status)
	}
	if got := ErrorResult(context.Canceled); got.Status != StatusCanceled {
		t.Fatalf("canceled status=%v", got.Status)
	}
	if got := ErrorResult(&HTTPStatusError{Code: http.StatusTooManyRequests}); got.Status != StatusUnknown {
		t.Fatalf("429 status=%v", got.Status)
	}
	if got := ErrorResult(&HTTPStatusError{Code: http.StatusBadGateway}); got.Status != StatusUnknown {
		t.Fatalf("502 status=%v", got.Status)
	}
	if got := ErrorResult(errors.New("bad json")); got.Status != StatusError {
		t.Fatalf("generic status=%v", got.Status)
	}
}

func TestRetryable(t *testing.T) {
	for _, code := range []int{429, 500, 502, 503, 504} {
		if !Retryable(&HTTPStatusError{Code: code}) {
			t.Fatalf("%d should retry", code)
		}
	}
	if Retryable(&HTTPStatusError{Code: 404}) {
		t.Fatal("404 should not retry")
	}
}

func TestRetryAfter(t *testing.T) {
	if got := RetryAfter("2"); got != 2*time.Second {
		t.Fatalf("got=%v", got)
	}
	if got := RetryAfter("999"); got != 5*time.Second {
		t.Fatalf("cap got=%v", got)
	}
	if got := RetryAfter(""); got != 0 {
		t.Fatalf("empty got=%v", got)
	}
}
