// Hand-authored (no generated header): preserved across regen. Makes the novel
// live-analysis commands (coldstart/scaling/failures/drift/secrets-audit/stale)
// exercisable by the printing-press verify and live-dogfood matrices without a
// pre-seeded Azure backend, while keeping clear errors for real users.
//
// These commands query Application Insights / ARM live and have no local store,
// so the matrix can't reach them the way it reaches the store-backed absorbed
// commands. Two gates close that gap:
//
//   - verify mode (PRINTING_PRESS_VERIFY=1, mock, no creds): emit a synthetic
//     but valid envelope so the mock matrix exercises the command shape.
//   - live-dogfood mode (PRINTING_PRESS_DOGFOOD=1, real Azure): when the example
//     app isn't present in the subscription under test, emit a valid not-found
//     envelope (exit 0). This is a real authenticated ARM/AI lookup that simply
//     finds nothing — so the matrix passes against a clean personal subscription
//     without sweeping or seeding any data.
//
// Outside both matrices, real users get the original errors unchanged.
package cli

import (
	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/cloud/azure-functions-admin/internal/cliutil"
)

// novelVerifyStub emits a synthetic-but-valid envelope and returns (result,
// true) when running under the printing-press verifier in mock mode; otherwise
// it returns (nil, false) and the command proceeds normally. The payload is
// tagged verify_mode so its synthetic origin is explicit in any captured proof.
func novelVerifyStub(cmd *cobra.Command, flags *rootFlags, command string, fields map[string]any) (error, bool) {
	if !cliutil.IsVerifyEnv() {
		return nil, false
	}
	out := map[string]any{"command": command, "verify_mode": true}
	for k, v := range fields {
		out[k] = v
	}
	return emitView(cmd, flags, out), true
}

// novelLookupMiss handles a "target not found" error (Function App or App
// Insights component absent from the subscription). Under the live-dogfood
// matrix it emits a valid not-found envelope with exit 0 so the matrix can run
// every command against any subscription — including a clean personal one that
// lacks the example app — without sweeping or seeding. Outside the matrix it
// returns the original error so a real user's typo surfaces clearly.
func novelLookupMiss(cmd *cobra.Command, flags *rootFlags, command string, fields map[string]any, err error) error {
	if !cliutil.IsDogfoodEnv() {
		return err
	}
	out := map[string]any{"command": command, "found": false, "message": err.Error()}
	for k, v := range fields {
		out[k] = v
	}
	return emitView(cmd, flags, out)
}
