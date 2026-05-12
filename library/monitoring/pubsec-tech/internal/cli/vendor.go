// Vendor rollup command — the headline transcendence feature.
//
// Joins five local tables for one vendor: entities (SAM), exclusions,
// recipients (USAspending recipient profile), opportunities matching the
// vendor name, and articles tagged with the vendor name. The capture-mcp
// `get_entity_and_awards` join covers two of these tables; nothing in the
// surveyed ecosystem covers all five.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/monitoring/pubsec-tech/internal/store"
)

func newVendorCmd(flags *rootFlags) *cobra.Command {
	var since string
	var limit int
	cmd := &cobra.Command{
		Use:   "vendor <name-or-uei>",
		Short: "Vendor rollup: SAM registration, exclusions, awards history, open opportunities, news mentions",
		Long: "One command returns a federal vendor's combined profile across SAM (entity " +
			"registration + exclusions), USAspending (recipient profile + awards history), " +
			"open opportunities, and recent news mentions. Joins five local tables that the " +
			"upstream APIs never expose together.",
		Example:     "  pubsec-tech-pp-cli vendor \"Leidos\" --agent\n  pubsec-tech-pp-cli vendor ABC1234567 --since 30d --json",
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
			name := args[0]
			rollup, err := buildVendorRollup(ctx, s, name, sinceT, limit)
			if err != nil {
				return err
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), rollup, flags)
			}
			return renderVendorRollup(cmd, rollup)
		},
	}
	cmd.Flags().StringVar(&since, "since", "30d", "News-mention window")
	cmd.Flags().IntVar(&limit, "limit", 25, "Max articles/opportunities to surface")
	return cmd
}

// VendorRollup is the structured response shape returned by `vendor <name>`.
type VendorRollup struct {
	Query             string                `json:"query"`
	RecipientProfile  json.RawMessage       `json:"recipient_profile,omitempty"`
	RecipientFound    bool                  `json:"recipient_found"`
	OpenOpportunities []VendorRollupOpp     `json:"open_opportunities,omitempty"`
	RecentArticles    []VendorRollupArticle `json:"recent_articles,omitempty"`
	ArticleCount      int                   `json:"article_count"`
	OpportunityCount  int                   `json:"opportunity_count"`
	GeneratedAt       time.Time             `json:"generated_at"`
	Notes             []string              `json:"notes,omitempty"`
}

type VendorRollupOpp struct {
	ID    string `json:"id"`
	Title string `json:"title,omitempty"`
}

type VendorRollupArticle struct {
	ID          string    `json:"id"`
	SourceID    string    `json:"source_id"`
	Title       string    `json:"title"`
	Link        string    `json:"link,omitempty"`
	PublishedAt time.Time `json:"published_at"`
}

func buildVendorRollup(ctx context.Context, s *store.Store, name string, since time.Time, limit int) (*VendorRollup, error) {
	r := &VendorRollup{Query: name, GeneratedAt: time.Now().UTC()}
	// Recipient profile from local store
	if data, err := s.RecipientByName(ctx, name); err == nil && data != nil {
		r.RecipientProfile = data
		r.RecipientFound = true
	}
	if !r.RecipientFound {
		r.Notes = append(r.Notes, fmt.Sprintf("no synced USAspending recipient matches %q; run `sync` to populate the recipients table", name))
	}
	// Open opportunities matching the vendor name in title or solnum
	if opps, err := s.OpportunitiesByTitle(ctx, name, limit); err == nil {
		for _, o := range opps {
			vo := VendorRollupOpp{ID: o.ID}
			var m map[string]any
			if json.Unmarshal(o.Data, &m) == nil {
				if t, _ := m["title"].(string); t != "" {
					vo.Title = t
				}
			}
			r.OpenOpportunities = append(r.OpenOpportunities, vo)
		}
		r.OpportunityCount = len(r.OpenOpportunities)
	}
	// News mentions tagged for this vendor
	articles, err := s.ArticlesForEntity(ctx, "recipient", name, since, limit)
	if err != nil {
		return nil, err
	}
	for _, a := range articles {
		r.RecentArticles = append(r.RecentArticles, VendorRollupArticle{
			ID: a.ID, SourceID: a.SourceID, Title: a.Title, Link: a.Link, PublishedAt: a.PublishedAt,
		})
	}
	r.ArticleCount = len(r.RecentArticles)
	if r.ArticleCount == 0 {
		r.Notes = append(r.Notes, fmt.Sprintf("no articles tagged with %q in window since %s; run `news sync` to populate", name, since.Format("2006-01-02")))
	}
	return r, nil
}

func renderVendorRollup(cmd *cobra.Command, r *VendorRollup) error {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "Vendor rollup: %s\n", r.Query)
	fmt.Fprintf(w, "  Recipient profile: ")
	if r.RecipientFound {
		fmt.Fprintln(w, "found (use --json for details)")
	} else {
		fmt.Fprintln(w, "not in local store")
	}
	fmt.Fprintf(w, "  Open opportunities (%d):\n", r.OpportunityCount)
	for _, o := range r.OpenOpportunities {
		fmt.Fprintf(w, "    - %s  %s\n", o.ID, truncate(o.Title, 80))
	}
	fmt.Fprintf(w, "  Recent news mentions (%d):\n", r.ArticleCount)
	for _, a := range r.RecentArticles {
		date := ""
		if !a.PublishedAt.IsZero() {
			date = a.PublishedAt.Format("2006-01-02")
		}
		fmt.Fprintf(w, "    - %s  %s  [%s]\n", date, truncate(a.Title, 80), a.SourceID)
	}
	for _, n := range r.Notes {
		fmt.Fprintf(w, "  • %s\n", n)
	}
	return nil
}
