// PATCH(novel-feature): events upcoming — fan out across a venue ID list (or
// saved watchlist) and return one merged deduplicated date-sorted event list
// from the local synced store. Hand-authored.

package cli

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/ticketmaster/internal/store"

	"github.com/spf13/cobra"
)

func newEventsUpcomingCmd(flags *rootFlags) *cobra.Command {
	var days int
	var venueIDsFlag string
	var venuesFile string
	var watchlistName string

	cmd := &cobra.Command{
		Use:   "upcoming",
		Short: "Fan out across a venue ID file or list and return one merged deduplicated event list",
		Long: strings.TrimSpace(`
Multi-venue watchlist sweep — the generic primitive behind any curated
"what's on at my venues" workflow. Reads venue IDs from:
  --venue-ids comma,separated,list
  --venues path/to/file.txt   (one ID per line; - for stdin)
  --watchlist name             (load IDs from a saved watchlist)

Queries the local events table for events at any of those venues whose start
date falls within --days from today, dedupes on event ID, sorts ascending.

Run 'sync --resource events' first to populate the local store.
`),
		Example: strings.Trim(`
  ticketmaster-pp-cli events upcoming --venue-ids KovZ917Ahkk,KovZpZAFkvEA --days 60 --json
  ticketmaster-pp-cli events upcoming --venues seattle-venues.txt --days 60
  ticketmaster-pp-cli events upcoming --watchlist seattle --days 30
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			dbPath := defaultDBPath("ticketmaster-pp-cli")
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer db.Close()

			venueIDs, err := collectVenueIDs(cmd.Context(), db.DB(), venueIDsFlag, venuesFile, watchlistName, cmd.InOrStdin())
			if err != nil {
				return err
			}
			if len(venueIDs) == 0 {
				return usageErr(fmt.Errorf("no venue IDs provided; use --venue-ids, --venues, or --watchlist"))
			}

			wl := &watchlist{VenueIDs: venueIDs}
			events, err := queryFilteredEvents(cmd.Context(), db.DB(), wl, days)
			if err != nil {
				return err
			}
			events = dedupEvents(events, "id")

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), events, flags)
			}
			renderEventTable(cmd.OutOrStdout(), events)
			return nil
		},
	}
	cmd.Flags().IntVar(&days, "days", 60, "Window in days from today")
	cmd.Flags().StringVar(&venueIDsFlag, "venue-ids", "", "Comma-separated Ticketmaster venue IDs")
	cmd.Flags().StringVar(&venuesFile, "venues", "", "File containing venue IDs (one per line; - for stdin)")
	cmd.Flags().StringVar(&watchlistName, "watchlist", "", "Load venue IDs from a saved watchlist")
	return cmd
}

func collectVenueIDs(ctx context.Context, db *sql.DB, csvFlag, filePath, watchlistName string, stdin io.Reader) ([]string, error) {
	var out []string
	if csvFlag != "" {
		out = append(out, splitCSV(csvFlag)...)
	}
	if filePath != "" {
		ids, err := readIDsFromFile(filePath, stdin)
		if err != nil {
			return nil, err
		}
		out = append(out, ids...)
	}
	if watchlistName != "" {
		wl, err := loadWatchlist(ctx, db, watchlistName)
		if err != nil {
			return nil, err
		}
		out = append(out, wl.VenueIDs...)
	}
	return uniqStrings(out), nil
}

func readIDsFromFile(path string, stdin io.Reader) ([]string, error) {
	var r io.Reader
	if path == "-" {
		r = stdin
	} else {
		f, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", path, err)
		}
		defer f.Close()
		r = f
	}
	var out []string
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// support "id,name" CSV rows too — take first field
		if idx := strings.IndexAny(line, ",\t"); idx > 0 {
			line = strings.TrimSpace(line[:idx])
		}
		if line != "" {
			out = append(out, line)
		}
	}
	return out, sc.Err()
}

func uniqStrings(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

// dedupEvents removes events with duplicate IDs by JSON path.
func dedupEvents(events []json.RawMessage, idField string) []json.RawMessage {
	seen := map[string]struct{}{}
	out := make([]json.RawMessage, 0, len(events))
	for _, e := range events {
		var obj map[string]any
		if err := json.Unmarshal(e, &obj); err != nil {
			out = append(out, e)
			continue
		}
		id, _ := obj[idField].(string)
		if id == "" {
			out = append(out, e)
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, e)
	}
	return out
}

// renderEventTable writes a compact human-readable table of events.
func renderEventTable(w io.Writer, events []json.RawMessage) {
	if len(events) == 0 {
		fmt.Fprintln(w, "No events. Run 'sync --resource events' to populate the local store.")
		return
	}
	tw := newTabWriter(w)
	fmt.Fprintln(tw, "DATE\tEVENT\tVENUE\tCITY\tSEGMENT\tSTATUS")
	for _, e := range events {
		var obj map[string]any
		if err := json.Unmarshal(e, &obj); err != nil {
			continue
		}
		date := extractStr(obj, "dates.start.localDate")
		if date == "" {
			date = extractStr(obj, "dates.start.dateTime")
			if len(date) >= 10 {
				date = date[:10]
			}
		}
		name := extractStr(obj, "name")
		venueName := extractFirstEmbedded(obj, "venues", "name")
		city := extractFirstEmbedded(obj, "venues", "city.name")
		segment := extractFirstClassification(obj, "segment")
		status := extractStr(obj, "dates.status.code")
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
			truncate(date, 10), truncate(name, 36), truncate(venueName, 24), truncate(city, 14),
			truncate(segment, 12), status)
	}
	_ = tw.Flush()
}

// extractStr drills into a map with a dotted path, returning a string or empty.
func extractStr(obj map[string]any, path string) string {
	parts := strings.Split(path, ".")
	var cur any = obj
	for _, p := range parts {
		m, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		cur = m[p]
	}
	if s, ok := cur.(string); ok {
		return s
	}
	return ""
}

func extractFirstEmbedded(obj map[string]any, arrayName, path string) string {
	emb, ok := obj["_embedded"].(map[string]any)
	if !ok {
		return ""
	}
	arr, ok := emb[arrayName].([]any)
	if !ok || len(arr) == 0 {
		return ""
	}
	first, ok := arr[0].(map[string]any)
	if !ok {
		return ""
	}
	return extractStr(first, path)
}

func extractFirstClassification(obj map[string]any, level string) string {
	arr, ok := obj["classifications"].([]any)
	if !ok || len(arr) == 0 {
		return ""
	}
	first, ok := arr[0].(map[string]any)
	if !ok {
		return ""
	}
	inner, ok := first[level].(map[string]any)
	if !ok {
		return ""
	}
	if s, ok := inner["name"].(string); ok {
		return s
	}
	return ""
}
