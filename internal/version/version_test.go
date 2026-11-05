package version

import "testing"

func TestString(t *testing.T) {
	oldV, oldC, oldD := Version, Commit, Date
	defer func() { Version, Commit, Date = oldV, oldC, oldD }()

	Version, Commit, Date = "1.2.3", "abcdef0123456789", "2026-01-02"
	if got := String(); got != "nameary 1.2.3 (abcdef0) 2026-01-02" {
		t.Fatalf("got %q", got)
	}

	Version, Commit, Date = "dev", "none", "unknown"
	if got := String(); got != "nameary dev" {
		t.Fatalf("got %q", got)
	}

	Version, Commit, Date = "", "", "2026-01-02"
	if got := String(); got != "nameary dev 2026-01-02" {
		t.Fatalf("got %q", got)
	}

	Version, Commit, Date = "2.0.0", "abc", ""
	if got := String(); got != "nameary 2.0.0 (abc)" {
		t.Fatalf("got %q", got)
	}
}
