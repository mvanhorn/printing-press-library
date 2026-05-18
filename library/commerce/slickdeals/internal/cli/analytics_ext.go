// Copyright 2026 david. Licensed under Apache-2.0. See LICENSE.

// Package cli: analytics_ext.go extends the existing `analytics` command tree
// with two snapshot-aware sub-commands — `top-stores` and `thumbs-velocity`.
// Lives in a separate file from analytics.go because analytics.go is generated
// and read-only; the integration step calls attachAnalyticsExt(parent, flags)
// after newAnalyticsCmd builds the parent to splice these in.
package cli

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/slickdeals/internal/store"

	"github.com/spf13/cobra"
)

// attachAnalyticsExt registers the snapshot-aware sub-commands under the
// analytics parent. Called by root.go after newAnalyticsCmd builds the tree.
func attachAnalyticsExt(parent *cobra.Command, flags *rootFlags) {
	parent.AddCommand(newTopStoresCmd(flags))
	parent.AddCommand(newThumbsVelocityCmd(flags))
}

func newTopStoresCmd(flags *rootFlags) *cobra.Command {
	var (
		window string
		limit  int
		dbPath string
	)

	cmd := &cobra.Command{
		Use:   "top-stores",
		Short: "Rank merchants by deal count and thumb score over a window",
		Long: `Aggregate the local deal_snapshots table by merchant over the
given window, sorted by distinct deal_count desc then max_thumbs desc.

Window accepts the same suffixes as --since: 30d, 24h, 1w, 30m.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  # 7-day store leaderboard
  slickdeals-pp-cli analytics top-stores --window 7d --json

  # All-time top 10
  slickdeals-pp-cli analytics top-stores --window 0 --limit 10 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("slickdeals-pp-cli")
			}

			d, err := parseWindowDuration(window)
			if err != nil {
				return fmt.Errorf("--window: %w", err)
			}

			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			stats, err := db.TopStores(d, limit)
			if err != nil {
				return fmt.Errorf("top stores: %w", err)
			}
			if stats == nil {
				stats = []store.StoreStats{}
			}
			if len(stats) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(),
					"no merchant data in window. run 'slickdeals-pp-cli watch' to populate snapshots first.")
			}

			now := time.Now()
			prov := DataProvenance{
				Source:       "local",
				SyncedAt:     &now,
				ResourceType: "top-stores",
			}
			raw, err := json.Marshal(stats)
			if err != nil {
				return fmt.Errorf("marshaling top stores: %w", err)
			}
			wrapped, err := wrapWithProvenance(raw, prov)
			if err != nil {
				return fmt.Errorf("wrapping provenance: %w", err)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), wrapped, flags)
		},
	}

	cmd.Flags().StringVar(&window, "window", "30d", "Time window (e.g. 7d, 24h, 1w, 30m, or 0 for all time)")
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum merchants to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/slickdeals-pp-cli/data.db)")

	return cmd
}

func newThumbsVelocityCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "thumbs-velocity <deal-id>",
		Short: "Time-series of thumb scores for one deal, with deltas",
		Long: `Return the chronological sequence of thumb observations for a
single deal_id. Each row carries the absolute thumb count plus the delta from
the previous observation (0 on the first point).

Fewer than 2 snapshots is valid — the series is returned anyway with
delta=0 throughout.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.ExactArgs(1),
		Example: `  slickdeals-pp-cli analytics thumbs-velocity 19510173 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("slickdeals-pp-cli")
			}

			dealID := strings.TrimSpace(args[0])
			if dealID == "" {
				return fmt.Errorf("deal-id is required")
			}

			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			pts, err := db.ThumbsVelocity(dealID)
			if err != nil {
				return fmt.Errorf("thumbs velocity: %w", err)
			}
			if pts == nil {
				pts = []store.VelocityPoint{}
			}
			if len(pts) == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"no snapshots for deal_id %q. run 'slickdeals-pp-cli watch' to populate first.\n",
					dealID)
			}

			syncedAt := time.Now()
			if len(pts) > 0 {
				syncedAt = pts[len(pts)-1].CapturedAt
			}
			prov := DataProvenance{
				Source:       "local",
				SyncedAt:     &syncedAt,
				ResourceType: "thumbs-velocity",
			}
			raw, err := json.Marshal(pts)
			if err != nil {
				return fmt.Errorf("marshaling velocity: %w", err)
			}
			wrapped, err := wrapWithProvenance(raw, prov)
			if err != nil {
				return fmt.Errorf("wrapping provenance: %w", err)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), wrapped, flags)
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/slickdeals-pp-cli/data.db)")

	return cmd
}

// windowRE matches the same shapes parseSinceDuration accepts, plus the
// literal "0" which means "all time" for top-stores.
var windowRE = regexp.MustCompile(`^(\d+)([dhwm])?$`)

// parseWindowDuration converts the top-stores --window flag into a
// time.Duration. Accepts "0" or "0d"/"0h"/etc. as "all time" (Duration 0).
// Reuses the same suffix vocabulary as parseSinceDuration so the two flags
// behave consistently.
func parseWindowDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty window")
	}
	if s == "0" {
		return 0, nil
	}
	matches := windowRE.FindStringSubmatch(s)
	if matches == nil {
		return 0, fmt.Errorf("expected format like 7d, 24h, 1w, 30m, or 0")
	}
	n, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, err
	}
	if n == 0 {
		return 0, nil
	}
	unit := matches[2]
	if unit == "" {
		// Bare integer -> treat as days, like "7" == "7d".
		unit = "d"
	}
	switch unit {
	case "d":
		return time.Duration(n) * 24 * time.Hour, nil
	case "h":
		return time.Duration(n) * time.Hour, nil
	case "w":
		return time.Duration(n) * 7 * 24 * time.Hour, nil
	case "m":
		return time.Duration(n) * time.Minute, nil
	default:
		return 0, fmt.Errorf("unknown unit %q", unit)
	}
}
