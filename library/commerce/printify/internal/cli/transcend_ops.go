package cli

import (
	"encoding/json"
	"strings"

	"github.com/spf13/cobra"
)

type catalogMarginRow struct {
	VariantID       string  `json:"variant_id,omitempty"`
	Title           string  `json:"title,omitempty"`
	Cost            float64 `json:"cost"`
	Shipping        float64 `json:"shipping"`
	TargetPrice     float64 `json:"target_price"`
	EstimatedMargin float64 `json:"estimated_margin"`
	MarginPercent   float64 `json:"margin_percent"`
}

type assetReuseRow struct {
	ImageID       string   `json:"image_id"`
	FileName      string   `json:"file_name,omitempty"`
	UseCount      int      `json:"use_count"`
	ProductIDs    []string `json:"product_ids,omitempty"`
	Unused        bool     `json:"unused"`
	SharedArtwork bool     `json:"shared_artwork"`
}

type fulfillmentRiskRow struct {
	OrderID   string   `json:"order_id"`
	Status    string   `json:"status,omitempty"`
	ProductID string   `json:"product_id,omitempty"`
	VariantID string   `json:"variant_id,omitempty"`
	Risks     []string `json:"risks"`
}

func newCatalogMarginMatrixCmd(flags *rootFlags) *cobra.Command {
	var blueprintID, providerID, variantsFile, shippingFile string
	var targetPrice float64
	cmd := &cobra.Command{
		Use:     "catalog-margin-matrix",
		Short:   "Estimate Printify catalog variant margins at a target retail price",
		Example: "  printify-pp-cli catalog-margin-matrix --variants-file ./examples/sample-variants.json --shipping-file ./examples/sample-shipping.json --target-price 24.99 --agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			needsLiveCatalog := variantsFile == "" || shippingFile == ""
			if ((needsLiveCatalog && (blueprintID == "" || providerID == "")) || targetPrice <= 0) && !flags.dryRun {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			variants, shipping, err := loadCatalogInputs(cmd, flags, blueprintID, providerID, variantsFile, shippingFile)
			if err != nil {
				return err
			}
			rows := buildCatalogMarginMatrix(variants, shipping, targetPrice)
			raw, err := json.Marshal(rows)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
		},
	}
	cmd.Flags().StringVar(&blueprintID, "blueprint-id", "", "Printify blueprint ID")
	cmd.Flags().StringVar(&providerID, "provider-id", "", "Print provider ID")
	cmd.Flags().Float64Var(&targetPrice, "target-price", 0, "Target retail price")
	cmd.Flags().StringVar(&variantsFile, "variants-file", "", "Variants JSON file instead of live catalog lookup")
	cmd.Flags().StringVar(&shippingFile, "shipping-file", "", "Shipping JSON file instead of live catalog lookup")
	return cmd
}

func newAssetReuseCmd(flags *rootFlags) *cobra.Command {
	var productsFile, uploadsFile, dbPath string
	cmd := &cobra.Command{
		Use:     "asset-reuse",
		Short:   "Map uploaded images to products and unused assets",
		Example: "  printify-pp-cli asset-reuse --products-file ./examples/sample-products.json --uploads-file ./examples/sample-uploads.json --agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			products, err := ppLoadObjectsFromFileOrStore(productsFile, dbPath, []string{"products", "products-json", "products_json", "shops-products-json"}, 10000)
			if err != nil {
				return err
			}
			uploads, err := ppLoadObjectsFromFileOrStore(uploadsFile, dbPath, []string{"uploads-json", "uploads_json"}, 10000)
			if err != nil {
				return err
			}
			rows := buildAssetReuse(products, uploads)
			raw, err := json.Marshal(rows)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
		},
	}
	cmd.Flags().StringVar(&productsFile, "products-file", "", "Products JSON file")
	cmd.Flags().StringVar(&uploadsFile, "uploads-file", "", "Uploads JSON file")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local SQLite store path")
	return cmd
}

func newFulfillmentRiskCmd(flags *rootFlags) *cobra.Command {
	var ordersFile, productsFile, dbPath string
	cmd := &cobra.Command{
		Use:     "fulfillment-risk",
		Short:   "Flag open orders with risky product, variant, or shipment state",
		Example: "  printify-pp-cli fulfillment-risk --orders-file ./examples/sample-orders.json --products-file ./examples/sample-products.json --agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			orders, err := ppLoadObjectsFromFileOrStore(ordersFile, dbPath, []string{"orders", "orders-json", "orders_json", "shops-orders-json"}, 10000)
			if err != nil {
				return err
			}
			products, err := ppLoadObjectsFromFileOrStore(productsFile, dbPath, []string{"products", "products-json", "products_json", "shops-products-json"}, 10000)
			if err != nil {
				return err
			}
			rows := buildFulfillmentRisk(orders, products)
			raw, err := json.Marshal(rows)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
		},
	}
	cmd.Flags().StringVar(&ordersFile, "orders-file", "", "Orders JSON file")
	cmd.Flags().StringVar(&productsFile, "products-file", "", "Products JSON file")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local SQLite store path")
	return cmd
}

func loadCatalogInputs(cmd *cobra.Command, flags *rootFlags, blueprintID, providerID, variantsFile, shippingFile string) ([]ppJSONObj, []ppJSONObj, error) {
	if variantsFile != "" && shippingFile != "" {
		variantsRaw, err := ppLoadJSONFile(variantsFile)
		if err != nil {
			return nil, nil, err
		}
		shippingRaw, err := ppLoadJSONFile(shippingFile)
		if err != nil {
			return nil, nil, err
		}
		variants, err := ppDecodeObjects(variantsRaw)
		if err != nil {
			return nil, nil, err
		}
		shipping, err := ppDecodeObjects(shippingRaw)
		return variants, shipping, err
	}
	client, err := flags.newClient()
	if err != nil {
		return nil, nil, err
	}
	base := "/v1/catalog/blueprints/{blueprint_id}/print_providers/{print_provider_id}"
	base = replacePathParam(base, "blueprint_id", blueprintID)
	base = replacePathParam(base, "print_provider_id", providerID)
	variantsRaw, err := client.Get(cmd.Context(), base+"/variants.json", map[string]string{})
	if err != nil {
		return nil, nil, classifyAPIError(err, flags)
	}
	shippingRaw, err := client.Get(cmd.Context(), base+"/shipping.json", map[string]string{})
	if err != nil {
		return nil, nil, classifyAPIError(err, flags)
	}
	variants, err := ppDecodeObjects(variantsRaw)
	if err != nil {
		return nil, nil, err
	}
	shipping, err := ppDecodeObjects(shippingRaw)
	return variants, shipping, err
}

func buildCatalogMarginMatrix(variants, shipping []ppJSONObj, targetPrice float64) []catalogMarginRow {
	shippingCost := lowestShippingCost(shipping)
	rows := make([]catalogMarginRow, 0, len(variants))
	for _, variant := range variants {
		cost := ppCentsToDollars(ppFloat(variant, "cost", "price", "cost_cents"))
		margin := targetPrice - cost - shippingCost
		marginPercent := 0.0
		if targetPrice > 0 {
			marginPercent = (margin / targetPrice) * 100
		}
		rows = append(rows, catalogMarginRow{
			VariantID:       ppString(variant, "id", "variant_id"),
			Title:           ppString(variant, "title", "name"),
			Cost:            ppRound2(cost),
			Shipping:        ppRound2(shippingCost),
			TargetPrice:     ppRound2(targetPrice),
			EstimatedMargin: ppRound2(margin),
			MarginPercent:   ppRound2(marginPercent),
		})
	}
	return rows
}

func lowestShippingCost(shipping []ppJSONObj) float64 {
	lowest := 0.0
	for _, item := range shipping {
		for _, key := range []string{"first_item", "cost", "price"} {
			value := shippingCostDollars(ppLookup(item, key))
			if value > 0 && (lowest == 0 || value < lowest) {
				lowest = value
				break
			}
		}
	}
	return lowest
}

func shippingCostDollars(value any) float64 {
	switch typed := value.(type) {
	case map[string]any:
		for _, key := range []string{"cost", "amount", "value"} {
			if dollars := shippingCostDollars(ppLookup(ppJSONObj(typed), key)); dollars > 0 {
				return dollars
			}
		}
	case json.RawMessage:
		var decoded any
		if json.Unmarshal(typed, &decoded) == nil {
			return shippingCostDollars(decoded)
		}
	default:
		return ppCentsToDollars(ppFloat(ppJSONObj{"value": value}, "value"))
	}
	return 0
}

func buildAssetReuse(products, uploads []ppJSONObj) []assetReuseRow {
	uses := map[string]map[string]bool{}
	for _, product := range products {
		productID := ppString(product, "id")
		for _, row := range buildPlacementMatrix(product, nil) {
			if row.ImageID == "" {
				continue
			}
			if uses[row.ImageID] == nil {
				uses[row.ImageID] = map[string]bool{}
			}
			uses[row.ImageID][productID] = true
		}
	}
	rows := make([]assetReuseRow, 0, len(uploads))
	for _, upload := range uploads {
		imageID := ppString(upload, "id")
		productIDs := setKeys(uses[imageID])
		rows = append(rows, assetReuseRow{
			ImageID:       imageID,
			FileName:      ppString(upload, "file_name", "filename", "name"),
			UseCount:      len(productIDs),
			ProductIDs:    productIDs,
			Unused:        len(productIDs) == 0,
			SharedArtwork: len(productIDs) > 1,
		})
	}
	return rows
}

func buildFulfillmentRisk(orders, products []ppJSONObj) []fulfillmentRiskRow {
	productRisks := map[string][]string{}
	for _, product := range products {
		productID := ppString(product, "id")
		if productID != "" {
			productRisks[productID] = productStateRisks(product)
		}
	}
	var rows []fulfillmentRiskRow
	for _, order := range orders {
		status := strings.ToLower(ppString(order, "status"))
		if strings.Contains(status, "fulfilled") || strings.Contains(status, "cancel") {
			continue
		}
		orderID := ppString(order, "id")
		lineItems := ppArray(firstNonNil(ppLookup(order, "line_items"), ppLookup(order, "items")))
		if len(lineItems) == 0 {
			rows = append(rows, fulfillmentRiskRow{OrderID: orderID, Status: status, Risks: []string{"missing line items"}})
			continue
		}
		for _, lineValue := range lineItems {
			line := ppObject(lineValue)
			productID := ppString(line, "product_id")
			variantID := ppString(line, "variant_id")
			risks := []string{}
			if productID == "" {
				risks = append(risks, "missing product id")
			} else if stateRisks, ok := productRisks[productID]; ok {
				risks = append(risks, stateRisks...)
			} else if len(products) > 0 {
				risks = append(risks, "product not found locally")
			}
			if variantID == "" {
				risks = append(risks, "missing variant id")
			}
			if len(ppArray(ppLookup(order, "shipments"))) == 0 {
				risks = append(risks, "no shipment records")
			}
			if len(risks) > 0 {
				rows = append(rows, fulfillmentRiskRow{OrderID: orderID, Status: status, ProductID: productID, VariantID: variantID, Risks: risks})
			}
		}
	}
	return rows
}

func productStateRisks(product ppJSONObj) []string {
	risks := []string{}
	if visible, ok := ppLookup(product, "visible").(bool); ok && !visible {
		risks = append(risks, "product hidden")
	}
	if ppBool(product, "is_locked") {
		risks = append(risks, "product locked")
	}
	status := strings.ToLower(ppString(product, "status"))
	if status != "" && status != "published" && status != "visible" {
		risks = append(risks, "product "+status)
	}
	return risks
}

func setKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		if key != "" {
			keys = append(keys, key)
		}
	}
	return keys
}

func firstNonNil(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}
