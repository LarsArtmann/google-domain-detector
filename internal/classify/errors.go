package classify

import (
	"context"
	"errors"
	"net"
)

// ErrorKind groups lookup failures by cause so an error tail can be
// explained with data (NXDOMAIN tail vs resolver timeouts vs server
// failures) instead of interpretation.
type ErrorKind string

const (
	// ErrorNone marks a result without a lookup error.
	ErrorNone ErrorKind = ""
	// ErrorNXDomain: the domain does not exist (authoritative NXDOMAIN).
	ErrorNXDomain ErrorKind = "nxdomain"
	// ErrorTimeout: the resolver gave up (timeouts, deadlines).
	ErrorTimeout ErrorKind = "timeout"
	// ErrorServFail: the lookup failed without a verdict (SERVFAIL, refused,
	// unreadable responses, unknown error types).
	ErrorServFail ErrorKind = "servfail"
	// ErrorCanceled: the run was interrupted before the domain finished.
	ErrorCanceled ErrorKind = "canceled"
)

// ErrorKindOf maps a lookup error to its kind; nil maps to ErrorNone.
// Unknown non-DNS errors classify as servfail so they stay visible.
func ErrorKindOf(err error) ErrorKind {
	switch {
	case err == nil:
		return ErrorNone
	case errors.Is(err, context.Canceled):
		return ErrorCanceled
	case errors.Is(err, context.DeadlineExceeded):
		return ErrorTimeout
	}

	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		switch {
		case dnsErr.IsNotFound:
			return ErrorNXDomain
		case dnsErr.IsTimeout:
			return ErrorTimeout
		}
	}

	return ErrorServFail
}

// IsTransient reports whether one cheap retry can plausibly clear the
// error: resolver-side trouble yes, authoritative answers never.
func IsTransient(err error) bool {
	switch ErrorKindOf(err) {
	case ErrorTimeout, ErrorServFail:
		return true
	default:
		return false
	}
}
