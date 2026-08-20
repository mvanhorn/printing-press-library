package cli

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	tpkg "github.com/mvanhorn/printing-press-library/library/marketing/trustpilot/internal/trustpilot"
)

func newTPSurgeCmd(flags *rootFlags) *cobra.Command {
	var baseline, window string
	var stars int
	cmd := &cobra.Command{
		Use:   "surge <domain>",
		Short: "Detect a review-volume or 1-star surge against a rolling baseline",
		Example: `  trustpilot-pp-cli surge www.thriftbooks.com --baseline 90d --window 7d --json
  trustpilot-pp-cli surge bookshop.org --stars 1`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			domain := normalizeDomain(args[0])
			baseDays := windowDays(baseline)
			recentDays := windowDays(window)
			if baseDays <= 0 || recentDays <= 0 {
				return fmt.Errorf("baseline and window must be positive windows like 90d or 7d")
			}
			ctx := cmd.Context()
			db, err := openTPStore(ctx)
			if err != nil {
				return err
			}
			defer db.Close()
			now := time.Now().UTC()
			starFilter := []int(nil)
			if stars > 0 {
				starFilter = []int{stars}
			}
			baselineReviews, err := tpkg.QueryReviews(ctx, db, tpkg.QueryFilters{
				Domain: domain, Stars: starFilter, MinDate: now.AddDate(0, 0, -baseDays), MaxDate: now.AddDate(0, 0, -recentDays),
			})
			if err != nil {
				return err
			}
			recentReviews, err := tpkg.QueryReviews(ctx, db, tpkg.QueryFilters{
				Domain: domain, Stars: starFilter, MinDate: now.AddDate(0, 0, -recentDays),
			})
			if err != nil {
				return err
			}
			baselineSpan := baseDays - recentDays
			if baselineSpan <= 0 {
				baselineSpan = baseDays
			}
			expected := float64(len(baselineReviews)) * float64(recentDays) / float64(baselineSpan)
			z := 0.0
			verdict := "normal"
			if len(baselineReviews) < 5 {
				verdict = "insufficient_data"
			} else if expected > 0 {
				z = (float64(len(recentReviews)) - expected) / math.Sqrt(expected)
				if z >= 2 {
					verdict = "surge"
				}
			}
			payload := map[string]any{
				"domain":             domain,
				"baselineDays":       baseDays,
				"windowDays":         recentDays,
				"baselineCount":      len(baselineReviews),
				"windowCount":        len(recentReviews),
				"baselineRatePerDay": float64(len(baselineReviews)) / float64(baselineSpan),
				"windowRatePerDay":   float64(len(recentReviews)) / float64(recentDays),
				"zScore":             z,
				"verdict":            verdict,
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, payload)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s: %s (z=%.2f, recent=%d, baseline=%d)\n", domain, verdict, z, len(recentReviews), len(baselineReviews))
			return nil
		},
	}
	cmd.Flags().StringVar(&baseline, "baseline", "90d", "Baseline window")
	cmd.Flags().StringVar(&window, "window", "7d", "Recent comparison window")
	cmd.Flags().IntVar(&stars, "stars", 0, "Star rating to analyze (0 = total volume)")
	return cmd
}

func windowDays(window string) int {
	w := strings.TrimSpace(strings.ToLower(window))
	if len(w) < 2 {
		return 0
	}
	n, err := strconv.Atoi(w[:len(w)-1])
	if err != nil || n <= 0 {
		return 0
	}
	switch w[len(w)-1] {
	case 'd':
		return n
	case 'w':
		return n * 7
	case 'm':
		return n * 30
	case 'y':
		return n * 365
	}
	return 0
}
