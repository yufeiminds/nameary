// Package config loads and writes nameary's TOML configuration.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
)

// Preset is a named set of targets and TLDs.
type Preset struct {
	Targets []string
	TLDs    []string
}

// Presets are intentionally opinionated: all sources is the default; use
// indie for a fast subset.
var Presets = map[string]Preset{
	"indie": {
		Targets: []string{"domains", "npm", "pypi", "github"},
		TLDs:    []string{"com", "io", "dev", "app", "ai", "co"},
	},
	"web": {
		Targets: []string{"domains", "npm", "pypi", "github", "jsr"},
		TLDs:    []string{"com", "io", "dev", "app"},
	},
	"go": {
		Targets: []string{"domains", "github", "homebrew"},
		TLDs:    []string{"com", "io", "dev"},
	},
	"rust": {
		Targets: []string{"domains", "crates", "github"},
		TLDs:    []string{"com", "io", "dev"},
	},
	"jvm": {
		Targets: []string{"domains", "maven", "github"},
		TLDs:    []string{"com", "io", "dev"},
	},
	"apple": {
		Targets: []string{"domains", "cocoapods", "github"},
		TLDs:    []string{"com", "io", "dev"},
	},
	"linux": {
		Targets: []string{"domains", "aur", "snap", "homebrew", "github"},
		TLDs:    []string{"com", "io", "dev"},
	},
	"data": {
		Targets: []string{"domains", "conda", "pypi", "github"},
		TLDs:    []string{"com", "io", "dev"},
	},
	"all": {
		Targets: []string{"domains", "npm", "pypi", "crates", "rubygems", "nuget", "dockerhub", "homebrew", "packagist", "hex", "pub", "github", "maven", "conda", "cocoapods", "aur", "snap", "chocolatey", "hackage", "jsr"},
		TLDs:    []string{"com", "io", "dev", "app", "ai", "co"},
	},
}

// PresetNames returns the stable list used in help and forms.
func PresetNames() []string {
	return []string{"indie", "web", "go", "rust", "jvm", "apple", "linux", "data", "all"}
}

// Config mirrors the TOML file.
type Config struct {
	Preset      string   `toml:"preset"`
	TLDs        []string `toml:"tlds"`
	Targets     []string `toml:"targets"`
	Concurrency int      `toml:"concurrency"`
	Timeout     string   `toml:"timeout,omitempty"`
	CacheTTL    string   `toml:"cache_ttl,omitempty"`
	Celebrate   bool     `toml:"celebrate"`

	Github struct {
		TokenEnv string `toml:"token_env"`
	} `toml:"github"`
}

// Defaults returns the built-in configuration.
func Defaults() Config {
	p := Presets["all"]
	return Config{
		Preset:      "all",
		TLDs:        append([]string(nil), p.TLDs...),
		Targets:     append([]string(nil), p.Targets...),
		Concurrency: 16,
		Timeout:     "3s",
		CacheTTL:    "",
		Github: struct {
			TokenEnv string `toml:"token_env"`
		}{TokenEnv: "GITHUB_TOKEN"}, // #nosec G101 -- environment variable name, not a secret.
	}
}

// Path returns the config file path, honoring NAMEARY_CONFIG.
func Path() string {
	if p := os.Getenv("NAMEARY_CONFIG"); p != "" {
		return p
	}
	dir, err := os.UserConfigDir()
	if err != nil || dir == "" {
		dir = "."
	}
	return filepath.Join(dir, "nameary", "config.toml")
}

// Load reads the config, falling back to defaults when the file is absent.
func Load() (Config, error) {
	var cfg Config
	path := Path()
	data, err := os.ReadFile(path) // #nosec G304 -- path is the user-selected config file.
	if err != nil {
		if os.IsNotExist(err) {
			cfg.applyDefaults()
			return cfg, nil
		}
		return cfg, err
	}
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse %s: %w", path, err)
	}
	cfg.applyDefaults()
	return cfg, nil
}

func (c *Config) applyDefaults() {
	d := Defaults()
	if c.Preset == "" {
		c.Preset = d.Preset
	}
	if len(c.Targets) == 0 {
		if p, ok := Presets[c.Preset]; ok {
			c.Targets = append([]string(nil), p.Targets...)
		} else {
			c.Targets = d.Targets
		}
	}
	if len(c.TLDs) == 0 {
		if p, ok := Presets[c.Preset]; ok {
			c.TLDs = append([]string(nil), p.TLDs...)
		} else {
			c.TLDs = d.TLDs
		}
	}
	if c.Concurrency <= 0 {
		c.Concurrency = d.Concurrency
	}
	if c.Timeout == "" {
		c.Timeout = d.Timeout
	}
	if c.CacheTTL == "" {
		c.CacheTTL = d.CacheTTL
	}
	if c.Github.TokenEnv == "" {
		c.Github.TokenEnv = d.Github.TokenEnv
	}
}

// Save writes the config to Path.
func Save(cfg Config) error {
	path := Path()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	// Write a 0600 temporary file and rename it into place. Unlike os.Create,
	// this does not follow an existing symlink at the destination.
	f, err := os.CreateTemp(dir, ".nameary-*.toml")
	if err != nil {
		return err
	}
	tmp := f.Name()
	enc := toml.NewEncoder(f)
	enc.Indent = "  "
	if err := enc.Encode(cfg); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		// Windows does not always replace an existing destination atomically.
		// Removing the link/file first still avoids writing through a symlink.
		if removeErr := os.Remove(path); removeErr == nil {
			if err2 := os.Rename(tmp, path); err2 == nil {
				return nil
			}
		}
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// ParseDuration parses a duration with a helpful error.
func ParseDuration(s string, fallback time.Duration) time.Duration {
	if s == "" {
		return fallback
	}
	d, err := time.ParseDuration(s)
	if err != nil || d <= 0 {
		return fallback
	}
	return d
}
