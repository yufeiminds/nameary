package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/yufeiminds/nameary/internal/config"
)

func tempConfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	t.Setenv("NAMEARY_CONFIG", path)
	return path
}

func rootWith(t *testing.T, args ...string) *cobra.Command {
	t.Helper()
	root := newRoot()
	root.SetArgs(args)
	return root
}

func TestArgError(t *testing.T) {
	err := argError([]string{"--bad"}, "check needs")
	var ee *exitError
	if !asExit(err, &ee) || ee.code != 2 || !strings.Contains(ee.Error(), "flags must come before") {
		t.Fatalf("err=%v", err)
	}
	err = argError([]string{"a", "b"}, "accepts at most 1 name")
	if !asExit(err, &ee) || ee.code != 2 || !strings.Contains(ee.Error(), "received 2") {
		t.Fatalf("err=%v", err)
	}
	if ee.Error() == "" {
		t.Fatal("empty error")
	}
	empty := &exitError{code: 4}
	if empty.Error() != "exit 4" {
		t.Fatalf("got %q", empty.Error())
	}
}

func asExit(err error, target **exitError) bool {
	e, ok := err.(*exitError)
	if ok {
		*target = e
	}
	return ok
}

func TestVersionString(t *testing.T) {
	if !strings.HasPrefix(versionString(), "nameary ") {
		t.Fatalf("got %q", versionString())
	}
}

func TestNewRootFlagsAndCommands(t *testing.T) {
	root := newRoot()
	for _, name := range []string{
		"preset", "targets", "tlds", "concurrency", "timeout", "format", "style",
		"only-available", "cache-mode", "cache", "cache-ttl", "celebrate",
		"fail-if-taken", "github-token", "debug",
	} {
		if root.Flags().Lookup(name) == nil {
			t.Fatalf("missing flag %s", name)
		}
	}
	names := map[string]bool{}
	for _, c := range root.Commands() {
		names[c.Name()] = true
	}
	for _, want := range []string{"check", "version", "config"} {
		if !names[want] {
			t.Fatalf("missing command %s in %v", want, names)
		}
	}
	var configCmd *cobra.Command
	for _, c := range root.Commands() {
		if c.Name() == "config" {
			configCmd = c
		}
	}
	subs := map[string]bool{}
	for _, c := range configCmd.Commands() {
		subs[c.Name()] = true
	}
	if !subs["path"] || !subs["init"] {
		t.Fatalf("config subcommands=%v", subs)
	}
}

func TestResolveOptionsDefaults(t *testing.T) {
	tempConfig(t)
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	root := newRoot()
	opts, err := resolveOptions(root)
	if err != nil {
		t.Fatal(err)
	}
	if opts.format != "auto" || opts.Concurrency != 16 || opts.NoCache || opts.Refresh {
		t.Fatalf("opts=%+v", opts)
	}
	if len(opts.Targets) != len(config.Presets["indie"].Targets) || len(opts.TLDs) == 0 {
		t.Fatalf("targets=%v tlds=%v", opts.Targets, opts.TLDs)
	}
}

func TestResolveOptionsPresetAndFlags(t *testing.T) {
	tempConfig(t)
	root := newRoot()
	for name, value := range map[string]string{
		"preset": "rust", "targets": "npm,pypi", "tlds": "com,io", "concurrency": "3",
		"timeout": "2s", "format": "json", "style": "ascii", "cache-mode": "off",
		"cache": "/tmp/nameary-test.db", "cache-ttl": "3h", "celebrate": "true",
		"only-available": "true", "fail-if-taken": "true", "github-token": "tok",
	} {
		if err := root.Flags().Set(name, value); err != nil {
			t.Fatalf("set %s: %v", name, err)
		}
	}
	opts, err := resolveOptions(root)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(opts.Targets, ",") != "npm,pypi" || strings.Join(opts.TLDs, ",") != "com,io" {
		t.Fatalf("targets=%v tlds=%v", opts.Targets, opts.TLDs)
	}
	if opts.Concurrency != 3 || opts.Timeout != 2*time.Second || opts.format != "json" {
		t.Fatalf("opts=%+v", opts)
	}
	if !opts.ASCII || !opts.NoCache || !opts.Celebrate || !opts.onlyAvailable || !opts.failIfTaken {
		t.Fatalf("opts=%+v", opts)
	}
	if opts.CachePath != "/tmp/nameary-test.db" || opts.CacheTTL != 3*time.Hour || opts.GithubToken != "tok" {
		t.Fatalf("opts=%+v", opts)
	}
}

func TestResolveOptionsPresetOnly(t *testing.T) {
	tempConfig(t)
	root := newRoot()
	if err := root.Flags().Set("preset", "rust"); err != nil {
		t.Fatal(err)
	}
	opts, err := resolveOptions(root)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(opts.Targets, ",") != strings.Join(config.Presets["rust"].Targets, ",") {
		t.Fatalf("targets=%v", opts.Targets)
	}
	if strings.Join(opts.TLDs, ",") != strings.Join(config.Presets["rust"].TLDs, ",") {
		t.Fatalf("tlds=%v", opts.TLDs)
	}
}

func TestResolveOptionsErrors(t *testing.T) {
	tempConfig(t)
	for _, tc := range []struct {
		flag, value string
	}{
		{"preset", "nope"},
		{"style", "loud"},
		{"cache-mode", "sometimes"},
	} {
		root := newRoot()
		if err := root.Flags().Set(tc.flag, tc.value); err != nil {
			t.Fatal(err)
		}
		if _, err := resolveOptions(root); err == nil {
			t.Fatalf("%s=%s should fail", tc.flag, tc.value)
		}
	}

	root := newRoot()
	t.Setenv("GITHUB_TOKEN", "bad\ntoken")
	if _, err := resolveOptions(root); err == nil {
		t.Fatal("control characters in token should fail")
	}
}

func TestResolveOptionsStylesAndCacheModes(t *testing.T) {
	tempConfig(t)
	for style, want := range map[string]struct {
		ascii, reduced bool
	}{
		"plain": {true, true},
		"ascii": {true, false},
		"mono":  {false, false},
		"calm":  {false, true},
		"":      {false, false},
		"auto":  {false, false},
	} {
		root := newRoot()
		if err := root.Flags().Set("style", style); err != nil {
			t.Fatal(err)
		}
		opts, err := resolveOptions(root)
		if err != nil {
			t.Fatalf("%q: %v", style, err)
		}
		if opts.ASCII != want.ascii || opts.ReducedMotion != want.reduced {
			t.Fatalf("%q: %+v", style, opts)
		}
	}
	for mode, want := range map[string]struct{ noCache, refresh bool }{
		"off":     {true, false},
		"refresh": {false, true},
		"auto":    {false, false},
		"":        {false, false},
	} {
		root := newRoot()
		if err := root.Flags().Set("cache-mode", mode); err != nil {
			t.Fatal(err)
		}
		opts, err := resolveOptions(root)
		if err != nil {
			t.Fatalf("%q: %v", mode, err)
		}
		if opts.NoCache != want.noCache || opts.Refresh != want.refresh {
			t.Fatalf("%q: %+v", mode, opts)
		}
	}
}

func TestResolveOptionsTokenEnv(t *testing.T) {
	path := tempConfig(t)
	if err := os.WriteFile(path, []byte("[github]\ntoken_env = \"MY_TOKEN\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MY_TOKEN", "from-env")
	opts, err := resolveOptions(newRoot())
	if err != nil {
		t.Fatal(err)
	}
	if opts.GithubToken != "from-env" {
		t.Fatalf("token=%q", opts.GithubToken)
	}
}

func TestConfigInit(t *testing.T) {
	path := tempConfig(t)
	if err := runConfigInit(false, "rust", nil, nil); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Preset != "rust" || len(cfg.Targets) != len(config.Presets["rust"].Targets) {
		t.Fatalf("cfg=%+v", cfg)
	}
	if err := runConfigInit(false, "", nil, nil); err == nil {
		t.Fatal("expected existing config error")
	}
	if err := runConfigInit(true, "", nil, nil); err != nil {
		t.Fatalf("force overwrite: %v", err)
	}
	if err := runConfigInit(true, "nope", nil, nil); err == nil {
		t.Fatal("expected unknown preset error")
	}
	if err := runConfigInit(true, "", []string{"npm"}, []string{"io"}); err != nil {
		t.Fatal(err)
	}
	cfg, err = config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Targets) != 1 || cfg.Targets[0] != "npm" || len(cfg.TLDs) != 1 || cfg.TLDs[0] != "io" {
		t.Fatalf("cfg=%+v", cfg)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func TestConfigInitSaveFailure(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("NAMEARY_CONFIG", filepath.Join(blocker, "config.toml"))
	if err := runConfigInit(true, "", nil, nil); err == nil {
		t.Fatal("expected save failure")
	}
}

func TestSetupDebug(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	setupDebug()
	if dir, err := os.UserCacheDir(); err == nil {
		if _, statErr := os.Stat(filepath.Join(dir, "nameary", "debug.log")); statErr != nil {
			t.Fatalf("debug log missing: %v", statErr)
		}
	}
}

func TestIsTTYSmoke(t *testing.T) {
	_ = isTTY()
}

func TestRootCommandBasics(t *testing.T) {
	tempConfig(t)
	root := rootWith(t, "version")
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}

	root = rootWith(t, "config", "path")
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}

	root = rootWith(t, "config", "init", "--force")
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
}

func TestRootArgValidation(t *testing.T) {
	tempConfig(t)
	root := rootWith(t, "a", "b")
	err := root.Execute()
	var ee *exitError
	if !asExit(err, &ee) || ee.code != 2 {
		t.Fatalf("err=%v", err)
	}

	root = rootWith(t, "check")
	err = root.Execute()
	if !asExit(err, &ee) || ee.code != 2 {
		t.Fatalf("err=%v", err)
	}
}

func TestRunRejectsBadFormatsBeforeNetwork(t *testing.T) {
	tempConfig(t)
	for _, args := range [][]string{
		{"acme", "--format", "bogus"},
		{"--format", "tui", "acme"},
		{"--format", "json"},
	} {
		root := rootWith(t, args...)
		err := root.Execute()
		var ee *exitError
		if err == nil || !asExit(err, &ee) || ee.code != 2 {
			t.Fatalf("args=%v err=%v", args, err)
		}
	}
}

func TestExecuteCodes(t *testing.T) {
	tempConfig(t)
	t.Setenv("TERM", "dumb")
	if code := Execute(); code != 2 {
		t.Fatalf("no args: code=%d", code)
	}

	root := newRoot()
	_ = root
	if code := Execute(); code != 2 {
		t.Fatalf("again: code=%d", code)
	}
}

func TestRunNameRequiredFormats(t *testing.T) {
	tempConfig(t)
	for _, format := range []string{"console", "table"} {
		root := rootWith(t, "--format", format)
		err := root.Execute()
		var ee *exitError
		if err == nil || !asExit(err, &ee) || ee.code != 2 {
			t.Fatalf("format=%s err=%v", format, err)
		}
	}
}
