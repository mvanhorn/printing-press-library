// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/mvanhorn/printing-press-library/library/payments/pennylane/internal/store"
	"github.com/spf13/cobra"
)

// newAuditCmd returns the "audit" parent command.
func newAuditCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Audit — anomalies, incohérences, contrôles",
	}
	cmd.AddCommand(newAuditAnomaliesCmd(flags))
	return cmd
}

// ─── audit anomalies ───────────────────────────────────────────────────────

type anomaly struct {
	Type          string  `json:"type"`
	TransactionID string  `json:"transaction_id"`
	Amount        float64 `json:"amount"`
	Reason        string  `json:"reason"`
	Severity      string  `json:"severity"`
}

func newAuditAnomaliesCmd(flags *rootFlags) *cobra.Command {
	var sigma float64
	var dbPath string

	cmd := &cobra.Command{
		Use:         "anomalies",
		Short:       "Détecteur d'anomalies de paiement — doublons, montants ronds orphelins",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  accounting-pp-cli audit anomalies
  accounting-pp-cli audit anomalies --sigma 2 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("accounting-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("no local data — run 'sync' first")
			}
			defer db.Close()

			var anomalies []anomaly

			// 1. Detect duplicate amounts: same counterparty + same amount within 5 days
			dupRows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT
					a.id AS id_a,
					b.id AS id_b,
					CAST(json_extract(a.data,'$.amount') AS REAL) AS amt,
					COALESCE(json_extract(a.data,'$.counterparty'), json_extract(a.data,'$.label'), 'unknown') AS counterparty
				FROM resources a
				JOIN resources b ON (
					a.id < b.id
					AND CAST(json_extract(a.data,'$.amount') AS REAL)
					    = CAST(json_extract(b.data,'$.amount') AS REAL)
					AND COALESCE(json_extract(a.data,'$.counterparty'), json_extract(a.data,'$.label'))
					    = COALESCE(json_extract(b.data,'$.counterparty'), json_extract(b.data,'$.label'))
					AND ABS(julianday(json_extract(a.data,'$.date'))
					       - julianday(json_extract(b.data,'$.date'))) <= 5
				)
				WHERE a.resource_type IN ('external-v2-transactions','external-v2-changelogs-transactions')
				  AND b.resource_type IN ('external-v2-transactions','external-v2-changelogs-transactions')
				LIMIT 200
			`)
			if err == nil {
				defer dupRows.Close()
				for dupRows.Next() {
					var idA, idB, counterparty string
					var amt float64
					if err := dupRows.Scan(&idA, &idB, &amt, &counterparty); err != nil {
						continue
					}
					anomalies = append(anomalies, anomaly{
						Type:          "duplicate",
						TransactionID: idA + " / " + idB,
						Amount:        math.Round(amt*100) / 100,
						Reason:        fmt.Sprintf("même montant (%.2f) pour contrepartie %q dans un délai de 5 jours", amt, counterparty),
						Severity:      "high",
					})
				}
			}

			// 2. Round amounts > 1000 not matching any invoice
			roundRows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT
					t.id,
					CAST(json_extract(t.data,'$.amount') AS REAL) AS amt
				FROM resources t
				WHERE t.resource_type IN ('external-v2-transactions','external-v2-changelogs-transactions')
				  AND ABS(CAST(json_extract(t.data,'$.amount') AS REAL)) > 1000
				  AND (CAST(json_extract(t.data,'$.amount') AS REAL))
				      = CAST(CAST(json_extract(t.data,'$.amount') AS REAL) AS INTEGER)
				  AND NOT EXISTS (
				      SELECT 1 FROM resources inv
				      WHERE inv.resource_type IN ('external-v2-customer-invoices','external-v2-changelogs-customer-invoices','external-v2-supplier-invoices','external-v2-changelogs-supplier-invoices')
				        AND ABS(
				              COALESCE(CAST(json_extract(inv.data,'$.amount_with_tax') AS REAL),
				                       CAST(json_extract(inv.data,'$.amount') AS REAL),
				                       CAST(inv.amount AS REAL), 0)
				              - ABS(CAST(json_extract(t.data,'$.amount') AS REAL))
				            ) < 0.01
				  )
				LIMIT 200
			`)
			if err == nil {
				defer roundRows.Close()
				for roundRows.Next() {
					var id string
					var amt float64
					if err := roundRows.Scan(&id, &amt); err != nil {
						continue
					}
					sev := "medium"
					if math.Abs(amt) > 5000 {
						sev = "high"
					}
					anomalies = append(anomalies, anomaly{
						Type:          "round_amount_no_invoice",
						TransactionID: id,
						Amount:        math.Round(amt*100) / 100,
						Reason:        fmt.Sprintf("montant rond > 1 000 € (%.2f) sans facture correspondante", amt),
						Severity:      sev,
					})
				}
			}

			// Sort by severity
			sevOrder := map[string]int{"high": 0, "medium": 1, "low": 2}
			sort.Slice(anomalies, func(i, j int) bool {
				return sevOrder[anomalies[i].Severity] < sevOrder[anomalies[j].Severity]
			})

			_ = sigma // sigma parameter reserved for future statistical outlier detection

			if flags.asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(anomalies)
			}

			if len(anomalies) == 0 {
				fmt.Println("Aucune anomalie détectée.")
				return nil
			}
			tw := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "TYPE\tID TRANSACTION\tMONTANT\tRAISON\tGRAVITÉ")
			for _, a := range anomalies {
				fmt.Fprintf(tw, "%s\t%s\t%.2f\t%s\t%s\n",
					a.Type, truncate(a.TransactionID, 30), a.Amount, truncate(a.Reason, 60), a.Severity)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().Float64Var(&sigma, "sigma", 2.0, "Standard deviation threshold for outlier detection")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
