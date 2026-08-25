// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
// Tests for the `award` (Seats.aero mileage availability) command.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

func runAwardCmd(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	flags := &rootFlags{}
	cmd := newAwardCmd(flags)
	var out, errb bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return out.String(), errb.String(), err
}

func TestAwardCmd_RequiresOriginAndDestination(t *testing.T) {
	_, _, err := runAwardCmd(t, "SFO")
	if err == nil {
		t.Fatal("expected error for single arg")
	}
	if !strings.Contains(err.Error(), "accepts 2 arg(s)") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAwardCmd_MissingKey(t *testing.T) {
	// No SEATS_AERO_API_KEY set (env assumed clean); live path must error
	// with a clear hint rather than silently returning empty.
	for _, k := range []string{"SEATS_AERO_API_KEY", "SEATS_AERO_PARTNER_PARTNER_AUTHORIZATION"} {
		t.Setenv(k, "")
	}
	_, _, err := runAwardCmd(t, "SFO", "HND")
	if err == nil {
		t.Fatal("expected error when no API key configured")
	}
	if !strings.Contains(err.Error(), "SEATS_AERO_API_KEY") {
		t.Fatalf("expected hint about SEATS_AERO_API_KEY, got: %v", err)
	}
}

func TestAwardCmd_OrderAliasesNormalize(t *testing.T) {
	for _, k := range []string{"SEATS_AERO_API_KEY", "SEATS_AERO_PARTNER_PARTNER_AUTHORIZATION"} {
		t.Setenv(k, "")
	}
	// "cheapest" must be normalized to the API enum value lowest_mileage, never
	// sent verbatim (the API enum is only "" or lowest_mileage).
	flags := &rootFlags{dryRun: true}
	cmd := newAwardCmd(flags)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"SFO", "HND", "--order", "cheapest", "--from", "2026-10-01"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	s := stdout.String()
	if !strings.Contains(s, "order_by=lowest_mileage") {
		t.Errorf("cheapest should normalize to order_by=lowest_mileage, got:\n%s", s)
	}
	if strings.Contains(s, "order_by=cheapest") {
		t.Errorf("cheapest must NOT be sent verbatim:\n%s", s)
	}
}

func TestAwardCmd_InvalidOrderRejected(t *testing.T) {
	for _, k := range []string{"SEATS_AERO_API_KEY", "SEATS_AERO_PARTNER_PARTNER_AUTHORIZATION"} {
		t.Setenv(k, "")
	}
	flags := &rootFlags{dryRun: true}
	cmd := newAwardCmd(flags)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"SFO", "HND", "--order", "bogus"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "invalid --order") {
		t.Fatalf("expected invalid --order error, got: %v", err)
	}
}

func TestAwardCmd_DryRunWithoutKey(t *testing.T) {
	for _, k := range []string{"SEATS_AERO_API_KEY", "SEATS_AERO_PARTNER_PARTNER_AUTHORIZATION"} {
		t.Setenv(k, "")
	}
	// --dry-run is a root persistent flag; set it directly on the flags in
	// this unit fixture (equivalent to `... award ... --dry-run`).
	flags := &rootFlags{dryRun: true}
	cmd := newAwardCmd(flags)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"SFO", "HND",
		"--from", "2026-10-01", "--to", "2026-10-31",
		"--cabin", "business", "--order", "lowest_mileage"})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("dry-run should not require a key: %v", err)
	}
	s := stdout.String()
	for _, want := range []string{
		"seatsaero.Search(SFO -> HND)", "2026-10-01..2026-10-31",
		"origin_airport=SFO", "destination_airport=HND",
		"order_by=lowest_mileage", "cabin=business",
		"dry run - no request sent",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("dry-run stdout missing %q:\n%s", want, s)
		}
	}
	// API key must never leak into dry-run output.
	if strings.Contains(s, "Partner-Authorization") {
		t.Errorf("dry-run leaked auth header into output")
	}
}
