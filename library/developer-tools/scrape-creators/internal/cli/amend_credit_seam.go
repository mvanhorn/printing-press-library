// Copyright 2026 Adrian Horning and contributors. Licensed under Apache-2.0. See LICENSE.
// PATCH(amend-2026-08-13: credit-spend contract seam) — hand-authored injection
// point for the credit-spending novel commands (comments sweep, comments
// thread, account estimate), so budget gating, route selection, and exit-code
// contracts are testable without live paid calls.

package cli

import (
	"context"
	"encoding/json"
)

// apiGetter is the minimal client surface the credit-spending novel commands
// consume. *client.Client satisfies it; unit tests substitute a scripted fake
// so every credit-spend contract is provable offline.
type apiGetter interface {
	Get(ctx context.Context, path string, params map[string]string) (json.RawMessage, error)
}
