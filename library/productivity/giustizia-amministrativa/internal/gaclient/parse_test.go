package gaclient

import "testing"

const sampleArticle = `<article class="ricerca--item">
<div class="ricerca--item__footer row">
<div class="col-sm-12">
<a data-sede="tar_rm" data-nrg="202600422" data-idprovv="EWXs3p4BkcXQLO6sDj_k"
href="https://mdp.giustizia-amministrativa.it/visualizza/?nodeRef=&schema=tar_rm&nrg=202600422&nomeFile=202611307_01.html&subDir=Provvedimenti"
class="visited-provvedimenti clickable" target="_blank"><img></a>
<a onclick='visualizzaProvvedimentoHighlighted("x", $(this)); return false;' >202611307 (ROMA, SEZIONE 5T) html</a>
</div>
<div class="col-sm-12"><b>SENTENZA</b> sede di <b>ROMA</b>, sezione <b>SEZIONE 5T</b>, numero provv.: <b>202611307</b></div>
<div class="col-sm-12 snippet">...di procedure di <em>appalto</em>, nelle quali...</div>
<div class="col-sm-12">Numero ricorso: <b>202600422</b></div>
<div class="col-sm-12"><b>ECLI:IT:TARLAZ:2026:11307SENT</b></div>
</div>
</article>`

func TestParseResults(t *testing.T) {
	page := `<div>Trovati 84845 risultati</div>` + sampleArticle
	if got := ParseTotal(page); got != 84845 {
		t.Errorf("ParseTotal = %d, want 84845", got)
	}
	items := ParseResults(page)
	if len(items) != 1 {
		t.Fatalf("ParseResults len = %d, want 1", len(items))
	}
	p := items[0]
	cases := map[string]struct{ got, want string }{
		"ecli":      {p.Ecli, "ECLI:IT:TARLAZ:2026:11307SENT"},
		"schema":    {p.Schema, "tar_rm"},
		"nrg":       {p.Nrg, "202600422"},
		"tipo":      {p.Tipo, "Sentenza"},
		"sede":      {p.Sede, "ROMA"},
		"sezione":   {p.Sezione, "SEZIONE 5T"},
		"idprovv":   {p.Idprovv, "EWXs3p4BkcXQLO6sDj_k"},
		"nome_file": {p.NomeFile, "202611307_01.html"},
		"formato":   {p.Formato, "html"},
	}
	for name, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", name, c.got, c.want)
		}
	}
	if p.Anno != 2026 || p.Numero != 11307 {
		t.Errorf("anno/numero = %d/%d, want 2026/11307", p.Anno, p.Numero)
	}
	if p.URL == "" || p.Snippet == "" {
		t.Errorf("url/snippet should be populated, got url=%q snippet=%q", p.URL, p.Snippet)
	}
}

func TestSplitAnnoNumero(t *testing.T) {
	tests := []struct {
		in     string
		anno   int
		numero int
	}{
		{"202611307", 2026, 11307},
		{"202600422", 2026, 422},
		{"123", 0, 123},
	}
	for _, tt := range tests {
		a, n := splitAnnoNumero(tt.in)
		if a != tt.anno || n != tt.numero {
			t.Errorf("splitAnnoNumero(%q) = %d/%d, want %d/%d", tt.in, a, n, tt.anno, tt.numero)
		}
	}
}

func TestMapTipoSede(t *testing.T) {
	if mapTipo("sentenza") != "Sentenza" || mapTipo("plenaria") != "P" {
		t.Errorf("mapTipo wrong")
	}
	if mapSede("roma") != "Roma" || mapSede("consiglio-di-stato") != "Consiglio di Stato" {
		t.Errorf("mapSede wrong")
	}
}

func TestTipoProvvFromNomeFile(t *testing.T) {
	if got := TipoProvvFromNomeFile("202611307_01.html"); got != "01" {
		t.Errorf("TipoProvvFromNomeFile = %q, want 01", got)
	}
}
