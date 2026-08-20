// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"errors"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/suno/internal/client"
)

func TestClassifyGateProbe(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"open", nil, gateOpen},
		{"tripped", errors.New(`HTTP 422: {"error_type":"token_validation_failed"}`), gateTripped},
		{"tripped-verify-phrase", errors.New("we couldn't verify your request"), gateTripped},
		{"auth", &client.APIError{StatusCode: 401, Body: "Unauthorized"}, gateAuthFailure},
		{"other-http", &client.APIError{StatusCode: 500, Body: "boom"}, gateReachableOther},
		{"transport", errors.New("dial tcp: connection refused"), gateUnreachable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyGateProbe(tc.err); got != tc.want {
				t.Errorf("classifyGateProbe(%v) = %q, want %q", tc.err, got, tc.want)
			}
		})
	}
}

func TestProbeClipIDs(t *testing.T) {
	data := []byte(`{"status":"ok","clips":[{"id":"abc"},{"id":"def"},{"id":""}]}`)
	ids := probeClipIDs(data)
	if len(ids) != 2 || ids[0] != "abc" || ids[1] != "def" {
		t.Errorf("probeClipIDs = %v, want [abc def]", ids)
	}
	if got := probeClipIDs([]byte("not json")); got != nil {
		t.Errorf("probeClipIDs(garbage) = %v, want nil", got)
	}
}

// TestDoctorExitForFailOn_GateAuthFailure locks the P1 fix: a generate_gate
// auth-failure verdict must trip --fail-on=error (CI must not stay green while
// generation is blocked by rejected credentials).
func TestDoctorExitForFailOn_GateAuthFailure(t *testing.T) {
	report := map[string]any{
		"generate_gate": "auth-failure (HTTP 401) at the generate endpoint — credentials were rejected",
	}
	if err := doctorExitForFailOn("error", report); err == nil {
		t.Errorf("--fail-on=error must trigger on a generate_gate auth-failure verdict")
	}

	// A tripped gate is transient, not an error — it must NOT trip --fail-on=error.
	tripped := map[string]any{
		"generate_gate": "tripped — the adaptive hCaptcha gate is active right now; no clip created and no credits spent.",
	}
	if err := doctorExitForFailOn("error", tripped); err != nil {
		t.Errorf("--fail-on=error must NOT trigger on a transient tripped-gate verdict, got %v", err)
	}

	// An open gate is healthy — must not trip.
	open := map[string]any{"generate_gate": "open — generation reachable"}
	if err := doctorExitForFailOn("error", open); err != nil {
		t.Errorf("--fail-on=error must NOT trigger on an open gate, got %v", err)
	}

	// gateReachableOther (unexpected error past the gate) must trip
	// --fail-on=error, consistent with its FAIL human indicator.
	reachableOther := map[string]any{
		"generate_gate": "reachable — gate not tripped, but the probe returned an unexpected error: HTTP 500",
	}
	if err := doctorExitForFailOn("error", reachableOther); err == nil {
		t.Errorf("--fail-on=error must trigger on a gateReachableOther (unexpected error) verdict")
	}
}
