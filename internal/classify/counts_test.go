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

// TestTallySkipsUnscannedPlaceholders pins the interrupted-run accounting
// fix: a placeholder for an input domain the canceled run never reached is
// not a scan, so neither Scanned nor the error kinds may count it.
func TestTallySkipsUnscannedPlaceholders(t *testing.T) {
	t.Parallel()

	var counts Counts
	counts.Tally(Result{Domain: "real.example"})
	counts.Tally(Result{Domain: "hung.example", Err: "context canceled", ErrKind: ErrorCanceled})
	counts.Tally(Result{Domain: "never.example", Err: "context canceled", ErrKind: ErrorCanceled, Unscanned: true})

	if counts.Scanned != 2 {
		t.Errorf("Scanned = %d, want 2 (the placeholder must not count)", counts.Scanned)
	}

	if counts.Errors != 1 || counts.ErrorKinds.Canceled != 1 {
		t.Errorf("error accounting = errors %d canceled %d, want 1/1 (in-flight only)", counts.Errors, counts.ErrorKinds.Canceled)
	}
}
