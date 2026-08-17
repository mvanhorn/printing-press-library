package potential

import "testing"

func TestParsePotentialFromLabelledHTML(t *testing.T) {
	html := `<html><body>
		<div class="rating"><span>Overall</span><strong>72</strong></div>
		<div class="rating"><span>Potential</span><strong>86</strong></div>
	</body></html>`

	overall, potential, ok := parsePotential(html)
	if !ok {
		t.Fatal("parsePotential() did not find the potential rating")
	}
	if overall != 72 || potential != 86 {
		t.Fatalf("parsePotential() = (%d, %d), want (72, 86)", overall, potential)
	}
}

func TestParsePotentialFromInlineJSON(t *testing.T) {
	html := `<script>window.player = {"overall": 78, "potential": 91};</script>`

	overall, potential, ok := parsePotential(html)
	if !ok || overall != 78 || potential != 91 {
		t.Fatalf("parsePotential() = (%d, %d, %t), want (78, 91, true)", overall, potential, ok)
	}
}
