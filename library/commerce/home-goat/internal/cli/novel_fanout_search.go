// Novel command: fan-out product search across all Tier 1 sources with
// category routing, partial-failure tolerance, and unified output.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"home-goat-pp-cli/internal/cliutil"
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
		Long: `Fan-out search across all active Tier 1 sources. Category-based routing
sends queries to the sources that carry each product type.

By default, all active sources are queried. Use --category, --room, or
--source to restrict the search scope.`,
		Example: `  home-goat-pp-cli product-search all "floating vanity"
  home-goat-pp-cli product-search all "pendant light" --room bathroom
  home-goat-pp-cli product-search all "sofa" --category furniture
  home-goat-pp-cli product-search all "faucet" --source ferguson,rejuvenation
  home-goat-pp-cli product-search all "table" --sort price-asc --max-price 500
  home-goat-pp-cli product-search all "mirror" --json --sort price-desc`,
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

	cmd.Flags().StringVar(&categoryFlag, "category", "", "Comma-separated categories: foundational, appliances, furniture, decor")
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

	// Apply price filters.
	if minPrice > 0 || maxPrice > 0 {
		filtered := make([]NormalizedProduct, 0, len(allProducts))
		for _, p := range allProducts {
			if minPrice > 0 && p.PriceMax < minPrice {
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
			return products[i].PriceMax > products[j].PriceMax
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

	// Report per-store errors on stderr (these are sub-source partial failures).
	for _, e := range errs {
		fmt.Fprintf(os.Stderr, "warn: shopify/%s: %s\n", e.Source, shortFanoutErrMsg(e.Err))
	}

	var all []NormalizedProduct
	for _, r := range results {
		all = append(all, r.Value...)
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
