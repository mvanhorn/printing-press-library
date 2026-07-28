// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source computed

package cli

import (
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/woocommerce/internal/source/storeapi"
	"github.com/mvanhorn/printing-press-library/library/commerce/woocommerce/internal/store"
)

type diffProduct struct {
	ProductID string  `json:"product_id"`
	Name      string  `json:"name"`
	SKU       string  `json:"sku,omitempty"`
	Price     float64 `json:"price"`
	Permalink string  `json:"permalink,omitempty"`
}

type diffPriceChange struct {
	ProductID string  `json:"product_id"`
	Name      string  `json:"name"`
	OldPrice  float64 `json:"old_price"`
	NewPrice  float64 `json:"new_price"`
	Change    float64 `json:"change"`
	ChangePct float64 `json:"change_pct"`
}

type diffSaleChange struct {
	ProductID    string  `json:"product_id"`
	Name         string  `json:"name"`
	RegularPrice float64 `json:"regular_price"`
	SalePrice    float64 `json:"sale_price"`
}

type diffStockFlip struct {
	ProductID   string `json:"product_id"`
	Name        string `json:"name"`
	FromInStock bool   `json:"from_in_stock"`
	ToInStock   bool   `json:"to_in_stock"`
}

type diffSummary struct {
	Added        int `json:"added"`
	Removed      int `json:"removed"`
	PriceChanges int `json:"price_changes"`
	SaleStarted  int `json:"sale_started"`
	SaleEnded    int `json:"sale_ended"`
	StockFlips   int `json:"stock_flips"`
}

type catalogDiffView struct {
	Store        string            `json:"store"`
	FromSnapshot string            `json:"from_snapshot"`
	ToSnapshot   string            `json:"to_snapshot"`
	SpanDays     float64           `json:"span_days"`
	Added        []diffProduct     `json:"added"`
	Removed      []diffProduct     `json:"removed"`
	PriceChanges []diffPriceChange `json:"price_changes"`
	SaleStarted  []diffSaleChange  `json:"sale_started"`
	SaleEnded    []diffSaleChange  `json:"sale_ended"`
	StockFlips   []diffStockFlip   `json:"stock_flips"`
	Summary      diffSummary       `json:"summary"`
	// MembershipComparable is false when either snapshot hit its page cap. A
	// partial snapshot makes added/removed meaningless, so they are withheld
	// rather than reported as if products had appeared or vanished.
	MembershipComparable bool   `json:"membership_comparable"`
	Warning              string `json:"warning,omitempty"`
	Note                 string `json:"note,omitempty"`
}

func newNovelCatalogDiffCmd(flags *rootFlags) *cobra.Command {
	var flagStore string
	var flagSince string
	var flagLimit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Compare two recorded catalog snapshots for price, sale, and stock movement",
		Long: "Compares two snapshots recorded by 'catalog watch' and reports what actually changed. It makes\n" +
			"no API call — the answers come entirely from local history, which is why it can answer\n" +
			"questions about the past that the live API has already forgotten.\n\n" +
			"Use this command to see what changed in a store's catalog between two recorded snapshots.\n" +
			"Do NOT use this command to fetch fresh data; run 'catalog watch' first to record a snapshot.\n" +
			"Do NOT use it to audit your own catalog for defects; use 'catalog audit' instead.",
		Example: "  woocommerce-pp-cli catalog diff --store https://woocommerce.com --since 30d --agent",
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:happy-args":  "--store=https://woocommerce.com",
			"pp:data-source": "computed",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compare recorded catalog snapshots from the local store")
				return nil
			}
			if flags != nil && flags.dataSource == "live" {
				return usageErr(fmt.Errorf("no live equivalent for \"catalog diff\": it compares snapshots already recorded by 'catalog watch'"))
			}
			if flagStore == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--store is required (for example --store https://woocommerce.com)"))
			}
			since, err := windowOrDefault(flagSince, "30d")
			if err != nil {
				return err
			}
			if flagLimit <= 0 {
				return usageErr(fmt.Errorf("--limit must be greater than zero"))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			client, err := storeapi.New(flagStore, flags.timeout)
			if err != nil {
				return usageErr(err)
			}
			storeKey := client.Origin()

			dbPath = resolveDBPath(dbPath)
			if localStoreMissing(cmd, flags, dbPath, "catalog") {
				return nil
			}
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			times, err := db.CatalogSnapshotTimes(ctx, storeKey)
			if err != nil {
				return err
			}
			if len(times) < 2 {
				view := emptyDiffView(storeKey)
				view.Note = fmt.Sprintf("only %d snapshot(s) recorded for %s; run 'woocommerce-pp-cli catalog watch --store %s' at least twice to compare",
					len(times), storeKey, storeKey)
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}

			newest := times[0]
			cutoff := time.Now().UTC().Add(-since)
			older := times[len(times)-1]
			note := ""
			picked := false
			for _, t := range times[1:] {
				if ts, ok := parseWooTime(t); ok && !ts.After(cutoff) {
					older = t
					picked = true
					break
				}
			}
			if !picked {
				note = fmt.Sprintf("no snapshot older than %s; compared against the oldest available instead", flagSince)
			}

			// Drain the older snapshot fully before reading the newer one.
			oldRows, err := db.CatalogSnapshotAt(ctx, storeKey, older)
			if err != nil {
				return err
			}
			newRows, err := db.CatalogSnapshotAt(ctx, storeKey, newest)
			if err != nil {
				return err
			}

			view := buildCatalogDiffView(storeKey, older, newest, oldRows, newRows, flagLimit)
			view.Note = note
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&flagStore, "store", "", "Store URL whose snapshots to compare")
	cmd.Flags().StringVar(&flagSince, "since", "30d", "Compare against the newest snapshot at least this old (30d, 1w)")
	cmd.Flags().IntVar(&flagLimit, "limit", 100, "Maximum entries per change category")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func emptyDiffView(storeKey string) catalogDiffView {
	return catalogDiffView{
		Store:        storeKey,
		Added:        make([]diffProduct, 0),
		Removed:      make([]diffProduct, 0),
		PriceChanges: make([]diffPriceChange, 0),
		SaleStarted:  make([]diffSaleChange, 0),
		SaleEnded:    make([]diffSaleChange, 0),
		StockFlips:   make([]diffStockFlip, 0),
	}
}

// buildCatalogDiffView is pure so the comparison logic stays unit-testable.
func buildCatalogDiffView(storeKey, from, to string, oldRows, newRows []store.CatalogSnapshotRow, limit int) catalogDiffView {
	view := emptyDiffView(storeKey)
	view.FromSnapshot, view.ToSnapshot = from, to
	if a, okA := parseWooTime(from); okA {
		if b, okB := parseWooTime(to); okB {
			view.SpanDays = round2(b.Sub(a).Hours() / 24)
		}
	}

	oldTruncated := len(oldRows) > 0 && oldRows[0].Truncated
	newTruncated := len(newRows) > 0 && newRows[0].Truncated
	view.MembershipComparable = !oldTruncated && !newTruncated

	oldByID := make(map[string]store.CatalogSnapshotRow, len(oldRows))
	for _, r := range oldRows {
		oldByID[r.ProductID] = r
	}
	seen := make(map[string]bool, len(newRows))

	for _, n := range newRows {
		seen[n.ProductID] = true
		o, existed := oldByID[n.ProductID]
		if !existed {
			view.Added = append(view.Added, diffProduct{
				ProductID: n.ProductID, Name: n.Name, SKU: n.SKU,
				Price: round2(n.Price), Permalink: n.Permalink,
			})
			continue
		}
		if n.Price != o.Price {
			var pct float64
			if o.Price != 0 {
				pct = (n.Price - o.Price) / o.Price * 100
			}
			view.PriceChanges = append(view.PriceChanges, diffPriceChange{
				ProductID: n.ProductID, Name: n.Name,
				OldPrice: round2(o.Price), NewPrice: round2(n.Price),
				Change: round2(n.Price - o.Price), ChangePct: round2(pct),
			})
		}
		switch {
		case n.OnSale && !o.OnSale:
			view.SaleStarted = append(view.SaleStarted, diffSaleChange{
				ProductID: n.ProductID, Name: n.Name,
				RegularPrice: round2(n.RegularPrice), SalePrice: round2(n.SalePrice),
			})
		case !n.OnSale && o.OnSale:
			view.SaleEnded = append(view.SaleEnded, diffSaleChange{
				ProductID: n.ProductID, Name: n.Name,
				RegularPrice: round2(n.RegularPrice), SalePrice: round2(o.SalePrice),
			})
		}
		if n.InStock != o.InStock {
			view.StockFlips = append(view.StockFlips, diffStockFlip{
				ProductID: n.ProductID, Name: n.Name,
				FromInStock: o.InStock, ToInStock: n.InStock,
			})
		}
	}
	for _, o := range oldRows {
		if !seen[o.ProductID] {
			view.Removed = append(view.Removed, diffProduct{
				ProductID: o.ProductID, Name: o.Name, SKU: o.SKU,
				Price: round2(o.Price), Permalink: o.Permalink,
			})
		}
	}

	sort.SliceStable(view.PriceChanges, func(i, j int) bool {
		return absFloat(view.PriceChanges[i].Change) > absFloat(view.PriceChanges[j].Change)
	})
	sort.SliceStable(view.Added, func(i, j int) bool { return view.Added[i].Name < view.Added[j].Name })
	sort.SliceStable(view.Removed, func(i, j int) bool { return view.Removed[i].Name < view.Removed[j].Name })

	if !view.MembershipComparable {
		// Price, sale and stock moves stay valid for products present in both
		// snapshots; only membership is unreliable.
		which := "both snapshots were"
		switch {
		case oldTruncated && !newTruncated:
			which = "the older snapshot was"
		case newTruncated && !oldTruncated:
			which = "the newer snapshot was"
		}
		view.Warning = "added/removed withheld: " + which + " truncated by --max-pages, so membership cannot be compared; re-run 'catalog watch' with a higher --max-pages. Price, sale and stock changes below remain valid."
		view.Added = make([]diffProduct, 0)
		view.Removed = make([]diffProduct, 0)
	}

	view.Summary = diffSummary{
		Added: len(view.Added), Removed: len(view.Removed),
		PriceChanges: len(view.PriceChanges), SaleStarted: len(view.SaleStarted),
		SaleEnded: len(view.SaleEnded), StockFlips: len(view.StockFlips),
	}

	if limit > 0 {
		if len(view.Added) > limit {
			view.Added = view.Added[:limit]
		}
		if len(view.Removed) > limit {
			view.Removed = view.Removed[:limit]
		}
		if len(view.PriceChanges) > limit {
			view.PriceChanges = view.PriceChanges[:limit]
		}
		if len(view.SaleStarted) > limit {
			view.SaleStarted = view.SaleStarted[:limit]
		}
		if len(view.SaleEnded) > limit {
			view.SaleEnded = view.SaleEnded[:limit]
		}
		if len(view.StockFlips) > limit {
			view.StockFlips = view.StockFlips[:limit]
		}
	}
	return view
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
