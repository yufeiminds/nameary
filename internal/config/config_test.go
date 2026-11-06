package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("NAMEARY_CONFIG", filepath.Join(t.TempDir(), "missing.toml"))
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Preset != "all" || len(cfg.Targets) != len(Presets["all"].Targets) || len(cfg.TLDs) == 0 || cfg.Timeout != "3s" {
		t.Fatalf("cfg=%+v", cfg)
	}
}

func TestSaveLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	t.Setenv("NAMEARY_CONFIG", path)
	cfg := Defaults()
	cfg.Preset = "rust"
	cfg.Targets = Presets["rust"].Targets
	cfg.TLDs = Presets["rust"].TLDs
	cfg.Concurrency = 8
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Preset != "rust" || got.Concurrency != 8 || len(got.Targets) != 3 {
		t.Fatalf("got=%+v", got)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func TestParseDuration(t *testing.T) {
	if got := ParseDuration("250ms", 0); got.String() != "250ms" {
		t.Fatalf("got=%v", got)
	}
	if got := ParseDuration("nope", 3*time.Second); got != 3*time.Second {
		t.Fatalf("got=%v", got)
	}
}

func TestPresetNamesAndDefaults(t *testing.T) {
	names := PresetNames()
	if len(names) < 5 {
		t.Fatalf("presets=%v", names)
	}
	for _, n := range names {
		if _, ok := Presets[n]; !ok {
			t.Fatalf("missing preset %s", n)
		}
	}
	if Defaults().Preset != "all" {
		t.Fatal("default preset")
	}
}

func TestPathHonorsEnv(t *testing.T) {
	want := filepath.Join(t.TempDir(), "custom.toml")
	t.Setenv("NAMEARY_CONFIG", want)
	if got := Path(); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestLoadInvalidTOML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	t.Setenv("NAMEARY_CONFIG", path)
	if err := os.WriteFile(path, []byte("this = [is not toml"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestLoadAppliesPresetDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	t.Setenv("NAMEARY_CONFIG", path)
	if err := os.WriteFile(path, []byte("preset = \"rust\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Targets) != len(Presets["rust"].Targets) || len(cfg.TLDs) == 0 {
		t.Fatalf("cfg=%+v", cfg)
	}
}

func TestLoadUnknownPresetWithTargets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	t.Setenv("NAMEARY_CONFIG", path)
	if err := os.WriteFile(path, []byte("preset = \"custom\"\ntargets = [\"npm\"]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Targets) != 1 || cfg.Targets[0] != "npm" {
		t.Fatalf("cfg=%+v", cfg)
	}
}

func TestSavePermissionsAndRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	t.Setenv("NAMEARY_CONFIG", path)
	cfg := Defaults()
	cfg.Concurrency = 7
	cfg.Preset = "custom"
	cfg.Targets = nil
	cfg.TLDs = nil
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("mode=%o", info.Mode().Perm())
	}
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Concurrency != 7 || len(got.Targets) == 0 || len(got.TLDs) == 0 {
		t.Fatalf("got=%+v", got)
	}
}

func TestSaveFailure(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("NAMEARY_CONFIG", filepath.Join(blocker, "config.toml"))
	if err := Save(Defaults()); err == nil {
		t.Fatal("expected mkdir failure")
	}
}

func TestPathFallback(t *testing.T) {
	t.Setenv("NAMEARY_CONFIG", "")
	t.Setenv("HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	got := Path()
	if got == "" || !strings.Contains(got, "nameary") {
		t.Fatalf("path=%q", got)
	}
}

func TestSaveCreateTempFailure(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NAMEARY_CONFIG", filepath.Join(dir, "config.toml"))
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(dir, 0o700)
	if err := Save(Defaults()); err == nil {
		t.Skip("directory permissions are not enforced in this environment")
	}
}
