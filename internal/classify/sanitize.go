package classify

import "strings"

// SanitizeEvidence coerces DNS-derived strings to valid UTF-8: record
// contents are attacker-controlled bytes, and downstream consumers
// reject invalid UTF-8 (the JSON v2 encoder errors, CBOR text strings must
// be valid UTF-8). SanitizeEvidence returns a copy of evidences with every
// DNS-derived string coerced to valid UTF-8, invalid bytes replaced by
// U+FFFD. Call it wherever evidence leaves the DNS boundary.
func SanitizeEvidence(evidences []Evidence) []Evidence {
	if len(evidences) == 0 {
		return evidences
	}

	out := make([]Evidence, 0, len(evidences))
	for _, ev := range evidences {
		ev.Host = strings.ToValidUTF8(ev.Host, "\ufffd")
		ev.Record = strings.ToValidUTF8(ev.Record, "\ufffd")
		out = append(out, ev)
	}

	return out
}
