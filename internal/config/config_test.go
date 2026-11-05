package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("NAMEARY_CONFIG", filepath.Join(t.TempDir(), "missing.toml"))
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Preset != "indie" || len(cfg.Targets) == 0 || len(cfg.TLDs) == 0 {
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
	if got := ParseDuration("nope", 3); got != 3 {
		t.Fatalf("got=%v", got)
	}
}
