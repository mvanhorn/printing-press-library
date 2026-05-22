// Hand-authored: novel feature `credits burn`.
//
// Live /account-information probe plus a derived "rows enriched per day"
// series from outreach.people.updated_at. No audit tables required.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/prospeo/internal/supa"
)

// BurnDayRow is one row of the per-day rows-enriched series.
type BurnDayRow struct {
	Day          string `json:"day"`
	RowsEnriched int    `json:"rows_enriched"`
}

// BurnReport is the credits-burn response shape.
type BurnReport struct {
	Plan                string       `json:"plan,omitempty"`
	RemainingCredits    int          `json:"remaining_credits"`
	UsedCredits         int          `json:"used_credits"`
	NextRenewalDays     int          `json:"next_renewal_days,omitempty"`
	NextRenewalDate     string       `json:"next_renewal_date,omitempty"`
	EnrichedPerDay      []BurnDayRow `json:"enriched_per_day"`
	AverageRowsPerDay   float64      `json:"average_rows_per_day"`
	ProjectedDaysToZero *int         `json:"projected_days_to_zero,omitempty"`
	Note                string       `json:"note,omitempty"`
}

func newCreditsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "credits",
		Short: "Credit-usage reports combining live /account-information with derived enrichment activity from outreach.people.",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newCreditsBurnCmd(flags))
	return cmd
}

func newCreditsBurnCmd(flags *rootFlags) *cobra.Command {
	var days int
	cmd := &cobra.Command{
		Use:         "burn",
		Short:       "Daily rows-enriched rate and projected credit runway.",
		Example:     "  prospeo-pp-cli credits burn --days 14",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would call /account-information and aggregate outreach.people for last %d days\n", days)
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			raw, err := c.Get(cmd.Context(), "/account-information", nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			report, err := buildBurnReportLive(raw)
			if err != nil {
				return err
			}

			// Try to enrich with derived per-day series; degrade gracefully.
			if supa.IsConfigured() {
				cfg, lerr := supa.LoadConfig()
				if lerr == nil {
					sc := supa.New(cfg)
					rows, ferr := fetchPeopleActivity(cmd.Context(), sc, days)
					if ferr != nil {
						body := ferr.Error()
						if strings.Contains(body, "PGRST106") || strings.Contains(body, "404") {
							report.Note = "outreach schema not reachable; see README setup step"
						} else {
							report.Note = "outreach query failed: " + body
						}
					} else {
						bk := map[string]int{}
						for _, r := range rows {
							if len(r.UpdatedAt) < 10 {
								continue
							}
							bk[r.UpdatedAt[:10]]++
						}
						series := make([]BurnDayRow, 0, len(bk))
						for d, n := range bk {
							series = append(series, BurnDayRow{Day: d, RowsEnriched: n})
						}
						sort.Slice(series, func(i, j int) bool { return series[i].Day < series[j].Day })
						report.EnrichedPerDay = series
						if len(series) > 0 {
							total := 0
							for _, r := range series {
								total += r.RowsEnriched
							}
							report.AverageRowsPerDay = float64(total) / float64(len(series))
							if report.AverageRowsPerDay > 0 && report.RemainingCredits > 0 {
								d := int(float64(report.RemainingCredits) / report.AverageRowsPerDay)
								report.ProjectedDaysToZero = &d
							}
						}
					}
				}
			} else {
				report.Note = "Supabase not configured; per-day enrichment series omitted"
			}
			if report.EnrichedPerDay == nil {
				report.EnrichedPerDay = []BurnDayRow{}
			}
			return printJSONFiltered(cmd.OutOrStdout(), report, flags)
		},
	}
	cmd.Flags().IntVar(&days, "days", 14, "Look back this many days when computing the enrichment series.")
	return cmd
}

func buildBurnReportLive(raw []byte) (*BurnReport, error) {
	// /account-information response shape varies; pull common fields defensively.
	var env map[string]any
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("decode account-information: %w", err)
	}
	report := &BurnReport{}
	// Common Prospeo shape: {plan: "...", remaining_credits: N, used_credits: N, next_renewal_date: "...", ...}
	if v, ok := env["plan"].(string); ok {
		report.Plan = v
	}
	if v, ok := env["remaining_credits"].(float64); ok {
		report.RemainingCredits = int(v)
	}
	if v, ok := env["used_credits"].(float64); ok {
		report.UsedCredits = int(v)
	}
	if v, ok := env["next_renewal_date"].(string); ok {
		report.NextRenewalDate = v
		if t, err := time.Parse("2006-01-02", v); err == nil {
			report.NextRenewalDays = int(time.Until(t).Hours() / 24)
		} else if t, err := time.Parse(time.RFC3339, v); err == nil {
			report.NextRenewalDays = int(time.Until(t).Hours() / 24)
		}
	}
	return report, nil
}
