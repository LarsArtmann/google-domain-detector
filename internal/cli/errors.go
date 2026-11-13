package cli

import "errors"

// Shared failure cores for the journal-backed commands. Callers wrap them
// with contextual detail (paths, domains); match with errors.Is, and
// usageError-classified ones exit 2.
var (
	errEventStoreRequired = errors.New("--event-store is required (or set GDD_EVENT_STORE)")
	errNoRuns             = errors.New("no scan runs found")
	errNoObservations     = errors.New("no observations for domain (scan with --event-store first)")
	errResumeNeedsStore   = errors.New("--resume requires --event-store (the journal IS the checkpoint)")
	errWaveNeedsStore     = errors.New("--wave requires --event-store (waves live in the journal)")

	errAuditExcludedCombination = errors.New("--audit-excluded audits the whole journal; drop --run/--since/--only")
	errAuditExcludedEmpty       = errors.New("--audit-excluded needs at least one run ID")
)
