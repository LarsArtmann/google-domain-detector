package cli

import (
	"strings"
	"testing"
)

// TestReplayAuditDataFormatsStayPure pins the audit table's data purity:
// the poison-count footer is a human decoration and must not leak into
// data formats (it did before the footer became table/markdown-only).
func TestReplayAuditDataFormatsStayPure(t *testing.T) {
	t.Parallel()

	store, _, secondRun := auditTestStore(t)

	stdout, _, err := runRes(t, workspaceDomain(), "",
		"replay", "--event-store", store, "--audit-excluded", secondRun, "--format", "csv")
	if err != nil {
		t.Fatalf("audit csv failed: %v", err)
	}

	for _, decoration := range []string{"poisoned of", "domain(s)", "Supersession Audit"} {
		if strings.Contains(stdout, decoration) {
			t.Errorf("audit csv must stay pure data: found %q in %q", decoration, stdout)
		}
	}

	if !strings.Contains(stdout, "acme.com") {
		t.Errorf("audit csv must still carry the finding row, got: %q", stdout)
	}
}
