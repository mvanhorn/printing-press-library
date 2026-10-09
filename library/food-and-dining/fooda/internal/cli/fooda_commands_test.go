package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/fooda/internal/client"
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
	dbPath := filepath.Join(t.TempDir(), "test_store.db")
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

	o1.ID = 1001
	o1.EventType = "delivery"
	o1.ItemNames = []string{"Scrubbed Burrito"}
	o1.OrderFulfilledTimeUnix = time.Now().Add(-2 * 24 * time.Hour).Unix()
	o1.VendorNames = []string{"Scrubbed Burrito Shop"}
	o1.Status = "checkout_complete"
	o1.Delivery = "Delivered today"
	o1.RequestID = "uuid-1111"
	o1.PaymentCents = 1500

	o2.ID = 1002
	o2.EventType = "delivery"
	o2.ItemNames = []string{"Scrubbed Taco"}
	o2.OrderFulfilledTimeUnix = time.Now().Add(-5 * 24 * time.Hour).Unix()
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

	ordersJSON, _ := json.Marshal(mockOrdersList)
	if err := db.Upsert("order", "orders_list", ordersJSON); err != nil {
		t.Fatalf("failed to upsert orders list: %v", err)
	}

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

	// Validate DB content is accessible
	list, err := db.List("order", 10)
	if err != nil {
		t.Fatalf("failed to list orders from DB: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("expected 1 list row, got %d", len(list))
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
