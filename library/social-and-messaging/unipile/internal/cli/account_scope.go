// Copyright 2026 fuushyn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// resolveAccountScope fills in the account_id scope that almost every Unipile
// route requires.
//
// account_id is an opaque 22-character id and is required on 52 of the 94
// endpoints. Most tenants have exactly one account per provider, so making the
// operator paste that id before anything works is pure friction - and without it
// a sync fails on eight provider-scoped resources with
// 400 errors/invalid_parameters.
//
// When no scope is configured this asks the API which accounts exist:
//   - exactly one account: adopt it and say so on stderr
//   - several accounts: refuse, and name them so the operator can choose
//   - none: leave the scope empty; the caller's own errors are clearer than ours
//
// A configured scope (UNIPILE_ACCOUNT_ID, --account-id, --global-param) always
// wins; this never overrides an explicit choice.
func resolveAccountScope(ctx context.Context, flags *rootFlags, userParams *syncUserParams, hint io.Writer) error {
	if userParams == nil {
		return nil
	}
	if _, ok := userParams.trueGlobal["account_id"]; ok {
		return nil
	}
	if _, ok := userParams.flatGlobal["account_id"]; ok {
		return nil
	}

	c, err := flags.newClient()
	if err != nil {
		// No usable client: let the resource-level errors speak instead.
		return nil
	}
	data, err := c.Get(ctx, "/api/v1/accounts", map[string]string{"limit": "250"})
	if err != nil {
		return nil
	}
	var envelope struct {
		Items []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"items"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil || len(envelope.Items) == 0 {
		return nil
	}
	if len(envelope.Items) > 1 {
		labels := make([]string, 0, len(envelope.Items))
		for _, a := range envelope.Items {
			labels = append(labels, fmt.Sprintf("%s (%s, %s)", a.ID, a.Type, a.Name))
		}
		return usageErr(fmt.Errorf("several accounts are connected, so the account scope is ambiguous: %s\nset UNIPILE_ACCOUNT_ID, or pass --global-param account_id=<id>", strings.Join(labels, "; ")))
	}

	only := envelope.Items[0]
	if only.ID == "" {
		return nil
	}
	if userParams.trueGlobal == nil {
		userParams.trueGlobal = map[string]string{}
	}
	userParams.trueGlobal["account_id"] = only.ID
	userParams.setGlobalDefault("account_id", only.ID)
	if hint != nil {
		fmt.Fprintf(hint, "using the only connected account for scope: %s (%s, %s). Set UNIPILE_ACCOUNT_ID to pin it.\n", only.ID, only.Type, only.Name)
	}
	return nil
}
