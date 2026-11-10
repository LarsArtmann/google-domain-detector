package scanstore

import (
	"context"
	"fmt"
	"slices"

	"github.com/larsartmann/go-cqrs-lite/metaengine/v4"
	"github.com/larsartmann/go-cqrs-lite/system/v4"
)

// LatestObservation reads one domain's read-model row: its most recent
// observation plus how many observations the store holds overall. found is
// false for domains the store has never seen. The caller passes the domain
// exactly as scans normalize it (domain.Parse → String).
func LatestObservation(ctx context.Context, sys *system.System, domainName string) (ObservationView, bool, error) {
	reader := metaengine.NewReader[ObservationView](sys.MetaEngine(), CollectionObservations)

	view, found, err := reader.Get(ctx, domainName)
	if err != nil {
		return ObservationView{}, false, fmt.Errorf("read observation view for %s: %w", domainName, err)
	}

	return view, found, nil
}

// ListRuns reads every run read-model row, newest first. Runs whose
// projections predate the started-at stamp (legacy stores) sort last.
func ListRuns(ctx context.Context, sys *system.System) ([]RunView, error) {
	reader := metaengine.NewReader[RunView](sys.MetaEngine(), CollectionRuns)

	views, err := reader.Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("scan run views: %w", err)
	}

	slices.SortStableFunc(views, func(a, b RunView) int {
		return b.StartedAt.Compare(a.StartedAt)
	})

	return views, nil
}
