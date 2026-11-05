// Package adapters contains the registry-specific checker implementations.
package adapters

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/yufeiminds/nameary/internal/check"
)

// Options are shared by all adapters.
type Options struct {
	HTTP        *check.HTTP
	GithubToken string
}

// All returns every registry checker (domains are added separately by app).
func All(opts Options) []check.Checker {
	if opts.HTTP == nil {
		opts.HTTP = check.NewHTTP()
	}
	return []check.Checker{
		NewNPM(opts.HTTP, ""),
		NewPyPI(opts.HTTP, ""),
		NewCrates(opts.HTTP, ""),
		NewRubyGems(opts.HTTP, ""),
		NewNuGet(opts.HTTP, ""),
		NewDockerHub(opts.HTTP, ""),
		NewHomebrew(opts.HTTP, ""),
		NewPackagist(opts.HTTP, ""),
		NewHex(opts.HTTP, ""),
		NewPub(opts.HTTP, ""),
		NewGitHub(opts.HTTP, "", opts.GithubToken),
		NewMaven(opts.HTTP, ""),
		NewConda(opts.HTTP, ""),
		NewCocoaPods(opts.HTTP, ""),
		NewAUR(opts.HTTP, ""),
		NewSnap(opts.HTTP, ""),
		NewChocolatey(opts.HTTP, ""),
		NewHackage(opts.HTTP, ""),
		NewJSR(opts.HTTP, ""),
	}
}

func available(query, source string, fields ...check.Field) check.Result {
	return check.Result{
		Status:     check.StatusAvailable,
		Confidence: check.Certain,
		Query:      query,
		Summary:    "no record",
		Fields:     fields,
		SourceURL:  source,
		FetchedAt:  time.Now(),
	}
}

func taken(query, source, summary string, fields ...check.Field) check.Result {
	return check.Result{
		Status:     check.StatusTaken,
		Confidence: check.Certain,
		Query:      query,
		Summary:    summary,
		Fields:     fields,
		SourceURL:  source,
		FetchedAt:  time.Now(),
	}
}

func httpStatusError(code int, url string, header http.Header) error {
	return check.NewHTTPError(code, url, header)
}

func cleanName(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func validate(s, re string, max int) (string, error) {
	s = cleanName(s)
	if s == "" {
		return "", fmt.Errorf("name is empty")
	}
	if max > 0 && len(s) > max {
		return "", fmt.Errorf("name is longer than %d characters", max)
	}
	if re != "" && !regexp.MustCompile(re).MatchString(s) {
		return "", fmt.Errorf("name contains characters this registry does not allow")
	}
	return s, nil
}

func validatePattern(s, re string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("name is empty")
	}
	if re != "" && !regexp.MustCompile(re).MatchString(s) {
		return "", fmt.Errorf("name contains characters this registry does not allow")
	}
	return s, nil
}

func humanCount(n int64) string {
	switch {
	case n >= 1_000_000_000:
		return fmt.Sprintf("%.1fB", float64(n)/1_000_000_000)
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

func formatFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', 2, 64)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
