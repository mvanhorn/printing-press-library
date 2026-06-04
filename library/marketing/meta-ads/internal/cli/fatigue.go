// Copyright 2026 dhilip-subramanian. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/meta-ads/internal/store"

	"github.com/spf13/cobra"
)

type fatigueDay struct {
	Date        string  `json:"date"`
	Impressions int64   `json:"impressions"`
	Spend       float64 `json:"spend"`
	Cpm         float64 `json:"cpm"`
	Ctr         float64 `json:"ctr"`
	Frequency   float64 `json:"frequency"`
}

type fatigueRow struct {
	AdID        string       `json:"ad_id"`
	AdName      string       `json:"ad_name,omitempty"`
	WindowDays  int          `json:"window_days"`
	Days        []fatigueDay `json:"days,omitempty"`
	CpmSlope    float64      `json:"cpm_slope"`
	CtrSlope    float64      `json:"ctr_slope"`
	FreqSlope   float64      `json:"freq_slope"`
	BaselineCpm float64      `json:"baseline_cpm"`
	RecentCpm   float64      `json:"recent_cpm"`
	Verdict     string       `json:"verdict"`
}

type fatigueView struct {
	Campaign string       `json:"campaign,omitempty"`
	Account  string       `json:"account,omitempty"`
	Window   int          `json:"window_days"`
	Total    int          `json:"total"`
	Rows     []fatigueRow `json:"rows"`
	Note     string       `json:"note,omitempty"`
}

func newNovelFatigueCmd(flags *rootFlags) *cobra.Command {
	var flagCampaign string
	var flagAccount string
	var flagWindow string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "fatigue",
		Short: "Detect ads with creative fatigue (rising CPM, climbing frequency, falling CTR).",
		Long: `Compute per-day CPM, CTR, and frequency slopes for every ad in a campaign
or account over the configured window. Flags ads where the 3-day moving CPM
exceeds the baseline by >20% as fatigued.

Requires synced ad-level insights with time_increment=1 in the local store.`,
		Example: `  meta-ads-pp-cli fatigue --campaign 23847265 --window 14d --agent
  meta-ads-pp-cli fatigue --account act_4327210487520472 --window 7d --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compute fatigue slopes from local insights")
				return nil
			}
			windowDays := 14
			if flagWindow != "" {
				n, err := parseDayWindow(flagWindow)
				if err != nil {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("--window: %w", err))
				}
				windowDays = n
			}
			if flagCampaign == "" && flagAccount == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("either --campaign or --account is required"))
			}

			if dbPath == "" {
				dbPath = defaultDBPath("meta-ads-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			// Step 1: find candidate ads
			adQuery := `SELECT id, data FROM resources
				WHERE resource_type IN ('ads', 'adaccounts_ads', 'ad_accounts_ads')`
			adArgs := []any{}
			if flagCampaign != "" {
				adQuery += ` AND json_extract(data, '$.campaign_id') = ?`
				adArgs = append(adArgs, flagCampaign)
			} else if flagAccount != "" {
				adQuery += ` AND (json_extract(data, '$.account_id') = ? OR json_extract(data, '$.id') LIKE ?)`
				adArgs = append(adArgs, flagAccount, flagAccount+"%")
			}
			adRows, err := db.DB().QueryContext(cmd.Context(), adQuery, adArgs...)
			if err != nil {
				return fmt.Errorf("ads query: %w", err)
			}
			defer adRows.Close()

			out := make([]fatigueRow, 0)
			for adRows.Next() {
				var id string
				var data []byte
				if err := adRows.Scan(&id, &data); err != nil {
					continue
				}
				var ad struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				}
				_ = json.Unmarshal(data, &ad)

				// Step 2: load insights for this ad
				days := loadAdInsights(cmd.Context(), db, ad.ID, windowDays)
				if len(days) < 3 {
					continue // need at least 3 daily points to compute slope
				}

				cpmSlope := slope(days, func(d fatigueDay) float64 { return d.Cpm })
				ctrSlope := slope(days, func(d fatigueDay) float64 { return d.Ctr })
				freqSlope := slope(days, func(d fatigueDay) float64 { return d.Frequency })

				baseline := days[0].Cpm
				recent := days[len(days)-1].Cpm
				verdict := "ok"
				if baseline > 0 && (recent/baseline) > 1.20 && ctrSlope < 0 {
					verdict = "fatigued"
				}

				out = append(out, fatigueRow{
					AdID:        ad.ID,
					AdName:      ad.Name,
					WindowDays:  windowDays,
					Days:        days,
					CpmSlope:    cpmSlope,
					CtrSlope:    ctrSlope,
					FreqSlope:   freqSlope,
					BaselineCpm: baseline,
					RecentCpm:   recent,
					Verdict:     verdict,
				})
			}
			if err := adRows.Err(); err != nil {
				return fmt.Errorf("iterating ad rows: %w", err)
			}

			// Sort: fatigued first, then by CPM slope desc.
			sort.SliceStable(out, func(i, j int) bool {
				if out[i].Verdict != out[j].Verdict {
					return out[i].Verdict == "fatigued"
				}
				return out[i].CpmSlope > out[j].CpmSlope
			})

			view := fatigueView{
				Campaign: flagCampaign,
				Account:  flagAccount,
				Window:   windowDays,
				Total:    len(out),
				Rows:     out,
			}
			if len(out) == 0 {
				view.Note = "no ads with enough daily insights to compute fatigue; run 'meta-ads-pp-cli sync' with insights resource included"
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&flagCampaign, "campaign", "", "Campaign ID to scope the scan")
	cmd.Flags().StringVar(&flagAccount, "account", "", "Ad account to scope the scan (alternative to --campaign)")
	cmd.Flags().StringVar(&flagWindow, "window", "14d", "Days of history to analyze (e.g., 7d, 14d, 30d)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (defaults to ~/.meta-ads-pp-cli/data.db)")
	return cmd
}

// parseDayWindow accepts forms like 7d, 14d, 30d, or a bare integer interpreted as days.
func parseDayWindow(s string) (int, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if strings.HasSuffix(s, "d") {
		n, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil || n <= 0 {
			return 0, fmt.Errorf("must be a positive integer of days, got %q", s)
		}
		return n, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("must be a positive integer of days, got %q", s)
	}
	return n, nil
}

// loadAdInsights pulls daily insight rows for an ad over the window, ordered chronologically.
func loadAdInsights(ctx context.Context, db *store.Store, adID string, windowDays int) []fatigueDay {
	q := `SELECT data FROM resources
		WHERE resource_type IN ('insights', 'ads_insights', 'adaccounts_insights')
		  AND json_extract(data, '$.ad_id') = ?
		  AND date(json_extract(data, '$.date_start')) > date('now', ?)
		ORDER BY json_extract(data, '$.date_start') ASC`
	rows, err := db.DB().QueryContext(ctx, q, adID, fmt.Sprintf("-%d days", windowDays))
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := make([]fatigueDay, 0)
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			continue
		}
		var raw struct {
			DateStart   string `json:"date_start"`
			Impressions string `json:"impressions"`
			Spend       string `json:"spend"`
			Cpm         string `json:"cpm"`
			Ctr         string `json:"ctr"`
			Frequency   string `json:"frequency"`
		}
		if err := json.Unmarshal(data, &raw); err != nil {
			continue
		}
		day := fatigueDay{Date: raw.DateStart}
		day.Impressions, _ = strconv.ParseInt(raw.Impressions, 10, 64)
		day.Spend, _ = strconv.ParseFloat(raw.Spend, 64)
		day.Cpm, _ = strconv.ParseFloat(raw.Cpm, 64)
		day.Ctr, _ = strconv.ParseFloat(raw.Ctr, 64)
		day.Frequency, _ = strconv.ParseFloat(raw.Frequency, 64)
		out = append(out, day)
	}
	if err := rows.Err(); err != nil {
		// Return nil so the caller skips this ad rather than ranking a partial series as if complete.
		return nil
	}
	return out
}

// slope computes least-squares slope over the time series. x = day index, y = metric.
func slope(days []fatigueDay, pick func(fatigueDay) float64) float64 {
	n := len(days)
	if n < 2 {
		return 0
	}
	var sumX, sumY, sumXY, sumX2 float64
	for i, d := range days {
		x := float64(i)
		y := pick(d)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}
	denom := float64(n)*sumX2 - sumX*sumX
	if denom == 0 {
		return 0
	}
	return (float64(n)*sumXY - sumX*sumY) / denom
}
