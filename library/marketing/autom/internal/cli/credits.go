// Copyright 2026 Josselin and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto

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

type creditBalance struct {
	Remaining    *int            `json:"remaining"`
	TotalUsed    *int            `json:"total_used"`
	RenewalDate  string          `json:"renewal_date,omitempty"`
	Subscription json.RawMessage `json:"subscription,omitempty"`
	Packs        json.RawMessage `json:"packs,omitempty"`
	Credits      json.RawMessage `json:"credits,omitempty"`
	RateLimit    json.RawMessage `json:"rate_limit,omitempty"`
}

type creditGroup struct {
	Key     string `json:"key"`
	Calls   int    `json:"calls"`
	Credits int    `json:"credits"`
	Pending int    `json:"pending_async"`
}

type creditsView struct {
	Balance       *creditBalance `json:"balance,omitempty"`
	BalanceError  string         `json:"balance_error,omitempty"`
	Since         string         `json:"since"`
	By            string         `json:"by"`
	Calls         int            `json:"calls"`
	BilledCredits int            `json:"billed_credits"`
	PendingAsync  int            `json:"pending_async"`
	Groups        []creditGroup  `json:"groups"`
	BurnPerDay    float64        `json:"burn_per_day"`
	DaysLeft      *float64       `json:"days_left,omitempty"`
	Note          string         `json:"note"`
}

func newNovelCreditsCmd(flags *rootFlags) *cobra.Command {
	var flagBy, flagSince string
	cmd := &cobra.Command{
		Use:   "credits",
		Short: "See your remaining quota, packs and credits plus where your credits went, by keyword, command or engine",
		Long: `Use this command for remaining balance PLUS where credits went and the burn rate.
The balance comes from Autom's free usage endpoint; the spend comes from a local ledger of
the billed calls this CLI made (other tools using the same key are not in the ledger).
Async searches count as pending: Autom bills them when the job completes.
Do NOT use this command for the raw account and key payload; use 'usage' instead.
Do NOT use it to check the key or connectivity; use 'doctor' instead.`,
		Example: strings.Trim(`
  autom-pp-cli credits --by engine --since 30d --agent
  autom-pp-cli credits --by keyword --since 7d`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "credits")
			}
			by := strings.ToLower(strings.TrimSpace(flagBy))
			switch by {
			case "", "engine":
				by = "engine"
			case "keyword", "query":
				by = "keyword"
			case "command":
			default:
				return usageErr(fmt.Errorf("--by must be engine, keyword or command"))
			}
			window, err := sinceCutoff(flagSince)
			if err != nil {
				return err
			}
			view := creditsView{Since: flagSince, By: by, Groups: make([]creditGroup, 0)}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if flags.dataSource != "local" {
				if c, err := flags.newClient(); err == nil {
					if raw, err := c.GetNoCache(ctx, "/v1/usage", fullResponse(nil)); err == nil {
						var b creditBalance
						if json.Unmarshal(raw, &b) == nil {
							view.Balance = &b
						}
					} else {
						view.BalanceError = firstLine(classifyAPIErrorOnly(err).Error()) // the hint lines are noise in JSON
					}
				} else {
					view.BalanceError = firstLine(err.Error())
				}
				if view.BalanceError != "" {
					flags.agentSource = "local" // only the local ledger answered
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: live balance unavailable, showing the local ledger only: %s\n", firstLine(view.BalanceError))
				}
			}
			all, err := readLedger(time.Time{})
			if err != nil {
				return fmt.Errorf("reading the credit ledger: %w", err)
			}
			// ledgerStart is when the ledger began, so the burn rate divides by the
			// time the ledger actually covers inside the window.
			var ledgerStart time.Time
			entries := make([]ledgerEntry, 0, len(all))
			for _, e := range all {
				if ledgerStart.IsZero() || e.At.Before(ledgerStart) {
					ledgerStart = e.At
				}
				if window.IsZero() || !e.At.Before(window) {
					entries = append(entries, e)
				}
			}
			groups := map[string]*creditGroup{}
			for _, e := range entries {
				view.Calls++
				view.BilledCredits += e.Credits
				if e.Pending {
					view.PendingAsync++
				}
				key := e.Engine
				switch by {
				case "keyword":
					key = e.Query
				case "command":
					key = e.Command
				}
				if key == "" {
					key = "(none)"
				}
				g := groups[key]
				if g == nil {
					g = &creditGroup{Key: key}
					groups[key] = g
				}
				g.Calls++
				g.Credits += e.Credits
				if e.Pending {
					g.Pending++
				}
			}
			for _, g := range groups {
				view.Groups = append(view.Groups, *g)
			}
			sort.Slice(view.Groups, func(i, j int) bool {
				if view.Groups[i].Credits != view.Groups[j].Credits {
					return view.Groups[i].Credits > view.Groups[j].Credits
				}
				return view.Groups[i].Key < view.Groups[j].Key
			})
			// Divide by the time the ledger actually covers: a ledger younger than
			// the window would otherwise understate the burn rate.
			start := window
			if start.IsZero() || ledgerStart.After(start) {
				start = ledgerStart
			}
			if !start.IsZero() && view.BilledCredits > 0 {
				days := math.Max(time.Since(start).Hours()/24, 1)
				view.BurnPerDay = math.Round(float64(view.BilledCredits)/days*100) / 100
				if view.Balance != nil && view.Balance.Remaining != nil && view.BurnPerDay > 0 {
					d := math.Round(float64(*view.Balance.Remaining)/view.BurnPerDay*10) / 10
					view.DaysLeft = &d
				}
			}
			view.Note = "spend covers the billed calls this CLI made with any key since the ledger started; calls from other tools are not included"
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			w := cmd.OutOrStdout()
			if view.Balance != nil && view.Balance.Remaining != nil {
				fmt.Fprintf(w, "Remaining: %d calls", *view.Balance.Remaining)
				if view.Balance.RenewalDate != "" {
					fmt.Fprintf(w, " (renews %s)", view.Balance.RenewalDate)
				}
				fmt.Fprintln(w)
			}
			fmt.Fprintf(w, "Spent by this CLI%s: %d credit(s) over %d call(s)", windowLabel(flagSince), view.BilledCredits, view.Calls)
			if view.PendingAsync > 0 {
				fmt.Fprintf(w, ", %d async job(s) pending billing", view.PendingAsync)
			}
			fmt.Fprintln(w)
			if view.BurnPerDay > 0 {
				fmt.Fprintf(w, "Burn: %.2f credit(s) per day", view.BurnPerDay)
				if view.DaysLeft != nil {
					fmt.Fprintf(w, ", about %.1f day(s) left at this rate", *view.DaysLeft)
				}
				fmt.Fprintln(w)
			}
			if len(view.Groups) == 0 {
				fmt.Fprintln(w, "No calls recorded yet.")
				return nil
			}
			rows := make([]map[string]any, 0, len(view.Groups))
			for _, g := range view.Groups {
				rows = append(rows, map[string]any{by: g.Key, "calls": g.Calls, "credits": g.Credits})
			}
			return printAutoTable(w, rows)
		},
	}
	cmd.Flags().StringVar(&flagBy, "by", "engine", "Group spend by engine, keyword or command")
	cmd.Flags().StringVar(&flagSince, "since", "30d", "Only spend within this window, for example 7d or 30d (empty = all)")
	return cmd
}

func windowLabel(since string) string {
	if strings.TrimSpace(since) == "" {
		return ""
	}
	return " in the last " + since
}
