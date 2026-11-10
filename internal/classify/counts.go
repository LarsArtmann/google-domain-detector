package classify

// Counts tallies verdicts across the results of one scan run.
type Counts struct {
	Scanned          int `json:"scanned"`
	WorkspaceCertain int `json:"workspace_certain"`
	WorkspaceLikely  int `json:"workspace_likely"`
	GCPCertain       int `json:"gcp_certain"`
	GenericGoogle    int `json:"generic_google"`
	Errors           int `json:"errors"`
	// ErrorKinds explains the Errors total: NXDOMAIN tail vs resolver
	// timeouts vs server failures.
	ErrorKinds ErrorKinds `json:"error_kinds"`
}

// ErrorKinds breaks the error count down by cause.
type ErrorKinds struct {
	NXDomain int `json:"nxdomain"`
	Timeout  int `json:"timeout"`
	ServFail int `json:"servfail"`
	Canceled int `json:"canceled"`
	Other    int `json:"other"`
}

// Tally folds one result into the counts.
func (c *Counts) Tally(r Result) {
	c.Scanned++

	if w := r.WorkspaceVerdict(); w.Detected {
		if w.Confidence == ConfidenceCertain {
			c.WorkspaceCertain++
		} else {
			c.WorkspaceLikely++
		}
	}

	if r.GCPVerdict().Detected {
		c.GCPCertain++
	}

	if r.GenericGoogle() {
		c.GenericGoogle++
	}

	if r.Err != "" {
		c.Errors++
		c.ErrorKinds.tally(r.ErrKind)
	}
}

func (k *ErrorKinds) tally(kind ErrorKind) {
	switch kind {
	case ErrorNXDomain:
		k.NXDomain++
	case ErrorTimeout:
		k.Timeout++
	case ErrorServFail:
		k.ServFail++
	case ErrorCanceled:
		k.Canceled++
	default:
		k.Other++
	}
}

// Count tallies verdicts over a whole result set.
func Count(results []Result) Counts {
	var c Counts

	for _, r := range results {
		c.Tally(r)
	}

	return c
}
