// Package amazon is the replayable HTTP client against amazon.com.
//
// Surface (all live, no Playwright):
//
//	GET  /                              - signed-in marker probe (doctor)
//	GET  /gp/cart/view.html             - cart contents (parse line items)
//	POST /gp/aws/cart/add.html          - add ASIN to cart (form-encoded)
//	GET  /gp/buy/spc/handlers/display.html - begin checkout (returns the
//	                                       "place your order" page; the
//	                                       form's hidden tokens go into the
//	                                       follow-up POST)
//	POST /gp/buy/spc/handlers/display.html - place order (with --yes confirm)
//
// All requests carry the persisted Session's cookies and a recent Chrome
// User-Agent. Anti-bot is best-effort — if Amazon serves a Robot Check page
// the doctor command surfaces it.
package amazon

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
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/amazon/internal/auth"
	"github.com/mvanhorn/printing-press-library/library/commerce/amazon/internal/config"
)

const defaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"

// ErrAuthExpired indicates the cookies no longer authenticate; surfaced as ExitAuth.
var ErrAuthExpired = errors.New("amazon: session expired (run `amazon-pp-cli auth login`)")

// ErrRobotCheck indicates Amazon served a CAPTCHA / Robot Check page.
var ErrRobotCheck = errors.New("amazon: robot check encountered (open amazon.com in your browser and clear the challenge, then retry)")

// Client wraps net/http with the session cookies and a base URL.
type Client struct {
	HTTP    *http.Client
	Session *auth.Session
	Profile config.Profile

	// DryRun, when true, causes mutating calls (AddToCart, PlaceOrder) to
	// short-circuit without dispatching the request.
	DryRun bool
}

// CartLine is one line in /gp/cart/view.html.
type CartLine struct {
	ASIN       string `json:"asin"`
	Title      string `json:"title"`
	Quantity   int    `json:"quantity"`
	PriceCents int64  `json:"price_cents,omitempty"`
}

// New returns a Client wired to the given profile and session.
func New(profile config.Profile, sess *auth.Session) (*Client, error) {
	if profile.MarketplaceBaseURL == "" {
		profile.MarketplaceBaseURL = "https://www.amazon.com"
	}
	jar, err := sess.CookieJar(profile.MarketplaceBaseURL)
	if err != nil {
		return nil, err
	}
	c := &http.Client{
		Timeout: 30 * time.Second,
		Jar:     jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > 5 {
				return errors.New("too many redirects")
			}
			return nil
		},
	}
	return &Client{HTTP: c, Session: sess, Profile: profile}, nil
}

// baseURL returns the marketplace root.
func (c *Client) baseURL() string { return strings.TrimRight(c.Profile.MarketplaceBaseURL, "/") }

func (c *Client) newRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL()+path, body)
	if err != nil {
		return nil, err
	}
	ua := c.Session.UserAgent
	if ua == "" {
		ua = defaultUserAgent
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/json;q=0.9,*/*;q=0.8")
	return req, nil
}

func (c *Client) do(req *http.Request) (*http.Response, []byte, error) {
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20)) // 16 MiB cap
	if err != nil {
		return resp, nil, err
	}
	classified, err := classify(resp, body)
	if err != nil {
		return resp, body, err
	}
	if classified != nil {
		return resp, body, classified
	}
	return resp, body, nil
}

// classify converts known auth/anti-bot pages into ErrAuthExpired / ErrRobotCheck.
func classify(resp *http.Response, body []byte) (error, error) {
	if resp.StatusCode >= 500 {
		return nil, fmt.Errorf("amazon: HTTP %d", resp.StatusCode)
	}
	lower := strings.ToLower(string(body[:min(2048, len(body))]))
	// Auth walls redirect to /ap/signin; inline re-auth interstitials embed
	// the signin form action. A bare "/ap/signin" substring is NOT a signal
	// on its own — authenticated pages carry it in nav-link JS blobs.
	if resp.Request != nil && strings.Contains(resp.Request.URL.Path, "/ap/signin") {
		return ErrAuthExpired, nil
	}
	if strings.Contains(lower, `action="/ap/signin"`) {
		return ErrAuthExpired, nil
	}
	if strings.Contains(lower, "to discuss automated access to amazon data") || strings.Contains(lower, "robot check") || strings.Contains(lower, "captcha") {
		return ErrRobotCheck, nil
	}
	return nil, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Ping fetches the homepage and reports whether the session looks authenticated.
// Returns the detected account name (best-effort) for doctor.
func (c *Client) Ping(ctx context.Context) (string, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/gp/css/homepage.html", nil)
	if err != nil {
		return "", err
	}
	_, body, err := c.do(req)
	if err != nil {
		return "", err
	}
	return extractAccountName(string(body)), nil
}

// AddToCart posts to /gp/aws/cart/add.html. quantity defaults to 1.
//
// The endpoint accepts ASIN as the "ASIN.1" parameter and quantity as
// "Quantity.1". `offerListingId.1` is set to "1" for the default offer when
// we don't have a specific offer ID. Amazon redirects on success to the cart
// page; we follow the redirect and return the parsed cart line count for the
// added ASIN so callers can confirm.
func (c *Client) AddToCart(ctx context.Context, asin string, quantity int) (CartLine, error) {
	if asin == "" {
		return CartLine{}, errors.New("ASIN cannot be empty")
	}
	if quantity <= 0 {
		quantity = 1
	}
	if c.DryRun {
		return CartLine{ASIN: asin, Quantity: quantity, Title: "(dry-run, no network call)"}, nil
	}
	form := url.Values{}
	form.Set("ASIN.1", asin)
	form.Set("Quantity.1", strconv.Itoa(quantity))
	form.Set("offerListingId.1", "1")
	form.Set("clientName", "RetailWebsite")
	req, err := c.newRequest(ctx, http.MethodPost, "/gp/aws/cart/add.html", strings.NewReader(form.Encode()))
	if err != nil {
		return CartLine{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if _, _, err := c.do(req); err != nil {
		return CartLine{}, err
	}
	// Verify by re-reading the cart.
	cart, err := c.CartView(ctx)
	if err != nil {
		return CartLine{}, err
	}
	for _, line := range cart {
		if line.ASIN == asin {
			return line, nil
		}
	}
	return CartLine{ASIN: asin, Quantity: quantity, Title: "(added; not visible in cart-view scrape)"}, nil
}

// CartView GETs /gp/cart/view.html and parses the line items.
func (c *Client) CartView(ctx context.Context) ([]CartLine, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/gp/cart/view.html", nil)
	if err != nil {
		return nil, err
	}
	_, body, err := c.do(req)
	if err != nil {
		return nil, err
	}
	return parseCartHTML(string(body)), nil
}

// CheckoutResult is the payload returned from PlaceOrder.
type CheckoutResult struct {
	OrderID    string `json:"order_id,omitempty"`
	Confirmed  bool   `json:"confirmed"`
	StatusNote string `json:"status_note,omitempty"`
}

// PlaceOrder runs the two-step Amazon "Single Page Checkout" handler. The
// caller MUST pass confirm=true; this function refuses otherwise. A successful
// confirmation returns the new order ID parsed from the thank-you page.
//
// Note: this is intentionally a thin replay path. It assumes the user has a
// default shipping address and default payment method configured on amazon.com
// — Amazon's checkout page renders fine without per-field overrides in that
// case. Carts that contain items needing a per-item shipping decision (split
// shipments, gift wrap) are out of scope: PlaceOrder will return a
// StatusNote and Confirmed=false rather than guess.
func (c *Client) PlaceOrder(ctx context.Context, confirm bool) (CheckoutResult, error) {
	if !confirm {
		return CheckoutResult{}, errors.New("PlaceOrder requires explicit confirm=true; pass --yes on the CLI")
	}
	if c.DryRun {
		return CheckoutResult{Confirmed: false, StatusNote: "dry-run: would POST to /gp/buy/spc/handlers/display.html"}, nil
	}
	// Step 1: GET the SPC page to obtain the hidden form tokens.
	req, err := c.newRequest(ctx, http.MethodGet, "/gp/buy/spc/handlers/display.html", nil)
	if err != nil {
		return CheckoutResult{}, err
	}
	_, body, err := c.do(req)
	if err != nil {
		return CheckoutResult{}, err
	}
	tokens, err := parseSPCTokens(string(body))
	if err != nil {
		return CheckoutResult{Confirmed: false, StatusNote: err.Error()}, nil
	}
	// Step 2: POST the place-order action with the captured tokens.
	form := url.Values{}
	for k, v := range tokens {
		form.Set(k, v)
	}
	form.Set("placeYourOrder1", "1")
	req2, err := c.newRequest(ctx, http.MethodPost, "/gp/buy/spc/handlers/display.html", strings.NewReader(form.Encode()))
	if err != nil {
		return CheckoutResult{}, err
	}
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, body2, err := c.do(req2)
	if err != nil {
		return CheckoutResult{}, err
	}
	// The thank-you page redirects to /gp/buy/thankyou/handlers/display.html?orderId=...
	if resp.Request != nil {
		if order := resp.Request.URL.Query().Get("orderId"); order != "" {
			return CheckoutResult{OrderID: order, Confirmed: true}, nil
		}
	}
	if id := extractThankYouOrderID(string(body2)); id != "" {
		return CheckoutResult{OrderID: id, Confirmed: true}, nil
	}
	return CheckoutResult{Confirmed: false, StatusNote: "POSTed but could not parse an order ID from response"}, nil
}

// MarshalSession is a debug helper used by `doctor --json`.
func MarshalSession(s *auth.Session) (string, error) {
	b, err := json.Marshal(map[string]any{
		"cookie_count": len(s.Cookies),
		"has_marker":   s.HasMarker(),
		"saved_at":     s.SavedAt,
	})
	if err != nil {
		return "", err
	}
	return string(b), nil
}
