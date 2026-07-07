// Hand-authored citation-tracking command. Not generated.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newCitedByCmd(flags *rootFlags) *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "cited-by <doi-or-id>",
		Short: "List works that cite a given Lancet article",
		Long: "Resolve a DOI or OpenAlex work ID, then list the works that cite it (newest\n" +
			"first) via OpenAlex. Live query; no local mirror required.",
		Example:     "  thelancet-pp-cli cited-by 10.1016/S0140-6736(20)30183-5 --json\n  thelancet-pp-cli cited-by W3009563180 --limit 50",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a DOI or OpenAlex work ID is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Resolve input to an OpenAlex work ID.
			ref := args[0]
			workID := ref
			if !strings.HasPrefix(ref, "W") || strings.Contains(ref, "/") || strings.Contains(ref, ".") {
				lookup := ref
				if !strings.HasPrefix(lookup, "doi:") && !strings.HasPrefix(lookup, "http") {
					lookup = "doi:" + lookup
				}
				raw, err := c.Get(ctx, "/works/"+lookup, map[string]string{"select": "id"})
				if err != nil {
					return classifyAPIError(err, flags)
				}
				var w struct {
					ID string `json:"id"`
				}
				if err := json.Unmarshal(raw, &w); err != nil || w.ID == "" {
					return fmt.Errorf("could not resolve %q to an OpenAlex work", ref)
				}
				workID = w.ID[strings.LastIndex(w.ID, "/")+1:]
			}

			raw, err := c.Get(ctx, "/works", map[string]string{
				"filter":   "cites:" + workID,
				"select":   "id,doi,title,publication_year,cited_by_count",
				"sort":     "publication_date:desc",
				"per-page": fmt.Sprintf("%d", limit),
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var page struct {
				Meta struct {
					Count int `json:"count"`
				} `json:"meta"`
				Results []struct {
					DOI             string `json:"doi"`
					Title           string `json:"title"`
					PublicationYear int    `json:"publication_year"`
					CitedByCount    int    `json:"cited_by_count"`
				} `json:"results"`
			}
			if err := json.Unmarshal(raw, &page); err != nil {
				return fmt.Errorf("decoding citing works: %w", err)
			}
			view := map[string]any{"work_id": workID, "total_citing": page.Meta.Count, "results": page.Results}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d works cite %s (showing %d)\n\n", page.Meta.Count, workID, len(page.Results))
			for _, r := range page.Results {
				fmt.Fprintf(cmd.OutOrStdout(), "[%d cites] %s (%d)\n", r.CitedByCount, r.Title, r.PublicationYear)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum citing works to return")
	return cmd
}
