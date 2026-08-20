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

	"github.com/mvanhorn/printing-press-library/library/productivity/bonusly/internal/store"
	"github.com/mvanhorn/printing-press-library/library/productivity/bonusly/internal/types"
	"github.com/spf13/cobra"
)

func newNovelRecognitionAuditCmd(flags *rootFlags) *cobra.Command {
	var flagDept string

	cmd := &cobra.Command{
		Use:         "audit",
		Short:       "See whether your team's recognition spend is on pace with its monthly budget",
		Example:     "  bonusly-pp-cli recognition audit --dept engineering --agent",
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
				fmt.Fprintf(cmd.OutOrStdout(), "would audit department %s\n", dept)
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

			memberMap := make(map[string]*deptMemberResult)
			memberIDs := make(map[string]bool)
			for _, m := range members {
				if m.ID != "" {
					memberIDs[m.ID] = true
					memberMap[m.ID] = &deptMemberResult{
						ID:    m.ID,
						Email: m.Email,
					}
				}
			}

			rows, err := db.DB().QueryContext(cmd.Context(), `SELECT id, giver_id, receiver_ids, amount FROM recognition`)
			if err != nil {
				return err
			}
			defer rows.Close()
			type rawRec struct {
				ID          string
				GiverID     sql.NullString
				ReceiverIDs sql.NullString
				Amount      sql.NullInt64
			}
			var recs []rawRec
			for rows.Next() {
				var r rawRec
				if err := rows.Scan(&r.ID, &r.GiverID, &r.ReceiverIDs, &r.Amount); err != nil {
					return err
				}
				recs = append(recs, r)
			}
			if err := rows.Err(); err != nil {
				return err
			}

			var totalGiven int64
			var totalReceived int64

			for _, r := range recs {
				amount := int64(0)
				if r.Amount.Valid {
					amount = r.Amount.Int64
				}

				if r.GiverID.Valid && r.GiverID.String != "" {
					giverID := r.GiverID.String
					if memberIDs[giverID] {
						memberMap[giverID].Given += amount
						totalGiven += amount
					}
				}

				if r.ReceiverIDs.Valid && r.ReceiverIDs.String != "" {
					receivers := parseArrayString(r.ReceiverIDs.String)
					for _, recID := range receivers {
						if memberIDs[recID] {
							memberMap[recID].Received += amount
							totalReceived += amount
						}
					}
				}
			}

			// pp:hand-edit bonusly-endpoint-fix — /users/points_balance 404s
			// live; balance fields live directly on /users/me, which wraps
			// the object in {"success":...,"result":{...}} (unlike the
			// generated command helpers, a raw c.Get() here does not
			// unwrap that envelope automatically). Routed through
			// resolveReadWithStrategyAndResponsePath rather than a raw
			// c.Get() — see resolveMyUser's doc comment in helpers.go.
			balRaw, _, err := resolveReadWithStrategyAndResponsePath(cmd.Context(), c, flags, "live", "users", false, "/users/me", nil, nil, "", cmd.ErrOrStderr())
			if err != nil {
				// pp:hand-edit bonusly-dogfood-exit-code — see the matching
				// comment in recognition_gap.go.
				return classifyAPIError(err, flags)
			}
			var balEnvelope struct {
				Result types.Balance `json:"result"`
			}
			if err := json.Unmarshal(balRaw, &balEnvelope); err != nil {
				return err
			}
			bal := balEnvelope.Result

			estimatedMonthlyBudget := int64(bal.MonthlyBudget) * headcount.Int64
			budgetNote := "estimated as your own monthly budget x headcount; assumes uniform per-person budgets, not verified against the department's actual admin-configured budget."

			var memberList []*deptMemberResult
			for _, m := range memberMap {
				memberList = append(memberList, m)
			}
			sort.Slice(memberList, func(i, j int) bool {
				return memberList[i].ID < memberList[j].ID
			})

			if flags.asJSON || flags.agent {
				res := map[string]any{
					"department":               deptName.String,
					"headcount":                headcount.Int64,
					"total_given":              totalGiven,
					"total_received":           totalReceived,
					"estimated_monthly_budget": estimatedMonthlyBudget,
					"budget_note":              budgetNote,
					"members":                  memberList,
				}
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}

			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintf(tw, "DEPARTMENT\t%s\n", deptName.String)
			fmt.Fprintf(tw, "HEADCOUNT\t%d\n", headcount.Int64)
			fmt.Fprintf(tw, "TOTAL GIVEN\t%d\n", totalGiven)
			fmt.Fprintf(tw, "TOTAL RECEIVED\t%d\n", totalReceived)
			fmt.Fprintf(tw, "ESTIMATED MONTHLY BUDGET\t%d\n", estimatedMonthlyBudget)
			fmt.Fprintf(tw, "BUDGET NOTE\t%s\n", budgetNote)
			fmt.Fprintln(tw)
			fmt.Fprintf(tw, "MEMBER ID\tEMAIL\tGIVEN\tRECEIVED\n")
			for _, m := range memberList {
				fmt.Fprintf(tw, "%s\t%s\t%d\t%d\n", m.ID, m.Email, m.Given, m.Received)
			}
			_ = tw.Flush()
			return nil
		},
	}
	cmd.Flags().StringVar(&flagDept, "dept", "", "TODO: describe --dept")
	return cmd
}
