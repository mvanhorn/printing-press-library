package icaroclient

import (
	"strings"
	"testing"
)

const bdSommariFixture = `
<html><body>
<ul class="tabella">
  <li class="intestazione">
    <div class="intesta intesta_10"><p>Legisl.</p></div>
    <div class="intesta intesta_10"><p>Data</p></div>
    <div class="intesta intesta_10"><p>N. Seduta</p></div>
    <div class="intesta intesta_40"><p>Commissione e Ordine del giorno</p></div>
  </li>
  <li>
    <div class="intesta intesta_10"><strong><span class="simobile">Legisl.</span></strong><p> XVIII </p></div>
    <div class="intesta intesta_10"><strong><span class="simobile">Data</span></strong><p> 14/07/2026 </p></div>
    <div class="intesta intesta_10"><strong><span class="simobile">N. Seduta</span></strong><p> 271 </p></div>
    <div class="intesta intesta_40"><strong><span class="simobile">Commissione e Ordine del giorno</span></strong>
      <h3><a href="javascript: openRisultati('18','116','271')"> I - Affari Istituzionali </a></h3>
      <p> 1) Esame del DEFR &quot;2027-2029&quot; </p></div>
  </li>
</ul>
<div class="pagination"><span class="pagina_di">Pagina 1 di 23</span></div>
</body></html>`

func TestParseBDList(t *testing.T) {
	rows, pages, err := parseBDList(bdSommariFixture, Archive{Slug: "sommari"}, "https://dati.ars.sicilia.it")
	if err != nil {
		t.Fatalf("parseBDList: %v", err)
	}
	if pages != 23 {
		t.Errorf("pages = %d, want 23", pages)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1 (header must be skipped)", len(rows))
	}
	r := rows[0]
	if r.Fields["Legisl."] != "18" { // "XVIII" normalizzato in arabo
		t.Errorf("Legisl. = %q, want 18", r.Fields["Legisl."])
	}
	if r.Fields["Data"] != "14/07/2026" {
		t.Errorf("Data = %q", r.Fields["Data"])
	}
	if r.Fields["Numero"] != "271" { // "N. Seduta" normalizzato su "Numero"
		t.Errorf("Numero = %q", r.Fields["Numero"])
	}
	if r.Title != "I - Affari Istituzionali" {
		t.Errorf("Title = %q", r.Title)
	}
	if r.Excerpt != `1) Esame del DEFR "2027-2029"` { // entità &quot; decodificata
		t.Errorf("Excerpt = %q", r.Excerpt)
	}
	if r.URL != "https://dati.ars.sicilia.it/bd/sommari/scheda/18/116/271" { // openRisultati('18','116','271')
		t.Errorf("URL = %q", r.URL)
	}
}

// TestParseBDList_Resoconti copre la forma resoconti: colonna "Numero" (non
// "N. Seduta") e ultima colonna "Titolo" con <h3><a> ma SENZA <p> (excerpt vuoto).
func TestParseBDList_Resoconti(t *testing.T) {
	const fixture = `<ul class="tabella">
  <li class="intestazione"><div class="intesta"><p>Legisl.</p></div></li>
  <li>
    <div class="intesta intesta_10"><strong><span class="simobile">Legisl.</span></strong><p> XVIII </p></div>
    <div class="intesta intesta_16"><strong><span class="simobile">Numero</span></strong><p> 264 </p></div>
    <div class="intesta intesta_16"><strong><span class="simobile">Data</span></strong><p> 14/07/2026 </p></div>
    <div class="intesta intesta_50"><strong><span class="simobile">Titolo</span></strong>
      <h3><a href="javascript: openRisultati('18','264')"> Resoconto d'Aula della Seduta n. 264 </a></h3></div>
  </li>
</ul><span class="pagina_di">Pagina 1 di 5</span>`
	rows, pages, err := parseBDList(fixture, Archive{Slug: "resoconti"}, "https://dati.ars.sicilia.it")
	if err != nil {
		t.Fatalf("parseBDList: %v", err)
	}
	if pages != 5 || len(rows) != 1 {
		t.Fatalf("pages=%d rows=%d, want 5/1", pages, len(rows))
	}
	r := rows[0]
	if r.Fields["Numero"] != "264" {
		t.Errorf("Numero = %q", r.Fields["Numero"])
	}
	if r.Fields["Data"] != "14/07/2026" {
		t.Errorf("Data = %q", r.Fields["Data"])
	}
	if r.Title != "Resoconto d'Aula della Seduta n. 264" {
		t.Errorf("Title = %q", r.Title)
	}
	if r.Excerpt != "" {
		t.Errorf("Excerpt = %q, want empty (nessun <p>)", r.Excerpt)
	}
	if r.URL != "https://dati.ars.sicilia.it/bd/resoconti/scheda/18/264" { // openRisultati('18','264')
		t.Errorf("URL = %q", r.URL)
	}
}

// TestParseBDList_Convocazioni copre la forma a 5 colonne: "Commissione" è una
// colonna propria (<p> semplice), "N. Foglio" -> "Numero", l'OdG è l'<h3>.
func TestParseBDList_Convocazioni(t *testing.T) {
	const fixture = `<ul class="tabella">
  <li class="intestazione"><div class="intesta"><p>Legisl.</p></div></li>
  <li>
    <div class="intesta intesta_10"><strong><span class="simobile">Legisl.</span></strong><p> XVIII </p></div>
    <div class="intesta intesta_10"><strong><span class="simobile">Data</span></strong><p> 22/07/2026 </p></div>
    <div class="intesta intesta_10"><strong><span class="simobile">N. Foglio</span></strong><p> 287 </p></div>
    <div class="intesta intesta_20"><strong><span class="simobile">Commissione</span></strong><p> I - Affari Istituzionali </p></div>
    <div class="intesta intesta_40"><strong><span class="simobile">Ordine del giorno</span></strong>
      <h3><a href="javascript: openRisultati('uuid')"> 1) Esame del ddl 779 </a></h3></div>
  </li>
</ul><span class="pagina_di">Pagina 1 di 28</span>`
	rows, pages, err := parseBDList(fixture, Archive{Slug: "convocazioni"}, "https://dati.ars.sicilia.it")
	if err != nil {
		t.Fatalf("parseBDList: %v", err)
	}
	if pages != 28 || len(rows) != 1 {
		t.Fatalf("pages=%d rows=%d, want 28/1", pages, len(rows))
	}
	r := rows[0]
	if r.Fields["Commissione"] != "I - Affari Istituzionali" {
		t.Errorf("Commissione = %q", r.Fields["Commissione"])
	}
	if r.Fields["Numero"] != "287" { // da "N. Foglio"
		t.Errorf("Numero = %q", r.Fields["Numero"])
	}
	if r.Title != "1) Esame del ddl 779" {
		t.Errorf("Title = %q", r.Title)
	}
	if r.URL != "https://dati.ars.sicilia.it/bd/convocazioni/results/uuid" { // openRisultati('uuid')
		t.Errorf("URL = %q", r.URL)
	}
}

func TestBDDateFilter(t *testing.T) {
	cases := []struct {
		in        string
		wantYears []string
		date      string // riga dd/mm/yyyy da testare
		keep      bool
	}{
		{"260714", []string{"2026"}, "14/07/2026", true},                // AAMMGG esatto
		{"260714", []string{"2026"}, "13/07/2026", false},               // altra data stesso anno
		{"2026-07-14", []string{"2026"}, "14/07/2026", true},            // ISO esatto
		{"260701/260731", []string{"2026"}, "14/07/2026", true},         // range AAMMGG, dentro
		{"260701/260731", []string{"2026"}, "01/08/2026", false},        // range AAMMGG, fuori
		{"2026-07-01:2026-07-31", []string{"2026"}, "14/07/2026", true}, // range ISO, dentro
		// range a cavallo di più anni: tutti gli anni vanno interrogati, dal
		// più recente. Con il solo anno del bound inferiore i record del 2026
		// sparivano dai risultati.
		{"2024-11-01:2026-02-28", []string{"2026", "2025", "2024"}, "15/01/2026", true},
		{"2024-11-01:2026-02-28", []string{"2026", "2025", "2024"}, "20/06/2025", true},
		{"2024-11-01:2026-02-28", []string{"2026", "2025", "2024"}, "10/10/2024", false}, // prima del bound
		{"2024-11-01:2026-02-28", []string{"2026", "2025", "2024"}, "01/03/2026", false}, // dopo il bound
		{"241101/260228", []string{"2026", "2025", "2024"}, "15/01/2026", true},          // stesso range in AAMMGG
	}
	for _, c := range cases {
		years, keep := bdDateFilter(c.in)
		if strings.Join(years, ",") != strings.Join(c.wantYears, ",") {
			t.Errorf("bdDateFilter(%q) years = %v, want %v", c.in, years, c.wantYears)
		}
		if keep == nil {
			t.Fatalf("bdDateFilter(%q) keep = nil", c.in)
		}
		if got := keep(c.date); got != c.keep {
			t.Errorf("bdDateFilter(%q).keep(%q) = %v, want %v", c.in, c.date, got, c.keep)
		}
	}
	// valore non interpretabile -> nessun filtro
	if years, keep := bdDateFilter("garbage"); keep != nil || years != nil {
		t.Errorf("bdDateFilter(garbage) = %v, keep should be nil", years)
	}
}

// I filtri senza equivalente sul backend /bd/ devono far fallire il comando:
// applicarne solo una parte restituirebbe più record di quanti ne siano chiesti.
func TestBDUnsupported(t *testing.T) {
	cases := []struct {
		name    string
		slug    string
		opts    SearchOptions
		wantErr string // sottostringa attesa nel messaggio; "" = nessun errore
	}{
		{"escludi su resoconti", "resoconti", SearchOptions{Params: map[string]string{"escludi": "bilancio"}}, "--escludi"},
		{"isis-query su sommari", "sommari", SearchOptions{ISISRaw: "Cracolici.FIRMAT"}, "--isis-query"},
		{"argomento su resoconti", "resoconti", SearchOptions{Params: map[string]string{"argomento": "sanità"}}, "--argomento"},
		{"presidente su sommari", "sommari", SearchOptions{Params: map[string]string{"presidente": "Galvagno"}}, "--presidente"},
		{"oratore su convocazioni", "convocazioni", SearchOptions{Params: map[string]string{"oratore": "Abbate"}}, "--oratore"},
		{"commissione su resoconti", "resoconti", SearchOptions{Params: map[string]string{"commissione": "PRIMA"}}, "--commissione"},
		{"param vuoto non conta", "resoconti", SearchOptions{Params: map[string]string{"escludi": "  "}}, ""},
		{"filtri supportati", "resoconti", SearchOptions{Params: map[string]string{"legisl": "18", "anno": "2026", "data": "260714", "oratore": "Abbate", "testo": "bilancio"}}, ""},
		{"commissione su sommari", "sommari", SearchOptions{Params: map[string]string{"commissione": "PRIMA", "codcom": "1"}}, ""},
	}
	for _, c := range cases {
		err := bdUnsupported(c.slug, bdArchives[c.slug], c.opts)
		if c.wantErr == "" {
			if err != nil {
				t.Errorf("%s: err = %v, want nil", c.name, err)
			}
			continue
		}
		if err == nil {
			t.Errorf("%s: err = nil, want error mentioning %s", c.name, c.wantErr)
			continue
		}
		if !strings.Contains(err.Error(), c.wantErr) {
			t.Errorf("%s: err = %q, want mention of %s", c.name, err, c.wantErr)
		}
	}
}

func TestBDSpeakers(t *testing.T) {
	// due spazi dopo <option, come nel markup del portale
	const form = `<select id="$Ispeakers" name="$Ispeakers" multiple="multiple">
<option  value="971" data-legs="18">Abbate Ignazio</option>
<option  value="32" data-legs="18,17,16,15,14,13">Cracolici Antonino</option>
<option  value="428" data-legs="13">Acanto Giuseppe</option>
<option  value="">Tutte</option>
</select>
<option selected value="18" >XVIII</option>`
	sp := parseSelectOptions(form, "$Ispeakers")
	if len(sp) != 3 { // "Tutte" e la legislatura (senza data-legs) esclusi
		t.Fatalf("parseSelectOptions = %d oratori, want 3: %+v", len(sp), sp)
	}
	cases := []struct {
		q, legisl string
		want      []string
	}{
		{"cracolici", "18", []string{"32"}}, // match + attivo in 18
		{"abbate", "18", []string{"971"}},   // case-insensitive
		{"acanto", "18", nil},               // Acanto è solo leg 13
		{"acanto", "", []string{"428"}},     // senza filtro legislatura
		{"nessuno", "18", nil},              // nessun match
	}
	for _, c := range cases {
		got := resolveOptionIDs(sp, c.q, c.legisl)
		if len(got) != len(c.want) || (len(got) == 1 && got[0] != c.want[0]) {
			t.Errorf("resolveOptionIDs(%q, legisl=%q) = %v, want %v", c.q, c.legisl, got, c.want)
		}
	}
	if !legsContains("18,17,16", "18") || legsContains("13", "18") {
		t.Error("legsContains errato")
	}
}

func TestResolveCommissioneIDs(t *testing.T) {
	// id commissione per-legislatura: "I - Affari Istituzionali" = 1 (leg13), 116 (leg18)
	opts := []bdOption{
		{ID: "1", Name: "I - Affari Istituzionali", Legs: "13"},
		{ID: "116", Name: "I - Affari Istituzionali", Legs: "18"},
		{ID: "2", Name: "II - Bilancio e Programmazione", Legs: "13"},
		{ID: "117", Name: "II - Bilancio", Legs: "18"},
		{ID: "11", Name: "Antimafia", Legs: "18"},
	}
	cases := []struct {
		cod, com, legisl string
		want             []string
	}{
		{"1", "", "18", []string{"116"}},        // codcom 1 -> "I " -> leg18
		{"1", "", "13", []string{"1"}},          // stessa commissione, leg diversa
		{"2", "", "18", []string{"117"}},        // "II " non confonde con "I "
		{"", "Bilancio", "18", []string{"117"}}, // nome, substring
		{"", "Antimafia", "18", []string{"11"}}, // commissione speciale
		{"", "inesistente", "18", []string{}},   // richiesto ma nessun match -> [] non nil
		{"7", "", "18", []string{}},             // codcom fuori 1-6 -> []
	}
	for _, c := range cases {
		got := resolveCommissioneIDs(opts, c.cod, c.com, c.legisl)
		if len(got) != len(c.want) || (len(got) == 1 && got[0] != c.want[0]) {
			t.Errorf("resolveCommissioneIDs(cod=%q com=%q leg=%q) = %v, want %v", c.cod, c.com, c.legisl, got, c.want)
		}
	}
	// nessun filtro richiesto -> nil
	if resolveCommissioneIDs(opts, "", "", "18") != nil {
		t.Error("senza codcom/commissione deve tornare nil")
	}
}

func TestDdmmyyyyToISO(t *testing.T) {
	if got := ddmmyyyyToISO("14/07/2026"); got != "20260714" {
		t.Errorf("ddmmyyyyToISO = %q", got)
	}
	if got := ddmmyyyyToISO("boh"); got != "" {
		t.Errorf("ddmmyyyyToISO(boh) = %q, want empty", got)
	}
}

func TestRomanToArabic(t *testing.T) {
	cases := map[string]string{
		"XVIII": "18", "XVII": "17", "I": "1", "IV": "4", "IX": "9", "XIV": "14",
		"18":  "18", // già arabo -> invariato
		"foo": "foo",
	}
	for in, want := range cases {
		if got := romanToArabic(in); got != want {
			t.Errorf("romanToArabic(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIsBDArchive(t *testing.T) {
	if !IsBDArchive("sommari") {
		t.Error("sommari deve essere un archivio /bd/")
	}
	if IsBDArchive("ddl") {
		t.Error("ddl NON deve essere /bd/ (resta su Icaro)")
	}
}

// Ogni archivio /bd/ con un filtro commissione deve dichiarare il modo "$S…":
// senza --legisl la commissione risolve a un id per legislatura (la IV ne ha
// nove) e il form riceve una lista. Su sommari, il cui <select> è renderizzato
// senza `multiple`, mandarla senza il selettore di modo fa rispondere 500 al
// backend — non zero risultati, proprio un errore.
func TestBDSpec_CommissioneModeDichiarato(t *testing.T) {
	for slug, spec := range bdArchives {
		if spec.commissioneField == "" {
			continue
		}
		if spec.commissioneMode == "" {
			t.Errorf("%s: commissioneField %q senza commissioneMode: una lista di id su un campo senza $S… fa 500", slug, spec.commissioneField)
		}
	}
}

// Un --data che non si parsa lasciava years e keep a nil, e searchBD proseguiva
// senza vincolo d'anno sul form e senza filtro client-side: la ricerca tornava
// l'archivio intero dall'inizio spacciandolo per l'intervallo chiesto. Qui si
// pinna il contratto su cui searchBD decide se fallire.
func TestBDDateFilter_ValoriRifiutati(t *testing.T) {
	rifiutati := []string{
		"garbage",
		"2025-01-01:garbage",
		"garbage:2025-01-01",
		"",
		"2025-13-45", // forma giusta, mese e giorno inesistenti
		"2025-02-30", // febbraio non arriva a 30
		"251345",     // stesso, in AAMMGG
	}
	for _, v := range rifiutati {
		years, keep := bdDateFilter(v)
		if years != nil || keep != nil {
			t.Errorf("bdDateFilter(%q) = (%v, keep!=nil:%v), want (nil, nil): un filtro che sparisce restituisce tutto l'archivio", v, years, keep != nil)
		}
	}
}

// Le forme accettate restano accettate, e l'intervallo copre tutti gli anni
// coinvolti: il campo `anno` del form ne prende uno solo, quindi years è la
// lista su cui searchBD cicla.
func TestBDDateFilter_ValoriAccettati(t *testing.T) {
	cases := []struct {
		in    string
		years []string
		tiene string // una data ddmmyyyy che deve passare il filtro
	}{
		{"2026-07-29", []string{"2026"}, "29/07/2026"},
		{"260729", []string{"2026"}, "29/07/2026"},
		{"2025-12-01:2026-01-31", []string{"2026", "2025"}, "15/12/2025"},
		{"251201/260131", []string{"2026", "2025"}, "31/01/2026"},
		{"2024-02-29", []string{"2024"}, "29/02/2024"}, // bisestile: esiste
	}
	for _, c := range cases {
		years, keep := bdDateFilter(c.in)
		if keep == nil {
			t.Errorf("bdDateFilter(%q): rifiutata, doveva essere accettata", c.in)
			continue
		}
		if strings.Join(years, ",") != strings.Join(c.years, ",") {
			t.Errorf("bdDateFilter(%q) years = %v, want %v", c.in, years, c.years)
		}
		if !keep(c.tiene) {
			t.Errorf("bdDateFilter(%q): la data %s doveva passare il filtro", c.in, c.tiene)
		}
	}
	// Estremi invertiti: si normalizzano, non si rifiutano.
	if _, keep := bdDateFilter("2026-01-31:2025-12-01"); keep == nil || !keep("15/12/2025") {
		t.Error("estremi invertiti: l'intervallo va normalizzato, non rifiutato")
	}
}

// AAMMGG non porta il secolo, e prefissare "20" senza guardare l'anno spediva
// nel futuro tutto l'archivio storico: `--data 510412` cercava il 2051 e
// rispondeva `[]` sulla seduta del 12/04/1951, che esiste. La finestra è
// fondata sull'archivio — il documento più antico servito da /bd/ è il
// resoconto della seduta inaugurale del 25/05/1947, e nel 1946 non c'è nulla.
func TestParseDateBounds_SecoloAAMMGG(t *testing.T) {
	cases := []struct{ in, lo, hi string }{
		{"510412", "19510412", "19510412"}, // il caso del rilievo
		{"470525", "19470525", "19470525"}, // seduta inaugurale: il confine basso
		{"991231", "19991231", "19991231"}, // ultimo giorno del Novecento
		{"000101", "20000101", "20000101"}, // primo del Duemila
		{"260722", "20260722", "20260722"}, // date correnti invariate
		{"460101", "20460101", "20460101"}, // sopra il confine: Duemila
		// Range a cavallo del secolo: gli estremi si risolvono ciascuno per sé.
		{"991201/000131", "19991201", "20000131"},
	}
	for _, c := range cases {
		lo, hi, ok := parseDateBounds(c.in)
		if !ok {
			t.Errorf("parseDateBounds(%q): rifiutata, doveva essere accettata", c.in)
			continue
		}
		if lo != c.lo || hi != c.hi {
			t.Errorf("parseDateBounds(%q) = (%s, %s), want (%s, %s)", c.in, lo, hi, c.lo, c.hi)
		}
	}
	// Il filtro client-side deve tenere la riga storica, non solo l'anno giusto.
	if _, keep := bdDateFilter("510412"); keep == nil || !keep("12/04/1951") {
		t.Error("510412: la seduta del 12/04/1951 deve passare il filtro")
	}
	if _, keep := bdDateFilter("510412"); keep != nil && keep("12/04/2051") {
		t.Error("510412: il 2051 non deve passare, è il bug che si sta correggendo")
	}
	// L'anno interrogato sul form segue il secolo scelto, o la richiesta parte
	// verso l'anno sbagliato e nessun filtro client-side può recuperarla.
	if years, _ := bdDateFilter("510412"); strings.Join(years, ",") != "1951" {
		t.Errorf("510412: years = %v, want [1951]", years)
	}
}
