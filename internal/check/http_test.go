package check

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetRawReturnsErrorBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("missing User-Agent")
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"title":"Object not found"}`))
	}))
	defer srv.Close()

	code, _, body, err := NewHTTP().GetRaw(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if code != 404 || string(body) != `{"title":"Object not found"}` {
		t.Fatalf("code=%d body=%s", code, body)
	}
}
