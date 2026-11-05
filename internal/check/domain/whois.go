package domain

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/yufeiminds/nameary/internal/check"
)

// whoisServers covers the TLDs we care about that do not publish RDAP. TLDs
// with RDAP never reach this path.
var whoisServers = map[string]string{
	"co":     "whois.registry.co",
	"io":     "whois.nic.io",
	"me":     "whois.nic.me",
	"ai":     "whois.nic.ai",
	"sh":     "whois.nic.sh",
	"so":     "whois.nic.so",
	"tv":     "whois.nic.tv",
	"cc":     "ccwhois.verisign-grs.com",
	"xyz":    "whois.nic.xyz",
	"app":    "whois.nic.google",
	"dev":    "whois.nic.google",
	"design": "whois.nic.design",
	"cloud":  "whois.nic.cloud",
}

func (c *Checker) whois(ctx context.Context, domain string) check.Result {
	tld := tldOf(domain)
	server, err := c.whoisServer(ctx, tld)
	if err != nil || server == "" {
		return unknownDomain(domain, "RDAP is not available for ."+tld, "WHOIS server could not be resolved")
	}
	text, err := queryWhois(ctx, server, domain)
	if err != nil {
		return unknownDomain(domain, "RDAP is not available for ."+tld, "WHOIS lookup failed: "+check.ShortError(err))
	}
	if res, ok := parseWhois(domain, text); ok {
		return res
	}
	return unknownDomain(domain, "RDAP is not available for ."+tld, "WHOIS response was inconclusive")
}

func (c *Checker) whoisServer(ctx context.Context, tld string) (string, error) {
	if s, ok := whoisServers[tld]; ok {
		return s, nil
	}
	c.whoisMu.Lock()
	defer c.whoisMu.Unlock()
	if c.whoisServers == nil {
		c.whoisServers = map[string]string{}
	}
	if v, ok := c.whoisServers[tld]; ok {
		return v, nil
	}
	text, err := queryWhois(ctx, "whois.iana.org", tld)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(text, "\n") {
		key, value, ok := cutKey(line)
		if !ok {
			continue
		}
		if key == "whois" || key == "refer" {
			server := strings.TrimSpace(strings.TrimPrefix(value, "whois://"))
			if server != "" {
				c.whoisServers[tld] = server
				return server, nil
			}
		}
	}
	return "", fmt.Errorf("no WHOIS server for .%s", tld)
}

func queryWhois(ctx context.Context, server, query string) (string, error) {
	d := net.Dialer{Timeout: 5 * time.Second}
	addr := server
	if _, _, splitErr := net.SplitHostPort(server); splitErr != nil {
		addr = net.JoinHostPort(server, "43")
	}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	deadline := time.Now().Add(8 * time.Second)
	if dl, ok := ctx.Deadline(); ok && dl.Before(deadline) {
		deadline = dl
	}
	_ = conn.SetDeadline(deadline)
	if _, err := fmt.Fprintf(conn, "%s\r\n", query); err != nil {
		return "", err
	}
	body, err := io.ReadAll(io.LimitReader(conn, 1<<20))
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func parseWhois(domain, text string) (check.Result, bool) {
	lower := strings.ToLower(text)
	for _, marker := range []string{
		"domain not found",
		"no match for",
		"no data found",
		"no entries found",
		"no object found",
		"not registered",
		"available for registration",
		"status: free",
		"the queried object does not exist",
	} {
		if strings.Contains(lower, marker) {
			return check.Result{
				Status:     check.StatusAvailable,
				Confidence: check.Likely,
				Query:      domain,
				Summary:    "no WHOIS record",
				SourceURL:  "https://www.whois.com/whois/" + domain,
				FetchedAt:  time.Now(),
				Fields: []check.Field{
					{Key: "source", Value: "WHOIS fallback (this TLD has no public RDAP)"},
					{Key: "note", Value: "Confirm with a registrar; reserved and premium names may still be unavailable."},
				},
			}, true
		}
	}

	values := map[string][]string{}
	for _, line := range strings.Split(text, "\n") {
		key, value, ok := cutKey(line)
		if !ok {
			continue
		}
		if strings.HasPrefix(key, "%") || strings.HasPrefix(key, "#") {
			continue
		}
		values[key] = append(values[key], strings.TrimSpace(value))
	}

	get := func(keys ...string) string {
		for _, k := range keys {
			if v := values[k]; len(v) > 0 && v[0] != "" {
				return v[0]
			}
		}
		return ""
	}
	domainName := get("domain name", "domain")
	registrar := get("registrar")
	if domainName == "" && registrar == "" {
		return check.Result{}, false
	}

	var fields []check.Field
	if domainName != "" {
		fields = append(fields, check.Field{Key: "domain", Value: domainName})
	}
	if registrar != "" {
		fields = append(fields, check.Field{Key: "registrar", Value: registrar})
	}
	for _, pair := range [][2]string{
		{"created", "creation date"},
		{"created", "created"},
		{"created", "created on"},
		{"expires", "registry expiry date"},
		{"expires", "expiry date"},
		{"expires", "expiration date"},
		{"updated", "updated date"},
		{"updated", "last updated"},
		{"updated", "last-modified"},
	} {
		if v := get(pair[1]); v != "" {
			fields = append(fields, check.Field{Key: pair[0], Value: v})
			break
		}
	}
	if statuses := values["domain status"]; len(statuses) > 0 {
		fields = append(fields, check.Field{Key: "status", Value: strings.Join(statuses, ", ")})
	}
	if ns := values["name server"]; len(ns) > 0 {
		fields = append(fields, check.Field{Key: "nameservers", Value: strings.Join(ns, ", ")})
	}
	summary := registrar
	if summary == "" {
		summary = "registered"
	}
	if created := get("creation date", "created", "created on"); created != "" {
		if year := yearOf(created); year != "" {
			if summary == registrar && registrar != "" {
				summary += " · " + year
			} else {
				summary = "registered " + year
			}
		}
	}
	return check.Result{
		Status:     check.StatusTaken,
		Confidence: check.Certain,
		Query:      domain,
		Summary:    summary,
		Fields:     fields,
		SourceURL:  "https://www.whois.com/whois/" + domain,
		FetchedAt:  time.Now(),
	}, true
}

func unknownDomain(domain, reason, detail string) check.Result {
	return check.Result{
		Status:     check.StatusUnknown,
		Confidence: check.Low,
		Query:      domain,
		Summary:    reason,
		SourceURL:  "https://www.whois.com/whois/" + domain,
		FetchedAt:  time.Now(),
		Fields: []check.Field{
			{Key: "reason", Value: reason},
			{Key: "detail", Value: detail},
			{Key: "note", Value: "No public RDAP service was found and the WHOIS fallback could not decide. Try a registrar search."},
		},
	}
}

func cutKey(line string) (string, string, bool) {
	line = strings.TrimSpace(line)
	i := strings.Index(line, ":")
	if i <= 0 {
		return "", "", false
	}
	return strings.ToLower(strings.TrimSpace(line[:i])), line[i+1:], true
}
