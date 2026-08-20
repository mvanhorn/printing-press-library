// PATCH(novel-feature): events brief — markdown "what's on" report grouped by
// date and venue, suitable for newsletter / Obsidian / iMessage / agent
// context. Hand-authored.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/ticketmaster/internal/store"

	"github.com/spf13/cobra"
)

func newEventsBriefCmd(flags *rootFlags) *cobra.Command {
	var window int
	var dmaID string
	var classification string
	var watchlistName string
	var title string

	cmd := &cobra.Command{
		Use:   "brief",
		Short: "Render a markdown 'what's on' report grouped by night → venue → events",
		Long: strings.TrimSpace(`
Renders a paste-ready markdown brief of upcoming events grouped by date
and venue, with classification labels and (when available) price ranges.
Designed for Obsidian, iMessage threads, newsletter drafts, and agent
context windows.

Run 'sync --resource events' first to populate the local store.
`),
		Example: strings.Trim(`
  ticketmaster-pp-cli events brief --window 7 --dma 383
  ticketmaster-pp-cli events brief --watchlist seattle --window 14
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			dbPath := defaultDBPath("ticketmaster-pp-cli")
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			wl := &watchlist{}
			if watchlistName != "" {
				loaded, err := loadWatchlist(cmd.Context(), db.DB(), watchlistName)
				if err != nil {
					return err
				}
				wl = loaded
			}
			if dmaID != "" {
				wl.DMAIDs = append(wl.DMAIDs, dmaID)
			}
			// PATCH(greptile P1 events_brief.go:65/70 — --classification needs
			// OR semantics, not AND): pushing the value into both
			// wl.Segments AND wl.Genres produced AND semantics inside
			// queryFilteredEvents (`segment IN (?) AND genre IN (?)`), which
			// excluded events that match only one taxonomy level — a "Rock"
			// concert with segment="Music"/genre="Rock" failed the segment
			// arm and was silently filtered out. Apply the classification
			// filter inline as a post-filter on the result set so segment
			// OR genre match is true OR semantics; matches the inline SQL
			// pattern of `events on-sale-soon --classification`.
			events, err := queryFilteredEvents(cmd.Context(), db.DB(), wl, window)
			if err != nil {
				return err
			}
			if classification != "" {
				events = filterEventsByClassification(events, classification)
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), events, flags)
			}
			if title == "" {
				title = fmt.Sprintf("What's On — Next %d Days", window)
			}
			renderMarkdownBrief(cmd.OutOrStdout(), title, events)
			return nil
		},
	}
	cmd.Flags().IntVar(&window, "window", 7, "Window in days from today")
	cmd.Flags().StringVar(&dmaID, "dma", "", "Restrict to a single DMA ID")
	cmd.Flags().StringVar(&classification, "classification", "", "Filter by segment OR genre name (Music, Arts & Theatre, Sports, Rock, Jazz, Comedy, etc.)")
	cmd.Flags().StringVar(&watchlistName, "watchlist", "", "Apply a saved watchlist's filters")
	cmd.Flags().StringVar(&title, "title", "", "Heading for the brief (default: 'What's On — Next N Days')")
	return cmd
}

type briefRow struct {
	Date    string
	Venue   string
	Name    string
	Segment string
	Genre   string
	Price   string
	Time    string
}

func renderMarkdownBrief(w io.Writer, title string, events []json.RawMessage) {
	if len(events) == 0 {
		fmt.Fprintf(w, "# %s\n\n_No events in window. Run `sync --resource events` to populate the local store._\n", title)
		return
	}
	rows := make([]briefRow, 0, len(events))
	for _, e := range events {
		var obj map[string]any
		if err := json.Unmarshal(e, &obj); err != nil {
			continue
		}
		r := briefRow{
			Date:    extractStr(obj, "dates.start.localDate"),
			Venue:   extractFirstEmbedded(obj, "venues", "name"),
			Name:    extractStr(obj, "name"),
			Segment: extractFirstClassification(obj, "segment"),
			Genre:   extractFirstClassification(obj, "genre"),
			Time:    extractStr(obj, "dates.start.localTime"),
			Price:   extractPriceRange(obj),
		}
		if r.Date == "" {
			r.Date = "TBD"
		}
		rows = append(rows, r)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Date != rows[j].Date {
			return rows[i].Date < rows[j].Date
		}
		if rows[i].Venue != rows[j].Venue {
			return rows[i].Venue < rows[j].Venue
		}
		return rows[i].Time < rows[j].Time
	})

	fmt.Fprintf(w, "# %s\n\n", title)
	currentDate := ""
	currentVenue := ""
	for _, r := range rows {
		if r.Date != currentDate {
			fmt.Fprintf(w, "\n## %s\n\n", r.Date)
			currentDate = r.Date
			currentVenue = ""
		}
		if r.Venue != currentVenue {
			fmt.Fprintf(w, "**%s**\n", r.Venue)
			currentVenue = r.Venue
		}
		tagBits := []string{}
		if r.Genre != "" {
			tagBits = append(tagBits, r.Genre)
		} else if r.Segment != "" {
			tagBits = append(tagBits, r.Segment)
		}
		if r.Price != "" {
			tagBits = append(tagBits, r.Price)
		}
		tag := ""
		if len(tagBits) > 0 {
			tag = " _(" + strings.Join(tagBits, " · ") + ")_"
		}
		timeBit := ""
		if r.Time != "" && len(r.Time) >= 5 {
			timeBit = r.Time[:5] + " — "
		}
		fmt.Fprintf(w, "- %s%s%s\n", timeBit, r.Name, tag)
	}
	fmt.Fprintln(w)
}

// filterEventsByClassification keeps events whose first classification's
// segment name OR genre name equals the supplied label (case-sensitive,
// matching the live Ticketmaster taxonomy values like "Music", "Rock",
// "Arts & Theatre", "Comedy"). Used as a post-filter so `events brief
// --classification` gets OR semantics across segment+genre without
// rewriting queryFilteredEvents' shared AND-of-EXISTS shape used by
// `events upcoming` and `events watchlist run`.
func filterEventsByClassification(events []json.RawMessage, label string) []json.RawMessage {
	out := make([]json.RawMessage, 0, len(events))
	for _, e := range events {
		var obj map[string]any
		if err := json.Unmarshal(e, &obj); err != nil {
			continue
		}
		seg := extractFirstClassification(obj, "segment")
		gen := extractFirstClassification(obj, "genre")
		if seg == label || gen == label {
			out = append(out, e)
		}
	}
	return out
}

// extractPriceRange formats $.priceRanges[0].min/max as "$min–$max" when both
// are present; falls back to "$min+" or "" when partial/missing.
func extractPriceRange(obj map[string]any) string {
	arr, ok := obj["priceRanges"].([]any)
	if !ok || len(arr) == 0 {
		return ""
	}
	first, ok := arr[0].(map[string]any)
	if !ok {
		return ""
	}
	min, _ := first["min"].(float64)
	max, _ := first["max"].(float64)
	currency, _ := first["currency"].(string)
	symbol := "$"
	if currency != "" && currency != "USD" {
		symbol = currency + " "
	}
	if min > 0 && max > 0 && max != min {
		return fmt.Sprintf("%s%.0f–%s%.0f", symbol, min, symbol, max)
	}
	if min > 0 {
		return fmt.Sprintf("%s%.0f+", symbol, min)
	}
	return ""
}
