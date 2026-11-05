// Package app wires checkers, the engine, the TUI and the headless renderers.
package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/yufeiminds/nameary/internal/cache"
	"github.com/yufeiminds/nameary/internal/check"
	"github.com/yufeiminds/nameary/internal/check/adapters"
	"github.com/yufeiminds/nameary/internal/check/domain"
	"github.com/yufeiminds/nameary/internal/engine"
)

// Options is the resolved runtime configuration.
type Options struct {
	Name          string
	Targets       []string
	TLDs          []string
	Concurrency   int
	Timeout       time.Duration
	CachePath     string
	CacheTTL      time.Duration
	NoCache       bool
	Refresh       bool
	GithubToken   string
	ReducedMotion bool
	ASCII         bool
	NoColor       bool
	Celebrate     bool
}

// Session owns the checker set, cache and engine for one process.
type Session struct {
	Options  Options
	Checkers map[check.Target]check.Checker
	Engine   *engine.Engine
	Cache    *cache.Store
	Jobs     []engine.Job
}

// AllTargets is the stable order used by `--targets all` and the all preset.
var AllTargets = []string{
	"domains", "npm", "pypi", "crates", "rubygems", "nuget", "dockerhub", "homebrew",
	"packagist", "hex", "pub", "github", "maven", "conda", "cocoapods", "aur", "snap",
	"chocolatey", "hackage", "jsr",
}

var targetAliases = map[string]check.Target{
	"domain":        check.TargetDomains,
	"domains":       check.TargetDomains,
	"npm":           check.TargetNPM,
	"pypi":          check.TargetPyPI,
	"crates":        check.TargetCrates,
	"cargo":         check.TargetCrates,
	"rubygems":      check.TargetRubyGems,
	"gem":           check.TargetRubyGems,
	"gems":          check.TargetRubyGems,
	"nuget":         check.TargetNuGet,
	"docker":        check.TargetDocker,
	"dockerhub":     check.TargetDocker,
	"homebrew":      check.TargetHomebrew,
	"brew":          check.TargetHomebrew,
	"packagist":     check.TargetPackagist,
	"composer":      check.TargetPackagist,
	"hex":           check.TargetHex,
	"pub":           check.TargetPub,
	"pubdev":        check.TargetPub,
	"github":        check.TargetGitHub,
	"maven":         check.TargetMaven,
	"maven-central": check.TargetMaven,
	"conda":         check.TargetConda,
	"anaconda":      check.TargetConda,
	"cocoapods":     check.TargetCocoaPods,
	"pods":          check.TargetCocoaPods,
	"pod":           check.TargetCocoaPods,
	"aur":           check.TargetAUR,
	"snap":          check.TargetSnap,
	"snapcraft":     check.TargetSnap,
	"chocolatey":    check.TargetChocolatey,
	"choco":         check.TargetChocolatey,
	"hackage":       check.TargetHackage,
	"haskell":       check.TargetHackage,
	"jsr":           check.TargetJSR,
	"deno":          check.TargetJSR,
}

// CanonicalTargets normalizes aliases and removes duplicates.
func CanonicalTargets(in []string) ([]string, error) {
	seen := map[check.Target]bool{}
	var out []string
	for _, raw := range in {
		raw = strings.ToLower(strings.TrimSpace(raw))
		if raw == "" {
			continue
		}
		if raw == "all" {
			for _, name := range AllTargets {
				t := targetAliases[name]
				if seen[t] {
					continue
				}
				seen[t] = true
				out = append(out, string(t))
			}
			continue
		}
		t, ok := targetAliases[raw]
		if !ok {
			return nil, fmt.Errorf("unknown target %q (try domains, npm, pypi, crates, rubygems, nuget, dockerhub, homebrew, packagist, hex, pub, github)", raw)
		}
		if seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, string(t))
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no targets selected")
	}
	return out, nil
}

// Prepare builds checkers, opens the cache and plans the initial jobs.
func Prepare(ctx context.Context, opts Options) (*Session, error) {
	targets, err := CanonicalTargets(opts.Targets)
	if err != nil {
		return nil, err
	}
	opts.Targets = targets
	if len(opts.TLDs) == 0 {
		opts.TLDs = []string{"com", "io", "dev"}
	}
	if len(opts.TLDs) > 64 {
		return nil, fmt.Errorf("too many TLDs (max 64)")
	}
	if len(opts.Name) > 512 {
		return nil, fmt.Errorf("name is too long (max 512 characters)")
	}
	if opts.Concurrency <= 0 {
		opts.Concurrency = 16
	}
	if opts.Concurrency > 128 {
		opts.Concurrency = 128
	}
	if opts.Timeout > 5*time.Minute {
		opts.Timeout = 5 * time.Minute
	}

	h := check.NewHTTP()
	baseURL := testBaseURL()
	s := &Session{Options: opts, Checkers: map[check.Target]check.Checker{}}

	for _, c := range adapters.All(adapters.Options{HTTP: h, GithubToken: opts.GithubToken, BaseURL: baseURL}) {
		s.Checkers[c.Target()] = c
	}
	domainChecker := domain.New(h, baseURL)
	if baseURL != "" {
		// E2E builds must not depend on public DNS.
		domainChecker.DisableDNS = true
	}
	s.Checkers[check.TargetDomains] = domainChecker

	var store *cache.Store
	if !opts.NoCache {
		path := opts.CachePath
		if path == "" {
			path = cache.DefaultPath()
		}
		if st, err := cache.Open(path); err == nil {
			store = st
		}
	}
	s.Cache = store
	s.Engine = engine.New(engine.Options{
		Concurrency: opts.Concurrency,
		Cache:       store,
		Refresh:     opts.Refresh,
		MaxRetries:  2,
		Timeout:     opts.Timeout,
		CacheTTL:    opts.CacheTTL,
	})

	if opts.Name != "" {
		jobs, err := s.Plan(opts.Name)
		if err != nil {
			s.Close()
			return nil, err
		}
		s.Jobs = jobs
	}
	return s, nil
}

// Plan expands a brand name into jobs for every selected target.
func (s *Session) Plan(name string) ([]engine.Job, error) {
	base, domains, err := ExpandName(name, s.Options.TLDs)
	if err != nil {
		return nil, err
	}
	var jobs []engine.Job
	seq := 0
	add := func(target check.Target, label, query string, checker check.Checker, jobErr error) {
		p := checker.Policy()
		cat := checker.Category()
		jobs = append(jobs, engine.Job{
			ID:          fmt.Sprintf("%s:%s:%d", target, query, seq),
			Target:      target,
			TargetLabel: checker.Label(),
			Category:    cat,
			Raw:         base,
			Query:       query,
			Label:       label,
			Priority:    p.Priority,
			Checker:     checker,
			Err:         jobErr,
		})
		seq++
	}

	for _, targetName := range s.Options.Targets {
		target := check.Target(targetName)
		checker := s.Checkers[target]
		if checker == nil {
			continue
		}
		if target == check.TargetDomains {
			if len(domains) == 0 {
				add(target, base, base, checker, fmt.Errorf("%q is not a valid domain", base))
				continue
			}
			for _, d := range domains {
				add(target, d, d, checker, nil)
			}
			continue
		}
		q, nerr := checker.Normalize(base)
		if nerr != nil {
			add(target, base, base, checker, nerr)
			continue
		}
		add(target, q, q, checker, nil)
	}
	if len(jobs) == 0 {
		return nil, fmt.Errorf("no jobs could be planned")
	}
	return jobs, nil
}

// ExpandName returns the package-name base and the fully qualified domains.
func ExpandName(raw string, tlds []string) (base string, domains []string, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil, fmt.Errorf("name is empty")
	}
	if len(raw) > 512 {
		return "", nil, fmt.Errorf("name is too long (max 512 characters)")
	}
	base = strings.ToLower(raw)

	if strings.Contains(raw, ".") {
		d, derr := domain.Normalize(raw)
		if derr != nil {
			// The input may still be a valid package name (npm allows dots).
			// Leave domains empty; Plan will add a skipped row if the user
			// selected the domains target.
			return base, nil, nil
		}
		return base, []string{d}, nil
	}

	label := strings.ToLower(raw)
	label = strings.ReplaceAll(label, " ", "-")
	label = strings.ReplaceAll(label, "_", "-")
	label = strings.Trim(label, "-")
	var b strings.Builder
	for _, r := range label {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		}
	}
	label = strings.Trim(b.String(), "-")
	if label == "" {
		return "", nil, fmt.Errorf("name contains no usable domain characters")
	}
	if len(label) > 63 {
		return "", nil, fmt.Errorf("name is too long for a domain label (max 63)")
	}
	if !strings.ContainsAny(raw, "@/") {
		base = label
	} else {
		base = strings.ToLower(strings.TrimSpace(raw))
	}
	seen := map[string]bool{}
	for _, tld := range tlds {
		tld = strings.Trim(strings.ToLower(strings.TrimSpace(tld)), ".")
		if tld == "" {
			continue
		}
		d := label + "." + tld
		if seen[d] {
			continue
		}
		if _, derr := domain.Normalize(d); derr != nil {
			continue
		}
		seen[d] = true
		domains = append(domains, d)
	}
	return base, domains, nil
}

// Close releases the cache.
func (s *Session) Close() {
	if s == nil {
		return
	}
	finished := true
	if s.Engine != nil {
		finished = s.Engine.WaitTimeout(300 * time.Millisecond)
	}
	if finished && s.Cache != nil {
		_ = s.Cache.Close()
	}
}

// Summary is the end-of-run aggregate used for exit codes and footers.
type Summary struct {
	Counts   map[check.Status]int
	Total    int
	Duration time.Duration
}

// NewSummary initializes a summary with all statuses present.
func NewSummary(total int) Summary {
	return Summary{Counts: map[check.Status]int{}, Total: total}
}

// Add records a settled status.
func (s *Summary) Add(st check.Status) {
	if s.Counts == nil {
		s.Counts = map[check.Status]int{}
	}
	s.Counts[st]++
}

// Count returns the count for a status.
func (s Summary) Count(st check.Status) int { return s.Counts[st] }

// Available returns the number of available results.
func (s Summary) Available() int { return s.Counts[check.StatusAvailable] }

// Taken returns the number of taken results.
func (s Summary) Taken() int { return s.Counts[check.StatusTaken] }

// Unknown returns the number of unknown results.
func (s Summary) Unknown() int { return s.Counts[check.StatusUnknown] }

// Finished returns how many jobs reached a final state.
func (s Summary) Finished() int {
	n := 0
	for st, c := range s.Counts {
		if st.IsFinal() {
			n += c
		}
	}
	return n
}
