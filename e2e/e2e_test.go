// Package e2e drives the compiled nameary binary against a deterministic mock
// registry. It covers every adapter, the renderers, exit codes and the cache
// without touching the real internet.
package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
)

const (
	modeTaken int32 = iota
	modeFree
	modeRateLimit
)

var (
	binary    string
	configDir string
	baseURL   string
	mode      atomic.Int32
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "nameary-e2e-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer os.RemoveAll(dir)
	configDir = dir

	binary = filepath.Join(dir, "nameary")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	build := exec.Command("go", "build", "-tags", "e2e", "-o", binary, "../cmd/nameary")
	build.Stdout, build.Stderr = os.Stdout, os.Stderr
	if err := build.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "e2e: build failed:", err)
		os.Exit(1)
	}

	srv := httptest.NewServer(http.HandlerFunc(mockRegistry))
	defer srv.Close()
	baseURL = srv.URL

	os.Exit(m.Run())
}

func TestEveryAdapterTaken(t *testing.T) {
	mode.Store(modeTaken)
	for _, tc := range adapters() {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, code := runCLI(t, "check", tc.query, "--targets", tc.target, "--format", "ndjson", "--cache-mode", "off")
			if code != 0 {
				t.Fatalf("exit=%d stderr=%s", code, stderr)
			}
			rec := oneRecord(t, stdout)
			if rec.Status != "taken" {
				t.Fatalf("status=%q record=%+v", rec.Status, rec)
			}
			if rec.Target != tc.target {
				t.Fatalf("target=%q want %q", rec.Target, tc.target)
			}
			if rec.URL == "" {
				t.Fatalf("missing source url: %+v", rec)
			}
		})
	}
}

func TestEveryAdapterAvailable(t *testing.T) {
	mode.Store(modeFree)
	for _, tc := range adapters() {
		t.Run(tc.name, func(t *testing.T) {
			q := tc.query
			if tc.target == "domains" {
				q = "acme.invalid"
			}
			stdout, stderr, code := runCLI(t, "check", q, "--targets", tc.target, "--format", "ndjson", "--cache-mode", "off")
			if code != 0 {
				t.Fatalf("exit=%d stderr=%s", code, stderr)
			}
			rec := oneRecord(t, stdout)
			if rec.Status != "available" {
				t.Fatalf("status=%q record=%+v", rec.Status, rec)
			}
			if rec.Error != "" {
				t.Fatalf("unexpected error: %q", rec.Error)
			}
		})
	}
}

func TestFanoutAcrossEveryTarget(t *testing.T) {
	mode.Store(modeTaken)
	stdout, stderr, code := runCLI(t, "check", "acme", "--preset", "all", "--format", "ndjson", "--cache-mode", "off", "--timeout", "10s")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr)
	}
	rows := records(t, stdout)
	if len(rows) != 25 {
		t.Fatalf("rows=%d want 25", len(rows))
	}
	seen := map[string]bool{}
	for _, r := range rows {
		seen[r.Target] = true
		if r.Status == "error" || r.Status == "skipped" {
			t.Fatalf("engine failure: %+v", r)
		}
	}
	for _, tc := range adapters() {
		if !seen[tc.target] {
			t.Fatalf("target %s missing from fan-out", tc.target)
		}
	}
}

func TestJSONAndConsoleOutput(t *testing.T) {
	mode.Store(modeTaken)
	stdout, stderr, code := runCLI(t, "check", "acme", "--targets", "npm", "--format", "json", "--cache-mode", "off")
	if code != 0 {
		t.Fatalf("json exit=%d stderr=%s", code, stderr)
	}
	var arr []record
	if err := json.Unmarshal([]byte(stdout), &arr); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, stdout)
	}
	if len(arr) != 1 || arr[0].Status != "taken" {
		t.Fatalf("json rows=%+v", arr)
	}

	stdout, stderr, code = runCLI(t, "check", "acme", "--targets", "domains,npm", "--tlds", "invalid", "--format", "console", "--style", "plain", "--cache-mode", "off")
	if code != 0 {
		t.Fatalf("console exit=%d stderr=%s", code, stderr)
	}
	for _, want := range []string{"DOMAINS", "PACKAGES", "acme.invalid", "npm"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("console output missing %q:\n%s", want, stdout)
		}
	}
}

func TestExitCodes(t *testing.T) {
	mode.Store(modeTaken)
	_, _, code := runCLI(t, "check", "acme", "--targets", "npm", "--format", "ndjson", "--cache-mode", "off", "--fail-if-taken")
	if code != 3 {
		t.Fatalf("taken exit=%d want 3", code)
	}

	_, stderr, code := runCLI(t, "check", "acme", "--targets", "not-a-target", "--format", "ndjson", "--cache-mode", "off")
	if code != 1 || !strings.Contains(stderr, "unknown target") {
		t.Fatalf("bad target exit=%d stderr=%q", code, stderr)
	}

	_, _, code = runCLI(t, "check")
	if code != 2 {
		t.Fatalf("missing name exit=%d want 2", code)
	}

	stdout, stderr, code := runCLI(t, "version")
	if code != 0 || !strings.HasPrefix(stdout, "nameary ") {
		t.Fatalf("version exit=%d out=%q err=%q", code, stdout, stderr)
	}
}

func TestRateLimitBecomesError(t *testing.T) {
	mode.Store(modeRateLimit)
	stdout, stderr, code := runCLI(t, "check", "acme", "--targets", "npm", "--format", "ndjson", "--cache-mode", "off", "--timeout", "2s")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr)
	}
	rec := oneRecord(t, stdout)
	if rec.Status != "rate_limit" || rec.Error == "" {
		t.Fatalf("rate limit should surface as rate_limit with an error: %+v", rec)
	}
}

func TestCacheRoundTrip(t *testing.T) {
	mode.Store(modeTaken)
	cachePath := filepath.Join(t.TempDir(), "cache.db")
	first, stderr, code := runCLI(t, "check", "acme", "--targets", "npm", "--format", "ndjson", "--cache", cachePath)
	if code != 0 {
		t.Fatalf("first exit=%d stderr=%s", code, stderr)
	}
	if r := oneRecord(t, first); r.Cached {
		t.Fatalf("first run should not be cached: %+v", r)
	}
	second, stderr, code := runCLI(t, "check", "acme", "--targets", "npm", "--format", "ndjson", "--cache", cachePath)
	if code != 0 {
		t.Fatalf("second exit=%d stderr=%s", code, stderr)
	}
	if r := oneRecord(t, second); !r.Cached {
		t.Fatalf("second run should come from cache: %+v", r)
	}
}

type adapterCase struct {
	name   string
	target string
	query  string
}

func adapters() []adapterCase {
	return []adapterCase{
		{"domains", "domains", "acme.invalid"},
		{"npm", "npm", "acme"},
		{"pypi", "pypi", "acme"},
		{"crates", "crates", "acme"},
		{"rubygems", "rubygems", "acme"},
		{"nuget", "nuget", "acme"},
		{"dockerhub", "dockerhub", "acme"},
		{"homebrew", "homebrew", "acme"},
		{"packagist", "packagist", "acme"},
		{"hex", "hex", "acme"},
		{"pub", "pub", "acme"},
		{"github", "github", "acme"},
		{"maven", "maven", "acme"},
		{"conda", "conda", "acme"},
		{"cocoapods", "cocoapods", "acme"},
		{"aur", "aur", "acme"},
		{"snap", "snap", "acme"},
		{"chocolatey", "chocolatey", "acme"},
		{"hackage", "hackage", "acme"},
		{"jsr", "jsr", "acme"},
	}
}

type record struct {
	Name       string `json:"name"`
	Target     string `json:"target"`
	Label      string `json:"label"`
	Status     string `json:"status"`
	Confidence string `json:"confidence"`
	Query      string `json:"query"`
	Summary    string `json:"summary"`
	URL        string `json:"url"`
	LatencyMS  int64  `json:"latency_ms"`
	Cached     bool   `json:"cached"`
	Error      string `json:"error"`
	Fields     []struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	} `json:"fields"`
}

func oneRecord(t *testing.T, out string) record {
	t.Helper()
	rows := records(t, out)
	if len(rows) != 1 {
		t.Fatalf("rows=%d want 1:\n%s", len(rows), out)
	}
	return rows[0]
}

func records(t *testing.T, out string) []record {
	t.Helper()
	var rows []record
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var r record
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatalf("bad ndjson line %q: %v", line, err)
		}
		rows = append(rows, r)
	}
	return rows
}

func runCLI(t *testing.T, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Env = append(os.Environ(),
		"NAMEARY_CONFIG="+filepath.Join(configDir, "missing.toml"),
		"NAMEARY_TEST_BASE_URL="+baseURL,
		"GITHUB_TOKEN=e2e-token",
		"NO_COLOR=1",
		"TERM=dumb",
		"NO_PROXY=*",
		"HTTP_PROXY=",
		"HTTPS_PROXY=",
		"ALL_PROXY=",
	)
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err := cmd.Run()
	code = 0
	if err != nil {
		var ee *exec.ExitError
		if !asExitError(err, &ee) {
			t.Fatalf("run %v: %v", args, err)
		}
		code = ee.ExitCode()
	}
	return out.String(), errOut.String(), code
}

func asExitError(err error, target **exec.ExitError) bool {
	ee, ok := err.(*exec.ExitError)
	if ok {
		*target = ee
	}
	return ok
}

func mockRegistry(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	query := r.URL.Query()

	// RDAP bootstrap. Every TLD used by the e2e suite maps back to this server.
	if path == "/" {
		_, _ = fmt.Fprintf(w, `{"services":[[["com","io","dev","app","ai","co","invalid"],["%s/rdap/"]]]}`, baseURL)
		return
	}
	if strings.HasPrefix(path, "/rdap/domain/") {
		if mode.Load() == modeFree {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("no registration record"))
			return
		}
		writeJSON(w, `{"ldhName":"ACME.INVALID","status":["active"],"events":[{"eventAction":"registration","eventDate":"2020-01-02T00:00:00Z"}],"entities":[{"roles":["registrar"],"vcardArray":["vcard",[["fn",{},"text","Mock Registrar"]]]}],"nameservers":[{"ldhName":"NS1.ACME.INVALID"}]}`)
		return
	}

	switch mode.Load() {
	case modeRateLimit:
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusTooManyRequests)
		return
	case modeFree:
		// Endpoints that encode "nothing found" in a 200 response.
		switch {
		case path == "/solrsearch/select":
			writeJSON(w, `{"response":{"numFound":0,"docs":[]}}`)
		case path == "/search":
			writeJSON(w, `[]`)
		case path == "/api/packages" && query.Get("query") != "":
			writeJSON(w, `{"items":[]}`)
		case path == "/api/v2/Packages":
			writeJSON(w, `<feed xmlns="http://www.w3.org/2005/Atom"></feed>`)
		case path == "/rpc/v5/info":
			writeJSON(w, `{"resultcount":0,"results":[]}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
		return
	}

	switch {
	case path == "/acme/latest":
		writeJSON(w, `{"name":"acme","version":"1.2.3","description":"mock package"}`)
	case path == "/pypi/acme/json":
		writeJSON(w, `{"info":{"name":"acme","version":"1.2.3","summary":"mock","author":"e2e","project_url":"https://example.test"}}`)
	case path == "/api/v1/crates/acme":
		writeJSON(w, `{"crate":{"name":"acme","max_version":"1.2.3","downloads":42,"description":"mock","updated_at":"2026-01-02T00:00:00Z"}}`)
	case path == "/api/v1/gems/acme.json":
		writeJSON(w, `{"name":"acme","version":"1.2.3","downloads":42,"info":"mock","authors":"e2e"}`)
	case path == "/v3-flatcontainer/acme/index.json":
		writeJSON(w, `{"versions":["1.0.0","1.2.3"]}`)
	case strings.HasPrefix(path, "/v2/users/"):
		writeJSON(w, `{"name":"acme"}`)
	case path == "/api/formula/acme.json":
		writeJSON(w, `{"name":"acme","desc":"mock","versions":{"stable":"1.2.3"}}`)
	case path == "/p2/acme/acme.json":
		writeJSON(w, `{"packages":{"acme/acme":[{"name":"acme/acme","description":"mock","version":"1.2.3"}]}}`)
	case path == "/api/packages/acme" && query.Get("query") == "":
		writeJSON(w, `{"name":"acme","latest_version":"1.2.3","downloads":{"all":42},"meta":{"description":"mock"}}`)
	case path == "/users/acme":
		writeJSON(w, `{"login":"acme","type":"User","name":"Acme","bio":"mock","public_repos":7,"followers":3,"created_at":"2020-01-02T00:00:00Z"}`)
	case path == "/solrsearch/select":
		writeJSON(w, `{"response":{"numFound":1,"docs":[{"id":"com.acme:acme","g":"com.acme","a":"acme","latestVersion":"1.2.3","versionCount":4}]}}`)
	case path == "/search":
		writeJSON(w, `[{"name":"acme","owner":"conda-forge","full_name":"conda-forge/acme","html_url":"https://anaconda.org/conda-forge/acme","latest_version":"1.2.3","versions":["1.2.3"]}]`)
	case path == "/pods/acme":
		w.WriteHeader(http.StatusNotFound)
	case path == "/pods/Acme":
		w.WriteHeader(http.StatusOK)
	case path == "/rpc/v5/info":
		writeJSON(w, `{"resultcount":1,"results":[{"Name":"acme","Version":"1.2.3","Description":"mock","NumVotes":2,"Popularity":1.5}]}`)
	case path == "/v2/snaps/info/acme":
		writeJSON(w, `{"name":"acme","snap-id":"mock","channel-map":[{"channel":{"track":"latest","risk":"stable"},"version":"1.2.3"}]}`)
	case path == "/api/v2/Packages":
		writeJSON(w, `<feed xmlns="http://www.w3.org/2005/Atom"><entry><title>acme</title><summary>mock</summary><properties><Version>1.2.3</Version><DownloadCount>42</DownloadCount></properties></entry></feed>`)
	case path == "/package/acme":
		w.WriteHeader(http.StatusOK)
	case path == "/api/packages" && query.Get("query") != "":
		writeJSON(w, `{"items":[{"scope":"std","name":"acme","latest":"1.2.3","description":"mock","githubRepository":{"owner":"e2e","name":"acme"}}]}`)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func writeJSON(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(body))
}
