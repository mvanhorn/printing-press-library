// Copyright 2026 Allen Lew and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source local

package cli

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/bonusly/internal/store"
	"github.com/spf13/cobra"
)

func newNovelRedemptionsForecastCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "forecast",
		Short:       "Project your reward-redemption spend from your own history -- a simple trend line, not a black box.",
		Example:     "  bonusly-pp-cli redemptions forecast --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would forecast redemptions")
				return nil
			}

			// check missing mirror
			isMissing, dbPath, err := checkMissingMirrorGuard(cmd, flags)
			if err != nil {
				return err
			}
			if isMissing {
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				}
				return nil
			}

			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			rows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT id, created_at
				FROM redemptions
				ORDER BY created_at ASC`)
			if err != nil {
				return err
			}
			defer rows.Close()

			type rawRedemption struct {
				ID        string
				CreatedAt sql.NullString
			}
			var list []rawRedemption
			for rows.Next() {
				var r rawRedemption
				if err := rows.Scan(&r.ID, &r.CreatedAt); err != nil {
					return err
				}
				list = append(list, r)
			}
			if err := rows.Err(); err != nil {
				return err
			}

			var totalRedemptions = int64(len(list))
			var dateRangeDays float64
			var redemptionsPerWeek *float64
			var note string

			if totalRedemptions < 2 {
				note = "at least 2 redemptions are required to compute a forecast"
			} else {
				first := list[0]
				last := list[len(list)-1]

				if first.CreatedAt.Valid && first.CreatedAt.String != "" && last.CreatedAt.Valid && last.CreatedAt.String != "" {
					earliest, err1 := time.Parse(time.RFC3339, first.CreatedAt.String)
					latest, err2 := time.Parse(time.RFC3339, last.CreatedAt.String)
					if err1 != nil {
						earliest, err1 = time.Parse("2006-01-02T15:04:05.000Z", first.CreatedAt.String)
					}
					if err2 != nil {
						latest, err2 = time.Parse("2006-01-02T15:04:05.000Z", last.CreatedAt.String)
					}

					if err1 == nil && err2 == nil {
						duration := latest.Sub(earliest)
						dateRangeDays = duration.Hours() / 24.0
						weeks := duration.Hours() / (24 * 7.0)
						if weeks > 0.0001 {
							rate := float64(totalRedemptions) / weeks
							redemptionsPerWeek = &rate
						} else {
							note = "not enough time elapsed between redemptions to compute a forecast rate"
						}
					} else {
						note = "could not parse redemption created_at timestamps"
					}
				} else {
					note = "missing redemption created_at timestamps"
				}
			}

			if flags.asJSON || flags.agent {
				res := map[string]any{
					"total_redemptions": totalRedemptions,
					"date_range_days":   dateRangeDays,
				}
				if note != "" {
					res["note"] = note
					res["redemptions_per_week"] = nil
				} else if redemptionsPerWeek != nil {
					res["redemptions_per_week"] = *redemptionsPerWeek
				}
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}

			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintf(tw, "TOTAL REDEMPTIONS\t%d\n", totalRedemptions)
			fmt.Fprintf(tw, "DATE RANGE DAYS\t%.2f\n", dateRangeDays)
			if note != "" {
				fmt.Fprintf(tw, "NOTE\t%s\n", note)
			}
			if redemptionsPerWeek != nil {
				fmt.Fprintf(tw, "REDEMPTIONS PER WEEK\t%.2f\n", *redemptionsPerWeek)
			}
			_ = tw.Flush()
			return nil
		},
	}
	return cmd
}
