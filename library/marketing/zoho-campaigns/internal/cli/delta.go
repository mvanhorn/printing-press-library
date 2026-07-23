// Copyright 2026 Kent Martin and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: report delta — diff a campaign's report metrics between
// local snapshots. Hand-authored; survives regeneration whole.
// pp:data-source local

package cli

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type deltaMetricRow struct {
	Metric    string  `json:"metric"`
	From      int64   `json:"from"`
	To        int64   `json:"to"`
	Change    int64   `json:"change"`
	PctChange float64 `json:"pct_change"`
}

type deltaView struct {
	CampaignKey  string           `json:"campaign_key"`
	CampaignName string           `json:"campaign_name"`
	FromSnapshot string           `json:"from_snapshot"`
	ToSnapshot   string           `json:"to_snapshot"`
	Snapshots    int              `json:"snapshots_in_window"`
	Metrics      []deltaMetricRow `json:"metrics"`
	Note         string           `json:"note,omitempty"`
}

func newNovelDeltaCmd(flags *rootFlags) *cobra.Command {
	var flagSince string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "delta <campaignkey>",
		Short: "How a campaign's metrics changed between local snapshots",
		Long: strings.TrimSpace(`
Use this command to see how one campaign's metrics changed between snapshots.
Do NOT use it for org-wide rollups across campaigns; use 'digest' instead.

Snapshots are written whenever 'digest' runs; at least two snapshots inside
the window are needed for a diff.`),
		Example: strings.Trim(`
  zoho-campaigns-pp-cli delta 3z44ba67f3e0a1bfdac6 --since 7d
  zoho-campaigns-pp-cli delta 3z44ba67f3e0a1bfdac6 --since 30d --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would diff local report snapshots")
				return nil
			}
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return err
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("campaignkey argument is required"))
			}
			key := strings.TrimSpace(args[0])
			since, err := parseSinceLoose(flagSince, "7d")
			if err != nil {
				return err
			}
			resolvedPath, exists := historyDBExists(dbPath)
			if !exists {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: zoho-campaigns-pp-cli digest   (writes report snapshots)\n", resolvedPath)
				empty := deltaView{CampaignKey: key, Metrics: []deltaMetricRow{}, Note: "no local mirror yet; run 'zoho-campaigns-pp-cli digest' to capture report snapshots"}
				return printJSONFiltered(cmd.OutOrStdout(), empty, flags)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := openHistoryStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			if !hintIfUnsynced(cmd, db, "campaigns") {
				hintIfStale(cmd, db, "campaigns", flags.maxAge)
			}

			cutoff := time.Now().UTC().Add(-since).Format(historyTimeFormat)
			rows, err := db.DB().QueryContext(ctx, `
				SELECT campaign_name, taken_at, emails_sent, delivered, opens, unique_clicks,
				       bounces, unsubscribes, spams
				FROM campaign_report_snapshots
				WHERE campaign_key = ? AND taken_at >= ?
				ORDER BY taken_at ASC`, key, cutoff)
			if err != nil {
				return fmt.Errorf("query snapshots: %w", err)
			}
			type snap struct {
				name    string
				takenAt string
				vals    map[string]int64
			}
			snaps := make([]snap, 0)
			for rows.Next() {
				var s snap
				var name sql.NullString
				var sent, delivered, opens, clicks, bounces, unsubs, spams int64
				if err := rows.Scan(&name, &s.takenAt, &sent, &delivered, &opens, &clicks, &bounces, &unsubs, &spams); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scan snapshot: %w", err)
				}
				s.name = name.String
				s.vals = map[string]int64{
					"emails_sent": sent, "delivered": delivered, "opens": opens,
					"unique_clicks": clicks, "bounces": bounces, "unsubscribes": unsubs, "spams": spams,
				}
				snaps = append(snaps, s)
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return fmt.Errorf("iterate snapshots: %w", err)
			}
			if err := rows.Close(); err != nil {
				return fmt.Errorf("close snapshots: %w", err)
			}

			view := deltaView{CampaignKey: key, Snapshots: len(snaps), Metrics: []deltaMetricRow{}}
			if len(snaps) == 0 {
				view.Note = "no snapshots for this campaign in the window; run 'zoho-campaigns-pp-cli digest' to capture one, then again later to diff"
			} else if len(snaps) == 1 {
				view.CampaignName = snaps[0].name
				view.FromSnapshot = snaps[0].takenAt
				view.ToSnapshot = snaps[0].takenAt
				view.Note = "one snapshot so far — run 'zoho-campaigns-pp-cli digest' again later to capture a second point for the diff"
			} else {
				first, last := snaps[0], snaps[len(snaps)-1]
				view.CampaignName = last.name
				view.FromSnapshot = first.takenAt
				view.ToSnapshot = last.takenAt
				order := []string{"emails_sent", "delivered", "opens", "unique_clicks", "bounces", "unsubscribes", "spams"}
				for _, m := range order {
					from, to := first.vals[m], last.vals[m]
					row := deltaMetricRow{Metric: m, From: from, To: to, Change: to - from}
					if from != 0 {
						row.PctChange = float64(to-from) / float64(from) * 100
					}
					view.Metrics = append(view.Metrics, row)
				}
			}

			if flags.asJSON || flags.csv || flags.quiet || flags.plain || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s (%s)\n", view.CampaignName, view.CampaignKey)
			if view.Note != "" {
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
			}
			for _, m := range view.Metrics {
				fmt.Fprintf(cmd.OutOrStdout(), "%-14s %8d -> %8d  (%+d, %+.1f%%)\n", m.Metric, m.From, m.To, m.Change, m.PctChange)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "7d", "Window of snapshots to diff (e.g. 24h, 7d, 4w)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
