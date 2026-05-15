package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/booking-com-demand/internal/store"

	"github.com/spf13/cobra"
)

func newSyncCmd(flags *rootFlags) *cobra.Command {
	var full bool
	var dbPath string
	var tables string

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync API data to the local SQLite store for offline search, sql, and novel commands",
		Long: `Sync fetches data from the Booking.com Demand API and stores it locally in SQLite.

By default, syncs accommodations, reviews, and review_scores for properties
previously searched. Use --full for a broader refresh.

Synced data powers: search, sql, accommodations drift, reviews rank,
orders upcoming, accommodations changelog, accommodations compare, and locations intel.`,
		Example: strings.Trim(`
  booking-com-demand-pp-cli sync
  booking-com-demand-pp-cli sync --full
  booking-com-demand-pp-cli sync --tables accommodations,reviews
  booking-com-demand-pp-cli sync --db /tmp/booking.db`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			if dbPath == "" {
				dbPath = store.DefaultPath()
			}
			s, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer s.Close()

			syncTables := map[string]bool{
				"accommodations": true,
				"reviews":        true,
				"review_scores":  true,
			}
			if full {
				syncTables["availability"] = true
				syncTables["orders"] = true
				syncTables["messages"] = true
				syncTables["locations_cities"] = true
				syncTables["locations_districts"] = true
				syncTables["locations_landmarks"] = true
			}
			if tables != "" {
				syncTables = map[string]bool{}
				for _, t := range strings.Split(tables, ",") {
					syncTables[strings.TrimSpace(t)] = true
				}
			}

			summary := map[string]int{}

			if syncTables["accommodations"] {
				n, err := syncAccommodations(c, s, full)
				if err != nil {
					fmt.Fprintf(os.Stderr, "warning: accommodations sync failed: %v\n", err)
				} else {
					summary["accommodations"] = n
				}
			}

			if syncTables["reviews"] {
				n, err := syncReviews(c, s)
				if err != nil {
					fmt.Fprintf(os.Stderr, "warning: reviews sync failed: %v\n", err)
				} else {
					summary["reviews"] = n
				}
			}

			if syncTables["review_scores"] {
				n, err := syncReviewScores(c, s)
				if err != nil {
					fmt.Fprintf(os.Stderr, "warning: review_scores sync failed: %v\n", err)
				} else {
					summary["review_scores"] = n
				}
			}

			if syncTables["availability"] {
				n, err := syncAvailability(c, s)
				if err != nil {
					fmt.Fprintf(os.Stderr, "warning: availability sync failed: %v\n", err)
				} else {
					summary["availability"] = n
				}
			}

			if syncTables["orders"] {
				n, err := syncOrders(c, s)
				if err != nil {
					fmt.Fprintf(os.Stderr, "warning: orders sync failed: %v\n", err)
				} else {
					summary["orders"] = n
				}
			}

			if syncTables["messages"] {
				n, err := syncMessages(c, s)
				if err != nil {
					fmt.Fprintf(os.Stderr, "warning: messages sync failed: %v\n", err)
				} else {
					summary["messages"] = n
				}
			}

			if syncTables["locations_cities"] {
				n, err := syncLocations(c, s, "cities", "/common/locations/cities")
				if err != nil {
					fmt.Fprintf(os.Stderr, "warning: locations_cities sync failed: %v\n", err)
				} else {
					summary["locations_cities"] = n
				}
			}

			if syncTables["locations_districts"] {
				n, err := syncLocations(c, s, "districts", "/common/locations/districts")
				if err != nil {
					fmt.Fprintf(os.Stderr, "warning: locations_districts sync failed: %v\n", err)
				} else {
					summary["locations_districts"] = n
				}
			}

			if syncTables["locations_landmarks"] {
				n, err := syncLocations(c, s, "landmarks", "/common/locations/landmarks")
				if err != nil {
					fmt.Fprintf(os.Stderr, "warning: locations_landmarks sync failed: %v\n", err)
				} else {
					summary["locations_landmarks"] = n
				}
			}

			result := map[string]any{
				"status":  "ok",
				"synced":  summary,
				"db_path": dbPath,
			}
			return flags.printJSON(cmd, result)
		},
	}

	cmd.Flags().BoolVar(&full, "full", false, "Full refresh: sync all tables including availability, orders, messages, and locations")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to SQLite store (default: ~/.cache/booking-com-demand-pp-cli/store.db)")
	cmd.Flags().StringVar(&tables, "tables", "", "Comma-separated list of tables to sync (overrides --full)")

	return cmd
}

type apiClient interface {
	Get(path string, params map[string]string) (json.RawMessage, error)
	Post(path string, body any) (json.RawMessage, int, error)
}

func syncAccommodations(c apiClient, s *store.Store, _ bool) (int, error) {
	// Fetch IDs from existing store to re-sync, or do an initial search
	ids, err := getStoredIDs(s, "accommodations")
	if err != nil || len(ids) == 0 {
		fmt.Fprintf(os.Stderr, "sync: no stored accommodations to refresh (run 'accommodations search' first to populate)\n")
		return 0, nil
	}

	count := 0
	for _, id := range ids {
		data, err := c.Get("/accommodations/details", map[string]string{"accommodations": id})
		if err != nil {
			fmt.Fprintf(os.Stderr, "sync: skipping accommodation %s: %v\n", id, err)
			continue
		}
		if err := s.UpsertAccommodation(id, data); err != nil {
			fmt.Fprintf(os.Stderr, "sync: upsert accommodation %s: %v\n", id, err)
			continue
		}
		count++
	}
	return count, nil
}

func syncReviews(c apiClient, s *store.Store) (int, error) {
	ids, err := getStoredIDs(s, "accommodations")
	if err != nil || len(ids) == 0 {
		return 0, nil
	}

	count := 0
	for _, id := range ids {
		body := map[string]any{"accommodations": []string{id}, "rows": 25}
		data, _, err := c.Post("/accommodations/reviews", body)
		if err != nil {
			continue
		}
		var items []map[string]any
		if json.Unmarshal(data, &items) == nil {
			for _, item := range items {
				rid := fmt.Sprintf("%v", item["id"])
				if rid == "" || rid == "<nil>" {
					rid = fmt.Sprintf("%s-%d", id, count)
				}
				raw, _ := json.Marshal(item)
				if err := s.UpsertReview(rid, raw); err == nil {
					count++
				}
			}
		}
	}
	return count, nil
}

func syncReviewScores(c apiClient, s *store.Store) (int, error) {
	ids, err := getStoredIDs(s, "accommodations")
	if err != nil || len(ids) == 0 {
		return 0, nil
	}

	count := 0
	for _, id := range ids {
		body := map[string]any{"accommodations": []string{id}}
		data, _, err := c.Post("/accommodations/reviews/scores", body)
		if err != nil {
			continue
		}
		if err := s.Upsert("review_scores", id, data); err == nil {
			count++
		}
	}
	return count, nil
}

func syncAvailability(c apiClient, s *store.Store) (int, error) {
	ids, err := getStoredIDs(s, "accommodations")
	if err != nil || len(ids) == 0 {
		return 0, nil
	}

	count := 0
	body := map[string]any{"accommodations": ids}
	data, _, err := c.Post("/accommodations/bulk-availability", body)
	if err != nil {
		return 0, err
	}
	var items []map[string]any
	if json.Unmarshal(data, &items) == nil {
		for _, item := range items {
			aid := fmt.Sprintf("%v", item["id"])
			if aid == "" || aid == "<nil>" {
				continue
			}
			raw, _ := json.Marshal(item)
			if err := s.Upsert("availability", aid, raw); err == nil {
				count++
			}
		}
	}
	return count, nil
}

func syncOrders(c apiClient, s *store.Store) (int, error) {
	data, err := c.Get("/orders", nil)
	if err != nil {
		return 0, err
	}
	return upsertArrayResult(s, "orders", data)
}

func syncMessages(c apiClient, s *store.Store) (int, error) {
	body := map[string]any{}
	data, _, err := c.Post("/messages/fetch-latest", body)
	if err != nil {
		return 0, err
	}
	return upsertArrayResult(s, "messages", data)
}

func syncLocations(c apiClient, s *store.Store, kind, path string) (int, error) {
	data, err := c.Get(path, nil)
	if err != nil {
		return 0, err
	}
	table := "locations_" + kind
	return upsertArrayResult(s, table, data)
}

func upsertArrayResult(s *store.Store, table string, data json.RawMessage) (int, error) {
	var items []map[string]any
	if err := json.Unmarshal(data, &items); err != nil {
		// Try unwrapping envelope
		var envelope map[string]json.RawMessage
		if json.Unmarshal(data, &envelope) == nil {
			for _, key := range []string{"data", "items", "results"} {
				if raw, ok := envelope[key]; ok {
					if json.Unmarshal(raw, &items) == nil {
						break
					}
				}
			}
		}
	}

	count := 0
	for _, item := range items {
		id := fmt.Sprintf("%v", item["id"])
		if id == "" || id == "<nil>" {
			id = fmt.Sprintf("row-%d", count)
		}
		raw, _ := json.Marshal(item)
		if err := s.Upsert(table, id, raw); err == nil {
			count++
		}
	}
	return count, nil
}

func getStoredIDs(s *store.Store, table string) ([]string, error) {
	rows, err := s.DB().Query(fmt.Sprintf("SELECT id FROM %s LIMIT 500", table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	return ids, nil
}
