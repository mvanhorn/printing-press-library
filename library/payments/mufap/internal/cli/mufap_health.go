// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"

	"github.com/mvanhorn/printing-press-library/library/payments/mufap/internal/client"
)

// mufapHealthGet performs the doctor's reachability probe.
//
// MUFAP serves HTML at every GET path -- its JSON endpoints are POST-only --
// so the generated doctor's plain c.Get() fails with "expected JSON, API
// returned HTML instead of JSON". That error is not a *client.APIError, so the
// doctor's switch falls through to its network-failure branch and reports the
// site as "unreachable" even though it answered HTTP 200. Opting this one
// request into the HTML-response path makes the verdict match reality.
func mufapHealthGet(ctx context.Context, c *client.Client, path string) (json.RawMessage, error) {
	return c.GetWithHeaders(ctx, path, nil, map[string]string{
		client.HTMLResponseHeader: "true",
	})
}
