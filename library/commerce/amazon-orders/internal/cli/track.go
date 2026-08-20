package cli

import (
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/commerce/amazon-orders/internal/parser"
	"github.com/spf13/cobra"
)

// newTrackCmd live-fetches and parses ship-track for one order's first
// shipment. Convenience wrapper over `shipments --order-id <id>`.
func newTrackCmd(flags *rootFlags) *cobra.Command {
	var shipmentID string
	var packageIndex int

	cmd := &cobra.Command{
		Use:   "track <order-id>",
		Short: "Live tracking for one order: carrier, tracking number, status, ETA.",
		Long: `Fetches /gp/your-account/ship-track for the given order ID, parses the
carrier name, tracking number, and current status, and emits structured
JSON. For multi-shipment orders use --shipment-id.`,
		Example:     "  amazon-orders-pp-cli track 111-1111111-1111111 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			orderID := args[0]
			if !isValidOrderID(orderID) {
				return fmt.Errorf("invalid order ID %q: expected canonical Amazon shape XXX-XXXXXXX-XXXXXXX (e.g. 111-1111111-1111111) or D01-XXXXXXX-XXXXXXX for digital orders", orderID)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			params := map[string]string{"orderId": orderID}
			if shipmentID != "" {
				params["shipmentId"] = shipmentID
			}
			if packageIndex > 0 {
				params["packageIndex"] = fmt.Sprintf("%d", packageIndex)
			}
			raw, err := authenticatedGet(c, "/gp/your-account/ship-track", params)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if err := parser.AuthInterstitialError(raw); err != nil {
				return classifyAPIError(err, flags)
			}
			track, perr := parser.ParseShipTrack(raw)
			if perr != nil {
				return fmt.Errorf("parsing ship-track HTML: %w", perr)
			}
			if track.OrderID == "" {
				track.OrderID = orderID
			}
			b, err := json.Marshal(track)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), b, flags)
		},
	}
	cmd.Flags().StringVar(&shipmentID, "shipment-id", "", "Amazon-internal shipment ID; required for multi-shipment orders.")
	cmd.Flags().IntVar(&packageIndex, "package-index", 0, "0-indexed package number within the shipment (default 0).")
	return cmd
}
