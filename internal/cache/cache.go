// Package cache stores definitive check results in a small bbolt database.
package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/yufeiminds/nameary/internal/check"
	bolt "go.etcd.io/bbolt"
)

var bucket = []byte("results")

// Store is a durable cache for "available" and "taken" results.
type Store struct {
	db *bolt.DB
}

type entry struct {
	Result    check.Result `json:"result"`
	FetchedAt time.Time    `json:"fetched_at"`
	TTL       int64        `json:"ttl_ns"`
}

// DefaultPath returns the platform-appropriate cache file.
func DefaultPath() string {
	dir, err := os.UserCacheDir()
	if err != nil || dir == "" {
		return filepath.Join(os.TempDir(), "nameary", "cache.db")
	}
	return filepath.Join(dir, "nameary", "cache.db")
}

// Open opens (and creates) the cache database.
func Open(path string) (*Store, error) {
	if path == "" {
		path = DefaultPath()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: time.Second})
	if err != nil {
		return nil, err
	}
	_ = os.Chmod(path, 0o600)
	if err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(bucket)
		return err
	}); err != nil {
		if cerr := db.Close(); cerr != nil {
			return nil, cerr
		}
		return nil, err
	}
	return &Store{db: db}, nil
}

// Get returns a fresh cached result.
func (s *Store) Get(target check.Target, query string) (check.Result, bool) {
	if s == nil || s.db == nil {
		return check.Result{}, false
	}
	var out check.Result
	err := s.db.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bucket).Get(key(target, query))
		if v == nil {
			return fmt.Errorf("not found")
		}
		var e entry
		if err := json.Unmarshal(v, &e); err != nil {
			return err
		}
		if time.Since(e.FetchedAt) > time.Duration(e.TTL) {
			return fmt.Errorf("expired")
		}
		out = e.Result
		out.Cached = true
		out.FetchedAt = e.FetchedAt
		return nil
	})
	if err != nil {
		return check.Result{}, false
	}
	return out, true
}

// Put stores a definitive result. Errors and unknowns are never cached.
func (s *Store) Put(target check.Target, query string, r check.Result, ttl time.Duration) {
	if s == nil || s.db == nil || ttl <= 0 {
		return
	}
	if r.Status != check.StatusAvailable && r.Status != check.StatusTaken {
		return
	}
	r.Err = nil
	r.Cached = false
	if r.FetchedAt.IsZero() {
		r.FetchedAt = time.Now()
	}
	e := entry{Result: r, FetchedAt: r.FetchedAt, TTL: int64(ttl)}
	data, err := json.Marshal(e)
	if err != nil {
		return
	}
	_ = s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucket).Put(key(target, query), data)
	})
}

// Close closes the database.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func key(target check.Target, query string) []byte {
	return []byte(fmt.Sprintf("v1:%s:%s", target, query))
}
