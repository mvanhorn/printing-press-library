// Copyright 2026 Kent Martin and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: engagement ranking — cross-campaign contact engagement from
// synced recipient actions. Hand-authored; survives regeneration whole.
// pp:data-source auto

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var engagementActions = []string{"sentcontacts", "openedcontacts", "clickedcontacts"}

type engagementRow struct {
	Email     string `json:"email"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
	Company   string `json:"company,omitempty"`
	Received  int    `json:"campaigns_received"`
	Opens     int    `json:"campaigns_opened"`
	Clicks    int    `json:"campaigns_clicked"`
	Score     int    `json:"score"`
}

type engagementView struct {
	Mode             string          `json:"mode"`
	Window           string          `json:"window"`
	Contacts         []engagementRow `json:"contacts"`
	ScannedCampaigns int             `json:"scanned_campaigns"`
	MaxCampaigns     int             `json:"max_campaigns"`
	Note             string          `json:"note,omitempty"`
}

func newNovelEngagementCmd(flags *rootFlags) *cobra.Command {
	var flagTop int
	var flagNeverOpened bool
	var flagSince string
	var maxCampaigns int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "engagement",
		Short: "Rank contacts by engagement across all synced campaigns",
		Long: strings.TrimSpace(`
Use this command for ranked cross-campaign contact engagement (most engaged,
never opened). Do NOT use it for one contact's full history; use 'journey'
instead.

In auto/live mode the command first fetches per-recipient actions (sent,
opened, clicked) for sent campaigns in the window that are not yet cached
locally, bounded by --max-campaigns per run.`),
		Example: strings.Trim(`
  zoho-campaigns-pp-cli engagement --top 20 --agent
  zoho-campaigns-pp-cli engagement --never-opened --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would rank contacts by cross-campaign engagement")
				return nil
			}
			if err := validateDataSourceStrategy(flags, "auto"); err != nil {
				return err
			}
			since, err := parseSinceLoose(flagSince, "180d")
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := openHistoryStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			cutoff := time.Now().UTC().Add(-since)
			view := engagementView{Window: flagSince, MaxCampaigns: maxCampaigns, Mode: "top"}
			if flagNeverOpened {
				view.Mode = "never-opened"
			}
			if view.Window == "" {
				view.Window = "180d"
			}
			var refs []campaignRef
			if flags.dataSource != "local" {
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				refs, err = sentCampaignsSince(ctx, c, db, cutoff, true)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				scanned, err := ensureRecipientActions(ctx, c, db, refs, engagementActions, maxCampaigns)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				view.ScannedCampaigns = scanned
			} else {
				if !hintIfUnsynced(cmd, db, "campaigns") {
					hintIfStale(cmd, db, "campaigns", flags.maxAge)
				}
				refs = localSentCampaigns(ctx, db, cutoff)
			}

			// Restrict the aggregation to campaigns known to be in the window
			// so --since actually bounds the ranking, not just the live fetch.
			windowKeys := inWindowCampaignKeys(ctx, db, refs, cutoff)
			query := `
				SELECT email,
				       MAX(first_name), MAX(last_name), MAX(company),
				       COUNT(DISTINCT CASE WHEN action = 'sentcontacts' THEN campaign_key END),
				       COUNT(DISTINCT CASE WHEN action = 'openedcontacts' THEN campaign_key END),
				       COUNT(DISTINCT CASE WHEN action = 'clickedcontacts' THEN campaign_key END)
				FROM recipient_actions`
			var qargs []any
			if len(windowKeys) > 0 {
				ph, args := sqlInPlaceholders(windowKeys)
				query += ` WHERE campaign_key IN (` + ph + `)`
				qargs = args
			} else {
				view.Note = "window membership unknown (no synced campaigns or snapshots); ranking aggregates all cached recipient data"
			}
			query += ` GROUP BY email`
			rows, err := db.DB().QueryContext(ctx, query, qargs...)
			if err != nil {
				return fmt.Errorf("query recipient actions: %w", err)
			}
			all := make([]engagementRow, 0)
			for rows.Next() {
				var r engagementRow
				if err := rows.Scan(&r.Email, &r.FirstName, &r.LastName, &r.Company, &r.Received, &r.Opens, &r.Clicks); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scan engagement row: %w", err)
				}
				r.Score = r.Opens + 2*r.Clicks
				all = append(all, r)
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return fmt.Errorf("iterate engagement rows: %w", err)
			}
			if err := rows.Close(); err != nil {
				return fmt.Errorf("close engagement rows: %w", err)
			}

			view.Contacts = []engagementRow{}
			if flagNeverOpened {
				for _, r := range all {
					if r.Received > 0 && r.Opens == 0 && r.Clicks == 0 {
						view.Contacts = append(view.Contacts, r)
					}
				}
				sort.Slice(view.Contacts, func(i, j int) bool { return view.Contacts[i].Received > view.Contacts[j].Received })
				if flagTop > 0 && len(view.Contacts) > flagTop {
					view.Contacts = view.Contacts[:flagTop]
				}
			} else {
				sort.Slice(all, func(i, j int) bool {
					if all[i].Score != all[j].Score {
						return all[i].Score > all[j].Score
					}
					return all[i].Email < all[j].Email
				})
				for _, r := range all {
					if r.Score == 0 {
						continue
					}
					view.Contacts = append(view.Contacts, r)
					if flagTop > 0 && len(view.Contacts) >= flagTop {
						break
					}
				}
			}
			if len(all) == 0 {
				view.Note = "no recipient actions cached yet; run without --data-source local so recipient data can be fetched (bounded by --max-campaigns)"
			}

			if flags.asJSON || flags.csv || flags.quiet || flags.plain || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Engagement (%s) — %d contacts\n", view.Mode, len(view.Contacts))
			for _, r := range view.Contacts {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-40s received %3d  opened %3d  clicked %3d  score %3d\n",
					r.Email, r.Received, r.Opens, r.Clicks, r.Score)
			}
			if view.Note != "" {
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&flagTop, "top", 20, "Maximum contacts to return")
	cmd.Flags().BoolVar(&flagNeverOpened, "never-opened", false, "Show contacts who received campaigns but never opened any")
	cmd.Flags().StringVar(&flagSince, "since", "180d", "Window of sent campaigns to consider (e.g. 90d, 180d)")
	cmd.Flags().IntVar(&maxCampaigns, "max-campaigns", 10, "Maximum campaigns to fetch recipient data for per run (rate-limit guard)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
