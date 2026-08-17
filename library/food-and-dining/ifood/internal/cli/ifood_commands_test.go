package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestCollectMarketsFiltersAndDeduplicates(t *testing.T) {
	input := map[string]any{"sections": []any{
		map[string]any{"id": "a", "name": "A", "userRating": 4.9, "distance": 2.0, "available": true},
		map[string]any{"nested": map[string]any{"id": "b", "name": "B", "userRating": 4.4}},
		map[string]any{"id": "a", "name": "A", "userRating": 4.9, "distance": 1.5},
	}}
	got := map[string]marketSummary{}
	collectMarkets(input, 4.5, got)
	if len(got) != 1 || got["a"].DistanceKM != 1.5 {
		t.Fatalf("unexpected markets: %#v", got)
	}
}

func TestSelectMarkets(t *testing.T) {
	markets := []marketSummary{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	got := selectMarkets(markets, []string{"b", "c"}, 1)
	if len(got) != 1 || got[0].ID != "b" {
		t.Fatalf("unexpected selection: %#v", got)
	}
}

func TestParseCartItems(t *testing.T) {
	got, err := parseCartItems([]string{"product-a", "product-b:3"}, "sem substituicao")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Quantity != 1 || got[1].Quantity != 3 || got[1].Observation != "sem substituicao" {
		t.Fatalf("unexpected cart items: %#v", got)
	}
	if _, err := parseCartItems([]string{"product-a:0"}, ""); err == nil {
		t.Fatal("expected zero quantity to fail")
	}
}

func TestCartAPIPath(t *testing.T) {
	got, err := cartAPIPath(2, "cart-123", "items")
	if err != nil || got != "/v2/carts/cart-123/items" {
		t.Fatalf("unexpected path %q, err=%v", got, err)
	}
	if _, err := cartAPIPath(1, "bad/id", ""); err == nil {
		t.Fatal("expected invalid cart ID to fail")
	}
}

func TestCartAddDefaultsToPreview(t *testing.T) {
	flags := &rootFlags{asJSON: true}
	cmd := newCartCmd(flags)
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{"add", "cart-123", "--item", "product-a:2"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var preview struct {
		Method   string          `json:"method"`
		Path     string          `json:"path"`
		Body     []cartItemInput `json:"body"`
		Executed bool            `json:"executed"`
	}
	if err := json.Unmarshal(output.Bytes(), &preview); err != nil {
		t.Fatalf("decode preview %q: %v", output.String(), err)
	}
	if preview.Method != "POST" || preview.Path != "/v1/carts/cart-123/items" || preview.Executed || len(preview.Body) != 1 || preview.Body[0].Quantity != 2 {
		t.Fatalf("unexpected preview: %#v", preview)
	}
}

func TestCartAddExecuteRequiresYes(t *testing.T) {
	flags := &rootFlags{asJSON: true}
	cmd := newCartCmd(flags)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"add", "cart-123", "--item", "product-a", "--execute"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--execute and --yes") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseQuantityTerms(t *testing.T) {
	got, err := parseQuantityTerms([]string{"alcool em gel:2", "filtro de linha"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Term != "alcool em gel" || got[0].Quantity != 2 || got[1].Quantity != 1 {
		t.Fatalf("unexpected terms: %#v", got)
	}
	if _, err := parseQuantityTerms([]string{"esponja:0"}); err == nil {
		t.Fatal("expected zero quantity to fail")
	}
}

func TestResolveCartBuildCombinesDuplicateProducts(t *testing.T) {
	terms := []quantityTerm{{Term: "alcool", Quantity: 2}, {Term: "alcool gel", Quantity: 1}}
	search := func(string) ([]productSummary, error) {
		return []productSummary{{ID: "same-product", Name: "Alcool gel", Price: 7.5, Available: true}}, nil
	}
	selections, items, subtotal, complete, err := resolveCartBuild(terms, "", search)
	if err != nil {
		t.Fatal(err)
	}
	if !complete || len(selections) != 2 || len(items) != 1 || items[0].Quantity != 3 || subtotal != 22.5 {
		t.Fatalf("unexpected build: selections=%#v items=%#v subtotal=%v complete=%v", selections, items, subtotal, complete)
	}
}

func TestResolveCartBuildReportsMissingTerm(t *testing.T) {
	terms := []quantityTerm{{Term: "guardanapo", Quantity: 1}, {Term: "extensao", Quantity: 1}}
	search := func(term string) ([]productSummary, error) {
		if term == "extensao" {
			return nil, nil
		}
		return []productSummary{{ID: "guardanapo-1", Price: 4}}, nil
	}
	selections, items, _, complete, err := resolveCartBuild(terms, "", search)
	if err != nil {
		t.Fatal(err)
	}
	if complete || len(items) != 1 || selections[1].Status != "not_found" {
		t.Fatalf("unexpected incomplete build: selections=%#v items=%#v complete=%v", selections, items, complete)
	}
}

func TestCartBuildDryRunDoesNotResolveProducts(t *testing.T) {
	flags := &rootFlags{asJSON: true, dryRun: true}
	cmd := newCartCmd(flags)
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{"build", "cart-123", "--merchant", "merchant-1", "--latitude", "-9.6", "--longitude", "-35.7", "--item", "papel toalha:2"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var preview struct {
		Resolved bool `json:"resolved"`
		Executed bool `json:"executed"`
		Request  struct {
			Path string `json:"path"`
		} `json:"request"`
		Terms []struct {
			Term     string `json:"term"`
			Quantity int    `json:"quantity"`
		} `json:"terms"`
	}
	if err := json.Unmarshal(output.Bytes(), &preview); err != nil {
		t.Fatalf("decode preview %q: %v", output.String(), err)
	}
	if preview.Resolved || preview.Executed || preview.Request.Path != "/v1/carts/cart-123/items" || len(preview.Terms) != 1 || preview.Terms[0].Quantity != 2 {
		t.Fatalf("unexpected dry-run: %#v", preview)
	}
}

func TestCartBuildExampleCreatesSafeCompletePlan(t *testing.T) {
	flags := &rootFlags{asJSON: true}
	cmd := newCartCmd(flags)
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{"build", "EXAMPLE_CART", "--example"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatalf("decode output %q: %v", output.String(), err)
	}
	if result["example"] != true || result["ready"] != true || result["executed"] != false || result["remote_write_performed"] != false {
		t.Fatalf("unexpected cart build example: %#v", result)
	}
	build, _, err := cmd.Find([]string{"build"})
	if err != nil {
		t.Fatal(err)
	}
	if build.Annotations["mcp:read-only"] != "true" || build.Annotations["pp:happy-args"] == "" || build.Flags().Lookup("execute") != nil {
		t.Fatalf("cart build must remain a read-only preview command: %#v", build.Annotations)
	}
}

func TestSelectAddress(t *testing.T) {
	addresses := []addressSummary{
		{ID: "active", Active: true, Coordinates: cartCoordinates{Latitude: -9.6, Longitude: -35.7}},
		{ID: "other"},
	}
	selected, err := selectAddress(addresses, "")
	if err != nil || selected.ID != "active" {
		t.Fatalf("unexpected default address: %#v, err=%v", selected, err)
	}
	selected, err = selectAddress(addresses, "other")
	if err != nil || selected.ID != "other" {
		t.Fatalf("unexpected requested address: %#v, err=%v", selected, err)
	}
	if _, err := selectAddress(addresses, "missing"); err == nil {
		t.Fatal("expected missing address to fail")
	}
}

func TestSelectDeliveryMethod(t *testing.T) {
	catalog := map[string]any{"merchant": map[string]any{"deliveryMethods": []any{
		map[string]any{"id": "disabled", "available": false},
		map[string]any{"id": "delivery-1", "mode": "DELIVERY", "deliveredBy": "IFOOD", "available": true, "schedule": map[string]any{"now": true}},
	}}}
	got, err := selectDeliveryMethod(catalog)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "delivery-1" || got.Mode != "DELIVERY" || got.DeliveredBy != "IFOOD" || !got.Now {
		t.Fatalf("unexpected delivery method: %#v", got)
	}
}

func TestDeliveryReadyRequiresTimeSlotWhenScheduled(t *testing.T) {
	if deliveryReady(cartDeliveryInput{ID: "delivery-1", Now: false}) {
		t.Fatal("scheduled delivery without a time slot must be incomplete")
	}
	if !deliveryReady(cartDeliveryInput{ID: "delivery-1", Now: false, SelectedTimeSlotID: "slot-1"}) {
		t.Fatal("scheduled delivery with a time slot must be complete")
	}
}

func TestBuildCartCreatePayload(t *testing.T) {
	coordinates := cartCoordinates{Latitude: -9.6, Longitude: -35.7}
	delivery := cartDeliveryInput{ID: "delivery-1", Now: true}
	items := []cartItemInput{{ID: "product-1", Quantity: 2}}
	got := buildCartCreatePayload("merchant-1", "address-1", coordinates, delivery, items)
	if got.Merchant.ID != "merchant-1" || got.Merchant.Context != "DEFAULT" || got.Address.ID != "address-1" || got.Address.Coordinates != coordinates || got.Delivery.ID != "delivery-1" || len(got.Items) != 1 {
		t.Fatalf("unexpected create payload: %#v", got)
	}
}

func TestFindCartID(t *testing.T) {
	response := map[string]any{"data": map[string]any{"cartResponse": map[string]any{"id": "cart-456"}}}
	if got := findCartID(response); got != "cart-456" {
		t.Fatalf("cart ID = %q", got)
	}
	unsafe := map[string]any{"data": map[string]any{"items": []any{map[string]any{"id": "product-id"}}}}
	if got := findCartID(unsafe); got != "" {
		t.Fatalf("must not mistake nested product ID for cart ID: %q", got)
	}
}

func TestCartCreateDryRunDoesNotUseNetwork(t *testing.T) {
	flags := &rootFlags{asJSON: true, dryRun: true}
	cmd := newCartCmd(flags)
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{"create", "--merchant", "merchant-1", "--item", "esponja:2"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var preview struct {
		Resolved bool `json:"resolved"`
		Executed bool `json:"executed"`
		Request  struct {
			Path string `json:"path"`
		} `json:"request"`
		Terms []struct {
			Term     string `json:"term"`
			Quantity int    `json:"quantity"`
		} `json:"terms"`
	}
	if err := json.Unmarshal(output.Bytes(), &preview); err != nil {
		t.Fatalf("decode preview %q: %v", output.String(), err)
	}
	if preview.Resolved || preview.Executed || preview.Request.Path != "/v1/carts" || len(preview.Terms) != 1 || preview.Terms[0].Term != "esponja" || preview.Terms[0].Quantity != 2 {
		t.Fatalf("unexpected create dry-run: %#v", preview)
	}
}

func TestAddressesListDryRunDoesNotUseNetwork(t *testing.T) {
	flags := &rootFlags{asJSON: true, dryRun: true}
	cmd := newAddressesCmd(flags)
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{"list"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `"path": "/v1/customers/me/addresses"`) {
		t.Fatalf("unexpected addresses dry-run: %s", output.String())
	}
}

func TestIfoodContractFixtures(t *testing.T) {
	addressData, err := os.ReadFile("testdata/ifood_addresses.json")
	if err != nil {
		t.Fatal(err)
	}
	var addresses []addressSummary
	if err := json.Unmarshal(addressData, &addresses); err != nil {
		t.Fatal(err)
	}
	sortAddressesForSelection(addresses)
	selected, err := selectAddress(addresses, "")
	if err != nil || selected.ID != "address-active" || selected.Coordinates.Latitude == 0 {
		t.Fatalf("unexpected fixture address: %#v, err=%v", selected, err)
	}

	catalogData, err := os.ReadFile("testdata/ifood_catalog_delivery.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog any
	if err := json.Unmarshal(catalogData, &catalog); err != nil {
		t.Fatal(err)
	}
	delivery, err := selectDeliveryMethod(catalog)
	if err != nil || delivery.ID != "delivery-default" || delivery.DeliveredBy != "IFOOD" {
		t.Fatalf("unexpected fixture delivery: %#v, err=%v", delivery, err)
	}
}

func TestCartCreatePreviewEndToEndDoesNotPost(t *testing.T) {
	addressData, err := os.ReadFile("testdata/ifood_addresses.json")
	if err != nil {
		t.Fatal(err)
	}
	catalogData, err := os.ReadFile("testdata/ifood_catalog_delivery.json")
	if err != nil {
		t.Fatal(err)
	}
	postCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/customers/me/addresses":
			_, _ = w.Write(addressData)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/merchants/multicategory/merchant-fixture/catalog":
			_, _ = w.Write(catalogData)
		case r.Method == http.MethodGet && r.URL.Path == "/v2/search/merchants/merchant-fixture/catalog-items":
			term := r.URL.Query().Get("term")
			_ = json.NewEncoder(w).Encode(map[string]any{"items": map[string]any{"data": []any{map[string]any{
				"id": "product-" + strings.ReplaceAll(term, " ", "-"), "name": term, "price": 5.5, "currency": "BRL", "available": true,
			}}}})
		case r.Method == http.MethodPost:
			postCount++
			http.Error(w, `{"error":"unexpected write"}`, http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	t.Setenv("IFOOD_BASE_URL", server.URL)
	t.Setenv("IFOOD_BEARER_AUTH", "synthetic-test-token")
	t.Setenv("IFOOD_HEADERS_FILE", "")
	t.Setenv("IFOOD_HOME", t.TempDir())

	flags := &rootFlags{asJSON: true}
	cmd := newCartCmd(flags)
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{"create", "--merchant", "merchant-fixture", "--item", "esponja", "--item", "papel toalha:2"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if postCount != 0 {
		t.Fatalf("preview performed %d POST requests", postCount)
	}
	var preview struct {
		Complete bool `json:"complete"`
		Executed bool `json:"executed"`
		Address  struct {
			ID string `json:"id"`
		} `json:"address"`
		Delivery cartDeliveryInput `json:"delivery"`
		Request  struct {
			Path string            `json:"path"`
			Body cartCreatePayload `json:"body"`
		} `json:"request"`
	}
	if err := json.Unmarshal(output.Bytes(), &preview); err != nil {
		t.Fatalf("decode preview %q: %v", output.String(), err)
	}
	if !preview.Complete || preview.Executed || preview.Address.ID != "address-active" || preview.Delivery.ID != "delivery-default" || preview.Request.Path != "/v1/carts" || len(preview.Request.Body.Items) != 2 {
		t.Fatalf("unexpected cart preview: %#v", preview)
	}
}

func TestCartCreateExecuteUsesResolvedPayload(t *testing.T) {
	addressData, err := os.ReadFile("testdata/ifood_addresses.json")
	if err != nil {
		t.Fatal(err)
	}
	var posted cartCreatePayload
	postCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/customers/me/addresses":
			_, _ = w.Write(addressData)
		case r.Method == http.MethodGet && r.URL.Path == "/v2/search/merchants/merchant-fixture/catalog-items":
			_ = json.NewEncoder(w).Encode(map[string]any{"items": map[string]any{"data": []any{map[string]any{
				"id": "product-esponja", "name": "Esponja", "price": 3.5, "currency": "BRL", "available": true,
			}}}})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/carts":
			postCount++
			if err := json.NewDecoder(r.Body).Decode(&posted); err != nil {
				t.Errorf("decode posted cart: %v", err)
			}
			_, _ = w.Write([]byte(`{"cartResponse":{"id":"cart-created"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	t.Setenv("IFOOD_BASE_URL", server.URL)
	t.Setenv("IFOOD_BEARER_AUTH", "synthetic-test-token")
	t.Setenv("IFOOD_HEADERS_FILE", "")
	t.Setenv("IFOOD_HOME", t.TempDir())

	flags := &rootFlags{asJSON: true, yes: true}
	cmd := newCartCmd(flags)
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{"create", "--merchant", "merchant-fixture", "--delivery-id", "delivery-fixture", "--item", "esponja", "--execute"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if postCount != 1 || posted.Merchant.ID != "merchant-fixture" || posted.Address.ID != "address-active" || posted.Delivery.ID != "delivery-fixture" || len(posted.Items) != 1 || posted.Items[0].ID != "product-esponja" {
		t.Fatalf("unexpected posted payload: count=%d payload=%#v", postCount, posted)
	}
	var result map[string]any
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatalf("decode result %q: %v", output.String(), err)
	}
	if result["cart_id"] != "cart-created" || result["executed"] != true {
		t.Fatalf("unexpected create result: %#v", result)
	}
}

func TestParseCurlHeaders(t *testing.T) {
	command := "curl 'https://www.ifood.com.br/site-api/v1/customers/me' \\\n  -H 'Authorization: Bearer secret-token' \\\n  --header \"X-Ifood-Device-Id: device-123\""
	headers, err := parseCurlHeaders(command)
	if err != nil {
		t.Fatal(err)
	}
	if headers["Authorization"] != "Bearer secret-token" || headers["X-Ifood-Device-Id"] != "device-123" {
		t.Fatalf("unexpected parsed headers: %#v", headers)
	}
	if _, err := parseCurlHeaders("curl https://www.ifood.com.br -H 'Accept: application/json'"); err == nil {
		t.Fatal("expected missing Authorization to fail")
	}
}

func TestImportSessionHeadersDryRunRedactsValues(t *testing.T) {
	flags := &rootFlags{asJSON: true, dryRun: true}
	cmd := &cobra.Command{Use: "test"}
	var output bytes.Buffer
	cmd.SetOut(&output)
	headers := map[string]string{"Authorization": "Bearer never-print-this", "X-Ifood-Device-Id": "device-secret"}
	if err := importSessionHeaders(cmd, flags, "/tmp/headers.json", headers); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	if strings.Contains(text, "never-print-this") || strings.Contains(text, "device-secret") || !strings.Contains(text, "Authorization") || !strings.Contains(text, `"written": false`) {
		t.Fatalf("unexpected redacted preview: %s", text)
	}
}

func TestSessionImportsDryRunWithoutReadingCredentialFiles(t *testing.T) {
	for _, subcommand := range []string{"import-curl", "import-headers"} {
		t.Run(subcommand, func(t *testing.T) {
			flags := &rootFlags{asJSON: true, dryRun: true}
			cmd := newSessionCmd(flags)
			var output bytes.Buffer
			cmd.SetOut(&output)
			cmd.SetErr(&output)
			missing := filepath.Join(t.TempDir(), "missing-input")
			out := filepath.Join(t.TempDir(), "headers.json")
			cmd.SetArgs([]string{subcommand, "--input", missing, "--out", out})
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			var result map[string]any
			if err := json.Unmarshal(output.Bytes(), &result); err != nil {
				t.Fatalf("decode output %q: %v", output.String(), err)
			}
			if result["dry_run"] != true || result["credential_values_read"] != false || result["installed"] != false {
				t.Fatalf("unexpected session import preview: %#v", result)
			}
			if _, err := os.Stat(out); !os.IsNotExist(err) {
				t.Fatalf("dry-run wrote output file: %v", err)
			}
		})
	}
}

func TestQuoteExampleComparesThreeMarkets(t *testing.T) {
	flags := &rootFlags{asJSON: true}
	cmd := newQuoteCmd(flags)
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{"--example"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Source           string                   `json:"source"`
		Quotes           []browserValidatedMarket `json:"quotes"`
		SelectedMarketID string                   `json:"selected_market_id"`
	}
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatalf("decode output %q: %v", output.String(), err)
	}
	if result.Source != "embedded_example" || len(result.Quotes) != 3 || result.SelectedMarketID == "" {
		t.Fatalf("unexpected quote example: %#v", result)
	}
}

func TestWritePrivateHeadersAndSessionStatus(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session", "headers.json")
	if err := writePrivateHeaders(path, map[string]string{"Authorization": "Bearer synthetic"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("headers mode = %o", info.Mode().Perm())
	}
	t.Setenv("IFOOD_HEADERS_FILE", path)
	t.Setenv("IFOOD_BEARER_AUTH", "")
	flags := &rootFlags{asJSON: true}
	cmd := newSessionCmd(flags)
	statusCommand, _, err := cmd.Find([]string{"status"})
	if err != nil {
		t.Fatal(err)
	}
	if statusCommand.Annotations["mcp:read-only"] != "true" {
		t.Fatalf("session status must be read-only: %#v", statusCommand.Annotations)
	}
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{"status"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var status map[string]any
	if err := json.Unmarshal(output.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status["ready"] != true || status["authorization_in_headers_file"] != true || status["bearer_configured"] != false {
		t.Fatalf("unexpected session status: %#v", status)
	}
}
