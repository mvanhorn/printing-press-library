// Copyright 2026 kjuju600. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/seykota/internal/corpus"
	"github.com/mvanhorn/printing-press-library/library/productivity/seykota/internal/store"
)

// hitView is the JSON shape returned by `search` for one result.
type hitView struct {
	Source       string   `json:"source"`
	Label        string   `json:"label"`
	Title        string   `json:"title"`
	Year         string   `json:"year,omitempty"`
	Month        string   `json:"month,omitempty"`
	Slug         string   `json:"slug,omitempty"`
	Section      string   `json:"section,omitempty"`
	Contributors []string `json:"contributors,omitempty"`
	Snippet      string   `json:"snippet"`
	URL          string   `json:"url"`
	DocID        string   `json:"doc_id"`
}

func toHitView(h store.SearchHit) hitView {
	return hitView{
		Source: h.Source, Label: h.Label(), Title: h.Title, Year: h.Year, Month: h.Month,
		Slug: h.Slug, Section: h.Section, Contributors: h.Contributors,
		Snippet: h.Snippet, URL: h.URL, DocID: h.ID,
	}
}

func runFTS(cmd *cobra.Command, flags *rootFlags, dbPath, query, source, year string, limit int) ([]store.SearchHit, *store.Store, error) {
	source = strings.ToLower(strings.TrimSpace(source))
	switch source {
	case "", "faq", "tsp", "risk":
	default:
		return nil, nil, usageErr(fmt.Errorf("--source must be faq, tsp, or risk (got %q)", source))
	}
	if strings.TrimSpace(query) == "" {
		return nil, nil, usageErr(fmt.Errorf("a search query is required, e.g. seykota-pp-cli search \"heat\""))
	}
	s, err := openCorpus(cmd.Context(), dbPath)
	if err != nil {
		return nil, nil, err
	}
	hits, err := s.SearchCorpus(query, store.SearchOpts{Source: source, Year: year, Limit: limit})
	if err != nil {
		s.Close()
		return nil, nil, fmt.Errorf("search failed: %w", err)
	}
	return hits, s, nil
}

func newSearchCmd(flags *rootFlags) *cobra.Command {
	var dbPath, source, year string
	var limit int
	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Full-text search across the FAQ, the Trading System Project, and the risk essay (offline)",
		Long: `Ranked full-text search over the bundled seykota.com archive: ~20 years of
FAQ months, the Trading System Project section pages, and the risk essay.
Each hit shows where it is (year/month for FAQ, section slug for TSP), a
snippet, and the source URL. Filter with --source and --year; narrow JSON
with --select.`,
		Example: strings.Trim(`
  seykota-pp-cli search "heat"
  seykota-pp-cli search "pyramiding" --source faq --limit 8
  seykota-pp-cli search "support and resistance" --source tsp
  seykota-pp-cli search "whipsaw" --year 2007 --json --select label,snippet,url
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			query := strings.Join(args, " ")
			if dryRunOK(flags) {
				return nil
			}
			hits, s, err := runFTS(cmd, flags, dbPath, query, source, year, limit)
			if err != nil {
				return err
			}
			defer s.Close()
			views := make([]hitView, 0, len(hits))
			for _, h := range hits {
				views = append(views, toHitView(h))
			}
			if wantsJSON(cmd, flags) {
				return emitJSON(cmd, flags, map[string]any{"query": query, "count": len(views), "results": views})
			}
			if len(views) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No matches for %q.\n", query)
				return nil
			}
			rows := make([][]string, 0, len(views))
			for _, v := range views {
				rows = append(rows, []string{v.Source, clip(v.Label, 28), clip(v.Snippet, 90), v.URL})
			}
			if err := printRows(cmd, []string{"SRC", "WHERE", "SNIPPET", "URL"}, rows); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n%d result(s) for %q. Open one: seykota-pp-cli faq show <year> <month> | tsp show <slug> | risk show\n", len(views), query)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Archive DB path (default: the standard data dir)")
	cmd.Flags().StringVar(&source, "source", "", "Limit to one source: faq, tsp, or risk")
	cmd.Flags().StringVar(&year, "year", "", "Limit FAQ hits to a 4-digit year, e.g. 2007")
	cmd.Flags().IntVar(&limit, "limit", 20, "Max results")
	return cmd
}

// ---- cite ----

type citation struct {
	Source  string `json:"source"`
	Date    string `json:"date,omitempty"` // year+month for FAQ
	Section string `json:"section,omitempty"`
	Snippet string `json:"snippet"`
	URL     string `json:"url"`
	BibTeX  string `json:"bibtex,omitempty"`
	Text    string `json:"text"` // ready-to-paste citation line
}

func newCiteCmd(flags *rootFlags) *cobra.Command {
	var dbPath, style, year string
	var limit int
	var bibtex bool
	cmd := &cobra.Command{
		Use:   "cite [query]",
		Short: "Search the archive and return ready-to-paste citations (source, date, snippet, URL) — or BibTeX",
		Long: `Like 'search', but each hit is formatted as a citation you can drop into a
research note, a tooltip, or a doc — with the date/section metadata and the
seykota.com URL the raw site never shows next to the text. Add --bibtex for
BibTeX @misc entries.`,
		Example: strings.Trim(`
  seykota-pp-cli cite "risk per trade" --style faq
  seykota-pp-cli cite "pyramiding" --bibtex
  seykota-pp-cli cite "Uncle Point" --json --select text,url
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			query := strings.Join(args, " ")
			if dryRunOK(flags) {
				return nil
			}
			hits, s, err := runFTS(cmd, flags, dbPath, query, style, year, limit)
			if err != nil {
				return err
			}
			defer s.Close()
			cites := make([]citation, 0, len(hits))
			for i, h := range hits {
				cites = append(cites, makeCitation(h, query, i+1, bibtex))
			}
			if wantsJSON(cmd, flags) {
				return emitJSON(cmd, flags, map[string]any{"query": query, "count": len(cites), "citations": cites})
			}
			if len(cites) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No matches for %q.\n", query)
				return nil
			}
			for _, c := range cites {
				if bibtex {
					fmt.Fprintln(cmd.OutOrStdout(), c.BibTeX)
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), c.Text)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Archive DB path (default: the standard data dir)")
	cmd.Flags().StringVar(&style, "style", "", "Limit to one source: faq, tsp, or risk")
	cmd.Flags().StringVar(&year, "year", "", "Limit FAQ hits to a 4-digit year")
	cmd.Flags().IntVar(&limit, "limit", 10, "Max citations")
	cmd.Flags().BoolVar(&bibtex, "bibtex", false, "Emit BibTeX @misc entries instead of citation lines")
	return cmd
}

func makeCitation(h store.SearchHit, query string, n int, withBibTeX bool) citation {
	date := ""
	if h.Source == corpus.SourceFAQ {
		date = strings.TrimSpace(h.Month + " " + h.Year)
	}
	c := citation{Source: h.Source, Date: date, Section: h.Section, Snippet: h.Snippet, URL: h.URL}
	srcName := map[string]string{
		corpus.SourceFAQ:  "Ed Seykota's FAQ",
		corpus.SourceTSP:  "Trading System Project",
		corpus.SourceRisk: "Ed Seykota, \"Risk Management\"",
	}[h.Source]
	switch h.Source {
	case corpus.SourceFAQ:
		c.Text = fmt.Sprintf("Ed Seykota, FAQ, %s — \"%s\" — %s", date, h.Snippet, h.URL)
	case corpus.SourceTSP:
		c.Text = fmt.Sprintf("Ed Seykota, Trading System Project — %s — \"%s\" — %s", h.Title, h.Snippet, h.URL)
	default:
		c.Text = fmt.Sprintf("Ed Seykota, \"Risk Management\" — \"%s\" — %s", h.Snippet, h.URL)
	}
	if withBibTeX {
		key := fmt.Sprintf("seykota%s%d", h.Source, n)
		yr := h.Year
		if yr == "" {
			yr = "2015"
		}
		title := h.Title
		if title == "" {
			title = srcName
		}
		c.BibTeX = fmt.Sprintf("@misc{%s,\n  author = {Seykota, Ed},\n  title  = {{%s}},\n  year   = {%s},\n  note   = {%s},\n  url    = {%s}\n}", key, title, yr, date, h.URL)
	}
	return c
}

// ---- timeline ----

type timelineYear struct {
	Year    string    `json:"year"`
	Entries []hitView `json:"entries"`
}

func newTimelineCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int
	cmd := &cobra.Command{
		Use:   "timeline [query]",
		Short: "Show, in chronological order, every FAQ month / TSP section / risk passage that matches a concept",
		Long: `How a concept appears across the archive over time. Full-text matches are
bucketed by year (FAQ months sorted within the year; TSP/risk hits — which
have no clean date — land in a separate "undated" group), so you can see
how Seykota's thinking on e.g. "heat" or "whipsaw" shows up across two
decades, rather than as one unordered list.`,
		Example: strings.Trim(`
  seykota-pp-cli timeline "heat"
  seykota-pp-cli timeline "trend following" --json --select year,entries
  seykota-pp-cli timeline "whipsaw" --limit 60
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			query := strings.Join(args, " ")
			if dryRunOK(flags) {
				return nil
			}
			hits, s, err := runFTS(cmd, flags, dbPath, query, "", "", limit)
			if err != nil {
				return err
			}
			defer s.Close()

			byYear := map[string][]hitView{}
			var undated []hitView
			for _, h := range hits {
				v := toHitView(h)
				if h.Source == corpus.SourceFAQ && h.Year != "" {
					byYear[h.Year] = append(byYear[h.Year], v)
				} else {
					undated = append(undated, v)
				}
			}
			years := make([]string, 0, len(byYear))
			for y := range byYear {
				years = append(years, y)
			}
			sort.Strings(years)
			// within a year, FAQ months newest-first by month name order — keep
			// FTS rank order (already meaningful); leave as-is.

			var out []timelineYear
			for _, y := range years {
				out = append(out, timelineYear{Year: y, Entries: byYear[y]})
			}
			if len(undated) > 0 {
				out = append(out, timelineYear{Year: "undated (TSP / risk essay)", Entries: undated})
			}

			if wantsJSON(cmd, flags) {
				return emitJSON(cmd, flags, map[string]any{"query": query, "count": len(hits), "timeline": out})
			}
			if len(hits) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No matches for %q.\n", query)
				return nil
			}
			for _, ty := range out {
				fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n", ty.Year)
				for _, e := range ty.Entries {
					where := e.Label
					fmt.Fprintf(cmd.OutOrStdout(), "  • %-30s %s\n      %s\n", clip(where, 30), clip(e.Snippet, 100), e.URL)
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n%d match(es) for %q across %d year(s).\n", len(hits), query, len(byYear))
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Archive DB path (default: the standard data dir)")
	cmd.Flags().IntVar(&limit, "limit", 60, "Max matches to place on the timeline")
	return cmd
}
