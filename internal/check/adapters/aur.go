package adapters

import (
	"context"
	"net/url"
	"time"

	"github.com/yufeiminds/nameary/internal/check"
)

// AUR checks the Arch User Repository.
type AUR struct {
	http *check.HTTP
	base string
}

func NewAUR(h *check.HTTP, base string) *AUR {
	if base == "" {
		base = "https://aur.archlinux.org"
	}
	return &AUR{http: h, base: base}
}

func (a *AUR) Target() check.Target     { return check.TargetAUR }
func (a *AUR) Label() string            { return "aur" }
func (a *AUR) Category() check.Category { return check.CategoryPackages }
func (a *AUR) Policy() check.Policy {
	return check.Policy{Priority: 44, Concurrency: 4, Timeout: 8 * time.Second, CacheTTL: 24 * time.Hour}
}

var aurName = `^[a-z0-9][a-z0-9@._+-]*$`

func (a *AUR) Normalize(raw string) (string, error) {
	return validate(raw, aurName, 100)
}

func (a *AUR) Homepage(q string) string {
	return "https://aur.archlinux.org/packages/" + url.PathEscape(q)
}

func (a *AUR) Check(ctx context.Context, q string) (check.Result, error) {
	u := check.JoinURL(a.base, "/rpc/v5/info?arg[]="+url.QueryEscape(q)+"&by=name")
	var payload struct {
		ResultCount int `json:"resultcount"`
		Results     []struct {
			Name           string  `json:"Name"`
			Version        string  `json:"Version"`
			Description    string  `json:"Description"`
			NumVotes       int     `json:"NumVotes"`
			Popularity     float64 `json:"Popularity"`
			OutOfDate      *int64  `json:"OutOfDate"`
			FirstSubmitted int64   `json:"FirstSubmitted"`
			LastModified   int64   `json:"LastModified"`
		} `json:"results"`
	}
	code, header, err := a.http.Get(ctx, u, &payload)
	if err != nil {
		return check.Result{}, err
	}
	switch code {
	case 200:
		if payload.ResultCount == 0 || len(payload.Results) == 0 {
			return available(q, a.Homepage(q), check.Field{Key: "registry", Value: "AUR"}), nil
		}
		r := payload.Results[0]
		fields := []check.Field{
			{Key: "package", Value: r.Name},
			{Key: "latest", Value: r.Version},
			{Key: "votes", Value: humanCount(int64(r.NumVotes))},
			{Key: "popularity", Value: formatFloat(r.Popularity)},
			{Key: "description", Value: r.Description},
		}
		if r.OutOfDate != nil {
			fields = append(fields, check.Field{Key: "out of date", Value: "yes"})
		}
		return taken(q, a.Homepage(q), "v"+r.Version, fields...), nil
	default:
		return check.Result{}, httpStatusError(code, u, header)
	}
}
