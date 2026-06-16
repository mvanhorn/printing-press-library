package cli

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestNormalizeBrayAndScarffProductUsesDealerPriceAndSourceMetadata(t *testing.T) {
	product := normalizeBrayAndScarffProduct(map[string]any{
		"id":                          "dealer-row",
		"regular":                     2449.99,
		"sale":                        1399.99,
		"validation_price":            1399.99,
		"validation_price_overridden": nil,
		"category": map[string]any{
			"category_translations": []any{map[string]any{"value": "Cooktops (Electric)", "slug_value": "cooktops-electric"}},
			"parent_category": map[string]any{
				"category_translations": []any{map[string]any{"value": "Cooktops", "slug_value": "cooktops"}},
				"parent_category": map[string]any{
					"category_translations": []any{map[string]any{"value": "Cooking", "slug_value": "cooking"}},
					"parent_category": map[string]any{
						"category_translations": []any{map[string]any{"value": "Appliances", "slug_value": "appliances"}},
					},
				},
			},
		},
		"default_source_product": map[string]any{
			"id":              "source-row",
			"manufacturer_pn": "CIT36YWBB",
			"pn":              "CIT36YWBB",
			"product_translations": []any{map[string]any{
				"short_description": `Freedom Series 36" Built-In Electric Induction Cooktop`,
				"long_description":  "56-element Freedom cooking surface.",
			}},
			"brand": map[string]any{"name": "THERMADOR", "slug": "thermador", "code": "THE"},
			"product_images": []any{map[string]any{
				"media_url": "https://cdn.nmg-platform.com/products/cit36ywbb.webp",
			}},
			"product_attributes": []any{
				map[string]any{"attribute_code": "width", "name": "Width", "value": `36"`},
			},
		},
	})

	if product.Source != "bray-and-scarff" {
		t.Fatalf("unexpected source: %#v", product)
	}
	if product.ID != "CIT36YWBB" || product.Brand != "THERMADOR" {
		t.Fatalf("expected source model and brand, got %#v", product)
	}
	if product.Title != `Freedom Series 36" Built-In Electric Induction Cooktop` {
		t.Fatalf("unexpected title: %q", product.Title)
	}
	if product.PriceMin != 1399.99 || product.RegularPriceMin != 2449.99 || !product.OnSale {
		t.Fatalf("expected dealer sale pricing, got %#v", product)
	}
	if product.Category != "Appliances > Cooking > Cooktops > Cooktops (Electric)" {
		t.Fatalf("unexpected category: %q", product.Category)
	}
	wantURL := "https://www.brayandscarff.com/appliances/cooking/cooktops/cooktops-electric/thermador/cit36ywbb/"
	if product.URL != wantURL {
		t.Fatalf("unexpected URL: %q", product.URL)
	}
	if product.ImageURL != "https://cdn.nmg-platform.com/products/cit36ywbb.webp" {
		t.Fatalf("unexpected image URL: %q", product.ImageURL)
	}
	if product.Description != "56-element Freedom cooking surface.\nWidth: 36\"" {
		t.Fatalf("unexpected description: %q", product.Description)
	}
}

func TestNormalizePCRichardProductUsesGA4TileData(t *testing.T) {
	product := normalizePCRichardProduct(`<div class="product-tile product-details"
		data-ga4-select-item="{&quot;event&quot;:&quot;select_item&quot;,&quot;ecommerce&quot;:{&quot;items&quot;:[{&quot;item_id&quot;:&quot;GDT670SYVFS&quot;,&quot;item_name&quot;:&quot;GE 24 in. Top Control Dishwasher&quot;,&quot;master_id&quot;:&quot;M-0006661&quot;,&quot;currency&quot;:&quot;USD&quot;,&quot;brand&quot;:&quot;GE&quot;,&quot;item_category2&quot;:&quot;Kitchen Appliances&quot;,&quot;item_category3&quot;:&quot;Dishwashers&quot;,&quot;item_category4&quot;:&quot;Built-In Dishwashers&quot;,&quot;item_category5&quot;:&quot;24 inch Built-In Dishwashers&quot;,&quot;price&quot;:598.97}]}}">
		<a href="https://www.pcrichard.com/ge-24-in-top-control-dishwasher/GDT670SYVFS.html">
			<img class="tile-image product-anchor" src="https://www.pcrichard.com/dw/image/v2/BFXM_PRD/GDT670SYVFS.jpg?sw=400&amp;sh=400">
		</a>
		<span class="value" content="1049.97"><span class="sr-only">Price reduced from</span></span>
	</div>`)

	if product.Source != "pc-richard" {
		t.Fatalf("unexpected source: %#v", product)
	}
	if product.ID != "GDT670SYVFS" || product.Brand != "GE" {
		t.Fatalf("expected model and brand, got %#v", product)
	}
	if product.Title != "GE 24 in. Top Control Dishwasher" {
		t.Fatalf("unexpected title: %q", product.Title)
	}
	if product.Category != "Kitchen Appliances > Dishwashers > Built-In Dishwashers > 24 inch Built-In Dishwashers" {
		t.Fatalf("unexpected category: %q", product.Category)
	}
	if product.PriceMin != 598.97 || product.RegularPriceMin != 1049.97 || !product.OnSale {
		t.Fatalf("expected sale pricing, got %#v", product)
	}
	if product.URL != "https://www.pcrichard.com/ge-24-in-top-control-dishwasher/GDT670SYVFS.html" {
		t.Fatalf("unexpected URL: %q", product.URL)
	}
	if product.ImageURL != "https://www.pcrichard.com/dw/image/v2/BFXM_PRD/GDT670SYVFS.jpg?sw=400&sh=400" {
		t.Fatalf("unexpected image URL: %q", product.ImageURL)
	}
}

func TestNormalizeApplianceFactoryProductUsesAVBRestFields(t *testing.T) {
	product := normalizeApplianceFactoryProduct(map[string]any{
		"entity_id":                           "1735149",
		"sku":                                 "1728338",
		"model_number":                        "JOESC330RM",
		"name":                                `Jennair® NOIR™ 30" Stainless Steel Single Electric Wall Oven with MultiMode® True Convection `,
		"manufacturer":                        "JennAir",
		"collection_name":                     "NOIR™",
		"color":                               "Stainless Steel",
		"regular_price_without_tax":           3899.00,
		"final_price_without_tax":             3299.00,
		"reviews_count":                       14,
		"reviews_rating":                      4.6,
		"short_description":                   "Smart wall oven with convection.",
		"inventory_label":                     "Out Of Stock",
		"energy_star_qualified":               "No",
		"upccode":                             "883049702933",
		"product_url":                         "https://www.appliancefactory.com/product/jennair-noir-30-stainless-steel-single-electric-wall-oven-with-multimode-true-convection-joesc330rm-1728338",
		"image_url":                           map[string]any{"normal": "https://linqcdn.avbportal.com/images/wall-oven.jpg?w=640"},
		"final_price_with_tax":                3299.00,
		"regular_price_with_tax":              3899.00,
		"discount_instant_savings":            600,
		"discount_consumer_rebate":            0,
		"consumer_rebate_to_date":             nil,
		"instant_savings_to_date":             "2026-06-30T23:59:59+00:00",
		"brandsource_point_of_sale_marketing": []any{"All Appliances", "Luxury"},
		"promotions": map[string]any{
			"product_promotions": []any{map[string]any{
				"catalog_title":   "JennAir Instant Rebate",
				"rebate_form_url": "https://linqcdn.avbportal.com/consumerrebates/jennair.pdf",
			}},
		},
	}, "Electric Single Oven Built-In")

	if product.Source != "appliance-factory" {
		t.Fatalf("unexpected source: %#v", product)
	}
	if product.ID != "JOESC330RM" || product.Brand != "JennAir" {
		t.Fatalf("expected model and brand, got %#v", product)
	}
	if product.Title != `Jennair® NOIR™ 30" Stainless Steel Single Electric Wall Oven with MultiMode® True Convection` {
		t.Fatalf("unexpected title: %q", product.Title)
	}
	if product.PriceMin != 3299.00 || product.RegularPriceMin != 3899.00 || !product.OnSale {
		t.Fatalf("expected sale pricing, got %#v", product)
	}
	if product.Category != "NOIR™ > Stainless Steel" {
		t.Fatalf("unexpected category: %q", product.Category)
	}
	if product.URL != "https://www.appliancefactory.com/product/jennair-noir-30-stainless-steel-single-electric-wall-oven-with-multimode-true-convection-joesc330rm-1728338" {
		t.Fatalf("unexpected URL: %q", product.URL)
	}
	if product.ImageURL != "https://linqcdn.avbportal.com/images/wall-oven.jpg?w=640" {
		t.Fatalf("unexpected image URL: %q", product.ImageURL)
	}
	if !strings.Contains(product.Description, "JennAir Instant Rebate https://linqcdn.avbportal.com/consumerrebates/jennair.pdf") {
		t.Fatalf("expected rebate metadata in description, got %q", product.Description)
	}
}

func TestApplianceFactoryCategoryKeyExtractsDidYouMeanCatalogLink(t *testing.T) {
	key := applianceFactoryCategoryKey([]string{`Did you mean:&nbsp; <a href="%2Fcatalog%2Fdishwashers">dishwasher</a>`})
	if key != "dishwashers" {
		t.Fatalf("expected dishwashers, got %q", key)
	}
}

func TestApplyApplianceFactoryHeadersMatchesReplayContract(t *testing.T) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://www.appliancefactory.com/api/rest/search/joesc330rm", nil)
	if err != nil {
		t.Fatal(err)
	}
	applyApplianceFactoryHeaders(req, "https://www.appliancefactory.com/search/joesc330rm")

	if req.Header.Get("Accept") != "application/json" || req.Header.Get("language") != "en-US" {
		t.Fatalf("missing JSON/language headers: %#v", req.Header)
	}
	if req.Header.Get("x-px-access-token") != applianceFactoryPXAccessToken {
		t.Fatalf("missing PX replay token")
	}
	if req.Header.Get("Origin") != "https://www.appliancefactory.com" || req.Header.Get("Referer") != "https://www.appliancefactory.com/search/joesc330rm" {
		t.Fatalf("missing origin/referer: %#v", req.Header)
	}
	if req.Header.Get("Sec-Fetch-Site") != "same-origin" {
		t.Fatalf("missing fetch metadata: %#v", req.Header)
	}
}

func TestHomewiseApplianceProductsFromBodyExtractsExactModel(t *testing.T) {
	body := []byte(`{"success":true,"msg":"Data fetched successfully","data":[{"price":3899,"pid":40142,"brand":"Jennair","large_image":["https://d2kv2tdugu7e0m.cloudfront.net/ProductImage/e9faa5da690d5bad.png"],"title":"JennAir NOIR™ 30 Inch Single Convection Electric Smart Wall Oven with 5.0 cu. ft. Capacity","thumb_image":"https://d2kv2tdugu7e0m.cloudfront.net/ProductImage/e9faa5da690d5bad.png","url":"https://www.homewiseappliance.com/jennair-noir-30-inch-single-convection-electric-smart-wall-oven-with-50-cu-ft-multimode-joesc330rm","status":"S","categoryId":355,"subCategoryId":351,"quantity":0,"msrp":3899,"pdpSeoUrl":"jennair-noir-30-inch-single-convection-electric-smart-wall-oven-with-50-cu-ft-multimode-joesc330rm","categoryName":"Cooking","id":40142,"sellableQuantity":0,"sku":"JOESC330RM","topSixSpecifications":"[{\"specificationId\":817210,\"key\":\"Wall Oven Size (in.)\",\"value\":\"30\\\"\"},{\"specificationId\":817234,\"key\":\"Fuel Type\",\"value\":\"Electric\"}]"}]}`)

	products, err := homewiseApplianceProductsFromBody(body, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(products) != 1 {
		t.Fatalf("expected one product, got %#v", products)
	}
	product := products[0]
	if product.Source != "homewise-appliance" || product.ID != "JOESC330RM" || product.Brand != "Jennair" {
		t.Fatalf("unexpected source/model/brand: %#v", product)
	}
	if product.PriceMin != 3899 || product.RegularPriceMin != 3899 {
		t.Fatalf("expected Homewise price, got %#v", product)
	}
	if product.Category != "Cooking" {
		t.Fatalf("unexpected category: %q", product.Category)
	}
	if product.URL != "https://www.homewiseappliance.com/jennair-noir-30-inch-single-convection-electric-smart-wall-oven-with-50-cu-ft-multimode-joesc330rm" {
		t.Fatalf("unexpected URL: %q", product.URL)
	}
	if product.ImageURL != "https://d2kv2tdugu7e0m.cloudfront.net/ProductImage/e9faa5da690d5bad.png" {
		t.Fatalf("unexpected image URL: %q", product.ImageURL)
	}
	if !strings.Contains(product.Description, `Wall Oven Size (in.): 30"`) || !strings.Contains(product.Description, "Fuel Type: Electric") {
		t.Fatalf("expected top-six specs in description, got %q", product.Description)
	}
}

func TestNormalizeHomewiseApplianceProductUsesDishwasherRow(t *testing.T) {
	product := normalizeHomewiseApplianceProduct(map[string]any{
		"large_image":      []any{"https://d29l4doet7kzwj.cloudfront.net/ProductImage/aec2bb07540646de.png"},
		"title":            "Whirlpool Eco Series 24 Inch Fully Integrated Dishwasher with 15 Place Settings",
		"price":            709.0,
		"thumb_image":      "https://d29l4doet7kzwj.cloudfront.net/ProductImage/aec2bb07540646de.png",
		"url":              "https://www.homewiseappliance.com/whirlpool-24-inch-eco-series-dishwasher-stainless-steel-wdts7024rz",
		"pid":              27381.0,
		"brand":            "Whirlpool",
		"sku":              "WDTS7024RZ",
		"msrp":             1049.0,
		"sellableQuantity": 11.0,
		"isInStock":        true,
		"categoryName":     "Dishwashers",
		"applianceType":    "Major Appliance",
		"topSixSpecifications": `[
			{"specificationId":698700,"key":"Dishwasher Size (in.)","value":"24\""},
			{"specificationId":698736,"key":"Decibels (dBA)","value":"41 dBA"}
		]`,
	})

	if product.Source != "homewise-appliance" || product.ID != "WDTS7024RZ" || product.Brand != "Whirlpool" {
		t.Fatalf("unexpected source/model/brand: %#v", product)
	}
	if product.PriceMin != 709 || product.RegularPriceMin != 1049 || !product.OnSale {
		t.Fatalf("expected sale pricing, got %#v", product)
	}
	if product.Category != "Dishwashers" {
		t.Fatalf("unexpected category: %q", product.Category)
	}
	if !strings.Contains(product.Description, "sellable quantity: 11") || !strings.Contains(product.Description, `Dishwasher Size (in.): 24"`) {
		t.Fatalf("expected inventory/spec metadata, got %q", product.Description)
	}
}

func TestApplyHomewiseApplianceHeadersMatchesReplayContract(t *testing.T) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, homewiseApplianceBaseURL+"/bloomreach-hw", nil)
	if err != nil {
		t.Fatal(err)
	}
	applyHomewiseApplianceHeaders(req)

	if req.Header.Get("Accept") != "application/json, text/plain, */*" {
		t.Fatalf("missing JSON accept header: %#v", req.Header)
	}
	if req.Header.Get("Origin") != "https://www.homewiseappliance.com" || req.Header.Get("Referer") != "https://www.homewiseappliance.com/dishwashers" {
		t.Fatalf("missing origin/referer: %#v", req.Header)
	}
	if req.Header.Get("Sec-Fetch-Site") != "cross-site" {
		t.Fatalf("missing cross-site fetch metadata: %#v", req.Header)
	}
}

func TestBestBuyProductsFromHTMLExtractsServerRenderedProductAndPrice(t *testing.T) {
	html := `<script>(window[Symbol.for("ApolloSSRDataTransport")] ??= []).push({"rehydrate":{
		":product":{"data":{"product":{"__typename":"Product","skuId":"6549423","openBoxCondition":null,"whatItIs":["Built-In Dishwasher","Dishwasher","Major Appliance"],"bsin":"J7645SG674","condition":{"__typename":"ProductCondition","type":"new"},"url":{"__typename":"ProductUrl","pdp":"https://www.bestbuy.com/product/ge-24-top-control-built-in-stainless-steel-tub-dishwasher-with-3rd-rack-sanitize-cycle-and-47-dba-stainless-steel/J7645SG674","relativePdp":"/product/ge-24-top-control-built-in-stainless-steel-tub-dishwasher-with-3rd-rack-sanitize-cycle-and-47-dba-stainless-steel/J7645SG674","skuSpecificUrl":"https://www.bestbuy.com/product/ge-24-top-control-built-in-stainless-steel-tub-dishwasher-with-3rd-rack-sanitize-cycle-and-47-dba-stainless-steel/J7645SG674/sku/6549423"},"primaryImage":{"__typename":"ProductImage","piscesHref":"https://pisces.bbystatic.com/image2/BestBuy_US/images/products/28d95010-d15c-4420-a82e-2b59a031e1b8.jpg"},"name":{"__typename":"ProductName","short":"GE - 24\" Top Control Built-In Stainless Steel Tub Dishwasher with 3rd Rack, Sanitize Cycle and 47 dBA - Stainless Steel"},"manufacturer":{"__typename":"Manufacturer","modelNumber":"GDT650SYVFS"},"brand":"GE","reviewInfo":{"__typename":"ProductReviewInfo","averageRating":4.4,"reviewCount":512}}}},
		":openbox":{"data":{"product":{"__typename":"Product","skuId":"6549423","price":{"__typename":"ItemPrice","customerPrice":426.99,"skuId":"6549423","openBoxCondition":0}}}},
		":price":{"data":{"product":{"__typename":"Product","price":{"__typename":"ItemPrice","customerPrice":568.99,"mobileContracts":[],"skuId":"6549423","displayableCustomerPrice":568.99,"totalNonPaidMemberSavings":286,"openBoxCondition":null}}}}
	}})</script>`

	products := bestBuyProductsFromHTML(html, 5)
	if len(products) != 1 {
		t.Fatalf("expected one product, got %#v", products)
	}
	product := products[0]
	if product.Source != "best-buy" {
		t.Fatalf("unexpected source: %#v", product)
	}
	if product.ID != "GDT650SYVFS" || product.Brand != "GE" {
		t.Fatalf("expected manufacturer model and brand, got %#v", product)
	}
	if product.Title != `GE - 24" Top Control Built-In Stainless Steel Tub Dishwasher with 3rd Rack, Sanitize Cycle and 47 dBA - Stainless Steel` {
		t.Fatalf("unexpected title: %q", product.Title)
	}
	if product.PriceMin != 568.99 || product.RegularPriceMin != 854.99 || !product.OnSale {
		t.Fatalf("expected new-item sale pricing, got %#v", product)
	}
	if product.URL != "https://www.bestbuy.com/product/ge-24-top-control-built-in-stainless-steel-tub-dishwasher-with-3rd-rack-sanitize-cycle-and-47-dba-stainless-steel/J7645SG674/sku/6549423" {
		t.Fatalf("unexpected URL: %q", product.URL)
	}
	if product.ImageURL != "https://pisces.bbystatic.com/image2/BestBuy_US/images/products/28d95010-d15c-4420-a82e-2b59a031e1b8.jpg" {
		t.Fatalf("unexpected image URL: %q", product.ImageURL)
	}
	if product.Category != "Built-In Dishwasher > Dishwasher > Major Appliance" {
		t.Fatalf("unexpected category: %q", product.Category)
	}
	if product.Rating != 4.4 || product.ReviewCount != 512 {
		t.Fatalf("expected review metadata, got %#v", product)
	}
}

func TestAbtProductsFromHTMLExtractsProductSchemaRedirect(t *testing.T) {
	html := `<script type="application/ld+json" id="productschema">{"@context":"https://schema.org","@type":"Product","@id":"https://www.abt.com/JennAir-NOIR-Single-Wall-Oven-With-MultiMode-30-Inch-Wide-in-Stainless-Steel-JOESC330RM/p/220958.html","name":"JennAir NOIR Single Wall Oven With MultiMode 30-Inch Wide in Stainless Steel - JOESC330RM","brand":{"@type":"Brand","name":"JennAir"},"description":"30-Inch Single Oven.","productID":220958,"sku":"JOESC330RM","model":"JOESC330RM","offers":{"@type":"Offer","price":"3899.00","priceCurrency":"USD"},"image":["https://content.abt.com/media/images/products/BDP_Images/jennair-wall-oven-JOESC330RM-main.jpg"]}</script>`

	products := abtProductsFromHTML(html, 5)
	if len(products) != 1 {
		t.Fatalf("expected one product, got %#v", products)
	}
	product := products[0]
	if product.Source != "abt" || product.ID != "JOESC330RM" || product.Brand != "JennAir" {
		t.Fatalf("unexpected source/model/brand: %#v", product)
	}
	if product.PriceMin != 3899 || product.RegularPriceMin != 3899 {
		t.Fatalf("expected schema price, got %#v", product)
	}
	if product.URL != "https://www.abt.com/JennAir-NOIR-Single-Wall-Oven-With-MultiMode-30-Inch-Wide-in-Stainless-Steel-JOESC330RM/p/220958.html" {
		t.Fatalf("unexpected URL: %q", product.URL)
	}
}

func TestAbtProductsFromHTMLExtractsCategoryRows(t *testing.T) {
	html := `<div class="category_item_container" role="group" aria-label="product">
		<img src="https://content.abt.com/image.php/x?image=/images/products/BDP_Images/bosch-dishwasher-SHP78CM5N-open-empty.jpg&amp;width=165" alt="Bosch dishwasher" />
		<div class="category_item_content_container">
			<div class="cl_title"><a href="https://www.abt.com/Bosch-800-Series-24-Inch-Dishwasher-in-Anti-Fingerprint-Stainless-Steel-SHP78CM5N/p/181764.html" class="categoryTitleLink productPageLink" data-productId="181764">Bosch 800 Series 24-Inch Dishwasher in Anti-Fingerprint Stainless Steel - SHP78CM5N</a></div>
			<div class="cl_abt_model">Abt Model: SHP78CM5SS</div>
			<div class="category_instock_text">In Stock</div>
			<div class="pricing_wrapper pricing-class-category"><div class="pricing-item-price"><span class="sr-only">Your Price: </span>$1,349</div><div class="pricing-was-price"><div class="pricing-regular-price"><span class="sr-only">Regular Price: </span>Comp. Value: $1,499</div></div></div>
			<button class="addToCart" data-category-name="Built In Dishwashers" data-productId="181764">ADD TO CART</button>
		</div>
	</div>`

	products := abtProductsFromHTML(html, 5)
	if len(products) != 1 {
		t.Fatalf("expected one product, got %#v", products)
	}
	product := products[0]
	if product.Source != "abt" || product.ID != "SHP78CM5SS" || product.Brand != "Bosch" {
		t.Fatalf("unexpected normalized product: %#v", product)
	}
	if product.PriceMin != 1349 || product.RegularPriceMin != 1499 || !product.OnSale {
		t.Fatalf("expected category sale price, got %#v", product)
	}
	if product.Category != "Built In Dishwashers" || !strings.Contains(product.Description, "In Stock") {
		t.Fatalf("unexpected category/availability: %#v", product)
	}
}

func TestQualityBathProductsFromHTMLExtractsHydratedSearchRows(t *testing.T) {
	html := `<script>
window.__INITIAL_QUERIES__ = JSON.parse('{"mutations":[],"queries":[{"state":{"data":{"products":[{"id":94648,"generatedSku":"1.00","category":{"name":"Bath"},"subCategory":"shower controls","brand":{"id":559,"name":"Sigma"},"title":"Series 620 1/2\\" Thermostatic Shower Set Valve","sku":"1.00","price":{"startingPrice":245.7,"discountedPrice":209.85},"coupon":{"customCouponMessage":"Use Code SAVE12 for an extra 12% off all Sigma"},"url":"/sigma-100-moderne-620-series-12-thermostatic-shower-set-valve-product-94648.htm","priceDisplay":{"original":{"type":"cross-out","value":245.7}},"image":{"cloudinaryId":"images/originals/156/156911"}}]}},"queryKey":["search.getFull",{"id":"shower valve","pageType":"search"}]}]}');
</script>`

	products, err := qualityBathProductsFromHTML(html, 5)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(products) != 1 {
		t.Fatalf("expected one product, got %#v", products)
	}
	product := products[0]
	if product.Source != "qualitybath" {
		t.Fatalf("unexpected source: %#v", product)
	}
	if product.ID != "1.00" || product.Brand != "Sigma" {
		t.Fatalf("expected SKU and brand, got %#v", product)
	}
	if product.Title != `Series 620 1/2" Thermostatic Shower Set Valve` {
		t.Fatalf("unexpected title: %q", product.Title)
	}
	if product.PriceMin != 209.85 || product.RegularPriceMin != 245.7 || !product.OnSale {
		t.Fatalf("expected sale pricing, got %#v", product)
	}
	if product.Category != "Bath > shower controls" {
		t.Fatalf("unexpected category: %q", product.Category)
	}
	if product.URL != "https://www.qualitybath.com/sigma-100-moderne-620-series-12-thermostatic-shower-set-valve-product-94648.htm" {
		t.Fatalf("unexpected URL: %q", product.URL)
	}
	if product.ImageURL != "https://qb-res.cloudinary.com/f_auto,q_auto/images/originals/156/156911" {
		t.Fatalf("unexpected image URL: %q", product.ImageURL)
	}
	if !strings.Contains(product.Description, "SAVE12") {
		t.Fatalf("expected coupon message, got %q", product.Description)
	}
}

func TestQualityBathProductsFromHTMLExtractsSavedLivePayload(t *testing.T) {
	body, err := os.ReadFile("../../.manuscripts/20260601-source-exploration/discovery/live-qualitybath-search-shower-valve.html")
	if err != nil {
		t.Skipf("saved QualityBath payload not available: %v", err)
	}
	products, err := qualityBathProductsFromHTML(string(body), 3)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(products) != 3 {
		t.Fatalf("expected three products, got %#v", products)
	}
	first := products[0]
	if first.Source != "qualitybath" || first.ID != "2-VR" || first.Brand != "California Faucets" {
		t.Fatalf("unexpected first product identity: %#v", first)
	}
	if first.Title != "2 Handle Tub or Shower Valve" || first.PriceMin != 351.75 {
		t.Fatalf("unexpected first product content: %#v", first)
	}
	if first.Category != "Bath > shower controls" {
		t.Fatalf("unexpected category: %q", first.Category)
	}
	if !strings.HasPrefix(first.URL, "https://www.qualitybath.com/") || !strings.HasPrefix(first.ImageURL, "https://qb-res.cloudinary.com/") {
		t.Fatalf("unexpected URL/image: %#v", first)
	}
}

func TestSignatureHardwareProductsFromHTMLExtractsSuggestionRows(t *testing.T) {
	html := `<div class="search-suggest-product" data-rfk-pid="447694">
		<a class="suggestion-link" href="https://www.signaturehardware.com/thermostatic--rough-in-shower-valve/948545.html" aria-label="Thermostatic Rough-In Shower Valve">
			<img class="suggestion-img" alt="Thermostatic Rough-In Shower Valve" src="http://images.signaturehardware.com/i/signaturehdwr/447694-thermo-valve-front-MV70.jpg" />
		</a>
		<div class="tile-body">
			<div class="pdp-link"><a class="link" href="https://www.signaturehardware.com/thermostatic--rough-in-shower-valve/948545.html">Thermostatic  Rough-In Shower Valve</a></div>
			<div class="price"><span class="sales"><span class="value" content="299.00"><span aria-hidden="true">$299<sup>00</sup></span></span></span></div>
		</div>
	</div>
	<div class="search-suggest-product" data-rfk-pid="346236">
		<a class="suggestion-link" href="/exposed-2-valve-shower---polished-chrome/346236.html" aria-label="Exposed 2 Valve Shower - Polished Chrome">
			<img class="suggestion-img" alt="Exposed 2 Valve Shower - Polished Chrome" src="http://images.signaturehardware.com/i/signaturehdwr/346236-Bostonian-exposed-pipe-shower-CP-Beauty10.jpg" />
		</a>
		<div class="tile-body">
			<div class="pdp-link"><a class="link" href="/exposed-2-valve-shower---polished-chrome/346236.html">Exposed 2 Valve Shower - Polished Chrome</a></div>
			<div class="price"><span aria-hidden="true">$209<sup>00</sup></span></div>
		</div>
	</div>`

	products := signatureHardwareProductsFromHTML(html, 5)
	if len(products) != 2 {
		t.Fatalf("expected two products, got %#v", products)
	}
	first := products[0]
	if first.Source != "signature-hardware" || first.Brand != "Signature Hardware" {
		t.Fatalf("unexpected source/brand: %#v", first)
	}
	if first.ID != "948545" || first.Title != "Thermostatic Rough-In Shower Valve" {
		t.Fatalf("unexpected normalized product: %#v", first)
	}
	if first.PriceMin != 299.00 || first.URL != "https://www.signaturehardware.com/thermostatic--rough-in-shower-valve/948545.html" {
		t.Fatalf("unexpected price/url: %#v", first)
	}
	if first.ImageURL != "https://images.signaturehardware.com/i/signaturehdwr/447694-thermo-valve-front-MV70.jpg" {
		t.Fatalf("unexpected image URL: %q", first.ImageURL)
	}
	second := products[1]
	if second.ID != "346236" || second.PriceMin != 209.00 {
		t.Fatalf("expected relative URL product with sup price parsed, got %#v", second)
	}
	if second.URL != "https://www.signaturehardware.com/exposed-2-valve-shower---polished-chrome/346236.html" {
		t.Fatalf("unexpected relative URL normalization: %q", second.URL)
	}
}

func TestNormalizeShopifySuggestProductUsesBodyMPNAndPDFs(t *testing.T) {
	product := normalizeShopifySuggestProduct(map[string]any{
		"id":                   927410947,
		"title":                `Newport Brass 1-684 3/4" Thermostatic Valve`,
		"vendor":               "Newport Brass",
		"type":                 "Thermostatic Valves",
		"url":                  "/products/newport-brass-1-684",
		"image":                "https://cdn.shopify.com/products/1-684.jpg",
		"price":                255.50,
		"compare_at_price_min": 365.00,
		"body": `Manufacturer's Part Number(s): 1-684<br>
			<a href="https://plumbtile.info/pdf/npb/NSP-B-SPEC_1-684%20Rev%20G.pdf">Spec Sheet</a>`,
	}, "plumbtile", "https://plumbtile.com")

	if product.ID != "1-684" {
		t.Fatalf("expected body MPN, got %#v", product)
	}
	if product.URL != "https://plumbtile.com/products/newport-brass-1-684" {
		t.Fatalf("unexpected URL: %q", product.URL)
	}
	if !product.OnSale || product.PriceMin != 255.50 || product.RegularPriceMin != 365.00 {
		t.Fatalf("expected sale pricing, got %#v", product)
	}
	if !strings.Contains(product.Description, "spec: https://plumbtile.info/pdf/npb/NSP-B-SPEC_1-684%20Rev%20G.pdf") {
		t.Fatalf("expected spec PDF in description, got %q", product.Description)
	}
}

func TestNormalizeShopifySuggestProductUsesBodySKUBeforeShopifyID(t *testing.T) {
	product := normalizeShopifySuggestProduct(map[string]any{
		"id":     1122334455,
		"title":  "Delta Vero Thermostatic Shower Valve Dual Control Trim",
		"vendor": "Delta",
		"url":    "/products/delta-vero-modern-chrome-thermostatic-shower-valve-dual-control-trim-521930",
		"price":  367.05,
		"body":   "SKU: D970V, MPN: T17494-SS-I , R10000-UNBX, UPC: '703610944404'",
	}, "faucetlist", "https://faucetlist.com")

	if product.ID != "D970V" {
		t.Fatalf("expected retailer SKU before Shopify ID, got %#v", product)
	}
}

func TestSearchShopifyAllAllowsEmptySuccessfulStoresWithPartialFailure(t *testing.T) {
	oldStores := shopifyStores
	shopifyStores = []shopifyStore{
		{Domain: "empty-store", Name: "Empty", Token: "token"},
		{Domain: "failing-store", Name: "Failing", Token: "token"},
	}
	defer func() { shopifyStores = oldStores }()

	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		status := http.StatusOK
		body := `{"data":{"search":{"edges":[]}}}`
		if req.URL.Host == "failing-store.myshopify.com" {
			status = http.StatusBadGateway
			body = `{"error":"temporary"}`
		}
		return &http.Response{
			StatusCode: status,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    req,
		}, nil
	})}

	products, err := searchShopifyAll(context.Background(), client, "definitely no rows", 5)
	if err != nil {
		t.Fatalf("expected partial failure with empty successful store to return empty results, got error: %v", err)
	}
	if len(products) != 0 {
		t.Fatalf("expected no products, got %#v", products)
	}
}

func TestFanoutSearchSourcesTreatEmptyProductSetsAsSuccess(t *testing.T) {
	qualityBathEmpty := `<script>window.__INITIAL_QUERIES__ = JSON.parse('{"mutations":[],"queries":[{"state":{"data":{"products":[]}},"queryKey":["search.getFull",{"id":"no matches","pageType":"search"}]}]}');</script>`
	tests := []struct {
		name   string
		body   string
		search func(context.Context, *http.Client, string, int) ([]NormalizedProduct, error)
	}{
		{name: "best-buy", body: `<html><body>No matching products</body></html>`, search: searchBestBuy},
		{name: "abt", body: `<html><body>No matching products</body></html>`, search: searchAbt},
		{name: "qualitybath", body: qualityBathEmpty, search: searchQualityBath},
		{name: "pc-richard", body: `<html><body>No matching products</body></html>`, search: searchPCRichard},
		{name: "lighting-new-york", body: `<html><body>No matching products</body></html>`, search: searchLightingNewYork},
		{name: "lightology", body: `<html><body>No matching products</body></html>`, search: searchLightology},
		{name: "kbauthority", body: `{"results":""}`, search: searchKBAuthority},
		{name: "vintage-tub", body: `{"results":[]}`, search: searchVintageTub},
		{name: "signature-hardware", body: `<html><body>No matching products</body></html>`, search: searchSignatureHardware},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(tt.body)),
					Request:    req,
				}, nil
			})}
			products, err := tt.search(context.Background(), client, "pendant light", 5)
			if err != nil {
				t.Fatalf("expected empty results to be a successful search, got error: %v", err)
			}
			if len(products) != 0 {
				t.Fatalf("expected no products, got %#v", products)
			}
		})
	}
}

func TestNormalizeShopifySuggestProductExtractsLightingSKUFromHandle(t *testing.T) {
	tests := []struct {
		name   string
		handle string
		want   string
	}{
		{
			name:   "satco compact model",
			handle: "satco-s11296-satco-starfish-gimbal-6-inch-smart-canless-led-recessed-light",
			want:   "S11296",
		},
		{
			name:   "rab compact model",
			handle: "rab-lighting-eclps6b-6-inch-eclipse-retrofit-downlight-with-nightlight",
			want:   "ECLPS6B",
		},
		{
			name:   "american lighting multipart model",
			handle: "american-lighting-mlink-120-30-36-microlink-36-inch-led-under-cabinet-lighting",
			want:   "MLINK-120-30-36",
		},
		{
			name:   "sylvania model before dimension",
			handle: "sylvania-sylv-62440-10-inch-under-cabinet-led-light-with-tilting-lenses-450-lumens",
			want:   "SYLV-62440",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			product := normalizeShopifySuggestProduct(map[string]any{
				"id":     998877,
				"title":  "Installed lighting fixture",
				"vendor": "Lighting Vendor",
				"handle": tt.handle,
				"url":    "/products/" + tt.handle,
				"price":  74.95,
			}, "bees-lighting", "https://www.beeslighting.com")
			if product.ID != tt.want {
				t.Fatalf("expected %s, got %#v", tt.want, product)
			}
		})
	}
}

func TestSkipShopifySuggestProductFiltersBeesPowerStripsForLightQuery(t *testing.T) {
	powerStrip := NormalizedProduct{Source: "bees-lighting", Title: "Under Cabinet Power Strip", Category: "Under Cabinet Power Strips"}
	light := NormalizedProduct{Source: "bees-lighting", Title: "Under Cabinet Light", Category: "Under Cabinet Lights"}

	if !skipShopifySuggestProduct("bees-lighting", "under cabinet light", powerStrip) {
		t.Fatalf("expected Bees Lighting power strips to be skipped for light query")
	}
	if skipShopifySuggestProduct("bees-lighting", "under cabinet light", light) {
		t.Fatalf("expected Bees Lighting light products to remain eligible")
	}
}

func TestNormalizeShopifySuggestProductUsesModernBathroomHandleForSelection(t *testing.T) {
	product := normalizeShopifySuggestProduct(map[string]any{
		"id":                   1234567890,
		"title":                "Beckett Bathroom Vanity Cabinet 54 inch Single Sink",
		"vendor":               "Wyndham Collection",
		"type":                 "Vanities",
		"handle":               "beckett-bathroom-vanity-cabinet-54-inch-single-sink",
		"url":                  "/products/beckett-bathroom-vanity-cabinet-54-inch-single-sink",
		"price":                "1119.20",
		"compare_at_price_min": "1399.00",
	}, "modern-bathroom", "https://www.modernbathroom.com")

	if product.ID != "BECKETT-VANITY-54-SINGLE-SINK" {
		t.Fatalf("expected compact handle selection ID, got %#v", product)
	}
	if product.PriceMin != 1119.20 || product.RegularPriceMin != 1399.00 || !product.OnSale {
		t.Fatalf("expected Modern Bathroom pricing, got %#v", product)
	}
}

func TestNormalizeVintageTubProductUsesSearchspringFields(t *testing.T) {
	product := normalizeVintageTubProduct(map[string]any{
		"sku":         "RMAS-VSQ2F-S",
		"name":        "Standard Valve - 2 Square Handles",
		"brand":       "Randolph Morris",
		"price":       "149.99",
		"url":         "https://www.vintagetub.com/randolph-morris-standard-valve-2-square-handles-rmas-vsq2f-s.html",
		"imageUrl":    "https://www.vintagetub.com/media/catalog/product/rmas-vsq2f.jpg",
		"description": "Solid brass construction|&lt;a href=&quot;/shipping&quot;&gt;Eligible for Free Shipping&lt;/a&gt;",
		"category_hierarchy": []any{
			"Root Catalog&gt;Default Category&gt;Bathroom",
			"Root Catalog&gt;Default Category&gt;Showers&gt;Shower Accessories",
		},
	})

	if product.Source != "vintage-tub" {
		t.Fatalf("unexpected source: %#v", product)
	}
	if product.ID != "RMAS-VSQ2F-S" || product.Brand != "Randolph Morris" {
		t.Fatalf("expected SKU and brand, got %#v", product)
	}
	if product.Title != "Standard Valve - 2 Square Handles" {
		t.Fatalf("unexpected title: %q", product.Title)
	}
	if product.PriceMin != 149.99 || product.PriceMax != 149.99 {
		t.Fatalf("expected Searchspring price, got %#v", product)
	}
	if product.Category != "Shower Accessories" {
		t.Fatalf("unexpected category: %q", product.Category)
	}
	if !strings.Contains(product.Description, "Solid brass construction; Eligible for Free Shipping") {
		t.Fatalf("unexpected description: %q", product.Description)
	}
}

func TestSkipShopifySuggestProductFiltersModernBathroomVanityAccessories(t *testing.T) {
	sidesplash := NormalizedProduct{
		Source:   "modern-bathroom",
		Title:    "Bathroom Vanity Sidesplash",
		Category: "Countertops",
	}
	vanity := NormalizedProduct{
		Source:   "modern-bathroom",
		Title:    "Beckett Bathroom Vanity Cabinet 54 inch Single Sink",
		Category: "Vanities",
	}

	if !skipShopifySuggestProduct("modern-bathroom", "bathroom vanity", sidesplash) {
		t.Fatalf("expected Modern Bathroom sidesplash row to be skipped for vanity query")
	}
	if skipShopifySuggestProduct("modern-bathroom", "bathroom vanity", vanity) {
		t.Fatalf("expected Modern Bathroom vanity row to remain eligible")
	}
}

func TestNormalizeLightingNewYorkProductUsesEmbeddedGTMData(t *testing.T) {
	product := normalizeLightingNewYorkProduct(`<div class="product" data-pid="341P18-65" data-gtmdata="{&quot;id&quot;:&quot;341P18-65&quot;,&quot;item_id&quot;:&quot;2448389&quot;,&quot;item_name&quot;:&quot;Z-Lite 341P18-RB Prescott 4 Light 18 inch Rubbed Brass Pendant Ceiling Light&quot;,&quot;item_sku&quot;:&quot;341P18-RB&quot;,&quot;name&quot;:&quot;Prescott 4 Light 18.00 inch Pendant&quot;,&quot;category&quot;:&quot;Pendants&quot;,&quot;price&quot;:561,&quot;imageURL&quot;:&quot;https://lightingnewyork.com/on/demandware.static/-/Sites-master-catalog-lny/default/product.jpg&quot;,&quot;brand&quot;:&quot;Z-Lite&quot;,&quot;compareAtPrice&quot;:&quot;660.00&quot;,&quot;productURL&quot;:&quot;https://lightingnewyork.com/product/lighting/ceiling-lights/pendants/z-lite-lighting-prescott-pendant-341p18-rb/341P18-65.html&quot;}"></div>`)

	if product.Source != "lighting-new-york" {
		t.Fatalf("unexpected source: %#v", product)
	}
	if product.ID != "341P18-RB" || product.Brand != "Z-Lite" {
		t.Fatalf("expected SKU and brand, got %#v", product)
	}
	if product.Title != "Prescott 4 Light 18.00 inch Pendant" || product.Category != "Pendants" {
		t.Fatalf("unexpected title/category: %#v", product)
	}
	if product.PriceMin != 561 || product.RegularPriceMin != 660 || !product.OnSale {
		t.Fatalf("expected sale pricing, got %#v", product)
	}
	if !strings.Contains(product.URL, "/z-lite-lighting-prescott-pendant-341p18-rb/341P18-65.html") {
		t.Fatalf("unexpected product URL: %q", product.URL)
	}
}

func TestNormalizeLightologyProductUsesGTMListItem(t *testing.T) {
	product := normalizeLightologyProduct(`<div class="col-md-4 prod-border" data-gtm_list_item="{&quot;item_id&quot;:&quot;BOLA SPH 4 CRM&quot;,&quot;item_name&quot;:&quot;Bola Sphere Pendant&quot;,&quot;item_brand&quot;:&quot;Pablo&quot;,&quot;item_category&quot;:&quot;Chandeliers &amp; Pendants&quot;,&quot;item_category2&quot;:&quot;Pendants&quot;,&quot;item_main_id&quot;:&quot;874290&quot;,&quot;item_variant&quot;:&quot;874290&quot;,&quot;price&quot;:&quot;340.00&quot;,&quot;index&quot;:&quot;1&quot;,&quot;item_list_id&quot;:&quot;cat-106&quot;}">
		<a href="/index.php?module=prod_detail&amp;prod_id=874290&amp;cat_id=106"><img src="https://www.lightology.com/img/prod/370/874290.jpg"></a>
		<span class=" bigprice">$340 - $1,325</span>
	</div>`)

	if product.Source != "lightology" {
		t.Fatalf("unexpected source: %#v", product)
	}
	if product.ID != "BOLA-SPH-4-CRM" || product.Brand != "Pablo" {
		t.Fatalf("expected SKU and brand, got %#v", product)
	}
	if product.Title != "Bola Sphere Pendant" || product.Category != "Chandeliers & Pendants > Pendants" {
		t.Fatalf("unexpected title/category: %#v", product)
	}
	if product.PriceMin != 340 || product.PriceMax != 1325 {
		t.Fatalf("expected price range, got %#v", product)
	}
	if !strings.Contains(product.URL, "prod_id=874290") {
		t.Fatalf("unexpected product URL: %q", product.URL)
	}
	if product.ImageURL != "https://www.lightology.com/img/prod/370/874290.jpg" {
		t.Fatalf("unexpected image URL: %q", product.ImageURL)
	}
}

func TestLightologyRouteForCeilingFan(t *testing.T) {
	route, err := lightologyRouteForQuery("ceiling fan")
	if err != nil {
		t.Fatalf("expected ceiling fan route: %v", err)
	}
	if !strings.Contains(route, "cat_id=49") {
		t.Fatalf("expected Lightology ceiling fan category, got %q", route)
	}
}

func TestNormalizeKBAuthorityProductUsesSearchspringFragment(t *testing.T) {
	product := normalizeKBAuthorityProduct(`<div class="item">
		<p class="image"><a href="https://www.kbauthority.com/ove-decors-15vva-berk48-181yj-gabi-48-inch-free-standing-single-sink-bathroom-vanity-in-warm-walnut-with-countertop.html"><img src="//www.kbauthority.com/images/W/555/15VVA-BERK48-174YJ_Ove_Decors_Rustic_Ash.jpg" /></a></p>
		<p class="name"><a href="https://www.kbauthority.com/ove-decors-15vva-berk48-181yj-gabi-48-inch-free-standing-single-sink-bathroom-vanity-in-warm-walnut-with-countertop.html">OVE DECORS 15VVA-BERK48-YJ GABI 48 INCH FREESTANDING SINGLE SINK BATHROOM VANITY WITH WHITE ENGINEERED MARBLE COUNTERTOP</a></p>
		<p class="price">1379.54</p>
	</div>`)

	if product.Source != "kbauthority" {
		t.Fatalf("unexpected source: %#v", product)
	}
	if product.ID != "15VVA-BERK48-YJ" || product.Brand != "Ove Decors" {
		t.Fatalf("expected SKU and brand, got %#v", product)
	}
	if !strings.Contains(product.URL, "15vva-berk48-181yj") {
		t.Fatalf("unexpected product URL: %q", product.URL)
	}
	if product.ImageURL != "https://www.kbauthority.com/images/W/555/15VVA-BERK48-174YJ_Ove_Decors_Rustic_Ash.jpg" {
		t.Fatalf("unexpected image URL: %q", product.ImageURL)
	}
	if product.PriceMin != 1379.54 || product.PriceMax != 1379.54 {
		t.Fatalf("expected price, got %#v", product)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
