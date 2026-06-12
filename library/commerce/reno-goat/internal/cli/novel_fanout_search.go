// Novel command: fan-out product search across all active sources with
// category routing, partial-failure tolerance, and unified output.

package cli

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/reno-goat/internal/cliutil"
	"github.com/spf13/cobra"
)

// shopifyStore describes one Shopify DTC storefront for fan-out search.
type shopifyStore struct {
	Domain string
	Name   string
	Token  string
}

// shopifyStores is the hardcoded store list from the spec. Each store has its
// own domain and storefront access token.
var shopifyStores = []shopifyStore{
	{Domain: "schoolhouseelectric", Name: "Schoolhouse", Token: "6b9644bb298124bc9ade899eaddea363"},
	{Domain: "bludot", Name: "Blu Dot", Token: "1e4672177051168711b9283f503746a7"},
	{Domain: "gus-design-group", Name: "Gus Modern", Token: "1875077237db56b54e58dac554913b32"},
	{Domain: "floyd-home", Name: "Floyd", Token: "a89468f33bb6a48a0db09360abcd89fb"},
	{Domain: "lulu-and-georgia", Name: "Lulu & Georgia", Token: "a1c43345d9845c6c42cd62ddb895ffbb"},
}

// newFanoutSearchCmd wires the fan-out search as the RunE on the product-search
// parent command when the user passes a positional query argument. It is also
// registered as a subcommand (`product-search all`) for explicit invocation.
func newFanoutSearchCmd(flags *rootFlags) *cobra.Command {
	var (
		categoryFlag string
		roomFlag     string
		sourceFlag   string
		sortFlag     string
		minPrice     float64
		maxPrice     float64
		perPage      int
	)

	cmd := &cobra.Command{
		Use:   "all <query>",
		Short: "Search ALL sources in parallel with category routing. Returns unified, normalized results.",
		Long: `Fan-out search across all active sources. Category-based routing
sends queries to the sources that carry each product type.

By default, all active sources are queried. Use --category, --room, or
--source to restrict the search scope.`,
		Example: `  reno-goat-pp-cli product-search all "floating vanity"
  reno-goat-pp-cli product-search all "pendant light" --room bathroom
  reno-goat-pp-cli product-search all "sofa" --category furniture
  reno-goat-pp-cli product-search all "faucet" --source ferguson,rejuvenation
  reno-goat-pp-cli product-search all "table" --sort price-asc --max-price 500
  reno-goat-pp-cli product-search all "mirror" --json --sort price-desc`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			query := args[0]
			return runFanoutSearch(cmd, flags, query, categoryFlag, roomFlag, sourceFlag, sortFlag, minPrice, maxPrice, perPage)
		},
	}

	cmd.Flags().StringVar(&categoryFlag, "category", "", "Comma-separated categories: foundational, plumbing, electrical, hvac, flooring, hardware, materials, appliances, furniture, decor")
	cmd.Flags().StringVar(&roomFlag, "room", "", "Room shortcut that expands to categories: bathroom, kitchen, bedroom, living, dining, outdoor")
	cmd.Flags().StringVar(&sourceFlag, "source", "", "Comma-separated source names to query (overrides category/room routing)")
	cmd.Flags().StringVar(&sortFlag, "sort", "relevance", "Sort merged results: relevance, price-asc, price-desc, rating")
	cmd.Flags().Float64Var(&minPrice, "min-price", 0, "Minimum price filter (inclusive)")
	cmd.Flags().Float64Var(&maxPrice, "max-price", 0, "Maximum price filter (inclusive, 0 = no limit)")
	cmd.Flags().IntVar(&perPage, "per-page", 20, "Max results per source")

	return cmd
}

// runFanoutSearch is the core fan-out logic, factored out of RunE so it can
// also be wired as the product-search parent's RunE for the bare
// `product-search <query>` invocation.
func runFanoutSearch(cmd *cobra.Command, flags *rootFlags, query, categoryFlag, roomFlag, sourceFlag, sortFlag string, minPrice, maxPrice float64, perPage int) error {
	sourceNames, categories, room, err := resolveSources(categoryFlag, roomFlag, sourceFlag)
	if err != nil {
		return usageErr(err)
	}

	if len(sourceNames) == 0 {
		return usageErr(fmt.Errorf("no active sources match the given filters"))
	}

	stderr := cmd.ErrOrStderr()
	if isTerminal(cmd.OutOrStdout()) {
		fmt.Fprintf(stderr, "Searching %d sources for %q...\n", len(sourceNames), query)
	}

	httpClient := &http.Client{Timeout: flags.timeout}

	// Fan out to all selected sources concurrently.
	type searchResult struct {
		Products []NormalizedProduct
	}

	results, fanoutErrs := cliutil.FanoutRun(
		cmd.Context(),
		sourceNames,
		func(s string) string { return s },
		func(ctx context.Context, sourceName string) (searchResult, error) {
			products, err := searchSource(ctx, httpClient, sourceName, query, perPage)
			if err != nil {
				return searchResult{}, err
			}
			return searchResult{Products: products}, nil
		},
		cliutil.WithConcurrency(len(sourceNames)),
	)

	// Report partial failures on stderr.
	cliutil.FanoutReportErrors(stderr, fanoutErrs)

	// Merge all products.
	var allProducts []NormalizedProduct
	var queriedSources []string
	for _, r := range results {
		allProducts = append(allProducts, r.Value.Products...)
		queriedSources = append(queriedSources, r.Source)
	}
	var failedSources []string
	for _, e := range fanoutErrs {
		failedSources = append(failedSources, e.Source)
	}

	// Apply price filters. When a source only sets PriceMin (single-priced
	// product, no variant range), PriceMax is the zero value. Fall back to
	// PriceMin so single-priced items aren't silently dropped by --min-price.
	if minPrice > 0 || maxPrice > 0 {
		filtered := make([]NormalizedProduct, 0, len(allProducts))
		for _, p := range allProducts {
			hi := p.PriceMax
			if hi == 0 {
				hi = p.PriceMin
			}
			if minPrice > 0 && hi < minPrice {
				continue
			}
			if maxPrice > 0 && p.PriceMin > maxPrice {
				continue
			}
			filtered = append(filtered, p)
		}
		allProducts = filtered
	}

	// Sort merged results.
	sortProducts(allProducts, sortFlag)

	envelope := FanoutResult{
		Query:          query,
		TotalResults:   len(allProducts),
		SourcesQueried: queriedSources,
		SourcesFailed:  failedSources,
		Products:       allProducts,
		Categories:     categories,
		Room:           room,
	}

	// Record search to history (best-effort; ignore errors).
	if histDB, histErr := openNovelDB(); histErr == nil {
		_, _ = histDB.Exec(
			`INSERT INTO search_history (query, categories, sources_queried, result_count) VALUES (?, ?, ?, ?)`,
			query,
			strings.Join(categories, ","),
			strings.Join(queriedSources, ","),
			len(allProducts),
		)
		histDB.Close()
	}

	// Output.
	if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
		return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
	}

	if flags.csv {
		return printFanoutCSV(cmd.OutOrStdout(), envelope)
	}

	return printFanoutTable(cmd.OutOrStdout(), envelope)
}

// sortProducts sorts the product slice in place.
func sortProducts(products []NormalizedProduct, sortFlag string) {
	switch sortFlag {
	case "price-asc":
		sort.Slice(products, func(i, j int) bool {
			return products[i].PriceMin < products[j].PriceMin
		})
	case "price-desc":
		sort.Slice(products, func(i, j int) bool {
			hi := func(p NormalizedProduct) float64 {
				if p.PriceMax > 0 {
					return p.PriceMax
				}
				return p.PriceMin
			}
			return hi(products[i]) > hi(products[j])
		})
	case "rating":
		sort.Slice(products, func(i, j int) bool {
			return products[i].Rating > products[j].Rating
		})
	default:
		// "relevance" — keep per-source ordering, interleave sources.
	}
}

// searchSource dispatches a search query to a single source and normalizes
// the response into []NormalizedProduct.
func searchSource(ctx context.Context, httpClient *http.Client, sourceName, query string, perPage int) ([]NormalizedProduct, error) {
	switch sourceName {
	case "west-elm":
		return searchConstructorIO(ctx, httpClient, query, perPage, "key_SQBuGmXjiXmP0UNI", "west-elm", "https://www.westelm.com")
	case "rejuvenation":
		return searchConstructorIO(ctx, httpClient, query, perPage, "key_9BhS51IOFNhJejk4", "rejuvenation", "https://www.rejuvenation.com")
	case "ferguson":
		return searchFerguson(ctx, httpClient, query, perPage)
	case "article":
		return searchArticle(ctx, httpClient, query, perPage)
	case "shopify-dtc":
		return searchShopifyAll(ctx, httpClient, query, perPage)
	case "ikea":
		return searchIKEA(ctx, httpClient, query, perPage)
	case "ge-appliances":
		return searchGEAppliances(ctx, httpClient, query, perPage)
	case "bray-and-scarff":
		return searchBrayAndScarff(ctx, httpClient, query, perPage)
	case "pc-richard":
		return searchPCRichard(ctx, httpClient, query, perPage)
	case "appliance-factory":
		return searchApplianceFactory(ctx, httpClient, query, perPage)
	case "best-buy":
		return searchBestBuy(ctx, httpClient, query, perPage)
	case "abt":
		return searchAbt(ctx, httpClient, query, perPage)
	case "homewise-appliance":
		return searchHomewiseAppliance(ctx, httpClient, query, perPage)
	case "floor-and-decor":
		return searchFloorAndDecor(ctx, httpClient, query, perPage)
	case "superbrightleds":
		return searchSuperBrightLEDs(ctx, httpClient, query, perPage)
	case "prolighting":
		return searchPROLIGHTING(ctx, httpClient, query, perPage)
	case "1000bulbs":
		return search1000Bulbs(ctx, httpClient, query, perPage)
	case "bees-lighting":
		return searchShopifySuggest(ctx, httpClient, "bees-lighting", "https://www.beeslighting.com", query, perPage)
	case "lighting-new-york":
		return searchLightingNewYork(ctx, httpClient, query, perPage)
	case "lightology":
		return searchLightology(ctx, httpClient, query, perPage)
	case "plumbersstock":
		return searchPlumbersStock(ctx, httpClient, query, perPage)
	case "faucetdepot":
		return searchFaucetDepot(ctx, httpClient, query, perPage)
	case "faucetlist":
		return searchShopifySuggest(ctx, httpClient, "faucetlist", "https://faucetlist.com", query, perPage)
	case "plumbtile":
		return searchShopifySuggest(ctx, httpClient, "plumbtile", "https://plumbtile.com", query, perPage)
	case "modern-bathroom":
		return searchShopifySuggest(ctx, httpClient, "modern-bathroom", "https://www.modernbathroom.com", query, perPage)
	case "kbauthority":
		return searchKBAuthority(ctx, httpClient, query, perPage)
	case "vintage-tub":
		return searchVintageTub(ctx, httpClient, query, perPage)
	case "signature-hardware":
		return searchSignatureHardware(ctx, httpClient, query, perPage)
	case "qualitybath":
		return searchQualityBath(ctx, httpClient, query, perPage)
	case "pioneer-mini-split":
		return searchShopifySuggest(ctx, httpClient, "pioneer-mini-split", "https://www.pioneerminisplit.com", query, perPage)
	case "sylvane":
		return searchShopifySuggest(ctx, httpClient, "sylvane", "https://www.sylvane.com", query, perPage)
	case "iwae":
		return searchIWAe(ctx, httpClient, query, perPage)
	case "hardware-hut":
		return searchHardwareHut(ctx, httpClient, query, perPage)
	default:
		return nil, fmt.Errorf("no search implementation for source %q", sourceName)
	}
}

// ---------- Per-source search implementations ----------

// searchConstructorIO queries the Constructor.io search API used by West Elm
// and Rejuvenation. Both share the same API shape with different API keys.
func searchConstructorIO(ctx context.Context, httpClient *http.Client, query string, perPage int, apiKey, sourceName, siteBaseURL string) ([]NormalizedProduct, error) {
	u, _ := url.Parse("https://ac.cnstrc.com/search/" + url.PathEscape(query))
	q := u.Query()
	q.Set("key", apiKey)
	q.Set("num_results_per_page", fmt.Sprintf("%d", perPage))
	q.Set("page", "1")
	q.Set("i", "ciojs-client-2.77.1")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", sourceName, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%s: reading body: %w", sourceName, err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("%s: HTTP %d: %s", sourceName, resp.StatusCode, truncate(string(body), 200))
	}

	// Constructor.io shape: { "response": { "results": [ { "data": {...}, "value": "..." } ] } }
	var cioResp struct {
		Response struct {
			Results []struct {
				Data  map[string]any `json:"data"`
				Value string         `json:"value"`
			} `json:"results"`
		} `json:"response"`
	}
	if err := json.Unmarshal(body, &cioResp); err != nil {
		return nil, fmt.Errorf("%s: parsing response: %w", sourceName, err)
	}

	products := make([]NormalizedProduct, 0, len(cioResp.Response.Results))
	for _, r := range cioResp.Response.Results {
		p := normalizeConstructorIO(r.Data, r.Value, sourceName, siteBaseURL)
		products = append(products, p)
	}
	return products, nil
}

func normalizeConstructorIO(data map[string]any, value, sourceName, siteBaseURL string) NormalizedProduct {
	p := NormalizedProduct{
		Source: sourceName,
		Title:  value,
	}
	if id, ok := data["id"].(string); ok {
		p.ID = id
	}
	if brand, ok := data["brand"].(string); ok {
		p.Brand = brand
	}
	if imgURL, ok := data["image_url"].(string); ok {
		p.ImageURL = imgURL
	}
	if desc, ok := data["description"].(string); ok {
		p.Description = desc
	}
	if productURL, ok := data["url"].(string); ok {
		if strings.HasPrefix(productURL, "/") {
			p.URL = siteBaseURL + productURL
		} else {
			p.URL = productURL
		}
	}

	// Price extraction — Constructor.io uses camelCase field names:
	// lowestPrice, highestPrice, regularPriceMin, regularPriceMax, salePriceMin, salePriceMax
	p.PriceMin = jsonFloat(data, "lowestPrice", "min_price", "price")
	p.PriceMax = jsonFloat(data, "highestPrice", "max_price", "price")
	if rp := jsonFloat(data, "regularPriceMin", "min_regular_price", "regular_price"); rp > 0 {
		p.RegularPriceMin = rp
	}
	if rp := jsonFloat(data, "regularPriceMax", "max_regular_price", "regular_price"); rp > 0 {
		p.RegularPriceMax = rp
	}
	if sp := jsonFloat(data, "salePriceMin", "min_sale_price", "sale_price"); sp > 0 {
		p.SalePriceMin = sp
		p.OnSale = true
	}
	if sp := jsonFloat(data, "salePriceMax", "max_sale_price", "sale_price"); sp > 0 {
		p.SalePriceMax = sp
	}
	if p.OnSale && p.RegularPriceMin > 0 && p.SalePriceMin > 0 {
		p.DiscountPercent = (1 - p.SalePriceMin/p.RegularPriceMin) * 100
	}
	if rating := jsonFloat(data, "rating", "review_rating"); rating > 0 {
		p.Rating = rating
	}
	if count := jsonInt(data, "review_count", "num_reviews"); count > 0 {
		p.ReviewCount = count
	}

	return p
}

// searchFerguson queries Ferguson's GraphQL product search.
func searchFerguson(ctx context.Context, httpClient *http.Client, query string, perPage int) ([]NormalizedProduct, error) {
	gqlQuery := `query ProductSearch($query: String!, $first: Int, $offset: Int) {
		productSearch(query: $query, first: $first, offset: $offset) {
			totalNumRecs
			products {
				id
				title
				brand
				url
				imageUrl
				minPrice
				maxPrice
				regularMinPrice
				regularMaxPrice
				saleMinPrice
				saleMaxPrice
				onSale
				rating
				reviewCount
			}
		}
	}`

	payload := map[string]any{
		"query": gqlQuery,
		"variables": map[string]any{
			"query":  query,
			"first":  perPage,
			"offset": 0,
		},
	}
	bodyBytes, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST", "https://www.fergusonhome.com/graphql/ProductSearch", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("x-fergy-client-name", "react-build-store")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ferguson: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ferguson: reading body: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("ferguson: HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}

	// Ferguson GraphQL response shape.
	var gqlResp struct {
		Data struct {
			ProductSearch struct {
				Products []map[string]any `json:"products"`
			} `json:"productSearch"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(body, &gqlResp); err != nil {
		return nil, fmt.Errorf("ferguson: parsing response: %w", err)
	}
	if len(gqlResp.Errors) > 0 {
		return nil, fmt.Errorf("ferguson: GraphQL error: %s", gqlResp.Errors[0].Message)
	}

	products := make([]NormalizedProduct, 0, len(gqlResp.Data.ProductSearch.Products))
	for _, item := range gqlResp.Data.ProductSearch.Products {
		p := normalizeFerguson(item)
		products = append(products, p)
	}
	return products, nil
}

func normalizeFerguson(item map[string]any) NormalizedProduct {
	p := NormalizedProduct{Source: "ferguson"}

	if id, ok := item["id"].(string); ok {
		p.ID = id
	}
	if title, ok := item["title"].(string); ok {
		p.Title = title
	}
	if brand, ok := item["brand"].(string); ok {
		p.Brand = brand
	}
	if u, ok := item["url"].(string); ok {
		if strings.HasPrefix(u, "/") {
			p.URL = "https://www.fergusonhome.com" + u
		} else {
			p.URL = u
		}
	}
	if img, ok := item["imageUrl"].(string); ok {
		p.ImageURL = img
	}

	p.PriceMin = jsonFloat(item, "minPrice")
	p.PriceMax = jsonFloat(item, "maxPrice")
	p.RegularPriceMin = jsonFloat(item, "regularMinPrice")
	p.RegularPriceMax = jsonFloat(item, "regularMaxPrice")
	p.SalePriceMin = jsonFloat(item, "saleMinPrice")
	p.SalePriceMax = jsonFloat(item, "saleMaxPrice")
	if onSale, ok := item["onSale"].(bool); ok {
		p.OnSale = onSale
	}
	if p.OnSale && p.RegularPriceMin > 0 && p.SalePriceMin > 0 {
		p.DiscountPercent = (1 - p.SalePriceMin/p.RegularPriceMin) * 100
	}
	p.Rating = jsonFloat(item, "rating")
	p.ReviewCount = jsonInt(item, "reviewCount")

	return p
}

// searchArticle queries Article's APQ GraphQL search.
func searchArticle(ctx context.Context, httpClient *http.Client, query string, perPage int) ([]NormalizedProduct, error) {
	// Article uses Apollo Persisted Queries with GET requests.
	u, _ := url.Parse("https://www.article.com/graphql")
	q := u.Query()

	// Build the variables and extensions for the APQ.
	variables := map[string]any{
		"query":    query,
		"pageSize": perPage,
		"page":     1,
	}
	varsJSON, _ := json.Marshal(variables)
	q.Set("variables", string(varsJSON))

	extensions := map[string]any{
		"persistedQuery": map[string]any{
			"version":    1,
			"sha256Hash": "SEARCH_PRODUCTS",
		},
	}
	extJSON, _ := json.Marshal(extensions)
	q.Set("extensions", string(extJSON))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("article: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("article: reading body: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("article: HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}

	// Article APQ response shape — may vary, parse generically.
	var gqlResp struct {
		Data struct {
			SearchProducts struct {
				Products []map[string]any `json:"products"`
				Items    []map[string]any `json:"items"`
				Results  []map[string]any `json:"results"`
			} `json:"searchProducts"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(body, &gqlResp); err != nil {
		return nil, fmt.Errorf("article: parsing response: %w", err)
	}
	if len(gqlResp.Errors) > 0 {
		return nil, fmt.Errorf("article: GraphQL error: %s", gqlResp.Errors[0].Message)
	}

	// Grab whichever array field was populated.
	items := gqlResp.Data.SearchProducts.Products
	if len(items) == 0 {
		items = gqlResp.Data.SearchProducts.Items
	}
	if len(items) == 0 {
		items = gqlResp.Data.SearchProducts.Results
	}

	products := make([]NormalizedProduct, 0, len(items))
	for _, item := range items {
		p := normalizeArticle(item)
		products = append(products, p)
	}
	return products, nil
}

func normalizeArticle(item map[string]any) NormalizedProduct {
	p := NormalizedProduct{Source: "article"}

	if id, ok := item["id"].(string); ok {
		p.ID = id
	} else if id, ok := item["sku"].(string); ok {
		p.ID = id
	}
	if title, ok := item["name"].(string); ok {
		p.Title = title
	} else if title, ok := item["title"].(string); ok {
		p.Title = title
	}
	if brand, ok := item["brand"].(string); ok {
		p.Brand = brand
	}
	if u, ok := item["url"].(string); ok {
		if strings.HasPrefix(u, "/") {
			p.URL = "https://www.article.com" + u
		} else {
			p.URL = u
		}
	} else if slug, ok := item["slug"].(string); ok {
		p.URL = "https://www.article.com/product/" + slug
	}
	if img, ok := item["imageUrl"].(string); ok {
		p.ImageURL = img
	} else if img, ok := item["image"].(string); ok {
		p.ImageURL = img
	}

	p.PriceMin = jsonFloat(item, "price", "minPrice")
	p.PriceMax = jsonFloat(item, "maxPrice", "price")
	p.RegularPriceMin = jsonFloat(item, "regularPrice", "comparePrice")
	p.SalePriceMin = jsonFloat(item, "salePrice")
	if p.SalePriceMin > 0 && p.RegularPriceMin > 0 {
		p.OnSale = true
		p.DiscountPercent = (1 - p.SalePriceMin/p.RegularPriceMin) * 100
	}
	p.Rating = jsonFloat(item, "rating", "averageRating")
	p.ReviewCount = jsonInt(item, "reviewCount", "numReviews")

	return p
}

// searchShopifyAll fans out to all Shopify DTC stores concurrently.
func searchShopifyAll(ctx context.Context, httpClient *http.Client, query string, perPage int) ([]NormalizedProduct, error) {
	results, errs := cliutil.FanoutRun(
		ctx,
		shopifyStores,
		func(s shopifyStore) string { return s.Domain },
		func(ctx context.Context, store shopifyStore) ([]NormalizedProduct, error) {
			return searchShopifyStore(ctx, httpClient, store, query, perPage)
		},
		cliutil.WithConcurrency(len(shopifyStores)),
	)

	var all []NormalizedProduct
	for _, r := range results {
		all = append(all, r.Value...)
	}

	// If all stores failed, return the first error.
	if len(results) == 0 && len(errs) > 0 {
		return nil, fmt.Errorf("shopify: all %d stores failed (first: %s)", len(errs), shortFanoutErrMsg(errs[0].Err))
	}

	// Partial failures: report on stderr but don't fail the overall search.
	for _, e := range errs {
		fmt.Fprintf(os.Stderr, "warn: shopify/%s: %s\n", e.Source, shortFanoutErrMsg(e.Err))
	}

	return all, nil
}

func searchShopifyStore(ctx context.Context, httpClient *http.Client, store shopifyStore, query string, perPage int) ([]NormalizedProduct, error) {
	gqlQuery := fmt.Sprintf(`{
		search(query: %q, first: %d, types: PRODUCT) {
			edges {
				node {
					... on Product {
						id
						title
						handle
						vendor
						description
						images(first: 1) { edges { node { url } } }
						priceRange {
							minVariantPrice { amount currencyCode }
							maxVariantPrice { amount currencyCode }
						}
						compareAtPriceRange {
							minVariantPrice { amount }
							maxVariantPrice { amount }
						}
					}
				}
			}
		}
	}`, query, perPage)

	payload := map[string]string{"query": gqlQuery}
	bodyBytes, _ := json.Marshal(payload)

	apiURL := fmt.Sprintf("https://%s.myshopify.com/api/2025-01/graphql.json", store.Domain)
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Shopify-Storefront-Access-Token", store.Token)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("shopify/%s: %w", store.Domain, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("shopify/%s: reading body: %w", store.Domain, err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("shopify/%s: HTTP %d: %s", store.Domain, resp.StatusCode, truncate(string(body), 200))
	}

	var gqlResp struct {
		Data struct {
			Search struct {
				Edges []struct {
					Node map[string]any `json:"node"`
				} `json:"edges"`
			} `json:"search"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(body, &gqlResp); err != nil {
		return nil, fmt.Errorf("shopify/%s: parsing: %w", store.Domain, err)
	}
	if len(gqlResp.Errors) > 0 {
		return nil, fmt.Errorf("shopify/%s: %s", store.Domain, gqlResp.Errors[0].Message)
	}

	products := make([]NormalizedProduct, 0, len(gqlResp.Data.Search.Edges))
	for _, edge := range gqlResp.Data.Search.Edges {
		p := normalizeShopify(edge.Node, store)
		products = append(products, p)
	}
	return products, nil
}

func normalizeShopify(node map[string]any, store shopifyStore) NormalizedProduct {
	p := NormalizedProduct{
		Source: "shopify-dtc/" + store.Domain,
		Brand:  store.Name,
	}

	if id, ok := node["id"].(string); ok {
		p.ID = id
	}
	if title, ok := node["title"].(string); ok {
		p.Title = title
	}
	if vendor, ok := node["vendor"].(string); ok && vendor != "" {
		p.Brand = vendor
	}
	if handle, ok := node["handle"].(string); ok {
		p.URL = fmt.Sprintf("https://%s.myshopify.com/products/%s", store.Domain, handle)
	}
	if desc, ok := node["description"].(string); ok {
		p.Description = desc
	}

	// Extract image URL from images.edges[0].node.url
	if images, ok := node["images"].(map[string]any); ok {
		if edges, ok := images["edges"].([]any); ok && len(edges) > 0 {
			if edge, ok := edges[0].(map[string]any); ok {
				if imgNode, ok := edge["node"].(map[string]any); ok {
					if imgURL, ok := imgNode["url"].(string); ok {
						p.ImageURL = imgURL
					}
				}
			}
		}
	}

	// Extract price range.
	p.PriceMin = extractShopifyPrice(node, "priceRange", "minVariantPrice")
	p.PriceMax = extractShopifyPrice(node, "priceRange", "maxVariantPrice")
	compareMin := extractShopifyPrice(node, "compareAtPriceRange", "minVariantPrice")
	compareMax := extractShopifyPrice(node, "compareAtPriceRange", "maxVariantPrice")
	if compareMin > 0 {
		p.RegularPriceMin = compareMin
		p.RegularPriceMax = compareMax
		if p.PriceMin < compareMin {
			p.OnSale = true
			p.SalePriceMin = p.PriceMin
			p.SalePriceMax = p.PriceMax
			p.DiscountPercent = (1 - p.SalePriceMin/p.RegularPriceMin) * 100
		}
	}

	return p
}

// searchShopifySuggest queries Shopify's unauthenticated suggest endpoint for
// stores that expose product rows without a Storefront API token. It is used
// only for sources verified by Printing Press reachability and direct replay.
func searchShopifySuggest(ctx context.Context, httpClient *http.Client, sourceName, siteBaseURL, query string, perPage int) ([]NormalizedProduct, error) {
	if perPage <= 0 {
		perPage = 20
	}
	u, _ := url.Parse(strings.TrimRight(siteBaseURL, "/") + "/search/suggest.json")
	q := u.Query()
	q.Set("q", query)
	q.Set("resources[type]", "product")
	q.Set("resources[limit]", fmt.Sprintf("%d", maxInt(perPage*5, 10)))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", sourceName, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%s: reading body: %w", sourceName, err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("%s: HTTP %d: %s", sourceName, resp.StatusCode, truncate(string(body), 200))
	}

	var suggestResp struct {
		Resources struct {
			Results struct {
				Products []map[string]any `json:"products"`
			} `json:"results"`
		} `json:"resources"`
	}
	if err := json.Unmarshal(body, &suggestResp); err != nil {
		return nil, fmt.Errorf("%s: parsing response: %w", sourceName, err)
	}

	preferred := make([]NormalizedProduct, 0, len(suggestResp.Resources.Results.Products))
	fallback := make([]NormalizedProduct, 0, len(suggestResp.Resources.Results.Products))
	for _, raw := range suggestResp.Resources.Results.Products {
		if available, ok := raw["available"].(bool); ok && !available {
			continue
		}
		p := normalizeShopifySuggestProduct(raw, sourceName, siteBaseURL)
		if p.ID == "" || p.Title == "" {
			continue
		}
		if skipShopifySuggestProduct(sourceName, query, p) {
			continue
		}
		if preferShopifySuggestProduct(sourceName, query, p) {
			preferred = append(preferred, p)
		} else {
			fallback = append(fallback, p)
		}
	}

	products := append(preferred, fallback...)
	if len(products) > perPage {
		products = products[:perPage]
	}
	return products, nil
}

func normalizeShopifySuggestProduct(raw map[string]any, sourceName, siteBaseURL string) NormalizedProduct {
	p := NormalizedProduct{Source: sourceName}
	if handle, ok := raw["handle"].(string); ok && sourcePrefersShopifyHandleID(sourceName) {
		p.ID = preferredShopifyHandleIdentifier(sourceName, handle)
	}
	if sku := shopifySuggestProductIdentifier(raw); sku != "" && p.ID == "" {
		p.ID = sku
	}
	if id := jsonInt(raw, "id"); id > 0 && p.ID == "" {
		p.ID = fmt.Sprintf("%d", id)
	}
	if handle, ok := raw["handle"].(string); ok && p.ID == "" {
		p.ID = handle
	}
	if title, ok := raw["title"].(string); ok {
		p.Title = html.UnescapeString(title)
	}
	if vendor, ok := raw["vendor"].(string); ok {
		p.Brand = vendor
	}
	if productType, ok := raw["type"].(string); ok {
		p.Category = productType
	}
	if desc, ok := raw["body"].(string); ok {
		p.Description = shopifySuggestDescription(desc, siteBaseURL)
	}
	if productURL, ok := raw["url"].(string); ok {
		productURL = html.UnescapeString(productURL)
		if strings.HasPrefix(productURL, "http://") || strings.HasPrefix(productURL, "https://") {
			p.URL = productURL
		} else if strings.HasPrefix(productURL, "/") {
			p.URL = strings.TrimRight(siteBaseURL, "/") + productURL
		}
	}
	if img, ok := raw["image"].(string); ok && strings.HasPrefix(img, "http") {
		p.ImageURL = html.UnescapeString(img)
	} else if featured, ok := raw["featured_image"].(map[string]any); ok {
		if img, ok := featured["url"].(string); ok && strings.HasPrefix(img, "http") {
			p.ImageURL = html.UnescapeString(img)
		}
	}

	p.PriceMin = jsonFloat(raw, "price_min", "price")
	p.PriceMax = jsonFloat(raw, "price_max", "price")
	p.RegularPriceMin = jsonFloat(raw, "compare_at_price_min")
	p.RegularPriceMax = jsonFloat(raw, "compare_at_price_max")
	if p.RegularPriceMin > 0 && p.PriceMin > 0 && p.PriceMin < p.RegularPriceMin {
		p.OnSale = true
		p.SalePriceMin = p.PriceMin
		p.SalePriceMax = p.PriceMax
		p.DiscountPercent = (1 - p.PriceMin/p.RegularPriceMin) * 100
	}
	return p
}

func shopifySuggestProductIdentifier(raw map[string]any) string {
	if variants, ok := raw["variants"].([]any); ok {
		for _, item := range variants {
			variant, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if sku, ok := variant["sku"].(string); ok {
				if normalized := normalizeSelectionSKU(sku); normalized != "" {
					return normalized
				}
			}
		}
	}
	if body, ok := raw["body"].(string); ok {
		if sku := shopifySuggestBodyIdentifier(body); sku != "" {
			return sku
		}
	}
	if title, ok := raw["title"].(string); ok {
		if model := extractBestModel(html.UnescapeString(title)); model != "" {
			return model
		}
	}
	if handle, ok := raw["handle"].(string); ok {
		if model := shopifySuggestHandleIdentifier(handle); model != "" {
			return model
		}
		if model := extractBestModel(handle); model != "" {
			return model
		}
	}
	return ""
}

func shopifySuggestHandleIdentifier(handle string) string {
	parts := strings.FieldsFunc(strings.ToUpper(handle), func(r rune) bool {
		return r == '-' || r == '_' || r == '/' || r == ' '
	})
	if len(parts) == 0 {
		return ""
	}
	for i := 0; i < len(parts); i++ {
		if shopifyHandlePartLooksLikeCompactSKU(parts[i]) {
			return normalizeSelectionSKU(parts[i])
		}
	}
	if model := shopifySuggestMultipartHandleIdentifier(parts); model != "" {
		return model
	}
	for n := 4; n >= 2; n-- {
		for i := 0; i+n <= len(parts); i++ {
			candidateParts := parts[i : i+n]
			if !shopifyHandlePartsLookLikeSKU(candidateParts) {
				continue
			}
			candidate := normalizeSelectionSKU(strings.Join(candidateParts, "-"))
			if candidate != "" {
				return candidate
			}
		}
	}
	return ""
}

func shopifySuggestMultipartHandleIdentifier(parts []string) string {
	for i := 0; i+1 < len(parts); i++ {
		first := parts[i]
		if shopifyHandlePartIsDescriptor(first) || !regexp.MustCompile(`^[A-Z]{2,12}$`).MatchString(first) {
			continue
		}
		if !regexp.MustCompile(`^[0-9]{2,}$`).MatchString(parts[i+1]) {
			continue
		}
		candidate := []string{first, parts[i+1]}
		for j := i + 2; j < len(parts); j++ {
			if j+1 < len(parts) && shopifyHandlePartIsDimensionUnit(parts[j+1]) {
				break
			}
			if !regexp.MustCompile(`^[0-9]{2,}$`).MatchString(parts[j]) {
				break
			}
			candidate = append(candidate, parts[j])
		}
		if sku := normalizeSelectionSKU(strings.Join(candidate, "-")); sku != "" {
			return sku
		}
	}
	return ""
}

func shopifyHandlePartsLookLikeSKU(parts []string) bool {
	if len(parts) == 0 {
		return false
	}
	joined := strings.Join(parts, "")
	if len(joined) < 4 || len(joined) > 40 || !strings.ContainsAny(joined, "0123456789") {
		return false
	}
	first := parts[0]
	if regexp.MustCompile(`^[0-9]+$`).MatchString(first) {
		return false
	}
	for _, part := range parts {
		if shopifyHandlePartIsDescriptor(part) {
			return false
		}
	}
	return regexp.MustCompile(`^[A-Z0-9]+$`).MatchString(first)
}

func shopifyHandlePartLooksLikeCompactSKU(part string) bool {
	if shopifyHandlePartIsDescriptor(part) {
		return false
	}
	return regexp.MustCompile(`^[A-Z]{2,}[A-Z0-9]*[0-9][A-Z0-9]*$|^[A-Z]+[0-9]{3,}[A-Z0-9]*$`).MatchString(part)
}

func shopifyHandlePartIsDescriptor(part string) bool {
	return regexp.MustCompile(`^(LED|LIGHT|LIGHTING|LAMP|BULB|RECESSED|UNDER|CABINET|SMART|CANLESS|GIMBAL|BLACK|WHITE|SATCO|NUVO|RAB|AMERICAN|SYLVANIA|LEDVANCE|STARFISH|ECLIPSE|RETROFIT|DOWNLIGHT|INCH)$`).MatchString(part)
}

func shopifyHandlePartIsDimensionUnit(part string) bool {
	return regexp.MustCompile(`^(IN|INCH|INCHES|FT|FOOT|FEET|MM|CM|M)$`).MatchString(part)
}

func shopifySuggestBodyIdentifier(body string) string {
	text := stripHTMLTags(html.UnescapeString(body))
	for _, pattern := range []string{
		`(?i)\bSKU:\s*([^<\n\r]+)`,
		`(?i)\bMPN:\s*([^<\n\r]+)`,
		`(?i)\bManufacturer'?s Part Number\(s\):\s*([^<\n\r]+)`,
	} {
		re := regexp.MustCompile(pattern)
		if match := re.FindStringSubmatch(text); len(match) > 1 {
			if sku := firstSelectionSKUToken(match[1]); sku != "" {
				return sku
			}
		}
	}
	return ""
}

func firstSelectionSKUToken(s string) string {
	s = strings.ReplaceAll(s, "\u00a0", " ")
	s = regexp.MustCompile(`\([^)]*\)`).ReplaceAllString(s, "")
	for _, token := range regexp.MustCompile(`[A-Za-z0-9][A-Za-z0-9._#/-]{2,}`).FindAllString(s, -1) {
		token = strings.Trim(token, " ,;:'\"")
		if token == "" || !strings.ContainsAny(token, "0123456789") {
			continue
		}
		switch strings.ToUpper(token) {
		case "UPC", "EAN", "ISBN":
			continue
		}
		if sku := normalizeSelectionSKU(token); sku != "" {
			return sku
		}
	}
	return ""
}

func shopifySuggestDescription(body, siteBaseURL string) string {
	text := strings.TrimSpace(stripHTMLTags(html.UnescapeString(body)))
	parts := make([]string, 0, 4)
	if text != "" {
		parts = append(parts, text)
	}

	seen := map[string]bool{}
	for _, raw := range regexp.MustCompile(`(?i)(?:https?://|/)[^"'\\\s<>]+\.pdf(?:\?[^"'\\\s<>]*)?`).FindAllString(html.UnescapeString(body), -1) {
		pdf := absoluteURL(siteBaseURL, strings.Trim(raw, " ,;:'\""))
		if pdf == "" || seen[pdf] {
			continue
		}
		seen[pdf] = true
		parts = append(parts, "spec: "+pdf)
	}
	return strings.Join(parts, "\n")
}

func skipShopifySuggestProduct(sourceName, query string, p NormalizedProduct) bool {
	category := strings.ToLower(p.Category)
	title := strings.ToLower(p.Title)
	q := strings.ToLower(query)
	if sourceName == "bees-lighting" && strings.Contains(q, "light") {
		return strings.Contains(category, "power strip")
	}
	if sourceName == "modern-bathroom" {
		if strings.Contains(q, "vanity") {
			return category == "countertops" || strings.Contains(title, "sidesplash") || strings.Contains(title, "side splash")
		}
		if strings.Contains(q, "shower") || strings.Contains(q, "valve") || strings.Contains(q, "trim") {
			return false
		}
		return category == "parts"
	}
	return false
}

func sourcePrefersShopifyHandleID(sourceName string) bool {
	switch sourceName {
	case "modern-bathroom":
		return true
	default:
		return false
	}
}

func preferredShopifyHandleIdentifier(sourceName, handle string) string {
	switch sourceName {
	case "modern-bathroom":
		return modernBathroomSelectionID(handle)
	default:
		return normalizeSelectionSKU(handle)
	}
}

func modernBathroomSelectionID(handle string) string {
	parts := strings.FieldsFunc(strings.ToUpper(handle), func(r rune) bool {
		return !((r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'))
	})
	if len(parts) == 0 {
		return ""
	}

	collection := ""
	for _, part := range parts {
		if modernBathroomHandleDescriptor(part) || regexp.MustCompile(`^[0-9]+$`).MatchString(part) {
			continue
		}
		collection = part
		break
	}
	if collection == "" {
		return normalizeSelectionSKU(handle)
	}

	var size, sink, hole string
	for i, part := range parts {
		if i+1 < len(parts) && regexp.MustCompile(`^[0-9]{2,3}$`).MatchString(part) && shopifyHandlePartIsDimensionUnit(parts[i+1]) {
			size = part
		}
		if i+1 < len(parts) && (part == "SINGLE" || part == "DOUBLE") && parts[i+1] == "SINK" {
			sink = part + "-SINK"
		}
		if i+1 < len(parts) && (part == "SINGLE" || part == "DOUBLE" || part == "3" || part == "THREE") && parts[i+1] == "HOLE" {
			if part == "THREE" {
				part = "3"
			}
			hole = part + "-HOLE"
		}
	}

	idParts := []string{collection, "VANITY"}
	if size != "" {
		idParts = append(idParts, size)
	}
	if sink != "" {
		idParts = append(idParts, sink)
	}
	if hole != "" {
		idParts = append(idParts, hole)
	}
	return normalizeSelectionSKU(strings.Join(idParts, "-"))
}

func modernBathroomHandleDescriptor(part string) bool {
	return regexp.MustCompile(`^(BATHROOM|VANITY|CABINET|WITH|COUNTERTOP|TOP|INCH|IN|FAUCET|SETUP|DOORS|DRAWERS|BASE|WALL|MOUNT|FREESTANDING)$`).MatchString(part)
}

func preferShopifySuggestProduct(sourceName, query string, p NormalizedProduct) bool {
	title := strings.ToLower(p.Title)
	category := strings.ToLower(p.Category)
	q := strings.ToLower(query)
	if sourceName == "pioneer-mini-split" {
		if category == "acc" {
			return false
		}
		return strings.Contains(title, "btu") || strings.Contains(title, "heat pump") || strings.Contains(title, "ductless")
	}
	if sourceName == "sylvane" && (strings.Contains(q, "mini split") || strings.Contains(q, "heat pump")) {
		return strings.Contains(title, "btu") || strings.Contains(title, "heat pump")
	}
	return true
}

// searchIKEA queries IKEA's SIK search endpoint observed from the public
// search page. The endpoint is anonymous standard HTTP and returns product
// cards plus category metadata.
func searchIKEA(ctx context.Context, httpClient *http.Client, query string, perPage int) ([]NormalizedProduct, error) {
	if perPage <= 0 {
		perPage = 20
	}
	payload := map[string]any{
		"searchParameters": map[string]any{
			"input": query,
			"type":  "QUERY",
		},
		"components": []map[string]any{
			{
				"component": "PRIMARY_AREA",
				"types": map[string]any{
					"main":      "PRODUCT",
					"breakouts": []string{},
				},
				"filterConfig": map[string]string{"subcategories-style": "tree-navigation"},
				"window":       map[string]int{"size": perPage, "offset": 0},
			},
		},
	}
	bodyBytes, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST", "https://sik.search.blue.cdtapps.com/us/en/search?c=sr&v=20250507", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ikea: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ikea: reading body: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("ikea: HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}

	var sikResp struct {
		Results []struct {
			Component string `json:"component"`
			Items     []struct {
				Type    string         `json:"type"`
				Product map[string]any `json:"product"`
			} `json:"items"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &sikResp); err != nil {
		return nil, fmt.Errorf("ikea: parsing response: %w", err)
	}

	var products []NormalizedProduct
	for _, result := range sikResp.Results {
		if result.Component != "PRIMARY_AREA" {
			continue
		}
		for _, item := range result.Items {
			if item.Type != "PRODUCT" || item.Product == nil {
				continue
			}
			products = append(products, normalizeIKEA(item.Product))
		}
	}
	return products, nil
}

func normalizeIKEA(item map[string]any) NormalizedProduct {
	p := NormalizedProduct{Source: "ikea"}
	if id, ok := item["itemNo"].(string); ok {
		p.ID = id
	} else if id, ok := item["id"].(string); ok {
		p.ID = id
	}
	if name, ok := item["name"].(string); ok {
		p.Title = name
	}
	if typeName, ok := item["typeName"].(string); ok && typeName != "" {
		if p.Title == "" {
			p.Title = typeName
		} else {
			p.Title += " " + typeName
		}
	}
	if u, ok := item["pipUrl"].(string); ok {
		p.URL = u
	}
	if img, ok := item["mainImageUrl"].(string); ok {
		p.ImageURL = img
	} else if img, ok := item["imageUrl"].(string); ok {
		p.ImageURL = img
	}
	if desc, ok := item["validDesignText"].(string); ok {
		p.Description = desc
	}
	if price, ok := item["salesPrice"].(map[string]any); ok {
		p.PriceMin = jsonFloat(price, "numeral")
		p.PriceMax = p.PriceMin
	}
	p.Rating = jsonFloat(item, "ratingValue")
	p.ReviewCount = jsonInt(item, "ratingCount")
	if path, ok := item["categoryPath"].([]any); ok {
		var parts []string
		for _, raw := range path {
			node, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			if name, ok := node["name"].(string); ok && name != "" {
				parts = append(parts, name)
			}
		}
		p.Category = strings.Join(parts, " > ")
	}
	return p
}

// searchGEAppliances queries the public Searchspring catalog API exposed by
// GE Appliances category pages. The endpoint returns appliance rows with
// model/SKU, price/MSRP, ratings, images, specs, and install/spec documents.
func searchGEAppliances(ctx context.Context, httpClient *http.Client, query string, perPage int) ([]NormalizedProduct, error) {
	if perPage <= 0 {
		perPage = 20
	}
	u, _ := url.Parse("https://q7rntw.a.searchspring.io/api/search/search.json")
	q := u.Query()
	q.Set("siteId", "q7rntw")
	q.Set("q", query)
	q.Set("resultsFormat", "native")
	q.Set("resultsPerPage", fmt.Sprintf("%d", perPage))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ge-appliances: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ge-appliances: reading body: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("ge-appliances: HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}

	var searchResp struct {
		Results []map[string]any `json:"results"`
	}
	if err := json.Unmarshal(body, &searchResp); err != nil {
		return nil, fmt.Errorf("ge-appliances: parsing response: %w", err)
	}

	products := make([]NormalizedProduct, 0, len(searchResp.Results))
	for _, raw := range searchResp.Results {
		p := normalizeGEAppliance(raw)
		if p.ID != "" && p.Title != "" {
			products = append(products, p)
		}
	}
	return products, nil
}

func normalizeGEAppliance(raw map[string]any) NormalizedProduct {
	p := NormalizedProduct{Source: "ge-appliances"}
	if sku, ok := raw["sku"].(string); ok {
		p.ID = sku
	}
	if p.ID == "" {
		if uid, ok := raw["uid"].(string); ok {
			p.ID = uid
		} else if id, ok := raw["id"].(string); ok {
			p.ID = id
		}
	}
	if title, ok := raw["name"].(string); ok {
		p.Title = html.UnescapeString(title)
	}
	if brand, ok := raw["brand"].(string); ok {
		p.Brand = html.UnescapeString(brand)
	}
	if productURL, ok := raw["ss_url"].(string); ok && productURL != "" {
		p.URL = html.UnescapeString(productURL)
	} else if productURL, ok := raw["url"].(string); ok && productURL != "" {
		p.URL = html.UnescapeString(productURL)
	}
	if imageURL, ok := raw["imageUrl"].(string); ok && imageURL != "" {
		p.ImageURL = html.UnescapeString(imageURL)
	} else if imageURL, ok := raw["thumbnailImageUrl"].(string); ok && imageURL != "" {
		p.ImageURL = html.UnescapeString(imageURL)
	}
	p.PriceMin = jsonFloat(raw, "price")
	p.PriceMax = p.PriceMin
	p.RegularPriceMin = jsonFloat(raw, "msrp")
	p.RegularPriceMax = p.RegularPriceMin
	if p.RegularPriceMin > 0 && p.PriceMin > 0 && p.PriceMin < p.RegularPriceMin {
		p.OnSale = true
		p.SalePriceMin = p.PriceMin
		p.SalePriceMax = p.PriceMax
		p.DiscountPercent = (1 - p.PriceMin/p.RegularPriceMin) * 100
	}
	p.Rating = jsonFloat(raw, "rating", "bazaarvoice_overall_rating")
	p.ReviewCount = jsonInt(raw, "ratingCount", "bazaarvoice_total_reviews")
	p.Category = geApplianceCategory(raw)
	p.Description = geApplianceDescription(raw)
	return p
}

func geApplianceCategory(raw map[string]any) string {
	if productType, ok := raw["spec_features_product_type"].(string); ok && productType != "" {
		return html.UnescapeString(productType)
	}
	if ssType, ok := raw["ss_type"].([]any); ok && len(ssType) > 0 {
		if first, ok := ssType[0].(string); ok {
			return html.UnescapeString(first)
		}
	}
	if productType, ok := raw["product_type_unigram"].(string); ok && productType != "" {
		return html.UnescapeString(productType)
	}
	if cats, ok := raw["categories_hierarchy"].([]any); ok && len(cats) > 0 {
		if last, ok := cats[len(cats)-1].(string); ok {
			return strings.ReplaceAll(html.UnescapeString(last), ">", " > ")
		}
	}
	return ""
}

func geApplianceDescription(raw map[string]any) string {
	var parts []string
	for _, key := range []string{
		"productdimensions",
		"spec_appearance_color_appearance",
		"spec_features_style",
		"spec_features_control_type",
		"spec_features_wifi_connect",
		"spec_features_exhaust_options",
		"documents_installation_instructions",
		"documents_quick_specs",
		"documents_energy_guide",
	} {
		if val, ok := raw[key].(string); ok && val != "" {
			parts = append(parts, fmt.Sprintf("%s: %s", strings.TrimPrefix(key, "spec_"), html.UnescapeString(val)))
		}
	}
	if benefits, ok := raw["ss_benefits"].([]any); ok && len(benefits) > 0 {
		var vals []string
		for _, benefit := range benefits {
			if s, ok := benefit.(string); ok && s != "" {
				vals = append(vals, html.UnescapeString(s))
			}
		}
		if len(vals) > 0 {
			parts = append(parts, "benefits: "+strings.Join(vals, "; "))
		}
	}
	return strings.Join(parts, "\n")
}

const (
	applianceFactoryBaseURL       = "https://www.appliancefactory.com/api/rest"
	applianceFactoryPXAccessToken = "nS7L8keRNNOBgtjB3vaDOhHKcLkpVS6SqejiwFtT2zDZYOpxIldCDsQK7sDMXHO5"
	homewiseApplianceBaseURL      = "https://0qcofybyvd.execute-api.us-west-1.amazonaws.com/hw-prod"
)

// searchApplianceFactory replays the AVB/LINQ REST API exposed by Appliance
// Factory's Next.js storefront. The API returns 403 over negotiated HTTP/2, so
// this source uses an HTTP/1.1-only transport while preserving the caller's
// timeout.
func searchApplianceFactory(ctx context.Context, httpClient *http.Client, query string, perPage int) ([]NormalizedProduct, error) {
	if perPage <= 0 {
		perPage = 20
	}
	body, err := applianceFactoryGET(ctx, applianceFactoryHTTP1Client(httpClient), "search/"+url.PathEscape(query), query, perPage)
	if err != nil {
		return nil, err
	}
	products, categoryKey, err := applianceFactoryProductsFromBody(body, perPage)
	if err != nil {
		return nil, fmt.Errorf("appliance-factory: parsing search: %w", err)
	}
	if len(products) > 0 || categoryKey == "" {
		return products, nil
	}
	body, err = applianceFactoryGET(ctx, applianceFactoryHTTP1Client(httpClient), "categories/"+url.PathEscape(categoryKey), categoryKey, perPage)
	if err != nil {
		return nil, err
	}
	products, _, err = applianceFactoryProductsFromBody(body, perPage)
	if err != nil {
		return nil, fmt.Errorf("appliance-factory: parsing category: %w", err)
	}
	return products, nil
}

func applianceFactoryHTTP1Client(httpClient *http.Client) *http.Client {
	timeout := 20 * time.Second
	if httpClient != nil && httpClient.Timeout > 0 {
		timeout = httpClient.Timeout
	}
	if httpClient != nil && httpClient.Transport != nil {
		return &http.Client{
			Timeout:       timeout,
			Transport:     httpClient.Transport,
			CheckRedirect: httpClient.CheckRedirect,
			Jar:           httpClient.Jar,
		}
	}
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			ForceAttemptHTTP2: false,
			TLSNextProto:      map[string]func(string, *tls.Conn) http.RoundTripper{},
		},
	}
}

func applianceFactoryGET(ctx context.Context, httpClient *http.Client, endpoint, refererKey string, limit int) ([]byte, error) {
	u, err := url.Parse(applianceFactoryBaseURL + "/" + strings.TrimLeft(endpoint, "/"))
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("limit", fmt.Sprintf("%d", limit))
	q.Set("embed", "products")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	applyApplianceFactoryHeaders(req, "https://www.appliancefactory.com/search/"+url.PathEscape(refererKey))
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("appliance-factory: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("appliance-factory: reading body: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("appliance-factory: HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	return body, nil
}

func applyApplianceFactoryHeaders(req *http.Request, referer string) {
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Expires", "0")
	req.Header.Set("language", "en-US")
	req.Header.Set("x-px-access-token", applianceFactoryPXAccessToken)
	req.Header.Set("Origin", "https://www.appliancefactory.com")
	req.Header.Set("Referer", referer)
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36")
}

func applianceFactoryProductsFromBody(body []byte, limit int) ([]NormalizedProduct, string, error) {
	var resp struct {
		Name     string           `json:"name"`
		Messages []string         `json:"messages"`
		Products []map[string]any `json:"products"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, "", err
	}
	products := make([]NormalizedProduct, 0, len(resp.Products))
	for _, raw := range resp.Products {
		p := normalizeApplianceFactoryProduct(raw, resp.Name)
		if p.ID != "" && p.Title != "" && p.PriceMin > 0 {
			products = append(products, p)
			if limit > 0 && len(products) >= limit {
				break
			}
		}
	}
	return products, applianceFactoryCategoryKey(resp.Messages), nil
}

func normalizeApplianceFactoryProduct(raw map[string]any, fallbackCategory string) NormalizedProduct {
	p := NormalizedProduct{Source: "appliance-factory"}
	p.ID = normalizeSelectionSKU(firstNonEmptyString(jsonString(raw, "model_number"), jsonString(raw, "sku"), jsonString(raw, "entity_id")))
	p.Title = strings.Join(strings.Fields(html.UnescapeString(jsonString(raw, "name"))), " ")
	p.Brand = strings.TrimSpace(html.UnescapeString(jsonString(raw, "manufacturer")))
	p.URL = html.UnescapeString(jsonString(raw, "product_url"))
	p.ImageURL = applianceFactoryImageURL(raw)
	p.PriceMin = jsonFloat(raw, "final_price_without_tax", "final_price_with_tax")
	p.PriceMax = p.PriceMin
	p.RegularPriceMin = jsonFloat(raw, "regular_price_without_tax", "regular_price_with_tax", "msrp")
	p.RegularPriceMax = p.RegularPriceMin
	if p.RegularPriceMin > 0 && p.PriceMin > 0 && p.PriceMin < p.RegularPriceMin {
		p.OnSale = true
		p.SalePriceMin = p.PriceMin
		p.SalePriceMax = p.PriceMax
		p.DiscountPercent = (1 - p.PriceMin/p.RegularPriceMin) * 100
	}
	p.Rating = jsonFloat(raw, "reviews_rating")
	p.ReviewCount = jsonInt(raw, "reviews_count")
	p.Category = applianceFactoryCategory(raw, fallbackCategory)
	p.Description = applianceFactoryDescription(raw)
	return p
}

func applianceFactoryImageURL(raw map[string]any) string {
	for _, key := range []string{"image_url", "preview_image_url"} {
		images := mapValue(raw, key)
		if imageURL := firstNonEmptyString(jsonString(images, "normal"), jsonString(images, "small"), jsonString(images, "thumbnail")); imageURL != "" {
			return html.UnescapeString(imageURL)
		}
	}
	return ""
}

func applianceFactoryCategory(raw map[string]any, fallback string) string {
	parts := []string{}
	if family := strings.TrimSpace(jsonString(raw, "collection_name")); family != "" {
		parts = append(parts, html.UnescapeString(family))
	}
	if color := strings.TrimSpace(jsonString(raw, "color")); color != "" {
		parts = append(parts, html.UnescapeString(color))
	}
	if len(parts) > 0 {
		return strings.Join(parts, " > ")
	}
	return html.UnescapeString(fallback)
}

func applianceFactoryDescription(raw map[string]any) string {
	var parts []string
	if short := strings.TrimSpace(html.UnescapeString(jsonString(raw, "short_description"))); short != "" {
		parts = append(parts, short)
	}
	for _, key := range []string{"inventory_label", "energy_star_qualified", "upccode"} {
		if val := strings.TrimSpace(html.UnescapeString(jsonString(raw, key))); val != "" {
			parts = append(parts, fmt.Sprintf("%s: %s", strings.ReplaceAll(key, "_", " "), val))
		}
	}
	if promos := applianceFactoryPromotionDescriptions(raw); len(promos) > 0 {
		parts = append(parts, "promotions: "+strings.Join(promos, "; "))
	}
	return strings.Join(parts, "\n")
}

func searchHomewiseAppliance(ctx context.Context, httpClient *http.Client, query string, perPage int) ([]NormalizedProduct, error) {
	if perPage <= 0 {
		perPage = 20
	}
	u, err := url.Parse(homewiseApplianceBaseURL + "/bloomreach-hw")
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("action", "search")
	q.Set("limit", fmt.Sprintf("%d", perPage))
	q.Set("page", "1")
	q.Set("q", query)
	q.Set("tag", "")
	q.Set("slug", "")
	q.Set("isLuxury", "false")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	applyHomewiseApplianceHeaders(req)
	client := httpClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("homewise-appliance: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("homewise-appliance: reading body: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("homewise-appliance: HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	products, err := homewiseApplianceProductsFromBody(body, perPage)
	if err != nil {
		return nil, fmt.Errorf("homewise-appliance: parsing search: %w", err)
	}
	return products, nil
}

func applyHomewiseApplianceHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Origin", "https://www.homewiseappliance.com")
	req.Header.Set("Referer", "https://www.homewiseappliance.com/dishwashers")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36")
}

func homewiseApplianceProductsFromBody(body []byte, limit int) ([]NormalizedProduct, error) {
	var resp struct {
		Success bool             `json:"success"`
		Msg     string           `json:"msg"`
		Data    []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	if !resp.Success && len(resp.Data) == 0 {
		return nil, fmt.Errorf("API returned unsuccessful response: %s", resp.Msg)
	}
	products := make([]NormalizedProduct, 0, len(resp.Data))
	for _, raw := range resp.Data {
		p := normalizeHomewiseApplianceProduct(raw)
		if p.ID != "" && p.Title != "" && p.PriceMin > 0 {
			products = append(products, p)
			if limit > 0 && len(products) >= limit {
				break
			}
		}
	}
	return products, nil
}

func normalizeHomewiseApplianceProduct(raw map[string]any) NormalizedProduct {
	p := NormalizedProduct{Source: "homewise-appliance"}
	p.ID = normalizeSelectionSKU(firstNonEmptyString(jsonString(raw, "sku"), jsonString(raw, "pid"), jsonString(raw, "id")))
	p.Title = strings.Join(strings.Fields(html.UnescapeString(jsonString(raw, "title"))), " ")
	p.Brand = strings.TrimSpace(html.UnescapeString(jsonString(raw, "brand")))
	p.URL = html.UnescapeString(firstNonEmptyString(jsonString(raw, "url"), homewiseApplianceURLFromSlug(jsonString(raw, "pdpSeoUrl"))))
	p.ImageURL = homewiseApplianceImageURL(raw)
	p.PriceMin = firstPositiveFloat(jsonFloat(raw, "sale_price"), jsonFloat(raw, "price"), jsonFloat(raw, "displayPrice"))
	p.PriceMax = p.PriceMin
	p.RegularPriceMin = firstPositiveFloat(jsonFloat(raw, "msrp"), jsonFloat(raw, "crossedPrice"), p.PriceMin)
	p.RegularPriceMax = p.RegularPriceMin
	if p.RegularPriceMin > 0 && p.PriceMin > 0 && p.PriceMin < p.RegularPriceMin {
		p.OnSale = true
		p.SalePriceMin = p.PriceMin
		p.SalePriceMax = p.PriceMax
		p.DiscountPercent = (1 - p.PriceMin/p.RegularPriceMin) * 100
	}
	p.Category = html.UnescapeString(firstNonEmptyString(jsonString(raw, "categoryName"), jsonString(raw, "applianceType")))
	p.ReviewCount = jsonInt(raw, "reviewCount", "reviews_count")
	p.Rating = jsonFloat(raw, "rating", "averageRating")
	p.Description = homewiseApplianceDescription(raw)
	return p
}

func homewiseApplianceURLFromSlug(slug string) string {
	slug = strings.Trim(slug, " /")
	if slug == "" {
		return ""
	}
	return "https://www.homewiseappliance.com/" + slug
}

func homewiseApplianceImageURL(raw map[string]any) string {
	if image := strings.TrimSpace(jsonString(raw, "thumb_image", "thumbnail", "image")); image != "" {
		return html.UnescapeString(image)
	}
	for _, key := range []string{"large_image", "images"} {
		if vals, ok := raw[key].([]any); ok {
			for _, val := range vals {
				if s, ok := val.(string); ok && strings.TrimSpace(s) != "" {
					return html.UnescapeString(strings.TrimSpace(s))
				}
			}
		}
	}
	return ""
}

func homewiseApplianceDescription(raw map[string]any) string {
	var parts []string
	for _, key := range []string{"status", "applianceType"} {
		if val := strings.TrimSpace(html.UnescapeString(jsonString(raw, key))); val != "" {
			parts = append(parts, key+": "+val)
		}
	}
	if qty := jsonInt(raw, "sellableQuantity", "quantity"); qty > 0 {
		parts = append(parts, fmt.Sprintf("sellable quantity: %d", qty))
	}
	specsRaw := strings.TrimSpace(jsonString(raw, "topSixSpecifications"))
	if specsRaw != "" {
		var specs []map[string]any
		if err := json.Unmarshal([]byte(specsRaw), &specs); err == nil {
			for _, spec := range specs {
				key := strings.TrimSpace(html.UnescapeString(jsonString(spec, "key")))
				val := strings.TrimSpace(html.UnescapeString(jsonString(spec, "value")))
				if key != "" && val != "" {
					parts = append(parts, key+": "+val)
				}
			}
		}
	}
	return strings.Join(parts, "\n")
}

func applianceFactoryPromotionDescriptions(raw map[string]any) []string {
	promotions := mapValue(raw, "promotions")
	rows, ok := promotions["product_promotions"].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		promo, ok := row.(map[string]any)
		if !ok {
			continue
		}
		label := firstNonEmptyString(jsonString(promo, "catalog_title"), jsonString(promo, "name"), jsonString(promo, "short_description"))
		if link := jsonString(promo, "rebate_form_url", "link"); link != "" {
			label = strings.TrimSpace(label + " " + link)
		}
		if label != "" {
			out = append(out, html.UnescapeString(label))
		}
	}
	return out
}

func applianceFactoryCategoryKey(messages []string) string {
	for _, msg := range messages {
		if i := strings.Index(msg, "%2Fcatalog%2F"); i >= 0 {
			rest := msg[i+len("%2Fcatalog%2F"):]
			if end := strings.IndexAny(rest, "\"'<> &"); end >= 0 {
				rest = rest[:end]
			}
			if decoded, err := url.QueryUnescape(rest); err == nil {
				rest = decoded
			}
			return strings.Trim(rest, "/ ")
		}
		if i := strings.Index(msg, "/catalog/"); i >= 0 {
			rest := msg[i+len("/catalog/"):]
			if end := strings.IndexAny(rest, "\"'<> &"); end >= 0 {
				rest = rest[:end]
			}
			return strings.Trim(rest, "/ ")
		}
	}
	return ""
}

func searchBestBuy(ctx context.Context, httpClient *http.Client, query string, perPage int) ([]NormalizedProduct, error) {
	u, _ := url.Parse("https://www.bestbuy.com/site/searchpage.jsp")
	q := u.Query()
	q.Set("st", query)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	applyBestBuyHeaders(req)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("best-buy: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("best-buy: reading body: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("best-buy: HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	products := bestBuyProductsFromHTML(string(body), perPage)
	return products, nil
}

func applyBestBuyHeaders(req *http.Request) {
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Referer", "https://www.bestbuy.com/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36")
}

func searchAbt(ctx context.Context, httpClient *http.Client, query string, perPage int) ([]NormalizedProduct, error) {
	if perPage <= 0 {
		perPage = 20
	}
	u, _ := url.Parse("https://www.abt.com/resources/pages/search.php")
	q := u.Query()
	q.Set("keywords", query)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	applyAbtHeaders(req)

	resp, err := applianceFactoryHTTP1Client(httpClient).Do(req)
	if err != nil {
		return nil, fmt.Errorf("abt: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("abt: reading body: %w", err)
	}
	if resp.StatusCode >= 500 || resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("abt: HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	products := abtProductsFromHTML(string(body), perPage)
	return products, nil
}

func applyAbtHeaders(req *http.Request) {
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Referer", "https://www.abt.com/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36")
}

func searchQualityBath(ctx context.Context, httpClient *http.Client, query string, perPage int) ([]NormalizedProduct, error) {
	if perPage <= 0 {
		perPage = 20
	}
	searchURL := "https://www.qualitybath.com/search/" + strings.ReplaceAll(url.PathEscape(query), "+", "%20")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, err
	}
	applyQualityBathHeaders(req)

	resp, err := applianceFactoryHTTP1Client(httpClient).Do(req)
	if err != nil {
		return nil, fmt.Errorf("qualitybath: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("qualitybath: reading body: %w", err)
	}
	if resp.StatusCode >= 500 || resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("qualitybath: HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	products, err := qualityBathProductsFromHTML(string(body), perPage)
	if err != nil {
		return nil, fmt.Errorf("qualitybath: %w", err)
	}
	return products, nil
}

func applyQualityBathHeaders(req *http.Request) {
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Referer", "https://www.qualitybath.com/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36")
}

func qualityBathProductsFromHTML(body string, limit int) ([]NormalizedProduct, error) {
	if limit <= 0 {
		limit = 20
	}
	raw, ok := jsParsedStringLiteral(body, "window.__INITIAL_QUERIES__ = JSON.parse('")
	if !ok {
		return nil, fmt.Errorf("hydrated query payload not found")
	}
	var envelope struct {
		Queries []struct {
			State struct {
				Data json.RawMessage `json:"data"`
			} `json:"state"`
			QueryKey []any `json:"queryKey"`
		} `json:"queries"`
	}
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&envelope); err != nil {
		return nil, fmt.Errorf("parsing hydrated query payload: %w", err)
	}

	var products []NormalizedProduct
	for _, query := range envelope.Queries {
		if len(query.QueryKey) == 0 || fmt.Sprint(query.QueryKey[0]) != "search.getFull" {
			continue
		}
		var data struct {
			Products []map[string]any `json:"products"`
		}
		if err := json.Unmarshal(query.State.Data, &data); err != nil {
			return nil, fmt.Errorf("parsing search products: %w", err)
		}
		for _, row := range data.Products {
			p := normalizeQualityBathProduct(row)
			if p.ID == "" || p.Title == "" || p.PriceMin <= 0 {
				continue
			}
			products = append(products, p)
			if len(products) >= limit {
				return products, nil
			}
		}
	}
	return products, nil
}

func jsParsedStringLiteral(body, marker string) (string, bool) {
	start := strings.Index(body, marker)
	if start < 0 {
		return "", false
	}
	start += len(marker)
	end := strings.Index(body[start:], "');")
	if end < 0 {
		return "", false
	}
	return jsStringUnescape(body[start : start+end]), true
}

func jsStringUnescape(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 >= len(s) {
			b.WriteByte(s[i])
			continue
		}
		i++
		switch s[i] {
		case 'n':
			b.WriteByte('\n')
		case 'r':
			b.WriteByte('\r')
		case 't':
			b.WriteByte('\t')
		case 'b':
			b.WriteByte('\b')
		case 'f':
			b.WriteByte('\f')
		case '\\':
			b.WriteByte('\\')
		case '"':
			b.WriteByte('"')
		case '\'':
			b.WriteByte('\'')
		default:
			b.WriteByte('\\')
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

func normalizeQualityBathProduct(row map[string]any) NormalizedProduct {
	p := NormalizedProduct{Source: "qualitybath"}
	p.ID = normalizeSelectionSKU(firstNonEmptyString(jsonString(row, "sku"), jsonString(row, "generatedSku"), fmt.Sprint(row["id"])))
	p.Title = strings.Join(strings.Fields(html.UnescapeString(jsonString(row, "title"))), " ")
	p.Brand = html.UnescapeString(jsonString(mapValue(row, "brand"), "name"))
	if path := jsonString(row, "url"); path != "" {
		p.URL = qualityBathAbsoluteURL(path)
	}
	p.Category = qualityBathCategory(row)
	if image := qualityBathImageURL(mapValue(row, "image")); image != "" {
		p.ImageURL = image
	}
	price := mapValue(row, "price")
	p.PriceMin = firstPositiveFloat(jsonFloat(price, "discountedPrice"), jsonFloat(price, "startingPrice"))
	p.PriceMax = p.PriceMin
	p.RegularPriceMin = firstPositiveFloat(jsonFloat(mapValue(mapValue(row, "priceDisplay"), "original"), "value"), jsonFloat(price, "startingPrice"), p.PriceMin)
	p.RegularPriceMax = p.RegularPriceMin
	if p.RegularPriceMin > 0 && p.PriceMin > 0 && p.PriceMin < p.RegularPriceMin {
		p.OnSale = true
		p.SalePriceMin = p.PriceMin
		p.SalePriceMax = p.PriceMax
		p.DiscountPercent = (1 - p.PriceMin/p.RegularPriceMin) * 100
	}
	if coupon := mapValue(row, "coupon"); len(coupon) > 0 {
		if message := firstNonEmptyString(jsonString(coupon, "customCouponMessage"), jsonString(coupon, "description")); message != "" {
			p.Description = strings.TrimSpace(html.UnescapeString(message))
		}
	}
	return p
}

func firstPositiveFloat(vals ...float64) float64 {
	for _, val := range vals {
		if val > 0 {
			return val
		}
	}
	return 0
}

func qualityBathAbsoluteURL(path string) string {
	path = html.UnescapeString(strings.TrimSpace(path))
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	if strings.HasPrefix(path, "/") {
		return "https://www.qualitybath.com" + path
	}
	return "https://www.qualitybath.com/" + path
}

func qualityBathCategory(row map[string]any) string {
	var parts []string
	if category := jsonString(mapValue(row, "category"), "name"); category != "" {
		parts = append(parts, html.UnescapeString(category))
	}
	if subCategory := jsonString(row, "subCategory"); subCategory != "" {
		parts = append(parts, html.UnescapeString(subCategory))
	}
	return strings.Join(parts, " > ")
}

func qualityBathImageURL(image map[string]any) string {
	cloudinaryID := strings.TrimSpace(jsonString(image, "cloudinaryId"))
	if cloudinaryID == "" {
		return ""
	}
	if strings.HasPrefix(cloudinaryID, "http://") || strings.HasPrefix(cloudinaryID, "https://") {
		return cloudinaryID
	}
	return "https://qb-res.cloudinary.com/f_auto,q_auto/" + strings.TrimPrefix(cloudinaryID, "/")
}

func abtProductsFromHTML(body string, limit int) []NormalizedProduct {
	if limit <= 0 {
		limit = 20
	}
	if product := abtProductFromSchema(body); product.ID != "" && product.Title != "" && product.PriceMin > 0 {
		return []NormalizedProduct{product}
	}

	positions := regexp.MustCompile(`<div[^>]+class="[^"]*category_item_container[^"]*"[^>]*role="group"[^>]*aria-label="product"`).FindAllStringIndex(body, -1)
	products := make([]NormalizedProduct, 0, minInt(len(positions), limit))
	for i, pos := range positions {
		end := len(body)
		if i+1 < len(positions) {
			end = positions[i+1][0]
		}
		if product := abtProductFromCategoryBlock(body[pos[0]:end]); product.ID != "" && product.Title != "" && product.PriceMin > 0 {
			products = append(products, product)
			if len(products) >= limit {
				break
			}
		}
	}
	return products
}

func abtProductFromSchema(body string) NormalizedProduct {
	match := regexp.MustCompile(`(?is)<script[^>]+id="productschema"[^>]*>(.*?)</script>`).FindStringSubmatch(body)
	if len(match) < 2 {
		return NormalizedProduct{}
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(html.UnescapeString(strings.TrimSpace(match[1]))), &raw); err != nil {
		return NormalizedProduct{}
	}

	p := NormalizedProduct{Source: "abt"}
	p.ID = strings.TrimSpace(fmt.Sprint(raw["productID"]))
	p.Title = strings.TrimSpace(html.UnescapeString(jsonString(raw, "name")))
	p.URL = html.UnescapeString(jsonString(raw, "url", "@id"))
	p.Brand = abtSchemaBrand(raw)
	p.ImageURL = abtSchemaImage(raw)
	p.Category = abtCategoryFromBody(body)
	p.Description = strings.TrimSpace(html.UnescapeString(jsonString(raw, "description")))
	p.PriceMin = abtOfferPrice(raw)
	p.PriceMax = p.PriceMin
	p.RegularPriceMin = p.PriceMin
	p.RegularPriceMax = p.PriceMax
	if model := firstNonEmptyString(jsonString(raw, "model"), jsonString(raw, "mpn"), jsonString(raw, "sku")); model != "" {
		p.ID = normalizeSelectionSKU(model)
	}
	return p
}

func abtSchemaBrand(raw map[string]any) string {
	if brand, ok := raw["brand"].(map[string]any); ok {
		if name := jsonString(brand, "name"); name != "" {
			return html.UnescapeString(name)
		}
	}
	if manufacturer, ok := raw["manufacturer"].(map[string]any); ok {
		if name := jsonString(manufacturer, "name"); name != "" {
			return html.UnescapeString(name)
		}
	}
	return ""
}

func abtSchemaImage(raw map[string]any) string {
	switch images := raw["image"].(type) {
	case []any:
		for _, image := range images {
			if s, ok := image.(string); ok && strings.HasPrefix(s, "http") {
				return html.UnescapeString(s)
			}
		}
	case string:
		return html.UnescapeString(images)
	}
	return ""
}

func abtOfferPrice(raw map[string]any) float64 {
	if offer, ok := raw["offers"].(map[string]any); ok {
		return jsonFloat(offer, "price")
	}
	return 0
}

func abtCategoryFromBody(body string) string {
	matches := regexp.MustCompile(`(?is)<script[^>]+type="application/ld\+json"[^>]*>\s*(\{.*?"@type"\s*:\s*"BreadcrumbList".*?\})\s*</script>`).FindAllStringSubmatch(body, -1)
	for _, match := range matches {
		var raw map[string]any
		if err := json.Unmarshal([]byte(html.UnescapeString(match[1])), &raw); err != nil {
			continue
		}
		items, ok := raw["itemListElement"].([]any)
		if !ok {
			continue
		}
		var parts []string
		for _, item := range items {
			row, ok := item.(map[string]any)
			if !ok {
				continue
			}
			name := strings.TrimSpace(html.UnescapeString(jsonString(row, "name")))
			if name != "" && !strings.EqualFold(name, "Home") {
				parts = append(parts, name)
			}
		}
		if len(parts) > 1 {
			return strings.Join(parts[:len(parts)-1], " > ")
		}
	}
	return "Appliances"
}

func abtProductFromCategoryBlock(block string) NormalizedProduct {
	p := NormalizedProduct{Source: "abt"}
	if link := regexp.MustCompile(`(?is)<a[^>]+class="[^"]*categoryTitleLink[^"]*"[^>]*>`).FindString(block); link != "" {
		p.URL = html.UnescapeString(htmlAttr(link, "href"))
		p.ID = firstNonEmptyString(htmlAttr(link, "data-productId"), abtIDFromURL(p.URL))
	}
	if titleMatch := regexp.MustCompile(`(?is)<a[^>]+class="[^"]*categoryTitleLink[^"]*"[^>]*>(.*?)</a>`).FindStringSubmatch(block); len(titleMatch) > 1 {
		p.Title = strings.Join(strings.Fields(html.UnescapeString(stripHTMLTags(titleMatch[1]))), " ")
	}
	if modelMatch := regexp.MustCompile(`(?is)<div[^>]+class="[^"]*cl_abt_model[^"]*"[^>]*>\s*Abt Model:\s*([^<]+)`).FindStringSubmatch(block); len(modelMatch) > 1 {
		p.ID = normalizeSelectionSKU(strings.TrimSpace(html.UnescapeString(modelMatch[1])))
	}
	if img := abtCategoryImage(block); img != "" {
		p.ImageURL = img
	}
	p.Brand = abtBrandFromTitle(p.Title)
	p.Category = strings.TrimSpace(html.UnescapeString(htmlAttr(block, "data-category-name")))
	if p.Category == "" {
		p.Category = "Appliances"
	}
	p.PriceMin = abtCategoryPrice(block, "pricing-item-price")
	p.PriceMax = p.PriceMin
	p.RegularPriceMin = abtCategoryPrice(block, "pricing-regular-price")
	p.RegularPriceMax = p.RegularPriceMin
	if p.RegularPriceMin > 0 && p.PriceMin > 0 && p.PriceMin < p.RegularPriceMin {
		p.OnSale = true
		p.SalePriceMin = p.PriceMin
		p.SalePriceMax = p.PriceMax
		p.DiscountPercent = (1 - p.PriceMin/p.RegularPriceMin) * 100
	}
	if strings.Contains(strings.ToLower(block), "in stock") {
		p.Description = "availability: In Stock"
	}
	return p
}

func abtIDFromURL(productURL string) string {
	match := regexp.MustCompile(`/p/([0-9]+)\.html`).FindStringSubmatch(productURL)
	if len(match) > 1 {
		return match[1]
	}
	return ""
}

func abtCategoryImage(block string) string {
	for _, match := range regexp.MustCompile(`(?is)<img[^>]+>`).FindAllString(block, -1) {
		src := html.UnescapeString(htmlAttr(match, "src"))
		if strings.HasPrefix(src, "https://content.abt.com/") && strings.Contains(src, "/products/") {
			return src
		}
	}
	return ""
}

func abtCategoryPrice(block, className string) float64 {
	match := regexp.MustCompile(`(?is)<div[^>]+class="[^"]*` + regexp.QuoteMeta(className) + `[^"]*"[^>]*>(.*?)</div>`).FindStringSubmatch(block)
	if len(match) < 2 {
		return 0
	}
	prices := regexp.MustCompile(`\$[0-9][0-9,]*(?:\.[0-9]{2})?`).FindAllString(stripHTMLTags(match[1]), -1)
	for _, price := range prices {
		if val := parsePriceString(strings.TrimPrefix(strings.ReplaceAll(price, ",", ""), "$")); val > 0 {
			return val
		}
	}
	return 0
}

func abtBrandFromTitle(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return ""
	}
	if before, _, ok := strings.Cut(title, " "); ok {
		return before
	}
	return title
}

func bestBuyProductsFromHTML(body string, limit int) []NormalizedProduct {
	prices := bestBuyPricesBySKU(body)
	var products []NormalizedProduct
	seen := map[string]bool{}
	positions := regexp.MustCompile(`"skuId":"[0-9]+","openBoxCondition":null,"whatItIs"`).FindAllStringIndex(body, -1)
	for _, pos := range positions {
		block := bestBuyProductWindow(body, pos[0])
		p := normalizeBestBuyProduct(block, prices)
		if p.ID == "" || p.Title == "" || p.PriceMin <= 0 {
			continue
		}
		dedupeKey := p.ID
		if dedupeKey == "" {
			dedupeKey = p.URL
		}
		if seen[dedupeKey] {
			continue
		}
		seen[dedupeKey] = true
		products = append(products, p)
		if limit > 0 && len(products) >= limit {
			break
		}
	}
	return products
}

func bestBuyProductWindow(body string, start int) string {
	end := start + 9000
	if end > len(body) {
		end = len(body)
	}
	return body[start:end]
}

func normalizeBestBuyProduct(block string, prices map[string]bestBuyPrice) NormalizedProduct {
	p := NormalizedProduct{Source: "best-buy"}
	sku := bestBuyStringField(block, "skuId")
	model := bestBuyNestedStringField(block, "manufacturer", "modelNumber")
	if model == "" {
		model = bestBuyStringField(block, "bsin")
	}
	p.ID = normalizeSelectionSKU(firstNonEmptyString(model, sku))
	p.Title = strings.Join(strings.Fields(html.UnescapeString(bestBuyNestedStringField(block, "name", "short"))), " ")
	if p.Title == "" {
		p.Title = strings.Join(strings.Fields(html.UnescapeString(bestBuyNestedStringField(block, "name", "title"))), " ")
	}
	p.Brand = strings.TrimSpace(html.UnescapeString(bestBuyStringField(block, "brand")))
	if p.Brand == "" && strings.Contains(p.Title, " - ") {
		p.Brand = strings.TrimSpace(strings.SplitN(p.Title, " - ", 2)[0])
	}
	p.URL = html.UnescapeString(bestBuyNestedStringField(block, "url", "skuSpecificUrl"))
	if p.URL == "" {
		p.URL = html.UnescapeString(bestBuyNestedStringField(block, "url", "pdp"))
	}
	p.ImageURL = html.UnescapeString(bestBuyNestedStringField(block, "primaryImage", "piscesHref"))
	if p.ImageURL == "" {
		p.ImageURL = html.UnescapeString(bestBuyNestedStringField(block, "primaryImage", "href"))
	}
	p.Category = strings.Join(bestBuyStringArrayField(block, "whatItIs"), " > ")
	if price := prices[sku]; price.Customer > 0 {
		p.PriceMin = price.Customer
		p.PriceMax = price.Customer
		if price.Regular > price.Customer {
			p.RegularPriceMin = price.Regular
			p.RegularPriceMax = price.Regular
			p.OnSale = true
			p.SalePriceMin = price.Customer
			p.SalePriceMax = price.Customer
			p.DiscountPercent = (1 - price.Customer/price.Regular) * 100
		}
	}
	p.Rating = bestBuyNestedFloatField(block, "reviewInfo", "averageRating")
	p.ReviewCount = int(bestBuyNestedFloatField(block, "reviewInfo", "reviewCount"))
	return p
}

type bestBuyPrice struct {
	Customer float64
	Regular  float64
}

func bestBuyPricesBySKU(body string) map[string]bestBuyPrice {
	out := map[string]bestBuyPrice{}
	re := regexp.MustCompile(`(?s)"price":\{"__typename":"ItemPrice",(.*?)\}`)
	for _, match := range re.FindAllStringSubmatch(body, -1) {
		if len(match) < 2 || strings.Contains(match[1], `"openBoxCondition":0`) || strings.Contains(match[1], `"openBoxCondition":1`) || strings.Contains(match[1], `"openBoxCondition":2`) {
			continue
		}
		sku := bestBuyStringField(match[1], "skuId")
		if sku == "" {
			continue
		}
		customer := bestBuyNumberField(match[1], "customerPrice")
		if customer <= 0 {
			continue
		}
		regular := customer + bestBuyNumberField(match[1], "totalNonPaidMemberSavings")
		if regular <= customer {
			regular = customer
		}
		if out[sku].Customer == 0 || customer > out[sku].Customer {
			out[sku] = bestBuyPrice{Customer: customer, Regular: regular}
		}
	}
	return out
}

func bestBuyStringField(block, key string) string {
	re := regexp.MustCompile(`"` + regexp.QuoteMeta(key) + `":"((?:\\.|[^"\\])*)"`)
	match := re.FindStringSubmatch(block)
	if len(match) < 2 {
		return ""
	}
	return bestBuyDecodeJSONString(match[1])
}

func bestBuyNestedStringField(block, objectKey, fieldKey string) string {
	idx := strings.Index(block, `"`+objectKey+`":`)
	if idx < 0 {
		return ""
	}
	return bestBuyStringField(block[idx:], fieldKey)
}

func bestBuyNestedFloatField(block, objectKey, fieldKey string) float64 {
	idx := strings.Index(block, `"`+objectKey+`":`)
	if idx < 0 {
		return 0
	}
	return bestBuyNumberField(block[idx:], fieldKey)
}

func bestBuyNumberField(block, key string) float64 {
	re := regexp.MustCompile(`"` + regexp.QuoteMeta(key) + `":([0-9]+(?:\.[0-9]+)?)`)
	match := re.FindStringSubmatch(block)
	if len(match) < 2 {
		return 0
	}
	return parsePriceString(match[1])
}

func bestBuyStringArrayField(block, key string) []string {
	re := regexp.MustCompile(`"` + regexp.QuoteMeta(key) + `":\[(.*?)\]`)
	match := re.FindStringSubmatch(block)
	if len(match) < 2 {
		return nil
	}
	var out []string
	for _, raw := range regexp.MustCompile(`"((?:\\.|[^"\\])*)"`).FindAllStringSubmatch(match[1], -1) {
		if len(raw) > 1 {
			if value := strings.TrimSpace(bestBuyDecodeJSONString(raw[1])); value != "" {
				out = append(out, value)
			}
		}
	}
	return out
}

func bestBuyDecodeJSONString(s string) string {
	var decoded string
	if err := json.Unmarshal([]byte(`"`+s+`"`), &decoded); err == nil {
		return decoded
	}
	return s
}

const brayAndScarffOrganizationID = "4708975a-2cca-4b84-a56c-5f97a82a7b59"

// searchBrayAndScarff replays the NMG Platform flow used by Bray & Scarff's
// product listing page: search materialized product IDs, then hydrate dealer
// override rows with prices and source product model metadata.
func searchBrayAndScarff(ctx context.Context, httpClient *http.Client, query string, perPage int) ([]NormalizedProduct, error) {
	if perPage <= 0 {
		perPage = 20
	}
	limit := perPage
	if limit < 1 {
		limit = 1
	}
	if limit > 50 {
		limit = 50
	}

	ids, err := brayAndScarffSearchProductIDs(ctx, httpClient, query, limit)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return []NormalizedProduct{}, nil
	}

	payload := map[string]any{
		"query": brayAndScarffProductsQuery(brayAndScarffGraphQLArray(ids)),
		"variables": map[string]any{
			"org_id":  brayAndScarffOrganizationID,
			"offset":  0,
			"limit":   limit,
			"sort_by": "DEFAULT",
		},
	}
	body, err := brayAndScarffGraphQL(ctx, httpClient, payload)
	if err != nil {
		return nil, fmt.Errorf("bray-and-scarff: products: %w", err)
	}

	var gqlResp struct {
		Errors []map[string]any `json:"errors"`
		Data   struct {
			Products []map[string]any `json:"products"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &gqlResp); err != nil {
		return nil, fmt.Errorf("bray-and-scarff: parsing products: %w", err)
	}
	if len(gqlResp.Errors) > 0 {
		return nil, fmt.Errorf("bray-and-scarff: products GraphQL errors: %s", truncate(string(body), 240))
	}

	products := make([]NormalizedProduct, 0, len(gqlResp.Data.Products))
	for _, raw := range gqlResp.Data.Products {
		p := normalizeBrayAndScarffProduct(raw)
		if p.ID != "" && p.Title != "" {
			products = append(products, p)
		}
	}
	return products, nil
}

func brayAndScarffSearchProductIDs(ctx context.Context, httpClient *http.Client, query string, limit int) ([]string, error) {
	searchQuery := fmt.Sprintf(`query SearchProducts { search_products_materialized(args: {search: %q, org_id: %q, limit_qty: %d}) { id } }`, query, brayAndScarffOrganizationID, limit)
	payload := map[string]any{
		"query":     searchQuery,
		"variables": map[string]any{},
	}
	body, err := brayAndScarffGraphQL(ctx, httpClient, payload)
	if err != nil {
		return nil, fmt.Errorf("bray-and-scarff: search: %w", err)
	}

	var gqlResp struct {
		Errors []map[string]any `json:"errors"`
		Data   struct {
			SearchProducts []struct {
				ID string `json:"id"`
			} `json:"search_products_materialized"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &gqlResp); err != nil {
		return nil, fmt.Errorf("bray-and-scarff: parsing search: %w", err)
	}
	if len(gqlResp.Errors) > 0 {
		return nil, fmt.Errorf("bray-and-scarff: search GraphQL errors: %s", truncate(string(body), 240))
	}

	ids := make([]string, 0, len(gqlResp.Data.SearchProducts))
	for _, row := range gqlResp.Data.SearchProducts {
		if row.ID != "" {
			ids = append(ids, row.ID)
		}
	}
	return ids, nil
}

func brayAndScarffGraphQL(ctx context.Context, httpClient *http.Client, payload map[string]any) ([]byte, error) {
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", "https://hasura.nmg-platform.com/v1/graphql", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://www.brayandscarff.com")
	req.Header.Set("Referer", "https://www.brayandscarff.com/products")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading body: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	return body, nil
}

func brayAndScarffProductsQuery(productIDs string) string {
	return fmt.Sprintf(`query ProductsQuery($language_code: iso_language_codes_enum = en, $offset: numeric = 0, $limit: numeric = 12, $org_id: uuid!, $sort_by: String) {
  products: get_products(args: {_organization_id: $org_id, _limit: $limit, _offset: $offset, _sort_by: $sort_by, _product_ids: %q}) {
    id
    regular
    sale
    validation_price
    validation_price_overridden
    category {
      category_translations { value slug_value }
      parent_category {
        category_translations { value slug_value }
        parent_category {
          category_translations { value slug_value }
          parent_category { category_translations { value slug_value } }
        }
      }
    }
    default_source_product {
      id
      manufacturer_pn
      pn
      product_translations(where: {iso_language_code: {_eq: $language_code}}) {
        short_description
        long_description
      }
      brand {
        name
        slug
        code
      }
      product_images(order_by: {image_order_number: asc_nulls_last}, limit: 3) {
        media_url
        media_name
        image_order_number
      }
      product_attributes {
        attribute_code
        name
        value
      }
    }
  }
}`, productIDs)
}

func brayAndScarffGraphQLArray(vals []string) string {
	return "{" + strings.Join(vals, ",") + "}"
}

func normalizeBrayAndScarffProduct(raw map[string]any) NormalizedProduct {
	source := mapValue(raw, "default_source_product")
	p := NormalizedProduct{Source: "bray-and-scarff"}
	p.ID = firstNonEmptyString(
		jsonString(source, "manufacturer_pn"),
		jsonString(source, "pn"),
		jsonString(source, "id"),
		jsonString(raw, "id"),
	)
	p.Title = brayTranslationString(source, "short_description")
	p.Description = brayAndScarffDescription(source)
	p.Brand = brayBrandName(source)
	p.Category = brayAndScarffCategory(raw)
	p.URL = brayAndScarffProductURL(raw, source)
	p.ImageURL = brayAndScarffImageURL(source)
	p.RegularPriceMin = jsonFloat(raw, "regular")
	p.RegularPriceMax = p.RegularPriceMin
	sale := jsonFloat(raw, "sale")
	validation := jsonFloat(raw, "validation_price_overridden", "validation_price")
	p.PriceMin = sale
	if validation > 0 && (p.PriceMin == 0 || validation < p.PriceMin) {
		p.PriceMin = validation
	}
	if p.PriceMin == 0 {
		p.PriceMin = p.RegularPriceMin
	}
	p.PriceMax = p.PriceMin
	if p.RegularPriceMin > 0 && p.PriceMin > 0 && p.PriceMin < p.RegularPriceMin {
		p.OnSale = true
		p.SalePriceMin = p.PriceMin
		p.SalePriceMax = p.PriceMax
		p.DiscountPercent = (1 - p.PriceMin/p.RegularPriceMin) * 100
	}
	return p
}

func brayTranslationString(source map[string]any, field string) string {
	translations, ok := source["product_translations"].([]any)
	if !ok || len(translations) == 0 {
		return ""
	}
	first, ok := translations[0].(map[string]any)
	if !ok {
		return ""
	}
	return html.UnescapeString(jsonString(first, field))
}

func brayBrandName(source map[string]any) string {
	brand := mapValue(source, "brand")
	return html.UnescapeString(jsonString(brand, "name"))
}

func brayAndScarffDescription(source map[string]any) string {
	var parts []string
	if longDesc := brayTranslationString(source, "long_description"); longDesc != "" {
		parts = append(parts, longDesc)
	}
	if attrs, ok := source["product_attributes"].([]any); ok {
		for _, attr := range attrs {
			m, ok := attr.(map[string]any)
			if !ok {
				continue
			}
			name := firstNonEmptyString(jsonString(m, "name"), jsonString(m, "attribute_code"))
			value := jsonString(m, "value")
			if name != "" && value != "" {
				parts = append(parts, fmt.Sprintf("%s: %s", html.UnescapeString(name), html.UnescapeString(value)))
			}
		}
	}
	return strings.Join(parts, "\n")
}

func brayAndScarffCategory(raw map[string]any) string {
	slugs := brayCategoryParts(raw, "value")
	return strings.Join(slugs, " > ")
}

func brayAndScarffProductURL(raw, source map[string]any) string {
	slugs := brayCategoryParts(raw, "slug_value")
	brandSlug := jsonString(mapValue(source, "brand"), "slug")
	model := strings.ToLower(firstNonEmptyString(jsonString(source, "pn"), jsonString(source, "manufacturer_pn")))
	pathParts := append(slugs, brandSlug, model)
	cleaned := make([]string, 0, len(pathParts))
	for _, part := range pathParts {
		part = strings.Trim(part, "/ ")
		if part != "" {
			cleaned = append(cleaned, part)
		}
	}
	if len(cleaned) == 0 {
		return "https://www.brayandscarff.com/products"
	}
	return "https://www.brayandscarff.com/" + strings.Join(cleaned, "/") + "/"
}

func brayCategoryParts(raw map[string]any, field string) []string {
	var reversed []string
	for cat := mapValue(raw, "category"); len(cat) > 0; cat = mapValue(cat, "parent_category") {
		translations, ok := cat["category_translations"].([]any)
		if !ok || len(translations) == 0 {
			continue
		}
		first, ok := translations[0].(map[string]any)
		if !ok {
			continue
		}
		val := html.UnescapeString(jsonString(first, field))
		if val != "" {
			reversed = append(reversed, val)
		}
	}
	parts := make([]string, 0, len(reversed))
	for i := len(reversed) - 1; i >= 0; i-- {
		parts = append(parts, reversed[i])
	}
	return parts
}

func brayAndScarffImageURL(source map[string]any) string {
	images, ok := source["product_images"].([]any)
	if !ok || len(images) == 0 {
		return ""
	}
	first, ok := images[0].(map[string]any)
	if !ok {
		return ""
	}
	return html.UnescapeString(jsonString(first, "media_url"))
}

func searchPCRichard(ctx context.Context, httpClient *http.Client, query string, perPage int) ([]NormalizedProduct, error) {
	u, _ := url.Parse("https://www.pcrichard.com/search")
	q := u.Query()
	q.Set("q", query)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pc-richard: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("pc-richard: reading body: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("pc-richard: HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	products := pcRichardProductsFromHTML(string(body), perPage)
	return products, nil
}

func pcRichardProductsFromHTML(body string, limit int) []NormalizedProduct {
	var products []NormalizedProduct
	positions := regexp.MustCompile(`data-ga4-select-item=`).FindAllStringIndex(body, -1)
	for i, pos := range positions {
		start := pos[0]
		end := len(body)
		if i+1 < len(positions) {
			end = positions[i+1][0]
		}
		block := body[start:end]
		if p := normalizePCRichardProduct(block); p.ID != "" && p.Title != "" && p.PriceMin > 0 {
			products = append(products, p)
			if limit > 0 && len(products) >= limit {
				break
			}
		}
	}
	return products
}

func normalizePCRichardProduct(block string) NormalizedProduct {
	raw := strings.TrimSpace(html.UnescapeString(htmlAttr(block, "data-ga4-select-item")))
	if raw == "" {
		return NormalizedProduct{Source: "pc-richard"}
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return NormalizedProduct{Source: "pc-richard"}
	}
	item := pcRichardGA4Item(data)
	p := NormalizedProduct{Source: "pc-richard"}
	p.ID = normalizeSelectionSKU(jsonString(item, "item_id"))
	if p.ID == "" {
		p.ID = normalizeSelectionSKU(jsonString(item, "master_id"))
	}
	p.Title = strings.Join(strings.Fields(html.UnescapeString(jsonString(item, "item_name"))), " ")
	p.Brand = strings.TrimSpace(html.UnescapeString(jsonString(item, "brand")))
	p.Category = pcRichardCategory(item)
	p.PriceMin = jsonFloat(item, "price")
	p.PriceMax = p.PriceMin
	if href := pcRichardProductHref(block); href != "" {
		p.URL = html.UnescapeString(href)
	}
	if img := pcRichardImageURL(block); img != "" {
		p.ImageURL = html.UnescapeString(img)
	}
	if regular := pcRichardRegularPrice(block); regular > 0 && p.PriceMin > 0 && p.PriceMin < regular {
		p.RegularPriceMin = regular
		p.OnSale = true
		p.SalePriceMin = p.PriceMin
		p.SalePriceMax = p.PriceMax
		p.DiscountPercent = (1 - p.PriceMin/regular) * 100
	}
	return p
}

func pcRichardGA4Item(data map[string]any) map[string]any {
	ecommerce := mapValue(data, "ecommerce")
	items, ok := ecommerce["items"].([]any)
	if !ok || len(items) == 0 {
		return map[string]any{}
	}
	first, ok := items[0].(map[string]any)
	if !ok {
		return map[string]any{}
	}
	return first
}

func pcRichardCategory(item map[string]any) string {
	var parts []string
	for _, key := range []string{"item_category2", "item_category3", "item_category4", "item_category5"} {
		value := strings.TrimSpace(html.UnescapeString(jsonString(item, key)))
		if value != "" && (len(parts) == 0 || parts[len(parts)-1] != value) {
			parts = append(parts, value)
		}
	}
	if len(parts) > 0 {
		return strings.Join(parts, " > ")
	}
	return strings.TrimSpace(html.UnescapeString(jsonString(item, "item_category")))
}

func pcRichardProductHref(block string) string {
	match := regexp.MustCompile(`(?is)<a[^>]+href="(https://www\.pcrichard\.com/[^"]+\.html)"`).FindStringSubmatch(block)
	if len(match) > 1 {
		return match[1]
	}
	return htmlAttr(block, "href")
}

func pcRichardImageURL(block string) string {
	match := regexp.MustCompile(`(?is)<img[^>]+class="[^"]*tile-image[^"]*"[^>]+src="([^"]+)"`).FindStringSubmatch(block)
	if len(match) > 1 {
		return match[1]
	}
	return ""
}

func pcRichardRegularPrice(block string) float64 {
	match := regexp.MustCompile(`(?is)<span[^>]+class="value"[^>]+content="([0-9][0-9,.]*)"[^>]*>\s*<span[^>]+class="sr-only">\s*Price reduced from`).FindStringSubmatch(block)
	if len(match) > 1 {
		return parsePriceString(match[1])
	}
	return 0
}

// searchFloorAndDecor queries the public Algolia index exposed by the Floor &
// Decor search page.
func searchFloorAndDecor(ctx context.Context, httpClient *http.Client, query string, perPage int) ([]NormalizedProduct, error) {
	if perPage <= 0 {
		perPage = 20
	}
	payload := map[string]any{
		"query":       query,
		"hitsPerPage": perPage,
	}
	bodyBytes, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST", "https://AR91I5G1KF-dsn.algolia.net/1/indexes/production__products__default/query", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Algolia-Application-Id", "AR91I5G1KF")
	req.Header.Set("X-Algolia-API-Key", "a107b054c16c35a5033915306c8eaf45")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("floor-and-decor: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("floor-and-decor: reading body: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("floor-and-decor: HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}

	var algoliaResp struct {
		Hits []map[string]any `json:"hits"`
	}
	if err := json.Unmarshal(body, &algoliaResp); err != nil {
		return nil, fmt.Errorf("floor-and-decor: parsing response: %w", err)
	}

	products := make([]NormalizedProduct, 0, len(algoliaResp.Hits))
	for _, hit := range algoliaResp.Hits {
		products = append(products, normalizeFloorAndDecor(hit))
	}
	return products, nil
}

func normalizeFloorAndDecor(hit map[string]any) NormalizedProduct {
	p := NormalizedProduct{Source: "floor-and-decor"}
	if id, ok := hit["id"].(string); ok {
		p.ID = id
	} else if id, ok := hit["objectID"].(string); ok {
		p.ID = id
	}
	if title, ok := hit["name"].(string); ok {
		p.Title = title
	}
	if brand, ok := hit["brand"].(string); ok {
		p.Brand = brand
	}
	if u, ok := hit["url"].(string); ok {
		if strings.HasPrefix(u, "/") {
			p.URL = "https://www.flooranddecor.com" + u
		} else {
			p.URL = u
		}
	}
	if images, ok := hit["images"].([]any); ok && len(images) > 0 {
		if img, ok := images[0].(string); ok {
			p.ImageURL = img
		}
	} else if img, ok := hit["image"].(string); ok {
		p.ImageURL = img
	}
	if desc, ok := hit["short_description"].(string); ok {
		p.Description = desc
	} else if desc, ok := hit["long_description"].(string); ok {
		p.Description = desc
	}
	p.PriceMin = algoliaUSDPrice(hit, "price")
	if p.PriceMin == 0 {
		p.PriceMin = jsonFloat(hit, "price")
	}
	p.PriceMax = p.PriceMin
	p.RegularPriceMin = algoliaUSDPrice(hit, "regularPrice")
	p.RegularPriceMax = p.RegularPriceMin
	if sale := algoliaUSDPrice(hit, "salePrice"); sale > 0 {
		p.SalePriceMin = sale
		p.SalePriceMax = sale
		p.OnSale = p.RegularPriceMin > 0 && sale < p.RegularPriceMin
		if p.OnSale {
			p.DiscountPercent = (1 - sale/p.RegularPriceMin) * 100
		}
	}
	if subtype, ok := hit["productSubtype"].(string); ok {
		p.Category = subtype
	} else if cats, ok := hit["categories"].([]any); ok && len(cats) > 0 {
		if cat, ok := cats[0].(string); ok {
			p.Category = cat
		}
	}
	return p
}

func algoliaUSDPrice(hit map[string]any, key string) float64 {
	raw, ok := hit[key]
	if !ok {
		return 0
	}
	if m, ok := raw.(map[string]any); ok {
		return jsonFloat(m, "USD", "usd")
	}
	return jsonFloat(map[string]any{key: raw}, key)
}

// searchFaucetDepot queries FaucetDepot's public Algolia product index. The
// index is broad plumbing supply, so normalization filters repair/service parts
// and keeps the homeowner-visible fixture/rough-in selection layer.
func searchFaucetDepot(ctx context.Context, httpClient *http.Client, query string, perPage int) ([]NormalizedProduct, error) {
	if perPage <= 0 {
		perPage = 20
	}
	payload := map[string]any{
		"query":                  query,
		"page":                   0,
		"hitsPerPage":            maxInt(perPage*3, perPage),
		"typoTolerance":          "min",
		"ignorePlurals":          true,
		"queryLanguages":         []string{"en"},
		"removeWordsIfNoResults": "allOptional",
		"advancedSyntax":         true,
	}
	bodyBytes, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST", "https://FSDN8N73JY-dsn.algolia.net/1/indexes/Products2025/query", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Algolia-Application-Id", "FSDN8N73JY")
	req.Header.Set("X-Algolia-API-Key", "da04002258047b626e1c1111dc91bec9")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("faucetdepot: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("faucetdepot: reading body: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("faucetdepot: HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}

	var algoliaResp struct {
		Hits []map[string]any `json:"hits"`
	}
	if err := json.Unmarshal(body, &algoliaResp); err != nil {
		return nil, fmt.Errorf("faucetdepot: parsing response: %w", err)
	}

	products := make([]NormalizedProduct, 0, minInt(len(algoliaResp.Hits), perPage))
	for _, hit := range algoliaResp.Hits {
		p, ok := normalizeFaucetDepot(hit)
		if !ok {
			continue
		}
		products = append(products, p)
		if len(products) >= perPage {
			break
		}
	}
	return products, nil
}

func normalizeFaucetDepot(hit map[string]any) (NormalizedProduct, bool) {
	p := NormalizedProduct{Source: "faucetdepot"}
	if id, ok := hit["SHIMSID"].(string); ok {
		p.ID = id
	} else if id, ok := hit["objectID"].(string); ok {
		p.ID = id
	}
	if title, ok := hit["PRODUCTNAME"].(string); ok {
		p.Title = html.UnescapeString(title)
	}
	if p.Title == "" {
		return p, false
	}
	if !faucetDepotSelectionTitle(p.Title) {
		return p, false
	}
	if brand, ok := hit["BRAND_NAME"].(string); ok {
		p.Brand = strings.TrimSpace(html.UnescapeString(brand))
	} else if brand, ok := hit["MANUFACTURER_NAME"].(string); ok {
		p.Brand = strings.TrimSpace(html.UnescapeString(brand))
	}
	if u, ok := hit["PRODUCTURL"].(string); ok && u != "" {
		if strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
			p.URL = html.UnescapeString(u)
		} else {
			p.URL = "https://faucetdepot.com" + html.UnescapeString(u)
		}
	}
	if img, ok := hit["IMAGE_URL"].(string); ok {
		p.ImageURL = html.UnescapeString(img)
	}
	if desc, ok := hit["LONGDESCRIPTION"].(string); ok && desc != "" {
		p.Description = html.UnescapeString(desc)
	} else if desc, ok := hit["SHORTDESCRIPTION"].(string); ok {
		p.Description = html.UnescapeString(desc)
	}
	p.PriceMin = jsonFloat(hit, "SELL_PRICE", "PRICE", "LIST_PRICE")
	p.PriceMax = p.PriceMin
	p.RegularPriceMin = jsonFloat(hit, "LIST_PRICE")
	p.RegularPriceMax = p.RegularPriceMin
	if p.PriceMin <= 0 {
		return p, false
	}
	if p.RegularPriceMin > 0 && p.PriceMin < p.RegularPriceMin {
		p.OnSale = true
		p.SalePriceMin = p.PriceMin
		p.SalePriceMax = p.PriceMax
		p.DiscountPercent = (1 - p.PriceMin/p.RegularPriceMin) * 100
	}
	if cat, ok := hit["CATEGORY_NAME"].(string); ok {
		p.Category = html.UnescapeString(cat)
	} else if path, ok := hit["category_path"].([]any); ok && len(path) > 0 {
		var cats []string
		for _, raw := range path {
			if s, ok := raw.(string); ok && s != "" {
				cats = append(cats, html.UnescapeString(s))
			}
		}
		p.Category = strings.Join(cats, " > ")
	}
	return p, true
}

func faucetDepotSelectionTitle(title string) bool {
	t := strings.ToLower(html.UnescapeString(title))
	if containsAny(t,
		"cartridge", "stem", "seat", "gasket", "connector", "supply line", "wax ring", "flange",
		"repair", "replacement", "for use with", "o-ring", "washer", "adapter", "coupling",
		"pipe", "tube", "hose", "fitting", "nipple", "union", "trap", "strainer", "stop valve",
		"solenoid", "valve cartridge", "handle kit", "spout assembly", "mounting kit", "escutcheon",
		"fill valve", "flush valve", "flush lever", "tank lever", "flapper", "ballcock",
	) {
		if !containsAny(t, "rough-in valve", "rough in valve") {
			return false
		}
	}
	return containsAny(t,
		"faucet", "sink", "toilet", "tub", "shower", "rough-in valve", "rough in valve",
		"trim", "vanity", "garbage disposal", "water dispenser", "water heater",
	) && !containsAny(t, "flashlight", "test", "tester", "glove", "tape", "lubricant", "cleaner")
}

// searchSuperBrightLEDs queries the public Magento/Algolia product index used
// by Super Bright LEDs.
func searchSuperBrightLEDs(ctx context.Context, httpClient *http.Client, query string, perPage int) ([]NormalizedProduct, error) {
	if perPage <= 0 {
		perPage = 20
	}
	payload := map[string]any{
		"query":       query,
		"hitsPerPage": perPage,
	}
	bodyBytes, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST", "https://VTAW7SB4LM-dsn.algolia.net/1/indexes/magento2_prod_default_products/query", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Algolia-Application-Id", "VTAW7SB4LM")
	req.Header.Set("X-Algolia-API-Key", "YzY4OGY1YTE0ZjA3ODA4ZjRkNGM5ZjkzN2IxZjg4MTY0NjJkZTFlMThlNmE1MTNkYWIwODFiMzUxM2ViMDc1NXRhZ0ZpbHRlcnM9JnZhbGlkVW50aWw9MTc4MDcxODQxNA==")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("superbrightleds: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("superbrightleds: reading body: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("superbrightleds: HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}

	var algoliaResp struct {
		Hits []map[string]any `json:"hits"`
	}
	if err := json.Unmarshal(body, &algoliaResp); err != nil {
		return nil, fmt.Errorf("superbrightleds: parsing response: %w", err)
	}

	products := make([]NormalizedProduct, 0, len(algoliaResp.Hits))
	for _, hit := range algoliaResp.Hits {
		products = append(products, normalizeSuperBrightLEDs(hit))
	}
	return products, nil
}

func normalizeSuperBrightLEDs(hit map[string]any) NormalizedProduct {
	p := NormalizedProduct{Source: "superbrightleds"}
	if id, ok := hit["objectID"].(string); ok {
		p.ID = id
	}
	if sku := stringOrStringSlice(hit["sku"]); sku != "" {
		if p.ID == "" {
			p.ID = sku
		}
	}
	if title, ok := hit["name"].(string); ok {
		p.Title = title
	}
	if brand, ok := hit["brand"].(string); ok {
		p.Brand = brand
	}
	if u, ok := hit["url"].(string); ok {
		p.URL = u
	}
	if img, ok := hit["image_url"].(string); ok {
		p.ImageURL = img
	} else if img, ok := hit["thumbnail_url"].(string); ok {
		p.ImageURL = img
	}
	if desc, ok := hit["description"].(string); ok {
		p.Description = desc
	}
	p.PriceMin = superBrightLEDsPrice(hit)
	p.PriceMax = p.PriceMin
	if p.Rating = jsonFloat(hit, "rating_summary"); p.Rating > 0 {
		p.Rating = p.Rating / 20
	}
	p.ReviewCount = jsonInt(hit, "reviews_count")
	if cats, ok := hit["categories_without_path"].([]any); ok && len(cats) > 0 {
		var parts []string
		for _, raw := range cats {
			if cat, ok := raw.(string); ok && cat != "" {
				parts = append(parts, cat)
			}
		}
		p.Category = strings.Join(parts, " > ")
	} else if cats, ok := hit["categories"].(map[string]any); ok {
		if level1, ok := cats["level1"].([]any); ok && len(level1) > 0 {
			if cat, ok := level1[0].(string); ok {
				p.Category = strings.ReplaceAll(cat, " /// ", " > ")
			}
		}
	}
	return p
}

func superBrightLEDsPrice(hit map[string]any) float64 {
	price, ok := hit["price"].(map[string]any)
	if !ok {
		return 0
	}
	usd, ok := price["USD"].(map[string]any)
	if !ok {
		return 0
	}
	return jsonFloat(usd, "group_0")
}

func stringOrStringSlice(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case []any:
		for _, raw := range val {
			if s, ok := raw.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

// searchPROLIGHTING queries the public Klevu search endpoint exposed by
// PROLIGHTING's search page.
func searchPROLIGHTING(ctx context.Context, httpClient *http.Client, query string, perPage int) ([]NormalizedProduct, error) {
	if perPage <= 0 {
		perPage = 20
	}
	payload := map[string]any{
		"context": map[string]any{
			"apiKeys": []string{"klevu-172734557401917584"},
		},
		"recordQueries": []map[string]any{
			{
				"id":            "productSearch",
				"typeOfRequest": "SEARCH",
				"settings": map[string]any{
					"query":         map[string]string{"term": query},
					"typeOfRecords": []string{"KLEVU_PRODUCT"},
					"limit":         perPage,
				},
			},
		},
	}
	bodyBytes, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST", "https://uscs34v2.ksearchnet.com/cs/v2/search", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("prolighting: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("prolighting: reading body: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("prolighting: HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}

	var klevuResp struct {
		QueryResults []struct {
			ID      string           `json:"id"`
			Records []map[string]any `json:"records"`
		} `json:"queryResults"`
	}
	if err := json.Unmarshal(body, &klevuResp); err != nil {
		return nil, fmt.Errorf("prolighting: parsing response: %w", err)
	}

	var products []NormalizedProduct
	for _, result := range klevuResp.QueryResults {
		if result.ID != "productSearch" {
			continue
		}
		for _, record := range result.Records {
			products = append(products, normalizePROLIGHTING(record))
		}
	}
	return products, nil
}

func normalizePROLIGHTING(record map[string]any) NormalizedProduct {
	p := NormalizedProduct{Source: "prolighting"}
	if id, ok := record["id"].(string); ok {
		p.ID = id
	}
	if sku, ok := record["sku"].(string); ok && p.ID == "" {
		p.ID = sku
	}
	if title, ok := record["name"].(string); ok {
		p.Title = html.UnescapeString(title)
	}
	if brand, ok := record["brand"].(string); ok {
		p.Brand = brand
	}
	if u, ok := record["url"].(string); ok {
		p.URL = u
	}
	if img, ok := record["imageUrl"].(string); ok {
		p.ImageURL = img
	} else if img, ok := record["image"].(string); ok {
		p.ImageURL = img
	}
	if desc, ok := record["shortDesc"].(string); ok {
		p.Description = html.UnescapeString(desc)
	}
	p.PriceMin = jsonFloat(record, "salePrice", "price", "startPrice")
	p.PriceMax = jsonFloat(record, "toPrice", "price", "salePrice")
	if p.PriceMax == 0 {
		p.PriceMax = p.PriceMin
	}
	p.RegularPriceMin = jsonFloat(record, "basePrice")
	p.RegularPriceMax = p.RegularPriceMin
	if p.RegularPriceMin > 0 && p.PriceMin > 0 && p.PriceMin < p.RegularPriceMin {
		p.OnSale = true
		p.SalePriceMin = p.PriceMin
		p.SalePriceMax = p.PriceMax
		p.DiscountPercent = (1 - p.PriceMin/p.RegularPriceMin) * 100
	}
	if cat, ok := record["category"].(string); ok && cat != "" {
		p.Category = strings.ReplaceAll(cat, ";;", " > ")
	} else if cat, ok := record["klevu_category"].(string); ok && cat != "" {
		p.Category = strings.ReplaceAll(strings.Split(cat, "@ku@")[0], ";;", " > ")
	}
	return p
}

// search1000Bulbs queries the public 1000Bulbs HTML search results page. The
// page renders product-card attributes server-side, so no browser session or
// JavaScript execution is required for product search.
func search1000Bulbs(ctx context.Context, httpClient *http.Client, query string, perPage int) ([]NormalizedProduct, error) {
	if perPage <= 0 {
		perPage = 20
	}
	u, _ := url.Parse("https://www.1000bulbs.com/fil/search")
	q := u.Query()
	q.Set("facet.multiselect", "true")
	q.Set("page", "1")
	q.Set("q", query)
	q.Set("rows", fmt.Sprintf("%d", perPage))
	q.Set("sort", "is_backordered asc")
	q.Set("start", "0")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("1000bulbs: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("1000bulbs: reading body: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("1000bulbs: HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}

	products := extract1000BulbsProducts(string(body), perPage)
	if len(products) == 0 && strings.Contains(strings.ToLower(string(body)), "captcha") {
		return nil, fmt.Errorf("1000bulbs: blocked by captcha")
	}
	return products, nil
}

func extract1000BulbsProducts(body string, limit int) []NormalizedProduct {
	const marker = "unbxdattr='product'"
	var products []NormalizedProduct
	searchFrom := 0
	for len(products) < limit {
		idx := strings.Index(body[searchFrom:], marker)
		if idx < 0 {
			break
		}
		start := searchFrom + idx
		nextRel := strings.Index(body[start+len(marker):], marker)
		end := len(body)
		if nextRel >= 0 {
			end = start + len(marker) + nextRel
		}
		block := body[start:end]
		if p := normalize1000BulbsProduct(block); p.ID != "" && p.Title != "" {
			products = append(products, p)
		}
		searchFrom = end
	}
	return products
}

func normalize1000BulbsProduct(block string) NormalizedProduct {
	p := NormalizedProduct{Source: "1000bulbs"}
	p.ID = htmlAttr(block, "data-id")
	if p.ID == "" {
		p.ID = htmlAttr(block, "unbxdparam_sku")
	}
	p.Brand = html.UnescapeString(htmlAttr(block, "data-brand"))
	p.Title = html.UnescapeString(htmlAttr(block, "data-name"))
	p.Category = strings.ReplaceAll(html.UnescapeString(htmlAttr(block, "data-category")), "_", " ")
	if href := htmlAttr(block, "href"); href != "" {
		if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
			p.URL = href
		} else if strings.HasPrefix(href, "/") {
			p.URL = "https://www.1000bulbs.com" + href
		}
	}
	if img := htmlAttr(block, "src"); img != "" && strings.HasPrefix(img, "http") {
		p.ImageURL = html.UnescapeString(img)
	}
	if price := priceFromHTMLBlock(block); price > 0 {
		p.PriceMin = price
		p.PriceMax = price
	}
	if reviews := reviewCountFrom1000Bulbs(block); reviews > 0 {
		p.ReviewCount = reviews
		p.Rating = 5
	}
	return p
}

func searchLightingNewYork(ctx context.Context, httpClient *http.Client, query string, perPage int) ([]NormalizedProduct, error) {
	u, _ := url.Parse("https://lightingnewyork.com/search")
	q := u.Query()
	q.Set("q", query)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("lighting-new-york: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("lighting-new-york: reading body: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("lighting-new-york: HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	products := lightingNewYorkProductsFromHTML(string(body), perPage)
	return products, nil
}

func lightingNewYorkProductsFromHTML(body string, limit int) []NormalizedProduct {
	var products []NormalizedProduct
	searchFrom := 0
	for {
		idx := strings.Index(body[searchFrom:], `data-gtmdata=`)
		if idx < 0 {
			break
		}
		start := searchFrom + idx
		blockEnd := strings.Index(body[start:], `</div>`)
		end := len(body)
		if blockEnd >= 0 {
			end = start + blockEnd + len(`</div>`)
		}
		block := body[start:end]
		if p := normalizeLightingNewYorkProduct(block); p.ID != "" && p.Title != "" && p.PriceMin > 0 {
			products = append(products, p)
			if limit > 0 && len(products) >= limit {
				break
			}
		}
		searchFrom = end
	}
	return products
}

func normalizeLightingNewYorkProduct(block string) NormalizedProduct {
	raw := strings.TrimSpace(html.UnescapeString(htmlAttr(block, "data-gtmdata")))
	if raw == "" {
		return NormalizedProduct{Source: "lighting-new-york"}
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return NormalizedProduct{Source: "lighting-new-york"}
	}

	p := NormalizedProduct{Source: "lighting-new-york"}
	if sku, ok := data["item_sku"].(string); ok && strings.TrimSpace(sku) != "" {
		p.ID = normalizeSelectionSKU(sku)
	}
	if p.ID == "" {
		if id, ok := data["id"].(string); ok {
			p.ID = normalizeSelectionSKU(id)
		}
	}
	if title, ok := data["name"].(string); ok {
		p.Title = html.UnescapeString(title)
	}
	if p.Title == "" {
		if title, ok := data["item_name"].(string); ok {
			p.Title = html.UnescapeString(title)
		}
	}
	if brand, ok := data["brand"].(string); ok {
		p.Brand = html.UnescapeString(brand)
	}
	if category, ok := data["category"].(string); ok {
		p.Category = html.UnescapeString(category)
	}
	if productURL, ok := data["productURL"].(string); ok {
		p.URL = absoluteURL("https://lightingnewyork.com", html.UnescapeString(productURL))
	}
	if imageURL, ok := data["imageURL"].(string); ok {
		p.ImageURL = absoluteURL("https://lightingnewyork.com", html.UnescapeString(imageURL))
	}
	p.PriceMin = jsonFloat(data, "price")
	p.PriceMax = p.PriceMin
	p.RegularPriceMin = jsonFloat(data, "compareAtPrice")
	if p.RegularPriceMin > 0 && p.PriceMin > 0 && p.PriceMin < p.RegularPriceMin {
		p.OnSale = true
		p.SalePriceMin = p.PriceMin
		p.SalePriceMax = p.PriceMax
		p.DiscountPercent = (1 - p.PriceMin/p.RegularPriceMin) * 100
	}
	return p
}

func searchLightology(ctx context.Context, httpClient *http.Client, query string, perPage int) ([]NormalizedProduct, error) {
	route, err := lightologyRouteForQuery(query)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, route, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("lightology: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("lightology: reading body: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("lightology: HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	products := lightologyProductsFromHTML(string(body), perPage)
	return products, nil
}

func lightologyRouteForQuery(query string) (string, error) {
	q := strings.ToLower(query)
	if strings.Contains(q, "ceiling fan") {
		return "https://www.lightology.com/index.php?module=cat&cat_id=49", nil
	}
	if strings.Contains(q, "pendant") || strings.Contains(q, "chandelier") || strings.Contains(q, "hanging light") || strings.Contains(q, "ceiling light") {
		return "https://www.lightology.com/index.php?module=cat&cat_id=106", nil
	}
	return "", fmt.Errorf("lightology: no category route for query %q", query)
}

func lightologyProductsFromHTML(body string, limit int) []NormalizedProduct {
	var products []NormalizedProduct
	searchFrom := 0
	for {
		idx := strings.Index(body[searchFrom:], `data-gtm_list_item=`)
		if idx < 0 {
			break
		}
		start := searchFrom + idx
		end := len(body)
		next := strings.Index(body[start+len(`data-gtm_list_item=`):], `data-gtm_list_item=`)
		if next >= 0 {
			end = start + len(`data-gtm_list_item=`) + next
		}
		block := body[start:end]
		if p := normalizeLightologyProduct(block); p.ID != "" && p.Title != "" && p.PriceMin > 0 {
			products = append(products, p)
			if limit > 0 && len(products) >= limit {
				break
			}
		}
		searchFrom = end
	}
	return products
}

func normalizeLightologyProduct(block string) NormalizedProduct {
	raw := strings.TrimSpace(html.UnescapeString(htmlAttr(block, "data-gtm_list_item")))
	if raw == "" {
		return NormalizedProduct{Source: "lightology"}
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return NormalizedProduct{Source: "lightology"}
	}

	p := NormalizedProduct{Source: "lightology"}
	if sku, ok := data["item_id"].(string); ok {
		p.ID = normalizeSelectionSKU(sku)
	}
	if p.ID == "" {
		if id, ok := data["item_main_id"].(string); ok {
			p.ID = normalizeSelectionSKU(id)
		}
	}
	if title, ok := data["item_name"].(string); ok {
		p.Title = html.UnescapeString(title)
	}
	if brand, ok := data["item_brand"].(string); ok {
		p.Brand = html.UnescapeString(brand)
	}
	p.Category = lightologyCategory(data)
	p.PriceMin = jsonFloat(data, "price")
	p.PriceMax = p.PriceMin
	if min, max := lightologyDisplayedPriceRange(block); min > 0 {
		p.PriceMin = min
		p.PriceMax = max
	}
	if href := lightologyProductHref(block); href != "" {
		p.URL = absoluteURL("https://www.lightology.com", html.UnescapeString(href))
	}
	if p.URL == "" {
		if mainID, ok := data["item_main_id"].(string); ok && strings.TrimSpace(mainID) != "" {
			p.URL = "https://www.lightology.com/index.php?module=prod_detail&prod_id=" + url.QueryEscape(strings.TrimSpace(mainID)) + "&cat_id=106"
		}
	}
	if img := lightologyImageURL(block); img != "" {
		p.ImageURL = absoluteURL("https://www.lightology.com", html.UnescapeString(img))
	}
	return p
}

func lightologyCategory(data map[string]any) string {
	var parts []string
	for _, key := range []string{"item_category", "item_category2", "item_category3"} {
		if value, ok := data[key].(string); ok && strings.TrimSpace(value) != "" {
			parts = append(parts, html.UnescapeString(strings.TrimSpace(value)))
		}
	}
	return strings.Join(parts, " > ")
}

func lightologyDisplayedPriceRange(block string) (float64, float64) {
	re := regexp.MustCompile(`(?is)<span[^>]+class="[^"]*bigprice[^"]*"[^>]*>(.*?)</span>`)
	match := re.FindStringSubmatch(block)
	if len(match) <= 1 {
		return 0, 0
	}
	text := strings.Join(strings.Fields(stripHTMLTags(match[1])), " ")
	prices := regexp.MustCompile(`\$[0-9][0-9,]*(?:\.[0-9]{2})?`).FindAllString(text, -1)
	if len(prices) == 0 {
		return 0, 0
	}
	min := parsePriceString(prices[0])
	max := min
	if len(prices) > 1 {
		max = parsePriceString(prices[len(prices)-1])
	}
	return min, max
}

func lightologyProductHref(block string) string {
	match := regexp.MustCompile(`(?is)<a[^>]+href="([^"]*module=prod_detail[^"]+)"`).FindStringSubmatch(block)
	if len(match) > 1 {
		return match[1]
	}
	return htmlAttr(block, "href")
}

func lightologyImageURL(block string) string {
	for _, attr := range []string{"data-pin-media", "src"} {
		if value := htmlAttr(block, attr); value != "" {
			return value
		}
	}
	return ""
}

func searchKBAuthority(ctx context.Context, httpClient *http.Client, query string, perPage int) ([]NormalizedProduct, error) {
	u, _ := url.Parse("https://api.searchspring.net/api/search/autocomplete.json")
	q := u.Query()
	q.Set("siteId", "9y1cqt")
	q.Set("q", query)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("kbauthority: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("kbauthority: reading body: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("kbauthority: HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}

	var payload struct {
		Results string `json:"results"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("kbauthority: decoding searchspring response: %w", err)
	}
	products := kbauthorityProductsFromHTML(payload.Results, perPage)
	return products, nil
}

func kbauthorityProductsFromHTML(body string, limit int) []NormalizedProduct {
	var products []NormalizedProduct
	searchFrom := 0
	for {
		idx := strings.Index(body[searchFrom:], `<div class="item">`)
		if idx < 0 {
			break
		}
		start := searchFrom + idx
		blockEnd := strings.Index(body[start:], `</div>`)
		end := len(body)
		if blockEnd >= 0 {
			end = start + blockEnd + len(`</div>`)
		}
		block := body[start:end]
		if p := normalizeKBAuthorityProduct(block); p.ID != "" && p.Title != "" && p.PriceMin > 0 {
			products = append(products, p)
			if limit > 0 && len(products) >= limit {
				break
			}
		}
		searchFrom = end
	}
	return products
}

func normalizeKBAuthorityProduct(block string) NormalizedProduct {
	p := NormalizedProduct{Source: "kbauthority"}
	if href := firstKBAuthorityHref(block); href != "" {
		p.URL = html.UnescapeString(href)
	}
	if img := firstKBAuthorityImage(block); img != "" {
		p.ImageURL = absoluteURL("https://www.kbauthority.com", html.UnescapeString(img))
	}
	if name := firstKBAuthorityClassText(block, "name"); name != "" {
		p.Title = strings.Join(strings.Fields(html.UnescapeString(name)), " ")
	}
	if price := firstKBAuthorityClassText(block, "price"); price != "" {
		p.PriceMin = parsePriceString(price)
		p.PriceMax = p.PriceMin
	}
	p.Brand = brandFromKBAuthorityTitle(p.Title)
	p.ID = kbauthoritySelectionID(p.Title, p.URL)
	return p
}

func firstKBAuthorityHref(block string) string {
	match := regexp.MustCompile(`(?is)<p\s+class="name"[^>]*>.*?<a[^>]+href="([^"]+)"`).FindStringSubmatch(block)
	if len(match) > 1 {
		return match[1]
	}
	return htmlAttr(block, "href")
}

func firstKBAuthorityImage(block string) string {
	match := regexp.MustCompile(`(?is)<p\s+class="image"[^>]*>.*?<img[^>]+src="([^"]+)"`).FindStringSubmatch(block)
	if len(match) > 1 {
		return match[1]
	}
	return htmlAttr(block, "src")
}

func firstKBAuthorityClassText(block, className string) string {
	re := regexp.MustCompile(`(?is)<p\s+class="` + regexp.QuoteMeta(className) + `"[^>]*>(.*?)</p>`)
	match := re.FindStringSubmatch(block)
	if len(match) <= 1 {
		return ""
	}
	return stripHTMLTags(match[1])
}

func brandFromKBAuthorityTitle(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return ""
	}
	parts := strings.Fields(title)
	var brandParts []string
	for _, part := range parts {
		if strings.ContainsAny(part, "0123456789") {
			break
		}
		brandParts = append(brandParts, part)
	}
	if len(brandParts) == 0 {
		return ""
	}
	if len(brandParts) > 5 {
		brandParts = brandParts[:5]
	}
	return titleCaseWords(strings.Join(brandParts, " "))
}

func titleCaseWords(s string) string {
	words := strings.Fields(strings.ToLower(s))
	for i, word := range words {
		if word == "" {
			continue
		}
		words[i] = strings.ToUpper(word[:1]) + word[1:]
	}
	return strings.Join(words, " ")
}

func kbauthoritySelectionID(title, productURL string) string {
	for _, source := range []string{title, productURL} {
		for _, token := range regexp.MustCompile(`[A-Za-z0-9][A-Za-z0-9._/-]{3,}`).FindAllString(source, -1) {
			token = strings.Trim(token, " .,/;:'\"")
			if !strings.ContainsAny(token, "0123456789") || strings.HasPrefix(strings.ToLower(token), "http") {
				continue
			}
			switch strings.ToUpper(token) {
			case "2026", "24", "30", "36", "39", "42", "48", "55", "60", "72", "84":
				continue
			}
			if sku := normalizeSelectionSKU(token); sku != "" {
				return sku
			}
		}
	}
	return ""
}

func searchVintageTub(ctx context.Context, httpClient *http.Client, query string, perPage int) ([]NormalizedProduct, error) {
	u, _ := url.Parse("https://api.searchspring.net/api/search/search.json")
	q := u.Query()
	q.Set("siteId", "yncuaq")
	q.Set("resultsFormat", "native")
	q.Set("q", query)
	q.Set("resultsPerPage", fmt.Sprintf("%d", perPage))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("vintage-tub: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("vintage-tub: reading body: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("vintage-tub: HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}

	var searchResp struct {
		Results []map[string]any `json:"results"`
	}
	if err := json.Unmarshal(body, &searchResp); err != nil {
		return nil, fmt.Errorf("vintage-tub: decoding searchspring response: %w", err)
	}
	products := make([]NormalizedProduct, 0, minInt(len(searchResp.Results), perPage))
	for _, raw := range searchResp.Results {
		p := normalizeVintageTubProduct(raw)
		if p.ID == "" || p.Title == "" || p.PriceMin <= 0 {
			continue
		}
		products = append(products, p)
		if perPage > 0 && len(products) >= perPage {
			break
		}
	}
	return products, nil
}

func normalizeVintageTubProduct(raw map[string]any) NormalizedProduct {
	p := NormalizedProduct{Source: "vintage-tub"}
	p.ID = normalizeSelectionSKU(jsonString(raw, "sku"))
	if p.ID == "" {
		p.ID = normalizeSelectionSKU(jsonString(raw, "id", "uid"))
	}
	p.Title = strings.Join(strings.Fields(html.UnescapeString(jsonString(raw, "name"))), " ")
	p.Brand = strings.TrimSpace(html.UnescapeString(jsonString(raw, "brand")))
	p.URL = html.UnescapeString(jsonString(raw, "url"))
	p.ImageURL = html.UnescapeString(jsonString(raw, "imageUrl", "thumbnailImageUrl"))
	p.PriceMin = jsonFloat(raw, "price")
	p.PriceMax = p.PriceMin
	p.Category = vintageTubCategory(raw)
	p.Description = vintageTubDescription(jsonString(raw, "description"))
	return p
}

func vintageTubCategory(raw map[string]any) string {
	hierarchy, ok := raw["category_hierarchy"].([]any)
	if !ok {
		return html.UnescapeString(jsonString(raw, "product_type_unigram"))
	}
	var best string
	for _, item := range hierarchy {
		s, ok := item.(string)
		if !ok {
			continue
		}
		s = html.UnescapeString(s)
		if !strings.Contains(s, ">") {
			continue
		}
		parts := strings.Split(s, ">")
		candidate := strings.TrimSpace(parts[len(parts)-1])
		if candidate != "" && !strings.EqualFold(candidate, "Default Category") {
			best = candidate
		}
	}
	if best != "" {
		return best
	}
	return html.UnescapeString(jsonString(raw, "product_type_unigram"))
}

func vintageTubDescription(raw string) string {
	raw = strings.ReplaceAll(html.UnescapeString(raw), "|", "; ")
	return strings.Join(strings.Fields(stripHTMLTags(raw)), " ")
}

func searchSignatureHardware(ctx context.Context, httpClient *http.Client, query string, perPage int) ([]NormalizedProduct, error) {
	u, _ := url.Parse("https://www.signaturehardware.com/on/demandware.store/Sites-SignatureHardware-Site/default/SearchServices-GetSuggestions")
	q := u.Query()
	q.Set("q", query)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Referer", "https://www.signaturehardware.com/")
	req.Header.Set("User-Agent", sourceProbeUserAgent)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("signature-hardware: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("signature-hardware: reading body: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("signature-hardware: HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	products := signatureHardwareProductsFromHTML(string(body), perPage)
	return products, nil
}

func signatureHardwareProductsFromHTML(body string, limit int) []NormalizedProduct {
	var products []NormalizedProduct
	seen := map[string]bool{}
	positions := regexp.MustCompile(`<div[^>]+class="[^"]*search-suggest-product[^"]*"`).FindAllStringIndex(body, -1)
	for i, pos := range positions {
		start := pos[0]
		end := len(body)
		if i+1 < len(positions) {
			end = positions[i+1][0]
		}
		p := normalizeSignatureHardwareProduct(body[start:end])
		if p.ID == "" || p.Title == "" || p.PriceMin <= 0 {
			continue
		}
		dedupeKey := firstNonEmptyString(p.URL, p.ID)
		if seen[dedupeKey] {
			continue
		}
		seen[dedupeKey] = true
		products = append(products, p)
		if limit > 0 && len(products) >= limit {
			break
		}
	}
	return products
}

func normalizeSignatureHardwareProduct(block string) NormalizedProduct {
	p := NormalizedProduct{Source: "signature-hardware", Brand: "Signature Hardware", Category: "Bathroom > Showers"}
	p.URL = signatureHardwareProductURL(block)
	p.ID = normalizeSelectionSKU(signatureHardwareIDFromURL(p.URL))
	p.Title = strings.Join(strings.Fields(html.UnescapeString(signatureHardwareProductTitle(block))), " ")
	p.ImageURL = signatureHardwareImageURL(block)
	p.PriceMin = signatureHardwarePrice(block)
	p.PriceMax = p.PriceMin
	return p
}

func signatureHardwareProductURL(block string) string {
	match := regexp.MustCompile(`(?is)<a[^>]+class="[^"]*suggestion-link[^"]*"[^>]+href="([^"]+)"`).FindStringSubmatch(block)
	if len(match) < 2 {
		match = regexp.MustCompile(`(?is)<a[^>]+class="[^"]*link[^"]*"[^>]+href="([^"]+)"`).FindStringSubmatch(block)
	}
	if len(match) < 2 {
		return ""
	}
	return absoluteURL("https://www.signaturehardware.com", html.UnescapeString(match[1]))
}

func signatureHardwareIDFromURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	base := parsed.Path
	if slash := strings.LastIndex(base, "/"); slash >= 0 {
		base = base[slash+1:]
	}
	base = strings.TrimSuffix(base, ".html")
	return base
}

func signatureHardwareProductTitle(block string) string {
	match := regexp.MustCompile(`(?is)<a[^>]+class="[^"]*link[^"]*"[^>]*>(.*?)</a>`).FindStringSubmatch(block)
	if len(match) > 1 {
		if title := strings.TrimSpace(stripHTMLTags(match[1])); title != "" {
			return title
		}
	}
	if alt := htmlAttr(block, "alt"); alt != "" {
		return alt
	}
	return htmlAttr(block, "aria-label")
}

func signatureHardwareImageURL(block string) string {
	src := htmlAttr(block, "src")
	if strings.HasPrefix(src, "http://") {
		src = "https://" + strings.TrimPrefix(src, "http://")
	}
	return html.UnescapeString(src)
}

func signatureHardwarePrice(block string) float64 {
	match := regexp.MustCompile(`class="value"\s+content="([0-9]+(?:\.[0-9]+)?)"`).FindStringSubmatch(block)
	if len(match) > 1 {
		return parsePriceString(match[1])
	}
	match = regexp.MustCompile(`\$[0-9][0-9,]*(?:<sup>[0-9]{2}</sup>|\.[0-9]{2})?`).FindStringSubmatch(block)
	if len(match) == 0 {
		return 0
	}
	price := strings.ReplaceAll(match[0], "<sup>", ".")
	price = strings.ReplaceAll(price, "</sup>", "")
	return parsePriceString(stripHTMLTags(price))
}

func htmlAttr(block, name string) string {
	for _, quote := range []byte{'"', '\''} {
		needle := name + "=" + string(quote)
		start := strings.Index(block, needle)
		if start < 0 {
			continue
		}
		start += len(needle)
		end := strings.IndexByte(block[start:], quote)
		if end < 0 {
			continue
		}
		return block[start : start+end]
	}
	return ""
}

func priceFromHTMLBlock(block string) float64 {
	priceIdx := strings.Index(block, "class='price'")
	if priceIdx < 0 {
		priceIdx = strings.Index(block, `class="price"`)
	}
	if priceIdx < 0 {
		return 0
	}
	start := strings.Index(block[priceIdx:], ">")
	if start < 0 {
		return 0
	}
	start += priceIdx + 1
	end := strings.Index(block[start:], "</div>")
	if end < 0 {
		return 0
	}
	text := stripHTMLTags(block[start : start+end])
	text = strings.TrimSpace(html.UnescapeString(text))
	text = strings.TrimPrefix(text, "$")
	text = strings.ReplaceAll(text, ",", "")
	return parsePriceString(text)
}

func stripHTMLTags(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch r {
		case '<':
			inTag = true
		case '>':
			inTag = false
		default:
			if !inTag {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}

func reviewCountFrom1000Bulbs(block string) int {
	ratingIdx := strings.Index(block, "ratings-container")
	if ratingIdx < 0 {
		return 0
	}
	part := block[ratingIdx:]
	open := strings.Index(part, "<span>(")
	if open < 0 {
		return 0
	}
	open += len("<span>(")
	close := strings.Index(part[open:], ")</span>")
	if close < 0 {
		return 0
	}
	var count int
	if _, err := fmt.Sscanf(part[open:open+close], "%d", &count); err == nil {
		return count
	}
	return 0
}

type plumbersStockRoute struct {
	Path     string
	Category string
}

// searchPlumbersStock queries PlumbersStock category pages that match Reno
// Goat's plumbing fixture scope. PlumbersStock has no replayable anonymous
// search endpoint; unsupported terms intentionally return no rows rather than
// broadening into commodity parts, tools, pipe, or waterworks categories.
func searchPlumbersStock(ctx context.Context, httpClient *http.Client, query string, perPage int) ([]NormalizedProduct, error) {
	if perPage <= 0 {
		perPage = 20
	}
	routes := plumbersStockRoutesForQuery(query)
	if len(routes) == 0 {
		return []NormalizedProduct{}, nil
	}

	var products []NormalizedProduct
	seen := map[string]bool{}
	for _, route := range routes {
		if len(products) >= perPage {
			break
		}
		req, err := http.NewRequestWithContext(ctx, "GET", "https://www.plumbersstock.com"+route.Path, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "text/html,application/xhtml+xml")
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36")

		resp, err := httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("plumbersstock: %w", err)
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("plumbersstock: reading body: %w", readErr)
		}
		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("plumbersstock: HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
		}

		for _, p := range extractPlumbersStockProducts(string(body), route.Category, perPage) {
			key := p.Source + "|" + p.ID
			if seen[key] {
				continue
			}
			seen[key] = true
			products = append(products, p)
			if len(products) >= perPage {
				break
			}
		}
	}
	return products, nil
}

func plumbersStockRoutesForQuery(query string) []plumbersStockRoute {
	q := strings.ToLower(query)
	routes := make([]plumbersStockRoute, 0, 3)
	add := func(path, category string) {
		for _, existing := range routes {
			if existing.Path == path {
				return
			}
		}
		routes = append(routes, plumbersStockRoute{Path: path, Category: category})
	}

	if containsAny(q, "faucet", "tap", "lavatory faucet", "kitchen faucet", "bathroom faucet") {
		add("/bathroom/faucets.html", "Bathroom > Faucets")
		add("/kitchen/faucets.html", "Kitchen > Faucets")
		add("/laundry-room/faucets.html", "Laundry Room > Faucets")
	}
	if containsAny(q, "sink", "lavatory", "basin") {
		add("/bathroom/sinks.html", "Bathroom > Sinks")
	}
	if containsAny(q, "toilet", "washlet", "bidet") {
		add("/bathroom/toilets.html", "Bathroom > Toilets")
	}
	if containsAny(q, "shower", "tub", "bathtub", "soaking tub", "tub shower") {
		add("/bathroom/tubs-showers.html", "Bathroom > Tubs & Showers")
	}
	return routes
}

func containsAny(s string, terms ...string) bool {
	for _, term := range terms {
		if strings.Contains(s, term) {
			return true
		}
	}
	return false
}

func extractPlumbersStockProducts(body, category string, limit int) []NormalizedProduct {
	const marker = `aria-label="View category `
	var products []NormalizedProduct
	searchFrom := 0
	for len(products) < limit {
		idx := strings.Index(body[searchFrom:], marker)
		if idx < 0 {
			break
		}
		labelStart := searchFrom + idx
		linkStart := strings.LastIndex(body[:labelStart], "<a ")
		if linkStart < 0 {
			searchFrom = labelStart + len(marker)
			continue
		}
		nextRel := strings.Index(body[labelStart+len(marker):], marker)
		end := len(body)
		if nextRel >= 0 {
			end = labelStart + len(marker) + nextRel
		}
		block := body[linkStart:end]
		if p := normalizePlumbersStockProduct(block, category); p.ID != "" && p.Title != "" && p.PriceMin > 0 {
			products = append(products, p)
		}
		searchFrom = end
	}
	return products
}

func normalizePlumbersStockProduct(block, category string) NormalizedProduct {
	p := NormalizedProduct{Source: "plumbersstock", Category: category}
	href := html.UnescapeString(htmlAttr(block, "href"))
	if strings.HasPrefix(href, "/") {
		p.URL = "https://www.plumbersstock.com" + href
	} else if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		p.URL = href
	}
	p.ID = plumbersStockIDFromURL(p.URL)
	title := html.UnescapeString(htmlAttr(block, "aria-label"))
	title = strings.TrimSpace(strings.TrimPrefix(title, "View category "))
	p.Title = title
	p.Brand = plumbersStockBrandFromBlock(block)
	if p.Brand == "" {
		p.Brand = firstWord(title)
	}
	if img := html.UnescapeString(htmlAttr(block, "src")); strings.HasPrefix(img, "http") {
		p.ImageURL = img
	}
	prices := pricesFromPlumbersStockBlock(block)
	if len(prices) > 0 {
		p.PriceMin = prices[0]
		p.PriceMax = prices[0]
	}
	if len(prices) > 1 && prices[1] > prices[0] {
		p.RegularPriceMin = prices[1]
		p.RegularPriceMax = prices[1]
		p.SalePriceMin = prices[0]
		p.SalePriceMax = prices[0]
		p.OnSale = true
		p.DiscountPercent = (1 - prices[0]/prices[1]) * 100
	}
	return p
}

func plumbersStockIDFromURL(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	path := rawURL
	if u, err := url.Parse(rawURL); err == nil {
		path = u.Path
	}
	path = strings.TrimSuffix(path, ".html")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

func plumbersStockBrandFromBlock(block string) string {
	start := strings.Index(block, "<strong>")
	if start < 0 {
		return ""
	}
	start += len("<strong>")
	end := strings.Index(block[start:], "</strong>")
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(html.UnescapeString(stripHTMLTags(block[start : start+end])))
}

func firstWord(s string) string {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return ""
	}
	return strings.Trim(fields[0], " ,.-")
}

func pricesFromPlumbersStockBlock(block string) []float64 {
	var prices []float64
	searchFrom := 0
	for len(prices) < 2 {
		idx := strings.Index(block[searchFrom:], "$")
		if idx < 0 {
			break
		}
		start := searchFrom + idx + 1
		end := start
		for end < len(block) && end-start < 80 {
			ch := block[end]
			if ch == '<' || ch == '>' || ch == '!' || ch == '-' || ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' || ch == ',' || ch == '.' || (ch >= '0' && ch <= '9') {
				end++
				continue
			}
			break
		}
		frag := block[start:end]
		frag = strings.ReplaceAll(frag, "<!-- -->", "")
		frag = strings.ReplaceAll(frag, ",", "")
		frag = strings.TrimSpace(stripHTMLTags(frag))
		if price := parsePriceString(frag); price > 0 {
			prices = append(prices, price)
		}
		searchFrom = end
	}
	return prices
}

type iwaeRoute struct {
	Path     string
	Category string
}

// searchIWAe queries IWAe/Ingrams Water & Air HVAC category pages. Their
// category page renders Hyva/Magento product cards with stable SKU, URL, title,
// product ID, and price attributes. The generic search route is noisier and the
// ductless category currently returns product-bearing HTML with a 404 status, so
// this source intentionally stays on the stable full-systems route.
func searchIWAe(ctx context.Context, httpClient *http.Client, query string, perPage int) ([]NormalizedProduct, error) {
	if perPage <= 0 {
		perPage = 20
	}
	routes := iwaeRoutesForQuery(query)
	if len(routes) == 0 {
		return nil, nil
	}

	var products []NormalizedProduct
	seen := map[string]bool{}
	for _, route := range routes {
		req, err := http.NewRequestWithContext(ctx, "GET", "https://iwae.com"+route.Path, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "text/html,application/xhtml+xml")
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36")

		resp, err := httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("iwae: %w", err)
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("iwae: reading body: %w", readErr)
		}
		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("iwae: HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
		}

		for _, p := range extractIWAeProducts(string(body), route.Category, perPage) {
			key := p.ID
			if key == "" {
				key = p.URL
			}
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			products = append(products, p)
			if len(products) >= perPage {
				break
			}
		}
		if len(products) >= perPage {
			break
		}
	}
	return products, nil
}

func iwaeRoutesForQuery(query string) []iwaeRoute {
	q := strings.ToLower(query)
	routes := make([]iwaeRoute, 0, 1)
	add := func(path, category string) {
		for _, existing := range routes {
			if existing.Path == path {
				return
			}
		}
		routes = append(routes, iwaeRoute{Path: path, Category: category})
	}

	if containsAny(q, "hvac", "mini split", "minisplit", "ductless", "furnace", "air conditioner", "central air", "condenser", "air handler", "full system", "split system", "package unit", "heat pump") {
		add("/shop/heating-air-conditioning/full-systems/", "HVAC > Full Systems")
	}
	return routes
}

func extractIWAeProducts(body, category string, limit int) []NormalizedProduct {
	const marker = ` product-item `
	var products []NormalizedProduct
	searchFrom := 0
	for len(products) < limit {
		idx := strings.Index(body[searchFrom:], marker)
		if idx < 0 {
			break
		}
		markerStart := searchFrom + idx
		formStart := strings.LastIndex(body[:markerStart], "<form ")
		if formStart < 0 {
			searchFrom = markerStart + len(marker)
			continue
		}
		formEndRel := strings.Index(body[markerStart:], "</form>")
		if formEndRel < 0 {
			break
		}
		formEnd := markerStart + formEndRel + len("</form>")
		block := body[formStart:formEnd]
		if p := normalizeIWAeProduct(block, category); p.ID != "" && p.Title != "" && p.PriceMin > 0 && isIWAeSelectionProduct(p.Title) {
			products = append(products, p)
		}
		searchFrom = formEnd
	}
	return products
}

func normalizeIWAeProduct(block, category string) NormalizedProduct {
	p := NormalizedProduct{Source: "iwae", Category: category}
	sku := strings.TrimSpace(html.UnescapeString(htmlAttr(block, "data-sku")))
	p.ID = sku
	if productID := htmlAttr(block, "data-product-id"); productID != "" {
		if sku != "" {
			p.ID = sku + "-" + productID
		} else {
			p.ID = productID
		}
	}
	p.Brand = iwaeBrandFromTitle(block)

	href := html.UnescapeString(htmlAttr(block, "href"))
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		p.URL = href
	} else if strings.HasPrefix(href, "/") {
		p.URL = "https://iwae.com" + href
	}
	title := strings.TrimSpace(html.UnescapeString(htmlAttr(block, "title")))
	if title == "" {
		title = strings.TrimSpace(html.UnescapeString(textBetween(block, `class="product-item-link`, "</a>")))
		title = stripHTMLTags(title)
	}
	p.Title = title
	if img := html.UnescapeString(htmlAttr(block, "src")); strings.HasPrefix(img, "http") {
		p.ImageURL = img
	}
	if price := parsePriceString(strings.ReplaceAll(htmlAttr(block, "data-price-amount"), ",", "")); price > 0 {
		p.PriceMin = price
		p.PriceMax = price
	}
	return p
}

func isIWAeSelectionProduct(title string) bool {
	t := strings.ToLower(title)
	if containsAny(t, "warranty", "control board", "circuit board", "line set", "adapter", "replacement", "capacitor", "compressor part", "repair part") {
		return false
	}
	return containsAny(t, "mini split", "ductless", "heat pump", "condenser", "air handler", "furnace", "air conditioner", "package unit", "split system")
}

func iwaeBrandFromTitle(block string) string {
	title := strings.ToLower(html.UnescapeString(htmlAttr(block, "title")))
	brands := []string{"MrCool", "Goodman", "Daikin", "Mitsubishi", "LG", "Gree", "Bosch", "Fujitsu", "Rheem", "RunTru", "Oxbox", "ACiQ"}
	for _, brand := range brands {
		if strings.Contains(title, strings.ToLower(brand)) {
			return brand
		}
	}
	return ""
}

func textBetween(s, startMarker, endMarker string) string {
	start := strings.Index(s, startMarker)
	if start < 0 {
		return ""
	}
	start += len(startMarker)
	if close := strings.Index(s[start:], ">"); close >= 0 {
		start += close + 1
	}
	end := strings.Index(s[start:], endMarker)
	if end < 0 {
		return ""
	}
	return s[start : start+end]
}

// searchHardwareHut queries The Hardware Hut's public search page. Search
// results are rendered as a window.shs_products JSON array in the HTML.
func searchHardwareHut(ctx context.Context, httpClient *http.Client, query string, perPage int) ([]NormalizedProduct, error) {
	if perPage <= 0 {
		perPage = 20
	}
	u, _ := url.Parse("https://hardwarehut.com/search")
	q := u.Query()
	q.Set("search_prod_no", query)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("hardware-hut: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("hardware-hut: reading body: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("hardware-hut: HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}

	rawProducts, err := extractHardwareHutProducts(string(body))
	if err != nil {
		return nil, err
	}
	if len(rawProducts) > perPage {
		rawProducts = rawProducts[:perPage]
	}
	products := make([]NormalizedProduct, 0, len(rawProducts))
	for _, raw := range rawProducts {
		products = append(products, normalizeHardwareHut(raw))
	}
	return products, nil
}

func extractHardwareHutProducts(body string) ([]map[string]any, error) {
	const marker = "window.shs_products ="
	start := strings.Index(body, marker)
	if start < 0 {
		return nil, fmt.Errorf("hardware-hut: products array not found")
	}
	start += len(marker)
	for start < len(body) && (body[start] == ' ' || body[start] == '\t' || body[start] == '\n' || body[start] == '\r') {
		start++
	}
	if start >= len(body) || body[start] != '[' {
		return nil, fmt.Errorf("hardware-hut: products array has unexpected shape")
	}

	end := findJSONArrayEnd(body, start)
	if end < 0 {
		return nil, fmt.Errorf("hardware-hut: products array was unterminated")
	}

	var products []map[string]any
	dec := json.NewDecoder(strings.NewReader(body[start : end+1]))
	dec.UseNumber()
	if err := dec.Decode(&products); err != nil {
		return nil, fmt.Errorf("hardware-hut: parsing products: %w", err)
	}
	return products, nil
}

func findJSONArrayEnd(s string, start int) int {
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(s); i++ {
		ch := s[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			switch ch {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}

		switch ch {
		case '"':
			inString = true
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func normalizeHardwareHut(item map[string]any) NormalizedProduct {
	p := NormalizedProduct{Source: "hardware-hut"}
	if id, ok := item["p_ref"].(string); ok {
		p.ID = id
	}
	if sku, ok := item["p_num"].(string); ok && p.ID == "" {
		p.ID = sku
	}
	if title, ok := item["productFinalName"].(string); ok {
		p.Title = html.UnescapeString(title)
	} else if title, ok := item["p_name"].(string); ok {
		p.Title = html.UnescapeString(title)
	}
	if sku, ok := item["p_num"].(string); ok {
		p.Brand = hardwareHutBrandFromSKU(sku)
	}
	if img, ok := item["finalimage"].(string); ok {
		p.ImageURL = img
	}
	if slug, ok := item["producturlprefix"].(string); ok && slug != "" {
		p.URL = "https://hardwarehut.com/products/" + slug
	} else if u, ok := item["p_url"].(string); ok && u != "" {
		if strings.HasPrefix(u, "/") {
			p.URL = "https://hardwarehut.com" + u
		} else {
			p.URL = "https://hardwarehut.com/products/" + u
		}
	}

	p.PriceMin, p.PriceMax = hardwareHutPriceRange(item["finalprice"])
	if p.PriceMin == 0 {
		p.PriceMin = jsonFloat(item, "p_price")
		p.PriceMax = p.PriceMin
	}
	if reviews, ok := item["product_reviews"].(map[string]any); ok {
		p.Rating = jsonFloat(reviews, "rating")
		p.ReviewCount = jsonInt(reviews, "total")
	}
	if cat, ok := item["cat_url"].(string); ok {
		p.Category = strings.ReplaceAll(cat, "-", " ")
	}
	return p
}

func hardwareHutPriceRange(raw any) (float64, float64) {
	switch v := raw.(type) {
	case []any:
		if len(v) == 2 {
			if _, ok := v[0].([]any); !ok {
				if _, ok := v[1].([]any); !ok {
					price := hardwareHutPricePart(v)
					return price, price
				}
			}
		}
		var vals []float64
		for _, part := range v {
			if price := hardwareHutPricePart(part); price > 0 {
				vals = append(vals, price)
			}
		}
		if len(vals) == 0 {
			return 0, 0
		}
		if len(vals) == 1 {
			return vals[0], vals[0]
		}
		return vals[0], vals[len(vals)-1]
	case float64:
		return v, v
	case json.Number:
		if f, err := v.Float64(); err == nil {
			return f, f
		}
	case string:
		return parsePriceString(v), parsePriceString(v)
	}
	return 0, 0
}

func hardwareHutPricePart(raw any) float64 {
	switch v := raw.(type) {
	case []any:
		if len(v) >= 2 {
			return parsePriceString(fmt.Sprintf("%v.%v", v[0], v[1]))
		}
	case string:
		return parsePriceString(v)
	case float64:
		return v
	case json.Number:
		if f, err := v.Float64(); err == nil {
			return f
		}
	}
	return 0
}

func parsePriceString(s string) float64 {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "$")
	s = strings.ReplaceAll(s, ",", "")
	var f float64
	if _, err := fmt.Sscanf(s, "%f", &f); err == nil {
		return f
	}
	return 0
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func hardwareHutBrandFromSKU(sku string) string {
	if before, _, ok := strings.Cut(sku, "-"); ok && before != "" {
		return before
	}
	return ""
}

func extractShopifyPrice(node map[string]any, rangeKey, variantKey string) float64 {
	priceRange, ok := node[rangeKey].(map[string]any)
	if !ok {
		return 0
	}
	variant, ok := priceRange[variantKey].(map[string]any)
	if !ok {
		return 0
	}
	if amount, ok := variant["amount"].(string); ok {
		var f float64
		if _, err := fmt.Sscanf(amount, "%f", &f); err == nil {
			return f
		}
	}
	if amount, ok := variant["amount"].(float64); ok {
		return amount
	}
	return 0
}

// ---------- JSON helpers ----------

// jsonFloat extracts a float64 from the first matching key in a map.
func jsonFloat(m map[string]any, keys ...string) float64 {
	for _, k := range keys {
		v, ok := m[k]
		if !ok {
			continue
		}
		switch val := v.(type) {
		case float64:
			return val
		case string:
			var f float64
			if _, err := fmt.Sscanf(val, "%f", &f); err == nil {
				return f
			}
		case json.Number:
			if f, err := val.Float64(); err == nil {
				return f
			}
		}
	}
	return 0
}

// jsonInt extracts an int from the first matching key in a map.
func jsonInt(m map[string]any, keys ...string) int {
	for _, k := range keys {
		v, ok := m[k]
		if !ok {
			continue
		}
		switch val := v.(type) {
		case float64:
			return int(val)
		case string:
			var i int
			if _, err := fmt.Sscanf(val, "%d", &i); err == nil {
				return i
			}
		case json.Number:
			if i, err := val.Int64(); err == nil {
				return int(i)
			}
		}
	}
	return 0
}

func jsonString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		v, ok := m[k]
		if !ok {
			continue
		}
		switch val := v.(type) {
		case string:
			return val
		case json.Number:
			return val.String()
		case float64:
			return fmt.Sprintf("%.0f", val)
		}
	}
	return ""
}

func mapValue(m map[string]any, key string) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	val, ok := m[key].(map[string]any)
	if !ok {
		return map[string]any{}
	}
	return val
}

func firstNonEmptyString(vals ...string) string {
	for _, val := range vals {
		if val != "" {
			return val
		}
	}
	return ""
}

// shortFanoutErrMsg condenses an error to a single-line reason string.
func shortFanoutErrMsg(err error) string {
	if err == nil {
		return ""
	}
	s := err.Error()
	if i := strings.Index(s, "\n"); i >= 0 {
		s = s[:i]
	}
	const max = 120
	if len(s) > max {
		s = s[:max] + "..."
	}
	return s
}

// ---------- Output formatters ----------

func printFanoutTable(w io.Writer, result FanoutResult) error {
	if result.TotalResults == 0 {
		fmt.Fprintln(w, "No results found.")
		return nil
	}

	tw := newTabWriter(w)
	fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
		bold("SOURCE"), bold("TITLE"), bold("BRAND"), bold("PRICE"), bold("URL"))

	for _, p := range result.Products {
		priceStr := formatPriceRange(p.PriceMin, p.PriceMax)
		if p.OnSale {
			priceStr += " *SALE*"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			p.Source,
			truncate(p.Title, 40),
			truncate(p.Brand, 20),
			priceStr,
			truncate(p.URL, 50),
		)
	}
	if err := tw.Flush(); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "\n%d results from %d sources", result.TotalResults, len(result.SourcesQueried))
	if len(result.SourcesFailed) > 0 {
		fmt.Fprintf(os.Stderr, " (%d failed: %s)", len(result.SourcesFailed), strings.Join(result.SourcesFailed, ", "))
	}
	fmt.Fprintln(os.Stderr)
	return nil
}

func printFanoutCSV(w io.Writer, result FanoutResult) error {
	headers := []string{"source", "id", "title", "brand", "price_min", "price_max", "on_sale", "rating", "url"}
	fmt.Fprintln(w, strings.Join(headers, ","))
	for _, p := range result.Products {
		row := []string{
			csvEscape(p.Source),
			csvEscape(p.ID),
			csvEscape(p.Title),
			csvEscape(p.Brand),
			fmt.Sprintf("%.2f", p.PriceMin),
			fmt.Sprintf("%.2f", p.PriceMax),
			fmt.Sprintf("%t", p.OnSale),
			fmt.Sprintf("%.1f", p.Rating),
			csvEscape(p.URL),
		}
		fmt.Fprintln(w, strings.Join(row, ","))
	}
	return nil
}

func csvEscape(s string) string {
	if strings.ContainsAny(s, ",\"\n") {
		return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
	}
	return s
}

func formatPriceRange(min, max float64) string {
	if min == 0 && max == 0 {
		return "-"
	}
	if min == max || max == 0 {
		return fmt.Sprintf("$%.2f", min)
	}
	if min == 0 {
		return fmt.Sprintf("$%.2f", max)
	}
	return fmt.Sprintf("$%.2f-$%.2f", min, max)
}

// Ensure time import is referenced — timeout is used indirectly via flags.timeout.
var _ = time.Second
