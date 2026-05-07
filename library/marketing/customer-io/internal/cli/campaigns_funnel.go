// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// newCampaignsFunnelCmd renders a step-by-step journey funnel for one
// campaign. The basic funnel is the live API's journey_metrics; with
// --segment, the command joins synced deliveries against the segment's
// live membership to render the per-segment cross-cut that the API itself
// does not expose.
func newCampaignsFunnelCmd(flags *rootFlags) *cobra.Command {
	var segmentID string
	var since string
	cmd := &cobra.Command{
		Use:   "funnel <environment-id> <campaign-id>",
		Short: "Render a step-by-step journey funnel (sent → delivered → opened → clicked → converted), optionally cross-cut by segment",
		Long: `Without --segment, this command returns Customer.io's journey_metrics for
the campaign verbatim — the same data the campaigns journey-metrics command
returns, framed as a funnel.

With --segment, the command also joins synced deliveries (from the local
store) against the segment's live membership to compute the per-segment
funnel cross-cut. Customer.io's journey_metrics endpoint does not expose
this breakdown; only a local join can produce it.

Run 'customer-io-pp-cli sync --resources deliveries --since 90d' first when
using --segment.`,
		Example: strings.Trim(`
  customer-io-pp-cli campaigns funnel 123457 1 --json
  customer-io-pp-cli campaigns funnel 123457 cmp_482 --segment 19 --since 30d
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
			campaignID := strings.TrimSpace(args[1])
			if envID == "" || campaignID == "" {
				return usageErr(fmt.Errorf("environment_id and campaign_id required"))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			// Try journey_metrics first (step-by-step counts); fall back to
			// /metrics when the API rejects journey_metrics. Customer.io's
			// journey_metrics endpoint has undocumented required params that
			// vary by campaign type; /metrics returns the same five funnel
			// counts (sent/delivered/opened/clicked/converted) reliably.
			params := map[string]string{"period": "days", "steps": "30"}
			source := "journey_metrics"
			rawMetrics, err := c.Get("/v1/environments/"+envID+"/campaigns/"+campaignID+"/journey_metrics", params)
			if err != nil {
				if alt, altErr := c.Get("/v1/environments/"+envID+"/campaigns/"+campaignID+"/metrics", params); altErr == nil {
					rawMetrics = alt
					source = "metrics_fallback"
					err = nil
				}
			}
			if err != nil {
				return classifyAPIError(err, flags)
			}

			out := map[string]any{
				"environment_id":  envID,
				"campaign":        campaignID,
				"metrics_source":  source,
				"journey_metrics": json.RawMessage(rawMetrics),
			}

			if segmentID == "" {
				if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
					return printJSONFiltered(cmd.OutOrStdout(), out, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Campaign %s — journey metrics\n\n", campaignID)
				fmt.Fprintln(cmd.OutOrStdout(), string(rawMetrics))
				return nil
			}

			// Per-segment cross-cut: join synced deliveries with live segment membership.
			cutoff, err := parseSinceCutoff(since)
			if err != nil {
				return usageErr(err)
			}

			members, err := liveSegmentMemberSet(c, envID, segmentID)
			if err != nil {
				return classifyAPIError(fmt.Errorf("segment %s membership: %w", segmentID, err), flags)
			}

			db, err := openTimelineStore()
			if err != nil {
				return apiErr(fmt.Errorf("opening local store: %w (run 'customer-io-pp-cli sync --resources deliveries' first)", err))
			}
			defer db.Close()

			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			steps, err := computeFunnelSteps(ctx, db.DB(), campaignID, members, cutoff)
			if err != nil {
				return apiErr(fmt.Errorf("computing funnel: %w", err))
			}
			out["segment"] = segmentID
			out["segment_size"] = len(members)
			out["funnel_steps"] = steps

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Campaign %s × Segment %s — funnel cross-cut\n", campaignID, segmentID)
			fmt.Fprintf(cmd.OutOrStdout(), "Segment size: %d members\n\n", len(members))
			for _, step := range steps {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-12s %d unique recipients\n", step.State, step.UniqueRecipients)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&segmentID, "segment", "", "Cross-cut the funnel by segment (joins synced deliveries with live segment membership)")
	cmd.Flags().StringVar(&since, "since", "", "Only count deliveries newer than this duration (e.g. 7d, 30d, 90d)")
	return cmd
}

func liveSegmentMemberSet(c clientGetter, envID, segmentID string) (map[string]struct{}, error) {
	data, err := c.Get("/v1/environments/"+envID+"/segments/"+segmentID+"/membership", nil)
	if err != nil {
		return nil, err
	}
	ids := extractMembershipIDs(data)
	set := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		set[id] = struct{}{}
	}
	return set, nil
}

// clientGetter narrows *client.Client down to the single method our novel
// commands need; lets us avoid pulling the full client package into helpers.
type clientGetter interface {
	Get(path string, params map[string]string) (json.RawMessage, error)
}

type funnelStep struct {
	State            string `json:"state"`
	UniqueRecipients int    `json:"unique_recipients"`
}

// computeFunnelSteps inspects deliveries.data for this campaign and counts
// distinct recipients at each pipeline state. Only deliveries to a member of
// the supplied segment are counted.
func computeFunnelSteps(ctx context.Context, db *sql.DB, campaignID string, members map[string]struct{}, cutoff int64) ([]funnelStep, error) {
	q := `SELECT json_extract(data, '$.customer_id') AS customer_id,
	             json_extract(data, '$.recipient')   AS recipient,
	             json_extract(data, '$.state')       AS state
	      FROM deliveries
	      WHERE json_extract(data, '$.campaign_id') = ?`
	args := []any{campaignID}
	if cutoff > 0 {
		q += ` AND IFNULL(json_extract(data, '$.created'), json_extract(data, '$.updated')) >= ?`
		args = append(args, cutoff)
	}
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stateUsers := map[string]map[string]struct{}{}
	for rows.Next() {
		var (
			customerID sql.NullString
			recipient  sql.NullString
			state      sql.NullString
		)
		if scanErr := rows.Scan(&customerID, &recipient, &state); scanErr != nil {
			return nil, scanErr
		}
		key := customerID.String
		if key == "" {
			key = recipient.String
		}
		if key == "" {
			continue
		}
		if len(members) > 0 {
			if _, ok := members[key]; !ok {
				continue
			}
		}
		st := strings.ToLower(state.String)
		if st == "" {
			st = "unknown"
		}
		bucket, ok := stateUsers[st]
		if !ok {
			bucket = make(map[string]struct{})
			stateUsers[st] = bucket
		}
		bucket[key] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	canonical := []string{"sent", "delivered", "opened", "clicked", "converted", "bounced", "failed", "dropped"}
	out := make([]funnelStep, 0, len(canonical))
	seen := map[string]bool{}
	for _, st := range canonical {
		out = append(out, funnelStep{State: st, UniqueRecipients: len(stateUsers[st])})
		seen[st] = true
	}
	for st, users := range stateUsers {
		if !seen[st] {
			out = append(out, funnelStep{State: st, UniqueRecipients: len(users)})
		}
	}
	return out, nil
}
