// Copyright 2026 Kent Martin and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: org digest — one-shot rollup of campaigns and lists over a
// window, refreshing report + list-count snapshots on the way (auto mode).
// Hand-authored; survives regeneration whole.
// pp:data-source auto

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/zoho-campaigns/internal/cliutil"

	"github.com/spf13/cobra"
)

type digestCampaignRow struct {
	CampaignKey  string  `json:"campaign_key"`
	CampaignName string  `json:"campaign_name"`
	SentAt       string  `json:"sent_at,omitempty"`
	EmailsSent   int64   `json:"emails_sent"`
	Delivered    int64   `json:"delivered"`
	Opens        int64   `json:"opens"`
	UniqueClicks int64   `json:"unique_clicks"`
	Bounces      int64   `json:"bounces"`
	Unsubscribes int64   `json:"unsubscribes"`
	OpenPercent  float64 `json:"open_percent"`
	ClickPercent float64 `json:"click_percent"`
	OpensChange  *int64  `json:"opens_change_since_prev_snapshot,omitempty"`
}

type digestListRow struct {
	ListKey  string `json:"listkey"`
	ListName string `json:"listname"`
	Contacts int64  `json:"contacts"`
	Unsubs   int64  `json:"unsubs"`
	Bounces  int64  `json:"bounces"`
}

type digestView struct {
	Window          string              `json:"window"`
	CampaignsSent   int                 `json:"campaigns_sent"`
	TotalSent       int64               `json:"total_emails_sent"`
	TotalDelivered  int64               `json:"total_delivered"`
	TotalOpens      int64               `json:"total_opens"`
	TotalClicks     int64               `json:"total_unique_clicks"`
	TotalBounces    int64               `json:"total_bounces"`
	TotalUnsubs     int64               `json:"total_unsubscribes"`
	AvgOpenPercent  float64             `json:"avg_open_percent"`
	AvgClickPercent float64             `json:"avg_click_percent"`
	Campaigns       []digestCampaignRow `json:"campaigns"`
	Lists           []digestListRow     `json:"lists"`
	TotalContacts   int64               `json:"total_contacts"`
	ScannedLive     int                 `json:"scanned_live_campaigns"`
	MaxCampaigns    int                 `json:"max_campaigns"`
	FetchFailures   []string            `json:"fetch_failures,omitempty"`
	Note            string              `json:"note,omitempty"`
}

func newNovelDigestCmd(flags *rootFlags) *cobra.Command {
	var flagSince string
	var maxCampaigns int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "digest",
		Short: "Org rollup for a window: sends, rates, list totals, movers",
		Long: strings.TrimSpace(`
Use this command for an org-wide summary of campaign and list performance over
a window (including 24h windows for daily briefs).
Do NOT use it for a single campaign's change over time; use 'delta' instead.
Do NOT use it for list-size trend lines; use 'growth' instead.

In auto/live mode digest also refreshes report and list-count snapshots in the
local store — these snapshots are what power 'delta' and 'growth'.`),
		Example: strings.Trim(`
  zoho-campaigns-pp-cli digest --since 30d --agent
  zoho-campaigns-pp-cli digest --since 24h --agent
  zoho-campaigns-pp-cli digest --since 90d --data-source local
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would refresh snapshots and compute the org digest")
				return nil
			}
			if err := validateDataSourceStrategy(flags, "auto"); err != nil {
				return err
			}
			since, err := parseSinceLoose(flagSince, "30d")
			if err != nil {
				return err
			}
			if cliutil.IsDogfoodEnv() && maxCampaigns > 2 {
				maxCampaigns = 2
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := openHistoryStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			cutoff := time.Now().UTC().Add(-since)
			view := digestView{Window: flagSince, MaxCampaigns: maxCampaigns}
			if view.Window == "" {
				view.Window = "30d"
			}
			live := flags.dataSource != "local"

			var refs []campaignRef
			if live {
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				lists, err := snapshotListCounts(ctx, c, db)
				if err != nil {
					view.FetchFailures = append(view.FetchFailures, err.Error())
				} else {
					for _, l := range lists {
						view.Lists = append(view.Lists, digestListRow{ListKey: l.ListKey, ListName: l.ListName, Contacts: l.Contacts, Unsubs: l.Unsubs, Bounces: l.Bounces})
						view.TotalContacts += l.Contacts
					}
				}
				refs, err = sentCampaignsSince(ctx, c, db, cutoff, true)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				for i, ref := range refs {
					if i >= maxCampaigns {
						view.Note = fmt.Sprintf("refreshed reports for the %d most recent of %d sent campaigns in the window; raise --max-campaigns to widen (Zoho allows 500 calls/5min)", maxCampaigns, len(refs))
						break
					}
					m, err := fetchReportMetrics(ctx, c, ref.Key)
					if err != nil {
						view.FetchFailures = append(view.FetchFailures, err.Error())
						continue
					}
					if _, err := snapshotReport(ctx, db, m, ref.SentGMT); err != nil {
						return err
					}
					view.ScannedLive++
				}
			} else {
				if !hintIfUnsynced(cmd, db, "campaigns") {
					hintIfStale(cmd, db, "campaigns", flags.maxAge)
				}
				refs = localSentCampaigns(ctx, db, cutoff)
				lrows, err := db.DB().QueryContext(ctx, `
					SELECT l.listkey, l.listname, l.contacts, l.unsubs, l.bounces
					FROM list_count_snapshots l
					JOIN (SELECT listkey, MAX(taken_at) AS mt FROM list_count_snapshots GROUP BY listkey) x
					  ON x.listkey = l.listkey AND x.mt = l.taken_at`)
				if err != nil {
					view.FetchFailures = append(view.FetchFailures, fmt.Sprintf("local list snapshots: %v", err))
				} else {
					for lrows.Next() {
						var r digestListRow
						if err := lrows.Scan(&r.ListKey, &r.ListName, &r.Contacts, &r.Unsubs, &r.Bounces); err != nil {
							_ = lrows.Close()
							return fmt.Errorf("scan list snapshot: %w", err)
						}
						view.Lists = append(view.Lists, r)
						view.TotalContacts += r.Contacts
					}
					if err := lrows.Err(); err != nil {
						_ = lrows.Close()
						return fmt.Errorf("iterate list snapshots: %w", err)
					}
					if err := lrows.Close(); err != nil {
						return fmt.Errorf("close list snapshots: %w", err)
					}
				}
			}

			// Latest + previous snapshot per campaign in the window (drain-first).
			srows, err := db.DB().QueryContext(ctx, `
				SELECT campaign_key, campaign_name, taken_at, emails_sent, delivered, opens,
				       unique_clicks, bounces, unsubscribes, open_percent, click_percent, sent_time
				FROM campaign_report_snapshots ORDER BY campaign_key, taken_at ASC`)
			if err != nil {
				return fmt.Errorf("query snapshots: %w", err)
			}
			type snapRow struct {
				row    digestCampaignRow
				sentAt time.Time
			}
			latest := map[string]snapRow{}
			prevOpens := map[string]int64{}
			for srows.Next() {
				var r digestCampaignRow
				var takenAt, sentTime string
				if err := srows.Scan(&r.CampaignKey, &r.CampaignName, &takenAt, &r.EmailsSent, &r.Delivered,
					&r.Opens, &r.UniqueClicks, &r.Bounces, &r.Unsubscribes, &r.OpenPercent, &r.ClickPercent, &sentTime); err != nil {
					_ = srows.Close()
					return fmt.Errorf("scan snapshot: %w", err)
				}
				if old, ok := latest[r.CampaignKey]; ok {
					prevOpens[r.CampaignKey] = old.row.Opens
				}
				st := parseEpochMillis(sentTime)
				r.SentAt = ""
				if !st.IsZero() {
					r.SentAt = st.Format(historyTimeFormat)
				}
				latest[r.CampaignKey] = snapRow{row: r, sentAt: st}
			}
			if err := srows.Err(); err != nil {
				_ = srows.Close()
				return fmt.Errorf("iterate snapshots: %w", err)
			}
			if err := srows.Close(); err != nil {
				return fmt.Errorf("close snapshots: %w", err)
			}

			// Restrict to campaigns sent in the window. When sent_time is
			// unknown, include only keys that are in the window's ref set.
			inWindow := map[string]bool{}
			for _, ref := range refs {
				inWindow[ref.Key] = true
			}
			var sumOpenPct, sumClickPct float64
			for key, sr := range latest {
				if !sr.sentAt.IsZero() {
					if sr.sentAt.Before(cutoff) {
						continue
					}
				} else if !inWindow[key] {
					continue
				}
				row := sr.row
				if po, ok := prevOpens[key]; ok {
					change := row.Opens - po
					row.OpensChange = &change
				}
				view.Campaigns = append(view.Campaigns, row)
				view.TotalSent += row.EmailsSent
				view.TotalDelivered += row.Delivered
				view.TotalOpens += row.Opens
				view.TotalClicks += row.UniqueClicks
				view.TotalBounces += row.Bounces
				view.TotalUnsubs += row.Unsubscribes
				sumOpenPct += row.OpenPercent
				sumClickPct += row.ClickPercent
			}
			view.CampaignsSent = len(view.Campaigns)
			if view.CampaignsSent > 0 {
				view.AvgOpenPercent = sumOpenPct / float64(view.CampaignsSent)
				view.AvgClickPercent = sumClickPct / float64(view.CampaignsSent)
			}
			sort.Slice(view.Campaigns, func(i, j int) bool { return view.Campaigns[i].SentAt > view.Campaigns[j].SentAt })
			if view.Campaigns == nil {
				view.Campaigns = []digestCampaignRow{}
			}
			if view.Lists == nil {
				view.Lists = []digestListRow{}
			}
			if len(view.FetchFailures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d fetches failed; digest computed over the remaining data\n", len(view.FetchFailures))
			}
			if view.CampaignsSent == 0 && view.Note == "" {
				view.Note = "no sent campaigns with report snapshots in the window; run without --data-source local (or widen --since) to refresh"
			}

			if flags.asJSON || flags.csv || flags.quiet || flags.plain || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Digest — last %s: %d campaigns sent, %d emails, %d delivered\n",
				view.Window, view.CampaignsSent, view.TotalSent, view.TotalDelivered)
			fmt.Fprintf(cmd.OutOrStdout(), "Opens %d (avg %.1f%%) · Clicks %d (avg %.1f%%) · Bounces %d · Unsubs %d\n",
				view.TotalOpens, view.AvgOpenPercent, view.TotalClicks, view.AvgClickPercent, view.TotalBounces, view.TotalUnsubs)
			for _, cr := range view.Campaigns {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-45s opens %5d (%.1f%%)  clicks %4d  bounces %3d  unsubs %2d\n",
					truncateName(cr.CampaignName, 45), cr.Opens, cr.OpenPercent, cr.UniqueClicks, cr.Bounces, cr.Unsubscribes)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Lists: %d total contacts\n", view.TotalContacts)
			for _, lr := range view.Lists {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-30s contacts %6d  unsubs %4d  bounces %4d\n", truncateName(lr.ListName, 30), lr.Contacts, lr.Unsubs, lr.Bounces)
			}
			if view.Note != "" {
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "30d", "Window to roll up (e.g. 24h, 7d, 30d)")
	cmd.Flags().IntVar(&maxCampaigns, "max-campaigns", 15, "Maximum campaigns to refresh live per run (rate-limit guard)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func truncateName(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
