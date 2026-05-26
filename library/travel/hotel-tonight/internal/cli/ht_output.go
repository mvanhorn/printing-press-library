// Shared output + fetch-and-record helpers for the HotelTonight novel
// commands. Hand-authored (survives `generate --force`).
package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/hotel-tonight/internal/client"
)

// emitJSONOrTable renders v as JSON for agents and pipes (the agent-native
// default), or a table for an interactive terminal. Mirrors the generated
// commands' output gate so --json/--select/--compact behave consistently.
func emitJSONOrTable(cmd *cobra.Command, flags *rootFlags, v any, headers []string, rows [][]string) error {
	if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
		return flags.printJSON(cmd, v)
	}
	return flags.printTable(cmd, headers, rows)
}

// fetchAndRecord pulls live inventory for a coordinate and appends a price
// snapshot to the local store. Snapshot-recording failures are non-fatal — a
// read should still succeed if the store is unavailable — so they surface as a
// stderr note, not an error return.
func fetchAndRecord(ctx context.Context, cmd *cobra.Command, c *client.Client, lat, lng, checkIn, checkOut string, rooms int) (*htInventory, error) {
	inv, err := fetchInventory(c, lat, lng, checkIn, checkOut, rooms)
	if err != nil {
		return nil, err
	}
	s, err := openPriceStore(ctx)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "note: price snapshot not recorded (%v)\n", err)
		return inv, nil
	}
	defer s.Close()
	if _, err := recordSnapshots(ctx, s.DB(), inv, timeNow()); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "note: price snapshot not recorded (%v)\n", err)
	}
	return inv, nil
}

// money formats a price as a dollar figure, or a dash when zero/absent.
func money(v float64) string {
	if v <= 0 {
		return "—"
	}
	return fmt.Sprintf("$%.0f", v)
}

// itoaPct renders a percent-off value as "N%", or a dash when zero.
func itoaPct(pct int) string {
	if pct <= 0 {
		return "—"
	}
	return fmt.Sprintf("%d%%", pct)
}
