package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/youtube/internal/config"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/youtube/internal/quota"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/youtube/internal/store"

	"github.com/spf13/cobra"
)

// newQuotaCmd: parent for `quota plan`, `quota cost`, and `quota table`.
func newQuotaCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "quota",
		Short: "Plan and audit YouTube Data API quota usage",
		Long:  "Plan a command sequence's quota cost before running, or list per-endpoint costs from the documented YouTube Data API v3 cost map.",
	}
	cmd.AddCommand(newQuotaPlanCmd(flags))
	cmd.AddCommand(newQuotaTableCmd(flags))
	return cmd
}

func newQuotaPlanCmd(flags *rootFlags) *cobra.Command {
	var dailyBudget int
	var includeLedger bool
	var times int

	cmd := &cobra.Command{
		Use:   "plan <endpoint>...",
		Short: "Project quota cost for a planned command sequence",
		Long: `Project the total quota cost of a planned command sequence against
the YouTube Data API v3 cost map and (optionally) your local 24h ledger.

Pass endpoint identifiers as ` + "`resource.method`" + ` (e.g. videos.list,
search.list, comments.insert). Repeat one endpoint with --times to model
batch loops.

The cost map is the documented map at developers.google.com/youtube/v3/determine_quota_cost.`,
		Example: strings.Trim(`
  youtube-pp-cli quota plan search.list videos.list --json
  youtube-pp-cli quota plan search.list --times 10 --include-ledger --json
  youtube-pp-cli quota plan videos.insert --daily-budget 10000`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if times < 1 {
				times = 1
			}
			plan := []map[string]any{}
			total := 0
			for _, ep := range args {
				ep = strings.ToLower(strings.TrimSpace(ep))
				parts := strings.SplitN(ep, ".", 2)
				if len(parts) != 2 {
					return fmt.Errorf("invalid endpoint %q: expected resource.method (e.g. videos.list)", ep)
				}
				cost := quota.Cost(parts[0], parts[1]) * times
				plan = append(plan, map[string]any{
					"endpoint": ep,
					"unit":     quota.Cost(parts[0], parts[1]),
					"times":    times,
					"cost":     cost,
				})
				total += cost
			}

			result := map[string]any{
				"plan":         plan,
				"total_units":  total,
				"daily_budget": dailyBudget,
			}

			if includeLedger {
				cfg, _ := config.Load(flags.configPath)
				dbPath := defaultDBPath("youtube-pp-cli")
				if db, err := store.OpenWithContext(cmd.Context(), dbPath); err == nil {
					defer db.Close()
					if err := db.EnsureYouTubeExtras(cmd.Context()); err == nil {
						spent, _ := readSpent24h(cmd.Context(), db, hashKeyFromConfig(cfg))
						result["spent_last_24h"] = spent
						remaining := dailyBudget - spent
						result["remaining_budget"] = remaining
						if total > remaining {
							result["fits_today"] = false
							result["over_by"] = total - remaining
						} else {
							result["fits_today"] = true
						}
					}
				}
			}

			if dryRunOK(flags) {
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().IntVar(&times, "times", 1, "Multiply each endpoint's cost by N (model batch loops)")
	cmd.Flags().IntVar(&dailyBudget, "daily-budget", quota.DailyDefault, "Project daily quota in units")
	cmd.Flags().BoolVar(&includeLedger, "include-ledger", false, "Subtract last-24h spend from local quota_log")
	return cmd
}

func newQuotaTableCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "table",
		Short:       "List all known endpoint costs",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			t := quota.All()
			rows := make([]map[string]any, 0, len(t))
			keys := make([]string, 0, len(t))
			for k := range t {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				rows = append(rows, map[string]any{"endpoint": k, "units": t[k]})
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	return cmd
}

// newCostLedgerCmd: top-level `cost ledger` for the quota_log table.
func newCostLedgerCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cost",
		Short: "Inspect quota spend recorded by this CLI",
	}
	cmd.AddCommand(newCostLedgerListCmd(flags))
	cmd.AddCommand(newCostLedgerLogCmd(flags))
	return cmd
}

func newCostLedgerListCmd(flags *rootFlags) *cobra.Command {
	var window string
	cmd := &cobra.Command{
		Use:   "ledger",
		Short: "Show recent quota spend grouped by command and endpoint",
		Long: `Show the rolling per-API-key quota ledger, aggregated by command, endpoint,
and day. Populates from yt_quota_log, which is written by every quota-aware
command (search, videos list batch, etc.) when run with --record-quota.`,
		Example:     "  youtube-pp-cli cost ledger --last 24h --json --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			cfg, _ := config.Load(flags.configPath)
			dbPath := defaultDBPath("youtube-pp-cli")
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			if err := db.EnsureYouTubeExtras(cmd.Context()); err != nil {
				return err
			}
			cutoff, err := parseWindow(window)
			if err != nil {
				return err
			}
			rows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT command, endpoint, SUM(units) AS units, COUNT(*) AS calls
				   FROM yt_quota_log
				   WHERE api_key_hash = ? AND ts >= ?
				   GROUP BY command, endpoint
				   ORDER BY units DESC`,
				hashKeyFromConfig(cfg), cutoff.UTC().Format(time.RFC3339))
			if err != nil {
				return err
			}
			defer rows.Close()
			out := []map[string]any{}
			total := 0
			for rows.Next() {
				var command, endpoint string
				var units, calls int
				if err := rows.Scan(&command, &endpoint, &units, &calls); err != nil {
					return err
				}
				out = append(out, map[string]any{
					"command":  command,
					"endpoint": endpoint,
					"units":    units,
					"calls":    calls,
				})
				total += units
			}
			env := map[string]any{
				"window_starts_at": cutoff.UTC().Format(time.RFC3339),
				"api_key_hash":     hashKeyFromConfig(cfg),
				"total_units":      total,
				"daily_budget":     quota.DailyDefault,
				"by_command":       out,
			}
			return printJSONFiltered(cmd.OutOrStdout(), env, flags)
		},
	}
	cmd.Flags().StringVar(&window, "last", "24h", "Window to aggregate (e.g. 1h, 24h, 7d)")
	return cmd
}

// hidden: `cost log` records a manual entry into the ledger. Useful for users
// running raw curl outside this CLI who still want one source of truth.
func newCostLedgerLogCmd(flags *rootFlags) *cobra.Command {
	var endpoint, command string
	var units int
	cmd := &cobra.Command{
		Use:   "log",
		Short: "Manually record a quota-spending call into the ledger",
		RunE: func(cmd *cobra.Command, args []string) error {
			if endpoint == "" {
				return fmt.Errorf("--endpoint is required (e.g. videos.list)")
			}
			if dryRunOK(flags) {
				return nil
			}
			cfg, _ := config.Load(flags.configPath)
			dbPath := defaultDBPath("youtube-pp-cli")
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			if err := db.EnsureYouTubeExtras(cmd.Context()); err != nil {
				return err
			}
			parts := strings.SplitN(endpoint, ".", 2)
			if units == 0 && len(parts) == 2 {
				units = quota.Cost(parts[0], parts[1])
			}
			if err := db.LogQuota(cmd.Context(), hashKeyFromConfig(cfg), command, endpoint, units, 0, "manual"); err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"logged": true, "endpoint": endpoint, "units": units}, flags)
		},
	}
	cmd.Flags().StringVar(&endpoint, "endpoint", "", "Endpoint identifier (resource.method)")
	cmd.Flags().StringVar(&command, "command", "manual", "Command label for the ledger row")
	cmd.Flags().IntVar(&units, "units", 0, "Override unit cost (default: documented cost for endpoint)")
	return cmd
}

func hashKeyFromConfig(cfg *config.Config) string {
	if cfg == nil {
		return "anon"
	}
	if cfg.APIKey != "" {
		return quota.HashKey(cfg.APIKey)
	}
	return "oauth-or-anon"
}

func readSpent24h(ctx context.Context, db *store.Store, keyHash string) (int, error) {
	cutoff := time.Now().Add(-24 * time.Hour).UTC().Format(time.RFC3339)
	row := db.DB().QueryRowContext(ctx,
		`SELECT COALESCE(SUM(units),0) FROM yt_quota_log WHERE api_key_hash = ? AND ts >= ?`,
		keyHash, cutoff)
	var n int
	if err := row.Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// parseWindow accepts shorthand windows like "24h", "7d", "30m" and returns
// the cutoff timestamp.
func parseWindow(s string) (time.Time, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		s = "24h"
	}
	if strings.HasSuffix(s, "d") {
		days := 0
		if _, err := fmt.Sscanf(s, "%dd", &days); err != nil || days <= 0 {
			return time.Time{}, fmt.Errorf("invalid window %q: expected e.g. 7d", s)
		}
		return time.Now().Add(-time.Duration(days) * 24 * time.Hour), nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid window %q: %w", s, err)
	}
	return time.Now().Add(-d), nil
}

var _ = json.RawMessage(nil) // satisfy import even when callers shrink
