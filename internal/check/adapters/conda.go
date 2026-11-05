package adapters

import (
	"context"
	"net/url"
	"strings"
	"time"

	"github.com/yufeiminds/nameary/internal/check"
)

// Conda checks Anaconda's package search across all channels.
type Conda struct {
	http *check.HTTP
	base string
}

func NewConda(h *check.HTTP, base string) *Conda {
	if base == "" {
		base = "https://api.anaconda.org"
	}
	return &Conda{http: h, base: base}
}

func (c *Conda) Target() check.Target     { return check.TargetConda }
func (c *Conda) Label() string            { return "conda" }
func (c *Conda) Category() check.Category { return check.CategoryPackages }
func (c *Conda) Policy() check.Policy {
	return check.Policy{Priority: 43, Concurrency: 3, Timeout: 10 * time.Second, CacheTTL: 24 * time.Hour}
}

var condaName = `^[a-z0-9][a-z0-9._-]*$`

func (c *Conda) Normalize(raw string) (string, error) {
	return validate(raw, condaName, 100)
}

func (c *Conda) Homepage(q string) string {
	return "https://anaconda.org/search?q=" + url.QueryEscape(q)
}

type condaPackage struct {
	Name          string   `json:"name"`
	Summary       string   `json:"summary"`
	Description   string   `json:"description"`
	Home          string   `json:"home"`
	Owner         string   `json:"owner"`
	FullName      string   `json:"full_name"`
	HTMLURL       string   `json:"html_url"`
	LatestVersion string   `json:"latest_version"`
	Versions      []string `json:"versions"`
}

func (c *Conda) Check(ctx context.Context, q string) (check.Result, error) {
	u := check.JoinURL(c.base, "/search?name="+url.QueryEscape(q)+"&limit=50")
	var items []condaPackage
	code, header, err := c.http.Get(ctx, u, &items)
	if err != nil {
		return check.Result{}, err
	}
	switch code {
	case 200:
		var exact *condaPackage
		for i := range items {
			if !strings.EqualFold(items[i].Name, q) {
				continue
			}
			if exact == nil ||
				(items[i].Owner == "conda-forge" && exact.Owner != "conda-forge") ||
				(items[i].Owner == exact.Owner && len(items[i].Versions) > len(exact.Versions)) {
				exact = &items[i]
			}
		}
		if exact == nil {
			if len(items) >= 50 {
				return check.Result{
					Status:     check.StatusUnknown,
					Confidence: check.Low,
					Query:      q,
					Summary:    "search truncated",
					SourceURL:  c.Homepage(q),
					FetchedAt:  time.Now(),
					Fields: []check.Field{
						{Key: "detail", Value: "Anaconda returned 50 fuzzy matches without an exact package name; the search may be truncated."},
						{Key: "note", Value: "Try the Anaconda search link to confirm."},
					},
				}, nil
			}
			return available(q, c.Homepage(q), check.Field{Key: "registry", Value: "Anaconda"}), nil
		}
		return taken(q, firstNonEmpty(exact.HTMLURL, c.Homepage(q)), "published",
			check.Field{Key: "package", Value: exact.FullName},
			check.Field{Key: "owner", Value: exact.Owner},
			check.Field{Key: "summary", Value: exact.Summary},
			check.Field{Key: "home", Value: exact.Home},
			check.Field{Key: "note", Value: "Anaconda search spans all channels; conda-forge is usually the canonical build."},
		), nil
	default:
		return check.Result{}, httpStatusError(code, u, header)
	}
}
