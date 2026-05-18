// Copyright 2026 magoo242. Licensed under Apache-2.0. See LICENSE.

// `edgar-pp-cli sections <TICKER> --form 10-K --items 1A,7` — extract
// requested Items from a 10-K or 10-Q with byte-offset boundaries.
// CRITICAL: applies the boundary safety contract — on ambiguity, emits
// {"error":"boundary_unverifiable",...} and returns exit code 2 rather
// than best-effort text. LODESTAR cites this verbatim in theses.

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/edgar/internal/store"
	"github.com/spf13/cobra"
)

func newSectionsCmd(flags *rootFlags) *cobra.Command {
	var form string
	var itemsArg string
	var accession string

	cmd := &cobra.Command{
		Use:   "sections <ticker-or-cik>",
		Short: "Extract named Items from a 10-K or 10-Q with byte-offset boundaries (boundary-safe)",
		Long: `Extract requested Items from the latest (or specified) 10-K or 10-Q with
byte-offset boundaries. Applies a strict boundary-safety contract: when an
Item header is missing, has multiple ambiguous matches, or no candidate has
substantial body content, the result is {"error":"boundary_unverifiable",...}
and the command exits with code 2 — NEVER best-effort text. LODESTAR
research cites these results verbatim; silent wrong-section answers
contaminate theses, so clean fails are the correct behavior.`,
		Example:     "  edgar-pp-cli sections AAPL --form 10-K --items 1A,7",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if itemsArg == "" {
				return usageErr(fmt.Errorf("--items is required (comma-separated, e.g., 1A,7)"))
			}
			if form == "" {
				form = "10-K"
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			db, err := store.OpenWithContext(cmd.Context(), edgarDBPath())
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer db.Close()
			if err := db.EnsureEdgarSchema(cmd.Context()); err != nil {
				return err
			}
			ec, err := resolveCIKOrTicker(cmd.Context(), c, db, args[0])
			if err != nil {
				return classifyAPIError(err, flags)
			}

			// Find target filing
			var target store.EdgarFiling
			if accession != "" {
				wd, _, aerr := normalizeAccession(accession)
				if aerr != nil {
					return usageErr(aerr)
				}
				target, err = db.GetEdgarFiling(cmd.Context(), wd)
				if err != nil {
					// Try fetching submissions to populate
					if _, ferr := fetchSubmissions(cmd.Context(), c, db, ec.CIK); ferr != nil {
						return classifyAPIError(ferr, flags)
					}
					target, err = db.GetEdgarFiling(cmd.Context(), wd)
					if err != nil {
						return notFoundErr(fmt.Errorf("accession %s not found for CIK %s", wd, ec.CIK))
					}
				}
			} else {
				if _, ferr := fetchSubmissions(cmd.Context(), c, db, ec.CIK); ferr != nil {
					return classifyAPIError(ferr, flags)
				}
				filings, lerr := db.ListEdgarFilings(cmd.Context(), ec.CIK, []string{form}, "", 1)
				if lerr != nil {
					return lerr
				}
				if len(filings) == 0 {
					return notFoundErr(fmt.Errorf("no %s filings found for CIK %s", form, ec.CIK))
				}
				target, err = db.GetEdgarFiling(cmd.Context(), filings[0].Accession)
				if err != nil {
					return err
				}
			}

			// Ensure body cached
			if target.BodyText == "" {
				if _, berr := fetchFilingBody(cmd.Context(), c, db, &target); berr != nil {
					return classifyAPIError(berr, flags)
				}
			}

			var items []string
			for _, it := range strings.Split(itemsArg, ",") {
				it = strings.TrimSpace(it)
				if it != "" {
					items = append(items, it)
				}
			}

			results, anyFailed := extractSections(target.BodyText, items)
			// Wrap with metadata
			payload := map[string]any{
				"cik":       ec.CIK,
				"accession": target.Accession,
				"form_type": target.FormType,
				"filed_at":  target.FiledAt,
				"sections":  results,
			}
			if err := emitJSON(cmd, flags, payload); err != nil {
				return err
			}
			if anyFailed {
				return &cliError{code: 2, err: fmt.Errorf("one or more items had boundary_unverifiable parse")}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&form, "form", "10-K", "Form type (10-K or 10-Q)")
	cmd.Flags().StringVar(&itemsArg, "items", "", "Comma-separated Item IDs to extract (e.g., 1A,7,9B)")
	cmd.Flags().StringVar(&accession, "accession", "", "Specific accession (default: latest filing of --form)")
	return cmd
}
