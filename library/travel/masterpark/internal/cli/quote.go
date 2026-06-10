package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/masterpark/internal/client"
)

func newQuoteCmd(g *globalOpts) *cobra.Command {
	var (
		lot         string
		dropoff     string
		pickup      string
		vehicleType string
		promoCode   string
	)
	cmd := &cobra.Command{
		Use:   "quote",
		Short: "Get parking price quotes for a date range",
		Example: `  masterpark-pp-cli quote --lot B --dropoff "2030-06-11 07:00" --pickup "2030-06-13 18:30"
  masterpark-pp-cli quote --lot G --dropoff "2030-06-11 07:00" --pickup "2030-06-13 18:30" --vehicle-type oversize --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if vehicleType != "standard" && vehicleType != "oversize" {
				return fmt.Errorf("--vehicle-type must be standard or oversize")
			}
			if dropoff == "" || pickup == "" {
				return fmt.Errorf("--dropoff and --pickup are required (format \"YYYY-MM-DD HH:MM\")")
			}
			codeID, err := client.ResolveLot(lot)
			if err != nil {
				return err
			}
			req := client.QuoteRequest{
				Location:       codeID,
				MultiLocations: nil,
				Reservation: client.Reservation{
					StartDate: dropoff,
					EndDate:   pickup,
					PromoCode: promoCode,
					Source:    "website",
					SourceID:  "",
					Quote:     -2,
					Services:  []interface{}{},
					Vehicle:   client.Vehicle{Type: vehicleType},
				},
				ResRate: false,
			}

			ctx, cancel := g.ctx()
			defer cancel()
			data, err := g.newClient().GetQuotes(ctx, req)
			if err != nil {
				return err
			}
			if g.json {
				return printRawJSON(data)
			}
			return renderQuotes(data)
		},
	}
	cmd.Flags().StringVar(&lot, "lot", "", "lot to quote: B, G, or a codeID (e.g. 2515-1-889)")
	cmd.Flags().StringVar(&dropoff, "dropoff", "", "drop-off date/time \"YYYY-MM-DD HH:MM\"")
	cmd.Flags().StringVar(&pickup, "pickup", "", "pick-up date/time \"YYYY-MM-DD HH:MM\"")
	cmd.Flags().StringVar(&vehicleType, "vehicle-type", "standard", "vehicle type: standard or oversize")
	cmd.Flags().StringVar(&promoCode, "promo-code", "", "optional promo code")
	_ = cmd.MarkFlagRequired("lot")
	return cmd
}

func renderQuotes(data json.RawMessage) error {
	var quotes []map[string]interface{}
	if err := json.Unmarshal(data, &quotes); err != nil || len(quotes) == 0 {
		// Unknown shape; fall back to pretty JSON.
		return printRawJSON(data)
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "INDEX\tNAME\tSUBTOTAL\tTOTAL\tDUE_AT_LOT")
	for _, q := range quotes {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			str(q, "index", "quote", "id", "quoteId"),
			str(q, "name", "rateName", "title"),
			str(q, "parking_subtotal", "subtotal", "rate", "dailyRate"),
			str(q, "grand_total", "balance_due", "total", "totalPrice", "amount"),
			str(q, "due_at_lot"),
		)
	}
	return w.Flush()
}

func formatNum(f float64) string {
	if f == float64(int64(f)) {
		return fmt.Sprintf("%d", int64(f))
	}
	return fmt.Sprintf("%.2f", f)
}

func str(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			switch t := v.(type) {
			case string:
				if t != "" {
					return t
				}
			case float64:
				return formatNum(t)
			default:
				return fmt.Sprintf("%v", t)
			}
		}
	}
	return ""
}
