package ranges

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestCacheWriteIsAtomicTightPermAndMtimeRefreshing(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(fixtureFeed))
	}))
	defer server.Close()

	cache := filepath.Join(t.TempDir(), "cloud.json")
	if _, _, err := Load(context.Background(), Options{URL: server.URL, CachePath: cache, Now: time.Now}); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	info, err := os.Stat(cache)
	if err != nil {
		t.Fatalf("cache not written: %v", err)
	}

	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("cache perms = %o, want 600 (feed cache is world-unreadable)", perm)
	}

	// A rewrite with IDENTICAL bytes must still refresh the mtime the TTL
	// check reads — pin the "never WriteIfChanged" invariant.
	old := time.Now().Add(-72 * time.Hour)
	if err := os.Chtimes(cache, old, old); err != nil {
		t.Fatal(err)
	}

	if _, _, err := Load(
		context.Background(),
		Options{URL: server.URL, CachePath: cache, TTL: time.Nanosecond, Now: time.Now},
	); err != nil {
		t.Fatalf("refresh Load failed: %v", err)
	}

	info, err = os.Stat(cache)
	if err != nil {
		t.Fatal(err)
	}

	if age := time.Since(info.ModTime()); age > time.Minute {
		t.Errorf(
			"identical-content rewrite did not refresh mtime (age %s) — WriteIfChanged semantics would break the TTL",
			age,
		)
	}
}

func TestConcurrentLoadSharesCacheSafely(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(fixtureFeed))
	}))
	defer server.Close()

	cache := filepath.Join(t.TempDir(), "cloud.json")

	const loadCount = 16

	var wg sync.WaitGroup

	errs := make(chan error, loadCount)

	for range loadCount {
		wg.Go(func() {
			set, _, err := Load(
				context.Background(),
				Options{URL: server.URL, CachePath: cache, TTL: time.Nanosecond, Now: time.Now},
			)
			if err == nil && set.Len() == 0 {
				err = os.ErrInvalid
			}

			errs <- err
		})
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent Load failed: %v", err)
		}
	}

	data, err := os.ReadFile(cache)
	if err != nil {
		t.Fatalf("cache missing after concurrent loads: %v", err)
	}

	if _, err := Parse(data); err != nil {
		t.Fatalf("cache is torn/corrupt after concurrent atomic writes: %v", err)
	}
}
