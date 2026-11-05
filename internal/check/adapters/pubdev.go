package adapters

import (
	"context"
	"net/url"
	"time"

	"github.com/yufeiminds/nameary/internal/check"
)

// Pub checks pub.dev.
type Pub struct {
	http *check.HTTP
	base string
}

func NewPub(h *check.HTTP, base string) *Pub {
	if base == "" {
		base = "https://pub.dev"
	}
	return &Pub{http: h, base: base}
}

func (p *Pub) Target() check.Target     { return check.TargetPub }
func (p *Pub) Label() string            { return "pub" }
func (p *Pub) Category() check.Category { return check.CategoryPackages }
func (p *Pub) Policy() check.Policy {
	return check.Policy{Priority: 49, Concurrency: 4, Timeout: 8 * time.Second, CacheTTL: 24 * time.Hour}
}

var pubName = `^[a-z0-9][a-z0-9_]*$`

func (p *Pub) Normalize(raw string) (string, error) {
	return validate(raw, pubName, 100)
}

func (p *Pub) Homepage(q string) string {
	return "https://pub.dev/packages/" + q
}

func (p *Pub) Check(ctx context.Context, q string) (check.Result, error) {
	u := check.JoinURL(p.base, "/api/packages/"+url.PathEscape(q))
	var payload struct {
		Name   string `json:"name"`
		Latest struct {
			Version string `json:"version"`
			Pubspec struct {
				Description string `json:"description"`
			} `json:"pubspec"`
		} `json:"latest"`
	}
	code, header, err := p.http.Get(ctx, u, &payload)
	if err != nil {
		return check.Result{}, err
	}
	switch code {
	case 200:
		return taken(q, p.Homepage(q), "v"+payload.Latest.Version,
			check.Field{Key: "package", Value: payload.Name},
			check.Field{Key: "latest", Value: payload.Latest.Version},
			check.Field{Key: "description", Value: payload.Latest.Pubspec.Description},
		), nil
	case 404:
		return available(q, p.Homepage(q), check.Field{Key: "registry", Value: "pub.dev"}), nil
	default:
		return check.Result{}, httpStatusError(code, u, header)
	}
}
