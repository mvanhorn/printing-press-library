package cli

import (
	"errors"
	"strings"
	"testing"
)

func TestParseSearchHTML_MalformedTileImpressionReturnsExtractionError(t *testing.T) {
	t.Parallel()

	body := []byte(`<div class="product-tile" data-product-tile-impression='{"id":'></div>`)
	_, err := parseSearchHTML("leite", 0, body)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var extraction *extractionError
	if !errors.As(err, &extraction) {
		t.Fatalf("expected extractionError, got %T (%v)", err, err)
	}
	if extraction.Operation != "search" || extraction.Reason != "malformed product tile impression" {
		t.Fatalf("unexpected extraction error: %+v", extraction)
	}
}

func TestParseProductHTML_NormalizesStructuredDetail(t *testing.T) {
	t.Parallel()

	body := []byte(strings.Join([]string{
		`<script type="application/ld+json">`,
		`{"name":"Leite UHT Meio Gordo Mimosa","mpn":"7745833","sku":"7745833","image":["https://www.continente.pt/image.jpg"],"brand":{"name":"Mimosa"},"offers":{"priceCurrency":"EUR","price":"15.84","availability":"https://schema.org/InStock"},"aggregateRating":{"ratingCount":12,"ratingValue":4.6}}`,
		`</script>`,
		`<div data-product-detail-impression="{&quot;currency&quot;:&quot;EUR&quot;,&quot;value&quot;:15.84,&quot;items&quot;:[{&quot;item_id&quot;:&quot;7745833&quot;,&quot;item_brand&quot;:&quot;Mimosa&quot;,&quot;item_category&quot;:&quot;Leite&quot;,&quot;item_category2&quot;:&quot;Meio Gordo&quot;,&quot;item_category3&quot;:&quot;&quot;,&quot;price&quot;:15.84,&quot;discount&quot;:2.16,&quot;pre_discount_price&quot;:18}]}"></div>`,
		`<div class="pdp-product-tile__price">`,
		`<span class="pvpr-info">PVPR</span> 18,00&euro;`,
		`<div class="ct-product-tile-badge-label"><span>Mais de</span></div>`,
		`<div class="ct-quantifier-container"><span class="ct-product-tile-badge-value--pvpr">10</span><span class="ct-product-tile-badge-value--pvpr-quantifier">%</span></div>`,
		`<img title="Exclusivo Online" src="/images/badges/online.svg" data-src="/images/badges/online.svg">`,
		`<span class="ct-pdp--unit col-pdp--unit">emb. 18 x 1 lt</span>`,
		`<div class="pwc-tile--price-secondary">0,88&euro;/lt</div>`,
		`<img title="Descarregar na App Store" src="/images/footer/apps/applestore.png">`,
		`</div>`,
		`<div data-url="/on/demandware.store/Sites-continente-Site/default/Product-ProductNutritionalInfoTab"></div>`,
	}, ""))

	product, err := parseProductHTML("leite-uht-meio-gordo-mimosa-mimosa-7745833", body)
	if err != nil {
		t.Fatalf("parseProductHTML returned error: %v", err)
	}
	if product.ID != "7745833" || product.Brand != "Mimosa" || product.Currency != "EUR" {
		t.Fatalf("unexpected product identity: %+v", product)
	}
	if product.Price != 15.84 {
		t.Fatalf("price = %v, want 15.84", product.Price)
	}
	if product.OriginalPrice != 18 {
		t.Fatalf("original price = %v, want 18", product.OriginalPrice)
	}
	if product.DiscountAmount != 2.16 {
		t.Fatalf("discount amount = %v, want 2.16", product.DiscountAmount)
	}
	if product.SavingsPercent != 12 {
		t.Fatalf("savings percent = %v, want 12", product.SavingsPercent)
	}
	if product.UnitPrice != 0.88 || product.UnitLabel != "lt" {
		t.Fatalf("unit price = %v/%q, want 0.88/lt", product.UnitPrice, product.UnitLabel)
	}
	if !product.HasPromotion || !product.HasDiscount {
		t.Fatalf("promo/discount flags = %+v", product)
	}
	if product.PackLabel != "emb. 18 x 1 lt" {
		t.Fatalf("pack label = %q, want emb. 18 x 1 lt", product.PackLabel)
	}
	if got, want := strings.Join(product.PromotionText, "|"), "Mais de 10 %|Exclusivo Online"; got != want {
		t.Fatalf("promotion text = %q, want %q", got, want)
	}
	if product.Availability != "InStock" {
		t.Fatalf("availability = %q, want InStock", product.Availability)
	}
	if got, want := len(product.Categories), 2; got != want {
		t.Fatalf("categories len = %d, want %d (%v)", got, want, product.Categories)
	}
}

func TestParseNutritionHTML_ExtractsPer100gAndPerServing(t *testing.T) {
	t.Parallel()

	body := []byte(strings.Join([]string{
		`<section class="nutriInfo-group">`,
		`<div class="serving-size"><p class="mb-0">Porção:</p><p class="mb-20">100</p></div>`,
		`<div class="serving-size--uom"><p class="mb-0">Unidade de Medida (Porção):</p><p class="mb-20">(GRM) Grama</p></div>`,
		`<div class="nutrients-row row"><div class="nutriInfo-details col-4 col-sm nutrients-cell">energia</div><div class="nutriInfo-details col-4 col-sm nutrients-cell">528,0</div><div class="nutriInfo-details col-4 col-sm nutrients-cell">(E14) Quilocaloria</div></div>`,
		`<div class="nutrients-row row"><div class="nutriInfo-details col-4 col-sm nutrients-cell">lípidos</div><div class="nutriInfo-details col-4 col-sm nutrients-cell">28,0</div><div class="nutriInfo-details col-4 col-sm nutrients-cell">(GRM) Grama</div></div>`,
		`<div class="nutrients-row row"><div class="nutriInfo-details col-4 col-sm nutrients-cell">hidratos de carbono > açúcares</div><div class="nutriInfo-details col-4 col-sm nutrients-cell">60,0</div><div class="nutriInfo-details col-4 col-sm nutrients-cell">(GRM) Grama</div></div>`,
		`</section>`,
		`<section class="nutriInfo-group">`,
		`<div class="serving-size"><p class="mb-0">Porção:</p><p class="mb-20">25</p></div>`,
		`<div class="serving-size--uom"><p class="mb-0">Unidade de Medida (Porção):</p><p class="mb-20">(GRM) Grama</p></div>`,
		`<div class="serving-size--household"><p class="mb-0">Tamanho da Porção:</p><p class="mb-20">25 g = 3 triângulos.</p></div>`,
		`<div class="nutrients-row row"><div class="nutriInfo-details col-4 col-sm nutrients-cell">energia</div><div class="nutriInfo-details col-4 col-sm nutrients-cell">132,0</div><div class="nutriInfo-details col-4 col-sm nutrients-cell">(E14) Quilocaloria</div></div>`,
		`<div class="nutrients-row row"><div class="nutriInfo-details col-4 col-sm nutrients-cell">proteínas</div><div class="nutriInfo-details col-4 col-sm nutrients-cell">1,4</div><div class="nutriInfo-details col-4 col-sm nutrients-cell">(GRM) Grama</div></div>`,
		`</section>`,
	}, ""))

	profile, err := parseNutritionHTML(body)
	if err != nil {
		t.Fatalf("parseNutritionHTML returned error: %v", err)
	}
	if profile.Per100g == nil || profile.Per100g.EnergyKCal != 528 || profile.Per100g.SugarsG != 60 {
		t.Fatalf("unexpected per_100g: %+v", profile.Per100g)
	}
	if profile.PerServing == nil || profile.PerServing.ServingAmount != 25 || profile.PerServing.ProteinG != 1.4 {
		t.Fatalf("unexpected per_serving: %+v", profile.PerServing)
	}
}

func TestProductResponseHumanRow_IncludesNutritionStatus(t *testing.T) {
	t.Parallel()

	row := productResponseHumanRow(productResponse{
		ID:              "2027778",
		Name:            "Tablete de Chocolate de Leite Crunch",
		Brand:           "Crunch",
		NutritionStatus: "not_provided",
	})
	if got := row["nutrition_status"]; got != "not_provided" {
		t.Fatalf("nutrition_status = %v, want not_provided", got)
	}
}
