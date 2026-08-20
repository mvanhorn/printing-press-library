// Copyright 2026 Allen Lew and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto

package cli

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/bonusly/internal/store"
	"github.com/spf13/cobra"
)

type hashtagCount struct {
	Hashtag string `json:"hashtag"`
	Count   int64  `json:"count"`
}

func newNovelRecognitionValuesCmd(flags *rootFlags) *cobra.Command {
	var flagDept string

	cmd := &cobra.Command{
		Use:         "values",
		Short:       "See which company-value hashtags are actually trending in a department, instead of manually tallying the feed.",
		Example:     "  bonusly-pp-cli recognition values --dept engineering --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				dept := "(no --dept given)"
				if flagDept != "" {
					dept = flagDept
				}
				fmt.Fprintf(cmd.OutOrStdout(), "would analyze recognition hashtag trends for department %q\n", dept)
				return nil
			}
			if flagDept == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--dept is required"))
			}

			// check missing mirror
			isMissing, dbPath, err := checkMissingMirrorGuard(cmd, flags)
			if err != nil {
				return err
			}
			if isMissing {
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "{}")
				}
				return nil
			}

			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			var deptName sql.NullString
			var headcount sql.NullInt64
			err = db.DB().QueryRowContext(cmd.Context(), `SELECT name, user_count FROM departments WHERE name = ? COLLATE NOCASE AND user_count IS NOT NULL LIMIT 1`, flagDept).Scan(&deptName, &headcount)
			if err == sql.ErrNoRows {
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "{}")
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "department not found locally; run sync --resources departments\n")
				}
				return nil
			} else if err != nil {
				return err
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			members, err := fetchDeptMembers(cmd.Context(), c, flags, deptName.String, cmd)
			if err != nil {
				return err
			}

			memberIDs := make(map[string]bool)
			for _, m := range members {
				if m.ID != "" {
					memberIDs[m.ID] = true
				}
			}

			rows, err := db.DB().QueryContext(cmd.Context(), `SELECT id, giver_id, receiver_ids, hashtags FROM recognition`)
			if err != nil {
				return err
			}
			defer rows.Close()
			type rawRec struct {
				ID          string
				GiverID     sql.NullString
				ReceiverIDs sql.NullString
				Hashtags    sql.NullString
			}
			var recs []rawRec
			for rows.Next() {
				var r rawRec
				if err := rows.Scan(&r.ID, &r.GiverID, &r.ReceiverIDs, &r.Hashtags); err != nil {
					return err
				}
				recs = append(recs, r)
			}
			if err := rows.Err(); err != nil {
				return err
			}

			var totalScanned int64
			hashtagCounts := make(map[string]int64)

			for _, r := range recs {
				match := false
				if r.GiverID.Valid && memberIDs[r.GiverID.String] {
					match = true
				} else if r.ReceiverIDs.Valid && r.ReceiverIDs.String != "" {
					receivers := parseArrayString(r.ReceiverIDs.String)
					for _, recID := range receivers {
						if memberIDs[recID] {
							match = true
							break
						}
					}
				}

				if match {
					totalScanned++
					if r.Hashtags.Valid && r.Hashtags.String != "" {
						tags := parseArrayString(r.Hashtags.String)
						for _, t := range tags {
							t = strings.ToLower(t)
							hashtagCounts[t]++
						}
					}
				}
			}

			var list []hashtagCount
			for h, c := range hashtagCounts {
				list = append(list, hashtagCount{Hashtag: h, Count: c})
			}
			sort.Slice(list, func(i, j int) bool {
				if list[i].Count != list[j].Count {
					return list[i].Count > list[j].Count
				}
				return list[i].Hashtag < list[j].Hashtag
			})

			if flags.asJSON || flags.agent {
				res := map[string]any{
					"department":                deptName.String,
					"hashtag_counts":            list,
					"total_recognitions_scanned": totalScanned,
				}
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}

			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintf(tw, "DEPARTMENT\t%s\n", deptName.String)
			fmt.Fprintf(tw, "TOTAL RECOGNITIONS SCANNED\t%d\n", totalScanned)
			fmt.Fprintln(tw)
			fmt.Fprintf(tw, "HASHTAG\tCOUNT\n")
			for _, item := range list {
				fmt.Fprintf(tw, "#%s\t%d\n", item.Hashtag, item.Count)
			}
			_ = tw.Flush()
			return nil
		},
	}
	cmd.Flags().StringVar(&flagDept, "dept", "", "TODO: describe --dept")
	return cmd
}
