// Copyright 2026 Martin Kessler and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/payments/qbo/internal/store"

	"github.com/spf13/cobra"
)

type duplicateGroup struct {
	Type        string  `json:"type"`
	VendorName  string  `json:"vendor_name"`
	Amount      float64 `json:"amount"`
	Item1ID     string  `json:"item1_id"`
	Item1Date   string  `json:"item1_date"`
	Item1RefNum string  `json:"item1_ref_num"`
	Item2ID     string  `json:"item2_id"`
	Item2Date   string  `json:"item2_date"`
	Item2RefNum string  `json:"item2_ref_num"`
}

func newDuplicatesCmd(flags *rootFlags) *cobra.Command {
	var daysFlag int

	cmd := &cobra.Command{
		Use:     "duplicates",
		Short:   "Find duplicate purchases or bills to prevent double-billing",
		Example: "  qbo-pp-cli duplicates --days 3",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := store.Open()
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer s.Close()

			windowSeconds := daysFlag * 24 * 3600
			var duplicates []duplicateGroup

			// 1. Check Purchases duplicates
			purchaseQuery := `
				SELECT 
					json_extract(p1.raw_json, '$.EntityRef.name') AS vendor_name,
					CAST(json_extract(p1.raw_json, '$.TotalAmt') AS REAL) AS amount,
					p1.id, json_extract(p1.raw_json, '$.TxnDate'), p1.doc_number,
					p2.id, json_extract(p2.raw_json, '$.TxnDate'), p2.doc_number
				FROM purchases p1
				JOIN purchases p2 ON p1.id < p2.id
				  AND json_extract(p1.raw_json, '$.EntityRef.value') = json_extract(p2.raw_json, '$.EntityRef.value')
				  AND abs(CAST(json_extract(p1.raw_json, '$.TotalAmt') AS REAL) - CAST(json_extract(p2.raw_json, '$.TotalAmt') AS REAL)) < 0.01
				  AND abs(strftime('%s', json_extract(p1.raw_json, '$.TxnDate')) - strftime('%s', json_extract(p2.raw_json, '$.TxnDate'))) <= ?
			`

			rows, err := s.DB().Query(purchaseQuery, windowSeconds)
			if err != nil {
				return fmt.Errorf("querying purchase duplicates: %w", err)
			}
			for rows.Next() {
				var d duplicateGroup
				d.Type = "Purchase"
				var vendorName sql.NullString
				if err := rows.Scan(&vendorName, &d.Amount, &d.Item1ID, &d.Item1Date, &d.Item1RefNum, &d.Item2ID, &d.Item2Date, &d.Item2RefNum); err != nil {
					rows.Close()
					return fmt.Errorf("scanning purchase duplicate row: %w", err)
				}
				d.VendorName = vendorName.String
				duplicates = append(duplicates, d)
			}
			if err := rows.Err(); err != nil {
				rows.Close()
				return fmt.Errorf("iterating purchase duplicate rows: %w", err)
			}
			rows.Close()

			// 2. Check Bills duplicates
			billQuery := `
				SELECT 
					json_extract(b1.raw_json, '$.VendorRef.name') AS vendor_name,
					CAST(json_extract(b1.raw_json, '$.TotalAmt') AS REAL) AS amount,
					b1.id, json_extract(b1.raw_json, '$.TxnDate'), b1.doc_number,
					b2.id, json_extract(b2.raw_json, '$.TxnDate'), b2.doc_number
				FROM bills b1
				JOIN bills b2 ON b1.id < b2.id
				  AND json_extract(b1.raw_json, '$.VendorRef.value') = json_extract(b2.raw_json, '$.VendorRef.value')
				  AND abs(CAST(json_extract(b1.raw_json, '$.TotalAmt') AS REAL) - CAST(json_extract(b2.raw_json, '$.TotalAmt') AS REAL)) < 0.01
				  AND abs(strftime('%s', json_extract(b1.raw_json, '$.TxnDate')) - strftime('%s', json_extract(b2.raw_json, '$.TxnDate'))) <= ?
			`

			rows, err = s.DB().Query(billQuery, windowSeconds)
			if err != nil {
				return fmt.Errorf("querying bill duplicates: %w", err)
			}
			for rows.Next() {
				var d duplicateGroup
				d.Type = "Bill"
				var vendorName sql.NullString
				if err := rows.Scan(&vendorName, &d.Amount, &d.Item1ID, &d.Item1Date, &d.Item1RefNum, &d.Item2ID, &d.Item2Date, &d.Item2RefNum); err != nil {
					rows.Close()
					return fmt.Errorf("scanning bill duplicate row: %w", err)
				}
				d.VendorName = vendorName.String
				duplicates = append(duplicates, d)
			}
			if err := rows.Err(); err != nil {
				rows.Close()
				return fmt.Errorf("iterating bill duplicate rows: %w", err)
			}
			rows.Close()

			if flags.asJSON {
				return flags.printJSON(cmd, duplicates)
			}

			if len(duplicates) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No duplicate transactions found.")
				return nil
			}

			headers := []string{"TYPE", "VENDOR", "AMOUNT", "ITEM 1 (ID/DATE/REF)", "ITEM 2 (ID/DATE/REF)"}
			var tableRows [][]string
			for _, d := range duplicates {
				ref1 := d.Item1RefNum
				if ref1 == "" {
					ref1 = "-"
				} else {
					ref1 = truncate(ref1, 10)
				}
				ref2 := d.Item2RefNum
				if ref2 == "" {
					ref2 = "-"
				} else {
					ref2 = truncate(ref2, 10)
				}
				item1 := fmt.Sprintf("%s (%s / #%s)", d.Item1ID, d.Item1Date, ref1)
				item2 := fmt.Sprintf("%s (%s / #%s)", d.Item2ID, d.Item2Date, ref2)
				tableRows = append(tableRows, []string{
					d.Type,
					d.VendorName,
					fmt.Sprintf("$%.2f", d.Amount),
					item1,
					item2,
				})
			}

			return flags.printTable(cmd, headers, tableRows)
		},
	}

	cmd.Flags().IntVar(&daysFlag, "days", 3, "Max transaction date difference in days to flag as duplicate")
	return cmd
}
