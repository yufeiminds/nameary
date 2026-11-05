package adapters

import (
	"context"
	"net/url"
	"strings"
	"time"

	"github.com/yufeiminds/nameary/internal/check"
)

// Crates checks crates.io.
type Crates struct {
	http *check.HTTP
	base string
}

func NewCrates(h *check.HTTP, base string) *Crates {
	if base == "" {
		base = "https://crates.io"
	}
	return &Crates{http: h, base: base}
}

func (c *Crates) Target() check.Target     { return check.TargetCrates }
func (c *Crates) Label() string            { return "crates" }
func (c *Crates) Category() check.Category { return check.CategoryPackages }
func (c *Crates) Policy() check.Policy {
	return check.Policy{Priority: 40, Concurrency: 1, Timeout: 10 * time.Second, CacheTTL: 24 * time.Hour}
}

var crateName = `^[a-z0-9][a-z0-9_-]*$`

func (c *Crates) Normalize(raw string) (string, error) {
	return validate(raw, crateName, 64)
}

func (c *Crates) Homepage(q string) string {
	return "https://crates.io/crates/" + q
}

func (c *Crates) Check(ctx context.Context, q string) (check.Result, error) {
	res, found, err := c.lookup(ctx, q)
	if err != nil {
		return check.Result{}, err
	}
	if found {
		return res, nil
	}

	// Cargo treats "-" and "_" as equivalent for dependency resolution. If the
	// exact spelling is free but its variant is taken, say so instead of
	// pretending the name is completely clear.
	variant := ""
	switch {
	case strings.Contains(q, "-"):
		variant = strings.ReplaceAll(q, "-", "_")
	case strings.Contains(q, "_"):
		variant = strings.ReplaceAll(q, "_", "-")
	}
	if variant != "" && variant != q {
		if _, vfound, verr := c.lookup(ctx, variant); verr == nil && vfound {
			return available(q, c.Homepage(q),
				check.Field{Key: "registry", Value: "crates.io"},
				check.Field{Key: "note", Value: "the " + variant + " variant is already taken"},
			), nil
		}
	}
	return available(q, c.Homepage(q), check.Field{Key: "registry", Value: "crates.io"}), nil
}

func (c *Crates) lookup(ctx context.Context, q string) (check.Result, bool, error) {
	u := check.JoinURL(c.base, "/api/v1/crates/"+url.PathEscape(q))
	var payload struct {
		Crate struct {
			Name        string `json:"name"`
			MaxVersion  string `json:"max_version"`
			Downloads   int64  `json:"downloads"`
			Description string `json:"description"`
			UpdatedAt   string `json:"updated_at"`
		} `json:"crate"`
	}
	code, header, err := c.http.Get(ctx, u, &payload)
	if err != nil {
		return check.Result{}, false, err
	}
	switch code {
	case 200:
		return taken(q, c.Homepage(q), "v"+payload.Crate.MaxVersion,
			check.Field{Key: "package", Value: payload.Crate.Name},
			check.Field{Key: "latest", Value: payload.Crate.MaxVersion},
			check.Field{Key: "downloads", Value: humanCount(payload.Crate.Downloads)},
			check.Field{Key: "description", Value: payload.Crate.Description},
			check.Field{Key: "updated", Value: payload.Crate.UpdatedAt},
		), true, nil
	case 404:
		return check.Result{}, false, nil
	default:
		return check.Result{}, false, httpStatusError(code, u, header)
	}
}
