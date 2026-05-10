// Hand-authored novel feature: surface accounts whose last_synced is past
// a threshold, grouped by institution, sorted by financial impact.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/monarch/internal/client"
)

func newAccountsStaleCmd(flags *rootFlags) *cobra.Command {
	var flagThreshold string
	var flagRefresh bool
	cmd := &cobra.Command{
		Use:   "stale",
		Short: "List accounts whose last_synced is past a threshold (silent connection failures)",
		Long: `Surfaces accounts the dashboard quietly hides — connections that broke
without notification. Sorted by absolute balance descending so big-balance
accounts surface first.`,
		Example:     "  monarch-pp-cli accounts stale --threshold 24h --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			dur, err := parseStaleThreshold(flagThreshold)
			if err != nil {
				return usageErr(err)
			}
			if dryRunOK(flags) {
				return emitDryRun(flags, novelDryRun{
					Command:    "accounts stale",
					Persona:    "Marcus — FIRE-track engineer with 14 accounts",
					Plan:       fmt.Sprintf("Filter accounts where now - updatedAt > %s, group by institution, sort by abs balance", dur),
					Operations: []string{"Web_GetAccountsPage"},
					Inputs:     map[string]string{"threshold": flagThreshold},
				})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := fetchGraphQL(c, client.AccountsListQuery, map[string]any{"filters": map[string]any{}})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var resp struct {
				Accounts []struct {
					ID             string  `json:"id"`
					DisplayName    string  `json:"displayName"`
					CurrentBalance float64 `json:"currentBalance"`
					Institution    *struct {
						Name string `json:"name"`
					} `json:"institution"`
					UpdatedAt    string `json:"updatedAt"`
					SyncDisabled bool   `json:"syncDisabled"`
					IsManual     bool   `json:"isManual"`
				} `json:"accounts"`
			}
			if err := json.Unmarshal(data, &resp); err != nil {
				return apiErr(fmt.Errorf("parsing accounts: %w", err))
			}
			cutoff := time.Now().UTC().Add(-dur)
			rows := []map[string]any{}
			for _, a := range resp.Accounts {
				if a.IsManual || a.SyncDisabled {
					continue
				}
				t, err := time.Parse(time.RFC3339, a.UpdatedAt)
				if err != nil {
					// Try alternate formats
					t, err = time.Parse("2006-01-02T15:04:05.000Z", a.UpdatedAt)
					if err != nil {
						continue
					}
				}
				if t.Before(cutoff) {
					inst := ""
					if a.Institution != nil {
						inst = a.Institution.Name
					}
					age := time.Since(t).Round(time.Hour)
					rows = append(rows, map[string]any{
						"id":          a.ID,
						"name":        a.DisplayName,
						"institution": inst,
						"balance":     a.CurrentBalance,
						"last_synced": a.UpdatedAt,
						"stale_for":   age.String(),
					})
				}
			}
			sort.Slice(rows, func(i, j int) bool {
				return absFloat(rows[i]["balance"].(float64)) > absFloat(rows[j]["balance"].(float64))
			})
			result := map[string]any{
				"threshold":      flagThreshold,
				"cutoff":         cutoff.Format(time.RFC3339),
				"stale_accounts": rows,
				"count":          len(rows),
			}
			if flagRefresh && len(rows) > 0 {
				result["refresh_requested"] = true
				result["refresh_note"] = "Refresh mutation wiring is a Phase 4+ task; the accounts refresh subcommand handles this surface explicitly."
			}
			return printJSONOrTable(flags, result)
		},
	}
	cmd.Flags().StringVar(&flagThreshold, "threshold", "24h", "Stale threshold (e.g. 24h, 48h, 7d)")
	cmd.Flags().BoolVar(&flagRefresh, "refresh", false, "Trigger a force-refresh on every stale account (uses accounts.refresh)")
	return cmd
}

func parseStaleThreshold(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "d") {
		var n int
		if _, err := fmt.Sscanf(s, "%dd", &n); err != nil {
			return 0, err
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

func absFloat(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}
