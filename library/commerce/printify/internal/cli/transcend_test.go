package cli

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildPersonalizationAuditReportsDocumentedFields(t *testing.T) {
	product := sampleProduct()

	rows := buildPersonalizationAudit(product)

	if len(rows) != 1 {
		t.Fatalf("expected 1 audit row, got %d", len(rows))
	}
	if !rows[0].Supported {
		t.Fatalf("expected documented image/text fields to be supported")
	}
	if rows[0].InputText != "Name" || rows[0].FontFamily != "Inter" {
		t.Fatalf("unexpected personalization row: %#v", rows[0])
	}
}

func TestBuildPlacementMatrixJoinsUploadNames(t *testing.T) {
	product := sampleProduct()
	uploads := []ppJSONObj{{"id": "img_1", "file_name": "front.png"}}

	rows := buildPlacementMatrix(product, uploads)

	if len(rows) != 2 {
		t.Fatalf("expected one row per variant, got %d", len(rows))
	}
	if rows[0].UploadFileName != "front.png" || rows[0].Scale != 1.2 {
		t.Fatalf("unexpected placement row: %#v", rows[0])
	}
}

func TestPlacementMatrixReturnsExplicitUploadsFileError(t *testing.T) {
	dir := t.TempDir()
	productPath := filepath.Join(dir, "product.json")
	rawProduct, err := json.Marshal(sampleProduct())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(productPath, rawProduct, 0o644); err != nil {
		t.Fatal(err)
	}

	missingUploadsPath := filepath.Join(dir, "missing-uploads.json")
	cmd := newPlacementMatrixCmd(&rootFlags{asJSON: true})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--product-file", productPath, "--uploads-file", missingUploadsPath})

	err = cmd.Execute()

	if err == nil || !strings.Contains(err.Error(), "missing-uploads.json") {
		t.Fatalf("expected uploads load error, got %v", err)
	}
}

func TestBuildProductDriftDetectsChangedTitle(t *testing.T) {
	expected := ppJSONObj{"title": "Original", "blueprint_id": float64(384)}
	actual := ppJSONObj{"title": "Changed", "blueprint_id": float64(384)}

	rows := buildProductDrift(expected, actual)

	foundTitleDrift := false
	for _, row := range rows {
		if row.Path == "title" && row.Status == "drift" {
			foundTitleDrift = true
		}
	}
	if !foundTitleDrift {
		t.Fatalf("expected title drift in %#v", rows)
	}
}

func TestBuildPersonalizationBatchWritesExpandedManifest(t *testing.T) {
	dir := t.TempDir()
	templatePath := filepath.Join(dir, "template.json")
	csvPath := filepath.Join(dir, "rows.csv")
	outDir := filepath.Join(dir, "out")
	if err := os.WriteFile(templatePath, []byte(`{"title":"{{title}}","print_areas":[{"placeholders":[{"images":[{"id":"{{image_id}}","input_text":"{{text}}"}]}]}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(csvPath, []byte("title,image_id,text\nMug,img_1,Sam\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	rows, err := buildPersonalizationBatch(templatePath, csvPath, outDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Title != "Mug" || !rows[0].TextUsed {
		t.Fatalf("unexpected batch rows: %#v", rows)
	}
	data, err := os.ReadFile(rows[0].Output)
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest["title"] != "Mug" {
		t.Fatalf("template token was not replaced: %#v", manifest)
	}
}

func TestBuildPersonalizationBatchUsesUniqueOutputNames(t *testing.T) {
	dir := t.TempDir()
	templatePath := filepath.Join(dir, "template.json")
	csvPath := filepath.Join(dir, "rows.csv")
	outDir := filepath.Join(dir, "out")
	if err := os.WriteFile(templatePath, []byte(`{"title":"{{title}}","note":"{{text}}"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(csvPath, []byte("title,text\nCustom Mug,First\nCustom Mug,Second\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	rows, err := buildPersonalizationBatch(templatePath, csvPath, outDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected two batch rows, got %d", len(rows))
	}
	if rows[0].Output == rows[1].Output {
		t.Fatalf("expected unique output paths, got %q", rows[0].Output)
	}
	if filepath.Base(rows[0].Output) != "custom-mug.json" || filepath.Base(rows[1].Output) != "custom-mug-2.json" {
		t.Fatalf("unexpected output paths: %#v", rows)
	}
	for index, expectedNote := range []string{"First", "Second"} {
		data, err := os.ReadFile(rows[index].Output)
		if err != nil {
			t.Fatal(err)
		}
		var manifest map[string]any
		if err := json.Unmarshal(data, &manifest); err != nil {
			t.Fatal(err)
		}
		if manifest["note"] != expectedNote {
			t.Fatalf("unexpected manifest %d: %#v", index+1, manifest)
		}
	}
}

func TestBuildPersonalizationBatchFallsBackWhenSlugIsEmpty(t *testing.T) {
	dir := t.TempDir()
	templatePath := filepath.Join(dir, "template.json")
	csvPath := filepath.Join(dir, "rows.csv")
	outDir := filepath.Join(dir, "out")
	if err := os.WriteFile(templatePath, []byte(`{"title":"{{title}}"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(csvPath, []byte("title\n!!!\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	rows, err := buildPersonalizationBatch(templatePath, csvPath, outDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected one batch row, got %d", len(rows))
	}
	if got := filepath.Base(rows[0].Output); got != "manifest-001.json" {
		t.Fatalf("output path = %q, want manifest-001.json", got)
	}
}

func TestBuildCatalogMarginMatrixComputesMargin(t *testing.T) {
	variants := []ppJSONObj{
		{"id": "1", "title": "S", "cost": float64(1200)},
		{"id": "2", "title": "XS", "cost": float64(99)},
	}
	shipping := []ppJSONObj{{"first_item": float64(500), "additional_items": float64(250)}}

	rows := buildCatalogMarginMatrix(variants, shipping, 24.99)

	if len(rows) != 2 {
		t.Fatalf("expected two margin rows, got %d", len(rows))
	}
	if rows[0].Cost != 12 || rows[0].Shipping != 5 || rows[0].EstimatedMargin != 7.99 {
		t.Fatalf("unexpected margin row: %#v", rows[0])
	}
	if rows[1].Cost != 0.99 || rows[1].EstimatedMargin != 19 {
		t.Fatalf("unexpected sub-dollar margin row: %#v", rows[1])
	}
}

func TestCatalogMarginMatrixUsesProfilesEnvelopeAndNestedFirstItemCost(t *testing.T) {
	variants, err := ppDecodeObjects(json.RawMessage(`{"variants":[{"id":"1","title":"S","cost":1200}]}`))
	if err != nil {
		t.Fatal(err)
	}
	shipping, err := ppDecodeObjects(json.RawMessage(`{"profiles":[{"first_item":{"cost":500,"currency":"USD"},"additional_items":{"cost":250,"currency":"USD"}}]}`))
	if err != nil {
		t.Fatal(err)
	}

	rows := buildCatalogMarginMatrix(variants, shipping, 24.99)

	if len(rows) != 1 {
		t.Fatalf("expected one margin row, got %d", len(rows))
	}
	if rows[0].Shipping != 5 || rows[0].EstimatedMargin != 7.99 {
		t.Fatalf("unexpected margin row: %#v", rows[0])
	}
}

func TestCentsToDollarsConvertsSmallCentValues(t *testing.T) {
	if ppRound2(ppCentsToDollars(99)) != 0.99 {
		t.Fatalf("expected 99 cents to convert to $0.99")
	}
	if ppRound2(ppCentsToDollars(5)) != 0.05 {
		t.Fatalf("expected 5 cents to convert to $0.05")
	}
}

func TestLoadStoreObjectsReturnsListErrors(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "empty.db")
	if err := os.WriteFile(dbPath, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := ppLoadStoreObjects(dbPath, []string{"products", "products-json"}, 100)

	if err == nil || !strings.Contains(err.Error(), "load local resources") {
		t.Fatalf("expected local resource load error, got %v", err)
	}
}

func TestBuildAssetReuseFlagsUnusedAndSharedUploads(t *testing.T) {
	products := []ppJSONObj{sampleProduct(), sampleProductWithID("prod_2")}
	uploads := []ppJSONObj{{"id": "img_1", "file_name": "front.png"}, {"id": "unused", "file_name": "unused.png"}}

	rows := buildAssetReuse(products, uploads)

	if len(rows) != 2 {
		t.Fatalf("expected two upload rows, got %d", len(rows))
	}
	if !rows[0].SharedArtwork || rows[0].UseCount != 2 {
		t.Fatalf("expected shared artwork row, got %#v", rows[0])
	}
	if !rows[1].Unused {
		t.Fatalf("expected unused upload row, got %#v", rows[1])
	}
}

func TestBuildFulfillmentRiskFlagsMissingShipment(t *testing.T) {
	orders := []ppJSONObj{{
		"id":     "order_1",
		"status": "pending",
		"line_items": []any{
			map[string]any{"product_id": "prod_1", "variant_id": "101"},
		},
	}}
	products := []ppJSONObj{{"id": "prod_1", "status": "visible"}}

	rows := buildFulfillmentRisk(orders, products)

	if len(rows) != 1 {
		t.Fatalf("expected one risk row, got %d", len(rows))
	}
	if rows[0].Risks[0] != "no shipment records" {
		t.Fatalf("unexpected risks: %#v", rows[0].Risks)
	}
}

func TestBuildFulfillmentRiskFlagsProductState(t *testing.T) {
	orders := []ppJSONObj{{
		"id":        "order_1",
		"status":    "pending",
		"shipments": []any{map[string]any{"carrier": "usps"}},
		"line_items": []any{
			map[string]any{"product_id": "prod_1", "variant_id": "101"},
		},
	}}
	products := []ppJSONObj{{
		"id":        "prod_1",
		"visible":   false,
		"is_locked": true,
		"status":    "unpublished",
	}}

	rows := buildFulfillmentRisk(orders, products)

	if len(rows) != 1 {
		t.Fatalf("expected one risk row, got %d", len(rows))
	}
	expectedRisks := []string{"product hidden", "product locked", "product unpublished"}
	if len(rows[0].Risks) != len(expectedRisks) {
		t.Fatalf("unexpected product state risks: %#v", rows[0].Risks)
	}
	for i, expected := range expectedRisks {
		if rows[0].Risks[i] != expected {
			t.Fatalf("unexpected product state risks: %#v", rows[0].Risks)
		}
	}
}

func TestFulfillmentRiskReturnsProductLoadError(t *testing.T) {
	dir := t.TempDir()
	ordersPath := filepath.Join(dir, "orders.json")
	if err := os.WriteFile(ordersPath, []byte(`[{"id":"order_1","status":"pending","line_items":[{"product_id":"prod_1","variant_id":"101"}]}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := newFulfillmentRiskCmd(&rootFlags{asJSON: true})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--orders-file", ordersPath, "--products-file", filepath.Join(dir, "missing.json")})

	err := cmd.Execute()

	if err == nil || !strings.Contains(err.Error(), "missing.json") {
		t.Fatalf("expected products load error, got %v", err)
	}
}

func sampleProduct() ppJSONObj {
	return sampleProductWithID("prod_1")
}

func sampleProductWithID(id string) ppJSONObj {
	return ppJSONObj{
		"id": id,
		"print_areas": []any{
			map[string]any{
				"variant_ids": []any{"101", "102"},
				"placeholders": []any{
					map[string]any{
						"position": "front",
						"images": []any{
							map[string]any{
								"id":          "img_1",
								"name":        "front art",
								"type":        "text",
								"input_text":  "Name",
								"font_family": "Inter",
								"x":           float64(0.5),
								"y":           float64(0.5),
								"scale":       float64(1.2),
							},
						},
					},
				},
			},
		},
	}
}
