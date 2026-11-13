// Package resolve defines the DNS surface the scanner depends on plus the
// production net.Resolver-backed implementation. Tests substitute fakes for
// the interface; nothing else in the tree touches DNS directly.
package resolve

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
)

// Existence is the authoritative name-existence answer: the two response
// codes that carry a verdict about the NAME itself. Every other outcome
// (timeout, SERVFAIL, refusal) is resolver/tool state and comes back as an
// error instead, so the caller can never mistake tool trouble for domain
// truth.
type Existence uint8

const (
	// Exists: the resolver answered NOERROR — the name exists. Record
	// types may still be absent (NODATA is not a verdict about the name).
	Exists Existence = iota
	// NotExists: the resolver answered NXDOMAIN — the name does not exist.
	NotExists
)

// Resolver is the subset of DNS lookups the scanner performs. It mirrors
// net.Resolver method-for-method so *net.Resolver (with PreferGo) satisfies
// it via a thin adapter, plus one oracle the Go resolver cannot provide:
// name existence by response code (Go conflates NXDOMAIN and NODATA into a
// single IsNotFound error).
type Resolver interface {
	LookupMX(ctx context.Context, host string) ([]*net.MX, error)
	LookupTXT(ctx context.Context, host string) ([]string, error)
	LookupCNAME(ctx context.Context, host string) (string, error)
	LookupIP(ctx context.Context, host string) ([]net.IP, error)
	LookupNS(ctx context.Context, host string) ([]*net.NS, error)
	// LookupExistence reports whether host exists (NOERROR) or does not
	// (NXDOMAIN), read from the response code of a direct A query. Any
	// other response is returned as a transient error.
	LookupExistence(ctx context.Context, host string) (Existence, error)
}

type netResolver struct {
	r *net.Resolver
	// dialAddr is the server the existence oracle sends raw queries to;
	// empty means the system resolver, where the first resolv.conf
	// nameserver is used instead.
	dialAddr string
}

// New returns the production resolver: Go's built-in DNS client (PreferGo)
// for deterministic behavior independent of the host's resolver library.
func New() Resolver {
	return &netResolver{r: &net.Resolver{PreferGo: true}}
}

// NewWithAddr returns a resolver that queries one DNS server directly
// (host or host:port, port defaults to 53) instead of the system resolver —
// for bulk scans that must not ride the local forwarding resolver. Bare
// IPv6 addresses are bracketed; an already-bracketed host:port form passes
// through unchanged (see resolverDialAddr).
func NewWithAddr(addr string) Resolver {
	return &netResolver{
		r: &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
				var dialer net.Dialer

				return dialer.DialContext(ctx, network, resolverDialAddr(addr))
			},
		},
		dialAddr: resolverDialAddr(addr),
	}
}

// LookupExistence implements the existence oracle: an A query read at the
// response-code level. NOERROR (with or without records) means the name
// exists; NXDOMAIN means it does not; everything else (SERVFAIL, refusal,
// network trouble) is a transient error, never a verdict. A truncated UDP
// response is retried over TCP.
func (n *netResolver) LookupExistence(ctx context.Context, host string) (Existence, error) {
	query, id, err := buildQuery(host)
	if err != nil {
		return Exists, fmt.Errorf("existence query for %s: %w", host, err)
	}

	addr := n.dialAddr
	if addr == "" {
		addr, err = firstSystemNameserver()
		if err != nil {
			return Exists, fmt.Errorf("existence query for %s: %w", host, err)
		}
	}

	resp, err := exchangeUDP(ctx, addr, query)
	if err != nil {
		return Exists, wrapExistenceNetErr(err, host, addr)
	}

	rcode, truncated, err := parseRcode(resp, id)
	if err != nil {
		return Exists, wrapExistenceNetErr(err, host, addr)
	}

	if truncated {
		resp, err = exchangeTCP(ctx, addr, query)
		if err != nil {
			return Exists, wrapExistenceNetErr(err, host, addr)
		}

		rcode, _, err = parseRcode(resp, id)
		if err != nil {
			return Exists, wrapExistenceNetErr(err, host, addr)
		}
	}

	switch rcode {
	case 0:
		return Exists, nil
	case dnsRcodeNameError:
		return NotExists, nil
	default:
		return Exists, transientRcodeErr(host, addr, rcode)
	}
}

// errNoNameserver means /etc/resolv.conf carries no usable nameserver
// line for the default-resolver oracle.
var errNoNameserver = errors.New("no nameserver line in /etc/resolv.conf")

// firstSystemNameserver returns the first nameserver from /etc/resolv.conf
// (lazy — only consulted by resolvers built with New(); the census always
// pins an explicit --resolver).
func firstSystemNameserver() (string, error) {
	f, err := os.Open("/etc/resolv.conf")
	if err != nil {
		return "", fmt.Errorf("no --resolver given and %w", err)
	}

	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if server, ok := strings.CutPrefix(line, "nameserver "); ok {
			return resolverDialAddr(strings.TrimSpace(server)), nil
		}
	}

	if scanErr := scanner.Err(); scanErr != nil {
		return "", fmt.Errorf("reading /etc/resolv.conf: %w", scanErr)
	}

	return "", fmt.Errorf("no --resolver given and %w", errNoNameserver)
}

// resolverDialAddr normalizes a --resolver value into a dialable address.
// net.SplitHostPort rejects a bare IPv6 literal (too many colons), which is
// exactly the signal to bracket it with the default DNS port; bracketed
// host:port forms already parse and pass through.
func resolverDialAddr(addr string) string {
	if _, _, err := net.SplitHostPort(addr); err != nil {
		return net.JoinHostPort(addr, "53")
	}

	return addr
}

func (n *netResolver) LookupMX(ctx context.Context, host string) ([]*net.MX, error) {
	return n.r.LookupMX(ctx, host)
}

func (n *netResolver) LookupTXT(ctx context.Context, host string) ([]string, error) {
	return n.r.LookupTXT(ctx, host)
}

func (n *netResolver) LookupCNAME(ctx context.Context, host string) (string, error) {
	return n.r.LookupCNAME(ctx, host)
}

func (n *netResolver) LookupIP(ctx context.Context, host string) ([]net.IP, error) {
	return n.r.LookupIP(ctx, "ip", host)
}

func (n *netResolver) LookupNS(ctx context.Context, host string) ([]*net.NS, error) {
	return n.r.LookupNS(ctx, host)
}
