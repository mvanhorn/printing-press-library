// Hand-authored: novel feature `idle` — surface running computers
// whose last CLI-recorded action is older than a threshold. Created
// during Phase 3 build; survives regen because it lives in its own file.
package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/cloud/orgo/internal/store"

	"github.com/spf13/cobra"
)

func newIdleCmd(flags *rootFlags) *cobra.Command {
	var thresholdHours float64
	var workspace string

	cmd := &cobra.Command{
		Use:   "idle",
		Short: "Sort running computers by hours-since-last-CLI-action, surfacing burns that could be stopped.",
		Long: `Walk every computer in the local store, join against the actions
ledger, and surface the ones that have been running longer than the
threshold without any CLI-recorded activity.

Computers must be in the local store first — run 'orgo-pp-cli sync' or
hit any computer once via 'computers get' to populate the cache.`,
		Example: strings.Trim(`
  # Default: anything running idle for more than 24 hours
  orgo-pp-cli idle

  # Tighter threshold for a daily cost pass
  orgo-pp-cli idle --threshold-hours 8

  # JSON for piping into automation
  orgo-pp-cli idle --agent --select id,name,hours_idle
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			dbPath := defaultDBPath("orgo-pp-cli")
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			computerRows, source, err := loadFleetComputers(flags, db)
			if err != nil {
				return fmt.Errorf("loading computers: %w", err)
			}
			if len(computerRows) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "No computers found locally or via the live API.")
				return notFoundErr(fmt.Errorf("no computers found"))
			}
			_ = source // provenance label, unused for now

			lastTimes, err := db.LastActionTimes(cmd.Context())
			if err != nil {
				return fmt.Errorf("loading last action times: %w", err)
			}

			now := time.Now().UTC()
			cutoff := time.Duration(thresholdHours * float64(time.Hour))

			type idleRow struct {
				ID           string  `json:"id"`
				Name         string  `json:"name"`
				Status       string  `json:"status"`
				HoursIdle    float64 `json:"hours_idle"`
				LastActionAt string  `json:"last_action_at,omitempty"`
				WorkspaceID  string  `json:"workspace_id,omitempty"`
			}

			var rows []idleRow
			for _, raw := range computerRows {
				var obj map[string]any
				if err := json.Unmarshal(raw, &obj); err != nil {
					continue
				}
				id := stringField(obj, "id")
				if id == "" {
					continue
				}
				status := stringField(obj, "status")
				if status != "running" {
					continue
				}
				if workspace != "" && stringField(obj, "workspace_id") != workspace {
					continue
				}

				var hoursIdle float64
				lastActionStr := ""
				if last, ok := lastTimes[id]; ok && !last.IsZero() {
					age := now.Sub(last)
					hoursIdle = age.Hours()
					lastActionStr = last.UTC().Format(time.RFC3339)
					if age < cutoff {
						continue
					}
				} else {
					// Never seen — treat as infinitely idle so it sorts first.
					hoursIdle = -1
				}

				rows = append(rows, idleRow{
					ID:           id,
					Name:         stringField(obj, "name"),
					Status:       status,
					HoursIdle:    hoursIdle,
					LastActionAt: lastActionStr,
					WorkspaceID:  stringField(obj, "workspace_id"),
				})
			}

			// Sort by hours-idle desc; -1 (never) sorts first because it's
			// the strongest signal of waste.
			sort.SliceStable(rows, func(i, j int) bool {
				ai, aj := rows[i].HoursIdle, rows[j].HoursIdle
				if ai < 0 && aj >= 0 {
					return true
				}
				if aj < 0 && ai >= 0 {
					return false
				}
				return ai > aj
			})

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "No idle computers above the threshold.")
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, bold("ID")+"\t"+bold("NAME")+"\t"+bold("STATUS")+"\t"+bold("HOURS_IDLE")+"\t"+bold("LAST_ACTION_AT"))
			for _, r := range rows {
				idle := fmt.Sprintf("%.1f", r.HoursIdle)
				if r.HoursIdle < 0 {
					idle = "never"
				}
				last := r.LastActionAt
				if last == "" {
					last = "—"
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", truncate(r.ID, 12), r.Name, r.Status, idle, last)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().Float64Var(&thresholdHours, "threshold-hours", 24, "Hide computers idle for fewer than this many hours")
	cmd.Flags().StringVar(&workspace, "workspace", "", "Filter by workspace ID")

	return cmd
}

// stringField fetches a JSON object field as a string with fall-through to
// snake/camel case rendering. Empty when absent or the wrong type.
func stringField(obj map[string]any, key string) string {
	v := store.LookupFieldValue(obj, key)
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}
