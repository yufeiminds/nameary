package adapters

import (
	"context"
	"net/url"
	"time"

	"github.com/yufeiminds/nameary/internal/check"
)

// Homebrew checks both formulae and casks.
type Homebrew struct {
	http *check.HTTP
	base string
}

func NewHomebrew(h *check.HTTP, base string) *Homebrew {
	if base == "" {
		base = "https://formulae.brew.sh"
	}
	return &Homebrew{http: h, base: base}
}

func (b *Homebrew) Target() check.Target     { return check.TargetHomebrew }
func (b *Homebrew) Label() string            { return "brew" }
func (b *Homebrew) Category() check.Category { return check.CategoryPackages }
func (b *Homebrew) Policy() check.Policy {
	return check.Policy{Priority: 46, Concurrency: 4, Timeout: 8 * time.Second, CacheTTL: 24 * time.Hour}
}

var brewName = `^[a-z0-9][a-z0-9@+._-]*$`

func (b *Homebrew) Normalize(raw string) (string, error) {
	return validate(raw, brewName, 100)
}

func (b *Homebrew) Homepage(q string) string {
	return "https://formulae.brew.sh/formula/" + q
}

func (b *Homebrew) Check(ctx context.Context, q string) (check.Result, error) {
	type formula struct {
		Name    string `json:"name"`
		Desc    string `json:"desc"`
		Version struct {
			Stable string `json:"stable"`
		} `json:"versions"`
	}
	fu := check.JoinURL(b.base, "/api/formula/"+url.PathEscape(q)+".json")
	var f formula
	code, header, err := b.http.Get(ctx, fu, &f)
	if err != nil {
		return check.Result{}, err
	}
	switch code {
	case 200:
		return taken(q, b.Homepage(q), "v"+f.Version.Stable,
			check.Field{Key: "kind", Value: "formula"},
			check.Field{Key: "latest", Value: f.Version.Stable},
			check.Field{Key: "description", Value: f.Desc},
		), nil
	case 404:
		// fall through to cask
	case 429:
		return check.Result{}, httpStatusError(code, fu, header)
	default:
		if code >= 500 {
			return check.Result{}, httpStatusError(code, fu, header)
		}
	}

	type cask struct {
		Token   string `json:"token"`
		Version string `json:"version"`
		Desc    string `json:"desc"`
	}
	cu := check.JoinURL(b.base, "/api/cask/"+url.PathEscape(q)+".json")
	var c cask
	code, header, err = b.http.Get(ctx, cu, &c)
	if err != nil {
		return check.Result{}, err
	}
	switch code {
	case 200:
		return taken(q, "https://formulae.brew.sh/cask/"+q, "v"+c.Version,
			check.Field{Key: "kind", Value: "cask"},
			check.Field{Key: "latest", Value: c.Version},
			check.Field{Key: "description", Value: c.Desc},
		), nil
	case 404:
		return available(q, b.Homepage(q), check.Field{Key: "registry", Value: "Homebrew"}), nil
	default:
		return check.Result{}, httpStatusError(code, cu, header)
	}
}
