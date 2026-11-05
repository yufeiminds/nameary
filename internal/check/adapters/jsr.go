package adapters

import (
	"context"
	"net/url"
	"strings"
	"time"

	"github.com/yufeiminds/nameary/internal/check"
)

// JSR checks the JavaScript Registry. A bare name is searched across scopes;
// a scoped @scope/name is checked directly.
type JSR struct {
	http *check.HTTP
	base string
}

func NewJSR(h *check.HTTP, base string) *JSR {
	if base == "" {
		base = "https://jsr.io"
	}
	return &JSR{http: h, base: base}
}

func (j *JSR) Target() check.Target     { return check.TargetJSR }
func (j *JSR) Label() string            { return "jsr" }
func (j *JSR) Category() check.Category { return check.CategoryPackages }
func (j *JSR) Policy() check.Policy {
	return check.Policy{Priority: 54, Concurrency: 2, Timeout: 8 * time.Second, CacheTTL: 24 * time.Hour}
}

var (
	jsrName       = `^[a-z0-9][a-z0-9-]*$`
	jsrScopedName = `^@[a-z0-9][a-z0-9-]*/[a-z0-9][a-z0-9-]*$`
)

func (j *JSR) Normalize(raw string) (string, error) {
	q := strings.ToLower(strings.TrimSpace(raw))
	if strings.HasPrefix(q, "@") {
		return validatePattern(q, jsrScopedName)
	}
	return validatePattern(q, jsrName)
}

func (j *JSR) Homepage(q string) string {
	if strings.HasPrefix(q, "@") {
		return "https://jsr.io/" + q
	}
	return "https://jsr.io/search?q=" + url.QueryEscape(q)
}

func (j *JSR) Check(ctx context.Context, q string) (check.Result, error) {
	if strings.HasPrefix(q, "@") {
		return j.checkScoped(ctx, q)
	}
	u := check.JoinURL(j.base, "/api/packages?query="+url.QueryEscape(q))
	var payload struct {
		Items []struct {
			Scope       string `json:"scope"`
			Name        string `json:"name"`
			Description string `json:"description"`
			Latest      string `json:"latest"`
			Repository  struct {
				Owner string `json:"owner"`
				Name  string `json:"name"`
			} `json:"githubRepository"`
		} `json:"items"`
	}
	code, header, err := j.http.Get(ctx, u, &payload)
	if err != nil {
		return check.Result{}, err
	}
	if code != 200 {
		return check.Result{}, httpStatusError(code, u, header)
	}
	for _, item := range payload.Items {
		if !strings.EqualFold(item.Name, q) {
			continue
		}
		full := "@" + item.Scope + "/" + item.Name
		return taken(q, "https://jsr.io/"+full, "published",
			check.Field{Key: "package", Value: full},
			check.Field{Key: "scope", Value: item.Scope},
			check.Field{Key: "latest", Value: item.Latest},
			check.Field{Key: "description", Value: item.Description},
			check.Field{Key: "repository", Value: item.Repository.Owner + "/" + item.Repository.Name},
		), nil
	}
	if len(payload.Items) >= 100 {
		return check.Result{
			Status:     check.StatusUnknown,
			Confidence: check.Low,
			Query:      q,
			Summary:    "search truncated",
			SourceURL:  j.Homepage(q),
			FetchedAt:  time.Now(),
			Fields: []check.Field{
				{Key: "detail", Value: "JSR returned 100 fuzzy matches without an exact package name; the search may be truncated."},
			},
		}, nil
	}
	return available(q, j.Homepage(q), check.Field{Key: "registry", Value: "JSR"}), nil
}

func (j *JSR) checkScoped(ctx context.Context, q string) (check.Result, error) {
	u := check.JoinURL(j.base, "/"+q+"/meta.json")
	var payload struct {
		Scope  string `json:"scope"`
		Name   string `json:"name"`
		Latest string `json:"latest"`
		Repo   struct {
			Owner string `json:"owner"`
			Name  string `json:"name"`
		} `json:"githubRepository"`
	}
	code, header, err := j.http.Get(ctx, u, &payload)
	if err != nil {
		return check.Result{}, err
	}
	switch code {
	case 200:
		return taken(q, "https://jsr.io/"+q, "v"+payload.Latest,
			check.Field{Key: "package", Value: "@" + payload.Scope + "/" + payload.Name},
			check.Field{Key: "latest", Value: payload.Latest},
			check.Field{Key: "repository", Value: payload.Repo.Owner + "/" + payload.Repo.Name},
		), nil
	case 404:
		return available(q, j.Homepage(q), check.Field{Key: "registry", Value: "JSR"}), nil
	default:
		return check.Result{}, httpStatusError(code, u, header)
	}
}
