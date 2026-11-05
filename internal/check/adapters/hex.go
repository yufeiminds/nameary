package adapters

import (
	"context"
	"net/url"
	"time"

	"github.com/yufeiminds/nameary/internal/check"
)

// Hex checks hex.pm.
type Hex struct {
	http *check.HTTP
	base string
}

func NewHex(h *check.HTTP, base string) *Hex {
	if base == "" {
		base = "https://hex.pm"
	}
	return &Hex{http: h, base: base}
}

func (h *Hex) Target() check.Target     { return check.TargetHex }
func (h *Hex) Label() string            { return "hex" }
func (h *Hex) Category() check.Category { return check.CategoryPackages }
func (h *Hex) Policy() check.Policy {
	return check.Policy{Priority: 48, Concurrency: 4, Timeout: 8 * time.Second, CacheTTL: 24 * time.Hour}
}

var hexName = `^[a-z0-9][a-z0-9_]*$`

func (h *Hex) Normalize(raw string) (string, error) {
	return validate(raw, hexName, 100)
}

func (h *Hex) Homepage(q string) string {
	return "https://hex.pm/packages/" + q
}

func (h *Hex) Check(ctx context.Context, q string) (check.Result, error) {
	u := check.JoinURL(h.base, "/api/packages/"+url.PathEscape(q))
	var payload struct {
		Name          string `json:"name"`
		LatestVersion string `json:"latest_version"`
		Downloads     struct {
			All int64 `json:"all"`
		} `json:"downloads"`
		Meta struct {
			Description string `json:"description"`
		} `json:"meta"`
	}
	code, header, err := h.http.Get(ctx, u, &payload)
	if err != nil {
		return check.Result{}, err
	}
	switch code {
	case 200:
		return taken(q, h.Homepage(q), "v"+payload.LatestVersion,
			check.Field{Key: "package", Value: payload.Name},
			check.Field{Key: "latest", Value: payload.LatestVersion},
			check.Field{Key: "downloads", Value: humanCount(payload.Downloads.All)},
			check.Field{Key: "description", Value: payload.Meta.Description},
		), nil
	case 404:
		return available(q, h.Homepage(q), check.Field{Key: "registry", Value: "Hex"}), nil
	default:
		return check.Result{}, httpStatusError(code, u, header)
	}
}
