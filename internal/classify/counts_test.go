package classify

import "testing"

func TestCountErrorKindsBreakdown(t *testing.T) {
	t.Parallel()

	counts := Count([]Result{
		{Domain: "a.example", Err: "not found", ErrKind: ErrorNXDomain},
		{Domain: "b.example", Err: "not found", ErrKind: ErrorNXDomain},
		{Domain: "c.example", Err: "timeout", ErrKind: ErrorTimeout},
		{Domain: "d.example", Err: "legacy error"}, // no kind -> Other
		{Domain: "e.example"},
	})

	if counts.Errors != 4 {
		t.Errorf("Errors = %d, want 4", counts.Errors)
	}

	if counts.ErrorKinds.NXDomain != 2 || counts.ErrorKinds.Timeout != 1 || counts.ErrorKinds.Other != 1 {
		t.Errorf("ErrorKinds = %+v, want nxdomain=2 timeout=1 other=1", counts.ErrorKinds)
	}

	if counts.ErrorKinds.ServFail != 0 || counts.ErrorKinds.Canceled != 0 {
		t.Errorf("ErrorKinds = %+v, want servfail=0 canceled=0", counts.ErrorKinds)
	}
}
