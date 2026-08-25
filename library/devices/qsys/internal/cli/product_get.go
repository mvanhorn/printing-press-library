// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/devices/qsys/internal/qsys"
)

type relatedPage struct {
	Title   string `json:"title"`
	Section string `json:"section"`
	URL     string `json:"url"`
}

type productCard struct {
	Model        string        `json:"model"`
	Family       string        `json:"family"`
	URL          string        `json:"url"`
	Discontinued bool          `json:"discontinued"`
	Overview     string        `json:"overview,omitempty"`
	SpecPDFURL   string        `json:"spec_pdf_url,omitempty"`
	ManualPDFURL string        `json:"manual_pdf_url,omitempty"`
	SpecText     string        `json:"spec_text,omitempty"`
	ConfigPages  []relatedPage `json:"config_pages"`
	ConnectPages []relatedPage `json:"connect_pages"`
	Note         string        `json:"note,omitempty"`
}

func newNovelProductGetCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath   string
		full     bool
		maxPages int
	)
	cmd := &cobra.Command{
		Use:   "get [model]",
		Short: "See a Q-SYS product's overview, spec-sheet text, configuration pages, and connection guidance in one record.",
		Long: strings.Trim(`
Get returns everything the corpus knows about one product as a single record.

Q-SYS splits this across two websites: qsys.com carries the product page and its
spec-sheet PDF, help.qsys.com carries the configuration and networking guidance,
and neither links comprehensively to the other. This command performs that join
locally so one lookup answers spec, configuration, and wiring questions.

Model lookup is deliberately tolerant. A BOM usually carries a SKU (CX-Q 8K8)
while the vendor indexes a series (CX-Q), so exact match is tried first, then
prefix, then substring.
`, "\n"),
		Example: strings.Trim(`
  qsys-pp-cli product get CX-Q --agent
  qsys-pp-cli product get TSC-70-G3 --full
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true", "pp:happy-args": "model=CX-Q"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "product get")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a product model is required, e.g. CX-Q"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			dbPath = corpusDBPath(dbPath)
			if corpusMissing(cmd, flags, dbPath) {
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), make([]productCard, 0), flags)
				}
				return nil
			}
			st, err := openCorpus(ctx, dbPath)
			if err != nil {
				return err
			}
			defer st.Close()

			model := args[0]
			p, found, err := findProduct(ctx, st.DB(), model)
			if err != nil {
				return err
			}
			if !found {
				card := productCard{
					Model:        model,
					ConfigPages:  make([]relatedPage, 0),
					ConnectPages: make([]relatedPage, 0),
					Note:         fmt.Sprintf("no product matching %q in the local corpus; run `qsys-pp-cli harvest --only products` or try the series name (CX-Q rather than CX-Q 8K8)", model),
				}
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), card, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), card.Note)
				return nil
			}

			// Prefer the vendor's own product name from the page heading over the
			// slug-derived form: "Core 110f" reads correctly, "CORE-110F" does not.
			display := p.Title
			if strings.TrimSpace(display) == "" {
				display = p.Model
			}
			card := productCard{
				Model:        display,
				Family:       p.Family,
				URL:          p.URL,
				Discontinued: p.Discontinued,
				SpecPDFURL:   p.SpecPDFURL,
				ManualPDFURL: p.ManualPDFURL,
				ConfigPages:  make([]relatedPage, 0),
				ConnectPages: make([]relatedPage, 0),
			}
			card.Overview = truncateSpec(p.Overview, full, 1200)
			card.SpecText = truncateSpec(p.SpecText, full, 1500)

			if maxPages <= 0 {
				maxPages = 5
			}
			card.ConfigPages, err = relatedFor(ctx, st.DB(), p, []string{"Hardware", "Core_Manager", "Peripheral_Manager", "Administrator"}, maxPages)
			if err != nil {
				return err
			}
			card.ConnectPages, err = relatedFor(ctx, st.DB(), p, []string{"Networking", "Connect"}, maxPages)
			if err != nil {
				return err
			}

			switch {
			case p.SpecPDFURL == "":
				card.Note = "no spec sheet linked from this product page; specifications are unavailable for this model (see `qsys-pp-cli coverage`)"
			case p.SpecText == "":
				card.Note = "spec sheet found but text not extracted; run `qsys-pp-cli harvest --only products --with-pdfs`. The source PDF URL above is authoritative."
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), card, flags)
			}
			printCard(cmd, card)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Corpus database path")
	cmd.Flags().BoolVar(&full, "full", false, "Return untruncated overview and spec text")
	cmd.Flags().IntVar(&maxPages, "max-pages", 5, "Maximum related pages per category")
	return cmd
}

// truncate keeps agent payloads bounded by default. Spec-sheet text runs to
// thousands of characters; returning all of it on every lookup burns context
// for no benefit when the caller wanted a model number and a PDF link.
func truncateSpec(s string, full bool, n int) string {
	if full || len(s) <= n {
		return s
	}
	return strings.TrimSpace(s[:n]) + "… (use --full for the rest)"
}

// relatedFor finds help pages plausibly about this product. Matching is by
// model token against title and body within the relevant sections; it is a
// relevance heuristic, not an authoritative mapping, and callers should treat
// an empty list as "nothing matched", not "nothing exists".
func relatedFor(ctx context.Context, db *sql.DB, p qsys.Product, sections []string, limit int) ([]relatedPage, error) {
	out := make([]relatedPage, 0, limit)
	if len(sections) == 0 {
		return out, nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(sections)), ",")
	args := make([]any, 0, len(sections)+3)
	for _, s := range sections {
		args = append(args, s)
	}
	token := strings.ToLower(p.Model)
	args = append(args, token, token, limit)

	rows, err := db.QueryContext(ctx, `
		SELECT title, section, url FROM qsys_pages
		WHERE section IN (`+placeholders+`)
		  AND (lower(title) LIKE '%' || ? || '%' OR lower(body) LIKE '%' || ? || '%')
		ORDER BY length(title) LIMIT ?`, args...)
	if err != nil {
		return nil, fmt.Errorf("finding related pages: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var r relatedPage
		var title, section sql.NullString
		if err := rows.Scan(&title, &section, &r.URL); err != nil {
			return nil, fmt.Errorf("scanning related page: %w", err)
		}
		r.Title, r.Section = title.String, section.String
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating related pages: %w", err)
	}
	return out, nil
}

func printCard(cmd *cobra.Command, c productCard) {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "%s  (%s)\n%s\n", c.Model, c.Family, c.URL)
	if c.Discontinued {
		fmt.Fprintln(w, "STATUS: DISCONTINUED")
	}
	if c.SpecPDFURL != "" {
		fmt.Fprintf(w, "\nSpec sheet: %s\n", c.SpecPDFURL)
	}
	if c.ManualPDFURL != "" {
		fmt.Fprintf(w, "Manual:     %s\n", c.ManualPDFURL)
	}
	if c.Overview != "" {
		fmt.Fprintf(w, "\nOVERVIEW\n%s\n", c.Overview)
	}
	if c.SpecText != "" {
		fmt.Fprintf(w, "\nSPECIFICATIONS\n%s\n", c.SpecText)
	}
	printPages(w, "CONFIGURATION", c.ConfigPages)
	printPages(w, "CONNECTION", c.ConnectPages)
	if c.Note != "" {
		fmt.Fprintf(w, "\nnote: %s\n", c.Note)
	}
}

func printPages(w interface{ Write([]byte) (int, error) }, label string, pages []relatedPage) {
	if len(pages) == 0 {
		return
	}
	fmt.Fprintf(w, "\n%s\n", label)
	for _, p := range pages {
		fmt.Fprintf(w, "  %-46s %s\n", trimTo(p.Title, 46), p.URL)
	}
}

func trimTo(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

// Self-registration: `product get` hangs off the generated `product` parent,
// which a regen re-emits from the spec and would drop the novel AddCommand
// line from. Attaching from this preserved file keeps the leaf resolvable
// either way; addNovelCommandIfAbsent makes the double-registration a no-op.
func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		productCmd, _, err := root.Find([]string{"product"})
		if err == nil && productCmd != nil && productCmd != root {
			addNovelCommandIfAbsent(productCmd, newNovelProductGetCmd(flags))
		}
	})
}
