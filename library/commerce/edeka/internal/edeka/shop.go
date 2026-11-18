package edeka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/edeka/internal/cliutil"
)

// UserAgent is sent on every request. The shop binds a session to the client,
// so the CLI keeps one stable identity and gets its own session through the
// remember-me cookie.
const UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

// Shop talks to one market's web shop with a persisted session.
type Shop struct {
	HTTP    *http.Client
	Session *Session
	Path    string // session file; "" means nothing is persisted
	Host    string // market host, e.g. doerflinger-ulm.edeka.shop
	base    string // https://<host>, or a test server URL
	portal  string
	limiter *cliutil.AdaptiveLimiter
	rec     *recordingTransport
	mu      sync.Mutex // guards Session.Stoken for concurrent page reads
}

// HTTPError is a non-2xx answer from the shop.
type HTTPError struct {
	Status int
	URL    string
	Body   string
}

func (e *HTTPError) Error() string {
	b := strings.TrimSpace(e.Body)
	if len(b) > 200 {
		b = b[:200] + "..."
	}
	return fmt.Sprintf("EDEKA shop answered HTTP %d for %s: %s", e.Status, e.URL, b)
}

// Open loads the session (when one exists) and builds a Shop for the market
// resolved from explicit/EDEKA_MARKET/session/saved selection. requireSession
// makes a missing session file an error.
func Open(market string, requireSession bool) (*Shop, error) {
	path, err := SessionPath()
	if err != nil {
		return nil, err
	}
	s, err := LoadSession(path)
	if errors.Is(err, ErrNoSession) && !requireSession {
		s, path = &Session{}, ""
	} else if err != nil {
		return nil, err
	}
	host, err := ResolveMarket(market, s)
	if err != nil {
		return nil, err
	}
	return OpenWith(s, path, host)
}

// OpenWith builds a Shop for a session, session file and market host.
func OpenWith(s *Session, path, host string) (*Shop, error) {
	jar, err := s.Jar()
	if err != nil {
		return nil, err
	}
	rec := &recordingTransport{next: shopOnlyTransport{next: http.DefaultTransport.(*http.Transport).Clone()}}
	hc := &http.Client{
		Timeout:   60 * time.Second,
		Jar:       jar,
		Transport: rec,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 8 {
				return errors.New("too many redirects")
			}
			if req.URL.Scheme != "https" || !IsShopHost(req.URL.Hostname()) {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
	return &Shop{HTTP: hc, Session: s, Path: path, Host: host, base: "https://" + host, portal: PortalURL, limiter: cliutil.NewAdaptiveLimiter(3), rec: rec}, nil
}

// NewTestShop builds a Shop against a test server (both market and portal).
func NewTestShop(hc *http.Client, s *Session, baseURL string) *Shop {
	rec := &recordingTransport{next: hc.Transport}
	if rec.next == nil {
		rec.next = http.DefaultTransport
	}
	hc.Transport = rec
	if hc.Jar == nil {
		hc.Jar, _ = s.Jar()
	}
	u, _ := url.Parse(baseURL)
	return &Shop{HTTP: hc, Session: s, Host: u.Host, base: strings.TrimRight(baseURL, "/"), portal: strings.TrimRight(baseURL, "/"), limiter: cliutil.NewAdaptiveLimiter(1000), rec: rec}
}

type shopOnlyTransport struct{ next http.RoundTripper }

func (t shopOnlyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Scheme != "https" || !IsShopHost(req.URL.Hostname()) {
		return nil, errors.New("refusing request outside https://*.edeka.shop: " + req.URL.Host)
	}
	return t.next.RoundTrip(req)
}

// setToken stores the shop's current form token.
func (sh *Shop) setToken(tok string) {
	if sh.Session == nil || tok == "" {
		return
	}
	sh.mu.Lock()
	sh.Session.Stoken = tok
	sh.mu.Unlock()
}

func (sh *Shop) cachedToken() string {
	if sh.Session == nil {
		return ""
	}
	sh.mu.Lock()
	defer sh.mu.Unlock()
	return sh.Session.Stoken
}

// BaseURL returns the market shop origin.
func (sh *Shop) BaseURL() string { return sh.base }

// Persist writes cookies the server set or rotated back into the session file.
// The market is not written: a Shop never changes it, and writing the copy
// loaded at Open would undo a 'markets select' another run made meanwhile.
func (sh *Shop) Persist() error {
	if sh.Path == "" || sh.Session == nil {
		return nil
	}
	var updates []setCookieUpdate
	if sh.rec != nil {
		updates = sh.rec.snapshot()
	}
	sh.Session.ApplySetCookies(updates)
	sh.mu.Lock()
	tok := sh.Session.Stoken
	sh.mu.Unlock()
	return sh.Session.SaveMerged(sh.Path, updates, "", tok)
}

func (sh *Shop) abs(p string) string {
	if strings.HasPrefix(p, "http://") || strings.HasPrefix(p, "https://") {
		return p
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return sh.base + p
}

// do sends one request (form-encoded when form != nil) and returns the body.
func (sh *Shop) do(ctx context.Context, method, rawURL string, form url.Values, xhr bool) ([]byte, *http.Response, error) {
	if err := sh.limiter.Wait(ctx); err != nil {
		return nil, nil, err
	}
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept-Language", "de-DE,de;q=0.9")
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	}
	if xhr {
		req.Header.Set("X-Requested-With", "XMLHttpRequest")
		req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	} else {
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	}
	resp, err := sh.HTTP.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, resp, err
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		sh.limiter.OnRateLimit()
		return b, resp, &cliutil.RateLimitError{URL: rawURL, Body: string(b[:min(len(b), 300)])}
	}
	sh.limiter.OnSuccess()
	if resp.StatusCode >= 400 {
		return b, resp, &HTTPError{Status: resp.StatusCode, URL: rawURL, Body: string(b[:min(len(b), 300)])}
	}
	return b, resp, nil
}

// Page fetches an HTML page of the market shop and remembers its form token.
func (sh *Shop) Page(ctx context.Context, path string) (string, error) {
	b, _, err := sh.do(ctx, http.MethodGet, sh.abs(path), nil, false)
	if err != nil {
		return "", err
	}
	page := string(b)
	if tok := ExtractStoken(page); tok != "" && sh.Session != nil {
		sh.setToken(tok)
	}
	return page, nil
}

// getJSON fetches a JSON endpoint and decodes it.
func (sh *Shop) getJSON(ctx context.Context, rawURL string, out any) error {
	b, _, err := sh.do(ctx, http.MethodGet, rawURL, nil, true)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, out); err != nil {
		return fmt.Errorf("EDEKA answered something other than JSON for %s: %w", rawURL, err)
	}
	return nil
}

// LoggedIn asks the small service-menu widget whether this session is logged
// in. Sending the remember-me cookie logs a fresh session in on the way.
func (sh *Shop) LoggedIn(ctx context.Context) (bool, error) {
	b, _, err := sh.do(ctx, http.MethodGet, sh.abs("/widget.php?lang=0&cl=oxwservicemenu"), nil, false)
	if err != nil {
		return false, err
	}
	s := string(b)
	if strings.Contains(s, `aria-label="Mein Konto"`) {
		return true, nil
	}
	if strings.Contains(s, `aria-label="Anmelden"`) {
		return false, nil
	}
	return false, errors.New("could not tell from the shop whether the session is logged in")
}

// Ensure makes sure the session is logged in for this market. Without a
// remember-me cookie for the market, or when the shop rejects it, it returns
// ErrReloginNeeded.
func (sh *Shop) Ensure(ctx context.Context) error {
	if sh.Session == nil || len(sh.Session.Cookies) == 0 {
		return ErrNoSession
	}
	ok, err := sh.LoggedIn(ctx)
	if err != nil {
		return err
	}
	if !ok {
		return ErrReloginNeeded
	}
	return nil
}

// token returns the cached form token or fetches the basket page for one.
func (sh *Shop) token(ctx context.Context, refresh bool) (string, error) {
	if tok := sh.cachedToken(); !refresh && tok != "" {
		return tok, nil
	}
	page, err := sh.Page(ctx, "/index.php?cl=basket")
	if err != nil {
		return "", err
	}
	tok := ExtractStoken(page)
	if tok == "" {
		return "", errors.New("the shop page carried no form token (stoken)")
	}
	return tok, nil
}

// postFnc posts a controller function with the form token. When the shop
// answers with an HTML page instead of JSON (stale token), it refreshes the
// token once and retries.
func (sh *Shop) postFnc(ctx context.Context, form url.Values, out any) error {
	for attempt := 0; attempt < 2; attempt++ {
		tok, err := sh.token(ctx, attempt > 0)
		if err != nil {
			return err
		}
		f := url.Values{}
		for k, v := range form {
			f[k] = v
		}
		f.Set("stoken", tok)
		f.Set("lang", "0")
		b, _, err := sh.do(ctx, http.MethodPost, sh.abs("/index.php?"), f, true)
		if err != nil {
			return err
		}
		if looksLikeHTML(b) {
			if tok := ExtractStoken(string(b)); tok != "" && sh.Session != nil {
				sh.setToken(tok)
			}
			continue
		}
		if out == nil {
			return nil
		}
		if err := json.Unmarshal(b, out); err != nil {
			return fmt.Errorf("unexpected answer from the shop: %w", err)
		}
		return nil
	}
	return errors.New("the shop refused the request (form token not accepted twice); log in again if this repeats")
}

// SearchMarkets finds markets by postal code; a town name is resolved through
// the portal's town lookup first.
func (sh *Shop) SearchMarkets(ctx context.Context, query string) ([]Market, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, errors.New("give a postal code or town")
	}
	if IsZip(q) {
		ms, err := sh.marketsByZip(ctx, q, "")
		if err != nil || len(ms) > 0 {
			return ms, err
		}
		return sh.nearbyMarkets(ctx, q)
	}
	var cities []City
	if err := sh.getJSON(ctx, sh.portal+"/index.php?cl=MxMarketSearch&fnc=searchCity&search="+url.QueryEscape(q), &cities); err != nil {
		return nil, err
	}
	var out []Market
	seen := map[string]bool{}
	lq := strings.ToLower(q)
	for _, c := range cities {
		if !strings.HasPrefix(strings.ToLower(c.Name), lq) || c.Zipcode == "" {
			continue
		}
		ms, err := sh.marketsByZip(ctx, c.Zipcode, c.Name)
		if err != nil {
			return nil, err
		}
		for _, m := range ms {
			if !seen[m.Host] {
				seen[m.Host] = true
				out = append(out, m)
			}
		}
	}
	return out, nil
}

// MaxNearbyProbes caps the extra market searches one nearby lookup may send.
const MaxNearbyProbes = 24

// nearbyMarkets is the fallback for a postal code no market serves: the portal
// has no radius search, so it asks for nearby postal codes (German codes are
// grouped by area) until it has found three markets or used MaxNearbyProbes.
func (sh *Shop) nearbyMarkets(ctx context.Context, zip string) ([]Market, error) {
	var out []Market
	seen := map[string]bool{}
	for _, z := range nearbyProbeZips(zip, MaxNearbyProbes) {
		ms, err := sh.marketsByZip(ctx, z, "")
		if err != nil {
			return out, err
		}
		for _, m := range ms {
			if !seen[m.Host] {
				seen[m.Host] = true
				m.Nearby, m.ServedZip = true, z
				out = append(out, m)
			}
		}
		if len(out) >= 3 {
			break
		}
	}
	return out, nil
}

// nearbyProbeZips lists up to max postal codes around zip, nearest first: zip±1,
// ±2, ... inside its three-digit block, then the same code in the neighbouring
// three-digit blocks.
func nearbyProbeZips(zip string, max int) []string {
	n, err := strconv.Atoi(zip)
	if err != nil || len(zip) != 5 {
		return nil
	}
	block := n / 100
	out := []string{}
	add := func(v int) {
		if v >= 1000 && v <= 99999 && len(out) < max {
			out = append(out, fmt.Sprintf("%05d", v))
		}
	}
	for d := 1; d < 100 && len(out) < max-4; d++ {
		for _, v := range []int{n - d, n + d} {
			if v/100 == block {
				add(v)
			}
		}
	}
	for d := 1; d <= 2; d++ {
		add(n - 100*d)
		add(n + 100*d)
	}
	return out
}

func (sh *Shop) marketsByZip(ctx context.Context, zip, city string) ([]Market, error) {
	var raw []RawMarket
	if err := sh.getJSON(ctx, sh.portal+"/index.php?cl=MxMarketSearch&fnc=searchMarketsByZipOrText&search="+url.QueryEscape(zip), &raw); err != nil {
		return nil, err
	}
	out := make([]Market, 0, len(raw))
	for _, r := range raw {
		if r.Domain == "" || !truthy(r.Active) && r.Active != nil {
			continue
		}
		m := r.ToMarket()
		m.MatchedByCity = city
		out = append(out, m)
	}
	return out, nil
}

// Search runs the shop's product search (one page of 32 tiles; page is zero-based).
func (sh *Shop) Search(ctx context.Context, query string, page int) (Grid, error) {
	p, err := sh.Page(ctx, "/index.php?cl=search&searchparam="+url.QueryEscape(query)+"&pgNr="+strconv.Itoa(page))
	if err != nil {
		return Grid{}, err
	}
	return ParseProductGrid(p, page)
}

// Offers reads one page of the market's offers, optionally one offer category
// (path segment under /Angebote/, e.g. "Kuehlprodukte").
func (sh *Shop) Offers(ctx context.Context, category string, page int) (Grid, error) {
	path := "/Angebote/"
	if c := strings.Trim(strings.TrimSpace(category), "/"); c != "" {
		path += url.PathEscape(c) + "/"
	}
	p, err := sh.Page(ctx, path+"?pgNr="+strconv.Itoa(page))
	if err != nil {
		var he *HTTPError
		if errors.As(err, &he) && he.Status == http.StatusNotFound {
			return Grid{}, fmt.Errorf("no offer category %q in this market (see `categories --offers`)", category)
		}
		return Grid{}, err
	}
	return ParseProductGrid(p, page)
}

// Categories reads the market's category tree from the offers page navigation.
func (sh *Shop) Categories(ctx context.Context) ([]Category, error) {
	p, err := sh.Page(ctx, "/Angebote/")
	if err != nil {
		return nil, err
	}
	cs := ParseCategories(p, sh.Host)
	if len(cs) == 0 {
		return nil, errors.New("no categories found on the shop page")
	}
	return cs, nil
}

// Product fetches a product page by article id or shop URL/path.
func (sh *Shop) Product(ctx context.Context, idOrURL string) (*ProductDetail, error) {
	path := idOrURL
	if !strings.Contains(idOrURL, "/") {
		path = "/index.php?cl=details&anid=" + url.QueryEscape(idOrURL)
	} else if u, err := url.Parse(idOrURL); err == nil && u.Host != "" {
		if !IsShopHost(u.Hostname()) {
			return nil, errors.New("not an edeka.shop product URL")
		}
		path = u.RequestURI()
	}
	p, err := sh.Page(ctx, path)
	if err != nil {
		return nil, err
	}
	d, err := ParseProductDetail(p)
	if err != nil {
		return nil, fmt.Errorf("product %q not found in this market", idOrURL)
	}
	if d.ID == "" && !strings.Contains(idOrURL, "/") {
		d.ID = idOrURL
	}
	if d.URL == "" {
		d.URL = sh.abs(path)
	}
	return d, nil
}

// Suggest returns the elastic suggester rows (JSON).
func (sh *Shop) Suggest(ctx context.Context, term string) ([]map[string]any, error) {
	var out []map[string]any
	err := sh.getJSON(ctx, sh.abs("/index.php?cl=ith_oxelastic_suggester&elastic_type=json&term="+url.QueryEscape(term)), &out)
	return out, err
}

// Basket reads the basket page.
func (sh *Shop) Basket(ctx context.Context) (*Basket, error) {
	p, err := sh.Page(ctx, "/index.php?cl=basket")
	if err != nil {
		return nil, err
	}
	return ParseBasket(p)
}

// ChangeResult is the shop's answer to a basket change.
type ChangeResult struct {
	Success      bool    `json:"success"`
	ProductCount int     `json:"productCount"`
	BruttoSum    float64 `json:"NewBruttoSum"`
	Message      any     `json:"message"`
	LimitMessage any     `json:"limit_message"`
}

// SetQuantity sets the absolute quantity of an article (0 removes it).
// grams > 0 adds a weight article (meat/cheese counter) with that weight.
func (sh *Shop) SetQuantity(ctx context.Context, articleID string, qty, grams int) (*ChangeResult, error) {
	if articleID == "" {
		return nil, errors.New("missing article id")
	}
	f := url.Values{"cl": {"alist"}, "fnc": {"changeBasket"}, "aid": {articleID}, "anid": {articleID}, "am": {strconv.Itoa(qty)}, "response": {"true"}}
	if grams > 0 {
		f.Set("persparam[Gewicht]", strconv.Itoa(grams))
		f.Set("sel[0]", "0")
	}
	var r ChangeResult
	if err := sh.postFnc(ctx, f, &r); err != nil {
		return nil, err
	}
	if !r.Success {
		msg := "the shop did not accept the basket change"
		if s, ok := r.LimitMessage.(string); ok && s != "" {
			msg += ": " + s
		} else if s, ok := r.Message.(string); ok && s != "" {
			msg += ": " + s
		}
		return &r, errors.New(msg)
	}
	return &r, nil
}

// UpdateBasketItem changes an existing basket line through the basket page's
// own form (cl=basket, fnc=changebasket): qty is absolute, remove drops the
// line. This also works for weight articles, whose lines carry a weight.
func (sh *Shop) UpdateBasketItem(ctx context.Context, it BasketItem, qty int, remove bool) (*Basket, error) {
	key := it.BasketItemID
	if key == "" {
		return nil, fmt.Errorf("basket line %s has no basket item id", it.ID)
	}
	rm := "0"
	if remove || qty <= 0 {
		rm, qty = "1", 0
	}
	f := url.Values{
		"cl": {"basket"}, "fnc": {"changebasket"},
		"aproducts[" + key + "][aid]":          {it.ID},
		"aproducts[" + key + "][basketitemid]": {key},
		"aproducts[" + key + "][override]":     {"1"},
		"aproducts[" + key + "][am]":           {strconv.Itoa(qty)},
		"aproducts[" + key + "][remove]":       {rm},
	}
	for attempt := 0; attempt < 2; attempt++ {
		tok, err := sh.token(ctx, attempt > 0)
		if err != nil {
			return nil, err
		}
		f.Set("stoken", tok)
		f.Set("lang", "0")
		if _, _, err := sh.do(ctx, http.MethodPost, sh.abs("/index.php?"), f, false); err != nil {
			return nil, err
		}
		b, err := sh.Basket(ctx)
		if err != nil {
			return nil, err
		}
		done := true
		for _, x := range b.Items {
			if x.BasketItemID == key && (rm == "1" || x.Quantity != qty) {
				done = false
			}
		}
		if done {
			return b, nil
		}
	}
	return nil, errors.New("the shop did not apply the basket change")
}

// Calendar reads the slot calendar widget.
func (sh *Shop) Calendar(ctx context.Context) (*Calendar, error) {
	b, _, err := sh.do(ctx, http.MethodPost, sh.abs("/widget.php?lang=0"), url.Values{"cl": {"MxCalendarWidget"}}, true)
	if err != nil {
		return nil, err
	}
	return ParseCalendar(string(b))
}

// SelectedSlotText returns the shop's text for the slot chosen in this session.
func (sh *Shop) SelectedSlotText(ctx context.Context) (string, error) {
	var r struct {
		Success bool   `json:"success"`
		Text    string `json:"mxdeliverycoltext"`
	}
	if err := sh.getJSON(ctx, sh.abs("/index.php?cl=mx_pickup_controller&fnc=getMxDeliveryColText"), &r); err != nil {
		return "", err
	}
	if !r.Success {
		return "", nil
	}
	doc, err := parseHTML(r.Text)
	if err != nil {
		return r.Text, nil
	}
	return text(doc), nil
}

// ReserveSlot chooses a slot for this session (the shop keeps it in cookies
// and may hold a time-limited reservation).
func (sh *Shop) ReserveSlot(ctx context.Context, s Slot) error {
	f := url.Values{
		"cl":                       {"mx_pickup_controller"},
		"fnc":                      {"setCookie"},
		"data[date]":               {s.Date},
		"data[ignoreDeliveryArea]": {"false"},
		"data[time]":               {strconv.FormatInt(s.Timestamp, 10)},
		"data[type]":               {s.Type},
		"data[timespan][from]":     {s.From},
		"data[timespan][until]":    {s.Until},
		"data[timespan][selected]": {s.From + " - " + s.Until},
		"data[premium]":            {strconv.FormatBool(s.Premium)},
	}
	b, _, err := sh.do(ctx, http.MethodPost, sh.abs("/index.php?"), f, true)
	if err != nil {
		return err
	}
	var r struct {
		Success bool `json:"success"`
	}
	if json.Unmarshal(b, &r) != nil || !r.Success {
		return errors.New("the shop did not accept the slot (it may be taken; run `slots list` again)")
	}
	_, _, err = sh.do(ctx, http.MethodPost, sh.abs("/index.php?"), url.Values{"cl": {"mx_pickup_controller"}, "fnc": {"changeshipping"}, "sShipSet": {"mx" + s.Type}}, true)
	return err
}

// ReleaseSlot asks the shop to drop the chosen slot. Some markets answer the
// shop's own unset call with HTTP 500 (also in the browser); then serverOK is
// false and the reservation simply lapses after its timer (see
// SelectedSlotText for the remaining minutes).
func (sh *Shop) ReleaseSlot(ctx context.Context) (serverOK bool, err error) {
	_, _, err = sh.do(ctx, http.MethodPost, sh.abs("/index.php?"), url.Values{"cl": {"mx_pickup_controller"}, "fnc": {"unsetPickUpDate"}}, true)
	var he *HTTPError
	if err != nil && errors.As(err, &he) && he.Status >= 500 {
		return false, nil
	}
	return err == nil, err
}

// PaymentOptions walks to the payment step and lists the payment methods.
func (sh *Shop) PaymentOptions(ctx context.Context) ([]PaymentOption, error) {
	page, err := sh.toPaymentStep(ctx)
	if err != nil {
		return nil, err
	}
	opts := ParsePaymentOptions(page)
	if len(opts) == 0 {
		return nil, errors.New("the payment step showed no payment methods (is the basket empty or no slot chosen?)")
	}
	return opts, nil
}

// ErrCheckoutBlocked means the shop did not let the checkout reach the step.
var ErrCheckoutBlocked = errors.New("the shop did not open the checkout step")

// toPaymentStep submits the address step unchanged and returns the payment page.
func (sh *Shop) toPaymentStep(ctx context.Context) (string, error) {
	user, err := sh.Page(ctx, "/index.php?cl=user")
	if err != nil {
		return "", err
	}
	fields, ok := FormFields(user, func(_, name string) bool { return name == "order" })
	if !ok {
		if strings.Contains(user, `name="paymentid"`) {
			return user, nil
		}
		return "", fmt.Errorf("%w: the address step has no form (basket empty or no slot chosen?)", ErrCheckoutBlocked)
	}
	f := url.Values(fields)
	f.Set("userform", "")
	b, resp, err := sh.do(ctx, http.MethodPost, sh.abs("/index.php?"), f, false)
	if err != nil {
		return "", err
	}
	page := string(b)
	if tok := ExtractStoken(page); tok != "" {
		sh.setToken(tok)
	}
	if !strings.Contains(page, `name="paymentid"`) {
		where := ""
		if resp != nil && resp.Request != nil {
			where = " (stopped at " + resp.Request.URL.Path + "?" + redactQuery(resp.Request.URL.RawQuery) + ")"
		}
		return "", fmt.Errorf("%w: payment methods not shown after the address step%s", ErrCheckoutBlocked, where)
	}
	return page, nil
}

// PreviewOrder walks address -> payment (with paymentID) -> review and returns
// the review without placing the order. The returned fields are the order
// form the shop expects for the final step.
func (sh *Shop) PreviewOrder(ctx context.Context, paymentID string) (*OrderReview, map[string][]string, error) {
	pay, err := sh.toPaymentStep(ctx)
	if err != nil {
		return nil, nil, err
	}
	fields, ok := FormFields(pay, func(_, name string) bool { return name == "order" })
	if !ok {
		return nil, nil, fmt.Errorf("%w: no payment form", ErrCheckoutBlocked)
	}
	known := false
	for _, o := range ParsePaymentOptions(pay) {
		if o.ID == paymentID {
			known = true
		}
	}
	if !known {
		return nil, nil, fmt.Errorf("payment method %q is not offered by this market (see `payment options`)", paymentID)
	}
	f := url.Values{}
	for k, v := range fields {
		if strings.HasPrefix(k, "telecash") || k == "paymentid" || k == "mxtelecashdebitcheckbox" {
			continue
		}
		f[k] = v
	}
	f.Set("paymentid", paymentID)
	f.Set("userform", "")
	b, _, err := sh.do(ctx, http.MethodPost, sh.abs("/index.php?"), f, false)
	if err != nil {
		return nil, nil, err
	}
	page := string(b)
	if tok := ExtractStoken(page); tok != "" {
		sh.setToken(tok)
	}
	review, err := ParseOrderReview(page)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: the order review did not open after choosing %s", ErrCheckoutBlocked, paymentID)
	}
	orderForm, _ := FormFields(page, func(id, _ string) bool { return id == "orderConfirmAgbBottom" })
	if orderForm == nil {
		orderForm, _ = FormFields(page, func(id, _ string) bool { return id == "orderConfirmAgbTop" })
	}
	return review, orderForm, nil
}

// PlaceOrder submits the order review form (the shop's "JETZT KAUFEN").
// Only call it after the customer confirmed the preview.
func (sh *Shop) PlaceOrder(ctx context.Context, orderForm map[string][]string) (string, error) {
	if len(orderForm) == 0 {
		return "", errors.New("no order form from the review step")
	}
	f := url.Values(orderForm)
	f.Set("cl", "order")
	f.Set("fnc", "execute")
	f.Set("ord_agb", "1")
	b, resp, err := sh.do(ctx, http.MethodPost, sh.abs("/index.php?"), f, false)
	if err != nil {
		return "", err
	}
	page := string(b)
	thankYou := false
	if resp != nil && resp.Request != nil {
		u := resp.Request.URL
		thankYou = u.Query().Get("cl") == "thankyou" || strings.Contains(strings.ToLower(u.Path), "thankyou")
	}
	if !thankYou && strings.Contains(page, `id="thankyouPage"`) {
		thankYou = true
	}
	if thankYou {
		if m := orderNrRe.FindStringSubmatch(text(mustParse(page))); m != nil {
			return m[1], nil
		}
		return "unknown", nil
	}
	return "", errors.New("the shop did not confirm the order (no thank-you page); check `order list` before retrying")
}

// Orders reads the order history.
func (sh *Shop) Orders(ctx context.Context) ([]OrderSummary, int, error) {
	p, err := sh.Page(ctx, "/index.php?cl=account_order")
	if err != nil {
		return nil, 0, err
	}
	if strings.Contains(p, `name="lgn_usr"`) && !strings.Contains(p, "order__title") {
		return nil, 0, ErrReloginNeeded
	}
	return ParseOrderHistory(p)
}

func mustParse(page string) *htmlNode {
	n, err := parseHTML(page)
	if err != nil {
		return nil
	}
	return n
}

func redactQuery(q string) string {
	v, err := url.ParseQuery(q)
	if err != nil {
		return ""
	}
	for _, k := range []string{"force_sid", "sid", "stoken"} {
		v.Del(k)
	}
	return v.Encode()
}
