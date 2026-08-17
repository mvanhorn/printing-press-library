package icaroclient

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
)

// serverTronca risponde con un corpo tagliato a metà per le prime `guaste`
// richieste, poi con il corpo intero. È il difetto misurato sul portale ARS il
// 2026-08-12: status 200, header regolari, e il corpo che si interrompe (6 volte
// su 20 sullo stesso URL). Dichiarare un Content-Length più lungo di quanto si
// scrive e poi abortire l'handler riproduce esattamente ciò che vede il client:
// un errore in lettura del corpo, non uno status di errore.
func serverTronca(guaste int32, chiamate *int32) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(chiamate, 1) <= guaste {
			w.Header().Set("Content-Length", "4096")
			w.WriteHeader(http.StatusOK)
			io.WriteString(w, "<html>meta risposta")
			panic(http.ErrAbortHandler)
		}
		fmt.Fprint(w, "<html>risposta intera</html>")
	}))
	srv.Config.ErrorLog = nil
	return srv
}

func clientDiProva(t *testing.T, baseURL string) *Client {
	t.Helper()
	c, err := New(nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	c.BaseURL = baseURL
	c.limiter = nil // niente pacing: qui si misura il retry, non la cadenza
	return c
}

// Due troncature di fila non devono diventare un comando fallito: il terzo
// tentativo porta a casa la risposta. Prima del 2026-08-12 non si ritentava
// affatto, e con ~30% di troncature un comando su tre moriva per nulla.
func TestReadRitentaLeRisposteTroncate(t *testing.T) {
	var chiamate int32
	srv := serverTronca(2, &chiamate)
	defer srv.Close()

	body, err := clientDiProva(t, srv.URL).get(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("get: dopo due troncature il terzo tentativo doveva riuscire, invece: %v", err)
	}
	if !strings.Contains(body, "risposta intera") {
		t.Errorf("corpo = %q, atteso quello intero", body)
	}
	if chiamate != 3 {
		t.Errorf("richieste = %d, attese 3 (due troncate + una buona)", chiamate)
	}
}

// I tentativi finiscono: quando finiscono, l'errore che esce è quello del
// trasporto. Chi chiama deve poter dire «il backend non ha risposto» e non
// «il documento non esiste».
func TestReadSiArrendeEDiceIlPerche(t *testing.T) {
	var chiamate int32
	srv := serverTronca(100, &chiamate)
	defer srv.Close()

	_, err := clientDiProva(t, srv.URL).get(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("get: con il backend sempre rotto ci si aspetta un errore")
	}
	if chiamate != transientAttempts {
		t.Errorf("richieste = %d, attese %d", chiamate, transientAttempts)
	}
	var rl *HTTPRateLimitError
	if errors.As(err, &rl) {
		t.Errorf("una troncatura non è un rate limit: %v", err)
	}
}

// Una risposta completa che dice 404 è definitiva: ritentarla è solo tempo
// sprecato, e il portale la riceverebbe tre volte per nulla.
func TestReadNonRitentaGliStatusDefinitivi(t *testing.T) {
	var chiamate int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&chiamate, 1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	if _, err := clientDiProva(t, srv.URL).get(context.Background(), srv.URL); err == nil {
		t.Fatal("get: un 404 deve restare un errore")
	}
	if chiamate != 1 {
		t.Errorf("richieste = %d, attesa 1: un 404 non si ritenta", chiamate)
	}
}

// Il 429 ha una ricetta sua — rallentare e riprovare più tardi — e un codice di
// uscita suo. Ritentarlo subito peggiorerebbe la situazione che segnala.
func TestReadNonRitentaIlRateLimit(t *testing.T) {
	var chiamate int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&chiamate, 1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	_, err := clientDiProva(t, srv.URL).get(context.Background(), srv.URL)
	var rl *HTTPRateLimitError
	if !errors.As(err, &rl) {
		t.Fatalf("errore = %v, atteso HTTPRateLimitError", err)
	}
	if chiamate != 1 {
		t.Errorf("richieste = %d, attesa 1: il 429 non si ritenta subito", chiamate)
	}
}

// Anche le POST vanno ritentate: sul backend /bd/ sono ricerche, e la ricerca
// paginata è la strada da cui passa quasi tutto (sommari, resoconti, oratori).
func TestPostRitentaComeLaGet(t *testing.T) {
	var chiamate int32
	srv := serverTronca(1, &chiamate)
	defer srv.Close()

	body, err := clientDiProva(t, srv.URL).post(context.Background(), srv.URL, url.Values{"page": {"1"}})
	if err != nil {
		t.Fatalf("post: la troncatura doveva essere assorbita, invece: %v", err)
	}
	if !strings.Contains(body, "risposta intera") {
		t.Errorf("corpo = %q, atteso quello intero", body)
	}
	if chiamate != 2 {
		t.Errorf("richieste = %d, attese 2 (una troncata + una buona)", chiamate)
	}
}

// Un contesto annullato non è il portale che sbaglia: è chi chiama che non
// aspetta più. Ritentare allungherebbe soltanto l'attesa.
func TestReadNonRitentaSeIlContestoEAnnullato(t *testing.T) {
	var chiamate int32
	srv := serverTronca(100, &chiamate)
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := clientDiProva(t, srv.URL).get(ctx, srv.URL); err == nil {
		t.Fatal("get: con contesto annullato ci si aspetta un errore")
	}
	if chiamate > 1 {
		t.Errorf("richieste = %d: con il contesto annullato non si ritenta", chiamate)
	}
}

// La classifica oratori è una richiesta per oratore — 91 per la legislatura
// XVIII — e finché bastava un errore per abbandonare tutto, su un portale che
// ne tronca una ogni tanto la classifica non arrivava mai. Ora chi non risponde
// viene elencato a parte e gli altri restano.
func TestSpeakerSessionCountsTieneIRiusciti(t *testing.T) {
	const form = `<select id="$Ispeakers" name="$Ispeakers" multiple="multiple">
<option  value="1" data-legs="18">Rossi Anna</option>
<option  value="2" data-legs="18">Bianchi Mario</option>
<option  value="3" data-legs="18">Verdi Luisa</option>
</select>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			fmt.Fprint(w, form)
			return
		}
		r.ParseForm()
		// L'oratore 2 non viene mai misurato: il portale tronca ogni volta.
		if r.PostForm.Get("$Ispeakers") == "2" {
			w.Header().Set("Content-Length", "4096")
			w.WriteHeader(http.StatusOK)
			io.WriteString(w, "<html>meta")
			panic(http.ErrAbortHandler)
		}
		fmt.Fprint(w, "<html>Trovati 7 risultati</html>")
	}))
	srv.Config.ErrorLog = nil
	defer srv.Close()

	counts, persi, err := clientDiProva(t, srv.URL).SpeakerSessionCounts(context.Background(), "18", "", nil)
	if err != nil {
		t.Fatalf("SpeakerSessionCounts: due oratori su tre erano misurabili, invece: %v", err)
	}
	if len(counts) != 2 {
		t.Errorf("oratori misurati = %d, attesi 2: %+v", len(counts), counts)
	}
	if len(persi) != 1 || persi[0] != "Bianchi Mario" {
		t.Errorf("oratori persi = %v, atteso [Bianchi Mario]", persi)
	}
	for _, sc := range counts {
		if sc.Count != 7 {
			t.Errorf("conteggio di %s = %d, atteso 7", sc.Name, sc.Count)
		}
	}
}

// Il 429 non è una richiesta persa fra le altre: è il portale che chiede
// tregua. Archiviarlo fra i "non misurati" e proseguire significava sparargli
// contro le altre novanta richieste, e perdere l'errore su cui il chiamante
// costruisce il codice di uscita 7.
func TestSpeakerSessionCountsSiFermaSul429(t *testing.T) {
	const form = `<select id="$Ispeakers" name="$Ispeakers" multiple="multiple">
<option  value="1" data-legs="18">Rossi Anna</option>
<option  value="2" data-legs="18">Bianchi Mario</option>
<option  value="3" data-legs="18">Verdi Luisa</option>
</select>`
	var post int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			fmt.Fprint(w, form)
			return
		}
		atomic.AddInt32(&post, 1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	_, _, err := clientDiProva(t, srv.URL).SpeakerSessionCounts(context.Background(), "18", "", nil)
	var rl *HTTPRateLimitError
	if !errors.As(err, &rl) {
		t.Fatalf("errore = %v, atteso HTTPRateLimitError: senza, il chiamante non può dare exit 7", err)
	}
	if post != 1 {
		t.Errorf("richieste dopo il 429 = %d, attesa 1: chi ha detto basta non va incalzato", post)
	}
}

// Zero misurati non è una classifica parziale: è un comando fallito. Se uscisse
// come lista vuota si leggerebbe «nessuno è mai intervenuto», che è falso.
func TestSpeakerSessionCountsFallisceSeNessunoRisponde(t *testing.T) {
	const form = `<select id="$Ispeakers" name="$Ispeakers" multiple="multiple">
<option  value="1" data-legs="18">Rossi Anna</option>
</select>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			fmt.Fprint(w, form)
			return
		}
		w.Header().Set("Content-Length", "4096")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "<html>meta")
		panic(http.ErrAbortHandler)
	}))
	srv.Config.ErrorLog = nil
	defer srv.Close()

	counts, persi, err := clientDiProva(t, srv.URL).SpeakerSessionCounts(context.Background(), "18", "", nil)
	if err == nil {
		t.Fatalf("attesa una classifica fallita, invece counts=%v persi=%v", counts, persi)
	}
	if counts != nil {
		t.Errorf("counts = %v, atteso nil: una classifica vuota si legge come «nessuno è intervenuto»", counts)
	}
}
