// Copyright 2026 Kent Martin and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: list growth — per-list subscriber/unsub/bounce trends from
// local count snapshots. Hand-authored; survives regeneration whole.
// pp:data-source local

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type growthPoint struct {
	TakenAt  string `json:"taken_at"`
	Contacts int64  `json:"contacts"`
	Unsubs   int64  `json:"unsubs"`
	Bounces  int64  `json:"bounces"`
}

type growthListView struct {
	ListKey       string        `json:"listkey"`
	ListName      string        `json:"listname"`
	Snapshots     int           `json:"snapshots_in_window"`
	NetContacts   int64         `json:"net_contacts_change"`
	NetUnsubs     int64         `json:"net_unsubs_change"`
	NetBounces    int64         `json:"net_bounces_change"`
	Trend         string        `json:"trend"`
	FirstSnapshot string        `json:"first_snapshot"`
	LastSnapshot  string        `json:"last_snapshot"`
	Contacts      int64         `json:"current_contacts"`
	Points        []growthPoint `json:"points,omitempty"`
}

type growthView struct {
	Window string           `json:"window"`
	Lists  []growthListView `json:"lists"`
	Note   string           `json:"note,omitempty"`
}

func newNovelGrowthCmd(flags *rootFlags) *cobra.Command {
	var flagSince string
	var flagList string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "growth",
		Short: "Mailing-list size and health trends from local snapshots",
		Long: strings.TrimSpace(`
Use this command for mailing-list size and health trends over time.
Do NOT use it for campaign open/click performance; use 'digest' instead.

Count snapshots are written whenever 'digest' runs in auto/live mode; at
least two snapshots inside the window are needed for a trend.`),
		Example: strings.Trim(`
  zoho-campaigns-pp-cli growth --since 90d --agent
  zoho-campaigns-pp-cli growth --list 3z4800a2141d15c8aa0c --since 30d
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compute list growth trends from local snapshots")
				return nil
			}
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return err
			}
			since, err := parseSinceLoose(flagSince, "90d")
			if err != nil {
				return err
			}
			resolvedPath, exists := historyDBExists(dbPath)
			if !exists {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: zoho-campaigns-pp-cli digest   (writes list-count snapshots)\n", resolvedPath)
				empty := growthView{Window: flagSince, Lists: []growthListView{}, Note: "no local mirror yet; run 'zoho-campaigns-pp-cli digest' to capture list-count snapshots"}
				return printJSONFiltered(cmd.OutOrStdout(), empty, flags)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := openHistoryStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			if !hintIfUnsynced(cmd, db, "contacts") {
				hintIfStale(cmd, db, "contacts", flags.maxAge)
			}

			cutoff := time.Now().UTC().Add(-since).Format(historyTimeFormat)
			query := `
				SELECT listkey, listname, taken_at, contacts, unsubs, bounces
				FROM list_count_snapshots WHERE taken_at >= ?`
			qargs := []any{cutoff}
			if flagList != "" {
				query += ` AND listkey = ?`
				qargs = append(qargs, flagList)
			}
			query += ` ORDER BY listkey, taken_at ASC`
			rows, err := db.DB().QueryContext(ctx, query, qargs...)
			if err != nil {
				return fmt.Errorf("query list snapshots: %w", err)
			}
			type rec struct {
				key, name string
				p         growthPoint
			}
			recs := make([]rec, 0)
			for rows.Next() {
				var r rec
				if err := rows.Scan(&r.key, &r.name, &r.p.TakenAt, &r.p.Contacts, &r.p.Unsubs, &r.p.Bounces); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scan list snapshot: %w", err)
				}
				recs = append(recs, r)
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return fmt.Errorf("iterate list snapshots: %w", err)
			}
			if err := rows.Close(); err != nil {
				return fmt.Errorf("close list snapshots: %w", err)
			}

			byList := map[string]*growthListView{}
			order := []string{}
			for _, r := range recs {
				lv, ok := byList[r.key]
				if !ok {
					lv = &growthListView{ListKey: r.key, ListName: r.name}
					byList[r.key] = lv
					order = append(order, r.key)
				}
				lv.Points = append(lv.Points, r.p)
			}
			view := growthView{Window: flagSince, Lists: []growthListView{}}
			if view.Window == "" {
				view.Window = "90d"
			}
			for _, key := range order {
				lv := byList[key]
				lv.Snapshots = len(lv.Points)
				first, last := lv.Points[0], lv.Points[len(lv.Points)-1]
				lv.FirstSnapshot, lv.LastSnapshot = first.TakenAt, last.TakenAt
				lv.Contacts = last.Contacts
				lv.NetContacts = last.Contacts - first.Contacts
				lv.NetUnsubs = last.Unsubs - first.Unsubs
				lv.NetBounces = last.Bounces - first.Bounces
				switch {
				case lv.Snapshots < 2:
					lv.Trend = "insufficient-history"
				case lv.NetContacts > 0:
					lv.Trend = "growing"
				case lv.NetContacts < 0:
					lv.Trend = "shrinking"
				default:
					lv.Trend = "flat"
				}
				if !flags.asJSON && !flags.agent {
					lv.Points = nil // keep human output compact; JSON carries the series
				}
				view.Lists = append(view.Lists, *lv)
			}
			if len(view.Lists) == 0 {
				view.Note = "no list-count snapshots in the window; run 'zoho-campaigns-pp-cli digest' to capture one, then again later for a trend"
			}

			if flags.asJSON || flags.csv || flags.quiet || flags.plain || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "List growth — last %s\n", view.Window)
			for _, lv := range view.Lists {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-30s %-10s contacts %6d (%+d)  unsubs %+d  bounces %+d  [%d snapshots]\n",
					truncateName(lv.ListName, 30), lv.Trend, lv.Contacts, lv.NetContacts, lv.NetUnsubs, lv.NetBounces, lv.Snapshots)
			}
			if view.Note != "" {
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "90d", "Window of snapshots to trend (e.g. 30d, 90d)")
	cmd.Flags().StringVar(&flagList, "list", "", "Restrict to one listkey")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
