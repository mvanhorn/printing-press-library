// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

// Shared helpers for the hand-authored nutrition novel commands (enrich,
// compare, find, meal, cite, log, rank). Kept in one file so the seven command
// files stay focused on their own logic. This is a whole hand-authored unit;
// generator regen preserves it.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/url"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/nutrition/internal/client"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/nutrition/internal/nutridata"
)

// fetchUSDAFood fetches one food by FDC id from USDA FoodData Central and
// returns both the normalized record and the raw JSON.
//
// pp:client-call
func fetchUSDAFood(ctx context.Context, c *client.Client, fdcID string) (nutridata.Food, json.RawMessage, error) {
	raw, err := c.Get(ctx, "/v1/food/"+fdcID, map[string]string{"format": "full"})
	if err != nil {
		return nutridata.Food{}, nil, err
	}
	f, err := nutridata.Normalize(raw)
	if err != nil {
		return nutridata.Food{}, raw, err
	}
	if f.FdcID == 0 {
		// The id was echoed only in the request; keep the requested id so
		// downstream joins (NutritionValue.org) still work.
		if n, convErr := strconv.Atoi(fdcID); convErr == nil {
			f.FdcID = n
		}
	}
	return f, raw, nil
}

// fetchUSDAFoods fetches up to 20 foods in one batch call. USDA's /v1/foods
// expects fdcIds as a REPEATED query param (fdcIds=X&fdcIds=Y); a single
// comma-joined value URL-encodes the comma to %2C and the API then returns only
// the first food. We build url.Values so each id is its own parameter.
//
// pp:client-call
func fetchUSDAFoods(ctx context.Context, c *client.Client, ids []string) ([]nutridata.Food, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	params := url.Values{}
	params.Set("format", "full")
	for _, id := range ids {
		params.Add("fdcIds", id)
	}
	raw, err := c.GetWithHeadersValues(ctx, "/v1/foods", params, nil)
	if err != nil {
		return nil, err
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err != nil {
		return nil, fmt.Errorf("parsing batch foods response: %w", err)
	}
	out := make([]nutridata.Food, 0, len(arr))
	for i, item := range arr {
		f, err := nutridata.Normalize(item)
		if err != nil {
			continue
		}
		if f.FdcID == 0 && i < len(ids) {
			if n, convErr := strconv.Atoi(ids[i]); convErr == nil {
				f.FdcID = n
			}
		}
		out = append(out, f)
	}
	return out, nil
}

// emitDryRun reports a dry-run action. Under --json it emits a parseable
// object (so json-fidelity checks and agents get valid JSON); otherwise it
// prints the plain human line.
func emitDryRun(cmd *cobra.Command, flags *rootFlags, would string) error {
	if flags.asJSON || flags.agent {
		return emitNutritionJSON(cmd.OutOrStdout(), map[string]any{"dry_run": true, "would": would}, flags)
	}
	fmt.Fprintln(cmd.OutOrStdout(), would)
	return nil
}

// emitNutritionJSON writes a Go value as JSON honoring --select/--compact/--csv
// via the generated filtered-output helper. Novel commands build typed structs
// and always emit machine output through this path so --agent and --select work
// for free.
func emitNutritionJSON(w io.Writer, v any, flags *rootFlags) error {
	return printJSONFiltered(w, v, flags)
}

// scaleAmount converts a per-100g nutrient amount to the given gram basis.
func scaleAmount(per100g, grams float64) float64 {
	return per100g * grams / 100.0
}

// round2 rounds to two decimals for display stability (correct for negatives).
func round2(v float64) float64 {
	return math.Round(v*100) / 100.0
}
