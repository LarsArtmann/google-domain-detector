package cli

import (
	"bytes"
	"context"
	"errors"
	"testing"

	v5 "github.com/larsartmann/cmdguard/v5/pkg/cmdguard/v5"
)

const fixtureFeed = `{
  "prefixes": [
    {"ipv4Prefix": "34.0.0.0/8", "service": "Google Cloud"},
    {"ipv6Prefix": "2600:1900::/28", "service": "Google Cloud"}
  ],
  "creationTimestamp": "2026-11-09T00:00:00Z"
}`

func TestRunRejectsBadFlagsAsUsageErrors(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer

	err := Run(context.Background(), []string{"scan", "--workers", "0"}, &stdout, &stderr, "test", "abc123")

	exitErr, ok := errors.AsType[*v5.ExitError](err)
	if !ok || exitErr.ExitCode() != exitCodeUsage {
		t.Fatalf("invalid --workers must be a usage error (exit 2), got: %v", err)
	}
}

func TestRunRejectsUnknownCommand(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer

	if err := Run(
		context.Background(),
		[]string{"definitely-not-a-command"},
		&stdout,
		&stderr,
		"test",
		"",
	); err == nil {
		t.Fatal("unknown command must fail")
	}
}
