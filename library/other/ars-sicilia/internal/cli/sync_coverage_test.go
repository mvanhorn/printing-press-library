package cli

import (
	"testing"
	"time"

	icaro "github.com/mvanhorn/printing-press-library/library/other/ars-sicilia/internal/icaroclient"
)

// Il caso che ha smentito la prima versione del comando: le prime dieci righe di
// `leggi` sono i dieci articoli della stessa legge, tutti con la stessa data.
// «Non sale mai» era vero e faceva fermare la misura alla prima riga, dando
// 2026-01-05 su un archivio che nel 2026 arriva al 22 luglio.
func TestDecrescente_DateCostantiNonProvanoLOrdine(t *testing.T) {
	costanti := []string{"2026-01-05", "2026-01-05", "2026-01-05"}
	if decrescente(costanti) {
		t.Fatal("date tutte uguali: l'ordine non è dimostrato, la prima riga non può valere come massimo")
	}
	crescenti := []string{"2026-01-05", "2026-01-30", "2026-03-13"}
	if decrescente(crescenti) {
		t.Fatal("date crescenti: il massimo sta in fondo, non in cima")
	}
	scendono := []string{"2026-07-28", "2026-07-21", "2026-07-21"}
	if !decrescente(scendono) {
		t.Fatal("date che scendono: la prima è il massimo e una pagina basta")
	}
	if decrescente([]string{"2026-07-28"}) {
		t.Fatal("una riga sola non dimostra nessun ordine")
	}
}

func TestDataOrdinabile_LeDueFormeDeiDueMotori(t *testing.T) {
	cases := map[string]string{
		"28.07.26":    "2026-07-28", // flusso Icaro, anno a due cifre
		"5.01.2026":   "2026-01-05", // flusso Icaro, giorno a una cifra
		"05/08/2026":  "2026-08-05", // pagine /bd/
		"":            "",
		"17 luglio 2": "", // pareri: data a parole e tagliata, non interpretabile
	}
	for in, want := range cases {
		if got := dataOrdinabile(in); got != want {
			t.Errorf("dataOrdinabile(%q) = %q, atteso %q", in, got, want)
		}
	}
}

func TestMassimo(t *testing.T) {
	if got := massimo([]string{"2026-01-05", "2026-07-22", "2026-05-28"}); got != "2026-07-22" {
		t.Fatalf("massimo = %q, atteso 2026-07-22", got)
	}
}

// La finestra deve nascere da un campo VERO: `anno` sugli archivi che ce l'hanno,
// l'intervallo su `data` per gli altri. Se `anno` finisse in ricerca libera su un
// archivio che non lo mappa, si cercherebbe «2026» come testo nei documenti e il
// risultato verrebbe presentato come una misura di copertura.
func TestFinestraAnno_UsaSoloCampiEsistenti(t *testing.T) {
	casi := []struct {
		slug   string
		chiave string // "" = archivio non misurabile
		valore string
	}{
		{"ddl", "anno", "2026"},
		{"leggi", "anno", "2026"},
		{"resoconti", "anno", "2026"},    // /bd/: anno nativo del form
		{"sommari", "anno", "2026"},      // /bd/
		{"convocazioni", "anno", "2026"}, // /bd/
		{"mozioni", "data", "2026-01-01:2026-12-31"},
		{"interrogazioni", "data", "2026-01-01:2026-12-31"},
		{"pareri", "", ""},     // né anno né data fra i campi interrogabili
		{"biblioteca", "", ""}, // nemmeno una colonna data
	}
	for _, c := range casi {
		arc := icaro.BySlug(c.slug)
		if arc == nil {
			t.Fatalf("archivio %q non trovato", c.slug)
		}
		got := finestraAnno(*arc, 2026)
		if c.chiave == "" {
			if got != nil {
				t.Errorf("%s: attesa nessuna finestra, ottenuto %v", c.slug, got)
			}
			continue
		}
		if got[c.chiave] != c.valore {
			t.Errorf("%s: finestra = %v, atteso %s=%s", c.slug, got, c.chiave, c.valore)
		}
	}
}

func TestHaColonnaData(t *testing.T) {
	if !haColonnaData(*icaro.BySlug("pareri")) {
		t.Error("pareri ha una colonna data nel listato")
	}
	if haColonnaData(*icaro.BySlug("biblioteca")) {
		t.Error("biblioteca non ha colonna data")
	}
}

// Uno slug scritto male deve fermare il comando: ignorarlo darebbe un rapporto
// silenziosamente più corto di quello chiesto.
func TestCoverageArchivi_SlugIgnotoEUnErrore(t *testing.T) {
	if _, err := coverageArchivi([]string{"ddl", "leggine"}); err == nil {
		t.Fatal("slug sconosciuto accettato senza errore")
	}
	got, err := coverageArchivi([]string{"ddl", "resoconti"})
	if err != nil || len(got) != 2 {
		t.Fatalf("coverageArchivi = %v, %v", got, err)
	}
	tutti, err := coverageArchivi(nil)
	if err != nil || len(tutti) != len(icaro.All) {
		t.Fatalf("senza --resources si misurano tutti gli archivi: %d", len(tutti))
	}
}

// Il ritardo si conta in giorni di calendario. Contarlo come ore/24 mescolava
// due fusi — `time.Parse` dà mezzanotte UTC, `oggi` è l'ora locale — e in agosto
// (UTC+2) una seduta annunciata per domani faceva −15h → int(−0,625) → 0: la
// nota sulle date future non usciva mai sotto le 24 ore di anticipo, e
// `convocazioni`, che quelle date le annuncia per mestiere, si leggeva come un
// archivio fermo a oggi.
func TestDatare_GiorniDiCalendarioNonOreDiviso24(t *testing.T) {
	// Ora locale tarda: è il momento in cui lo scarto fra i fusi è massimo.
	oggi := time.Date(2026, 8, 14, 23, 30, 0, 0, time.FixedZone("CEST", 2*60*60))

	casi := []struct {
		nome, max string
		giorni    int
		nota      bool
	}{
		{"documento di oggi", "2026-08-14", 0, false},
		{"documento di ieri", "2026-08-13", 1, false},
		{"seduta di domani", "2026-08-15", -1, true},
		{"seduta fra tre settimane", "2026-09-02", -19, true},
		{"archivio indietro di un mese", "2026-07-14", 31, false},
	}
	for _, c := range casi {
		t.Run(c.nome, func(t *testing.T) {
			var e coverageEntry
			datare(&e, c.max, oggi)
			if e.RitardoGiorni == nil {
				t.Fatalf("ritardo non misurato per %q", c.max)
			}
			if *e.RitardoGiorni != c.giorni {
				t.Errorf("ritardo = %d, atteso %d", *e.RitardoGiorni, c.giorni)
			}
			if hasNota := e.Nota != ""; hasNota != c.nota {
				t.Errorf("nota = %q, attesa presente=%v: una data futura va spiegata, una passata no", e.Nota, c.nota)
			}
		})
	}
}

// Zero giorni di ritardo e "non l'abbiamo misurato" sono due risposte diverse:
// con un int e omitempty finivano tutte e due fuori dal JSON.
func TestDatare_NonMisuratoNonEZero(t *testing.T) {
	oggi := time.Date(2026, 8, 14, 11, 0, 0, 0, time.UTC)

	var fresco coverageEntry
	datare(&fresco, "2026-08-14", oggi)
	if fresco.RitardoGiorni == nil || *fresco.RitardoGiorni != 0 {
		t.Fatalf("ritardo = %v, atteso 0 valorizzato: un archivio aggiornato a oggi non deve sparire", fresco.RitardoGiorni)
	}

	// Data che il portale scrive in una forma che non sappiamo leggere.
	var illeggibile coverageEntry
	datare(&illeggibile, "17 luglio 2", oggi)
	if illeggibile.RitardoGiorni != nil {
		t.Errorf("ritardo = %d su una data non interpretabile: doveva restare non misurato", *illeggibile.RitardoGiorni)
	}
	if illeggibile.UltimoRecord != "17 luglio 2" {
		t.Errorf("ultimo record = %q: quello che la fonte ha scritto va riportato comunque", illeggibile.UltimoRecord)
	}
}
