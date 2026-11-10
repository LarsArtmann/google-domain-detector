package cli

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// The doctor tests swap the package probe vars, so they must not run in
// parallel with each other. Other tests never execute the doctor command,
// so the swaps are invisible to them.

func TestDoctorFailingDNSCheckFails(t *testing.T) {
	origDNS, origRanges := probeDNS, probeRanges
	probeDNS = func(_ context.Context) error { return errors.New("resolver down") }
	probeRanges = func(_ context.Context) error { return nil }

	t.Cleanup(func() { probeDNS, probeRanges = origDNS, origRanges })

	if _, _, err := run(t, workspaceDomain(), "", "doctor"); err == nil {
		t.Fatal("doctor with a failing dns-resolver check must fail")
	}
}

func TestDoctorFailingRangesCheckFails(t *testing.T) {
	origDNS, origRanges := probeDNS, probeRanges
	probeDNS = func(_ context.Context) error { return nil }
	probeRanges = func(_ context.Context) error {
		return errors.New("feed unreachable")
	}

	t.Cleanup(func() { probeDNS, probeRanges = origDNS, origRanges })

	if _, _, err := run(t, workspaceDomain(), "", "doctor"); err == nil {
		t.Fatal("doctor with a failing ranges-feed check must fail")
	}
}

func TestDoctorGreenWithPassingProbes(t *testing.T) {
	origDNS, origRanges := probeDNS, probeRanges
	probeDNS = func(_ context.Context) error { return nil }
	probeRanges = func(_ context.Context) error { return nil }

	t.Cleanup(func() { probeDNS, probeRanges = origDNS, origRanges })

	if _, _, err := run(t, workspaceDomain(), "", "doctor"); err != nil {
		t.Fatalf("doctor with green probes must pass: %v", err)
	}
}

func TestCheckDNSResolverNamesBrokenServer(t *testing.T) {
	t.Setenv("GDD_RESOLVER", "127.0.0.1:1")

	err := checkDNSResolver(context.Background())
	if err == nil {
		t.Fatal("checkDNSResolver with a dead GDD_RESOLVER must fail")
	}

	if !strings.Contains(err.Error(), "GDD_RESOLVER") {
		t.Errorf("failure must name the env var, got: %v", err)
	}
}
