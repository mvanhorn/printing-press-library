package cli

import (
	"encoding/json"
	"fmt"
	"html"
	"math"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/continente/internal/domain"
	"github.com/mvanhorn/printing-press-library/library/commerce/continente/internal/normalize"
	"github.com/spf13/cobra"
)

const continenteBaseURL = "https://www.continente.pt"

var (
	productURLRe               = regexp.MustCompile(`href="(/produto/[^"?]+\.html)(?:\?[^"]*)?"`)
	imageURLRe                 = regexp.MustCompile(`src="([^"]+)"`)
	suggestionItemRe           = regexp.MustCompile(`(?s)<div class="suggestion-product-item"[^>]*data-pid="([^"]+)"[^>]*data-product-tile-impression='([^']+)'[^>]*>`)
	searchTileRe               = regexp.MustCompile(`(?s)<div class="product-tile[^"]*"[^>]*data-product-tile-impression='([^']+)'[^>]*>`)
	pageDataLayerRe            = regexp.MustCompile(`data-page-data-layer="([^"]+)"`)
	searchResultsCountRe       = regexp.MustCompile(`data-gtm-results="(\d+)"`)
	refinementBlockRe          = regexp.MustCompile(`(?s)<div class="collapsible-lg[^"]* refinement [^"]*" data-refinement-type="([^"]+)" >`)
	refinementTitleRe          = regexp.MustCompile(`(?s)<span class="refinement-title-text">\s*(.*?)\s*</span>`)
	categoryButtonRe           = regexp.MustCompile(`(?s)<button class="category-refinement-btn[^"]*" data-cgid="([^"]+)" data-href="([^"]+)">.*?<span title="([^"]+)"[^>]*>.*?<span class="hit-count">\((\d+)\)</span>`)
	sortOptionsRe              = regexp.MustCompile(`data-sort-options="([^"]+)"`)
	tagRe                      = regexp.MustCompile(`<[^>]+>`)
	dataHrefAttrRe             = regexp.MustCompile(`data-href="([^"]+)"`)
	labelBlockRe               = regexp.MustCompile(`(?s)<label[^>]*>(.*?)</label>`)
	hitCountRe                 = regexp.MustCompile(`<span class="hit-count">\((\d+)\)</span>`)
	productLDJSONRe            = regexp.MustCompile(`(?s)<script type="application/ld\+json">\s*(\{.*?\})\s*</script>`)
	productDetailImpressionRe  = regexp.MustCompile(`data-product-detail-impression="([^"]+)"`)
	productNutritionEndpointRe = regexp.MustCompile(`data-url="([^"]*Product-ProductNutritionalInfoTab[^"]*)"`)
	pvprPriceRe                = regexp.MustCompile(`(?s)<span class="pvpr-info">PVPR</span>\s*([0-9]+(?:[.,][0-9]+)?)&euro;`)
	unitPriceRe                = regexp.MustCompile(`(?s)<div class="pwc-tile--price-secondary">\s*([0-9]+(?:[.,][0-9]+)?)&euro;/([^<\s]+)`)
	packLabelRe                = regexp.MustCompile(`(?s)<span class="ct-(?:pdp|pwc)-[^"]*unit[^"]*">\s*(.*?)\s*</span>`)
	promoLabelRe               = regexp.MustCompile(`(?s)<div class="ct-product-tile-badge-label[^"]*">\s*<span[^>]*>\s*(.*?)\s*</span>\s*</div>`)
	promoValueRe               = regexp.MustCompile(`(?s)<div class="ct-quantifier-container[^"]*">.*?<span class="ct-product-tile-badge-value--pvpr[^"]*">\s*(.*?)\s*</span>\s*<span class="ct-product-tile-badge-value--pvpr-quantifier[^"]*">\s*(.*?)\s*</span>`)
	badgeImageRe               = regexp.MustCompile(`(?s)<img[^>]*(?:data-src|src)="[^"]*/images/badges/[^"]+"[^>]*>`)
	titleAttrRe                = regexp.MustCompile(`title="([^"]+)"`)
)

type storefrontItem struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Brand          string   `json:"brand,omitempty"`
	Category       string   `json:"category,omitempty"`
	Categories     []string `json:"categories,omitempty"`
	Price          float64  `json:"price,omitempty"`
	OriginalPrice  float64  `json:"original_price,omitempty"`
	DiscountAmount float64  `json:"discount_amount,omitempty"`
	SavingsPercent float64  `json:"savings_percent,omitempty"`
	UnitPrice      float64  `json:"unit_price,omitempty"`
	UnitLabel      string   `json:"unit_label,omitempty"`
	PackLabel      string   `json:"pack_label,omitempty"`
	HasPromotion   bool     `json:"has_promotion,omitempty"`
	HasDiscount    bool     `json:"has_discount,omitempty"`
	PromotionText  []string `json:"promotion_text,omitempty"`
	URL            string   `json:"url,omitempty"`
	Image          string   `json:"image,omitempty"`
}

type suggestionsResponse struct {
	Query string           `json:"query"`
	Count int              `json:"count"`
	Items []storefrontItem `json:"items"`
}

type searchResponse struct {
	Query         string             `json:"query,omitempty"`
	CategoryID    string             `json:"category_id,omitempty"`
	Start         int                `json:"start"`
	PageSize      int                `json:"page_size,omitempty"`
	NextStart     *int               `json:"next_start,omitempty"`
	SortRule      string             `json:"sort_rule,omitempty"`
	ActiveFilters map[string]string  `json:"active_filters,omitempty"`
	TotalCount    int                `json:"total_count,omitempty"`
	SortOptions   []searchSortOption `json:"sort_options,omitempty"`
	Refinements   []searchRefinement `json:"refinements,omitempty"`
	Count         int                `json:"count"`
	Items         []storefrontItem   `json:"items"`
}

type searchSortOption struct {
	ID          string `json:"id,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
	URL         string `json:"url,omitempty"`
}

type searchRefinement struct {
	Key     string                   `json:"key"`
	Label   string                   `json:"label,omitempty"`
	Options []searchRefinementOption `json:"options,omitempty"`
}

type searchRefinementOption struct {
	Label      string            `json:"label"`
	Count      int               `json:"count,omitempty"`
	URL        string            `json:"url,omitempty"`
	CategoryID string            `json:"category_id,omitempty"`
	Params     map[string]string `json:"params,omitempty"`
}

type productResponse struct {
	ID                 string            `json:"id"`
	Name               string            `json:"name"`
	Brand              string            `json:"brand,omitempty"`
	SKU                string            `json:"sku,omitempty"`
	MPN                string            `json:"mpn,omitempty"`
	Price              float64           `json:"price,omitempty"`
	OriginalPrice      float64           `json:"original_price,omitempty"`
	DiscountAmount     float64           `json:"discount_amount,omitempty"`
	SavingsPercent     float64           `json:"savings_percent,omitempty"`
	UnitPrice          float64           `json:"unit_price,omitempty"`
	UnitLabel          string            `json:"unit_label,omitempty"`
	PackLabel          string            `json:"pack_label,omitempty"`
	HasPromotion       bool              `json:"has_promotion,omitempty"`
	HasDiscount        bool              `json:"has_discount,omitempty"`
	PromotionText      []string          `json:"promotion_text,omitempty"`
	Currency           string            `json:"currency,omitempty"`
	Availability       string            `json:"availability,omitempty"`
	RatingValue        float64           `json:"rating_value,omitempty"`
	RatingCount        int               `json:"rating_count,omitempty"`
	Category           string            `json:"category,omitempty"`
	Categories         []string          `json:"categories,omitempty"`
	URL                string            `json:"url,omitempty"`
	Image              string            `json:"image,omitempty"`
	NutritionalInfoURL string            `json:"nutritional_info_url,omitempty"`
	NutritionStatus    string            `json:"nutrition_status,omitempty"`
	Nutrition          *nutritionProfile `json:"nutrition,omitempty"`
}

type tileImpression struct {
	Name     string  `json:"name"`
	ID       string  `json:"id"`
	Price    float64 `json:"price"`
	Brand    string  `json:"brand"`
	Category string  `json:"category"`
}

type searchPageData struct {
	PageData struct {
		BaseURL string `json:"base_url"`
	} `json:"page_data"`
}

type productLDJSON struct {
	Name  string   `json:"name"`
	MPN   string   `json:"mpn"`
	SKU   string   `json:"sku"`
	Image []string `json:"image"`
	Brand struct {
		Name string `json:"name"`
	} `json:"brand"`
	Offers struct {
		PriceCurrency string `json:"priceCurrency"`
		Price         string `json:"price"`
		Availability  string `json:"availability"`
	} `json:"offers"`
	AggregateRating struct {
		RatingCount int     `json:"ratingCount"`
		RatingValue float64 `json:"ratingValue"`
	} `json:"aggregateRating"`
}

type productDetailImpression struct {
	Currency string  `json:"currency"`
	Value    float64 `json:"value"`
	Items    []struct {
		ItemID           string  `json:"item_id"`
		ItemBrand        string  `json:"item_brand"`
		ItemCategory     string  `json:"item_category"`
		ItemCategory2    string  `json:"item_category2"`
		ItemCategory3    string  `json:"item_category3"`
		Price            float64 `json:"price"`
		Discount         float64 `json:"discount"`
		PreDiscountPrice float64 `json:"pre_discount_price"`
	} `json:"items"`
}

type extractionError struct {
	Operation string
	Reason    string
	Err       error
}

func (e *extractionError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err == nil {
		return fmt.Sprintf("%s extraction failed: %s", e.Operation, e.Reason)
	}
	return fmt.Sprintf("%s extraction failed: %s: %v", e.Operation, e.Reason, e.Err)
}

func (e *extractionError) Unwrap() error { return e.Err }

func parseSuggestionsHTML(query string, body []byte) (suggestionsResponse, error) {
	matches := suggestionItemRe.FindAllSubmatchIndex(body, -1)
	if len(matches) == 0 {
		return suggestionsResponse{}, &extractionError{Operation: "suggestions", Reason: "no suggestion items found"}
	}
	items := make([]storefrontItem, 0, len(matches))
	var decodeErr error
	for i, match := range matches {
		blockStart := match[0]
		blockEnd := len(body)
		if i+1 < len(matches) {
			blockEnd = matches[i+1][0]
		}
		block := string(body[blockStart:blockEnd])
		imp, err := decodeTileImpression(string(body[match[4]:match[5]]))
		if err != nil {
			decodeErr = err
			continue
		}
		item, err := storefrontItemFromImpression(imp)
		if err != nil {
			decodeErr = err
			continue
		}
		if item.ID == "" {
			item.ID = string(body[match[2]:match[3]])
		}
		item.URL = firstProductURL(block)
		item.Image = firstImageURL(block)
		items = append(items, item)
	}
	if len(items) == 0 {
		return suggestionsResponse{}, &extractionError{Operation: "suggestions", Reason: "malformed suggestion tile impression", Err: decodeErr}
	}
	return suggestionsResponse{Query: query, Count: len(items), Items: items}, nil
}

func parseSearchHTML(query string, start int, body []byte) (searchResponse, error) {
	matches := searchTileRe.FindAllSubmatchIndex(body, -1)
	if len(matches) == 0 {
		return searchResponse{}, &extractionError{Operation: "search", Reason: "no product tiles found"}
	}
	items := make([]storefrontItem, 0, len(matches))
	var decodeErr error
	for i, match := range matches {
		blockStart := match[0]
		blockEnd := len(body)
		if i+1 < len(matches) {
			blockEnd = matches[i+1][0]
		}
		block := string(body[blockStart:blockEnd])
		imp, err := decodeTileImpression(string(body[match[2]:match[3]]))
		if err != nil {
			decodeErr = err
			continue
		}
		item, err := storefrontItemFromImpression(imp)
		if err != nil {
			decodeErr = err
			continue
		}
		item.URL = firstProductURL(block)
		item.Image = firstImageURL(block)
		enrichStorefrontItemPricing(&item, block)
		items = append(items, item)
	}
	if len(items) == 0 {
		return searchResponse{}, &extractionError{Operation: "search", Reason: "malformed product tile impression", Err: decodeErr}
	}

	resp := searchResponse{
		Query: query,
		Start: start,
		Count: len(items),
		Items: items,
	}
	applyBrowseMetadata(&resp, body)
	if pageMatch := pageDataLayerRe.FindSubmatch(body); len(pageMatch) == 2 {
		var page searchPageData
		if json.Unmarshal([]byte(html.UnescapeString(string(pageMatch[1]))), &page) == nil {
			if page.PageData.BaseURL != "" {
				if parsed, err := url.Parse(page.PageData.BaseURL); err == nil {
					resp.CategoryID = parsed.Query().Get("cgid")
					if query == "" {
						resp.Query = parsed.Query().Get("q")
					}
				}
			}
		}
	}
	return resp, nil
}

func parseProductHTML(slugAndPID string, body []byte) (productResponse, error) {
	var record normalize.StorefrontProductRecord
	if match := productLDJSONRe.FindSubmatch(body); len(match) == 2 {
		var ld productLDJSON
		if err := json.Unmarshal(match[1], &ld); err == nil {
			record.ID = firstNonEmpty(ld.SKU, ld.MPN)
			record.Name = ld.Name
			record.Brand = ld.Brand.Name
			record.SKU = ld.SKU
			record.MPN = ld.MPN
			record.Currency = ld.Offers.PriceCurrency
			record.Availability = simplifyAvailability(ld.Offers.Availability)
			record.RatingCount = ld.AggregateRating.RatingCount
			record.RatingValue = ld.AggregateRating.RatingValue
			if len(ld.Image) > 0 {
				record.Image = ld.Image[0]
			}
			if ld.Offers.Price != "" {
				if v, err := strconv.ParseFloat(ld.Offers.Price, 64); err == nil {
					record.DisplayPrice = v
				}
			}
		}
	}
	if match := productDetailImpressionRe.FindSubmatch(body); len(match) == 2 {
		var detail productDetailImpression
		if json.Unmarshal([]byte(html.UnescapeString(string(match[1]))), &detail) == nil {
			if record.Currency == "" {
				record.Currency = detail.Currency
			}
			if record.DisplayPrice == 0 {
				record.DisplayPrice = detail.Value
			}
			if len(detail.Items) > 0 {
				item := detail.Items[0]
				record.ID = firstNonEmpty(record.ID, item.ItemID)
				record.Brand = firstNonEmpty(record.Brand, item.ItemBrand)
				record.Category = joinCategories(item.ItemCategory, item.ItemCategory2, item.ItemCategory3)
				record.Categories = splitCategories(record.Category)
				if record.DisplayPrice == 0 {
					record.DisplayPrice = item.Price
				}
				if record.DiscountAmount == 0 {
					record.DiscountAmount = item.Discount
				}
				if record.OriginalPrice == 0 {
					record.OriginalPrice = item.PreDiscountPrice
				}
			}
		}
	}
	if match := productNutritionEndpointRe.FindSubmatch(body); len(match) == 2 {
		record.NutritionalInfoURL = absoluteURL(html.UnescapeString(string(match[1])))
	}
	enrichStorefrontRecordPricing(&record, string(body))
	record.URL = absoluteURL("/produto/" + slugAndPID + ".html")
	product, err := normalize.ProductFromStorefront(record)
	if err != nil {
		return productResponse{}, &extractionError{Operation: "product", Reason: "missing structured product data", Err: err}
	}
	return productResponseFromDomain(product), nil
}

func storefrontItemFromImpression(imp tileImpression) (storefrontItem, error) {
	product, err := normalize.ProductFromStorefront(normalize.StorefrontProductRecord{
		ID:           imp.ID,
		Name:         imp.Name,
		Brand:        imp.Brand,
		Category:     imp.Category,
		Categories:   splitCategories(imp.Category),
		DisplayPrice: imp.Price,
	})
	if err != nil {
		return storefrontItem{}, err
	}
	return storefrontItemFromDomain(product), nil
}

func decodeTileImpression(raw string) (tileImpression, error) {
	var imp tileImpression
	err := json.Unmarshal([]byte(html.UnescapeString(raw)), &imp)
	return imp, err
}

func firstProductURL(block string) string {
	match := productURLRe.FindStringSubmatch(block)
	if len(match) != 2 {
		return ""
	}
	return absoluteURL(match[1])
}

func firstImageURL(block string) string {
	matches := imageURLRe.FindAllStringSubmatch(block, -1)
	fallback := ""
	for _, match := range matches {
		if len(match) != 2 {
			continue
		}
		src := html.UnescapeString(match[1])
		if strings.Contains(src, "noimagelarge_product.png") {
			continue
		}
		if strings.HasPrefix(src, "data:") {
			continue
		}
		if strings.Contains(src, "/images/badges/") {
			continue
		}
		if strings.Contains(src, "/Sites-col-master-catalog/") || strings.Contains(src, "/images/col/") {
			return absoluteURL(src)
		}
		if fallback == "" {
			fallback = absoluteURL(src)
		}
	}
	return fallback
}

func absoluteURL(raw string) string {
	raw = html.UnescapeString(strings.TrimSpace(raw))
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		return raw
	}
	if strings.HasPrefix(raw, "/") {
		return continenteBaseURL + raw
	}
	return continenteBaseURL + "/" + raw
}

func splitCategories(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, "/")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}

func joinCategories(parts ...string) string {
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		filtered = append(filtered, part)
	}
	return strings.Join(filtered, "/")
}

func simplifyAvailability(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if idx := strings.LastIndex(raw, "/"); idx >= 0 && idx+1 < len(raw) {
		return raw[idx+1:]
	}
	return raw
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func applyBrowseMetadata(resp *searchResponse, body []byte) {
	if resp == nil || len(body) == 0 {
		return
	}
	if match := searchResultsCountRe.FindSubmatch(body); len(match) == 2 {
		if total, err := strconv.Atoi(string(match[1])); err == nil {
			resp.TotalCount = total
		}
	}
	resp.SortOptions = parseSortOptions(body)
	resp.Refinements = parseRefinements(body)
}

func parseSortOptions(body []byte) []searchSortOption {
	match := sortOptionsRe.FindSubmatch(body)
	if len(match) != 2 {
		return nil
	}
	var envelope struct {
		Options []struct {
			DisplayName string `json:"displayName"`
			ID          string `json:"id"`
			URL         string `json:"url"`
		} `json:"options"`
	}
	raw := html.UnescapeString(string(match[1]))
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		return nil
	}
	out := make([]searchSortOption, 0, len(envelope.Options))
	for _, option := range envelope.Options {
		out = append(out, searchSortOption{
			ID:          strings.TrimSpace(option.ID),
			DisplayName: strings.TrimSpace(option.DisplayName),
			URL:         strings.TrimSpace(option.URL),
		})
	}
	return out
}

func parseRefinements(body []byte) []searchRefinement {
	titleByKey := collectRefinementTitles(body)
	grouped := map[string][]searchRefinementOption{}
	if categoryOptions := parseCategoryRefinementOptions(string(body)); len(categoryOptions) > 0 {
		grouped["category"] = categoryOptions
	}
	for key, options := range parseGenericRefinementOptions(string(body)) {
		grouped[key] = options
	}
	out := make([]searchRefinement, 0, len(grouped))
	for key, options := range grouped {
		label := titleByKey[key]
		if label == "" {
			label = humanizeRefinementKey(key)
		}
		out = append(out, searchRefinement{Key: key, Label: label, Options: options})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Label < out[j].Label })
	return out
}

func collectRefinementTitles(body []byte) map[string]string {
	matches := refinementBlockRe.FindAllSubmatchIndex(body, -1)
	if len(matches) == 0 {
		return nil
	}
	out := map[string]string{}
	for i, match := range matches {
		blockStart := match[0]
		blockEnd := len(body)
		if i+1 < len(matches) {
			blockEnd = matches[i+1][0]
		}
		block := string(body[blockStart:blockEnd])
		key := html.UnescapeString(string(body[match[2]:match[3]]))
		if titleMatch := refinementTitleRe.FindStringSubmatch(block); len(titleMatch) == 2 {
			out[key] = normalizeLabel(titleMatch[1])
		}
	}
	return out
}

func parseGenericRefinementOptions(body string) map[string][]searchRefinementOption {
	grouped := map[string][]searchRefinementOption{}
	seen := map[string]map[string]bool{}
	segments := strings.Split(body, "<button")
	if len(segments) < 2 {
		return nil
	}
	for _, segment := range segments[1:] {
		end := strings.Index(segment, "</button>")
		if end < 0 {
			continue
		}
		block := "<button" + segment[:end+len("</button>")]
		openEnd := strings.Index(block, ">")
		if openEnd < 0 {
			continue
		}
		openTag := block[:openEnd]
		if !strings.Contains(openTag, "refinement-btn") {
			continue
		}
		hrefMatch := dataHrefAttrRe.FindStringSubmatch(openTag)
		if len(hrefMatch) != 2 {
			continue
		}
		params := extractSearchParamsFromHref(hrefMatch[1])
		key := refinementKeyFromParams(params)
		if key == "" {
			continue
		}
		labelMatch := labelBlockRe.FindStringSubmatch(block)
		if len(labelMatch) != 2 {
			continue
		}
		label := normalizeLabel(hitCountRe.ReplaceAllString(labelMatch[1], ""))
		if label == "" {
			continue
		}
		if seen[key] == nil {
			seen[key] = map[string]bool{}
		}
		if seen[key][label] {
			continue
		}
		seen[key][label] = true
		option := searchRefinementOption{
			Label:  label,
			URL:    absoluteURL(html.UnescapeString(hrefMatch[1])),
			Params: params,
		}
		if countMatch := hitCountRe.FindStringSubmatch(block); len(countMatch) == 2 {
			if count, err := strconv.Atoi(countMatch[1]); err == nil {
				option.Count = count
			}
		}
		grouped[key] = append(grouped[key], option)
	}
	return grouped
}

func parseCategoryRefinementOptions(block string) []searchRefinementOption {
	matches := categoryButtonRe.FindAllStringSubmatch(block, -1)
	if len(matches) == 0 {
		return nil
	}
	options := make([]searchRefinementOption, 0, len(matches))
	seen := map[string]bool{}
	for _, match := range matches {
		if len(match) != 5 {
			continue
		}
		label := normalizeLabel(match[3])
		if label == "" || seen[match[1]] {
			continue
		}
		seen[match[1]] = true
		option := searchRefinementOption{
			Label:      label,
			CategoryID: html.UnescapeString(match[1]),
			URL:        absoluteURL(html.UnescapeString(match[2])),
			Params:     extractSearchParamsFromHref(match[2]),
		}
		if count, err := strconv.Atoi(match[4]); err == nil {
			option.Count = count
		}
		options = append(options, option)
	}
	return options
}

func extractSearchParamsFromHref(rawHref string) map[string]string {
	rawHref = html.UnescapeString(strings.TrimSpace(rawHref))
	if rawHref == "" {
		return nil
	}
	parsed, err := url.Parse(rawHref)
	if err != nil {
		return nil
	}
	values := parsed.Query()
	if len(values) == 0 {
		return nil
	}
	params := make(map[string]string, len(values))
	for key, vals := range values {
		if len(vals) == 0 {
			continue
		}
		params[key] = vals[0]
	}
	return params
}

func normalizeLabel(raw string) string {
	raw = html.UnescapeString(raw)
	raw = tagRe.ReplaceAllString(raw, " ")
	raw = strings.TrimSpace(strings.Join(strings.Fields(raw), " "))
	return raw
}

func refinementKeyFromParams(params map[string]string) string {
	if len(params) == 0 {
		return ""
	}
	if key := strings.TrimSpace(params["prefn1"]); key != "" {
		return key
	}
	if params["pmax"] != "" || params["pmin"] != "" {
		return "price"
	}
	return ""
}

func humanizeRefinementKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	if key == "price" {
		return "Preço"
	}
	key = strings.ReplaceAll(key, ".", " ")
	key = strings.ReplaceAll(key, "_", " ")
	parts := strings.Fields(key)
	for i, part := range parts {
		if part == strings.ToUpper(part) {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, " ")
}

func enrichStorefrontItemPricing(item *storefrontItem, block string) {
	if item == nil {
		return
	}
	record := normalize.StorefrontProductRecord{
		ID:           item.ID,
		Name:         item.Name,
		Brand:        item.Brand,
		Category:     item.Category,
		Categories:   item.Categories,
		URL:          item.URL,
		Image:        item.Image,
		DisplayPrice: item.Price,
	}
	enrichStorefrontRecordPricing(&record, block)
	normalized, err := normalize.ProductFromStorefront(record)
	if err != nil {
		return
	}
	*item = storefrontItemFromDomain(normalized)
}

func enrichStorefrontRecordPricing(record *normalize.StorefrontProductRecord, block string) {
	if record == nil || block == "" {
		return
	}
	if record.OriginalPrice == 0 {
		if price, ok := extractEuroAmount(pvprPriceRe, block); ok {
			record.OriginalPrice = price
		}
	}
	if record.UnitPrice == 0 {
		if match := unitPriceRe.FindStringSubmatch(block); len(match) == 3 {
			if amount, err := parseEuroNumber(match[1]); err == nil {
				record.UnitPrice = amount
				record.UnitLabel = normalizeLabel(match[2])
			}
		}
	}
	if record.PackLabel == "" {
		if match := packLabelRe.FindStringSubmatch(block); len(match) == 2 {
			record.PackLabel = normalizeLabel(match[1])
		}
	}
	if record.DiscountAmount == 0 && record.OriginalPrice != 0 && record.DisplayPrice != 0 && record.OriginalPrice > record.DisplayPrice {
		record.DiscountAmount = roundMoney(record.OriginalPrice - record.DisplayPrice)
	}
	record.PromotionText = uniqueStrings(append(record.PromotionText, extractPromotionText(block)...))
}

func extractPromotionText(block string) []string {
	var out []string
	prefix := ""
	if match := promoLabelRe.FindStringSubmatch(block); len(match) == 2 {
		prefix = normalizeLabel(match[1])
	}
	if match := promoValueRe.FindStringSubmatch(block); len(match) == 3 {
		value := normalizeLabel(match[1])
		quantifier := normalizeLabel(match[2])
		label := strings.TrimSpace(strings.Join([]string{prefix, value, quantifier}, " "))
		if label != "" {
			out = append(out, label)
		}
	}
	for _, badge := range badgeImageRe.FindAllString(block, -1) {
		match := titleAttrRe.FindStringSubmatch(badge)
		if len(match) != 2 {
			continue
		}
		label := normalizeLabel(match[1])
		if !isCommercialPromotionLabel(label) {
			continue
		}
		out = append(out, label)
	}
	return out
}

func isCommercialPromotionLabel(label string) bool {
	label = normalizeLabel(label)
	if label == "" || label == "%" || label == "Crachá do produto" {
		return false
	}
	if strings.HasPrefix(label, "PVP Recomendado:") {
		return false
	}
	switch label {
	case "Produzido em Portugal":
		return false
	}
	return true
}

func extractEuroAmount(re *regexp.Regexp, block string) (float64, bool) {
	match := re.FindStringSubmatch(block)
	if len(match) != 2 {
		return 0, false
	}
	value, err := parseEuroNumber(match[1])
	if err != nil {
		return 0, false
	}
	return value, true
}

func parseEuroNumber(raw string) (float64, error) {
	raw = normalizeLabel(raw)
	raw = strings.ReplaceAll(raw, ".", "")
	raw = strings.ReplaceAll(raw, ",", ".")
	return strconv.ParseFloat(raw, 64)
}

func roundMoney(v float64) float64 {
	return math.Round(v*100) / 100
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func storefrontItemFromDomain(product domain.Product) storefrontItem {
	return storefrontItem{
		ID:         product.ID,
		Name:       product.Name,
		Brand:      product.Brand,
		Category:   product.Category,
		Categories: product.Categories,
		Price:      product.LegacyPrice(),
		OriginalPrice: func() float64 {
			if product.Price.OriginalAmount != nil {
				return *product.Price.OriginalAmount
			}
			return 0
		}(),
		DiscountAmount: func() float64 {
			if product.Price.DiscountAmount != nil {
				return *product.Price.DiscountAmount
			}
			return 0
		}(),
		SavingsPercent: func() float64 {
			if product.Price.SavingsPercent != nil {
				return *product.Price.SavingsPercent
			}
			return 0
		}(),
		UnitPrice: func() float64 {
			if product.Price.UnitAmount != nil {
				return *product.Price.UnitAmount
			}
			return 0
		}(),
		UnitLabel:     product.Price.UnitLabel,
		PackLabel:     product.Price.PackLabel,
		HasPromotion:  product.Price.HasPromotion,
		HasDiscount:   product.Price.HasDiscount,
		PromotionText: product.Price.PromotionText,
		URL:           product.URL,
		Image:         product.Image,
	}
}

func productResponseFromDomain(product domain.Product) productResponse {
	return productResponse{
		ID:    product.ID,
		Name:  product.Name,
		Brand: product.Brand,
		SKU:   product.SKU,
		MPN:   product.MPN,
		Price: product.LegacyPrice(),
		OriginalPrice: func() float64 {
			if product.Price.OriginalAmount != nil {
				return *product.Price.OriginalAmount
			}
			return 0
		}(),
		DiscountAmount: func() float64 {
			if product.Price.DiscountAmount != nil {
				return *product.Price.DiscountAmount
			}
			return 0
		}(),
		SavingsPercent: func() float64 {
			if product.Price.SavingsPercent != nil {
				return *product.Price.SavingsPercent
			}
			return 0
		}(),
		UnitPrice: func() float64 {
			if product.Price.UnitAmount != nil {
				return *product.Price.UnitAmount
			}
			return 0
		}(),
		UnitLabel:          product.Price.UnitLabel,
		PackLabel:          product.Price.PackLabel,
		HasPromotion:       product.Price.HasPromotion,
		HasDiscount:        product.Price.HasDiscount,
		PromotionText:      product.Price.PromotionText,
		Currency:           product.Price.Currency,
		Availability:       product.Availability,
		RatingValue:        product.RatingValue,
		RatingCount:        product.RatingCount,
		Category:           product.Category,
		Categories:         product.Categories,
		URL:                product.URL,
		Image:              product.Image,
		NutritionalInfoURL: product.NutritionalInfoURL,
		NutritionStatus:    product.NutritionStatus,
	}
}

func storefrontItemHumanRow(item storefrontItem) map[string]any {
	row := map[string]any{
		"id":       item.ID,
		"name":     item.Name,
		"brand":    item.Brand,
		"price":    item.Price,
		"category": item.Category,
		"url":      item.URL,
	}
	if item.OriginalPrice != 0 {
		row["original_price"] = item.OriginalPrice
	}
	if item.DiscountAmount != 0 {
		row["discount_amount"] = item.DiscountAmount
	}
	if item.SavingsPercent != 0 {
		row["savings_percent"] = item.SavingsPercent
	}
	if item.UnitPrice != 0 {
		row["unit_price"] = item.UnitPrice
	}
	if item.UnitLabel != "" {
		row["unit_label"] = item.UnitLabel
	}
	if item.PackLabel != "" {
		row["pack_label"] = item.PackLabel
	}
	if len(item.PromotionText) > 0 {
		row["promotion_text"] = strings.Join(item.PromotionText, " | ")
	}
	if item.HasPromotion {
		row["has_promotion"] = true
	}
	if item.HasDiscount {
		row["has_discount"] = true
	}
	return row
}

func productResponseHumanRow(product productResponse) map[string]any {
	row := map[string]any{
		"id":           product.ID,
		"name":         product.Name,
		"brand":        product.Brand,
		"price":        product.Price,
		"currency":     product.Currency,
		"availability": product.Availability,
		"category":     product.Category,
		"url":          product.URL,
	}
	if product.OriginalPrice != 0 {
		row["original_price"] = product.OriginalPrice
	}
	if product.DiscountAmount != 0 {
		row["discount_amount"] = product.DiscountAmount
	}
	if product.SavingsPercent != 0 {
		row["savings_percent"] = product.SavingsPercent
	}
	if product.UnitPrice != 0 {
		row["unit_price"] = product.UnitPrice
	}
	if product.UnitLabel != "" {
		row["unit_label"] = product.UnitLabel
	}
	if product.NutritionStatus != "" {
		row["nutrition_status"] = product.NutritionStatus
	}
	if product.Nutrition != nil && product.Nutrition.Per100g != nil {
		if product.Nutrition.Per100g.EnergyKCal != 0 {
			row["energy_kcal_100g"] = product.Nutrition.Per100g.EnergyKCal
		}
		if product.Nutrition.Per100g.SugarsG != 0 {
			row["sugars_g_100g"] = product.Nutrition.Per100g.SugarsG
		}
	}
	if product.PackLabel != "" {
		row["pack_label"] = product.PackLabel
	}
	if len(product.PromotionText) > 0 {
		row["promotion_text"] = strings.Join(product.PromotionText, " | ")
	}
	if product.HasPromotion {
		row["has_promotion"] = true
	}
	if product.HasDiscount {
		row["has_discount"] = true
	}
	return row
}

func emitStructuredOutput(cmd *cobra.Command, flags *rootFlags, payload any, prov DataProvenance, count int, humanRows []map[string]any) error {
	return emitStructuredOutputWithCompact(cmd, flags, payload, nil, prov, count, humanRows)
}

func emitStructuredOutputWithCompact(cmd *cobra.Command, flags *rootFlags, payload any, compactPayload any, prov DataProvenance, count int, humanRows []map[string]any) error {
	prov = applyPreferredStoreProvenance(flags, prov)
	selected := payload
	if compactPayload != nil && flags.compact && flags.selectFields == "" {
		selected = compactPayload
	}
	raw, err := json.Marshal(selected)
	if err != nil {
		return err
	}
	if wantsHumanTable(cmd.OutOrStdout(), flags) {
		printProvenance(cmd, count, prov)
		if len(humanRows) > 0 {
			return printAutoTable(cmd.OutOrStdout(), humanRows)
		}
		return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
	}
	if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
		filtered := raw
		if flags.selectFields != "" {
			filtered = filterFields(filtered, flags.selectFields)
		} else if flags.compact {
			filtered = compactFields(filtered)
		}
		wrapped, err := wrapWithProvenance(filtered, prov, resolvedMetaMode(flags))
		if err != nil {
			return err
		}
		return printOutput(cmd.OutOrStdout(), wrapped, true)
	}
	return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
}
