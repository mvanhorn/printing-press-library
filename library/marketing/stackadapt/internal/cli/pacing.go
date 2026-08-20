// Hand-authored novel command (no generated header): preserved across regen.
package cli

import (
	"encoding/json"
	"sort"

	"github.com/spf13/cobra"
)

type pacingRow struct {
	Campaign       string   `json:"campaign"`
	Name           string   `json:"name"`
	Status         string   `json:"status,omitempty"`
	PacePercent    *float64 `json:"pace_percent,omitempty"`
	OverallPacing  string   `json:"overall_pacing,omitempty"`
	LifetimeBudget *float64 `json:"lifetime_budget,omitempty"`
	ProjectedSpend *float64 `json:"projected_spend,omitempty"`
	Assessment     string   `json:"assessment"`
}

// assessPace classifies a pace percentage into under/on-track/over.
func assessPace(p *float64) string {
	if p == nil {
		return "unknown (no pacing data — campaign may be flat-budget or not delivering)"
	}
	switch {
	case *p < 90:
		return "under-pacing (will likely underspend the budget)"
	case *p > 110:
		return "over-pacing (will likely overspend or exhaust the budget early)"
	default:
		return "on track"
	}
}

func newNovelPacingCmd(flags *rootFlags) *cobra.Command {
	var advertiser string
	var limit int

	cmd := &cobra.Command{
		Use:   "pacing",
		Short: "Show which campaigns are under- or over-pacing against budget",
		Long: "List active campaigns with their budget pacing — the calculated pace percent, lifetime budget, and " +
			"projected spend from StackAdapt's flight pacing. Flags campaigns that will likely underspend or overspend. " +
			"Scope to one advertiser with --advertiser. Read-only.",
		Example:     "  stackadapt-pp-cli pacing --advertiser 123 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return emitDryRun(cmd, flags, "pacing", "would report campaign budget pacing")
			}
			filter := map[string]any{"archived": false}
			if advertiser != "" {
				filter["advertiserIds"] = []string{advertiser}
			}
			q := `query($n:Int,$f:CampaignFilters){ campaigns(first:$n, filterBy:$f){ nodes {
				id name campaignStatus { state status }
				pacing { flightPacing { calculatedPacePercent overallPacing lifetimeBudget totalProjectedSpend } }
			} } }`
			data, err := runQuery(cmd.Context(), flags, q, map[string]any{"n": limit, "f": filter})
			if err != nil {
				return err
			}
			nodes, _, err := nodesAt(data, "campaigns")
			if err != nil {
				return err
			}

			rows := make([]pacingRow, 0, len(nodes))
			for _, n := range nodes {
				var c struct {
					ID             json.Number `json:"id"`
					Name           string      `json:"name"`
					CampaignStatus struct {
						State  string `json:"state"`
						Status string `json:"status"`
					} `json:"campaignStatus"`
					Pacing struct {
						FlightPacing struct {
							CalculatedPacePercent *float64 `json:"calculatedPacePercent"`
							OverallPacing         string   `json:"overallPacing"`
							LifetimeBudget        *float64 `json:"lifetimeBudget"`
							TotalProjectedSpend   *float64 `json:"totalProjectedSpend"`
						} `json:"flightPacing"`
					} `json:"pacing"`
				}
				// id may be string or number; tolerate both
				var raw map[string]json.RawMessage
				_ = json.Unmarshal(n, &raw)
				_ = json.Unmarshal(n, &c)
				fp := c.Pacing.FlightPacing
				status := c.CampaignStatus.Status
				if status == "" {
					status = c.CampaignStatus.State
				}
				rows = append(rows, pacingRow{
					Campaign:       trimQuotes(raw["id"]),
					Name:           c.Name,
					Status:         status,
					PacePercent:    fp.CalculatedPacePercent,
					OverallPacing:  fp.OverallPacing,
					LifetimeBudget: fp.LifetimeBudget,
					ProjectedSpend: fp.TotalProjectedSpend,
					Assessment:     assessPace(fp.CalculatedPacePercent),
				})
			}
			// Sort: most off-pace first (largest deviation from 100).
			sort.SliceStable(rows, func(i, j int) bool {
				return paceDeviation(rows[i].PacePercent) > paceDeviation(rows[j].PacePercent)
			})

			return emitView(cmd, flags, map[string]any{
				"advertiser":     advertiser,
				"campaign_count": len(rows),
				"campaigns":      rows,
			})
		},
	}
	cmd.Flags().StringVar(&advertiser, "advertiser", "", "Limit to one advertiser ID")
	cmd.Flags().IntVar(&limit, "limit", 200, "Max campaigns to evaluate")
	return cmd
}

func paceDeviation(p *float64) float64 {
	if p == nil {
		return -1
	}
	d := *p - 100
	if d < 0 {
		d = -d
	}
	return d
}

func trimQuotes(r json.RawMessage) string {
	s := string(r)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}
