// PATCH(novel-feature): events dedup — stream-shaped dedup transform that
// reads an event JSON array from stdin or the local store, applies a
// strategy, and writes a deduped stream to stdout. Hand-authored.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/ticketmaster/internal/store"

	"github.com/spf13/cobra"
)

func newEventsDedupCmd(flags *rootFlags) *cobra.Command {
	var strategy string
	var inputPath string
	var fromStore bool
	var window int

	cmd := &cobra.Command{
		Use:   "dedup",
		Short: "Read an event JSON array from stdin/store and emit a deduplicated stream",
		Long: strings.TrimSpace(`
A composable stream filter. Reads a JSON array of Discovery events from
stdin (or a file with --input, or the local store with --from-store) and
emits a deduplicated JSON array to stdout.

Strategies:
  id              dedup by event ID (the default; cheap and exact)
  name-venue-date dedup by (name, _embedded.venues[0].id, dates.start.localDate)
  tour-leg        dedup by (attractionId, city, year-month)
`),
		Example: strings.Trim(`
  ticketmaster-pp-cli events search --keyword phish --json | ticketmaster-pp-cli events dedup --strategy tour-leg
  ticketmaster-pp-cli events dedup --from-store --strategy name-venue-date --window 60
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			var events []json.RawMessage
			var err error
			switch {
			case fromStore:
				events, err = readStoreEventsForDedup(cmd.Context(), window)
			case inputPath != "" && inputPath != "-":
				f, ferr := os.Open(inputPath)
				if ferr != nil {
					return fmt.Errorf("open %s: %w", inputPath, ferr)
				}
				defer f.Close()
				events, err = decodeEvents(f)
			default:
				events, err = decodeEvents(cmd.InOrStdin())
			}
			if err != nil {
				return err
			}
			out := dedupByStrategy(events, strategy)
			// Always emit JSON unless explicitly disabled — dedup is a
			// composable transform, the human-friendly path is opt-in.
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			renderEventTable(cmd.OutOrStdout(), out)
			return nil
		},
	}
	cmd.Flags().StringVar(&strategy, "strategy", "id", "Dedup strategy: id | name-venue-date | tour-leg")
	cmd.Flags().StringVar(&inputPath, "input", "", "Read JSON from path (- for stdin; default stdin)")
	cmd.Flags().BoolVar(&fromStore, "from-store", false, "Read all synced events from local store instead of stdin")
	cmd.Flags().IntVar(&window, "window", 0, "When --from-store is set, restrict to events within N days (0 = no window)")
	return cmd
}

func decodeEvents(r io.Reader) ([]json.RawMessage, error) {
	// Read entire input; tolerate either a top-level array OR a Discovery
	// envelope { _embedded: { events: [...] } }.
	body, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read input: %w", err)
	}
	body = []byte(strings.TrimSpace(string(body)))
	if len(body) == 0 {
		return nil, nil
	}
	if body[0] == '[' {
		var raws []json.RawMessage
		if err := json.Unmarshal(body, &raws); err != nil {
			return nil, fmt.Errorf("parse JSON array: %w", err)
		}
		return raws, nil
	}
	if body[0] == '{' {
		var env struct {
			Embedded struct {
				Events []json.RawMessage `json:"events"`
			} `json:"_embedded"`
		}
		if err := json.Unmarshal(body, &env); err == nil && len(env.Embedded.Events) > 0 {
			return env.Embedded.Events, nil
		}
		// Single event object
		return []json.RawMessage{body}, nil
	}
	return nil, fmt.Errorf("dedup expects a JSON array or Discovery envelope")
}

func readStoreEventsForDedup(ctx context.Context, window int) ([]json.RawMessage, error) {
	db, err := store.OpenWithContext(ctx, defaultDBPath("ticketmaster-pp-cli"))
	if err != nil {
		return nil, err
	}
	defer db.Close()
	var rowsQ string
	var argv []any
	if window > 0 {
		rowsQ = `SELECT data FROM events WHERE date(json_extract(data, '$.dates.start.localDate')) BETWEEN date('now') AND date('now', ?)`
		argv = append(argv, fmt.Sprintf("+%d days", window))
	} else {
		rowsQ = `SELECT data FROM events`
	}
	rows, err := db.DB().QueryContext(ctx, rowsQ, argv...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []json.RawMessage
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, json.RawMessage(s))
	}
	return out, rows.Err()
}

func dedupByStrategy(events []json.RawMessage, strategy string) []json.RawMessage {
	seen := map[string]struct{}{}
	out := make([]json.RawMessage, 0, len(events))
	for _, e := range events {
		key := dedupKey(e, strategy)
		if key == "" {
			out = append(out, e)
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, e)
	}
	return out
}

func dedupKey(raw json.RawMessage, strategy string) string {
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return ""
	}
	switch strategy {
	case "name-venue-date":
		return strings.Join([]string{
			extractStr(obj, "name"),
			extractFirstEmbedded(obj, "venues", "id"),
			extractStr(obj, "dates.start.localDate"),
		}, "|")
	case "tour-leg":
		date := extractStr(obj, "dates.start.localDate")
		bucket := ""
		if len(date) >= 7 {
			bucket = date[:7]
		}
		return strings.Join([]string{
			extractFirstEmbedded(obj, "attractions", "id"),
			extractFirstEmbedded(obj, "venues", "city.name"),
			bucket,
		}, "|")
	default:
		if s, ok := obj["id"].(string); ok {
			return s
		}
		return ""
	}
}
