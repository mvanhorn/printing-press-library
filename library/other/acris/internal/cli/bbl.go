// Copyright 2026 not0xjarvis and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/mvanhorn/printing-press-library/library/other/acris/internal/cliutil"

	"github.com/spf13/cobra"
)

// bblDocument is one recorded document tied to a BBL, joined from Legals + Master.
type bblDocument struct {
	DocumentID       string `json:"document_id"`
	DocType          string `json:"doc_type"`
	DocumentAmt      string `json:"document_amt,omitempty"`
	DocumentDate     string `json:"document_date,omitempty"`
	RecordedDatetime string `json:"recorded_datetime,omitempty"`
	CRFN             string `json:"crfn,omitempty"`
}

// bblView is the full recorded-document history for a single BBL.
type bblView struct {
	BBL           string        `json:"bbl"`
	Borough       string        `json:"borough"`
	BoroughName   string        `json:"borough_name,omitempty"`
	Block         string        `json:"block"`
	Lot           string        `json:"lot"`
	DocumentCount int           `json:"document_count"`
	Documents     []bblDocument `json:"documents"`
	Note          string        `json:"note,omitempty"`
}

func newNovelBblCmd(flags *rootFlags) *cobra.Command {
	var flagBorough string
	var flagBlock string
	var flagLot string
	var flagMaxDocuments int

	cmd := &cobra.Command{
		Use:   "bbl",
		Short: "Resolve a borough/block/lot to its full recorded-document history in one call.",
		Long: "Resolve a borough/block/lot (BBL) to every recorded ACRIS document for that\n" +
			"tax lot, joining the Legals dataset (document <-> BBL) to the Master dataset\n" +
			"(document type, amount, and dates). Block and lot are not zero-padded.\n" +
			"Borough codes: 1=Manhattan 2=Bronx 3=Brooklyn 4=Queens 5=Staten Island.",
		Example:     "  acris-pp-cli bbl --borough 1 --block 852 --lot 134 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would resolve BBL to its recorded-document history")
				return nil
			}
			if flagBorough == "" || flagBlock == "" || flagLot == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--borough, --block, and --lot are all required"))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			maxDocs := flagMaxDocuments
			if cliutil.IsDogfoodEnv() && maxDocs > 25 {
				maxDocs = 25
			}

			legals, err := fetchACRISRows(ctx, c, acrisLegalsPath, map[string]string{
				"borough": flagBorough,
				"block":   flagBlock,
				"lot":     flagLot,
				"$limit":  strconv.Itoa(maxDocs * 4), // a lot can carry several legal rows per document
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}

			ids := distinctDocumentIDs(legals, maxDocs)
			view := bblView{
				BBL:         flagBorough + "-" + flagBlock + "-" + flagLot,
				Borough:     flagBorough,
				BoroughName: boroughNames[flagBorough],
				Block:       flagBlock,
				Lot:         flagLot,
				Documents:   []bblDocument{},
			}

			if len(ids) == 0 {
				view.Note = "no recorded documents found for this BBL; verify the borough code and that block/lot are not zero-padded"
				return emitBBLView(cmd, flags, view)
			}

			masters, err := fetchMastersByID(ctx, c, ids)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			for _, id := range ids {
				m, ok := masters[id]
				if !ok {
					continue
				}
				view.Documents = append(view.Documents, bblDocument{
					DocumentID:       id,
					DocType:          strField(m, "doc_type"),
					DocumentAmt:      strField(m, "document_amt"),
					DocumentDate:     strField(m, "document_date"),
					RecordedDatetime: strField(m, "recorded_datetime"),
					CRFN:             strField(m, "crfn"),
				})
			}
			// Most recent recordings first.
			sort.SliceStable(view.Documents, func(i, j int) bool {
				return view.Documents[i].RecordedDatetime > view.Documents[j].RecordedDatetime
			})
			view.DocumentCount = len(view.Documents)
			if maxDocs > 0 && len(ids) >= maxDocs {
				view.Note = fmt.Sprintf("results capped at %d documents; increase --max-documents to see more", maxDocs)
			}

			return emitBBLView(cmd, flags, view)
		},
	}
	cmd.Flags().StringVar(&flagBorough, "borough", "", "Borough code: 1=Manhattan 2=Bronx 3=Brooklyn 4=Queens 5=Staten Island")
	cmd.Flags().StringVar(&flagBlock, "block", "", "Tax block number (not zero-padded)")
	cmd.Flags().StringVar(&flagLot, "lot", "", "Tax lot number (not zero-padded)")
	cmd.Flags().IntVar(&flagMaxDocuments, "max-documents", 100, "Maximum documents to resolve for the BBL")
	return cmd
}

func emitBBLView(cmd *cobra.Command, flags *rootFlags, view bblView) error {
	if wantsHumanTable(cmd.OutOrStdout(), flags) {
		if view.DocumentCount == 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "BBL %s (%s): %s\n", view.BBL, view.BoroughName, view.Note)
			return nil
		}
		fmt.Fprintf(cmd.OutOrStdout(), "BBL %s (%s) — %d documents\n", view.BBL, view.BoroughName, view.DocumentCount)
		rows := make([]map[string]any, 0, len(view.Documents))
		for _, d := range view.Documents {
			rows = append(rows, map[string]any{
				"document_id":  d.DocumentID,
				"doc_type":     d.DocType,
				"document_amt": d.DocumentAmt,
				"recorded":     d.RecordedDatetime,
			})
		}
		if err := printAutoTable(cmd.OutOrStdout(), rows); err != nil {
			return err
		}
		if view.Note != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "note: %s\n", view.Note)
		}
		return nil
	}
	return printJSONFiltered(cmd.OutOrStdout(), view, flags)
}
