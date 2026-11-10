package cli

import (
	"context"
	"io"
	"time"

	"github.com/larsartmann/go-output/nom"

	"github.com/larsartmann/google-domain-detector/internal/classify"
)

// nomProgress drives the live inline progress tree during interactive
// scans: one activity per domain, started when its lookup begins and
// completed or failed when its result lands. The caller only builds one
// for a TTY stdout that is not a jsonl stream — pipes get byte-identical
// output to before, and jsonl owns stdout exclusively.
type nomProgress struct {
	sub      *nom.NOMSubscriber
	renderer *nom.InlineRenderer
}

// progressMaxActivities caps the rendered activity tree height;
// progressRedrawInterval keeps the frame rate livewire but invisible.
const (
	progressMaxActivities  = 10
	progressRedrawInterval = 250 * time.Millisecond
)

// newNomProgress starts the redraw loop (250ms cadence — fast enough to
// feel live, slow enough to stay invisible at 8 workers).
func newNomProgress(ctx context.Context, w io.Writer) *nomProgress {
	sub := nom.NewNOMSubscriber()
	renderer := nom.NewInlineRenderer(sub, w, progressMaxActivities)
	renderer.SetAppName(Name)

	_ = sub.OnEvent(ctx, nom.WorkflowStarted{ID: nom.NewWorkflowID("scan"), Name: nom.NewWorkflowName("Scan")})
	renderer.Start(ctx, progressRedrawInterval)

	return &nomProgress{sub: sub, renderer: renderer}
}

// resultError adapts a classify.Result's error string to the error
// interface for the progress UI; the string IS the payload, so a static
// type beats constructing a dynamic error per event.
type resultError string

func (e resultError) Error() string { return string(e) }

// domainStarted marks a domain's lookup as running.
func (p *nomProgress) domainStarted(domain string) {
	_ = p.sub.OnEvent(context.Background(), nom.ActivityStarted{
		ID:   nom.NewActivityID(domain),
		Name: nom.NewActivityName(domain),
	})
}

// domainDone marks a domain's lookup completed or failed (an error cell is
// a failed activity; verdict absence is not).
func (p *nomProgress) domainDone(r classify.Result) {
	if r.Err != "" {
		_ = p.sub.OnEvent(context.Background(), nom.ActivityFailed{
			ID:   nom.NewActivityID(r.Domain),
			Name: nom.NewActivityName(r.Domain),
			Err:  resultError(r.Err),
		})

		return
	}

	_ = p.sub.OnEvent(context.Background(), nom.ActivityCompleted{
		ID:   nom.NewActivityID(r.Domain),
		Name: nom.NewActivityName(r.Domain),
	})
}

// finish closes the workflow and the redraw loop; the report renders after
// this returns, so the final frame never fights the table for the screen.
// The completion event publishes WithoutCancel so Ctrl-C still tears the
// workflow down cleanly.
func (p *nomProgress) finish(ctx context.Context) {
	_ = p.sub.OnEvent(context.WithoutCancel(ctx), nom.WorkflowCompleted{ID: nom.NewWorkflowID("scan")})
	p.renderer.Finish()
}
