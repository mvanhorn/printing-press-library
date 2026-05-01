// Hand-written: search command. Searches the YC directory snapshot
// in-memory for companies matching a query — names, descriptions,
// industries. The full local-store FTS5 path will be wired in a polish
// pass; today's implementation is good enough to demonstrate the
// "personal research database" value prop.

package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/company-goat/internal/source/yc"
	"github.com/spf13/cobra"
)

type searchHit struct {
	Name        string   `json:"name"`
	Domain      string   `json:"domain,omitempty"`
	Source      string   `json:"source"` // currently always "yc"
	OneLiner    string   `json:"one_liner,omitempty"`
	Batch       string   `json:"batch,omitempty"`
	Status      string   `json:"status,omitempty"`
	Industry    string   `json:"industry,omitempty"`
	Country     string   `json:"country,omitempty"`
	MatchFields []string `json:"match_fields,omitempty"`
}

func newCompanySearchCmd(flags *rootFlags) *cobra.Command {
	var maxHits int
	var batch string
	var industry string

	cmd := &cobra.Command{
		Use:   "search <query>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Full-text search across companies in the YC directory by name, description, industry, location.",
		Long: `search finds companies in the YC directory snapshot (~5000+ companies) by query terms across name, one-liner description, industry, and location. Useful for "find the fintech with the strong London presence" or "companies in the AI batch with 'agent' in their description."

Filters:
  --batch    Match a specific YC batch (e.g. W22, S20)
  --industry Match the industry tag (e.g. fintech, healthcare)

Note: the full local-store FTS5 search across previously-synced companies (any source) is on the v1 roadmap; today's search is YC-only. Sync still populates the local store so you can SQL it directly.`,
		Example: strings.Trim(`
  company-goat-pp-cli search "fintech"
  company-goat-pp-cli search "ai agent" --batch S23
  company-goat-pp-cli search "pizza" --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(cmd, flags) {
				return nil
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			query := strings.ToLower(strings.Join(args, " "))
			if maxHits <= 0 {
				maxHits = 20
			}

			c := yc.NewClient()
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			all, err := c.LoadAll(ctx)
			if err != nil {
				return fmt.Errorf("yc: %w", err)
			}

			batch = strings.ToLower(strings.TrimSpace(batch))
			industry = strings.ToLower(strings.TrimSpace(industry))

			type scored struct {
				hit   searchHit
				score int
			}
			var hits []scored
			for _, co := range all {
				if batch != "" && strings.ToLower(co.Batch) != batch {
					continue
				}
				if industry != "" && strings.ToLower(co.Industry) != industry && strings.ToLower(co.Subindustry) != industry {
					continue
				}
				score := 0
				match := []string{}
				name := strings.ToLower(co.Name)
				one := strings.ToLower(co.OneLiner)
				ind := strings.ToLower(co.Industry)
				sub := strings.ToLower(co.Subindustry)
				loc := strings.ToLower(co.LocationCity + " " + co.Country)
				if strings.Contains(name, query) {
					score += 10
					match = append(match, "name")
				}
				if strings.Contains(one, query) {
					score += 4
					match = append(match, "one_liner")
				}
				if strings.Contains(ind, query) || strings.Contains(sub, query) {
					score += 3
					match = append(match, "industry")
				}
				if strings.Contains(loc, query) {
					score += 2
					match = append(match, "location")
				}
				if score == 0 {
					continue
				}
				domain := ""
				if co.Website != "" {
					d := strings.TrimPrefix(co.Website, "https://")
					d = strings.TrimPrefix(d, "http://")
					d = strings.TrimPrefix(d, "www.")
					if i := strings.Index(d, "/"); i > 0 {
						d = d[:i]
					}
					domain = d
				}
				hits = append(hits, scored{
					hit: searchHit{
						Name:        co.Name,
						Domain:      domain,
						Source:      "yc",
						OneLiner:    co.OneLiner,
						Batch:       co.Batch,
						Status:      co.Status,
						Industry:    co.Industry,
						Country:     co.Country,
						MatchFields: match,
					},
					score: score,
				})
			}
			sort.SliceStable(hits, func(i, j int) bool { return hits[i].score > hits[j].score })
			if len(hits) > maxHits {
				hits = hits[:maxHits]
			}

			out := make([]searchHit, 0, len(hits))
			for _, h := range hits {
				out = append(out, h.hit)
			}

			w := cmd.OutOrStdout()
			asJSON := flags.asJSON || !isTerminal(w)
			if asJSON {
				return flags.printJSON(cmd, map[string]any{
					"query":            query,
					"hits":             out,
					"total":            len(hits),
					"sources_searched": []string{"yc"},
				})
			}
			if len(out) == 0 {
				fmt.Fprintf(w, "no matches for %q\n", query)
				return nil
			}
			fmt.Fprintf(w, "Search results for %q (%d hits):\n\n", query, len(out))
			for _, h := range out {
				fmt.Fprintf(w, "  %-30s  %s  %-8s  %s\n", h.Name, h.Domain, h.Batch, h.Status)
				if h.OneLiner != "" {
					fmt.Fprintf(w, "    %s\n", fundingTruncate(h.OneLiner, 90))
				}
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&maxHits, "max", 20, "Maximum results to show")
	cmd.Flags().StringVar(&batch, "batch", "", "Filter to a specific YC batch (e.g. W22, S20)")
	cmd.Flags().StringVar(&industry, "industry", "", "Filter to a specific industry tag")
	return cmd
}
