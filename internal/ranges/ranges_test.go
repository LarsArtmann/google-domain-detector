package ranges

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const fixtureFeed = `{
  "prefixes": [
    {"ipv4Prefix": "34.0.0.0/8", "service": "Google Cloud"},
    {"ipv6Prefix": "2600:1900::/28", "service": "Google Cloud"}
  ],
  "creationTimestamp": "2026-11-09T00:00:00Z"
}`

func TestFetchSendsUserAgent(t *testing.T) {
	t.Parallel()

	var got string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.UserAgent()
		_, _ = w.Write([]byte(fixtureFeed))
	}))
	defer server.Close()

	cache := filepath.Join(t.TempDir(), "cloud.json")
	if _, _, err := Load(
		context.Background(),
		Options{URL: server.URL, CachePath: cache, UserAgent: "google-domain-detector/test", Now: time.Now},
	); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if got != "google-domain-detector/test" {
		t.Errorf("User-Agent = %q, want the explicit agent", got)
	}

	cache2 := filepath.Join(t.TempDir(), "cloud.json")
	if _, _, err := Load(
		context.Background(),
		Options{URL: server.URL, CachePath: cache2, Now: time.Now},
	); err != nil {
		t.Fatalf("Load with default agent failed: %v", err)
	}

	if got != DefaultUserAgent {
		t.Errorf("User-Agent = %q, want default %q", got, DefaultUserAgent)
	}
}

func TestLoadFetchFailureExits75(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	_, _, err := Load(context.Background(), Options{
		URL:       server.URL,
		CachePath: filepath.Join(t.TempDir(), "absent", "cloud.json"),
		Now:       time.Now,
	})

	feedErr, ok := errors.AsType[*Error](err)
	if !ok || feedErr.Kind != KindFetch || feedErr.ExitCode() != ExitFetch {
		t.Fatalf("fetch failure must be Kind fetch with exit 75, got %v", err)
	}
}

func TestLoadCorruptFeedExits65(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("not json at all"))
	}))
	defer server.Close()

	_, _, err := Load(context.Background(), Options{
		URL:       server.URL,
		CachePath: filepath.Join(t.TempDir(), "absent", "cloud.json"),
		Now:       time.Now,
	})

	feedErr, ok := errors.AsType[*Error](err)
	if !ok || feedErr.Kind != KindCorrupt || feedErr.ExitCode() != ExitCorrupt {
		t.Fatalf("corrupt feed must be Kind corrupt with exit 65, got %v", err)
	}
}

func TestParseAndContains(t *testing.T) {
	t.Parallel()

	set, err := Parse([]byte(fixtureFeed))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if set.Len() != 2 {
		t.Errorf("Len() = %d, want 2", set.Len())
	}

	matching := []string{"34.1.2.3", "34.255.255.255", "2600:1900:ffff::1"}
	for _, ip := range matching {
		if !set.Contains(net.ParseIP(ip)) {
			t.Errorf("Contains(%s) = false, want true", ip)
		}
	}

	nonMatching := []string{
		"35.0.0.1",             // outside 34/8
		"104.16.0.1",           // cloudflare
		"142.250.0.1",          // google corp space, not cloud.json
		"2600:1fff::1",         // outside the v6 block
		"2001:4860:4860::8888", // google dns, not cloud.json
	}
	for _, ip := range nonMatching {
		if set.Contains(net.ParseIP(ip)) {
			t.Errorf("Contains(%s) = true, want false", ip)
		}
	}

	var nilSet *Set
	if nilSet.Contains(net.ParseIP("34.0.0.1")) {
		t.Error("nil set must contain nothing")
	}

	if nilSet.Len() != 0 {
		t.Error("nil set must have Len 0")
	}
}

func TestParseRejects(t *testing.T) {
	t.Parallel()

	for name, data := range map[string]string{
		"not json":       "csv,data,header",
		"empty prefixes": `{"prefixes":[]}`,
		"broken cidr":    `{"prefixes":[{"ipv4Prefix":"34.0.0.0/64"}]}`,
		"not a cidr":     `{"prefixes":[{"ipv4Prefix":"example.com"}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if _, err := Parse([]byte(data)); err == nil {
				t.Errorf("Parse(%q) unexpectedly succeeded", data)
			}
		})
	}
}

func TestLoadNetworkFetchesAndCaches(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(fixtureFeed))
	}))
	defer server.Close()

	cache := filepath.Join(t.TempDir(), "sub", "cloud.json")
	opts := Options{URL: server.URL, CachePath: cache, Now: time.Now}

	set, source, err := Load(context.Background(), opts)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if source != SourceNetwork {
		t.Errorf("source = %s, want network", source)
	}

	if !set.Contains(net.ParseIP("34.1.1.1")) {
		t.Error("fetched set must match 34/8")
	}

	if _, err := os.Stat(cache); err != nil {
		t.Errorf("cache file not written: %v", err)
	}

	fetched := 0

	counting := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fetched++
		_, _ = w.Write([]byte(fixtureFeed))
	}))
	defer counting.Close()

	if _, source, err := Load(
		context.Background(),
		Options{URL: counting.URL, CachePath: cache, Now: time.Now},
	); err != nil {
		t.Fatalf("second Load failed: %v", err)
	} else if source != SourceCache {
		t.Errorf("source = %s, want cache", source)
	}

	if fetched != 0 {
		t.Errorf("fresh cache must prevent network fetch, got %d fetches", fetched)
	}
}

func TestLoadFallsBackToStaleCacheOnNetworkFailure(t *testing.T) {
	t.Parallel()

	cache := filepath.Join(t.TempDir(), "cloud.json")
	if err := os.WriteFile(cache, []byte(fixtureFeed), 0o600); err != nil {
		t.Fatal(err)
	}
	// Age the cache beyond any TTL.
	old := time.Now().Add(-72 * time.Hour)
	if err := os.Chtimes(cache, old, old); err != nil {
		t.Fatal(err)
	}

	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer dead.Close()

	set, source, err := Load(
		context.Background(),
		Options{URL: dead.URL, CachePath: cache, TTL: time.Hour, Now: time.Now},
	)
	if err != nil {
		t.Fatalf("Load must fall back to the stale cache: %v", err)
	}

	if source != SourceCache || !set.Contains(net.ParseIP("34.1.1.1")) {
		t.Errorf("stale fallback broken: source=%s", source)
	}
}

func TestLoadFailsWithoutCacheOrNetwork(t *testing.T) {
	t.Parallel()

	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer dead.Close()

	_, _, err := Load(
		context.Background(),
		Options{URL: dead.URL, CachePath: filepath.Join(t.TempDir(), "cloud.json"), Now: time.Now},
	)
	if err == nil {
		t.Fatal("Load must fail when the network fails and no cache exists")
	}
}
