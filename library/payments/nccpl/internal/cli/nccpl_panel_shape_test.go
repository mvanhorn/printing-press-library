package cli

import "testing"

func TestNCCPLSortPanel(t *testing.T) {
	mk := func() []nccplPanelRow {
		return []nccplPanelRow{
			{Date: "2026-09-03", Key: "b", Metric: "m2", Value: -100},
			{Date: "2026-09-01", Key: "c", Metric: "m1", Value: 50},
			{Date: "2026-09-02", Key: "a", Metric: "m3", Value: 10},
		}
	}
	rows := mk()
	if err := nccplSortPanel(rows, "date"); err != nil {
		t.Fatal(err)
	}
	if rows[0].Date != "2026-09-01" || rows[2].Date != "2026-09-03" {
		t.Errorf("date sort wrong: %v", []string{rows[0].Date, rows[1].Date, rows[2].Date})
	}
	rows = mk()
	if err := nccplSortPanel(rows, "abs-value"); err != nil {
		t.Fatal(err)
	}
	if rows[0].Value != -100 || rows[1].Value != 50 || rows[2].Value != 10 {
		t.Errorf("abs-value sort must rank by magnitude regardless of sign: %v",
			[]float64{rows[0].Value, rows[1].Value, rows[2].Value})
	}
	rows = mk()
	if err := nccplSortPanel(rows, "value"); err != nil {
		t.Fatal(err)
	}
	if rows[0].Value != 50 || rows[2].Value != -100 {
		t.Errorf("value sort must be signed descending: %v",
			[]float64{rows[0].Value, rows[1].Value, rows[2].Value})
	}
	if err := nccplSortPanel(mk(), "sideways"); err == nil {
		t.Error("unknown --sort must error")
	}
	// Validation must work on a nil slice so the flag can be checked before any query.
	if err := nccplSortPanel(nil, "abs-value"); err != nil {
		t.Errorf("nil slice validation should succeed: %v", err)
	}
}

func TestNCCPLMetricMatchesCurrency(t *testing.T) {
	cases := []struct {
		metric, currency string
		want             bool
	}{
		{"net_value", "pkr", true},
		{"net_value", "usd", false},
		{"net_value_USD", "usd", true},
		{"net_value_USD", "pkr", false},
		{"USD", "usd", true},
		{"net_value", "", true},
		{"net_value_USD", "", true},
	}
	for _, c := range cases {
		if got := nccplMetricMatchesCurrency(c.metric, c.currency); got != c.want {
			t.Errorf("match(%q,%q)=%v want %v", c.metric, c.currency, got, c.want)
		}
	}
	if !nccplValidCurrency("usd") || !nccplValidCurrency("") || nccplValidCurrency("yen") {
		t.Error("currency validation wrong")
	}
}

func TestNCCPLPivotPanel(t *testing.T) {
	rows := []nccplPanelRow{
		{Key: "0801|Commercial Banks|FI", Metric: "NET_VALUE", Value: -6.7},
		{Key: "0801|Commercial Banks|LI", Metric: "NET_VALUE", Value: 6.1},
		{Key: "0802|Cement|FI", Metric: "NET_VALUE", Value: -1.8},
		{Key: "0802|Cement|LI", Metric: "NET_VALUE", Value: 2.6},
		{Key: "0802|Cement|FI", Metric: "BUY_VALUE", Value: 999}, // must be filtered out by metric
	}
	pv := nccplPivotPanel(rows, "NET_VALUE")
	if pv.RowsUsed != 4 {
		t.Errorf("rows_used=%d want 4 (the BUY_VALUE row must be excluded)", pv.RowsUsed)
	}
	if len(pv.Investors) != 2 || pv.Investors[0] != "FI" {
		t.Errorf("investors=%v want [FI LI]", pv.Investors)
	}
	if len(pv.Sectors) != 2 {
		t.Fatalf("sectors=%d want 2", len(pv.Sectors))
	}
	banks := pv.Sectors[0]
	if banks.By["FI"] != -6.7 || banks.By["LI"] != 6.1 {
		t.Errorf("bank cells wrong: %v", banks.By)
	}
	// Rows with no investor segment must be skipped, not folded into a phantom sector.
	pv = nccplPivotPanel([]nccplPanelRow{{Key: "nosegments", Metric: "NET_VALUE", Value: 1}}, "NET_VALUE")
	if pv.RowsUsed != 0 || pv.Note == "" {
		t.Errorf("unsegmented key must be skipped with a note, got rows=%d note=%q", pv.RowsUsed, pv.Note)
	}
}
