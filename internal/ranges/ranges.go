// Package ranges parses Google's published GCP IP-range feed
// (https://www.gstatic.com/ipranges/cloud.json), caches it on disk, and
// answers membership queries. cloud.json is the GCP-specific feed (Google
// Cloud Customer's public IP space); goog.json would also cover Google's own
// properties and is deliberately NOT used — the signal sought is "served by
// GCP", not "touches Google anywhere".
package ranges

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"time"

	atomicwrite "github.com/larsartmann/go-atomic-write"
)

// DefaultURL is Google's official machine-readable GCP range feed.
const DefaultURL = "https://www.gstatic.com/ipranges/cloud.json"

// DefaultUserAgent identifies the tool to the feed server when Options
// carries no explicit agent.
const DefaultUserAgent = "google-domain-detector"

// DefaultTTL is how long a cached feed stays fresh.
const DefaultTTL = 24 * time.Hour

// maxFeedBytes caps the downloaded feed (the real one is ~100 KiB).
const maxFeedBytes = 10 << 20

// anyAge admits a cache file of any age (stale-cache fallback).
const anyAge = time.Duration(math.MaxInt64)

// Set is a parsed collection of GCP CIDR prefixes.
type Set struct {
	v4 []netip.Prefix
	v6 []netip.Prefix
}

type feed struct { //nolint:tagliatelle // field names are Google cloud.json's wire format
	Prefixes []struct {
		IPv4Prefix string `json:"ipv4Prefix"`
		IPv6Prefix string `json:"ipv6Prefix"`
	} `json:"prefixes"`
}

// Source says where a loaded set came from.
type Source string

const (
	SourceNetwork Source = "network"
	SourceCache   Source = "cache"
)

// ErrorKind classifies a range-feed failure: the feed could not be fetched
// (transient, worth retrying later) or could not be parsed (corrupt, not
// worth retrying unchanged).
type ErrorKind string

const (
	KindFetch   ErrorKind = "fetch"
	KindCorrupt ErrorKind = "corrupt"
)

// Exit codes for the feed-failure families, from sysexits(3): 75
// (EX_TEMPFAIL) when the feed cannot be fetched, 65 (EX_DATAERR) when a
// fetched feed is corrupt.
const (
	ExitFetch   = 75
	ExitCorrupt = 65
)

// Failure cores for the two feed-failure families. Load wraps them in
// Error carrying the Kind and exit code; tests and callers match with
// errors.Is.
var (
	ErrNoPrefixes = errors.New("range feed contains no prefixes")
	ErrBadStatus  = errors.New("range feed returned an unexpected HTTP status")
)

const (
	defaultFetchTimeout = 10 * time.Second
	cacheDirPerm        = 0o700
	cacheFilePerm       = 0o600
)

// Error is a range-feed failure carrying its exit-code family. It satisfies
// cmdguard's ExitCoder structurally, so a scan whose feed dies exits 75/65
// instead of the generic 1.
type Error struct {
	Kind ErrorKind
	err  error
}

func (e *Error) Error() string { return e.err.Error() }

func (e *Error) Unwrap() error { return e.err }

func (e *Error) ExitCode() int {
	if e.Kind == KindCorrupt {
		return ExitCorrupt
	}

	return ExitFetch
}

// Parse decodes a cloud.json document.
func Parse(data []byte) (*Set, error) {
	var f feed
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse range feed: %w", err)
	}

	set := &Set{}

	for _, p := range f.Prefixes {
		switch {
		case p.IPv4Prefix != "":
			prefix, err := netip.ParsePrefix(p.IPv4Prefix)
			if err != nil {
				return nil, fmt.Errorf("parse IPv4 prefix %q: %w", p.IPv4Prefix, err)
			}

			set.v4 = append(set.v4, prefix)
		case p.IPv6Prefix != "":
			prefix, err := netip.ParsePrefix(p.IPv6Prefix)
			if err != nil {
				return nil, fmt.Errorf("parse IPv6 prefix %q: %w", p.IPv6Prefix, err)
			}

			set.v6 = append(set.v6, prefix)
		}
	}

	if set.Len() == 0 {
		return nil, ErrNoPrefixes
	}

	return set, nil
}

// Len returns the number of prefixes in the set.
func (s *Set) Len() int {
	if s == nil {
		return 0
	}

	return len(s.v4) + len(s.v6)
}

// Contains reports whether the IP falls inside any GCP prefix.
func (s *Set) Contains(ip net.IP) bool {
	if s == nil || ip == nil {
		return false
	}

	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}

	if addr.Is4In6() {
		addr = addr.Unmap()
	}

	list := s.v4
	if addr.Is6() {
		list = s.v6
	}

	for _, prefix := range list {
		if prefix.Contains(addr) {
			return true
		}
	}

	return false
}

// Options controls Load.
type Options struct {
	// URL of the feed; empty means DefaultURL.
	URL string
	// CachePath of the on-disk cache file; empty disables caching.
	CachePath string
	// TTL of the cache; zero means DefaultTTL. A cache older than TTL is
	// still used as a fallback when the network fetch fails.
	TTL time.Duration
	// Client used for the fetch; nil means a client with a 10 s timeout.
	Client *http.Client
	// UserAgent sent to the feed server; empty means DefaultUserAgent.
	UserAgent string
	// Now overrides the clock (tests); nil means time.Now.
	Now func() time.Time
}

// Load returns the GCP ranges: from cache when fresh, otherwise fetched over
// the network (and written back to the cache). A stale cache is the fallback
// when the fetch fails. The only error is "no ranges from anywhere".
func Load(ctx context.Context, opts Options) (*Set, Source, error) {
	if opts.URL == "" {
		opts.URL = DefaultURL
	}

	if opts.TTL <= 0 {
		opts.TTL = DefaultTTL
	}

	client := opts.Client
	if client == nil {
		client = &http.Client{Timeout: defaultFetchTimeout}
	}

	userAgent := opts.UserAgent
	if userAgent == "" {
		userAgent = DefaultUserAgent
	}

	now := time.Now
	if opts.Now != nil {
		now = opts.Now
	}

	if set, ok := readCache(opts.CachePath, opts.TTL, now); ok {
		return set, SourceCache, nil
	}

	data, err := fetch(ctx, client, opts.URL, userAgent)
	if err == nil {
		set, parseErr := Parse(data)
		if parseErr == nil {
			writeCache(opts.CachePath, data)

			return set, SourceNetwork, nil
		}

		return nil, Source(""), &Error{Kind: KindCorrupt, err: fmt.Errorf("fetched feed is unusable: %w", parseErr)}
	}

	if set, ok := readCache(opts.CachePath, anyAge, now); ok {
		return set, SourceCache, nil
	}

	return nil, Source(""), &Error{Kind: KindFetch, err: fmt.Errorf("fetch %s: %w", opts.URL, err)}
}

func readCache(path string, maxAge time.Duration, now func() time.Time) (*Set, bool) {
	if path == "" {
		return nil, false
	}

	info, err := os.Stat(path)
	if err != nil || now().Sub(info.ModTime()) > maxAge {
		return nil, false
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, false
	}

	data, readErr := io.ReadAll(f)

	closeErr := f.Close()
	if readErr != nil || closeErr != nil {
		return nil, false
	}

	set, err := Parse(data)
	if err != nil {
		return nil, false
	}

	return set, true
}

func writeCache(path string, data []byte) {
	if path == "" {
		return
	}

	if err := os.MkdirAll(filepath.Dir(path), cacheDirPerm); err != nil {
		return
	}

	// WriteWithPerm, never WriteIfChanged: a feed whose bytes did not change
	// must still refresh the mtime the TTL freshness check reads.
	_ = atomicwrite.WriteWithPerm(path, data, cacheFilePerm)
}

func fetch(ctx context.Context, client *http.Client, url, userAgent string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %w", resp.Status, ErrBadStatus)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxFeedBytes))
	if err != nil {
		return nil, err
	}

	return body, nil
}
