package scan

import (
	"strings"

	"github.com/larsartmann/google-domain-detector/internal/classify"
)

// sanitizeResult coerces every DNS-derived string in the result to valid
// UTF-8 (see classify.SanitizeEvidence): a hostile record must neither break
// the JSON render nor the journal payload. Applied at every ScanDomain exit.
func sanitizeResult(result classify.Result) classify.Result {
	result.Domain = strings.ToValidUTF8(result.Domain, "\ufffd")
	result.Err = strings.ToValidUTF8(result.Err, "\ufffd")
	result.Evidences = classify.SanitizeEvidence(result.Evidences)

	return result
}
