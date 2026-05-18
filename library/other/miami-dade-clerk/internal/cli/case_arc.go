// Copyright 2026 alex-kleis. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/other/miami-dade-clerk/internal/store"
)

// newCaseArcCmd walks a single court case across all its recorded
// documents in chronological order and classifies the current state.
// The state machine is intentionally simple — the goal is "what
// stage is this case at?" not a full litigation tracker.
func newCaseArcCmd(flags *rootFlags) *cobra.Command {
	var caseNumber string
	cmd := &cobra.Command{
		Use:         "case-arc",
		Short:       "Walk a court case across all recorded documents in chronological order; classify the current state (open/judgment-entered/sale-complete/dismissed/satisfied).",
		Long:        "Walk a court case across all recorded documents in chronological order; classify the current state (open/judgment-entered/sale-complete/dismissed/satisfied). Reads from local store; sync first.",
		Example:     "  miami-dade-clerk-pp-cli case-arc --case 2024-020991-CA-01",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if caseNumber == "" {
				if flags.dryRun {
					return nil
				}
				return fmt.Errorf("required flag \"%s\" not set", "case")
			}
			if dryRunOK(flags) {
				return nil
			}
			s, err := openStoreOrFail(cmd.Context())
			if err != nil {
				return err
			}
			docs, err := s.QueryRecordings(store.RecordingFilter{
				CaseNumber: caseNumber,
				OrderBy:    "recording_date ASC, cfn_master_id ASC",
			})
			if err != nil {
				return fmt.Errorf("query case docs: %w", err)
			}

			status, currentOwner := classifyCaseStatus(docs)
			events := make([]map[string]any, 0, len(docs))
			for _, d := range docs {
				events = append(events, map[string]any{
					"recording_date": d.RecordingDate,
					"doc_type_code":  d.DocTypeCode,
					"doc_type_label": docTypeLabel(d.DocTypeCode),
					"parties":        joinParties(d.FirstParty, d.SecondParty),
					"cfn_master_id":  d.CFNMasterID,
					"viewer_url":     viewerURL(d.ViewerQS),
				})
			}
			return flags.printJSON(cmd, map[string]any{
				"case_number":   caseNumber,
				"status":        status,
				"events":        events,
				"current_owner": currentOwner,
			})
		},
	}
	cmd.Flags().StringVar(&caseNumber, "case", "", "Case number (e.g. 2024-020991-CA-01)")
	return cmd
}

// classifyCaseStatus walks the event list and emits the latest matching
// state. CTI (certificate of title) takes precedence over JUD because a
// post-judgment sale supersedes the judgment entry. SJU (Satisfaction
// of Judgment) after JUD is "satisfied"; DIS / "voluntary dismissal"
// closes a case without sale. Returns the current owner inferred from
// the most recent CTI grantee when a sale has occurred.
func classifyCaseStatus(docs []*store.Recording) (string, string) {
	status := "open"
	var currentOwner string
	hasJud := false
	for _, d := range docs {
		switch d.DocTypeCode {
		case "LIS":
			// Lis pendens filed — the case is "open" by default and a
			// LIS doesn't transition status. Listed explicitly so the
			// switch documents the intentional no-op (rather than the
			// reader wondering whether LIS was forgotten).
		case "JUD":
			status = "judgment-entered"
			hasJud = true
		case "CTI":
			status = "sale-complete"
			if d.SecondParty != "" {
				currentOwner = d.SecondParty
			}
		case "DIS":
			if status != "sale-complete" {
				status = "dismissed"
			}
		case "SJU":
			// Satisfaction of Judgment closes a JUD case without sale.
			// (SAT is Satisfaction of *Mortgage* — a different
			// instrument that closes a mortgage, not a court case;
			// surviving_liens.releaseMap pairs JUD with SJU, this
			// switch must use the same code.)
			if hasJud && status != "sale-complete" {
				status = "satisfied"
			}
		}
	}
	return status, currentOwner
}

func joinParties(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	return a + " v. " + b
}
