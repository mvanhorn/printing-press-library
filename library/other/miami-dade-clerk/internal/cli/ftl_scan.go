// Copyright 2026 alex-kleis. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/other/miami-dade-clerk/internal/store"
)

// newFTLScanCmd finds every Federal Tax Lien recorded since a given
// date. FTLs survive tax-deed foreclosure (26 USC §7425), so any FTL on
// a TD-bound property is a deal-breaker red flag. Optionally restricts
// the scan to a folio watchlist.
func newFTLScanCmd(flags *rootFlags) *cobra.Command {
	var (
		since         string
		folioListPath string
	)
	cmd := &cobra.Command{
		Use:         "ftl-scan",
		Short:       "Find every Federal Tax Lien recorded since a given date, optionally filtered to a folio watchlist.",
		Long:        "Find every Federal Tax Lien recorded since a given date, optionally filtered to a folio watchlist. FTLs survive tax-deed foreclosure — any FTL on a TD-bound property is a deal-breaker red flag.",
		Example:     "  miami-dade-clerk-pp-cli ftl-scan --since 2025-01-01",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if since == "" {
				if flags.dryRun {
					return nil
				}
				return fmt.Errorf("required flag \"%s\" not set", "since")
			}
			if dryRunOK(flags) {
				return nil
			}
			s, err := openStoreOrFail(cmd.Context())
			if err != nil {
				return err
			}

			hits, err := s.QueryRecordings(store.RecordingFilter{
				DocTypeCodes: []string{"FTL"},
				SinceDate:    since,
				OrderBy:      "recording_date DESC, cfn_master_id DESC",
			})
			if err != nil {
				return fmt.Errorf("query FTLs: %w", err)
			}

			// Optional folio watchlist filter — restrict to FTLs whose
			// folio_number matches any of the requested folios.
			var folioFilter map[int64]bool
			if folioListPath != "" {
				folios, err := readFolioList(folioListPath)
				if err != nil {
					return fmt.Errorf("reading folio list: %w", err)
				}
				folioFilter = map[int64]bool{}
				for _, f := range folios {
					if n := NormalizeFolio(f); n > 0 {
						folioFilter[n] = true
					}
				}
			}

			rows := make([]map[string]any, 0, len(hits))
			for _, r := range hits {
				if folioFilter != nil && !folioFilter[r.FolioNumber] {
					continue
				}
				ident := fmt.Sprintf("%d", r.FolioNumber)
				if r.FolioNumber == 0 {
					// No folio — fall back to subdivision/plat signature.
					ident = r.SubdivisionName
				}
				rows = append(rows, map[string]any{
					"cfn_master_id":      r.CFNMasterID,
					"recording_date":     r.RecordingDate,
					"taxpayer_name":      r.FirstParty,
					"irs_amount_cents":   r.ConsiderationCents,
					"viewer_url":         viewerURL(r.ViewerQS),
					"folio_or_signature": ident,
				})
			}
			return flags.printJSON(cmd, rows)
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "Inclusive lower bound on recording_date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&folioListPath, "folio-list", "", "Optional CSV file of folios to restrict the scan")
	return cmd
}
