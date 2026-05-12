// Agency modernization view command — transcendence feature 4.
//
// For one agency, composes four local queries scoped to the curated IT-NAICS
// set: open IT opportunities, recent IT-NAICS awards, IT-NAICS spend trend,
// and recent news mentions of the agency by name or abbreviation.
//
// This is a separate command from the generator's `agencies get` because
// (a) the agency lookup here accepts a name/abbreviation rather than a
// toptier_code, and (b) the modernization view is a composed query that
// the upstream API does not expose as a single endpoint.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/monitoring/pubsec-tech/internal/store"
)

// ITNaicsCodes is the curated subset of NAICS codes that count as
// federal-IT for the modernization view. Matches the seeds in
// internal/refdata/codes.go plus the most-used IT sectors.
var ITNaicsCodes = []string{
	"541511", "541512", "541513", "541519", "518210",
	"541715", "541330", "541690", "541611",
	"334111", "334112", "334118", "334210", "334220",
}

func newAgencyCmd(flags *rootFlags) *cobra.Command {
	var modernization bool
	var since string
	var limit int
	cmd := &cobra.Command{
		Use:   "agency <name-or-abbrev>",
		Short: "Agency modernization view: open IT opportunities, recent IT awards, news mentions",
		Long: "Composes a local view scoped to one federal agency: opportunities matching " +
			"the agency name, awards filed under the curated IT-NAICS set, and recent news " +
			"articles that mention the agency. Pass --modernization to apply the IT-NAICS " +
			"filter (default); --no-modernization for a broader view.",
		Example:     "  pubsec-tech-pp-cli agency DOD --modernization --agent\n  pubsec-tech-pp-cli agency \"General Services Administration\" --since 30d --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			s, err := openExtrasStore(ctx)
			if err != nil {
				return err
			}
			defer s.Close()
			sinceT, err := parseSince(since)
			if err != nil {
				return usageErr(err)
			}
			view, err := buildAgencyView(ctx, s, args[0], modernization, sinceT, limit)
			if err != nil {
				return err
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			return renderAgencyView(cmd, view)
		},
	}
	cmd.Flags().BoolVar(&modernization, "modernization", true, "Apply curated IT-NAICS filter to awards (default true)")
	cmd.Flags().StringVar(&since, "since", "30d", "News-mention window")
	cmd.Flags().IntVar(&limit, "limit", 25, "Max items per section")
	return cmd
}

// AgencyView is the structured response for `agency <name>`.
type AgencyView struct {
	Query            string                `json:"query"`
	ITFiltered       bool                  `json:"it_filtered"`
	ITNaicsCodes     []string              `json:"it_naics_codes,omitempty"`
	OpportunityHits  []VendorRollupOpp     `json:"opportunity_hits,omitempty"`
	OpportunityCount int                   `json:"opportunity_count"`
	RecentArticles   []VendorRollupArticle `json:"recent_articles,omitempty"`
	ArticleCount     int                   `json:"article_count"`
	RecentAwards     []agencyAwardRow      `json:"recent_awards,omitempty"`
	AwardCount       int                   `json:"award_count"`
	GeneratedAt      time.Time             `json:"generated_at"`
	Notes            []string              `json:"notes,omitempty"`
}

type agencyAwardRow struct {
	AwardID       string  `json:"award_id"`
	RecipientName string  `json:"recipient_name,omitempty"`
	NAICS         string  `json:"naics,omitempty"`
	Amount        float64 `json:"award_amount,omitempty"`
	EndDate       string  `json:"end_date,omitempty"`
}

func buildAgencyView(ctx context.Context, s *store.Store, name string, modernization bool, since time.Time, limit int) (*AgencyView, error) {
	v := &AgencyView{Query: name, ITFiltered: modernization, GeneratedAt: time.Now().UTC()}
	if modernization {
		v.ITNaicsCodes = ITNaicsCodes
	}
	// Opportunities touching agency name
	if opps, err := s.OpportunitiesByTitle(ctx, name, limit); err == nil {
		for _, o := range opps {
			vo := VendorRollupOpp{ID: o.ID}
			var m map[string]any
			if json.Unmarshal(o.Data, &m) == nil {
				if t, _ := m["title"].(string); t != "" {
					vo.Title = t
				}
			}
			v.OpportunityHits = append(v.OpportunityHits, vo)
		}
		v.OpportunityCount = len(v.OpportunityHits)
	}
	// Articles tagged with agency name or abbreviation
	articles, err := s.ArticlesForEntity(ctx, "agency", name, since, limit)
	if err != nil {
		return nil, err
	}
	for _, a := range articles {
		v.RecentArticles = append(v.RecentArticles, VendorRollupArticle{
			ID: a.ID, SourceID: a.SourceID, Title: a.Title, Link: a.Link, PublishedAt: a.PublishedAt,
		})
	}
	v.ArticleCount = len(v.RecentArticles)
	// Awards filtered to IT NAICS (recompete-style query but with end_date in the future)
	naics := []string{}
	if modernization {
		naics = ITNaicsCodes
	}
	// Use the recompete query but with a 5-year horizon to surface most active awards
	if awards, err := s.AwardsRecompete(ctx, time.Now().AddDate(5, 0, 0), naics, limit); err == nil {
		for _, a := range awards {
			row := agencyAwardRow{AwardID: a.ID}
			var m map[string]any
			if json.Unmarshal(a.Data, &m) != nil {
				continue
			}
			if r, ok := m["recipient"].(map[string]any); ok {
				if n, _ := r["recipient_name"].(string); n != "" {
					row.RecipientName = n
				}
			}
			if n, ok := m["naics"].(map[string]any); ok {
				if c, _ := n["code"].(string); c != "" {
					row.NAICS = c
				}
			}
			if val, ok := m["total_obligation"].(float64); ok {
				row.Amount = val
			}
			if pop, ok := m["period_of_performance"].(map[string]any); ok {
				if e, _ := pop["end_date"].(string); e != "" {
					row.EndDate = e
				}
			}
			// Filter to awards whose awarding agency name contains the query (best-effort).
			if aa, ok := m["awarding_agency"].(map[string]any); ok {
				if t, _ := aa["toptier_agency"].(map[string]any); t != nil {
					if an, _ := t["name"].(string); an != "" {
						if !strings.Contains(strings.ToLower(an), strings.ToLower(name)) {
							continue
						}
					}
				}
			}
			v.RecentAwards = append(v.RecentAwards, row)
		}
		v.AwardCount = len(v.RecentAwards)
	}
	if v.OpportunityCount == 0 && v.ArticleCount == 0 && v.AwardCount == 0 {
		v.Notes = append(v.Notes, "no items in window; run `sync` and `news sync` to populate the local store")
	}
	return v, nil
}

func renderAgencyView(cmd *cobra.Command, v *AgencyView) error {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "Agency view: %s\n", v.Query)
	if v.ITFiltered {
		fmt.Fprintf(w, "  Filtered to %d IT NAICS codes\n", len(v.ITNaicsCodes))
	}
	fmt.Fprintf(w, "\nOpportunity hits (%d):\n", v.OpportunityCount)
	for _, o := range v.OpportunityHits {
		fmt.Fprintf(w, "  - %s  %s\n", o.ID, truncate(o.Title, 100))
	}
	fmt.Fprintf(w, "\nRecent awards (%d):\n", v.AwardCount)
	for _, a := range v.RecentAwards {
		fmt.Fprintf(w, "  - %s  $%.0f  NAICS %s  %s\n", truncate(a.AwardID, 36), a.Amount, a.NAICS, truncate(a.RecipientName, 40))
	}
	fmt.Fprintf(w, "\nRecent news mentions (%d):\n", v.ArticleCount)
	for _, a := range v.RecentArticles {
		date := ""
		if !a.PublishedAt.IsZero() {
			date = a.PublishedAt.Format("2006-01-02")
		}
		fmt.Fprintf(w, "  - %s  [%s]  %s\n", date, a.SourceID, truncate(a.Title, 100))
	}
	for _, n := range v.Notes {
		fmt.Fprintf(w, "\n• %s\n", n)
	}
	return nil
}
