package scanstore

import (
	"time"

	"github.com/larsartmann/go-cqrs-lite/metaengine/projectionadapter/v4"
	"github.com/larsartmann/go-cqrs-lite/metaengine/v4"
	"github.com/larsartmann/go-cqrs-lite/record/v4"

	"github.com/larsartmann/google-domain-detector/internal/classify"
)

// Read-model collections (metaengine Map queries).
const (
	CollectionObservations = "observation_views"
	CollectionRuns         = "run_views"
)

// Volume hints for the cost-based planner: observations scale with every
// domain ever scanned (whole-TLD millions), runs with every invocation.
const (
	volumeObservations = 1_000_000
	volumeRuns         = 100
)

// ObservationView is the read model for one domain: its latest observation.
type ObservationView struct {
	Domain       string              `json:"domain"`
	LastRunID    string              `json:"last_run_id"`
	Evidence     []classify.Evidence `json:"evidence"`
	Err          string              `json:"error,omitempty"`
	ErrKind      classify.ErrorKind  `json:"error_kind,omitempty"`
	ObservedAt   time.Time           `json:"observed_at"` //cqrs-lint:ignore(C013) always UTC: built from event.Instant payloads, so no timezone exists to lose in storage
	Observations int                 `json:"observations"`
}

// RunView is the read model for one run stream.
type RunView struct {
	RunID       string          `json:"run_id"`
	Started     bool            `json:"started"`
	Completed   bool            `json:"completed"`
	StartedAt   time.Time       `json:"started_at"` //cqrs-lint:ignore(C013) always UTC: built from event.Instant payloads, so no timezone exists to lose in storage
	Wave        string          `json:"wave,omitempty"`
	ResumedFrom string          `json:"resumed_from,omitempty"`
	Counts      classify.Counts `json:"counts"`
}

func projections() (
	metaengine.QueryDecl[struct{}, ObservationView],
	metaengine.QueryDecl[struct{}, RunView],
	*projectionadapter.TypeDecoder,
) {
	observations := metaengine.Query[struct{}, ObservationView](CollectionObservations,
		metaengine.Volume(volumeObservations),
		metaengine.OnRecordTyped(
			string(evtDomainObserved),
			projectionadapter.EventWithID[DomainObservedPayload]{},
			func(_ record.Record, e projectionadapter.EventWithID[DomainObservedPayload]) (string, ObservationView) {
				return e.ID, observationViewFrom(e.Payload, 1)
			},
		),
		metaengine.OnRecordTyped(
			string(evtDomainObserved),
			projectionadapter.EventWithID[DomainObservedPayload]{},
			func(_ record.Record, e projectionadapter.EventWithID[DomainObservedPayload], prev ObservationView) ObservationView {
				return observationViewFrom(e.Payload, prev.Observations+1)
			},
		),
	)

	runs := metaengine.Query[struct{}, RunView](CollectionRuns,
		metaengine.Volume(volumeRuns),
		metaengine.OnRecordTyped(
			string(evtScanStarted),
			projectionadapter.EventWithID[ScanStartedPayload]{},
			func(_ record.Record, e projectionadapter.EventWithID[ScanStartedPayload]) (string, RunView) {
				return e.ID, RunView{
					RunID:       e.ID,
					Started:     true,
					StartedAt:   e.Payload.StartedAt.Time(),
					Wave:        e.Payload.Wave,
					ResumedFrom: e.Payload.ResumedFrom,
				}
			},
		),
		metaengine.OnRecordTyped(
			string(evtScanCompleted),
			projectionadapter.EventWithID[ScanCompletedPayload]{},
			func(_ record.Record, e projectionadapter.EventWithID[ScanCompletedPayload], prev RunView) RunView {
				prev.Completed = true
				prev.Counts = e.Payload.Counts

				return prev
			},
		),
	)

	decoder := projectionadapter.NewTypeDecoder(
		projectionadapter.Register(evtScanStarted, ScanStartedPayload{}),
		projectionadapter.Register(evtDomainObserved, DomainObservedPayload{}),
		projectionadapter.Register(evtScanCompleted, ScanCompletedPayload{}),
	)

	return observations, runs, decoder
}

func observationViewFrom(p DomainObservedPayload, observations int) ObservationView {
	return ObservationView{
		Domain:       p.Domain,
		LastRunID:    p.RunID,
		Evidence:     p.Evidence,
		Err:          p.Err,
		ErrKind:      p.ErrKind,
		ObservedAt:   p.ObservedAt.Time(),
		Observations: observations,
	}
}
