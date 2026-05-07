// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// newBroadcastsPreflightCmd checks whether a broadcast is safe to trigger.
// Returns a structured verdict (green / yellow / red) backed by:
//
//   - segment size (live API)
//   - suppression overlap (live API)
//   - last-sent recency from the local deliveries cache
//
// The Customer.io broadcast trigger is rate-limited to 1 request per 10 s,
// so this check exists explicitly to avoid 429 storms and double-sends.
func newBroadcastsPreflightCmd(flags *rootFlags) *cobra.Command {
	var segmentID string
	var window string
	cmd := &cobra.Command{
		Use:   "preflight <environment-id> <broadcast-id>",
		Short: "Check whether a broadcast is safe to trigger (segment size, suppression overlap, last-sent recency)",
		Long: `Pre-trigger safety check for a broadcast. Examines the target segment's
size, the suppression count in the workspace (a coarse overlap proxy), and
the recency of any deliveries to those members in the local cache.

Verdict is one of:
  green  — safe to send.
  yellow — suspicious signal (e.g. >5%% suppressed, or any recipient hit in
           the last hour). Review structured reasons before triggering.
  red    — blocking signal (e.g. >25%% suppressed, or any recipient hit in
           the last 5 minutes). Do not trigger without intent.`,
		Example: strings.Trim(`
  customer-io-pp-cli broadcasts preflight 123457 1 --segment 1
  customer-io-pp-cli broadcasts preflight 123457 1 --segment 1 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			envID := strings.TrimSpace(args[0])
			broadcastID := strings.TrimSpace(args[1])
			if envID == "" || broadcastID == "" {
				return usageErr(fmt.Errorf("environment_id and broadcast_id required"))
			}
			if segmentID == "" {
				return usageErr(fmt.Errorf("--segment is required (preflight needs a target segment to check)"))
			}
			windowDur, err := parseSimpleDuration(nonEmpty(window, "24h"))
			if err != nil {
				return usageErr(fmt.Errorf("invalid --window: %w", err))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// 1. Segment members.
			members, err := liveSegmentMemberSet(c, envID, segmentID)
			if err != nil {
				return classifyAPIError(fmt.Errorf("segment membership: %w", err), flags)
			}

			// 2. Suppression count (workspace-level proxy; per-member overlap
			// requires a full export and is too heavy for preflight).
			overlap, err := suppressedCount(c, envID)
			if err != nil {
				return classifyAPIError(fmt.Errorf("suppressions count: %w", err), flags)
			}

			// 3. Recently-sent (last-sent recency from local store).
			recent, err := recentlySentToSegment(cmd.Context(), members, windowDur)
			if err != nil {
				// Local store missing is informative, not fatal — surface as null.
				recent = -1
			}

			result := map[string]any{
				"broadcast":               broadcastID,
				"segment":                 segmentID,
				"segment_size":            len(members),
				"suppressed_in_segment":   overlap,
				"recently_sent_in_window": recent,
				"window":                  windowDur.String(),
			}
			verdict, reasons := classifyPreflight(len(members), overlap, recent)
			result["verdict"] = verdict
			result["reasons"] = reasons

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Broadcast %s preflight (segment %s)\n\n", broadcastID, segmentID)
			fmt.Fprintf(cmd.OutOrStdout(), "  Segment size:       %d\n", len(members))
			fmt.Fprintf(cmd.OutOrStdout(), "  Suppressed overlap: %d\n", overlap)
			if recent >= 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "  Recently sent (%s): %d\n", windowDur, recent)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "  Recently sent: (no local store; run sync first)\n")
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n  Verdict: %s\n", strings.ToUpper(verdict))
			for _, r := range reasons {
				fmt.Fprintf(cmd.OutOrStdout(), "    - %s\n", r)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&segmentID, "segment", "", "Target segment ID (required)")
	cmd.Flags().StringVar(&window, "window", "24h", "Recency window for last-sent check (e.g. 1h, 24h, 7d)")
	return cmd
}

// suppressedCount calls the workspace-scoped count endpoint. Used as a
// coarse overlap proxy in broadcast preflight; the API offers no list
// endpoint that returns the membership directly.
func suppressedCount(c clientGetter, envID string) (int, error) {
	data, err := c.Get("/v1/environments/"+envID+"/customers_suppression_count", nil)
	if err != nil {
		return 0, err
	}
	var raw struct {
		Count int `json:"count"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return 0, err
	}
	return raw.Count, nil
}

// suppressedSet returns identifiers known to be suppressed in this account.
// Currently unused by preflight (replaced by suppressedCount); retained for
// future audit/triage commands. Customer.io's `exclusions` endpoint returns
// HTML in the new SA-token flow, so callers should switch to the export
// path or the workspace-level count proxy.
func suppressedSet(c clientGetter, limit int) (map[string]struct{}, error) {
	params := map[string]string{}
	if limit > 0 {
		params["limit"] = fmt.Sprintf("%d", limit)
	}
	data, err := c.Get("/v1/api/exclusions", params)
	if err != nil {
		return nil, err
	}
	var raw struct {
		Exclusions []struct {
			ID    string `json:"id"`
			Email string `json:"email"`
		} `json:"exclusions"`
	}
	if err := json.Unmarshal(data, &raw); err == nil && len(raw.Exclusions) > 0 {
		set := make(map[string]struct{}, len(raw.Exclusions))
		for _, e := range raw.Exclusions {
			if e.ID != "" {
				set[e.ID] = struct{}{}
			}
			if e.Email != "" {
				set[e.Email] = struct{}{}
			}
		}
		return set, nil
	}
	// Fallback: array of objects with email.
	var arr []struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	}
	if err := json.Unmarshal(data, &arr); err == nil {
		set := make(map[string]struct{}, len(arr))
		for _, e := range arr {
			if e.ID != "" {
				set[e.ID] = struct{}{}
			}
			if e.Email != "" {
				set[e.Email] = struct{}{}
			}
		}
		return set, nil
	}
	return map[string]struct{}{}, nil
}

func recentlySentToSegment(ctx context.Context, members map[string]struct{}, window time.Duration) (int, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	db, err := openTimelineStore()
	if err != nil {
		return 0, err
	}
	defer db.Close()
	cutoff := time.Now().Add(-window).Unix()
	rows, err := db.DB().QueryContext(ctx, `SELECT DISTINCT
	    IFNULL(json_extract(data, '$.customer_id'), json_extract(data, '$.recipient')) AS who
	    FROM deliveries
	    WHERE IFNULL(json_extract(data, '$.created'), json_extract(data, '$.updated')) >= ?`, cutoff)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var who sql.NullString
		if scanErr := rows.Scan(&who); scanErr != nil {
			return 0, scanErr
		}
		if who.Valid {
			if _, ok := members[who.String]; ok {
				count++
			}
		}
	}
	return count, rows.Err()
}

func classifyPreflight(segmentSize, overlap, recent int) (string, []string) {
	reasons := []string{}
	verdict := "green"
	if segmentSize == 0 {
		return "red", []string{"target segment is empty"}
	}
	pct := float64(overlap) / float64(segmentSize)
	switch {
	case pct >= 0.25:
		verdict = "red"
		reasons = append(reasons, fmt.Sprintf("%.1f%% of segment is currently suppressed (>= 25%%)", pct*100))
	case pct >= 0.05:
		verdict = "yellow"
		reasons = append(reasons, fmt.Sprintf("%.1f%% of segment is currently suppressed (>= 5%%)", pct*100))
	}
	if recent >= 0 {
		recentPct := float64(recent) / float64(segmentSize)
		switch {
		case recentPct >= 0.10:
			verdict = "red"
			reasons = append(reasons, fmt.Sprintf("%.1f%% of segment received a delivery in the window (>= 10%%); double-send risk", recentPct*100))
		case recent > 0:
			if verdict == "green" {
				verdict = "yellow"
			}
			reasons = append(reasons, fmt.Sprintf("%d segment members received a delivery in the window", recent))
		}
	}
	if verdict == "green" {
		reasons = append(reasons, "no blocking or warning signals detected")
	}
	return verdict, reasons
}
