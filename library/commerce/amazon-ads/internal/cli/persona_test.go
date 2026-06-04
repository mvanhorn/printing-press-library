package cli

import "testing"

func TestSellerNoDSPPersonaDefinitionAndAgentContext(t *testing.T) {
	t.Parallel()
	view := personaDefinition(sellerNoDSPPersona)
	if view == nil {
		t.Fatalf("seller-no-dsp persona missing")
	}
	want := map[string]bool{
		"sp": true, "sponsored-products-sp": true,
		"sb": true, "sponsored-brands-sb": true,
		"sd": true, "sponsored-display-sd": true,
		"break-even-acos": true, "true-profit": true, "acos-vs-tacos": true,
		"portfolio-dashboard": true, "product-ad-profitability": true, "campaign-comparison": true,
		"auth": true, "profile": true, "doctor": true, "reports": true,
	}
	got := map[string]bool{}
	for _, group := range view.Groups {
		for _, command := range group.Commands {
			got[command] = true
		}
	}
	for command := range want {
		if !got[command] {
			t.Fatalf("seller-no-dsp persona missing %s in %+v", command, view.Groups)
		}
	}
	root := RootCmd()
	if err := root.PersistentFlags().Set("persona", sellerNoDSPPersona); err != nil {
		t.Fatal(err)
	}
	ctx := buildAgentContext(root)
	if ctx.Persona != sellerNoDSPPersona || ctx.PersonaView == nil || !ctx.PersonaView.FullSurface.Collapsed {
		t.Fatalf("agent persona context = %+v", ctx.PersonaView)
	}
}
