// Copyright 2026 not0xjarvis and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// documentParty is one party named on a document.
type documentParty struct {
	PartyType string `json:"party_type"`
	Name      string `json:"name"`
	Address1  string `json:"address_1,omitempty"`
	City      string `json:"city,omitempty"`
	State     string `json:"state,omitempty"`
	Zip       string `json:"zip,omitempty"`
}

// documentLegal is one property (BBL) tied to a document.
type documentLegal struct {
	Borough      string `json:"borough"`
	Block        string `json:"block"`
	Lot          string `json:"lot"`
	PropertyType string `json:"property_type,omitempty"`
	StreetNumber string `json:"street_number,omitempty"`
	StreetName   string `json:"street_name,omitempty"`
	Unit         string `json:"unit,omitempty"`
}

// documentView assembles one recorded document from Master + Legals + Parties.
type documentView struct {
	DocumentID       string          `json:"document_id"`
	DocType          string          `json:"doc_type,omitempty"`
	DocumentAmt      string          `json:"document_amt,omitempty"`
	DocumentDate     string          `json:"document_date,omitempty"`
	RecordedDatetime string          `json:"recorded_datetime,omitempty"`
	CRFN             string          `json:"crfn,omitempty"`
	RecordedBorough  string          `json:"recorded_borough,omitempty"`
	Legals           []documentLegal `json:"legals"`
	Parties          []documentParty `json:"parties"`
}

func newNovelDocumentCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "document <document_id>",
		Short: "Assemble one document's master record, all its property (BBL) legals, and all its parties into a single object.",
		Long: "Fetch a single recorded document by its ACRIS document ID and merge its Master\n" +
			"record (type, amount, dates), every property (BBL) it touches from Legals, and\n" +
			"every party from Parties into one object — the complete picture of one recording.",
		Example:     "  acris-pp-cli document 2023012300123001 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would assemble the full document view")
				return nil
			}
			if len(args) < 1 || args[0] == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a document ID positional argument is required"))
			}
			docID := args[0]

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			view := documentView{
				DocumentID: docID,
				Legals:     []documentLegal{},
				Parties:    []documentParty{},
			}

			masters, err := fetchACRISRows(ctx, c, acrisMasterPath, map[string]string{
				"document_id": docID,
				"$limit":      "1",
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			// A document is its Master record; if that does not exist the ID
			// resolves to nothing, so this is a not-found error (non-zero exit),
			// not an empty-but-valid result like bbl/debt.
			if len(masters) == 0 {
				return fmt.Errorf("document %q not found in ACRIS (no master record); verify the document ID", docID)
			}
			m := masters[0]
			view.DocType = strField(m, "doc_type")
			view.DocumentAmt = strField(m, "document_amt")
			view.DocumentDate = strField(m, "document_date")
			view.RecordedDatetime = strField(m, "recorded_datetime")
			view.CRFN = strField(m, "crfn")
			view.RecordedBorough = strField(m, "recorded_borough")

			legals, err := fetchACRISRows(ctx, c, acrisLegalsPath, map[string]string{
				"document_id": docID,
				"$limit":      "200",
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			for _, l := range legals {
				view.Legals = append(view.Legals, documentLegal{
					Borough:      strField(l, "borough"),
					Block:        strField(l, "block"),
					Lot:          strField(l, "lot"),
					PropertyType: strField(l, "property_type"),
					StreetNumber: strField(l, "street_number"),
					StreetName:   strField(l, "street_name"),
					Unit:         strField(l, "unit"),
				})
			}

			parties, err := fetchACRISRows(ctx, c, acrisPartiesPath, map[string]string{
				"document_id": docID,
				"$limit":      "200",
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			for _, p := range parties {
				view.Parties = append(view.Parties, documentParty{
					PartyType: strField(p, "party_type"),
					Name:      strField(p, "name"),
					Address1:  strField(p, "address_1"),
					City:      strField(p, "city"),
					State:     strField(p, "state"),
					Zip:       strField(p, "zip"),
				})
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "document %s — %s, amount %s, recorded %s\n",
					docID, view.DocType, view.DocumentAmt, view.RecordedDatetime)
				fmt.Fprintf(cmd.OutOrStdout(), "  %d propert(ies), %d part(ies)\n", len(view.Legals), len(view.Parties))
				for _, p := range view.Parties {
					fmt.Fprintf(cmd.OutOrStdout(), "  party %s: %s\n", p.PartyType, p.Name)
				}
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	return cmd
}
