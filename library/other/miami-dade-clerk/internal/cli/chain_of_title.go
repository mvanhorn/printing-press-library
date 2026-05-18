// Copyright 2026 alex-kleis. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/other/miami-dade-clerk/internal/store"
)

// newChainOfTitleCmd builds chain-of-title: ordered deeds with gap
// detection. A "gap" is any deed whose grantor doesn't match the prior
// deed's grantee — the canonical sign of a missing intermediate
// conveyance, a misspelled name, or a name-change event (marriage,
// trust, LLC reorganization). Surfaces the gap so the caller knows to
// dig further before assuming clean title.
func newChainOfTitleCmd(flags *rootFlags) *cobra.Command {
	var (
		folio string
		since string
	)
	cmd := &cobra.Command{
		Use:         "chain-of-title",
		Short:       "Ordered list of every deed conveying a property, with grantor/grantee/consideration, and gap detection.",
		Long:        "Ordered list of every deed conveying a property, with grantor/grantee/consideration, and gap detection where a grantee doesn't match the next deed's grantor.",
		Example:     "  miami-dade-clerk-pp-cli chain-of-title --folio 30-2232-066-1610",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if folio == "" {
				if flags.dryRun {
					return nil
				}
				return fmt.Errorf("required flag \"%s\" not set", "folio")
			}
			if dryRunOK(flags) {
				return nil
			}
			folioN := NormalizeFolio(folio)
			if folioN == 0 {
				return fmt.Errorf("invalid --folio: %q", folio)
			}
			sinceNorm, err := validateSinceFlag("since", since)
			if err != nil {
				return err
			}
			s, err := openStoreOrFail(cmd.Context())
			if err != nil {
				return err
			}

			deeds, err := s.QueryRecordings(store.RecordingFilter{
				FolioNumber:  folioN,
				DocTypeCodes: deedDocTypes,
				SinceDate:    sinceNorm,
			})
			if err != nil {
				return fmt.Errorf("query deeds: %w", err)
			}

			deedList := make([]map[string]any, 0, len(deeds))
			for _, d := range deeds {
				deedList = append(deedList, map[string]any{
					"cfn_master_id":       d.CFNMasterID,
					"recording_date":      d.RecordingDate,
					"grantor":             d.FirstParty,
					"grantee":             d.SecondParty,
					"consideration_cents": d.ConsiderationCents,
					"doc_type":            d.DocTypeCode,
					"viewer_url":          viewerURL(d.ViewerQS),
				})
			}

			gaps := detectChainGaps(deeds)

			return flags.printJSON(cmd, map[string]any{
				"folio": folioN,
				"since": sinceNorm,
				"deeds": deedList,
				"gaps":  gaps,
			})
		},
	}
	cmd.Flags().StringVar(&folio, "folio", "", "Property folio number")
	cmd.Flags().StringVar(&since, "since", "", "Inclusive lower bound on recording_date (YYYY-MM-DD)")
	return cmd
}

// detectChainGaps walks an ordered deed list and flags every transition
// where deeds[i].grantee != deeds[i+1].grantor. The match is
// case-insensitive and whitespace-tolerant — clerk records have
// inconsistent casing (KLEIS,ALEX vs Kleis, Alex) and extra spacing in
// the legal_description-derived party fields. Trust/LLC name changes
// will still flag here, but that's the desired behavior — surface the
// transition so a human can decide.
func detectChainGaps(deeds []*store.Recording) []map[string]any {
	var gaps []map[string]any
	for i := 0; i+1 < len(deeds); i++ {
		expected := normalizePartyForCompare(deeds[i].SecondParty)
		actual := normalizePartyForCompare(deeds[i+1].FirstParty)
		if expected == "" || actual == "" {
			continue
		}
		if expected == actual {
			continue
		}
		gaps = append(gaps, map[string]any{
			"between_date_a":   deeds[i].RecordingDate,
			"between_date_b":   deeds[i+1].RecordingDate,
			"expected_grantor": deeds[i].SecondParty,
			"actual_grantor":   deeds[i+1].FirstParty,
		})
	}
	return gaps
}

// normalizePartyForCompare lower-cases, trims, and collapses internal
// whitespace so trivially-different renderings of the same party match.
// Punctuation is preserved (the clerk indexes ",", "&", "JR" as
// meaningful tokens of name distinction).
func normalizePartyForCompare(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return ""
	}
	return strings.Join(strings.Fields(s), " ")
}
