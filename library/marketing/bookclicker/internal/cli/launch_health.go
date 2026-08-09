// Copyright 2026 wmiles81 and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: launch window coverage.
// pp:data-source local

package cli

import (
	"database/sql"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

type launchDay struct {
	Date   string   `json:"date"`
	Promos []string `json:"promos"`
	Count  int      `json:"count"`
}

type launchHealthResult struct {
	BookID       int64       `json:"book_id"`
	BookTitle    string      `json:"book_title,omitempty"`
	LaunchDate   string      `json:"launch_date,omitempty"`
	From         string      `json:"from"`
	To           string      `json:"to"`
	Days         []launchDay `json:"days"`
	CoveredDays  int         `json:"covered_days"`
	UncoveredDay []string    `json:"uncovered_days"`
	TotalPromos  int         `json:"total_promos"`
	Note         string      `json:"note,omitempty"`
}

func newNovelLaunchHealthCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath   string
		flagBook int64
		flagBefore int
		flagAfter  int
	)

	cmd := &cobra.Command{
		Use:   "health",
		Short: "Show which dates in a book's launch window still have no promo booked.",
		Long: "Join a book's launch window against its booked promotions to expose gaps.\n\n" +
			"Reads the local mirror populated by 'reservations pull'.",
		Example:     "  bookclicker-pp-cli launch health --book 12345 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "launch health")
			}
			if flagBook <= 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--book is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			empty := launchHealthResult{BookID: flagBook, Days: make([]launchDay, 0), UncoveredDay: make([]string, 0), Note: reservationsEmptyNote}
			db, handled, err := openMirror(ctx, dbPath, cmd.OutOrStdout(), cmd.ErrOrStderr(), flags, empty)
			if err != nil || handled {
				return err
			}
			defer db.Close()

			var title, launchDate sql.NullString
			_ = db.DB().QueryRowContext(ctx,
				`SELECT title, launch_date FROM books WHERE CAST(id AS INTEGER) = ?`, flagBook).
				Scan(&title, &launchDate)

			anchor, ok := parseLooseDate(launchDate.String)
			if !ok {
				anchor = time.Now()
			}
			from := anchor.AddDate(0, 0, -flagBefore)
			to := anchor.AddDate(0, 0, flagAfter)

			res, err := loadReservations(ctx, db, flagBook)
			if err != nil {
				return err
			}
			byDate := map[string][]string{}
			for _, r := range res {
				byDate[r.Date] = append(byDate[r.Date], fmt.Sprintf("%s (%s)", r.ListName, r.InvType))
			}

			days := make([]launchDay, 0)
			uncovered := make([]string, 0)
			covered, total := 0, 0
			for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
				key := d.Format("2006-01-02")
				promos := byDate[key]
				if promos == nil {
					promos = []string{}
				}
				days = append(days, launchDay{Date: key, Promos: promos, Count: len(promos)})
				total += len(promos)
				if len(promos) > 0 {
					covered++
				} else {
					uncovered = append(uncovered, key)
				}
			}
			sort.Strings(uncovered)

			out := launchHealthResult{
				BookID: flagBook, BookTitle: title.String, LaunchDate: launchDate.String,
				From: from.Format("2006-01-02"), To: to.Format("2006-01-02"),
				Days: days, CoveredDays: covered, UncoveredDay: uncovered, TotalPromos: total,
			}
			if len(res) == 0 {
				out.Note = reservationsEmptyNote
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Launch health for book %d %s\n", flagBook, title.String)
			fmt.Fprintf(w, "Window %s .. %s (launch %s)\n\n", out.From, out.To, out.LaunchDate)
			for _, d := range days {
				mark := "—"
				if d.Count > 0 {
					mark = fmt.Sprintf("%d promo(s)", d.Count)
				}
				fmt.Fprintf(w, "  %-12s %s\n", d.Date, mark)
			}
			fmt.Fprintf(w, "\n%d/%d days covered, %d promo(s) total.\n", covered, len(days), total)
			if out.Note != "" {
				fmt.Fprintln(w, out.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	cmd.Flags().Int64Var(&flagBook, "book", 0, "Book id to assess")
	cmd.Flags().IntVar(&flagBefore, "days-before", 7, "Days before launch date to include")
	cmd.Flags().IntVar(&flagAfter, "days-after", 21, "Days after launch date to include")
	return cmd
}
