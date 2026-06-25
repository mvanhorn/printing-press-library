// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"errors"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/suno/internal/client"
)

func TestNeedsCaptchaSolve(t *testing.T) {
	tokenVal := errors.New(`POST /api/generate/v2-web/ returned HTTP 422: {"error_type":"token_validation_failed"}`)
	server500 := &client.APIError{Method: "POST", Path: "/api/generate/v2-web/", StatusCode: 500, Body: `{"error_type":"server_error"}`}
	client400 := errors.New(`POST /api/generate/v2-web/ returned HTTP 400: bad request`)

	cases := []struct {
		name        string
		err         error
		tokenWasNil bool
		want        bool
	}{
		{"token_validation always solves (nil token)", tokenVal, true, true},
		{"token_validation always solves (token present)", tokenVal, false, true},
		{"null-token 500 solves", server500, true, true},
		{"500 with token present does NOT solve", server500, false, false},
		{"client 400 never solves", client400, true, false},
		{"nil error never solves", nil, true, false},
	}
	for _, tc := range cases {
		if got := needsCaptchaSolve(tc.err, tc.tokenWasNil); got != tc.want {
			t.Errorf("%s: needsCaptchaSolve = %v, want %v", tc.name, got, tc.want)
		}
	}
}
