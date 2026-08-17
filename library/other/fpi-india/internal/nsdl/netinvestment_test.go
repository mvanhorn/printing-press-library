package nsdl

import "testing"

// yearwiseFixture mirrors the confirmed live 14-column Yearwise.aspx GridView
// shape (id="rpt"): Period | Equity | Debt-General | Debt-VRR | Debt-FAR |
// Hybrid | MF-Equity | MF-Debt | MF-Hybrid | MF-SolutionOriented | MF-Other |
// AIF | Total | Cumulative, plus the trailing "Total" summary row and a
// provisional "** " current-year row NSDL appends.
const yearwiseFixture = `<html><body><table id="rpt">
<tr><th colspan="14">FPI Net Investments - Financial Year</th></tr>
<tr><th rowspan="3">Financial Year</th><th colspan="13">INR crores</th></tr>
<tr><th>Equity</th><th colspan="3">Debt</th><th>Hybrid</th><th colspan="5">Mutual Funds</th><th>AIF</th><th rowspan="2">Total for the FY</th><th rowspan="2">Cumulative total (upto the FY)</th></tr>
<tr><th>Equity</th><th>Debt-General Limit</th><th>Debt-VRR</th><th>Debt-FAR</th><th>Hybrid</th><th>Equity</th><th>Debt</th><th>Hybrid</th><th>Solution Oriented</th><th>Other</th><th>AIF</th></tr>
<tr><td>1992-93</td><td>13</td><td>0</td><td>0</td><td>0</td><td>0</td><td>0</td><td>0</td><td>0</td><td>0</td><td>0</td><td>0</td><td>13</td><td>13</td></tr>
<tr><td>1998-99</td><td>29973</td><td>-147</td><td>0</td><td>0</td><td>0</td><td>0</td><td>0</td><td>0</td><td>0</td><td>0</td><td>0</td><td>29826</td><td>61237</td></tr>
<tr><td>2026-27 **</td><td>-128259</td><td>26460</td><td>-2553</td><td>30963</td><td>123</td><td>265</td><td>-70</td><td>-1</td><td>0</td><td>266</td><td>6</td><td>-72801</td><td>1411601</td></tr>
<tr><td>Total</td><td>712464</td><td>469465</td><td>58193</td><td>127103</td><td>41732</td><td>2348</td><td>-789</td><td>-66</td><td>-3</td><td>1140</td><td>11</td><td>1411601</td></tr>
</table></body></html>`

func TestParseNetInvestment(t *testing.T) {
	rows, err := ParseNetInvestment([]byte(yearwiseFixture), "fy", "INR")
	if err != nil {
		t.Fatalf("ParseNetInvestment() error = %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("got %d rows, want 3 (Total summary row must be excluded): %+v", len(rows), rows)
	}
	if rows[0].Period != "1992-93" || rows[0].Equity != 13 || rows[0].Total != 13 {
		t.Fatalf("unexpected first row: %+v", rows[0])
	}
	if rows[1].Period != "1998-99" || rows[1].DebtGeneralLimit != -147 || rows[1].Debt != -147 {
		t.Fatalf("unexpected second row: %+v", rows[1])
	}
	if rows[2].Period != "2026-27" {
		t.Fatalf("provisional-year suffix not trimmed: %q", rows[2].Period)
	}
	for _, r := range rows {
		if r.Period == "Total" {
			t.Fatalf("lifetime Total row leaked into parsed periods: %+v", r)
		}
	}
}

func TestNetInvestmentRow_AssetValue(t *testing.T) {
	row := NetInvestmentRow{
		Equity: 100, Debt: 50, Hybrid: 10, MutualFunds: 5, AIF: 2, Total: 167,
	}
	tests := []struct {
		asset  string
		want   float64
		wantOk bool
	}{
		{"equity", 100, true},
		{"debt", 50, true},
		{"hybrid", 10, true},
		{"mutual_funds", 5, true},
		{"mf", 5, true},
		{"aif", 2, true},
		{"total", 167, true},
		{"", 167, true},
		{"bogus", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.asset, func(t *testing.T) {
			got, ok := row.AssetValue(tt.asset)
			if ok != tt.wantOk {
				t.Fatalf("AssetValue(%q) ok = %v, want %v", tt.asset, ok, tt.wantOk)
			}
			if ok && got != tt.want {
				t.Fatalf("AssetValue(%q) = %v, want %v", tt.asset, got, tt.want)
			}
		})
	}
}
