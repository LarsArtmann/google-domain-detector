// Package resolve defines the DNS surface the scanner depends on plus the
// production net.Resolver-backed implementation. Tests substitute fakes for
// the interface; nothing else in the tree touches DNS directly.
package resolve

import (
	"context"
	"net"
)

// Resolver is the subset of DNS lookups the scanner performs. It mirrors
// net.Resolver method-for-method so *net.Resolver (with PreferGo) satisfies
// it via a thin adapter.
type Resolver interface {
	LookupMX(ctx context.Context, host string) ([]*net.MX, error)
	LookupTXT(ctx context.Context, host string) ([]string, error)
	LookupCNAME(ctx context.Context, host string) (string, error)
	LookupIP(ctx context.Context, host string) ([]net.IP, error)
	LookupNS(ctx context.Context, host string) ([]*net.NS, error)
}

type netResolver struct{ r *net.Resolver }

// New returns the production resolver: Go's built-in DNS client (PreferGo)
// for deterministic behavior independent of the host's resolver library.
func New() Resolver {
	return &netResolver{r: &net.Resolver{PreferGo: true}}
}

// NewWithAddr returns a resolver that queries one DNS server directly
// (host or host:port, port defaults to 53) instead of the system resolver —
// for bulk scans that must not ride the local forwarding resolver.
func NewWithAddr(addr string) Resolver {
	if _, _, err := net.SplitHostPort(addr); err != nil {
		addr = net.JoinHostPort(addr, "53")
	}

	return &netResolver{r: &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			var dialer net.Dialer

			return dialer.DialContext(ctx, network, addr)
		},
	}}
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
