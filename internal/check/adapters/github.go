package adapters

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/yufeiminds/nameary/internal/check"
)

// GitHub checks the availability of a user/org handle. With a token it uses
// the REST API; without one it falls back to the public HTML page so a single
// run does not exhaust the 60/hour anonymous quota.
type GitHub struct {
	http    *check.HTTP
	apiBase string
	webBase string
	token   string
}

func NewGitHub(h *check.HTTP, apiBase, token string) *GitHub {
	if apiBase == "" {
		apiBase = "https://api.github.com"
	}
	return &GitHub{http: h, apiBase: apiBase, webBase: "https://github.com", token: token}
}

func (g *GitHub) Target() check.Target     { return check.TargetGitHub }
func (g *GitHub) Label() string            { return "github" }
func (g *GitHub) Category() check.Category { return check.CategoryHandles }

func (g *GitHub) Policy() check.Policy {
	concurrency := 1
	if g.token != "" {
		concurrency = 4
	}
	return check.Policy{Priority: 30, Concurrency: concurrency, Timeout: 8 * time.Second, CacheTTL: 12 * time.Hour}
}

var githubName = `^[a-z0-9-]+$`

var githubReserved = map[string]bool{
	"about": true, "account": true, "admin": true, "api": true, "apps": true,
	"blog": true, "business": true, "collections": true, "contact": true,
	"customer-stories": true, "dashboard": true, "developer": true, "education": true,
	"enterprise": true, "events": true, "explore": true, "features": true,
	"gist": true, "github": true, "help": true, "home": true, "issues": true,
	"join": true, "login": true, "logout": true, "marketplace": true,
	"new": true, "notifications": true, "organizations": true, "orgs": true,
	"pricing": true, "pulls": true, "readme": true, "search": true,
	"security": true, "sessions": true, "settings": true, "site": true,
	"sponsors": true, "stars": true, "topics": true, "trending": true,
	"users": true, "watching": true,
}

func (g *GitHub) Normalize(raw string) (string, error) {
	q, err := validate(raw, githubName, 39)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(q, "-") || strings.HasSuffix(q, "-") || strings.Contains(q, "--") {
		return "", fmt.Errorf("GitHub handles cannot start or end with a hyphen, or contain two in a row")
	}
	return q, nil
}

func (g *GitHub) Homepage(q string) string {
	return g.webBase + "/" + q
}

func (g *GitHub) Check(ctx context.Context, q string) (check.Result, error) {
	if githubReserved[q] {
		return check.Result{
			Status:     check.StatusTaken,
			Confidence: check.Certain,
			Query:      q,
			Summary:    "reserved by GitHub",
			SourceURL:  g.Homepage(q),
			FetchedAt:  time.Now(),
			Fields:     []check.Field{{Key: "reason", Value: "reserved path, not a claimable handle"}},
		}, nil
	}
	if g.token != "" {
		return g.checkAPI(ctx, q)
	}
	return g.checkHTML(ctx, q)
}

func (g *GitHub) checkAPI(ctx context.Context, q string) (check.Result, error) {
	u := check.JoinURL(g.apiBase, "/users/"+url.PathEscape(q))
	headers := map[string]string{
		"Authorization":        "Bearer " + g.token,
		"Accept":               "application/vnd.github+json",
		"X-GitHub-Api-Version": "2022-11-28",
	}
	var payload struct {
		Login       string `json:"login"`
		Type        string `json:"type"`
		Name        string `json:"name"`
		Bio         string `json:"bio"`
		PublicRepos int    `json:"public_repos"`
		Followers   int    `json:"followers"`
		CreatedAt   string `json:"created_at"`
	}
	code, header, err := g.http.GetWithHeaders(ctx, u, headers, &payload)
	if err != nil {
		return check.Result{}, err
	}
	switch code {
	case 200:
		kind := strings.ToLower(payload.Type)
		if kind == "" {
			kind = "account"
		}
		summary := kind + " · " + strconv.Itoa(payload.PublicRepos) + " repos"
		return taken(q, g.Homepage(q), summary,
			check.Field{Key: "login", Value: payload.Login},
			check.Field{Key: "type", Value: payload.Type},
			check.Field{Key: "name", Value: payload.Name},
			check.Field{Key: "bio", Value: payload.Bio},
			check.Field{Key: "repos", Value: strconv.Itoa(payload.PublicRepos)},
			check.Field{Key: "followers", Value: strconv.Itoa(payload.Followers)},
			check.Field{Key: "created", Value: payload.CreatedAt},
		), nil
	case 404:
		return available(q, g.Homepage(q), check.Field{Key: "registry", Value: "GitHub"}), nil
	case 403, 429:
		if header.Get("X-RateLimit-Remaining") == "0" || code == 429 {
			return check.Result{}, check.NewHTTPError(http.StatusTooManyRequests, u, header)
		}
		return check.Result{}, httpStatusError(code, u, header)
	default:
		return check.Result{}, httpStatusError(code, u, header)
	}
}

func (g *GitHub) checkHTML(ctx context.Context, q string) (check.Result, error) {
	u := g.webBase + "/" + url.PathEscape(q)
	code, header, err := g.http.Head(ctx, u)
	if err != nil {
		return check.Result{}, err
	}
	switch code {
	case 200:
		return taken(q, g.Homepage(q), "handle exists",
			check.Field{Key: "note", Value: "checked via public profile page (no token)"},
		), nil
	case 404:
		return available(q, g.Homepage(q),
			check.Field{Key: "registry", Value: "GitHub"},
			check.Field{Key: "note", Value: "checked via public profile page (no token)"},
		), nil
	case 429:
		return check.Result{}, check.NewHTTPError(http.StatusTooManyRequests, u, header)
	default:
		if code >= 500 {
			return check.Result{}, httpStatusError(code, u, header)
		}
		return check.Result{}, httpStatusError(code, u, header)
	}
}
