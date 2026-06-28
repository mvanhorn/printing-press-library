package insurance

import "testing"

func findField(sheet AnswerSheet, name string) (AnswerField, bool) {
	for _, f := range sheet.Fields {
		if f.Field == name {
			return f, true
		}
	}
	return AnswerField{}, false
}

func TestGenerateAnswerSheet_CoreFields(t *testing.T) {
	reg := mustRegistry(t)
	canopy, _ := reg.Get("insurance-canopy")
	sheet := GenerateAnswerSheet(importerProfile(), canopy)

	if sheet.ProviderID != "insurance-canopy" {
		t.Errorf("ProviderID = %q", sheet.ProviderID)
	}
	if sheet.QuoteURL == "" {
		t.Errorf("QuoteURL is empty")
	}

	gl, ok := findField(sheet, "GL limits")
	if !ok {
		t.Fatalf("GL limits field missing")
	}
	if gl.Value != "$1,000,000 per occurrence / $2,000,000 aggregate" {
		t.Errorf("GL limits = %q", gl.Value)
	}

	eff, ok := findField(sheet, "Desired effective date")
	if !ok || eff.Value != "2026-06-29" {
		t.Errorf("effective date = %q (ok=%v), want 2026-06-29", eff.Value, ok)
	}

	imp, ok := findField(sheet, "Importer / private-label / manufacturer")
	if !ok || !containsFold(imp.Value, "Importer") {
		t.Errorf("importer status = %q", imp.Value)
	}

	mk, ok := findField(sheet, "Marketing / SMS consent")
	if !ok || !containsFold(mk.Value, "DECLINE") {
		t.Errorf("marketing consent field = %q, want a DECLINE instruction", mk.Value)
	}
}

func TestGenerateAnswerSheet_IncludesProviderHints(t *testing.T) {
	reg := mustRegistry(t)
	canopy, _ := reg.Get("insurance-canopy")
	sheet := GenerateAnswerSheet(importerProfile(), canopy)
	// Insurance Canopy ships a "Program" field hint.
	if _, ok := findField(sheet, "Program"); !ok {
		t.Errorf("answer sheet should include Insurance Canopy's Program field hint")
	}
}

func TestGenerateAnswerSheet_NonImporterStatus(t *testing.T) {
	reg := mustRegistry(t)
	hiscox, _ := reg.Get("hiscox")
	sheet := GenerateAnswerSheet(retailProfile(), hiscox)
	imp, ok := findField(sheet, "Importer / private-label / manufacturer")
	if !ok {
		t.Fatalf("importer status field missing")
	}
	if containsFold(imp.Value, "Importer of record") {
		t.Errorf("non-importer status should not claim importer of record: %q", imp.Value)
	}
}
