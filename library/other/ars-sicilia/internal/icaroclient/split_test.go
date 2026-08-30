// Copyright 2026 aborruso. Licensed under Apache-2.0. See LICENSE.

package icaroclient

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

var ddlArchive = Archive{
	ID:       "221",
	Slug:     "ddl",
	FieldMap: map[string]string{"legisl": "LEGISL", "data": "DATPRE", "anno": "DATPRE"},
	Columns:  []string{"Legisl.", "Numero", "Data", "Titolo"},
}

func TestSpezzaPerAnno(t *testing.T) {
	casi := []struct {
		lo, hi string
		want   []string
	}{
		// Il caso del report: 14 mesi rifiutati, tre anni solari toccati.
		{"230101", "240229", []string{"240101/240229", "230101/231231"}},
		{"221001", "240301", []string{"240101/240301", "230101/231231", "221001/221231"}},
		// Dentro un anno solo non c'è confine di calendario da usare.
		{"230101", "231231", nil},
	}
	for _, c := range casi {
		if got := spezzaPerAnno(c.lo, c.hi); !reflect.DeepEqual(got, c.want) {
			t.Errorf("spezzaPerAnno(%s,%s) = %v, atteso %v", c.lo, c.hi, got, c.want)
		}
	}
}

// Le fette escono dalla più recente alla più vecchia: concatenarle impone
// comunque un ordine, e questo è quello che gli altri comandi già danno.
func TestSpezzaPerAnno_PiuRecentePrima(t *testing.T) {
	fette := spezzaPerAnno("230101", "240229")
	if len(fette) == 0 || !strings.HasPrefix(fette[0], "2401") {
		t.Fatalf("prima fetta = %v, attesa quella del 2024", fette)
	}
}

func TestSpezzaAMeta(t *testing.T) {
	fette := spezzaAMeta("230101", "231231")
	if len(fette) != 2 {
		t.Fatalf("spezzaAMeta = %v, attese 2 fette", fette)
	}
	if fette[0] != "230703/231231" || fette[1] != "230101/230702" {
		t.Errorf("fette = %v: attese contigue, senza buchi e senza sovrapposizioni", fette)
	}
	// Un giorno solo non si taglia oltre: è dove la discesa deve fermarsi.
	if got := spezzaAMeta("230101", "230101"); got != nil {
		t.Errorf("range di un giorno: atteso nil, ottenuto %v", got)
	}
	// Due giorni è il caso stretto che arriva davvero fin qui (chiaveRange
	// scarta gli estremi uguali): il punto medio cade dentro il primo giorno, e
	// le fette devono restare due range validi, non uno vuoto o rovesciato.
	due := spezzaAMeta("230101", "230102")
	if len(due) != 2 || due[0] != "230102/230102" || due[1] != "230101/230101" {
		t.Fatalf("range di due giorni = %v, atteso [230102/230102 230101/230101]", due)
	}
	for _, f := range due {
		lo, hi, _ := strings.Cut(f, "/")
		if lo > hi {
			t.Errorf("fetta rovesciata %q: il portale riceverebbe un range malformato", f)
		}
	}
}

// Il perno di secolo. `time.Parse("060102", …)` perna sul 68/69: leggeva
// `470101` come 2047 e `690101` come 1969, e su un range storico che attraversa
// quel confine gli estremi uscivano invertiti — `spezzaPerAnno` lo scartava e il
// rifiuto del portale tornava al chiamante invece di essere riprovato a fette.
// La finestra è quella dell'archivio: l'ARS nasce con la seduta del 25/05/1947.
func TestDaAAMMGG_PernoDiSecolo(t *testing.T) {
	casi := map[string]int{
		"470101": 1947, // la prima seduta: da qui in su è Novecento
		"680101": 1968, // con il perno di Go sarebbe stato 2068
		"690101": 1969,
		"990101": 1999,
		"000101": 2000, // sotto il 47 è Duemila
		"260829": 2026,
	}
	for in, want := range casi {
		got, err := daAAMMGG(in)
		if err != nil {
			t.Errorf("daAAMMGG(%q) = %v", in, err)
			continue
		}
		if got.Year() != want {
			t.Errorf("daAAMMGG(%q).Year() = %d, atteso %d", in, got.Year(), want)
		}
	}
	if _, err := daAAMMGG("26082"); err == nil {
		t.Error("cinque cifre: atteso errore")
	}
	if _, err := daAAMMGG("2608xx"); err == nil {
		t.Error("non numerica: atteso errore")
	}
}

// Un range storico a cavallo del perno deve spezzarsi, non essere scartato.
func TestSpezzaPerAnno_AttraversoIlPerno(t *testing.T) {
	fette := spezzaPerAnno("680101", "690630")
	if len(fette) != 2 {
		t.Fatalf("spezzaPerAnno(680101,690630) = %v, attese 2 fette", fette)
	}
	if fette[0] != "690101/690630" || fette[1] != "680101/681231" {
		t.Errorf("fette = %v, attese [690101/690630 680101/681231]", fette)
	}
}

func TestChiaveRange(t *testing.T) {
	// --anno su ddl è compilato in un range DATPRE: è spezzabile come --data.
	if k, lo, hi, ok := chiaveRange(map[string]string{"anno": "230101/231231"}); !ok || k != "anno" || lo != "230101" || hi != "231231" {
		t.Errorf("anno: %q %q %q %v", k, lo, hi, ok)
	}
	// Una data singola non è un range: non c'è niente da spezzare.
	if _, _, _, ok := chiaveRange(map[string]string{"data": "230101"}); ok {
		t.Error("data singola scambiata per range")
	}
	if _, _, _, ok := chiaveRange(map[string]string{"testo": "province"}); ok {
		t.Error("nessun parametro di data: atteso false")
	}
}

// Il confronto degli estremi deve essere cronologico, non lessicografico: dopo
// il perno di secolo le due cose non concordano piu', e `990101/001231` (il
// 1999-2000) usciva scartato pur essendo un range legittimo.
func TestChiaveRange_AttraversoIlSecolo(t *testing.T) {
	k, lo, hi, ok := chiaveRange(map[string]string{"data": "990101/001231"})
	if !ok || k != "data" || lo != "990101" || hi != "001231" {
		t.Fatalf("990101/001231: %q %q %q %v — atteso riconosciuto come range", k, lo, hi, ok)
	}
	fette := spezzaPerAnno(lo, hi)
	if len(fette) != 2 || fette[0] != "000101/001231" || fette[1] != "990101/991231" {
		t.Errorf("fette = %v, attese [000101/001231 990101/991231]", fette)
	}
	// Un range davvero rovesciato resta scartato.
	if _, _, _, ok := chiaveRange(map[string]string{"data": "001231/990101"}); ok {
		t.Error("range rovesciato (2000 -> 1999) accettato")
	}
}

// serverSpezzato rifiuta le query il cui range supera maxGiorni, come fa il
// portale, e risponde con la sua pagina d'errore. Sotto quella soglia serve una
// riga per fetta, marcata col range, così l'unione è verificabile.
func serverSpezzato(t *testing.T, maxGiorni int, viste *[]string) *httptest.Server {
	t.Helper()
	var corrente string
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "default.jsp") {
			q := r.URL.Query().Get("icaQuery")
			corrente = q
			lo, hi, _ := strings.Cut(estraiRange(q), "/")
			a, errA := daAAMMGG(lo)
			b, errB := daAAMMGG(hi)
			if errA != nil || errB != nil || int(b.Sub(a).Hours()/24) > maxGiorni {
				fmt.Fprint(w, `<div class="message ko"> (QR997)</div>`)
				return
			}
			*viste = append(*viste, estraiRange(q))
			fmt.Fprint(w, `<h3><a>Lista Documenti</a>(1)</h3>`)
			return
		}
		fmt.Fprintf(w, `<html><body><ul id="shortListTable"><li class="intestazione">h</li>
			<li href="javascript: showDoc(1)"><div>18</div><div>%s</div><div>1.01.2024</div>
			<div><h3>Titolo</h3></div></li></ul>
			<div class="pagination"><span class="pagina_di">Pagina 1 di 1</span></div></body></html>`,
			estraiRange(corrente))
	}))
}

func estraiRange(q string) string {
	i := strings.Index(q, ".DATPRE")
	if i < 0 {
		return ""
	}
	j := strings.LastIndexAny(q[:i], "( ")
	return q[j+1 : i]
}

// Il difetto: lo stesso intervallo che contiene risultati provati tornava [].
// Ora il rifiuto viene riconosciuto e la domanda rifatta a fette.
func TestSearch_RangeRifiutatoVieneSpezzato(t *testing.T) {
	var viste []string
	srv := serverSpezzato(t, 400, &viste)
	defer srv.Close()
	c, _ := New(nil)
	c.BaseURL = srv.URL

	var troncato bool
	recs, err := c.Search(context.Background(), ddlArchive, SearchOptions{
		Params:    map[string]string{"legisl": "18", "data": "230101/240229"},
		MaxPages:  1,
		Truncated: &troncato,
	})
	if err != nil {
		t.Fatalf("Search = %v, atteso il recupero a fette", err)
	}
	if len(recs) != 2 {
		t.Fatalf("righe = %d, attese 2 (una per fetta)", len(recs))
	}
	want := []string{"240101/240229", "230101/231231"}
	if !reflect.DeepEqual(viste, want) {
		t.Errorf("fette interrogate = %v, attese %v", viste, want)
	}
}

// Una fetta annuale può cedere ancora: la soglia è sul numero di documenti, non
// sul calendario. Qui il server accetta solo finestre corte, quindi il taglio
// per anno non basta e serve il secondo livello.
func TestSearch_FettaAnnualeCheCedeVieneTagliataAncora(t *testing.T) {
	var viste []string
	srv := serverSpezzato(t, 200, &viste)
	defer srv.Close()
	c, _ := New(nil)
	c.BaseURL = srv.URL

	recs, err := c.Search(context.Background(), ddlArchive, SearchOptions{
		Params:   map[string]string{"legisl": "18", "data": "230101/240229"},
		MaxPages: 1,
	})
	if err != nil {
		t.Fatalf("Search = %v", err)
	}
	if len(recs) < 3 {
		t.Fatalf("righe = %d: la fetta 2023 doveva essere tagliata a metà", len(recs))
	}
	for _, v := range viste {
		if v == "230101/231231" {
			t.Error("la fetta annuale rifiutata è finita fra quelle riuscite")
		}
	}
}

// Quando non c'è un range da spezzare il rifiuto deve arrivare al chiamante:
// una lista vuota qui sarebbe la stessa bugia di prima, con un giro in più.
func TestSearch_RifiutoSenzaRangePropaga(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<div class="message ko"> (QR997)</div>`)
	}))
	defer srv.Close()
	c, _ := New(nil)
	c.BaseURL = srv.URL

	recs, err := c.Search(context.Background(), ddlArchive, SearchOptions{
		Params:   map[string]string{"legisl": "18", "testo": "province"},
		MaxPages: 1,
	})
	if err == nil {
		t.Fatalf("Search = %v, nil: atteso un errore, non una lista vuota", recs)
	}
	var rifiutata *QueryFailedError
	if !asQueryFailed(err, &rifiutata) {
		t.Fatalf("errore = %T (%v), atteso *QueryFailedError", err, err)
	}
	if rifiutata.Code != "QR997" || rifiutata.Archive != "ddl" {
		t.Errorf("errore = %+v: attesi codice e archivio", rifiutata)
	}
}

// Anche quando ogni fetta cede fino in fondo: meglio l'errore che un vuoto.
func TestSearch_RifiutoAOgniLivelloPropaga(t *testing.T) {
	var viste []string
	srv := serverSpezzato(t, 0, &viste)
	defer srv.Close()
	c, _ := New(nil)
	c.BaseURL = srv.URL

	if _, err := c.Search(context.Background(), ddlArchive, SearchOptions{
		Params:   map[string]string{"legisl": "18", "data": "230101/240229"},
		MaxPages: 1,
	}); err == nil {
		t.Fatal("atteso errore quando nessuna fetta passa")
	}
}

// Limit vale sul totale unito, non per fetta: altrimenti --limit 3 su tre fette
// darebbe nove righe.
func TestSearch_LimitValeSulTotaleUnito(t *testing.T) {
	var viste []string
	srv := serverSpezzato(t, 400, &viste)
	defer srv.Close()
	c, _ := New(nil)
	c.BaseURL = srv.URL

	var troncato bool
	recs, err := c.Search(context.Background(), ddlArchive, SearchOptions{
		Params:    map[string]string{"legisl": "18", "data": "221001/240301"},
		MaxPages:  1,
		Limit:     1,
		Truncated: &troncato,
	})
	if err != nil {
		t.Fatalf("Search = %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("righe = %d, atteso 1 (Limit sul totale)", len(recs))
	}
	if !troncato {
		t.Error("troncato = false: restavano fette non interrogate, va dichiarato")
	}
}

func asQueryFailed(err error, target **QueryFailedError) bool {
	e, ok := err.(*QueryFailedError)
	if ok {
		*target = e
	}
	return ok
}
