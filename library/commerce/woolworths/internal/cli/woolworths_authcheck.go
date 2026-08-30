// Copyright 2026 Richard Gill and contributors. Licensed under Apache-2.0. See LICENSE.

// Soft-401 detection for the /api/v3/ui/* surface.
//
// Woolworths runs two API generations. The older /apis/ui/* endpoints return a
// real HTTP 401 when the session is missing or expired, which the generated
// client already turns into exit 4 (pastshops behaves correctly for this
// reason). The newer /api/v3/ui/* endpoints do NOT: `savedlists` answers with
// HTTP 200 and buries the failure in the envelope:
//
//	{"success": false, "data": {"q": null}, "statusCode": 401, ...}
//
// Nothing in the generated path inspects that, so the command exited 0 and
// handed back what looked like an empty-but-successful result. Since the
// session cookies are ~1-hour JWTs, this is the *normal* end state of every
// authenticated session: the user's token quietly expires and the CLI reports
// success with no lists. That is the worst possible shape for an agent, which
// cannot distinguish "you have no saved lists" from "your session died".
//
// This file is hand-authored so `generate --force` preserves it. The call sites
// in the generated savedlists commands are re-applied by
// scripts/reapply-templated-hand-edits.sh.

package cli

import (
	"encoding/json"
	"fmt"
)

// woolworthsEnvelopeStatus is the subset of the /api/v3/ui/* envelope that
// carries the real outcome.
type woolworthsEnvelopeStatus struct {
	Success    *bool `json:"success"`
	StatusCode *int  `json:"statusCode"`
}

// checkWoolworthsSoftAuthFailure inspects a /api/v3/ui/* response body and
// returns a typed auth error when the envelope reports an authentication
// failure despite the HTTP status being 200.
//
// It returns nil for any body it does not positively recognise as a soft auth
// failure, so a malformed or unexpected payload never blocks a real result.
func checkWoolworthsSoftAuthFailure(body []byte) error {
	if len(body) == 0 {
		return nil
	}
	var env woolworthsEnvelopeStatus
	if err := json.Unmarshal(body, &env); err != nil {
		return nil // not this envelope shape; leave the response alone
	}
	// Only act when the envelope explicitly reports failure AND names an
	// auth-ish status. success:false with a 500 is a server fault, not an
	// auth problem, and must keep its own error path.
	if env.Success == nil || *env.Success {
		return nil
	}
	if env.StatusCode == nil {
		return nil
	}
	switch *env.StatusCode {
	case 401, 403:
		return authErrf("Woolworths returned HTTP 200 with an inner %d: the session is missing or expired.\n"+
			"Session cookies are short-lived (about an hour), so this is the normal end of an imported session.\n"+
			"hint: re-import from a logged-in browser with 'woolworths-pp-cli auth login --cookies-file <cookie-header file>'",
			*env.StatusCode)
	default:
		return nil
	}
}

// authErrf builds an error carrying the CLI's auth exit code (4), matching what
// the generated client produces for a real HTTP 401.
func authErrf(format string, args ...any) error {
	return &cliError{code: 4, err: fmt.Errorf(format, args...)}
}
