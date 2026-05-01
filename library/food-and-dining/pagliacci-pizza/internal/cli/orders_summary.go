// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/pagliacci-pizza/internal/store"
)

// SummaryByStore is the per-store rollup row in the orders summary output.
type SummaryByStore struct {
	StoreID   int     `json:"store_id"`
	StoreName string  `json:"store_name,omitempty"`
	Count     int     `json:"count"`
	Total     float64 `json:"total"`
}

// TopItem is one row in the top_items aggregation.
type TopItem struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// OrdersSummary is the aggregated output of `orders summary`.
type OrdersSummary struct {
	Since         string           `json:"since"`
	TotalSpend    float64          `json:"total_spend"`
	OrderCount    int              `json:"order_count"`
	AvgOrderValue float64          `json:"avg_order_value"`
	TopItems      []TopItem        `json:"top_items"`
	ByStore       []SummaryByStore `json:"by_store"`
}

// computeOrdersSummary aggregates a slice of order records into a summary.
// Each record is the raw JSON of one order from the local store.
// storeNames is an optional ID -> name lookup; missing names are left blank.
// since is the cutoff time; orders older than this are dropped.
func computeOrdersSummary(orders []json.RawMessage, since time.Time, storeNames map[int]string) OrdersSummary {
	itemCounts := map[string]int{}
	storeAgg := map[int]*SummaryByStore{}
	var totalSpend float64
	var orderCount int

	for _, raw := range orders {
		var o map[string]any
		if err := json.Unmarshal(raw, &o); err != nil {
			continue
		}

		// Order time: try a few likely field names. Pagliacci's
		// /OrderListGC uses "Date" (string), "OrderDate", "CreatedAt".
		t, ok := extractOrderTime(o)
		if ok && t.Before(since) {
			continue
		}

		// Total: try several common field names for an order's grand total.
		total := extractFloat(o, "Total", "OrderTotal", "GrandTotal", "Amount", "total")
		totalSpend += total
		orderCount++

		// Per-store: StoreID (preferred) or Store.ID
		sid := extractInt(o, "StoreID", "StoreId", "storeId", "store_id")
		if sid == 0 {
			if sub, ok := o["Store"].(map[string]any); ok {
				sid = extractInt(sub, "ID", "Id", "id")
			}
		}
		if _, ok := storeAgg[sid]; !ok {
			storeAgg[sid] = &SummaryByStore{StoreID: sid}
		}
		storeAgg[sid].Count++
		storeAgg[sid].Total += total

		// Items: walk an "Items" or "Products" array, count by Name.
		for _, key := range []string{"Items", "Products", "items", "LineItems"} {
			if arr, ok := o[key].([]any); ok {
				for _, it := range arr {
					if im, ok := it.(map[string]any); ok {
						name, _ := im["Name"].(string)
						if name == "" {
							name, _ = im["name"].(string)
						}
						if name != "" {
							itemCounts[name]++
						}
					}
				}
				break
			}
		}
	}

	// Top 5 items by count
	type kv struct {
		name string
		n    int
	}
	all := make([]kv, 0, len(itemCounts))
	for k, v := range itemCounts {
		all = append(all, kv{k, v})
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].n == all[j].n {
			return all[i].name < all[j].name
		}
		return all[i].n > all[j].n
	})
	topN := 5
	if len(all) < topN {
		topN = len(all)
	}
	top := make([]TopItem, 0, topN)
	for i := 0; i < topN; i++ {
		top = append(top, TopItem{Name: all[i].name, Count: all[i].n})
	}

	// Stable by-store output (sort by descending total)
	byStore := make([]SummaryByStore, 0, len(storeAgg))
	for _, v := range storeAgg {
		if storeNames != nil {
			v.StoreName = storeNames[v.StoreID]
		}
		byStore = append(byStore, *v)
	}
	sort.Slice(byStore, func(i, j int) bool {
		if byStore[i].Total == byStore[j].Total {
			return byStore[i].StoreID < byStore[j].StoreID
		}
		return byStore[i].Total > byStore[j].Total
	})

	avg := 0.0
	if orderCount > 0 {
		avg = totalSpend / float64(orderCount)
	}

	return OrdersSummary{
		Since:         since.UTC().Format(time.RFC3339),
		TotalSpend:    totalSpend,
		OrderCount:    orderCount,
		AvgOrderValue: avg,
		TopItems:      top,
		ByStore:       byStore,
	}
}

func extractOrderTime(o map[string]any) (time.Time, bool) {
	for _, key := range []string{"Date", "OrderDate", "CreatedAt", "created_at", "date"} {
		if v, ok := o[key]; ok {
			if s, ok := v.(string); ok && s != "" {
				for _, layout := range []string{
					time.RFC3339,
					time.RFC3339Nano,
					"2006-01-02T15:04:05",
					"2006-01-02 15:04:05",
					"2006-01-02",
					"01/02/2006",
				} {
					if t, err := time.Parse(layout, s); err == nil {
						return t, true
					}
				}
			}
		}
	}
	return time.Time{}, false
}

func extractFloat(o map[string]any, keys ...string) float64 {
	for _, k := range keys {
		if v, ok := o[k]; ok {
			switch x := v.(type) {
			case float64:
				return x
			case int:
				return float64(x)
			case string:
				if f, err := strconv.ParseFloat(x, 64); err == nil {
					return f
				}
			}
		}
	}
	return 0
}

func extractInt(o map[string]any, keys ...string) int {
	for _, k := range keys {
		if v, ok := o[k]; ok {
			switch x := v.(type) {
			case float64:
				return int(x)
			case int:
				return x
			case string:
				if n, err := strconv.Atoi(x); err == nil {
					return n
				}
			}
		}
	}
	return 0
}

func newOrdersSummaryCmd(flags *rootFlags) *cobra.Command {
	var since string

	cmd := &cobra.Command{
		Use:   "summary",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Aggregate order spend, order count, top items, and per-store breakdown from local sync data",
		Example: `  pagliacci-pizza-pp-cli orders summary
  pagliacci-pizza-pp-cli orders summary --since 90d --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cutoff, err := parseSinceForSummary(since)
			if err != nil {
				return usageErr(err)
			}

			db, err := openStoreForRead(cmd.Context(), "pagliacci-pizza-pp-cli")
			if err != nil {
				return &cliError{code: 5, err: fmt.Errorf("opening local store: %w", err)}
			}
			if db == nil {
				return &cliError{code: 5, err: fmt.Errorf("no local data. Run `pagliacci-pizza-pp-cli sync orders` first")}
			}
			defer db.Close()

			orders, err := db.List("orders", 0)
			if err != nil {
				return &cliError{code: 5, err: fmt.Errorf("querying local store: %w", err)}
			}
			if len(orders) == 0 {
				return &cliError{code: 5, err: fmt.Errorf("no synced orders. Run `pagliacci-pizza-pp-cli sync orders` first")}
			}

			storeNames := loadStoreNames(db)
			summary := computeOrdersSummary(orders, cutoff, storeNames)

			out, err := json.Marshal(summary)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&since, "since", "30d", "Only include orders newer than this (e.g. 7d, 30d, 90d, 1y)")
	return cmd
}

// parseSinceForSummary accepts 30d/90d/1y/24h-style durations. Returns the
// absolute cutoff time. An empty string defaults to 30d ago.
func parseSinceForSummary(s string) (time.Time, error) {
	if s == "" {
		s = "30d"
	}
	if len(s) < 2 {
		return time.Time{}, fmt.Errorf("invalid --since %q", s)
	}
	unit := s[len(s)-1]
	n, err := strconv.Atoi(s[:len(s)-1])
	if err != nil || n <= 0 {
		return time.Time{}, fmt.Errorf("invalid --since %q (use forms like 30d, 90d, 1y, 24h)", s)
	}
	now := time.Now()
	switch unit {
	case 'd':
		return now.Add(-time.Duration(n) * 24 * time.Hour), nil
	case 'h':
		return now.Add(-time.Duration(n) * time.Hour), nil
	case 'w':
		return now.Add(-time.Duration(n) * 7 * 24 * time.Hour), nil
	case 'y':
		return now.Add(-time.Duration(n) * 365 * 24 * time.Hour), nil
	}
	return time.Time{}, fmt.Errorf("invalid --since %q (unit must be d/h/w/y)", s)
}

// loadStoreNames builds an ID -> Name map from the local store cache.
// Returns an empty map if no store data is synced yet.
func loadStoreNames(db *store.Store) map[int]string {
	out := map[int]string{}
	items, err := db.List("store", 0)
	if err != nil {
		return out
	}
	for _, raw := range items {
		var s map[string]any
		if json.Unmarshal(raw, &s) != nil {
			continue
		}
		id := extractInt(s, "ID", "Id", "id")
		name, _ := s["Name"].(string)
		if id != 0 && name != "" {
			out[id] = name
		}
	}
	return out
}
