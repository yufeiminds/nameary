package app

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yufeiminds/nameary/internal/check"
)

func TestSummary(t *testing.T) {
	s := NewSummary(4)
	if s.Total != 4 {
		t.Fatal("total")
	}
	s.Add(check.StatusAvailable)
	s.Add(check.StatusAvailable)
	s.Add(check.StatusTaken)
	s.Add(check.StatusUnknown)
	if s.Count(check.StatusAvailable) != 2 || s.Available() != 2 || s.Taken() != 1 || s.Unknown() != 1 {
		t.Fatalf("summary=%+v", s)
	}
	if s.Finished() != 4 {
		t.Fatalf("finished=%d", s.Finished())
	}
}

func TestCanonicalTargetsMore(t *testing.T) {
	if _, err := CanonicalTargets(nil); err == nil {
		t.Fatal("empty targets should fail")
	}
	got, err := CanonicalTargets([]string{"maven-central", "anaconda", "pods", "snapcraft", "choco", "haskell", "deno"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 7 {
		t.Fatalf("got=%v", got)
	}
}

func TestPrepareBranches(t *testing.T) {
	ctx := context.Background()
	if _, err := Prepare(ctx, Options{Targets: []string{"nope"}, NoCache: true}); err == nil {
		t.Fatal("unknown target should fail")
	}
	tooMany := make([]string, 65)
	for i := range tooMany {
		tooMany[i] = "com"
	}
	if _, err := Prepare(ctx, Options{Targets: []string{"domains"}, TLDs: tooMany, NoCache: true}); err == nil {
		t.Fatal("too many TLDs should fail")
	}
	if _, err := Prepare(ctx, Options{Targets: []string{"domains"}, Name: strings.Repeat("x", 513), NoCache: true}); err == nil {
		t.Fatal("long name should fail")
	}

	cachePath := filepath.Join(t.TempDir(), "cache.db")
	s, err := Prepare(ctx, Options{
		Targets:     []string{"domains", "npm"},
		TLDs:        []string{"com"},
		Name:        "acme",
		Concurrency: 999,
		Timeout:     10 * time.Minute,
		CachePath:   cachePath,
	})
	if err != nil {
		t.Fatal(err)
	}
	if s.Cache == nil || s.Engine == nil || len(s.Jobs) != 2 {
		t.Fatalf("session=%+v jobs=%d", s, len(s.Jobs))
	}
	s.Close()

	s, err = Prepare(ctx, Options{Targets: []string{"npm"}, Name: "acme", NoCache: true})
	if err != nil {
		t.Fatal(err)
	}
	if s.Cache != nil {
		t.Fatal("cache should be disabled")
	}
	s.Close()
	if (&Session{}).Close(); false {
		t.Fatal("unreachable")
	}
	var nilSession *Session
	nilSession.Close()
}

func TestPlanAllTargets(t *testing.T) {
	s, err := Prepare(context.Background(), Options{
		Targets: AllTargets,
		TLDs:    []string{"com"},
		NoCache: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	jobs, err := s.Plan("acme")
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != len(AllTargets) {
		t.Fatalf("jobs=%d targets=%d", len(jobs), len(AllTargets))
	}
	seen := map[check.Target]bool{}
	for _, j := range jobs {
		seen[j.Target] = true
	}
	for _, target := range AllTargets {
		if !seen[check.Target(target)] {
			t.Fatalf("missing target %s", target)
		}
	}
}

func TestPlanScopedAndInvalid(t *testing.T) {
	s, err := Prepare(context.Background(), Options{
		Targets: []string{"npm", "jsr", "domains"},
		TLDs:    []string{"com"},
		NoCache: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	jobs, err := s.Plan("@scope/pkg")
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 3 {
		t.Fatalf("jobs=%d", len(jobs))
	}
	jobs, err = s.Plan("foo.1")
	if err != nil {
		t.Fatal(err)
	}
	var skipped bool
	for _, j := range jobs {
		if j.Target == check.TargetDomains && j.Err != nil {
			skipped = true
		}
	}
	if !skipped {
		t.Fatal("expected skipped domain job")
	}
}

func TestExpandNameBranches(t *testing.T) {
	if _, _, err := ExpandName("", []string{"com"}); err == nil {
		t.Fatal("empty")
	}
	if _, _, err := ExpandName(strings.Repeat("a", 513), []string{"com"}); err == nil {
		t.Fatal("too long")
	}
	if _, _, err := ExpandName("acme", []string{"not a tld"}); err != nil {
		t.Fatalf("no valid domains should be handled by Plan: %v", err)
	}
	base, domains, err := ExpandName("acme.com", nil)
	if err != nil || base != "acme.com" || len(domains) != 1 {
		t.Fatalf("base=%q domains=%v err=%v", base, domains, err)
	}
	base, domains, err = ExpandName("acme", []string{"com", "com", ""})
	if err != nil || len(domains) != 1 || domains[0] != "acme.com" {
		t.Fatalf("base=%q domains=%v err=%v", base, domains, err)
	}
}
