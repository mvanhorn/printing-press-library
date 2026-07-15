package gaclient

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/giustizia-amministrativa/internal/cliutil"
)

const (
	// BaseURL is the portal host serving the search form/action.
	BaseURL = "https://www.giustizia-amministrativa.it"
	// formPath is the "Decisioni e Pareri" search page (the handshake target).
	formPath = "/web/guest/dcsnprr"
	// portletPrefix is the Liferay namespace of the search portlet instance.
	portletPrefix = "_decisioni_pareri_web_DecisioniPareriWebPortlet_INSTANCE_XKc17mrB8J10"
	// portletID is the matching p_p_id value.
	portletID = "decisioni_pareri_web_DecisioniPareriWebPortlet_INSTANCE_XKc17mrB8J10"

	defaultUA       = "giustizia-amministrativa-pp-cli/0.1.0 (+https://github.com/aborruso)"
	defaultPageSize = 10
	// politeRate keeps requests gentle against a public institutional site.
	politeRate = 2.0
)

var rePAuth = regexp.MustCompile(`p_auth=([A-Za-z0-9]+)`)

// Client talks to the giustizia-amministrativa public search over plain HTTP,
// managing the Liferay session handshake (p_auth + affinity cookies).
type Client struct {
	http    *http.Client
	limiter *cliutil.AdaptiveLimiter
	ua      string

	mu    sync.Mutex
	pAuth string
}

// New returns a ready Client with a cookie jar and a polite adaptive limiter.
func New() *Client {
	jar, _ := cookiejar.New(nil)
	return &Client{
		http:    &http.Client{Jar: jar, Timeout: 30 * time.Second},
		limiter: cliutil.NewAdaptiveLimiter(politeRate),
		ua:      defaultUA,
	}
}

// SearchOptions describes a provvedimenti query.
type SearchOptions struct {
	Testo   string // simple full-text
	All     string // advanced: all of these words
	Any     string // advanced: any of these words
	Not     string // advanced: none of these words
	Phrase  string // advanced: exact phrase
	Tipo    string // sentenza|ordinanza|decreto|parere|plenaria|generale
	Sede    string // roma|milano|consiglio-di-stato|...
	Anno    int
	Numero  int
	Nrg     int
	AnnoNrg int
	Limit   int // max results to return
}

func (c *Client) get(ctx context.Context, rawURL string) ([]byte, int, error) {
	// Retry on transient throttling (429): the public institutional site rate-
	// limits bursts; back off and retry a few times before surfacing the error.
	const maxAttempts = 4
	var body []byte
	var status int
	var err error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if c.limiter != nil {
			c.limiter.Wait()
		}
		body, status, err = c.doGet(ctx, rawURL)
		if err != nil {
			return nil, 0, err
		}
		if status == http.StatusTooManyRequests {
			if c.limiter != nil {
				c.limiter.OnRateLimit()
			}
			if attempt == maxAttempts {
				return body, status, &cliutil.RateLimitError{URL: rawURL, Body: "giustizia-amministrativa"}
			}
			if waitErr := sleepCtx(ctx, time.Duration(attempt)*2*time.Second); waitErr != nil {
				return nil, 0, waitErr
			}
			continue
		}
		if c.limiter != nil {
			c.limiter.OnSuccess()
		}
		return body, status, nil
	}
	return body, status, err
}

func (c *Client) doGet(ctx context.Context, rawURL string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", c.ua)
	req.Header.Set("Accept-Language", "it-IT,it;q=0.9")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return body, resp.StatusCode, nil
}

// sleepCtx waits for d or until ctx is cancelled.
func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// handshake fetches the form page to obtain the p_auth token and affinity
// cookies (stored in the jar). Safe to call repeatedly; refreshes the token.
func (c *Client) handshake(ctx context.Context) error {
	body, status, err := c.get(ctx, BaseURL+formPath)
	if err != nil {
		return fmt.Errorf("handshake: %w", err)
	}
	if status != http.StatusOK {
		return fmt.Errorf("handshake: HTTP %d", status)
	}
	m := rePAuth.FindSubmatch(body)
	if m == nil {
		return fmt.Errorf("handshake: token p_auth non trovato nella pagina del form")
	}
	c.mu.Lock()
	c.pAuth = string(m[1])
	c.mu.Unlock()
	return nil
}

func (c *Client) token() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.pAuth
}

// buildSearchURL constructs a paginated search action URL for page cur (1-based).
func (c *Client) buildSearchURL(opts SearchOptions, cur int) string {
	v := url.Values{}
	v.Set("p_p_id", portletID)
	v.Set("p_p_lifecycle", "1")
	v.Set("p_p_state", "normal")
	v.Set("p_p_mode", "view")
	v.Set(portletPrefix+"_javax.portlet.action", "search")
	v.Set("p_auth", c.token())

	p := func(name, val string) { v.Set(portletPrefix+"_"+name, val) }

	advanced := opts.All != "" || opts.Any != "" || opts.Not != "" || opts.Phrase != ""
	if advanced {
		p("isAdvancedSearch", "true")
		p("searchAllWords", opts.All)
		p("searchAnyWords", opts.Any)
		p("searchNotWords", opts.Not)
		p("searchPhrase", opts.Phrase)
	} else {
		p("isAdvancedSearch", "false")
		p("searchtextProvvedimenti", opts.Testo)
	}
	if t := mapTipo(opts.Tipo); t != "" {
		p("TipoProvvedimentoItem", t)
	}
	if s := mapSede(opts.Sede); s != "" {
		p("sedeProvvedimenti", s)
	}
	if opts.Anno != 0 {
		p("DataYearItem", strconv.Itoa(opts.Anno))
	}
	if opts.Numero != 0 {
		p("numeroProvvedimenti", strconv.Itoa(opts.Numero))
	}
	if opts.Nrg != 0 {
		p("numeroNrg", strconv.Itoa(opts.Nrg))
		p("asSearchMode", "nrg")
	} else {
		p("asSearchMode", "provv")
	}
	if opts.AnnoNrg != 0 {
		p("DataNrgItem", strconv.Itoa(opts.AnnoNrg))
	}
	p("pageSize", strconv.Itoa(defaultPageSize))
	p("changePage", "true")
	p("cur", strconv.Itoa(cur))
	return BaseURL + formPath + "?" + v.Encode()
}

// SearchResult bundles the rows of a search with the reported total.
type SearchResult struct {
	Items []Provvedimento
	Total int
}

// Search runs a query, paginating until Limit results are collected. It performs
// the session handshake on first use and retries once on a 403 (expired token).
func (c *Client) Search(ctx context.Context, opts SearchOptions) (*SearchResult, error) {
	if opts.Limit <= 0 {
		opts.Limit = defaultPageSize
	}
	if c.token() == "" {
		if err := c.handshake(ctx); err != nil {
			return nil, err
		}
	}
	res := &SearchResult{}
	maxPages := (opts.Limit + defaultPageSize - 1) / defaultPageSize
	for page := 1; page <= maxPages; page++ {
		body, status, err := c.get(ctx, c.buildSearchURL(opts, page))
		if err != nil {
			if rle, ok := err.(*cliutil.RateLimitError); ok {
				return nil, rle
			}
			return nil, err
		}
		if status == http.StatusForbidden {
			// Expired/stale token: refresh the handshake and retry this page once.
			if herr := c.handshake(ctx); herr != nil {
				return nil, herr
			}
			body, status, err = c.get(ctx, c.buildSearchURL(opts, page))
			if err != nil {
				return nil, err
			}
		}
		if status != http.StatusOK {
			return nil, fmt.Errorf("ricerca: HTTP %d", status)
		}
		text := string(body)
		if page == 1 {
			res.Total = ParseTotal(text)
		}
		items := ParseResults(text)
		if len(items) == 0 {
			break
		}
		res.Items = append(res.Items, items...)
		if len(res.Items) >= opts.Limit {
			res.Items = res.Items[:opts.Limit]
			break
		}
	}
	return res, nil
}

// FullText fetches the raw HTML of a single provvedimento document. It uses
// p.URL when present, otherwise rebuilds it from schema/nrg/nome_file.
func (c *Client) FullText(ctx context.Context, p Provvedimento) (string, error) {
	docURL := p.URL
	if docURL == "" {
		if p.Schema == "" || p.Nrg == "" || p.NomeFile == "" {
			return "", fmt.Errorf("dati insufficienti per costruire l'URL del documento (servono schema, nrg, nome_file)")
		}
		docURL = DocURL(p.Schema, p.Nrg, p.NomeFile)
	}
	body, status, err := c.get(ctx, docURL)
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("testo integrale: HTTP %d", status)
	}
	return string(body), nil
}

// DocURL builds the public full-text URL for a provvedimento.
func DocURL(schema, nrg, nomeFile string) string {
	v := url.Values{}
	v.Set("nodeRef", "")
	v.Set("schema", schema)
	v.Set("nrg", nrg)
	v.Set("nomeFile", nomeFile)
	v.Set("subDir", "Provvedimenti")
	return "https://mdp.giustizia-amministrativa.it/visualizzah2/?" + v.Encode()
}

// mapTipo translates a CLI-friendly tipo into the portal option value.
func mapTipo(t string) string {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "", "tutti", "all":
		return ""
	case "sentenza", "sentenze":
		return "Sentenza"
	case "ordinanza", "ordinanze":
		return "Ordinanza"
	case "decreto", "decreti":
		return "Decreto"
	case "parere", "pareri":
		return "Parere"
	case "plenaria", "adunanza-plenaria":
		return "P"
	case "generale", "adunanza-generale":
		return "C"
	default:
		return ""
	}
}

// sedeMap maps CLI-friendly sede slugs to portal option values.
var sedeMap = map[string]string{
	"consiglio-di-stato": "Consiglio di Stato",
	"cds":                "Consiglio di Stato",
	"cgars":              "C.G.A.R.S",
	"ancona":             "Ancona", "aosta": "Aosta", "bari": "Bari", "bologna": "Bologna",
	"bolzano": "Bolzano", "brescia": "Brescia", "cagliari": "Cagliari", "campobasso": "Campobasso",
	"catania": "Catania", "catanzaro": "Catanzaro", "firenze": "Firenze", "genova": "Genova",
	"laquila": "L'Aquila", "l-aquila": "L'Aquila", "latina": "Latina", "lecce": "Lecce",
	"milano": "Milano", "napoli": "Napoli", "palermo": "Palermo", "parma": "Parma",
	"perugia": "Perugia", "pescara": "Pescara", "potenza": "Potenza",
	"reggio-calabria": "Reggio Calabria", "roma": "Roma", "salerno": "Salerno",
	"torino": "Torino", "trento": "Trento", "trieste": "Trieste", "venezia": "Venezia",
}

func mapSede(s string) string {
	key := strings.ToLower(strings.TrimSpace(s))
	if key == "" {
		return ""
	}
	if v, ok := sedeMap[key]; ok {
		return v
	}
	// Accept an already-correct portal value as-is.
	return strings.TrimSpace(s)
}
