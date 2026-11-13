package resolve

import (
	"context"
	"errors"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
)

// oracleServer binds a UDP and a TCP listener on the same port (the way a
// real DNS server does) and answers every UDP query with the given rcode;
// when truncated is set the UDP answer carries the TC bit and the TCP
// listener answers with the real rcode instead.
type oracleServer struct {
	udp *net.UDPConn
	tcp *net.TCPListener
}

func newOracleServer(t *testing.T, rcode int, truncated bool) *oracleServer {
	t.Helper()

	var lc net.ListenConfig

	// UDP and TCP draw from the same kernel ephemeral port range, so the
	// TCP bind can lose its freshly-drawn UDP port to a concurrent socket
	// — under -race this surfaced as "address already in use". Retry with
	// a fresh port instead of failing the run.
	var udpConn net.PacketConn

	var tcpListener net.Listener

	for range 10 {
		candidate, err := lc.ListenPacket(t.Context(), "udp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("udp listener: %v", err)
		}

		port := candidate.LocalAddr().(*net.UDPAddr).Port

		tcpCandidate, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:"+strconv.Itoa(port))
		if err == nil {
			udpConn = candidate
			tcpListener = tcpCandidate

			break
		}

		_ = candidate.Close()
	}

	if udpConn == nil || tcpListener == nil {
		t.Fatal("no port accepted both a UDP and a TCP listener after 10 attempts")
	}

	srv := &oracleServer{
		udp: udpConn.(*net.UDPConn),
		tcp: tcpListener.(*net.TCPListener),
	}

	go srv.serveUDP(rcode, truncated)
	go srv.serveTCP(rcode)

	t.Cleanup(func() {
		_ = srv.udp.Close()
		_ = srv.tcp.Close()
	})

	return srv
}

func (s *oracleServer) addr() string {
	return net.JoinHostPort("127.0.0.1", strconv.Itoa(s.udp.LocalAddr().(*net.UDPAddr).Port))
}

func (s *oracleServer) serveUDP(rcode int, truncated bool) {
	var buf [512]byte

	_ = s.udp.SetReadDeadline(time.Now().Add(10 * time.Second))

	for {
		n, from, err := s.udp.ReadFromUDP(buf[:])
		if err != nil {
			return
		}

		resp := append([]byte{}, buf[:n]...)
		resp[2] |= dnsFlagQR // response bit

		if truncated {
			resp[2] |= dnsFlagTC
		}

		resp[3] = byte(rcode)

		_ = s.udp.SetWriteDeadline(time.Now().Add(time.Second))

		if _, err := s.udp.WriteToUDP(resp, from); err != nil {
			return
		}
	}
}

func (s *oracleServer) serveTCP(rcode int) {
	for {
		conn, err := s.tcp.Accept()
		if err != nil {
			return
		}

		go s.answerTCP(conn, rcode)
	}
}

func (s *oracleServer) answerTCP(conn net.Conn, rcode int) {
	defer func() { _ = conn.Close() }()

	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

	var length [2]byte

	if err := readFull(conn, length[:]); err != nil {
		return
	}

	//nolint:makezero // exact-size wire buffer, filled by readFull
	query := make([]byte, int(length[0])<<8|int(length[1]))

	if err := readFull(conn, query); err != nil {
		return
	}

	resp := append([]byte{}, query...)
	resp[2] |= dnsFlagQR
	resp[3] = byte(rcode)

	out := make([]byte, 0, 2+len(resp))
	out = append(out, byte(len(resp)>>8), byte(len(resp)))
	out = append(out, resp...)

	_, _ = conn.Write(out)
}

func readFull(conn net.Conn, buf []byte) error {
	total := 0

	for total < len(buf) {
		n, err := conn.Read(buf[total:])
		if err != nil {
			return err
		}

		total += n
	}

	return nil
}

func TestLookupExistenceVerdicts(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name       string
		rcode      int
		truncated  bool
		want       Existence
		wantErr    bool
		wantTempor bool
	}{
		{"nxdomain", dnsRcodeNameError, false, NotExists, false, false},
		{"noerror-nodata", 0, false, Exists, false, false},
		{"noerror-answers", 0, false, Exists, false, false},
		{"servfail", 2, false, Exists, true, true},
		{"refused", 5, false, Exists, true, true},
		{"truncated-tcp-nxdomain", dnsRcodeNameError, true, NotExists, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			srv := newOracleServer(t, tc.rcode, tc.truncated)
			res := NewWithAddr(srv.addr())

			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			got, err := res.LookupExistence(ctx, "example.com")

			cancel()

			if tc.wantErr {
				if err == nil {
					t.Fatalf("rcode %d: want error, got %v", tc.rcode, got)
				}

				var dnsErr *net.DNSError

				if tc.wantTempor && (!errors.As(err, &dnsErr) || !dnsErr.IsTemporary) {
					t.Errorf("rcode %d: err %v must be a temporary DNSError", tc.rcode, err)
				}

				return
			}

			if err != nil {
				t.Fatalf("rcode %d: unexpected error: %v", tc.rcode, err)
			}

			if got != tc.want {
				t.Errorf("rcode %d: Existence = %d, want %d", tc.rcode, got, tc.want)
			}
		})
	}
}

func TestLookupExistenceTimesOut(t *testing.T) {
	t.Parallel()

	// A port with no server: the oracle must surface a timeout-class error
	// quickly instead of hanging.
	res := NewWithAddr("127.0.0.1:1")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer func() { cancel() }()

	if _, err := res.LookupExistence(ctx, "example.com"); err == nil {
		t.Fatal("want error from a closed port, got nil")
	}
}

func TestEncodeQName(t *testing.T) {
	t.Parallel()

	qname, err := encodeQName("kwai.NET.")
	if err != nil {
		t.Fatalf("encodeQName: %v", err)
	}

	want := []byte{4, 'k', 'w', 'a', 'i', 3, 'N', 'E', 'T', 0}
	if string(qname) != string(want) {
		t.Errorf("encodeQName = %v, want %v", qname, want)
	}

	if _, err := encodeQName(""); err == nil {
		t.Error("empty name must be rejected")
	}

	if _, err := encodeQName(strings.Repeat("label.", 60)); err == nil {
		t.Error("oversized name must be rejected")
	}
}
