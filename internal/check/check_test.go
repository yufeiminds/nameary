package check

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
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

type timeoutErr struct{}

func (timeoutErr) Error() string   { return "timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return true }

func TestStatusAndConfidenceStrings(t *testing.T) {
	statuses := map[Status]string{
		StatusQueued: "queued", StatusChecking: "checking", StatusAvailable: "available",
		StatusTaken: "taken", StatusUnknown: "unknown", StatusError: "error",
		StatusSkipped: "skipped", StatusCanceled: "canceled", Status(99): "unknown",
	}
	for s, want := range statuses {
		if got := s.String(); got != want {
			t.Fatalf("%d: got %q want %q", s, got, want)
		}
	}
	if !StatusAvailable.IsFinal() || !StatusCanceled.IsFinal() || StatusQueued.IsFinal() || StatusChecking.IsFinal() {
		t.Fatal("IsFinal mapping wrong")
	}
	conf := map[Confidence]string{Certain: "certain", Likely: "likely", Low: "low", Confidence(99): "unknown"}
	for c, want := range conf {
		if got := c.String(); got != want {
			t.Fatalf("%d: got %q want %q", c, got, want)
		}
	}
}

func TestResultFieldAndErrors(t *testing.T) {
	r := Result{Fields: []Field{{Key: "a", Value: "b"}}}
	if r.Field("a") != "b" || r.Field("x") != "" {
		t.Fatal("Field lookup failed")
	}
	e := &HTTPStatusError{Code: 429, URL: "https://x", RetryAfter: time.Second}
	if e.Error() == "" || !e.Retryable() || StatusCode(e) != 429 {
		t.Fatalf("bad error: %v", e)
	}
	e.RetryAfter = 0
	if e.Error() == "" {
		t.Fatal("empty error string")
	}
	if StatusCode(errors.New("x")) != 0 {
		t.Fatal("status code should be 0")
	}
	if !Retryable(&HTTPStatusError{Code: 503}) {
		t.Fatal("503 retryable")
	}
	if Retryable(timeoutErr{}) {
		t.Fatal("timeouts should not be retried")
	}
	if Retryable(errors.New("no")) {
		t.Fatal("generic error retryable")
	}
}

func TestErrorResultBranches(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want Status
	}{
		{context.Canceled, StatusCanceled},
		{context.DeadlineExceeded, StatusUnknown},
		{timeoutErr{}, StatusUnknown},
		{&HTTPStatusError{Code: 429}, StatusUnknown},
		{&HTTPStatusError{Code: 500}, StatusUnknown},
		{&HTTPStatusError{Code: 403}, StatusError},
		{errors.New("bad"), StatusError},
	} {
		if got := ErrorResult(tc.err); got.Status != tc.want {
			t.Fatalf("%v: got %v want %v", tc.err, got.Status, tc.want)
		}
	}
}

func TestShortErrorAndRetryAfter(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want string
	}{
		{&HTTPStatusError{Code: 429}, "rate limited"},
		{&HTTPStatusError{Code: 403}, "forbidden"},
		{&HTTPStatusError{Code: 404}, "not found"},
		{&HTTPStatusError{Code: 502}, "server 502"},
		{context.DeadlineExceeded, "timeout"},
		{errors.New(strings.Repeat("x", 100)), strings.Repeat("x", 77) + "..."},
	} {
		if got := ShortError(tc.err); got != tc.want {
			t.Fatalf("got %q want %q", got, tc.want)
		}
	}
	if ShortError(nil) != "" {
		t.Fatal("nil short error")
	}
	if got := RetryAfter("1"); got != time.Second {
		t.Fatalf("got %v", got)
	}
	if got := RetryAfter(time.Now().Add(time.Second).UTC().Format(http.TimeFormat)); got <= 0 {
		t.Fatalf("date retry-after: %v", got)
	}
	if got := RetryAfter("bad"); got != 0 {
		t.Fatalf("bad retry-after: %v", got)
	}
}

func TestHTTPHelpers(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true}`))
		case "/bad":
			_, _ = w.Write([]byte(`{`))
		case "/redirect":
			http.Redirect(w, r, "http://example.com/", http.StatusFound)
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`missing`))
		}
	}))
	defer srv.Close()

	h := NewHTTP()
	if h == nil || h.Client == nil {
		t.Fatal("nil HTTP")
	}
	var out struct {
		OK bool `json:"ok"`
	}
	code, _, err := h.Get(context.Background(), srv.URL+"/json", &out)
	if err != nil || code != 200 || !out.OK {
		t.Fatalf("get: %d %+v %v", code, out, err)
	}
	code, _, err = h.GetWithHeaders(context.Background(), srv.URL+"/json", map[string]string{"X-Test": "yes"}, &out)
	if err != nil || code != 200 {
		t.Fatalf("get with headers: %d %v", code, err)
	}
	if code, _, err := h.Get(context.Background(), srv.URL+"/bad", &out); err == nil || code != 200 {
		t.Fatalf("bad json: %d %v", code, err)
	}
	code, _, err = h.Get(context.Background(), srv.URL+"/missing", &out)
	if err != nil || code != 404 {
		t.Fatalf("404: %d %v", code, err)
	}
	code, _, err = h.Head(context.Background(), srv.URL+"/json")
	if err != nil || code != 200 {
		t.Fatalf("head: %d %v", code, err)
	}
	code, _, body, err := h.GetRawWithHeaders(context.Background(), srv.URL+"/missing", map[string]string{"X-Test": "yes"})
	if err != nil || code != 404 || string(body) != "missing" {
		t.Fatalf("raw: %d %q %v", code, body, err)
	}
	if _, _, err := h.Get(context.Background(), srv.URL+"/redirect", &out); err == nil {
		t.Fatal("expected redirect downgrade error")
	}
	if NewHTTPError(200, "x", nil) != nil {
		t.Fatal("200 should not be an error")
	}
	if JoinURL("https://x/", "/a") != "https://x/a" {
		t.Fatal("JoinURL")
	}
}
