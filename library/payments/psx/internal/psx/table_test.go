// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package psx

import "testing"

func TestNormalizeHeader(t *testing.T) {
	cases := []struct{ in, want string }{
		{"SYMBOL", "symbol"},
		{"CHANGE (%)", "change_pct"},
		{"MARKET CAP. (B)", "market_cap_b"},
		{"PE RATIO (TTM)", "pe_ratio_ttm"},
		{"1-YEAR CH. (%) *", "1_year_ch_pct"},
		{"30D VOLUME AVG.", "30d_volume_avg"},
		{"LISTED IN", "listed_in"},
		{"  Sector  Name  ", "sector_name"},
		{"", ""},
	}
	for _, c := range cases {
		if got := NormalizeHeader(c.in); got != c.want {
			t.Errorf("NormalizeHeader(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

const marketWatchFragment = `
<div class="tbl__wrapper"><table class="tbl" id="marketWatchTable">
<thead class="tbl__head"><tr>
  <th>SYMBOL</th><th>SECTOR</th><th>LDCP</th><th>CURRENT</th><th>CHANGE (%)</th><th>VOLUME</th>
</tr></thead>
<tbody class="tbl__body">
  <tr><td>OGDC</td><td>0821</td><td>316.10</td><td>314.33</td><td>-0.56%</td><td>4,560,170</td></tr>
  <tr><td>LUCK</td><td>0813</td><td>800.00</td><td>812.50</td><td>1.56%</td><td>1,200,000</td></tr>
</tbody></table></div>`

func TestParseTables_HeaderKeyedRows(t *testing.T) {
	tables, err := ParseTables(marketWatchFragment)
	if err != nil {
		t.Fatalf("ParseTables: %v", err)
	}
	if len(tables) != 1 {
		t.Fatalf("want 1 table, got %d", len(tables))
	}
	tbl := tables[0]
	if tbl.ID != "marketWatchTable" {
		t.Errorf("ID = %q, want marketWatchTable", tbl.ID)
	}
	if len(tbl.Rows) != 2 {
		t.Fatalf("want 2 rows, got %d", len(tbl.Rows))
	}
	if got := tbl.Rows[0]["symbol"]; got != "OGDC" {
		t.Errorf("row0 symbol = %q, want OGDC", got)
	}
	if got := tbl.Rows[0]["change_pct"]; got != "-0.56%" {
		t.Errorf("row0 change_pct = %q, want -0.56%%", got)
	}
	if got := tbl.Rows[1]["volume"]; got != "1,200,000" {
		t.Errorf("row1 volume = %q, want 1,200,000", got)
	}
}

// TestParseTables_ColumnReorderIsNoOp is the whole point of header-name-driven
// parsing: PSX reorders columns without notice, and a position-indexed parser
// would silently return wrong-but-plausible numbers.
func TestParseTables_ColumnReorderIsNoOp(t *testing.T) {
	reordered := `<table><tr><th>VOLUME</th><th>SYMBOL</th><th>CURRENT</th></tr>
	<tr><td>4,560,170</td><td>OGDC</td><td>314.33</td></tr></table>`
	tables, err := ParseTables(reordered)
	if err != nil {
		t.Fatalf("ParseTables: %v", err)
	}
	row := tables[0].Rows[0]
	if row["symbol"] != "OGDC" || row["current"] != "314.33" || row["volume"] != "4,560,170" {
		t.Errorf("reordered columns mis-keyed: %v", row)
	}
}

func TestParseTables_UnescapesEntitiesOnce(t *testing.T) {
	frag := `<table><tr><th>NAME</th></tr><tr><td>Oil &amp; Gas Development</td></tr></table>`
	tables, err := ParseTables(frag)
	if err != nil {
		t.Fatalf("ParseTables: %v", err)
	}
	if got := tables[0].Rows[0]["name"]; got != "Oil & Gas Development" {
		t.Errorf("name = %q, want unescaped ampersand", got)
	}
}

func TestParseTables_DuplicateHeadersDisambiguated(t *testing.T) {
	frag := `<table><tr><th>PRICE</th><th>PRICE</th></tr><tr><td>1</td><td>2</td></tr></table>`
	tables, err := ParseTables(frag)
	if err != nil {
		t.Fatalf("ParseTables: %v", err)
	}
	row := tables[0].Rows[0]
	if row["price"] != "1" || row["price_2"] != "2" {
		t.Errorf("duplicate headers not disambiguated: %v", row)
	}
}

func TestParseTables_NoTableReturnsEmptyNotNil(t *testing.T) {
	tables, err := ParseTables(`<div>no tables here</div>`)
	if err != nil {
		t.Fatalf("ParseTables: %v", err)
	}
	if tables == nil {
		t.Fatal("want empty slice, got nil")
	}
	if len(tables) != 0 {
		t.Fatalf("want 0 tables, got %d", len(tables))
	}
}

func TestFindTable_ByRequiredHeaders(t *testing.T) {
	multi := `<table><tr><th>A</th></tr><tr><td>1</td></tr></table>
	          <table id="want"><tr><th>SYMBOL</th><th>VOLUME</th></tr><tr><td>OGDC</td><td>5</td></tr></table>`
	tables, err := ParseTables(multi)
	if err != nil {
		t.Fatalf("ParseTables: %v", err)
	}
	got, ok := FindTable(tables, "", "symbol", "volume")
	if !ok {
		t.Fatal("FindTable did not find the symbol/volume table")
	}
	if got.ID != "want" {
		t.Errorf("ID = %q, want %q", got.ID, "want")
	}
	if _, missing := FindTable(tables, "", "nonexistent"); missing {
		t.Error("FindTable matched a table lacking the required header")
	}
}

// TestParseTables_ColspanRowspanHeaders pins the two-level header shape PSX
// uses for bid/ask depth. Before colspan expansion the symbol landed under
// "volume" and the bid price under "volume_2" — wrong-but-plausible numbers,
// exactly what header-keyed parsing exists to prevent.
func TestParseTables_ColspanRowspanHeaders(t *testing.T) {
	frag := `<table><thead>
	  <tr><th rowspan="2">SYMBOL</th><th colspan="2">BID</th><th colspan="2">ASK</th></tr>
	  <tr><th>VOLUME</th><th>PRICE</th><th>VOLUME</th><th>PRICE</th></tr>
	</thead><tbody>
	  <tr><td>OGDC</td><td>500</td><td>314.00</td><td>700</td><td>314.50</td></tr>
	</tbody></table>`
	tables, err := ParseTables(frag)
	if err != nil {
		t.Fatalf("ParseTables: %v", err)
	}
	if len(tables) != 1 {
		t.Fatalf("want 1 table, got %d", len(tables))
	}
	row := tables[0].Rows[0]
	if got := row["symbol"]; got != "OGDC" {
		t.Errorf("symbol = %q, want OGDC (headers=%v row=%v)", got, tables[0].Headers, row)
	}
	if got := row["bid_price"]; got != "314.00" {
		t.Errorf("bid_price = %q, want 314.00 (headers=%v)", got, tables[0].Headers)
	}
	if got := row["ask_price"]; got != "314.50" {
		t.Errorf("ask_price = %q, want 314.50 (headers=%v)", got, tables[0].Headers)
	}
	if got := row["bid_volume"]; got != "500" {
		t.Errorf("bid_volume = %q, want 500", got)
	}
}

// TestParseTables_DoesNotDoubleUnescape guards the entity-decoding contract:
// x/net/html already decodes entities, so a second unescape would turn a
// literal "&amp;" in a filing title into a bare "&".
func TestParseTables_DoesNotDoubleUnescape(t *testing.T) {
	frag := `<table><tr><th>TITLE</th></tr><tr><td>Notice re &amp;amp; and &amp;lt;b&amp;gt;</td></tr></table>`
	tables, err := ParseTables(frag)
	if err != nil {
		t.Fatalf("ParseTables: %v", err)
	}
	got := tables[0].Rows[0]["title"]
	want := "Notice re &amp; and &lt;b&gt;"
	if got != want {
		t.Errorf("title = %q, want %q (double-unescape regression)", got, want)
	}
}
