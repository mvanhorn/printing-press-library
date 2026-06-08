package normalize

import "testing"

func TestProductFromStorefront_NormalizesCoreFields(t *testing.T) {
	t.Parallel()

	product, err := ProductFromStorefront(StorefrontProductRecord{
		ID:             "7745833",
		Name:           "Leite UHT Meio Gordo Mimosa",
		Brand:          "Mimosa",
		Category:       "Leite/Meio Gordo",
		URL:            "https://www.continente.pt/produto/leite-uht-meio-gordo-mimosa-mimosa-7745833.html",
		Image:          "https://www.continente.pt/image.jpg",
		DisplayPrice:   15.84,
		OriginalPrice:  18.00,
		DiscountAmount: 2.16,
		UnitPrice:      0.88,
		UnitLabel:      "lt",
		PackLabel:      "emb. 18 x 1 lt",
		PromotionText:  []string{"Mais de 10 %", "Exclusivo Online"},
		Currency:       "EUR",
	})
	if err != nil {
		t.Fatalf("ProductFromStorefront returned error: %v", err)
	}
	if product.ID != "7745833" || product.Name == "" || product.Brand != "Mimosa" {
		t.Fatalf("unexpected normalized identity: %+v", product)
	}
	if got, want := len(product.Categories), 2; got != want {
		t.Fatalf("categories len = %d, want %d (%v)", got, want, product.Categories)
	}
	if product.Price.DisplayAmount != 15.84 {
		t.Fatalf("display price = %v, want 15.84", product.Price.DisplayAmount)
	}
	if product.Price.Effective == nil || *product.Price.Effective != 15.84 {
		t.Fatalf("effective price = %v, want 15.84", product.Price.Effective)
	}
	if product.Price.OriginalAmount == nil || *product.Price.OriginalAmount != 18 {
		t.Fatalf("original amount = %v, want 18.00", product.Price.OriginalAmount)
	}
	if product.Price.DiscountAmount == nil || *product.Price.DiscountAmount != 2.16 {
		t.Fatalf("discount amount = %v, want 2.16", product.Price.DiscountAmount)
	}
	if product.Price.UnitAmount == nil || *product.Price.UnitAmount != 0.88 || product.Price.UnitLabel != "lt" {
		t.Fatalf("unit price = %v/%s, want 0.88/lt", product.Price.UnitAmount, product.Price.UnitLabel)
	}
	if product.Price.SavingsPercent == nil || *product.Price.SavingsPercent != 12 {
		t.Fatalf("savings percent = %v, want 12", product.Price.SavingsPercent)
	}
	if !product.Price.HasPromotion || !product.Price.HasDiscount {
		t.Fatalf("promo/discount flags = %+v", product.Price)
	}
	if product.Price.PackLabel != "emb. 18 x 1 lt" {
		t.Fatalf("pack label = %q", product.Price.PackLabel)
	}
	if len(product.Price.PromotionText) != 2 {
		t.Fatalf("promotion text = %v", product.Price.PromotionText)
	}
	if len(product.MissingFields) != 0 {
		t.Fatalf("missing fields = %v, want none", product.MissingFields)
	}
}

func TestProductFromStorefront_MarksMissingOptionalFields(t *testing.T) {
	t.Parallel()

	product, err := ProductFromStorefront(StorefrontProductRecord{
		ID:   "123",
		Name: "Produto sem marca",
	})
	if err != nil {
		t.Fatalf("ProductFromStorefront returned error: %v", err)
	}
	if got, want := len(product.MissingFields), 2; got != want {
		t.Fatalf("missing fields len = %d, want %d (%v)", got, want, product.MissingFields)
	}
	if product.MissingFields[0] != "brand" || product.MissingFields[1] != "image" {
		t.Fatalf("missing fields = %v, want [brand image]", product.MissingFields)
	}
}
