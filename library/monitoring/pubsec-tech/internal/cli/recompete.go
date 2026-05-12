// Recompete radar command — transcendence feature 2.
//
// Lists federal awards whose period-of-performance end date falls inside a
// chosen window, joined to the incumbent recipient profile and to any
// already-posted follow-on RFP. The cross-source join (awards × recipients ×
// opportunities) is the differentiator; no existing MCP offers it.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newRecompeteCmd(flags *rootFlags) *cobra.Command {
	var naicsList string
	var within string
	var limit int
	var minCeiling float64
	cmd := &cobra.Command{
		Use:   "recompete",
		Short: "Federal IT awards whose period of performance ends inside a window",
		Long: "Recompete radar: lists locally-synced awards with a period_of_performance.end_date " +
			"that falls within --within from today, optionally filtered to a set of NAICS codes. " +
			"For each award, includes the incumbent recipient (if synced) and any opportunities " +
			"whose title or solicitation number contains the recipient name (best-effort follow-on " +
			"detection). Requires `sync --resources awards,recipients` to populate the local store.",
		Example:     "  pubsec-tech-pp-cli recompete --naics 541512 --within 18m --agent\n  pubsec-tech-pp-cli recompete --naics 541511,541512,541519 --within 12m --min-ceiling 50000000 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			s, err := openExtrasStore(ctx)
			if err != nil {
				return err
			}
			defer s.Close()
			windowEnd, err := parseWindow(within)
			if err != nil {
				return usageErr(err)
			}
			codes := splitAndTrim(naicsList)
			awards, err := s.AwardsRecompete(ctx, windowEnd, codes, limit)
			if err != nil {
				return err
			}
			type row struct {
				AwardID        string   `json:"award_id"`
				EndDate        string   `json:"period_of_performance_end_date"`
				RecipientName  string   `json:"recipient_name,omitempty"`
				AwardingAgency string   `json:"awarding_agency,omitempty"`
				NAICS          string   `json:"naics,omitempty"`
				Ceiling        float64  `json:"ceiling_amount,omitempty"`
				FollowOnOppIDs []string `json:"follow_on_opportunity_ids,omitempty"`
			}
			rows := make([]row, 0, len(awards))
			for _, a := range awards {
				var m map[string]any
				if err := json.Unmarshal(a.Data, &m); err != nil {
					continue
				}
				r := row{AwardID: a.ID}
				if pop, ok := m["period_of_performance"].(map[string]any); ok {
					if end, _ := pop["end_date"].(string); end != "" {
						r.EndDate = end
					}
				}
				if recipient, ok := m["recipient"].(map[string]any); ok {
					if n, _ := recipient["recipient_name"].(string); n != "" {
						r.RecipientName = n
					}
				}
				if aa, ok := m["awarding_agency"].(map[string]any); ok {
					if t, _ := aa["toptier_agency"].(map[string]any); t != nil {
						if n, _ := t["name"].(string); n != "" {
							r.AwardingAgency = n
						}
					}
				}
				if n, ok := m["naics"].(map[string]any); ok {
					if c, _ := n["code"].(string); c != "" {
						r.NAICS = c
					}
				}
				if v, ok := m["base_and_all_options_value"].(float64); ok {
					r.Ceiling = v
				}
				if minCeiling > 0 && r.Ceiling < minCeiling {
					continue
				}
				if r.RecipientName != "" {
					if opps, _ := s.OpportunitiesByTitle(ctx, r.RecipientName, 5); len(opps) > 0 {
						for _, o := range opps {
							r.FollowOnOppIDs = append(r.FollowOnOppIDs, o.ID)
						}
					}
				}
				rows = append(rows, r)
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "END DATE\tNAICS\tCEILING\tAGENCY\tINCUMBENT\tFOLLOW-ONS")
			for _, r := range rows {
				followOns := len(r.FollowOnOppIDs)
				fmt.Fprintf(tw, "%s\t%s\t$%.0f\t%s\t%s\t%d\n",
					r.EndDate, r.NAICS, r.Ceiling, truncate(r.AwardingAgency, 30), truncate(r.RecipientName, 30), followOns)
			}
			if err := tw.Flush(); err != nil {
				return err
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No awards in window. Run `sync --resources awards` first to populate.")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&naicsList, "naics", "", "Comma-separated NAICS codes to filter on")
	cmd.Flags().StringVar(&within, "within", "18m", "Look-ahead window (e.g. 6m, 18m, 24m)")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum awards to return")
	cmd.Flags().Float64Var(&minCeiling, "min-ceiling", 0, "Filter to awards with base+options ceiling >= this dollar amount")
	return cmd
}

// parseWindow accepts "6m", "18m", "12d", "30d" — months are 30-day proxies.
func parseWindow(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Now().AddDate(0, 18, 0), nil
	}
	now := time.Now()
	if strings.HasSuffix(s, "m") {
		var n int
		if _, err := fmt.Sscanf(strings.TrimSuffix(s, "m"), "%d", &n); err == nil {
			return now.AddDate(0, n, 0), nil
		}
	}
	if strings.HasSuffix(s, "d") {
		var n int
		if _, err := fmt.Sscanf(strings.TrimSuffix(s, "d"), "%d", &n); err == nil {
			return now.AddDate(0, 0, n), nil
		}
	}
	if strings.HasSuffix(s, "y") {
		var n int
		if _, err := fmt.Sscanf(strings.TrimSuffix(s, "y"), "%d", &n); err == nil {
			return now.AddDate(n, 0, 0), nil
		}
	}
	return time.Time{}, fmt.Errorf("could not parse --within %q (try 6m, 18m, 30d, 1y)", s)
}

func splitAndTrim(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
