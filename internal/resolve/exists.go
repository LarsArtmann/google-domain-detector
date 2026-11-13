package resolve

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
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

	dnsFlagZero byte = 0x00 // header byte 3: opcode 0, rcode 0, no RA/AD/CD

	dnsQTypeA   uint16 = 1 // A
	dnsQClassIN uint16 = 1 // IN

	// dnsRcodeNameError is the NXDOMAIN response code; rcode 0 is NOERROR.
	dnsRcodeNameError = 3
	dnsRcodeNoError   = 0

	// DNS wire sizes: fixed header, question tail (qtype + qclass), root
	// label terminator, and the RFC 1035 name/label limits.
	dnsHeaderLen     = 12
	dnsQuestionLen   = 4
	dnsLenPrefixLen  = 2
	dnsMaxLabelLen   = 63
	dnsMaxNameLen    = 253
	dnsMaxUDPMsgLen  = 512
	dnsRcodeMask     = 0x0F
	dnsWireOverhead  = 2 // one label-length byte plus the root byte
	dnsMaxMsgIDBytes = 2

	// existsTimeout bounds one oracle round trip when the caller's context
	// carries no deadline.
	existsTimeout = 5 * time.Second
)

// Query-shape and response-shape failures are static sentinels: callers
// wrap them with the host name for context.
var (
	errEmptyQueryName = errors.New("empty query name")
	errEmptyLabel     = errors.New("empty label")
	errLabelTooLong   = errors.New("label longer than 63 bytes")
	errNameTooLong    = errors.New("query name longer than 253 bytes")
	errShortResponse  = errors.New("dns response shorter than header")
	errIDMismatch     = errors.New("dns response id mismatch")
	errNoQRBit        = errors.New("dns response lacks QR bit")
)

// buildQuery packs an A-record query for host with a random message ID.
func buildQuery(host string) ([]byte, uint16, error) {
	var id [dnsMaxMsgIDBytes]byte

	if _, err := rand.Read(id[:]); err != nil {
		return nil, 0, fmt.Errorf("existence query id: %w", err)
	}

	qname, err := encodeQName(host)
	if err != nil {
		return nil, 0, err
	}

	msg := make([]byte, 0, dnsHeaderLen+len(qname)+dnsQuestionLen)
	msg = append(msg, id[0], id[1])
	msg = append(msg, dnsFlagRD, dnsFlagZero)   // RD set; opcode/rcode zero
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
		return nil, fmt.Errorf("%w", errEmptyQueryName)
	}

	qname := make([]byte, 0, len(host)+dnsWireOverhead)

	for label := range strings.SplitSeq(host, ".") {
		if len(label) == 0 {
			return nil, fmt.Errorf("%w in %q", errEmptyLabel, host)
		}

		if len(label) > dnsMaxLabelLen {
			return nil, fmt.Errorf("%w in %q", errLabelTooLong, host)
		}

		qname = append(qname, byte(len(label)))
		qname = append(qname, label...)
	}

	if len(qname) > dnsMaxNameLen {
		return nil, fmt.Errorf("%w: %q", errNameTooLong, host)
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
func parseRcode(msg []byte, wantID uint16) (int, bool, error) {
	if len(msg) < dnsHeaderLen {
		return 0, false, fmt.Errorf("%w: %d bytes", errShortResponse, len(msg))
	}

	if id := binary.BigEndian.Uint16(msg[0:2]); id != wantID {
		return 0, false, fmt.Errorf("%w: got %d, want %d", errIDMismatch, id, wantID)
	}

	if msg[2]&dnsFlagQR == 0 {
		return 0, false, errNoQRBit
	}

	return int(msg[3] & dnsRcodeMask), msg[2]&dnsFlagTC != 0, nil
}

// exchangeUDP sends query to addr over UDP and returns the response.
func exchangeUDP(ctx context.Context, addr string, query []byte) ([]byte, error) {
	var d net.Dialer

	conn, err := d.DialContext(ctx, "udp", addr)
	if err != nil {
		return nil, fmt.Errorf("udp dial %s: %w", addr, err)
	}

	defer func() { _ = conn.Close() }()

	if err := bindDeadline(ctx, conn); err != nil {
		return nil, fmt.Errorf("udp deadline %s: %w", addr, err)
	}

	if _, err := conn.Write(query); err != nil {
		return nil, fmt.Errorf("udp write %s: %w", addr, err)
	}

	var buf [dnsMaxUDPMsgLen]byte

	n, readErr := conn.Read(buf[:])
	if readErr != nil {
		return nil, fmt.Errorf("udp read %s: %w", addr, readErr)
	}

	return append([]byte{}, buf[:n]...), nil
}

// exchangeTCP sends query to addr length-prefixed over TCP and returns
// the response body.
func exchangeTCP(ctx context.Context, addr string, query []byte) ([]byte, error) {
	var d net.Dialer

	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("tcp dial %s: %w", addr, err)
	}

	defer func() { _ = conn.Close() }()

	if err := bindDeadline(ctx, conn); err != nil {
		return nil, fmt.Errorf("tcp deadline %s: %w", addr, err)
	}

	prefixed := make([]byte, 0, dnsLenPrefixLen+len(query))
	prefixed = binary.BigEndian.AppendUint16(prefixed, uint16(len(query)))
	prefixed = append(prefixed, query...)

	if _, err := conn.Write(prefixed); err != nil {
		return nil, fmt.Errorf("tcp write %s: %w", addr, err)
	}

	var length [dnsLenPrefixLen]byte

	if _, err := io.ReadFull(conn, length[:]); err != nil {
		return nil, fmt.Errorf("tcp length read %s: %w", addr, err)
	}

	body := make([]byte, binary.BigEndian.Uint16(length[:])) //nolint:makezero // exact-size wire buffer, filled by ReadFull

	if _, err := io.ReadFull(conn, body); err != nil {
		return nil, fmt.Errorf("tcp body read %s: %w", addr, err)
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

// wrapExistenceNetErr re-labels transport failures so the scan's
// timeout/servfail taxonomy recognizes them: net timeouts become
// IsTimeout DNSError values, other transport trouble temporary ones.
// Already-classified DNSError values pass through unchanged.
func wrapExistenceNetErr(err error, host, server string) error {
	if _, ok := errors.AsType[*net.DNSError](err); ok {
		return err
	}

	out := &net.DNSError{
		Err:         err.Error(),
		Name:        host,
		Server:      server,
		IsTemporary: true,
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		out.IsTimeout = true
	}

	return out
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
