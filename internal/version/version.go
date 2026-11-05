// Package version holds build-time version metadata.
package version

import (
	"runtime/debug"
	"strings"
)

var (
	// Version is injected by goreleaser/Makefile. "dev" means a local build.
	Version = "dev"
	// Commit is the short git revision, injected at build time.
	Commit = "none"
	// Date is the build date, injected at build time.
	Date = "unknown"
)

// String returns a compact version string, falling back to Go's build info for
// `go install` builds that do not use ldflags.
func String() string {
	v := Version
	commit := Commit
	if v == "" || v == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok {
			if info.Main.Version != "" && info.Main.Version != "(devel)" {
				v = info.Main.Version
			}
			for _, s := range info.Settings {
				if s.Key == "vcs.revision" && commit == "none" {
					commit = s.Value
				}
			}
		}
	}
	if v == "" || v == "dev" {
		v = "dev"
	}
	out := "nameary " + v
	if commit != "" && commit != "none" {
		if len(commit) > 7 {
			commit = commit[:7]
		}
		out += " (" + commit + ")"
	}
	if Date != "" && Date != "unknown" {
		out += " " + Date
	}
	return strings.TrimSpace(out)
}
