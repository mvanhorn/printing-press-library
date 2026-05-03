// Hand-written novel feature: customers journey.
// See every link a customer clicked, when they became a lead, and when they
// purchased — in one timeline.

package cli

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type journeyEvent struct {
	When   string `json:"when"`
	Type   string `json:"type"`
	LinkID string `json:"linkId,omitempty"`
	URL    string `json:"url,omitempty"`
	Detail string `json:"detail,omitempty"`
}

func newCustomersJourneyCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "journey [customer-id]",
		Short: "See every link a customer clicked, when they became a lead, and when they purchased",
		Long: `Combine local events × commissions × link metadata to assemble a single
chronological timeline for one customer.

The customer ID is whatever Dub returned for that customer (typically
` + "`cust_xxx`" + `). Use ` + "`dub-pp-cli customers list --json`" + ` to find IDs.`,
		Example: strings.Trim(`
  dub-pp-cli customers journey cust_abc123 --json
  dub-pp-cli customers journey cust_abc123 --json --select when,type,url
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			customerID := args[0]
			db, err := openLocalStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			var events []journeyEvent

			// 1) Click events from /events.
			rows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT COALESCE(synced_at,''), COALESCE(event,''), COALESCE(link_id,''), COALESCE(url,''), COALESCE(data,'{}')
				FROM events
				WHERE customer_id = ?
			`, customerID)
			if err == nil {
				for rows.Next() {
					var when, typ, linkID, url string
					var blob sql.NullString
					if err := rows.Scan(&when, &typ, &linkID, &url, &blob); err == nil {
						events = append(events, journeyEvent{
							When: when, Type: typ, LinkID: linkID, URL: url,
						})
					}
				}
				rows.Close()
			}

			// 2) Customer record itself.
			var custBlob sql.NullString
			if err := db.DB().QueryRowContext(cmd.Context(),
				`SELECT COALESCE(data,'{}') FROM customers WHERE id = ?`, customerID).Scan(&custBlob); err == nil && custBlob.Valid {
				blobBytes := []byte(custBlob.String)
				if created := extractFromData(blobBytes, "createdAt"); created != "" {
					events = append(events, journeyEvent{
						When: created, Type: "customer.created",
						Detail: extractFromData(blobBytes, "name"),
					})
				}
			}

			// 3) Commission rows tied to this customer (lead/sale conversion).
			rows2, err := db.DB().QueryContext(cmd.Context(), `
				SELECT COALESCE(synced_at,''), COALESCE(type,''), COALESCE(amount,0), COALESCE(currency,''), COALESCE(data,'{}')
				FROM commissions
				WHERE customer_id = ?
			`, customerID)
			if err == nil {
				for rows2.Next() {
					var when, typ, currency string
					var amount float64
					var blob sql.NullString
					if err := rows2.Scan(&when, &typ, &amount, &currency, &blob); err == nil {
						events = append(events, journeyEvent{
							When: when, Type: "commission." + typ,
							Detail: fmt.Sprintf("%.2f %s", amount, currency),
						})
					}
				}
				rows2.Close()
			}

			if len(events) == 0 {
				return fmt.Errorf("no journey data for customer %q (run `dub-pp-cli sync` and ensure the customer exists)", customerID)
			}
			sort.Slice(events, func(i, j int) bool { return events[i].When < events[j].When })

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), events, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-25s %-22s %s\n", "WHEN", "TYPE", "DETAIL")
			for _, e := range events {
				detail := e.URL
				if e.Detail != "" {
					detail = e.Detail
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-25s %-22s %s\n",
					truncate(e.When, 25), truncate(e.Type, 22), truncate(detail, 80))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
