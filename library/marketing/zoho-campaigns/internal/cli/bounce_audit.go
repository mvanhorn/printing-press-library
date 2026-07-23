// Copyright 2026 Kent Martin and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: bounce audit — cross-campaign bounced contacts, cleanup
// candidates pipeable into 'contacts do-not-mail'. Hand-authored; survives
// regeneration whole.
// pp:data-source auto

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var bounceActions = []string{"senthardbounce", "sentsoftbounce"}

type bounceRow struct {
	Email        string `json:"email"`
	FirstName    string `json:"first_name,omitempty"`
	LastName     string `json:"last_name,omitempty"`
	Company      string `json:"company,omitempty"`
	HardBounces  int    `json:"hard_bounce_campaigns"`
	SoftBounces  int    `json:"soft_bounce_campaigns"`
	LastCampaign string `json:"last_campaign,omitempty"`
}

type bounceAuditView struct {
	Window           string      `json:"window"`
	Contacts         []bounceRow `json:"contacts"`
	ScannedCampaigns int         `json:"scanned_campaigns"`
	MaxCampaigns     int         `json:"max_campaigns"`
	Note             string      `json:"note,omitempty"`
}

func newNovelBounceAuditCmd(flags *rootFlags) *cobra.Command {
	var flagSince string
	var flagHardOnly bool
	var maxCampaigns int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "bounce-audit",
		Short: "Bounced contacts across campaigns — deliverability cleanup",
		Long: strings.TrimSpace(`
Use this command to find bounced contacts and deliverability cleanup
candidates. Do NOT use it for engagement ranking of healthy contacts; use
'engagement' instead.

Hard-bouncing contacts are prime candidates for
'zoho-campaigns-pp-cli contacts do-not-mail'. In auto/live mode the command
first fetches bounce data for sent campaigns in the window not yet cached,
bounded by --max-campaigns per run.`),
		Example: strings.Trim(`
  zoho-campaigns-pp-cli bounce-audit --since 90d --csv
  zoho-campaigns-pp-cli bounce-audit --hard-only --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would audit bounced contacts across synced campaigns")
				return nil
			}
			if err := validateDataSourceStrategy(flags, "auto"); err != nil {
				return err
			}
			since, err := parseSinceLoose(flagSince, "90d")
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
			view := bounceAuditView{Window: flagSince, MaxCampaigns: maxCampaigns}
			if view.Window == "" {
				view.Window = "90d"
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
				scanned, err := ensureRecipientActions(ctx, c, db, refs, bounceActions, maxCampaigns)
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

			// Restrict aggregation to in-window campaigns so --since actually
			// bounds the audit, not just the live fetch.
			windowKeys := inWindowCampaignKeys(ctx, db, refs, cutoff)
			query := `
				SELECT email, first_name, last_name, company, campaign_key, action
				FROM recipient_actions
				WHERE action IN ('senthardbounce','sentsoftbounce')`
			var qargs []any
			if len(windowKeys) > 0 {
				ph, args := sqlInPlaceholders(windowKeys)
				query += ` AND campaign_key IN (` + ph + `)`
				qargs = args
			} else {
				view.Note = "window membership unknown (no synced campaigns or snapshots); auditing all cached bounce data"
			}
			// Drain-first: collect raw rows, then aggregate and resolve names.
			rows, err := db.DB().QueryContext(ctx, query, qargs...)
			if err != nil {
				return fmt.Errorf("query bounce actions: %w", err)
			}
			type rawRow struct {
				email, first, last, company, key, action string
			}
			raws := make([]rawRow, 0)
			for rows.Next() {
				var r rawRow
				if err := rows.Scan(&r.email, &r.first, &r.last, &r.company, &r.key, &r.action); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scan bounce row: %w", err)
				}
				raws = append(raws, r)
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return fmt.Errorf("iterate bounce rows: %w", err)
			}
			if err := rows.Close(); err != nil {
				return fmt.Errorf("close bounce rows: %w", err)
			}

			// Resolve each campaign's name and sent time once.
			type campMeta struct {
				name string
				sent time.Time
			}
			meta := map[string]campMeta{}
			for _, r := range raws {
				if _, ok := meta[r.key]; ok {
					continue
				}
				name, sentMS := campaignNameSentFromLocal(ctx, db, r.key)
				meta[r.key] = campMeta{name: name, sent: parseEpochMillis(sentMS)}
			}

			type agg struct {
				row      bounceRow
				hardKeys map[string]bool
				softKeys map[string]bool
				lastKey  string
			}
			byEmail := map[string]*agg{}
			order := []string{}
			for _, r := range raws {
				a, ok := byEmail[r.email]
				if !ok {
					a = &agg{
						row:      bounceRow{Email: r.email, FirstName: r.first, LastName: r.last, Company: r.company},
						hardKeys: map[string]bool{}, softKeys: map[string]bool{},
					}
					byEmail[r.email] = a
					order = append(order, r.email)
				}
				if r.action == "senthardbounce" {
					a.hardKeys[r.key] = true
				} else {
					a.softKeys[r.key] = true
				}
				// Track the genuinely latest campaign by sent time; unknown
				// sent times only win when nothing better is known.
				if a.lastKey == "" || meta[r.key].sent.After(meta[a.lastKey].sent) {
					a.lastKey = r.key
				}
			}

			view.Contacts = []bounceRow{}
			for _, email := range order {
				a := byEmail[email]
				a.row.HardBounces = len(a.hardKeys)
				a.row.SoftBounces = len(a.softKeys)
				if flagHardOnly && a.row.HardBounces == 0 {
					continue
				}
				a.row.LastCampaign = meta[a.lastKey].name
				view.Contacts = append(view.Contacts, a.row)
			}
			sort.Slice(view.Contacts, func(i, j int) bool {
				if view.Contacts[i].HardBounces != view.Contacts[j].HardBounces {
					return view.Contacts[i].HardBounces > view.Contacts[j].HardBounces
				}
				return view.Contacts[i].SoftBounces > view.Contacts[j].SoftBounces
			})
			extra := "clean up hard bouncers with: zoho-campaigns-pp-cli contacts do-not-mail --contactinfo '{\"Contact Email\":\"<email>\"}'"
			if len(view.Contacts) == 0 {
				extra = "no bounced contacts cached; run without --data-source local so bounce data can be fetched (bounded by --max-campaigns), or the org genuinely has no bounces"
			}
			if view.Note == "" {
				view.Note = extra
			} else {
				view.Note += "; " + extra
			}

			if flags.asJSON || flags.csv || flags.quiet || flags.plain || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Bounce audit — %d contacts\n", len(view.Contacts))
			for _, r := range view.Contacts {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-40s hard %2d  soft %2d  last: %s\n",
					r.Email, r.HardBounces, r.SoftBounces, truncateName(r.LastCampaign, 40))
			}
			if view.Note != "" {
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "90d", "Window of sent campaigns to consider (e.g. 30d, 90d)")
	cmd.Flags().BoolVar(&flagHardOnly, "hard-only", false, "Only contacts with hard bounces")
	cmd.Flags().IntVar(&maxCampaigns, "max-campaigns", 10, "Maximum campaigns to fetch bounce data for per run (rate-limit guard)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
