// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/mvanhorn/printing-press-library/library/payments/pennylane/internal/store"
	"github.com/spf13/cobra"
)

// newYearEndCmd returns the "yearend" parent command.
func newYearEndCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "yearend",
		Short: "Clôture annuelle — checklist et contrôles",
	}
	cmd.AddCommand(newYearEndCheckCmd(flags))
	return cmd
}

// ─── yearend check ─────────────────────────────────────────────────────────

type yearendCheck struct {
	CheckName   string `json:"check_name"`
	Status      string `json:"status"`
	Count       int    `json:"count"`
	Description string `json:"description"`
}

func newYearEndCheckCmd(flags *rootFlags) *cobra.Command {
	var fiscalYear int
	var dbPath string

	cmd := &cobra.Command{
		Use:         "check",
		Short:       "Checklist de clôture d'exercice — non-lettrés, brouillons, fournisseurs sans OD, avoirs ouverts",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  accounting-pp-cli yearend check --fiscal-year 2025
  accounting-pp-cli yearend check --fiscal-year 2025 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("accounting-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("no local data — run 'sync' first")
			}
			defer db.Close()

			start := fmt.Sprintf("%d-01-01", fiscalYear)
			end := fmt.Sprintf("%d-12-31", fiscalYear)

			var checks []yearendCheck

			// 1. Unreconciled transactions
			var unreconciledCount int
			_ = db.DB().QueryRowContext(cmd.Context(), `
				SELECT COUNT(*) FROM resources
				WHERE resource_type IN ('external-v2-transactions','external-v2-changelogs-transactions')
				  AND json_extract(data,'$.date') BETWEEN ? AND ?
				  AND (json_extract(data,'$.reconciled') IS NULL OR json_extract(data,'$.reconciled') = 0)
			`, start, end).Scan(&unreconciledCount)
			status := "ok"
			if unreconciledCount > 0 {
				status = "warn"
			}
			checks = append(checks, yearendCheck{
				CheckName:   "transactions_non_lettrees",
				Status:      status,
				Count:       unreconciledCount,
				Description: fmt.Sprintf("%d transaction(s) non lettrée(s) sur l'exercice %d", unreconciledCount, fiscalYear),
			})

			// 2. Draft invoices (should be 0 at year-end)
			var draftCount int
			_ = db.DB().QueryRowContext(cmd.Context(), `
				SELECT COUNT(*) FROM resources
				WHERE resource_type IN ('external-v2-customer-invoices','external-v2-changelogs-customer-invoices')
				  AND json_extract(data,'$.date') BETWEEN ? AND ?
				  AND json_extract(data,'$.draft') = 1
			`, start, end).Scan(&draftCount)
			draftStatus := "ok"
			if draftCount > 0 {
				draftStatus = "fail"
			}
			checks = append(checks, yearendCheck{
				CheckName:   "factures_brouillon",
				Status:      draftStatus,
				Count:       draftCount,
				Description: fmt.Sprintf("%d facture(s) en brouillon sur l'exercice %d (devrait être 0 à la clôture)", draftCount, fiscalYear),
			})

			// 3. Supplier invoices without accounting entries (simple proxy: no ledger entry journal reference)
			var supplierNoJournalCount int
			_ = db.DB().QueryRowContext(cmd.Context(), `
				SELECT COUNT(*) FROM resources
				WHERE resource_type IN ('external-v2-supplier-invoices','external-v2-changelogs-supplier-invoices')
				  AND json_extract(data,'$.date') BETWEEN ? AND ?
				  AND (json_extract(data,'$.accounting_status') IS NULL
				       OR json_extract(data,'$.accounting_status') NOT IN ('accounted','validated','posted'))
			`, start, end).Scan(&supplierNoJournalCount)
			supplierStatus := "ok"
			if supplierNoJournalCount > 0 {
				supplierStatus = "warn"
			}
			checks = append(checks, yearendCheck{
				CheckName:   "fournisseurs_sans_od",
				Status:      supplierStatus,
				Count:       supplierNoJournalCount,
				Description: fmt.Sprintf("%d facture(s) fournisseur sans statut comptabilisé sur %d", supplierNoJournalCount, fiscalYear),
			})

			// 4. Open credit notes
			var openCreditNotes int
			_ = db.DB().QueryRowContext(cmd.Context(), `
				SELECT COUNT(*) FROM resources
				WHERE json_extract(data,'$.type') IN ('credit_note','avoir')
				  AND json_extract(data,'$.date') BETWEEN ? AND ?
				  AND (json_extract(data,'$.status') NOT IN ('applied','linked','lettered')
				       OR json_extract(data,'$.status') IS NULL)
			`, start, end).Scan(&openCreditNotes)
			creditStatus := "ok"
			if openCreditNotes > 0 {
				creditStatus = "warn"
			}
			checks = append(checks, yearendCheck{
				CheckName:   "avoirs_ouverts",
				Status:      creditStatus,
				Count:       openCreditNotes,
				Description: fmt.Sprintf("%d avoir(s) non imputé(s) sur l'exercice %d", openCreditNotes, fiscalYear),
			})

			if flags.asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(checks)
			}

			fmt.Printf("Exercice : %d\n\n", fiscalYear)
			tw := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "CONTRÔLE\tSTATUT\tNOMBRE\tDESCRIPTION")
			for _, c := range checks {
				fmt.Fprintf(tw, "%s\t%s\t%d\t%s\n", c.CheckName, c.Status, c.Count, c.Description)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().IntVar(&fiscalYear, "fiscal-year", 0, "Fiscal year to check (e.g. 2025)")
	_ = cmd.MarkFlagRequired("fiscal-year")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
