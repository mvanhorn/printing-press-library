package rewe

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/rewe/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/rewe/internal/cliutil"
)

// BaseURL is the REWE web shop origin.
const BaseURL = "https://www.rewe.de"

const basketAccept = "application/vnd.com.rewe.digital.basket-v2+json"

// Shop talks to the logged-in REWE web shop with a persisted session.
type Shop struct {
	HTTP    *http.Client
	Session *Session
	Path    string
	limiter *cliutil.AdaptiveLimiter
	renewed bool
}

// HTTPError is a non-2xx answer from REWE.
type HTTPError struct {
	Status int
	URL    string
	Body   string
}

func (e *HTTPError) Error() string {
	b := strings.TrimSpace(e.Body)
	if len(b) > 300 {
		b = b[:300] + "..."
	}
	return fmt.Sprintf("REWE answered HTTP %d for %s: %s", e.Status, e.URL, b)
}

// IsCloudflareBlock reports whether the error is a Cloudflare challenge page.
func IsCloudflareBlock(err error) bool {
	var he *HTTPError
	return errors.As(err, &he) && he.Status == http.StatusForbidden && (strings.Contains(he.Body, "challenge") || strings.Contains(he.Body, "cf-") || strings.Contains(he.Body, "Just a moment"))
}

// Open loads the session file and builds the browser-fingerprint client.
func Open() (*Shop, error) {
	path, err := SessionPath()
	if err != nil {
		return nil, err
	}
	s, err := LoadSession(path)
	if err != nil {
		return nil, err
	}
	return OpenWith(s, path)
}

// OpenWith builds a Shop for a given session and session file path.
func OpenWith(s *Session, path string) (*Shop, error) {
	jar, err := s.Jar()
	if err != nil {
		return nil, err
	}
	hc, err := client.NewBrowserClient(45*time.Second, jar)
	if err != nil {
		return nil, err
	}
	return &Shop{HTTP: hc, Session: s, Path: path, limiter: cliutil.NewAdaptiveLimiter(2)}, nil
}

// NewShopWithClient is used by tests (httptest servers cannot use the rewe.de-only client).
func NewShopWithClient(hc *http.Client, s *Session) *Shop {
	return &Shop{HTTP: hc, Session: s, limiter: cliutil.NewAdaptiveLimiter(50)}
}

// Persist writes cookies the server rotated back into the session file.
func (sh *Shop) Persist() error {
	if sh.Path == "" || sh.HTTP.Jar == nil {
		return nil
	}
	sh.Session.MergeFromJar(sh.HTTP.Jar)
	return sh.Session.Save(sh.Path)
}

// do sends one request with rate limiting and typed errors.
func (sh *Shop) do(ctx context.Context, method, rawURL string, body io.Reader, hdr map[string]string) (*http.Response, []byte, error) {
	if err := sh.limiter.Wait(ctx); err != nil {
		return nil, nil, err
	}
	u := rawURL
	if strings.HasPrefix(u, "/") {
		u = BaseURL + u
	}
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Accept", "application/json")
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	resp, err := sh.HTTP.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	if err != nil {
		return resp, nil, err
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		sh.limiter.OnRateLimit()
		return resp, b, &cliutil.RateLimitError{URL: u, Body: string(b)}
	}
	sh.limiter.OnSuccess()
	if resp.StatusCode >= 400 {
		return resp, b, &HTTPError{Status: resp.StatusCode, URL: u, Body: string(b)}
	}
	return resp, b, nil
}

// JSON sends a request with an optional JSON body and decodes the answer into out.
func (sh *Shop) JSON(ctx context.Context, method, path string, in any, out any, hdr map[string]string) error {
	var body io.Reader
	h := map[string]string{}
	for k, v := range hdr {
		h[k] = v
	}
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
		h["Content-Type"] = "application/json"
	}
	_, b, err := sh.do(ctx, method, path, body, h)
	if err != nil {
		return err
	}
	if out == nil || len(bytes.TrimSpace(b)) == 0 {
		return nil
	}
	if err := json.Unmarshal(b, out); err != nil {
		return fmt.Errorf("decoding %s %s: %w", method, path, err)
	}
	return nil
}

// Form posts an application/x-www-form-urlencoded body (checkout steps) and
// returns the final response after redirects.
func (sh *Shop) Form(ctx context.Context, path string, values url.Values) (*http.Response, []byte, error) {
	return sh.do(ctx, http.MethodPost, path, strings.NewReader(values.Encode()), map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
		"Accept":       "text/html,application/xhtml+xml,application/json;q=0.9,*/*;q=0.8",
	})
}

// HTML fetches a page.
func (sh *Shop) HTML(ctx context.Context, path string) (string, string, error) {
	resp, b, err := sh.do(ctx, http.MethodGet, path, nil, map[string]string{"Accept": "text/html,application/xhtml+xml"})
	if err != nil {
		return "", "", err
	}
	return string(b), resp.Request.URL.String(), nil
}

// Configuration is the shop's view of the current session.
type Configuration struct {
	SelectedService string `json:"selectedService"`
	SelectedMarket  string `json:"selectedMarket"`
	IsLoggedIn      bool   `json:"isLoggedIn"`
	CustomerZipCode string `json:"customerZipCode"`
}

// Config reads /api/marketselection/configuration.
func (sh *Shop) Config(ctx context.Context) (*Configuration, error) {
	var c Configuration
	err := sh.JSON(ctx, http.MethodGet, "/api/marketselection/configuration?checkMarketSelection=false", nil, &c, nil)
	return &c, err
}

var loginFormRe = regexp.MustCompile(`<form[^>]*class="login-confirmation[^"]*"[^>]*action="([^"]+)"`)

// Renew replays the Keycloak single-sign-on: GET /mydata/login, confirm "continue
// with this account", follow the callback. It needs only the account.rewe.de
// cookies (KEYCLOAK_IDENTITY / KEYCLOAK_SESSION), no password.
func (sh *Shop) Renew(ctx context.Context) error {
	if !sh.Session.Has("KEYCLOAK_IDENTITY") && !sh.Session.Has("KEYCLOAK_REMEMBER_ME") {
		return ErrReloginNeeded
	}
	page, final, err := sh.HTML(ctx, "/mydata/login")
	if err != nil {
		return fmt.Errorf("renew: %w", err)
	}
	if strings.Contains(final, "www.rewe.de") && !strings.Contains(final, "account.rewe.de") {
		// Keycloak redirected straight back: already renewed.
		return sh.afterRenew(ctx)
	}
	m := loginFormRe.FindStringSubmatch(page)
	if m == nil {
		// A password form means the SSO session is gone.
		return ErrReloginNeeded
	}
	action := strings.ReplaceAll(m[1], "&amp;", "&")
	if _, _, err := sh.do(ctx, http.MethodPost, action, strings.NewReader("login="), map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
		"Accept":       "text/html,application/xhtml+xml",
		"Referer":      "https://account.rewe.de/",
	}); err != nil {
		return fmt.Errorf("renew: confirming account: %w", err)
	}
	return sh.afterRenew(ctx)
}

func (sh *Shop) afterRenew(ctx context.Context) error {
	c, err := sh.Config(ctx)
	if err != nil {
		return err
	}
	if !c.IsLoggedIn {
		return ErrReloginNeeded
	}
	sh.renewed = true
	sh.Session.RenewedAt = time.Now().UTC()
	return sh.Persist()
}

// Ensure checks the shop session and renews it once when it is gone.
func (sh *Shop) Ensure(ctx context.Context) (*Configuration, error) {
	c, err := sh.Config(ctx)
	if err == nil && c.IsLoggedIn {
		return c, nil
	}
	if err != nil && IsCloudflareBlock(err) {
		return nil, err
	}
	if rerr := sh.Renew(ctx); rerr != nil {
		return nil, rerr
	}
	return sh.Config(ctx)
}

// Renewed reports whether this run renewed the session.
func (sh *Shop) Renewed() bool { return sh.renewed }

var basketIDRe = regexp.MustCompile(`window\.ReweBasket\.id\s*=\s*'([0-9a-f-]{36})'`)

// ExtractBasketID finds the basket id in a shop page.
func ExtractBasketID(html string) string {
	if m := basketIDRe.FindStringSubmatch(html); m != nil {
		return m[1]
	}
	return ""
}

// BasketID reads the current basket id from the basket page.
func (sh *Shop) BasketID(ctx context.Context) (string, error) {
	page, _, err := sh.HTML(ctx, "/shop/checkout/basket")
	if err != nil {
		return "", err
	}
	id := ExtractBasketID(page)
	if id == "" {
		if emptyBasketRe.MatchString(page) {
			return "", ErrEmptyBasket
		}
		return "", errors.New("could not find the basket id on /shop/checkout/basket (page layout changed?)")
	}
	return id, nil
}

var emptyBasketRe = regexp.MustCompile(`window\.ReweBasket\.id\s*=\s*''`)

// ErrEmptyBasket means the customer has no basket yet (e.g. right after an order).
var ErrEmptyBasket = errors.New("the basket is empty")

// Basket is the basket-v2 representation (only the fields the CLI uses; Raw keeps everything).
type Basket struct {
	ID               string `json:"id"`
	Version          int    `json:"version"`
	CustomerUUID     string `json:"customerUuid"`
	ServiceSelection struct {
		WWIdent     string `json:"wwIdent"`
		ServiceType string `json:"serviceType"`
		ZipCode     string `json:"zipCode"`
	} `json:"serviceSelection"`
	LineItems []struct {
		Quantity   int `json:"quantity"`
		Price      int `json:"price"`
		TotalPrice int `json:"totalPrice"`
		Product    struct {
			ProductID string `json:"productId"`
			Title     string `json:"title"`
			Listing   struct {
				ListingID string `json:"listingId"`
			} `json:"listing"`
		} `json:"product"`
	} `json:"lineItems"`
	Summary struct {
		ArticlePrice int `json:"articlePrice"`
		TotalPrice   int `json:"totalPrice"`
	} `json:"summary"`
	TimeSlotInformation *struct {
		TimeSlotID   string `json:"timeSlotId"`
		StartTime    string `json:"startTime"`
		EndTime      string `json:"endTime"`
		TimeSlotText string `json:"timeSlotText"`
	} `json:"timeSlotInformation"`
	Raw json.RawMessage `json:"-"`
}

func decodeBasket(b []byte) (*Basket, error) {
	var bk Basket
	if err := json.Unmarshal(b, &bk); err != nil {
		return nil, err
	}
	bk.Raw = append(json.RawMessage(nil), b...)
	return &bk, nil
}

func (sh *Shop) basketCall(ctx context.Context, method, path string, in any) (*Basket, error) {
	var raw json.RawMessage
	if err := sh.JSON(ctx, method, path, in, &raw, map[string]string{"Accept": basketAccept, "x-application-id": "rewe-basket"}); err != nil {
		return nil, err
	}
	return decodeBasket(raw)
}

// GetBasket reads a basket.
func (sh *Shop) GetBasket(ctx context.Context, id string) (*Basket, error) {
	return sh.basketCall(ctx, http.MethodGet, "/shop/api/baskets/"+url.PathEscape(id)+"?includeTimeslot=true", nil)
}

// SetQuantity sets a listing's quantity (adds when missing).
func (sh *Shop) SetQuantity(ctx context.Context, listingID string, qty int) (*Basket, error) {
	return sh.basketCall(ctx, http.MethodPost, "/shop/api/baskets/listings/"+url.PathEscape(listingID)+"?includeTimeslot=true",
		map[string]any{"quantity": qty, "includeTimeslot": true, "context": "product-list-search"})
}

// RemoveListing removes a listing.
func (sh *Shop) RemoveListing(ctx context.Context, basketID, listingID string) (*Basket, error) {
	return sh.basketCall(ctx, http.MethodDelete, "/shop/api/baskets/"+url.PathEscape(basketID)+"/listings/"+url.PathEscape(listingID)+"?includeTimeslot=true", nil)
}

// LineItem is one entry of a multi-add.
type LineItem struct {
	ListingID string `json:"listingId"`
	Quantity  int    `json:"quantity"`
}

// AddMany adds several listings at once.
func (sh *Shop) AddMany(ctx context.Context, items []LineItem, contextLabel string) (json.RawMessage, error) {
	var raw json.RawMessage
	err := sh.JSON(ctx, http.MethodPost, "/shop/api/baskets/listings", map[string]any{"context": contextLabel, "lineItems": items}, &raw, map[string]string{"Accept": basketAccept})
	return raw, err
}

// RefreshBasket re-validates prices and availability before checkout.
func (sh *Shop) RefreshBasket(ctx context.Context, id string) (*Basket, error) {
	return sh.basketCall(ctx, http.MethodPost, "/shop/api/baskets/"+url.PathEscape(id)+"/changes", map[string]any{"includeTimeslot": true})
}

// Slot is a pickup timeslot.
type Slot struct {
	ID         string          `json:"id"`
	StartTime  time.Time       `json:"startTime"`
	EndTime    time.Time       `json:"endTime"`
	Selected   bool            `json:"selected"`
	ServiceFee json.RawMessage `json:"serviceFee,omitempty"`
	Labels     json.RawMessage `json:"labels,omitempty"`
}

// Slots lists pickup slots.
func (sh *Shop) Slots(ctx context.Context) ([]Slot, error) {
	var s []Slot
	err := sh.JSON(ctx, http.MethodGet, "/shop/api/timeslots/pickup/checkout", nil, &s, nil)
	return s, err
}

// Reservation is the answer to a slot reservation.
type Reservation struct {
	SlotID         string          `json:"slotId"`
	ExpireTime     string          `json:"expireTime"`
	SlotStartTime  string          `json:"slotStartTime"`
	SlotEndTime    string          `json:"slotEndTime"`
	SlotCutoffTime string          `json:"slotCutoffTime"`
	Fee            json.RawMessage `json:"fee"`
}

// Reserve reserves a slot for the customer at the market.
func (sh *Shop) Reserve(ctx context.Context, slotID, customerID, market, zip string) (*Reservation, error) {
	var r Reservation
	err := sh.JSON(ctx, http.MethodPost, "/shop/api/timeslot-reservations", map[string]string{"slotId": slotID, "customerId": customerID, "wwIdent": market, "zipCode": zip}, &r, nil)
	return &r, err
}

// PaymentOptions returns the raw payment options for an order total in cents.
func (sh *Shop) PaymentOptions(ctx context.Context, totalCents int) (json.RawMessage, error) {
	var raw json.RawMessage
	err := sh.JSON(ctx, http.MethodGet, fmt.Sprintf("/shop/api/payment-options?totalOrderAmount=%d&overchargeAmount=900&shippingAmount=0", totalCents), nil, &raw, nil)
	return raw, err
}

// MarketPaymentEnabled reports whether pay-at-pickup is enabled in a payment-options answer.
func MarketPaymentEnabled(raw json.RawMessage) bool {
	var v struct {
		PaymentOptions struct {
			EnabledOfflinePaymentMethods []struct {
				Key string `json:"key"`
			} `json:"enabledOfflinePaymentMethods"`
		} `json:"paymentOptions"`
	}
	if json.Unmarshal(raw, &v) != nil {
		return false
	}
	for _, m := range v.PaymentOptions.EnabledOfflinePaymentMethods {
		if m.Key == "MARKET_PAYMENT" {
			return true
		}
	}
	return false
}

// OrderResult is what the confirmation endpoint answered.
type OrderResult struct {
	Status   int             `json:"status"`
	Location string          `json:"location,omitempty"`
	Body     json.RawMessage `json:"body,omitempty"`
}

// SubmitCheckout walks the web checkout: basket submit, timeslot submit,
// payment MARKET_PAYMENT. It does not place the order.
func (sh *Shop) SubmitCheckout(ctx context.Context, basketID, slotID string) error {
	if _, _, err := sh.do(ctx, http.MethodGet, "/shop/checkout/basket/submit", nil, map[string]string{"Accept": "text/html"}); err != nil {
		return fmt.Errorf("checkout basket step: %w", err)
	}
	if _, _, err := sh.Form(ctx, "/shop/checkout/timeslot/submit", url.Values{"basketId": {basketID}, "timeslotId": {slotID}}); err != nil {
		return fmt.Errorf("checkout timeslot step: %w", err)
	}
	if _, _, err := sh.Form(ctx, "/shop/checkout/payment-options/submit", url.Values{
		"paymentOption": {"MARKET_PAYMENT"}, "dateOfBirth": {""}, "reusePaymentDataOption": {"NONE"}, "invoiceAddressId": {""}, "invoiceAddressVersion": {""},
	}); err != nil {
		return fmt.Errorf("checkout payment step: %w", err)
	}
	return nil
}

// PlaceOrder submits the confirmation. This spends money.
func (sh *Shop) PlaceOrder(ctx context.Context, basketID, phone, message string) (*OrderResult, error) {
	b, _ := json.Marshal(map[string]string{"phoneNumber": phone, "additionalMessage": message, "basketId": basketID})
	resp, body, err := sh.do(ctx, http.MethodPost, "/shop/api/confirmation/submit", bytes.NewReader(b), map[string]string{"Content-Type": "application/json", "Accept": "application/json"})
	if err != nil {
		return nil, err
	}
	res := &OrderResult{Status: resp.StatusCode, Location: resp.Header.Get("Location")}
	if json.Valid(body) {
		res.Body = body
	}
	// A 200 answer names the next hop ({method,url,bodyParameters}); the web
	// page auto-submits it. Follow it the same way so the order completes.
	var next struct {
		Method         string            `json:"method"`
		URL            string            `json:"url"`
		BodyParameters map[string]string `json:"bodyParameters"`
	}
	// A 204 answer carries the next hop in Location (the page navigates
	// there, e.g. /shop/checkout/paymentcompletion/submit); the order is only
	// final after that hop.
	if resp.StatusCode == http.StatusNoContent && res.Location != "" {
		fr, _, err := sh.do(ctx, http.MethodGet, res.Location, nil, map[string]string{"Accept": "text/html"})
		if err != nil {
			return res, fmt.Errorf("order submitted, but completing it at %s failed: %w", res.Location, err)
		}
		res.Location = fr.Request.URL.String()
		return res, nil
	}
	if resp.StatusCode == http.StatusOK && json.Unmarshal(body, &next) == nil && next.URL != "" {
		vals := url.Values{}
		for k, v := range next.BodyParameters {
			vals.Set(k, v)
		}
		var fr *http.Response
		if strings.EqualFold(next.Method, "POST") {
			fr, _, err = sh.Form(ctx, next.URL, vals)
		} else {
			fr, _, err = sh.do(ctx, http.MethodGet, next.URL, nil, map[string]string{"Accept": "text/html"})
		}
		if err != nil {
			return res, fmt.Errorf("order submitted, but following %s failed: %w", next.URL, err)
		}
		res.Location = fr.Request.URL.String()
	}
	return res, nil
}
