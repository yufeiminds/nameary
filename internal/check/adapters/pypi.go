package adapters

import (
	"context"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/yufeiminds/nameary/internal/check"
)

// PyPI checks the Python Package Index.
type PyPI struct {
	http *check.HTTP
	base string
}

func NewPyPI(h *check.HTTP, base string) *PyPI {
	if base == "" {
		base = "https://pypi.org"
	}
	return &PyPI{http: h, base: base}
}

func (p *PyPI) Target() check.Target     { return check.TargetPyPI }
func (p *PyPI) Label() string            { return "pypi" }
func (p *PyPI) Category() check.Category { return check.CategoryPackages }
func (p *PyPI) Policy() check.Policy {
	return check.Policy{Priority: 21, Concurrency: 4, Timeout: 8 * time.Second, CacheTTL: 24 * time.Hour}
}

var pypiSep = regexp.MustCompile(`[-_.]+`)
var pypiName = `^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`

func (p *PyPI) Normalize(raw string) (string, error) {
	q := strings.ToLower(strings.TrimSpace(raw))
	q = pypiSep.ReplaceAllString(q, "-")
	return validate(q, pypiName, 100)
}

func (p *PyPI) Homepage(q string) string {
	return "https://pypi.org/project/" + q + "/"
}

func (p *PyPI) Check(ctx context.Context, q string) (check.Result, error) {
	u := check.JoinURL(p.base, "/pypi/"+url.PathEscape(q)+"/json")
	var payload struct {
		Info struct {
			Name       string `json:"name"`
			Version    string `json:"version"`
			Summary    string `json:"summary"`
			Author     string `json:"author"`
			ProjectURL string `json:"project_url"`
		} `json:"info"`
	}
	code, header, err := p.http.Get(ctx, u, &payload)
	if err != nil {
		return check.Result{}, err
	}
	switch code {
	case 200:
		return taken(q, p.Homepage(q), "v"+payload.Info.Version,
			check.Field{Key: "package", Value: payload.Info.Name},
			check.Field{Key: "latest", Value: payload.Info.Version},
			check.Field{Key: "summary", Value: payload.Info.Summary},
			check.Field{Key: "author", Value: payload.Info.Author},
			check.Field{Key: "project", Value: payload.Info.ProjectURL},
		), nil
	case 404:
		return available(q, p.Homepage(q), check.Field{Key: "registry", Value: "PyPI"}), nil
	default:
		return check.Result{}, httpStatusError(code, u, header)
	}
}
