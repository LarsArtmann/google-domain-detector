package cli

import (
	"bytes"
	"context"
	"io"
	"path/filepath"
	"testing"

	"github.com/larsartmann/google-domain-detector/internal/classify"
	"github.com/larsartmann/google-domain-detector/internal/scanstore"
)

func TestWarnRulesetMismatch(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	cases := []struct {
		name    string
		from    *scanstore.RunSummary
		to      *scanstore.RunSummary
		wantMsg bool
	}{
		{"nil sides stay silent", nil, &scanstore.RunSummary{Ruleset: "r1"}, false},
		{"nil target stays silent", &scanstore.RunSummary{Ruleset: "r1"}, nil, false},
		{
			"same ruleset stays silent",
			&scanstore.RunSummary{Ruleset: "r1"},
			&scanstore.RunSummary{Ruleset: "r1"},
			false,
		},
		{"empty rulesets stay silent", &scanstore.RunSummary{}, &scanstore.RunSummary{Ruleset: "r2"}, false},
		{"mismatch warns", &scanstore.RunSummary{Ruleset: "r1"}, &scanstore.RunSummary{Ruleset: "r2"}, true},
	}

	for _, tc := range cases {
		buf.Reset()
		warnRulesetMismatch(&buf, tc.from, tc.to)

		if got := buf.Len() > 0; got != tc.wantMsg {
			t.Errorf("%s: warned = %v, want %v (output %q)", tc.name, got, tc.wantMsg, buf.String())
		}
	}
}

func TestReplayHelpers(t *testing.T) {
	t.Parallel()

	if got := provenanceSuffix("", ""); got != "" {
		t.Errorf("empty provenance = %q, want empty", got)
	}

	if got := provenanceSuffix("w1", ""); got != " [wave w1]" {
		t.Errorf("wave-only provenance = %q", got)
	}

	if got := provenanceSuffix("w1", classify.Ruleset("v1")); got != " [wave w1, ruleset v1]" {
		t.Errorf("full provenance = %q", got)
	}

	if got := shortRunID("short"); got != "short" {
		t.Errorf("short id must pass through, got %q", got)
	}

	if got := shortRunID("01M794ZEB4HYR5PM25XC000001"); got != "01M794ZEB4HY…" {
		t.Errorf("long id must truncate, got %q", got)
	}

	if runSummary(scanstore.Replay{RunID: "absent"}) != nil {
		t.Error("runSummary must return nil for an unknown run id")
	}
}

// TestRunReplayAndShowFlagBranches covers the programmatic flag-parse
// branches the CLI enum binding normally front-runs: direct calls with
// unparseable Only/Format/Color values must error before any store I/O.
// TestRunReplayAndShowFlagBranches covers the programmatic flag-parse
// branches the CLI enum binding normally front-runs: direct calls with
// unparseable Only/Format/Color values must error before any store I/O.
func TestRunReplayAndShowFlagBranches(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := filepath.Join(t.TempDir(), "scans.db")

	for name, call := range map[string]func() error{
		"replay bad --only": func() error {
			return runReplay(ctx, ReplayFlags{EventStore: store, Only: "bogus"}, io.Discard, io.Discard)
		},
		"replay bad --format": func() error {
			return runReplay(ctx, ReplayFlags{EventStore: store, Format: "nope"}, io.Discard, io.Discard)
		},
		"replay bad --color": func() error {
			return runReplay(ctx, ReplayFlags{EventStore: store, Color: "nope"}, io.Discard, io.Discard)
		},
		"show bad --format": func() error {
			return runShow(ctx, ShowFlags{EventStore: store, Format: "nope"}, []string{"acme.com"}, io.Discard, io.Discard)
		},
		"show bad --color": func() error {
			return runShow(ctx, ShowFlags{EventStore: store, Color: "nope"}, []string{"acme.com"}, io.Discard, io.Discard)
		},
	} {
		if err := call(); err == nil {
			t.Errorf("%s must error", name)
		}
	}
}
