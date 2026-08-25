// Copyright 2026 matthew.martin and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelCustomerTimelineCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "timeline <customer-id>",
		Short:       "Follow one customer's orders, payments, refunds, loyalty activity, invoices, and bookings in chronological order.",
		Example:     "  square-pp-cli customer timeline CUSTOMER_ABC123 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		Args:        cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return err
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "customer timeline")
			}
			resources := []string{"orders", "payments", "refunds", "loyalty", "invoices", "bookings"}
			db, err := openNovelLocalStore(cmd, flags, resources)
			if err != nil {
				return err
			}
			defer db.Close()
			records, err := loadLocalSquareRecords(cmd.Context(), db, resources)
			if err != nil {
				return err
			}
			matched := make([]localSquareRecord, 0)
			for _, record := range records {
				if referencesCustomer(record.Data, args[0]) {
					matched = append(matched, record)
				}
			}
			sortRecordsChronologically(matched)
			type event struct {
				At       interface{} `json:"at"`
				Resource string      `json:"resource"`
				ID       string      `json:"id"`
				Status   string      `json:"status,omitempty"`
				Amount   int64       `json:"amount,omitempty"`
				Currency string      `json:"currency,omitempty"`
			}
			events := make([]event, 0, len(matched))
			for _, record := range matched {
				events = append(events, event{At: recordTime(record), Resource: record.ResourceType, ID: record.ID, Status: stringValue(record.Data, "status"), Amount: intValue(record.Data, "amount_money", "amount"), Currency: stringValue(record.Data, "amount_money", "currency")})
			}
			return flags.printJSON(cmd, map[string]any{"data_source": "local", "customer_id": args[0], "event_count": len(events), "events": events})
		},
	}
	return cmd
}
