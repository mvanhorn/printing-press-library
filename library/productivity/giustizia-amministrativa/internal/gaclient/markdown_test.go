package gaclient

import (
	"strings"
	"testing"
)

const sampleDoc = `<html><body class="corpo">Pubblicato il 19/06/2026
<p class="registri">N. 11307/2026 REG.PROV.COLL.</p>
<p class="sezione">SENTENZA</p>
<p class="popolo">sul ricorso proposto dall'avvocato <i>Tizio</i>;<br></p>
<table><tbody><tr><td>L'ESTENSORE</td><td>IL PRESIDENTE</td></tr></tbody></table>
</body></html>`

func TestExtractDataDeposito(t *testing.T) {
	if got := ExtractDataDeposito(sampleDoc); got != "19/06/2026" {
		t.Errorf("ExtractDataDeposito = %q, want 19/06/2026", got)
	}
}

func TestHTMLToMarkdown(t *testing.T) {
	md := HTMLToMarkdown(sampleDoc)
	for _, want := range []string{"N. 11307/2026 REG.PROV.COLL.", "SENTENZA", "*Tizio*"} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q in:\n%s", want, md)
		}
	}
	if strings.Contains(md, "<p") || strings.Contains(md, "<i>") {
		t.Errorf("markdown should not contain raw tags:\n%s", md)
	}
}

func TestHTMLToText(t *testing.T) {
	txt := HTMLToText(sampleDoc)
	if strings.Contains(txt, "*") || strings.Contains(txt, "<") {
		t.Errorf("text output should be plain, got:\n%s", txt)
	}
	if !strings.Contains(txt, "SENTENZA") {
		t.Errorf("text missing content")
	}
}
