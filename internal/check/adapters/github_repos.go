package adapters

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/yufeiminds/nameary/internal/check"
)

// GitHubRepos searches GitHub repositories for an exact name. It is an
// advisory signal, so a match is reported as conflict rather than taken.
type GitHubRepos struct {
	http     *check.HTTP
	apiBase  string
	token    string
	minStars int
}

// NewGitHubRepos returns a GitHub repository-name search checker.
func NewGitHubRepos(h *check.HTTP, apiBase, token string, minStars int) *GitHubRepos {
	if h == nil {
		h = check.NewHTTP()
	}
	if apiBase == "" {
		apiBase = "https://api.github.com"
	}
	if minStars <= 0 {
		minStars = 1000
	}
	return &GitHubRepos{http: h, apiBase: apiBase, token: token, minStars: minStars}
}

func (g *GitHubRepos) Target() check.Target     { return check.TargetGitHubRepos }
func (g *GitHubRepos) Label() string            { return "github-repos" }
func (g *GitHubRepos) Category() check.Category { return check.CategoryHandles }

func (g *GitHubRepos) Policy() check.Policy {
	return check.Policy{Priority: 31, Concurrency: 1, Timeout: 8 * time.Second, CacheTTL: 24 * time.Hour}
}

var githubRepoName = `^[a-z0-9][a-z0-9._-]*$`

func (g *GitHubRepos) Normalize(raw string) (string, error) {
	return validate(raw, githubRepoName, 100)
}

func (g *GitHubRepos) Homepage(q string) string {
	return "https://github.com/search?q=" + url.QueryEscape(q+" in:name") + "&type=repositories"
}

type githubRepo struct {
	Name        string `json:"name"`
	FullName    string `json:"full_name"`
	HTMLURL     string `json:"html_url"`
	Description string `json:"description"`
	Language    string `json:"language"`
	Stars       int64  `json:"stargazers_count"`
	Forks       int64  `json:"forks_count"`
	Fork        bool   `json:"fork"`
	Archived    bool   `json:"archived"`
	UpdatedAt   string `json:"updated_at"`
}

func (g *GitHubRepos) Check(ctx context.Context, q string) (check.Result, error) {
	q, err := g.Normalize(q)
	if err != nil {
		return check.Result{}, err
	}

	query := fmt.Sprintf(`"%s" in:name`, q)
	if g.minStars > 0 {
		query += fmt.Sprintf(" stars:>=%d", g.minStars)
	}
	params := url.Values{}
	params.Set("q", query)
	params.Set("sort", "stars")
	params.Set("order", "desc")
	params.Set("per_page", "100")
	u := check.JoinURL(g.apiBase, "/search/repositories?"+params.Encode())

	headers := map[string]string{
		"Accept":               "application/vnd.github+json",
		"X-GitHub-Api-Version": "2022-11-28",
	}
	if g.token != "" {
		headers["Authorization"] = "Bearer " + g.token
	}

	var payload struct {
		TotalCount        int          `json:"total_count"`
		IncompleteResults bool         `json:"incomplete_results"`
		Items             []githubRepo `json:"items"`
	}
	code, header, err := g.http.GetWithHeaders(ctx, u, headers, &payload)
	if err != nil {
		return check.Result{}, err
	}
	switch code {
	case http.StatusOK:
	case http.StatusForbidden, http.StatusTooManyRequests:
		if header.Get("X-RateLimit-Remaining") == "0" || code == http.StatusTooManyRequests {
			return check.Result{}, check.NewHTTPError(http.StatusTooManyRequests, u, header)
		}
		return check.Result{}, httpStatusError(code, u, header)
	default:
		return check.Result{}, httpStatusError(code, u, header)
	}

	matches := make([]githubRepo, 0, len(payload.Items))
	for _, item := range payload.Items {
		if strings.EqualFold(item.Name, q) {
			matches = append(matches, item)
		}
	}
	sort.SliceStable(matches, func(i, j int) bool { return matches[i].Stars > matches[j].Stars })

	if len(matches) == 0 {
		if payload.IncompleteResults {
			return check.Result{
				Status:     check.StatusUnknown,
				Confidence: check.Low,
				Query:      q,
				Summary:    "project search incomplete",
				SourceURL:  g.Homepage(q),
				FetchedAt:  time.Now(),
				Fields: []check.Field{
					{Key: "threshold", Value: fmt.Sprintf("%d stars", g.minStars)},
					{Key: "note", Value: "GitHub reported incomplete search results; no exact match was found in the returned page."},
				},
			}, nil
		}
		return check.Result{
			Status:     check.StatusAvailable,
			Confidence: check.Likely,
			Query:      q,
			Summary:    "no high-star project",
			SourceURL:  g.Homepage(q),
			FetchedAt:  time.Now(),
			Fields: []check.Field{
				{Key: "threshold", Value: fmt.Sprintf("%d stars", g.minStars)},
				{Key: "matches", Value: "0"},
			},
		}, nil
	}

	top := matches[0]
	fields := []check.Field{
		{Key: "conflicts", Value: conflictList(matches, 10)},
		{Key: "top_project", Value: top.FullName},
		{Key: "stars", Value: humanCount(top.Stars)},
		{Key: "threshold", Value: fmt.Sprintf("%d stars", g.minStars)},
		{Key: "matches", Value: fmt.Sprintf("%d", len(matches))},
		{Key: "url", Value: top.HTMLURL},
		{Key: "language", Value: top.Language},
		{Key: "fork", Value: fmt.Sprintf("%t", top.Fork)},
		{Key: "archived", Value: fmt.Sprintf("%t", top.Archived)},
	}
	if top.Description != "" {
		fields = append(fields, check.Field{Key: "description", Value: top.Description})
	}
	if top.UpdatedAt != "" {
		fields = append(fields, check.Field{Key: "updated", Value: top.UpdatedAt})
	}
	return check.Result{
		Status:     check.StatusConflict,
		Confidence: check.Certain,
		Query:      q,
		Summary:    conflictSummary(matches),
		SourceURL:  top.HTMLURL,
		FetchedAt:  time.Now(),
		Fields:     fields,
	}, nil
}

// conflictSummary keeps the top projects visible in one-line table and row
// summaries, while the detail fields carry the full conflict list.
func conflictSummary(matches []githubRepo) string {
	const shown = 3
	parts := make([]string, 0, shown+1)
	for i := 0; i < len(matches) && i < shown; i++ {
		parts = append(parts, fmt.Sprintf("%s (%s stars)", matches[i].FullName, humanCount(matches[i].Stars)))
	}
	if len(matches) > shown {
		parts = append(parts, fmt.Sprintf("+%d more", len(matches)-shown))
	}
	return strings.Join(parts, ", ")
}

func conflictList(matches []githubRepo, limit int) string {
	if limit <= 0 || limit > len(matches) {
		limit = len(matches)
	}
	parts := make([]string, 0, limit+1)
	for i := 0; i < limit; i++ {
		parts = append(parts, fmt.Sprintf("%s (%s stars)", matches[i].FullName, humanCount(matches[i].Stars)))
	}
	if len(matches) > limit {
		parts = append(parts, fmt.Sprintf("+%d more", len(matches)-limit))
	}
	return strings.Join(parts, "; ")
}
