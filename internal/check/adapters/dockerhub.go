package adapters

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/yufeiminds/nameary/internal/check"
)

// DockerHub checks namespace and official-image availability.
type DockerHub struct {
	http *check.HTTP
	base string
}

func NewDockerHub(h *check.HTTP, base string) *DockerHub {
	if base == "" {
		base = "https://hub.docker.com"
	}
	return &DockerHub{http: h, base: base}
}

func (d *DockerHub) Target() check.Target     { return check.TargetDocker }
func (d *DockerHub) Label() string            { return "docker" }
func (d *DockerHub) Category() check.Category { return check.CategoryPackages }
func (d *DockerHub) Policy() check.Policy {
	return check.Policy{Priority: 50, Concurrency: 4, Timeout: 8 * time.Second, CacheTTL: 24 * time.Hour}
}

var dockerName = `^[a-z0-9][a-z0-9._-]*$`

func (d *DockerHub) Normalize(raw string) (string, error) {
	return validate(raw, dockerName, 100)
}

func (d *DockerHub) Homepage(q string) string {
	return "https://hub.docker.com/u/" + q
}

func (d *DockerHub) Check(ctx context.Context, q string) (check.Result, error) {
	type probe struct {
		name string
		url  string
		kind string
	}
	probes := []probe{
		{"user", check.JoinURL(d.base, "/v2/users/"+url.PathEscape(q)+"/"), "user namespace"},
		{"org", check.JoinURL(d.base, "/v2/orgs/"+url.PathEscape(q)+"/"), "organization namespace"},
		{"official", check.JoinURL(d.base, "/v2/repositories/library/"+url.PathEscape(q)+"/"), "official image"},
	}
	for _, p := range probes {
		var payload map[string]any
		code, header, err := d.http.Get(ctx, p.url, &payload)
		if err != nil {
			return check.Result{}, err
		}
		switch code {
		case 200:
			summary := p.kind
			if p.name == "official" {
				if pulls, ok := payload["pull_count"]; ok {
					summary = fmt.Sprintf("official image · %v pulls", pulls)
				}
			}
			return taken(q, d.Homepage(q), summary,
				check.Field{Key: "namespace", Value: q},
				check.Field{Key: "found", Value: p.kind},
			), nil
		case 404:
			continue
		case 429:
			return check.Result{}, httpStatusError(code, p.url, header)
		default:
			if code >= 500 {
				return check.Result{}, httpStatusError(code, p.url, header)
			}
			// Docker Hub sometimes answers 3xx/4xx for odd names; keep probing
			// other namespaces rather than declaring the name free.
			continue
		}
	}
	return available(q, d.Homepage(q), check.Field{Key: "registry", Value: "Docker Hub"}), nil
}
