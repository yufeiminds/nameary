package adapters

import (
	"context"
	"net/url"
	"time"

	"github.com/yufeiminds/nameary/internal/check"
)

// RubyGems checks rubygems.org.
type RubyGems struct {
	http *check.HTTP
	base string
}

func NewRubyGems(h *check.HTTP, base string) *RubyGems {
	if base == "" {
		base = "https://rubygems.org"
	}
	return &RubyGems{http: h, base: base}
}

func (r *RubyGems) Target() check.Target     { return check.TargetRubyGems }
func (r *RubyGems) Label() string            { return "gem" }
func (r *RubyGems) Category() check.Category { return check.CategoryPackages }
func (r *RubyGems) Policy() check.Policy {
	return check.Policy{Priority: 45, Concurrency: 4, Timeout: 8 * time.Second, CacheTTL: 24 * time.Hour}
}

var gemName = `^[a-z0-9][a-z0-9._-]*$`

func (r *RubyGems) Normalize(raw string) (string, error) {
	return validate(raw, gemName, 100)
}

func (r *RubyGems) Homepage(q string) string {
	return "https://rubygems.org/gems/" + q
}

func (r *RubyGems) Check(ctx context.Context, q string) (check.Result, error) {
	u := check.JoinURL(r.base, "/api/v1/gems/"+url.PathEscape(q)+".json")
	var payload struct {
		Name      string `json:"name"`
		Version   string `json:"version"`
		Downloads int64  `json:"downloads"`
		Info      string `json:"info"`
		Authors   string `json:"authors"`
	}
	code, header, err := r.http.Get(ctx, u, &payload)
	if err != nil {
		return check.Result{}, err
	}
	switch code {
	case 200:
		return taken(q, r.Homepage(q), "v"+payload.Version,
			check.Field{Key: "package", Value: payload.Name},
			check.Field{Key: "latest", Value: payload.Version},
			check.Field{Key: "downloads", Value: humanCount(payload.Downloads)},
			check.Field{Key: "summary", Value: payload.Info},
			check.Field{Key: "author", Value: payload.Authors},
		), nil
	case 404:
		return available(q, r.Homepage(q), check.Field{Key: "registry", Value: "RubyGems"}), nil
	default:
		return check.Result{}, httpStatusError(code, u, header)
	}
}
