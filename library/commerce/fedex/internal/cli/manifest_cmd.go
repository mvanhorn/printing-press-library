// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/store"

	"github.com/spf13/cobra"
)

func newManifestCmd(flags *rootFlags) *cobra.Command {
	var (
		date       string
		account    string
		closeIt    bool
		outputPath string
	)
	cmd := &cobra.Command{
		Use:   "manifest",
		Short: "Generate a daily shipment manifest report from the local archive (text/markdown; PDF deferred)",
		Long: `Builds a markdown manifest of every shipment created on the given day from the local
archive. The text/markdown report has the same content a PDF would; PDF generation
requires external tooling and is not bundled. With --close, also calls the FedEx
end-of-day endpoint to formally close the manifest.`,
		Example: strings.Trim(`
  fedex-pp-cli manifest
  fedex-pp-cli manifest --date 2026-04-30 --output today.md
  fedex-pp-cli manifest --close
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			day := time.Now().UTC()
			if date != "" && strings.ToLower(date) != "today" {
				parsed, err := time.Parse("2006-01-02", date)
				if err != nil {
					return usageErr(fmt.Errorf("invalid --date %q (expected YYYY-MM-DD or today)", date))
				}
				day = parsed
			}
			start := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC)
			end := start.Add(24 * time.Hour)

			st, err := store.Open("")
			if err != nil {
				return err
			}
			defer st.Close()

			query := `SELECT tracking_number, recipient_name, recipient_city, recipient_state,
				service_type, weight_value, weight_units,
				net_charge_amount, net_charge_currency, created_at
				FROM shipments
				WHERE created_at >= ? AND created_at < ?`
			qargs := []any{start, end}
			if account != "" {
				query += " AND account = ?"
				qargs = append(qargs, account)
			}
			query += " ORDER BY created_at"

			rows, err := st.DB().QueryContext(context.Background(), query, qargs...)
			if err != nil {
				return err
			}
			defer rows.Close()
			items, err := rowsToMaps(rows)
			if err != nil {
				return err
			}

			var buf bytes.Buffer
			fmt.Fprintf(&buf, "# FedEx Manifest — %s\n\n", start.Format("2006-01-02"))
			if account != "" {
				fmt.Fprintf(&buf, "Account: %s\n\n", account)
			}
			fmt.Fprintf(&buf, "Total shipments: %d\n\n", len(items))
			var totalNet float64
			ccy := ""
			for _, r := range items {
				totalNet += getFloat(r, "net_charge_amount")
				if c := getString(r, "net_charge_currency"); c != "" {
					ccy = c
				}
			}
			fmt.Fprintf(&buf, "Total net charges: %.2f %s\n\n", totalNet, ccy)
			fmt.Fprintln(&buf, "| Tracking | Recipient | City | State | Service | Weight | Net |")
			fmt.Fprintln(&buf, "|---|---|---|---|---|---|---|")
			for _, r := range items {
				fmt.Fprintf(&buf, "| %s | %s | %s | %s | %s | %.2f %s | %.2f %s |\n",
					getString(r, "tracking_number"),
					getString(r, "recipient_name"),
					getString(r, "recipient_city"),
					getString(r, "recipient_state"),
					getString(r, "service_type"),
					getFloat(r, "weight_value"), getString(r, "weight_units"),
					getFloat(r, "net_charge_amount"), getString(r, "net_charge_currency"),
				)
			}

			if closeIt {
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				closeBody := map[string]any{
					"accountNumber": map[string]any{"value": account},
					"shipDate":      start.Format("2006-01-02"),
				}
				_, _, cerr := c.Post("/ship/v1/endofday/", closeBody)
				if cerr != nil {
					fmt.Fprintf(&buf, "\n> end-of-day close failed: %v\n", cerr)
				} else {
					fmt.Fprintf(&buf, "\n> end-of-day close: ok\n")
				}
			}

			if outputPath != "" {
				if err := os.WriteFile(outputPath, buf.Bytes(), 0o644); err != nil {
					return err
				}
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"output":    outputPath,
					"shipments": len(items),
					"total_net": totalNet,
					"currency":  ccy,
					"date":      start.Format("2006-01-02"),
				}, flags)
			}
			fmt.Fprint(cmd.OutOrStdout(), buf.String())
			return nil
		},
	}
	cmd.Flags().StringVar(&date, "date", "today", "Day to summarize (YYYY-MM-DD or today)")
	cmd.Flags().StringVar(&account, "account", "", "Restrict to one account")
	cmd.Flags().BoolVar(&closeIt, "close", false, "Also call /ship/v1/endofday/ to close the manifest")
	cmd.Flags().StringVar(&outputPath, "output", "", "Write the manifest to this path (default stdout)")
	return cmd
}
