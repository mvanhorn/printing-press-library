// Copyright 2026 wmiles81 and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: confirmations awaiting action.
// pp:data-source local

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type confirmDueResult struct {
	Pending []novelReservation `json:"pending"`
	Count   int                `json:"count"`
	Note    string             `json:"note,omitempty"`
}

// confirmPendingStatuses are the rendered statuses that still need the user to
// name the campaign that carried the promotion.
var confirmPendingStatuses = map[string]bool{
	"sent": true, "pending": true, "awaiting confirmation": true, "unconfirmed": true,
}

func newNovelConfirmDueCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath   string
		flagBook int64
		flagAll  bool
	)

	cmd := &cobra.Command{
		Use:   "confirm-due",
		Short: "List every promo awaiting your confirmation, oldest first.",
		Long: "List booked promotions that have gone out but are not yet confirmed.\n\n" +
			"Reads the local mirror. Reservations are ingested from your launch pages\n" +
			"by 'reservations pull', because Bookclicker exposes no JSON index for them.",
		Example:     "  bookclicker-pp-cli confirm-due --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "confirm-due")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			empty := confirmDueResult{Pending: make([]novelReservation, 0), Note: reservationsEmptyNote}
			db, handled, err := openMirror(ctx, dbPath, cmd.OutOrStdout(), cmd.ErrOrStderr(), flags, empty)
			if err != nil || handled {
				return err
			}
			defer db.Close()

			all, err := loadReservations(ctx, db, flagBook)
			if err != nil {
				return err
			}
			pending := make([]novelReservation, 0, len(all))
			for _, r := range all {
				if flagAll || confirmPendingStatuses[strings.ToLower(strings.TrimSpace(r.Status))] {
					pending = append(pending, r)
				}
			}
			sort.SliceStable(pending, func(i, j int) bool { return pending[i].Date < pending[j].Date })

			res := confirmDueResult{Pending: pending, Count: len(pending)}
			if len(all) == 0 {
				res.Note = reservationsEmptyNote
			} else if len(pending) == 0 {
				res.Note = fmt.Sprintf("nothing awaiting confirmation across %d mirrored reservations", len(all))
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			if len(pending) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), res.Note)
				return nil
			}
			w := cmd.OutOrStdout()
			now := time.Now()
			fmt.Fprintf(w, "%-12s %-34s %-8s %-10s %s\n", "DATE", "NEWSLETTER", "TYPE", "STATUS", "AGE")
			for _, r := range pending {
				age := ""
				if t, ok := parseLooseDate(r.Date); ok {
					age = fmt.Sprintf("%dd", int(now.Sub(t).Hours()/24))
				}
				fmt.Fprintf(w, "%-12s %-34s %-8s %-10s %s\n",
					r.Date, truncPad(r.ListName, 34), r.InvType, r.Status, age)
			}
			fmt.Fprintf(w, "\n%d promo(s) awaiting confirmation.\n", len(pending))
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	cmd.Flags().Int64Var(&flagBook, "book", 0, "Only promotions for this book id")
	cmd.Flags().BoolVar(&flagAll, "all", false, "Show every mirrored reservation, not just unconfirmed ones")
	return cmd
}
