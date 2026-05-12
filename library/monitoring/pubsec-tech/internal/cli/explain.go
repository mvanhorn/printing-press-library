// Explain command — transcendence feature 6.
//
// Given a news article URL or headline, returns the article plus the awards
// and opportunities it references with the matched mention spans.

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func newExplainCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "explain <article-url-or-headline>",
		Short: "Given a news article URL or headline, return the linked federal contracts and opportunities",
		Long: "Looks up the article in the local store (exact URL match first, then LIKE on title) " +
			"and returns the persisted tags rows pointing to the vendors and agencies it mentions, " +
			"along with any awards and opportunities in the local store that match.",
		Example:     "  pubsec-tech-pp-cli explain \"https://fedscoop.com/some-article-slug/\" --agent\n  pubsec-tech-pp-cli explain \"DoD picks Leidos for cloud\" --json",
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
			art, err := s.ArticleByURLOrTitle(ctx, args[0])
			if err != nil {
				return notFoundErr(err)
			}
			tags, _ := s.TagsForArticle(ctx, art.ID)
			type result struct {
				Article struct {
					ID          string    `json:"id"`
					SourceID    string    `json:"source_id"`
					Title       string    `json:"title"`
					Link        string    `json:"link"`
					Summary     string    `json:"summary,omitempty"`
					PublishedAt time.Time `json:"published_at"`
				} `json:"article"`
				LinkedEntities []taggedHit `json:"linked_entities"`
				Awards         []entityHit `json:"linked_recipients"`
				Opps           []entityHit `json:"linked_opportunities"`
				Notes          []string    `json:"notes,omitempty"`
			}
			r := result{}
			r.Article.ID = art.ID
			r.Article.SourceID = art.SourceID
			r.Article.Title = art.Title
			r.Article.Link = art.Link
			r.Article.Summary = art.Summary
			r.Article.PublishedAt = art.PublishedAt
			r.LinkedEntities = make([]taggedHit, 0, len(tags))
			for _, t := range tags {
				r.LinkedEntities = append(r.LinkedEntities, taggedHit{Kind: t.Kind, Value: t.Value, MatchSpan: t.MatchSpan})
				if t.Kind == "recipient" {
					if data, _ := s.RecipientByName(ctx, t.Value); data != nil {
						var m map[string]any
						if json.Unmarshal(data, &m) == nil {
							if id, _ := m["recipient_id"].(string); id != "" {
								r.Awards = append(r.Awards, entityHit{ID: id, Name: t.Value, Kind: "recipient_profile"})
							}
						}
					}
					if opps, _ := s.OpportunitiesByTitle(ctx, t.Value, limit); len(opps) > 0 {
						for _, o := range opps {
							r.Opps = append(r.Opps, entityHit{ID: o.ID, Name: t.Value, Kind: "opportunity"})
						}
					}
				}
			}
			if len(r.LinkedEntities) == 0 {
				r.Notes = append(r.Notes, "no entities tagged for this article; run `news sync --rebuild-tags` after `sync --resources agencies,recipients` to populate")
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), r, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Article: %s\n", r.Article.Title)
			fmt.Fprintf(w, "Source: %s   Link: %s\n", r.Article.SourceID, r.Article.Link)
			fmt.Fprintf(w, "Published: %s\n", r.Article.PublishedAt.Format("2006-01-02"))
			fmt.Fprintf(w, "\nLinked entities (%d):\n", len(r.LinkedEntities))
			for _, t := range r.LinkedEntities {
				fmt.Fprintf(w, "  - [%s] %s\n", t.Kind, t.Value)
			}
			fmt.Fprintf(w, "\nLinked recipient profiles (%d):\n", len(r.Awards))
			for _, a := range r.Awards {
				fmt.Fprintf(w, "  - %s  (%s)\n", a.Name, a.ID)
			}
			fmt.Fprintf(w, "\nLinked opportunities (%d):\n", len(r.Opps))
			for _, o := range r.Opps {
				fmt.Fprintf(w, "  - %s  %s\n", o.ID, o.Name)
			}
			for _, n := range r.Notes {
				fmt.Fprintf(w, "\n• %s\n", n)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 5, "Max linked opportunities per vendor")
	return cmd
}
