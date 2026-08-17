// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.
// Phase 3: overrides generated stub with NEJM meta-tag extraction.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func newArticlePromotedCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var enrich bool
	var citeFormat string

	cmd := &cobra.Command{
		Use:         "article <doi>",
		Short:       "Fetch NEJM article metadata and abstract by DOI",
		Long:        "Fetch title, authors, abstract, article type, specialties and free/paywalled flag for an NEJM article by DOI. Checks the local corpus first; fetches live via Surf transport when not cached.",
		Example:     "  nejm-pp-cli article 10.1056/NEJMoa2506905\n  nejm-pp-cli article 10.1056/NEJMoa2506905 --json",
		Annotations: map[string]string{"pp:endpoint": "article.get", "pp:method": "GET", "pp:path": "/doi/full/{doi}", "mcp:read-only": "true"},
	}

	cmd.PersistentFlags().StringVar(&dbPath, "db", "", "Path to local SQLite database (default: ~/.local/share/nejm-pp-cli/data.db)")
	cmd.PersistentFlags().BoolVar(&enrich, "enrich", false, "Force live fetch of article detail even if cached locally")

	// cite subcommand
	citeCmd := &cobra.Command{
		Use:     "cite <doi>",
		Short:   "Emit a citation for an article from the local corpus",
		Example: "  nejm-pp-cli article cite 10.1056/NEJMoa2506905\n  nejm-pp-cli article cite 10.1056/NEJMoa2506905 --format ris",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if len(args) < 1 {
				return usageErr(fmt.Errorf("doi is required"))
			}
			doi := strings.TrimSpace(args[0])

			if dbPath == "" {
				dbPath = defaultDBPath("nejm-pp-cli")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			db, err := nejmOpenStore(ctx)
			if err != nil {
				return err
			}
			defer db.Close()

			rows, queryErr := nejmQueryArticles(db, `doi = ?`, []any{doi}, 1)
			if queryErr != nil || len(rows) == 0 {
				// Try live fetch
				c, clientErr := flags.newClient()
				if clientErr != nil {
					return fmt.Errorf("article %s not in local corpus; live fetch failed: %w", doi, clientErr)
				}
				rec, fetchErr := nejmFetchAndParseArticle(ctx, c, doi)
				if fetchErr != nil {
					return fmt.Errorf("article %s not found: %w", doi, fetchErr)
				}
				rows = []map[string]any{
					{"doi": rec.DOI, "title": rec.Title, "authors": rec.Authors,
						"date": rec.Date, "url": rec.URL},
				}
			}

			row := rows[0]
			return nejmEmitCitation(cmd.OutOrStdout(), row, citeFormat)
		},
	}
	citeCmd.Flags().StringVar(&citeFormat, "format", "bibtex", "Citation format: bibtex or ris")
	cmd.AddCommand(citeCmd)

	// Default (get) behavior
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			if flags.asJSON {
				_ = printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"error": "doi is required",
					"usage": fmt.Sprintf("%s <doi>", cmd.CommandPath()),
				}, flags)
			}
			return usageErr(fmt.Errorf("doi is required\nUsage: %s <doi>", cmd.CommandPath()))
		}
		doi := strings.TrimSpace(args[0])

		if dryRunOK(flags) {
			return nil
		}

		if dbPath == "" {
			dbPath = defaultDBPath("nejm-pp-cli")
		}

		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()

		// Try local first (unless --enrich forces live)
		if !enrich {
			db, err := nejmOpenStore(ctx)
			if err == nil {
				defer db.Close()
				rows, err := nejmQueryArticles(db, `doi = ?`, []any{doi}, 1)
				if err == nil && len(rows) > 0 {
					data, _ := json.Marshal(rows[0])
					return printOutputWithFlags(cmd.OutOrStdout(), json.RawMessage(data), flags)
				}
			}
		}

		// Live fetch via Surf
		c, err := flags.newClient()
		if err != nil {
			return err
		}
		rec, err := nejmFetchAndParseArticle(ctx, c, doi)
		if err != nil {
			return fmt.Errorf("fetching article %s: %w", doi, err)
		}

		// Cache to local store (best-effort)
		if db, err := nejmOpenStore(ctx); err == nil {
			data, _ := json.Marshal(rec)
			_ = db.UpsertArticle(json.RawMessage(data))
			db.Close()
		}

		data, err := json.Marshal(rec)
		if err != nil {
			return err
		}

		if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
			filtered := json.RawMessage(data)
			if flags.selectFields != "" {
				filtered = filterFields(filtered, flags.selectFields)
			}
			return printOutput(cmd.OutOrStdout(), filtered, true)
		}

		var m map[string]any
		if json.Unmarshal(data, &m) == nil {
			if err := printAutoTable(cmd.OutOrStdout(), []map[string]any{m}); err != nil {
				return err
			}
			return nil
		}
		_ = os.Stderr
		return printOutputWithFlags(cmd.OutOrStdout(), json.RawMessage(data), flags)
	}

	return cmd
}

// nejmEmitCitation writes a BibTeX or RIS citation for a row to w.
func nejmEmitCitation(w interface{ Write([]byte) (int, error) }, row map[string]any, format string) error {
	doi, _ := row["doi"].(string)
	title, _ := row["title"].(string)
	authors, _ := row["authors"].(string)
	date, _ := row["date"].(string)
	url, _ := row["url"].(string)
	year := ""
	if len(date) >= 4 {
		year = date[:4]
	}
	// Build a BibTeX key from first-author surname + year
	key := "nejm" + year
	if parts := strings.Fields(authors); len(parts) > 0 {
		last := parts[len(parts)-1]
		last = strings.Map(func(r rune) rune {
			if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' {
				return r
			}
			return -1
		}, last)
		if last != "" {
			key = last + year
		}
	}

	switch strings.ToLower(strings.TrimSpace(format)) {
	case "ris":
		fmt.Fprintf(w, "TY  - JOUR\nTI  - %s\nAU  - %s\nPY  - %s\nDO  - %s\nUR  - %s\nJO  - New England Journal of Medicine\nER  -\n",
			title, authors, year, doi, url)
	default: // bibtex
		fmt.Fprintf(w, "@article{%s,\n  title={%s},\n  author={%s},\n  year={%s},\n  doi={%s},\n  url={%s},\n  journal={New England Journal of Medicine}\n}\n",
			key, title, authors, year, doi, url)
	}
	return nil
}
