package gaclient

import (
	"strings"
	"testing"
)

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

func TestIsErrorPage(t *testing.T) {
	// The portal answers a stale nomeFile with HTTP 200 and this page.
	errPage := []byte(`<!DOCTYPE html><html lang="it"><head><title>404 - Pagina non trovata</title></head>` +
		`<body><div class="container"><h1>Ops!</h1><p>Il documento che stai cercando non esiste o è stato rimosso.</p></div></body></html>`)
	if !isErrorPage(errPage) {
		t.Error("la pagina di errore del portale non e' stata riconosciuta")
	}
	cases := map[string][]byte{
		"xml":  []byte(`<?xml version="1.0" encoding="UTF-8"?><GA><Provvedimento><meta id="1"/></Provvedimento></GA>`),
		"pdf":  []byte("%PDF-1.4\n1 0 obj\n"),
		"html": []byte(`<html><body class="corpo">Pubblicato il 14/08/2026<p class="registri">N. 14259/2026 REG.PROV.COLL.</p></body></html>`),
		// Una sentenza che parla di una pagina web mancante resta una
		// sentenza: i marcatori del portale vengono prima di quelli d'errore.
		"sentenza sul tema": []byte(`<html><body class="corpo">Pubblicato il 14/08/2026<p class="registri">N. 1/2026 REG.PROV.COLL.</p>` +
			`<p>il ricorrente lamenta che l'atto era irraggiungibile: il portale mostrava "Pagina non trovata"</p></body></html>`),
	}
	for name, body := range cases {
		if isErrorPage(body) {
			t.Errorf("%s: documento valido scambiato per pagina di errore", name)
		}
	}
}

func TestParseFormLeggeIstanzaDallaPagina(t *testing.T) {
	// Same markup as the portal's, with a different _INSTANCE_ hash: what the
	// page declares must win over the hardcoded constants.
	page := []byte(`<form class="form-provvedimenti" method="post" ` +
		`action="https://www.giustizia-amministrativa.it/web/guest/dcsnprr?p_p_id=decisioni_pareri_web_DecisioniPareriWebPortlet_INSTANCE_NuOvOhAsH&amp;` +
		`p_p_lifecycle=1&p_p_state=normal&p_p_mode=view&_decisioni_pareri_web_DecisioniPareriWebPortlet_INSTANCE_NuOvOhAsH_javax.portlet.action=search&p_auth=abc123" ` +
		`id="_decisioni_pareri_web_DecisioniPareriWebPortlet_INSTANCE_NuOvOhAsH_provvedimentiForm"></form>`)
	action, portlet := parseForm(page)
	// The attribute above escapes its separators: left as "&amp;" the query
	// would parse into keys named "amp;p_p_lifecycle".
	if strings.Contains(action, "&amp;") {
		t.Errorf("separatori ancora escapati nell'action: %q", action)
	}
	if portlet != "decisioni_pareri_web_DecisioniPareriWebPortlet_INSTANCE_NuOvOhAsH" {
		t.Errorf("portlet id letto = %q", portlet)
	}
	if !strings.Contains(action, "p_auth=abc123") {
		t.Errorf("action letta = %q", action)
	}

	c := &Client{portlet: portlet, action: action}
	if got := c.portletNS(); got != "_"+portlet {
		t.Errorf("namespace = %q", got)
	}
	url := c.buildSearchURL(SearchOptions{Testo: "appalto"}, 1)
	if !strings.Contains(url, "_INSTANCE_NuOvOhAsH_searchtextProvvedimenti=appalto") {
		t.Errorf("i campi non usano il namespace della pagina: %s", url)
	}
	for _, atteso := range []string{"p_p_lifecycle=1", "p_p_state=normal", "p_p_mode=view"} {
		if !strings.Contains(url, atteso) {
			t.Errorf("parametro %s perso dall'action: %s", atteso, url)
		}
	}
	if strings.Contains(url, "amp%3B") || strings.Contains(url, "amp;") {
		t.Errorf("chiavi malformate dall'escape HTML: %s", url)
	}

	// No form on the page: the constants keep the client working.
	fallback := &Client{}
	if got := fallback.portletNS(); got != portletPrefix {
		t.Errorf("fallback namespace = %q", got)
	}
	if u := fallback.buildSearchURL(SearchOptions{Testo: "appalto"}, 1); !strings.Contains(u, portletID) {
		t.Errorf("fallback url senza portlet id: %s", u)
	}
}
