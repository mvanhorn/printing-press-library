// Copyright 2026 Paul Taramona and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
)

// PATCH(gocli1219-live-gate-defects): `imports blame --latest` and `imports watch --latest` resolve the account's most
// recent import job (the first row of GET /imports, which lists newest first), so the two commands do real work
// without a job id copied from somewhere else. "" means the account has no import job yet.
func resolveLatestImportJob(ctx context.Context, c interface {
	Get(context.Context, string, map[string]string) (json.RawMessage, error)
}) (string, error) {
	data, err := c.Get(ctx, "/imports", map[string]string{"per_page": "1"})
	if err != nil {
		return "", err
	}
	var env struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return "", fmt.Errorf("parsing import jobs: %w", err)
	}
	var rows []map[string]any
	if err := json.Unmarshal(env.Data, &rows); err != nil {
		var nested struct {
			Data []map[string]any `json:"data"`
		}
		if err := json.Unmarshal(env.Data, &nested); err != nil {
			return "", fmt.Errorf("parsing import jobs: %w", err)
		}
		rows = nested.Data
	}
	if len(rows) == 0 {
		return "", nil
	}
	id, _ := rows[0]["id"].(string)
	return id, nil
}

// noImportJobYet is the exit-0 answer of --latest on an account without import jobs.
func noImportJobYet() map[string]any {
	return map[string]any{"jobId": nil, "note": "this account has no import job yet"}
}
