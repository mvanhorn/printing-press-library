// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/store"

	"github.com/spf13/cobra"
)

func newSpendCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "spend",
		Short: "Local-archive spend reports (group by service, account, lane, day)",
	}
	cmd.AddCommand(newSpendReportCmd(flags))
	return cmd
}

func newSpendReportCmd(flags *rootFlags) *cobra.Command {
	var (
		since   string
		groupBy string
		account string
	)
	cmd := &cobra.Command{
		Use:         "report",
		Short:       "Aggregate net charges from the local archive",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  fedex-pp-cli spend report
  fedex-pp-cli spend report --by lane --since 720h
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			dur, err := time.ParseDuration(since)
			if err != nil {
				dur = 30 * 24 * time.Hour
			}
			cutoff := time.Now().Add(-dur)

			st, err := store.Open("")
			if err != nil {
				return err
			}
			defer st.Close()

			groupExpr := "service_type"
			switch strings.ToLower(groupBy) {
			case "account":
				groupExpr = "account"
			case "lane":
				groupExpr = "(shipper_postal || '->' || recipient_postal)"
			case "day":
				groupExpr = "date(created_at)"
			case "service", "":
				groupExpr = "service_type"
			default:
				return usageErr(fmt.Errorf("invalid --by value %q (valid: service, account, lane, day)", groupBy))
			}

			query := fmt.Sprintf(`
				SELECT %s AS group_key,
				       count(*) AS shipment_count,
				       coalesce(sum(net_charge_amount), 0) AS total_net_charge,
				       coalesce(max(net_charge_currency), '') AS currency
				FROM shipments
				WHERE created_at >= ?
			`, groupExpr)
			qargs := []any{cutoff}
			if account != "" {
				query += " AND account = ?"
				qargs = append(qargs, account)
			}
			query += " GROUP BY group_key ORDER BY total_net_charge DESC"

			rows, err := st.DB().QueryContext(context.Background(), query, qargs...)
			if err != nil {
				return err
			}
			defer rows.Close()
			data, err := rowsToMaps(rows)
			if err != nil {
				return err
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				headers := []string{"GROUP", "SHIPMENTS", "TOTAL_NET", "CCY"}
				tableRows := make([][]string, 0, len(data))
				for _, r := range data {
					tableRows = append(tableRows, []string{
						getString(r, "group_key"),
						fmt.Sprintf("%d", int64(getFloat(r, "shipment_count"))),
						fmt.Sprintf("%.2f", getFloat(r, "total_net_charge")),
						getString(r, "currency"),
					})
				}
				return flags.printTable(cmd, headers, tableRows)
			}
			return printJSONFiltered(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&since, "since", "720h", "Window to aggregate over (default 30d)")
	cmd.Flags().StringVar(&groupBy, "by", "service", "Group by: service, account, lane, day")
	cmd.Flags().StringVar(&account, "account", "", "Restrict to one account")
	return cmd
}
