// Copyright 2026 Martin Kessler and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/payments/qbo/internal/store"
	"math"

	"github.com/spf13/cobra"
)

type netWorthAccount struct {
	Name    string  `json:"name"`
	Type    string  `json:"type"`
	Balance float64 `json:"balance"`
}

type netWorthReport struct {
	TotalAssets      float64           `json:"total_assets"`
	TotalLiabilities float64           `json:"total_liabilities"`
	NetWorth         float64           `json:"net_worth"`
	Assets           []netWorthAccount `json:"asset_accounts"`
	Liabilities      []netWorthAccount `json:"liability_accounts"`
}

func newNetWorthCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "net-worth",
		Short:   "Calculate net worth from asset and liability accounts",
		Example: "  qbo-pp-cli net-worth",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := store.Open()
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer s.Close()

			query := `
				SELECT 
					name, 
					json_extract(raw_json, '$.AccountType') AS account_type,
					CAST(json_extract(raw_json, '$.CurrentBalance') AS REAL) AS balance
				FROM accounts
			`
			rows, err := s.DB().Query(query)
			if err != nil {
				return fmt.Errorf("querying accounts: %w", err)
			}
			defer rows.Close()

			var report netWorthReport

			for rows.Next() {
				var name string
				var accType sql.NullString
				var balance sql.NullFloat64
				if err := rows.Scan(&name, &accType, &balance); err != nil {
					return fmt.Errorf("scanning account row: %w", err)
				}

				acc := netWorthAccount{
					Name:    name,
					Type:    accType.String,
					Balance: math.Round(balance.Float64*100) / 100,
				}

				if isAssetAccount(accType.String) {
					report.Assets = append(report.Assets, acc)
					report.TotalAssets += acc.Balance
				} else if isLiabilityAccount(accType.String) {
					report.Liabilities = append(report.Liabilities, acc)
					report.TotalLiabilities += acc.Balance
				}
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("reading accounts: %w", err)
			}

			report.TotalAssets = math.Round(report.TotalAssets*100) / 100
			report.TotalLiabilities = math.Round(report.TotalLiabilities*100) / 100
			report.NetWorth = math.Round((report.TotalAssets-report.TotalLiabilities)*100) / 100

			if flags.asJSON {
				return flags.printJSON(cmd, report)
			}

			w := cmd.OutOrStdout()

			fmt.Fprintln(w, "========================================")
			fmt.Fprintln(w, "       NET WORTH LEDGER REPORT          ")
			fmt.Fprintln(w, "========================================")
			fmt.Fprintln(w, "")

			fmt.Fprintln(w, "ASSETS:")
			for _, a := range report.Assets {
				fmt.Fprintf(w, "  %-30s %10.2f (%s)\n", truncate(a.Name, 30), a.Balance, a.Type)
			}
			fmt.Fprintf(w, "  --------------------------------------\n")
			fmt.Fprintf(w, "  TOTAL ASSETS:                  $%.2f\n\n", report.TotalAssets)

			fmt.Fprintln(w, "LIABILITIES:")
			for _, l := range report.Liabilities {
				fmt.Fprintf(w, "  %-30s %10.2f (%s)\n", truncate(l.Name, 30), l.Balance, l.Type)
			}
			fmt.Fprintf(w, "  --------------------------------------\n")
			fmt.Fprintf(w, "  TOTAL LIABILITIES:             $%.2f\n\n", report.TotalLiabilities)

			fmt.Fprintln(w, "========================================")
			fmt.Fprintf(w, "  NET WORTH:                     $%.2f\n", report.NetWorth)
			fmt.Fprintln(w, "========================================")

			return nil
		},
	}

	return cmd
}

func isAssetAccount(accType string) bool {
	switch accType {
	case "Bank", "Other Current Asset", "Fixed Asset", "Other Asset", "Accounts Receivable":
		return true
	}
	return false
}

func isLiabilityAccount(accType string) bool {
	switch accType {
	case "Credit Card", "Other Current Liability", "Long Term Liability", "Accounts Payable":
		return true
	}
	return false
}
