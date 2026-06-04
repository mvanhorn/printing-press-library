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
