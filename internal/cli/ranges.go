package cli

import (
	"context"
	"os"

	"github.com/larsartmann/google-domain-detector/internal/ranges"
)

// rangesFile loads the GCP IP-range set for --ranges (cached, stale-safe).

func loadRanges(ctx context.Context, f ScanFlags, version string) (*ranges.Set, error) {
	if f.NoRanges {
		return nil, nil //nolint:nilnil // --no-ranges disables the set by design; absence is not an error
	}

	cache := f.CachePath
	if cache == "" {
		defaultCache, err := defaultRangesCache()
		if err != nil {
			return nil, err
		}

		cache = defaultCache
	}

	userAgent := Name
	if version != "" {
		userAgent = Name + "/" + version
	}

	set, _, err := ranges.Load(ctx, ranges.Options{URL: f.RangesURL, CachePath: cache, UserAgent: userAgent})
	if err != nil {
		return nil, err
	}

	return set, nil
}

func stdinIsTTY() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}

	return info.Mode()&os.ModeCharDevice != 0
}
