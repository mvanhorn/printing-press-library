// Copyright 2026 Allen Lew and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/bonusly/internal/store"
	"github.com/spf13/cobra"
)

type neglectedReport struct {
	ID               string  `json:"id"`
	Email            string  `json:"email"`
	DisplayName      string  `json:"display_name"`
	LastRecognizedAt *string `json:"last_recognized_at"`
}

func newNovelRecognitionGapCmd(flags *rootFlags) *cobra.Command {
	var flagManager string
	var flagDays int

	cmd := &cobra.Command{
		Use:         "gap",
		Short:       "Find direct reports you haven't recognized recently, without an admin's Participation Report.",
		Example:     "  bonusly-pp-cli recognition gap --manager me --days 30 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				mgr := "(no --manager given)"
				if flagManager != "" {
					mgr = flagManager
				}
				fmt.Fprintf(cmd.OutOrStdout(), "would find recognition gaps for direct reports of manager %q not recognized in the last %d days\n", mgr, flagDays)
				return nil
			}
			if flagManager == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--manager is required"))
			}
			if flagDays <= 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--days must be positive"))
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

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			resolvedManagerID := flagManager
			if resolvedManagerID == "me" {
				me, err := resolveMyUser(cmd.Context(), c, flags, cmd.ErrOrStderr())
				if err != nil {
					// pp:hand-edit bonusly-dogfood-exit-code — was a raw
					// `return err`, so a live 401 exited 1 instead of the
					// authoritative auth exit code 4 that classifyAPIError
					// assigns. Live dogfood's retry/skip-on-persistent-401
					// handling keys off exit code 4, not the vendor's 401
					// body text, so this masked real auth failures as opaque
					// command failures instead of a recognizable auth denial.
					return classifyAPIError(err, flags)
				}
				resolvedManagerID = me.Id
			}

			// live-call direct reports
			path := replacePathParam("/users/{manager}/direct_reports", "manager", resolvedManagerID)
			reportsRaw, _, err := resolveReadWithStrategyAndResponsePath(cmd.Context(), c, flags, "live", "org", false, path, nil, nil, "", cmd.ErrOrStderr())
			if err != nil {
				return classifyAPIError(err, flags)
			}

			type directReport struct {
				ID          string `json:"id"`
				Email       string `json:"email"`
				DisplayName string `json:"display_name"`
			}
			// pp:hand-edit bonusly-endpoint-fix — verified live response shape
			// is {"data":{"users":[...]}}, not a bare array.
			var directReportsEnvelope struct {
				Data struct {
					Users []directReport `json:"users"`
				} `json:"data"`
			}
			if err := json.Unmarshal(reportsRaw, &directReportsEnvelope); err != nil {
				return err
			}
			reports := directReportsEnvelope.Data.Users

			// open store and query recognitions given by this manager
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			rows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT id, giver_id, receiver_ids, created_at
				FROM recognition
				WHERE giver_id = ?`, resolvedManagerID)
			if err != nil {
				return err
			}
			defer rows.Close()

			type rawRec struct {
				ID          string
				GiverID     sql.NullString
				ReceiverIDs sql.NullString
				CreatedAt   sql.NullString
			}
			var recs []rawRec
			for rows.Next() {
				var r rawRec
				if err := rows.Scan(&r.ID, &r.GiverID, &r.ReceiverIDs, &r.CreatedAt); err != nil {
					return err
				}
				recs = append(recs, r)
			}
			if err := rows.Err(); err != nil {
				return err
			}

			// find max created_at for each direct report
			lastTimes := make(map[string]time.Time)
			for _, r := range recs {
				if r.ReceiverIDs.Valid && r.ReceiverIDs.String != "" {
					receivers := parseArrayString(r.ReceiverIDs.String)
					for _, recID := range receivers {
						if r.CreatedAt.Valid && r.CreatedAt.String != "" {
							t, err := time.Parse(time.RFC3339, r.CreatedAt.String)
							if err != nil {
								t, err = time.Parse("2006-01-02T15:04:05.000Z", r.CreatedAt.String)
							}
							if err == nil {
								if current, exists := lastTimes[recID]; !exists || t.After(current) {
									lastTimes[recID] = t
								}
							}
						}
					}
				}
			}

			threshold := time.Now().Add(-time.Duration(flagDays) * 24 * time.Hour)
			var neglected []neglectedReport
			for _, rep := range reports {
				t, exists := lastTimes[rep.ID]
				if !exists {
					neglected = append(neglected, neglectedReport{
						ID:               rep.ID,
						Email:            rep.Email,
						DisplayName:      rep.DisplayName,
						LastRecognizedAt: nil,
					})
				} else if t.Before(threshold) {
					tStr := t.Format(time.RFC3339)
					neglected = append(neglected, neglectedReport{
						ID:               rep.ID,
						Email:            rep.Email,
						DisplayName:      rep.DisplayName,
						LastRecognizedAt: &tStr,
					})
				}
			}

			sort.Slice(neglected, func(i, j int) bool {
				return neglected[i].ID < neglected[j].ID
			})

			if flags.asJSON || flags.agent {
				res := map[string]any{
					"manager_id":     resolvedManagerID,
					"days_threshold": flagDays,
					"neglected":      neglected,
					"checked_count":  len(reports),
				}
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}

			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintf(tw, "MANAGER ID\t%s\n", resolvedManagerID)
			fmt.Fprintf(tw, "DAYS THRESHOLD\t%d\n", flagDays)
			fmt.Fprintf(tw, "CHECKED REPORTS\t%d\n", len(reports))
			fmt.Fprintf(tw, "NEGLECTED REPORTS\t%d\n", len(neglected))
			fmt.Fprintln(tw)
			fmt.Fprintf(tw, "REPORT ID\tNAME\tEMAIL\tLAST RECOGNIZED\n")
			for _, n := range neglected {
				last := "never"
				if n.LastRecognizedAt != nil {
					last = *n.LastRecognizedAt
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", n.ID, n.DisplayName, n.Email, last)
			}
			_ = tw.Flush()
			return nil
		},
	}
	cmd.Flags().StringVar(&flagManager, "manager", "", "TODO: describe --manager")
	cmd.Flags().IntVar(&flagDays, "days", 30, "TODO: describe --days")
	return cmd
}
