package classify

// Ruleset names the frozen detection-rule set that produced a piece of
// evidence. Detection rules live in this package; a census wave freezes the
// ruleset it runs under, and cross-run deltas are only meaningful between
// observations stamped with the same ruleset. Any change to classification
// behavior (new/changed evidence kinds, weights, verdict logic) MUST bump
// CurrentRuleset in the same commit — a silent mid-census change would
// poison every downstream delta (census plan guards 1+2).
type Ruleset string

// RulesetV1 is the initial frozen ruleset: MX/SPF/DKIM/CNAME/NS/IP-range
// detection exactly as shipped for the 2026-11-09 Tranco .de/.ch/.uk run.
const RulesetV1 Ruleset = "v1-2026-11-10"

// CurrentRuleset is stamped into every observation the scanner records.
const CurrentRuleset = RulesetV1
