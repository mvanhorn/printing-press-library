// Copyright 2026 Prashant Kamani and contributors. Licensed under Apache-2.0. See LICENSE.
//
// N131.5.5 F-1 — the --fail-on gate and the printed report must agree.
//
// `doctor` prints one indicator per check (OK / INFO / WARN / FAIL) from
// doctorCheckIndicator. Before this test the gate ran its own weaker token
// scan, so a home with no credentials printed "FAIL Auth: not configured" and
// still exited 0 under --fail-on warn and --fail-on error: a check the tool
// itself labels FAIL that no exit gate reports.

package cli

import "testing"

func TestDoctorFailOnHonoursThePrintedVerdicts(t *testing.T) {
	notConfigured := map[string]any{
		"config":  "ok",
		"auth":    "not configured",
		"api":     "reachable (HTTP 404 at /)",
		"version": "0.0.0-dev",
	}
	for _, failOn := range []string{"warn", "error"} {
		if err := doctorExitForFailOn(failOn, notConfigured); err == nil {
			t.Errorf("--fail-on %s: want a non-nil error when the report prints FAIL Auth: not configured, got nil", failOn)
		}
	}

	warnOnly := map[string]any{
		"config":        "ok",
		"auth":          "configured",
		"paths_warning": "WARN paths: home override shadowed",
	}
	if err := doctorExitForFailOn("warn", warnOnly); err == nil {
		t.Error("--fail-on warn: want a non-nil error on a WARN section, got nil")
	}
	if err := doctorExitForFailOn("error", warnOnly); err != nil {
		t.Errorf("--fail-on error: a WARN section must not trip the error gate, got %v", err)
	}

	// The healthy owner-home shape: every rendered check is OK or INFO, so no
	// gate value may fail. "present, not verified" renders INFO on purpose
	// (doctor.go: credentials loaded, no probe run), and must stay below warn.
	healthy := map[string]any{
		"config":      "ok",
		"auth":        "configured",
		"api":         "reachable (HTTP 404 at /)",
		"credentials": "present, not verified",
		"env_vars":    "OK 0/2 available",
		"version":     "0.0.0-dev",
		"config_path": "/home/someone/.config/garmin-pp-cli/config.toml",
		"auth_source": "credentials-file",
		"cache": map[string]any{
			"status": "ok",
		},
	}
	for _, failOn := range []string{"warn", "error", "stale"} {
		if err := doctorExitForFailOn(failOn, healthy); err != nil {
			t.Errorf("--fail-on %s on a healthy report: want nil, got %v", failOn, err)
		}
	}
}

func TestDoctorCheckIndicatorAgreesWithTheGate(t *testing.T) {
	cases := []struct {
		value string
		want  string
	}{
		{"not configured", doctorIndicatorFail},
		{"ok", doctorIndicatorOK},
		{"configured", doctorIndicatorOK},
		{"WARN paths: cache dir missing from PATH", doctorIndicatorWarn},
		{"present, not verified", doctorIndicatorInfo},
		{"optional", doctorIndicatorInfo},
		{"not required", doctorIndicatorOK},
		{"refused: keychain denied", doctorIndicatorFail},
		{"reachable (HTTP 404 at /)", doctorIndicatorOK},
	}
	for _, c := range cases {
		if got := doctorCheckIndicator(c.value); got != c.want {
			t.Errorf("doctorCheckIndicator(%q) = %q, want %q", c.value, got, c.want)
		}
	}
}
