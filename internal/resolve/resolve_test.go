package resolve

import (
	"context"
	"errors"
	"net"
	"strconv"
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
// IPv6 literal end to end. A one-shot responder on [::1] echoes the query
// back with the QR bit set (NOERROR, zero answers); the Go resolver parses
// that as "domain name has no TXT record" — a *net.DNSError with
// IsNotFound. A broken v6 dial path would surface as an i/o timeout
// instead, which fails the test. Skipped on hosts without IPv6 loopback.
func TestNewWithAddrQueriesIPv6Server(t *testing.T) {
	t.Parallel()

	//nolint:noctx // test-local listener; no cancellation contract needed
	conn, err := net.ListenPacket("udp", "[::1]:0")
	if err != nil {
		t.Skipf("no IPv6 loopback on this host: %v", err)
	}

	defer func() { _ = conn.Close() }()

	srvAddr := conn.LocalAddr().(*net.UDPAddr)

	go echoOneDNSResponse(conn)

	res := NewWithAddr(net.JoinHostPort(srvAddr.IP.String(), strconv.Itoa(srvAddr.Port))) // bracketed [::1]:port

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = res.LookupTXT(ctx, "example.com")

	dnsErr, ok := errors.AsType[*net.DNSError](err)
	if !ok || !dnsErr.IsNotFound {
		t.Fatalf("LookupTXT over IPv6 resolver must parse the empty response (IsNotFound), got: %v", err)
	}
}

// echoOneDNSResponse answers exactly one DNS query by echoing it with the
// QR bit set: same ID, same question, zero answers — a well-formed empty
// NOERROR response the Go resolver accepts.
func echoOneDNSResponse(conn net.PacketConn) {
	buf := make([]byte, 512)

	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))

	n, from, err := conn.ReadFrom(buf)
	if err != nil {
		return
	}

	resp := append([]byte{}, buf[:n]...)
	resp[2] |= 0x80 // response bit

	_ = conn.SetWriteDeadline(time.Now().Add(time.Second))

	_, _ = conn.WriteTo(resp, from)
}
