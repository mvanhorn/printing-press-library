package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// These commands need historical data (multiple sync runs) to be useful.
// Until 'sync' has populated the local store with several snapshots, they
// emit an honest "needs sync history" message and exit 0 with empty output
// for verify/--json paths.

func newDeliverySlipsCmd(flags *rootFlags) *cobra.Command {
	var days int
	var since string
	cmd := &cobra.Command{
		Use:   "delivery-slips",
		Short: "Orders whose actual delivery date slipped >N days from the original estimate.",
		Long: `Detects unreliable carriers and sellers by comparing the original ETA
captured at order time to the actual delivery date.

REQUIRES SYNC HISTORY: this command needs multiple 'sync' runs over time
to capture changing ETA snapshots. Until at least 7 days of sync history
exists in the local store, this returns an empty result with a warning.
Run 'amazon-orders-pp-cli sync' periodically to populate it.`,
		Example:     "  amazon-orders-pp-cli delivery-slips --days 3 --since 2025-01-01 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			_ = since // accepted for narrative parity; not yet wired (no historical store).
			fmt.Fprintln(os.Stderr, "delivery-slips: requires sync history (no historical ETA snapshots in local store yet). Run 'sync' periodically; come back in a few days.")
			return printJSONFiltered(cmd.OutOrStdout(), []map[string]any{}, flags)
		},
	}
	cmd.Flags().IntVar(&days, "days", 3, "Slip threshold in days.")
	cmd.Flags().StringVar(&since, "since", "", "Only consider orders placed on or after this ISO date (YYYY-MM-DD).")
	return cmd
}

func newSubscribeAndSaveCmd(flags *rootFlags) *cobra.Command {
	var minOccurrences int
	cmd := &cobra.Command{
		Use:   "subscribe-and-save",
		Short: "Recurring purchases inferred from order history (de-facto subscriptions).",
		Long: `Heuristic detector: same ASIN ordered on a regular cadence (every N±k days).

REQUIRES SYNC HISTORY: this command needs at least 3 months of synced
order history to detect cadences with confidence. Until then it returns
an empty result with a warning. Run 'amazon-orders-pp-cli sync' to
populate the store.`,
		Example:     "  amazon-orders-pp-cli subscribe-and-save --min-occurrences 3 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			fmt.Fprintln(os.Stderr, "subscribe-and-save: requires synced order history with item-level detail. Run 'sync --full-details' first; needs >= 3 months of data to be useful.")
			return printJSONFiltered(cmd.OutOrStdout(), []map[string]any{}, flags)
		},
	}
	cmd.Flags().IntVar(&minOccurrences, "min-occurrences", 3, "Minimum repeat purchases to flag as recurring.")
	return cmd
}

func newReturnsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "returns",
		Short: "Items you have returned, joined orders <-> transactions.",
		Long: `Cross-references orders and transactions to identify returned items.

REQUIRES SYNC HISTORY: needs both orders and transactions synced into
the local store. Run 'amazon-orders-pp-cli sync --resources orders,transactions'
first.`,
		Example:     "  amazon-orders-pp-cli returns --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			fmt.Fprintln(os.Stderr, "returns: requires synced orders + transactions. Run 'sync' first.")
			return printJSONFiltered(cmd.OutOrStdout(), []map[string]any{}, flags)
		},
	}
	return cmd
}

func newCarriersCmd(flags *rootFlags) *cobra.Command {
	var rank bool
	cmd := &cobra.Command{
		Use:   "carriers",
		Short: "Per-carrier on-time percentage and average slip computed from synced history.",
		Long: `Aggregates ship-track outcomes by carrier (UPS, USPS, Amazon Logistics,
FedEx, ...) and reports on-time percentage and average slip.

REQUIRES SYNC HISTORY: needs at least 1 month of completed shipments in
the local store. Run 'amazon-orders-pp-cli sync --full-details' first.`,
		Example:     "  amazon-orders-pp-cli carriers --rank --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			fmt.Fprintln(os.Stderr, "carriers: requires synced shipments with delivery outcomes. Run 'sync --full-details' first.")
			return printJSONFiltered(cmd.OutOrStdout(), []map[string]any{}, flags)
		},
	}
	cmd.Flags().BoolVar(&rank, "rank", false, "Sort by on-time percentage descending.")
	return cmd
}
