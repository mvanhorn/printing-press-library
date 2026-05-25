// Hand-written novel command: pivot active plan-connections across all members.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/auth/memberstack/internal/store"
)

type planRow struct {
	PlanID          string `json:"planId"`
	ActiveMembers   int    `json:"activeMembers"`
	TrialingMembers int    `json:"trialingMembers"`
	PaidMembers     int    `json:"paidMembers"`
	FreeMembers     int    `json:"freeMembers"`
	TotalMembers    int    `json:"totalMembers"`
}

type memberCoverage struct {
	ID              string   `json:"id"`
	Email           string   `json:"email,omitempty"`
	ActivePlanCount int      `json:"activePlanCount"`
	PlanIDs         []string `json:"planIds"`
}

type planCoverageReport struct {
	Plans           []planRow        `json:"plans"`
	ZeroPlanMembers []memberCoverage `json:"zeroPlanMembers"`
	Summary         struct {
		Members            int `json:"totalMembers"`
		MembersWithPlan    int `json:"membersWithAtLeastOnePlan"`
		MembersWithoutPlan int `json:"membersWithZeroPlans"`
		DistinctPlans      int `json:"distinctPlans"`
	} `json:"summary"`
}

func newPlanCoverageCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var membersOnly bool
	var plansOnly bool

	cmd := &cobra.Command{
		Use:   "plan-coverage",
		Short: "Pivot active plan-connections across all members; flag members with zero active plans.",
		Long: `Reads from the local SQLite mirror and computes:
  • Per-plan: count of active / trialing / paid / free members
  • Per-member: list of plan IDs they belong to
  • Members with zero active plans (re-engagement / pruning candidates)

Run 'memberstack-pp-cli sync --full' first to populate the local store.`,
		Example: `  memberstack-pp-cli plan-coverage --json
  memberstack-pp-cli plan-coverage --members-only --json | jq 'map(select(.activePlanCount == 0))'`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compute plan-coverage from local store")
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("memberstack-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local store: %w (hint: run 'sync --full' first)", err)
			}
			defer db.Close()

			rows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT id, data FROM resources
				WHERE resource_type IN ('members', 'member')
				LIMIT ?`, 100000)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			planAgg := map[string]*planRow{}
			memberCoverages := []memberCoverage{}
			zeroPlanMembers := []memberCoverage{}
			totalMembers := 0
			withPlan := 0

			for rows.Next() {
				var id string
				var data sql.NullString
				if err := rows.Scan(&id, &data); err != nil {
					continue
				}
				if !data.Valid {
					continue
				}
				var m map[string]any
				if err := json.Unmarshal([]byte(data.String), &m); err != nil {
					continue
				}
				totalMembers++

				email := ""
				if auth, ok := m["auth"].(map[string]any); ok {
					email = stringFromAny(auth["email"])
				}

				mc := memberCoverage{ID: id, Email: email, PlanIDs: []string{}}

				conns, _ := m["planConnections"].([]any)
				for _, c := range conns {
					obj, ok := c.(map[string]any)
					if !ok {
						continue
					}
					planID := stringFromAny(obj["planId"])
					if planID == "" {
						continue
					}
					active, _ := obj["active"].(bool)
					status := stringFromAny(obj["status"])
					ptype := stringFromAny(obj["type"])

					isActive := active || status == "ACTIVE" || status == "TRIALING"
					if !isActive {
						continue
					}
					mc.ActivePlanCount++
					mc.PlanIDs = append(mc.PlanIDs, planID)

					row := planAgg[planID]
					if row == nil {
						row = &planRow{PlanID: planID}
						planAgg[planID] = row
					}
					row.TotalMembers++
					row.ActiveMembers++
					if status == "TRIALING" {
						row.TrialingMembers++
					}
					if ptype == "PAID" {
						row.PaidMembers++
					} else {
						row.FreeMembers++
					}
				}

				if mc.ActivePlanCount > 0 {
					withPlan++
				} else {
					zeroPlanMembers = append(zeroPlanMembers, mc)
				}
				memberCoverages = append(memberCoverages, mc)
			}

			plans := make([]planRow, 0, len(planAgg))
			for _, r := range planAgg {
				plans = append(plans, *r)
			}
			sort.SliceStable(plans, func(i, j int) bool { return plans[i].ActiveMembers > plans[j].ActiveMembers })

			report := planCoverageReport{Plans: plans, ZeroPlanMembers: zeroPlanMembers}
			report.Summary.Members = totalMembers
			report.Summary.MembersWithPlan = withPlan
			report.Summary.MembersWithoutPlan = len(zeroPlanMembers)
			report.Summary.DistinctPlans = len(plans)

			var out any = report
			if membersOnly {
				out = memberCoverages
			} else if plansOnly {
				out = plans
			}

			data, err := json.MarshalIndent(out, "", "  ")
			if err != nil {
				return err
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printOutputWithFlags(cmd.OutOrStdout(), json.RawMessage(data), flags)
			}
			// Human table
			fmt.Fprintf(cmd.OutOrStdout(), "Plan coverage across %d members (%d with at least one active plan, %d with none)\n\n",
				report.Summary.Members, report.Summary.MembersWithPlan, report.Summary.MembersWithoutPlan)
			fmt.Fprintln(cmd.OutOrStdout(), "Plan ID                    Active  Trialing  Paid  Free")
			for _, p := range plans {
				fmt.Fprintf(cmd.OutOrStdout(), "%-26s %6d %9d %5d %5d\n", truncateMid(p.PlanID, 26), p.ActiveMembers, p.TrialingMembers, p.PaidMembers, p.FreeMembers)
			}
			if len(zeroPlanMembers) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "\n%d member(s) with zero active plans:\n", len(zeroPlanMembers))
				for _, m := range zeroPlanMembers {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s\t%s\n", m.ID, m.Email)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Override local SQLite path")
	cmd.Flags().BoolVar(&membersOnly, "members-only", false, "Output only the per-member coverage array")
	cmd.Flags().BoolVar(&plansOnly, "plans-only", false, "Output only the per-plan aggregate")
	return cmd
}

func truncateMid(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 4 {
		return s[:n]
	}
	head := (n - 1) / 2
	tail := n - 1 - head
	return s[:head] + "…" + s[len(s)-tail:]
}

// imports placeholder to keep `strings` if needed in future edits.
var _ = strings.TrimSpace
