package scanstore //cqrs-lint:ignore(F005,F007) F005: schema v1 is the initial version, upcasters arrive with the first v2; F007: commands dispatch in-process, sequentially, single-flight (start/complete handlers are naturally idempotent) — no at-least-once bus delivery exists to dedupe

import (
	"github.com/larsartmann/google-domain-detector/internal/classify"
)

// Delta is the verdict flip of one domain between two runs: the domain,
// its observation in the earlier run, and its observation in the later one.
type Delta struct {
	Domain string
	From   classify.Result
	To     classify.Result
}

// verdictState is the comparable detection state of one observation.
// Errored lookups are NOT a "not detected" state: a domain that answered
// in one run and timed out in the next did not leave Google — its state
// is unknown, so it can never register as a flip.
type verdictState struct {
	known     bool
	workspace classify.Confidence
	gcp       bool
}

func stateOf(r classify.Result) verdictState {
	if r.Err != "" || r.ErrKind != classify.ErrorNone {
		return verdictState{}
	}

	return verdictState{
		known:     true,
		workspace: r.WorkspaceVerdict().Confidence,
		gcp:       r.GCPVerdict().Detected,
	}
}

// FlipDiff compares two run rebuilds and returns every domain whose
// workspace or GCP detection state changed, in the "to" run's order.
// Domains present in only one side are not flips: absent from "to" means
// not rescanned, absent from "from" means newly covered — neither says
// anything about adoption. Confidence moves within detection (likely →
// certain) DO count as flips; they are visible in From/To.
func FlipDiff(from, to []classify.Result) []Delta {
	byDomain := make(map[string]classify.Result, len(from))
	for _, r := range from {
		byDomain[r.Domain] = r
	}

	var flips []Delta

	for _, toResult := range to {
		fromResult, seen := byDomain[toResult.Domain]
		if !seen {
			continue
		}

		fromState, toState := stateOf(fromResult), stateOf(toResult)
		if !fromState.known || !toState.known {
			continue
		}

		if fromState != toState {
			flips = append(flips, Delta{Domain: toResult.Domain, From: fromResult, To: toResult})
		}
	}

	return flips
}

// FlipCounts summarizes a FlipDiff for one-line reporting.
type FlipCounts struct {
	Compared         int
	WorkspaceAdopted int
	WorkspaceLeft    int
	GCPAdopted       int
	GCPLeft          int
}

// CountFlips tallies adoption direction per platform. A single domain can
// contribute to both platforms.
func CountFlips(flips []Delta) FlipCounts {
	var counts FlipCounts

	counts.Compared = len(flips)

	for _, f := range flips {
		fromWorkspace := stateOf(f.From).workspace != classify.ConfidenceNone
		toWorkspace := stateOf(f.To).workspace != classify.ConfidenceNone

		switch {
		case !fromWorkspace && toWorkspace:
			counts.WorkspaceAdopted++
		case fromWorkspace && !toWorkspace:
			counts.WorkspaceLeft++
		}

		fromGCP := stateOf(f.From).gcp
		toGCP := stateOf(f.To).gcp

		switch {
		case !fromGCP && toGCP:
			counts.GCPAdopted++
		case fromGCP && !toGCP:
			counts.GCPLeft++
		}
	}

	return counts
}
