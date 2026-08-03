// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// Crawls Crestron.com and writes the local mirror. Hand-authored because the
// generator emits sync only for JSON endpoints and every Crestron surface is
// server-rendered HTML.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/crestron/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/devices/crestron/internal/crestronparse"
	"github.com/mvanhorn/printing-press-library/library/devices/crestron/internal/crestronstore"

	"github.com/spf13/cobra"
)

type syncReport struct {
	Categories  int      `json:"categories"`
	Products    int      `json:"products"`
	Releases    int      `json:"releases"`
	NotesPulled int      `json:"release_notes_pulled"`
	Requests    int      `json:"requests"`
	Elapsed     string   `json:"elapsed"`
	Warnings    []string `json:"warnings,omitempty"`
	Note        string   `json:"note,omitempty"`
}

func newCrestronSyncCmd(flags *rootFlags) *cobra.Command {
	var (
		flagResources   string
		flagDB          string
		flagMaxCats     int
		flagNotes       bool
		flagMaxNotes    int
		flagConcurrent  int
		flagMaxDuration time.Duration
	)

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Build the local mirror of the Crestron catalog and firmware releases",
		Long: strings.Trim(`
Crawl Crestron.com into a local SQLite mirror so products, categories, and
firmware releases can be queried offline.

Crestron publishes no API and no sitemap of products, so the catalog is walked
the way the website itself does: /sitemap lists category paths, each category
page carries the ids its product-tile endpoint needs, and each tile page is
paged until the category's own product count is reached.

Firmware releases are stored with their covered-model list expanded, which is
what makes "which release covers model X" answerable locally — a single release
can cover seven models and the website never exposes that mapping.

With --notes and a signed-in session, release notes and change logs are pulled
too, making them full-text searchable via 'search'. Without a session the
version and date are still recorded; only the notes are skipped.
`, "\n"),
		Example: strings.Trim(`
  crestron-pp-cli sync
  crestron-pp-cli sync --resources categories,products
  crestron-pp-cli sync --resources releases --notes --max-notes 50
`, "\n"),
		// No mcp:read-only: sync rewrites the local mirror and issues a long
		// run of outbound requests, so MCP hosts should prompt before calling it.
		Annotations: map[string]string{},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "sync")
			}
			// NOTE: deliberately not boundCtx here. --timeout is a *per-request*
			// timeout and the generated client already applies it to every call.
			// Applying it to the whole command would kill a full catalog crawl,
			// which takes minutes. --max-duration bounds the crawl instead.
			ctx := cmd.Context()
			if flagMaxDuration > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, flagMaxDuration)
				defer cancel()
			}

			want := map[string]bool{}
			for _, r := range strings.Split(flagResources, ",") {
				if r = strings.TrimSpace(strings.ToLower(r)); r != "" {
					want[r] = true
				}
			}
			if len(want) == 0 || want["all"] {
				want = map[string]bool{"categories": true, "products": true, "releases": true}
			}

			if flagDB == "" {
				flagDB = defaultDBPath("crestron-pp-cli")
			}
			st, err := crestronstore.Open(ctx, flagDB)
			if err != nil {
				return fmt.Errorf("opening local mirror: %w", err)
			}
			defer func() { _ = st.Close() }()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Live dogfood runs under a flat per-command timeout, so curtail the
			// crawl rather than substituting mock data.
			if cliutil.IsDogfoodEnv() {
				if flagMaxCats <= 0 || flagMaxCats > 2 {
					flagMaxCats = 2
				}
				flagNotes = false
			}

			rep := syncReport{Warnings: make([]string, 0)}
			started := time.Now()
			progress := cmd.ErrOrStderr()

			// --- categories -------------------------------------------------
			var cats []crestronstore.Category
			if want["categories"] || want["products"] {
				fmt.Fprintln(progress, "syncing categories...")
				body, err := c.Get(ctx, "/sitemap", nil)
				if err != nil {
					return fmt.Errorf("fetching catalog sitemap: %w", err)
				}
				rep.Requests++
				paths, err := crestronparse.ParseCatalogPaths(body)
				if err != nil {
					return fmt.Errorf("parsing catalog sitemap: %w", err)
				}
				if flagMaxCats > 0 && len(paths) > flagMaxCats {
					rep.Warnings = append(rep.Warnings, fmt.Sprintf(
						"category crawl capped at %d of %d paths by --max-categories", flagMaxCats, len(paths)))
					paths = paths[:flagMaxCats]
				}
				for i, p := range paths {
					rel := strings.TrimPrefix(p, "/Products/Catalog/")
					body, err := c.Get(ctx, "/Products/Catalog/"+rel, nil)
					if err != nil {
						rep.Warnings = append(rep.Warnings, fmt.Sprintf("category %s: %v", p, err))
						continue
					}
					rep.Requests++
					cat, err := crestronparse.ParseCategoryPage(body, p)
					if err != nil || cat.DocumentID == "" {
						continue
					}
					cats = append(cats, crestronstore.Category{
						Path: p, DocumentID: cat.DocumentID, NodeID: cat.NodeID,
						ProductCount: cat.ProductCount,
					})
					if (i+1)%10 == 0 {
						fmt.Fprintf(progress, "  %d/%d categories\n", i+1, len(paths))
					}
				}
				if err := st.UpsertCategories(ctx, cats); err != nil {
					return fmt.Errorf("writing categories: %w", err)
				}
				rep.Categories = len(cats)
			}

			// --- products ---------------------------------------------------
			if want["products"] {
				fmt.Fprintf(progress, "syncing products across %d categories...\n", len(cats))
				workers := flagConcurrent
				if workers < 1 {
					workers = 3
				}
				type catResult struct {
					products []crestronstore.Product
					reqs     int
					warn     string
				}
				results := make([]catResult, len(cats))
				sem := make(chan struct{}, workers)
				var wg sync.WaitGroup
				for i := range cats {
					wg.Add(1)
					go func(idx int) {
						defer wg.Done()
						sem <- struct{}{}
						defer func() { <-sem }()
						results[idx] = crawlCategoryProducts(ctx, c, cats[idx])
					}(i)
				}
				wg.Wait()

				all := make([]crestronstore.Product, 0)
				for _, r := range results {
					rep.Requests += r.reqs
					if r.warn != "" {
						rep.Warnings = append(rep.Warnings, r.warn)
					}
					all = append(all, r.products...)
				}
				// Write with a fresh context: if the crawl budget expired mid-walk
				// the partial results are still worth keeping, and a cancelled
				// ctx would discard them.
				writeCtx, writeCancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Minute)
				err := st.UpsertProducts(writeCtx, all)
				writeCancel()
				if err != nil {
					return fmt.Errorf("writing products: %w", err)
				}
				if ctxErr := ctx.Err(); ctxErr != nil {
					rep.Warnings = append(rep.Warnings,
						"crawl budget reached (--max-duration); partial results were saved")
				}
				rep.Products = len(all)
			}

			// --- releases ---------------------------------------------------
			if want["releases"] {
				fmt.Fprintln(progress, "syncing firmware releases...")
				releases := make([]crestronstore.Release, 0)
				seen := map[string]bool{}
				for page := 1; page <= 20; page++ {
					body, err := c.Get(ctx, "/Support/Search-Results", map[string]string{
						"c": "4", "type": "Firmware", "o": "Created:desc",
						"p": strconv.Itoa(page), "m": "50",
					})
					if err != nil {
						rep.Warnings = append(rep.Warnings, fmt.Sprintf("release page %d: %v", page, err))
						break
					}
					rep.Requests++
					sp, err := crestronparse.ParseSearchResults(body)
					if err != nil || sp.Count == 0 {
						break
					}
					fresh := 0
					for _, row := range sp.Results {
						if row.URL == "" || seen[row.URL] {
							continue
						}
						seen[row.URL] = true
						fresh++
						parts, version := crestronparse.SplitReleaseTitle(row.Title)
						releases = append(releases, crestronstore.Release{
							URL: row.URL, Title: row.Title, Version: version,
							Date: row.Date, Type: row.Type,
							Models: crestronparse.ExpandModelFamily(parts),
						})
					}
					if fresh == 0 || !sp.HasMore {
						break
					}
					if cliutil.IsDogfoodEnv() {
						break
					}
				}

				if flagNotes && len(releases) > 0 {
					limit := flagMaxNotes
					if limit <= 0 || limit > len(releases) {
						limit = len(releases)
					}
					fmt.Fprintf(progress, "pulling release notes for %d releases...\n", limit)
					var authWarned bool
					for i := 0; i < limit; i++ {
						body, err := c.Get(ctx, releases[i].URL, nil)
						if err != nil {
							continue
						}
						rep.Requests++
						fr, err := crestronparse.ParseFirmwareRelease(body)
						if err != nil {
							continue
						}
						if fr.RequiresAuth {
							if !authWarned {
								rep.Warnings = append(rep.Warnings,
									"release notes require a signed-in session; run 'crestron-pp-cli auth login --chrome' then re-run sync --notes")
								authWarned = true
							}
							break
						}
						releases[i].Notes = fr.ReleaseNotes
						releases[i].ChangeLog = fr.ChangeLog
						if fr.Version != "" {
							releases[i].Version = fr.Version
						}
						rep.NotesPulled++
					}
				}

				// Write with a fresh context, matching the products walk above:
				// a crawl budget that expired mid-walk must not discard the
				// releases already gathered.
				writeCtx, writeCancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Minute)
				err := st.UpsertReleases(writeCtx, releases)
				writeCancel()
				if err != nil {
					return fmt.Errorf("writing releases: %w", err)
				}
				if ctxErr := ctx.Err(); ctxErr != nil {
					rep.Warnings = append(rep.Warnings,
						"crawl budget reached (--max-duration); partial results were saved")
				}
				rep.Releases = len(releases)
			}

			rep.Elapsed = time.Since(started).Round(time.Second).String()
			if rep.NotesPulled == 0 && want["releases"] && flagNotes {
				rep.Note = "no release notes were captured; sign in with 'crestron-pp-cli auth login --chrome' to make them searchable"
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), rep, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"synced %d categories, %d products, %d releases (%d with notes) in %s across %d requests\n",
				rep.Categories, rep.Products, rep.Releases, rep.NotesPulled, rep.Elapsed, rep.Requests)
			for _, w := range rep.Warnings {
				fmt.Fprintln(cmd.OutOrStdout(), "  warning:", w)
			}
			if rep.Note != "" {
				fmt.Fprintln(cmd.OutOrStdout(), rep.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagResources, "resources", "", "Comma-separated: categories, products, releases (default all)")
	cmd.Flags().StringVar(&flagDB, "db", "", "Database path")
	cmd.Flags().IntVar(&flagMaxCats, "max-categories", 0, "Cap how many catalog categories are walked (0 = all)")
	cmd.Flags().BoolVar(&flagNotes, "notes", false, "Also pull release notes and change logs (requires a signed-in session)")
	cmd.Flags().IntVar(&flagMaxNotes, "max-notes", 100, "Cap how many release-note pages are fetched")
	cmd.Flags().IntVar(&flagConcurrent, "concurrency", 3, "Parallel category crawls")
	cmd.Flags().DurationVar(&flagMaxDuration, "max-duration", 30*time.Minute,
		"Overall crawl budget; the root --timeout applies per request, not to the whole sync")
	return cmd
}

// crawlCategoryProducts pages one category's tile endpoint until the category's
// own product count is reached. That count is the loop's termination condition,
// so the crawl never guesses when to stop.
func crawlCategoryProducts(ctx context.Context, c apiGetter, cat crestronstore.Category) (res struct {
	products []crestronstore.Product
	reqs     int
	warn     string
}) {
	res.products = make([]crestronstore.Product, 0)
	if cat.DocumentID == "" || cat.NodeID == "" || cat.ProductCount == 0 {
		return res
	}
	const pageSize = 50
	seen := map[string]bool{}
	for offset := 0; offset < cat.ProductCount; offset += pageSize {
		body, err := c.Get(ctx, "/CMSPages/ProductSubcategoryItemTemplate.aspx", map[string]string{
			"dId": cat.DocumentID, "nId": cat.NodeID,
			"ps": strconv.Itoa(pageSize), "os": strconv.Itoa(offset),
			"cult": "en-us", "sn": "Crestron", "sort": "PageViews", "fltr": "",
		})
		if err != nil {
			res.warn = fmt.Sprintf("category %s offset %d: %v", cat.Path, offset, err)
			return res
		}
		res.reqs++
		tiles, _, err := crestronparse.ParseProductTiles(body)
		if err != nil || len(tiles) == 0 {
			return res
		}
		for _, t := range tiles {
			if t.Model == "" || seen[t.Model] {
				continue
			}
			seen[t.Model] = true
			res.products = append(res.products, crestronstore.Product{
				Model: t.Model, Description: t.Description, URL: t.URL,
				DocumentID: t.DocumentID, CategoryPath: cat.Path,
				ImageURL: t.ImageURL, Discontinued: t.Discontinued,
			})
		}
	}
	return res
}

// apiGetter is the slice of the generated client this file needs, declared here
// so the crawl helpers stay testable.
type apiGetter interface {
	Get(ctx context.Context, path string, params map[string]string) (json.RawMessage, error)
}
