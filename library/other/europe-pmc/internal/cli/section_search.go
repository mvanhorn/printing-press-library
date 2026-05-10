package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newSectionSearchCmd(flags *rootFlags) *cobra.Command {
	var flagQuery string
	var flagSection string
	var flagPageSize int
	var flagPage int

	cmd := &cobra.Command{
		Use:   "section-search",
		Short: "Search within specific article sections (Methods, Results, Discussion)",
		Long: `Search within specific article sections using Europe PMC's section-qualified
query syntax. Supported sections: INTRO, METHODS, RESULTS, DISCUSS, CONCL, CASE,
FIG, TABLE, SUPPL, OTHER, ACK, AUTH_CONT, COMP_INT, ABBR, KEYWORD, REF.`,
		Example: `  europe-pmc-pp-cli section-search --query "CRISPR" --section METHODS
  europe-pmc-pp-cli section-search --query "machine learning" --section RESULTS
  europe-pmc-pp-cli section-search --query "side effects" --section DISCUSS`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if flagQuery == "" {
				return fmt.Errorf("--query is required")
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Build section-qualified query
			query := flagQuery
			if flagSection != "" {
				query = fmt.Sprintf("%s:%s", flagSection, flagQuery)
			}

			params := map[string]string{
				"query":      query,
				"format":     "json",
				"resultType": "core",
				"pageSize":   fmt.Sprintf("%d", flagPageSize),
				"page":       fmt.Sprintf("%d", flagPage),
			}

			data, err := c.Get("/search", params)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			var envelope struct {
				HitCount   int `json:"hitCount"`
				ResultList struct {
					Result []json.RawMessage `json:"result"`
				} `json:"resultList"`
			}
			if err := json.Unmarshal(data, &envelope); err != nil {
				return fmt.Errorf("parsing response: %w", err)
			}

			type sectionResult struct {
				ID      string `json:"id"`
				Source  string `json:"source"`
				Title   string `json:"title"`
				DOI     string `json:"doi,omitempty"`
				PMID    string `json:"pmid,omitempty"`
				PubYear string `json:"pubYear,omitempty"`
			}

			var results []sectionResult
			for _, raw := range envelope.ResultList.Result {
				var article struct {
					ID      string `json:"id"`
					Source  string `json:"source"`
					Title   string `json:"title"`
					DOI     string `json:"doi"`
					PMID    string `json:"pmid"`
					PubYear string `json:"pubYear"`
				}
				if err := json.Unmarshal(raw, &article); err != nil {
					continue
				}
				results = append(results, sectionResult{
					ID:      article.ID,
					Source:  article.Source,
					Title:   truncate(article.Title, 120),
					DOI:     article.DOI,
					PMID:    article.PMID,
					PubYear: article.PubYear,
				})
			}

			output := map[string]any{
				"section":   flagSection,
				"query":     flagQuery,
				"hit_count": envelope.HitCount,
				"page":      flagPage,
				"results":   results,
			}
			return printJSONFiltered(cmd.OutOrStdout(), output, flags)
		},
	}

	cmd.Flags().StringVar(&flagQuery, "query", "", "Search term")
	cmd.Flags().StringVar(&flagSection, "section", "", "Article section: INTRO, METHODS, RESULTS, DISCUSS, CONCL, FIG, TABLE, REF")
	cmd.Flags().IntVar(&flagPageSize, "page-size", 25, "Results per page")
	cmd.Flags().IntVar(&flagPage, "page", 1, "Page number")

	return cmd
}
