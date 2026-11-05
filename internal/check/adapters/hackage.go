package adapters

import (
	"context"
	"net/url"
	"time"

	"github.com/yufeiminds/nameary/internal/check"
)

// Hackage checks the Haskell package repository. Hackage has no JSON API for
// package metadata, but a HEAD request is enough for availability.
type Hackage struct {
	http *check.HTTP
	base string
}

func NewHackage(h *check.HTTP, base string) *Hackage {
	if base == "" {
		base = "https://hackage.haskell.org"
	}
	return &Hackage{http: h, base: base}
}

func (h *Hackage) Target() check.Target     { return check.TargetHackage }
func (h *Hackage) Label() string            { return "hackage" }
func (h *Hackage) Category() check.Category { return check.CategoryPackages }
func (h *Hackage) Policy() check.Policy {
	return check.Policy{Priority: 53, Concurrency: 4, Timeout: 8 * time.Second, CacheTTL: 24 * time.Hour}
}

var hackageName = `^[a-z0-9][a-z0-9-]*$`

func (h *Hackage) Normalize(raw string) (string, error) {
	return validate(raw, hackageName, 100)
}

func (h *Hackage) Homepage(q string) string {
	return "https://hackage.haskell.org/package/" + url.PathEscape(q)
}

func (h *Hackage) Check(ctx context.Context, q string) (check.Result, error) {
	u := check.JoinURL(h.base, "/package/"+url.PathEscape(q))
	code, header, err := h.http.Head(ctx, u)
	if err != nil {
		return check.Result{}, err
	}
	switch code {
	case 200:
		return taken(q, h.Homepage(q), "published",
			check.Field{Key: "package", Value: q},
			check.Field{Key: "note", Value: "Hackage package page exists"},
		), nil
	case 404:
		return available(q, h.Homepage(q), check.Field{Key: "registry", Value: "Hackage"}), nil
	default:
		return check.Result{}, httpStatusError(code, u, header)
	}
}
