// Copyright 2026 Kent Martin and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: contact journey — one contact's chronological history across
// every synced campaign. Hand-authored; survives regeneration whole.
// pp:data-source local

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type journeyEvent struct {
	CampaignKey  string   `json:"campaign_key"`
	CampaignName string   `json:"campaign_name,omitempty"`
	SentAt       string   `json:"sent_at,omitempty"`
	Actions      []string `json:"actions"`
}

type journeyView struct {
	Email  string         `json:"email"`
	Events []journeyEvent `json:"events"`
	Note   string         `json:"note,omitempty"`
}

func newNovelJourneyCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "journey <email>",
		Short: "One contact's chronological history across all campaigns",
		Long: strings.TrimSpace(`
Use this command for one contact's chronological history across all campaigns.
Do NOT use it to rank or list many contacts; use 'engagement' instead.

Reads recipient actions cached by 'engagement' and 'bounce-audit'; run either
of those first to populate the local store.`),
		Example: strings.Trim(`
  zoho-campaigns-pp-cli journey ola.nordmann@example.com --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would show the contact's cross-campaign history")
				return nil
			}
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return err
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("email argument is required"))
			}
			email := strings.ToLower(strings.TrimSpace(args[0]))
			resolvedPath, exists := historyDBExists(dbPath)
			if !exists {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: zoho-campaigns-pp-cli engagement   (caches recipient actions)\n", resolvedPath)
				empty := journeyView{Email: email, Events: []journeyEvent{}, Note: "no local mirror yet; run 'zoho-campaigns-pp-cli engagement' to cache recipient actions"}
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

			// Drain-first: collect actions per campaign, then resolve
			// campaign names/sent times in follow-up queries.
			rows, err := db.DB().QueryContext(ctx, `
				SELECT campaign_key, action FROM recipient_actions
				WHERE email = ? ORDER BY campaign_key`, email)
			if err != nil {
				return fmt.Errorf("query journey: %w", err)
			}
			byCampaign := map[string][]string{}
			order := []string{}
			for rows.Next() {
				var key, action string
				if err := rows.Scan(&key, &action); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scan journey row: %w", err)
				}
				if _, ok := byCampaign[key]; !ok {
					order = append(order, key)
				}
				byCampaign[key] = append(byCampaign[key], action)
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return fmt.Errorf("iterate journey rows: %w", err)
			}
			if err := rows.Close(); err != nil {
				return fmt.Errorf("close journey rows: %w", err)
			}

			view := journeyView{Email: email, Events: []journeyEvent{}}
			for _, key := range order {
				ev := journeyEvent{CampaignKey: key, Actions: byCampaign[key]}
				sort.Strings(ev.Actions)
				name, sentTime := campaignNameSentFromLocal(ctx, db, key)
				ev.CampaignName = name
				if st := parseEpochMillis(sentTime); !st.IsZero() {
					ev.SentAt = st.Format(historyTimeFormat)
				}
				view.Events = append(view.Events, ev)
			}
			sort.Slice(view.Events, func(i, j int) bool { return view.Events[i].SentAt < view.Events[j].SentAt })
			if len(view.Events) == 0 {
				view.Note = "no cached actions for this contact; run 'zoho-campaigns-pp-cli engagement' or 'bounce-audit' first to fetch recipient data, or the contact received no synced campaigns"
			}

			if flags.asJSON || flags.csv || flags.quiet || flags.plain || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Journey — %s (%d campaigns)\n", view.Email, len(view.Events))
			for _, ev := range view.Events {
				when := ev.SentAt
				if when == "" {
					when = "unknown date"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %-24s %-45s %s\n", when, truncateName(ev.CampaignName, 45), strings.Join(ev.Actions, ", "))
			}
			if view.Note != "" {
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
