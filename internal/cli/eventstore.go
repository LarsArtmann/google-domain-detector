package cli

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/larsartmann/google-domain-detector/internal/classify"
	"github.com/larsartmann/google-domain-detector/internal/scan"
	"github.com/larsartmann/google-domain-detector/internal/scanstore"
)

// attachEventStore, when --event-store is set, opens the event-sourced scan
// store, starts a run, and hooks observation recording into opts. It returns
// the closer that drains and completes the run; without an event store the
// closer is a no-op.
func attachEventStore(
	ctx context.Context,
	f ScanFlags,
	resumedFrom string,
	inputDomain, skipped int,
	stderr io.Writer,
	opts *scan.Options,
) (func(context.Context, time.Duration, classify.Counts) error, error) {
	if f.EventStore == "" {
		return func(context.Context, time.Duration, classify.Counts) error { return nil }, nil
	}

	sys, err := scanstore.Open(ctx, f.EventStore)
	if err != nil {
		return nil, fmt.Errorf("open event store %s: %w", f.EventStore, err)
	}

	runID := scanstore.NewRunID()

	recorder := scanstore.NewRecorder(sys)
	if err := recorder.StartScan(ctx, runID, resumedFrom, f.Wave, inputDomain, skipped, opts.Workers); err != nil {
		if closeErr := sys.GracefulClose(context.WithoutCancel(ctx)); closeErr != nil {
			_, _ = fmt.Fprintf(stderr, "warning: close event store after failed start: %v\n", closeErr)
		}

		return nil, fmt.Errorf("start run in event store: %w", err)
	}

	//nolint:contextcheck // record() detaches on purpose: a canceled scan still journals (resume checkpoint)
	sink := scanstore.NewSink(
		recorder,
		runID,
		sinkDepth,
	)
	opts.OnResult = sink.OnResult

	return func(closeCtx context.Context, duration time.Duration, counts classify.Counts) error {
		if err := sink.Close(closeCtx, duration, counts); err != nil {
			_, _ = fmt.Fprintf(stderr, "warning: event store recording incomplete: %v\n", err)
		}

		if err := sys.GracefulClose(context.WithoutCancel(closeCtx)); err != nil {
			_, _ = fmt.Fprintf(stderr, "warning: close event store: %v\n", err)
		}

		_, _ = fmt.Fprintf(stderr, "run %s recorded to %s (replay: %s replay --event-store %s --run %s)\n",
			runID.Get(), f.EventStore, Name, f.EventStore, runID.Get())

		return nil // recording problems are reported, never fatal to the scan report
	}, nil
}

// sinkDepth bounds the buffer between the DNS pool and the event store.
const sinkDepth = 64
