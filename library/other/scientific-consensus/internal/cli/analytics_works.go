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
	Rank  int    `json:"rank"`
	Title string `json:"title"`
	// Authors is the verbatim OpenAlex author list, in OpenAlex order.
	// BibTeX joins these with " and "; omitted when the work has none.
	Authors []string `json:"authors,omitempty"`
	Year    int      `json:"year,omitempty"`
	DOI     string   `json:"doi,omitempty"`
	CitedBy int      `json:"cited_by_count"`
	Venue   string   `json:"venue,omitempty"`
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
					Rank: len(items) + 1, Title: wk.Title, Authors: wk.Authors, Year: wk.Year, DOI: wk.DOI, CitedBy: wk.CitedBy, Venue: wk.Venue,
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

// bibtexEscapeValue makes a raw OpenAlex string safe to interpolate into a
// braced BibTeX field value. Braces and backslashes are dropped rather than
// escaped: there is no reliable way to emit a literal brace inside a field
// value, and an unbalanced one terminates the field early and corrupts every
// entry after it. Typographic quotes are deliberately left alone — they are
// the source data.
func bibtexEscapeValue(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case '{', '}', '\\':
			// Dropped: not representable in a braced field value.
		case '\n', '\r', '\t':
			b.WriteByte(' ')
		case '&', '%', '$', '#', '_':
			b.WriteByte('\\')
			b.WriteRune(r)
		default:
			b.WriteRune(r)
		}
	}
	// Collapse runs of whitespace to one space and trim.
	return strings.Join(strings.Fields(b.String()), " ")
}

// bibtexCiteKey reduces a DOI to a legal BibTeX citation key: ASCII letters,
// digits, "-" and "_" only. Real DOIs carry characters BibTeX and Zotero
// reject — 10.1016/s2213-8587(21)00051-6 is a production example whose
// parentheses break import. Returns "" when nothing legal survives.
func bibtexCiteKey(doi string) string {
	var b strings.Builder
	b.Grow(len(doi))
	for _, r := range doi {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	out := b.String()
	for strings.Contains(out, "__") {
		out = strings.ReplaceAll(out, "__", "_")
	}
	return strings.Trim(out, "_")
}

func renderCurateBibtex(w io.Writer, items []curateItem) {
	used := map[string]bool{}
	for _, it := range items {
		key := bibtexCiteKey(it.DOI)
		if key == "" {
			key = fmt.Sprintf("ref%d", it.Rank)
		}
		// Distinct DOIs can normalize onto the same key; disambiguate in the
		// order the entries appear so the output stays deterministic.
		if used[key] {
			base := key
			for n := 2; used[key]; n++ {
				key = fmt.Sprintf("%s_%d", base, n)
			}
		}
		used[key] = true

		fmt.Fprintf(w, "@article{%s,\n  title = {%s},\n  year = {%d},\n", key, bibtexEscapeValue(it.Title), it.Year)
		if len(it.Authors) > 0 {
			names := make([]string, 0, len(it.Authors))
			for _, a := range it.Authors {
				names = append(names, bibtexEscapeValue(a))
			}
			fmt.Fprintf(w, "  author = {%s},\n", strings.Join(names, " and "))
		}
		if it.Venue != "" {
			fmt.Fprintf(w, "  journal = {%s},\n", bibtexEscapeValue(it.Venue))
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
				WorkID  string      `json:"work_id"`
				Total   int         `json:"total_citing"`
				CitedBy []workBrief `json:"cited_by"`
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
