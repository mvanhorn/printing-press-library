package parser

import (
	"testing"
)

func TestExtractOrderID(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"ORDER # 111-1111111-1111111 next stuff", "111-1111111-1111111"},
		{"D01-1111111-1111111 digital order", "D01-1111111-1111111"},
		{"no order id here", ""},
		{"foo 12-1234567-1234567 short prefix won't match", ""},
	}
	for _, tt := range tests {
		got := ExtractOrderID(tt.in)
		if got != tt.want {
			t.Errorf("ExtractOrderID(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestExtractASIN(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"https://www.amazon.com/dp/B0EXAMPLE1", "B0EXAMPLE1"},
		{"/dp/B0EXAMPLE4/ref=ci_mcx", "B0EXAMPLE4"},
		{"/dp/short", ""},
		{"https://www.amazon.com/EPICKA-Universal/dp/B0EXAMPLE4/ref=...", "B0EXAMPLE4"},
		{"no /dp/ here", ""},
	}
	for _, tt := range tests {
		got := ExtractASIN(tt.in)
		if got != tt.want {
			t.Errorf("ExtractASIN(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestExtractMoney(t *testing.T) {
	tests := []struct {
		in   string
		want float64
	}{
		{"TOTAL $51.46 SHIP TO", 51.46},
		{"$1,234.56", 1234.56},
		{"-$5.00", -5.0},
		{"TOTAL ₹1,234.56 SHIP TO", 1234.56},
		{"TOTAL Rs. 999.00", 999.00},
		{"TOTAL INR 2,500.00", 2500.00},
		{"-₹5.00", -5.0},
		{"no money here", 0},
		{"$ 12.34 spaced", 12.34},
	}
	for _, tt := range tests {
		got := ExtractMoney(tt.in)
		if got != tt.want {
			t.Errorf("ExtractMoney(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestExtractMoneyWithCurrency(t *testing.T) {
	tests := []struct {
		in           string
		wantAmount   float64
		wantCurrency string
		wantOK       bool
	}{
		{"TOTAL ₹1,234.56", 1234.56, "INR", true},
		{"TOTAL Rs. 999.00", 999.00, "INR", true},
		{"TOTAL INR 2,500.00", 2500.00, "INR", true},
		{"TOTAL $51.46", 51.46, "USD", true},
		{"TOTAL £12.34", 12.34, "GBP", true},
		{"no money", 0, "", false},
	}
	for _, tt := range tests {
		gotAmount, gotCurrency, gotOK := ExtractMoneyWithCurrency(tt.in)
		if gotOK != tt.wantOK || gotAmount != tt.wantAmount || gotCurrency != tt.wantCurrency {
			t.Errorf("ExtractMoneyWithCurrency(%q) = (%v, %q, %v), want (%v, %q, %v)", tt.in, gotAmount, gotCurrency, gotOK, tt.wantAmount, tt.wantCurrency, tt.wantOK)
		}
	}
}

func TestParseDate(t *testing.T) {
	tests := []struct {
		in          string
		wantNonZero bool
		wantDay     int
	}{
		{"May 5, 2026", true, 5},
		{"May 5", true, 5},
		{"Jan 1, 2026", true, 1},
		{"May 5 - May 7, 2026", true, 7},
		{"not a date", false, 0},
		{"", false, 0},
	}
	for _, tt := range tests {
		got := ParseDate(tt.in)
		if tt.wantNonZero && got.IsZero() {
			t.Errorf("ParseDate(%q) returned zero, expected non-zero", tt.in)
		}
		if !tt.wantNonZero && !got.IsZero() {
			t.Errorf("ParseDate(%q) returned %v, expected zero", tt.in, got)
		}
		if tt.wantNonZero && got.Day() != tt.wantDay {
			t.Errorf("ParseDate(%q) day = %d, want %d", tt.in, got.Day(), tt.wantDay)
		}
	}
}

func TestExtractLast4(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"Visa ending in 1234", "1234"},
		{"Mastercard ****1234", "1234"},
		{"no card here", ""},
	}
	for _, tt := range tests {
		got := ExtractLast4(tt.in)
		if got != tt.want {
			t.Errorf("ExtractLast4(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestParseOrderListMinimal(t *testing.T) {
	// Synthetic order-card sample matching Amazon's structure.
	html := `<html><body>
<div class="order-card js-order-card">
  <div class="a-box-group">
    <span>ORDER PLACED</span><span>May 5, 2026</span>
    <span>TOTAL</span><span>$51.46</span>
    <span>SHIP TO</span><span>Test User</span>
    <span>ORDER # 111-1111111-1111111</span>
    <a href="/your-orders/order-details?orderID=111-1111111-1111111">View order details</a>
    <a href="/gp/your-account/ship-track?orderId=111-1111111-1111111">Track package</a>
    <a href="/dp/B0EXAMPLE1">Test Product Title</a>
    <span>Arriving May 20, 2026</span>
  </div>
</div>
</body></html>`
	page, err := ParseOrderList([]byte(html))
	if err != nil {
		t.Fatalf("ParseOrderList: %v", err)
	}
	if len(page.Orders) != 1 {
		t.Fatalf("got %d orders, want 1", len(page.Orders))
	}
	o := page.Orders[0]
	if o.OrderID != "111-1111111-1111111" {
		t.Errorf("OrderID = %q", o.OrderID)
	}
	if o.Total != 51.46 {
		t.Errorf("Total = %v, want 51.46", o.Total)
	}
	if o.PlacedDate != "2026-05-05" {
		t.Errorf("PlacedDate = %q, want 2026-05-05", o.PlacedDate)
	}
	if o.ShipTo != "Test User" {
		t.Errorf("ShipTo = %q", o.ShipTo)
	}
	if o.Status != "Arriving" {
		t.Errorf("Status = %q, want Arriving", o.Status)
	}
	if o.ETADate != "2026-05-20" {
		t.Errorf("ETADate = %q, want 2026-05-20", o.ETADate)
	}
	if len(o.ASINs) != 1 || o.ASINs[0] != "B0EXAMPLE1" {
		t.Errorf("ASINs = %v, want [B0EXAMPLE1]", o.ASINs)
	}
}

func TestParseOrderListINRTotalAndCurrency(t *testing.T) {
	html := `<html><body>
<div class="order-card js-order-card">
  <span>ORDER PLACED</span><span>May 5, 2026</span>
  <span>TOTAL</span><span>₹1,234.56</span>
  <span>SHIP TO</span><span>Test User</span>
  <span>ORDER # 111-1111111-1111111</span>
</div>
</body></html>`
	page, err := ParseOrderList([]byte(html))
	if err != nil {
		t.Fatalf("ParseOrderList: %v", err)
	}
	if len(page.Orders) != 1 {
		t.Fatalf("got %d orders, want 1", len(page.Orders))
	}
	o := page.Orders[0]
	if o.Total != 1234.56 {
		t.Errorf("Total = %v, want 1234.56", o.Total)
	}
	if o.Currency != "INR" {
		t.Errorf("Currency = %q, want INR", o.Currency)
	}
}

func TestExtractStatus(t *testing.T) {
	tests := []struct {
		in     string
		status string
	}{
		{"Delivered May 5", "Delivered"},
		{"Arriving May 20", "Arriving"},
		{"Out for delivery", "Out for delivery"},
		{"Cancelled", "Cancelled"},
		{"Shipped via UPS", "Shipped"},
		{"Preparing for shipment", "Preparing"},
		{"random text", ""},
	}
	for _, tt := range tests {
		st, _, _ := extractStatus(tt.in)
		if st != tt.status {
			t.Errorf("extractStatus(%q) = %q, want %q", tt.in, st, tt.status)
		}
	}
}
