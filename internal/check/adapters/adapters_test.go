package adapters

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yufeiminds/nameary/internal/check"
)

func testHTTP() *check.HTTP { return check.NewHTTP() }

func TestNPM(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/react/latest":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"name":"react","version":"19.0.0","description":"UI library"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	n := NewNPM(testHTTP(), srv.URL)
	res, err := n.Check(context.Background(), "react")
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != check.StatusTaken {
		t.Fatalf("status = %v, want taken", res.Status)
	}
	if res.Summary != "v19.0.0" {
		t.Fatalf("summary = %q", res.Summary)
	}

	res, err = n.Check(context.Background(), "zzqxjvnamearytest")
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != check.StatusAvailable {
		t.Fatalf("status = %v, want available", res.Status)
	}
}

func TestNormalize(t *testing.T) {
	tests := []struct {
		name    string
		checker check.Checker
		in      string
		want    string
		wantErr bool
	}{
		{"npm lower", NewNPM(testHTTP(), ""), "React", "react", false},
		{"npm scoped", NewNPM(testHTTP(), ""), "@Scope/Pkg", "@scope/pkg", false},
		{"npm leading dot", NewNPM(testHTTP(), ""), ".foo", "", true},
		{"pypi underscores", NewPyPI(testHTTP(), ""), "My_Package.Name", "my-package-name", false},
		{"crates", NewCrates(testHTTP(), ""), "Serde_JSON", "serde_json", false},
		{"github", NewGitHub(testHTTP(), "", ""), "Foo-Bar", "foo-bar", false},
		{"github double hyphen", NewGitHub(testHTTP(), "", ""), "foo--bar", "", true},
		{"brew", NewHomebrew(testHTTP(), ""), "Foo@1", "foo@1", false},
		{"packagist bare", NewPackagist(testHTTP(), ""), "Acme", "acme/acme", false},
		{"maven", NewMaven(testHTTP(), ""), "Guava", "guava", false},
		{"snap", NewSnap(testHTTP(), ""), "Hello-World", "hello-world", false},
		{"snap bad", NewSnap(testHTTP(), ""), "Hello_World", "", true},
		{"cocoapods case", NewCocoaPods(testHTTP(), ""), "AFNetworking", "AFNetworking", false},
		{"jsr scoped", NewJSR(testHTTP(), ""), "@STD/Async", "@std/async", false},
		{"hackage", NewHackage(testHTTP(), ""), "Aeson", "aeson", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.checker.Normalize(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPyPI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/pypi/requests/json" {
			_, _ = w.Write([]byte(`{"info":{"name":"requests","version":"2.32.0","summary":"HTTP","author":"Kenneth"}}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	p := NewPyPI(testHTTP(), srv.URL)
	res, err := p.Check(context.Background(), "requests")
	if err != nil || res.Status != check.StatusTaken || res.Summary != "v2.32.0" {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}

func TestCrates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/crates/serde":
			_, _ = w.Write([]byte(`{"crate":{"name":"serde","max_version":"1.0.200","downloads":500000000,"description":"serialization"}}`))
		case "/api/v1/crates/foo-bar":
			_, _ = w.Write([]byte(`{"crate":{"name":"foo-bar","max_version":"1.0.0","downloads":1}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := NewCrates(testHTTP(), srv.URL)
	res, err := c.Check(context.Background(), "serde")
	if err != nil || res.Status != check.StatusTaken {
		t.Fatalf("res=%+v err=%v", res, err)
	}

	// Exact name free, underscore variant taken: still available with a note.
	res, err = c.Check(context.Background(), "foo_bar")
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != check.StatusAvailable {
		t.Fatalf("status = %v", res.Status)
	}
	if !strings.Contains(res.Field("note"), "foo-bar") {
		t.Fatalf("note = %q", res.Field("note"))
	}
}

func TestGitHubAPIToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("missing auth header: %q", r.Header.Get("Authorization"))
		}
		if r.URL.Path == "/users/acme" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"login": "acme", "type": "Organization", "public_repos": 12, "followers": 3,
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	g := NewGitHub(testHTTP(), srv.URL, "tok")
	res, err := g.Check(context.Background(), "acme")
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != check.StatusTaken || !strings.Contains(res.Summary, "organization") {
		t.Fatalf("res=%+v", res)
	}
}

func TestRateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "2")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	n := NewNPM(testHTTP(), srv.URL)
	_, err := n.Check(context.Background(), "react")
	if err == nil {
		t.Fatal("expected error")
	}
	if !check.Retryable(err) {
		t.Fatal("429 should be retryable")
	}
}

func TestNuGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v3-flatcontainer/newtonsoft.json/index.json" {
			_, _ = w.Write([]byte(`{"versions":["13.0.1","13.0.3"]}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	n := NewNuGet(testHTTP(), srv.URL)
	res, err := n.Check(context.Background(), "newtonsoft.json")
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != check.StatusTaken || res.Summary != "v13.0.3" {
		t.Fatalf("res=%+v", res)
	}
}

func TestMaven(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/solrsearch/select" {
			http.NotFound(w, r)
			return
		}
		q := r.URL.Query().Get("q")
		if q == `a:"guava"` {
			_, _ = w.Write([]byte(`{"response":{"numFound":1,"docs":[{"id":"com.google.guava:guava","g":"com.google.guava","a":"guava","latestVersion":"33.4.8-jre","versionCount":150}]}}`))
			return
		}
		_, _ = w.Write([]byte(`{"response":{"numFound":0,"docs":[]}}`))
	}))
	defer srv.Close()

	m := NewMaven(testHTTP(), srv.URL)
	res, err := m.Check(context.Background(), "guava")
	if err != nil || res.Status != check.StatusTaken || res.Summary != "v33.4.8-jre" {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	res, err = m.Check(context.Background(), "zzqxjvnamearytest")
	if err != nil || res.Status != check.StatusAvailable {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}

func TestConda(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("name") == "acme" {
			_, _ = w.Write([]byte(`[{"name":"acme-extra"},{"name":"acme","owner":"cjrh","full_name":"cjrh/acme","html_url":"https://anaconda.org/cjrh/acme"},{"name":"acme","owner":"conda-forge","full_name":"conda-forge/acme","html_url":"https://anaconda.org/conda-forge/acme"}]`))
			return
		}
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := NewConda(testHTTP(), srv.URL)
	res, err := c.Check(context.Background(), "acme")
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != check.StatusTaken || res.Field("owner") != "conda-forge" {
		t.Fatalf("res=%+v", res)
	}
	res, err = c.Check(context.Background(), "zzqxjvnamearytest")
	if err != nil || res.Status != check.StatusAvailable {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}

func TestCocoaPods(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			t.Errorf("method=%s", r.Method)
		}
		if r.URL.Path == "/pods/AFNetworking" {
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	c := NewCocoaPods(testHTTP(), srv.URL)
	res, err := c.Check(context.Background(), "AFNetworking")
	if err != nil || res.Status != check.StatusTaken {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	res, err = c.Check(context.Background(), "zzqxjvnamearytest")
	if err != nil || res.Status != check.StatusAvailable {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}

func TestAUR(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("arg[]") == "yay" {
			_, _ = w.Write([]byte(`{"resultcount":1,"results":[{"Name":"yay","Version":"12.5.7","Description":"AUR helper","NumVotes":2300,"Popularity":30.5}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"resultcount":0,"results":[]}`))
	}))
	defer srv.Close()

	a := NewAUR(testHTTP(), srv.URL)
	res, err := a.Check(context.Background(), "yay")
	if err != nil || res.Status != check.StatusTaken || res.Summary != "v12.5.7" {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	res, err = a.Check(context.Background(), "zzqxjvnamearytest")
	if err != nil || res.Status != check.StatusAvailable {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}

func TestSnap(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Snap-Device-Series") != "16" {
			t.Errorf("missing snap header")
		}
		if r.URL.Path == "/v2/snaps/info/hello" {
			_, _ = w.Write([]byte(`{"name":"hello","snap-id":"abc","channel-map":[{"channel":{"track":"latest","risk":"stable"},"version":"2.10"}]}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error-list":[]}`))
	}))
	defer srv.Close()

	s := NewSnap(testHTTP(), srv.URL)
	res, err := s.Check(context.Background(), "hello")
	if err != nil || res.Status != check.StatusTaken || res.Summary != "v2.10" {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	res, err = s.Check(context.Background(), "zzqxjvnamearytest")
	if err != nil || res.Status != check.StatusAvailable {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}

func TestChocolatey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "application/atom+xml" {
			t.Errorf("accept=%q", r.Header.Get("Accept"))
		}
		if strings.Contains(r.URL.RawQuery, "git") {
			_, _ = w.Write([]byte(`<feed><entry><title>git</title><summary>Git for Windows</summary><properties><Version>2.51.0</Version><DownloadCount>16000000</DownloadCount></properties></entry></feed>`))
			return
		}
		_, _ = w.Write([]byte(`<feed></feed>`))
	}))
	defer srv.Close()

	c := NewChocolatey(testHTTP(), srv.URL)
	res, err := c.Check(context.Background(), "git")
	if err != nil || res.Status != check.StatusTaken || res.Summary != "v2.51.0" {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	res, err = c.Check(context.Background(), "zzqxjvnamearytest")
	if err != nil || res.Status != check.StatusAvailable {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}

func TestHackage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			t.Errorf("method=%s", r.Method)
		}
		if r.URL.Path == "/package/aeson" {
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	h := NewHackage(testHTTP(), srv.URL)
	res, err := h.Check(context.Background(), "aeson")
	if err != nil || res.Status != check.StatusTaken {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}

func TestJSR(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/packages":
			if r.URL.Query().Get("query") == "acme" {
				_, _ = w.Write([]byte(`{"items":[{"scope":"fishballpkg","name":"acme","description":"ACME client"},{"scope":"other","name":"acme-test"}]}`))
			} else {
				_, _ = w.Write([]byte(`{"items":[]}`))
			}
		case "/@std/async/meta.json":
			_, _ = w.Write([]byte(`{"scope":"std","name":"async","latest":"1.5.1","githubRepository":{"owner":"denoland","name":"std"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	j := NewJSR(testHTTP(), srv.URL)
	res, err := j.Check(context.Background(), "acme")
	if err != nil || res.Status != check.StatusTaken {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	res, err = j.Check(context.Background(), "@std/async")
	if err != nil || res.Status != check.StatusTaken || res.Summary != "v1.5.1" {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}

func TestPackagist(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/p2/laravel/framework.json" {
			_, _ = w.Write([]byte(`{"packages":{"laravel/framework":[{"name":"laravel/framework","description":"The Laravel Framework.","version":"12.0.0"}]}}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	p := NewPackagist(testHTTP(), srv.URL)
	res, err := p.Check(context.Background(), "laravel/framework")
	if err != nil || res.Status != check.StatusTaken || res.Summary != "v12.0.0" {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}
