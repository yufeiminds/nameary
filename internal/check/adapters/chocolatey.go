package adapters

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/yufeiminds/nameary/internal/check"
)

// Chocolatey checks the Chocolatey community feed. Its OData endpoint returns
// an Atom XML feed; an empty feed means the package ID is free.
type Chocolatey struct {
	http *check.HTTP
	base string
}

func NewChocolatey(h *check.HTTP, base string) *Chocolatey {
	if base == "" {
		base = "https://community.chocolatey.org"
	}
	return &Chocolatey{http: h, base: base}
}

func (c *Chocolatey) Target() check.Target     { return check.TargetChocolatey }
func (c *Chocolatey) Label() string            { return "choco" }
func (c *Chocolatey) Category() check.Category { return check.CategoryPackages }
func (c *Chocolatey) Policy() check.Policy {
	return check.Policy{Priority: 52, Concurrency: 3, Timeout: 10 * time.Second, CacheTTL: 24 * time.Hour}
}

var chocoName = `^[a-z0-9][a-z0-9._-]*$`

func (c *Chocolatey) Normalize(raw string) (string, error) {
	return validate(raw, chocoName, 100)
}

func (c *Chocolatey) Homepage(q string) string {
	return "https://community.chocolatey.org/packages/" + url.PathEscape(q)
}

func (c *Chocolatey) Check(ctx context.Context, q string) (check.Result, error) {
	filter := fmt.Sprintf("Id eq '%s'", q)
	encoded := strings.ReplaceAll(url.QueryEscape(filter), "+", "%20")
	u := check.JoinURL(c.base, "/api/v2/Packages?$filter="+encoded)
	headers := map[string]string{"Accept": "application/atom+xml"}
	code, header, body, err := c.http.GetRawWithHeaders(ctx, u, headers)
	if err != nil {
		return check.Result{}, err
	}
	switch code {
	case 200:
		var feed struct {
			Entries []struct {
				Title   string `xml:"title"`
				Summary string `xml:"summary"`
				Props   struct {
					Version       string `xml:"Version"`
					DownloadCount int64  `xml:"DownloadCount"`
				} `xml:"properties"`
			} `xml:"entry"`
		}
		if err := xml.Unmarshal(body, &feed); err != nil {
			return check.Result{}, fmt.Errorf("decode Chocolatey feed: %w", err)
		}
		if len(feed.Entries) == 0 {
			return available(q, c.Homepage(q), check.Field{Key: "registry", Value: "Chocolatey"}), nil
		}
		e := feed.Entries[0]
		summary := "published"
		if e.Props.Version != "" {
			summary = "v" + e.Props.Version
		}
		return taken(q, c.Homepage(q), summary,
			check.Field{Key: "package", Value: e.Title},
			check.Field{Key: "latest", Value: e.Props.Version},
			check.Field{Key: "downloads", Value: humanCount(e.Props.DownloadCount)},
			check.Field{Key: "summary", Value: e.Summary},
		), nil
	case 404:
		return available(q, c.Homepage(q), check.Field{Key: "registry", Value: "Chocolatey"}), nil
	default:
		return check.Result{}, httpStatusError(code, u, header)
	}
}
