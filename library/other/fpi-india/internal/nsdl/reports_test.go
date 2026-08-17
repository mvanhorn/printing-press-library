package nsdl

import "testing"

const aucFixture = `<html><body><table><tr><th>Sr. No.</th><th>Country</th><th>Equity</th><th>Total</th></tr>
<tr><td>1</td><td>UNITED STATES OF AMERICA</td><td>30,44,694</td><td>31,41,205</td></tr>
<tr><td>2</td><td>MAURITIUS</td><td>5,00,000</td><td>5,20,000</td></tr>
</table></body></html>`

func TestParseGenericRecords(t *testing.T) {
	recs, err := ParseGenericRecords([]byte(aucFixture))
	if err != nil {
		t.Fatalf("ParseGenericRecords() error = %v", err)
	}
	if len(recs) != 2 {
		t.Fatalf("got %d records, want 2", len(recs))
	}
	if recs[0]["Country"] != "UNITED STATES OF AMERICA" {
		t.Fatalf("unexpected first record: %v", recs[0])
	}
	if recs[1]["Total"] != "5,20,000" {
		t.Fatalf("unexpected second record total: %v", recs[1])
	}
}

func TestParseAUC(t *testing.T) {
	recs, err := ParseAUC([]byte(aucFixture))
	if err != nil {
		t.Fatalf("ParseAUC() error = %v", err)
	}
	if len(recs) != 2 || recs[0]["Country"] != "UNITED STATES OF AMERICA" {
		t.Fatalf("unexpected AUC records: %v", recs)
	}
}

const tradesFixture = `<html><body><table><tr><th>Company</th><th>Volume</th></tr>
<tr><td>ACME LTD</td><td>1000</td></tr>
</table></body></html>`

func TestParseTrades(t *testing.T) {
	recs, err := ParseTrades([]byte(tradesFixture))
	if err != nil {
		t.Fatalf("ParseTrades() error = %v", err)
	}
	if len(recs) != 1 || recs[0]["Company"] != "ACME LTD" {
		t.Fatalf("unexpected trades records: %v", recs)
	}
}

const registryFixture = `<html><body><table><tr><th>Category</th><th>Count</th></tr>
<tr><td>Asset Management Company</td><td>46</td></tr>
</table></body></html>`

func TestParseRegistry(t *testing.T) {
	recs, err := ParseRegistry([]byte(registryFixture))
	if err != nil {
		t.Fatalf("ParseRegistry() error = %v", err)
	}
	if len(recs) != 1 || recs[0]["Category"] != "Asset Management Company" {
		t.Fatalf("unexpected registry records: %v", recs)
	}
}

func TestParseSectorPeriods(t *testing.T) {
	html := []byte(`<select id="ddlfortnighly">
<option value="~/StaticReports/Fortnightly_Sector_wise_FII_Investment_Data/FIIInvestSector_June302026.html">JUNE 30, 2026</option>
<option value="~/StaticReports/Fortnightly_Sector_wise_FII_Investment_Data/FIIInvestSector_June152026.html">JUNE 15, 2026</option>
</select>`)
	periods := ParseSectorPeriods(html)
	if len(periods) != 2 {
		t.Fatalf("got %d periods, want 2", len(periods))
	}
	if periods[0].Label != "JUNE 30, 2026" {
		t.Fatalf("unexpected first label: %q", periods[0].Label)
	}
	want := "/web/StaticReports/Fortnightly_Sector_wise_FII_Investment_Data/FIIInvestSector_June302026.html"
	if periods[0].Path != want {
		t.Fatalf("path missing /web prefix: got %q, want %q", periods[0].Path, want)
	}
}

func TestParseSectorSnapshot(t *testing.T) {
	html := []byte(`<html><body><table><tr><th>Sr No</th><th>Sectors</th><th>Total</th></tr>
<tr><td>1</td><td>Financial Services</td><td>2137845</td></tr>
<tr><td></td><td>Grand Total</td><td>7622124</td></tr>
</table></body></html>`)
	recs, err := ParseSectorSnapshot(html)
	if err != nil {
		t.Fatalf("ParseSectorSnapshot() error = %v", err)
	}
	if len(recs) != 2 {
		t.Fatalf("got %d records, want 2 (filtering Grand Total happens at the caller, not the parser)", len(recs))
	}
}
