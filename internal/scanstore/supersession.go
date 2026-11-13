package scanstore

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/larsartmann/go-cqrs-lite/event/v4"
	"github.com/larsartmann/go-cqrs-lite/system/v4"

	"github.com/larsartmann/google-domain-detector/internal/classify"
)

// ErrNoExcludedRuns reports AuditSupersession called without excluded runs;
// an audit of nothing would silently report clean.
var ErrNoExcludedRuns = errors.New("no excluded runs given")

// SupersessionFinding is one domain whose last journal observation comes
// from an excluded run: the report any replay rebuilds for it currently
// derives from data the operator marked untrusted.
type SupersessionFinding struct {
	Domain  string
	RunID   string
	ErrKind classify.ErrorKind
	Err     string
}

// SupersessionAudit is the supersession-coverage check's result: which
// domains' last-wins verdicts derive from an excluded run, out of every
// domain the journal has ever observed.
type SupersessionAudit struct {
	Excluded     []string
	TotalDomains int
	Findings     []SupersessionFinding
}

// AuditSupersession walks the journal once and reports every domain whose
// most recent observation was recorded by one of the excluded runs. It is
// the mechanical check that a correction pass really covered the poisoned
// supersessions: "replay last-wins supersedes the false verdicts" is a
// per-domain claim, and this is where it is proven instead of assumed.
// Findings keep first-seen domain order, so output diffs cleanly between
// audits. Excluded run IDs that name no journal run fail the audit: a
// typo auditing clean would hide exactly the poison it exists to find.
func AuditSupersession(ctx context.Context, sys *system.System, excluded []string) (SupersessionAudit, error) {
	if len(excluded) == 0 {
		return SupersessionAudit{}, ErrNoExcludedRuns
	}

	lastByDomain, domainOrder, knownRuns, err := walkSupersessionState(ctx, sys)
	if err != nil {
		return SupersessionAudit{}, err
	}

	if len(knownRuns) == 0 {
		return SupersessionAudit{}, ErrNoRuns
	}

	if missing := findMissingRuns(knownRuns, excluded); len(missing) > 0 {
		return SupersessionAudit{}, fmt.Errorf("%w: %s", ErrUnknownRun, strings.Join(missing, ", "))
	}

	return collectSupersessionFindings(lastByDomain, domainOrder, excluded), nil
}

// walkSupersessionState is the single journal pass: each domain's last
// observation, the domains in first-seen order, and every run the journal
// started.
func walkSupersessionState(
	ctx context.Context,
	sys *system.System,
) (map[string]SupersessionFinding, []string, map[string]struct{}, error) {
	lastByDomain := map[string]SupersessionFinding{}
	knownRuns := map[string]struct{}{}

	var domainOrder []string

	err := walkJournal(ctx, sys.EventStore(), func(evt event.Event) error {
		switch evt.Type() {
		case evtScanStarted:
			payload, err := event.DecodePayloadAuto[ScanStartedPayload](evt)
			if err != nil {
				return fmt.Errorf("decode %s: %w", evtScanStarted, err)
			}

			knownRuns[payload.RunID] = struct{}{}
		case evtDomainObserved:
			p, err := decodeObserved(evt)
			if err != nil {
				return fmt.Errorf("decode %s: %w", evtDomainObserved, err)
			}

			if _, seen := lastByDomain[p.Domain]; !seen {
				domainOrder = append(domainOrder, p.Domain)
			}

			lastByDomain[p.Domain] = SupersessionFinding{
				Domain:  p.Domain,
				RunID:   p.RunID,
				ErrKind: p.ErrKind,
				Err:     p.Err,
			}
		}

		return nil
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("read event journal: %w", err)
	}

	return lastByDomain, domainOrder, knownRuns, nil
}

// findMissingRuns lists the excluded IDs that name no journal run, in the
// order they were asked for.
func findMissingRuns(knownRuns map[string]struct{}, excluded []string) []string {
	var missing []string

	for _, runID := range excluded {
		if _, known := knownRuns[runID]; !known {
			missing = append(missing, runID)
		}
	}

	return missing
}

// collectSupersessionFindings narrows the last-wins state to the domains
// whose surviving observation derives from an excluded run.
func collectSupersessionFindings(
	lastByDomain map[string]SupersessionFinding,
	domainOrder []string,
	excluded []string,
) SupersessionAudit {
	excludedSet := make(map[string]struct{}, len(excluded))
	for _, runID := range excluded {
		excludedSet[runID] = struct{}{}
	}

	audit := SupersessionAudit{Excluded: slices.Clone(excluded), TotalDomains: len(domainOrder)}

	for _, domain := range domainOrder {
		finding := lastByDomain[domain]
		if _, poisoned := excludedSet[finding.RunID]; poisoned {
			audit.Findings = append(audit.Findings, finding)
		}
	}

	return audit
}
