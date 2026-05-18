// Copyright 2026 alex-kleis. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/other/miami-dade-clerk/internal/store"
)

// flSurvivability is the static Florida lien-survivability table. Used
// when sale_type is "foreclosure" (mortgage foreclosure) or "taxdeed"
// (tax-deed sale). Each entry answers the question: "if there is an
// open lien of this type on the property at the time of sale, does it
// survive the sale?"
//
// Confidence level: "static-table" — encodes the well-settled FL
// statute/caselaw rules. Edge cases (HOA super-lien safe harbor, IRS
// 120-day redemption right after TD, government priority liens beyond
// FTL) are NOT modeled here; downstream callers needing those should
// consult the BlueStone v3 lien-priority-analysis skill.
var flSurvivability = map[string]struct {
	SurvivesForeclosure bool
	SurvivesTaxDeed     bool
}{
	"MOR": {SurvivesForeclosure: false, SurvivesTaxDeed: false}, // junior wiped; senior already paid off at sale
	"FTL": {SurvivesForeclosure: true, SurvivesTaxDeed: true},   // federal tax liens survive both (26 USC §7425)
	"LIE": {SurvivesForeclosure: false, SurvivesTaxDeed: true},  // most state liens survive TD; junior wiped on FC
	"JUD": {SurvivesForeclosure: false, SurvivesTaxDeed: false}, // junior judgment liens wiped
	"LIS": {SurvivesForeclosure: false, SurvivesTaxDeed: false}, // lis pendens is procedural, not an open lien
}

// releaseMap maps lien doc types to the doc types that release them.
// Used to pair open liens against their satisfactions; an unmatched lien
// is considered still open at the time of sale.
var releaseMap = map[string][]string{
	"MOR": {"SAT", "REL"},
	"JUD": {"SJU"},
	"LIS": {"CLP", "DIS"},
	// FTL releases are filed as REL — federal-style "Certificate of
	// Release of Federal Tax Lien" (Form 668(Z)) shows up as REL.
	"FTL": {"REL"},
	// State LIE has no canonical release in this set; treat as always open.
}

// newSurvivingLiensCmd computes which open liens survive the given sale
// type. The output includes the surviving list, total cents, count, and
// a "survivability_confidence" string so downstream consumers can
// distinguish the static-table answer from a future case-by-case engine.
func newSurvivingLiensCmd(flags *rootFlags) *cobra.Command {
	var (
		folio      string
		assumeSale string
	)
	cmd := &cobra.Command{
		Use:         "surviving-liens",
		Short:       "Which liens on this property survive a foreclosure or tax-deed sale, with totals in cents.",
		Long:        "Which liens on this property survive a foreclosure or tax-deed sale, with totals in cents. Federal Tax Liens always survive tax deed; junior mortgages get wiped on senior foreclosure.",
		Example:     "  miami-dade-clerk-pp-cli surviving-liens --folio 30-2232-066-1610 --assume-sale-type taxdeed",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if folio == "" {
				if flags.dryRun {
					return nil
				}
				return fmt.Errorf("required flag \"%s\" not set", "folio")
			}
			if assumeSale != "foreclosure" && assumeSale != "taxdeed" {
				return fmt.Errorf("--assume-sale-type must be 'foreclosure' or 'taxdeed', got %q", assumeSale)
			}
			if dryRunOK(flags) {
				return nil
			}
			folioN := NormalizeFolio(folio)
			if folioN == 0 {
				return fmt.Errorf("invalid --folio: %q", folio)
			}

			s, err := openStoreOrFail(cmd.Context())
			if err != nil {
				return err
			}

			all, err := s.QueryRecordings(store.RecordingFilter{FolioNumber: folioN})
			if err != nil {
				return fmt.Errorf("query recordings: %w", err)
			}

			// Index releases by their reference target. The clerk indexes
			// the satisfaction's misC_REF field with the original lien's
			// clerk_File; we don't extract that here, so we pair by
			// approximate party-match across the lien/release pair.
			// First pass: collect releases keyed by doc_type_code.
			releases := map[string][]*store.Recording{}
			for _, r := range all {
				releases[r.DocTypeCode] = append(releases[r.DocTypeCode], r)
			}

			// Each release record can satisfy at most one lien. Track
			// already-consumed releases so two liens that share a party
			// name don't both get marked as released by the same
			// satisfaction. Bias is toward "still open in doubt" (FL
			// Statute 197.522/45.0315 favors conservative bidder
			// disclosure of unreleased encumbrances).
			usedReleases := map[int64]bool{}
			openLiens := make([]*store.Recording, 0, len(all))
			for _, r := range all {
				if _, ok := flSurvivability[r.DocTypeCode]; !ok {
					continue
				}
				if matchedReleaseID := findMatchingRelease(r, releases, usedReleases); matchedReleaseID != 0 {
					usedReleases[matchedReleaseID] = true
					continue
				}
				openLiens = append(openLiens, r)
			}

			surviving := make([]map[string]any, 0, len(openLiens))
			var totalCents int64
			for _, r := range openLiens {
				rule := flSurvivability[r.DocTypeCode]
				survives := rule.SurvivesForeclosure
				if assumeSale == "taxdeed" {
					survives = rule.SurvivesTaxDeed
				}
				if !survives {
					continue
				}
				parties := r.FirstParty
				if r.SecondParty != "" {
					parties += " / " + r.SecondParty
				}
				surviving = append(surviving, map[string]any{
					"cfn_master_id":  r.CFNMasterID,
					"doc_type_code":  r.DocTypeCode,
					"doc_type_label": docTypeLabel(r.DocTypeCode),
					"parties":        parties,
					"recording_date": r.RecordingDate,
					"amount_cents":   r.ConsiderationCents,
					"viewer_url":     viewerURL(r.ViewerQS),
				})
				totalCents += r.ConsiderationCents
			}

			out := map[string]any{
				"folio":                    folioN,
				"assume_sale_type":         assumeSale,
				"surviving":                surviving,
				"totals_cents":             totalCents,
				"count":                    len(surviving),
				"survivability_confidence": "static-table",
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().StringVar(&folio, "folio", "", "Property folio number (dashed or numeric)")
	cmd.Flags().StringVar(&assumeSale, "assume-sale-type", "foreclosure", "Assumed sale type: foreclosure | taxdeed")
	return cmd
}

// findMatchingRelease pairs a lien against an unconsumed release in the
// indexed release map and returns the matching release's CFNMasterID
// (or 0 when none matches). The match heuristic requires (a) the
// release type to be a recognized counterpart for the lien type, (b)
// the release recording_date to be on or after the lien recording_date,
// (c) BOTH parties of the lien to appear on the release in either
// position (mortgagor and lender — not just the mortgagor), and (d) the
// release not yet consumed by an earlier lien. This is conservative:
// without misC_REF extraction we'd rather mark a released lien as still
// open than mark an open lien as released, because surviving-liens
// drives bidder disclosure under FL Statute 197.522 / 45.0315.
func findMatchingRelease(lien *store.Recording, releases map[string][]*store.Recording, used map[int64]bool) int64 {
	candidates, ok := releaseMap[lien.DocTypeCode]
	if !ok {
		return 0
	}
	lienFirst := strings.TrimSpace(lien.FirstParty)
	lienSecond := strings.TrimSpace(lien.SecondParty)
	if lienFirst == "" || lienSecond == "" {
		return 0
	}
	// Clerk recordings have inconsistent casing across instruments
	// (the same party can appear as "JOHN DOE" on one filing and
	// "John Doe" on another). Case-sensitive comparison would keep a
	// satisfied lien in the surviving-liens output and inflate
	// totals_cents — drift we deliberately don't want here, especially
	// for bidder disclosure. EqualFold is ASCII-locale-safe; party
	// names from the clerk are uppercase ASCII in practice.
	for _, relType := range candidates {
		for _, rel := range releases[relType] {
			if rel == nil || used[rel.CFNMasterID] {
				continue
			}
			if rel.RecordingDate < lien.RecordingDate {
				continue
			}
			relFirst := strings.TrimSpace(rel.FirstParty)
			relSecond := strings.TrimSpace(rel.SecondParty)
			firstMatches := strings.EqualFold(lienFirst, relFirst) || strings.EqualFold(lienFirst, relSecond)
			secondMatches := strings.EqualFold(lienSecond, relFirst) || strings.EqualFold(lienSecond, relSecond)
			if firstMatches && secondMatches {
				return rel.CFNMasterID
			}
		}
	}
	return 0
}
