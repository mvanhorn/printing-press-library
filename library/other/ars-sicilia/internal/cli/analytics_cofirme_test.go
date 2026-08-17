package cli

import (
	"context"
	"errors"
	"strings"
	"testing"

	icaro "github.com/mvanhorn/printing-press-library/library/other/ars-sicilia/internal/icaroclient"
)

// L'espressione è il cuore della funzione: se cambia, la classifica cambia senza
// che nulla fallisca. Questa è la forma pubblicata dal sito istituzionale dietro
// i propri contatori, e verificata contro di essi (Cracolici 302, Catanzaro 306).
func TestCofirmaExpr(t *testing.T) {
	got := cofirmaExpr(18, "Cracolici Antonino")
	want := "(18.LEGISL E ((Cracolici Antonino.FIRMAT) NOT (1 ADJ Cracolici Antonino).FIRMAT))"
	if got != want {
		t.Errorf("cofirmaExpr:\n  ottenuto %s\n  atteso   %s", got, want)
	}
	// La legislatura entra nell'espressione: una classifica della XVII che
	// interrogasse la XVIII sarebbe sbagliata senza dare errore.
	if e := cofirmaExpr(17, "Tizio Caio"); e[1:3] != "17" {
		t.Errorf("la legislatura non finisce nell'espressione: %s", e)
	}
}

// contatoreFinto risponde secondo una tabella nome→esito, e tiene il conto
// delle chiamate: serve a verificare non solo cosa esce, ma anche che il ciclo
// smetta quando deve smettere.
type contatoreFinto struct {
	esiti    map[string]int
	errori   map[string]error
	chiamate int
}

func (f *contatoreFinto) Count(ctx context.Context, arc icaro.Archive, opts icaro.SearchOptions) (int, error) {
	f.chiamate++
	for nome, err := range f.errori {
		if strings.Contains(opts.ISISRaw, nome) {
			return 0, err
		}
	}
	for nome, n := range f.esiti {
		if strings.Contains(opts.ISISRaw, nome) {
			return n, nil
		}
	}
	return 0, nil
}

var deputati = []cofirmaNome{
	{Display: "Rossi Anna", Query: "Rossi Anna"},
	{Display: "Bianchi Mario", Query: "Bianchi Mario"},
	{Display: "Verdi Luisa", Query: "Verdi Luisa"},
}

// La forma con cui il portale indicizza il nome non e' quella dell'anagrafica:
// accenti tolti e punteggiatura sciolta in spazi. Interrogare il display al suo
// posto restituisce zero su 49 degli 864 firmatari del seed, e la classifica li
// perde in silenzio.
func TestIsisNomeEstraeLaFormaIndicizzata(t *testing.T) {
	casi := map[string]string{
		"1 ADJ2   Ando Oscar.firmat":         "Ando Oscar",
		"1 ADJ2   D Acquisto Mario.firmat":   "D Acquisto Mario",
		"1 ADJ2   Calanducci F sco.firmat":   "Calanducci F sco",
		"1 ADJ    Cracolici Antonino.FIRMAT": "Cracolici Antonino",
		"espressione che non e' un nome":     "",
	}
	for expr, want := range casi {
		if got := isisNome(expr); got != want {
			t.Errorf("isisNome(%q) = %q, atteso %q", expr, got, want)
		}
	}
}

// Il nome interrogato e' quello indicizzato, ma la classifica deve mostrare
// quello leggibile: se si confondono, l'utente legge "D Acquisto Mario".
func TestClassificaUsaQueryPerCercareEDisplayPerMostrare(t *testing.T) {
	f := &contatoreFinto{esiti: map[string]int{"D Acquisto Mario": 7}}
	rows, persi, err := classificaCofirme(context.Background(), f, *icaro.BySlug("ddl"), 18,
		[]cofirmaNome{{Display: "D'Acquisto Mario", Query: "D Acquisto Mario"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(persi) != 0 {
		t.Fatalf("nessuno doveva restare non misurato, persi=%v", persi)
	}
	if len(rows) != 1 || rows[0].Chiave != "D'Acquisto Mario" || rows[0].Conteggio != 7 {
		t.Fatalf("atteso display D'Acquisto Mario con 7 cofirme, ottenuto %+v", rows)
	}
}

// Una richiesta persa non deve portarsi via le altre: chi non risponde finisce
// fra i non misurati e la classifica esce lo stesso, con il buco dichiarato.
func TestClassificaCofirme_TieneIRiusciti(t *testing.T) {
	f := &contatoreFinto{
		esiti:  map[string]int{"Rossi Anna": 12, "Verdi Luisa": 5},
		errori: map[string]error{"Bianchi Mario": errors.New("risposta troncata")},
	}
	rows, persi, err := classificaCofirme(context.Background(), f, *icaro.BySlug("ddl"), 18, deputati, nil)
	if err != nil {
		t.Fatalf("due deputati su tre erano misurabili, invece: %v", err)
	}
	if len(rows) != 2 {
		t.Errorf("righe = %d, attese 2: %+v", len(rows), rows)
	}
	if len(persi) != 1 || persi[0] != "Bianchi Mario" {
		t.Errorf("persi = %v, atteso [Bianchi Mario]", persi)
	}
}

// Il caso che questa branch esiste per chiudere: se non si misura nessuno, una
// lista vuota con exit 0 si legge come «nessun deputato ha mai cofirmato».
func TestClassificaCofirme_ZeroMisuratiEUnErrore(t *testing.T) {
	f := &contatoreFinto{errori: map[string]error{
		"Rossi Anna": errors.New("troncata"), "Bianchi Mario": errors.New("troncata"), "Verdi Luisa": errors.New("troncata"),
	}}
	rows, persi, err := classificaCofirme(context.Background(), f, *icaro.BySlug("ddl"), 18, deputati, nil)
	if err == nil {
		t.Fatalf("attesa una classifica fallita, invece rows=%v persi=%v", rows, persi)
	}
	if rows != nil {
		t.Errorf("rows = %v, atteso nil: una lista vuota afferma il falso", rows)
	}
}

// Una classifica in cui nessuno ha cofirmato è però un risultato legittimo, se
// il portale ha risposto a tutti: lì la lista vuota è la verità.
func TestClassificaCofirme_TuttiAZeroNonEUnErrore(t *testing.T) {
	f := &contatoreFinto{}
	rows, persi, err := classificaCofirme(context.Background(), f, *icaro.BySlug("ddl"), 18, deputati, nil)
	if err != nil {
		t.Fatalf("il portale ha risposto a tutti: %v", err)
	}
	if len(rows) != 0 || len(persi) != 0 {
		t.Errorf("rows=%v persi=%v: attese entrambe vuote", rows, persi)
	}
	if f.chiamate != 3 {
		t.Errorf("chiamate = %d, attese 3", f.chiamate)
	}
}

// Il 429 ferma il giro: incalzare un backend che ha chiesto tregua peggiora la
// situazione che segnala, e il codice di uscita 7 è quello su cui uno script
// decide di aspettare invece di dare per rotto il comando.
func TestClassificaCofirme_IlRateLimitFermaIlGiro(t *testing.T) {
	f := &contatoreFinto{
		esiti:  map[string]int{"Rossi Anna": 12},
		errori: map[string]error{"Bianchi Mario": &icaro.HTTPRateLimitError{URL: "https://dati.ars.sicilia.it/icaro"}},
	}
	_, _, err := classificaCofirme(context.Background(), f, *icaro.BySlug("ddl"), 18, deputati, nil)
	var ce *cliError
	if !errors.As(err, &ce) || ce.code != 7 {
		t.Fatalf("errore = %v, atteso cliError con codice 7", err)
	}
	if f.chiamate != 2 {
		t.Errorf("chiamate = %d, attese 2: dopo il 429 non si prosegue", f.chiamate)
	}
}
