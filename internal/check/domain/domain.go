// Package domain checks domain registration using DNS delegation as a fast
// negative signal and RDAP as the authoritative record.
package domain

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/idna"

	"github.com/yufeiminds/nameary/internal/check"
)

const (
	ianaBootstrap = "https://data.iana.org/rdap/dns.json"
	rdapFallback  = "https://rdap.org"
)

// extraRDAP fills the few popular TLD gaps in the IANA bootstrap. Keep this
// list small and evidence-based: a wrong base URL is worse than "unknown".
var extraRDAP = map[string]string{
	"io": "https://rdap.identitydigital.services/rdap/",
}

// Resolver is the small slice of net.Resolver we need, so tests can fake it.
type Resolver interface {
	LookupNS(ctx context.Context, host string) ([]*net.NS, error)
}

// Checker checks one fully qualified domain per call.
type Checker struct {
	http      *check.HTTP
	resolver  Resolver
	bootstrap string
	// Servers lets tests inject a TLD -> RDAP base map.
	Servers map[string]string
	// DisableWhois is used by tests to keep the no-RDAP path offline.
	DisableWhois bool

	mu      sync.Mutex
	loaded  bool
	servers map[string]string

	whoisMu      sync.Mutex
	whoisServers map[string]string
}

// New returns a domain checker.
func New(h *check.HTTP, bootstrapURL string) *Checker {
	if h == nil {
		h = check.NewHTTP()
	}
	if bootstrapURL == "" {
		bootstrapURL = ianaBootstrap
	}
	return &Checker{http: h, resolver: &net.Resolver{PreferGo: true}, bootstrap: bootstrapURL, whoisServers: map[string]string{}}
}

func (c *Checker) Target() check.Target     { return check.TargetDomains }
func (c *Checker) Label() string            { return "domain" }
func (c *Checker) Category() check.Category { return check.CategoryDomains }
func (c *Checker) Policy() check.Policy {
	return check.Policy{Priority: 10, Concurrency: 4, Timeout: 8 * time.Second, CacheTTL: 6 * time.Hour}
}

func (c *Checker) Homepage(q string) string {
	return rdapFallback + "/domain/" + q
}

var labelRE = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

// Normalize validates and canonicalizes a domain. It accepts IDN and converts
// it to its A-label form.
func (c *Checker) Normalize(raw string) (string, error) {
	return Normalize(raw)
}

// Normalize is exported because the planner needs it before creating jobs.
func Normalize(raw string) (string, error) {
	q := strings.ToLower(strings.TrimSpace(raw))
	q = strings.TrimSuffix(q, ".")
	if q == "" {
		return "", fmt.Errorf("domain is empty")
	}
	ascii, err := idna.Lookup.ToASCII(q)
	if err != nil {
		return "", fmt.Errorf("invalid internationalized domain: %w", err)
	}
	if len(ascii) > 253 {
		return "", fmt.Errorf("domain is longer than 253 characters")
	}
	labels := strings.Split(ascii, ".")
	if len(labels) < 2 {
		return "", fmt.Errorf("domain needs at least one dot")
	}
	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 {
			return "", fmt.Errorf("invalid label length in %q", ascii)
		}
		if !labelRE.MatchString(label) {
			return "", fmt.Errorf("invalid domain label %q", label)
		}
	}
	tld := labels[len(labels)-1]
	if _, err := strconv.Atoi(tld); err == nil {
		return "", fmt.Errorf("top-level domain cannot be numeric")
	}
	return ascii, nil
}

// Valid reports whether raw looks like a fully qualified domain.
func Valid(raw string) bool {
	_, err := Normalize(raw)
	return err == nil
}

func (c *Checker) Check(ctx context.Context, q string) (check.Result, error) {
	domain, err := Normalize(q)
	if err != nil {
		return check.Result{}, err
	}

	// Fast path: if the domain is delegated, it is definitely registered. A
	// missing NS record does NOT mean it is free (it may be registered but
	// parked), so we still ask RDAP.
	ns, nsErr := c.resolver.LookupNS(ctx, domain)
	if nsErr == nil && len(ns) > 0 {
		return check.Result{
			Status:     check.StatusTaken,
			Confidence: check.Certain,
			Query:      domain,
			Summary:    "delegated · " + strconv.Itoa(len(ns)) + " NS",
			SourceURL:  "https://www.whois.com/whois/" + domain,
			FetchedAt:  time.Now(),
			Fields: []check.Field{
				{Key: "dns", Value: nameservers(ns)},
				{Key: "note", Value: "DNS delegation is a fast signal; RDAP has the registration details."},
			},
		}, nil
	}

	base := c.serverFor(ctx, tldOf(domain))
	u := rdapURL(base, domain)
	code, header, body, err := c.http.GetRaw(ctx, u)
	if err != nil {
		return check.Result{}, err
	}
	switch code {
	case 200:
		var payload rdapResponse
		if err := json.Unmarshal(body, &payload); err != nil {
			return check.Result{}, fmt.Errorf("decode RDAP for %s: %w", domain, err)
		}
		return c.fromRDAP(domain, payload, u), nil
	case 404:
		if noRDAPService(body) {
			if c.DisableWhois {
				return unknownDomain(domain, "RDAP is not available for ."+tldOf(domain), "WHOIS fallback disabled"), nil
			}
			return c.whois(ctx, domain), nil
		}
		return check.Result{
			Status:     check.StatusAvailable,
			Confidence: check.Likely,
			Query:      domain,
			Summary:    "no registration record",
			SourceURL:  u,
			FetchedAt:  time.Now(),
			Fields: []check.Field{
				{Key: "dns", Value: "no delegation"},
				{Key: "note", Value: "RDAP has no record. Reserved or premium names can still be unavailable; confirm with a registrar before relying on this."},
			},
		}, nil
	case 429:
		return check.Result{}, check.NewHTTPError(code, u, header)
	default:
		return check.Result{}, check.NewHTTPError(code, u, header)
	}
}

func noRDAPService(body []byte) bool {
	lower := strings.ToLower(string(body))
	return strings.Contains(lower, "no rdap service") || strings.Contains(lower, "no service is available")
}

func (c *Checker) fromRDAP(domain string, p rdapResponse, source string) check.Result {
	fields := []check.Field{{Key: "domain", Value: p.LDHName}}
	if domain != "" && p.LDHName != "" {
		fields[0].Value = p.LDHName
	}
	if registrar := registrarName(p.Entities); registrar != "" {
		fields = append(fields, check.Field{Key: "registrar", Value: registrar})
	}
	events := map[string]string{}
	for _, e := range p.Events {
		events[strings.ToLower(e.Action)] = e.Date
	}
	add := func(key, event string) {
		if v := events[event]; v != "" {
			fields = append(fields, check.Field{Key: key, Value: v})
		}
	}
	add("created", "registration")
	add("expires", "expiration")
	add("updated", "last changed")
	if len(p.Status) > 0 {
		fields = append(fields, check.Field{Key: "status", Value: strings.Join(p.Status, ", ")})
	}
	if len(p.Nameservers) > 0 {
		var names []string
		for _, n := range p.Nameservers {
			if n.LDHName != "" {
				names = append(names, strings.ToLower(n.LDHName))
			}
		}
		if len(names) > 0 {
			fields = append(fields, check.Field{Key: "nameservers", Value: strings.Join(names, ", ")})
		}
	}

	summary := "registered"
	if r := registrarName(p.Entities); r != "" {
		summary = r
	}
	if created := events["registration"]; created != "" {
		if year := yearOf(created); year != "" {
			if summary == "registered" {
				summary = "registered " + year
			} else {
				summary += " · " + year
			}
		}
	}
	return check.Result{
		Status:     check.StatusTaken,
		Confidence: check.Certain,
		Query:      domain,
		Summary:    summary,
		Fields:     fields,
		SourceURL:  source,
		FetchedAt:  time.Now(),
	}
}

func (c *Checker) serverFor(ctx context.Context, tld string) string {
	if base, ok := extraRDAP[tld]; ok {
		return base
	}
	if c.Servers != nil {
		if base, ok := c.Servers[tld]; ok {
			return base
		}
		return rdapFallback
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.loaded {
		c.loadServers(ctx)
	}
	if base, ok := c.servers[tld]; ok {
		return base
	}
	return rdapFallback
}

func (c *Checker) loadServers(ctx context.Context) {
	c.loaded = true
	c.servers = map[string]string{}
	fctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var doc struct {
		Services [][][]string `json:"services"`
	}
	code, _, err := c.http.Get(fctx, c.bootstrap, &doc)
	if err != nil || code != 200 {
		return
	}
	for _, svc := range doc.Services {
		if len(svc) < 2 {
			continue
		}
		base := ""
		for _, u := range svc[1] {
			if strings.HasPrefix(u, "https://") {
				base = u
				break
			}
			if base == "" {
				base = u
			}
		}
		if base == "" {
			continue
		}
		for _, tld := range svc[0] {
			c.servers[strings.ToLower(tld)] = base
		}
	}
}

type rdapEvent struct {
	Action string `json:"eventAction"`
	Date   string `json:"eventDate"`
}

type rdapEntity struct {
	Roles      []string `json:"roles"`
	VCardArray []any    `json:"vcardArray"`
}

type rdapNameserver struct {
	LDHName string `json:"ldhName"`
}

type rdapResponse struct {
	LDHName     string           `json:"ldhName"`
	Status      []string         `json:"status"`
	Events      []rdapEvent      `json:"events"`
	Entities    []rdapEntity     `json:"entities"`
	Nameservers []rdapNameserver `json:"nameservers"`
}

func registrarName(entities []rdapEntity) string {
	for _, e := range entities {
		isRegistrar := false
		for _, r := range e.Roles {
			if strings.EqualFold(r, "registrar") {
				isRegistrar = true
				break
			}
		}
		if !isRegistrar {
			continue
		}
		if fn := vcardFN(e.VCardArray); fn != "" {
			return fn
		}
	}
	return ""
}

func vcardFN(v []any) string {
	if len(v) < 2 {
		return ""
	}
	items, ok := v[1].([]any)
	if !ok {
		return ""
	}
	for _, item := range items {
		field, ok := item.([]any)
		if !ok || len(field) < 4 {
			continue
		}
		name, _ := field[0].(string)
		if !strings.EqualFold(name, "fn") {
			continue
		}
		if s, ok := field[3].(string); ok {
			return s
		}
	}
	return ""
}

func nameservers(ns []*net.NS) string {
	var out []string
	for _, n := range ns {
		if n.Host != "" {
			out = append(out, strings.ToLower(strings.TrimSuffix(n.Host, ".")))
		}
	}
	return strings.Join(out, ", ")
}

func tldOf(domain string) string {
	i := strings.LastIndex(domain, ".")
	if i < 0 {
		return domain
	}
	return strings.ToLower(domain[i+1:])
}

func rdapURL(base, domain string) string {
	base = strings.TrimRight(base, "/")
	return base + "/domain/" + domain
}

func yearOf(ts string) string {
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05Z", "2006-01-02"} {
		if t, err := time.Parse(layout, ts); err == nil {
			return strconv.Itoa(t.Year())
		}
	}
	if len(ts) >= 4 {
		return ts[:4]
	}
	return ""
}

// Snapshot is a tiny helper used by tests to freeze the bootstrap map.
func (c *Checker) Snapshot() map[string]string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := map[string]string{}
	for k, v := range c.servers {
		out[k] = v
	}
	return out
}

// DecodeRDAP is exported for tests that want to assert parsing.
func DecodeRDAP(data []byte) (rdapResponse, error) {
	var p rdapResponse
	err := json.Unmarshal(data, &p)
	return p, err
}
