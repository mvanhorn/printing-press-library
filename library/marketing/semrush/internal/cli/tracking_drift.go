// Copyright 2026 Charles Garrison and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Novel feature #8 — tracking drift.

package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newTrackingDriftCmd(flags *rootFlags) *cobra.Command {
	var since string
	var minDelta int
	var region string
	var device string
	var limit int

	cmd := &cobra.Command{
		Use:         "drift [project-id]",
		Short:       "Window-function over Position Tracking snapshots: keywords that moved by >= --min-delta positions.",
		Long:        "drift groups tracking_position rows by (phrase, region, device), compares the latest snapshot to the prior one, and emits movers whose absolute position delta meets the threshold.",
		Example:     "  semrush-pp-cli tracking drift 12345 --since 30d --min-delta 3 --device desktop",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			db, err := openNovelStore(ctx)
			if err != nil {
				return err
			}
			defer db.Close()

			recordBalanceSnapshotForCmd(ctx, db, flags, cmd.CommandPath(), cmd.ErrOrStderr())

			if !hintIfUnsynced(cmd, db, "tracking") {
				hintIfStale(cmd, db, "tracking", flags.maxAge)
			}

			projectID := args[0]
			window, err := parseSince(since)
			if err != nil {
				return usageErr(err)
			}
			cutoff := time.Now().Add(-window)

			rows, err := db.DB().QueryContext(ctx,
				`SELECT COALESCE(json_extract(data, '$.keyword'), json_extract(data, '$.phrase'), '') AS phrase,
				        COALESCE(json_extract(data, '$.position'), -1) AS position,
				        COALESCE(json_extract(data, '$.region'), json_extract(data, '$.country'), '') AS region,
				        COALESCE(json_extract(data, '$.device'), '') AS device,
				        synced_at,
				        COALESCE(json_extract(data, '$.date'), '') AS snap_date
				 FROM resources
				 WHERE resource_type IN ('tracking', 'tracking_position', 'tracking_positions', 'tracking_organic_positions', 'tracking_paid_positions')
				   AND (json_extract(data, '$.project_id') = ? OR json_extract(data, '$.project_id') = CAST(? AS INTEGER) OR json_extract(data, '$.campaign_id') = ?)
				 ORDER BY synced_at DESC`,
				projectID, projectID, projectID)
			if err != nil {
				return fmt.Errorf("query tracking_positions: %w", err)
			}
			defer rows.Close()

			type snap struct {
				phrase   string
				position float64
				region   string
				device   string
				when     time.Time
			}
			byKey := map[string][]snap{}
			for rows.Next() {
				var s snap
				var when time.Time
				var snapDate string
				if err := rows.Scan(&s.phrase, &s.position, &s.region, &s.device, &when, &snapDate); err != nil {
					return fmt.Errorf("scan tracking row: %w", err)
				}
				if strings.TrimSpace(s.phrase) == "" || s.position < 0 {
					continue
				}
				if region != "" && !strings.EqualFold(s.region, region) {
					continue
				}
				if device != "" && !strings.EqualFold(s.device, device) {
					continue
				}
				if t, ok := parseFlexibleTime(snapDate); ok {
					s.when = t
				} else {
					s.when = when
				}
				if s.when.Before(cutoff) {
					continue
				}
				key := s.phrase + "\x1f" + s.region + "\x1f" + s.device
				byKey[key] = append(byKey[key], s)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterate tracking rows: %w", err)
			}

			type mover struct {
				Phrase         string  `json:"phrase"`
				Region         string  `json:"region"`
				Device         string  `json:"device"`
				LatestPosition float64 `json:"latest_position"`
				PriorPosition  float64 `json:"prior_position"`
				DeltaPosition  float64 `json:"delta_position"`
				CrossedTop3    bool    `json:"crossed_top3"`
				CrossedPage1   bool    `json:"crossed_page1"`
				Direction      string  `json:"direction"`
			}
			var movers []mover
			for _, snaps := range byKey {
				if len(snaps) < 2 {
					continue
				}
				sort.SliceStable(snaps, func(i, j int) bool { return snaps[i].when.After(snaps[j].when) })
				latest := snaps[0]
				prior := snaps[1]
				delta := latest.position - prior.position
				if math.Abs(delta) < float64(minDelta) {
					continue
				}
				m := mover{
					Phrase:         latest.phrase,
					Region:         latest.region,
					Device:         latest.device,
					LatestPosition: latest.position,
					PriorPosition:  prior.position,
					DeltaPosition:  delta,
				}
				if (prior.position > 3 && latest.position <= 3) || (prior.position <= 3 && latest.position > 3) {
					m.CrossedTop3 = true
				}
				if (prior.position > 10 && latest.position <= 10) || (prior.position <= 10 && latest.position > 10) {
					m.CrossedPage1 = true
				}
				if delta < 0 {
					m.Direction = "up"
				} else {
					m.Direction = "down"
				}
				movers = append(movers, m)
			}
			sort.SliceStable(movers, func(i, j int) bool { return math.Abs(movers[i].DeltaPosition) > math.Abs(movers[j].DeltaPosition) })
			// Capture pre-truncation count so the response distinguishes
			// "exactly N movers exist" from "N+ movers exist, --limit capped
			// the response." Without this, an agent reading mover_count=200
			// (the default --limit) can't tell whether to paginate or
			// follow up to see the rest.
			totalMoverCount := len(movers)
			truncated := false
			if limit > 0 && len(movers) > limit {
				movers = movers[:limit]
				truncated = true
			}

			out := map[string]any{
				"project_id":        projectID,
				"since":             since,
				"min_delta":         minDelta,
				"region":            region,
				"device":            device,
				"mover_count":       totalMoverCount,
				"mover_count_shown": len(movers),
				"truncated":         truncated,
				"movers":            movers,
			}
			raw, err := json.Marshal(out)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
		},
	}
	cmd.Flags().StringVar(&since, "since", "30d", "Window to inspect (e.g. 7d, 30d, 12w)")
	cmd.Flags().IntVar(&minDelta, "min-delta", 3, "Minimum absolute position delta to report")
	cmd.Flags().StringVar(&region, "region", "", "Filter to a single region/country code")
	cmd.Flags().StringVar(&device, "device", "", "Filter to desktop | mobile | tablet")
	cmd.Flags().IntVar(&limit, "limit", 200, "Maximum movers to return (0 disables)")
	return cmd
}
