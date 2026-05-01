package cli

import (
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/espn/internal/store"
	"github.com/spf13/cobra"
)

func newSearchCmd(flags *rootFlags) *cobra.Command {
	var sport, league, dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:   "search <query>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Full-text search across synced events and news",
		Example: `  espn-pp-cli search "Lakers"
  espn-pp-cli search "touchdown" --sport football --limit 10
  espn-pp-cli search "trade" --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return usageErr(fmt.Errorf("query is required\nUsage: search <query>"))
			}
			query := args[0]

			if dbPath == "" {
				dbPath = defaultDBPath("espn-pp-cli")
			}

			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w\nhint: run 'espn-pp-cli sync' first", err)
			}
			defer db.Close()

			// Search domain-specific tables
			events, err := db.SearchEvents(query, limit)
			if err != nil {
				events = nil
			}

			news, err := db.SearchNews(query, limit)
			if err != nil {
				news = nil
			}

			// Also search the generic resources table (populated by write-through
			// caching from live API calls). This catches data that was looked up
			// live but never synced into domain-specific tables.
			general, _ := db.Search(query, limit)

			// Filter by sport/league if specified
			if sport != "" || league != "" {
				events = filterByField(events, sport, league)
				news = filterByField(news, sport, league)
				general = filterByField(general, sport, league)
			}

			// Deduplicate general results against domain results
			seen := make(map[string]bool)
			for _, items := range [][]json.RawMessage{events, news} {
				for _, raw := range items {
					var obj map[string]json.RawMessage
					if json.Unmarshal(raw, &obj) == nil {
						if id, ok := obj["id"]; ok {
							seen[string(id)] = true
						}
					}
				}
			}
			var extra []json.RawMessage
			for _, raw := range general {
				var obj map[string]json.RawMessage
				if json.Unmarshal(raw, &obj) == nil {
					if id, ok := obj["id"]; ok {
						if !seen[string(id)] {
							extra = append(extra, raw)
							seen[string(id)] = true
						}
					}
				}
			}

			type searchResults struct {
				Events  []json.RawMessage `json:"events"`
				News    []json.RawMessage `json:"news"`
				General []json.RawMessage `json:"general,omitempty"`
				Total   int               `json:"total"`
			}

			results := searchResults{
				Events:  events,
				News:    news,
				General: extra,
				Total:   len(events) + len(news) + len(extra),
			}

			if results.Events == nil {
				results.Events = []json.RawMessage{}
			}
			if results.News == nil {
				results.News = []json.RawMessage{}
			}

			// JSON output
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(results)
			}

			// Table output
			w := cmd.OutOrStdout()
			if len(events) > 0 {
				fmt.Fprintf(w, "%s (%d)\n", bold("EVENTS"), len(events))
				tw := newTabWriter(w)
				fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\n",
					bold("ID"), bold("NAME"), bold("DATE"), bold("STATUS"))
				for _, raw := range events {
					var ev map[string]any
					if json.Unmarshal(raw, &ev) != nil {
						continue
					}
					name := jsonStrAny(ev, "shortName")
					if name == "" {
						name = jsonStrAny(ev, "name")
					}
					fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\n",
						truncate(jsonStrAny(ev, "id"), 12),
						truncate(name, 30),
						truncate(jsonStrAny(ev, "date"), 10),
						statusFromEvent(ev))
				}
				tw.Flush()
			}

			if len(news) > 0 {
				if len(events) > 0 {
					fmt.Fprintln(w)
				}
				fmt.Fprintf(w, "%s (%d)\n", bold("NEWS"), len(news))
				tw := newTabWriter(w)
				fmt.Fprintf(tw, "  %s\t%s\t%s\n",
					bold("HEADLINE"), bold("BYLINE"), bold("PUBLISHED"))
				for _, raw := range news {
					var article map[string]any
					if json.Unmarshal(raw, &article) != nil {
						continue
					}
					fmt.Fprintf(tw, "  %s\t%s\t%s\n",
						truncate(jsonStrAny(article, "headline"), 40),
						truncate(jsonStrAny(article, "byline"), 20),
						truncate(jsonStrAny(article, "published"), 10))
				}
				tw.Flush()
			}

			if len(extra) > 0 {
				if len(events) > 0 || len(news) > 0 {
					fmt.Fprintln(w)
				}
				fmt.Fprintf(w, "%s (%d)\n", bold("CACHED"), len(extra))
				tw := newTabWriter(w)
				fmt.Fprintf(tw, "  %s\t%s\n", bold("ID"), bold("SNIPPET"))
				for _, raw := range extra {
					var obj map[string]any
					if json.Unmarshal(raw, &obj) != nil {
						continue
					}
					id := jsonStrAny(obj, "id")
					// Try common title/name fields for a useful snippet
					snippet := jsonStrAny(obj, "headline")
					if snippet == "" {
						snippet = jsonStrAny(obj, "shortName")
					}
					if snippet == "" {
						snippet = jsonStrAny(obj, "name")
					}
					if snippet == "" {
						snippet = jsonStrAny(obj, "description")
					}
					fmt.Fprintf(tw, "  %s\t%s\n", truncate(id, 15), truncate(snippet, 50))
				}
				tw.Flush()
			}

			if results.Total == 0 {
				fmt.Fprintf(w, "No results for %q. Run 'espn-pp-cli sync' to populate data.\n", query)
			} else {
				fmt.Fprintf(w, "\n%d results\n", results.Total)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&sport, "sport", "", "Filter by sport")
	cmd.Flags().StringVar(&league, "league", "", "Filter by league")
	cmd.Flags().IntVar(&limit, "limit", 20, "Max results per category")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")

	return cmd
}

func statusFromEvent(ev map[string]any) string {
	if statusObj, ok := ev["status"].(map[string]any); ok {
		if typeObj, ok := statusObj["type"].(map[string]any); ok {
			if d := jsonStrAny(typeObj, "detail"); d != "" {
				return d
			}
			return jsonStrAny(typeObj, "state")
		}
	}
	return ""
}

func filterByField(items []json.RawMessage, sport, league string) []json.RawMessage {
	if sport == "" && league == "" {
		return items
	}
	var filtered []json.RawMessage
	for _, raw := range items {
		var obj map[string]any
		if json.Unmarshal(raw, &obj) != nil {
			continue
		}
		// Check direct sport/league fields (from domain tables)
		// or nested paths for raw ESPN data
		match := true
		if sport != "" {
			// Try direct field or check competitions
			if s, ok := obj["sport"].(string); ok {
				if s != sport {
					match = false
				}
			}
		}
		if league != "" {
			if l, ok := obj["league"].(string); ok {
				if l != league {
					match = false
				}
			}
		}
		if match {
			filtered = append(filtered, raw)
		}
	}
	return filtered
}
