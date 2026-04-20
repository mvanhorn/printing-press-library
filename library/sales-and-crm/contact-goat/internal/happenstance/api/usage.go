// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// Usage calls GET /v1/usage. Returns the live credit balance plus purchase
// history, recent usage events, and auto-reload settings. Free probe — no
// credits spent. Doctor and any credit-spending command should call this
// before charging the user so the cost preview is accurate.
//
// Note: Me (GET /v1/users/me) lives on Client in client.go alongside the
// constructor because it is the canonical liveness probe — every doctor
// invocation hits Me first to confirm the bearer key is valid before
// touching this endpoint. Splitting them across files would force readers
// to jump around to follow the auth-check flow.
func (c *Client) Usage(ctx context.Context) (Usage, error) {
	raw, err := c.do(ctx, http.MethodGet, "/usage", nil)
	if err != nil {
		return Usage{}, err
	}
	var u Usage
	if err := json.Unmarshal(raw, &u); err != nil {
		return Usage{}, fmt.Errorf("happenstance api: decoding /usage response: %w", err)
	}
	if u.Purchases == nil {
		u.Purchases = []UsagePurchase{}
	}
	if u.Usage == nil {
		u.Usage = []UsageEvent{}
	}
	return u, nil
}
