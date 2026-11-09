package classify

// Counts tallies verdicts across the results of one scan run.
type Counts struct {
	Scanned          int `json:"scanned"`
	WorkspaceCertain int `json:"workspace_certain"`
	WorkspaceLikely  int `json:"workspace_likely"`
	GCPCertain       int `json:"gcp_certain"`
	GenericGoogle    int `json:"generic_google"`
	Errors           int `json:"errors"`
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
