// Copyright 2026 joltsconsulting. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/adminbyrequest/internal/store"
	"github.com/spf13/cobra"
)

// dailyQuotaLimit is the documented daily quota for the Admin By Request API.
// AbR blocks the tenant when this is hit until next business day.
const dailyQuotaLimit = 100000

type quotaSnapshot struct {
	Day              string  `json:"day"`
	CallsToday       int     `json:"calls_today"`
	Limit            int     `json:"limit"`
	PercentUsed      float64 `json:"percent_used"`
	HoursElapsed     float64 `json:"hours_elapsed"`
	ProjectedTotal   int     `json:"projected_total,omitempty"`
	ProjectedAtRisk  bool    `json:"projected_at_risk,omitempty"`
	RecommendedSleep string  `json:"recommended_sleep,omitempty"`
	Source           string  `json:"source"`
}

func newQuotaCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "quota",
		Short: "Local tracking of AdminByRequest's 100,000-call daily quota",
		Long: `Admin By Request enforces a hard 100,000-call per-tenant daily quota.
Exceeding it blocks the tenant until next business day. The CLI counts every
API call it makes locally and exposes the running count plus a forecast.`,
	}
	cmd.AddCommand(newQuotaShowCmd(flags))
	cmd.AddCommand(newQuotaForecastCmd(flags))
	return cmd
}

func newQuotaShowCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:     "show",
		Short:   "Print current local quota usage",
		Example: "  adminbyrequest-pp-cli quota show --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			snap, err := computeQuotaSnapshot(cmd.Context(), dbPath, false)
			if err != nil {
				return err
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), snap, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Quota for %s: %d / %d (%.1f%%) - source: %s\n",
				snap.Day, snap.CallsToday, snap.Limit, snap.PercentUsed, snap.Source)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard CLI location)")
	return cmd
}

func newQuotaForecastCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:     "forecast",
		Short:   "Forecast end-of-day quota burn based on the current rate",
		Example: "  adminbyrequest-pp-cli quota forecast --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			snap, err := computeQuotaSnapshot(cmd.Context(), dbPath, true)
			if err != nil {
				return err
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), snap, flags)
			}
			risk := "(within quota)"
			if snap.ProjectedAtRisk {
				risk = "(over quota by EOD)"
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"Forecast for %s: %d calls in %.1fh -> projected %d / %d (%.1f%%) %s\n",
				snap.Day, snap.CallsToday, snap.HoursElapsed, snap.ProjectedTotal, snap.Limit,
				snap.PercentUsed, risk)
			if snap.RecommendedSleep != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Suggested cooldown between calls: %s\n", snap.RecommendedSleep)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard CLI location)")
	return cmd
}

// computeQuotaSnapshot counts today's API calls. Prefers the http-cache
// directory (every cached GET = one call routed through client.Get); falls
// back to sync_state writes; finally degrades gracefully if neither exists.
func computeQuotaSnapshot(ctx context.Context, dbPath string, withForecast bool) (quotaSnapshot, error) {
	day := time.Now().Format("2006-01-02")
	startOfDay, _ := time.Parse("2006-01-02", day)
	hoursElapsed := time.Since(startOfDay).Hours()
	if hoursElapsed < 0.01 {
		hoursElapsed = 0.01
	}

	calls, source := 0, "unavailable"

	// 1) http-cache directory: every cached GET leaves a file. Mtime today == call today.
	if home, err := os.UserHomeDir(); err == nil {
		cacheDir := filepath.Join(home, ".cache", "adminbyrequest-pp-cli", "http")
		if st, errStat := os.Stat(cacheDir); errStat == nil && st.IsDir() {
			source = "http-cache"
			_ = filepath.Walk(cacheDir, func(_ string, info os.FileInfo, walkErr error) error {
				if walkErr != nil || info == nil || info.IsDir() {
					return nil
				}
				if info.ModTime().Format("2006-01-02") == day {
					calls++
				}
				return nil
			})
		}
	}

	// 2) Fallback: sync_state writes today.
	if source == "unavailable" {
		if dbPath == "" {
			dbPath = defaultDBPath("adminbyrequest-pp-cli")
		}
		db, err := store.OpenWithContext(ctx, dbPath)
		if err == nil {
			defer db.Close()
			row := db.DB().QueryRowContext(ctx,
				`SELECT COUNT(*) FROM sync_state WHERE DATE(updated_at) = ?`, day)
			if scanErr := row.Scan(&calls); scanErr == nil {
				source = "sync-state"
			}
		}
	}

	snap := quotaSnapshot{
		Day:          day,
		CallsToday:   calls,
		Limit:        dailyQuotaLimit,
		PercentUsed:  100.0 * float64(calls) / float64(dailyQuotaLimit),
		HoursElapsed: hoursElapsed,
		Source:       source,
	}
	if withForecast && hoursElapsed > 0 {
		rate := float64(calls) / hoursElapsed
		projected := int(rate * 24.0)
		snap.ProjectedTotal = projected
		snap.ProjectedAtRisk = projected > dailyQuotaLimit
		if snap.ProjectedAtRisk {
			remHours := 24.0 - hoursElapsed
			if remHours < 0.01 {
				remHours = 0.01
			}
			remCalls := dailyQuotaLimit - calls
			if remCalls <= 0 {
				snap.RecommendedSleep = "stop until next business day"
			} else {
				maxRate := float64(remCalls) / remHours // calls/hour we can still make
				if maxRate > 0 {
					sleepSeconds := 3600.0 / maxRate
					snap.RecommendedSleep = time.Duration(sleepSeconds * float64(time.Second)).Round(time.Second).String()
				} else {
					snap.RecommendedSleep = "stop until next business day"
				}
			}
		}
	}
	return snap, nil
}
