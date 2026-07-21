// Copyright 2026 Matthew Vassallo and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/woot/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/woot/internal/graphqlguard"
	"github.com/spf13/cobra"
)

type wootDealsResponse struct {
	Data *wootDealsData `json:"data"`
}

type wootDealsData struct {
	SearchOffers *wootSearchOffers `json:"searchOffers"`
}

type wootSearchOffers struct {
	Offers    *[]wootGraphQLDeal `json:"Offers"`
	TotalHits *int               `json:"TotalHits"`
}

type wootGraphQLDeal struct {
	ID            string          `json:"Id"`
	IsAppFeatured bool            `json:"IsAppFeatured"`
	IsFeatured    bool            `json:"IsFeatured"`
	Title         string          `json:"Title"`
	Slug          string          `json:"Slug"`
	EndDate       string          `json:"EndDate"`
	SoldOut       bool            `json:"SoldOut"`
	Items         []wootDealItem  `json:"Items,omitempty"`
	Photos        []wootDealPhoto `json:"Photos,omitempty"`
}

type wootDeal struct {
	ID            string          `json:"id,omitempty"`
	Title         string          `json:"title"`
	Slug          string          `json:"slug"`
	EndDate       string          `json:"end_date,omitempty"`
	SoldOut       bool            `json:"sold_out"`
	IsFeatured    bool            `json:"is_featured,omitempty"`
	IsAppFeatured bool            `json:"is_app_featured,omitempty"`
	Items         []wootDealItem  `json:"items,omitempty"`
	Photos        []wootDealPhoto `json:"photos,omitempty"`
	URL           string          `json:"url"`
	Min           *float64        `json:"min_price,omitempty"`
	Max           *float64        `json:"max_price,omitempty"`
}

type wootDealItem struct {
	SalePrice *float64            `json:"SalePrice,omitempty"`
	ListPrice *float64            `json:"ListPrice,omitempty"`
	Attrs     []wootDealAttribute `json:"Attributes,omitempty"`
}

type wootDealAttribute struct {
	Key   string `json:"Key"`
	Value string `json:"Value"`
}

type wootDealPhoto struct {
	Width  int    `json:"Width,omitempty"`
	Height int    `json:"Height,omitempty"`
	URL    string `json:"Url,omitempty"`
}

type wootPriceRange struct {
	Min float64
	Max float64
}

var wootBracketPriceRangeRE = regexp.MustCompile(`\[\s*([0-9]+(?:\.[0-9]+)?)\s*,\s*([0-9]+(?:\.[0-9]+)?)\s*\]`)

func newDealsCmd(flags *rootFlags) *cobra.Command {
	var categories []string
	var priceRangeArgs []string
	var limit int
	var skip int
	var page int
	var pageSize int
	var sortMode string
	var includeFeatured bool
	var includeSoldOut bool
	var keyword string
	var fromURL string

	cmd := &cobra.Command{
		Use:   "deals [keyword]",
		Short: "List current Woot All Deals offers and optionally filter by keyword",
		Long: "List current Woot offers from the same frontend GraphQL searchOffers call used by Woot's All Deals page. " +
			"Keyword matching is applied locally to fetched titles, slugs, and item attributes.",
		Example: `  woot-pp-cli deals laptop
  woot-pp-cli deals rayon --limit 2000
  woot-pp-cli deals rayon --from-url 'https://www.woot.com/alldeals?selectedCategories=sport&selectedPriceRanges=[0,24.99]-[25,49.99]&page=13'
  woot-pp-cli deals --category sport --price-range under-25 --price-range 25-50 --page 13 rayon
  woot-pp-cli deals --json --compact software`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 && keyword == "" {
				parts := make([]string, 0, len(args))
				for _, arg := range args {
					if fromURL == "" && strings.Contains(arg, "woot.com/alldeals") {
						fromURL = arg
						continue
					}
					parts = append(parts, arg)
				}
				keyword = strings.Join(parts, " ")
			}
			if limit <= 0 {
				return usageErr(fmt.Errorf("--limit must be greater than 0"))
			}
			if skip < 0 {
				return usageErr(fmt.Errorf("--skip must be >= 0"))
			}
			if page < 0 {
				return usageErr(fmt.Errorf("--page must be >= 0"))
			}
			if pageSize <= 0 {
				return usageErr(fmt.Errorf("--page-size must be greater than 0"))
			}

			if fromURL != "" {
				parsedFilters, err := parseWootAllDealsURL(fromURL)
				if err != nil {
					return usageErr(err)
				}
				if !cmd.Flags().Changed("category") {
					categories = parsedFilters.Categories
				}
				if !cmd.Flags().Changed("price-range") {
					priceRangeArgs = parsedFilters.PriceRanges
				}
				if !cmd.Flags().Changed("page") {
					page = parsedFilters.Page
				}
			}
			if page > 0 && !cmd.Flags().Changed("page-size") {
				pageSize = 12
			}

			priceRanges, err := parseWootPriceRanges(priceRangeArgs)
			if err != nil {
				return usageErr(err)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			deals, totalHits, scanned, err := fetchWootDeals(cmd, c, wootDealsFetchOptions{
				Categories:      categories,
				PriceRanges:     priceRanges,
				SortMode:        sortMode,
				Limit:           limit,
				Skip:            skip,
				Page:            page,
				PageSize:        pageSize,
				IncludeFeatured: includeFeatured,
				IncludeSoldOut:  includeSoldOut,
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			deals, duplicateRows, missingIDRows := uniqueWootDeals(deals)
			uniqueScanned := len(deals)
			deals = filterWootDeals(deals, keyword)
			expectedScan := expectedWootDealsScan(wootDealsFetchOptions{
				Limit:    limit,
				Skip:     skip,
				Page:     page,
				PageSize: pageSize,
			}, totalHits)
			incomplete := scanned < expectedScan || duplicateRows > 0 || missingIDRows > 0

			if flags.csv || flags.plain || flags.quiet {
				return printJSONFiltered(cmd.OutOrStdout(), deals, flags)
			}
			if flags.asJSON || flags.compact || flags.selectFields != "" || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, map[string]any{
					"meta": map[string]any{
						"source":          "live",
						"keyword":         keyword,
						"categories":      categories,
						"price_ranges":    priceRanges,
						"server_limit":    limit,
						"server_skip":     skip,
						"page":            page,
						"page_size":       pageSize,
						"server_sort":     sortMode,
						"total_hits":      totalHits,
						"scanned":         scanned,
						"unique_scanned":  uniqueScanned,
						"duplicate_rows":  duplicateRows,
						"missing_id_rows": missingIDRows,
						"expected_scan":   expectedScan,
						"incomplete":      incomplete,
						"client_filtered": keyword != "",
						"from_url":        fromURL,
						"count":           len(deals),
					},
					"results": deals,
				})
			}
			if incomplete {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: Woot returned an incomplete result window: %d rows, %d after duplicate removal, %d missing IDs; expected %d rows. Results shown may be partial.\n", scanned, uniqueScanned, missingIDRows, expectedScan)
			}

			if len(deals) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No matching Woot deals found for %q. Try copying the Woot All Deals URL with --from-url, or use a larger --limit to scan more results.\n", keyword)
				return nil
			}

			rows := make([][]string, 0, len(deals))
			for _, d := range deals {
				rows = append(rows, []string{
					d.Title,
					formatWootPriceRange(d.Min, d.Max),
					formatWootEndDate(d.EndDate),
					d.URL,
				})
			}
			return flags.printTable(cmd, []string{"TITLE", "PRICE", "ENDS", "URL"}, rows)
		},
	}

	cmd.Flags().StringSliceVar(&categories, "category", nil, "Woot category slug from All Deals, e.g. sport, tech, pc; repeatable")
	cmd.Flags().StringArrayVar(&priceRangeArgs, "price-range", nil, "Woot price range: under-25, 25-50, 50-100, over-100, or [min,max]; repeatable")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum offers to scan from Woot before local keyword filtering")
	cmd.Flags().IntVar(&skip, "skip", 0, "Number of offers to skip server-side")
	cmd.Flags().IntVar(&page, "page", 0, "Fetch a single Woot All Deals page number (1-based); 0 scans from --skip")
	cmd.Flags().IntVar(&pageSize, "page-size", 100, "Offers to fetch per GraphQL request; Woot's UI uses 12 when --page is set")
	cmd.Flags().StringVar(&sortMode, "sort", "BestSelling", "Woot sort mode, e.g. BestSelling or NewestFirst")
	cmd.Flags().BoolVar(&includeFeatured, "include-featured", true, "Include featured offers")
	cmd.Flags().BoolVar(&includeSoldOut, "include-sold-out", false, "Include sold-out offers")
	cmd.Flags().StringVarP(&keyword, "query", "q", "", "Keyword to match locally against title, slug, and attributes")
	cmd.Flags().StringVar(&fromURL, "from-url", "", "Import category, price range, and page filters from a Woot /alldeals URL")

	return cmd
}

func uniqueWootDeals(deals []wootDeal) ([]wootDeal, int, int) {
	unique := make([]wootDeal, 0, len(deals))
	seen := make(map[string]struct{}, len(deals))
	duplicates := 0
	missingIDs := 0
	for _, deal := range deals {
		if deal.ID == "" {
			missingIDs++
			unique = append(unique, deal)
			continue
		}
		if _, duplicate := seen[deal.ID]; duplicate {
			duplicates++
			continue
		}
		seen[deal.ID] = struct{}{}
		unique = append(unique, deal)
	}
	return unique, duplicates, missingIDs
}

func expectedWootDealsScan(opts wootDealsFetchOptions, totalHits int) int {
	if totalHits <= 0 {
		return 0
	}
	serverSkip := opts.Skip
	window := opts.Limit
	if opts.Page > 0 {
		serverSkip += (opts.Page - 1) * opts.PageSize
		window = opts.PageSize
	}
	remaining := totalHits - serverSkip
	if remaining <= 0 {
		return 0
	}
	return minInt(window, remaining)
}

type wootDealsFetchOptions struct {
	Categories      []string
	PriceRanges     []wootPriceRange
	SortMode        string
	Limit           int
	Skip            int
	Page            int
	PageSize        int
	IncludeFeatured bool
	IncludeSoldOut  bool
}

func fetchWootDeals(cmd *cobra.Command, c *client.Client, opts wootDealsFetchOptions) ([]wootDeal, int, int, error) {
	if opts.PageSize <= 0 {
		return nil, 0, 0, fmt.Errorf("--page-size must be greater than 0")
	}
	serverSkip := opts.Skip
	maxToScan := opts.Limit
	if opts.Page > 0 {
		serverSkip += (opts.Page - 1) * opts.PageSize
		maxToScan = opts.PageSize
	}

	all := make([]wootGraphQLDeal, 0, minInt(maxToScan, opts.PageSize))
	totalHits := 0
	scanned := 0
	for requested := 0; requested < maxToScan; {
		requestLimit := minInt(opts.PageSize, maxToScan-requested)
		query, err := buildWootDealsQuery(opts.Categories, opts.PriceRanges, opts.SortMode, requestLimit, serverSkip+requested, opts.IncludeFeatured, opts.IncludeSoldOut)
		if err != nil {
			return nil, 0, 0, err
		}
		data, err := c.GetWithHeadersNoCache(cmd.Context(), "/graphql", map[string]string{"query": query}, nil)
		if err != nil {
			return nil, 0, 0, err
		}
		if isDryRunResponse(data) {
			return nil, 0, 0, nil
		}

		batch, reportedTotal, err := decodeWootDealsPage(data)
		if err != nil {
			return nil, 0, 0, err
		}
		if reportedTotal > totalHits {
			totalHits = reportedTotal
		}
		if len(batch) == 0 {
			if totalHits == 0 || opts.Page > 0 {
				break
			}
			requested += requestLimit
			if serverSkip+requested >= totalHits {
				break
			}
			continue
		}
		if totalHits > 0 && serverSkip+requested >= totalHits {
			break
		}
		all = append(all, batch...)
		scanned += len(batch)
		if opts.Page > 0 {
			break
		}
		requested += requestLimit
		if totalHits > 0 && serverSkip+requested >= totalHits {
			break
		}
	}

	return normalizeWootDeals(all), totalHits, scanned, nil
}

func decodeWootDealsPage(data json.RawMessage) ([]wootGraphQLDeal, int, error) {
	if err := graphqlguard.ValidateResponse(data); err != nil {
		return nil, 0, fmt.Errorf("Woot %w", err)
	}
	var parsed wootDealsResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, 0, fmt.Errorf("decoding Woot deals response: %w", err)
	}
	if parsed.Data == nil || parsed.Data.SearchOffers == nil {
		return nil, 0, fmt.Errorf("Woot GraphQL response is missing data.searchOffers")
	}
	if parsed.Data.SearchOffers.Offers == nil {
		return nil, 0, fmt.Errorf("Woot GraphQL response is missing data.searchOffers.Offers")
	}
	if parsed.Data.SearchOffers.TotalHits == nil {
		return nil, 0, fmt.Errorf("Woot GraphQL response is missing data.searchOffers.TotalHits")
	}
	if *parsed.Data.SearchOffers.TotalHits < 0 {
		return nil, 0, fmt.Errorf("Woot GraphQL response has negative data.searchOffers.TotalHits")
	}
	return *parsed.Data.SearchOffers.Offers, *parsed.Data.SearchOffers.TotalHits, nil
}

func buildWootDealsQuery(categories []string, priceRanges []wootPriceRange, sortMode string, limit int, skip int, includeFeatured bool, includeSoldOut bool) (string, error) {
	if sortMode == "" {
		sortMode = "BestSelling"
	}
	if !safeGraphQLEnum(sortMode) {
		return "", fmt.Errorf("--sort contains unsupported characters")
	}
	filterParts := make([]string, 0, 4)
	quotedCategories := make([]string, 0, len(categories))
	for _, category := range categories {
		category = strings.TrimSpace(category)
		if category == "" {
			continue
		}
		if !safeGraphQLString(category) {
			return "", fmt.Errorf("--category %q contains unsupported characters", category)
		}
		quotedCategories = append(quotedCategories, strconv.Quote(category))
	}
	if len(quotedCategories) > 0 {
		filterParts = append(filterParts, "Categories:["+strings.Join(quotedCategories, ",")+"]")
	}

	if len(priceRanges) > 0 {
		parts := make([]string, 0, len(priceRanges))
		for _, priceRange := range priceRanges {
			if priceRange.Min < 0 || priceRange.Max < 0 || priceRange.Min > priceRange.Max {
				return "", fmt.Errorf("invalid --price-range %.2f-%.2f", priceRange.Min, priceRange.Max)
			}
			parts = append(parts, fmt.Sprintf("{between:[%s,%s]}", formatGraphQLFloat(priceRange.Min), formatGraphQLFloat(priceRange.Max)))
		}
		filterParts = append(filterParts, "PriceFilterInputs:["+strings.Join(parts, ",")+"]")
	}

	if !includeFeatured {
		filterParts = append(filterParts, "IsFeatured:{exclude:true}")
	}
	if !includeSoldOut {
		filterParts = append(filterParts, "IsSoldOut:{exclude:true}")
	}

	return fmt.Sprintf(`{ searchOffers(Filter:{%s}, Sort:%s, Limit:%d, Skip:%d){ Offers{Id IsAppFeatured IsFeatured SoldOut Title Photos{Width Height Url} EndDate Items{ListPrice SalePrice Attributes{Key Value}} Slug} TotalHits }}`,
		strings.Join(filterParts, ","), sortMode, limit, skip), nil
}

func normalizeWootDeals(deals []wootGraphQLDeal) []wootDeal {
	out := make([]wootDeal, 0, len(deals))
	for i := range deals {
		deal := wootDeal{
			ID:            deals[i].ID,
			Title:         deals[i].Title,
			Slug:          deals[i].Slug,
			EndDate:       deals[i].EndDate,
			SoldOut:       deals[i].SoldOut,
			IsFeatured:    deals[i].IsFeatured,
			IsAppFeatured: deals[i].IsAppFeatured,
			Items:         deals[i].Items,
			Photos:        deals[i].Photos,
			URL:           "https://www.woot.com/offers/" + url.PathEscape(deals[i].Slug),
		}
		minPrice := math.Inf(1)
		maxPrice := math.Inf(-1)
		for _, item := range deals[i].Items {
			if item.SalePrice == nil {
				continue
			}
			minPrice = math.Min(minPrice, *item.SalePrice)
			maxPrice = math.Max(maxPrice, *item.SalePrice)
		}
		if !math.IsInf(minPrice, 1) {
			min := minPrice
			max := maxPrice
			deal.Min = &min
			deal.Max = &max
		}
		out = append(out, deal)
	}
	return out
}

func filterWootDeals(deals []wootDeal, keyword string) []wootDeal {
	keyword = strings.TrimSpace(strings.ToLower(keyword))
	if keyword == "" {
		return deals
	}
	filtered := make([]wootDeal, 0, len(deals))
	for _, deal := range deals {
		if strings.Contains(strings.ToLower(deal.Title), keyword) || strings.Contains(strings.ToLower(deal.Slug), keyword) {
			filtered = append(filtered, deal)
			continue
		}
		if wootDealItemsContain(deal.Items, keyword) {
			filtered = append(filtered, deal)
		}
	}
	return filtered
}

func wootDealItemsContain(items []wootDealItem, keyword string) bool {
	for _, item := range items {
		for _, attr := range item.Attrs {
			if strings.Contains(strings.ToLower(attr.Key), keyword) || strings.Contains(strings.ToLower(attr.Value), keyword) {
				return true
			}
		}
	}
	return false
}

func formatWootPriceRange(min *float64, max *float64) string {
	if min == nil || max == nil {
		return ""
	}
	if *min == *max {
		return fmt.Sprintf("$%.2f", *min)
	}
	return fmt.Sprintf("$%.2f-$%.2f", *min, *max)
}

func formatWootEndDate(value string) string {
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return value
	}
	return t.Format("2006-01-02 15:04 MST")
}

type wootAllDealsURLFilters struct {
	Categories  []string
	PriceRanges []string
	Page        int
}

func parseWootAllDealsURL(raw string) (wootAllDealsURLFilters, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return wootAllDealsURLFilters{}, fmt.Errorf("parse --from-url: %w", err)
	}
	host := strings.ToLower(parsed.Hostname())
	if (host != "woot.com" && !strings.HasSuffix(host, ".woot.com")) || !strings.Contains(strings.ToLower(parsed.Path), "/alldeals") {
		return wootAllDealsURLFilters{}, fmt.Errorf("--from-url must be a Woot /alldeals URL")
	}
	query := parsed.Query()
	filters := wootAllDealsURLFilters{}
	if selectedCategories := strings.TrimSpace(query.Get("selectedCategories")); selectedCategories != "" {
		for _, category := range strings.Split(selectedCategories, ",") {
			category = strings.TrimSpace(category)
			if category != "" {
				filters.Categories = append(filters.Categories, category)
			}
		}
	}
	if selectedPriceRanges := strings.TrimSpace(query.Get("selectedPriceRanges")); selectedPriceRanges != "" {
		filters.PriceRanges = []string{selectedPriceRanges}
	}
	if pageValue := strings.TrimSpace(query.Get("page")); pageValue != "" {
		page, err := strconv.Atoi(pageValue)
		if err != nil || page < 1 {
			return wootAllDealsURLFilters{}, fmt.Errorf("--from-url has invalid page %q", pageValue)
		}
		filters.Page = page
	}
	return filters, nil
}

func parseWootPriceRanges(values []string) ([]wootPriceRange, error) {
	var ranges []wootPriceRange
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || strings.EqualFold(value, "all") || strings.EqualFold(value, "all-prices") {
			continue
		}
		bracketMatches := wootBracketPriceRangeRE.FindAllStringSubmatch(value, -1)
		if len(bracketMatches) > 0 {
			for _, match := range bracketMatches {
				priceRange, err := parseWootPriceRangePair(match[1], match[2])
				if err != nil {
					return nil, err
				}
				ranges = append(ranges, priceRange)
			}
			continue
		}

		normalized := strings.ToLower(strings.TrimSpace(value))
		normalized = strings.ReplaceAll(normalized, "$", "")
		normalized = strings.ReplaceAll(normalized, " ", "")
		switch normalized {
		case "under25", "under-25", "0-24.99", "0:24.99":
			ranges = append(ranges, wootPriceRange{Min: 0, Max: 24.99})
			continue
		case "25-50", "25:49.99", "25-49.99":
			ranges = append(ranges, wootPriceRange{Min: 25, Max: 49.99})
			continue
		case "50-100", "50:100", "50-99.99":
			ranges = append(ranges, wootPriceRange{Min: 50, Max: 99.99})
			continue
		case "over100", "over-100", "100+":
			ranges = append(ranges, wootPriceRange{Min: 100, Max: 999999})
			continue
		}

		separator := ":"
		if strings.Contains(normalized, "-") {
			separator = "-"
		} else if strings.Contains(normalized, ",") {
			separator = ","
		}
		parts := strings.Split(normalized, separator)
		if len(parts) != 2 {
			return nil, fmt.Errorf("--price-range %q must be under-25, 25-50, 50-100, over-100, [min,max], or min:max", value)
		}
		priceRange, err := parseWootPriceRangePair(parts[0], parts[1])
		if err != nil {
			return nil, fmt.Errorf("--price-range %q: %w", value, err)
		}
		ranges = append(ranges, priceRange)
	}
	return ranges, nil
}

func parseWootPriceRangePair(minValue string, maxValue string) (wootPriceRange, error) {
	minPrice, err := strconv.ParseFloat(strings.TrimSpace(minValue), 64)
	if err != nil {
		return wootPriceRange{}, fmt.Errorf("invalid minimum price %q", minValue)
	}
	maxPrice, err := strconv.ParseFloat(strings.TrimSpace(maxValue), 64)
	if err != nil {
		return wootPriceRange{}, fmt.Errorf("invalid maximum price %q", maxValue)
	}
	if minPrice < 0 || maxPrice < 0 || minPrice > maxPrice {
		return wootPriceRange{}, fmt.Errorf("invalid range %.2f-%.2f", minPrice, maxPrice)
	}
	return wootPriceRange{Min: minPrice, Max: maxPrice}, nil
}

func formatGraphQLFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func safeGraphQLString(value string) bool {
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			continue
		}
		return false
	}
	return true
}

func safeGraphQLEnum(value string) bool {
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' {
			continue
		}
		return false
	}
	return true
}
