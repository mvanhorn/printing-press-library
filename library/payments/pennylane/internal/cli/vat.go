// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/pennylane/internal/store"
	"github.com/spf13/cobra"
)

// newVATCmd returns the "vat" parent command.
func newVATCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vat",
		Short: "TVA — aperçu, déclaration, simulation",
	}
	cmd.AddCommand(newVATPreviewCmd(flags))
	return cmd
}

// ─── vat preview ───────────────────────────────────────────────────────────

type vatLine struct {
	Rate       string  `json:"rate"`
	Collected  float64 `json:"tva_collectee"`
	Deductible float64 `json:"tva_deductible"`
	NetPayable float64 `json:"tva_a_decaisser"`
}

type vatPreviewResult struct {
	Period string    `json:"period"`
	Lines  []vatLine `json:"lignes"`
	Total  vatLine   `json:"total"`
}

func newVATPreviewCmd(flags *rootFlags) *cobra.Command {
	var period string
	var dbPath string

	cmd := &cobra.Command{
		Use:         "preview",
		Short:       "Aperçu TVA pour une période — collectée, déductible, net à décaisser",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  accounting-pp-cli vat preview --period 2026-Q1
  accounting-pp-cli vat preview --period 2026-01 --json
  accounting-pp-cli vat preview --period 2025`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("accounting-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("no local data — run 'sync' first")
			}
			defer db.Close()

			start, end, label, err := parsePeriod(period)
			if err != nil {
				return fmt.Errorf("invalid period %q: %w (use YYYY-QN, YYYY-MM or YYYY)", period, err)
			}

			type rateAgg struct {
				collected  float64
				deductible float64
			}
			byRate := make(map[string]*rateAgg)

			// Customer invoices → TVA collectée
			salesRows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT
					COALESCE(json_extract(data,'$.vat_rate'), '0') AS rate,
					COALESCE(CAST(json_extract(data,'$.tax') AS REAL),
					         0) AS tax_amount
				FROM resources
				WHERE json_extract(data,'$.date') BETWEEN ? AND ?
				  AND resource_type IN ('external-v2-customer-invoices','external-v2-changelogs-customer-invoices')
			`, start, end)
			if err != nil {
				return fmt.Errorf("querying sales: %w", err)
			}
			defer salesRows.Close()
			for salesRows.Next() {
				var rate string
				var tax float64
				if err := salesRows.Scan(&rate, &tax); err != nil {
					continue
				}
				if byRate[rate] == nil {
					byRate[rate] = &rateAgg{}
				}
				byRate[rate].collected += tax
			}

			// Supplier invoices → TVA déductible
			purchRows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT
					COALESCE(json_extract(data,'$.vat_rate'), '0') AS rate,
					COALESCE(CAST(json_extract(data,'$.tax') AS REAL),
					         0) AS tax_amount
				FROM resources
				WHERE json_extract(data,'$.date') BETWEEN ? AND ?
				  AND resource_type IN ('external-v2-supplier-invoices','external-v2-changelogs-supplier-invoices')
			`, start, end)
			if err != nil {
				return fmt.Errorf("querying purchases: %w", err)
			}
			defer purchRows.Close()
			for purchRows.Next() {
				var rate string
				var tax float64
				if err := purchRows.Scan(&rate, &tax); err != nil {
					continue
				}
				if byRate[rate] == nil {
					byRate[rate] = &rateAgg{}
				}
				byRate[rate].deductible += tax
			}

			var lines []vatLine
			var totCollected, totDeductible float64
			var rateKeys []string
			for k := range byRate {
				rateKeys = append(rateKeys, k)
			}
			sort.Strings(rateKeys)

			for _, rate := range rateKeys {
				agg := byRate[rate]
				c := math.Round(agg.collected*100) / 100
				d := math.Round(agg.deductible*100) / 100
				lines = append(lines, vatLine{
					Rate:       rate + "%",
					Collected:  c,
					Deductible: d,
					NetPayable: math.Round((c-d)*100) / 100,
				})
				totCollected += c
				totDeductible += d
			}

			res := vatPreviewResult{
				Period: label,
				Lines:  lines,
				Total: vatLine{
					Rate:       "TOTAL",
					Collected:  math.Round(totCollected*100) / 100,
					Deductible: math.Round(totDeductible*100) / 100,
					NetPayable: math.Round((totCollected-totDeductible)*100) / 100,
				},
			}

			if flags.asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(res)
			}

			fmt.Printf("Période : %s\n\n", label)
			tw := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "TAUX\tTVA COLLECTÉE\tTVA DÉDUCTIBLE\tTVA À DÉCAISSER")
			for _, l := range lines {
				fmt.Fprintf(tw, "%s\t%.2f\t%.2f\t%.2f\n",
					l.Rate, l.Collected, l.Deductible, l.NetPayable)
			}
			fmt.Fprintf(tw, "TOTAL\t%.2f\t%.2f\t%.2f\n",
				res.Total.Collected, res.Total.Deductible, res.Total.NetPayable)
			return tw.Flush()
		},
	}

	cmd.Flags().StringVar(&period, "period", currentQuarter(), "Period: YYYY-QN, YYYY-MM or YYYY")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// parsePeriod returns (start, end, label) for a period string.
func parsePeriod(p string) (string, string, string, error) {
	p = strings.TrimSpace(p)
	// YYYY-QN
	if len(p) == 7 && p[4] == '-' && p[5] == 'Q' {
		year, err := strconv.Atoi(p[:4])
		if err != nil {
			return "", "", "", err
		}
		q, err := strconv.Atoi(p[6:])
		if err != nil || q < 1 || q > 4 {
			return "", "", "", fmt.Errorf("quarter must be 1-4")
		}
		startMonth := time.Month((q-1)*3 + 1)
		endMonth := startMonth + 2
		start := time.Date(year, startMonth, 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(year, endMonth+1, 0, 23, 59, 59, 0, time.UTC)
		return start.Format("2006-01-02"), end.Format("2006-01-02"), p, nil
	}
	// YYYY-MM
	if len(p) == 7 && p[4] == '-' {
		t, err := time.Parse("2006-01", p)
		if err != nil {
			return "", "", "", err
		}
		start := t.Format("2006-01-02")
		end := t.AddDate(0, 1, -1).Format("2006-01-02")
		return start, end, p, nil
	}
	// YYYY
	if len(p) == 4 {
		year, err := strconv.Atoi(p)
		if err != nil {
			return "", "", "", err
		}
		return fmt.Sprintf("%d-01-01", year), fmt.Sprintf("%d-12-31", year), p, nil
	}
	return "", "", "", fmt.Errorf("unrecognized format")
}

func currentQuarter() string {
	now := time.Now()
	q := (int(now.Month())-1)/3 + 1
	return fmt.Sprintf("%d-Q%d", now.Year(), q)
}
