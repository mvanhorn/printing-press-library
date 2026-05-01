package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"text/tabwriter"

	"github.com/mvanhorn/printing-press-library/library/commerce/dominos/internal/client"
	"github.com/spf13/cobra"
)

type bestDeal struct {
	Code        string   `json:"code"`
	Name        string   `json:"name"`
	Original    *float64 `json:"original,omitempty"`
	AfterDeal   *float64 `json:"after_deal,omitempty"`
	YouSave     *float64 `json:"you_save,omitempty"`
	LoyaltyOnly bool     `json:"loyalty_only"`
	Expires     string   `json:"expires,omitempty"`
	Source      string   `json:"source,omitempty"`
}

type couponCandidate struct {
	Code, Name, Expires, Source, Tags string
	LoyaltyOnly                       bool
}

func newDealsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "deals", Short: "Find and rank Domino's deals"}
	cmd.AddCommand(newDealsBestCmd(flags))
	return cmd
}

func newDealsBestCmd(flags *rootFlags) *cobra.Command {
	var cartPath, storeID string
	var includeLoyalty bool
	var topN int
	cmd := &cobra.Command{
		Use:   "best",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Find the best deal for a cart",
		Example: "  dominos-pp-cli deals best --cart cart.json\n" +
			"  dominos-pp-cli deals best --store 7094 --include-loyalty=false",
		RunE: func(cmd *cobra.Command, args []string) error {
			if cartPath == "" && storeID == "" && !flags.dryRun {
				return fmt.Errorf("required flag \"cart\" or \"store\" not set")
			}
			if cartPath != "" && storeID != "" {
				return fmt.Errorf("use either --cart or --store, not both")
			}
			if topN <= 0 {
				return fmt.Errorf("--top must be greater than 0")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			var order map[string]any
			if cartPath != "" {
				if order, err = loadTemplateOrder(cartPath); err != nil {
					return err
				}
				storeID = stringValue(order["StoreID"])
				if storeID == "" && !flags.dryRun {
					return fmt.Errorf("cart requires StoreID")
				}
			}

			deals, err := dealsFetch(c, storeID, includeLoyalty)
			if err != nil {
				return err
			}
			if len(deals) == 0 {
				return fmt.Errorf("no deals found for store %s", storeID)
			}

			results, skipped, heuristic := make([]bestDeal, 0, len(deals)), 0, false
			for _, deal := range deals {
				results = append(results, bestDeal{Code: deal.Code, Name: deal.Name, LoyaltyOnly: deal.LoyaltyOnly, Expires: deal.Expires, Source: deal.Source})
			}
			sort.Slice(results, func(i, j int) bool {
				if results[i].LoyaltyOnly != results[j].LoyaltyOnly {
					return !results[i].LoyaltyOnly
				}
				return results[i].Code < results[j].Code
			})
			if order != nil {
				results, skipped, heuristic, err = dealsEvaluate(c, order, deals)
				if err != nil {
					return err
				}
				if heuristic {
					fmt.Fprintln(os.Stderr, "warning: price-order auth failed; falling back to tag-based filtering without savings amounts")
				} else {
					fmt.Fprintf(os.Stderr, "%d deals not applicable to this cart\n", skipped)
				}
			}
			return dealsPrint(cmd, flags, results, skipped, topN)
		},
	}
	cmd.Flags().StringVar(&cartPath, "cart", "", "Path to a cart JSON file")
	cmd.Flags().StringVar(&storeID, "store", "", "Store ID to inspect without cart matching")
	cmd.Flags().BoolVar(&includeLoyalty, "include-loyalty", true, "Include loyalty-exclusive deals")
	cmd.Flags().IntVar(&topN, "top", 5, "Show top N deal options")
	return cmd
}

func dealsFetch(c *client.Client, storeID string, includeLoyalty bool) ([]couponCandidate, error) {
	menuRaw, err := c.Get("/power/store/"+storeID+"/menu", map[string]string{"lang": "en", "structured": "true"})
	if err != nil {
		return nil, classifyAPIError(err)
	}
	var menu map[string]any
	if err := json.Unmarshal(extractResponseData(menuRaw), &menu); err != nil {
		return nil, fmt.Errorf("parse menu response: %w", err)
	}
	seen, out := map[string]struct{}{}, dealsCollect(menu["Coupons"], false, "public", nil)
	for _, deal := range out {
		seen[deal.Code] = struct{}{}
	}
	if !includeLoyalty {
		return out, nil
	}
	data, _, err := c.Post("/api/web-bff/graphql", map[string]any{"operationName": "LoyaltyDeals"})
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: loyalty deals unavailable: %v\n", classifyAPIError(err))
		return out, nil
	}
	var payload any
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("parse loyalty deals response: %w", err)
	}
	return dealsCollect(payload, true, "loyalty", seen), nil
}

func dealsCollect(raw any, loyalty bool, source string, seen map[string]struct{}) []couponCandidate {
	if seen == nil {
		seen = map[string]struct{}{}
	}
	var out []couponCandidate
	var walk func(any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			pick := func(keys ...string) string {
				for _, key := range keys {
					if s := comparePricesString(x[key]); s != "" {
						return s
					}
				}
				return ""
			}
			code := pick("Code", "code", "CouponCode", "couponCode", "ID", "id")
			name := pick("Name", "name", "Title", "title", "Description", "description")
			if code != "" && name != "" {
				tagValues := []string{}
				switch tags := x["Tags"].(type) {
				case []any:
					for _, item := range tags {
						if s := comparePricesString(item); s != "" {
							tagValues = append(tagValues, s)
						}
					}
				case []string:
					tagValues = tags
				default:
					if s := comparePricesString(tags); s != "" {
						tagValues = []string{s}
					}
				}
				if _, ok := seen[code]; !ok {
					seen[code] = struct{}{}
					out = append(out, couponCandidate{
						Code:        code,
						Name:        name,
						Expires:     pick("ExpirationDate", "expirationDate", "ExpireDate", "expiresAt", "EndDate"),
						Source:      source,
						LoyaltyOnly: loyalty,
						Tags:        strings.ToLower(strings.Join(tagValues, " ")),
					})
				}
			}
			for _, child := range x {
				walk(child)
			}
		case []any:
			for _, child := range x {
				walk(child)
			}
		}
	}
	walk(raw)
	return out
}

func dealsEvaluate(c *client.Client, order map[string]any, deals []couponCandidate) ([]bestDeal, int, bool, error) {
	original, _, err := dealsPrice(c, order, "")
	if err != nil {
		if dealsAuthError(err) {
			return dealsHeuristic(order, deals), 0, true, nil
		}
		return nil, 0, false, classifyAPIError(err)
	}
	var (
		results      []bestDeal
		skipped      int
		authFallback bool
		mu           sync.Mutex
		wg           sync.WaitGroup
		sem          = make(chan struct{}, 4)
	)
	for _, deal := range deals {
		deal := deal
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			total, savings, err := dealsPrice(c, order, deal.Code)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				if dealsAuthError(err) {
					authFallback = true
				} else {
					skipped++
				}
				return
			}
			if savings <= 0 && total >= original {
				skipped++
				return
			}
			results = append(results, bestDeal{
				Code:        deal.Code,
				Name:        deal.Name,
				Original:    &original,
				AfterDeal:   &total,
				YouSave:     &savings,
				LoyaltyOnly: deal.LoyaltyOnly,
				Expires:     deal.Expires,
				Source:      deal.Source,
			})
		}()
	}
	wg.Wait()
	if authFallback {
		return dealsHeuristic(order, deals), 0, true, nil
	}
	sort.Slice(results, func(i, j int) bool {
		if *results[i].YouSave != *results[j].YouSave {
			return *results[i].YouSave > *results[j].YouSave
		}
		if *results[i].AfterDeal != *results[j].AfterDeal {
			return *results[i].AfterDeal < *results[j].AfterDeal
		}
		return results[i].Code < results[j].Code
	})
	return results, skipped, false, nil
}

func dealsPrice(c *client.Client, order map[string]any, code string) (float64, float64, error) {
	cloned := cloneMap(order)
	if code == "" {
		delete(cloned, "Coupons")
	} else {
		cloned["Coupons"] = []map[string]any{{"Code": code}}
	}
	data, _, err := c.Post("/power/price-order", map[string]any{"Order": ensureOrderTaker(cloned)})
	if err != nil {
		return 0, 0, err
	}
	var payload any
	if err := json.Unmarshal(data, &payload); err != nil {
		return 0, 0, fmt.Errorf("parse price-order response: %w", err)
	}
	total, ok := dealsNumber(payload, func(path string) bool {
		return path == "Order.AmountsBreakdown.Customer" || path == "AmountsBreakdown.Customer" || strings.HasSuffix(path, ".Customer")
	})
	if !ok {
		return 0, 0, fmt.Errorf("price-order response missing customer total")
	}
	savings, _ := dealsNumber(payload, func(path string) bool {
		path = strings.ToLower(path)
		return strings.Contains(path, "savingsamount") || strings.HasSuffix(path, ".savings")
	})
	return total, savings, nil
}

func dealsHeuristic(order map[string]any, deals []couponCandidate) []bestDeal {
	productsRaw, _ := json.Marshal(order["Products"])
	products, service := strings.ToLower(string(productsRaw)), strings.ToLower(stringValue(order["ServiceMethod"]))
	var out []bestDeal
	for _, deal := range deals {
		if strings.Contains(deal.Tags, "carryout") && service != "carryout" {
			continue
		}
		if strings.Contains(deal.Tags, "delivery") && service != "delivery" {
			continue
		}
		if (strings.Contains(deal.Tags, "pizza") || strings.Contains(deal.Tags, "mixandmatch") || strings.Contains(deal.Tags, "pizzadeal")) &&
			!(strings.Contains(products, "pizza") || strings.Contains(products, "screen") || strings.Contains(products, "handtossed")) {
			continue
		}
		out = append(out, bestDeal{Code: deal.Code, Name: deal.Name, LoyaltyOnly: deal.LoyaltyOnly, Expires: deal.Expires, Source: deal.Source})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
	return out
}

func dealsPrint(cmd *cobra.Command, flags *rootFlags, results []bestDeal, skipped, topN int) error {
	if len(results) > topN {
		results = results[:topN]
	}
	money := func(v *float64) string {
		if v == nil {
			return "—"
		}
		return fmt.Sprintf("$%.2f", *v)
	}
	var recommended any
	if len(results) > 0 {
		recommended = results[0]
	}
	if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
		return flags.printJSON(cmd, map[string]any{"applicable_deals": results, "not_applicable_count": skipped, "recommended": recommended})
	}
	headers := "CODE\tNAME\tORIGINAL\tAFTER DEAL\tYOU SAVE\tELIGIBILITY"
	if results == nil || (len(results) > 0 && results[0].Original == nil && results[0].AfterDeal == nil && results[0].YouSave == nil) {
		headers = "CODE\tNAME\tELIGIBILITY\tSOURCE"
	}
	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
	fmt.Fprintln(tw, headers)
	for _, deal := range results {
		eligibility := "standard"
		if deal.LoyaltyOnly && deal.Expires != "" {
			eligibility = "loyalty-only, expires " + deal.Expires
		} else if deal.LoyaltyOnly {
			eligibility = "loyalty-only"
		} else if deal.Expires != "" {
			eligibility = "expires " + deal.Expires
		}
		if headers == "CODE\tNAME\tELIGIBILITY\tSOURCE" {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", deal.Code, deal.Name, eligibility, deal.Source)
		} else {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", deal.Code, deal.Name, money(deal.Original), money(deal.AfterDeal), money(deal.YouSave), eligibility)
		}
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	if recommended != nil && headers != "CODE\tNAME\tELIGIBILITY\tSOURCE" {
		fmt.Fprintf(cmd.OutOrStdout(), "\nRecommended: %s (%s saved)\n", results[0].Code, money(results[0].YouSave))
	}
	return nil
}

func dealsNumber(v any, match func(string) bool) (float64, bool) {
	var walk func(any, string) (float64, bool)
	walk = func(cur any, path string) (float64, bool) {
		switch x := cur.(type) {
		case map[string]any:
			for k, child := range x {
				next := k
				if path != "" {
					next = path + "." + k
				}
				if match(next) {
					if n, ok := comparePricesFloat(child); ok {
						return n, true
					}
				}
				if n, ok := walk(child, next); ok {
					return n, true
				}
			}
		case []any:
			for _, child := range x {
				if n, ok := walk(child, path); ok {
					return n, true
				}
			}
		}
		return 0, false
	}
	return walk(v, "")
}

func dealsAuthError(err error) bool {
	var apiErr *client.APIError
	if As(err, &apiErr) {
		return apiErr.StatusCode == 401 || apiErr.StatusCode == 403 || (apiErr.StatusCode == 400 && looksLikeAuthError(apiErr.Body))
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "http 401") || strings.Contains(msg, "http 403") || looksLikeAuthError(msg)
}
