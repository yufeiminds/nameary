package adapters

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/yufeiminds/nameary/internal/check"
)

// NPM checks the public npm registry.
type NPM struct {
	http *check.HTTP
	base string
}

func NewNPM(h *check.HTTP, base string) *NPM {
	if base == "" {
		base = "https://registry.npmjs.org"
	}
	return &NPM{http: h, base: base}
}

func (n *NPM) Target() check.Target     { return check.TargetNPM }
func (n *NPM) Label() string            { return "npm" }
func (n *NPM) Category() check.Category { return check.CategoryPackages }
func (n *NPM) Policy() check.Policy {
	return check.Policy{Priority: 20, Concurrency: 6, Timeout: 8 * time.Second, CacheTTL: 24 * time.Hour}
}

var npmName = `^(@[a-z0-9][a-z0-9._-]*/)?[a-z0-9][a-z0-9._-]*$`

func (n *NPM) Normalize(raw string) (string, error) {
	q, err := validate(raw, npmName, 214)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(q, ".") || strings.HasPrefix(q, "_") {
		return "", fmt.Errorf("npm names cannot start with %q", q[0])
	}
	return q, nil
}

func (n *NPM) Homepage(q string) string {
	return "https://www.npmjs.com/package/" + q
}

func (n *NPM) Check(ctx context.Context, q string) (check.Result, error) {
	u := check.JoinURL(n.base, url.PathEscape(q)+"/latest")
	var p struct {
		Name        string `json:"name"`
		Version     string `json:"version"`
		Description string `json:"description"`
	}
	code, header, err := n.http.Get(ctx, u, &p)
	if err != nil {
		return check.Result{}, err
	}
	switch code {
	case 200:
		latest := p.Version
		fields := []check.Field{
			{Key: "package", Value: firstNonEmpty(p.Name, q)},
			{Key: "latest", Value: latest},
			{Key: "description", Value: p.Description},
		}
		summary := "registered"
		if latest != "" {
			summary = "v" + latest
		}
		return taken(q, n.Homepage(q), summary, fields...), nil
	case 404:
		return available(q, n.Homepage(q), check.Field{Key: "registry", Value: "npm"}), nil
	default:
		return check.Result{}, httpStatusError(code, u, header)
	}
}
