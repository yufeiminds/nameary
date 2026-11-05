package adapters

import (
	"context"
	"fmt"
	"net/url"
	"time"
	"unicode"

	"github.com/yufeiminds/nameary/internal/check"
)

// CocoaPods checks the public pod page. Pod names are case-sensitive, so we
// also try a title-cased variant when the exact spelling is free.
type CocoaPods struct {
	http *check.HTTP
	base string
}

func NewCocoaPods(h *check.HTTP, base string) *CocoaPods {
	if base == "" {
		base = "https://cocoapods.org"
	}
	return &CocoaPods{http: h, base: base}
}

func (c *CocoaPods) Target() check.Target     { return check.TargetCocoaPods }
func (c *CocoaPods) Label() string            { return "pods" }
func (c *CocoaPods) Category() check.Category { return check.CategoryPackages }
func (c *CocoaPods) Policy() check.Policy {
	return check.Policy{Priority: 47, Concurrency: 2, Timeout: 6 * time.Second, CacheTTL: 24 * time.Hour}
}

var cocoapodsName = `^[A-Za-z0-9][A-Za-z0-9._+-]*$`

func (c *CocoaPods) Normalize(raw string) (string, error) {
	q, err := validatePattern(raw, cocoapodsName)
	if err != nil {
		return "", err
	}
	if len(q) > 100 {
		return "", fmt.Errorf("pod name is longer than 100 characters")
	}
	return q, nil
}

func (c *CocoaPods) Homepage(q string) string {
	return "https://cocoapods.org/pods/" + url.PathEscape(q)
}

func (c *CocoaPods) Check(ctx context.Context, q string) (check.Result, error) {
	res, found, err := c.lookup(ctx, q)
	if err != nil {
		return check.Result{}, err
	}
	if found {
		return res, nil
	}

	variant := titleCase(q)
	if variant != q {
		vres, vfound, verr := c.lookup(ctx, variant)
		if verr != nil {
			return check.Result{}, verr
		}
		if vfound {
			vres.Fields = append(vres.Fields, check.Field{Key: "note", Value: "matched the case variant " + variant})
			return vres, nil
		}
	}
	return available(q, c.Homepage(q), check.Field{Key: "registry", Value: "CocoaPods"}), nil
}

func (c *CocoaPods) lookup(ctx context.Context, q string) (check.Result, bool, error) {
	u := check.JoinURL(c.base, "/pods/"+url.PathEscape(q))
	code, header, err := c.http.Head(ctx, u)
	if err != nil {
		return check.Result{}, false, err
	}
	switch code {
	case 200:
		return taken(q, c.Homepage(q), "published",
			check.Field{Key: "pod", Value: q},
			check.Field{Key: "note", Value: "CocoaPods pod page exists"},
		), true, nil
	case 404:
		return check.Result{}, false, nil
	default:
		return check.Result{}, false, httpStatusError(code, u, header)
	}
}

func titleCase(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}
