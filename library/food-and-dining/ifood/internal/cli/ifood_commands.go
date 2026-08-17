package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/ifood/internal/client"
	"github.com/spf13/cobra"
)

const (
	defaultChannel = "IFOOD"
	marketHomePath = "/v2/bm/home"
)

var defaultQuoteItems = []string{
	"alcool gel",
	"guardanapo",
	"papel toalha",
	"garrafa agua",
	"esponja",
	"filtro de linha",
}

type marketSummary struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	Slug           string  `json:"slug,omitempty"`
	Rating         float64 `json:"rating"`
	RatingCount    int     `json:"rating_count,omitempty"`
	DistanceKM     float64 `json:"distance_km,omitempty"`
	Available      bool    `json:"available"`
	DeliveryFeeBRL float64 `json:"delivery_fee_brl,omitempty"`
}

type productSummary struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
	EAN         string        `json:"ean,omitempty"`
	Price       float64       `json:"price"`
	Currency    string        `json:"currency"`
	Available   bool          `json:"available"`
	Merchant    marketSummary `json:"merchant"`
}

type quoteLine struct {
	Term    string          `json:"term"`
	Product *productSummary `json:"product,omitempty"`
	Status  string          `json:"status"`
}

type storeQuote struct {
	Merchant marketSummary `json:"merchant"`
	Items    []quoteLine   `json:"items"`
	Subtotal float64       `json:"subtotal"`
	Complete bool          `json:"complete"`
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newAddressesCmd(flags))
		addNovelCommandIfAbsent(root, newMarketsCmd(flags))
		addNovelCommandIfAbsent(root, newProductsCmd(flags))
		addNovelCommandIfAbsent(root, newQuoteCmd(flags))
		addNovelCommandIfAbsent(root, newCartCmd(flags))
		addNovelCommandIfAbsent(root, newSessionCmd(flags))
		addNovelCommandIfAbsent(root, newBrowserCmd(flags))
	})
	registerClientHook(loadIfoodHeadersFromEnv)
}

type cartItemInput struct {
	ID          string        `json:"id"`
	Quantity    int           `json:"quantity"`
	Observation string        `json:"observation,omitempty"`
	SubItems    []cartSubItem `json:"subItems,omitempty"`
}

type cartSubItem struct {
	ID       string `json:"id"`
	Quantity int    `json:"quantity"`
}

type cartBuildSelection struct {
	Term     string          `json:"term"`
	Quantity int             `json:"quantity"`
	Product  *productSummary `json:"product,omitempty"`
	Status   string          `json:"status"`
}

type quantityTerm struct {
	Term     string
	Quantity int
}

type addressSummary struct {
	ID           string          `json:"id"`
	Favorite     bool            `json:"favorite"`
	Active       bool            `json:"active"`
	City         string          `json:"city,omitempty"`
	Neighborhood string          `json:"neighborhood,omitempty"`
	Coordinates  cartCoordinates `json:"coordinates"`
}

type cartCoordinates struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type cartDeliveryInput struct {
	ID                 string `json:"id"`
	Mode               string `json:"mode,omitempty"`
	Now                bool   `json:"now"`
	DeliveredBy        string `json:"deliveryBy,omitempty"`
	SelectedTimeSlotID string `json:"selectedTimeSlotId,omitempty"`
}

type cartCreatePayload struct {
	Items    []cartItemInput `json:"items"`
	Merchant struct {
		ID      string `json:"id"`
		Context string `json:"context"`
	} `json:"merchant"`
	Address struct {
		ID          string          `json:"id"`
		Coordinates cartCoordinates `json:"coordinates"`
	} `json:"address"`
	Delivery cartDeliveryInput `json:"delivery"`
}

func newAddressesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "addresses", Short: "Discover saved delivery addresses"}
	var activeOnly bool
	list := &cobra.Command{
		Use:         "list",
		Short:       "List saved address IDs and coordinates",
		Example:     "  ifood-pp-cli addresses list --dry-run --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if dryRunOK(flags) {
				return printNovelJSON(cmd, flags, map[string]any{"method": "GET", "path": "/v1/customers/me/addresses", "executed": false})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			addresses, err := fetchAddresses(cmd, c)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			if activeOnly {
				filtered := addresses[:0]
				for _, address := range addresses {
					if address.Active {
						filtered = append(filtered, address)
					}
				}
				addresses = filtered
			}
			return printNovelJSON(cmd, flags, addresses)
		},
	}
	list.Flags().BoolVar(&activeOnly, "active-only", false, "Return only active addresses")
	cmd.AddCommand(list)
	return cmd
}

func fetchAddresses(cmd *cobra.Command, c *client.Client) ([]addressSummary, error) {
	raw, err := c.Get(cmd.Context(), "/v1/customers/me/addresses", nil)
	if err != nil {
		return nil, err
	}
	var addresses []addressSummary
	if err := json.Unmarshal(raw, &addresses); err != nil {
		return nil, fmt.Errorf("decode addresses: %w", err)
	}
	sortAddressesForSelection(addresses)
	return addresses, nil
}

func sortAddressesForSelection(addresses []addressSummary) {
	sort.SliceStable(addresses, func(i, j int) bool {
		if addresses[i].Active != addresses[j].Active {
			return addresses[i].Active
		}
		if addresses[i].Favorite != addresses[j].Favorite {
			return addresses[i].Favorite
		}
		return addresses[i].ID < addresses[j].ID
	})
}

func newCartCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cart",
		Short: "Inspect and update an existing iFood cart",
		Long:  "Inspect and update an existing iFood cart through the private web API. Cart writes default to preview mode.",
	}

	var showVersion int
	show := &cobra.Command{
		Use:         "show <cart-id>",
		Short:       "Show the current cart",
		Example:     "  ifood-pp-cli cart show CART_ID --dry-run --json",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := cartAPIPath(showVersion, args[0], "")
			if err != nil {
				return usageErr(err)
			}
			if dryRunOK(flags) {
				return printNovelJSON(cmd, flags, map[string]any{"method": "GET", "path": path, "executed": false})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			raw, err := c.Get(cmd.Context(), path, nil)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			return printOutput(cmd.OutOrStdout(), raw, flags.asJSON || !isTerminal(cmd.OutOrStdout()))
		},
	}
	show.Flags().IntVar(&showVersion, "api-version", 1, "Cart API version (1 or 2)")

	var addVersion int
	var itemSpecs []string
	var observation string
	var execute bool
	add := &cobra.Command{
		Use:     "add <cart-id>",
		Short:   "Preview or add products to an existing cart",
		Example: "  ifood-pp-cli cart add CART_ID --item PRODUCT_ID --dry-run --json",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := cartAPIPath(addVersion, args[0], "items")
			if err != nil {
				return usageErr(err)
			}
			items, err := parseCartItems(itemSpecs, observation)
			if err != nil {
				return usageErr(err)
			}
			preview := map[string]any{"method": "POST", "path": path, "body": items, "executed": false}
			if flags.dryRun || !execute {
				return printNovelJSON(cmd, flags, preview)
			}
			if !flags.yes {
				return usageErr(errors.New("cart update requires both --execute and --yes"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			raw, status, err := c.Post(cmd.Context(), path, items)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			var response any
			if len(raw) > 0 && json.Unmarshal(raw, &response) != nil {
				response = string(raw)
			}
			return printNovelJSON(cmd, flags, map[string]any{"executed": true, "status": status, "response": response})
		},
	}
	add.Flags().IntVar(&addVersion, "api-version", 1, "Cart API version (1 or 2)")
	add.Flags().StringSliceVar(&itemSpecs, "item", nil, "Product ID with optional quantity, ID[:QTY] (repeatable)")
	add.Flags().StringVar(&observation, "observation", "", "Observation applied to every item")
	add.Flags().BoolVar(&execute, "execute", false, "Send the cart update; also requires --yes")

	var buildVersion int
	var buildMerchant string
	var buildLatitude, buildLongitude float64
	var buildItemSpecs []string
	var buildObservation string
	var buildSearchLimit int
	var buildExample bool
	build := &cobra.Command{
		Use:     "build <cart-id>",
		Short:   "Resolve product terms and preview a cart update",
		Example: "  ifood-pp-cli cart build EXAMPLE_CART --example --json",
		Args:    cobra.ExactArgs(1),
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "<cart-id>=EXAMPLE_CART",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := cartAPIPath(buildVersion, args[0], "items")
			if err != nil {
				return usageErr(err)
			}
			if buildExample {
				observation, exampleErr := loadBrowserExampleQuote()
				if exampleErr != nil {
					return exampleErr
				}
				validation, exampleErr := validateBrowserQuote(observation, 4.5, 3)
				if exampleErr != nil {
					return exampleErr
				}
				plan := buildBrowserCartPlan(validation, "")
				plan["example"] = true
				plan["request"] = map[string]any{"method": "POST", "path": path, "body": plan["items"]}
				plan["executed"] = false
				return printNovelJSON(cmd, flags, plan)
			}
			if strings.TrimSpace(buildMerchant) == "" || buildLatitude == 0 || buildLongitude == 0 {
				return usageErr(errors.New("--merchant, --latitude, and --longitude are required"))
			}
			if len(buildItemSpecs) == 0 {
				buildItemSpecs = append([]string(nil), defaultQuoteItems...)
			}
			terms, err := parseQuantityTerms(buildItemSpecs)
			if err != nil {
				return usageErr(err)
			}
			if flags.dryRun {
				return printNovelJSON(cmd, flags, map[string]any{
					"merchant_id": buildMerchant,
					"terms":       termsForJSON(terms),
					"request":     map[string]any{"method": "POST", "path": path, "body": nil},
					"resolved":    false,
					"executed":    false,
				})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			search := func(term string) ([]productSummary, error) {
				return searchProducts(cmd, c, buildMerchant, buildLatitude, buildLongitude, term, buildSearchLimit, false)
			}
			selections, items, subtotal, complete, err := resolveCartBuild(terms, buildObservation, search)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			result := map[string]any{
				"merchant_id": buildMerchant,
				"selections":  selections,
				"subtotal":    subtotal,
				"complete":    complete,
				"request":     map[string]any{"method": "POST", "path": path, "body": items},
				"executed":    false,
			}
			return printNovelJSON(cmd, flags, result)
		},
	}
	build.Flags().IntVar(&buildVersion, "api-version", 1, "Cart API version (1 or 2)")
	build.Flags().StringVar(&buildMerchant, "merchant", "", "Merchant UUID used for product searches")
	build.Flags().Float64Var(&buildLatitude, "latitude", 0, "Delivery latitude")
	build.Flags().Float64Var(&buildLongitude, "longitude", 0, "Delivery longitude")
	build.Flags().StringSliceVar(&buildItemSpecs, "item", nil, "Search term with optional quantity, TERM[:QTY] (repeatable; defaults to the original list)")
	build.Flags().StringVar(&buildObservation, "observation", "", "Observation applied to every resolved item")
	build.Flags().IntVar(&buildSearchLimit, "search-limit", 20, "Candidates fetched per search term")
	build.Flags().BoolVar(&buildExample, "example", false, "Build a preview from the embedded credential-free three-market example")

	var createVersion int
	var createMerchant, createAddressID string
	var createLatitude, createLongitude float64
	var createDeliveryID, createDeliveryMode, createDeliveredBy, createTimeSlotID string
	var createItems []string
	var createObservation string
	var createExecute bool
	var createSearchLimit int
	var createNow bool
	create := &cobra.Command{
		Use:     "create",
		Short:   "Resolve products and preview or create a new cart",
		Example: "  ifood-pp-cli cart create --merchant MERCHANT_ID --item esponja --dry-run --json",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(createMerchant) == "" {
				return usageErr(errors.New("--merchant is required"))
			}
			if (createLatitude == 0) != (createLongitude == 0) {
				return usageErr(errors.New("--latitude and --longitude must be provided together"))
			}
			if len(createItems) == 0 {
				createItems = append([]string(nil), defaultQuoteItems...)
			}
			terms, err := parseQuantityTerms(createItems)
			if err != nil {
				return usageErr(err)
			}
			path, err := cartCollectionPath(createVersion)
			if err != nil {
				return usageErr(err)
			}
			if flags.dryRun {
				return printNovelJSON(cmd, flags, map[string]any{
					"merchant_id": createMerchant,
					"address_id":  createAddressID,
					"terms":       termsForJSON(terms),
					"request":     map[string]any{"method": "POST", "path": path, "body": nil},
					"resolved":    false,
					"executed":    false,
				})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			addresses, err := fetchAddresses(cmd, c)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			address, err := selectAddress(addresses, createAddressID)
			if err != nil {
				return usageErr(err)
			}
			coordinates := address.Coordinates
			if createLatitude != 0 && createLongitude != 0 {
				coordinates = cartCoordinates{Latitude: createLatitude, Longitude: createLongitude}
			}
			if coordinates.Latitude == 0 || coordinates.Longitude == 0 {
				return usageErr(errors.New("selected address has no coordinates; pass --latitude and --longitude"))
			}
			search := func(term string) ([]productSummary, error) {
				return searchProducts(cmd, c, createMerchant, coordinates.Latitude, coordinates.Longitude, term, createSearchLimit, false)
			}
			selections, items, subtotal, productsComplete, err := resolveCartBuild(terms, createObservation, search)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			delivery := cartDeliveryInput{ID: strings.TrimSpace(createDeliveryID), Mode: strings.TrimSpace(createDeliveryMode), Now: createNow, DeliveredBy: strings.TrimSpace(createDeliveredBy), SelectedTimeSlotID: strings.TrimSpace(createTimeSlotID)}
			if delivery.ID == "" {
				catalog, catalogErr := fetchMerchantCatalog(cmd, c, createMerchant, coordinates)
				if catalogErr != nil {
					return classifyAPIError(cmd.OutOrStdout(), catalogErr, flags)
				}
				delivery, err = selectDeliveryMethod(catalog)
				if err != nil {
					return usageErr(fmt.Errorf("automatic delivery selection failed: %w; pass --delivery-id", err))
				}
			}
			payload := buildCartCreatePayload(createMerchant, address.ID, coordinates, delivery, items)
			complete := productsComplete && len(items) > 0 && deliveryReady(delivery)
			result := map[string]any{
				"merchant_id": createMerchant,
				"address":     address,
				"delivery":    delivery,
				"selections":  selections,
				"subtotal":    subtotal,
				"complete":    complete,
				"request":     map[string]any{"method": "POST", "path": path, "body": payload},
				"executed":    false,
			}
			if !createExecute {
				return printNovelJSON(cmd, flags, result)
			}
			if !complete {
				return usageErr(errors.New("cart create is incomplete; every term, delivery method, and scheduled time slot must resolve before --execute"))
			}
			if !flags.yes {
				return usageErr(errors.New("cart creation requires both --execute and --yes"))
			}
			raw, status, err := c.Post(cmd.Context(), path, payload)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			var response any
			if len(raw) > 0 && json.Unmarshal(raw, &response) != nil {
				response = string(raw)
			}
			result["executed"] = true
			result["status"] = status
			result["response"] = response
			if cartID := findCartID(response); cartID != "" {
				result["cart_id"] = cartID
				result["next_command"] = fmt.Sprintf("ifood-pp-cli --json cart show %s", cartID)
			}
			return printNovelJSON(cmd, flags, result)
		},
	}
	create.Flags().IntVar(&createVersion, "api-version", 1, "Cart API version (1 or 2)")
	create.Flags().StringVar(&createMerchant, "merchant", "", "Merchant UUID")
	create.Flags().StringVar(&createAddressID, "address-id", "", "Saved address ID; defaults to active/favorite")
	create.Flags().Float64Var(&createLatitude, "latitude", 0, "Override delivery latitude")
	create.Flags().Float64Var(&createLongitude, "longitude", 0, "Override delivery longitude")
	create.Flags().StringVar(&createDeliveryID, "delivery-id", "", "Delivery method ID; auto-detected when omitted")
	create.Flags().StringVar(&createDeliveryMode, "delivery-mode", "", "Delivery mode override")
	create.Flags().StringVar(&createDeliveredBy, "delivered-by", "", "Delivery provider override")
	create.Flags().StringVar(&createTimeSlotID, "time-slot-id", "", "Scheduled delivery time-slot ID")
	create.Flags().BoolVar(&createNow, "now", true, "Request the next available delivery window")
	create.Flags().StringSliceVar(&createItems, "item", nil, "Search term with optional quantity, TERM[:QTY] (repeatable; defaults to the original list)")
	create.Flags().StringVar(&createObservation, "observation", "", "Observation applied to every resolved item")
	create.Flags().IntVar(&createSearchLimit, "search-limit", 20, "Candidates fetched per search term")
	create.Flags().BoolVar(&createExecute, "execute", false, "Create the cart; also requires --yes")

	cmd.AddCommand(show, add, build, create)
	return cmd
}

func cartAPIPath(version int, cartID, suffix string) (string, error) {
	base, err := cartCollectionPath(version)
	if err != nil {
		return "", err
	}
	cartID = strings.TrimSpace(cartID)
	if cartID == "" || strings.ContainsAny(cartID, "/?#") {
		return "", errors.New("cart ID must be a non-empty path segment")
	}
	path := base + "/" + cartID
	if suffix != "" {
		path += "/" + suffix
	}
	return path, nil
}

func cartCollectionPath(version int) (string, error) {
	if version != 1 && version != 2 {
		return "", errors.New("--api-version must be 1 or 2")
	}
	return fmt.Sprintf("/v%d/carts", version), nil
}

func selectAddress(addresses []addressSummary, requestedID string) (addressSummary, error) {
	requestedID = strings.TrimSpace(requestedID)
	if requestedID != "" {
		for _, address := range addresses {
			if address.ID == requestedID {
				return address, nil
			}
		}
		return addressSummary{}, fmt.Errorf("address %q was not found", requestedID)
	}
	if len(addresses) == 0 {
		return addressSummary{}, errors.New("no saved delivery addresses found")
	}
	return addresses[0], nil
}

func fetchMerchantCatalog(cmd *cobra.Command, c *client.Client, merchantID string, coordinates cartCoordinates) (any, error) {
	raw, err := c.Get(cmd.Context(), "/v1/merchants/multicategory/"+merchantID+"/catalog", map[string]string{
		"latitude":  strconv.FormatFloat(coordinates.Latitude, 'f', -1, 64),
		"longitude": strconv.FormatFloat(coordinates.Longitude, 'f', -1, 64),
	})
	if err != nil {
		return nil, err
	}
	var catalog any
	if err := json.Unmarshal(raw, &catalog); err != nil {
		return nil, fmt.Errorf("decode merchant catalog: %w", err)
	}
	return catalog, nil
}

func selectDeliveryMethod(catalog any) (cartDeliveryInput, error) {
	methods := collectDeliveryMethods(catalog)
	for _, method := range methods {
		if available, ok := method["available"].(bool); ok && !available {
			continue
		}
		id := firstString(method, "id", "code")
		if id == "" {
			continue
		}
		now := true
		if schedule, ok := method["schedule"].(map[string]any); ok {
			if value, ok := schedule["now"].(bool); ok {
				now = value
			}
			if selected, ok := schedule["selectedTimeSlot"].(map[string]any); ok {
				method["selectedTimeSlotId"] = firstString(selected, "id")
			}
		}
		return cartDeliveryInput{
			ID:                 id,
			Mode:               firstString(method, "mode", "type"),
			Now:                now,
			DeliveredBy:        firstString(method, "deliveredBy", "delivered_by"),
			SelectedTimeSlotID: firstString(method, "selectedTimeSlotId"),
		}, nil
	}
	return cartDeliveryInput{}, errors.New("no available delivery method found in merchant catalog")
}

func deliveryReady(delivery cartDeliveryInput) bool {
	return delivery.ID != "" && (delivery.Now || delivery.SelectedTimeSlotID != "")
}

func collectDeliveryMethods(value any) []map[string]any {
	methods := []map[string]any{}
	var walk func(any)
	walk = func(current any) {
		switch typed := current.(type) {
		case []any:
			for _, child := range typed {
				walk(child)
			}
		case map[string]any:
			for key, child := range typed {
				if key == "deliveryMethods" || key == "delivery_methods" {
					if list, ok := child.([]any); ok {
						for _, item := range list {
							if method, ok := item.(map[string]any); ok {
								methods = append(methods, method)
							}
						}
					}
					continue
				}
				walk(child)
			}
		}
	}
	walk(value)
	return methods
}

func firstString(value map[string]any, keys ...string) string {
	for _, key := range keys {
		if text, ok := value[key].(string); ok && strings.TrimSpace(text) != "" {
			return text
		}
	}
	return ""
}

func buildCartCreatePayload(merchantID, addressID string, coordinates cartCoordinates, delivery cartDeliveryInput, items []cartItemInput) cartCreatePayload {
	payload := cartCreatePayload{Items: items, Delivery: delivery}
	payload.Merchant.ID = merchantID
	payload.Merchant.Context = "DEFAULT"
	payload.Address.ID = addressID
	payload.Address.Coordinates = coordinates
	return payload
}

func findCartID(value any) string {
	if object, ok := value.(map[string]any); ok {
		if cartResponse, ok := object["cartResponse"].(map[string]any); ok {
			if id := firstString(cartResponse, "id"); id != "" {
				return id
			}
		}
		if id := firstString(object, "cartId", "cart_id", "id"); id != "" {
			return id
		}
		for _, key := range []string{"data", "cart", "response"} {
			if id := findCartID(object[key]); id != "" {
				return id
			}
		}
	}
	return ""
}

func parseCartItems(specs []string, observation string) ([]cartItemInput, error) {
	if len(specs) == 0 {
		return nil, errors.New("at least one --item ID[:QTY] is required")
	}
	items := make([]cartItemInput, 0, len(specs))
	for _, spec := range specs {
		parts := strings.Split(strings.TrimSpace(spec), ":")
		if len(parts) > 2 || strings.TrimSpace(parts[0]) == "" {
			return nil, fmt.Errorf("invalid --item %q; expected ID[:QTY]", spec)
		}
		quantity := 1
		if len(parts) == 2 {
			parsed, err := strconv.Atoi(parts[1])
			if err != nil || parsed < 1 {
				return nil, fmt.Errorf("invalid quantity in --item %q", spec)
			}
			quantity = parsed
		}
		items = append(items, cartItemInput{ID: strings.TrimSpace(parts[0]), Quantity: quantity, Observation: observation})
	}
	return items, nil
}

func parseQuantityTerms(specs []string) ([]quantityTerm, error) {
	terms := make([]quantityTerm, 0, len(specs))
	for _, spec := range specs {
		trimmed := strings.TrimSpace(spec)
		if trimmed == "" {
			return nil, errors.New("item search terms cannot be empty")
		}
		term, quantity := trimmed, 1
		if colon := strings.LastIndex(trimmed, ":"); colon >= 0 {
			candidate := strings.TrimSpace(trimmed[colon+1:])
			if parsed, err := strconv.Atoi(candidate); err == nil {
				if parsed < 1 {
					return nil, fmt.Errorf("invalid quantity in --item %q", spec)
				}
				term = strings.TrimSpace(trimmed[:colon])
				quantity = parsed
			}
		}
		if term == "" {
			return nil, fmt.Errorf("invalid --item %q; expected TERM[:QTY]", spec)
		}
		terms = append(terms, quantityTerm{Term: term, Quantity: quantity})
	}
	return terms, nil
}

func termsForJSON(terms []quantityTerm) []map[string]any {
	result := make([]map[string]any, 0, len(terms))
	for _, term := range terms {
		result = append(result, map[string]any{"term": term.Term, "quantity": term.Quantity})
	}
	return result
}

func resolveCartBuild(terms []quantityTerm, observation string, search func(string) ([]productSummary, error)) ([]cartBuildSelection, []cartItemInput, float64, bool, error) {
	selections := make([]cartBuildSelection, 0, len(terms))
	itemsByID := map[string]cartItemInput{}
	itemOrder := make([]string, 0, len(terms))
	subtotal := 0.0
	complete := true
	for _, term := range terms {
		products, err := search(term.Term)
		if err != nil {
			return nil, nil, 0, false, fmt.Errorf("search %q: %w", term.Term, err)
		}
		selection := cartBuildSelection{Term: term.Term, Quantity: term.Quantity, Status: "not_found"}
		if len(products) == 0 {
			complete = false
			selections = append(selections, selection)
			continue
		}
		product := products[0]
		selection.Product = &product
		selection.Status = "found"
		selections = append(selections, selection)
		subtotal += product.Price * float64(term.Quantity)
		if existing, ok := itemsByID[product.ID]; ok {
			existing.Quantity += term.Quantity
			itemsByID[product.ID] = existing
		} else {
			itemsByID[product.ID] = cartItemInput{ID: product.ID, Quantity: term.Quantity, Observation: observation}
			itemOrder = append(itemOrder, product.ID)
		}
	}
	items := make([]cartItemInput, 0, len(itemOrder))
	for _, id := range itemOrder {
		items = append(items, itemsByID[id])
	}
	return selections, items, subtotal, complete, nil
}

func loadIfoodHeadersFromEnv(c *client.Client) error {
	if c == nil || c.Config == nil {
		return nil
	}
	path := strings.TrimSpace(os.Getenv("IFOOD_HEADERS_FILE"))
	if path == "" {
		return nil
	}
	// #nosec G304,G703 -- IFOOD_HEADERS_FILE is an explicit operator-selected credential source, not remote input.
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("read IFOOD_HEADERS_FILE: %w", err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("IFOOD_HEADERS_FILE must be private (chmod 600 %s)", path)
	}
	// #nosec G304,G703 -- the same operator-selected path is permission-checked above before reading.
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read IFOOD_HEADERS_FILE: %w", err)
	}
	var headers map[string]string
	if err := json.Unmarshal(data, &headers); err != nil {
		return fmt.Errorf("parse IFOOD_HEADERS_FILE: %w", err)
	}
	if c.Config.Headers == nil {
		c.Config.Headers = map[string]string{}
	}
	for key, value := range headers {
		if strings.TrimSpace(key) != "" && strings.TrimSpace(value) != "" {
			c.Config.Headers[key] = value
		}
	}
	return nil
}

func newSessionCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "session", Short: "Inspect iFood web-session configuration"}
	cmd.AddCommand(&cobra.Command{
		Use:         "status",
		Short:       "Report credential/header presence without revealing values",
		Example:     "  ifood-pp-cli session status --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			path := strings.TrimSpace(os.Getenv("IFOOD_HEADERS_FILE"))
			report := map[string]any{
				"bearer_configured":             os.Getenv("IFOOD_BEARER_AUTH") != "",
				"headers_file_configured":       path != "",
				"headers_file_readable":         false,
				"authorization_in_headers_file": false,
				"ready":                         false,
			}
			if path != "" {
				// #nosec G304,G703 -- status inspects only the operator-selected IFOOD_HEADERS_FILE path.
				if info, err := os.Stat(path); err == nil {
					report["headers_file_readable"] = true
					report["headers_file_private"] = info.Mode().Perm()&0o077 == 0
					// #nosec G304,G703 -- this read reports header-name presence only and never emits values.
					if data, readErr := os.ReadFile(path); readErr == nil {
						var headers map[string]string
						if json.Unmarshal(data, &headers) == nil {
							report["authorization_in_headers_file"] = hasHeader(headers, "authorization")
						}
					}
				}
			}
			authReady := report["bearer_configured"].(bool) || report["authorization_in_headers_file"].(bool)
			private, _ := report["headers_file_private"].(bool)
			report["ready"] = authReady && report["headers_file_readable"].(bool) && private
			return flags.printJSON(cmd, report)
		},
	})

	var curlInput, curlOutput string
	importCurl := &cobra.Command{
		Use:         "import-curl",
		Short:       "Import private session headers from a DevTools Copy-as-cURL file",
		Example:     "  ifood-pp-cli session import-curl --input ./ifood-request.curl --out ./private-headers.json --dry-run --json",
		Annotations: map[string]string{"mcp:local-write": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(curlInput) == "" {
				return usageErr(errors.New("--input is required"))
			}
			output, err := resolveHeadersOutputPath(curlOutput)
			if err != nil {
				return usageErr(err)
			}
			if flags.dryRun {
				return printNovelJSON(cmd, flags, map[string]any{"dry_run": true, "input": curlInput, "output": output, "credential_values_read": false, "installed": false})
			}
			// #nosec G304,G703 -- --input explicitly selects the local DevTools export to import.
			data, err := os.ReadFile(curlInput)
			if err != nil {
				return fmt.Errorf("read curl input: %w", err)
			}
			headers, err := parseCurlHeaders(string(data))
			if err != nil {
				return usageErr(err)
			}
			return importSessionHeaders(cmd, flags, output, headers)
		},
	}
	importCurl.Flags().StringVar(&curlInput, "input", "", "File containing a DevTools Copy-as-cURL command")
	importCurl.Flags().StringVar(&curlOutput, "out", "", "Private JSON headers file (or set IFOOD_HEADERS_FILE)")

	var jsonInput, jsonOutput string
	importHeaders := &cobra.Command{
		Use:         "import-headers",
		Short:       "Validate and install a private JSON session-headers file",
		Example:     "  ifood-pp-cli session import-headers --input ./private-headers.json --out ./installed-headers.json --dry-run --json",
		Annotations: map[string]string{"mcp:local-write": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(jsonInput) == "" {
				return usageErr(errors.New("--input is required"))
			}
			output, err := resolveHeadersOutputPath(jsonOutput)
			if err != nil {
				return usageErr(err)
			}
			if flags.dryRun {
				return printNovelJSON(cmd, flags, map[string]any{"dry_run": true, "input": jsonInput, "output": output, "credential_values_read": false, "installed": false})
			}
			// #nosec G304,G703 -- --input explicitly selects the local header JSON to import.
			data, err := os.ReadFile(jsonInput)
			if err != nil {
				return fmt.Errorf("read headers input: %w", err)
			}
			var headers map[string]string
			if err := json.Unmarshal(data, &headers); err != nil {
				return usageErr(fmt.Errorf("parse headers JSON: %w", err))
			}
			return importSessionHeaders(cmd, flags, output, headers)
		},
	}
	importHeaders.Flags().StringVar(&jsonInput, "input", "", "JSON object containing request headers")
	importHeaders.Flags().StringVar(&jsonOutput, "out", "", "Private JSON headers file (or set IFOOD_HEADERS_FILE)")
	cmd.AddCommand(importCurl, importHeaders)
	return cmd
}

var curlHeaderPattern = regexp.MustCompile(`(?:^|\s)(?:-H|--header)\s+('(?:[^']*)'|"(?:\\.|[^"])*"|[^\s\\]+)`)

func parseCurlHeaders(command string) (map[string]string, error) {
	command = strings.ReplaceAll(command, "\\\r\n", " ")
	command = strings.ReplaceAll(command, "\\\n", " ")
	headers := map[string]string{}
	for _, match := range curlHeaderPattern.FindAllStringSubmatch(command, -1) {
		value := match[1]
		if strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'") {
			value = strings.TrimSuffix(strings.TrimPrefix(value, "'"), "'")
		} else if strings.HasPrefix(value, `"`) {
			unquoted, err := strconv.Unquote(value)
			if err != nil {
				return nil, fmt.Errorf("parse quoted curl header: %w", err)
			}
			value = unquoted
		}
		name, headerValue, ok := strings.Cut(value, ":")
		if !ok || strings.TrimSpace(name) == "" {
			return nil, fmt.Errorf("invalid curl header %q", value)
		}
		headers[strings.TrimSpace(name)] = strings.TrimSpace(headerValue)
	}
	if len(headers) == 0 {
		return nil, errors.New("no -H/--header values found in curl input")
	}
	if !hasHeader(headers, "authorization") {
		return nil, errors.New("curl input does not contain an Authorization header")
	}
	return headers, nil
}

func hasHeader(headers map[string]string, wanted string) bool {
	for name, value := range headers {
		if strings.EqualFold(strings.TrimSpace(name), wanted) && strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}

func resolveHeadersOutputPath(flagValue string) (string, error) {
	path := strings.TrimSpace(flagValue)
	if path == "" {
		path = strings.TrimSpace(os.Getenv("IFOOD_HEADERS_FILE"))
	}
	if path == "" {
		return "", errors.New("--out or IFOOD_HEADERS_FILE is required")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve headers path: %w", err)
	}
	return abs, nil
}

func importSessionHeaders(cmd *cobra.Command, flags *rootFlags, output string, headers map[string]string) error {
	if len(headers) == 0 || !hasHeader(headers, "authorization") {
		return usageErr(errors.New("session headers must include a non-empty Authorization header"))
	}
	names := make([]string, 0, len(headers))
	for name := range headers {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool { return strings.ToLower(names[i]) < strings.ToLower(names[j]) })
	result := map[string]any{"path": output, "mode": "0600", "header_names": names, "written": false}
	if flags.dryRun {
		return printNovelJSON(cmd, flags, result)
	}
	if !flags.yes {
		return usageErr(errors.New("installing session credentials requires --yes"))
	}
	if err := writePrivateHeaders(output, headers); err != nil {
		return err
	}
	result["written"] = true
	result["next_env"] = fmt.Sprintf("export IFOOD_HEADERS_FILE=%q", output)
	result["next_command"] = "ifood-pp-cli --json session status"
	return printNovelJSON(cmd, flags, result)
}

func writePrivateHeaders(path string, headers map[string]string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create headers directory: %w", err)
	}
	data, err := json.MarshalIndent(headers, "", "  ")
	if err != nil {
		return fmt.Errorf("encode headers: %w", err)
	}
	temp, err := os.CreateTemp(dir, ".ifood-headers-*")
	if err != nil {
		return fmt.Errorf("create temporary headers file: %w", err)
	}
	tempName := temp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tempName)
		}
	}()
	if err := temp.Chmod(0o600); err != nil {
		_ = temp.Close()
		return fmt.Errorf("secure temporary headers file: %w", err)
	}
	if _, err := temp.Write(append(data, '\n')); err != nil {
		_ = temp.Close()
		return fmt.Errorf("write temporary headers file: %w", err)
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return fmt.Errorf("sync temporary headers file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temporary headers file: %w", err)
	}
	if err := os.Rename(tempName, path); err != nil {
		return fmt.Errorf("install headers file: %w", err)
	}
	cleanup = false
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("secure headers file: %w", err)
	}
	return nil
}

func newMarketsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "markets", Short: "Discover grocery markets"}
	var latitude, longitude, minRating float64
	var limit int
	list := &cobra.Command{
		Use: "list", Short: "List markets near a delivery location",
		Example:     "  ifood-pp-cli markets list --latitude -9.65 --longitude -35.71 --min-rating 4.5 --dry-run --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "list iFood markets")
			}
			if latitude == 0 || longitude == 0 {
				return usageErr(errors.New("--latitude and --longitude are required"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			markets, err := fetchMarkets(cmd, c, latitude, longitude, minRating)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			if limit > 0 && len(markets) > limit {
				markets = markets[:limit]
			}
			return printNovelJSON(cmd, flags, markets)
		},
	}
	list.Flags().Float64Var(&latitude, "latitude", 0, "Delivery latitude")
	list.Flags().Float64Var(&longitude, "longitude", 0, "Delivery longitude")
	list.Flags().Float64Var(&minRating, "min-rating", 0, "Minimum user rating")
	list.Flags().IntVar(&limit, "limit", 20, "Maximum markets")
	cmd.AddCommand(list)
	return cmd
}

func newProductsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "products", Short: "Search grocery products"}
	var latitude, longitude float64
	var term string
	var size int
	var includeUnavailable bool
	search := &cobra.Command{
		Use: "search <merchant-id>", Short: "Search products inside one market", Args: cobra.ExactArgs(1),
		Example:     "  ifood-pp-cli products search MERCHANT_ID --latitude -9.65 --longitude -35.71 --term esponja --dry-run --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "search iFood products")
			}
			if latitude == 0 || longitude == 0 || strings.TrimSpace(term) == "" {
				return usageErr(errors.New("--latitude, --longitude, and --term are required"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			products, err := searchProducts(cmd, c, args[0], latitude, longitude, term, size, includeUnavailable)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			return printNovelJSON(cmd, flags, products)
		},
	}
	search.Flags().Float64Var(&latitude, "latitude", 0, "Delivery latitude")
	search.Flags().Float64Var(&longitude, "longitude", 0, "Delivery longitude")
	search.Flags().StringVar(&term, "term", "", "Product search term")
	search.Flags().IntVar(&size, "limit", 20, "Maximum products")
	search.Flags().BoolVar(&includeUnavailable, "include-unavailable", false, "Include unavailable products")
	cmd.AddCommand(search)
	return cmd
}

func newQuoteCmd(flags *rootFlags) *cobra.Command {
	var latitude, longitude, minRating float64
	var merchantIDs, items []string
	var marketLimit int
	var useExample bool
	cmd := &cobra.Command{
		Use:         "quote",
		Short:       "Price a list across eligible grocery markets",
		Example:     "  ifood-pp-cli quote --example --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if useExample {
				observation, err := loadBrowserExampleQuote()
				if err != nil {
					return err
				}
				validation, err := validateBrowserQuote(observation, 4.5, 3)
				if err != nil {
					return err
				}
				return printNovelJSON(cmd, flags, map[string]any{"source": "embedded_example", "items": validation.RequestedItems, "quotes": validation.Markets, "selected_market_id": validation.SelectedMarketID})
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "quote iFood products")
			}
			if latitude == 0 || longitude == 0 {
				return usageErr(errors.New("--latitude and --longitude are required"))
			}
			if len(items) == 0 {
				items = append([]string(nil), defaultQuoteItems...)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			markets, err := fetchMarkets(cmd, c, latitude, longitude, minRating)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			markets = selectMarkets(markets, merchantIDs, marketLimit)
			quotes := make([]storeQuote, 0, len(markets))
			for _, market := range markets {
				quote := storeQuote{Merchant: market, Complete: true}
				for _, term := range items {
					products, searchErr := searchProducts(cmd, c, market.ID, latitude, longitude, term, 100, false)
					line := quoteLine{Term: term, Status: "not_found"}
					if searchErr != nil {
						line.Status = "error: " + searchErr.Error()
						quote.Complete = false
					} else if len(products) > 0 {
						best := products[0]
						line.Product = &best
						line.Status = "found"
						quote.Subtotal += best.Price
					} else {
						quote.Complete = false
					}
					quote.Items = append(quote.Items, line)
				}
				quotes = append(quotes, quote)
			}
			return printNovelJSON(cmd, flags, map[string]any{"items": items, "min_rating": minRating, "quotes": quotes})
		},
	}
	cmd.Flags().Float64Var(&latitude, "latitude", 0, "Delivery latitude")
	cmd.Flags().Float64Var(&longitude, "longitude", 0, "Delivery longitude")
	cmd.Flags().Float64Var(&minRating, "min-rating", 4.5, "Minimum market rating")
	cmd.Flags().StringSliceVar(&merchantIDs, "merchant", nil, "Merchant UUID (repeat or comma-separate)")
	cmd.Flags().StringSliceVar(&items, "item", nil, "Item search term (repeat or comma-separate); defaults to the original six-item list")
	cmd.Flags().IntVar(&marketLimit, "markets", 3, "Maximum eligible markets")
	cmd.Flags().BoolVar(&useExample, "example", false, "Compare the embedded credential-free three-market example")
	return cmd
}

func fetchMarkets(cmd *cobra.Command, c *client.Client, latitude, longitude, minRating float64) ([]marketSummary, error) {
	body := map[string]any{
		"supported-headers": []string{"OPERATION_HEADER"},
		"supported-cards":   []string{"MERCHANT_LIST", "MERCHANT_LIST_V2", "FEATURED_MERCHANT_LIST", "MERCHANT_CAROUSEL", "MERCHANT_TILE_CAROUSEL", "SIMPLE_MERCHANT_CAROUSEL"},
		"supported-actions": []string{"merchant", "page", "card-content", "search", "groceries", "groceries-details", "home-tab"},
		"feed-feature-name": "", "faster-overrides": "",
	}
	raw, _, err := c.PostQueryWithParams(cmd.Context(), marketHomePath, map[string]string{
		"latitude": strconv.FormatFloat(latitude, 'f', -1, 64), "longitude": strconv.FormatFloat(longitude, 'f', -1, 64),
		"channel": defaultChannel, "size": "100", "alias": "HOME_GROCERIES_V3",
	}, body)
	if err != nil {
		return nil, err
	}
	var root any
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, fmt.Errorf("decode markets: %w", err)
	}
	byID := map[string]marketSummary{}
	collectMarkets(root, minRating, byID)
	markets := make([]marketSummary, 0, len(byID))
	for _, m := range byID {
		markets = append(markets, m)
	}
	sort.Slice(markets, func(i, j int) bool {
		if markets[i].Rating != markets[j].Rating {
			return markets[i].Rating > markets[j].Rating
		}
		return markets[i].DistanceKM < markets[j].DistanceKM
	})
	return markets, nil
}

func collectMarkets(value any, minRating float64, out map[string]marketSummary) {
	switch typed := value.(type) {
	case []any:
		for _, child := range typed {
			collectMarkets(child, minRating, out)
		}
	case map[string]any:
		id, _ := typed["id"].(string)
		name, _ := typed["name"].(string)
		rating, hasRating := numberValue(typed["userRating"])
		if id != "" && name != "" && hasRating && rating >= minRating {
			m := marketSummary{ID: id, Name: name, Rating: rating}
			m.Slug, _ = typed["slug"].(string)
			m.DistanceKM, _ = numberValue(typed["distance"])
			m.RatingCount, _ = intValue(typed["userRatingCount"])
			m.Available, _ = typed["available"].(bool)
			if fee, ok := typed["deliveryFee"].(map[string]any); ok {
				m.DeliveryFeeBRL, _ = numberValue(fee["value"])
			}
			out[id] = m
		}
		for _, child := range typed {
			collectMarkets(child, minRating, out)
		}
	}
}

func searchProducts(cmd *cobra.Command, c *client.Client, merchantID string, latitude, longitude float64, term string, size int, includeUnavailable bool) ([]productSummary, error) {
	if size <= 0 || size > 100 {
		size = 100
	}
	raw, err := c.Get(cmd.Context(), "/v2/search/merchants/"+merchantID+"/catalog-items", map[string]string{
		"latitude": strconv.FormatFloat(latitude, 'f', -1, 64), "longitude": strconv.FormatFloat(longitude, 'f', -1, 64), "channel": defaultChannel,
		"term": term, "size": strconv.Itoa(size), "page": "0", "item_from_merchant_ids": merchantID,
	})
	if err != nil {
		return nil, err
	}
	var response struct {
		Items struct {
			Data []struct {
				ID, Name, Description, EAN, Currency string
				Price                                float64
				Available                            bool
				Merchant                             map[string]any
			} `json:"data"`
		} `json:"items"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, fmt.Errorf("decode products: %w", err)
	}
	products := make([]productSummary, 0, len(response.Items.Data))
	for _, item := range response.Items.Data {
		if !includeUnavailable && !item.Available {
			continue
		}
		m := marketSummary{ID: merchantID}
		if item.Merchant != nil {
			m.Name, _ = item.Merchant["name"].(string)
			m.Rating, _ = numberValue(item.Merchant["userRating"])
			m.Available, _ = item.Merchant["available"].(bool)
		}
		products = append(products, productSummary{ID: item.ID, Name: item.Name, Description: item.Description, EAN: item.EAN, Price: item.Price, Currency: item.Currency, Available: item.Available, Merchant: m})
	}
	sort.SliceStable(products, func(i, j int) bool { return products[i].Price < products[j].Price })
	return products, nil
}

func selectMarkets(markets []marketSummary, ids []string, limit int) []marketSummary {
	if len(ids) > 0 {
		wanted := map[string]bool{}
		for _, id := range ids {
			wanted[id] = true
		}
		filtered := markets[:0]
		for _, m := range markets {
			if wanted[m.ID] {
				filtered = append(filtered, m)
			}
		}
		markets = filtered
	}
	if limit > 0 && len(markets) > limit {
		markets = markets[:limit]
	}
	return markets
}

func printNovelJSON(cmd *cobra.Command, flags *rootFlags, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return printOutput(cmd.OutOrStdout(), data, flags.asJSON || !isTerminal(cmd.OutOrStdout()))
}

func numberValue(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case json.Number:
		f, e := n.Float64()
		return f, e == nil
	}
	return 0, false
}
func intValue(v any) (int, bool) { f, ok := numberValue(v); return int(f), ok }
