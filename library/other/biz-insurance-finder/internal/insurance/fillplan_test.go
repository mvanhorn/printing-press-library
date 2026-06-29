package insurance

import "testing"

func gateKinds(fp FillPlan) map[string]bool {
	m := map[string]bool{}
	for _, g := range fp.HumanGates {
		m[g.Gate] = true
	}
	return m
}

func autoByLabel(fp FillPlan, label string) (FillField, bool) {
	for _, f := range fp.AutoFill {
		if f.Label == label {
			return f, true
		}
	}
	return FillField{}, false
}

func TestGenerateFillPlan_AutoFillFromProfile(t *testing.T) {
	reg := mustRegistry(t)
	canopy, _ := reg.Get("insurance-canopy")
	fp := GenerateFillPlan(importerProfile(), canopy)

	if fp.QuoteURL == "" {
		t.Fatalf("fill plan has no quote URL")
	}
	if len(fp.AutoFill) == 0 {
		t.Fatalf("fill plan has no auto-fill fields")
	}
	// Auto-fill fields must never be human-gated.
	for _, f := range fp.AutoFill {
		if f.HumanGated {
			t.Errorf("auto-fill field %q must not be human-gated", f.Label)
		}
	}
	// A value the user provided shows up to be typed automatically.
	gl, ok := autoByLabel(fp, "GL limits")
	if !ok || gl.Value != "$1,000,000 per occurrence / $2,000,000 aggregate" {
		t.Errorf("GL limits auto-fill = %q (ok=%v)", gl.Value, ok)
	}
	if gl.Type != "select" {
		t.Errorf("GL limits type hint = %q, want select", gl.Type)
	}
}

func TestGenerateFillPlan_HumanGates(t *testing.T) {
	reg := mustRegistry(t)
	canopy, _ := reg.Get("insurance-canopy")
	fp := GenerateFillPlan(importerProfile(), canopy)

	gates := gateKinds(fp)
	for _, want := range []string{GateCaptcha, GateAccount, GateGovID, GatePayment, GateSubmit} {
		if !gates[want] {
			t.Errorf("fill plan missing human gate %q", want)
		}
	}
	// Every gate must be flagged human_gated and the submit gate must be last.
	for _, g := range fp.HumanGates {
		if !g.HumanGated {
			t.Errorf("gate %q must be human_gated", g.Gate)
		}
	}
	last := fp.HumanGates[len(fp.HumanGates)-1]
	if last.Gate != GateSubmit {
		t.Errorf("final gate = %q, want submit", last.Gate)
	}
	// EIN/SSN must be a gate, never an auto-fill field (we don't store it).
	for _, f := range fp.AutoFill {
		if f.Type == "" && (f.Label == "EIN / SSN / government ID") {
			t.Errorf("gov id must never be an auto-fill field")
		}
	}
}

func TestGenerateFillPlan_SubmitNoteSurfaced(t *testing.T) {
	reg := mustRegistry(t)
	insureon, _ := reg.Get("insureon")
	fp := GenerateFillPlan(importerProfile(), insureon)
	// Insureon captures the lead at the contact step; the submit gate note must
	// carry that warning so the user is not surprised.
	var submit FillField
	for _, g := range fp.HumanGates {
		if g.Gate == GateSubmit {
			submit = g
		}
	}
	if !containsFold(submit.Note, "CONTACT step") {
		t.Errorf("insureon submit gate note should warn about the contact step, got %q", submit.Note)
	}
}
