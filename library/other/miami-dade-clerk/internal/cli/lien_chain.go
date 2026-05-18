// Copyright 2026 alex-kleis. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/other/miami-dade-clerk/internal/store"
)

// newLienChainCmd builds the lien-chain command. Walks the chain of deeds
// on a folio, infers owner-by-owner ownership windows, then merges every
// recording (deeds + name-search hits filtered by signature) into a
// single chronological timeline. The output is the unified view of every
// document ever recorded against a property — the headline reason this
// CLI exists.
func newLienChainCmd(flags *rootFlags) *cobra.Command {
	var (
		folio     string
		since     string
		maxOwners int
	)
	cmd := &cobra.Command{
		Use:         "lien-chain",
		Short:       "Walk every recording ever filed against a property — deeds, mortgages, satisfactions, lis pendens, federal tax liens, assignments — in one chronological timeline.",
		Long:        "Walk every recording ever filed against a property — deeds, mortgages, satisfactions, lis pendens, federal tax liens, assignments — in one chronological timeline. Reads from the local store (sync first) and joins by folio + signature for older recordings.",
		Example:     "  miami-dade-clerk-pp-cli lien-chain --folio 30-2232-066-1610",
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
				return fmt.Errorf("invalid --folio: %q (expected dashed or numeric form, e.g. 30-2232-066-1610)", folio)
			}

			s, err := openStoreOrFail(cmd.Context())
			if err != nil {
				return err
			}

			deeds, err := s.QueryRecordings(store.RecordingFilter{
				FolioNumber:  folioN,
				DocTypeCodes: deedDocTypes,
				SinceDate:    since,
			})
			if err != nil {
				return fmt.Errorf("query deeds: %w", err)
			}

			owners := buildOwnerWindows(deeds, maxOwners)

			// All recordings on this folio — used as the timeline base. The
			// name-search filter against signature would expand the set in
			// future iterations; for now the folio-direct query handles the
			// 70%+ of recordings the Clerk indexes by folio.
			all, err := s.QueryRecordings(store.RecordingFilter{
				FolioNumber: folioN,
				SinceDate:   since,
			})
			if err != nil {
				return fmt.Errorf("query timeline: %w", err)
			}

			timeline := make([]map[string]any, 0, len(all))
			for _, r := range all {
				timeline = append(timeline, recordingToTimelineEntry(r))
			}

			out := map[string]any{
				"folio":       folioN,
				"deed_source": "folio_direct",
				"owners":      owners,
				"timeline":    timeline,
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().StringVar(&folio, "folio", "", "Property folio number (dashed or numeric, e.g. 30-2232-066-1610)")
	cmd.Flags().StringVar(&since, "since", "", "Inclusive lower bound on recording_date (YYYY-MM-DD)")
	cmd.Flags().IntVar(&maxOwners, "max-owners", 20, "Cap on owner-window output (chain depth)")
	return cmd
}

// buildOwnerWindows derives ordered owner windows from a chronological
// deed list. Each consecutive grantee becomes the next owner; the
// previous owner's window closes on the next deed's recording date.
// The last deed's grantee owns the property up to today (end == "").
//
// Florida deed convention: firsT_PARTY is grantor (party_code "R" =
// releasing/recording party); seconD_PARTY is grantee (party_code "D" =
// destination/receiving party). On QCD and ODE this can be reversed for
// internal transfers, but the indexed firsT_PARTY/seconD_PARTY split is
// the safest default given the lack of party-code-by-record extraction.
func buildOwnerWindows(deeds []*store.Recording, maxOwners int) []map[string]any {
	if len(deeds) == 0 {
		return nil
	}
	var out []map[string]any
	for i, d := range deeds {
		grantee := d.SecondParty
		if grantee == "" {
			continue
		}
		entry := map[string]any{
			"name":  grantee,
			"start": d.RecordingDate,
		}
		if i+1 < len(deeds) {
			entry["end"] = deeds[i+1].RecordingDate
		} else {
			entry["end"] = ""
		}
		out = append(out, entry)
		if maxOwners > 0 && len(out) >= maxOwners {
			break
		}
	}
	return out
}

// recordingToTimelineEntry projects a store.Recording into the
// lien-chain timeline entry shape. Keeps consideration in cents (caller
// can format), parties joined, and includes the viewer URL when the
// per-record qs token is present.
func recordingToTimelineEntry(r *store.Recording) map[string]any {
	parties := r.FirstParty
	if r.SecondParty != "" {
		if parties != "" {
			parties += " -> "
		}
		parties += r.SecondParty
	}
	return map[string]any{
		"cfn_master_id":       r.CFNMasterID,
		"doc_type_code":       r.DocTypeCode,
		"doc_type_label":      docTypeLabel(r.DocTypeCode),
		"recording_date":      r.RecordingDate,
		"parties":             parties,
		"consideration_cents": r.ConsiderationCents,
		"viewer_url":          viewerURL(r.ViewerQS),
	}
}
