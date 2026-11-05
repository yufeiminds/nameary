package adapters

import (
	"context"
	"net/url"
	"time"

	"github.com/yufeiminds/nameary/internal/check"
)

// Maven checks Maven Central by artifact ID. A bare name has no group, so this
// searches all groups; any hit means the artifact name is already in use.
type Maven struct {
	http *check.HTTP
	base string
}

func NewMaven(h *check.HTTP, base string) *Maven {
	if base == "" {
		base = "https://search.maven.org"
	}
	return &Maven{http: h, base: base}
}

func (m *Maven) Target() check.Target     { return check.TargetMaven }
func (m *Maven) Label() string            { return "maven" }
func (m *Maven) Category() check.Category { return check.CategoryPackages }
func (m *Maven) Policy() check.Policy {
	return check.Policy{Priority: 42, Concurrency: 4, Timeout: 8 * time.Second, CacheTTL: 24 * time.Hour}
}

var mavenName = `^[a-z0-9][a-z0-9._-]*$`

func (m *Maven) Normalize(raw string) (string, error) {
	return validate(raw, mavenName, 100)
}

func (m *Maven) Homepage(q string) string {
	return "https://central.sonatype.com/search?q=" + url.QueryEscape(q)
}

func (m *Maven) Check(ctx context.Context, q string) (check.Result, error) {
	query := `a:"` + q + `"`
	u := check.JoinURL(m.base, "/solrsearch/select?q="+url.QueryEscape(query)+"&rows=1&wt=json")
	var payload struct {
		Response struct {
			NumFound int `json:"numFound"`
			Docs     []struct {
				ID            string `json:"id"`
				Group         string `json:"g"`
				Artifact      string `json:"a"`
				LatestVersion string `json:"latestVersion"`
				VersionCount  int    `json:"versionCount"`
			} `json:"docs"`
		} `json:"response"`
	}
	code, header, err := m.http.Get(ctx, u, &payload)
	if err != nil {
		return check.Result{}, err
	}
	switch code {
	case 200:
		if payload.Response.NumFound == 0 || len(payload.Response.Docs) == 0 {
			return available(q, m.Homepage(q), check.Field{Key: "registry", Value: "Maven Central"}), nil
		}
		doc := payload.Response.Docs[0]
		summary := "published"
		if doc.LatestVersion != "" {
			summary = "v" + doc.LatestVersion
		}
		return taken(q, m.Homepage(q), summary,
			check.Field{Key: "artifact", Value: doc.Artifact},
			check.Field{Key: "group", Value: doc.Group},
			check.Field{Key: "latest", Value: doc.LatestVersion},
			check.Field{Key: "versions", Value: humanCount(int64(doc.VersionCount))},
			check.Field{Key: "coordinates", Value: doc.ID},
		), nil
	default:
		return check.Result{}, httpStatusError(code, u, header)
	}
}
