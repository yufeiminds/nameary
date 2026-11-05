package domain

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yufeiminds/nameary/internal/check"
)

func TestCheckerInterface(t *testing.T) {
	c := New(check.NewHTTP(), "")
	if c.Target() != check.TargetDomains || c.Label() != "domain" || c.Category() != check.CategoryDomains {
		t.Fatalf("metadata: %v %s %v", c.Target(), c.Label(), c.Category())
	}
	if p := c.Policy(); p.Concurrency <= 0 || p.Timeout <= 0 || p.CacheTTL <= 0 {
		t.Fatalf("policy=%+v", p)
	}
	if c.Homepage("acme.com") == "" {
		t.Fatal("empty homepage")
	}
	if _, err := c.Normalize("ACME.COM."); err != nil {
		t.Fatal(err)
	}
	if !Valid("acme.com") || Valid("not a domain") {
		t.Fatal("Valid mapping")
	}
}

func TestBootstrapDiscovery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"services":[[["test"],["https://rdap.test/rdap/"]]]}`))
	}))
	defer srv.Close()
	c := New(check.NewHTTP(), srv.URL)
	c.resolver = fakeResolver{}
	if got := c.serverFor(context.Background(), "test"); got != "https://rdap.test/rdap/" {
		t.Fatalf("server=%q", got)
	}
	if got := c.serverFor(context.Background(), "unknown"); got != rdapFallback {
		t.Fatalf("fallback=%q", got)
	}
	if len(c.Snapshot()) != 1 {
		t.Fatalf("snapshot=%v", c.Snapshot())
	}
}

func TestBootstrapFailureFallsBack(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	c := New(check.NewHTTP(), srv.URL)
	if got := c.serverFor(context.Background(), "test"); got != rdapFallback {
		t.Fatalf("server=%q", got)
	}
}

func TestRDAPErrorStatuses(t *testing.T) {
	for _, code := range []int{429, 500} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(code)
		}))
		c := New(check.NewHTTP(), srv.URL)
		c.Servers = map[string]string{"test": srv.URL}
		c.resolver = fakeResolver{}
		_, err := c.Check(context.Background(), "example.test")
		srv.Close()
		if err == nil || !check.Retryable(err) {
			t.Fatalf("code %d: %v", code, err)
		}
	}
}

func TestRDAPMalformed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{`))
	}))
	defer srv.Close()
	c := New(check.NewHTTP(), srv.URL)
	c.Servers = map[string]string{"test": srv.URL}
	c.resolver = fakeResolver{}
	if _, err := c.Check(context.Background(), "example.test"); err == nil {
		t.Fatal("expected decode error")
	}
}

func TestWhoisLookup(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want check.Status
	}{
		{"taken", "Domain Name: EXAMPLE.TEST\nRegistrar: Example\nCreation Date: 2020-01-02T00:00:00Z\nName Server: NS1.TEST\n", check.StatusTaken},
		{"free", "Domain not found.\n", check.StatusAvailable},
		{"inconclusive", "some unrelated text\n", check.StatusUnknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ln, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer ln.Close()
			go func() {
				conn, err := ln.Accept()
				if err != nil {
					return
				}
				defer conn.Close()
				buf := make([]byte, 256)
				_, _ = conn.Read(buf)
				_, _ = conn.Write([]byte(tc.body))
			}()

			c := New(check.NewHTTP(), "")
			c.whoisServers["test"] = ln.Addr().String()
			res := c.whois(context.Background(), "example.test")
			if res.Status != tc.want {
				t.Fatalf("status=%v, want %v (%+v)", res.Status, tc.want, res)
			}
		})
	}
}

func TestWhoisConnectionFailure(t *testing.T) {
	c := New(check.NewHTTP(), "")
	c.whoisServers["test"] = "127.0.0.1:1" // nothing listens here
	res := c.whois(context.Background(), "example.test")
	if res.Status != check.StatusUnknown {
		t.Fatalf("status=%v", res.Status)
	}
}

func TestWhoisParseEdgeCases(t *testing.T) {
	if _, ok := parseWhois("x.test", "no relevant data"); ok {
		t.Fatal("expected inconclusive")
	}
	res, ok := parseWhois("x.test", "Domain Name: X.TEST\nRegistrar: R\n")
	if !ok || res.Status != check.StatusTaken || res.Summary == "" {
		t.Fatalf("res=%+v ok=%v", res, ok)
	}
}

func TestVcardAndYear(t *testing.T) {
	if got := vcardFN([]any{"vcard", []any{[]any{"fn", map[string]any{}, "text", "Registrar"}}}); got != "Registrar" {
		t.Fatalf("vcard=%q", got)
	}
	if got := vcardFN(nil); got != "" {
		t.Fatal("empty vcard")
	}
	if got := yearOf("2020-01-02T00:00:00Z"); got != "2020" {
		t.Fatalf("year=%q", got)
	}
	if got := yearOf("2020-01-02"); got != "2020" {
		t.Fatalf("year=%q", got)
	}
	if got := yearOf("2020"); got != "2020" {
		t.Fatalf("year=%q", got)
	}
	if got := tldOf("a.b.test"); got != "test" {
		t.Fatalf("tld=%q", got)
	}
	if got := tldOf("test"); got != "test" {
		t.Fatalf("tld=%q", got)
	}
}

func TestDecodeRDAP(t *testing.T) {
	p, err := DecodeRDAP([]byte(`{"ldhName":"EXAMPLE.TEST","status":["active"]}`))
	if err != nil || p.LDHName != "EXAMPLE.TEST" || len(p.Status) != 1 {
		t.Fatalf("p=%+v err=%v", p, err)
	}
	if _, err := DecodeRDAP([]byte(`{`)); err == nil {
		t.Fatal("expected decode error")
	}
}

func TestRDAPResponseHelpers(t *testing.T) {
	var p rdapResponse
	raw := `{"ldhName":"EXAMPLE.TEST","status":["active"],"events":[{"eventAction":"registration","eventDate":"2020-01-02T00:00:00Z"}],"entities":[{"roles":["registrar"],"vcardArray":["vcard",[["fn",{},"text","Reg"]]]}],"nameservers":[{"ldhName":"NS1.TEST"}]}`
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatal(err)
	}
	if registrarName(p.Entities) != "Reg" {
		t.Fatal("registrar")
	}
	if nameservers([]*net.NS{{Host: "NS1.TEST."}}) != "ns1.test" {
		t.Fatal("nameservers")
	}
}
