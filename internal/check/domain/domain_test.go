package domain

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yufeiminds/nameary/internal/check"
)

type fakeResolver struct {
	ns  []*net.NS
	err error
}

func (f fakeResolver) LookupNS(context.Context, string) ([]*net.NS, error) {
	return f.ns, f.err
}

func newTestChecker(t *testing.T, handler http.HandlerFunc) *Checker {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c := New(check.NewHTTP(), srv.URL)
	c.Servers = map[string]string{"test": srv.URL}
	c.resolver = fakeResolver{}
	c.DisableWhois = true
	return c
}

func TestNormalize(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"Example.COM", "example.com", false},
		{"example.com.", "example.com", false},
		{"bücher.example", "xn--bcher-kva.example", false},
		{"no-dot", "", true},
		{"-bad.com", "", true},
		{"bad..com", "", true},
		{"example.123", "", true},
	}
	for _, tt := range tests {
		got, err := Normalize(tt.in)
		if (err != nil) != tt.wantErr {
			t.Fatalf("%q: err=%v wantErr=%v", tt.in, err, tt.wantErr)
		}
		if !tt.wantErr && got != tt.want {
			t.Fatalf("%q: got %q want %q", tt.in, got, tt.want)
		}
	}
}

func TestDNSDelegationIsTaken(t *testing.T) {
	called := false
	c := newTestChecker(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		http.NotFound(w, r)
	})
	c.resolver = fakeResolver{ns: []*net.NS{{Host: "ns1.example.com."}}}
	res, err := c.Check(context.Background(), "example.test")
	if err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("RDAP should not be called when DNS is delegated")
	}
	if res.Status != check.StatusTaken || res.Confidence != check.Certain {
		t.Fatalf("res=%+v", res)
	}
}

func TestRDAPTaken(t *testing.T) {
	c := newTestChecker(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/domain/example.test" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{
			"ldhName":"EXAMPLE.TEST",
			"status":["active"],
			"events":[
				{"eventAction":"registration","eventDate":"1995-08-14T04:00:00Z"},
				{"eventAction":"expiration","eventDate":"2027-08-13T04:00:00Z"}
			],
			"entities":[{"roles":["registrar"],"vcardArray":["vcard",[["version",{},"text","4.0"],["fn",{},"text","Example Registrar"]]]}],
			"nameservers":[{"ldhName":"NS1.EXAMPLE.TEST"}]
		}`))
	})
	res, err := c.Check(context.Background(), "example.test")
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != check.StatusTaken {
		t.Fatalf("status=%v", res.Status)
	}
	if res.Field("registrar") != "Example Registrar" {
		t.Fatalf("registrar=%q", res.Field("registrar"))
	}
	if res.Field("created") != "1995-08-14T04:00:00Z" {
		t.Fatalf("created=%q", res.Field("created"))
	}
	if res.Field("nameservers") != "ns1.example.test" {
		t.Fatalf("nameservers=%q", res.Field("nameservers"))
	}
}

func TestRDAPNotFoundIsLikelyAvailable(t *testing.T) {
	c := newTestChecker(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"errorCode":404,"title":"Object not found"}`))
	})
	res, err := c.Check(context.Background(), "example.test")
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != check.StatusAvailable || res.Confidence != check.Likely {
		t.Fatalf("res=%+v", res)
	}
}

func TestRDAPNoServiceIsUnknown(t *testing.T) {
	c := newTestChecker(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"errorCode":404,"title":"No RDAP service is available for this resource"}`))
	})
	res, err := c.Check(context.Background(), "example.test")
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != check.StatusUnknown {
		t.Fatalf("status=%v, want unknown", res.Status)
	}
}

func TestWhoisParse(t *testing.T) {
	text := `Domain Name: EXAMPLE.CO
Registry Domain ID: D123-CNIC
Registrar WHOIS Server: whois.example.com
Registrar: Example Registrar
Creation Date: 2010-02-25T01:04:59.0Z
Registry Expiry Date: 2027-02-24T23:59:59.0Z
Domain Status: clientTransferProhibited https://icann.org/epp#clientTransferProhibited
Name Server: NS1.EXAMPLE.COM
Name Server: NS2.EXAMPLE.COM
`
	res, ok := parseWhois("example.co", text)
	if !ok {
		t.Fatal("expected parse")
	}
	if res.Status != check.StatusTaken || res.Field("registrar") != "Example Registrar" {
		t.Fatalf("res=%+v", res)
	}
	if res.Field("nameservers") != "NS1.EXAMPLE.COM, NS2.EXAMPLE.COM" {
		t.Fatalf("ns=%q", res.Field("nameservers"))
	}

	res, ok = parseWhois("nope.co", "Domain not found.\n")
	if !ok || res.Status != check.StatusAvailable {
		t.Fatalf("res=%+v ok=%v", res, ok)
	}
}
