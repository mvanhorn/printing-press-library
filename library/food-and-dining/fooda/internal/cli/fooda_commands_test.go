package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/fooda/internal/client"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/fooda/internal/config"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/fooda/internal/store"
)

func TestFoodaCommandsHelpWiring(t *testing.T) {
	cmds := []string{
		"events", "restaurants", "menu", "orders", "orders get",
		"subsidy", "card", "whoami", "recommend", "order", "sync",
		"served-history", "venue-rotation", "spend-trends",
		"week-ahead", "subsidy-status", "menu-search",
	}

	for _, sub := range cmds {
		t.Run(sub, func(t *testing.T) {
			cmd := RootCmd()
			args := strings.Fields(sub)
			args = append(args, "--help")
			cmd.SetArgs(args)

			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)

			if err := cmd.Execute(); err != nil {
				t.Fatalf("%s --help error = %v", sub, err)
			}

			help := out.String()
			if !strings.Contains(help, "Usage:") {
				t.Errorf("%s --help missing 'Usage:' in output:\n%s", sub, help)
			}
		})
	}
}

func TestStoreBackedAnalytics(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("FOODA_HOME", tempHome)

	dbPath := defaultDBPath("fooda-pp-cli")
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Seed some mock synced past orders
	mockOrdersList := client.PastOrdersProps{}
	var o1, o2 struct {
		ID                     int      `json:"id"`
		EventType              string   `json:"event_type"`
		ItemNames              []string `json:"item_names"`
		OrderFulfilledTimeUnix int64    `json:"order_fulfilled_time_unix"`
		VendorNames            []string `json:"vendor_names"`
		Status                 string   `json:"status"`
		Delivery               string   `json:"delivery"`
		RequestID              string   `json:"request_id"`
		PaymentCents           int      `json:"payment_cents"`
	}

	fixedTime := time.Now().Truncate(24 * time.Hour).Add(-12 * time.Hour) // yesterday noon

	o1.ID = 1001
	o1.EventType = "delivery"
	o1.ItemNames = []string{"Scrubbed Burrito"}
	o1.OrderFulfilledTimeUnix = fixedTime.Unix()
	o1.VendorNames = []string{"Scrubbed Burrito Shop"}
	o1.Status = "checkout_complete"
	o1.Delivery = "Delivered today"
	o1.RequestID = "uuid-1111"
	o1.PaymentCents = 1500

	o2.ID = 1002
	o2.EventType = "delivery"
	o2.ItemNames = []string{"Scrubbed Taco"}
	o2.OrderFulfilledTimeUnix = fixedTime.Unix()
	o2.VendorNames = []string{"Scrubbed Taco Shop"}
	o2.Status = "checkout_complete"
	o2.Delivery = "Delivered last week"
	o2.RequestID = "uuid-2222"
	o2.PaymentCents = 0 // Fully subsidized

	mockOrdersList.Presenter = []struct {
		Order struct {
			ID                     int      `json:"id"`
			EventType              string   `json:"event_type"`
			ItemNames              []string `json:"item_names"`
			OrderFulfilledTimeUnix int64    `json:"order_fulfilled_time_unix"`
			VendorNames            []string `json:"vendor_names"`
			Status                 string   `json:"status"`
			Delivery               string   `json:"delivery"`
			RequestID              string   `json:"request_id"`
			PaymentCents           int      `json:"payment_cents"`
		} `json:"order"`
	}{
		{Order: o1},
		{Order: o2},
	}

	// Seed individual orders for new sync format (Item 3)
	var o1Prop, o2Prop client.PastOrdersProps
	o1Prop.Presenter = []struct {
		Order struct {
			ID                     int      `json:"id"`
			EventType              string   `json:"event_type"`
			ItemNames              []string `json:"item_names"`
			OrderFulfilledTimeUnix int64    `json:"order_fulfilled_time_unix"`
			VendorNames            []string `json:"vendor_names"`
			Status                 string   `json:"status"`
			Delivery               string   `json:"delivery"`
			RequestID              string   `json:"request_id"`
			PaymentCents           int      `json:"payment_cents"`
		} `json:"order"`
	}{{Order: o1}}
	o2Prop.Presenter = []struct {
		Order struct {
			ID                     int      `json:"id"`
			EventType              string   `json:"event_type"`
			ItemNames              []string `json:"item_names"`
			OrderFulfilledTimeUnix int64    `json:"order_fulfilled_time_unix"`
			VendorNames            []string `json:"vendor_names"`
			Status                 string   `json:"status"`
			Delivery               string   `json:"delivery"`
			RequestID              string   `json:"request_id"`
			PaymentCents           int      `json:"payment_cents"`
		} `json:"order"`
	}{{Order: o2}}

	o1JSON, _ := json.Marshal(o1Prop)
	o2JSON, _ := json.Marshal(o2Prop)
	_ = db.Upsert("order", "uuid-1111", o1JSON)
	_ = db.Upsert("order", "uuid-2222", o2JSON)

	// Seed order details to test subsidy-vs-paid parsing in spend-trends
	var det1, det2 client.OrderDetailProps
	det1.OrderData.OrderUuid = "uuid-1111"
	det1.OrderData.FormattedTotalAmount = "$15.00"
	det1.OrderData.FormattedSubsidyAmount = "$0.00"

	det2.OrderData.OrderUuid = "uuid-2222"
	det2.OrderData.FormattedTotalAmount = "$0.00"
	det2.OrderData.FormattedSubsidyAmount = "-$12.50"

	det1JSON, _ := json.Marshal(det1)
	det2JSON, _ := json.Marshal(det2)

	_ = db.Upsert("order_detail", "uuid-1111", det1JSON)
	_ = db.Upsert("order_detail", "uuid-2222", det2JSON)

	// We also test fallback to legacy orders_list key (Item 3)
	ordersJSON, _ := json.Marshal(mockOrdersList)
	if err := db.Upsert("order", "orders_list", ordersJSON); err != nil {
		t.Fatalf("failed to upsert orders list: %v", err)
	}

	// Validate DB content is accessible
	list, err := db.List("order", 10)
	if err != nil {
		t.Fatalf("failed to list orders from DB: %v", err)
	}
	if len(list) != 3 { // orders_list + uuid-1111 + uuid-2222 (deduplicated during read)
		t.Errorf("expected 3 list rows, got %d", len(list))
	}

	// Close seeded DB so commands can open it read-only
	db.Close()

	// Run served-history command (Item 7)
	{
		cmd := RootCmd()
		cmd.SetArgs([]string{"served-history", "--data-source", "local", "--since", "30d"})
		var stdout, stderr bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)

		if err := cmd.Execute(); err != nil {
			t.Fatalf("served-history failed: %v, stderr: %s", err, stderr.String())
		}
		out := stdout.String()
		if !strings.Contains(out, "Scrubbed Burrito Shop") || !strings.Contains(out, "Scrubbed Taco Shop") {
			t.Errorf("served-history missing expected vendors, output:\n%s", out)
		}
		if !strings.Contains(out, "$15.00") || !strings.Contains(out, "$0.00") {
			t.Errorf("served-history missing expected prices, output:\n%s", out)
		}
	}

	// Run venue-rotation command (Item 7)
	{
		cmd := RootCmd()
		cmd.SetArgs([]string{"venue-rotation", "--data-source", "local", "--since", "30d"})
		var stdout, stderr bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)

		if err := cmd.Execute(); err != nil {
			t.Fatalf("venue-rotation failed: %v, stderr: %s", err, stderr.String())
		}
		out := stdout.String()
		if !strings.Contains(out, "Scrubbed Burrito Shop") || !strings.Contains(out, "Scrubbed Taco Shop") {
			t.Errorf("venue-rotation missing expected vendors, output:\n%s", out)
		}
	}

	// Run spend-trends command (Item 7)
	{
		cmd := RootCmd()
		cmd.SetArgs([]string{"spend-trends", "--data-source", "local", "--since", "30d", "--group-by", "month"})
		var stdout, stderr bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)

		if err := cmd.Execute(); err != nil {
			t.Fatalf("spend-trends failed: %v, stderr: %s", err, stderr.String())
		}
		out := stdout.String()
		if !strings.Contains(out, "$12.50") || !strings.Contains(out, "$15.00") || !strings.Contains(out, "$27.50") {
			t.Errorf("spend-trends missing expected totals, output:\n%s", out)
		}
	}

	// Test local-only / live-only data-source rejections (Item 4)
	{
		cmd := RootCmd()
		cmd.SetArgs([]string{"served-history", "--data-source", "live"})
		var stdout, stderr bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "local-only") {
			t.Errorf("expected served-history to reject --data-source live, got err: %v, stderr: %s", err, stderr.String())
		}

		cmd = RootCmd()
		cmd.SetArgs([]string{"events", "--data-source", "local"})
		var stdout2, stderr2 bytes.Buffer
		cmd.SetOut(&stdout2)
		cmd.SetErr(&stderr2)
		err = cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "live-only") {
			t.Errorf("expected events to reject --data-source local, got err: %v, stderr: %s", err, stderr2.String())
		}
	}
}

func TestParseSince(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"90d", false},
		{"2w", false},
		{"6mo", false},
		{"1y", false},
		{"invalid", true},
		{"", true},
		{"abc", true},
		{"-5d", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseSince(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseSince(%q) error = %v, wantErr = %v", tt.input, err, tt.wantErr)
			}
			if err == nil && got.IsZero() {
				t.Errorf("parseSince(%q) returned zero time but no error", tt.input)
			}
		})
	}
}

func TestURLToPath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://app.fooda.com/accounts/8358/select_events/S609639/items", "/accounts/8358/select_events/S609639/items"},
		{"https://app.fooda.com/accounts/8358/select_events/S609639", "/accounts/8358/select_events/S609639/items"},
		{"/accounts/8358/select_events/S609639/", "/accounts/8358/select_events/S609639/items"},
		{"invalid-url", "invalid-url"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := urlToPath(tt.input); got != tt.want {
				t.Errorf("urlToPath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestGetEventServiceDate(t *testing.T) {
	evPopup := parsedEvent{
		StartTime: "2026-10-15T12:00:00Z",
		Typename:  "PopupEventPublic",
	}
	tPopup, datePopup, err := getEventServiceDate(evPopup)
	if err != nil {
		t.Fatalf("unexpected error for Popup: %v", err)
	}
	if datePopup != "2026-10-15" {
		t.Errorf("expected 2026-10-15, got %s", datePopup)
	}
	if tPopup.Year() != 2026 || tPopup.Month() != 10 || tPopup.Day() != 15 {
		t.Errorf("incorrect service parsed time for Popup: %v", tPopup)
	}

	evDelivery := parsedEvent{
		StartTime: "2026-10-14T12:00:00Z",
		Typename:  "DeliveryEventPublic",
	}
	evDelivery.DeliveryWindow.StartTime = "2026-10-16T11:30:00Z"
	_, dateDelivery, err := getEventServiceDate(evDelivery)
	if err != nil {
		t.Fatalf("unexpected error for Delivery: %v", err)
	}
	if dateDelivery != "2026-10-16" {
		t.Errorf("expected 2026-10-16, got %s", dateDelivery)
	}
}

func TestGetAccountAndBuildingMemoization(t *testing.T) {
	resetMemoizedAccountAndBuilding()
	defer resetMemoizedAccountAndBuilding()

	var myPageResponse string
	var myPageStatus int = http.StatusOK
	var callCount int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/my" {
			callCount++
			w.WriteHeader(myPageStatus)
			_, _ = w.Write([]byte(myPageResponse))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	flags := &rootFlags{}
	cfgObj, err := config.Load("")
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	cfgObj.BaseURL = server.URL
	c := client.New(cfgObj, 0, 0)

	// Test 1: Discovery Success
	{
		resetMemoizedAccountAndBuilding()
		myPageStatus = http.StatusOK
		myPageResponse = `<html><body><div props='{"filters":{"account_id":["9999"],"locations":{"building_id":["8888"]}},"userId":777}' id='my-event-list'></div></body></html>`
		callCount = 0

		acct, bldg, err := getAccountAndBuilding(context.Background(), c, flags)
		if err != nil {
			t.Fatalf("unexpected discovery error: %v", err)
		}
		if acct != "9999" || bldg != "8888" {
			t.Errorf("expected 9999, 8888; got %s, %s", acct, bldg)
		}
		if callCount != 1 {
			t.Errorf("expected 1 call to /my, got %d", callCount)
		}

		// Test 1b: Memoization (second call should NOT hit the server)
		acct2, bldg2, err := getAccountAndBuilding(context.Background(), c, flags)
		if err != nil {
			t.Fatalf("unexpected memoized error: %v", err)
		}
		if acct2 != "9999" || bldg2 != "8888" {
			t.Errorf("expected memoized values 9999, 8888; got %s, %s", acct2, bldg2)
		}
		if callCount != 1 {
			t.Errorf("expected memoized call to bypass network, but it hit the server (callCount=%d)", callCount)
		}
	}

	// Test 2: Flags Override Discovery
	{
		resetMemoizedAccountAndBuilding()
		callCount = 0
		overrideFlags := &rootFlags{
			accountID:  "over-acct",
			buildingID: "over-bldg",
		}

		acct, bldg, err := getAccountAndBuilding(context.Background(), c, overrideFlags)
		if err != nil {
			t.Fatalf("unexpected error with overrides: %v", err)
		}
		if acct != "over-acct" || bldg != "over-bldg" {
			t.Errorf("expected over-acct, over-bldg; got %s, %s", acct, bldg)
		}
		if callCount != 0 {
			t.Errorf("expected flags override to bypass discovery network, but got %d calls", callCount)
		}
	}

	// Test 3: Discovery Failure with Login Hint
	{
		resetMemoizedAccountAndBuilding()
		myPageStatus = http.StatusForbidden // e.g. auth required
		myPageResponse = "Forbidden"

		_, _, err := getAccountAndBuilding(context.Background(), c, flags)
		if err == nil {
			t.Errorf("expected discovery failure, but got success")
		} else {
			errStr := err.Error()
			if !strings.Contains(errStr, "failed to discover Fooda account and building") {
				t.Errorf("missing expected error context, got: %s", errStr)
			}
			if !strings.Contains(errStr, "log in to app.fooda.com in Chrome and re-run 'auth login --chrome'") {
				t.Errorf("missing Chrome login hint in error: %s", errStr)
			}
		}
	}
}

func TestIsNetworkError(t *testing.T) {
	tests := []struct {
		err  error
		want bool
	}{
		{nil, false},
		{fmt.Errorf("some random error"), false},
		{fmt.Errorf("dial tcp 127.0.0.1:80: connect: connection refused"), true},
		{fmt.Errorf("read tcp 127.0.0.1:80: i/o timeout"), true},
		{fmt.Errorf("no such host: app.fooda.com"), true},
		{&net.OpError{Op: "dial", Net: "tcp"}, true},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%v", tt.err), func(t *testing.T) {
			if got := isNetworkError(tt.err); got != tt.want {
				t.Errorf("isNetworkError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestParsePriceDelta(t *testing.T) {
	tests := []struct {
		input string
		want  float64
	}{
		{"Extra Sauce (+$1.00)", 1.0},
		{"Dressing on the side", 0.0},
		{"Large (+$5.99)", 5.99},
		{"Discount (-$2.50)", -2.50},
		{"Add-on (+1.50)", 1.50},
		{"No brackets", 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := parsePriceDelta(tt.input); got != tt.want {
				t.Errorf("parsePriceDelta(%q) = %f, want %f", tt.input, got, tt.want)
			}
		})
	}
}

func TestPlanOrder(t *testing.T) {
	items := []client.MenuItem{
		{ID: "1", Name: "Beef Bulgogi Bowl", Price: 15.99},
		{ID: "2", Name: "Pork Mandoo (Korean Dumplings)", Price: 3.99},
		{ID: "3", Name: "Kimchi", Price: 1.99},
		{ID: "4", Name: "Soda", Price: 2.50},
	}

	tests := []struct {
		budget    float64
		anchor    string
		wantAddOn string
		wantFound bool
	}{
		{20.0, "Beef Bulgogi Bowl", "Pork Mandoo (Korean Dumplings)", true},
		{18.0, "Beef Bulgogi Bowl", "Kimchi", true},
		{15.0, "Beef Bulgogi Bowl", "", false},                              // no budget left
		{25.0, "Beef Bulgogi Bowl", "Pork Mandoo (Korean Dumplings)", true}, // Dumpling is best <= 9.01 leftover
		{20.0, "Missing Item", "", false},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("budget-%f-anchor-%s", tt.budget, tt.anchor), func(t *testing.T) {
			got, found := planOrder(tt.budget, tt.anchor, items)
			if found != tt.wantFound {
				t.Errorf("planOrder() found = %t, want %t", found, tt.wantFound)
			}
			if found && got.Name != tt.wantAddOn {
				t.Errorf("planOrder() got add-on = %q, want %q", got.Name, tt.wantAddOn)
			}
		})
	}
}

func TestParseItemPageHTML(t *testing.T) {
	// Parse Beef Bulgogi Bowl item.html
	{
		data, err := os.ReadFile(filepath.Join("..", "client", "testdata", "item.html"))
		if err != nil {
			t.Fatalf("failed to read item.html: %v", err)
		}
		form, err := parseItemPageHTML(string(data))
		if err != nil {
			t.Fatalf("failed to parse item.html: %v", err)
		}
		if form.ParentID != "608348" {
			t.Errorf("expected ParentID 608348, got %s", form.ParentID)
		}
		if form.ItemID != "589968" {
			t.Errorf("expected ItemID 589968, got %s", form.ItemID)
		}
		if form.ParentType != "SelectEvent" || form.ItemType != "InventoryItem" {
			t.Errorf("unexpected types: %s, %s", form.ParentType, form.ItemType)
		}
		if form.CSRFToken != "dummy-csrf-token" {
			t.Errorf("expected dummy-csrf-token, got %s", form.CSRFToken)
		}
		if len(form.Options) < 2 {
			t.Errorf("expected at least 2 options, got %d", len(form.Options))
		}
	}

	// Parse Pork Mandoo item_options.html
	{
		data, err := os.ReadFile(filepath.Join("..", "client", "testdata", "item_options.html"))
		if err != nil {
			t.Fatalf("failed to read item_options.html: %v", err)
		}
		form, err := parseItemPageHTML(string(data))
		if err != nil {
			t.Fatalf("failed to parse item_options.html: %v", err)
		}
		if form.ParentID != "608348" {
			t.Errorf("expected ParentID 608348, got %s", form.ParentID)
		}
		if form.ItemID != "732845" {
			t.Errorf("expected ItemID 732845, got %s", form.ItemID)
		}
	}
}

func TestOrderCommandsDryRun(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("FOODA_HOME", tempHome)

	var requestCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Initialize commands with a dry-run flag
	cmd := RootCmd()
	cmd.SetArgs([]string{"order", "add", "Beef Bulgogi Bowl", "--event", "S609348", "--dry-run"})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("order add dry-run failed: %v, stderr: %s", err, stderr.String())
	}
	if requestCount != 0 {
		t.Errorf("expected dry-run to send 0 network requests, but got %d", requestCount)
	}
}

func TestOrderPlanJSON(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("FOODA_HOME", tempHome)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Mock list of menu items from the select event items list page HTML
		_, _ = w.Write([]byte(`
<html>
<body>
<div class="item" data-vendor_name="Yum Yubu" data-category="SSams">
  <a href="/accounts/8358/select_events/S609348/items/1" class="item__link js-item-show-link">
    <div class="item__name">Beef Bulgogi Bowl</div>
    <div class="item__price">$15.99</div>
  </a>
</div>
<div class="item" data-vendor_name="Yum Yubu" data-category="SSams">
  <a href="/accounts/8358/select_events/S609348/items/2" class="item__link js-item-show-link">
    <div class="item__name">Pork Mandoo</div>
    <div class="item__price">$3.99</div>
  </a>
</div>
</body>
</html>
		`))
	}))
	defer server.Close()

	cfg, _ := config.Load("")
	cfg.BaseURL = server.URL
	_ = client.New(cfg, 0, 0)

	cmd := RootCmd()
	cmd.SetArgs([]string{"order", "plan", "--event", "S609348", "--budget", "20.0", "--anchor", "Beef Bulgogi Bowl", "--account", "over-acct", "--building", "over-bldg", "--json"})
	// set config BaseURL on flags so command uses test server
	// wait, flags are read from command run, so we can set config to server URL or override via FOODA_HOME and writing config.json
	t.Setenv("FOODA_HOME", tempHome)
	// We can write fooda_config.json or pass --config or just override base url on config
	cfgPath := filepath.Join(tempHome, "config", "fooda_config.json")
	_ = os.MkdirAll(filepath.Join(tempHome, "config"), 0o700)
	cfgJSON, _ := json.Marshal(map[string]any{"base_url": server.URL})
	_ = os.WriteFile(cfgPath, cfgJSON, 0o600)

	cmd.PersistentFlags().Set("config", cfgPath)

	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("order plan --json failed: %v, stderr: %s", err, stderr.String())
	}

	out := stdout.String()
	var parsed map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("order plan output is not valid JSON: %q, error: %v", out, err)
	}

	if parsed["anchor"] == nil || parsed["addon"] == nil {
		t.Errorf("expected anchor and addon fields in JSON, got: %s", out)
	}
}

func TestMalformedPriceAbortsPlace(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("FOODA_HOME", tempHome)

	var ordersPostCount int
	var cartPriceCount int
	var pricingJSON string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/my" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<html><head><meta name="api-client-token" content="dummy-client-token"><meta name="api-session-token" content="dummy-session-token"></head></html>`))
			return
		}
		if r.URL.Path == "/api/v1/orders" {
			ordersPostCount++
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.URL.Path == "/api/v1/cart/price" {
			cartPriceCount++
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(pricingJSON))
			return
		}
		if r.URL.Path == "/api/v1/cart/items" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":{"items":[{"name":"Taco","price":"10.00","quantity":1}]}}`))
			return
		}
		if r.URL.Path == "/api/v2/select_events/by" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"select_event":{"id":608348,"locations":[{"id":123}]}}`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg, _ := config.Load("")
	cfg.BaseURL = server.URL
	_ = client.New(cfg, 0, 0)

	cfgPath := filepath.Join(tempHome, "config", "fooda_config.json")
	_ = os.MkdirAll(filepath.Join(tempHome, "config"), 0o700)
	cfgJSON, _ := json.Marshal(map[string]any{"base_url": server.URL})
	_ = os.WriteFile(cfgPath, cfgJSON, 0o600)

	tests := []struct {
		name          string
		pricingBody   string
		expectedError string
	}{
		{"Malformed JSON", `{"data":{"pricing": malformed-json}}`, "malformed pricing JSON response"},
		{"Null Total", `{"data":{"pricing":{"total_cents": null, "subtotal_cents": 1000}}}`, "pricing total_cents is missing, null, or negative"},
		{"Missing Pricing", `{"data":{}}`, "pricing total_cents is missing, null, or negative"},
		{"String Value", `{"data":{"pricing":{"total_cents": "not-a-number", "subtotal_cents": 1000}}}`, "malformed pricing JSON response"},
		{"Negative Total", `{"data":{"pricing":{"total_cents": -500, "subtotal_cents": 1000}}}`, "pricing total_cents is missing, null, or negative"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ordersPostCount = 0
			cartPriceCount = 0
			pricingJSON = tt.pricingBody

			cmd := RootCmd()
			cmd.SetArgs([]string{"order", "place", "--event", "S608348", "--confirm", "--account", "over-acct", "--building", "over-bldg"})
			cmd.PersistentFlags().Set("config", cfgPath)

			var stdout, stderr bytes.Buffer
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)

			err := cmd.Execute()
			if err == nil {
				t.Fatalf("expected order place to fail, but succeeded")
			}
			if ordersPostCount != 0 {
				t.Errorf("expected 0 calls to /api/v1/orders, but got %d", ordersPostCount)
			}
			if cartPriceCount != 1 {
				t.Errorf("expected 1 call to /api/v1/cart/price, but got %d", cartPriceCount)
			}
			if !strings.Contains(err.Error(), tt.expectedError) {
				t.Errorf("expected error containing %q, got: %v", tt.expectedError, err)
			}
		})
	}
}

func TestOrderAddVendorFilter(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("FOODA_HOME", tempHome)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`
<html>
<body>
<div class="item" data-vendor_name="Taco Shop">
  <a href="/items/1" class="item__link js-item-show-link">
    <div class="item__name">Taco</div>
  </a>
</div>
<div class="item" data-vendor_name="Burrito Shop">
  <a href="/items/2" class="item__link js-item-show-link">
    <div class="item__name">Burrito</div>
  </a>
</div>
</body>
</html>
		`))
	}))
	defer server.Close()

	cfg, _ := config.Load("")
	cfg.BaseURL = server.URL
	_ = client.New(cfg, 0, 0)

	cmd := RootCmd()
	cmd.SetArgs([]string{"order", "add", "Burrito", "--event", "S608348", "--vendor", "Taco Shop", "--account", "over-acct", "--building", "over-bldg"})
	cfgPath := filepath.Join(tempHome, "config", "fooda_config.json")
	_ = os.MkdirAll(filepath.Join(tempHome, "config"), 0o700)
	cfgJSON, _ := json.Marshal(map[string]any{"base_url": server.URL})
	_ = os.WriteFile(cfgPath, cfgJSON, 0o600)
	cmd.PersistentFlags().Set("config", cfgPath)

	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected order add to fail since Burrito is not sold by Taco Shop")
	}
}

func TestOrderAddOptionAmbiguity(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("FOODA_HOME", tempHome)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if strings.Contains(r.URL.Path, "/items/") {
			// Item details page with ambiguous options
			_, _ = w.Write([]byte(`
<html>
<body>
<form>
  <label><input name="info[options][]" value="101">Extra Sauce (+$1.00)</label>
  <label><input name="info[options][]" value="102">Extra Cheese (+$2.00)</label>
</form>
</body>
</html>
			`))
			return
		}
		_, _ = w.Write([]byte(`
<html>
<body>
<div class="item" data-vendor_name="Yum Yubu">
  <a href="/items/1" class="item__link js-item-show-link">
    <div class="item__name">Beef Bowl</div>
  </a>
</div>
</body>
</html>
		`))
	}))
	defer server.Close()

	cfg, _ := config.Load("")
	cfg.BaseURL = server.URL
	_ = client.New(cfg, 0, 0)

	cmd := RootCmd()
	// Option "Extra" is ambiguous as it matches both "Extra Sauce" and "Extra Cheese"
	cmd.SetArgs([]string{"order", "add", "Beef Bowl", "--event", "S608348", "--option", "Extra", "--account", "over-acct", "--building", "over-bldg"})
	cfgPath := filepath.Join(tempHome, "config", "fooda_config.json")
	_ = os.MkdirAll(filepath.Join(tempHome, "config"), 0o700)
	cfgJSON, _ := json.Marshal(map[string]any{"base_url": server.URL})
	_ = os.WriteFile(cfgPath, cfgJSON, 0o600)
	cmd.PersistentFlags().Set("config", cfgPath)

	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected order add to fail due to ambiguous option 'Extra'")
	}
	if !strings.Contains(err.Error(), "ambiguous option") {
		t.Errorf("unexpected error message: %v", err)
	}
}
