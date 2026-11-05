package cache

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yufeiminds/nameary/internal/check"
)

func TestDefaultPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, "xdg"))
	got := DefaultPath()
	if !strings.HasSuffix(got, filepath.Join("nameary", "cache.db")) {
		t.Fatalf("path=%q", got)
	}
}

func TestOpenDefaultAndNilStore(t *testing.T) {
	var nilStore *Store
	if _, ok := nilStore.Get(check.TargetNPM, "x"); ok {
		t.Fatal("nil store returned a hit")
	}
	nilStore.Put(check.TargetNPM, "x", check.Result{Status: check.StatusTaken}, time.Hour)
	if err := nilStore.Close(); err != nil {
		t.Fatal(err)
	}

	store, err := Open("")
	if err != nil {
		t.Fatal(err)
	}
	if store.db == nil {
		t.Fatal("no db")
	}
	if _, ok := store.Get(check.TargetNPM, "missing"); ok {
		t.Fatal("unexpected hit")
	}
	store.Put(check.TargetNPM, "acme", check.Result{Status: check.StatusTaken, Confidence: check.Certain}, time.Hour)
	res, ok := store.Get(check.TargetNPM, "acme")
	if !ok || !res.Cached || res.Status != check.StatusTaken {
		t.Fatalf("res=%+v ok=%v", res, ok)
	}
	// Unknowns, errors, and non-positive TTLs are never stored.
	store.Put(check.TargetNPM, "unknown", check.Result{Status: check.StatusUnknown}, time.Hour)
	store.Put(check.TargetNPM, "zero", check.Result{Status: check.StatusAvailable}, 0)
	if _, ok := store.Get(check.TargetNPM, "unknown"); ok {
		t.Fatal("unknown should not be cached")
	}
	if _, ok := store.Get(check.TargetNPM, "zero"); ok {
		t.Fatal("zero ttl should not be cached")
	}
	store.Put(check.TargetNPM, "expired", check.Result{Status: check.StatusAvailable}, -time.Hour)
	if _, ok := store.Get(check.TargetNPM, "expired"); ok {
		t.Fatal("expired should not be returned")
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestOpenFailure(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(filepath.Join(blocker, "cache.db")); err == nil {
		t.Fatal("expected mkdir failure")
	}
}

func TestOpenLockFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.db")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := Open(path); err == nil {
		t.Fatal("expected lock timeout")
	}
}

func TestExpiredEntry(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "cache.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	store.Put(check.TargetNPM, "acme", check.Result{Status: check.StatusTaken}, time.Hour)
	// Rewrite the same key with a tiny TTL and a past timestamp.
	r := check.Result{Status: check.StatusTaken, FetchedAt: time.Now().Add(-2 * time.Hour)}
	store.Put(check.TargetNPM, "acme", r, time.Hour)
	if _, ok := store.Get(check.TargetNPM, "acme"); ok {
		t.Fatal("expected expiration")
	}
}

func TestKeyShape(t *testing.T) {
	if got := string(key(check.TargetNPM, "acme")); got != "v1:npm:acme" {
		t.Fatalf("key=%q", got)
	}
}
