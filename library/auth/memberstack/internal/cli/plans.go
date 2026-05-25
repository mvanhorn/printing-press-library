// Hand-written novel command: infer the plans index from observed
// plan-connections (Memberstack does not expose plans via REST).

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/auth/memberstack/internal/store"
)

func newPlansCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plans",
		Short: "Plans index inferred from observed plan-connections (REST has no plans endpoint)",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newPlansListCmd(flags))
	return cmd
}

type planInfo struct {
	PlanID      string `json:"planId"`
	MemberCount int    `json:"memberCount"`
	ActiveCount int    `json:"activeCount"`
	PaidCount   int    `json:"paidCount"`
	FreeCount   int    `json:"freeCount"`
	FirstSeen   string `json:"firstSeen,omitempty"`
	LastSeen    string `json:"lastSeen,omitempty"`
}

func newPlansListCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List plan IDs observed across synced members, with per-plan counts and first/last seen.",
		Long: `Memberstack does not expose plans via the Admin REST API; plan metadata lives
in the dashboard. This command builds an index from every planConnection in the
local store so agents can see which plan IDs are in use without opening the UI.

Run 'memberstack-pp-cli sync --full' first to populate the local store.`,
		Example: `  memberstack-pp-cli plans list --json
  memberstack-pp-cli plans list --json | jq 'sort_by(.memberCount) | reverse'`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would build plans index from local store")
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
				SELECT id, data FROM resources WHERE resource_type IN ('members','member')`)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			agg := map[string]*planInfo{}
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
				createdAt := stringFromAny(m["createdAt"])
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
					p := agg[planID]
					if p == nil {
						p = &planInfo{PlanID: planID, FirstSeen: createdAt, LastSeen: createdAt}
						agg[planID] = p
					}
					p.MemberCount++

					active, _ := obj["active"].(bool)
					status := stringFromAny(obj["status"])
					if active || status == "ACTIVE" || status == "TRIALING" {
						p.ActiveCount++
					}
					if stringFromAny(obj["type"]) == "PAID" {
						p.PaidCount++
					} else {
						p.FreeCount++
					}
					if createdAt != "" && (p.FirstSeen == "" || createdAt < p.FirstSeen) {
						p.FirstSeen = createdAt
					}
					if createdAt != "" && createdAt > p.LastSeen {
						p.LastSeen = createdAt
					}
				}
			}

			out := make([]planInfo, 0, len(agg))
			for _, p := range agg {
				out = append(out, *p)
			}
			sort.SliceStable(out, func(i, j int) bool { return out[i].MemberCount > out[j].MemberCount })

			data, err := json.MarshalIndent(out, "", "  ")
			if err != nil {
				return err
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printOutputWithFlags(cmd.OutOrStdout(), json.RawMessage(data), flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d distinct plan ID(s) observed in local store:\n", len(out))
			fmt.Fprintln(cmd.OutOrStdout(), "Plan ID                    Members  Active  Paid  Free  First seen")
			for _, p := range out {
				fmt.Fprintf(cmd.OutOrStdout(), "%-26s %7d %7d %5d %5d  %s\n",
					truncateMid(p.PlanID, 26), p.MemberCount, p.ActiveCount, p.PaidCount, p.FreeCount, p.FirstSeen)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "(no plan-connections observed — sync first or no members have plans)")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Override local SQLite path")
	return cmd
}
