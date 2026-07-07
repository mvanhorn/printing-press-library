package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/spf13/cobra"
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

func TestFilterDealRowsMaxPriceGate(t *testing.T) {
	t.Parallel()

	rows := []DealRow{
		{ReleaseID: 1, SalePrice: 14.99, PercentOff: 50},
		{ReleaseID: 2, SalePrice: 15.01, PercentOff: 50},
	}

	got := filterDealRows(rows, 40, 15, 0)
	if len(got) != 1 {
		t.Fatalf("expected 1 filtered row, got %d: %+v", len(got), got)
	}
	if got[0].ReleaseID != 1 {
		t.Fatalf("expected release 1 after max price gate, got %+v", got[0])
	}
}

func TestFilterDealRowsMinDiscountGate(t *testing.T) {
	t.Parallel()

	rows := []DealRow{
		{ReleaseID: 1, SalePrice: 14.99, PercentOff: 40},
		{ReleaseID: 2, SalePrice: 14.99, PercentOff: 39},
	}

	got := filterDealRows(rows, 40, 15, 0)
	if len(got) != 1 {
		t.Fatalf("expected 1 filtered row, got %d: %+v", len(got), got)
	}
	if got[0].ReleaseID != 1 {
		t.Fatalf("expected release 1 after min discount gate, got %+v", got[0])
	}
}

func TestPrintDealRowsWrapsProvenanceAndHonorsSelect(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)
	flags := &rootFlags{asJSON: true, selectFields: "release_id,title"}
	rows := []DealRow{
		{
			ReleaseID:  123,
			Title:      "Example Deal",
			Kind:       "4k",
			SalePrice:  14.99,
			ListPrice:  29.99,
			PercentOff: 50,
			PostedAgo:  "2 hours ago",
			RetailerID: 7,
			DetailURL:  "https://www.blu-ray.com/movies/Example-4K-Blu-ray/123/",
		},
	}

	if err := printDealRows(cmd, flags, rows, DataProvenance{Source: "live"}); err != nil {
		t.Fatal(err)
	}

	var got struct {
		Results []map[string]json.RawMessage `json:"results"`
		Meta    map[string]string            `json:"meta"`
	}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal output: %v\n%s", err, buf.String())
	}
	if got.Meta["source"] != "live" {
		t.Fatalf("meta.source = %q, want live; output: %s", got.Meta["source"], buf.String())
	}
	if len(got.Results) != 1 {
		t.Fatalf("results len = %d, want 1; output: %s", len(got.Results), buf.String())
	}
	if _, ok := got.Results[0]["release_id"]; !ok {
		t.Fatalf("selected release_id missing: %#v", got.Results[0])
	}
	if _, ok := got.Results[0]["title"]; !ok {
		t.Fatalf("selected title missing: %#v", got.Results[0])
	}
	if _, ok := got.Results[0]["sale_price"]; ok {
		t.Fatalf("unselected sale_price present after --select: %#v", got.Results[0])
	}
}

func TestPrintDealRowsHonorsCompact(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)
	// --compact, no --select: exercises the compactFields branch of printDealRows.
	flags := &rootFlags{asJSON: true, compact: true}
	// cover_url is populated on only 1 of 5 rows (20% < the 80% keep threshold),
	// so compactFields must drop it; core fields (release_id/title) survive.
	rows := []DealRow{
		{ReleaseID: 1, Title: "A", Kind: "4k", SalePrice: 9.99, PostedAgo: "1h", RetailerID: 7, DetailURL: "u1", CoverURL: "c1"},
		{ReleaseID: 2, Title: "B", Kind: "4k", SalePrice: 8.99, PostedAgo: "2h", RetailerID: 7, DetailURL: "u2"},
		{ReleaseID: 3, Title: "C", Kind: "4k", SalePrice: 7.99, PostedAgo: "3h", RetailerID: 7, DetailURL: "u3"},
		{ReleaseID: 4, Title: "D", Kind: "4k", SalePrice: 6.99, PostedAgo: "4h", RetailerID: 7, DetailURL: "u4"},
		{ReleaseID: 5, Title: "E", Kind: "4k", SalePrice: 5.99, PostedAgo: "5h", RetailerID: 7, DetailURL: "u5"},
	}

	// Source "local" (not "live") also covers a non-live provenance value.
	if err := printDealRows(cmd, flags, rows, DataProvenance{Source: "local"}); err != nil {
		t.Fatal(err)
	}

	var got struct {
		Results []map[string]json.RawMessage `json:"results"`
		Meta    map[string]json.RawMessage   `json:"meta"`
	}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal output: %v\n%s", err, buf.String())
	}
	// Provenance envelope must survive the compact path (the P1 contract).
	if string(got.Meta["source"]) != `"local"` {
		t.Fatalf("meta.source = %s, want \"local\"; output: %s", got.Meta["source"], buf.String())
	}
	if len(got.Results) != 5 {
		t.Fatalf("results len = %d, want 5; output: %s", len(got.Results), buf.String())
	}
	// compactFields had to run: cover_url (20% of rows) is dropped, core kept.
	// Mutation guard — if printDealRows skips compactFields, cover_url survives
	// on row 1 and this fails.
	if _, ok := got.Results[0]["cover_url"]; ok {
		t.Fatalf("cover_url should be dropped by --compact (20%% of rows): %#v", got.Results[0])
	}
	if _, ok := got.Results[0]["release_id"]; !ok {
		t.Fatalf("core field release_id missing after --compact: %#v", got.Results[0])
	}
	if _, ok := got.Results[0]["title"]; !ok {
		t.Fatalf("core field title missing after --compact: %#v", got.Results[0])
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
