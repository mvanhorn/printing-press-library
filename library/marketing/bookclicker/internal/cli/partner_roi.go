// Copyright 2026 wmiles81 and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: partner performance against cost.
// pp:data-source local

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type partnerROI struct {
	Partner     string   `json:"partner"`
	Promos      int      `json:"promos"`
	TotalReach  int64    `json:"total_reach"`
	TotalSpend  int64    `json:"total_spend"`
	SwapCount   int      `json:"swap_count"`
	PaidCount   int      `json:"paid_count"`
	ReachPerUSD *float64 `json:"reach_per_usd"`
	LastDate    string   `json:"last_date,omitempty"`
}

type partnerROIResult struct {
	Partners []partnerROI `json:"partners"`
	Count    int          `json:"count"`
	Since    string       `json:"since,omitempty"`
	Note     string       `json:"note,omitempty"`
}

func newNovelPartnerRoiCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath    string
		flagSince string
		flagLimit int
		flagPaid  bool
	)

	cmd := &cobra.Command{
		Use:   "partner-roi",
		Short: "Rank past promo partners by delivered reach against what they cost.",
		Long: "Rank the newsletters you have actually booked by reach per dollar.\n\n" +
			"Needs history: it is thin until several syncs have accumulated. Reads the\n" +
			"local mirror populated by 'reservations pull'.",
		Example:     "  bookclicker-pp-cli partner-roi --since 180d --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "partner-roi")
			}
			var cutoff time.Time
			if flagSince != "" {
				d, err := partnerROIParseSince(flagSince)
				if err != nil {
					return usageErr(err)
				}
				cutoff = time.Now().Add(-d)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			empty := partnerROIResult{Partners: make([]partnerROI, 0), Since: flagSince, Note: reservationsEmptyNote}
			db, handled, err := openMirror(ctx, dbPath, cmd.OutOrStdout(), cmd.ErrOrStderr(), flags, empty)
			if err != nil || handled {
				return err
			}
			defer db.Close()

			all, err := loadReservations(ctx, db, 0)
			if err != nil {
				return err
			}

			agg := map[string]*partnerROI{}
			considered := 0
			for _, r := range all {
				if !cutoff.IsZero() {
					if t, ok := parseLooseDate(r.Date); ok && t.Before(cutoff) {
						continue
					}
				}
				if flagPaid && r.IsSwap {
					continue
				}
				name := strings.TrimSpace(r.ListName)
				if name == "" {
					name = strings.TrimSpace(r.Counterpar)
				}
				if name == "" {
					continue
				}
				considered++
				p, ok := agg[name]
				if !ok {
					p = &partnerROI{Partner: name}
					agg[name] = p
				}
				p.Promos++
				p.TotalReach += r.ListSize
				if r.IsSwap {
					p.SwapCount++
				} else {
					p.PaidCount++
					if r.Price != nil {
						p.TotalSpend += *r.Price
					}
				}
				if r.Date > p.LastDate {
					p.LastDate = r.Date
				}
			}

			out := make([]partnerROI, 0, len(agg))
			for _, p := range agg {
				if p.TotalSpend > 0 {
					v := float64(p.TotalReach) / float64(p.TotalSpend)
					p.ReachPerUSD = &v
				}
				out = append(out, *p)
			}
			sort.SliceStable(out, func(i, j int) bool {
				iv, jv := 0.0, 0.0
				if out[i].ReachPerUSD != nil {
					iv = *out[i].ReachPerUSD
				}
				if out[j].ReachPerUSD != nil {
					jv = *out[j].ReachPerUSD
				}
				if iv != jv {
					return iv > jv
				}
				return out[i].TotalReach > out[j].TotalReach
			})
			if flagLimit > 0 && len(out) > flagLimit {
				out = out[:flagLimit]
			}

			res := partnerROIResult{Partners: out, Count: len(out), Since: flagSince}
			if len(all) == 0 {
				res.Note = reservationsEmptyNote
			} else if considered == 0 {
				res.Note = "no reservations in the selected window"
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), res.Note)
				return nil
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%-38s %6s %10s %8s %12s\n", "PARTNER", "PROMOS", "REACH", "SPEND", "REACH/$")
			for _, p := range out {
				rpd := "-"
				if p.ReachPerUSD != nil {
					rpd = fmt.Sprintf("%.0f", *p.ReachPerUSD)
				}
				fmt.Fprintf(w, "%-38s %6d %10d %8s %12s\n",
					truncPad(p.Partner, 38), p.Promos, p.TotalReach, "$"+fmt.Sprint(p.TotalSpend), rpd)
			}
			fmt.Fprintf(w, "\nSwap promos count toward reach with no spend, so they have no reach/$ figure.\n")
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	cmd.Flags().StringVar(&flagSince, "since", "", "Only promotions since this window (e.g. 90d, 26w)")
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "Maximum partners to return")
	cmd.Flags().BoolVar(&flagPaid, "paid-only", false, "Exclude swaps and rank paid promotions only")
	return cmd
}

// partnerROIParseSince accepts the day/week shorthand agents reach for, which
// time.ParseDuration rejects.
func partnerROIParseSince(s string) (time.Duration, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 0, fmt.Errorf("--since is empty")
	}
	mult := time.Duration(0)
	switch {
	case strings.HasSuffix(s, "d"):
		mult = 24 * time.Hour
		s = strings.TrimSuffix(s, "d")
	case strings.HasSuffix(s, "w"):
		mult = 7 * 24 * time.Hour
		s = strings.TrimSuffix(s, "w")
	default:
		d, err := time.ParseDuration(s)
		if err != nil {
			return 0, fmt.Errorf("--since must look like 90d, 26w, or 720h: %w", err)
		}
		return d, nil
	}
	var n int
	if _, err := fmt.Sscanf(s, "%d", &n); err != nil || n <= 0 {
		return 0, fmt.Errorf("--since must be a positive number followed by d or w")
	}
	return time.Duration(n) * mult, nil
}
