package resolve

import (
	"context"
	"encoding/binary"
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
// IPv6 literal end to end: a one-shot DNS responder on [::1] answers the
// resolver's query with an empty NOERROR response, and LookupTXT must come
// back clean (empty, no error) rather than with a dial failure. Skipped on
// hosts without IPv6 loopback.
func TestNewWithAddrQueriesIPv6Server(t *testing.T) {
	conn, err := net.ListenPacket("udp", "[::1]:0")
	if err != nil {
		t.Skipf("no IPv6 loopback on this host: %v", err)
	}

	defer func() { _ = conn.Close() }()

	srvAddr := conn.LocalAddr().(*net.UDPAddr)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go serveOneEmptyResponse(ctx, conn)

	res := NewWithAddr(net.JoinHostPort(srvAddr.IP.String(), strconv.Itoa(srvAddr.Port))) // bracketed [::1]:port

	txts, err := res.LookupTXT(ctx, "example.com")
	if err != nil {
		t.Fatalf("LookupTXT over IPv6 resolver failed: %v", err)
	}

	if len(txts) != 0 {
		t.Errorf("empty response must yield no TXT records, got %v", txts)
	}
}

// serveOneEmptyResponse answers exactly one DNS query with a header-only
// NOERROR response: the question is echoed back with zero answer records,
// which every Lookup maps to "no records, no error".
func serveOneEmptyResponse(ctx context.Context, conn net.PacketConn) {
	buf := make([]byte, 512)

	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))

	n, from, err := conn.ReadFrom(buf)
	if err != nil {
		return
	}

	query := buf[:n]

	// Header (12 bytes): ID echoed, QR=1, RD copied; counts all zero
	// except QDCOUNT=1 (the echoed question).
	resp := make([]byte, 0, len(query))
	resp = append(resp, query[0], query[1])
	resp = append(resp, 0x80|(query[2]&0x01), 0x00) //nolint:gocritic // DNS header flags: QR=1 + RD bit copied

	var counts [8]byte
	binary.BigEndian.PutUint16(counts[0:], 1) // QDCOUNT
	resp = append(resp, counts[:]...)

	// Question section: copy until the terminating zero label plus the
	// 4 type/class bytes.
	i := 12
	for i < len(query) {
		if query[i] == 0 {
			resp = append(resp, query[i:i+5]...)

			break
		}

		resp = append(resp, query[i])
		i++
	}

	_ = conn.SetWriteDeadline(time.Now().Add(time.Second))

	_, _ = conn.WriteTo(resp, from)
}
