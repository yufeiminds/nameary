package adapters

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yufeiminds/nameary/internal/check"
)

type adapterEntry struct {
	name     string
	target   check.Target
	query    string
	notFound bool
	build    func(*check.HTTP, string) check.Checker
}

func allAdapters() []adapterEntry {
	return []adapterEntry{
		{"npm", check.TargetNPM, "acme", true, func(h *check.HTTP, b string) check.Checker { return NewNPM(h, b) }},
		{"pypi", check.TargetPyPI, "acme", true, func(h *check.HTTP, b string) check.Checker { return NewPyPI(h, b) }},
		{"crates", check.TargetCrates, "acme", true, func(h *check.HTTP, b string) check.Checker { return NewCrates(h, b) }},
		{"rubygems", check.TargetRubyGems, "acme", true, func(h *check.HTTP, b string) check.Checker { return NewRubyGems(h, b) }},
		{"nuget", check.TargetNuGet, "acme", true, func(h *check.HTTP, b string) check.Checker { return NewNuGet(h, b) }},
		{"dockerhub", check.TargetDocker, "acme", true, func(h *check.HTTP, b string) check.Checker { return NewDockerHub(h, b) }},
		{"homebrew", check.TargetHomebrew, "acme", true, func(h *check.HTTP, b string) check.Checker { return NewHomebrew(h, b) }},
		{"packagist", check.TargetPackagist, "acme/acme", true, func(h *check.HTTP, b string) check.Checker { return NewPackagist(h, b) }},
		{"hex", check.TargetHex, "acme", true, func(h *check.HTTP, b string) check.Checker { return NewHex(h, b) }},
		{"pub", check.TargetPub, "acme", true, func(h *check.HTTP, b string) check.Checker { return NewPub(h, b) }},
		{"github", check.TargetGitHub, "acme", true, func(h *check.HTTP, b string) check.Checker { return NewGitHub(h, b, "tok") }},
		{"maven", check.TargetMaven, "acme", false, func(h *check.HTTP, b string) check.Checker { return NewMaven(h, b) }},
		{"conda", check.TargetConda, "acme", false, func(h *check.HTTP, b string) check.Checker { return NewConda(h, b) }},
		{"cocoapods", check.TargetCocoaPods, "Acme", true, func(h *check.HTTP, b string) check.Checker { return NewCocoaPods(h, b) }},
		{"aur", check.TargetAUR, "acme", false, func(h *check.HTTP, b string) check.Checker { return NewAUR(h, b) }},
		{"snap", check.TargetSnap, "acme", true, func(h *check.HTTP, b string) check.Checker { return NewSnap(h, b) }},
		{"chocolatey", check.TargetChocolatey, "acme", true, func(h *check.HTTP, b string) check.Checker { return NewChocolatey(h, b) }},
		{"hackage", check.TargetHackage, "acme", true, func(h *check.HTTP, b string) check.Checker { return NewHackage(h, b) }},
		{"jsr", check.TargetJSR, "acme", false, func(h *check.HTTP, b string) check.Checker { return NewJSR(h, b) }},
	}
}

func TestAdapterInterfacesAndNormalization(t *testing.T) {
	for _, tc := range allAdapters() {
		t.Run(tc.name, func(t *testing.T) {
			c := tc.build(testHTTP(), "")
			if c.Target() != tc.target {
				t.Fatalf("target=%q", c.Target())
			}
			if c.Label() == "" || c.Category() == "" {
				t.Fatal("empty label/category")
			}
			p := c.Policy()
			if p.Concurrency <= 0 || p.Timeout <= 0 || p.CacheTTL <= 0 {
				t.Fatalf("bad policy: %+v", p)
			}
			q, err := c.Normalize(tc.query)
			if err != nil || q == "" {
				t.Fatalf("normalize: %q %v", q, err)
			}
			if c.Homepage(q) == "" {
				t.Fatal("empty homepage")
			}
		})
	}
}

func TestAdapterRateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	for _, tc := range allAdapters() {
		t.Run(tc.name, func(t *testing.T) {
			c := tc.build(testHTTP(), srv.URL)
			_, err := c.Check(context.Background(), tc.query)
			if err == nil || !check.Retryable(err) {
				t.Fatalf("expected retryable 429, got %v", err)
			}
		})
	}
}

func TestAdapterNotFound(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()

	for _, tc := range allAdapters() {
		if !tc.notFound {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			c := tc.build(testHTTP(), srv.URL)
			res, err := c.Check(context.Background(), tc.query)
			if err != nil {
				t.Fatal(err)
			}
			if res.Status != check.StatusAvailable {
				t.Fatalf("status=%v", res.Status)
			}
		})
	}
}

func TestAdapterEmptySuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/search":
			_, _ = w.Write([]byte(`[]`))
		case strings.Contains(r.URL.Path, "/api/v2/Packages"):
			_, _ = w.Write([]byte(`<feed></feed>`))
		case r.URL.Path == "/api/packages":
			_, _ = w.Write([]byte(`{"items":[]}`))
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()

	for _, tc := range allAdapters() {
		if tc.notFound {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			c := tc.build(testHTTP(), srv.URL)
			if _, err := c.Check(context.Background(), tc.query); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestDockerHubNamespaceFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/users/acme/":
			http.NotFound(w, r)
		case "/v2/orgs/acme/":
			_, _ = w.Write([]byte(`{"name":"acme"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	d := NewDockerHub(testHTTP(), srv.URL)
	res, err := d.Check(context.Background(), "acme")
	if err != nil || res.Status != check.StatusTaken || res.Field("found") != "organization namespace" {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}

func TestHomebrewCaskFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/formula/acme.json":
			http.NotFound(w, r)
		case "/api/cask/acme.json":
			_, _ = w.Write([]byte(`{"token":"acme","version":"1.2.3","desc":"app"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	b := NewHomebrew(testHTTP(), srv.URL)
	res, err := b.Check(context.Background(), "acme")
	if err != nil || res.Status != check.StatusTaken || res.Field("kind") != "cask" {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}

func TestCocoaPodsCaseFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/pods/Acme" {
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	c := NewCocoaPods(testHTTP(), srv.URL)
	res, err := c.Check(context.Background(), "acme")
	if err != nil || res.Status != check.StatusTaken {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if res.Field("note") == "" {
		t.Fatal("expected case-variant note")
	}
}

func TestGitHubHTMLFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			t.Errorf("method=%s", r.Method)
		}
		if r.URL.Path == "/acme" {
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	g := NewGitHub(testHTTP(), "", "")
	g.webBase = srv.URL
	res, err := g.Check(context.Background(), "acme")
	if err != nil || res.Status != check.StatusTaken {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	res, err = g.Check(context.Background(), "nobody")
	if err != nil || res.Status != check.StatusAvailable {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}

func TestCondaTruncatedUnknown(t *testing.T) {
	items := make([]string, 50)
	for i := range items {
		items[i] = `{"name":"other"}`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("[" + strings.Join(items, ",") + "]"))
	}))
	defer srv.Close()
	c := NewConda(testHTTP(), srv.URL)
	res, err := c.Check(context.Background(), "acme")
	if err != nil || res.Status != check.StatusUnknown {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}

func TestNPMLatestAndScoped(t *testing.T) {
	var seen string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.URL.Path
		_, _ = w.Write([]byte(`{"name":"@scope/pkg","version":"1.0.0","description":"x"}`))
	}))
	defer srv.Close()
	n := NewNPM(testHTTP(), srv.URL)
	res, err := n.Check(context.Background(), "@scope/pkg")
	if err != nil || res.Status != check.StatusTaken || res.Summary != "v1.0.0" {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if !strings.Contains(seen, "scope") {
		t.Fatalf("path=%q", seen)
	}
}

func TestAllCheckers(t *testing.T) {
	all := All(Options{})
	if len(all) != 19 {
		t.Fatalf("len=%d", len(all))
	}
	seen := map[check.Target]bool{}
	for _, c := range all {
		if c == nil || c.Target() == "" || c.Label() == "" {
			t.Fatalf("bad checker: %+v", c)
		}
		if seen[c.Target()] {
			t.Fatalf("duplicate target %s", c.Target())
		}
		seen[c.Target()] = true
	}
	if got := All(Options{HTTP: testHTTP()}); len(got) != 19 {
		t.Fatalf("len=%d", len(got))
	}
}
