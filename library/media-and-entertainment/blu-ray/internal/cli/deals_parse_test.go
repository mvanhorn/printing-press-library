package cli

import (
	"os"
	"testing"
)

func TestParseDealsHTMLHappyPath(t *testing.T) {
	body := []byte(`<html><body><table><tr>
		<td><a class="active" href="https://www.blu-ray.com/link/click.php?p=1&retailerid=7">Buy</a></td>
		<td><a href="https://www.blu-ray.com/movies/Example-4K-Blu-ray/12345/">DETAILS</a><b title="2 hours ago"></b>$14.99 $29.99 50%</td>
	</tr></table></body></html>`)
	if fixture, err := os.ReadFile("/tmp/printing-press/blu-ray-probe/deals.html"); err == nil && len(fixture) > 0 {
		body = fixture
	}

	rows, err := parseDealsHTML(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 {
		t.Fatal("expected at least one parsed deal row")
	}
	if rows[0].ReleaseID == 0 || rows[0].SalePrice == 0 {
		t.Fatalf("row missing id or sale price: %+v", rows[0])
	}
}

func TestFilterDealRowsHappyPath(t *testing.T) {
	rows := []DealRow{
		{ReleaseID: 1, SalePrice: 24.99, PercentOff: 20},
		{ReleaseID: 2, SalePrice: 14.99, PercentOff: 50},
		{ReleaseID: 3, SalePrice: 9.99, PercentOff: 45},
	}

	got := filterDealRows(rows, 40, 15, 1)
	if len(got) != 1 {
		t.Fatalf("expected 1 filtered row, got %d: %+v", len(got), got)
	}
	if got[0].ReleaseID != 2 {
		t.Fatalf("expected release 2 after discount, price, and limit filters, got %+v", got[0])
	}
}

func TestPriceRegexAcceptsSubDollarAndRejectsBareDollar(t *testing.T) {
	t.Parallel()

	// PATCH: Sub-dollar prices are valid, but a bare dollar sign is not.
	matches := priceRE.FindAllStringSubmatch("$.99 $0 $ alone", -1)
	if len(matches) != 2 {
		t.Fatalf("price matches = %d, want 2: %#v", len(matches), matches)
	}
	if got := normalizePriceMatch(matches[0][1]); got != "0.99" {
		t.Fatalf("first price = %q, want 0.99", got)
	}
	if got := normalizePriceMatch(matches[1][1]); got != "0" {
		t.Fatalf("second price = %q, want 0", got)
	}
}

func TestParseDealsHTMLRejectsZeroDollarSalePrice(t *testing.T) {
	t.Parallel()

	// PATCH: Preserve the existing SalePrice > 0 gate after price regex widening.
	body := []byte(`<html><body><table><tr>
		<td><a class="active" href="https://www.blu-ray.com/link/click.php?p=1&retailerid=7">Buy</a></td>
		<td><a href="https://www.blu-ray.com/movies/Example-Blu-ray/12345/">DETAILS</a><b title="2 hours ago"></b>$0</td>
	</tr></table></body></html>`)

	rows, err := parseDealsHTML(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("rows len = %d, want $0 sale row rejected: %+v", len(rows), rows)
	}
}
