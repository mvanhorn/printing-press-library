package betrvg

import (
	"testing"
)

func TestAll_NotEmpty(t *testing.T) {
	ps := All()
	if len(ps) == 0 {
		t.Fatal("All() returned empty paragraph list")
	}
}

func TestByNumber_KnownParagraph(t *testing.T) {
	p := ByNumber(102)
	if p == nil {
		t.Fatal("ByNumber(102) returned nil, want non-nil")
	}
	if p.Number != 102 {
		t.Errorf("ByNumber(102).Number = %d, want 102", p.Number)
	}
}

func TestByNumber_Unknown(t *testing.T) {
	p := ByNumber(99999)
	if p != nil {
		t.Errorf("ByNumber(99999) = %v, want nil", p)
	}
}

func TestByKeywords_Kündigung(t *testing.T) {
	results := ByKeywords([]string{"kündigung"})
	if len(results) == 0 {
		t.Fatal("ByKeywords([kündigung]) returned empty, want at least one result")
	}
}

func TestByKeywords_Empty(t *testing.T) {
	results := ByKeywords([]string{})
	if len(results) != 0 {
		t.Errorf("ByKeywords([]) = %v, want empty", results)
	}
}

func TestDeadlines_NotEmpty(t *testing.T) {
	ds := Deadlines()
	if len(ds) == 0 {
		t.Fatal("Deadlines() returned empty list")
	}
}

func TestDeadlines_OrdentlicheKündigung(t *testing.T) {
	for _, d := range Deadlines() {
		if d.Situation == "ordentliche kündigung" {
			if d.Days != 7 {
				t.Errorf("ordentliche kündigung deadline = %d days, want 7", d.Days)
			}
			return
		}
	}
	t.Fatal("no deadline rule found for 'ordentliche kündigung'")
}

func TestAllChecklists_NotEmpty(t *testing.T) {
	cls := AllChecklists()
	if len(cls) == 0 {
		t.Fatal("AllChecklists() returned empty list")
	}
}

func TestGetChecklist_Kündigung(t *testing.T) {
	cl := GetChecklist("kündigung")
	if cl == nil {
		t.Fatal("GetChecklist(kündigung) returned nil, want non-nil")
	}
	if len(cl.Steps) == 0 {
		t.Error("GetChecklist(kündigung).Steps is empty")
	}
}

func TestGetChecklist_Unknown(t *testing.T) {
	cl := GetChecklist("xyzzy_not_a_real_situation_12345")
	if cl != nil {
		t.Errorf("GetChecklist(unknown) = %v, want nil", cl)
	}
}

func TestContainsFold(t *testing.T) {
	if !ContainsFold("Kündigung", "kündigung") {
		t.Error("ContainsFold should be case-insensitive")
	}
	if ContainsFold("Betriebsrat", "kündigung") {
		t.Error("ContainsFold should return false for non-matching")
	}
}
