// Copyright 2026 wmiles81 and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: aging unanswered offers.
// pp:data-source local

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type staleOffer struct {
	novelReservation
	AgeDays int `json:"age_days"`
}

type staleResult struct {
	Offers    []staleOffer `json:"offers"`
	Count     int          `json:"count"`
	Threshold int          `json:"threshold_days"`
	Note      string       `json:"note,omitempty"`
}

func newNovelStaleCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath   string
		flagDays int
	)

	cmd := &cobra.Command{
		Use:   "stale",
		Short: "List pending offers that have gone unanswered the longest.",
		Long: "Surface offers still sitting in 'pending' past a threshold, oldest first.\n\n" +
			"The web UI shows an offer's status but never its age, so offers can go\n" +
			"quietly unanswered. Reads the local mirror populated by 'reservations pull'.",
		Example:     "  bookclicker-pp-cli stale --days 7 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "stale")
			}
			if flagDays < 0 {
				return usageErr(fmt.Errorf("--days must be zero or positive"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			empty := staleResult{Offers: make([]staleOffer, 0), Threshold: flagDays, Note: reservationsEmptyNote}
			db, handled, err := openMirror(ctx, dbPath, cmd.OutOrStdout(), cmd.ErrOrStderr(), flags, empty)
			if err != nil || handled {
				return err
			}
			defer db.Close()

			all, err := loadReservations(ctx, db, 0)
			if err != nil {
				return err
			}
			now := time.Now()
			offers := make([]staleOffer, 0)
			for _, r := range all {
				if !strings.EqualFold(strings.TrimSpace(r.Status), "pending") {
					continue
				}
				age := 0
				if t, ok := parseLooseDate(r.Date); ok {
					age = int(now.Sub(t).Hours() / 24)
				}
				if age < flagDays {
					continue
				}
				offers = append(offers, staleOffer{novelReservation: r, AgeDays: age})
			}
			sort.SliceStable(offers, func(i, j int) bool { return offers[i].AgeDays > offers[j].AgeDays })

			res := staleResult{Offers: offers, Count: len(offers), Threshold: flagDays}
			if len(all) == 0 {
				res.Note = reservationsEmptyNote
			} else if len(offers) == 0 {
				res.Note = fmt.Sprintf("no pending offers older than %d day(s) across %d mirrored reservations", flagDays, len(all))
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			if len(offers) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), res.Note)
				return nil
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%-6s %-12s %-36s %-8s %s\n", "AGE", "DATE", "NEWSLETTER", "TYPE", "COUNTERPARTY")
			for _, o := range offers {
				fmt.Fprintf(w, "%-6s %-12s %-36s %-8s %s\n",
					fmt.Sprintf("%dd", o.AgeDays), o.Date, truncPad(o.ListName, 36), o.InvType, truncPad(o.Counterpar, 24))
			}
			fmt.Fprintf(w, "\n%d pending offer(s) at or beyond %d day(s).\n", len(offers), flagDays)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	cmd.Flags().IntVar(&flagDays, "days", 7, "Minimum age in days for an offer to count as stale")
	return cmd
}
