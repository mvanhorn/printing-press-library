package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/delta-trip/internal/delta"
	"github.com/spf13/cobra"
)

// newSeatMapCmd is the top-level `seatmap` command.
func newSeatMapCmd(flags *rootFlags) *cobra.Command {
	var flagFlight int
	cmd := &cobra.Command{
		Use:   "seatmap <confirmation> <first-name> <last-name>",
		Short: "Show full seat availability map for a flight",
		Long: `Show every seat on a flight with availability status.

Opens a Chrome window to load the delta.com "View Seats" page for the specified
flight, then displays each seat as available, occupied, blocked, or your-seat.
Subsequent calls within the 4-hour trip cache are faster (flight metadata
is reused; only the seat map page is re-fetched).`,
		Example: strings.TrimRight(`
  delta-trip-pp-cli seatmap ABC123 JANE SMITH
  delta-trip-pp-cli seatmap ABC123 JANE SMITH --flight 2
  delta-trip-pp-cli seatmap ABC123 JANE SMITH --json
  delta-trip-pp-cli seatmap ABC123 JANE SMITH --json --select cabins,availableSeats`, "\n"),
		Args:        cobra.ExactArgs(3),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        seatMapRunE(flags, &flagFlight),
	}
	cmd.Flags().IntVar(&flagFlight, "flight", 1, "Flight within the itinerary to map (1-based, default 1)")
	return cmd
}

// newTripSeatMapCmd is the same feature wired as `trip seatmap`.
func newTripSeatMapCmd(flags *rootFlags) *cobra.Command {
	var flagFlight int
	cmd := &cobra.Command{
		Use:   "seatmap <confirmation> <first-name> <last-name>",
		Short: "Show full seat availability map for a flight",
		Example: strings.TrimRight(`
  delta-trip-pp-cli trip seatmap ABC123 JANE SMITH
  delta-trip-pp-cli trip seatmap ABC123 JANE SMITH --flight 2`, "\n"),
		Args:        cobra.ExactArgs(3),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        seatMapRunE(flags, &flagFlight),
	}
	cmd.Flags().IntVar(&flagFlight, "flight", 1, "Flight within the itinerary to map (1-based, default 1)")
	return cmd
}

func seatMapRunE(flags *rootFlags, flagFlight *int) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return nil
		}

		conf := strings.ToUpper(args[0])
		first := strings.ToUpper(args[1])
		last := strings.ToUpper(args[2])

		fmt.Fprintf(os.Stderr, "Fetching seat map for %s flight %d (opens a browser window)...\n", conf, *flagFlight)

		timeout := flags.timeout
		if timeout < 150*time.Second {
			timeout = 150 * time.Second
		}
		scrapeCtx, cancel := context.WithTimeout(cmd.Context(), timeout)
		defer cancel()

		seatMap, err := delta.GetSeatMap(scrapeCtx, conf, first, last, *flagFlight)
		if err != nil {
			return fmt.Errorf("fetching seat map: %w", err)
		}

		if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
			return printJSONFiltered(cmd.OutOrStdout(), seatMap, flags)
		}
		return printSeatMapTable(cmd.OutOrStdout(), seatMap)
	}
}

func printSeatMapTable(w io.Writer, sm *delta.SeatMapResult) error {
	// Header line.
	header := "Seat Map: " + sm.ConfirmationNumber
	if sm.FlightNumber != "" {
		header += " — Flight " + sm.FlightNumber
	}
	if sm.Route != "" {
		header += " (" + sm.Route + ")"
	}
	if sm.Aircraft != "" {
		header += " — " + sm.Aircraft
	}
	fmt.Fprintln(w, header)

	summary := fmt.Sprintf("Seats: %d total  ●%d available  ✗%d occupied",
		sm.TotalSeats, sm.AvailableSeats, sm.OccupiedSeats)
	if sm.BlockedSeats > 0 {
		summary += fmt.Sprintf("  ○%d blocked", sm.BlockedSeats)
	}
	fmt.Fprintln(w, summary)
	fmt.Fprintln(w)

	for _, cabin := range sm.Cabins {
		fmt.Fprintf(w, "── %s ──────────────────────────\n", cabin.Name)

		tw := tabwriter.NewWriter(w, 0, 0, 1, ' ', 0)
		for _, row := range cabin.Rows {
			exitLabel := ""
			if row.ExitRow {
				exitLabel = " [EXIT]"
			}
			fmt.Fprintf(tw, "  Row %2d%s\t", row.Number, exitLabel)
			for _, seat := range row.Seats {
				fmt.Fprintf(tw, " %s%-4s", seatStatusGlyph(seat.Status), seat.Number)
			}
			fmt.Fprintln(tw)
		}
		tw.Flush()
		fmt.Fprintln(w)
	}

	fmt.Fprintln(w, "Legend:  ● available   ✗ occupied   ○ blocked   ★ your seat")
	return nil
}

func seatStatusGlyph(status string) string {
	switch status {
	case "available":
		return "●"
	case "occupied":
		return "✗"
	case "blocked":
		return "○"
	case "your-seat":
		return "★"
	default:
		return "?"
	}
}
