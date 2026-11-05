package app

import (
	"context"
	"testing"

	"github.com/yufeiminds/nameary/internal/check"
)

func TestExpandName(t *testing.T) {
	base, domains, err := ExpandName("Acme Inc", []string{"com", "io"})
	if err != nil {
		t.Fatal(err)
	}
	if base != "acme-inc" {
		t.Fatalf("base=%q", base)
	}
	if len(domains) != 2 || domains[0] != "acme-inc.com" || domains[1] != "acme-inc.io" {
		t.Fatalf("domains=%v", domains)
	}

	base, domains, err = ExpandName("acme.dev", []string{"com"})
	if err != nil {
		t.Fatal(err)
	}
	if base != "acme.dev" || len(domains) != 1 || domains[0] != "acme.dev" {
		t.Fatalf("base=%q domains=%v", base, domains)
	}

	_, domains, err = ExpandName("foo.1", []string{"com"})
	if err != nil {
		t.Fatal(err)
	}
	if len(domains) != 0 {
		t.Fatalf("expected no domains, got %v", domains)
	}
}

func TestCanonicalTargets(t *testing.T) {
	got, err := CanonicalTargets([]string{"domains", "npm", "NPM", "docker", "brew"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"domains", "npm", "dockerhub", "homebrew"}
	if len(got) != len(want) {
		t.Fatalf("got=%v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got=%v want=%v", got, want)
		}
	}
	if _, err := CanonicalTargets([]string{"nope"}); err == nil {
		t.Fatal("expected unknown target error")
	}
	all, err := CanonicalTargets([]string{"all"})
	if err != nil || len(all) != len(AllTargets) {
		t.Fatalf("all=%v err=%v", all, err)
	}
}

func TestPlan(t *testing.T) {
	s, err := Prepare(context.Background(), Options{
		Targets: []string{"domains", "npm", "pypi"},
		TLDs:    []string{"com", "io"},
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
	if len(jobs) != 4 {
		t.Fatalf("jobs=%d, want 4: %+v", len(jobs), jobs)
	}
	seen := map[check.Target]int{}
	for _, j := range jobs {
		seen[j.Target]++
		if j.ID == "" {
			t.Fatal("empty job id")
		}
	}
	if seen[check.TargetDomains] != 2 || seen[check.TargetNPM] != 1 || seen[check.TargetPyPI] != 1 {
		t.Fatalf("seen=%v", seen)
	}

	jobs, err = s.Plan("foo.1")
	if err != nil {
		t.Fatal(err)
	}
	var skipped int
	for _, j := range jobs {
		if j.Err != nil {
			skipped++
		}
	}
	if skipped != 1 {
		t.Fatalf("expected one skipped domain job, got %d: %+v", skipped, jobs)
	}
}
