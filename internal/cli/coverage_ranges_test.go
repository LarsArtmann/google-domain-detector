package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/google-domain-detector/internal/ranges"
)

func TestLoadRangesUsesFreshCacheWithoutNetwork(t *testing.T) {
	t.Parallel()

	cache := filepath.Join(t.TempDir(), "cloud.json")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(fixtureFeed))
	}))
	defer server.Close()

	if _, _, err := ranges.Load(
		context.Background(),
		ranges.Options{URL: server.URL, CachePath: cache, Now: time.Now},
	); err != nil {
		t.Fatalf("seed cache: %v", err)
	}

	set, err := loadRanges(context.Background(), ScanFlags{
		CachePath: cache,
		RangesURL: "http://127.0.0.1:1/unreachable",
	}, "test")
	if err != nil {
		t.Fatalf("fresh cache must load without network: %v", err)
	}

	if set == nil {
		t.Fatal("ranges set missing despite valid cache")
	}
}

func TestLoadRangesNoRangesReturnsNilSet(t *testing.T) {
	t.Parallel()

	set, err := loadRanges(context.Background(), ScanFlags{NoRanges: true}, "test")
	if err != nil {
		t.Fatalf("--no-ranges must not error: %v", err)
	}

	if set != nil {
		t.Error("--no-ranges must disable the set (nil), got one")
	}
}

func TestDoctorCheckRangesFeedFromCache(t *testing.T) {
	cache := filepath.Join(t.TempDir(), "cloud.json")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(fixtureFeed))
	}))
	defer server.Close()

	if _, _, err := ranges.Load(
		context.Background(),
		ranges.Options{URL: server.URL, CachePath: cache, Now: time.Now},
	); err != nil {
		t.Fatalf("seed cache: %v", err)
	}

	t.Setenv("GDD_RANGES_URL", "http://127.0.0.1:1/unreachable")
	t.Setenv("GDD_RANGES_CACHE", cache)

	if err := checkRangesFeed(context.Background()); err != nil {
		t.Errorf("readable cache must pass the feed probe: %v", err)
	}
}

func TestDoctorCheckRangesFeedReportsFailure(t *testing.T) {
	t.Setenv("GDD_RANGES_URL", "http://127.0.0.1:1/unreachable")
	t.Setenv("GDD_RANGES_CACHE", filepath.Join(t.TempDir(), "absent", "cloud.json"))

	if err := checkRangesFeed(context.Background()); err == nil {
		t.Error("unreachable feed without cache must fail the probe")
	}
}

func TestDefaultRangesCacheIsUnderUserCacheDir(t *testing.T) {
	t.Parallel()

	cache, err := defaultRangesCache()
	if err != nil {
		t.Fatalf("default cache path: %v", err)
	}

	if !strings.Contains(cache, "google-domain-detector") || !strings.HasSuffix(cache, "cloud.json") {
		t.Errorf("default cache = %q, want .../google-domain-detector/cloud.json", cache)
	}
}

func TestLoadRangesFetchFailureWithoutCache(t *testing.T) {
	t.Parallel()

	_, err := loadRanges(context.Background(), ScanFlags{
		RangesURL: "http://127.0.0.1:1/unreachable",
		CachePath: filepath.Join(t.TempDir(), "absent", "cloud.json"),
	}, "test")
	if err == nil {
		t.Error("unreachable feed without cache must fail")
	}
}

func TestLoadRangesDefaultCacheUnderXDGBundle(t *testing.T) {
	cacheHome := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheHome)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(fixtureFeed))
	}))
	defer server.Close()

	set, err := loadRanges(context.Background(), ScanFlags{RangesURL: server.URL}, "test")
	if err != nil {
		t.Fatalf("load via default cache path failed: %v", err)
	}

	if set == nil {
		t.Fatal("expected a ranges set")
	}

	defaultPath, err := defaultRangesCache()
	if err != nil {
		t.Fatalf("default cache path: %v", err)
	}

	if !strings.HasPrefix(defaultPath, cacheHome) {
		t.Errorf("default cache = %q, want under XDG_CACHE_HOME %q", defaultPath, cacheHome)
	}
}

func TestCheckRangesFeedCorruptFeedNamesFix(t *testing.T) {
	garbage := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("{not json"))
	}))
	defer garbage.Close()

	t.Setenv("GDD_RANGES_URL", garbage.URL)
	t.Setenv("GDD_RANGES_CACHE", filepath.Join(t.TempDir(), "cloud.json"))

	err := checkRangesFeed(context.Background())
	if err == nil || !strings.Contains(err.Error(), "corrupt") {
		t.Errorf("unparseable feed must fail the probe with the corrupt family, got: %v", err)
	}
}

func TestDefaultRangesCacheFailsWithoutHome(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("HOME", "")

	if _, err := defaultRangesCache(); err == nil {
		t.Error("no HOME and no XDG_CACHE_HOME must fail the cache-path lookup")
	}
}

func TestCheckRangesFeedHealthyFeedPasses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(fixtureFeed))
	}))
	defer server.Close()

	t.Setenv("GDD_RANGES_URL", server.URL)
	t.Setenv("GDD_RANGES_CACHE", filepath.Join(t.TempDir(), "cloud.json"))

	if err := checkRangesFeed(context.Background()); err != nil {
		t.Errorf("healthy feed must pass the probe, got: %v", err)
	}
}

// TestEnrichMalformedCSVFailsCleanly covers the row-read error branch: a
// bare-quote line must abort with the file line named, not silently drop.
