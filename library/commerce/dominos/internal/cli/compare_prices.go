package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

type comparePricesStore struct {
	StoreID  string
	Distance float64
}

type comparePricesResult struct {
	StoreID         string             `json:"store_id"`
	Distance        float64            `json:"distance"`
	Items           map[string]float64 `json:"items"`
	Total           float64            `json:"total"`
	ItemsFoundCount int                `json:"items_found_count"`
	ItemsMissing    []string           `json:"items_missing"`
}

func newComparePricesCmd(flags *rootFlags) *cobra.Command {
	var flagAddress string
	var flagCity string
	var flagItems string
	var flagService string
	var flagMaxStores int

	cmd := &cobra.Command{
		Use:     "compare-prices",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:   "Compare nearby store pricing for a list of menu item codes",
		Long:    "Find the cheapest nearby Domino's store for a given list of menu item codes by syncing menus from nearby stores and joining on item codes locally.",
		Example: "  dominos-pp-cli compare-prices --address \"421 N 63rd St\" --city \"Seattle WA\" --items 14SCREEN,W08PHOTW\n  dominos-pp-cli compare-prices --address \"421 N 63rd St\" --city \"Seattle, WA 98103\" --items 14SCREEN,W08PHOTW,P_SAUCEEZE --service Carryout --max-stores 3 --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			// When invoked with no flags at all, print help so users (and verify) get a useful response.
			if !flags.dryRun && strings.TrimSpace(flagAddress) == "" && strings.TrimSpace(flagCity) == "" && strings.TrimSpace(flagItems) == "" {
				return cmd.Help()
			}
			if strings.TrimSpace(flagAddress) == "" && !flags.dryRun {
				return fmt.Errorf("required flag \"%s\" not set", "address")
			}
			if strings.TrimSpace(flagCity) == "" && !flags.dryRun {
				return fmt.Errorf("required flag \"%s\" not set", "city")
			}
			if strings.TrimSpace(flagItems) == "" && !flags.dryRun {
				return fmt.Errorf("required flag \"%s\" not set", "items")
			}
			if flagMaxStores <= 0 {
				return fmt.Errorf("--max-stores must be greater than 0")
			}
			switch flagService {
			case "Delivery", "Carryout":
			default:
				return fmt.Errorf("invalid --service value %q: must be Delivery or Carryout", flagService)
			}

			itemCodes := comparePricesParseItems(flagItems)
			if len(itemCodes) == 0 && !flags.dryRun {
				return fmt.Errorf("no valid item codes provided in --items")
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			fmt.Fprintf(os.Stderr, "Looking up nearby %s stores for %s, %s...\n", strings.ToLower(flagService), flagAddress, flagCity)
			storesRaw, err := c.Get("/power/store-locator", map[string]string{
				"s":    flagAddress,
				"c":    flagCity,
				"type": flagService,
			})
			if err != nil {
				return classifyAPIError(err)
			}
			if flags.dryRun {
				return nil
			}

			stores, err := comparePricesParseStores(storesRaw)
			if err != nil {
				return fmt.Errorf("parse store locator response: %w", err)
			}
			if len(stores) == 0 {
				return fmt.Errorf("no stores found for the given address")
			}
			if len(stores) > flagMaxStores {
				stores = stores[:flagMaxStores]
			}

			fmt.Fprintf(os.Stderr, "Fetching menus for %d stores and pricing %d item(s)...\n", len(stores), len(itemCodes))
			results, err := comparePricesFetchResults(c, stores, itemCodes)
			if err != nil {
				return err
			}

			sort.Slice(results, func(i, j int) bool {
				iComplete := len(results[i].ItemsMissing) == 0
				jComplete := len(results[j].ItemsMissing) == 0
				if iComplete != jComplete {
					return iComplete
				}
				if results[i].Total != results[j].Total {
					return results[i].Total < results[j].Total
				}
				if results[i].Distance != results[j].Distance {
					return results[i].Distance < results[j].Distance
				}
				return results[i].StoreID < results[j].StoreID
			})

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, results)
			}
			return comparePricesPrintTable(cmd, itemCodes, results)
		},
	}

	cmd.Flags().StringVar(&flagAddress, "address", "", "Street address line")
	cmd.Flags().StringVar(&flagCity, "city", "", "City, state, zip")
	cmd.Flags().StringVar(&flagItems, "items", "", "Comma-separated menu item codes")
	cmd.Flags().StringVar(&flagService, "service", "Delivery", "Service type: Delivery or Carryout")
	cmd.Flags().IntVar(&flagMaxStores, "max-stores", 5, "Max number of nearby stores to compare")

	return cmd
}

func comparePricesParseItems(raw string) []string {
	parts := strings.Split(raw, ",")
	items := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		code := strings.TrimSpace(part)
		if code == "" {
			continue
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}
		items = append(items, code)
	}
	return items
}

func comparePricesParseStores(data json.RawMessage) ([]comparePricesStore, error) {
	var envelope struct {
		Stores []map[string]any `json:"Stores"`
	}
	if err := json.Unmarshal(data, &envelope); err == nil && len(envelope.Stores) > 0 {
		return comparePricesNormalizeStores(envelope.Stores), nil
	}

	var items []map[string]any
	if err := json.Unmarshal(data, &items); err == nil && len(items) > 0 {
		return comparePricesNormalizeStores(items), nil
	}

	return nil, fmt.Errorf("unexpected store locator response shape")
}

func comparePricesNormalizeStores(items []map[string]any) []comparePricesStore {
	stores := make([]comparePricesStore, 0, len(items))
	for _, item := range items {
		storeID := comparePricesString(item["StoreID"])
		if storeID == "" {
			storeID = comparePricesString(item["StoreId"])
		}
		if storeID == "" {
			storeID = comparePricesString(item["store_id"])
		}
		if storeID == "" {
			continue
		}
		distance, _ := comparePricesFloat(item["MinDistance"])
		if distance == 0 {
			distance, _ = comparePricesFloat(item["Distance"])
		}
		if distance == 0 {
			distance, _ = comparePricesFloat(item["distance"])
		}
		stores = append(stores, comparePricesStore{StoreID: storeID, Distance: distance})
	}
	sort.Slice(stores, func(i, j int) bool {
		if stores[i].Distance != stores[j].Distance {
			return stores[i].Distance < stores[j].Distance
		}
		return stores[i].StoreID < stores[j].StoreID
	})
	return stores
}

func comparePricesFetchResults(c interface {
	Get(path string, params map[string]string) (json.RawMessage, error)
}, stores []comparePricesStore, itemCodes []string) ([]comparePricesResult, error) {
	results := make([]comparePricesResult, len(stores))
	errCh := make(chan error, len(stores))
	sem := make(chan struct{}, 4)
	var wg sync.WaitGroup

	for i, store := range stores {
		wg.Add(1)
		go func(i int, store comparePricesStore) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			menuRaw, err := c.Get("/power/store/"+store.StoreID+"/menu", map[string]string{
				"lang":       "en",
				"structured": "true",
			})
			if err != nil {
				errCh <- fmt.Errorf("fetch menu for store %s: %w", store.StoreID, classifyAPIError(err))
				return
			}

			menuPrices, err := comparePricesParseMenuPrices(extractResponseData(menuRaw))
			if err != nil {
				errCh <- fmt.Errorf("parse menu for store %s: %w", store.StoreID, err)
				return
			}

			result := comparePricesResult{
				StoreID:  store.StoreID,
				Distance: store.Distance,
				Items:    make(map[string]float64, len(itemCodes)),
			}
			for _, code := range itemCodes {
				price, ok := menuPrices[code]
				if !ok {
					result.ItemsMissing = append(result.ItemsMissing, code)
					continue
				}
				result.Items[code] = price
				result.ItemsFoundCount++
				result.Total += price
			}
			results[i] = result
		}(i, store)
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			return nil, err
		}
	}
	return results, nil
}

func comparePricesParseMenuPrices(data json.RawMessage) (map[string]float64, error) {
	var menu map[string]any
	if err := json.Unmarshal(data, &menu); err != nil {
		return nil, err
	}

	prices := make(map[string]float64)
	comparePricesReadPriceMap(prices, menu["Products"])
	comparePricesReadPriceMap(prices, menu["Variants"])
	return prices, nil
}

func comparePricesReadPriceMap(dst map[string]float64, raw any) {
	items, ok := raw.(map[string]any)
	if !ok {
		return
	}
	for code, rawItem := range items {
		obj, ok := rawItem.(map[string]any)
		if !ok {
			continue
		}
		price, ok := comparePricesFloat(obj["Price"])
		if !ok {
			price, ok = comparePricesFloat(obj["price"])
		}
		if ok {
			dst[code] = price
		}
	}
}

func comparePricesPrintTable(cmd *cobra.Command, itemCodes []string, results []comparePricesResult) error {
	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "RANK\tSTOREID\tDISTANCE\tITEMS FOUND\tTOTAL\tPER-ITEM PRICES")
	for i, result := range results {
		fmt.Fprintf(
			tw,
			"%d\t%s\t%s\t%d/%d\t%s\t%s\n",
			i+1,
			result.StoreID,
			comparePricesFormatDistance(result.Distance),
			result.ItemsFoundCount,
			len(itemCodes),
			comparePricesFormatMoney(result.Total),
			comparePricesPerItemSummary(itemCodes, result),
		)
	}
	return tw.Flush()
}

func comparePricesPerItemSummary(itemCodes []string, result comparePricesResult) string {
	parts := make([]string, 0, len(itemCodes))
	for _, code := range itemCodes {
		price, ok := result.Items[code]
		if !ok {
			parts = append(parts, code+"=—")
			continue
		}
		parts = append(parts, code+"="+comparePricesFormatMoney(price))
	}
	return strings.Join(parts, ", ")
}

func comparePricesFormatMoney(v float64) string {
	return fmt.Sprintf("$%.2f", v)
}

func comparePricesFormatDistance(v float64) string {
	if v == 0 {
		return "—"
	}
	return fmt.Sprintf("%.2f mi", v)
}

func comparePricesString(v any) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	case json.Number:
		return x.String()
	case float64:
		return strconv.FormatInt(int64(x), 10)
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", v))
	}
}

func comparePricesFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case json.Number:
		f, err := x.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(x), 64)
		return f, err == nil
	default:
		return 0, false
	}
}
