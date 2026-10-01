// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
// Tests for the `award` (Seats.aero mileage availability) command.

package cli

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func runAwardCmd(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	return runAwardCmdWithFlags(t, &rootFlags{}, args...)
}

func runAwardCmdWithFlags(t *testing.T, flags *rootFlags, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	cmd := newAwardCmd(flags)
	var out, errb bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return out.String(), errb.String(), err
}

func clearSeatsAeroAuth(t *testing.T) {
	t.Helper()
	t.Setenv("SEATS_AERO_API_KEY", "")
	t.Setenv("SEATS_AERO_PARTNER_PARTNER_AUTHORIZATION", "")
	t.Setenv("SEATS_AERO_CONFIG", t.TempDir()+"/missing-config.toml")
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
	// With every credential source isolated, the live path must error with a
	// clear hint rather than silently returning empty.
	clearSeatsAeroAuth(t)
	_, _, err := runAwardCmd(t, "SFO", "HND")
	if err == nil {
		t.Fatal("expected error when no API key configured")
	}
	if !strings.Contains(err.Error(), "SEATS_AERO_API_KEY") {
		t.Fatalf("expected hint about SEATS_AERO_API_KEY, got: %v", err)
	}
}

func TestAwardCmd_OrderAliasesNormalize(t *testing.T) {
	clearSeatsAeroAuth(t)
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
	clearSeatsAeroAuth(t)
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
	clearSeatsAeroAuth(t)
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

func TestAwardCmd_ValidatesDocumentedInputsBeforeDryRunOrLiveRequest(t *testing.T) {
	clearSeatsAeroAuth(t)
	tests := map[string]struct {
		args []string
		want string
	}{
		"short origin":       {[]string{"SF", "HND"}, "invalid origin"},
		"punctuated airport": {[]string{"SFO", "HND!"}, "invalid destination"},
		"bad origin list":    {[]string{"SFO,LA", "HND"}, "invalid origin"},
		"malformed from":     {[]string{"SFO", "HND", "--from", "2026-13-40"}, "invalid --from"},
		"malformed to":       {[]string{"SFO", "HND", "--to", "tomorrow"}, "invalid --to"},
		"reversed window":    {[]string{"SFO", "HND", "--from", "2026-10-31", "--to", "2026-10-01"}, "invalid date window"},
		"unsupported cabin":  {[]string{"SFO", "HND", "--cabin", "suite"}, "invalid --cabin"},
		"bad cabin list":     {[]string{"SFO", "HND", "--cabin", "business,suite"}, "invalid --cabin"},
		"negative take":      {[]string{"SFO", "HND", "--take", "-1"}, "invalid --take"},
		"zero take":          {[]string{"SFO", "HND", "--take", "0"}, "invalid --take"},
		"small take":         {[]string{"SFO", "HND", "--take", "9"}, "invalid --take"},
		"large take":         {[]string{"SFO", "HND", "--take", "1001"}, "invalid --take"},
	}
	for name, test := range tests {
		for _, dryRun := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/dry-run=%t", name, dryRun), func(t *testing.T) {
				_, _, err := runAwardCmdWithFlags(t, &rootFlags{dryRun: dryRun}, test.args...)
				if err == nil || !strings.Contains(err.Error(), test.want) {
					t.Fatalf("error = %v, want %q", err, test.want)
				}
			})
		}
	}
}

func TestAwardCmd_RejectsInvalidTakeFromProfile(t *testing.T) {
	clearSeatsAeroAuth(t)
	t.Setenv("FLIGHT_GOAT_CONFIG_DIR", t.TempDir())
	if err := saveProfileStore(&profileStore{Profiles: map[string]Profile{
		"bad-take": {
			Name:   "bad-take",
			Values: map[string]string{"take": "5"},
		},
		"zero-take": {
			Name:   "zero-take",
			Values: map[string]string{"take": "0"},
		},
	}}); err != nil {
		t.Fatalf("save profile: %v", err)
	}
	for _, tc := range []struct{ profile, value string }{{"bad-take", "5"}, {"zero-take", "0"}} {
		stdout, _, err := runRootArgs(t,
			"award", "SFO", "HND", "--profile", tc.profile, "--dry-run", "--no-learn")
		if err == nil || !strings.Contains(err.Error(), "invalid --take "+tc.value) {
			t.Fatalf("profile %s error = %v, want invalid take", tc.profile, err)
		}
		if strings.Contains(stdout, "take="+tc.value) {
			t.Fatalf("invalid profile take reached dry-run serialization: %s", stdout)
		}
	}
}

func TestAwardCmd_NormalizesAirportListsAndCabin(t *testing.T) {
	clearSeatsAeroAuth(t)
	out, _, err := runAwardCmdWithFlags(t, &rootFlags{dryRun: true},
		"sfo, lax", "hnd,nrt", "--cabin", "BUSINESS", "--take", "10")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"origin_airport=SFO,LAX", "destination_airport=HND,NRT", "cabin=business", "take=10"} {
		if !strings.Contains(out, want) {
			t.Errorf("dry-run output missing %q: %s", want, out)
		}
	}
}

func TestAwardCmd_NormalizesMultipleCabins(t *testing.T) {
	clearSeatsAeroAuth(t)
	out, _, err := runAwardCmdWithFlags(t, &rootFlags{dryRun: true},
		"SFO", "HND", "--cabin", "BUSINESS, economy, BUSINESS")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "cabins=business,economy") {
		t.Fatalf("dry-run output missing plural cabins parameter: %s", out)
	}
	if !strings.Contains(out, "seatsaero.Search(SFO -> HND) cabins=business,economy") {
		t.Fatalf("dry-run summary disagrees with plural cabins request: %s", out)
	}
	if strings.Contains(out, "&cabin=business,economy") {
		t.Fatalf("multi-cabin filter used singular cabin parameter: %s", out)
	}
}
