package insurance

import "testing"

func warningTitles(ws []Warning) map[string]Warning {
	m := make(map[string]Warning, len(ws))
	for _, w := range ws {
		m[w.Title] = w
	}
	return m
}

func TestWarnings_ImporterLandmines(t *testing.T) {
	ws := Warnings(importerProfile())
	titles := warningTitles(ws)

	fp, ok := titles["Foreign-products exclusion"]
	if !ok {
		t.Fatalf("importer warnings must include the foreign-products exclusion")
	}
	if fp.Severity != SeverityCritical {
		t.Errorf("foreign-products severity = %q, want critical", fp.Severity)
	}

	if _, ok := titles["GL Coverage B does not protect your brand IP"]; !ok {
		t.Errorf("importer warnings must include the Coverage B / IP gap")
	}
	if _, ok := titles["Route to specialty markets, not mainstream instant-quote carriers"]; !ok {
		t.Errorf("importer warnings must include the specialty-routing lesson")
	}
}

func TestWarnings_NonImporterNoForeignProducts(t *testing.T) {
	ws := Warnings(retailProfile())
	titles := warningTitles(ws)
	if _, ok := titles["Foreign-products exclusion"]; ok {
		t.Errorf("a non-importer retailer should NOT get the foreign-products exclusion warning")
	}
	// But process lessons still apply.
	if _, ok := titles["Decline optional marketing / SMS consents"]; !ok {
		t.Errorf("the marketing-consent process lesson should always be present")
	}
}
