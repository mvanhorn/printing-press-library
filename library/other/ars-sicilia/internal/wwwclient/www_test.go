package wwwclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/ars-sicilia/internal/cache"
)

func newTestClient(baseURL string, cacheDir string) *Client {
	return &Client{
		BaseURL: baseURL,
		HTTP:    &http.Client{Timeout: 5 * time.Second},
		cache:   cache.New(cacheDir, time.Minute),
	}
}

// Il 404 del portale è un vero 404 e deve arrivare come NotFoundError,
// distinguibile da un errore di rete: il chiamante lo traduce in
// «gruppo non trovato» (exit 3), non in un fallimento generico.
func TestGetNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()
	c := newTestClient(srv.URL, t.TempDir())
	_, err := c.Get(context.Background(), "/gruppi-parlamentari/XVIII-non-esiste", nil)
	if err == nil {
		t.Fatal("atteso errore")
	}
	if _, ok := err.(*NotFoundError); !ok {
		t.Fatalf("atteso *NotFoundError, ottenuto %T: %v", err, err)
	}
}

// La seconda lettura della stessa pagina non deve rifare la richiesta;
// --no-cache la bypassa e la rifà.
func TestGetCacheHitEMiss(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html><body>pagina</body></html>"))
	}))
	defer srv.Close()
	c := newTestClient(srv.URL, t.TempDir())
	ctx := context.Background()

	if _, err := c.Get(ctx, "/p", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Get(ctx, "/p", nil); err != nil {
		t.Fatal(err)
	}
	if n := atomic.LoadInt32(&hits); n != 1 {
		t.Fatalf("%d richieste al server, attesa 1 (la seconda serve da cache)", n)
	}

	c.NoCache = true
	if _, err := c.Get(ctx, "/p", nil); err != nil {
		t.Fatal(err)
	}
	if n := atomic.LoadInt32(&hits); n != 2 {
		t.Fatalf("%d richieste con --no-cache, attese 2", n)
	}
}

// Chiavi distinte per parametri diversi (idLeg 18 vs 17): leggere la XVIII
// non deve restituire la copia cache della XVII. L'ordine di inserimento
// della map non conta.
func TestCacheKeyDistintaPerParametri(t *testing.T) {
	a := cacheKey("/gruppi-parlamentari", map[string]string{"idLeg": "18"})
	b := cacheKey("/gruppi-parlamentari", map[string]string{"idLeg": "17"})
	if a == b {
		t.Fatalf("chiavi uguali per idLeg diversi: %q", a)
	}
	c := cacheKey("/gruppi-parlamentari", map[string]string{"idLeg": "17"})
	if b != c {
		t.Fatal("chiave non deterministica")
	}
	d := cacheKey("/gruppi-parlamentari/XVIII-misto", nil)
	if d == a || d == b {
		t.Fatal("slug e elenco collidono in cache")
	}
}
