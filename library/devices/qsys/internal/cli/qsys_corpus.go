// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/devices/qsys/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/devices/qsys/internal/qsys"
	"github.com/mvanhorn/printing-press-library/library/devices/qsys/internal/store"
)

// corpusDBPath resolves the local corpus database.
func corpusDBPath(override string) string {
	if override != "" {
		return override
	}
	return defaultDBPath("qsys-pp-cli")
}

// openCorpus opens the local corpus for reading and ensures the schema exists.
func openCorpus(ctx context.Context, dbPath string) (*store.Store, error) {
	st, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening corpus at %s: %w", dbPath, err)
	}
	if err := store.EnsureQSYSSchema(ctx, st.DB()); err != nil {
		st.Close()
		return nil, err
	}
	return st, nil
}

// corpusMissing reports whether the corpus file exists yet. Novel read commands
// call this before opening SQLite so a first run returns an honest empty result
// plus a hint, rather than a raw database error.
func corpusMissing(cmd *cobra.Command, flags *rootFlags, dbPath string) bool {
	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		return false
	}
	fmt.Fprintf(cmd.ErrOrStderr(),
		"no local corpus at %s\nrun: qsys-pp-cli harvest --db %s\n", dbPath, dbPath)
	return true
}

// ---------- harvest ----------

type harvestReport struct {
	Pages             int      `json:"pages"`
	PagesAttempted    int      `json:"pages_attempted"`
	Products          int      `json:"products"`
	ProductsAttempted int      `json:"products_attempted"`
	WithSpecSheet     int      `json:"products_with_spec_sheet"`
	SpecTextExtracted int      `json:"products_with_spec_text"`
	CompatRows        int      `json:"compat_rows"`
	Support           int      `json:"support_articles"`
	SupportAttempted  int      `json:"support_articles_attempted"`
	Errors            []string `json:"errors"`
	Note              string   `json:"note,omitempty"`
}

func newHarvestCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath   string
		only     string
		limit    int
		withPDFs bool
	)
	cmd := &cobra.Command{
		Use:   "harvest",
		Short: "Build the local Q-SYS corpus from help.qsys.com, qsys.com, and support.qsys.com",
		Long: strings.Trim(`
Harvest walks all three vendor sitemaps and builds the local corpus that every
other command reads.

This is separate from 'sync' on purpose: 'sync' handles the generated endpoint
resources, while the Q-SYS corpus is three scraped websites plus a PDF layer
that must be joined locally. Run this first.

The full harvest fetches roughly 750 help pages, 270 product pages, and 1,900
support articles, rate limited to be polite to the vendor servers. Use --only
and --limit to narrow it.

--only support is what makes 'fault', 'bom risks', and 'qds' work: those three
read the knowledge base, and without it they return empty with a hint.
`, "\n"),
		Example: strings.Trim(`
  qsys-pp-cli harvest
  qsys-pp-cli harvest --only compat
  qsys-pp-cli harvest --only support
  qsys-pp-cli harvest --only products --limit 25 --with-pdfs
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "harvest")
			}
			// Verify-mode short-circuit: a harvest is a long network-bound
			// build of the local corpus. Under the verifier we report the
			// intent and exit cleanly instead of walking both vendor sites.
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would harvest help.qsys.com, qsys.com, and support.qsys.com into the local corpus")
				return nil
			}
			switch only {
			case "", "all", "pages", "products", "compat", "support":
			default:
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--only must be one of: all, pages, products, compat, support"))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			// A full harvest is far larger than the live-dogfood timeout allows,
			// so curtail rather than time out. Never substitute mock data.
			if cliutil.IsDogfoodEnv() {
				if only == "" || only == "all" {
					only = "compat"
				}
				if limit == 0 || limit > 3 {
					limit = 3
				}
				withPDFs = false
			}

			dbPath = corpusDBPath(dbPath)
			st, err := openCorpus(ctx, dbPath)
			if err != nil {
				return err
			}
			defer st.Close()

			c := qsys.New()
			rep := harvestReport{Errors: make([]string, 0)}
			now := time.Now().UTC().Format(time.RFC3339)

			if only == "" || only == "all" || only == "compat" {
				rows, err := c.CompatMatrix(ctx)
				if err != nil {
					rep.Errors = append(rep.Errors, "compat: "+err.Error())
				} else {
					for _, r := range rows {
						if _, err := st.DB().ExecContext(ctx,
							`INSERT INTO qsys_compat(qds_version, release_date, added_hardware, removed_hardware, synced_at)
							 VALUES(?,?,?,?,?)
							 ON CONFLICT(qds_version) DO UPDATE SET
							   release_date=excluded.release_date,
							   added_hardware=excluded.added_hardware,
							   removed_hardware=excluded.removed_hardware,
							   synced_at=excluded.synced_at`,
							r.QDSVersion, r.ReleaseDate, r.AddedHardware, r.RemovedHardware, now); err != nil {
							return fmt.Errorf("writing compat row %s: %w", r.QDSVersion, err)
						}
						rep.CompatRows++
					}
				}
			}

			if only == "" || only == "all" || only == "pages" {
				urls, err := c.Sitemap(ctx, qsys.HelpHost+"/sitemap.xml")
				if err != nil {
					rep.Errors = append(rep.Errors, "help sitemap: "+err.Error())
				} else {
					pages := qsys.HelpPages(urls)
					if limit > 0 && limit < len(pages) {
						pages = pages[:limit]
					}
					rep.PagesAttempted = len(pages)
					for _, u := range pages {
						p, err := c.Page(ctx, u)
						if err != nil {
							rep.Errors = appendBounded(rep.Errors, "page "+u+": "+err.Error())
							continue
						}
						if _, err := st.DB().ExecContext(ctx,
							`INSERT INTO qsys_pages(url, section, title, body, synced_at)
							 VALUES(?,?,?,?,?)
							 ON CONFLICT(url) DO UPDATE SET
							   section=excluded.section, title=excluded.title,
							   body=excluded.body, synced_at=excluded.synced_at`,
							p.URL, p.Section, p.Title, p.Text, now); err != nil {
							return fmt.Errorf("writing page %s: %w", u, err)
						}
						rep.Pages++
					}
					// Rebuild the page search index from the base table so the
					// harvested pages are findable. The corpus is harvest-owned
					// and the FTS tables have no triggers, so a full rebuild
					// per phase keeps the index consistent with whatever
					// subset was just harvested.
					if _, err := st.DB().ExecContext(ctx, `DELETE FROM qsys_pages_fts`); err != nil {
						return fmt.Errorf("clearing page search index: %w", err)
					}
					if _, err := st.DB().ExecContext(ctx,
						`INSERT INTO qsys_pages_fts(url, section, title, body)
						 SELECT url, section, title, body FROM qsys_pages`); err != nil {
						return fmt.Errorf("rebuilding page search index: %w", err)
					}
				}
			}

			if only == "" || only == "all" || only == "products" {
				urls, err := c.Sitemap(ctx, qsys.ProductHost+"/sitemap.xml")
				if err != nil {
					rep.Errors = append(rep.Errors, "product sitemap: "+err.Error())
				} else {
					prods := qsys.ProductPages(urls)
					if limit > 0 && limit < len(prods) {
						prods = prods[:limit]
					}
					rep.ProductsAttempted = len(prods)
					for _, u := range prods {
						p, err := c.Product(ctx, u)
						if err != nil {
							rep.Errors = appendBounded(rep.Errors, "product "+u+": "+err.Error())
							continue
						}
						if p.SpecPDFURL != "" {
							rep.WithSpecSheet++
							if withPDFs {
								if txt, err := c.PDFText(ctx, p.SpecPDFURL); err != nil {
									// Missing pdftotext is a soft degrade: the PDF
									// URL is still stored and returned to the user.
									rep.Errors = appendBounded(rep.Errors, "pdf "+p.SpecPDFURL+": "+err.Error())
								} else {
									p.SpecText = txt
									rep.SpecTextExtracted++
								}
							}
						}
						disc, isProd := 0, 0
						if p.Discontinued {
							disc = 1
						}
						if p.IsProduct {
							isProd = 1
						}
						if _, err := st.DB().ExecContext(ctx,
							`INSERT INTO qsys_products(model, title, is_product, family, slug, url, overview, spec_pdf_url, manual_pdf_url, spec_text, discontinued, synced_at)
							 VALUES(?,?,?,?,?,?,?,?,?,?,?,?)
							 ON CONFLICT(model) DO UPDATE SET
							   title=excluded.title, is_product=excluded.is_product,
							   family=excluded.family, slug=excluded.slug, url=excluded.url,
							   overview=excluded.overview, spec_pdf_url=excluded.spec_pdf_url,
							   manual_pdf_url=excluded.manual_pdf_url,
							   spec_text=CASE WHEN excluded.spec_text != '' THEN excluded.spec_text ELSE qsys_products.spec_text END,
							   discontinued=excluded.discontinued, synced_at=excluded.synced_at`,
							p.Model, p.Title, isProd, p.Family, p.Slug, p.URL, p.Overview, p.SpecPDFURL, p.ManualPDFURL, p.SpecText, disc, now); err != nil {
							return fmt.Errorf("writing product %s: %w", p.Model, err)
						}
						rep.Products++
					}
					// Rebuild the product search index from the base table
					// (model, family, overview, spec text).
					if _, err := st.DB().ExecContext(ctx, `DELETE FROM qsys_products_fts`); err != nil {
						return fmt.Errorf("clearing product search index: %w", err)
					}
					if _, err := st.DB().ExecContext(ctx,
						`INSERT INTO qsys_products_fts(model, family, overview, spec_text)
						 SELECT model, family, overview, spec_text FROM qsys_products`); err != nil {
						return fmt.Errorf("rebuilding product search index: %w", err)
					}
				}
			}

			if only == "" || only == "all" || only == "support" {
				urls, err := c.Sitemap(ctx, qsys.SupportHost+"/sitemap.xml")
				if err != nil {
					rep.Errors = append(rep.Errors, "support sitemap: "+err.Error())
				} else {
					arts := qsys.SupportArticles(urls)
					if limit > 0 && limit < len(arts) {
						arts = arts[:limit]
					}
					rep.SupportAttempted = len(arts)
					for _, u := range arts {
						a, err := c.SupportArticle(ctx, u)
						if err != nil {
							rep.Errors = appendBounded(rep.Errors, "support "+u+": "+err.Error())
							continue
						}
						if _, err := st.DB().ExecContext(ctx,
							`INSERT INTO qsys_support(url, category, slug, title, body, synced_at)
							 VALUES(?,?,?,?,?,?)
							 ON CONFLICT(url) DO UPDATE SET
							   category=excluded.category, slug=excluded.slug,
							   title=excluded.title, body=excluded.body,
							   synced_at=excluded.synced_at`,
							a.URL, a.Category, a.Slug, a.Title, a.Text, now); err != nil {
							return fmt.Errorf("writing support article %s: %w", u, err)
						}
						rep.Support++
					}
					// Same full-rebuild rule as the other two sources: the FTS
					// tables carry no triggers, so the index is rebuilt from
					// the base table after each phase.
					if _, err := st.DB().ExecContext(ctx, `DELETE FROM qsys_support_fts`); err != nil {
						return fmt.Errorf("clearing support search index: %w", err)
					}
					if _, err := st.DB().ExecContext(ctx,
						`INSERT INTO qsys_support_fts(url, category, title, body)
						 SELECT url, category, title, body FROM qsys_support`); err != nil {
						return fmt.Errorf("rebuilding support search index: %w", err)
					}
				}
			}

			if err := recordHarvest(ctx, st.DB(), rep, now); err != nil {
				return err
			}
			if !withPDFs && rep.WithSpecSheet > 0 {
				rep.Note = "spec-sheet text not extracted; re-run with --with-pdfs to make specifications searchable"
			}
			if len(rep.Errors) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"warning: %d item(s) failed to harvest; counts above exclude them\n", len(rep.Errors))
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), rep, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"pages     %d/%d\nproducts  %d/%d (%d with spec sheet, %d with extracted text)\ncompat    %d rows\nsupport   %d/%d articles\nerrors    %d\ndatabase  %s\n",
				rep.Pages, rep.PagesAttempted, rep.Products, rep.ProductsAttempted,
				rep.WithSpecSheet, rep.SpecTextExtracted, rep.CompatRows,
				rep.Support, rep.SupportAttempted, len(rep.Errors), dbPath)
			if rep.Note != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "note      %s\n", rep.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Corpus database path")
	cmd.Flags().StringVar(&only, "only", "", "Harvest a subset: all, pages, products, compat, support")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum items per source (0 = no limit)")
	cmd.Flags().BoolVar(&withPDFs, "with-pdfs", false, "Also download and text-extract spec-sheet PDFs (slower; needs pdftotext)")
	return cmd
}

// appendBounded caps the retained error list so a site-wide outage cannot
// produce a multi-megabyte JSON payload.
func appendBounded(errs []string, msg string) []string {
	const max = 25
	if len(errs) >= max {
		return errs
	}
	return append(errs, msg)
}

func recordHarvest(ctx context.Context, db *sql.DB, rep harvestReport, now string) error {
	rows := []struct {
		source               string
		attempted, ok, specs int
	}{
		{"pages", rep.PagesAttempted, rep.Pages, 0},
		{"products", rep.ProductsAttempted, rep.Products, rep.WithSpecSheet},
		{"compat", rep.CompatRows, rep.CompatRows, 0},
		{"support", rep.SupportAttempted, rep.Support, 0},
	}
	lastErr := ""
	if len(rep.Errors) > 0 {
		lastErr = rep.Errors[len(rep.Errors)-1]
	}
	for _, r := range rows {
		if r.attempted == 0 && r.ok == 0 {
			continue // this source was not part of the run; leave prior stats intact
		}
		if _, err := db.ExecContext(ctx,
			`INSERT INTO qsys_harvest(source, attempted, succeeded, with_specs, last_error, finished_at)
			 VALUES(?,?,?,?,?,?)
			 ON CONFLICT(source) DO UPDATE SET
			   attempted=excluded.attempted, succeeded=excluded.succeeded,
			   with_specs=excluded.with_specs, last_error=excluded.last_error,
			   finished_at=excluded.finished_at`,
			r.source, r.attempted, r.ok, r.specs, lastErr, now); err != nil {
			return fmt.Errorf("recording harvest stats: %w", err)
		}
	}
	return nil
}

// ---------- shared lookup ----------

// findProduct resolves a user-supplied model to a stored product.
//
// Lookup is tolerant because the same device is spelled three different ways
// across the sources this CLI joins: the vendor slug is "core-110f", the spec
// sheet says "Core 110f", and a BOM might carry "CORE 110F" or a fuller SKU
// like "CX-Q 8K8". Matching therefore compares an alphanumeric-only fold of
// both sides, so separators never decide a match.
//
// Order: exact fold, then stored-model-is-a-prefix-of-query (series match for a
// SKU), then substring. Real products outrank marketing articles at every step.
func findProduct(ctx context.Context, db *sql.DB, model string) (qsys.Product, bool, error) {
	if strings.TrimSpace(model) == "" {
		return qsys.Product{}, false, nil
	}
	want := foldModel(model)

	rows, err := db.QueryContext(ctx,
		`SELECT model, title, family, slug, url, overview, spec_pdf_url, manual_pdf_url, spec_text, discontinued, is_product
		 FROM qsys_products`)
	if err != nil {
		return qsys.Product{}, false, fmt.Errorf("looking up %q: %w", model, err)
	}
	defer rows.Close()

	type scored struct {
		p    qsys.Product
		rank int
	}
	best := scored{rank: -1}
	for rows.Next() {
		var p qsys.Product
		var disc, isProd int
		var title, overview, specURL, manualURL, specText sql.NullString
		if err := rows.Scan(&p.Model, &title, &p.Family, &p.Slug, &p.URL,
			&overview, &specURL, &manualURL, &specText, &disc, &isProd); err != nil {
			return qsys.Product{}, false, fmt.Errorf("scanning product: %w", err)
		}
		p.Title, p.Overview = title.String, overview.String
		p.SpecPDFURL, p.ManualPDFURL, p.SpecText = specURL.String, manualURL.String, specText.String
		p.Discontinued, p.IsProduct = disc == 1, isProd == 1

		have := foldModel(p.Model)
		haveTitle := foldModel(p.Title)
		rank := -1
		switch {
		case have == want || haveTitle == want:
			rank = 400
		case have != "" && strings.HasPrefix(want, have):
			rank = 300 + len(have) // prefer the longest series that still matches
		case have != "" && strings.Contains(have, want):
			rank = 200
		case haveTitle != "" && strings.Contains(haveTitle, want):
			rank = 100
		}
		if rank < 0 {
			continue
		}
		if p.IsProduct {
			rank += 1000 // a real product always outranks an article
		}
		if rank > best.rank {
			best = scored{p: p, rank: rank}
		}
	}
	if err := rows.Err(); err != nil {
		return qsys.Product{}, false, fmt.Errorf("iterating products: %w", err)
	}
	if best.rank < 0 {
		return qsys.Product{}, false, nil
	}
	return best.p, true, nil
}

// foldModel reduces a model string to lowercase alphanumerics so "Core 110f",
// "core-110f", and "CORE110F" all compare equal.
func foldModel(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// readModels merges positional args with newline-separated stdin, so an
// equipment list can be piped in from a BOM export.
func readModels(cmd *cobra.Command, args []string) []string {
	out := make([]string, 0, len(args))
	out = append(out, args...)
	if info, err := os.Stdin.Stat(); err == nil && (info.Mode()&os.ModeCharDevice) == 0 {
		buf := make([]byte, 0, 4096)
		tmp := make([]byte, 4096)
		for {
			n, err := cmd.InOrStdin().Read(tmp)
			buf = append(buf, tmp[:n]...)
			if err != nil || n == 0 {
				break
			}
		}
		for _, line := range strings.Split(string(buf), "\n") {
			if s := strings.TrimSpace(line); s != "" {
				out = append(out, s)
			}
		}
	}
	return out
}

var registerCorpusOnce sync.Once

func init() {
	registerCorpusOnce.Do(func() {
		registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
			addNovelCommandIfAbsent(root, newHarvestCmd(flags))
		})
	})
}
