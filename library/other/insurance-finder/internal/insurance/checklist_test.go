package insurance

import "testing"

func checklistHas(cl ProviderChecklist, substr string) bool {
	for _, it := range cl.Items {
		if containsFold(it.Action, substr) || containsFold(it.Detail, substr) {
			return true
		}
	}
	return false
}

func TestGenerateChecklist_CoreManualActions(t *testing.T) {
	reg := mustRegistry(t)
	canopy, _ := reg.Get("insurance-canopy")
	cl := GenerateChecklist(canopy)

	for _, want := range []string{"CAPTCHA", "password", "EIN", "payment", "two-gate", "marketing"} {
		if !checklistHas(cl, want) {
			t.Errorf("checklist missing an item about %q", want)
		}
	}
}

func TestGenerateChecklist_AppendsSubmitNote(t *testing.T) {
	reg := mustRegistry(t)
	insureon, _ := reg.Get("insureon")
	cl := GenerateChecklist(insureon)
	// Insureon captures the lead at the contact step; the submit_note must
	// surface as a "where the real submit happens" item.
	if !checklistHas(cl, "CONTACT step") {
		t.Errorf("insureon checklist should surface its submit_note about the contact step")
	}
}

func TestGenerateChecklist_AppendsManualNote(t *testing.T) {
	reg := mustRegistry(t)
	veracity, _ := reg.Get("veracity")
	cl := GenerateChecklist(veracity)
	if !checklistHas(cl, "veracityinsurance.com") {
		t.Errorf("veracity checklist should surface its agent-only manual note")
	}
}
