// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.
//
// pp:data-source live
//
// `credits` — show remaining credits and plan from the live billing endpoint.
// With --forecast, also reports recent local generation volume against Suno's
// captcha-throttle threshold. Reads billing live (auth required); --forecast
// also reads the local clip store. Read-only (does not mutate Suno).

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/suno/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/suno/internal/store"
	"github.com/spf13/cobra"
)

// captchaThrottleThreshold is the documented approximate credit-usage level at
// which Suno begins inserting captcha challenges that throttle generation.
const captchaThrottleThreshold = 200

func newSunoCreditsCmd(flags *rootFlags) *cobra.Command {
	var (
		forecast bool
		window   string
		dbPath   string
	)
	cmd := &cobra.Command{
		Use:   "credits",
		Short: "Show remaining credits and plan (live); --forecast adds recent volume",
		Long: "Show remaining credits and plan from the live Suno billing endpoint.\n\n" +
			"With --forecast, also counts locally-stored clips created in the trailing " +
			"--window (default 7d) and reports your recent generation volume against " +
			"Suno's documented ~200-credit captcha-throttle threshold.",
		Example:     "  suno-pp-cli credits\n  suno-pp-cli credits --forecast --window 7d --json",
		Annotations: map[string]string{"pp:data-source": "live", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			windowDur, err := cliutil.ParseDurationLoose(window)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --window %q: %w", window, err))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.GetWithHeaders(cmd.Context(), "/api/billing/info/", map[string]string{}, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			data = extractResponseData(data)

			result := parseCredits(data)

			if forecast {
				generations, ferr := countRecentClipsImpl(dbPath, windowDur, time.Now())
				fc := &creditsForecast{
					Window:            window,
					ThrottleThreshold: captchaThrottleThreshold,
				}
				if ferr != nil {
					// Forecast is best-effort: a missing/empty local store should
					// not fail the live credits read. Report the volume as
					// unavailable rather than a measured zero so a consumer
					// reading only the note isn't misled into thinking no
					// generations occurred.
					fmt.Fprintf(cmd.ErrOrStderr(), "hint: forecast unavailable: %v\n", ferr)
					fc.Note = fmt.Sprintf("%d credits left; recent generation volume unavailable (local store unreadable); throttle typically ~%d credits of use", result.Credits, captchaThrottleThreshold)
				} else {
					fc.GenerationsInWindow = generations
					fc.Note = fmt.Sprintf("%d credits left; %d generations in last %s; throttle typically ~%d credits of use", result.Credits, generations, window, captchaThrottleThreshold)
				}
				result.Forecast = fc
			}

			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().BoolVar(&forecast, "forecast", false, "Also report recent local generation volume vs the throttle threshold")
	cmd.Flags().StringVar(&window, "window", "7d", "Trailing window for --forecast (e.g. 7d, 24h, 1w)")
	cmd.Flags().StringVar(&dbPath, "db", defaultDBPath("suno-pp-cli"), "Path to local SQLite store (for --forecast)")
	return cmd
}

// creditsResult is the command output.
type creditsResult struct {
	Credits  int64            `json:"credits"`
	Plan     string           `json:"plan,omitempty"`
	Period   string           `json:"period,omitempty"`
	Forecast *creditsForecast `json:"forecast,omitempty"`
}

type creditsForecast struct {
	Window              string `json:"window"`
	GenerationsInWindow int64  `json:"generations_in_window"`
	ThrottleThreshold   int    `json:"throttle_threshold"`
	Note                string `json:"note"`
}

// parseCredits tolerantly extracts credits, plan, and billing period from the
// billing body. Fields may be absent; total_credits_left falls back to credits.
func parseCredits(data json.RawMessage) creditsResult {
	var obj map[string]any
	_ = json.Unmarshal(data, &obj)

	res := creditsResult{Credits: creditsFromBilling(obj)}
	res.Plan = planName(obj)
	res.Period, _ = obj["period"].(string)
	return res
}

// planName extracts a human plan name from the billing body. Suno's live shape
// nests the plan under a "plan" object (plan.name, e.g. "Premier Plan", or
// plan.plan_key, e.g. "premier"); simpler/legacy shapes expose it as a
// top-level string. "period" (the billing interval, e.g. "year") is the
// account's billing cadence, not its plan, so it is deliberately excluded.
func planName(obj map[string]any) string {
	if plan, ok := obj["plan"].(map[string]any); ok {
		for _, key := range []string{"name", "plan_key"} {
			if v, ok := plan[key].(string); ok && v != "" {
				return v
			}
		}
	}
	for _, key := range []string{"plan", "subscription_type", "subscription", "tier"} {
		if v, ok := obj[key].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

// creditsFromBilling reads the remaining-credit count, preferring
// total_credits_left and falling back to credits.
func creditsFromBilling(obj map[string]any) int64 {
	for _, key := range []string{"total_credits_left", "credits"} {
		if v, ok := obj[key]; ok {
			if n, ok := toInt64(v); ok {
				return n
			}
		}
	}
	return 0
}

func toInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case float64:
		return int64(n), true
	case int64:
		return n, true
	case int:
		return int64(n), true
	case json.Number:
		if i, err := n.Int64(); err == nil {
			return i, true
		}
	}
	return 0, false
}

// countRecentClipsImpl counts locally-stored clips created within the trailing
// window. created_at is an ISO-8601 string in the clip JSON / typed column;
// it's parsed in Go so heterogeneous timestamp formats are handled. now is
// injected so the window math is unit-testable.
func countRecentClipsImpl(dbPath string, window time.Duration, now time.Time) (int64, error) {
	db, err := store.Open(dbPath)
	if err != nil {
		return 0, fmt.Errorf("opening local store: %w", err)
	}
	defer db.Close()

	rows, err := db.DB().Query(`SELECT created_at FROM clips WHERE created_at IS NOT NULL`)
	if err != nil {
		return 0, fmt.Errorf("reading clips: %w", err)
	}
	defer rows.Close()

	cutoff := now.Add(-window)
	var count int64
	for rows.Next() {
		var ca string
		if err := rows.Scan(&ca); err != nil {
			return 0, err
		}
		if t, ok := parseClipTime(ca); ok && t.After(cutoff) {
			count++
		}
	}
	return count, rows.Err()
}

// parseClipTime parses a Suno created_at string across the common layouts.
func parseClipTime(s string) (time.Time, bool) {
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.999999",
		"2006-01-02 15:04:05",
		"2006-01-02",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
