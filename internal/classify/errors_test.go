package classify

import (
	"context"
	"errors"
	"net"
	"testing"
)

func TestErrorKindOf(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
		want ErrorKind
	}{
		{"nil", nil, ErrorNone},
		{"nxdomain", &net.DNSError{Err: "no such host", Name: "gone.example", IsNotFound: true}, ErrorNXDomain},
		{"dns timeout", &net.DNSError{Err: "i/o timeout", Name: "slow.example", IsTimeout: true}, ErrorTimeout},
		{"context deadline", context.DeadlineExceeded, ErrorTimeout},
		{"context canceled", context.Canceled, ErrorCanceled},
		{"servfail", &net.DNSError{Err: "server misbehaving", Name: "sick.example", IsTemporary: true}, ErrorServFail},
		{"plain error", errors.New("connection refused"), ErrorServFail},
	}

	for _, tc := range cases {
		if got := ErrorKindOf(tc.err); got != tc.want {
			t.Errorf("ErrorKindOf(%s) = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestIsTransient(t *testing.T) {
	t.Parallel()

	transient := []error{
		&net.DNSError{Err: "i/o timeout", IsTimeout: true},
		&net.DNSError{Err: "server misbehaving", IsTemporary: true},
		errors.New("connection refused"),
	}
	for _, err := range transient {
		if !IsTransient(err) {
			t.Errorf("IsTransient(%v) = false, want true", err)
		}
	}

	permanent := []error{
		nil,
		&net.DNSError{Err: "no such host", IsNotFound: true},
		context.Canceled,
	}
	for _, err := range permanent {
		if IsTransient(err) {
			t.Errorf("IsTransient(%v) = true, want false", err)
		}
	}
}
