package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	icaro "github.com/mvanhorn/printing-press-library/library/other/ars-sicilia/internal/icaroclient"
)

func TestToISISDate(t *testing.T) {
	cases := []struct{ in, want string }{
		{"2026-02-25", "260225"},
		{"2026-02-24:2026-02-25", "260224/260225"},
		{"260225", "260225"},                         // already AAMMGG → unchanged
		{"non-data", "non-data"},                     // unparseable → unchanged
		{"abcd-ef-gh", "abcd-ef-gh"},                 // right shape but non-numeric → unchanged
		{"2026-02-25:", "2026-02-25:"},               // trailing colon → not a range; unparseable → unchanged
		{":2026-02-25", ":2026-02-25"},               // leading colon → not a range; unparseable → unchanged
		{"2026-02-25:garbage", "2026-02-25:garbage"}, // one invalid bound → no malformed range
		{"260224:260225", "260224/260225"},           // already-AAMMGG bounds → valid range
	}
	for _, c := range cases {
		if got := toISISDate(c.in); got != c.want {
			t.Errorf("toISISDate(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestYearToISISRange(t *testing.T) {
	cases := []struct{ in, want string }{
		{"2024", "240101/241231"},
		{"1999", "990101/991231"},
		{"24", "24"},                       // not 4 digits → unchanged
		{"20245", "20245"},                 // not 4 digits → unchanged
		{"abcd", "abcd"},                   // non-numeric → unchanged
		{"240101/241231", "240101/241231"}, // already a range → unchanged
	}
	for _, c := range cases {
		if got := yearToISISRange(c.in); got != c.want {
			t.Errorf("yearToISISRange(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestNormalizeParams_AnnoOnDdlBecomesDatpreRange covers the bug where
// `ddl cerca --anno 2024` matched "2024" as free text anywhere in the
// document (no DATPRE field for ddl to qualify a plain year against),
// returning DDLs from other years that merely mention "2024" in the text.
func TestNormalizeParams_AnnoOnDdlBecomesDatpreRange(t *testing.T) {
	arc := *icaro.BySlug("ddl")
	out := normalizeParams(arc, map[string]string{"anno": "2024"})
	if out["anno"] != "240101/241231" {
		t.Errorf("anno = %q, want 240101/241231", out["anno"])
	}
}

// TestNormalizeParams_AnnoOnLeggiUnchanged covers archives that already have
// a real year field (leggi.LEGANN, resoconti.ANNSED): --anno must stay a
// bare year there, not be rewritten into a DATPRE-style range.
func TestNormalizeParams_AnnoOnLeggiUnchanged(t *testing.T) {
	arc := *icaro.BySlug("leggi")
	out := normalizeParams(arc, map[string]string{"anno": "2024"})
	if out["anno"] != "2024" {
		t.Errorf("anno = %q, want unchanged 2024", out["anno"])
	}
}

func TestCommissioneOrdinale(t *testing.T) {
	cases := []struct{ in, want string }{
		{"1", "PRIMA"},
		{"6", "SESTA"},
		{"7", ""},
		{"SESTA", ""},
	}
	for _, c := range cases {
		if got := commissioneOrdinale(c.in); got != c.want {
			t.Errorf("commissioneOrdinale(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestNormalizeParams_DataAndCodcom(t *testing.T) {
	arc := *icaro.BySlug("convocazioni")
	out := normalizeParams(arc, map[string]string{
		"legisl": "18",
		"data":   "2026-02-25",
		"codcom": "6",
	})
	if out["data"] != "260225" {
		t.Errorf("data = %q, want 260225", out["data"])
	}
	if _, ok := out["codcom"]; ok {
		t.Errorf("codcom should be rerouted/removed, got %q", out["codcom"])
	}
	if out["commissione"] != "SESTA" {
		t.Errorf("commissione = %q, want SESTA", out["commissione"])
	}
}

// TestBuildQuery_DataOnAttiIspettivi pins that --data is qualified on DATPRE
// (presentation date) for the five acts archives that gained the flag, and
// that a range still becomes ISIS interval syntax. These archives are pure
// Icaro (bdArchives covers only sommari/resoconti/convocazioni), so the flag
// travels this path and no other.
func TestBuildQuery_DataOnAttiIspettivi(t *testing.T) {
	for _, slug := range []string{"mozioni", "interrogazioni", "interpellanze", "odg", "risoluzioni"} {
		arc := *icaro.BySlug(slug)
		if got := arc.FieldMap["data"]; got != "DATPRE" {
			t.Errorf("%s: FieldMap[data] = %q, want DATPRE", slug, got)
		}
		out := normalizeParams(arc, map[string]string{"data": "2020-02-01:2020-02-29"})
		if out["data"] != "200201/200229" {
			t.Errorf("%s: data = %q, want 200201/200229", slug, out["data"])
		}
		q := icaro.BuildQuery(arc, normalizeParams(arc, map[string]string{
			"legisl": "17", "data": "2020-01-28",
		}), "")
		if !strings.Contains(q, "200128.DATPRE") {
			t.Errorf("%s: query = %q, want it to qualify 200128 on DATPRE", slug, q)
		}
	}
}

// TestBuildQuery_DataOnDdl pins the date range `ddl cerca --data` produces.
// Su ddl --data condivide DATPRE con --anno: qui si fissa che il range con
// estremi liberi arriva intatto al motore, mentre la mutua esclusione con
// --anno (due range in AND sullo stesso campo = zero risultati) è imposta
// dal comando.
func TestBuildQuery_DataOnDdl(t *testing.T) {
	arc := *icaro.BySlug("ddl")
	if got := arc.FieldMap["data"]; got != "DATPRE" {
		t.Fatalf("ddl: FieldMap[data] = %q, want DATPRE", got)
	}
	q := icaro.BuildQuery(arc, normalizeParams(arc, map[string]string{
		"legisl": "18", "data": "2026-07-01:2026-07-28",
	}), "")
	if !strings.Contains(q, "260701/260728.DATPRE") {
		t.Errorf("query = %q, want it to qualify 260701/260728 on DATPRE", q)
	}
	q = icaro.BuildQuery(arc, normalizeParams(arc, map[string]string{
		"legisl": "18", "data": "2026-07-28",
	}), "")
	if !strings.Contains(q, "260728.DATPRE") {
		t.Errorf("query = %q, want it to qualify 260728 on DATPRE", q)
	}
}

// TestDdlCerca_AnnoDataMutuallyExclusive: entrambe qualificano DATPRE, quindi
// insieme darebbero (260101/261231.DATPRE E 260701/260728.DATPRE) — zero
// risultati silenziosi invece dell'intersezione attesa. Meglio un errore.
func TestDdlCerca_AnnoDataMutuallyExclusive(t *testing.T) {
	cmd := newDdlCercaCmd(&rootFlags{})
	cmd.SetArgs([]string{"--anno", "2026", "--data", "2026-07-01"})
	cmd.SilenceErrors, cmd.SilenceUsage = true, true
	if err := cmd.Execute(); err == nil {
		t.Fatal("--anno insieme a --data deve fallire, non produrre una query vuota")
	}
}

// L'avviso di troncamento esiste perché una lista corta è indistinguibile da
// un archivio che non contiene il documento: deve tacere solo quando i
// risultati sono completi.
func TestTruncatedHint(t *testing.T) {
	if got := truncatedHint(false, 10, "leggi"); got != "" {
		t.Errorf("risultati completi: atteso nessun avviso, ottenuto %q", got)
	}
	got := truncatedHint(true, 10, "leggi")
	if got == "" {
		t.Fatal("risultati troncati: atteso un avviso, ottenuta stringa vuota")
	}
	for _, want := range []string{"troncati", "10", "leggi", "--limit"} {
		if !strings.Contains(got, want) {
			t.Errorf("avviso %q: manca %q (conteggio, archivio e rimedio devono esserci)", got, want)
		}
	}
}

// Senza --anno la cronologia esce coerente ma può riferirsi all'atto sbagliato:
// nella XVIII ci sono due L.R. 26 (7.10.2024 e 10.06.2025) e l'archivio ne
// restituisce una sola. L'avviso deve dire QUALE è stata presa — è la data che
// permette di accorgersene — e tacere quando --anno è stato indicato.
func TestAnnoNonPinnatoHint(t *testing.T) {
	if got := annoNonPinnatoHint(2025, 26, "10.06.2025"); got != "" {
		t.Errorf("--anno indicato: atteso nessun avviso, ottenuto %q", got)
	}
	if got := annoNonPinnatoHint(0, 26, "  "); got != "" {
		t.Errorf("data assente: atteso nessun avviso, ottenuto %q", got)
	}
	got := annoNonPinnatoHint(0, 26, "7.10.2024")
	if got == "" {
		t.Fatal("--anno non indicato: atteso un avviso, ottenuta stringa vuota")
	}
	for _, want := range []string{"26", "7.10.2024", "--anno"} {
		if !strings.Contains(got, want) {
			t.Errorf("avviso %q: manca %q (numero, data scelta e rimedio devono esserci)", got, want)
		}
	}
}

// La busta esiste per un motivo solo: in --agent il payload è un array e
// l'avviso di troncamento nasceva su stderr, dove un consumatore JSON non lo
// legge. Se le chiavi di servizio non ci sono, la busta non serve a niente.
func TestEmitEnvelope(t *testing.T) {
	var buf bytes.Buffer
	payload := []map[string]any{{"data": "06/11/2019", "title": "Seduta n. 150"}}
	if err := emitEnvelope(&buf, payload, true, "hint: risultati troncati", &rootFlags{asJSON: true}); err != nil {
		t.Fatalf("emitEnvelope: %v", err)
	}
	var env struct {
		Risultati []map[string]any `json:"risultati"`
		Troncato  bool             `json:"troncato"`
		Hint      string           `json:"hint"`
	}
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("output non è un oggetto JSON: %v — %s", err, buf.String())
	}
	if !env.Troncato {
		t.Error("troncato: atteso true, il chiamante l'ha dichiarato")
	}
	if env.Hint == "" {
		t.Error("hint: atteso non vuoto, è l'informazione che stderr non consegnava")
	}
	if len(env.Risultati) != 1 || env.Risultati[0]["data"] != "06/11/2019" {
		t.Errorf("risultati = %v, want la riga passata intatta", env.Risultati)
	}
}

// --select deve filtrare DENTRO risultati: applicato alla busta cancellerebbe
// i record (nessuno ha un campo "risultati") e lascerebbe le sole chiavi di
// servizio, cioè l'opposto di quel che chiede chi scrive --select data.
func TestEmitEnvelopeConSelect(t *testing.T) {
	var buf bytes.Buffer
	payload := []map[string]any{{"data": "06/11/2019", "title": "Seduta n. 150", "url": "https://x"}}
	flags := &rootFlags{asJSON: true, selectFields: "data"}
	if err := emitEnvelope(&buf, payload, false, "", flags); err != nil {
		t.Fatalf("emitEnvelope: %v", err)
	}
	var env struct {
		Risultati []map[string]any `json:"risultati"`
		Troncato  bool             `json:"troncato"`
	}
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("output non è un oggetto JSON: %v — %s", err, buf.String())
	}
	if len(env.Risultati) != 1 {
		t.Fatalf("risultati = %v, want una riga (il filtro non deve svuotare la busta)", env.Risultati)
	}
	if _, ok := env.Risultati[0]["data"]; !ok {
		t.Error("il campo selezionato «data» è sparito dai risultati")
	}
	if _, ok := env.Risultati[0]["url"]; ok {
		t.Error("«url» non era selezionato e non deve uscire")
	}
	if !strings.Contains(buf.String(), "\"troncato\"") {
		t.Error("--select ha mangiato le chiavi della busta: sono di servizio, restano sempre")
	}
}

// La busta è JSON. Con --csv l'output è una tabella e avvolgerla produrrebbe
// un ibrido che nessuno dei due consumatori sa leggere.
func TestEnvelopeWanted(t *testing.T) {
	var buf bytes.Buffer // non è un terminale: il default è JSON
	cases := []struct {
		nome  string
		flags rootFlags
		want  bool
	}{
		{"flag assente", rootFlags{}, false},
		{"envelope su json", rootFlags{envelope: true, asJSON: true}, true},
		{"envelope su pipe", rootFlags{envelope: true}, true},
		{"envelope con csv", rootFlags{envelope: true, csv: true}, false},
	}
	for _, c := range cases {
		if got := envelopeWanted(&buf, &c.flags); got != c.want {
			t.Errorf("%s: envelopeWanted = %v, want %v", c.nome, got, c.want)
		}
	}
}

// Il portale ordina per data, non per pertinenza: chi ha i termini nel titolo
// va davanti, e dentro i due gruppi l'ordine di partenza resta leggibile.
func TestOrdinaPerPertinenza(t *testing.T) {
	recs := []icaro.Record{
		{Title: "Riconoscimento debiti fuori bilancio"},
		{Title: "Riforma degli ambiti territoriali e gestione integrata dei rifiuti"},
		{Title: "Legge di stabilità regionale"},
		{Title: "Norme sulla gestione dei rifiuti speciali"},
	}
	got := ordinaPerPertinenza(recs, []string{"gestione", "rifiuti"})
	if !strings.Contains(got[0].Title, "Riforma") || !strings.Contains(got[1].Title, "Norme") {
		t.Errorf("i due titoli pertinenti devono venire davanti, nell'ordine originale; got %q, %q", got[0].Title, got[1].Title)
	}
	if !strings.Contains(got[2].Title, "debiti") {
		t.Errorf("la coda deve restare nell'ordine del portale; got %q", got[2].Title)
	}
	if len(got) != len(recs) {
		t.Errorf("record persi nel riordino: %d, want %d", len(got), len(recs))
	}
	// Senza termini non si tocca nulla: --isis-query è una query costruita a
	// mano e riordinarla sarebbe una sorpresa.
	if got := ordinaPerPertinenza(recs, nil); got[0].Title != recs[0].Title {
		t.Error("nessun termine: l'ordine del portale deve restare intatto")
	}
}

// titoloAlCap restituisce il prefisso allungato fino al taglio della fonte: è
// quello che la lista del portale mostra di un titolo più lungo di 256
// caratteri, con la coda — dove stanno i termini cercati — già persa.
func titoloAlCap(prefisso string) string {
	r := []rune(prefisso)
	if len(r) >= titoloCapFonte {
		return string(r[:titoloCapFonte])
	}
	return prefisso + strings.Repeat("x", titoloCapFonte-len(r))
}

// Il portale taglia i titoli a 256 caratteri: un titolo troncato che non matcha
// non è un fuori tema, è un forse. Va davanti ai fuori tema, dietro ai match
// dimostrati — è il caso del ddl 199 sull'insularità.
func TestOrdinaPerPertinenzaTitoliTroncati(t *testing.T) {
	recs := []icaro.Record{
		{Title: "Legge di stabilità regionale"},
		{Title: titoloAlCap("Disegno di legge voto ")},
		{Title: "Norme sulla condizione di insularità delle isole minori"},
	}
	got := ordinaPerPertinenza(recs, []string{"condizione", "insularità"})
	if !strings.Contains(got[0].Title, "Norme sulla") {
		t.Errorf("il match dimostrato deve restare primo; got %q", got[0].Title)
	}
	if !strings.HasPrefix(got[1].Title, "Disegno di legge voto") {
		t.Errorf("il titolo troncato va davanti ai fuori tema; got %q", got[1].Title)
	}
	if !strings.Contains(got[2].Title, "stabilità") {
		t.Errorf("il fuori tema col titolo intero va in coda; got %q", got[2].Title)
	}
}

// L'hint scatta quando NESSUN titolo matcha: è il sintomo che il documento
// cercato sta oltre la finestra, non che non esista.
func TestPertinenzaHint(t *testing.T) {
	fuoriTema := []icaro.Record{{Title: "Riconoscimento debiti fuori bilancio"}, {Title: "Legge di stabilità"}}
	got := pertinenzaHint(fuoriTema, []string{"gestione", "rifiuti"}, "ddl")
	if got == "" {
		t.Fatal("nessun titolo pertinente: atteso un avviso")
	}
	for _, want := range []string{"gestione", "rifiuti", "--limit", "--frase"} {
		if !strings.Contains(got, want) {
			t.Errorf("avviso %q: manca %q (termini e rimedi devono esserci)", got, want)
		}
	}
	conMatch := append(fuoriTema, icaro.Record{Title: "Norme sulla gestione dei rifiuti"})
	if got := pertinenzaHint(conMatch, []string{"gestione", "rifiuti"}, "ddl"); got != "" {
		t.Errorf("c'è un titolo pertinente: atteso silenzio, ottenuto %q", got)
	}
	if got := pertinenzaHint(fuoriTema, nil, "ddl"); got != "" {
		t.Errorf("ricerca non testuale: atteso silenzio, ottenuto %q", got)
	}
}

// Sugli archivi /bd/ l'avviso non può consigliare --frase: lì il flag è
// rifiutato con un errore, quindi chi seguisse il rimedio alla lettera
// finirebbe in un vicolo cieco. Il resto dell'avviso resta identico.
func TestPertinenzaHintSenzaFraseSuBD(t *testing.T) {
	fuoriTema := []icaro.Record{{Title: "Riconoscimento debiti fuori bilancio"}, {Title: "Legge di stabilità"}}
	troncati := []icaro.Record{{Title: "Legge di stabilità"}, {Title: titoloAlCap("Disegno di legge voto ")}}
	for _, slug := range []string{"resoconti", "sommari", "convocazioni"} {
		for _, recs := range [][]icaro.Record{fuoriTema, troncati} {
			got := pertinenzaHint(recs, []string{"gestione", "rifiuti"}, slug)
			if got == "" {
				t.Fatalf("%s: nessun titolo pertinente, atteso un avviso", slug)
			}
			if strings.Contains(got, "--frase") {
				t.Errorf("%s: l'avviso consiglia --frase, che questo archivio rifiuta; got %q", slug, got)
			}
			if !strings.Contains(got, "--limit") {
				t.Errorf("%s: senza --frase deve restare almeno --limit; got %q", slug, got)
			}
		}
	}
	// Sul flusso Icaro il consiglio resta: lì --frase funziona davvero.
	if got := pertinenzaHint(fuoriTema, []string{"gestione", "rifiuti"}, "ddl"); !strings.Contains(got, "--frase") {
		t.Errorf("archivio Icaro: --frase va ancora consigliato; got %q", got)
	}
}

// Con un titolo troncato dalla fonte l'avviso non può dire «nessuno ha i
// termini nel titolo»: sarebbe falso, il termine può stare nei caratteri
// tagliati. Deve dirlo, e mandare ad aprire il documento.
func TestPertinenzaHintTitoliTroncati(t *testing.T) {
	recs := []icaro.Record{
		{Title: "Riconoscimento debiti fuori bilancio"},
		{Title: titoloAlCap("Disegno di legge voto ")},
	}
	got := pertinenzaHint(recs, []string{"condizione", "insularità"}, "ddl")
	if got == "" {
		t.Fatal("nessun titolo pertinente: atteso un avviso")
	}
	for _, want := range []string{"1 titolo è tagliato", "256", "titolo visibile", "apri il documento"} {
		if !strings.Contains(strings.ToLower(got), strings.ToLower(want)) {
			t.Errorf("avviso %q: manca %q", got, want)
		}
	}
	// «Apri il documento» va detto col comando che lo apre: chi legge l'avviso è
	// chi ha meno contesto, e cercarsi il sottocomando è lavoro evitabile.
	if !strings.Contains(got, "ddl get <legisl> <numero>") {
		t.Errorf("avviso %q: manca il comando che apre il documento", got)
	}
	// Sugli archivi senza `get` (sommari, convocazioni, biblioteca) non si
	// inventa un comando che non esiste: resta il consiglio generico.
	sommari := pertinenzaHint(recs, []string{"condizione", "insularità"}, "sommari")
	if strings.Contains(sommari, " get <legisl>") {
		t.Errorf("avviso %q: sommari non ha un sottocomando get", sommari)
	}
	// Sulle leggi il numero da solo non identifica l'atto: senza --anno il
	// comando suggerito apre la legge di un altro anno.
	leggi := pertinenzaHint(recs, []string{"condizione", "insularità"}, "leggi")
	if !strings.Contains(leggi, "leggi get <legisl> <numero> --anno <anno>") {
		t.Errorf("avviso %q: sulle leggi il comando va dato con --anno", leggi)
	}
	// Due troncati: il conteggio si accorda, «1 titoli» è un bug che si legge.
	due := append(recs, icaro.Record{Title: titoloAlCap("Schema di progetto di legge ")})
	if got := pertinenzaHint(due, []string{"condizione", "insularità"}, "ddl"); !strings.Contains(got, "2 titoli sono tagliati") {
		t.Errorf("avviso %q: atteso il plurale accordato", got)
	}
	// Senza troncati resta l'avviso di prima, che non parla di taglio.
	interi := []icaro.Record{{Title: "Riconoscimento debiti fuori bilancio"}}
	if got := pertinenzaHint(interi, []string{"gestione", "rifiuti"}, "ddl"); strings.Contains(got, "tagliat") {
		t.Errorf("nessun titolo troncato: l'avviso non deve parlare di taglio; got %q", got)
	}
}

// Solo il testo libero viene riordinato. Le parole corte e gli operatori ISIS
// non sono termini di ricerca, e una query con parentesi è roba costruita a
// mano da chi sa cosa sta facendo.
func TestTerminiRicerca(t *testing.T) {
	cases := []struct {
		in   map[string]string
		want int
	}{
		{map[string]string{"testo": "gestione rifiuti"}, 2},
		{map[string]string{"frase": "aree idonee"}, 2},
		{map[string]string{"testo": "la e di gestione"}, 1}, // parole corte e operatori fuori
		{map[string]string{"testo": "(aree E idonee)"}, 0},  // query esplicita: non si tocca
		{map[string]string{"legisl": "18"}, 0},
	}
	for _, c := range cases {
		if got := terminiRicerca(c.in); len(got) != c.want {
			t.Errorf("terminiRicerca(%v) = %v (%d termini), want %d", c.in, got, len(got), c.want)
		}
	}
}

// Due righe con lo stesso numero non sono un doppione da scartare: sul ddl 6030
// il portale tiene due documenti diversi — uno con il testo e l'iter
// aggiornato, l'altro la sola scheda ferma a due settimane prima — identici in
// ogni campo della lista. Chi legge deve saperlo, o ne butta via uno a caso.
func TestOmonimiHint(t *testing.T) {
	riga := func(legisl, numero string) icaro.Record {
		return icaro.Record{Fields: map[string]string{"Legisl.": legisl, "Numero": numero}}
	}
	due := []icaro.Record{riga("18", "6030"), riga("18", "6030")}
	got := omonimiHint(due, "ddl")
	for _, want := range []string{"1 numero compare", "6030", "ddl get <legisl> <numero>", "docno"} {
		if !strings.Contains(got, want) {
			t.Errorf("avviso %q: manca %q", got, want)
		}
	}
	// Numeri distinti: nessun avviso, o si presenterebbe a ogni ricerca.
	distinti := []icaro.Record{riga("18", "6030"), riga("18", "6031")}
	if got := omonimiHint(distinti, "ddl"); got != "" {
		t.Errorf("numeri distinti: atteso nessun avviso; got %q", got)
	}
	// Stesso numero ma legislature diverse: sono due atti, non due versioni.
	crossLeg := []icaro.Record{riga("17", "199"), riga("18", "199")}
	if got := omonimiHint(crossLeg, "ddl"); got != "" {
		t.Errorf("legislature diverse: atteso nessun avviso; got %q", got)
	}
	// leggi è indicizzato per articolo e resoconti per punto dell'ordine del
	// giorno: lì le righe ripetute sono la norma e l'avviso sarebbe rumore a
	// ogni ricerca.
	for _, slug := range []string{"leggi", "resoconti"} {
		if got := omonimiHint(due, slug); got != "" {
			t.Errorf("%s: righe ripetute sono la norma, atteso nessun avviso; got %q", slug, got)
		}
	}
}

// Sugli archivi Icaro il campo ISIS "Titolo" arriva già dentro Fields e
// coincide con Record.Title. Sui tre archivi /bd/ (resoconti, sommari,
// convocazioni) quel campo non esiste nel parsing: senza fallback, "titolo"
// resterebbe vuoto mentre "title" è popolato — vedi
// docs/news-agent/2026-08-07_08-43.md.
func TestTitoloAlias(t *testing.T) {
	icaroRecord := icaro.Record{Title: "Legge di stabilità", Fields: map[string]string{"Titolo": "Legge di stabilità"}}
	if got := titoloAlias(icaroRecord); got != "Legge di stabilità" {
		t.Errorf("record Icaro: titoloAlias = %q, want %q", got, "Legge di stabilità")
	}
	bdRecord := icaro.Record{Title: "Resoconto d'Aula provvisorio della Seduta n. 270", Fields: map[string]string{"Legisl.": "18", "Numero": "270"}}
	if got := titoloAlias(bdRecord); got != bdRecord.Title {
		t.Errorf("record /bd/ senza Fields[Titolo]: titoloAlias = %q, want fallback a Title %q", got, bdRecord.Title)
	}
}

func TestFlatRecordsTitoloSempreValorizzato(t *testing.T) {
	recs := []icaro.Record{
		{Title: "Atto Icaro", Fields: map[string]string{"Titolo": "Atto Icaro", "Legisl.": "18"}},
		{Title: "Seduta /bd/", Fields: map[string]string{"Legisl.": "18", "Numero": "270"}},
	}
	flat := flatRecords(recs, nil)
	for i, row := range flat {
		titolo, _ := row["titolo"].(string)
		if titolo == "" {
			t.Errorf("riga %d (%q): titolo assente in output", i, recs[i].Title)
		}
		if titolo != recs[i].Title {
			t.Errorf("riga %d: titolo = %q, want %q", i, titolo, recs[i].Title)
		}
	}
}

// Il `get` che non produce il documento ha due esiti diversi da tenere
// distinti: il backend ha risposto e il record non c'è, oppure il backend non
// ha risposto affatto. Fino al 2026-08-12 finivano entrambi in «nessun
// documento trovato», che sul secondo caso afferma il falso: la seduta 268
// esisteva, era il portale a troncare le risposte.
func TestGetMissingErr(t *testing.T) {
	t.Run("backend risponde, record assente", func(t *testing.T) {
		err := getMissingErr("resoconti", 18, 999, nil)
		if !strings.Contains(err.Error(), "nessun documento trovato") {
			t.Errorf("il not-found vero deve restare tale, ho avuto: %v", err)
		}
	})

	t.Run("backend non risponde", func(t *testing.T) {
		bdErr := errors.New("bd session (resoconti): stream error: stream ID 1; INTERNAL_ERROR")
		err := getMissingErr("resoconti", 18, 268, bdErr)
		if strings.Contains(err.Error(), "nessun documento trovato") {
			t.Errorf("un backend muto non è un documento inesistente, ho avuto: %v", err)
		}
		if !strings.Contains(err.Error(), "backend /bd/") {
			t.Errorf("l'errore deve nominare il backend, ho avuto: %v", err)
		}
		if !errors.Is(err, bdErr) {
			t.Errorf("la causa originale deve restare leggibile, ho avuto: %v", err)
		}
	})

	t.Run("429 tiene il suo codice di uscita", func(t *testing.T) {
		err := getMissingErr("resoconti", 18, 268, &icaro.HTTPRateLimitError{URL: "https://dati.ars.sicilia.it/bd/resoconti"})
		var ce *cliError
		if !errors.As(err, &ce) {
			t.Fatalf("il 429 deve restare un cliError con codice dedicato, ho avuto: %v", err)
		}
		if ce.code != 7 {
			t.Errorf("codice di uscita = %d, atteso 7 (rate limit)", ce.code)
		}
	})
}

// «Riprova» non è un consiglio utile su un portale che tronca le risposte
// grandi: quello che cambia l'esito è chiedere meno righe. Misurato su sommari
// il 2026-08-12: la ricerca di una singola seduta è arrivata 8 volte su 8,
// quella senza filtri zero su 8.
func TestRestringiHint(t *testing.T) {
	// Il guasto generico del backend: è il caso in cui restringere serve davvero.
	errBoom := errors.New("bd session (sommari): stream error: stream ID 1; INTERNAL_ERROR")

	t.Run("suggerisce i filtri che mancano", func(t *testing.T) {
		h := restringiHint(errBoom, "sommari", map[string]string{"legisl": "18"})
		for _, atteso := range []string{"--numero", "--anno", "--commissione"} {
			if !strings.Contains(h, atteso) {
				t.Errorf("hint = %q, ci si aspettava %s", h, atteso)
			}
		}
	})

	t.Run("non ripete i filtri già messi", func(t *testing.T) {
		h := restringiHint(errBoom, "sommari", map[string]string{"legisl": "18", "anno": "2025", "codcom": "1"})
		if strings.Contains(h, "--anno") || strings.Contains(h, "--commissione") {
			t.Errorf("hint = %q: non deve chiedere filtri già presenti (codcom vale per commissione)", h)
		}
		if !strings.Contains(h, "--numero") {
			t.Errorf("hint = %q: --numero manca ancora, va suggerito", h)
		}
	})

	t.Run("tace se non c'è più niente da stringere", func(t *testing.T) {
		h := restringiHint(errBoom, "sommari", map[string]string{"legisl": "18", "numero": "270", "data": "2026-07-08", "commissione": "Affari"})
		if h != "" {
			t.Errorf("hint = %q, atteso vuoto: dare la colpa a chi ha già ristretto tutto è scorretto", h)
		}
	})

	t.Run("convocazioni non ha il numero di seduta", func(t *testing.T) {
		// Verificato sul form del portale: /bd/convocazioni espone legislatura,
		// anno, commissioni, invitati e testo. Suggerire --numero manderebbe
		// l'utente contro un flag che non esiste.
		h := restringiHint(errBoom, "convocazioni", map[string]string{"legisl": "18"})
		if strings.Contains(h, "--numero") {
			t.Errorf("hint = %q: convocazioni non ha un filtro per numero di seduta", h)
		}
	})

	t.Run("archivi Icaro non c'entrano", func(t *testing.T) {
		if h := restringiHint(errBoom, "ddl", map[string]string{}); h != "" {
			t.Errorf("hint = %q: la troncatura è un difetto del backend /bd/", h)
		}
	})

	// Un nome che non esiste in anagrafica non si trova restringendo: l'hint
	// uscirebbe sopra il messaggio che spiega il vero rimedio («prova con il solo
	// cognome») e manderebbe a mettere --numero o --anno per nulla.
	t.Run("tace se il filtro non esiste in anagrafica", func(t *testing.T) {
		irrisolto := &icaro.UnresolvedFilterError{Filtro: "--oratore", Valore: "Pincopallo", Legisl: "18"}
		if h := restringiHint(irrisolto, "resoconti", map[string]string{"legisl": "18"}); h != "" {
			t.Errorf("hint = %q: restringere non fa esistere un oratore che non c'è", h)
		}
		wrapped := fmt.Errorf("ricerca resoconti: %w", irrisolto)
		if h := restringiHint(wrapped, "resoconti", map[string]string{"legisl": "18"}); h != "" {
			t.Errorf("hint = %q: vale anche quando l'errore arriva incartato", h)
		}
	})
}
