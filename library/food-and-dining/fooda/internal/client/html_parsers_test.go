package client

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParsePastOrders(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "settings_orders.html"))
	if err != nil {
		t.Fatalf("failed to read settings_orders.html: %v", err)
	}

	props, err := ParsePastOrders(string(data))
	if err != nil {
		t.Fatalf("failed to parse past orders: %v", err)
	}

	if len(props.Presenter) != 1 {
		t.Errorf("expected 1 order, got %d", len(props.Presenter))
	}
	ord := props.Presenter[0].Order
	if ord.ID != 1111111 {
		t.Errorf("expected ID 1111111, got %d", ord.ID)
	}
	if ord.RequestID != "11111111-2222-3333-4444-555555555555" {
		t.Errorf("expected RequestID, got %s", ord.RequestID)
	}
}

func TestParseOrderDetail(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "order_detail.html"))
	if err != nil {
		t.Fatalf("failed to read order_detail.html: %v", err)
	}

	props, err := ParseOrderDetail(string(data))
	if err != nil {
		t.Fatalf("failed to parse order detail: %v", err)
	}

	if props.OrderData.OrderUuid != "11111111-2222-3333-4444-555555555555" {
		t.Errorf("expected OrderUuid, got %s", props.OrderData.OrderUuid)
	}
	if props.OrderData.FormattedVendorNames != "Scrubbed Taco Shop" {
		t.Errorf("expected vendor, got %s", props.OrderData.FormattedVendorNames)
	}
}

func TestParseMyPage(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "my.html"))
	if err != nil {
		t.Fatalf("failed to read my.html: %v", err)
	}

	acct, bldg, err := ParseMyPage(string(data))
	if err != nil {
		t.Fatalf("failed to parse my page: %v", err)
	}

	if acct != "1234" {
		t.Errorf("expected account 1234, got %s", acct)
	}
	if bldg != "5678" {
		t.Errorf("expected building 5678, got %s", bldg)
	}
}

func TestParseMenuItems(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "menu_items.html"))
	if err != nil {
		t.Fatalf("failed to read menu_items.html: %v", err)
	}

	items, err := ParseMenuItems(string(data))
	if err != nil {
		t.Fatalf("failed to parse menu items: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	item := items[0]
	if item.VendorName != "The SSam" {
		t.Errorf("expected vendor, got %s", item.VendorName)
	}
	if item.Name != "Tofu SSam" {
		t.Errorf("expected name, got %s", item.Name)
	}
	if item.Price != 13.99 {
		t.Errorf("expected price 13.99, got %f", item.Price)
	}
	if item.Description != "Tofu, Egg, Cucumber, Carrot, Apple" {
		t.Errorf("expected description, got %q", item.Description)
	}
}

func TestParseHTML_ReorderedAndSingleQuoted(t *testing.T) {
	// attributes single-quoted and reordered: props first, then id
	htmlStr := `<html><body><div props='{"filters":{"account_id":["9999"],"locations":{"building_id":["8888"]}},"userId":777}' id='my-event-list'></div></body></html>`
	acct, bldg, err := ParseMyPage(htmlStr)
	if err != nil {
		t.Fatalf("failed to parse with reordered/single-quoted attributes: %v", err)
	}
	if acct != "9999" || bldg != "8888" {
		t.Errorf("expected acct 9999, bldg 8888; got acct=%s, bldg=%s", acct, bldg)
	}
}
