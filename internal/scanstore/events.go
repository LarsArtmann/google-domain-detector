package scanstore

import (
	"time"

	"github.com/larsartmann/go-cqrs-lite/command/v4" //cqrs-lint:ignore(F032) deliberate: single-user CLI, event contract documented in README + events.go; catalog wiring lands when the journal grows external consumers
	"github.com/larsartmann/go-cqrs-lite/event/v4"

	"github.com/larsartmann/google-domain-detector/internal/classify"
)

// Stream types: one stream per scanned domain, one per scan run.
const (
	StreamTypeDomain  = "Domain"
	StreamTypeScanRun = "ScanRun"
)

// Event types persisted to the journal.
const (
	evtScanStarted    = event.Type("scan.started")
	evtDomainObserved = event.Type("scan.domain_observed")
	evtScanCompleted  = event.Type("scan.completed")
)

// Command types, exported so external tooling and tests can dispatch the
// same commands the Recorder uses.
const (
	CmdStartScan    = command.Type("scan.start")
	CmdObserve      = command.Type("scan.observe")
	CmdCompleteScan = command.Type("scan.complete")
)

// ScanStartedPayload opens a run.
type ScanStartedPayload struct {
	RunID       string        `json:"run_id"`
	StartedAt   event.Instant `json:"started_at"`
	InputDomain int           `json:"input_domains"`
	Skipped     int           `json:"skipped"`
	Workers     int           `json:"workers"`
}

// DomainObservedPayload records everything one scan learned about one
// domain. Verdicts are deliberately absent: they are derived from evidence
// (classify.Result methods), and storing derived state next to the facts it
// comes from would be a split brain.
type DomainObservedPayload struct {
	Domain     string              `json:"domain"`
	RunID      string              `json:"run_id"`
	Evidence   []classify.Evidence `json:"evidence"`
	Err        string              `json:"error,omitempty"`
	ObservedAt event.Instant       `json:"observed_at"`
}

// ScanCompletedPayload closes a run with the scanner's own tally.
type ScanCompletedPayload struct {
	RunID       string          `json:"run_id"`
	CompletedAt event.Instant   `json:"completed_at"`
	Duration    time.Duration   `json:"duration"`
	Counts      classify.Counts `json:"counts"`
}

// StartScanCmd opens a run on its own stream.
type StartScanCmd struct {
	*command.BasicCommand

	InputDomain int
	Skipped     int
	Workers     int
}

// RecordObservationCmd appends one observation to the domain's stream.
type RecordObservationCmd struct {
	*command.BasicCommand

	RunID  string
	Result classify.Result
}

// CompleteScanCmd closes the run stream.
type CompleteScanCmd struct {
	*command.BasicCommand

	Duration time.Duration
	Counts   classify.Counts
}

// ScanRunState is the write-side state of one run stream.
type ScanRunState struct {
	Started   bool
	Completed bool
	Counts    classify.Counts
}

// DomainState is the write-side state of one domain stream.
type DomainState struct {
	Observations int
}

func applyDomain(s DomainState, evt event.Event) (DomainState, error) {
	switch evt.Type() {
	case evtDomainObserved:
		s.Observations++
	}

	return s, nil
}

func applyScanRun(s ScanRunState, evt event.Event) (ScanRunState, error) {
	switch evt.Type() {
	case evtScanStarted:
		s.Started = true
	case evtScanCompleted:
		p, err := event.DecodePayloadAuto[ScanCompletedPayload](evt)
		if err != nil {
			return s, err
		}

		s.Completed = true
		s.Counts = p.Counts
	}

	return s, nil
}
