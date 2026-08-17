// Copyright 2026 aborruso. Licensed under Apache-2.0. See LICENSE.
//
// pp:data-source live
// Client per il sito istituzionale www.ars.sicilia.it (Drupal 10).
//
// È la seconda sorgente della CLI, dopo il motore documentale
// dati.ars.sicilia.it (icaroclient): qui stanno le anagrafiche — gruppi
// parlamentari, composizione, collegi di elezione — che il portale
// documentale non ha. Non esiste API (/jsonapi, /rest/session/token e
// /sitemap.xml rispondono 404): si legge HTML, quindi i selettori sono
// verificati contro le pagine vive e certificati da test su fixture
// (testdata/), come i Columns degli archivi Icaro. Un restyling li rompe
// in silenzio: i test devono fallire, non passare su un elenco vuoto.

package wwwclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/ars-sicilia/internal/cache"
	"github.com/mvanhorn/printing-press-library/library/other/ars-sicilia/internal/cliutil"
)

// DefaultBaseURL punta al sito istituzionale (le anagrafiche).
const DefaultBaseURL = "https://www.ars.sicilia.it"

// DefaultUserAgent è dichiarato a ogni richiesta: il sito è pubblico ma
// resta il server di qualcuno, e farsi identificare nei log è il minimo.
const DefaultUserAgent = "ars-sicilia-pp-cli (+https://github.com/aborruso/ars-trasparente)"

// DefaultCacheTTL mantiene in cache le pagine www per 5 minuti, come le
// risposte del client generato: anche qui si leggono dati vivi.
const DefaultCacheTTL = 5 * time.Minute

// NotFoundError segnala HTTP 404: per i gruppi significa «slug inesistente»,
// non «errore di rete», e il chiamante lo trasforma in un errore di tipo
// not-found (exit 3) invece che generico.
type NotFoundError struct {
	URL string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("pagina non trovata (HTTP 404): %s", e.URL)
}

// Client legge le pagine HTML di www.ars.sicilia.it: cache su disco con
// chiavi www:<path>?<query>, rate limit adattivo, User-Agent proprio.
type Client struct {
	BaseURL   string
	UserAgent string
	HTTP      *http.Client
	NoCache   bool
	cache     *cache.Store
	limiter   *cliutil.AdaptiveLimiter
}

// New costruisce il client www. timeout e rateLimit arrivano dalle flag
// di radice della CLI, come per gli altri client.
func New(timeout time.Duration, rateLimit float64) *Client {
	homeDir, _ := os.UserHomeDir()
	cacheDir := filepath.Join(homeDir, ".cache", "ars-sicilia-pp-cli", "www")
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &Client{
		BaseURL:   DefaultBaseURL,
		UserAgent: DefaultUserAgent,
		HTTP:      &http.Client{Timeout: timeout},
		cache:     cache.New(cacheDir, DefaultCacheTTL),
		limiter:   cliutil.NewAdaptiveLimiter(rateLimit),
	}
}

// cacheKey è deterministica e dipende da path e parametri (ordinati), così
// pagine diverse non collidono e la stessa pagina riconosce la propria copia.
func cacheKey(path string, params map[string]string) string {
	key := "www:" + path
	if len(params) > 0 {
		keys := make([]string, 0, len(params))
		for k := range params {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			key += "&" + k + "=" + params[k]
		}
	}
	return key
}

// Get scarica una pagina e ne restituisce il corpo grezzo. La cache serve
// in lettura e scrittura; --no-cache la bypassa ma aggiorna comunque la
// copia fresca (stessa semantica del client generato).
func (c *Client) Get(ctx context.Context, path string, params map[string]string) ([]byte, error) {
	if c == nil {
		return nil, fmt.Errorf("nil wwwclient.Client")
	}
	key := cacheKey(path, params)
	if !c.NoCache {
		if raw, ok := c.cache.Get(key); ok {
			return []byte(raw), nil
		}
	}

	u := strings.TrimRight(c.BaseURL, "/") + path
	if len(params) > 0 {
		keys := make([]string, 0, len(params))
		for k := range params {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if strings.Contains(u, "?") {
				u += "&"
			} else {
				u += "?"
			}
			u += k + "=" + params[k]
		}
	}

	if c.limiter != nil {
		c.limiter.Wait()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("costruzione richiesta %s: %w", u, err)
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("richiesta %s: %w", u, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, &NotFoundError{URL: u}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d da %s", resp.StatusCode, u)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, fmt.Errorf("lettura risposta da %s: %w", u, err)
	}
	c.cache.Set(key, json.RawMessage(body))
	return body, nil
}
