// Copyright 2026 wmiles81 and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: swap reliability ledger.
// pp:data-source local

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// swapPartner aggregates one counterparty's swap record.
//
// Bookclicker creates both halves of a swap at booking time and links them via
// swap_reservation_id, so a classic "who owes whom" ledger has nothing to
// measure: in practice every live swap is already paired. What varies, and what
// costs a launch slot, is whether an agreed swap actually ran.
type swapPartner struct {
	Partner     string  `json:"partner"`
	Booked      int     `json:"booked"`
	Delivered   int     `json:"delivered"`
	Cancelled   int     `json:"cancelled"`
	Declined    int     `json:"declined"`
	Unpaired    int     `json:"unpaired"`
	Reliability float64 `json:"reliability"`
	LastDate    string  `json:"last_date,omitempty"`
}

type swapBalanceResult struct {
	Partners     []swapPartner `json:"partners"`
	Count        int           `json:"count"`
	TotalSwaps   int           `json:"total_swaps"`
	PairedSwaps  int           `json:"paired_swaps"`
	Note         string        `json:"note,omitempty"`
}

// swapDelivered marks the statuses that mean the promotion actually ran.
var swapDelivered = map[string]bool{"sent": true, "swapped": true, "paid": true}

func newNovelSwapBalanceCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath    string
		flagFlaky bool
		flagLimit int
		flagMin   int
	)

	cmd := &cobra.Command{
		Use:   "swap-balance",
		Short: "See which swap partners agreed to a swap and then did not deliver.",
		Long: "Per-partner swap reliability, from the local reservation mirror.\n\n" +
			"Bookclicker pairs both halves of a swap when it is booked, so there is no\n" +
			"population of partners quietly owing you one. What does vary is whether an\n" +
			"agreed swap ran: this reports delivered against cancelled and declined, so\n" +
			"repeat cancellers are visible before you book them again.\n\n" +
			"Populate the mirror with 'reservations pull'.",
		Example:     "  bookclicker-pp-cli swap-balance --flaky --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "swap-balance")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			empty := swapBalanceResult{Partners: make([]swapPartner, 0), Note: reservationsEmptyNote}
			db, handled, err := openMirror(ctx, dbPath, cmd.OutOrStdout(), cmd.ErrOrStderr(), flags, empty)
			if err != nil || handled {
				return err
			}
			defer db.Close()

			all, err := loadReservations(ctx, db, 0)
			if err != nil {
				return err
			}

			byPartner := map[string]*swapPartner{}
			swaps, paired := 0, 0
			for _, r := range all {
				if !r.IsSwap {
					continue
				}
				swaps++
				if r.SwapPairID > 0 {
					paired++
				}
				name := strings.TrimSpace(r.Counterpar)
				if name == "" {
					name = strings.TrimSpace(r.ListName)
				}
				if name == "" {
					continue
				}
				p, ok := byPartner[name]
				if !ok {
					p = &swapPartner{Partner: name}
					byPartner[name] = p
				}
				p.Booked++
				status := strings.ToLower(strings.TrimSpace(r.Status))
				switch {
				case status == "cancelled":
					p.Cancelled++
				case status == "declined":
					p.Declined++
				case swapDelivered[status]:
					p.Delivered++
				}
				if r.SwapPairID == 0 {
					p.Unpaired++
				}
				if r.Date > p.LastDate {
					p.LastDate = r.Date
				}
			}

			partners := make([]swapPartner, 0, len(byPartner))
			for _, p := range byPartner {
				if p.Booked > 0 {
					p.Reliability = float64(p.Delivered) / float64(p.Booked)
				}
				if flagMin > 0 && p.Booked < flagMin {
					continue
				}
				if flagFlaky && p.Cancelled == 0 && p.Declined == 0 {
					continue
				}
				partners = append(partners, *p)
			}
			// Least reliable first; ties broken by volume so a 0/5 outranks a 0/1.
			sort.SliceStable(partners, func(i, j int) bool {
				if partners[i].Reliability != partners[j].Reliability {
					return partners[i].Reliability < partners[j].Reliability
				}
				return partners[i].Booked > partners[j].Booked
			})
			if flagLimit > 0 && len(partners) > flagLimit {
				partners = partners[:flagLimit]
			}

			res := swapBalanceResult{
				Partners: partners, Count: len(partners),
				TotalSwaps: swaps, PairedSwaps: paired,
			}
			switch {
			case len(all) == 0:
				res.Note = reservationsEmptyNote
			case swaps == 0:
				res.Note = fmt.Sprintf("no swap reservations among %d mirrored rows (paid promotions are excluded)", len(all))
			case len(partners) == 0 && flagFlaky:
				res.Note = fmt.Sprintf("no partner cancelled or declined a swap across %d swaps", swaps)
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			if len(partners) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), res.Note)
				return nil
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%-40s %7s %6s %6s %6s %7s  %s\n",
				"PARTNER", "BOOKED", "RAN", "CANC", "DECL", "RELIAB", "LAST")
			for _, p := range partners {
				fmt.Fprintf(w, "%-40s %7d %6d %6d %6d %6.0f%%  %s\n",
					truncPad(p.Partner, 40), p.Booked, p.Delivered, p.Cancelled, p.Declined,
					p.Reliability*100, p.LastDate)
			}
			fmt.Fprintf(w, "\n%d of %d swaps are paired with their reciprocal booking.\n", paired, swaps)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	cmd.Flags().BoolVar(&flagFlaky, "flaky", false, "Only partners who cancelled or declined at least one swap")
	cmd.Flags().IntVar(&flagMin, "min-swaps", 0, "Only partners with at least this many booked swaps")
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "Maximum partners to return")
	return cmd
}
