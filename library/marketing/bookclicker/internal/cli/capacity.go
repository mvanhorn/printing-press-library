// Copyright 2026 wmiles81 and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: remaining promo slots against platform caps.
// pp:data-source local

package cli

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// Bookclicker's per-newsletter caps, documented in its FAQ: one Solo, one
// Feature, and up to nine Mentions per newsletter send.
var promoCaps = map[string]int{"solo": 1, "feature": 1, "mention": 9}

type capacityDay struct {
	Date      string         `json:"date"`
	Offered   []string       `json:"offered_types"`
	Booked    map[string]int `json:"booked"`
	Remaining map[string]int `json:"remaining"`
}

type capacityResult struct {
	ListID int64         `json:"list_id"`
	Name   string        `json:"name,omitempty"`
	From   string        `json:"from"`
	To     string        `json:"to"`
	Days   []capacityDay `json:"days"`
	Note   string        `json:"note,omitempty"`
}

func newNovelCapacityCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath   string
		flagList int64
		flagFrom string
		flagTo   string
	)

	cmd := &cobra.Command{
		Use:   "capacity",
		Short: "Show remaining Solo, Feature and Mention slots per newsletter per date.",
		Long: "Compute remaining promo capacity per date against Bookclicker's caps\n" +
			"(Solo 1, Feature 1, Mention 9 per newsletter).\n\n" +
			"Offered weekdays come from the synced newsletter; bookings come from the\n" +
			"local reservation mirror, so counts are only as complete as your last pull.",
		Example:     "  bookclicker-pp-cli capacity --list 12345 --from 2026-09-01 --to 2026-09-14",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "capacity")
			}
			if flagList <= 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--list is required"))
			}
			if flagFrom == "" || flagTo == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--from and --to are required (YYYY-MM-DD)"))
			}
			from, err := time.Parse("2006-01-02", flagFrom)
			if err != nil {
				return usageErr(fmt.Errorf("--from must be YYYY-MM-DD: %w", err))
			}
			to, err := time.Parse("2006-01-02", flagTo)
			if err != nil {
				return usageErr(fmt.Errorf("--to must be YYYY-MM-DD: %w", err))
			}
			if to.Before(from) {
				return usageErr(fmt.Errorf("--to (%s) is before --from (%s)", flagTo, flagFrom))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			empty := capacityResult{ListID: flagList, From: flagFrom, To: flagTo, Days: make([]capacityDay, 0)}
			db, handled, err := openMirror(ctx, dbPath, cmd.OutOrStdout(), cmd.ErrOrStderr(), flags, empty)
			if err != nil || handled {
				return err
			}
			defer db.Close()

			var name, data sql.NullString
			if err := db.DB().QueryRowContext(ctx,
				`SELECT name, data FROM lists WHERE CAST(id AS INTEGER) = ?`, flagList).
				Scan(&name, &data); err != nil && err != sql.ErrNoRows {
				return fmt.Errorf("looking up newsletter: %w", err)
			}
			if !data.Valid {
				res := empty
				res.Note = fmt.Sprintf("newsletter %d is not in the local mirror; run 'bookclicker-pp-cli sync' first", flagList)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), res, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), res.Note)
				return nil
			}

			offeredByType := map[string]map[int]bool{}
			for t := range promoCaps {
				offeredByType[t] = planOfferedWeekdays(data.String, t)
			}

			booked := map[string]map[string]int{}
			res, err := loadReservations(ctx, db, 0)
			if err != nil {
				return err
			}
			for _, r := range res {
				if !strings.EqualFold(strings.TrimSpace(r.ListName), strings.TrimSpace(name.String)) {
					continue
				}
				if booked[r.Date] == nil {
					booked[r.Date] = map[string]int{}
				}
				booked[r.Date][strings.ToLower(r.InvType)]++
			}

			days := make([]capacityDay, 0)
			for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
				key := d.Format("2006-01-02")
				wd := int(d.Weekday())
				offered := make([]string, 0, len(promoCaps))
				remaining := map[string]int{}
				dayBooked := map[string]int{}
				for t, cap := range promoCaps {
					if !offeredByType[t][wd] {
						continue
					}
					offered = append(offered, t)
					used := booked[key][t]
					dayBooked[t] = used
					left := cap - used
					if left < 0 {
						left = 0
					}
					remaining[t] = left
				}
				sort.Strings(offered)
				days = append(days, capacityDay{Date: key, Offered: offered, Booked: dayBooked, Remaining: remaining})
			}

			out := capacityResult{ListID: flagList, Name: name.String, From: flagFrom, To: flagTo, Days: days}
			if len(res) == 0 {
				out.Note = "no mirrored reservations, so booked counts are zero; run 'reservations pull' for accurate remaining capacity"
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Capacity for %s (list %d)\n\n", name.String, flagList)
			fmt.Fprintf(w, "%-12s %-10s %-24s\n", "DATE", "OFFERED", "REMAINING")
			for _, d := range days {
				if len(d.Offered) == 0 {
					fmt.Fprintf(w, "%-12s %-10s %s\n", d.Date, "—", "not offered this weekday")
					continue
				}
				parts := make([]string, 0, len(d.Remaining))
				for _, t := range d.Offered {
					parts = append(parts, fmt.Sprintf("%s %d/%d", t, d.Remaining[t], promoCaps[t]))
				}
				fmt.Fprintf(w, "%-12s %-10s %s\n", d.Date, strings.Join(d.Offered, ","), strings.Join(parts, "  "))
			}
			if out.Note != "" {
				fmt.Fprintf(w, "\n%s\n", out.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	cmd.Flags().Int64Var(&flagList, "list", 0, "Newsletter (list) id")
	cmd.Flags().StringVar(&flagFrom, "from", "", "Window start (YYYY-MM-DD)")
	cmd.Flags().StringVar(&flagTo, "to", "", "Window end (YYYY-MM-DD)")
	return cmd
}
