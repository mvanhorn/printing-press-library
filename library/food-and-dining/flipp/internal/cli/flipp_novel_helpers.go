package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/flipp/internal/client"
	"github.com/spf13/cobra"
)

const defaultFlippLocale = "en-us"

var staplePacks = map[string][]string{
	"groceries": {"milk", "eggs", "bread", "butter", "cheese", "yogurt", "chicken", "beef", "rice", "pasta", "coffee", "apples", "bananas", "potatoes"},
	"household": {"paper towel", "toilet paper", "laundry detergent", "dish soap", "garbage bags", "cleaner"},
	"baby":      {"diapers", "formula", "baby wipes", "baby food"},
	"pet":       {"dog food", "cat food", "cat litter", "dog treats"},
	"snacks":    {"chips", "crackers", "cookies", "granola bars", "popcorn"},
	"beverages": {"juice", "soda", "water", "sparkling water", "coffee"},
	"pharmacy":  {"vitamins", "pain relief", "cold medicine", "allergy medicine"},
	"produce":   {"apples", "bananas", "berries", "lettuce", "potatoes", "onions"},
	"protein":   {"chicken", "beef", "pork", "fish", "eggs"},
	"pantry":    {"rice", "pasta", "cereal", "flour", "sugar", "beans"},
}

type flippSearchResponse struct {
	Items     []flippItem       `json:"items"`
	EcomItems []flippItem       `json:"ecom_items"`
	Coupons   []json.RawMessage `json:"coupons"`
	CouponsV2 []json.RawMessage `json:"coupons_v2"`
}

type flippItem struct {
	ID               int      `json:"id"`
	ItemID           string   `json:"item_id"`
	Name             string   `json:"name"`
	Description      string   `json:"description"`
	Brand            string   `json:"brand"`
	CurrentPrice     *float64 `json:"current_price"`
	OriginalPrice    *float64 `json:"original_price"`
	Merchant         string   `json:"merchant"`
	MerchantName     string   `json:"merchant_name"`
	ItemType         string   `json:"item_type"`
	SaleStory        string   `json:"sale_story"`
	ValidTo          string   `json:"valid_to"`
	FlyerID          *int     `json:"flyer_id"`
	ImageURL         string   `json:"image_url"`
	ClippingImageURL string   `json:"clipping_image_url"`
	CleanImageURL    string   `json:"clean_image_url"`
	Category1        string   `json:"_L1"`
	Category2        string   `json:"_L2"`
}

type flippFlyerData struct {
	RefreshedAt             string            `json:"refreshed_at"`
	CategorySortCSV         string            `json:"category_sort_csv"`
	CouponCategorySortCSV   string            `json:"coupon_category_sort_csv"`
	Flyers                  []flippFlyer      `json:"flyers"`
	Coupons                 []flippCoupon     `json:"coupons"`
	LoyaltyProgramCoupons   []flippCoupon     `json:"loyalty_program_coupons"`
	FlyerItemCoupons        []flippCoupon     `json:"flyer_item_coupons"`
	UnstructuredCouponBlobs []json.RawMessage `json:"-"`
}

type flippFlyer struct {
	ID           int      `json:"id"`
	Name         string   `json:"name"`
	Merchant     string   `json:"merchant"`
	MerchantID   int      `json:"merchant_id"`
	ValidFrom    string   `json:"valid_from"`
	ValidTo      string   `json:"valid_to"`
	Categories   []string `json:"categories"`
	ThumbnailURL string   `json:"thumbnail_url"`
}

type flippCoupon struct {
	ID          int      `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Merchant    string   `json:"merchant"`
	ValidTo     string   `json:"valid_to"`
	Clipped     bool     `json:"clipped"`
	Value       string   `json:"value"`
	Price       *float64 `json:"price"`
}

type fetchFailure struct {
	Query string `json:"query,omitempty"`
	ID    string `json:"id,omitempty"`
	Error string `json:"error"`
}

func splitCSV(input string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, part := range strings.Split(input, ",") {
		part = strings.TrimSpace(part)
		if part == "" || seen[strings.ToLower(part)] {
			continue
		}
		seen[strings.ToLower(part)] = true
		out = append(out, part)
	}
	return out
}

func merchantName(item flippItem) string {
	if item.MerchantName != "" {
		return item.MerchantName
	}
	if item.Merchant != "" {
		return item.Merchant
	}
	return "Unknown merchant"
}

func itemPrice(item flippItem) float64 {
	if item.CurrentPrice == nil {
		return math.Inf(1)
	}
	return *item.CurrentPrice
}

func discountPct(item flippItem) *int {
	if item.CurrentPrice == nil || item.OriginalPrice == nil || *item.OriginalPrice <= 0 {
		return nil
	}
	pct := int(math.Round(((*item.OriginalPrice - *item.CurrentPrice) / *item.OriginalPrice) * 100))
	return &pct
}

func itemImage(item flippItem) string {
	for _, value := range []string{item.ClippingImageURL, item.CleanImageURL, item.ImageURL} {
		if value != "" {
			return value
		}
	}
	return ""
}

func fetchFlippSearch(ctx context.Context, c *client.Client, query, zip, locale, sortType string) (flippSearchResponse, error) {
	if locale == "" {
		locale = defaultFlippLocale
	}
	params := map[string]string{
		"q":           query,
		"postal_code": zip,
		"locale":      locale,
	}
	if sortType != "" {
		params["sort_type"] = sortType
	}
	data, err := c.GetNoCache(ctx, "/items/search", params)
	if err != nil {
		return flippSearchResponse{}, err
	}
	var res flippSearchResponse
	if err := json.Unmarshal(data, &res); err != nil {
		return flippSearchResponse{}, fmt.Errorf("parse search response: %w", err)
	}
	return res, nil
}

func fetchFlippData(ctx context.Context, c *client.Client, zip, locale string) (flippFlyerData, error) {
	if locale == "" {
		locale = defaultFlippLocale
	}
	data, err := c.GetNoCache(ctx, "/data", map[string]string{"postal_code": zip, "locale": locale})
	if err != nil {
		return flippFlyerData{}, err
	}
	var res flippFlyerData
	if err := json.Unmarshal(data, &res); err != nil {
		return flippFlyerData{}, fmt.Errorf("parse flyer data: %w", err)
	}
	return res, nil
}

func parseFlippTime(value string) (time.Time, bool) {
	if value == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05-07:00", "2006-01-02 15:04:05 -0700"} {
		t, err := time.Parse(layout, value)
		if err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

type unitPriceInfo struct {
	Value   *float64 `json:"value"`
	Unit    string   `json:"unit,omitempty"`
	Size    *float64 `json:"size,omitempty"`
	Warning string   `json:"warning,omitempty"`
}

var quantityPattern = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?|\d+\s*/\s*\d+)\s*(kg|g|l|ml|fl\s*oz|floz|fz|oz|lb|lbs|gal|gallon|gallons|qt|quart|quarts|pt|pint|pints)\b`)

func parseQuantitySize(value string) (float64, error) {
	value = strings.ReplaceAll(value, " ", "")
	if parts := strings.Split(value, "/"); len(parts) == 2 {
		numerator, err := strconv.ParseFloat(parts[0], 64)
		if err != nil {
			return 0, err
		}
		denominator, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			return 0, err
		}
		if denominator == 0 {
			return 0, fmt.Errorf("zero denominator")
		}
		return numerator / denominator, nil
	}
	return strconv.ParseFloat(value, 64)
}

func parseUnitPrice(name string, price *float64) unitPriceInfo {
	if price == nil || *price <= 0 {
		return unitPriceInfo{Warning: "missing_price"}
	}
	m := quantityPattern.FindStringSubmatch(name)
	if len(m) < 3 {
		return unitPriceInfo{Warning: "size_not_found"}
	}
	size, err := parseQuantitySize(m[1])
	if err != nil || size <= 0 {
		return unitPriceInfo{Warning: "size_not_parseable"}
	}
	unit := strings.ReplaceAll(strings.ToLower(m[2]), " ", "")
	baseUnit := unit
	baseSize := size
	switch unit {
	case "g":
		baseUnit = "kg"
		baseSize = size / 1000
	case "ml":
		baseUnit = "l"
		baseSize = size / 1000
	case "floz", "fz":
		baseUnit = "gal"
		baseSize = size / 128
	case "oz":
		baseUnit = "lb"
		baseSize = size / 16
	case "lbs":
		baseUnit = "lb"
	case "gallon", "gallons":
		baseUnit = "gal"
	case "qt", "quart", "quarts":
		baseUnit = "gal"
		baseSize = size / 4
	case "pt", "pint", "pints":
		baseUnit = "gal"
		baseSize = size / 8
	}
	if baseSize <= 0 {
		return unitPriceInfo{Warning: "size_not_parseable"}
	}
	value := *price / baseSize
	value = math.Round(value*100) / 100
	return unitPriceInfo{Value: &value, Unit: baseUnit, Size: &baseSize}
}

func printRowsOrJSON(cmd *cobra.Command, flags *rootFlags, payload any, headers []string, rows [][]string) error {
	if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) || flags.selectFields != "" || flags.compact || flags.csv || flags.quiet || flags.plain {
		return printJSONFiltered(cmd.OutOrStdout(), payload, flags)
	}
	return flags.printTable(cmd, headers, rows)
}

func sortByPrice(items []flippItem) {
	sort.SliceStable(items, func(i, j int) bool {
		return itemPrice(items[i]) < itemPrice(items[j])
	})
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func hasCategory(categories []string, want string) bool {
	for _, category := range categories {
		if strings.EqualFold(category, want) {
			return true
		}
	}
	return false
}

func containsStringFold(values []string, want string) bool {
	for _, value := range values {
		if strings.EqualFold(value, want) {
			return true
		}
	}
	return false
}

func valueOrZero(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}

func matchesSearchIntent(item flippItem, query string) bool {
	name := strings.ToLower(item.Name)
	haystack := strings.ToLower(strings.Join([]string{
		item.Name,
		item.Description,
		item.Brand,
		item.Category1,
		item.Category2,
	}, " "))
	tokens := strings.FieldsFunc(strings.ToLower(query), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9')
	})
	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if len(token) < 3 {
			continue
		}
		if (strings.Contains(haystack, token) || strings.Contains(haystack, strings.TrimSuffix(token, "s"))) && !isPoorStapleMatch(name, token) {
			return true
		}
	}
	return false
}

func isPoorStapleMatch(name, token string) bool {
	switch token {
	case "milk":
		if strings.Contains(name, "milk-bone") || strings.Contains(name, "milk bone") {
			return true
		}
		candyMatch := strings.Contains(name, "choc") ||
			strings.Contains(name, " chg") ||
			strings.Contains(name, "candy") ||
			strings.Contains(name, "crunch bar")
		return candyMatch && !strings.Contains(name, "chocolate milk")
	}
	return false
}
