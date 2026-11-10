package classify

import "testing"

// TestCurrentRulesetPinned makes every ruleset bump an explicit, reviewable
// diff: the golden value changes only in a commit that also changes
// detection behavior (see Ruleset's doc comment).
func TestCurrentRulesetPinned(t *testing.T) {
	t.Parallel()

	const want Ruleset = "v1-2026-11-10"
	if CurrentRuleset != want {
		t.Fatalf(
			"CurrentRuleset = %q, want %q (did detection rules change? bump the ruleset AND update this pin)",
			CurrentRuleset,
			want,
		)
	}

	if CurrentRuleset == "" {
		t.Fatal("CurrentRuleset must never be empty: every observation carries it as provenance")
	}
}
