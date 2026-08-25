package wwwclient

import (
	"os"
	"path/filepath"
	"testing"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("lettura fixture %s: %v", name, err)
	}
	return body
}

// L'elenco della XVIII ha 10 gruppi, la XVII 11, la XVI 15 (quest'ultima
// pagina contiene anche card div.dep senza a.underline: contano solo le
// card col link al gruppo, per questo il numero è 15 e non 18).
func TestParseGruppiElenco(t *testing.T) {
	casi := []struct {
		legisl  int
		fixture string
		want    int
		primo   string
	}{
		{18, "gruppi-elenco-18.html", 10, "XVIII-forza-italia-allars"},
		{17, "gruppi-elenco-17.html", 11, "XVII-movimento-cinque-stelle"},
		{16, "gruppi-elenco-16.html", 15, "XVI-movimento-cinque-stelle"},
	}
	for _, c := range casi {
		got, err := ParseGruppiElenco(fixture(t, c.fixture), c.legisl)
		if err != nil {
			t.Fatalf("elenco legisl %d: %v", c.legisl, err)
		}
		if len(got) != c.want {
			t.Fatalf("elenco legisl %d: %d gruppi, attesi %d", c.legisl, len(got), c.want)
		}
		if got[0].Slug != c.primo {
			t.Fatalf("elenco legisl %d: primo slug %q, atteso %q", c.legisl, got[0].Slug, c.primo)
		}
		if got[0].Legisl != c.legisl || got[0].Nome == "" || got[0].URL == "" {
			t.Fatalf("elenco legisl %d: riga incompleta %+v", c.legisl, got[0])
		}
	}
}

// Un'estrazione vuota è errore, non risultato: zero gruppi significa
// selettore rotto, e un test su pagina sconosciuta deve vederlo.
func TestParseGruppiElencoVuoto(t *testing.T) {
	if _, err := ParseGruppiElenco([]byte("<html><body><h1>Altro</h1></body></html>"), 18); err == nil {
		t.Fatal("atteso errore su pagina senza gruppi")
	}
}

func TestParseGruppoDettaglioMisto18(t *testing.T) {
	det, err := ParseGruppoDettaglio(fixture(t, "gruppi-dettaglio-XVIII-misto.html"), "XVIII-misto")
	if err != nil {
		t.Fatal(err)
	}
	if det.Gruppo != "Misto" {
		t.Fatalf("nome gruppo %q, atteso \"Misto\"", det.Gruppo)
	}
	if det.Email != "GruppoMisto@ars.sicilia.it" {
		t.Fatalf("email gruppo %q", det.Email)
	}
	if len(det.Componenti) != 3 {
		t.Fatalf("%d componenti, attesi 3", len(det.Componenti))
	}
	primo := det.Componenti[0]
	if primo.Deputato != "On.Miccichè Gianfranco" || primo.Ruolo != "Presidente Gruppo Parlamentare" || primo.Collegio != "Palermo" {
		t.Fatalf("primo componente %+v", primo)
	}
	if primo.Scheda != "https://www.ars.sicilia.it/deputati/micciche-gianfranco" {
		t.Fatalf("scheda %q", primo.Scheda)
	}
	if primo.Email != "gfmicciche@ars.sicilia.it" {
		t.Fatalf("email deputato %q", primo.Email)
	}
	// Caronia: nessun ruolo — il campo deve restare vuoto, non "Nessuno".
	if det.Componenti[1].Ruolo != "" || det.Componenti[1].Collegio != "Regionale" {
		t.Fatalf("secondo componente %+v", det.Componenti[1])
	}
	if det.Legisl != 18 {
		t.Fatalf("legislatura %d", det.Legisl)
	}
}

func TestParseGruppoDettaglioPD18(t *testing.T) {
	det, err := ParseGruppoDettaglio(fixture(t, "gruppi-dettaglio-XVIII-partito-democratico-xviii-legislatura.html"), "XVIII-partito-democratico-xviii-legislatura")
	if err != nil {
		t.Fatal(err)
	}
	if len(det.Componenti) != 11 {
		t.Fatalf("%d componenti, attesi 11", len(det.Componenti))
	}
	if det.Componenti[0].Deputato != "On.Catanzaro Michele" || det.Componenti[0].Ruolo != "Presidente Gruppo Parlamentare" || det.Componenti[0].Collegio != "Agrigento" {
		t.Fatalf("primo componente %+v", det.Componenti[0])
	}
	if det.Gruppo != "Partito Democratico XVIII Legislatura" {
		t.Fatalf("nome gruppo %q (l'apostrofo e le entità HTML devono essere decodificati)", det.Gruppo)
	}
}

// La XVII ha lo stesso markup: il parser non deve assumere nulla di
// specifico della legislatura corrente.
func TestParseGruppoDettaglioMisto17(t *testing.T) {
	det, err := ParseGruppoDettaglio(fixture(t, "gruppi-dettaglio-XVII-misto.html"), "XVII-misto")
	if err != nil {
		t.Fatal(err)
	}
	if len(det.Componenti) != 5 {
		t.Fatalf("%d componenti, attesi 5", len(det.Componenti))
	}
	if det.Componenti[0].Ruolo != "Presidente Gruppo Parlamentare" || det.Componenti[0].Collegio != "Messina" {
		t.Fatalf("primo componente %+v", det.Componenti[0])
	}
	if det.Legisl != 17 {
		t.Fatalf("legislatura %d", det.Legisl)
	}
}

// L'invariante dell'Assemblea: la somma dei componenti dei gruppi della
// XVIII fa 70, il numero dei deputati ARS. Se il parser perde un campo o
// un componente, la somma scende e questo test fallisce.
func TestSommaComponentiXVIIIE70(t *testing.T) {
	elenco, err := ParseGruppiElenco(fixture(t, "gruppi-elenco-18.html"), 18)
	if err != nil {
		t.Fatal(err)
	}
	tot := 0
	for _, g := range elenco {
		det, err := ParseGruppoDettaglio(fixture(t, "gruppi-dettaglio-"+g.Slug+".html"), g.Slug)
		if err != nil {
			t.Fatalf("gruppo %s: %v", g.Slug, err)
		}
		for _, c := range det.Componenti {
			if c.Deputato == "" || c.Scheda == "" {
				t.Fatalf("gruppo %s: componente incompleto %+v", g.Slug, c)
			}
		}
		tot += len(det.Componenti)
	}
	if tot != 70 {
		t.Fatalf("somma componenti XVIII = %d, attesi 70 (deputati ARS)", tot)
	}
}

func TestLegislatureDaSlug(t *testing.T) {
	casi := map[string]int{"XVIII-misto": 18, "XVII-movimento-cinque-stelle": 17, "VI-nope": 0, "qualsiasi": 0}
	for slug, want := range casi {
		if got := legislatureDaSlug(slug); got != want {
			t.Fatalf("slug %q → %d, atteso %d", slug, got, want)
		}
	}
}
