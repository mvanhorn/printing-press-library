// Copyright 2026 Mayank Lavania and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source local

package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/other/fpi-india/internal/nsdl"

	"github.com/spf13/cobra"
)

type aucTrendSnapshot struct {
	SyncedDate string  `json:"synced_date"`
	Total      float64 `json:"total"`
}

type aucTrendView struct {
	By        string             `json:"by"`
	Key       string             `json:"key"`
	Snapshots []aucTrendSnapshot `json:"snapshots"`
	Note      string             `json:"note,omitempty"`
}

func newNovelAucTrendCmd(flags *rootFlags) *cobra.Command {
	var flagBy string
	var flagCountry string

	cmd := &cobra.Command{
		Use:         "trend",
		Short:       "Track how a country's or category's share of assets under custody has changed across synced snapshots.",
		Example:     "  fpi-india-pp-cli auc trend --by country --country Mauritius --json",
		Long:        "Reports the assets-under-custody total for one country or category across every synced snapshot date. Requires at least two 'sync --resources auc' runs on different days; NSDL's page itself only ever shows the current snapshot. Use 'auc country'/'auc category' for the latest single snapshot instead.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compute AUC historical trend")
				return nil
			}
			by := flagBy
			if by == "" {
				by = "country"
			}
			if by != "country" && by != "category" {
				_ = cmd.Usage()
				return usageErrf("--by must be country or category")
			}
			if flagCountry == "" {
				_ = cmd.Usage()
				return usageErrf("--country is required (the country or category name to trend, e.g. --country Mauritius)")
			}

			db, exists, err := openLocalStore(cmd.Context())
			if err != nil {
				return err
			}
			if !exists {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: fpi-india-pp-cli sync --resources auc\n", defaultDBPath("fpi-india-pp-cli"))
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				}
				return nil
			}
			defer db.Close()

			recs, err := loadAUCSnapshots(db, by)
			if err != nil {
				return fmt.Errorf("reading synced AUC data: %w", err)
			}

			view := aucTrendView{By: by, Key: flagCountry}
			for _, r := range recs {
				if r.Key != flagCountry {
					continue
				}
				total, ok := nsdl.ParseNumber(r.Fields["Total"])
				if !ok {
					continue
				}
				view.Snapshots = append(view.Snapshots, aucTrendSnapshot{SyncedDate: r.SyncedDate, Total: total})
			}
			if len(view.Snapshots) == 0 {
				view.Note = fmt.Sprintf("no synced AUC snapshots found for %s %q; run sync --resources auc, wait for a later date, then sync again", by, flagCountry)
			} else if len(view.Snapshots) == 1 {
				view.Note = "only one synced snapshot so far; run sync --resources auc again on a later date to see a trend"
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printOutputWithFlagsMeta(cmd.OutOrStdout(), mustJSON(view), flags, map[string]any{"source": "local"})
			}
			for _, s := range view.Snapshots {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%.0f\n", s.SyncedDate, s.Total)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagBy, "by", "", "Dimension: country or category (default country)")
	cmd.Flags().StringVar(&flagCountry, "country", "", "Country or category name to trend (matches the exact name from 'auc country'/'auc category')")
	return cmd
}
