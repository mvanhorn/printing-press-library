// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"context"
	"encoding/xml"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/payments/psx/internal/cliutil"
	"net/url"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/psx/internal/psx"
)

// sitemapURL is one <url> entry from the corporate site's sitemap.
type sitemapURL struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod"`
}

type sitemapDoc struct {
	URLs []sitemapURL `xml:"url"`
}

// docHit is one matched document.
type docHit struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Updated string `json:"updated,omitempty"`
	Kind    string `json:"kind"`
}

// newNovelDocsSearchCmd searches the corporate site's document map. The site
// publishes a sitemap but offers no search across its PDF library, so this
// builds the index the site never exposed.
func newNovelDocsSearchCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var kind string
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search PSX regulatory documents, listing guides and notices by keyword.",
		Long: "Use this command to find PSX rulebooks, listing guides and notices by keyword.\n" +
			"Do NOT use it for company corporate-action filings; use 'announcements' instead.\n" +
			"Searches the corporate site's published document map, which has no search of its own.",
		Example:     "  psx-pp-cli docs search \"rule book\" --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "query=listing", "pp:typed-exit-codes": "0,3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "docs search")
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a search query is required, e.g. \"rule book\""))
			}
			query := strings.ToLower(strings.TrimSpace(strings.Join(args, " ")))
			if query == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a non-empty search query is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			docs, err := fetchDocIndex(ctx, psxClient(flags))
			if err != nil {
				return err
			}
			terms := strings.Fields(query)
			type scoredHit struct {
				doc   docHit
				score int
			}
			scored := make([]scoredHit, 0)
			for _, d := range docs {
				if kind != "" && !strings.EqualFold(kind, d.Kind) {
					continue
				}
				title := strings.ToLower(d.Title)
				link := strings.ToLower(d.URL)
				// Every term must still appear somewhere, so recall is unchanged,
				// but a term found only in a URL path segment scores far lower than
				// one found in the title: such a page is filed under that section
				// rather than being about it.
				matched := true
				score := 0
				for _, t := range terms {
					switch {
					case strings.Contains(title, t):
						score += 4
					case strings.Contains(link, t):
						score++
					default:
						matched = false
					}
					if !matched {
						break
					}
				}
				if !matched {
					continue
				}
				// A whole-phrase or exact title hit is what a human means when the
				// query reads like a document name.
				if strings.Contains(title, query) {
					score += 6
				}
				if title == query {
					score += 10
				}
				scored = append(scored, scoredHit{doc: d, score: score})
			}
			// Relevance first; Updated only breaks ties, because nearly every
			// indexed page shares one site-wide lastmod date.
			sort.Slice(scored, func(i, j int) bool {
				if scored[i].score != scored[j].score {
					return scored[i].score > scored[j].score
				}
				if scored[i].doc.Updated != scored[j].doc.Updated {
					return scored[i].doc.Updated > scored[j].doc.Updated
				}
				return scored[i].doc.Title < scored[j].doc.Title
			})
			hits := make([]docHit, 0, len(scored))
			for _, sh := range scored {
				hits = append(hits, sh.doc)
			}
			if limit > 0 && len(hits) > limit {
				hits = hits[:limit]
			}
			view := struct {
				Query   string   `json:"query"`
				Indexed int      `json:"indexed"`
				Count   int      `json:"count"`
				Results []docHit `json:"results"`
				Note    string   `json:"note,omitempty"`
			}{Query: query, Indexed: len(docs), Count: len(hits), Results: hits}
			if len(hits) == 0 {
				view.Note = fmt.Sprintf("no document in the %d-entry index matched %q; try a broader term", len(docs), query)
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(hits) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				return nil
			}
			for _, h := range hits {
				fmt.Fprintf(cmd.OutOrStdout(), "%-10s %-11s %s\n    %s\n", h.Kind, cliutil.ScrubTerminal(h.Updated), cliutil.ScrubTerminal(h.Title), cliutil.ScrubTerminal(h.URL))
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "maximum documents to return (0 = all)")
	cmd.Flags().StringVar(&kind, "kind", "", "restrict to a kind: pdf or page")
	return cmd
}

// fetchDocIndex pulls and parses the corporate site's sitemap into a searchable
// document list. Titles are derived from the URL slug because the sitemap
// carries no title field; deriving is honest, inventing would not be.
func fetchDocIndex(ctx context.Context, c *psx.Client) ([]docHit, error) {
	corp := c.WithBaseURL(psx.CorporateBaseURL)
	body, err := corp.Get(ctx, "/sitemap.xml")
	if err != nil {
		return nil, fmt.Errorf("fetching document index: %w", err)
	}
	var doc sitemapDoc
	if err := xml.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("parsing document index: %w", err)
	}
	out := make([]docHit, 0, len(doc.URLs))
	for _, u := range doc.URLs {
		loc := strings.TrimSpace(u.Loc)
		if loc == "" {
			continue
		}
		kind := "page"
		if strings.HasSuffix(strings.ToLower(loc), ".pdf") {
			kind = "pdf"
		}
		out = append(out, docHit{
			Title:   titleFromURL(loc),
			URL:     loc,
			Updated: firstTen(u.LastMod),
			Kind:    kind,
		})
	}
	return out, nil
}

// titleFromURL turns the last path segment into readable words.
func titleFromURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	seg := strings.Trim(u.Path, "/")
	if seg == "" {
		return "PSX home"
	}
	if i := strings.LastIndex(seg, "/"); i >= 0 {
		seg = seg[i+1:]
	}
	if unescaped, err := url.PathUnescape(seg); err == nil {
		seg = unescaped
	}
	for _, ext := range []string{".pdf", ".PDF", ".html", ".htm"} {
		seg = strings.TrimSuffix(seg, ext)
	}
	seg = strings.NewReplacer("-", " ", "_", " ", "+", " ").Replace(seg)
	return strings.Join(strings.Fields(seg), " ")
}

func firstTen(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}
