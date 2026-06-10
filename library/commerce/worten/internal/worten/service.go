package worten

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/worten/internal/client"
)

const baseURL = "https://www.worten.pt"

var (
	uuidPattern    = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	gtinPattern    = regexp.MustCompile(`"gtin13":"((?i:[0-9a-f-]{36}))"`)
	detailsPattern = regexp.MustCompile(`/worten-api/products/details",\{"query":\{"id":\["([^"]+)"\],"ref":"product_id"\}\}`)
)

var defaultHeaders = map[string]string{
	"accept":          "application/json, text/html;q=0.9, */*;q=0.8",
	"accept-language": "pt-PT,pt;q=0.9,en;q=0.8",
	"user-agent":      "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/137.0.0.0 Safari/537.36",
	"referer":         baseURL + "/",
	"origin":          baseURL,
}

type Service struct {
	client *client.Client
	paths  Paths
}

type Paths struct {
	SnapshotDir string
	IDCachePath string
}

type ResolveOutput struct {
	Input        string `json:"input"`
	ProductID    string `json:"productId"`
	CanonicalURL string `json:"canonicalUrl,omitempty"`
	SKU          string `json:"sku,omitempty"`
	Brand        string `json:"brand,omitempty"`
}

type Snapshot struct {
	CapturedAt   string         `json:"capturedAt"`
	SourceInput  string         `json:"sourceInput"`
	Product      map[string]any `json:"product"`
	Buyer        map[string]any `json:"buyer"`
	Specs        map[string]any `json:"specs"`
	SnapshotPath string         `json:"snapshotPath,omitempty"`
}

type SnapshotOptions struct {
	Refresh   bool
	CacheOnly bool
}

type StockOptions struct {
	PostalCode string
	RadiusKm   int
}

func New(c *client.Client) (*Service, error) {
	if c == nil {
		return nil, errors.New("worten service requires a client")
	}
	paths, err := resolvePaths()
	if err != nil {
		return nil, err
	}
	return &Service{client: c, paths: paths}, nil
}

func resolvePaths() (Paths, error) {
	if snapshotDir := os.Getenv("WORTEN_PP_SNAPSHOT_DIR"); snapshotDir != "" {
		idCachePath := os.Getenv("WORTEN_PP_ID_CACHE_PATH")
		if idCachePath == "" {
			idCachePath = filepath.Join(filepath.Dir(snapshotDir), "id-cache.json")
		}
		return Paths{SnapshotDir: snapshotDir, IDCachePath: idCachePath}, nil
	}

	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return Paths{}, fmt.Errorf("resolve user home: %w", err)
		}
		dataHome = filepath.Join(homeDir, ".local", "share")
	}

	root := filepath.Join(dataHome, "worten-pp-cli")
	return Paths{
		SnapshotDir: filepath.Join(root, "snapshots"),
		IDCachePath: filepath.Join(root, "id-cache.json"),
	}, nil
}

func (s *Service) Resolve(ctx context.Context, input string, raw bool) (any, error) {
	productID, err := s.resolveProductID(ctx, input)
	if err != nil {
		return nil, err
	}
	cache, err := s.loadIDCache()
	if err != nil {
		return nil, err
	}
	product, err := s.fetchProduct(ctx, productID)
	if err != nil {
		return nil, err
	}
	record := updateIDCacheEntry(cache, input, product)
	if err := s.saveIDCache(cache); err != nil {
		return nil, err
	}
	if raw {
		return map[string]any{
			"productId": productID,
			"record":    record,
		}, nil
	}
	return ResolveOutput{
		Input:        input,
		ProductID:    productID,
		CanonicalURL: toString(record["canonicalUrl"]),
		SKU:          toString(record["sku"]),
		Brand:        toString(record["brand"]),
	}, nil
}

func (s *Service) Product(ctx context.Context, input string, raw bool) (any, error) {
	productID, err := s.resolveProductID(ctx, input)
	if err != nil {
		return nil, err
	}
	detailsRaw, err := s.fetchDetailsRaw(ctx, productID)
	if err != nil {
		return nil, err
	}
	if raw {
		return detailsRaw, nil
	}
	return normalizeProductDetails(detailsRaw)
}

func (s *Service) Buyer(ctx context.Context, input string, raw bool) (any, error) {
	productID, err := s.resolveProductID(ctx, input)
	if err != nil {
		return nil, err
	}
	detailsRaw, specsRaw, err := s.fetchBuyerPayload(ctx, productID)
	if err != nil {
		return nil, err
	}
	if raw {
		return map[string]any{
			"details": detailsRaw,
			"specs":   specsRaw,
		}, nil
	}
	return normalizeBuyerView(detailsRaw, specsRaw)
}

func (s *Service) Specs(ctx context.Context, input string, raw bool) (any, error) {
	productID, err := s.resolveProductID(ctx, input)
	if err != nil {
		return nil, err
	}
	specsRaw, err := s.fetchSpecsRaw(ctx, productID)
	if err != nil {
		return nil, err
	}
	if raw {
		return specsRaw, nil
	}
	return map[string]any{
		"productId":      productID,
		"specifications": specsRaw,
	}, nil
}

func (s *Service) Stock(ctx context.Context, input string, options StockOptions, raw bool) (any, error) {
	productID, err := s.resolveProductID(ctx, input)
	if err != nil {
		return nil, err
	}
	detailsRaw, err := s.fetchDetailsRaw(ctx, productID)
	if err != nil {
		return nil, err
	}
	product, err := normalizeProductDetails(detailsRaw)
	if err != nil {
		return nil, err
	}
	offerID := toString(deepGet(detailsRaw, "productsData", "products", 0, "woffer", "offer_id"))
	offer := findOffer(detailsRaw, offerID)
	seller := findSeller(detailsRaw, toString(valueAt(offer, "seller_id")))
	var storeSearch map[string]any
	if options.PostalCode != "" && offerID != "" {
		storeSearch, err = s.fetchOfferStoreStock(ctx, offerID, options.PostalCode, options.RadiusKm)
		if err != nil {
			return nil, err
		}
	}
	stock := normalizeStock(detailsRaw, offer, seller, storeSearch)
	if raw {
		return map[string]any{
			"product":     detailsRaw,
			"storeSearch": storeSearch,
		}, nil
	}
	return map[string]any{
		"productId": product["productId"],
		"sku":       product["sku"],
		"name":      product["name"],
		"url":       product["url"],
		"stock":     stock,
	}, nil
}

func (s *Service) Suggest(ctx context.Context, query string, max int, raw bool) (any, error) {
	params := map[string]string{
		"q":   query,
		"max": strconv.Itoa(max),
	}
	payload, err := s.getJSON(ctx, "/worten-api/search-suggestions", params)
	if err != nil {
		return nil, err
	}
	if raw {
		return payload, nil
	}
	suggestions := getSlice(payload, "suggestions")
	rows := make([]map[string]any, 0, len(suggestions))
	for _, item := range suggestions {
		itemMap, ok := item.(map[string]any)
		if !ok {
			continue
		}
		categoryRows := make([]map[string]any, 0)
		for _, category := range getSlice(itemMap, "categories") {
			categoryMap, ok := category.(map[string]any)
			if !ok {
				continue
			}
			categoryRows = append(categoryRows, map[string]any{
				"label":  toString(categoryMap["lbl"]),
				"filter": toString(categoryMap["filter"]),
				"value":  toString(categoryMap["value"]),
			})
		}
		rows = append(rows, map[string]any{
			"text":       toString(itemMap["text"]),
			"categories": categoryRows,
		})
	}
	return map[string]any{
		"query":       query,
		"count":       len(rows),
		"suggestions": rows,
	}, nil
}

func (s *Service) Search(ctx context.Context, query string, contexts []string, page int, raw bool) (any, error) {
	body := map[string]any{
		"params": map[string]any{
			"query": query,
			"page":  page,
		},
		"contexts": contexts,
	}
	payload, err := s.postQueryJSON(ctx, "/worten-api/search-products", body)
	if err != nil {
		return nil, err
	}
	if raw {
		return payload, nil
	}
	searchResponse := getMap(payload, "searchResponse")
	hits := normalizeSearchHits(sliceMaps(getSlice(searchResponse, "hits")))
	return map[string]any{
		"experimental": true,
		"payload":      body,
		"count":        len(hits),
		"hasNextPage":  searchResponse["hasNextPage"],
		"items":        hits,
	}, nil
}

func (s *Service) Snapshot(ctx context.Context, input string, options SnapshotOptions, raw bool) (any, error) {
	snapshot, err := s.getSnapshot(ctx, input, options)
	if err != nil {
		return nil, err
	}
	if raw {
		return snapshot, nil
	}
	return summarizeSnapshot(snapshot), nil
}

func (s *Service) getSnapshot(ctx context.Context, input string, options SnapshotOptions) (Snapshot, error) {
	cache, err := s.loadIDCache()
	if err != nil {
		return Snapshot{}, err
	}
	productID, err := s.resolveProductID(ctx, input)
	if err != nil {
		return Snapshot{}, err
	}
	if existing, err := s.readSnapshot(productID); err == nil && !options.Refresh {
		return existing, nil
	}
	if options.CacheOnly {
		return Snapshot{}, fmt.Errorf("no cached snapshot available for %s", productID)
	}
	detailsRaw, specsRaw, err := s.fetchBuyerPayload(ctx, productID)
	if err != nil {
		if existing, readErr := s.readSnapshot(productID); readErr == nil && options.Refresh {
			return existing, nil
		}
		return Snapshot{}, err
	}
	product, err := normalizeProductDetails(detailsRaw)
	if err != nil {
		return Snapshot{}, err
	}
	buyer, err := normalizeBuyerView(detailsRaw, specsRaw)
	if err != nil {
		return Snapshot{}, err
	}
	snapshot := Snapshot{
		CapturedAt:  time.Now().UTC().Format(time.RFC3339Nano),
		SourceInput: input,
		Product:     product,
		Buyer:       buyer,
		Specs: map[string]any{
			"productId":      productID,
			"specifications": specsRaw,
		},
	}
	updateIDCacheEntry(cache, input, product)
	if err := s.saveIDCache(cache); err != nil {
		return Snapshot{}, err
	}
	return s.writeSnapshot(snapshot)
}

func (s *Service) resolveProductID(ctx context.Context, input string) (string, error) {
	cache, err := s.loadIDCache()
	if err != nil {
		return "", err
	}
	if cached := findCachedProductID(cache, input); cached != "" {
		return cached, nil
	}
	if isUUID(input) {
		return input, nil
	}
	productURL := asProductURL(input)
	var lastHTML string
	for attempt := 0; attempt < 3; attempt++ {
		html, err := s.fetchHTML(ctx, productURL)
		if err != nil {
			return "", err
		}
		lastHTML = html
		if match := gtinPattern.FindStringSubmatch(html); len(match) > 1 && isUUID(match[1]) {
			return match[1], nil
		}
		if match := detailsPattern.FindStringSubmatch(html); len(match) > 1 {
			return match[1], nil
		}
		if !isChallengePage(html) {
			break
		}
		if err := sleepContext(ctx, time.Duration(attempt+1)*500*time.Millisecond); err != nil {
			return "", err
		}
	}
	return "", fmt.Errorf("could not extract product id from %s", normalizeCacheInput(productURL)+": "+challengeHint(lastHTML))
}

func sleepContext(ctx context.Context, wait time.Duration) error {
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func challengeHint(html string) string {
	if isChallengePage(html) {
		return "challenge page returned"
	}
	return "no product identifier markers found"
}

func (s *Service) fetchBuyerPayload(ctx context.Context, productID string) (map[string]any, map[string]any, error) {
	detailsRaw, err := s.fetchDetailsRaw(ctx, productID)
	if err != nil {
		return nil, nil, err
	}
	specsRaw, err := s.fetchSpecsRaw(ctx, productID)
	if err != nil {
		return nil, nil, err
	}
	return detailsRaw, specsRaw, nil
}

func (s *Service) fetchProduct(ctx context.Context, productID string) (map[string]any, error) {
	detailsRaw, err := s.fetchDetailsRaw(ctx, productID)
	if err != nil {
		return nil, err
	}
	return normalizeProductDetails(detailsRaw)
}

func (s *Service) fetchDetailsRaw(ctx context.Context, productID string) (map[string]any, error) {
	return s.getJSON(ctx, "/worten-api/products/details", map[string]string{
		"id":  productID,
		"ref": "product_id",
	})
}

func (s *Service) fetchSpecsRaw(ctx context.Context, productID string) (map[string]any, error) {
	return s.getJSON(ctx, "/worten-api/products/technical-specifications", map[string]string{
		"id":  productID,
		"ref": "product_id",
	})
}

func (s *Service) fetchOfferStoreStock(ctx context.Context, offerID, searchQuery string, radiusKm int) (map[string]any, error) {
	return s.getJSON(ctx, "/worten-api/offers/get-offer-stock", map[string]string{
		"offerId":     offerID,
		"searchQuery": searchQuery,
		"radius":      strconv.Itoa(radiusKm),
	})
}

func (s *Service) getJSON(ctx context.Context, path string, params map[string]string) (map[string]any, error) {
	data, err := s.client.GetWithHeaders(ctx, path, params, defaultHeaders)
	if err != nil {
		return nil, err
	}
	return decodeObject(data)
}

func (s *Service) postQueryJSON(ctx context.Context, path string, body any) (map[string]any, error) {
	headers := cloneHeaders(defaultHeaders)
	headers["content-type"] = "application/json"
	data, _, err := s.client.PostQueryWithParamsAndHeaders(ctx, path, nil, body, headers)
	if err != nil {
		return nil, err
	}
	return decodeObject(data)
}

func (s *Service) fetchHTML(ctx context.Context, rawURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	for key, value := range defaultHeaders {
		req.Header.Set(key, value)
	}
	resp, err := s.client.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("GET %s returned HTTP %d", rawURL, resp.StatusCode)
	}
	body, err := ioReadAllString(resp)
	if err != nil {
		return "", err
	}
	return body, nil
}

func ioReadAllString(resp *http.Response) (string, error) {
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *Service) readSnapshot(productID string) (Snapshot, error) {
	raw, err := os.ReadFile(filepath.Join(s.paths.SnapshotDir, productID+".json"))
	if err != nil {
		return Snapshot{}, err
	}
	var snapshot Snapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return Snapshot{}, err
	}
	snapshot.SnapshotPath = filepath.Join(s.paths.SnapshotDir, productID+".json")
	return snapshot, nil
}

func (s *Service) writeSnapshot(snapshot Snapshot) (Snapshot, error) {
	if err := os.MkdirAll(s.paths.SnapshotDir, 0o755); err != nil {
		return Snapshot{}, err
	}
	path := filepath.Join(s.paths.SnapshotDir, toString(snapshot.Product["productId"])+".json")
	raw, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return Snapshot{}, err
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return Snapshot{}, err
	}
	snapshot.SnapshotPath = path
	return snapshot, nil
}

func (s *Service) loadIDCache() (map[string]any, error) {
	raw, err := os.ReadFile(s.paths.IDCachePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]any{"products": map[string]any{}}, nil
		}
		return nil, err
	}
	var cache map[string]any
	if err := json.Unmarshal(raw, &cache); err != nil {
		return nil, err
	}
	if cache == nil {
		cache = map[string]any{}
	}
	if _, ok := cache["products"]; !ok {
		cache["products"] = map[string]any{}
	}
	return cache, nil
}

func (s *Service) saveIDCache(cache map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(s.paths.IDCachePath), 0o755); err != nil {
		return err
	}
	cache["updatedAt"] = time.Now().UTC().Format(time.RFC3339Nano)
	raw, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.paths.IDCachePath, raw, 0o644)
}

func summarizeSnapshot(snapshot Snapshot) map[string]any {
	stock := snapshot.Buyer["stock"]
	if stock == nil {
		stock = snapshot.Product["stock"]
	}
	return map[string]any{
		"capturedAt":   snapshot.CapturedAt,
		"productId":    snapshot.Product["productId"],
		"sku":          snapshot.Product["sku"],
		"name":         snapshot.Product["name"],
		"brand":        snapshot.Product["brand"],
		"category":     snapshot.Buyer["category"],
		"soldByWorten": snapshot.Buyer["soldByWorten"],
		"price":        snapshot.Buyer["price"],
		"inStock":      snapshot.Buyer["inStock"],
		"stock":        stock,
		"snapshotPath": snapshot.SnapshotPath,
	}
}

func decodeObject(data []byte) (map[string]any, error) {
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func cloneHeaders(headers map[string]string) map[string]string {
	cloned := make(map[string]string, len(headers))
	for key, value := range headers {
		cloned[key] = value
	}
	return cloned
}

func isUUID(value string) bool {
	return uuidPattern.MatchString(strings.TrimSpace(value))
}

func asProductURL(input string) string {
	switch {
	case strings.HasPrefix(input, "http://"), strings.HasPrefix(input, "https://"):
		return input
	case strings.HasPrefix(input, "/"):
		return baseURL + input
	default:
		return baseURL + "/produtos/" + input
	}
}

func normalizeCacheInput(input string) string {
	value := strings.TrimSpace(input)
	value = strings.ReplaceAll(value, "&amp;", "&")
	return regexp.MustCompile(`\?bvstate=[^"'\s)]+`).ReplaceAllString(value, "")
}

func findCachedProductID(cache map[string]any, input string) string {
	if isUUID(input) {
		return input
	}
	products, ok := cache["products"].(map[string]any)
	if !ok {
		return ""
	}
	normalizedInput := normalizeCacheInput(input)
	for rawURL, rawRecord := range products {
		record, ok := rawRecord.(map[string]any)
		if !ok {
			continue
		}
		productID := toString(record["productId"])
		if productID == "" {
			continue
		}
		if normalizeCacheInput(rawURL) == normalizedInput {
			return productID
		}
		if canonical := toString(record["canonicalUrl"]); canonical != "" && normalizeCacheInput(canonical) == normalizedInput {
			return productID
		}
	}
	return ""
}

func updateIDCacheEntry(cache map[string]any, input string, product map[string]any) map[string]any {
	products, _ := cache["products"].(map[string]any)
	if products == nil {
		products = map[string]any{}
		cache["products"] = products
	}
	urlKey := normalizeCacheURL(input, toString(product["url"]))
	entry := map[string]any{
		"productId":    product["productId"],
		"canonicalUrl": product["url"],
		"sku":          product["sku"],
		"brand":        product["brand"],
		"refreshedAt":  time.Now().UTC().Format(time.RFC3339Nano),
	}
	products[urlKey] = entry
	if canonicalURL := toString(product["url"]); canonicalURL != "" {
		products[canonicalURL] = entry
	}
	return entry
}

func normalizeCacheURL(input, fallbackURL string) string {
	if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") {
		return normalizeCacheInput(input)
	}
	if fallbackURL != "" {
		return normalizeCacheInput(fallbackURL)
	}
	return normalizeCacheInput(input)
}

func isChallengePage(html string) bool {
	normalized := strings.ToLower(html)
	return strings.Contains(normalized, "<title>just a moment...</title>") ||
		strings.Contains(normalized, "challenges.cloudflare.com") ||
		strings.Contains(normalized, "/cdn-cgi/challenge-platform/")
}

func normalizeProductDetails(response map[string]any) (map[string]any, error) {
	product := firstMap(getSliceMap(response, "productsData", "products"))
	if product == nil {
		return nil, errors.New("no product data returned")
	}
	offerID := toString(deepGet(product, "woffer", "offer_id"))
	offer := findOffer(response, offerID)
	seller := findSeller(response, toString(valueAt(offer, "seller_id")))
	canonical := firstMap(getSliceMap(response, "productsCanonicalsData", "web_items"))
	stock := normalizeStock(response, offer, seller, nil)
	imageTransform := toString(deepGet(product, "image", "transforms", "default"))

	return map[string]any{
		"productId":       product["id"],
		"sku":             deepGet(product, "meta", "refs", "sku"),
		"entityId":        deepGet(product, "meta", "refs", "entity_id"),
		"ironModelId":     deepGet(product, "meta", "refs", "iron_model_id"),
		"ean":             defaultSlice(deepGet(product, "meta", "refs", "ean")),
		"name":            deepGet(product, "properties", "text", "name"),
		"brand":           deepGet(product, "properties", "text", "brand_name"),
		"longDescription": deepGet(product, "properties", "text", "long_description"),
		"energyClass":     deepGet(product, "properties", "text", "energy-class-new"),
		"url":             prefixURL(toString(valueAt(canonical, "url"))),
		"imageUrl":        prefixImageURL(imageTransform),
		"badges":          defaultSlice(deepGet(product, "labels", "badges")),
		"categories":      defaultSlice(deepGet(product, "labels", "categories")),
		"ratings":         product["ratings"],
		"price":           offer["price"],
		"inStock":         stock["available"],
		"stock":           stock,
		"shipping":        offer["shipping"],
		"seller": map[string]any{
			"id":           valueAt(seller, "seller_id"),
			"name":         valueAt(seller, "name"),
			"premiumState": valueAt(seller, "premium_state"),
			"rating":       valueAt(seller, "rating"),
			"orders":       deepGet(seller, "status", "orders"),
		},
		"soldByWorten": toString(valueAt(seller, "name")) == "Worten",
	}, nil
}

func normalizeStock(response, offer, seller, storeSearchResponse map[string]any) map[string]any {
	available := deepGet(offer, "stock", "is_in_stock")
	shipping := getMap(offer, "shipping")
	sellerName := toString(valueAt(seller, "name"))
	soldByWorten := sellerName == "Worten"
	storePricingData := getMap(response, "storePricingData")
	localArea := normalizeLocalAreaStock(storePricingData)
	storeSearch := normalizeOfferStoreSearch(storeSearchResponse)
	notes := make([]string, 0)

	if storePricingData == nil {
		if storeSearchResponse != nil {
			notes = append(notes, "No store-level stock was returned by the base product payload, so the postcode lookup was used instead.")
		} else {
			notes = append(notes, "No store-level stock was returned by the current Worten payload.")
		}
	}
	if storeSearchResponse != nil && storeSearch == nil {
		notes = append(notes, "The nearby-store lookup did not return a usable store list.")
	}
	if localArea != nil && localArea["nearLisbon"] == nil {
		notes = append(notes, "Local-area stock exists, but the payload does not expose enough location detail to tag Lisbon automatically.")
	}

	return map[string]any{
		"available": available,
		"status": func() string {
			switch available {
			case true:
				return "in_stock"
			case false:
				return "out_of_stock"
			default:
				return "unknown"
			}
		}(),
		"seller": map[string]any{
			"name":         sellerName,
			"soldByWorten": soldByWorten,
		},
		"shipping":    shippingSummary(shipping),
		"localArea":   localArea,
		"storeSearch": storeSearch,
		"signals": map[string]any{
			"storePickupUiEnabled":      deepGet(response, "features", "show_free_delivery_installation_pickup"),
			"storePricingAvailable":     storePricingData != nil,
			"postalCodeLookupAvailable": storeSearchResponse != nil,
		},
		"notes": notes,
	}
}

func shippingSummary(shipping map[string]any) map[string]any {
	if shipping == nil {
		return nil
	}
	return map[string]any{
		"price":        shipping["price"],
		"leadTimeDays": shipping["lead_time"],
		"minDays":      shipping["min_time"],
		"maxDays":      shipping["max_time"],
		"safetyDays":   shipping["safety_time"],
	}
}

func normalizeOfferStoreSearch(response map[string]any) map[string]any {
	if response == nil {
		return nil
	}
	entries := getSlice(response, "entries")
	stores := make([]map[string]any, 0, len(entries))
	availableStoreCount := 0
	values := make([]string, 0)
	for _, entry := range entries {
		entryMap, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		store := normalizeOfferStoreEntry(entryMap)
		stores = append(stores, store)
		values = append(values, toString(store["name"]), toString(store["city"]))
		if status := toString(store["status"]); status == "available" || status == "low" {
			availableStoreCount++
		}
	}
	notes := []string{}
	switch {
	case len(stores) == 0:
		notes = append(notes, "Worten returned no nearby stores for this query.")
	case availableStoreCount == 0:
		notes = append(notes, "Nearby stores were found, but none reported pickup stock for this offer.")
	}
	return map[string]any{
		"code":                response["code"],
		"resolvedLocation":    maybeLocation(response["latitude"], response["longitude"]),
		"storeCount":          len(stores),
		"availableStoreCount": availableStoreCount,
		"stores":              stores,
		"nearLisbon":          detectLisbonArea(values),
		"notes":               notes,
	}
}

func normalizeOfferStoreEntry(entry map[string]any) map[string]any {
	store := getMap(entry, "store")
	if store == nil {
		store = map[string]any{}
	}
	status := strings.ToLower(toString(entry["status"]))
	features := defaultStringSlice(store["features"])
	if len(features) == 0 {
		features = defaultStringSlice(entry["features"])
	}
	return map[string]any{
		"id":               firstNonEmpty(toString(store["id"]), toString(deepGet(store, "_meta", "store_id"))),
		"name":             store["name"],
		"city":             deepGet(store, "address", "city"),
		"postalCode":       deepGet(store, "address", "postalCode"),
		"address":          deepGet(store, "address", "address"),
		"distanceKm":       roundNumber(toFloat(store["distance"]), 2),
		"status":           status,
		"hasStock":         hasStockStatus(status),
		"features":         features,
		"hasExpressPickup": slices.Contains(features, "PIS_EXPRESS"),
		"hasPickup":        slices.Contains(features, "PIS"),
		"nearLisbon":       detectLisbonArea([]string{toString(store["name"]), toString(deepGet(store, "address", "city")), toString(deepGet(store, "address", "district"))}),
		"url":              prefixURL(toString(store["URL"])),
	}
}

func normalizeLocalAreaStock(storePricingData map[string]any) map[string]any {
	if storePricingData == nil {
		return nil
	}
	favoriteStores := make([]map[string]any, 0)
	values := []string{
		toString(storePricingData["storeName"]),
		toString(deepGet(storePricingData, "store", "name")),
		toString(deepGet(storePricingData, "store", "city")),
		toString(deepGet(storePricingData, "location", "city")),
	}
	for _, storeRaw := range getSlice(storePricingData, "favoriteStores") {
		store, ok := storeRaw.(map[string]any)
		if !ok {
			continue
		}
		row := map[string]any{
			"id":   firstNonEmpty(toString(store["id"]), toString(store["retekId"])),
			"name": firstNonEmpty(toString(store["name"]), toString(store["storeName"])),
			"city": firstNonEmpty(toString(store["city"]), toString(deepGet(store, "address", "city"))),
		}
		values = append(values, toString(row["name"]), toString(row["city"]))
		favoriteStores = append(favoriteStores, row)
	}
	return map[string]any{
		"storeId":        storePricingData["storeId"],
		"storeName":      firstNonEmpty(toString(storePricingData["storeName"]), toString(deepGet(storePricingData, "store", "name")), toString(deepGet(storePricingData, "store", "storeName"))),
		"favoriteStores": favoriteStores,
		"nearLisbon":     detectLisbonArea(values),
		"raw": map[string]any{
			"retekSkusCount": sliceLen(deepGet(storePricingData, "retekSkus")),
			"fetchError":     storePricingData["fetchError"],
		},
	}
}

func normalizeSearchHits(hits []map[string]any) []map[string]any {
	rows := make([]map[string]any, 0, len(hits))
	for _, hit := range hits {
		product := getMap(hit, "product")
		if product == nil {
			product = getMap(hit, "pack")
		}
		winningOffer := getMap(hit, "winningOffer")
		seller := getMap(winningOffer, "seller")
		price := firstNonNil(
			winningOffer["price"],
			deepGet(winningOffer, "pricing", "final"),
			deepGet(product, "pricing", "final"),
			product["price"],
		)
		rows = append(rows, map[string]any{
			"type": func() string {
				if getMap(hit, "pack") != nil {
					return "pack"
				}
				return "product"
			}(),
			"id":             product["id"],
			"sku":            product["sku"],
			"name":           product["name"],
			"url":            prefixURL(toString(product["url"])),
			"price":          price,
			"inStock":        firstNonNil(winningOffer["isInStock"], product["isInStock"], deepGet(hit, "data", "is_in_stock")),
			"seller":         firstNonEmpty(toString(seller["name"]), toString(deepGet(hit, "data", "seller_uid"))),
			"sellerCategory": winningOffer["sellerCategory"],
			"soldByWorten":   toString(winningOffer["sellerCategory"]) == "FIRST_PARTY" || toString(seller["name"]) == "Worten",
			"categories":     normalizeCategoryIDs(getSlice(product, "categories")),
		})
	}
	return rows
}

func normalizeBuyerView(detailsResponse, specsResponse map[string]any) (map[string]any, error) {
	product, err := normalizeProductDetails(detailsResponse)
	if err != nil {
		return nil, err
	}
	specMap := createSpecMap(specsResponse)
	category := inferBuyerCategory(product, specMap)
	combinedDimensions := parseDimensions(spec(specMap, "caracteristicas fisicas.dimensoes"))
	lifecycle := extractLifecycle(product, specMap)
	annualEnergyKwh, energyPer100CyclesKwh, warnings := extractEnergyMetrics(specMap, category)

	base := map[string]any{
		"category":              category,
		"productId":             product["productId"],
		"sku":                   product["sku"],
		"name":                  product["name"],
		"brand":                 product["brand"],
		"model":                 specMap["referencias.modelo"],
		"url":                   product["url"],
		"soldByWorten":          product["soldByWorten"],
		"price":                 deepGet(product, "price", "final"),
		"listPrice":             deepGet(product, "price", "original"),
		"inStock":               product["inStock"],
		"stock":                 product["stock"],
		"energyClass":           firstNonNil(product["energyClass"], specMap["eficiencia energetica.eficiencia energetica nova"], specMap["caracteristicas especificas.eficiencia energetica"]),
		"annualEnergyKwh":       annualEnergyKwh,
		"energyPer100CyclesKwh": energyPer100CyclesKwh,
		"specWarnings":          warnings,
		"ratings":               product["ratings"],
		"dimensionsCm": map[string]any{
			"height": firstNonNil(parseNumber(specMap["caracteristicas fisicas.altura"]), valueAt(combinedDimensions, "height")),
			"width":  firstNonNil(parseNumber(specMap["caracteristicas fisicas.largura"]), valueAt(combinedDimensions, "width")),
			"depth":  firstNonNil(parseNumber(specMap["caracteristicas fisicas.profundidade"]), valueAt(combinedDimensions, "depth")),
		},
		"lifecycle":       lifecycle,
		"rawSpecSections": extractSectionTitles(specsResponse),
	}

	switch category {
	case "fridge":
		base["capacitiesLiters"] = map[string]any{
			"total": parseNumber(specMap["capacidade.capacidade liquida total l"]),
			"fridge": firstNonNil(
				parseNumber(spec(specMap, "capacidade.cap liquida refrigerador l", "capacidade.capacidade liquida refrigerador l")),
				extractNumber(toString(product["longDescription"]), `capacidade liquida do frigorifico\s+(\d+)`),
			),
			"freezer": firstNonNil(
				parseNumber(spec(specMap, "capacidade.cap liquida congelador l", "capacidade.capacidade liquida congelador l")),
				extractNumber(toString(product["longDescription"]), `capacidade liquida do congelador\s+(\d+)`),
			),
		}
		base["cooling"] = map[string]any{
			"fridge":  specMap["caracteristicas especificas.sistema de frio refrigerador"],
			"freezer": specMap["caracteristicas especificas.sistema de frio cong"],
			"noFrost": strings.Contains(strings.ToLower(toString(product["name"])), "no frost"),
		}
		base["noiseDb"] = firstNonNil(
			parseNumber(specMap["eficiencia energetica.nivel de ruido db"]),
			extractNumber(toString(product["longDescription"]), `nivel de ruido\s+(\d+)\s*db`),
		)
		base["doorReversible"] = strings.Contains(strings.ToLower(toString(specMap["caracteristicas especificas.sentido de abertura da porta"])), "revers")
	case "washing_machine":
		base["capacityKg"] = parseNumber(specMap["caracteristicas especificas.capacidade"])
		base["spinRpm"] = parseNumber(specMap["caracteristicas especificas.centrifugacao"])
		base["noiseDb"] = map[string]any{
			"wash": parseNumber(specMap["eficiencia energetica.ruido lavagem db"]),
			"spin": parseNumber(specMap["eficiencia energetica.ruido centrifugacao db"]),
		}
		base["waterLitersPerCycle"] = parseNumber(specMap["eficiencia energetica.consumo agua l ciclo"])
		base["connectivity"] = specMap["extras.conectividade"]
		base["technologies"] = splitList(toString(specMap["caracteristicas especificas.tecnologia"]))
	case "dishwasher":
		base["capacitySets"] = parseNumber(specMap["caracteristicas especificas.capacidade"])
		base["installation"] = specMap["caracteristicas especificas.instalacao"]
		base["dryingType"] = specMap["caracteristicas especificas.tipo de secagem"]
		base["noiseDb"] = parseNumber(specMap["eficiencia energetica.nivel de ruido db"])
		base["waterLitersPerCycle"] = parseNumber(specMap["eficiencia energetica.consumo agua l ciclo"])
		base["thirdCutleryLevel"] = parseBoolean(specMap["extras.3 nivel para talheres"])
		base["adjustableBasket"] = parseBoolean(specMap["extras.cesto regulavel em altura"])
		base["connectivity"] = specMap["extras.conectividade"]
	case "hood":
		base["extractionMaxM3h"] = parseNumber(specMap["caracteristicas especificas.extracao max m3 h"])
		base["material"] = firstNonNil(specMap["caracteristicas fisicas.material"], specMap["caracteristicas especificas.material"])
		base["hob2Hood"] = strings.Contains(strings.ToLower(toString(product["name"])), "hob2hood") || strings.Contains(strings.ToLower(toString(specMap["mais informacoes.mais informacoes"])), "hob2hood")
		base["oldEnergyClass"] = specMap["caracteristicas especificas.eficiencia energetica"]
	case "oven":
		base["capacityLiters"] = firstNonNil(parseNumber(spec(specMap, "caracteristicas especificas.capacidade l", "capacidade.capacidade")), nil)
		base["cleaning"] = firstNonNil(specMap["caracteristicas especificas.auto limpeza"], matchFirst(toString(product["name"]), []string{"pirolit", "vapor", "steam"}))
		base["steam"] = strings.Contains(strings.ToLower(toString(product["name"])), "steam") || strings.Contains(strings.ToLower(toString(product["name"])), "vapor")
	case "hob":
		base["widthCm"] = parseNumber(specMap["caracteristicas fisicas.largura"])
		base["bridge"] = strings.Contains(strings.ToLower(toString(product["name"])), "bridge")
		base["boilAssist"] = strings.Contains(strings.ToLower(toString(product["name"])), "senseboil")
	}

	return base, nil
}

func createSpecMap(specsResponse map[string]any) map[string]any {
	result := map[string]any{}
	for _, sectionRaw := range getSlice(specsResponse, "sections") {
		section, ok := sectionRaw.(map[string]any)
		if !ok {
			continue
		}
		sectionKey := normalizePath(toString(section["title"]))
		for _, rowRaw := range getSlice(section, "rows") {
			row, ok := rowRaw.(map[string]any)
			if !ok {
				continue
			}
			result[sectionKey+"."+normalizePath(toString(row["subtitle"]))] = row["specs"]
		}
	}
	return result
}

func extractLifecycle(product map[string]any, specMap map[string]any) map[string]any {
	launchYear := firstNonZero(
		parseYear(spec(specMap, "mais informacoes.ano de lancamento", "mais informacoes.ano modelo", "referencias.ano modelo", "caracteristicas especificas.ano de lancamento")),
		parseYear(toString(product["longDescription"])),
	)
	warrantyYears := firstNonZero(
		parseWarrantyYears(spec(specMap, "mais informacoes.garantia", "mais informacoes.garantia motor", "caracteristicas especificas.garantia")),
		parseWarrantyYears(toString(product["longDescription"])),
	)
	lifecycleText := toString(spec(specMap, "mais informacoes.estado de gama", "mais informacoes.estado do produto", "caracteristicas especificas.estado do produto"))
	discontinued := regexp.MustCompile(`(?i)descontinuad|fim de linha|end of line`).MatchString(lifecycleText)
	if launchYear == 0 && warrantyYears == 0 && !discontinued {
		return nil
	}
	currentYear := time.Now().UTC().Year()
	productAgeYears := any(nil)
	status := "unknown"
	if discontinued {
		status = "end_of_line"
	} else if launchYear != 0 {
		age := currentYear - launchYear
		productAgeYears = age
		switch {
		case age <= 2:
			status = "current"
		case age <= 5:
			status = "mid_cycle"
		default:
			status = "aging"
		}
	}
	return map[string]any{
		"launchYear":      zeroToNil(launchYear),
		"productAgeYears": productAgeYears,
		"warrantyYears":   zeroToNil(warrantyYears),
		"discontinued":    discontinued,
		"status":          status,
	}
}

func extractEnergyMetrics(specMap map[string]any, category string) (any, any, []string) {
	raw := toString(spec(specMap,
		"eficiencia energetica.consumo de energia",
		"eficiencia energetica.consumo energia",
		"eficiencia energetica.consumo anual de energia",
		"eficiencia energetica.consumo de energia anual kwh ano",
		"eficiencia energetica.consumo anual kwh",
		"caracteristicas especificas.consumo de energia",
	))
	numeric := parseNumber(raw)
	if numeric == nil {
		return nil, nil, []string{}
	}
	normalized := normalizeKey(raw)
	categoryUsesCycleBasis := category == "washing_machine" || category == "dishwasher"
	looksCycleBased := strings.Contains(normalized, "100 ciclos") || strings.Contains(normalized, "100 cycles")
	looksYearBased := strings.Contains(normalized, "ano") || strings.Contains(normalized, "anual") || strings.Contains(normalized, "year")
	warnings := []string{}
	if categoryUsesCycleBasis || looksCycleBased {
		if categoryUsesCycleBasis && looksYearBased {
			warnings = append(warnings, "Retailer energy field looks year-based on a category that should usually use kWh per 100 cycles. Treat the value cautiously.")
		}
		if category == "washing_machine" {
			if value, ok := numeric.(float64); ok && value > 150 {
				warnings = append(warnings, "Energy figure looks unusually high for a modern 9 kg washing machine. Verify against the manufacturer sheet before trusting it.")
			}
		}
		return nil, numeric, warnings
	}
	return numeric, nil, warnings
}

func spec(specMap map[string]any, candidates ...string) any {
	for _, candidate := range candidates {
		normalized := normalizePath(candidate)
		if value, ok := specMap[normalized]; ok {
			return value
		}
	}
	for _, candidate := range candidates {
		normalized := normalizePath(candidate)
		for key, value := range specMap {
			if strings.HasSuffix(key, normalized) {
				return value
			}
		}
	}
	for _, candidate := range candidates {
		collapsedCandidate := normalizeKey(strings.ReplaceAll(candidate, ".", " "))
		for key, value := range specMap {
			if normalizeKey(strings.ReplaceAll(key, ".", " ")) == collapsedCandidate {
				return value
			}
		}
	}
	return nil
}

func normalizePath(value string) string {
	parts := strings.Split(value, ".")
	rows := make([]string, 0, len(parts))
	for _, part := range parts {
		part = normalizeKey(part)
		if part != "" {
			rows = append(rows, part)
		}
	}
	return strings.Join(rows, ".")
}

func normalizeKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer("á", "a", "à", "a", "ã", "a", "â", "a", "é", "e", "ê", "e", "í", "i", "ó", "o", "ô", "o", "õ", "o", "ú", "u", "ç", "c")
	value = replacer.Replace(value)
	var builder strings.Builder
	lastSpace := false
	for _, r := range value {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			builder.WriteRune(r)
			lastSpace = false
		default:
			if !lastSpace {
				builder.WriteRune(' ')
				lastSpace = true
			}
		}
	}
	return strings.TrimSpace(builder.String())
}

func inferBuyerCategory(product map[string]any, specMap map[string]any) string {
	haystack := normalizeKey(fmt.Sprintf("%v %v %v", product["name"], product["url"], mapsKeys(specMap)))
	switch {
	case strings.Contains(haystack, "frigorifico") || strings.Contains(haystack, "congelador"):
		return "fridge"
	case strings.Contains(haystack, "lavar roupa"):
		return "washing_machine"
	case strings.Contains(haystack, "lavar loica") || strings.Contains(haystack, "lava loica"):
		return "dishwasher"
	case strings.Contains(haystack, "exaustor"):
		return "hood"
	case regexp.MustCompile(`(^| )forno( |$)`).MatchString(haystack):
		return "oven"
	case strings.Contains(haystack, "placa"):
		return "hob"
	default:
		return "appliance"
	}
}

func mapsKeys(specMap map[string]any) string {
	keys := make([]string, 0, len(specMap))
	for key := range specMap {
		keys = append(keys, key)
	}
	return strings.Join(keys, " ")
}

func parseNumber(value any) any {
	if value == nil {
		return nil
	}
	raw := strings.TrimSpace(fmt.Sprintf("%v", value))
	normalized := raw
	if strings.Contains(raw, ",") && strings.Contains(raw, ".") {
		normalized = strings.ReplaceAll(strings.ReplaceAll(raw, ".", ""), ",", ".")
	} else if strings.Contains(raw, ",") {
		normalized = strings.ReplaceAll(raw, ",", ".")
	}
	match := regexp.MustCompile(`-?\d+(?:\.\d+)?`).FindString(normalized)
	if match == "" {
		return nil
	}
	number, err := strconv.ParseFloat(match, 64)
	if err != nil {
		return nil
	}
	return number
}

func parseYear(value any) int {
	match := regexp.MustCompile(`\b(20\d{2})\b`).FindStringSubmatch(fmt.Sprintf("%v", value))
	if len(match) < 2 {
		return 0
	}
	year, _ := strconv.Atoi(match[1])
	if year < 2000 || year > 2099 {
		return 0
	}
	return year
}

func parseWarrantyYears(value any) int {
	match := regexp.MustCompile(`(?i)(\d+)\s+anos?`).FindStringSubmatch(fmt.Sprintf("%v", value))
	if len(match) < 2 {
		return 0
	}
	years, _ := strconv.Atoi(match[1])
	return years
}

func parseBoolean(value any) any {
	text := strings.TrimSpace(strings.ToLower(fmt.Sprintf("%v", value)))
	switch text {
	case "sim", "yes", "true":
		return true
	case "nao", "não", "no", "false":
		return false
	default:
		return nil
	}
}

func parseDimensions(value any) map[string]any {
	raw := fmt.Sprintf("%v", value)
	parts := regexp.MustCompile(`\d+(?:[.,]\d+)?`).FindAllString(raw, -1)
	if len(parts) < 3 {
		return nil
	}
	width, _ := strconv.ParseFloat(strings.ReplaceAll(parts[0], ",", "."), 64)
	depth, _ := strconv.ParseFloat(strings.ReplaceAll(parts[1], ",", "."), 64)
	height, _ := strconv.ParseFloat(strings.ReplaceAll(parts[2], ",", "."), 64)
	return map[string]any{"width": width, "depth": depth, "height": height}
}

func splitList(value string) []string {
	if strings.TrimSpace(value) == "" {
		return []string{}
	}
	parts := strings.Split(value, ",")
	rows := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			rows = append(rows, part)
		}
	}
	return rows
}

func matchFirst(text string, patterns []string) any {
	for _, pattern := range patterns {
		if regexp.MustCompile(`(?i)` + pattern).MatchString(text) {
			return pattern
		}
	}
	return nil
}

func extractNumber(text, pattern string) any {
	normalized := normalizeKey(text)
	match := regexp.MustCompile(`(?i)` + pattern).FindStringSubmatch(normalized)
	if len(match) < 2 {
		return nil
	}
	value, err := strconv.ParseFloat(strings.ReplaceAll(match[1], ",", "."), 64)
	if err != nil {
		return nil
	}
	return value
}

func extractSectionTitles(specsResponse map[string]any) []string {
	sections := getSlice(specsResponse, "sections")
	rows := make([]string, 0, len(sections))
	for _, sectionRaw := range sections {
		section, ok := sectionRaw.(map[string]any)
		if !ok {
			continue
		}
		rows = append(rows, toString(section["title"]))
	}
	return rows
}

func detectLisbonArea(values []string) any {
	text := normalizeKey(strings.Join(values, " "))
	if text == "" {
		return nil
	}
	if regexp.MustCompile(`\b(lisboa|lisbon|oeiras|cascais|amadora|odivelas|almada|sintra|loures)\b`).MatchString(text) {
		return true
	}
	return false
}

func maybeLocation(latitude, longitude any) any {
	if latitude == nil || longitude == nil {
		return nil
	}
	return map[string]any{
		"latitude":  toFloat(latitude),
		"longitude": toFloat(longitude),
	}
}

func hasStockStatus(status string) any {
	switch status {
	case "available", "low":
		return true
	case "none", "unavailable":
		return false
	default:
		return nil
	}
}

func normalizeCategoryIDs(values []any) []any {
	rows := make([]any, 0, len(values))
	for _, value := range values {
		if category, ok := value.(map[string]any); ok {
			rows = append(rows, category["id"])
		} else {
			rows = append(rows, value)
		}
	}
	return rows
}

func prefixURL(path string) any {
	if path == "" {
		return nil
	}
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	return baseURL + path
}

func prefixImageURL(transform string) any {
	if transform == "" {
		return nil
	}
	return baseURL + "/i/" + transform
}

func defaultSlice(value any) []any {
	if rows, ok := value.([]any); ok {
		return rows
	}
	return []any{}
}

func defaultStringSlice(value any) []string {
	rows, ok := value.([]any)
	if !ok {
		return []string{}
	}
	out := make([]string, 0, len(rows))
	for _, item := range rows {
		if text := toString(item); text != "" {
			out = append(out, text)
		}
	}
	return out
}

func roundNumber(value float64, places int) any {
	if value == 0 {
		return nil
	}
	format := "%." + strconv.Itoa(places) + "f"
	number, err := strconv.ParseFloat(fmt.Sprintf(format, value), 64)
	if err != nil {
		return value
	}
	return number
}

func zeroToNil(value int) any {
	if value == 0 {
		return nil
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func firstNonNil(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func firstNonZero(values ...int) int {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func sliceLen(value any) any {
	if rows, ok := value.([]any); ok {
		return len(rows)
	}
	return nil
}

func toString(value any) string {
	if value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return fmt.Sprintf("%v", typed)
	}
}

func toFloat(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case int:
		return float64(typed)
	case string:
		number, _ := strconv.ParseFloat(strings.ReplaceAll(typed, ",", "."), 64)
		return number
	default:
		return 0
	}
}

func getMap(value any, path ...string) map[string]any {
	current := value
	for _, key := range path {
		next, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = next[key]
	}
	result, _ := current.(map[string]any)
	return result
}

func valueAt(value any, path ...string) any {
	current := value
	for _, key := range path {
		next, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = next[key]
	}
	return current
}

func getSlice(value any, path ...string) []any {
	current := value
	for _, key := range path {
		next, ok := current.(map[string]any)
		if !ok {
			return []any{}
		}
		current = next[key]
	}
	result, _ := current.([]any)
	return result
}

func getSliceMap(value any, path ...string) []map[string]any {
	rows := getSlice(value, path...)
	result := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		if item, ok := row.(map[string]any); ok {
			result = append(result, item)
		}
	}
	return result
}

func firstMap(values []map[string]any) map[string]any {
	if len(values) == 0 {
		return nil
	}
	return values[0]
}

func deepGet(value any, path ...any) any {
	current := value
	for _, part := range path {
		switch index := part.(type) {
		case string:
			next, ok := current.(map[string]any)
			if !ok {
				return nil
			}
			current = next[index]
		case int:
			next, ok := current.([]any)
			if !ok || index < 0 || index >= len(next) {
				return nil
			}
			current = next[index]
		default:
			return nil
		}
	}
	return current
}

func sliceMaps(values []any) []map[string]any {
	rows := make([]map[string]any, 0, len(values))
	for _, value := range values {
		if row, ok := value.(map[string]any); ok {
			rows = append(rows, row)
		}
	}
	return rows
}

func findOffer(response map[string]any, offerID string) map[string]any {
	for _, offer := range getSliceMap(response, "offersData", "offers") {
		if toString(offer["offer_id"]) == offerID {
			return offer
		}
	}
	return nil
}

func findSeller(response map[string]any, sellerID string) map[string]any {
	for _, sellerRaw := range getSlice(response, "sellersData") {
		seller, ok := sellerRaw.(map[string]any)
		if !ok {
			continue
		}
		if toString(seller["seller_id"]) == sellerID {
			return seller
		}
	}
	return nil
}
