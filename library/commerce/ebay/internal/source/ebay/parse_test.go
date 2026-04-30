package ebay

import (
	"testing"
	"time"
)

func TestParseSearchHTML_ExtractsListing(t *testing.T) {
	body := []byte(`
<html><body>
<ul class="srp-results srp-list">
  <li class="s-card s-card--horizontal" data-listingid="306898461995">
    <a class="s-card__link" href="https://www.ebay.com/itm/306898461995?_skw=panini">link</a>
    <div class="s-card__title">Panini Donruss Optic NBA 2022-23 6 Card Lot Pritchard Murphy</div>
    <div class="s-card__price">$4.99</div>
    <div class="s-card__caption">2 bids · Time left 1m 4s left</div>
  </li>
  <li class="s-card s-card--horizontal" data-listingid="0">
    <div class="s-card__title">Shop on eBay</div>
    <div class="s-card__price">$20.00</div>
  </li>
</ul>
</body></html>`)
	got, err := ParseSearchHTML(body)
	if err != nil {
		t.Fatalf("ParseSearchHTML returned %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 real listing (sponsored Shop on eBay placeholder skipped), got %d", len(got))
	}
	if got[0].ItemID != "306898461995" {
		t.Errorf("ItemID = %q, want 306898461995", got[0].ItemID)
	}
	if got[0].Price != 4.99 {
		t.Errorf("Price = %v, want 4.99", got[0].Price)
	}
	if got[0].Bids != 2 {
		t.Errorf("Bids = %d, want 2", got[0].Bids)
	}
}

func TestParseSoldHTML_ExtractsSoldItem(t *testing.T) {
	body := []byte(`
<html><body>
<ul class="srp-results srp-list">
  <li class="s-card s-card--horizontal" data-listingid="137265224820">
    <a class="s-card__link" href="https://www.ebay.com/itm/137265224820">link</a>
    <div class="s-card__title">2025-26 Topps Chrome Cooper Flagg Gold Refractor /50</div>
    <div class="s-card__price">$3,500.00</div>
    <div class="s-card__caption">Sold Apr 30, 2026</div>
  </li>
  <li class="s-card s-card--horizontal" data-listingid="267653197023">
    <a class="s-card__link" href="https://www.ebay.com/itm/267653197023">link</a>
    <div class="s-card__title">Cooper Flagg Topps Chrome Sapphire Auto /50</div>
    <div class="s-card__price">$14,550.00</div>
    <div class="s-card__caption">Sold Apr 30, 2026</div>
  </li>
</ul>
</body></html>`)
	got, err := ParseSoldHTML(body)
	if err != nil {
		t.Fatalf("ParseSoldHTML returned %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 sold items, got %d", len(got))
	}
	if got[0].SoldPrice != 3500.00 {
		t.Errorf("SoldPrice[0] = %v, want 3500.00", got[0].SoldPrice)
	}
	if got[0].SoldDate.IsZero() {
		t.Errorf("SoldDate[0] is zero; expected parsed date")
	}
	wantDate := time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)
	if !got[0].SoldDate.Equal(wantDate) {
		t.Errorf("SoldDate[0] = %v, want %v", got[0].SoldDate, wantDate)
	}
	if got[1].SoldPrice != 14550.00 {
		t.Errorf("SoldPrice[1] = %v, want 14550.00", got[1].SoldPrice)
	}
}

func TestExtractTimeLeft(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{"days+hours", "Time left 3d 11h left (Sun, 07:30 PM)", "3d 11h"},
		{"hours+minutes", "Time left 1h 4m left", "1h 4m"},
		{"minutes only", "Time left 9m 30s left", "9m 30s"},
		{"seconds", "Time left 44s left", "44s left"},
		{"missing", "Time left absent", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractTimeLeft(tt.text); got != tt.want {
				t.Errorf("extractTimeLeft(%q) = %q, want %q", tt.text, got, tt.want)
			}
		})
	}
}

func TestParseTimeLeftDuration(t *testing.T) {
	tests := []struct {
		text string
		want time.Duration
	}{
		{"3d 11h", 3*24*time.Hour + 11*time.Hour},
		{"1h 4m", time.Hour + 4*time.Minute},
		{"9m 30s", 9 * time.Minute},
		{"44s left", 44 * time.Second},
		{"junk", 0},
	}
	for _, tt := range tests {
		got := parseTimeLeftDuration(tt.text)
		if got != tt.want {
			t.Errorf("parseTimeLeftDuration(%q) = %v, want %v", tt.text, got, tt.want)
		}
	}
}

func TestExtractBidTokens(t *testing.T) {
	body := []byte(`
<html>
  <input type="hidden" name="srt" value="01000b00000050abc123def" />
  <script>window.__bootstrap = {"forterToken": "deadbeef_1777562166738__UDF43_15ck_tt", "other": 1};</script>
</html>`)
	srt, forter := ExtractBidTokens(body)
	if srt != "01000b00000050abc123def" {
		t.Errorf("srt = %q, want 01000b00000050abc123def", srt)
	}
	if forter != "deadbeef_1777562166738__UDF43_15ck_tt" {
		t.Errorf("forterToken = %q, want deadbeef_1777562166738__UDF43_15ck_tt", forter)
	}
}
