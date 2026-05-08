// Compound command: quota forecast.
// Hand-built transcendence feature — not generated from OpenAPI.

package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"time"

	"multimail-pp-cli/internal/store"
	"github.com/spf13/cobra"
)

type quotaForecast struct {
	Plan            string  `json:"plan"`
	QuotaLimit      int     `json:"quota_limit"`
	Used            int     `json:"used"`
	Remaining       int     `json:"remaining"`
	UsedPercent     float64 `json:"used_percent"`
	DailyRate       float64 `json:"daily_rate"`
	DaysRemaining   int     `json:"days_remaining"`
	ExhaustionDate  string  `json:"exhaustion_date,omitempty"`
	Confidence      string  `json:"confidence"`
	SampleDays      int     `json:"sample_days"`
	Recommendation  string  `json:"recommendation"`
}

func newQuotaForecastCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var sampleDays int

	cmd := &cobra.Command{
		Use:   "forecast",
		Short: "Predict when email quota will be exhausted",
		Long: `Predict when your email quota will be exhausted based on rolling
send rate. Shows days remaining with confidence interval.

Requires synced data — run 'multimail-pp-cli sync' first.`,
		Example: `  # Quota forecast with default 30-day sample
  multimail-pp-cli quota forecast

  # Forecast based on last 7 days of activity
  multimail-pp-cli quota forecast --sample-days 7

  # JSON for alerting pipelines
  multimail-pp-cli quota forecast --json | jq '.days_remaining'`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("multimail-pp-cli")
			}

			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'multimail-pp-cli sync' first.", err)
			}
			defer db.Close()

			forecast, err := computeQuotaForecast(db, sampleDays)
			if err != nil {
				return err
			}

			jsonMode := flags.asJSON || !isTerminal(cmd.OutOrStdout())
			if jsonMode {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(forecast)
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Quota Forecast")
			fmt.Fprintln(cmd.OutOrStdout(), "==============")
			fmt.Fprintf(cmd.OutOrStdout(), "Plan:           %s\n", forecast.Plan)
			fmt.Fprintf(cmd.OutOrStdout(), "Used:           %d / %d (%.1f%%)\n", forecast.Used, forecast.QuotaLimit, forecast.UsedPercent)
			fmt.Fprintf(cmd.OutOrStdout(), "Remaining:      %d emails\n", forecast.Remaining)
			fmt.Fprintf(cmd.OutOrStdout(), "Daily Rate:     %.1f emails/day (based on %d-day sample)\n", forecast.DailyRate, forecast.SampleDays)
			fmt.Fprintf(cmd.OutOrStdout(), "Days Remaining: %d (%s confidence)\n", forecast.DaysRemaining, forecast.Confidence)
			if forecast.ExhaustionDate != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Exhaustion:     %s\n", forecast.ExhaustionDate)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n", forecast.Recommendation)
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&sampleDays, "sample-days", 30, "Days of send history to sample for rate calculation")

	return cmd
}

func newQuotaCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "quota",
		Short: "Quota management and forecasting",
		Long:  `Quota-related commands: forecast quota exhaustion based on send rate.`,
	}

	cmd.AddCommand(newQuotaForecastCmd(flags))
	return cmd
}

func computeQuotaForecast(db *store.Store, sampleDays int) (*quotaForecast, error) {
	sqlDB := db.DB()

	// Get account info for quota limits
	var plan string
	var sentThisMonth, monthlyQuota int

	row := sqlDB.QueryRow(`SELECT
		COALESCE(json_extract(data, '$.plan'), 'unknown'),
		COALESCE(json_extract(data, '$.emails_sent_this_month'), 0),
		COALESCE(json_extract(data, '$.monthly_email_quota'), 0)
		FROM account LIMIT 1`)
	if err := row.Scan(&plan, &sentThisMonth, &monthlyQuota); err != nil {
		return nil, fmt.Errorf("no account data found. Run 'multimail-pp-cli sync' first")
	}

	if monthlyQuota == 0 {
		return &quotaForecast{
			Plan:           plan,
			QuotaLimit:     0,
			Used:           sentThisMonth,
			Remaining:      0,
			Recommendation: "No quota limit found. Check your plan details.",
			Confidence:     "none",
		}, nil
	}

	remaining := monthlyQuota - sentThisMonth
	if remaining < 0 {
		remaining = 0
	}
	usedPercent := float64(sentThisMonth) / float64(monthlyQuota) * 100

	// Calculate daily send rate from recent email history
	sampleStart := time.Now().AddDate(0, 0, -sampleDays)
	var emailCount int
	err := sqlDB.QueryRow(`SELECT COUNT(*) FROM emails WHERE
		json_extract(data, '$.direction') = 'outbound' AND
		json_extract(data, '$.received_at') >= ?`,
		sampleStart.Format(time.RFC3339)).Scan(&emailCount)
	if err != nil {
		emailCount = 0
	}

	dailyRate := 0.0
	confidence := "low"
	daysRemaining := 999
	exhaustionDate := ""

	if emailCount > 0 {
		dailyRate = float64(emailCount) / float64(sampleDays)

		if emailCount >= 30 {
			confidence = "high"
		} else if emailCount >= 7 {
			confidence = "medium"
		}

		if dailyRate > 0 {
			daysRemaining = int(math.Ceil(float64(remaining) / dailyRate))
			if daysRemaining > 365 {
				daysRemaining = 365
			}
			exhaustion := time.Now().AddDate(0, 0, daysRemaining)
			exhaustionDate = exhaustion.Format("2006-01-02")
		}
	} else {
		// No send history — can't forecast
		confidence = "none"
		if sentThisMonth > 0 {
			// Estimate from current month's usage
			now := time.Now()
			dayOfMonth := now.Day()
			if dayOfMonth > 0 {
				dailyRate = float64(sentThisMonth) / float64(dayOfMonth)
				if dailyRate > 0 {
					daysRemaining = int(math.Ceil(float64(remaining) / dailyRate))
					exhaustion := time.Now().AddDate(0, 0, daysRemaining)
					exhaustionDate = exhaustion.Format("2006-01-02")
					confidence = "low"
				}
			}
		}
	}

	recommendation := quotaRecommendation(usedPercent, daysRemaining, dailyRate)

	return &quotaForecast{
		Plan:           plan,
		QuotaLimit:     monthlyQuota,
		Used:           sentThisMonth,
		Remaining:      remaining,
		UsedPercent:    usedPercent,
		DailyRate:      math.Round(dailyRate*10) / 10,
		DaysRemaining:  daysRemaining,
		ExhaustionDate: exhaustionDate,
		Confidence:     confidence,
		SampleDays:     sampleDays,
		Recommendation: recommendation,
	}, nil
}

func quotaRecommendation(usedPercent float64, daysRemaining int, dailyRate float64) string {
	if usedPercent >= 90 {
		return "Critical: quota nearly exhausted. Consider upgrading your plan immediately."
	}
	if usedPercent >= 75 {
		return "Warning: quota usage above 75%. Monitor closely or consider upgrading."
	}
	if daysRemaining <= 7 && dailyRate > 0 {
		return fmt.Sprintf("At current rate (%.1f/day), quota will be exhausted in %d days. Consider reducing volume or upgrading.", dailyRate, daysRemaining)
	}
	if daysRemaining <= 14 && dailyRate > 0 {
		return fmt.Sprintf("Quota on track to last %d more days at current rate. Plan ahead for next billing cycle.", daysRemaining)
	}
	return "Quota is healthy. No action needed."
}
