package cache

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/yufeiminds/nameary/internal/check"
)

func TestCacheRoundTrip(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "cache.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	r := check.Result{Status: check.StatusAvailable, Confidence: check.Certain, Query: "acme", SourceURL: "https://example.test"}
	store.Put(check.TargetNPM, "acme", r, time.Hour)
	got, ok := store.Get(check.TargetNPM, "acme")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if !got.Cached || got.Status != check.StatusAvailable {
		t.Fatalf("got=%+v", got)
	}
	if _, ok := store.Get(check.TargetNPM, "other"); ok {
		t.Fatal("unexpected hit")
	}
}

func TestCacheExpiry(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "cache.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	store.Put(check.TargetNPM, "acme", check.Result{Status: check.StatusAvailable}, time.Nanosecond)
	time.Sleep(2 * time.Millisecond)
	if _, ok := store.Get(check.TargetNPM, "acme"); ok {
		t.Fatal("expected expired entry")
	}
}

func TestCacheSkipsErrors(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "cache.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	store.Put(check.TargetNPM, "acme", check.Result{Status: check.StatusUnknown}, time.Hour)
	if _, ok := store.Get(check.TargetNPM, "acme"); ok {
		t.Fatal("unknown results must not be cached")
	}
}
