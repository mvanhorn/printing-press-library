package gaclient

import (
	"bytes"
	"context"
	"errors"
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

	"github.com/ledongthuc/pdf"

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

	defaultUA       = "github.com/mvanhorn/printing-press-library/library/productivity/giustizia-amministrativa/0.1.0 (+https://github.com/aborruso)"
	defaultPageSize = 10
	// politeRate keeps requests gentle against a public institutional site.
	politeRate = 2.0
)

var (
	rePAuth = regexp.MustCompile(`p_auth=([A-Za-z0-9]+)`)
	// The search form declares its own action, carrying p_p_id, p_p_lifecycle
	// and p_auth together, and its id carries the portlet namespace.
	reFormAction = regexp.MustCompile(`action="(https://[^"]*javax\.portlet\.action=search[^"]*)"`)
	rePortletID  = regexp.MustCompile(`p_p_id=(decisioni_pareri_web[A-Za-z0-9_]*)`)
)

// Client talks to the giustizia-amministrativa public search over plain HTTP,
// managing the Liferay session handshake (p_auth + affinity cookies).
type Client struct {
	http    *http.Client
	limiter *cliutil.AdaptiveLimiter
	ua      string

	mu sync.Mutex
	// pAuth is the CSRF token, action the search form's own action URL and
	// portlet the portlet id read from the form page. See handshake.
	pAuth   string
	action  string
	portlet string
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
	Testo     string // simple full-text
	All       string // advanced: all of these words
	Any       string // advanced: any of these words
	Not       string // advanced: none of these words
	Phrase    string // advanced: exact phrase
	Tipo      string // sentenza|ordinanza|decreto|parere|plenaria|generale
	Sede      string // roma|milano|consiglio-di-stato|...
	SedeSweep bool   // iterate every sede instead of accepting the portal's Roma-first order
	// SedeQuota decides how a sede sweep spends Limit across the sedi.
	// "" or SedeQuotaProporzionale weights each sede by the total the portal
	// declares for it, so the sample mirrors where the case law actually is.
	// SedeQuotaUguale gives every sede the same share, which answers the
	// different question "does any sede have anything on this at all".
	SedeQuota string
	Anno      int
	AnnoFrom  int // year-range sweep: first year (inclusive)
	AnnoTo    int // year-range sweep: last year (inclusive)
	Numero    int
	Nrg       int
	AnnoNrg   int
	Limit     int // max results to return (per year when sweeping a year range)
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
	action, portlet := parseForm(body)
	c.mu.Lock()
	c.pAuth = string(m[1])
	c.action = action
	c.portlet = portlet
	c.mu.Unlock()
	return nil
}

// parseForm reads from the search page the two things that identify the
// portlet instance: the form's action URL and the portlet id. Both are
// hardcoded as constants for the fallback, but the id embeds an
// _INSTANCE_<hash> that the portal can change on a redeploy, and the action
// carries p_p_id, p_p_lifecycle and p_auth at once — so a rename of any of
// them survives as long as we replay what the page itself declares.
func parseForm(body []byte) (action, portlet string) {
	if m := reFormAction.FindSubmatch(body); m != nil {
		// The portal writes the separators bare, but an HTML attribute is
		// allowed to escape them: left as "&amp;" the query would parse into
		// keys named "amp;p_p_lifecycle", and the search would fail after a
		// handshake that looked successful.
		action = strings.ReplaceAll(string(m[1]), "&amp;", "&")
	}
	if m := rePortletID.FindSubmatch(body); m != nil {
		portlet = string(m[1])
	}
	return action, portlet
}

// portletNS returns the namespace prefixing every form field of the portlet
// instance ("_" + portlet id), from the page when the handshake read it.
func (c *Client) portletNS() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.portlet != "" {
		return "_" + c.portlet
	}
	return portletPrefix
}

// searchAction returns the form action URL read from the page, or "" when the
// handshake could not find it and the caller has to build one.
func (c *Client) searchAction() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.action
}

func (c *Client) token() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.pAuth
}

// buildSearchURL constructs a paginated search action URL for page cur (1-based).
// The base is the form's own action when the handshake could read it, so the
// portlet id, the lifecycle and the token are the page's own; the constants
// serve only as fallback.
func (c *Client) buildSearchURL(opts SearchOptions, cur int) string {
	base := BaseURL + formPath
	ns := c.portletNS()
	v := url.Values{}
	if action := c.searchAction(); action != "" {
		if u, err := url.Parse(action); err == nil {
			v = u.Query()
			u.RawQuery = ""
			base = u.String()
		}
	}
	if len(v) == 0 {
		v.Set("p_p_id", portletID)
		v.Set("p_p_lifecycle", "1")
		v.Set("p_p_state", "normal")
		v.Set("p_p_mode", "view")
		v.Set(ns+"_javax.portlet.action", "search")
	}
	v.Set("p_auth", c.token())

	p := func(name, val string) { v.Set(ns+"_"+name, val) }

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
	return base + "?" + v.Encode()
}

// SearchResult bundles the rows of a search with the reported total. Warnings
// carries non-fatal notices (a year skipped during a sweep) for the caller to
// surface on stderr; results are still usable.
type SearchResult struct {
	Items    []Provvedimento
	Total    int
	Warnings []string
	// TotalsBySede holds, after a sede sweep, the count the portal declares
	// for each sede. The swept sample cannot answer "how much of this case law
	// belongs to each sede": it carries an equal quota per sede by
	// construction, so counting the rows measures the quota, not the country.
	// These are the real figures, and the sweep already receives them.
	TotalsBySede map[string]int
}

// Search runs a query and returns up to Limit results. It performs the session
// handshake on first use. When a year range (AnnoFrom/AnnoTo) is set it sweeps
// the years one by one — the portal has no relevance sort and only a single-year
// filter, so historical coverage requires iterating the year filter — applying
// Limit per year and de-duplicating the union by id.
func (c *Client) Search(ctx context.Context, opts SearchOptions) (*SearchResult, error) {
	if opts.Limit <= 0 {
		opts.Limit = defaultPageSize
	}
	if opts.Anno != 0 && (opts.AnnoFrom != 0 || opts.AnnoTo != 0) {
		return nil, fmt.Errorf("usa --anno oppure --anno-from/--anno-to, non entrambi")
	}
	if opts.SedeSweep && opts.Sede != "" {
		return nil, fmt.Errorf("usa --sede oppure --sede-sweep, non entrambi")
	}
	// Reject unknown filter values instead of quietly ignoring them. An
	// unrecognised sede reaches the portal verbatim and matches nothing, so
	// the caller reads "no results" and concludes there is no case law on the
	// subject; an unrecognised tipo is dropped altogether, so they get
	// provvedimenti of every kind while believing the filter applied. Both
	// answers look valid and neither is.
	if !validSede(opts.Sede) {
		return nil, fmt.Errorf("sede %q non riconosciuta: usa %s", opts.Sede, sediSuggestion())
	}
	if !validTipo(opts.Tipo) {
		return nil, fmt.Errorf("tipo %q non riconosciuto: usa sentenza, ordinanza, decreto, parere, plenaria o generale", opts.Tipo)
	}
	if opts.SedeSweep && (opts.AnnoFrom != 0 || opts.AnnoTo != 0) {
		return nil, fmt.Errorf("--sede-sweep e --anno-from/--anno-to non si combinano: sarebbero %d sedi per ogni anno. Restringi con --anno, oppure fai uno sweep per volta", len(sediSweepList))
	}
	// La quota si valida sempre, anche senza sweep: un valore scritto male non
	// deve passare inosservato solo perche' il flag che lo usa e' assente.
	if _, err := normalizeSedeQuota(opts.SedeQuota); err != nil {
		return nil, err
	}
	if opts.SedeQuota != "" && !opts.SedeSweep {
		return nil, fmt.Errorf("--sede-quota vale solo con --sede-sweep: senza sweep si interroga una sede sola e non c'e' nulla da ripartire")
	}
	if c.token() == "" {
		if err := c.handshake(ctx); err != nil {
			return nil, err
		}
	}
	var res *SearchResult
	var err error
	if opts.SedeSweep {
		res, err = c.searchSedeSweep(ctx, opts)
	} else if opts.AnnoFrom != 0 || opts.AnnoTo != 0 {
		res, err = c.searchSweep(ctx, opts)
	} else {
		res, err = c.searchOnce(ctx, opts)
		if err == nil {
			res = withSedeCoverageWarning(res, opts)
		}
	}
	if err != nil {
		return nil, err
	}
	if nota := sedeAliasWarning(opts.Sede); nota != "" && res != nil {
		res.Warnings = append(res.Warnings, nota)
	}
	return applySnippetDedup(res), nil
}

// sediSweepList is every sede the portal's own form offers, in form order.
// It is taken from the <select> options rather than from sedeMap, whose
// aliases (cds/consiglio-di-stato, laquila/l-aquila) would make a sweep query
// the same sede twice.
var sediSweepList = []string{
	"Consiglio di Stato", "C.G.A.R.S", "Ancona", "Aosta", "Bari", "Bologna",
	"Bolzano", "Brescia", "Cagliari", "Campobasso", "Catania", "Catanzaro",
	"Firenze", "Genova", "L'Aquila", "Latina", "Lecce", "Milano", "Napoli",
	"Palermo", "Parma", "Perugia", "Pescara", "Potenza", "Reggio Calabria",
	"Roma", "Salerno", "Torino", "Trento", "Trieste", "Venezia",
}

// withSedeCoverageWarning flags the portal's sede-ordered result set. With no
// sede filter the search really does span the whole country — the declared
// Total proves it — but the rows come back grouped by sede, Roma first, so any
// ordinary --limit returns TAR Lazio alone while reporting a national total.
// That reads as "the top N nationally" and is not: the gap has to be stated,
// not inferred by the reader.
func withSedeCoverageWarning(res *SearchResult, opts SearchOptions) *SearchResult {
	if res == nil || opts.Sede != "" || res.Total <= len(res.Items) || len(res.Items) == 0 {
		return res
	}
	sedi := map[string]bool{}
	for _, p := range res.Items {
		sedi[p.Sede] = true
	}
	if len(sedi) != 1 {
		return res
	}
	var only string
	for s := range sedi {
		only = s
	}
	res.Warnings = append(res.Warnings, fmt.Sprintf(
		"tutti i %d risultati mostrati sono della sede %s, su %d totali dichiarati dal portale: il portale ordina per sede, non per pertinenza o data, quindi questi non sono i piu' rilevanti a livello nazionale. Usa --sede per una sede precisa, o --sede-sweep per interrogare tutte le %d sedi",
		len(res.Items), only, res.Total, len(sediSweepList)))
	return res
}

// searchSedeSweep queries every sede in turn and merges the results, so a
// national question gets a national answer instead of the portal's Roma-first
// prefix. Limit is the total across all sedi (not per sede as in the year
// sweep): a per-sede limit would turn --limit 10 into 310 rows and land the
// MCP tool straight in its truncation envelope.
//
// The skip/abort policy matches the year sweep: a transient failure on one
// sede is reported and skipped, a rate limit or cancelled context stops the
// sweep but keeps what was collected.
func (c *Client) searchSedeSweep(ctx context.Context, opts SearchOptions) (*SearchResult, error) {
	quota, err := normalizeSedeQuota(opts.SedeQuota)
	if err != nil {
		return nil, err
	}

	// Prima passata: una pagina per sede. In quota uguale la fetta e' nota in
	// partenza; in proporzionale non lo e' ancora - i totali si scoprono
	// interrogando - quindi si prende una pagina piena e si decide dopo.
	// Costa quanto prima: searchOnce scaricava comunque una pagina intera e ne
	// buttava il resto.
	perSede := (opts.Limit + len(sediSweepList) - 1) / len(sediSweepList)
	if perSede < 1 {
		perSede = 1
	}
	primaPassata := perSede
	if quota == SedeQuotaProporzionale {
		primaPassata = defaultPageSize
	}
	sedeOpts := opts
	sedeOpts.SedeSweep = false
	sedeOpts.SedeQuota = ""
	sedeOpts.Limit = primaPassata

	// Items non-nil: a zero-result search must marshal as [] for machine
	// callers, never null.
	res := &SearchResult{Items: make([]Provvedimento, 0)}
	raccolto := map[string][]Provvedimento{}
	var conRisultati []string // sedi che hanno prodotto righe, nell'ordine del portale
	var lists [][]Provvedimento
	var skipped []string
	var lastErr error
	for _, sede := range sediSweepList {
		sedeOpts.Sede = sede
		part, err := c.searchOnce(ctx, sedeOpts)
		if err != nil {
			if fatalSweepError(ctx, err) {
				if len(lists) == 0 {
					return nil, err
				}
				res.Warnings = append(res.Warnings, fmt.Sprintf("sweep sedi interrotto su %s: %v; risultati parziali dalle sedi gia' interrogate", sede, err))
				break
			}
			lastErr = err
			skipped = append(skipped, sede)
			continue
		}
		res.Total += part.Total
		if res.TotalsBySede == nil {
			res.TotalsBySede = map[string]int{}
		}
		res.TotalsBySede[sede] = part.Total
		if len(part.Items) > 0 {
			raccolto[sede] = part.Items
			conRisultati = append(conRisultati, sede)
			lists = append(lists, part.Items)
		}
	}

	// Seconda passata, solo in proporzionale e solo dove serve: le sedi la cui
	// quota supera la pagina gia' scaricata. Su --limit 100 sono le tre grandi.
	var quote map[string]int
	if quota == SedeQuotaProporzionale && len(conRisultati) > 0 {
		quote = allocaProporzionale(res.TotalsBySede, conRisultati, opts.Limit)
		for _, sede := range conRisultati {
			n := quote[sede]
			if n <= len(raccolto[sede]) {
				continue
			}
			sedeOpts.Sede = sede
			sedeOpts.Limit = n
			part, err := c.searchOnce(ctx, sedeOpts)
			if err != nil {
				if fatalSweepError(ctx, err) {
					res.Warnings = append(res.Warnings, fmt.Sprintf("approfondimento su %s interrotto: %v; per quella sede restano le righe della prima passata", sede, err))
					break
				}
				// Non fatale: si tiene quanto gia' raccolto per quella sede.
				lastErr = err
				continue
			}
			if len(part.Items) > len(raccolto[sede]) {
				raccolto[sede] = part.Items
			}
		}
		// Le liste per il merge seguono l'ordine del portale, non quello della mappa.
		lists = lists[:0]
		for _, sede := range conRisultati {
			lists = append(lists, raccolto[sede])
		}
	}

	if len(lists) == 0 && lastErr != nil {
		return nil, lastErr
	}
	// Round-robin fra le sedi, cosi' il taglio a Limit campiona il paese invece
	// di esaurire la sede che capita per prima. In proporzionale il giro e'
	// pesato: ogni sede smette quando ha dato la sua quota, quindi le prime
	// righe restano varie ma il totale rispetta la distribuzione reale.
	presi := map[string]int{}
	seen := map[string]bool{}
	for i := 0; len(res.Items) < opts.Limit; i++ {
		advanced := false
		for idx, list := range lists {
			if i >= len(list) {
				continue
			}
			advanced = true
			if quote != nil {
				sede := conRisultati[idx]
				if presi[sede] >= quote[sede] {
					continue
				}
			}
			key := dedupKey(list[i])
			if seen[key] {
				continue
			}
			seen[key] = true
			res.Items = append(res.Items, list[i])
			if quote != nil {
				presi[conRisultati[idx]]++
			}
			if len(res.Items) >= opts.Limit {
				break
			}
		}
		if !advanced {
			break
		}
	}
	// With Limit below the number of sedi the round-robin never reaches the
	// later ones, so a national sweep can silently exclude Roma or Milano
	// purely because of their position in the form. Say which sedi actually
	// contributed instead of leaving the reader to notice the absence.
	if len(res.Items) > 0 && opts.Limit < len(lists) {
		represented := map[string]bool{}
		for _, p := range res.Items {
			represented[p.Sede] = true
		}
		if quota == SedeQuotaProporzionale {
			// Qui le sedi mancanti non sono una lacuna: sono quelle la cui fetta
			// del totale nazionale non arriva a un posto. Dirlo, invece di
			// suggerire una "copertura equilibrata" che questa modalita' non
			// insegue.
			res.Warnings = append(res.Warnings, fmt.Sprintf(
				"--limit %d si ripartisce fra %d sedi in proporzione ai totali del portale: %d sedi entrano nel campione, le altre hanno una quota inferiore a un provvedimento. Alza --limit per farle emergere, oppure usa --sede-quota uguale per una riga da ogni sede",
				opts.Limit, len(lists), len(represented)))
		} else {
			res.Warnings = append(res.Warnings, fmt.Sprintf(
				"--limit %d e' piu' basso del numero di sedi interrogate (%d): sono rappresentate solo %d sedi, le altre restano fuori. Alza --limit per una copertura nazionale piu' equilibrata",
				opts.Limit, len(lists), len(represented)))
		}
	}
	res.Warnings = appendSkippedWarning(res.Warnings, skipped, "sedi", lastErr)
	return res, nil
}

// yearRange normalizes an inclusive year span: a missing bound mirrors the
// other, and a reversed span is swapped so from <= to.
func yearRange(from, to int) (int, int) {
	if from == 0 {
		from = to
	}
	if to == 0 {
		to = from
	}
	if from > to {
		from, to = to, from
	}
	return from, to
}

// dedupKey identifies a provvedimento for de-duplication: ECLI, else idprovv,
// else the document coordinates (schema|nrg|nome_file). It never returns "" for
// a real result, so items lacking an ECLI/idprovv still de-duplicate instead of
// being appended once per swept year.
func dedupKey(p Provvedimento) string {
	if p.Ecli != "" {
		return p.Ecli
	}
	if p.Idprovv != "" {
		return p.Idprovv
	}
	return p.Schema + "|" + p.Nrg + "|" + p.NomeFile
}

// fatalSweepError reports whether an error must abort a whole sweep instead of
// skipping the failing year: rate limiting (further years would only hit more
// of it) and a cancelled/expired context.
func fatalSweepError(ctx context.Context, err error) bool {
	var rle *cliutil.RateLimitError
	if errors.As(err, &rle) {
		return true
	}
	return ctx.Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

// searchSweep iterates the year filter from AnnoFrom to AnnoTo (inclusive),
// running searchOnce per year and de-duplicating the union by dedupKey. Limit
// applies per year. Total is the sum of per-year totals.
//
// A transient failure on one year (timeout, network) does not discard the years
// already collected: that year is skipped and reported in Warnings. Rate limits
// and a cancelled context abort the sweep, and an error is returned only when no
// year succeeded at all.
func (c *Client) searchSweep(ctx context.Context, opts SearchOptions) (*SearchResult, error) {
	from, to := yearRange(opts.AnnoFrom, opts.AnnoTo)
	yearOpts := opts
	yearOpts.AnnoFrom, yearOpts.AnnoTo = 0, 0
	res, err := sweepYears(ctx, from, to, func(y int) (*SearchResult, error) {
		yearOpts.Anno = y
		return c.searchOnce(ctx, yearOpts)
	})
	// Limit applies per year here, so the union routinely exceeds it. Saying so
	// beats letting the caller find that a limit of 10 produced 30 rows and
	// wonder which of the two numbers to believe.
	if err == nil && res != nil && len(res.Items) > opts.Limit {
		res.Warnings = append(res.Warnings, fmt.Sprintf(
			"nello sweep per anni --limit %d vale per ciascun anno, non sul totale: sono stati restituiti %d provvedimenti su %d anni. Per un tetto complessivo interroga un anno per volta",
			opts.Limit, len(res.Items), to-from+1))
	}
	return res, err
}

// appendSkippedWarning records the years dropped by a transient failure, so a
// gap in the swept range is never silent — including when the sweep later
// aborts for a different reason.
func appendSkippedWarning(warnings, skipped []string, noun string, lastErr error) []string {
	if len(skipped) == 0 {
		return warnings
	}
	return append(warnings, fmt.Sprintf("%s non recuperati: %s (ultimo errore: %v)", noun, strings.Join(skipped, ", "), lastErr))
}

// sweepYears merges the per-year results of fetch over an inclusive year span,
// applying the skip/abort policy described on searchSweep.
func sweepYears(ctx context.Context, from, to int, fetch func(year int) (*SearchResult, error)) (*SearchResult, error) {
	// Items non-nil: a zero-result search must marshal as [] for machine
	// callers, never null.
	res := &SearchResult{Items: make([]Provvedimento, 0)}
	seen := map[string]bool{}
	var skipped []string
	var lastErr error
	for y := from; y <= to; y++ {
		part, err := fetch(y)
		if err != nil {
			if fatalSweepError(ctx, err) {
				if len(res.Items) == 0 {
					return nil, err
				}
				res.Warnings = append(res.Warnings, fmt.Sprintf("sweep interrotto all'anno %d: %v; risultati parziali dagli anni %d-%d", y, err, from, y-1))
				res.Warnings = appendSkippedWarning(res.Warnings, skipped, "anni", lastErr)
				return res, nil
			}
			lastErr = err
			skipped = append(skipped, strconv.Itoa(y))
			continue
		}
		res.Total += part.Total
		for _, p := range part.Items {
			key := dedupKey(p)
			if seen[key] {
				continue
			}
			seen[key] = true
			res.Items = append(res.Items, p)
		}
	}
	if len(skipped) > 0 && len(res.Items) == 0 {
		return nil, lastErr
	}
	res.Warnings = appendSkippedWarning(res.Warnings, skipped, "anni", lastErr)
	return res, nil
}

// searchOnce paginates a single query until Limit results are collected,
// retrying once on a 403 (expired token).
func (c *Client) searchOnce(ctx context.Context, opts SearchOptions) (*SearchResult, error) {
	// Items non-nil: a zero-result search must marshal as [] for machine
	// callers, never null.
	res := &SearchResult{Items: make([]Provvedimento, 0)}
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
	doc, err := c.Document(ctx, p)
	if err != nil {
		return "", err
	}
	return doc.Raw, nil
}

// Document is a fetched provvedimento. Raw is the document as it can be read:
// the served HTML, or the text layer extracted from a PDF. IsPDF says which,
// because the two need different rendering downstream — running the HTML
// converter over already-plain text would mangle it.
type Document struct {
	Raw   string
	IsPDF bool
}

// Document fetches a provvedimento and returns something readable regardless
// of how the portal publishes it. Roughly one ruling in eight is served as a
// PDF; those carry a real text layer, so they are extracted rather than
// declared unavailable.
func (c *Client) Document(ctx context.Context, p Provvedimento) (Document, error) {
	docURL := p.URL
	if docURL == "" {
		if p.Schema == "" || p.Nrg == "" || p.NomeFile == "" {
			return Document{}, fmt.Errorf("dati insufficienti per costruire l'URL del documento (servono schema, nrg, nome_file)")
		}
		docURL = DocURL(p.Schema, p.Nrg, p.NomeFile)
	}
	// A row stored before the endpoint fix can still carry the visualizzah2
	// URL for a PDF, which answers with an error string instead of the file.
	if isPDFPath(p.NomeFile) || isPDFPath(docURL) {
		docURL = strings.Replace(docURL, "/visualizzah2/?", "/visualizza/?", 1)
	}
	body, status, err := c.get(ctx, docURL)
	if err != nil {
		return Document{}, err
	}
	if status != http.StatusOK {
		return Document{}, fmt.Errorf("testo integrale: HTTP %d", status)
	}
	if isErrorPage(body) {
		return Document{}, fmt.Errorf("documento non disponibile: il portale ha risposto con la propria pagina di errore per %s. "+
			"I riferimenti (schema, nrg, nome_file) di una ricerca datata non restano validi: rifai la ricerca e riprova", docURL)
	}
	if !bytes.HasPrefix(body, []byte("%PDF-")) {
		return Document{Raw: string(body)}, nil
	}
	text, terr := pdfText(body)
	if terr != nil {
		// Not fatal: the caller renders the "no extractable text" notice, which
		// is still better than passing PDF bytes off as a document.
		return Document{Raw: "", IsPDF: true}, nil
	}
	return Document{Raw: text, IsPDF: true}, nil
}

// pdfText pulls the text layer out of a PDF. Scanned rulings have no such
// layer and yield the empty string, which the caller reports explicitly.
func pdfText(body []byte) (string, error) {
	r, err := pdf.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for i := 1; i <= r.NumPage(); i++ {
		page := r.Page(i)
		if page.V.IsNull() {
			continue
		}
		t, perr := page.GetPlainText(nil)
		if perr != nil {
			continue
		}
		b.WriteString(t)
	}
	return b.String(), nil
}

// DocURL builds the public full-text URL for a provvedimento.
func DocURL(schema, nrg, nomeFile string) string {
	v := url.Values{}
	v.Set("nodeRef", "")
	v.Set("schema", schema)
	v.Set("nrg", nrg)
	v.Set("nomeFile", nomeFile)
	v.Set("subDir", "Provvedimenti")
	// visualizzah2 renders a document as a highlighted HTML page and fails on
	// a PDF; visualizza serves the file itself. Picking the wrong one is the
	// difference between the document and a 159-byte error string.
	endpoint := "visualizzah2"
	if isPDFPath(nomeFile) {
		endpoint = "visualizza"
	}
	return "https://mdp.giustizia-amministrativa.it/" + endpoint + "/?" + v.Encode()
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

// validTipo reports whether t names a known tipo. mapTipo answers "" both for
// "no filter" and for a value it does not recognise, so on its own an unknown
// tipo silently drops the filter and the caller gets provvedimenti of every
// kind while believing they asked for one.
func validTipo(t string) bool {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "", "tutti", "all",
		"sentenza", "sentenze", "ordinanza", "ordinanze",
		"decreto", "decreti", "parere", "pareri",
		"plenaria", "adunanza-plenaria", "generale", "adunanza-generale":
		return true
	}
	return false
}

// validSede reports whether s names a known sede, accepting both the CLI slug
// and the portal's own label. mapSede passes an unrecognised value straight
// through to the portal, which then matches nothing: the caller is told there
// is no case law on the subject when they have in fact only mistyped a city.
func validSede(s string) bool {
	if strings.TrimSpace(s) == "" {
		return true
	}
	if _, ok := resolveSede(s); ok {
		return true
	}
	for _, v := range sediSweepList {
		if strings.EqualFold(v, strings.TrimSpace(s)) {
			return true
		}
	}
	return false
}

// sediSuggestion lists a few valid sedi for an error message.
func sediSuggestion() string {
	return "roma, milano, napoli, consiglio-di-stato, cgars e altre " +
		strconv.Itoa(len(sediSweepList)-4) + " (una per ogni TAR). " +
		"Valgono anche il nome della regione (lazio, sicilia-catania) e il codice dell'ECLI (TARLAZ, TARMI)"
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
	if strings.TrimSpace(s) == "" {
		return ""
	}
	if key, ok := resolveSede(s); ok {
		return sedeMap[key]
	}
	// Accept an already-correct portal value as-is.
	return strings.TrimSpace(s)
}
