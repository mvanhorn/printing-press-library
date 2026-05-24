// Copyright 2026 magoo242. Licensed under Apache-2.0. See LICENSE.

// `edgar-pp-cli eightk-items <TICKER>` — enumerate 8-K filings with parsed
// Item numbers and a --material-only flag that excludes exhibits-only (Item
// 9.01 alone) refilings.

package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/other/edgar/internal/store"
	"github.com/spf13/cobra"
)

type eightKItemResult struct {
	Accession      string            `json:"accession"`
	FiledAt        string            `json:"filed_at"`
	Items          []string          `json:"items"`
	IsMaterial     bool              `json:"is_material"`
	SummaryPerItem map[string]string `json:"summary_per_item,omitempty"`
}

func newEightKItemsCmd(flags *rootFlags) *cobra.Command {
	var since string
	var materialOnly bool

	cmd := &cobra.Command{
		Use:         "eightk-items <ticker-or-cik>",
		Short:       "Enumerate 8-K filings with parsed Item numbers; --material-only skips exhibits-only refilings",
		Example:     "  edgar-pp-cli eightk-items AAPL --since 90d --material-only",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
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
			sinceISO, perr := parseSinceDate(since)
			if perr != nil {
				return usageErr(perr)
			}

			// Ensure cache populated
			if _, ferr := fetchSubmissions(cmd.Context(), c, db, ec.CIK); ferr != nil {
				return classifyAPIError(ferr, flags)
			}

			filings, err := db.ListEdgarFilings(cmd.Context(), ec.CIK, []string{"8-K", "8-K/A"}, sinceISO, 100)
			if err != nil {
				return err
			}

			var out []eightKItemResult
			for _, f := range filings {
				// Fetch body text if not cached
				cached, _ := db.GetEdgarFiling(cmd.Context(), f.Accession)
				bodyText := cached.BodyText
				if bodyText == "" && cached.PrimaryDocURL != "" {
					if text, berr := fetchFilingBody(cmd.Context(), c, db, &cached); berr == nil {
						bodyText = text
					}
				}
				items := parseEightKItems(bodyText)
				isMaterial := false
				for _, it := range items {
					if it != "9.01" {
						isMaterial = true
						break
					}
				}
				if materialOnly && !isMaterial {
					continue
				}
				summary := map[string]string{}
				for _, it := range items {
					if !validEightKItems[it] {
						continue
					}
					summary[it] = extractFirstSentenceAfterItem(bodyText, it, 200)
				}
				out = append(out, eightKItemResult{
					Accession:      f.Accession,
					FiledAt:        f.FiledAt,
					Items:          items,
					IsMaterial:     isMaterial,
					SummaryPerItem: summary,
				})
			}
			return emitJSON(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "Earliest filing date (ISO or 90d/12mo)")
	cmd.Flags().BoolVar(&materialOnly, "material-only", false, "Exclude filings whose only Item is 9.01 (exhibits)")
	return cmd
}
