// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/devices/averusa/internal/averusa"
	"github.com/mvanhorn/printing-press-library/library/devices/averusa/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/devices/averusa/internal/store"
)

// corpusDBPath resolves the local corpus database.
func corpusDBPath(override string) string {
	if override != "" {
		return override
	}
	return defaultDBPath("averusa-pp-cli")
}

// appendBounded keeps an error slice from growing without bound during a long
// harvest; the first max entries are kept, then a summary marker.
func appendBounded(errs []string, e string) []string {
	const max = 25
	if e == "" {
		return errs
	}
	if len(errs) >= max {
		if errs[len(errs)-1] != "..." {
			errs = append(errs[:max-1], "...")
		}
		return errs
	}
	return append(errs, e)
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newHarvestCmd(flags))
	})
}

// openCorpus opens the local corpus for reading and ensures the schema exists.
func openCorpus(ctx context.Context, dbPath string) (*store.Store, error) {
	st, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening corpus at %s: %w", dbPath, err)
	}
	if err := store.EnsureAVERUSASchema(ctx, st.DB()); err != nil {
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
		"no local corpus at %s\nrun: averusa-pp-cli harvest --db %s\n", dbPath, dbPath)
	return true
}

// ---------- harvest ----------

type harvestReport struct {
	DocsAttempted     int      `json:"docs_attempted"`
	Docs              int      `json:"docs"`
	DocsWithFile      int      `json:"docs_with_file"`
	EntitiesResolved  int      `json:"entities_resolved"`
	ProductsAttempted int      `json:"products_attempted"`
	Products          int      `json:"products"`
	ProductsWithSpec  int      `json:"products_with_spec_sheet"`
	SpecFields        int      `json:"spec_fields"`
	Discontinued      int      `json:"discontinued_models"`
	Errors            []string `json:"errors"`
	Note              string   `json:"note,omitempty"`
}

func newHarvestCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath    string
		only      string
		limit     int
		withSpecs bool
	)
	cmd := &cobra.Command{
		Use:   "harvest",
		Short: "Build the local AVer USA corpus from the support portal and averusa.com",
		Long: strings.Trim(`
Harvest walks the support-portal article sitemap (737 articles), fetches each
article's crawler-UA SSR page, resolves its Salesforce entityId through the Aura
action API, and records the doc type (user manual, spec sheet, white paper, ...)
and any attached file. It also walks the averusa.com product sitemap and scrapes
the /support/ discontinued-devices lists. Every read command (docs search,
compare, coverage, whats-new, products status, ...) works against this local
corpus.

This is separate from 'sync' on purpose: 'sync' handles the generated endpoint
resources, while the AVer corpus is two scraped websites plus a file layer that
must be joined locally. Run this first.

The full docs harvest makes roughly 1,500 rate-limited requests (article SSR +
entityId resolution each), so use --only and --limit to narrow it.
`, "\n"),
		Example: strings.Trim(`
  averusa-pp-cli harvest
  averusa-pp-cli harvest --only docs --limit 20
  averusa-pp-cli harvest --only products --with-specs
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "harvest")
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would harvest averusa.my.site.com and www.averusa.com into the local corpus")
				return nil
			}
			switch only {
			case "", "all", "docs", "products":
			default:
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--only must be one of: all, docs, products"))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			// A full harvest is far larger than the live-dogfood timeout allows,
			// so curtail rather than time out. Never substitute mock data.
			if cliutil.IsDogfoodEnv() {
				if only == "" || only == "all" {
					only = "docs"
				}
				if limit == 0 || limit > 3 {
					limit = 3
				}
				withSpecs = false
			}

			dbPath = corpusDBPath(dbPath)
			st, err := openCorpus(ctx, dbPath)
			if err != nil {
				return err
			}
			defer st.Close()

			c := averusa.New()
			rep := harvestReport{Errors: make([]string, 0)}
			now := time.Now().UTC().Format(time.RFC3339)

			if only == "" || only == "all" || only == "docs" {
				if err := harvestDocs(ctx, cmd, st.DB(), c, &rep, limit, now); err != nil {
					return err
				}
			}
			if only == "" || only == "all" || only == "products" {
				if err := harvestProducts(ctx, st.DB(), c, &rep, limit, withSpecs, now); err != nil {
					return err
				}
			}

			if err := recordAVERUSAHarvest(ctx, st.DB(), rep, now); err != nil {
				return err
			}
			return printAVERUSAHarvestReport(cmd, flags, rep)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "corpus database path (default: CLI state dir)")
	cmd.Flags().StringVar(&only, "only", "", "harvest only: all, docs, products")
	cmd.Flags().IntVar(&limit, "limit", 0, "cap the number of pages fetched per source (0 = no cap)")
	cmd.Flags().BoolVar(&withSpecs, "with-specs", false, "extract spec fields from datasheet PDFs (requires pdftotext)")
	return cmd
}

func harvestDocs(ctx context.Context, cmd *cobra.Command, db *sql.DB, c *averusa.Client, rep *harvestReport, limit int, now string) error {
	names, err := c.ArticleNames(ctx)
	if err != nil {
		rep.Errors = appendBounded(rep.Errors, "article sitemap: "+err.Error())
		return nil
	}
	if limit > 0 && limit < len(names) {
		names = names[:limit]
	}
	rep.DocsAttempted = len(names)

	fwuid, err := c.FetchFWUID(ctx)
	if err != nil {
		rep.Errors = appendBounded(rep.Errors, "fwuid: "+err.Error())
		fwuid = ""
	}

	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return err
		}
		a, err := c.Article(ctx, name)
		if err != nil {
			rep.Errors = appendBounded(rep.Errors, "article "+name+": "+err.Error())
			continue
		}
		entityID := ""
		if fwuid != "" {
			if id, err := c.ResolveEntityID(ctx, fwuid, name); err != nil {
				rep.Errors = appendBounded(rep.Errors, "entity "+name+": "+err.Error())
			} else {
				entityID = id
				if entityID != "" {
					rep.EntitiesResolved++
				}
			}
		}
		docType := averusa.ClassifyDocType(a.Title, a.URLName)
		model := averusa.ExtractModel(a.Title, a.URLName)
		hasFile := 0
		if a.HasFile {
			hasFile = 1
			rep.DocsWithFile++
		}
		pdfURL := ""
		if entityID != "" {
			pdfURL = averusa.FileFieldPath + "?entityId=" + entityID + "&field=File__Body__s"
		}
		if _, err := db.ExecContext(ctx,
			`INSERT INTO averusa_documents
			   (url_name, title, doc_type, model, entity_id, pdf_url, has_file, updated_at, body, synced_at)
			 VALUES(?,?,?,?,?,?,?,?,?,?)
			 ON CONFLICT(url_name) DO UPDATE SET
			   title=excluded.title, doc_type=excluded.doc_type, model=excluded.model,
			   entity_id=excluded.entity_id, pdf_url=excluded.pdf_url, has_file=excluded.has_file,
			   updated_at=excluded.updated_at, body=excluded.body, synced_at=excluded.synced_at`,
			a.URLName, a.Title, docType, model, entityID, pdfURL, hasFile, a.UpdatedAt, a.Body, now); err != nil {
			return fmt.Errorf("writing document %s: %w", name, err)
		}
		rep.Docs++
	}
	// Rebuild the document search index from the base table so the harvested
	// subset is findable. The corpus is harvest-owned and the FTS table has no
	// triggers, so a full rebuild keeps the index consistent with whatever
	// subset was just harvested.
	if _, err := db.ExecContext(ctx, `DELETE FROM averusa_documents_fts`); err != nil {
		return fmt.Errorf("clearing document search index: %w", err)
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO averusa_documents_fts(url_name, doc_type, model, title, body)
		 SELECT url_name, doc_type, model, title, body FROM averusa_documents`); err != nil {
		return fmt.Errorf("rebuilding document search index: %w", err)
	}
	return nil
}

func harvestProducts(ctx context.Context, db *sql.DB, c *averusa.Client, rep *harvestReport, limit int, withSpecs bool, now string) error {
	refs, err := c.ProductRefs(ctx)
	if err != nil {
		rep.Errors = appendBounded(rep.Errors, "product sitemap: "+err.Error())
		return nil
	}
	if limit > 0 && limit < len(refs) {
		refs = refs[:limit]
	}
	rep.ProductsAttempted = len(refs)

	disc, err := c.DiscontinuedModels(ctx)
	if err != nil {
		rep.Errors = appendBounded(rep.Errors, "discontinued list: "+err.Error())
	}
	discSet := map[string]bool{}
	for _, d := range disc {
		discSet[d.Slug] = true
	}
	rep.Discontinued = len(disc)

	for _, ref := range refs {
		if err := ctx.Err(); err != nil {
			return err
		}
		p, err := c.ProductPage(ctx, ref)
		if err != nil {
			rep.Errors = appendBounded(rep.Errors, "product "+ref.Slug+": "+err.Error())
			continue
		}
		discInt := 0
		if discSet[p.Slug] {
			discInt = 1
		}
		if _, err := db.ExecContext(ctx,
			`INSERT INTO averusa_products(slug, category, name, url, datasheet_url, discontinued, synced_at)
			 VALUES(?,?,?,?,?,?,?)
			 ON CONFLICT(slug) DO UPDATE SET
			   category=excluded.category, name=excluded.name, url=excluded.url,
			   datasheet_url=excluded.datasheet_url, discontinued=excluded.discontinued,
			   synced_at=excluded.synced_at`,
			p.Slug, p.Category, p.Name, p.URL, p.DatasheetURL, discInt, now); err != nil {
			return fmt.Errorf("writing product %s: %w", p.Slug, err)
		}
		rep.Products++
		if p.DatasheetURL != "" {
			rep.ProductsWithSpec++
			if withSpecs {
				txt, err := c.PDFText(ctx, p.DatasheetURL)
				if err != nil {
					// Missing pdftotext or a textless PDF is a soft degrade:
					// the datasheet URL is still stored and returned.
					rep.Errors = appendBounded(rep.Errors, "pdf "+p.DatasheetURL+": "+err.Error())
				} else {
					fields := averusa.ExtractSpecFields(p.Slug, txt)
					for field, value := range fields {
						if _, err := db.ExecContext(ctx,
							`INSERT INTO averusa_spec_fields(model, field, value, synced_at)
							 VALUES(?,?,?,?)
							 ON CONFLICT(model, field) DO UPDATE SET
							   value=excluded.value, synced_at=excluded.synced_at`,
							p.Slug, field, value, now); err != nil {
							return fmt.Errorf("writing spec field %s/%s: %w", p.Slug, field, err)
						}
						rep.SpecFields++
					}
				}
			}
		}
	}
	// Rebuild the product search index.
	if _, err := db.ExecContext(ctx, `DELETE FROM averusa_products_fts`); err != nil {
		return fmt.Errorf("clearing product search index: %w", err)
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO averusa_products_fts(slug, category, name)
		 SELECT slug, category, name FROM averusa_products`); err != nil {
		return fmt.Errorf("rebuilding product search index: %w", err)
	}
	// Insert discontinued-only models (present in the /support/ list but not
	// in the live catalog sitemap) so `products status` reports the full
	// discontinued roster, not just the overlap.
	for _, d := range disc {
		if discSet[d.Slug] {
			if _, err := db.ExecContext(ctx,
				`INSERT INTO averusa_products(slug, category, name, url, datasheet_url, discontinued, synced_at)
				 VALUES(?,?,?,?,?,1,?)
				 ON CONFLICT(slug) DO NOTHING`,
				d.Slug, "discontinued", d.Name, "", "", now); err != nil {
				return fmt.Errorf("writing discontinued product %s: %w", d.Slug, err)
			}
		}
	}
	return nil
}

func recordAVERUSAHarvest(ctx context.Context, db *sql.DB, rep harvestReport, now string) error {
	if _, err := db.ExecContext(ctx,
		`INSERT INTO averusa_harvest(source, attempted, succeeded, with_file, last_error, finished_at)
		 VALUES('docs',?,?,?,?,?)
		 ON CONFLICT(source) DO UPDATE SET
		   attempted=excluded.attempted, succeeded=excluded.succeeded,
		   with_file=excluded.with_file, last_error=excluded.last_error,
		   finished_at=excluded.finished_at`,
		"docs", rep.DocsAttempted, rep.Docs, rep.DocsWithFile,
		strings.Join(appendBounded(nil, strings.Join(rep.Errors, "; ")), "; "), now); err != nil {
		return fmt.Errorf("recording docs harvest: %w", err)
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO averusa_harvest(source, attempted, succeeded, with_file, last_error, finished_at)
		 VALUES('products',?,?,?,?,?)
		 ON CONFLICT(source) DO UPDATE SET
		   attempted=excluded.attempted, succeeded=excluded.succeeded,
		   with_file=excluded.with_file, last_error=excluded.last_error,
		   finished_at=excluded.finished_at`,
		"products", rep.ProductsAttempted, rep.Products, rep.ProductsWithSpec,
		strings.Join(rep.Errors, "; "), now); err != nil {
		return fmt.Errorf("recording products harvest: %w", err)
	}
	return nil
}

func printAVERUSAHarvestReport(cmd *cobra.Command, flags *rootFlags, rep harvestReport) error {
	if flags.asJSON {
		return flags.printJSON(cmd, rep)
	}
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "docs: %d/%d harvested (%d with files, %d entityIds resolved)\n",
		rep.Docs, rep.DocsAttempted, rep.DocsWithFile, rep.EntitiesResolved)
	fmt.Fprintf(w, "products: %d/%d harvested (%d with datasheets, %d discontinued found)\n",
		rep.Products, rep.ProductsAttempted, rep.ProductsWithSpec, rep.Discontinued)
	if rep.SpecFields > 0 {
		fmt.Fprintf(w, "spec fields: %d extracted\n", rep.SpecFields)
	}
	if len(rep.Errors) > 0 {
		fmt.Fprintf(cmd.ErrOrStderr(), "warnings (%d):\n", len(rep.Errors))
		for _, e := range rep.Errors {
			fmt.Fprintf(cmd.ErrOrStderr(), "  - %s\n", e)
		}
	}
	return nil
}
