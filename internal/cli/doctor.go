package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/larsartmann/google-domain-detector/internal/ranges"
	"github.com/larsartmann/google-domain-detector/internal/resolve"
)

// probeDNS and probeRanges are the doctor's domain preflights, behind
// package variables so tests can swap them (the doctor battery prints to
// process stdout and probes the real network; the offline suite pins the
// error paths with fakes instead).
var (
	probeDNS = checkDNSResolver
	//nolint:gochecknoglobals // test seam for the offline suite
	probeRanges = checkRangesFeed
)

// checkDNSResolver proves the resolver a scan would use answers a real
// lookup inside a 5s budget: GDD_RESOLVER when set (mirroring scan's env
// tag), the system resolver otherwise. Failure names the fix.
func checkDNSResolver(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	res := resolve.New()

	addr := os.Getenv("GDD_RESOLVER")
	if addr != "" {
		res = resolve.NewWithAddr(addr)
	}

	const probe = "example.com"

	if _, err := res.LookupIP(ctx, probe); err != nil {
		hint := "check network connectivity; bulk scans want --resolver host[:port]"
		if addr != "" {
			hint = fmt.Sprintf("verify the GDD_RESOLVER address %s or unset it", addr)
		}

		return fmt.Errorf("resolver did not answer a lookup for %s: %w (hint: %s)", probe, err, hint)
	}

	return nil
}

// checkRangesFeed proves the GCP range feed is loadable inside a 10s
// budget — reachable network feed or a readable cache. Failure carries the
// ranges error family (75 fetch / 65 corrupt) and names the fix.
func checkRangesFeed(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	url := os.Getenv("GDD_RANGES_URL")
	if url == "" {
		url = ranges.DefaultURL
	}

	cache, err := defaultRangesCache()
	if err != nil {
		return err
	}

	if envCache := os.Getenv("GDD_RANGES_CACHE"); envCache != "" {
		cache = envCache
	}

	if _, _, err := ranges.Load(ctx, ranges.Options{URL: url, CachePath: cache, UserAgent: Name}); err != nil {
		if feedErr, ok := errors.AsType[*ranges.Error](err); ok {
			switch feedErr.Kind {
			case ranges.KindFetch:
				return fmt.Errorf(
					"GCP range feed unreachable: %w (scan would degrade to no IP evidence; check connectivity or --ranges-url)",
					err,
				)
			case ranges.KindCorrupt:
				return fmt.Errorf(
					"GCP range feed corrupt: %w (try a different --ranges-url or clear the cache file %s)",
					err,
					cache,
				)
			}
		}

		return err
	}

	return nil
}

// defaultRangesCache is the cache path scans read when no --ranges-cache
// was passed.
func defaultRangesCache() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve cache dir: %w", err)
	}

	return filepath.Join(home, ".cache", Name, "cloud.json"), nil
}
