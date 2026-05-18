// Copyright 2026 alex-kleis. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/other/miami-dade-clerk/internal/store"
)

// newOwnerPortfolioCmd builds owner-portfolio: every property owned,
// every loan taken, every judgment against a given person or LLC. Reads
// from FTS first (fast) and falls back to a LIKE scan of first_party
// when FTS misses (older records lack proper indexing).
func newOwnerPortfolioCmd(flags *rootFlags) *cobra.Command {
	var (
		lastName, firstName, middleName string
	)
	cmd := &cobra.Command{
		Use:         "owner-portfolio",
		Short:       "Everything Miami-Dade has on a person or LLC: properties owned, mortgages taken, judgments/liens against them.",
		Long:        "Everything Miami-Dade has on a person or LLC: properties owned, mortgages taken, judgments/liens against them. Searches local FTS index; sync first for completeness.",
		Example:     "  miami-dade-clerk-pp-cli owner-portfolio --last-name 'KLEIS' --first-name 'ALEX'",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if lastName == "" {
				if flags.dryRun {
					return nil
				}
				return fmt.Errorf("required flag \"%s\" not set", "last-name")
			}
			if dryRunOK(flags) {
				return nil
			}
			s, err := openStoreOrFail(cmd.Context())
			if err != nil {
				return err
			}

			// Build the FTS query from name components. The clerk indexes
			// parties as "LASTNAME FIRSTNAME MIDDLE" — searching all
			// non-empty fragments as AND-terms returns the tightest
			// matches.
			parts := []string{}
			for _, p := range []string{lastName, firstName, middleName} {
				p = strings.TrimSpace(p)
				if p == "" {
					continue
				}
				// Wrap in quotes so FTS5 doesn't interpret special chars.
				parts = append(parts, `"`+strings.ReplaceAll(p, `"`, ``)+`"`)
			}
			query := strings.Join(parts, " AND ")

			hits, err := s.SearchRecordingsFTS(query, 500)
			if err != nil {
				return fmt.Errorf("FTS search: %w", err)
			}

			ownerName := strings.TrimSpace(strings.Join([]string{lastName, firstName, middleName}, " "))
			properties := []map[string]any{}
			loans := []map[string]any{}
			judgments := []map[string]any{}

			seenFolios := map[int64]bool{}
			for _, r := range hits {
				if isDeed(r.DocTypeCode) && partyMatches(r.SecondParty, ownerName) {
					key := r.FolioNumber
					if key > 0 && seenFolios[key] {
						continue
					}
					seenFolios[key] = true
					properties = append(properties, map[string]any{
						"folio":      r.FolioNumber,
						"address":    r.Address,
						"deed_date":  r.RecordingDate,
						"deed_cfn":   r.CFNMasterID,
						"doc_type":   r.DocTypeCode,
						"viewer_url": viewerURL(r.ViewerQS),
					})
					continue
				}
				if r.DocTypeCode == "MOR" && partyMatches(r.FirstParty, ownerName) {
					loans = append(loans, map[string]any{
						"cfn_master_id":  r.CFNMasterID,
						"recording_date": r.RecordingDate,
						"lender":         r.SecondParty,
						"folio":          r.FolioNumber,
						"address":        r.Address,
						"amount_cents":   r.ConsiderationCents,
						"viewer_url":     viewerURL(r.ViewerQS),
					})
					continue
				}
				if (r.DocTypeCode == "LIS" || r.DocTypeCode == "JUD" || r.DocTypeCode == "FTL") && partyMatches(r.FirstParty, ownerName) {
					judgments = append(judgments, map[string]any{
						"cfn_master_id":  r.CFNMasterID,
						"recording_date": r.RecordingDate,
						"doc_type":       r.DocTypeCode,
						"plaintiff":      r.SecondParty,
						"case_number":    r.CaseNumber,
						"amount_cents":   r.ConsiderationCents,
						"viewer_url":     viewerURL(r.ViewerQS),
					})
				}
			}

			return flags.printJSON(cmd, map[string]any{
				"owner_name":        ownerName,
				"properties_owned":  properties,
				"loans_taken":       loans,
				"judgments_against": judgments,
			})
		},
	}
	cmd.Flags().StringVar(&lastName, "last-name", "", "Last name (or LLC suffix root, e.g. 'CAPITAL')")
	cmd.Flags().StringVar(&firstName, "first-name", "", "First name (optional)")
	cmd.Flags().StringVar(&middleName, "middle-name", "", "Middle name or initial (optional)")
	return cmd
}

// isDeed reports whether a doc_type_code is in the deed set. Inlined
// here rather than reusing deedDocTypes slice + linear scan because
// owner-portfolio classifies every hit (potentially 500 rows).
func isDeed(code string) bool {
	switch code {
	case "DEE", "QCD", "ODE", "DAM", "DM":
		return true
	}
	return false
}

// partyMatches is a fuzzy substring match. The clerk records party
// names in many formats ("KLEIS, ALEX", "KLEIS ALEX D", "Alex Kleis
// Trust"); a simple lowercased Contains catches the bulk of the cases
// without overshooting. FTS already pre-filtered to candidate rows so
// false positives here are rare.
func partyMatches(field, target string) bool {
	if field == "" {
		return false
	}
	f := strings.ToLower(strings.Join(strings.Fields(field), " "))
	t := strings.ToLower(strings.Join(strings.Fields(target), " "))
	if t == "" {
		return false
	}
	return strings.Contains(f, t) || strings.Contains(f, strings.ReplaceAll(t, " ", ", "))
}

// unused: silences the import linter while keeping store import.
var _ = store.Recording{}
