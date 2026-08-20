package cli

import "testing"

import icaro "github.com/mvanhorn/printing-press-library/library/other/ars-sicilia/internal/icaroclient"

func rigaLegge(legisl, atto, docum, data, titolo string) icaro.Record {
	return icaro.Record{
		Title: titolo,
		Fields: map[string]string{
			"Legisl.": legisl, "Atto": atto, "Docum.": docum, "Data": data,
		},
	}
}

func TestCollapseLeggi(t *testing.T) {
	recs := []icaro.Record{
		rigaLegge("18", "L.R. 1", "Art. 1", "9.01.2025", "Legge di stabilità 2025-2027"),
		rigaLegge("18", "L.R. 1", "Art. 2", "9.01.2025", "Legge di stabilità 2025-2027"),
		rigaLegge("18", "L.R. 1", "Art. 3", "9.01.2025", "Legge di stabilità 2025-2027"),
		rigaLegge("18", "L.R. 2", "Art. 1", "9.01.2025", "Bilancio di previsione"),
	}
	out := collapseLeggi(recs)
	if len(out) != 2 {
		t.Fatalf("leggi = %d, attese 2: %+v", len(out), out)
	}
	if out[0].Atto != "L.R. 1" || out[0].ArticoliTrovati != 3 {
		t.Errorf("prima legge = %+v, attesa L.R. 1 con 3 articoli", out[0])
	}
	if len(out[0].Articoli) != 3 || out[0].Articoli[2] != "Art. 3" {
		t.Errorf("articoli = %v, attesi tre in ordine", out[0].Articoli)
	}
	if out[1].Atto != "L.R. 2" || out[1].ArticoliTrovati != 1 {
		t.Errorf("seconda legge = %+v, attesa L.R. 2 con 1 articolo", out[1])
	}
}

// "L.R. 1" si ripete ogni anno: senza la data nella chiave, la legge di
// stabilità del 2025 e quella del 2026 collasserebbero in una riga sola —
// cioè lo stesso errore di sotto-numerazione che l'aggregazione deve togliere.
func TestCollapseLeggiNonFondeAnniDiversi(t *testing.T) {
	recs := []icaro.Record{
		rigaLegge("18", "L.R. 1", "Art. 1", "9.01.2025", "Legge di stabilità 2025-2027"),
		rigaLegge("18", "L.R. 1", "Art. 1", "5.01.2026", "Legge di stabilità 2026-2028"),
	}
	if out := collapseLeggi(recs); len(out) != 2 {
		t.Fatalf("leggi = %d, attese 2 (anni diversi): %+v", len(out), out)
	}
}

// L'ordine di arrivo dal portale (per data decrescente) non va perso.
func TestCollapseLeggiConservaOrdine(t *testing.T) {
	recs := []icaro.Record{
		rigaLegge("18", "L.R. 10", "Art. 1", "1.07.2026", "Debiti fuori bilancio"),
		rigaLegge("18", "L.R. 7", "Art. 1", "2.06.2026", "Altro"),
		rigaLegge("18", "L.R. 10", "Art. 2", "1.07.2026", "Debiti fuori bilancio"),
	}
	out := collapseLeggi(recs)
	if len(out) != 2 || out[0].Atto != "L.R. 10" || out[1].Atto != "L.R. 7" {
		t.Fatalf("ordine = %+v, atteso L.R. 10 poi L.R. 7", out)
	}
	if out[0].ArticoliTrovati != 2 {
		t.Errorf("articoli L.R. 10 = %d, attesi 2 (righe non adiacenti)", out[0].ArticoliTrovati)
	}
}

// Il limite è espresso in leggi: le righe da scaricare sono molte di più,
// perché ogni legge ne occupa una per articolo. Qui è solo il tetto — quando
// fermarsi lo decide StopWhen contando le leggi raccolte.
func TestLeggiRawLimit(t *testing.T) {
	casi := map[int]int{0: 300, 1: 30, 10: 300, 25: 500, 1000: 500}
	for in, vuole := range casi {
		if got := leggiRawLimit(in); got != vuole {
			t.Errorf("leggiRawLimit(%d) = %d, atteso %d", in, got, vuole)
		}
	}
	// Il tetto deve restare sopra la finestra che serviva alle finanziarie:
	// con ~25 righe-articolo l'una, 100 righe rendevano 4 leggi su 10 chieste.
	if leggiRawLimit(10) <= 100 {
		t.Errorf("tetto per 10 leggi = %d: troppo basso, è il caso che il fix esiste per risolvere", leggiRawLimit(10))
	}
}
