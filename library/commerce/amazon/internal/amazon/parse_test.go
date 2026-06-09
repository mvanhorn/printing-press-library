package amazon

import "testing"

const sampleCart = `
<div data-asin="B07ABCD123" data-quantity="2" data-item-name="Charmin Ultra Strong 24 Mega Rolls" data-fooie="x">
</div>
<div data-asin="B0XYZ987GH" data-quantity="1" data-item-name="AAA Batteries &amp; Stuff 48-pack">
</div>
`

const sampleCartAlt = `
<div data-item-asin="B0ALT00001" data-item-quantity="3" data-item-title="Whole Bean Coffee 2lb">
</div>
`

func TestParseCartHTML_Primary(t *testing.T) {
	lines := parseCartHTML(sampleCart)
	if len(lines) != 2 {
		t.Fatalf("want 2 lines, got %d", len(lines))
	}
	if lines[0].ASIN != "B07ABCD123" {
		t.Errorf("line[0].ASIN = %q", lines[0].ASIN)
	}
	if lines[0].Quantity != 2 {
		t.Errorf("line[0].Quantity = %d", lines[0].Quantity)
	}
	if lines[1].Title != "AAA Batteries & Stuff 48-pack" {
		t.Errorf("line[1].Title decoded incorrectly: %q", lines[1].Title)
	}
}

func TestParseCartHTML_Alt(t *testing.T) {
	lines := parseCartHTML(sampleCartAlt)
	if len(lines) != 1 {
		t.Fatalf("want 1 line, got %d", len(lines))
	}
	if lines[0].ASIN != "B0ALT00001" || lines[0].Quantity != 3 {
		t.Errorf("bad alt parse: %+v", lines[0])
	}
}

func TestParseCartHTML_DedupeQuantities(t *testing.T) {
	dup := sampleCart + sampleCart
	lines := parseCartHTML(dup)
	if len(lines) != 2 {
		t.Fatalf("dedupe expected 2 distinct ASINs, got %d", len(lines))
	}
	if lines[0].Quantity != 4 {
		t.Errorf("dedupe should sum quantities: got %d, want 4", lines[0].Quantity)
	}
}

func TestParseSPCTokensRejectsNonCheckoutPage(t *testing.T) {
	if _, err := parseSPCTokens("<html><body>regular cart page</body></html>"); err == nil {
		t.Fatal("expected error on non-checkout page")
	}
}

func TestParseSPCTokensExtractsKnown(t *testing.T) {
	page := `
		<form id="spc-place-order-form">
			<input type="hidden" name="purchase_id" value="amzn1.spc.purchase.abc"/>
			<input type="hidden" name="anti-csrftoken-a2z" value="csrf-1234"/>
			<input type="hidden" name="ignored-marketing-blob" value="X"/>
			<button id="spc-place-order-button">Place your order</button>
		</form>
	`
	tokens, err := parseSPCTokens(page)
	if err != nil {
		t.Fatalf("parseSPCTokens: %v", err)
	}
	if tokens["purchase_id"] != "amzn1.spc.purchase.abc" {
		t.Errorf("purchase_id missing: %#v", tokens)
	}
	if tokens["anti-csrftoken-a2z"] != "csrf-1234" {
		t.Errorf("anti-csrftoken-a2z missing: %#v", tokens)
	}
	if _, blocked := tokens["ignored-marketing-blob"]; blocked {
		t.Errorf("non-interesting token leaked through: %#v", tokens)
	}
}

func TestExtractThankYouOrderID(t *testing.T) {
	cases := []struct {
		body, want string
	}{
		{"Thanks! Order # 112-3456789-1234567 placed.", "112-3456789-1234567"},
		{"Order number: 999-1234567-7654321", "999-1234567-7654321"},
		{"no order here", ""},
	}
	for _, c := range cases {
		if got := extractThankYouOrderID(c.body); got != c.want {
			t.Errorf("extractThankYouOrderID(%q)=%q want %q", c.body, got, c.want)
		}
	}
}

func TestParseSPCTokensAttributeOrderAgnostic(t *testing.T) {
	page := `
		<form id="spc-place-order-form">
			<input name="purchase_id" type="hidden" value="amzn1.spc.purchase.def"/>
			<input value="csrf-5678" name="anti-csrftoken-a2z" type="hidden"/>
			<input name="not-hidden-token" value="X" type="text"/>
			<button id="spc-place-order-button">Place your order</button>
		</form>
	`
	tokens, err := parseSPCTokens(page)
	if err != nil {
		t.Fatalf("parseSPCTokens: %v", err)
	}
	if tokens["purchase_id"] != "amzn1.spc.purchase.def" {
		t.Errorf("purchase_id missing with name-first attribute order: %#v", tokens)
	}
	if tokens["anti-csrftoken-a2z"] != "csrf-5678" {
		t.Errorf("anti-csrftoken-a2z missing with value-first attribute order: %#v", tokens)
	}
	if _, leaked := tokens["not-hidden-token"]; leaked {
		t.Errorf("non-hidden input leaked through: %#v", tokens)
	}
}
