package resolve

import (
	"context"
	"errors"
	"net"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

func TestResolverDialAddr(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ in, want string }{
		{"9.9.9.9", "9.9.9.9:53"},
		{"9.9.9.9:5353", "9.9.9.9:5353"},
		{"127.0.0.1", "127.0.0.1:53"},
		{"::1", "[::1]:53"},
		{"2001:db8::1", "[2001:db8::1]:53"},
		{"[2001:db8::1]:5353", "[2001:db8::1]:5353"},
		{"hostname.example", "hostname.example:53"},
	} {
		if got := resolverDialAddr(tc.in); got != tc.want {
			t.Errorf("resolverDialAddr(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestNewWithAddrQueriesIPv6Server proves the --resolver flag dials a real
// IPv6 literal end to end. A responder on [::1] echoes every query back
// with the QR bit set (NOERROR, zero answers); the Go resolver parses
// that as "domain name has no TXT record" — a *net.DNSError with
// IsNotFound. A broken v6 dial path would surface as an i/o timeout
// instead, which fails the test. The responder loops (a probe or retried
// query must not starve it) and records whether any query arrived, so a
// failure states whether the dial path or the response parsing broke.
// Skipped on hosts without IPv6 loopback.
func TestNewWithAddrQueriesIPv6Server(t *testing.T) {
	t.Parallel()

	//nolint:noctx // test-local listener; no cancellation contract needed
	conn, err := net.ListenPacket("udp", "[::1]:0")
	if err != nil {
		t.Skipf("no IPv6 loopback on this host: %v", err)
	}

	defer func() { _ = conn.Close() }()

	srvAddr := conn.LocalAddr().(*net.UDPAddr)

	var gotQuery atomic.Bool

	go echoDNSResponses(conn, 10*time.Second, &gotQuery)

	res := NewWithAddr(net.JoinHostPort(srvAddr.IP.String(), strconv.Itoa(srvAddr.Port))) // bracketed [::1]:port

	var lastErr error

	for range 2 {
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)

		_, err = res.LookupTXT(ctx, "example.com")

		cancel()

		if dnsErr, ok := errors.AsType[*net.DNSError](err); ok && dnsErr.IsNotFound {
			return
		}

		lastErr = err

		time.Sleep(200 * time.Millisecond)
	}

	if !gotQuery.Load() {
		t.Fatalf("resolver never sent a query to the [::1] responder (dial path broken?): %v", lastErr)
	}

	t.Fatalf("query reached the responder but no echoed response was accepted: %v", lastErr)
}

// echoDNSResponses answers DNS queries until the deadline by echoing each
// one with the QR bit set: same ID, same question, zero answers — a
// well-formed empty NOERROR response the Go resolver accepts.
func echoDNSResponses(conn net.PacketConn, window time.Duration, gotQuery *atomic.Bool) {
	var buf [512]byte

	_ = conn.SetReadDeadline(time.Now().Add(window))

	for {
		n, from, err := conn.ReadFrom(buf[:])
		if err != nil {
			return
		}

		gotQuery.Store(true)

		resp := append([]byte{}, buf[:n]...)
		resp[2] |= 0x80 // response bit

		_ = conn.SetWriteDeadline(time.Now().Add(time.Second))

		if _, err := conn.WriteTo(resp, from); err != nil {
			return
		}
	}
}
