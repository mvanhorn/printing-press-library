// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature for retraction-checker-pp-cli.

package cli

import (
	"fmt"
	"strconv"

	"github.com/mvanhorn/printing-press-library/library/other/retraction-checker/internal/cliutil"
	"github.com/spf13/cobra"
)

type supersededResult struct {
	DOI       string         `json:"doi"`
	Title     string         `json:"title,omitempty"`
	Retracted bool           `json:"retracted"`
	FromYear  int            `json:"from_year,omitempty"`
	Query     string         `json:"query"`
	Related   []openAlexWork `json:"related"`
}

func newNovelSupersededCmd(flags *rootFlags) *cobra.Command {
	var (
		mailto       string
		limit        int
		query        string
		fromYearFlag int // Holds the user-provided value for --from-year
	)
	cmd := &cobra.Command{
		Use:   "superseded <doi>",
		Short: "For a retracted or older paper, find related more-recent research on the same topic, ranked by citation count.",
		Long: "Given a DOI, look up the paper's title and publication year in Crossref, then query\n" +
			"OpenAlex for related research on the same topic published afterwards, ranked by\n" +
			"citation count. Use --query to override the search terms and --from-year to change\n" +
			"the cutoff. Keyless; both Crossref and OpenAlex require no API key.",
		Example:     "  retraction-checker-pp-cli superseded 10.1016/j.micpro.2020.103768 --json",
		Args:        cobra.ArbitraryArgs,
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
				return usageErr(fmt.Errorf("a DOI argument is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			doi := cleanDOI(args[0])
			v, err := checkDOI(ctx, c, mailto, doi)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			search := query
			if search == "" {
				search = v.Title
			}
			if search == "" {
				return fmt.Errorf("no title found for %s; pass --query to search explicitly", doi)
			}

			// Determine the cutoff year.
			// Use the user-provided --from-year flag if set; otherwise, fall back to the publication year.
			var fromYear int
			if cmd.Flags().Changed("from-year") {
				fromYear = fromYearFlag
			} else {
				// Default: extract the year from the published date string.
				if len(v.Published) >= 4 {
					if y, convErr := strconv.Atoi(v.Published[:4]); convErr == nil {
						fromYear = y
					}
				}
			}

			related, err := fetchOpenAlexRelated(ctx, search, mailto, fromYear, limit, cliutil.NewAdaptiveLimiter(flags.rateLimit))
			if err != nil {
				return fmt.Errorf("fetching related research: %w", err)
			}
			res := supersededResult{
				DOI:       v.DOI,
				Title:     v.Title,
				Retracted: v.Retracted,
				FromYear:  fromYear, // reflects the actual year used in the query
				Query:     search,
				Related:   related,
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			if v.Retracted {
				fmt.Fprintf(cmd.OutOrStdout(), "RETRACTED: %s\n", v.Title)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\n", v.Title)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "More-recent related research (ranked by citations):\n\n")
			for _, w := range related {
				fmt.Fprintf(cmd.OutOrStdout(), "  [%d cites] %d  %s\n", w.CitedByCount, w.PublicationYear, w.Title)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&mailto, "mailto", "", "Contact email for the Crossref/OpenAlex polite pool")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum related works to return")
	cmd.Flags().StringVar(&query, "query", "", "Override the topic search terms (defaults to the paper's title)")
	// Register the previously missing --from-year flag so it appears in --help and is functional.
	cmd.Flags().IntVar(&fromYearFlag, "from-year", 0, "Cutoff year for related research (defaults to publication year of the paper)")
	return cmd
}
