// Hand-authored absorbed analytics over works: landmark, curate, cited-by.
// Not generated.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

func newLandmarkCmd(flags *rootFlags) *cobra.Command {
	var limit, yearFrom int
	cmd := &cobra.Command{
		Use:         "landmark <query>",
		Short:       "List the most-cited, field-defining papers for a topic",
		Long:        "Return the highest-impact works for a topic, ranked by citation count. Use this\nto find the landmark / field-defining studies an agent should read first.",
		Example:     "  scientific-consensus-pp-cli landmark \"crispr gene editing\" --limit 10 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would list landmark papers")
				return nil
			}
			query, err := requireQuery(args)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			filter := ""
			if yearFrom > 0 {
				filter = fmt.Sprintf("from_publication_date:%d-01-01", yearFrom)
			}
			works, _, err := fetchWorks(ctx, c, query, filter, "cited_by_count:desc", limit)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			briefs := make([]workBrief, 0, len(works))
			for _, wk := range works {
				briefs = append(briefs, workBrief{Title: wk.Title, Year: wk.Year, DOI: wk.DOI, CitedBy: wk.CitedBy})
			}
			return emit(cmd, flags, briefs, func(w io.Writer) {
				fmt.Fprintf(w, "Landmark papers: %s\n\n", query)
				for i, b := range briefs {
					fmt.Fprintf(w, "  %2d. [%d, cites=%d] %s\n", i+1, b.Year, b.CitedBy, truncate(b.Title, 76))
				}
			})
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 15, "number of papers to return (max 200)")
	cmd.Flags().IntVar(&yearFrom, "year-from", 0, "only include works published from this year onward")
	return cmd
}

type curateItem struct {
	Rank    int    `json:"rank"`
	Title   string `json:"title"`
	Year    int    `json:"year,omitempty"`
	DOI     string `json:"doi,omitempty"`
	CitedBy int    `json:"cited_by_count"`
	Venue   string `json:"venue,omitempty"`
}

func newCurateCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var format, sortBy string
	cmd := &cobra.Command{
		Use:         "curate <query>",
		Short:       "Build a ranked, deduplicated reading list (markdown, bibtex, or json)",
		Long:        "Assemble a ranked reading list for a topic, deduplicated by DOI, and render it as\nmarkdown, BibTeX, or JSON for import into a reference manager.",
		Example:     "  scientific-consensus-pp-cli curate \"crispr off-target effects\" --format bibtex --limit 25",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would build a curated reading list")
				return nil
			}
			query, err := requireQuery(args)
			if err != nil {
				return err
			}
			sortParam := "cited_by_count:desc"
			switch sortBy {
			case "", "citations":
				sortParam = "cited_by_count:desc"
			case "date", "recent":
				sortParam = "publication_date:desc"
			case "relevance":
				sortParam = "relevance_score:desc"
			default:
				return usageErr(fmt.Errorf("invalid --sort %q: use citations, date, or relevance", sortBy))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			works, _, err := fetchWorks(ctx, c, query, "", sortParam, limit)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			// Dedup by normalized DOI.
			seen := map[string]bool{}
			items := make([]curateItem, 0, len(works))
			for _, wk := range works {
				key := strings.ToLower(wk.DOI)
				if key != "" && seen[key] {
					continue
				}
				seen[key] = true
				items = append(items, curateItem{
					Rank: len(items) + 1, Title: wk.Title, Year: wk.Year, DOI: wk.DOI, CitedBy: wk.CitedBy, Venue: wk.Venue,
				})
			}
			switch format {
			case "", "markdown", "md":
				if flags.asJSON {
					return emit(cmd, flags, items, nil)
				}
				renderCurateMarkdown(cmd.OutOrStdout(), query, items)
			case "json":
				return printJSONFiltered(cmd.OutOrStdout(), items, flags)
			case "bibtex":
				renderCurateBibtex(cmd.OutOrStdout(), items)
			default:
				return usageErr(fmt.Errorf("invalid --format %q: use markdown, bibtex, or json", format))
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 25, "number of works in the list (max 200)")
	cmd.Flags().StringVar(&format, "format", "markdown", "output format: markdown, bibtex, json")
	cmd.Flags().StringVar(&sortBy, "sort", "citations", "ranking: citations, date, relevance")
	return cmd
}

func renderCurateMarkdown(w io.Writer, query string, items []curateItem) {
	fmt.Fprintf(w, "# Reading list: %s\n\n", query)
	for _, it := range items {
		doi := ""
		if it.DOI != "" {
			doi = fmt.Sprintf(" — [doi:%s](https://doi.org/%s)", it.DOI, it.DOI)
		}
		fmt.Fprintf(w, "%d. **%s** (%d, %d citations)%s\n", it.Rank, it.Title, it.Year, it.CitedBy, doi)
	}
}

func renderCurateBibtex(w io.Writer, items []curateItem) {
	for _, it := range items {
		key := fmt.Sprintf("ref%d", it.Rank)
		if it.DOI != "" {
			key = strings.NewReplacer("/", "_", ".", "_").Replace(it.DOI)
		}
		fmt.Fprintf(w, "@article{%s,\n  title = {%s},\n  year = {%d},\n", key, it.Title, it.Year)
		if it.Venue != "" {
			fmt.Fprintf(w, "  journal = {%s},\n", it.Venue)
		}
		if it.DOI != "" {
			fmt.Fprintf(w, "  doi = {%s},\n", it.DOI)
		}
		fmt.Fprintf(w, "}\n\n")
	}
}

func newCitedByCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:         "cited-by <work-id>",
		Short:       "List works that cite a given paper (OpenAlex ID or DOI)",
		Long:        "Return works that cite a given paper. Accepts an OpenAlex work ID (W...) or a\nDOI (with or without the doi: prefix).",
		Example:     "  scientific-consensus-pp-cli cited-by W2741809807 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would list citing works")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a work id (W...) or DOI is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			id, err := resolveOpenAlexID(ctx, c, args[0])
			if err != nil {
				return classifyAPIError(err, flags)
			}
			works, total, err := fetchWorks(ctx, c, "", "cites:"+id, "cited_by_count:desc", limit)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			briefs := make([]workBrief, 0, len(works))
			for _, wk := range works {
				briefs = append(briefs, workBrief{Title: wk.Title, Year: wk.Year, DOI: wk.DOI, CitedBy: wk.CitedBy})
			}
			return emit(cmd, flags, struct {
				WorkID   string      `json:"work_id"`
				Total    int         `json:"total_citing"`
				CitedBy  []workBrief `json:"cited_by"`
			}{id, total, briefs}, func(w io.Writer) {
				fmt.Fprintf(w, "Works citing %s (%d total):\n\n", id, total)
				for i, b := range briefs {
					fmt.Fprintf(w, "  %2d. [%d, cites=%d] %s\n", i+1, b.Year, b.CitedBy, truncate(b.Title, 74))
				}
			})
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 25, "number of citing works to return (max 200)")
	return cmd
}

// resolveOpenAlexID returns a bare OpenAlex work ID (W...) for an input that may
// be a W-id or a DOI.
func resolveOpenAlexID(ctx context.Context, c apiGetter, input string) (string, error) {
	in := strings.TrimSpace(input)
	if strings.HasPrefix(in, "W") {
		return in, nil
	}
	lookup := in
	if !strings.HasPrefix(lookup, "doi:") && !strings.HasPrefix(lookup, "http") {
		lookup = "doi:" + lookup
	}
	raw, err := c.Get(ctx, "/works/"+lookup, map[string]string{"select": "id", "mailto": defaultMailto})
	if err != nil {
		return "", err
	}
	var resp struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", err
	}
	id := resp.ID
	if i := strings.LastIndex(id, "/"); i >= 0 {
		id = id[i+1:]
	}
	if id == "" {
		return "", fmt.Errorf("could not resolve %q to an OpenAlex work id", input)
	}
	return id, nil
}
