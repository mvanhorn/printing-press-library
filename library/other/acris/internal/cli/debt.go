// Copyright 2026 not0xjarvis and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/mvanhorn/printing-press-library/library/other/acris/internal/cliutil"

	"github.com/spf13/cobra"
)

// debtDocument is one mortgage/debt-instrument recording for a BBL.
type debtDocument struct {
	DocumentID       string `json:"document_id"`
	DocType          string `json:"doc_type"`
	DocTypeDesc      string `json:"doc_type_description,omitempty"`
	DocumentAmt      string `json:"document_amt,omitempty"`
	DocumentDate     string `json:"document_date,omitempty"`
	RecordedDatetime string `json:"recorded_datetime,omitempty"`
	CRFN             string `json:"crfn,omitempty"`
}

// debtView is the mortgage/debt-instrument history for a single BBL.
type debtView struct {
	BBL                   string         `json:"bbl"`
	Borough               string         `json:"borough"`
	BoroughName           string         `json:"borough_name,omitempty"`
	Block                 string         `json:"block"`
	Lot                   string         `json:"lot"`
	MortgageCount         int            `json:"mortgage_count"`
	TotalRecordedMortgage string         `json:"total_recorded_mortgage"`
	Documents             []debtDocument `json:"documents"`
	Note                  string         `json:"note,omitempty"`
}

func newNovelDebtCmd(flags *rootFlags) *cobra.Command {
	var flagBorough string
	var flagBlock string
	var flagLot string
	var flagMaxDocuments int

	cmd := &cobra.Command{
		Use:   "debt",
		Short: "List the mortgage and debt-instrument recordings for a BBL with amounts and dates.",
		Long: "List the recorded mortgages and other debt instruments for a borough/block/lot\n" +
			"(BBL). Joins Legals (document <-> BBL) to Master (amounts, dates) and keeps only\n" +
			"ACRIS mortgage/debt document types (MTGE, M&CON, ASST, SAT, AALR, SUBM, PREL, ...).\n" +
			"total_recorded_mortgage sums only originating mortgage recordings (MTGE, M&CON,\n" +
			"SMTG) — it indicates recorded principal, not a current payoff balance, and does\n" +
			"not net out satisfactions. Block and lot are not zero-padded.",
		Example:     "  acris-pp-cli debt --borough 1 --block 852 --lot 134 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would list mortgage and debt recordings for the BBL")
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
				"$limit":  strconv.Itoa(maxDocs * 8),
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}

			view := debtView{
				BBL:                   flagBorough + "-" + flagBlock + "-" + flagLot,
				Borough:               flagBorough,
				BoroughName:           boroughNames[flagBorough],
				Block:                 flagBlock,
				Lot:                   flagLot,
				TotalRecordedMortgage: "0",
				Documents:             []debtDocument{},
			}

			ids := distinctDocumentIDs(legals, 0)
			if len(ids) == 0 {
				view.Note = "no recorded documents found for this BBL; verify the borough code and that block/lot are not zero-padded"
				return emitDebtView(cmd, flags, view)
			}

			masters, err := fetchMastersByID(ctx, c, ids)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			var total int64
			for _, id := range ids {
				m, ok := masters[id]
				if !ok {
					continue
				}
				docType := strField(m, "doc_type")
				desc, isMortgage := mortgageClassCodes[docType]
				if !isMortgage {
					continue
				}
				amt := strField(m, "document_amt")
				if originatingMortgageCodes[docType] {
					if val, perr := strconv.ParseFloat(amt, 64); perr == nil {
						total += int64(val)
					}
				}
				view.Documents = append(view.Documents, debtDocument{
					DocumentID:       id,
					DocType:          docType,
					DocTypeDesc:      desc,
					DocumentAmt:      amt,
					DocumentDate:     strField(m, "document_date"),
					RecordedDatetime: strField(m, "recorded_datetime"),
					CRFN:             strField(m, "crfn"),
				})
				if len(view.Documents) >= maxDocs {
					break
				}
			}

			sort.SliceStable(view.Documents, func(i, j int) bool {
				return view.Documents[i].RecordedDatetime > view.Documents[j].RecordedDatetime
			})
			view.MortgageCount = len(view.Documents)
			view.TotalRecordedMortgage = strconv.FormatInt(total, 10)
			if view.MortgageCount == 0 {
				view.Note = "no mortgage or debt-instrument recordings found for this BBL"
			}

			return emitDebtView(cmd, flags, view)
		},
	}
	cmd.Flags().StringVar(&flagBorough, "borough", "", "Borough code: 1=Manhattan 2=Bronx 3=Brooklyn 4=Queens 5=Staten Island")
	cmd.Flags().StringVar(&flagBlock, "block", "", "Tax block number (not zero-padded)")
	cmd.Flags().StringVar(&flagLot, "lot", "", "Tax lot number (not zero-padded)")
	cmd.Flags().IntVar(&flagMaxDocuments, "max-documents", 100, "Maximum mortgage documents to return")
	return cmd
}

func emitDebtView(cmd *cobra.Command, flags *rootFlags, view debtView) error {
	if wantsHumanTable(cmd.OutOrStdout(), flags) {
		if view.MortgageCount == 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "BBL %s (%s): %s\n", view.BBL, view.BoroughName, view.Note)
			return nil
		}
		fmt.Fprintf(cmd.OutOrStdout(), "BBL %s (%s) — %d mortgage/debt recordings, total recorded $%s\n",
			view.BBL, view.BoroughName, view.MortgageCount, view.TotalRecordedMortgage)
		rows := make([]map[string]any, 0, len(view.Documents))
		for _, d := range view.Documents {
			rows = append(rows, map[string]any{
				"document_id":  d.DocumentID,
				"doc_type":     d.DocType,
				"document_amt": d.DocumentAmt,
				"recorded":     d.RecordedDatetime,
			})
		}
		return printAutoTable(cmd.OutOrStdout(), rows)
	}
	return printJSONFiltered(cmd.OutOrStdout(), view, flags)
}
