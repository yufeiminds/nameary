package adapters

import (
	"context"
	"net/url"
	"strings"
	"time"

	"github.com/yufeiminds/nameary/internal/check"
)

// Packagist checks Composer packages. It expects vendor/package; a bare name
// is expanded to name/name.
type Packagist struct {
	http *check.HTTP
	base string
}

func NewPackagist(h *check.HTTP, base string) *Packagist {
	if base == "" {
		base = "https://repo.packagist.org"
	}
	return &Packagist{http: h, base: base}
}

func (p *Packagist) Target() check.Target     { return check.TargetPackagist }
func (p *Packagist) Label() string            { return "composer" }
func (p *Packagist) Category() check.Category { return check.CategoryPackages }
func (p *Packagist) Policy() check.Policy {
	return check.Policy{Priority: 70, Concurrency: 2, Timeout: 12 * time.Second, CacheTTL: 24 * time.Hour}
}

var composerName = `^[a-z0-9]([a-z0-9_.-]*)/([a-z0-9]([a-z0-9_.-]*))$`

func (p *Packagist) Normalize(raw string) (string, error) {
	q := strings.ToLower(strings.TrimSpace(raw))
	if !strings.Contains(q, "/") {
		q = q + "/" + q
	}
	return validate(q, composerName, 200)
}

func (p *Packagist) Homepage(q string) string {
	return "https://packagist.org/packages/" + q
}

func (p *Packagist) Check(ctx context.Context, q string) (check.Result, error) {
	u := check.JoinURL(p.base, "/p2/"+url.PathEscape(q)+".json")
	var payload struct {
		Packages map[string][]struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Version     string `json:"version"`
		} `json:"packages"`
	}
	code, header, err := p.http.Get(ctx, u, &payload)
	if err != nil {
		return check.Result{}, err
	}
	switch code {
	case 200:
		versions := payload.Packages[q]
		if len(versions) == 0 {
			return available(q, p.Homepage(q), check.Field{Key: "registry", Value: "Packagist"}), nil
		}
		latest := versions[0]
		summary := "published"
		if latest.Version != "" {
			summary = "v" + latest.Version
		}
		return taken(q, p.Homepage(q), summary,
			check.Field{Key: "package", Value: latest.Name},
			check.Field{Key: "latest", Value: latest.Version},
			check.Field{Key: "versions", Value: humanCount(int64(len(versions)))},
			check.Field{Key: "description", Value: latest.Description},
		), nil
	case 404:
		return available(q, p.Homepage(q), check.Field{Key: "registry", Value: "Packagist"}), nil
	default:
		return check.Result{}, httpStatusError(code, u, header)
	}
}
