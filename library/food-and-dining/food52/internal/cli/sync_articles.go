// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/food52/internal/client"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/food52/internal/food52"
)

func newSyncArticlesCmd(flags *rootFlags) *cobra.Command {
	var (
		concurrency int
		summaryOnly bool
		limit       int
	)
	cmd := &cobra.Command{
		Use:   "articles <vertical> [<subvertical>]",
		Short: "Pull Food52 articles for a vertical (and optional subvertical) into the local store",
		Long: strings.TrimSpace(`
Walks /<vertical> (or /<vertical>/<subvertical>), then fetches each article's
full body and writes them into the local SQLite store. After sync the
articles are searchable offline via 'search' and discoverable via
'articles for-recipe <slug>'.
`),
		Example: strings.Trim(`
  food52-pp-cli sync articles food
  food52-pp-cli sync articles food baking --concurrency 6
  food52-pp-cli sync articles life travel --summary-only
`, "\n"),
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			vertical := strings.TrimSpace(args[0])
			sub := ""
			if len(args) > 1 {
				sub = strings.TrimSpace(args[1])
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			db, err := openStoreOrErr()
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer db.Close()

			path := "/" + vertical
			if sub != "" {
				path += "/" + sub
			}
			html, err := fetchHTML(c, path, nil)
			if err != nil {
				return classifyAPIError(err)
			}
			summaries, err := food52.ExtractArticlesByVertical(html)
			if err != nil {
				return fmt.Errorf("food52 articles browse %s: %w", path, err)
			}
			if limit > 0 && len(summaries) > limit {
				summaries = summaries[:limit]
			}

			out := struct {
				Vertical       string   `json:"vertical"`
				SubVertical    string   `json:"sub_vertical,omitempty"`
				ArticlesPulled int      `json:"articles_pulled"`
				DetailsPulled  int      `json:"details_pulled"`
				Errors         []string `json:"errors,omitempty"`
			}{Vertical: vertical, SubVertical: sub}

			for _, as := range summaries {
				if err := execStoreArticle(db.DB(), as.ID, as.Slug, vertical, mustJSON(as)); err != nil {
					out.Errors = append(out.Errors, fmt.Sprintf("store summary %s: %v", as.Slug, err))
					continue
				}
				out.ArticlesPulled++
			}

			if !summaryOnly {
				det := fetchArticleDetailsConcurrent(c, summaries, concurrency)
				for _, fd := range det {
					if fd.err != nil {
						out.Errors = append(out.Errors, fmt.Sprintf("detail %s: %v", fd.slug, fd.err))
						continue
					}
					if err := execStoreArticle(db.DB(), fd.article.ID, fd.article.Slug, vertical, mustJSON(fd.article)); err != nil {
						out.Errors = append(out.Errors, fmt.Sprintf("store detail %s: %v", fd.slug, err))
						continue
					}
					out.DetailsPulled++
				}
			}

			return emitFromFlags(flags, out, func() {
				where := vertical
				if sub != "" {
					where = vertical + "/" + sub
				}
				fmt.Printf("Synced articles for %s\n", where)
				fmt.Printf("  summaries:  %d\n", out.ArticlesPulled)
				fmt.Printf("  details:    %d\n", out.DetailsPulled)
				if n := len(out.Errors); n > 0 {
					fmt.Printf("  errors:     %d (run with --json for details)\n", n)
				}
			})
		},
	}
	cmd.Flags().IntVar(&concurrency, "concurrency", 4, "Concurrent article-detail fetches (1-16)")
	cmd.Flags().BoolVar(&summaryOnly, "summary-only", false, "Skip per-article body fetch")
	cmd.Flags().IntVar(&limit, "limit", 0, "Cap the number of articles pulled (0 = all on the first page)")
	return cmd
}

type articleFetch struct {
	slug    string
	article *food52.Article
	err     error
}

func fetchArticleDetailsConcurrent(c *client.Client, summaries []food52.ArticleSummary, n int) []articleFetch {
	if n <= 0 {
		n = 4
	}
	if n > 16 {
		n = 16
	}
	out := make([]articleFetch, len(summaries))
	sem := make(chan struct{}, n)
	var wg sync.WaitGroup
	for i, as := range summaries {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, as food52.ArticleSummary) {
			defer wg.Done()
			defer func() { <-sem }()
			html, err := fetchHTML(c, "/story/"+as.Slug, nil)
			if err != nil {
				out[i] = articleFetch{slug: as.Slug, err: err}
				return
			}
			a, err := food52.ExtractArticle(html, canonicalArticleURL(as.Slug))
			out[i] = articleFetch{slug: as.Slug, article: a, err: err}
		}(i, as)
	}
	wg.Wait()
	return out
}
