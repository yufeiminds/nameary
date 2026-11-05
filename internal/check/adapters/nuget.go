package adapters

import (
	"context"
	"net/url"
	"time"

	"github.com/yufeiminds/nameary/internal/check"
)

// NuGet checks nuget.org's flat-container API.
type NuGet struct {
	http *check.HTTP
	base string
}

func NewNuGet(h *check.HTTP, base string) *NuGet {
	if base == "" {
		base = "https://api.nuget.org"
	}
	return &NuGet{http: h, base: base}
}

func (n *NuGet) Target() check.Target     { return check.TargetNuGet }
func (n *NuGet) Label() string            { return "nuget" }
func (n *NuGet) Category() check.Category { return check.CategoryPackages }
func (n *NuGet) Policy() check.Policy {
	return check.Policy{Priority: 44, Concurrency: 4, Timeout: 8 * time.Second, CacheTTL: 24 * time.Hour}
}

var nugetName = `^[a-z0-9][a-z0-9._-]*$`

func (n *NuGet) Normalize(raw string) (string, error) {
	return validate(raw, nugetName, 100)
}

func (n *NuGet) Homepage(q string) string {
	return "https://www.nuget.org/packages/" + q
}

func (n *NuGet) Check(ctx context.Context, q string) (check.Result, error) {
	u := check.JoinURL(n.base, "/v3-flatcontainer/"+url.PathEscape(q)+"/index.json")
	var payload struct {
		Versions []string `json:"versions"`
	}
	code, header, err := n.http.Get(ctx, u, &payload)
	if err != nil {
		return check.Result{}, err
	}
	switch code {
	case 200:
		latest := ""
		if len(payload.Versions) > 0 {
			latest = payload.Versions[len(payload.Versions)-1]
		}
		return taken(q, n.Homepage(q), "v"+latest,
			check.Field{Key: "package", Value: q},
			check.Field{Key: "latest", Value: latest},
			check.Field{Key: "versions", Value: humanCount(int64(len(payload.Versions)))},
		), nil
	case 404:
		return available(q, n.Homepage(q), check.Field{Key: "registry", Value: "NuGet"}), nil
	default:
		return check.Result{}, httpStatusError(code, u, header)
	}
}
