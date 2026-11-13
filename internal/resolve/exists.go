package resolve

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

// Minimal DNS wire client for name-existence queries. Go's Resolver
// collapses "name does not exist" (rcode NXDOMAIN) and "name exists but
// has no such record" (NODATA) into the same DNSError with IsNotFound,
// which misclassifies every live domain without an MX record as
// NXDOMAIN-dead (the 2026-11-13 census finding: 27 of 30 sampled wave-A
// "NXDOMAIN" verdicts were live domains, e.g. windowsupdate.com). The
// oracle speaks just enough DNS to read the response rcode and separate
// the two: A-record query, header rcode, TC-bit fallback to TCP.

const (
	dnsFlagRD byte = 0x01 // recursion desired (header byte 2)
	dnsFlagTC byte = 0x02 // truncation (header byte 2, read back)
	dnsFlagQR byte = 0x80 // response (header byte 2, read back)

	dnsQTypeA   uint16 = 1 // A
	dnsQClassIN uint16 = 1 // IN

	// dnsRcodeNameError is the NXDOMAIN response code.
	dnsRcodeNameError = 3

	// existsTimeout bounds one oracle round trip when the caller's context
	// carries no deadline.
	existsTimeout = 5 * time.Second
)

// buildQuery packs an A-record query for host with a random message ID.
func buildQuery(host string) ([]byte, uint16, error) {
	var id [2]byte

	if _, err := rand.Read(id[:]); err != nil {
		return nil, 0, fmt.Errorf("existence query id: %w", err)
	}

	qname, err := encodeQName(host)
	if err != nil {
		return nil, 0, err
	}

	msg := make([]byte, 0, 12+len(qname)+4)
	msg = append(msg, id[0], id[1])
	msg = append(msg, dnsFlagRD, 0x00)          // RD set; opcode/rcode zero
	msg = binary.BigEndian.AppendUint16(msg, 1) // QDCOUNT
	msg = binary.BigEndian.AppendUint16(msg, 0) // ANCOUNT
	msg = binary.BigEndian.AppendUint16(msg, 0) // NSCOUNT
	msg = binary.BigEndian.AppendUint16(msg, 0) // ARCOUNT
	msg = append(msg, qname...)
	msg = binary.BigEndian.AppendUint16(msg, dnsQTypeA)
	msg = binary.BigEndian.AppendUint16(msg, dnsQClassIN)

	return msg, binary.BigEndian.Uint16(id[:]), nil
}

// encodeQName renders host as DNS wire-format labels terminated by the
// root byte.
func encodeQName(host string) ([]byte, error) {
	host = trimTrailingDot(host)
	if host == "" {
		return nil, fmt.Errorf("empty query name")
	}

	qname := make([]byte, 0, len(host)+2)

	for label := range strings.SplitSeq(host, ".") {
		if len(label) == 0 {
			return nil, fmt.Errorf("empty label in %q", host)
		}

		if len(label) > 63 {
			return nil, fmt.Errorf("label longer than 63 bytes in %q", host)
		}

		qname = append(qname, byte(len(label)))
		qname = append(qname, label...)
	}

	if len(qname) > 253 {
		return nil, fmt.Errorf("query name longer than 253 bytes: %q", host)
	}

	return append(qname, 0x00), nil
}

func trimTrailingDot(host string) string {
	if len(host) > 0 && host[len(host)-1] == '.' {
		return host[:len(host)-1]
	}

	return host
}

// parseRcode validates a DNS response and returns its rcode and TC bit.
// The message must carry the header, answer the query ID, and have the QR
// bit set.
func parseRcode(msg []byte, wantID uint16) (rcode int, truncated bool, err error) {
	if len(msg) < 12 {
		return 0, false, fmt.Errorf("dns response shorter than header: %d bytes", len(msg))
	}

	if id := binary.BigEndian.Uint16(msg[0:2]); id != wantID {
		return 0, false, fmt.Errorf("dns response id mismatch: got %d, want %d", id, wantID)
	}

	if msg[2]&dnsFlagQR == 0 {
		return 0, false, fmt.Errorf("dns response lacks QR bit")
	}

	return int(msg[3] & 0x0F), msg[2]&dnsFlagTC != 0, nil
}

// exchangeUDP sends query to addr over UDP and returns the response.
func exchangeUDP(ctx context.Context, addr string, query []byte) ([]byte, error) {
	var d net.Dialer

	conn, err := d.DialContext(ctx, "udp", addr)
	if err != nil {
		return nil, err
	}

	defer func() { _ = conn.Close() }()

	if err := bindDeadline(ctx, conn); err != nil {
		return nil, err
	}

	if _, err := conn.Write(query); err != nil {
		return nil, err
	}

	buf := make([]byte, 512)

	n, readErr := conn.Read(buf)
	if readErr != nil {
		return nil, readErr
	}

	return buf[:n], nil
}

// exchangeTCP sends query to addr length-prefixed over TCP and returns
// the response body.
func exchangeTCP(ctx context.Context, addr string, query []byte) ([]byte, error) {
	var d net.Dialer

	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}

	defer func() { _ = conn.Close() }()

	if err := bindDeadline(ctx, conn); err != nil {
		return nil, err
	}

	prefixed := make([]byte, 0, 2+len(query))
	prefixed = binary.BigEndian.AppendUint16(prefixed, uint16(len(query)))
	prefixed = append(prefixed, query...)

	if _, err := conn.Write(prefixed); err != nil {
		return nil, err
	}

	var length [2]byte

	if _, err := io.ReadFull(conn, length[:]); err != nil {
		return nil, err
	}

	body := make([]byte, binary.BigEndian.Uint16(length[:]))

	if _, err := io.ReadFull(conn, body); err != nil {
		return nil, err
	}

	return body, nil
}

// bindDeadline pins conn to the context deadline (or a short default when
// the context carries none), so a silent resolver cannot hang the oracle.
func bindDeadline(ctx context.Context, conn net.Conn) error {
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(existsTimeout)
	}

	return conn.SetDeadline(deadline)
}

// transientRcodeErr wraps a non-authoritative DNS response code as a
// temporary DNSError so the scan's retry/timeout taxonomy treats it like
// any other resolver-side failure.
func transientRcodeErr(host, server string, rcode int) error {
	return &net.DNSError{
		Err:         fmt.Sprintf("resolver returned rcode %d for existence query", rcode),
		Name:        host,
		Server:      server,
		IsTemporary: true,
	}
}
