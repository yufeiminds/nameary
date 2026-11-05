package adapters

import (
	"context"
	"net/url"
	"time"

	"github.com/yufeiminds/nameary/internal/check"
)

// Snap checks the Snap Store.
type Snap struct {
	http *check.HTTP
	base string
}

func NewSnap(h *check.HTTP, base string) *Snap {
	if base == "" {
		base = "https://api.snapcraft.io"
	}
	return &Snap{http: h, base: base}
}

func (s *Snap) Target() check.Target     { return check.TargetSnap }
func (s *Snap) Label() string            { return "snap" }
func (s *Snap) Category() check.Category { return check.CategoryPackages }
func (s *Snap) Policy() check.Policy {
	return check.Policy{Priority: 51, Concurrency: 4, Timeout: 8 * time.Second, CacheTTL: 24 * time.Hour}
}

var snapName = `^[a-z0-9][a-z0-9-]*$`

func (s *Snap) Normalize(raw string) (string, error) {
	return validate(raw, snapName, 40)
}

func (s *Snap) Homepage(q string) string {
	return "https://snapcraft.io/" + url.PathEscape(q)
}

func (s *Snap) Check(ctx context.Context, q string) (check.Result, error) {
	u := check.JoinURL(s.base, "/v2/snaps/info/"+url.PathEscape(q))
	headers := map[string]string{"Snap-Device-Series": "16"}
	var payload struct {
		Name       string `json:"name"`
		SnapID     string `json:"snap-id"`
		ChannelMap []struct {
			Channel struct {
				Name         string `json:"name"`
				Track        string `json:"track"`
				Risk         string `json:"risk"`
				Architecture string `json:"architecture"`
			} `json:"channel"`
			Version  string `json:"version"`
			Revision int    `json:"revision"`
		} `json:"channel-map"`
	}
	code, header, err := s.http.GetWithHeaders(ctx, u, headers, &payload)
	if err != nil {
		return check.Result{}, err
	}
	switch code {
	case 200:
		latest := ""
		for _, ch := range payload.ChannelMap {
			if ch.Channel.Track == "latest" && ch.Channel.Risk == "stable" {
				latest = ch.Version
				break
			}
		}
		if latest == "" && len(payload.ChannelMap) > 0 {
			latest = payload.ChannelMap[0].Version
		}
		summary := "published"
		if latest != "" {
			summary = "v" + latest
		}
		return taken(q, s.Homepage(q), summary,
			check.Field{Key: "snap", Value: payload.Name},
			check.Field{Key: "latest", Value: latest},
			check.Field{Key: "snap-id", Value: payload.SnapID},
		), nil
	case 404:
		return available(q, s.Homepage(q), check.Field{Key: "registry", Value: "Snap Store"}), nil
	default:
		return check.Result{}, httpStatusError(code, u, header)
	}
}
