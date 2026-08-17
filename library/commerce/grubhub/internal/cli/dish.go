// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source auto

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/mvanhorn/printing-press-library/library/commerce/grubhub/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/grubhub/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/grubhub/internal/grubhub"
	"github.com/mvanhorn/printing-press-library/library/commerce/grubhub/internal/store"
	"github.com/spf13/cobra"
)

const menuItemsResource = "menu_items"

type cachedMenuItem struct {
	ID             string `json:"id"`
	ItemID         string `json:"item_id"`
	RestaurantID   string `json:"restaurant_id"`
	RestaurantName string `json:"restaurant_name"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	PriceCents     int    `json:"price_cents"`
	Category       string `json:"category"`
}

type dishMatch struct {
	Item         string `json:"item"`
	Price        string `json:"price"`
	PriceCents   int    `json:"price_cents"`
	Restaurant   string `json:"restaurant"`
	RestaurantID string `json:"restaurant_id"`
	ItemID       string `json:"item_id"`
	Category     string `json:"category,omitempty"`
}

type dishFailure struct {
	RestaurantID string `json:"restaurant_id"`
	Error        string `json:"error"`
}

type dishView struct {
	Query         string        `json:"query"`
	Matches       []dishMatch   `json:"matches"`
	ScannedMenus  int           `json:"scanned_menus"`
	MaxScanMenus  int           `json:"max_scan_menus"`
	DataSource    string        `json:"data_source"`
	FetchFailures []dishFailure `json:"fetch_failures"`
	Note          string        `json:"note,omitempty"`
}

func newNovelDishCmd(flags *rootFlags) *cobra.Command {
	var maxPrice float64
	var diet string
	var limit, maxScan int

	cmd := &cobra.Command{
		Use:   "dish <address> <query>",
		Short: "Find which nearby restaurants carry a specific dish, with price",
		Long: "Search the menus of nearby restaurants for a dish by name. In live/auto mode it scans nearby restaurant menus and caches them locally; in local mode it searches only menus already cached from a previous run.\n\n" +
			"Use this to find a specific menu item across all nearby restaurants. Do NOT use it to browse one known restaurant's full menu; use 'menu <restaurant-id>' for that. The --diet flag is a mechanical keyword match on item text, not a verified dietary certification.",
		Example:     "  grubhub-pp-cli dish \"350 5th Ave, New York, NY\" \"poke bowl\" --max-price 15",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would search nearby menus for the dish")
				return nil
			}
			local := flags.dataSource == "local"
			if !local && len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("an address and a dish query are required, e.g. dish \"350 5th Ave, New York, NY\" \"poke\""))
			}
			if local && len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a dish query is required in local mode, e.g. dish \"poke\" --data-source local"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			query := args[len(args)-1]
			if cliutil.IsDogfoodEnv() && maxScan > 3 {
				maxScan = 3
			}

			if local {
				return runDishLocal(cmd, flags, query, maxPrice, diet, limit, maxScan)
			}
			return runDishLive(ctx, cmd, flags, args[0], query, maxPrice, diet, limit, maxScan)
		},
	}
	cmd.Flags().Float64Var(&maxPrice, "max-price", 0, "Only items at or below this many dollars")
	cmd.Flags().StringVar(&diet, "diet", "", "Keyword match on item text (e.g. vegan, gluten-free) — not a certified dietary filter")
	cmd.Flags().IntVar(&limit, "limit", 30, "Maximum matching items to return")
	cmd.Flags().IntVar(&maxScan, "max-scan-restaurants", 12, "Maximum nearby restaurant menus to scan")
	return cmd
}

func runDishLive(ctx context.Context, cmd *cobra.Command, flags *rootFlags, address, query string, maxPrice float64, diet string, limit, maxScan int) error {
	c, err := grubhubClient(ctx, flags)
	if err != nil {
		return err
	}
	coord, err := geocodeAddress(ctx, c, address)
	if err != nil {
		return err
	}
	cards, _, err := searchCards(ctx, c, coord, searchOptions{pageSize: maxScan})
	if err != nil {
		return err
	}
	if len(cards) > maxScan {
		cards = cards[:maxScan]
	}

	point := grubhub.FormatPoint(coord.Lng, coord.Lat)
	items, failures := fetchMenus(ctx, c, cards, point)
	cacheMenuItems(ctx, items) // best-effort; offline reuse

	matches := matchDishes(items, query, maxPrice, diet)
	scanCapHit := len(cards) >= maxScan
	return renderDish(cmd, flags, dishView{
		Query:         query,
		Matches:       capMatches(matches, limit),
		ScannedMenus:  len(cards) - len(failures),
		MaxScanMenus:  maxScan,
		DataSource:    "live",
		FetchFailures: failures,
		Note:          dishNote(len(matches), scanCapHit, false),
	})
}

func runDishLocal(cmd *cobra.Command, flags *rootFlags, query string, maxPrice float64, diet string, limit, maxScan int) error {
	dbPath := defaultDBPath("grubhub-pp-cli")
	if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
		fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: grubhub-pp-cli dish \"<address>\" \"%s\" (live) first to populate menus\n", dbPath, query)
		if wantsJSON(cmd, flags) {
			return emitJSON(cmd, flags, dishView{Query: query, Matches: []dishMatch{}, DataSource: "local", FetchFailures: []dishFailure{}, Note: "no local mirror; run dish live for an address first to populate menus"})
		}
		return nil
	}
	db, err := store.Open(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	raws, err := db.List(menuItemsResource, 20000)
	if err != nil {
		return err
	}
	items := make([]cachedMenuItem, 0, len(raws))
	for _, raw := range raws {
		var it cachedMenuItem
		if json.Unmarshal(raw, &it) == nil {
			items = append(items, it)
		}
	}
	matches := matchDishes(items, query, maxPrice, diet)
	return renderDish(cmd, flags, dishView{
		Query:         query,
		Matches:       capMatches(matches, limit),
		ScannedMenus:  countRestaurants(items),
		MaxScanMenus:  maxScan,
		DataSource:    "local",
		FetchFailures: []dishFailure{},
		Note:          dishNote(len(matches), false, true),
	})
}

func fetchMenus(ctx context.Context, c *client.Client, cards []grubhub.Card, point string) ([]cachedMenuItem, []dishFailure) {
	var (
		mu       sync.Mutex
		items    []cachedMenuItem
		failures []dishFailure
		wg       sync.WaitGroup
	)
	sem := make(chan struct{}, 6)
	for _, card := range cards {
		wg.Add(1)
		go func(card grubhub.Card) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			raw, err := c.Get(ctx, "/restaurants/"+card.ID, map[string]string{
				"version":                  "4",
				"orderType":                "standard",
				"hideUnavailableMenuItems": "true",
				"location":                 point,
				"locationMode":             "delivery",
			})
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				failures = append(failures, dishFailure{RestaurantID: card.ID, Error: err.Error()})
				return
			}
			_, menuItems, perr := grubhub.ParseMenu(raw)
			if perr != nil {
				failures = append(failures, dishFailure{RestaurantID: card.ID, Error: perr.Error()})
				return
			}
			for _, it := range menuItems {
				items = append(items, cachedMenuItem{
					ID:             card.ID + ":" + it.ID,
					ItemID:         it.ID,
					RestaurantID:   card.ID,
					RestaurantName: card.Name,
					Name:           cliutil.CleanText(it.Name),
					Description:    cliutil.CleanText(it.Description),
					PriceCents:     it.PriceCents(),
					Category:       it.Category,
				})
			}
		}(card)
	}
	wg.Wait()
	return items, failures
}

func cacheMenuItems(ctx context.Context, items []cachedMenuItem) {
	if len(items) == 0 {
		return
	}
	dbPath := defaultDBPath("grubhub-pp-cli")
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return
	}
	defer db.Close()
	for _, it := range items {
		data, err := json.Marshal(it)
		if err != nil {
			continue
		}
		_ = db.Upsert(menuItemsResource, it.ID, data)
	}
}

func matchDishes(items []cachedMenuItem, query string, maxPrice float64, diet string) []dishMatch {
	// Tokenize the query so multi-word, reordered queries still match. A dish
	// matches when every query token appears somewhere in the haystack (AND,
	// order-independent), so `dish ... "poke bowl"` finds "Poke Time Bowl"
	// instead of requiring the literal contiguous phrase "poke bowl".
	qTokens := strings.Fields(strings.ToLower(strings.TrimSpace(query)))
	d := strings.ToLower(strings.TrimSpace(diet))
	maxCents := int(maxPrice * 100)
	// nameHits (all tokens in the item name) rank above descHits (some tokens
	// only in the description) so a dish search for "bowl" surfaces actual
	// bowls before a sauce whose description happens to mention bowls.
	var nameHits, descHits []dishMatch
	for _, it := range items {
		name := strings.ToLower(it.Name)
		hay := name + " " + strings.ToLower(it.Description)
		nameMatch := len(qTokens) == 0 || containsAll(name, qTokens)
		if len(qTokens) > 0 && !containsAll(hay, qTokens) {
			continue
		}
		if d != "" && !strings.Contains(hay, d) {
			continue
		}
		if maxCents > 0 && it.PriceCents > maxCents {
			continue
		}
		m := dishMatch{
			Item:         it.Name,
			Price:        grubhub.Dollars(it.PriceCents),
			PriceCents:   it.PriceCents,
			Restaurant:   it.RestaurantName,
			RestaurantID: it.RestaurantID,
			ItemID:       it.ItemID,
			Category:     it.Category,
		}
		if nameMatch {
			nameHits = append(nameHits, m)
		} else {
			descHits = append(descHits, m)
		}
	}
	sort.SliceStable(nameHits, func(i, j int) bool { return nameHits[i].PriceCents < nameHits[j].PriceCents })
	sort.SliceStable(descHits, func(i, j int) bool { return descHits[i].PriceCents < descHits[j].PriceCents })
	return append(nameHits, descHits...)
}

// containsAll reports whether haystack contains every token (order-independent).
func containsAll(haystack string, tokens []string) bool {
	for _, t := range tokens {
		if t != "" && !strings.Contains(haystack, t) {
			return false
		}
	}
	return true
}

func capMatches(m []dishMatch, limit int) []dishMatch {
	if limit > 0 && len(m) > limit {
		return m[:limit]
	}
	if m == nil {
		return []dishMatch{}
	}
	return m
}

func countRestaurants(items []cachedMenuItem) int {
	seen := map[string]struct{}{}
	for _, it := range items {
		seen[it.RestaurantID] = struct{}{}
	}
	return len(seen)
}

func dishNote(matchCount int, scanCapHit, localMode bool) string {
	if matchCount > 0 {
		return ""
	}
	if localMode {
		return "no cached menu items matched; run dish live for an address first to populate the cache"
	}
	if scanCapHit {
		return "no matches within the scanned menus; raise --max-scan-restaurants to widen the search"
	}
	return "no matching dishes found nearby"
}

func renderDish(cmd *cobra.Command, flags *rootFlags, view dishView) error {
	if len(view.FetchFailures) > 0 {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d restaurant menu(s) failed to load; results cover the remaining %d menu(s)\n", len(view.FetchFailures), view.ScannedMenus)
	}
	if wantsJSON(cmd, flags) {
		return emitJSON(cmd, flags, view)
	}
	out := cmd.OutOrStdout()
	if len(view.Matches) == 0 {
		if view.Note != "" {
			fmt.Fprintln(out, view.Note)
		} else {
			fmt.Fprintf(out, "No dishes matching %q found.\n", view.Query)
		}
		return nil
	}
	fmt.Fprintf(out, "%d dishes matching %q (scanned %d menus)\n\n", len(view.Matches), view.Query, view.ScannedMenus)
	tw := newTabWriter(out)
	fmt.Fprintln(tw, "DISH\tPRICE\tRESTAURANT\tRESTAURANT ID\tITEM ID")
	for _, m := range view.Matches {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", truncate(m.Item, 34), m.Price, truncate(m.Restaurant, 24), m.RestaurantID, m.ItemID)
	}
	return tw.Flush()
}
