package classify

import "testing"

func TestCountErrorKindsBreakdown(t *testing.T) {
	t.Parallel()

	c := Count([]Result{
		{Domain: "a.example", Err: "not found", ErrKind: ErrorNXDomain},
		{Domain: "b.example", Err: "not found", ErrKind: ErrorNXDomain},
		{Domain: "c.example", Err: "timeout", ErrKind: ErrorTimeout},
		{Domain: "d.example", Err: "legacy error"}, // no kind -> Other
		{Domain: "e.example"},
	})

	if c.Errors != 4 {
		t.Errorf("Errors = %d, want 4", c.Errors)
	}

	if c.ErrorKinds.NXDomain != 2 || c.ErrorKinds.Timeout != 1 || c.ErrorKinds.Other != 1 {
		t.Errorf("ErrorKinds = %+v, want nxdomain=2 timeout=1 other=1", c.ErrorKinds)
	}

	if c.ErrorKinds.ServFail != 0 || c.ErrorKinds.Canceled != 0 {
		t.Errorf("ErrorKinds = %+v, want servfail=0 canceled=0", c.ErrorKinds)
	}
}
