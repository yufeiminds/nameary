package check

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	// DefaultUserAgent identifies the tool politely. Registries such as
	// crates.io require a meaningful UA.
	DefaultUserAgent = "nameary/0.1 (+https://github.com/yufeiminds/nameary)"
	maxBodyBytes     = 4 << 20
)

// HTTP wraps a shared client with the headers we use everywhere.
type HTTP struct {
	Client *http.Client
	UA     string
}

// NewHTTP returns a client tuned for many small parallel API requests.
func NewHTTP() *HTTP {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.MaxIdleConns = 100
	tr.MaxIdleConnsPerHost = 8
	tr.IdleConnTimeout = 90 * time.Second
	tr.ForceAttemptHTTP2 = true
	return &HTTP{
		Client: &http.Client{
			Transport: tr,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return fmt.Errorf("too many redirects")
				}
				if req.URL.Scheme != "https" {
					return fmt.Errorf("refusing non-https redirect to %s", req.URL.Redacted())
				}
				return nil
			},
		},
		UA: DefaultUserAgent,
	}
}

// Get fetches url, optionally decodes JSON into out on 200, and returns the
// status code. Non-2xx responses are not errors: callers map 404 etc.
func (h *HTTP) Get(ctx context.Context, url string, out any) (int, http.Header, error) {
	return h.do(ctx, http.MethodGet, url, nil, out)
}

// GetWithHeaders is Get with extra request headers (used for GitHub tokens).
func (h *HTTP) GetWithHeaders(ctx context.Context, url string, headers map[string]string, out any) (int, http.Header, error) {
	return h.do(ctx, http.MethodGet, url, headers, out)
}

// Head performs a HEAD request.
func (h *HTTP) Head(ctx context.Context, url string) (int, http.Header, error) {
	code, header, _, err := h.doRaw(ctx, http.MethodHead, url, nil)
	return code, header, err
}

// GetRaw fetches url and returns the body even for non-2xx responses. RDAP
// uses this to distinguish "domain not found" from "no RDAP service".
func (h *HTTP) GetRaw(ctx context.Context, url string) (int, http.Header, []byte, error) {
	return h.doRaw(ctx, http.MethodGet, url, nil)
}

// GetRawWithHeaders is GetRaw with extra request headers.
func (h *HTTP) GetRawWithHeaders(ctx context.Context, url string, headers map[string]string) (int, http.Header, []byte, error) {
	return h.doRaw(ctx, http.MethodGet, url, headers)
}

func (h *HTTP) do(ctx context.Context, method, url string, headers map[string]string, out any) (int, http.Header, error) {
	code, header, body, err := h.doRaw(ctx, method, url, headers)
	if err != nil {
		return code, header, err
	}
	if code == http.StatusOK && out != nil && len(body) > 0 {
		if err := json.Unmarshal(body, out); err != nil {
			return code, header, fmt.Errorf("decode %s: %w", url, err)
		}
	}
	return code, header, nil
}

func (h *HTTP) doRaw(ctx context.Context, method, url string, headers map[string]string) (int, http.Header, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return 0, nil, nil, err
	}
	ua := h.UA
	if ua == "" {
		ua = DefaultUserAgent
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.8")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := h.Client.Do(req)
	if err != nil {
		return 0, nil, nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return resp.StatusCode, resp.Header, body, err
	}
	return resp.StatusCode, resp.Header, body, nil
}

// StatusError converts a non-2xx response into a typed error.
func NewHTTPError(code int, url string, header http.Header) error {
	if code >= 200 && code < 300 {
		return nil
	}
	var retry time.Duration
	if header != nil {
		retry = RetryAfter(header.Get("Retry-After"))
	}
	return &HTTPStatusError{Code: code, URL: url, RetryAfter: retry}
}

// JoinURL joins a base and a path without doubling slashes.
func JoinURL(base, path string) string {
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(path, "/")
}
