// Hand-written novel command: find members whose lastLogin is older than N days.
// Reads from the local SQLite mirror (run `sync --full` first).

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/auth/memberstack/internal/store"
)

type staleMember struct {
	ID         string `json:"id"`
	Email      string `json:"email,omitempty"`
	LastLogin  string `json:"lastLogin,omitempty"`
	CreatedAt  string `json:"createdAt,omitempty"`
	DaysSince  int    `json:"daysSinceLastLogin"`
	Plans      int    `json:"activePlanCount"`
	IsVerified bool   `json:"verified"`
}

func newStaleCmd(flags *rootFlags) *cobra.Command {
	var days int
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:   "stale",
		Short: "Find members whose lastLogin is older than N days.",
		Long: `Reads from the local SQLite mirror and returns members whose lastLogin is
older than --days (default 30). Members who have never logged in (lastLogin null
or zero) are included. Sorted oldest-first.

Run 'memberstack-pp-cli sync --full' first to populate the local store.`,
		Example: `  memberstack-pp-cli stale --days 30 --json
  memberstack-pp-cli stale --days 90 --json --select id,email,lastLogin | jq -r '.[].id'`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 && isTerminal(cmd.OutOrStdout()) {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would query local store for members with lastLogin > %d days\n", days)
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

			cutoff := time.Now().UTC().AddDate(0, 0, -days)
			rows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT id, data FROM resources
				WHERE resource_type IN ('members', 'member')
				LIMIT ?`, 100000)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			out := make([]staleMember, 0, 64)
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

				lastLoginStr := stringFromAny(m["lastLogin"])
				createdAtStr := stringFromAny(m["createdAt"])
				lastLoginT, hasLogin := parseRFC3339(lastLoginStr)

				// Members who never logged in count as stale.
				if hasLogin && lastLoginT.After(cutoff) {
					continue
				}

				email := ""
				if auth, ok := m["auth"].(map[string]any); ok {
					email = stringFromAny(auth["email"])
				}

				verified, _ := m["verified"].(bool)
				planCount := countActivePlans(m["planConnections"])

				daysSince := -1
				if hasLogin {
					daysSince = int(time.Since(lastLoginT).Hours() / 24)
				}

				out = append(out, staleMember{
					ID:         id,
					Email:      email,
					LastLogin:  lastLoginStr,
					CreatedAt:  createdAtStr,
					DaysSince:  daysSince,
					Plans:      planCount,
					IsVerified: verified,
				})
			}

			// Oldest first; never-logged-in last (DaysSince == -1).
			sort.SliceStable(out, func(i, j int) bool {
				if out[i].DaysSince == -1 {
					return false
				}
				if out[j].DaysSince == -1 {
					return true
				}
				return out[i].DaysSince > out[j].DaysSince
			})

			if limit > 0 && len(out) > limit {
				out = out[:limit]
			}

			data, err := json.MarshalIndent(out, "", "  ")
			if err != nil {
				return err
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printOutputWithFlags(cmd.OutOrStdout(), json.RawMessage(data), flags)
			}
			// Human table
			fmt.Fprintf(cmd.OutOrStdout(), "%d stale member(s) (last login > %d days ago)\n", len(out), days)
			for _, m := range out {
				dsTxt := "never"
				if m.DaysSince >= 0 {
					dsTxt = fmt.Sprintf("%d days ago", m.DaysSince)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %s\t%s\t(%s)\n", m.ID, m.Email, dsTxt)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&days, "days", 30, "Cutoff in days — members with lastLogin older than this are stale")
	cmd.Flags().StringVar(&dbPath, "db", "", "Override local SQLite path (default platform user data dir)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Cap the number of stale members returned (0 = unlimited)")
	return cmd
}

func stringFromAny(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func parseRFC3339(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

func countActivePlans(v any) int {
	arr, ok := v.([]any)
	if !ok {
		return 0
	}
	n := 0
	for _, item := range arr {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if active, _ := obj["active"].(bool); active {
			n++
			continue
		}
		if status, _ := obj["status"].(string); status == "ACTIVE" || status == "TRIALING" {
			n++
		}
	}
	return n
}
