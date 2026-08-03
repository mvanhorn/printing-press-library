package crestronparse

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Skipf("fixture %s unavailable: %v", name, err)
	}
	return b
}

func TestParseSearchResultsFirmware(t *testing.T) {
	page, err := ParseSearchResults(fixture(t, "search_firmware.html"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if page.Count == 0 {
		t.Fatal("expected firmware search rows, got none")
	}
	var sawDMNVX, sawGated bool
	for _, r := range page.Results {
		if r.Title == "" {
			t.Errorf("row with empty title: %+v", r)
		}
		if strings.Contains(r.Title, "DM-NVX") {
			sawDMNVX = true
		}
		if r.Gated {
			sawGated = true
		}
		if r.Type != "" && r.Type != "Firmware" && r.Type != "Software" && r.Type != "Advanced-Support-Tools" {
			t.Errorf("unexpected type %q in a firmware-category search", r.Type)
		}
	}
	if !sawDMNVX {
		t.Error("expected at least one DM-NVX release in the DM-NVX firmware search")
	}
	if !sawGated {
		t.Error("expected firmware rows to be flagged as auth-gated")
	}
}

func TestParseSearchResultsSpecSheets(t *testing.T) {
	page, err := ParseSearchResults(fixture(t, "search_specsheets.html"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if page.Count == 0 {
		t.Fatal("expected spec-sheet rows, got none")
	}
	for _, r := range page.Results {
		if r.Gated {
			t.Errorf("spec sheets are public but %q was flagged gated", r.Title)
		}
		if r.URL != "" && !strings.HasPrefix(r.URL, "/getmedia/") {
			t.Errorf("expected a direct /getmedia/ asset link, got %q", r.URL)
		}
	}
}

// A deliberately mismatching query must not return unrelated rows.
func TestParseSearchResultsNegative(t *testing.T) {
	page, err := ParseSearchResults([]byte(`<html><body><div class="search-results"></div></body></html>`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if page.Count != 0 {
		t.Fatalf("expected zero rows for an empty result page, got %d", page.Count)
	}
	if page.Results == nil {
		t.Fatal("Results must be non-nil so JSON renders [] not null")
	}
}

func TestParseProductTiles(t *testing.T) {
	products, total, err := ParseProductTiles(fixture(t, "product_tiles.html"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if total == 0 {
		t.Error("expected a non-zero productCount, which the sync loop needs as its stop condition")
	}
	if len(products) == 0 {
		t.Fatal("expected product tiles, got none")
	}
	for _, p := range products {
		if p.Model == "" {
			t.Errorf("tile with empty model: %+v", p)
		}
		if p.URL == "" {
			t.Errorf("tile %q has no product URL", p.Model)
		}
	}
	if len(products) > total && total > 0 {
		t.Errorf("parsed %d tiles but productCount is %d", len(products), total)
	}
}

func TestParseProductPage(t *testing.T) {
	p, err := ParseProductPage(fixture(t, "product_page.html"),
		"/Products/Catalog/AV-Over-IP/DM-NVX-AV-Over-IP/Video-Endpoint/DM-NVX-360")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if p.Model != "DM-NVX-360" {
		t.Errorf("model = %q, want DM-NVX-360", p.Model)
	}
	if p.SKU != "DM-NVX-360" {
		t.Errorf("sku = %q, want DM-NVX-360", p.SKU)
	}
	if p.Brand != "Crestron" {
		t.Errorf("brand = %q, want Crestron", p.Brand)
	}
	if p.Description == "" {
		t.Error("expected a description from the JSON-LD block")
	}
	if p.DocumentID == "" {
		t.Error("expected a document id; ResourceHandler.ashx?dID= depends on it")
	}
	if p.Discontinued {
		t.Error("DM-NVX-360 is an active product but was marked discontinued")
	}
}

func TestParseSpecTable(t *testing.T) {
	sections, err := ParseSpecTable(fixture(t, "product_page.html"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(sections) < 5 {
		t.Fatalf("expected several spec sections, got %d", len(sections))
	}
	rows := 0
	for _, s := range sections {
		if s.Name == "" {
			t.Error("spec section with empty name")
		}
		rows += len(s.Rows)
		for _, r := range s.Rows {
			if r.Key == "" || r.Value == "" {
				t.Errorf("section %q has an incomplete row %+v", s.Name, r)
			}
		}
	}
	if rows < 20 {
		t.Errorf("expected a substantial spec table, got %d rows", rows)
	}
}

func TestParseCategoryPage(t *testing.T) {
	c, err := ParseCategoryPage(fixture(t, "category_page.html"),
		"/Products/Catalog/AV-Over-IP/DM-NVX-AV-Over-IP/Video-Endpoint")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if c.DocumentID == "" || c.NodeID == "" {
		t.Fatalf("document/node id missing (%q/%q); the tile endpoint cannot be called without them",
			c.DocumentID, c.NodeID)
	}
	if len(c.Subcategories) == 0 {
		t.Error("expected subcategories with counts")
	}
	var withCount int
	for _, s := range c.Subcategories {
		if s.Count > 0 {
			withCount++
		}
		if strings.Contains(s.Name, "(") {
			t.Errorf("subcategory name %q still contains its count suffix", s.Name)
		}
	}
	if withCount == 0 {
		t.Error("expected at least one subcategory to carry a product count")
	}
}

func TestParseAssets(t *testing.T) {
	assets, err := ParseAssets(fixture(t, "product_assets.html"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(assets) == 0 {
		t.Fatal("expected per-product assets, got none")
	}
	seen := map[string]bool{}
	kinds := map[string]bool{}
	for _, a := range assets {
		if a.Title == "" || a.URL == "" {
			t.Errorf("incomplete asset %+v", a)
		}
		low := strings.ToLower(a.Title)
		if low == "download" || low == "pdf" {
			t.Errorf("bare action anchor %q leaked into the asset list", a.Title)
		}
		if seen[a.URL] {
			t.Errorf("duplicate asset URL %q", a.URL)
		}
		seen[a.URL] = true
		kinds[a.Kind] = true
	}
	if len(kinds) < 2 {
		t.Errorf("expected several asset classes, got %v", kinds)
	}
}

func TestParseFirmwareRelease(t *testing.T) {
	fr, err := ParseFirmwareRelease(fixture(t, "firmware_release.html"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if fr.RequiresAuth {
		t.Skip("fixture was captured unauthenticated")
	}
	if fr.Version == "" {
		t.Error("expected a version")
	}
	if fr.DownloadURL == "" {
		t.Error("expected a /firmware_files/ download link")
	} else if !strings.HasPrefix(fr.DownloadURL, "/firmware_files/") {
		t.Errorf("download url = %q, want a /firmware_files/ path", fr.DownloadURL)
	}
	if fr.ChangeLog == "" && fr.ReleaseNotes == "" {
		t.Error("expected release notes or a change log on an authenticated page")
	}
}

func TestParseFirmwareReleaseDetectsSignIn(t *testing.T) {
	fr, err := ParseFirmwareRelease([]byte(
		`<html><head><title>SignIn [Crestron Electronics, Inc.]</title></head>
		 <body>Crestron Authentication - Sign In</body></html>`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !fr.RequiresAuth {
		t.Fatal("an unauthenticated firmware page must report RequiresAuth so the CLI can tell the user to sign in")
	}
	if fr.DownloadURL != "" {
		t.Error("a sign-in page must not yield a download URL")
	}
}

func TestParseCatalogPaths(t *testing.T) {
	paths, err := ParseCatalogPaths(fixture(t, "sitemap.html"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(paths) < 20 {
		t.Fatalf("expected the full catalog taxonomy, got %d paths", len(paths))
	}
	seen := map[string]bool{}
	for _, p := range paths {
		if !strings.HasPrefix(p, "/Products/Catalog/") {
			t.Errorf("non-catalog path leaked in: %q", p)
		}
		if seen[p] {
			t.Errorf("duplicate path %q", p)
		}
		seen[p] = true
	}
}

func TestSplitReleaseTitle(t *testing.T) {
	cases := []struct {
		title      string
		wantModels []string
		wantVer    string
	}{
		{
			"DM-NVX-384(C)_DM-NVX-385(C) 7.4.0255.22319",
			[]string{"DM-NVX-384(C)", "DM-NVX-385(C)"},
			"7.4.0255.22319",
		},
		{
			"CP4N 2.8006.00322.01",
			[]string{"CP4N"},
			"2.8006.00322.01",
		},
		{
			"TSW-570/TSW-770/TSW-1070/TSS-770/TSS-1070/TS-770/TS-1070 3.0.1234",
			[]string{"TSW-570", "TSW-770", "TSW-1070", "TSS-770", "TSS-1070", "TS-770", "TS-1070"},
			"3.0.1234",
		},
		{
			"DM-NVX-DIR2 5.3.276",
			[]string{"DM-NVX-DIR2"},
			"5.3.276",
		},
	}
	for _, tc := range cases {
		models, ver := SplitReleaseTitle(tc.title)
		if ver != tc.wantVer {
			t.Errorf("%q: version = %q, want %q", tc.title, ver, tc.wantVer)
		}
		if len(models) != len(tc.wantModels) {
			t.Errorf("%q: got %d models %v, want %d %v",
				tc.title, len(models), models, len(tc.wantModels), tc.wantModels)
			continue
		}
		for i := range models {
			if models[i] != tc.wantModels[i] {
				t.Errorf("%q: model[%d] = %q, want %q", tc.title, i, models[i], tc.wantModels[i])
			}
		}
	}
}

// A release title covering seven models must map to all seven, because that is
// the many-to-many relationship `fleet status` depends on.
func TestSplitReleaseTitleFamilyBreadth(t *testing.T) {
	models, ver := SplitReleaseTitle("TSW-570/TSW-770/TSW-1070/TSS-770/TSS-1070/TS-770/TS-1070 3.0.1234")
	if len(models) != 7 {
		t.Fatalf("family release resolved to %d models %v, want 7", len(models), models)
	}
	if ver == "" {
		t.Fatal("family release lost its version")
	}
	want := "TSW-1070"
	var found bool
	for _, m := range models {
		if m == want {
			found = true
		}
	}
	if !found {
		t.Errorf("%q missing from %v; a fleet lookup for it would miss its own release", want, models)
	}
}

func TestExpandModelFamilyAbbreviated(t *testing.T) {
	got := ExpandModelFamily([]string{"DM-NVX-D10", "D20", "E10", "E20"})
	want := []string{"DM-NVX-D10", "DM-NVX-D20", "DM-NVX-E10", "DM-NVX-E20"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestClassifyAsset(t *testing.T) {
	cases := map[string]string{
		"Spec Sheet: DM-NVX-385":                      "spec-sheet",
		"DM-NVX-x6x Guide Spec":                       "guide-spec",
		"DM-NVX-36x Revit":                            "revit",
		"CAD Drawing for: DM-NVX-360 / DM-NVX-363":    "cad",
		"End-of-Sale Notice: FlexCarts":               "end-of-sale",
		"Security Reference Guide: DM NVX and DM-NAX": "security-reference",
		"Product Manual: DM NVX AV-over-IP":           "manual",
		"Quick Start: UC-FCM Series":                  "quick-start",
	}
	for title, want := range cases {
		if got := classifyAsset(title); got != want {
			t.Errorf("classifyAsset(%q) = %q, want %q", title, got, want)
		}
	}
}

// The product page carries several unrelated data-id attributes, including an
// embedded video modal whose id is a Vimeo id. The document id must be the
// product's own, because ResourceHandler.ashx?dID= depends on it.
func TestParseProductPageDocumentIDIgnoresVideoID(t *testing.T) {
	p, err := ParseProductPage(fixture(t, "product_page.html"), "/x")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if p.DocumentID != "21965" {
		t.Fatalf("document id = %q, want 21965 (a wrong id silently returns another product's assets)", p.DocumentID)
	}
}

func TestParseModelTable(t *testing.T) {
	rows, err := ParseModelTable(fixture(t, "variants.html"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(rows) == 0 {
		t.Fatal("expected series member models, got none")
	}
	var sawNVX360 bool
	for _, r := range rows {
		if r.Model == "" {
			t.Errorf("row with empty model: %+v", r)
		}
		// The model column also renders the internal id in a sibling div; they
		// must not be concatenated into the model name.
		if strings.ContainsAny(r.Model, " ") {
			t.Errorf("model %q contains whitespace, so the internal id leaked into it", r.Model)
		}
		if r.Model == "DM-NVX-360" {
			sawNVX360 = true
			if r.InternalID == "" {
				t.Error("expected an internal id alongside the model")
			}
			if r.Description == "" {
				t.Error("expected a description")
			}
		}
	}
	if !sawNVX360 {
		t.Errorf("DM-NVX-360 missing from its own series roster: %+v", rows)
	}
}

// Category pages and product pages share the /Products/Catalog/ prefix and both
// embed schema.org JSON-LD of type Product, so only the inline request block
// distinguishes them.
func TestIsCategoryPage(t *testing.T) {
	if !IsCategoryPage(fixture(t, "category_page.html")) {
		t.Error("category page was not recognized; it would be parsed as a product")
	}
	if IsCategoryPage(fixture(t, "product_page.html")) {
		t.Error("product page was misread as a category")
	}
}
