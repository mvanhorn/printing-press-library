// Hand-authored transcendence command. generate --force preserves this file.
// pp:data-source local
package cli

import (
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/hostex/internal/cliutil"
)

func newNovelOpsGapsCmd(flags *rootFlags) *cobra.Command {
	var flagWithin string
	var flagDB string

	cmd := &cobra.Command{
		Use:   "ops-gaps",
		Short: "Find imminent or occupied stays with no cleaning task or missing check-in details.",
		Long: "Joins locally synced reservations against tasks and check-in details to surface\n" +
			"upcoming or in-progress stays missing a cleaning task or with no check-in\n" +
			"details set — problems no single Hostex endpoint reports.\n\n" +
			"Reads the local mirror; run `hostex-pp-cli sync --resources reservations,tasks` first.",
		Example:     "  hostex-pp-cli ops-gaps --within 7d --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := rejectLiveDataSource(flags); err != nil {
				return err
			}
			within, err := cliutil.ParseDurationLoose(flagWithin)
			if err != nil {
				return usageErr(err)
			}

			db, done, err := openLocalMirror(cmd, flags, flagDB)
			if done {
				return err
			}
			defer db.Close()
			if !hintIfUnsynced(cmd, db, "reservations") {
				hintIfStale(cmd, db, "reservations", flags.maxAge)
			}

			reservations, err := listObjs(db, "reservations")
			if err != nil {
				return err
			}
			tasks, err := listObjs(db, "tasks")
			if err != nil {
				return err
			}

			// stay_codes that already have a cleaning task.
			cleaningStays := map[string]bool{}
			for _, t := range tasks {
				sc := novStr(t, "stay_code")
				if sc == "" {
					continue
				}
				if strings.Contains(strings.ToLower(novStr(t, "type")), "clean") {
					cleaningStays[sc] = true
				}
			}

			now := nowUTC()
			today := now.Truncate(24 * time.Hour)
			windowEnd := now.Add(within)

			type gapRow struct {
				StayCode     string   `json:"stay_code"`
				PropertyID   any      `json:"property_id,omitempty"`
				GuestName    string   `json:"guest_name,omitempty"`
				ChannelType  string   `json:"channel_type,omitempty"`
				CheckInDate  string   `json:"check_in_date,omitempty"`
				CheckOutDate string   `json:"check_out_date,omitempty"`
				Status       string   `json:"status,omitempty"`
				Gaps         []string `json:"gaps"`
			}

			rows := make([]gapRow, 0)
			scanned := 0
			for _, r := range reservations {
				scanned++
				if novStr(r, "cancelled_at") != "" {
					continue
				}
				if s := strings.ToLower(novStr(r, "status")); strings.Contains(s, "cancel") {
					continue
				}
				ci, ciOK := novTime(r["check_in_date"])
				co, coOK := novTime(r["check_out_date"])
				if !ciOK && !coOK {
					continue
				}
				// Relevant = not already departed, and arriving within the window.
				if coOK && co.Before(today) {
					continue
				}
				if ciOK && ci.After(windowEnd) {
					continue
				}
				sc := novStr(r, "stay_code")
				var gaps []string
				if sc == "" || !cleaningStays[sc] {
					gaps = append(gaps, "no_cleaning_task")
				}
				if cid := novMap(r, "check_in_details"); len(cid) == 0 {
					gaps = append(gaps, "missing_checkin_details")
				}
				if len(gaps) == 0 {
					continue
				}
				rows = append(rows, gapRow{
					StayCode:     sc,
					PropertyID:   r["property_id"],
					GuestName:    novStr(r, "guest_name"),
					ChannelType:  novStr(r, "channel_type"),
					CheckInDate:  novStr(r, "check_in_date"),
					CheckOutDate: novStr(r, "check_out_date"),
					Status:       novStr(r, "status"),
					Gaps:         gaps,
				})
			}

			sort.SliceStable(rows, func(i, j int) bool {
				return rows[i].CheckInDate < rows[j].CheckInDate
			})

			view := struct {
				Within        string   `json:"within"`
				ScannedStays  int      `json:"scanned_reservations"`
				StaysWithGaps int      `json:"stays_with_gaps"`
				Gaps          []gapRow `json:"gaps"`
			}{
				Within:        flagWithin,
				ScannedStays:  scanned,
				StaysWithGaps: len(rows),
				Gaps:          rows,
			}
			return novEmit(cmd, flags, view)
		},
	}
	cmd.Flags().StringVar(&flagWithin, "within", "7d", "Only consider stays arriving within this window (e.g. 7d, 48h, 2w)")
	cmd.Flags().StringVar(&flagDB, "db", "", "Path to the local SQLite mirror (default: per-user data dir)")
	return cmd
}
