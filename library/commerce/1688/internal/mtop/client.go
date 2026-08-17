// Hand-authored signed mtop client for 1688.com. Not generator-emitted.
//
// 1688's PC offer search is served by the mtop JSON gateway at
// h5api.m.1688.com. Every call is md5-signed against a short-lived
// _m_h5_tk token cookie. The generated internal/client cannot produce this
// signature, so this package implements the token-bootstrap -> sign ->
// signed-call protocol directly. Verified live 2026-06-12.
package mtop

import (
	"context"
	"crypto/md5" // #nosec G501 -- 1688 mtop gateway mandates an md5-signed request; md5 is the API's required signature algorithm, not a security primitive
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/1688/internal/cliutil"
)

const (
	appKey     = "12574478"
	appID      = "32517"
	mtopBase   = "https://h5api.m.1688.com/h5/mtop.relationrecommend.wirelessrecommend.recommend/2.0/"
	apiName    = "mtop.relationrecommend.WirelessRecommend.recommend"
	userAgent  = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36"
	tokenTTL   = 80 * time.Minute // server Max-Age is 5400s; refresh a little early
	maxRetries = 3
)

var fontTagRE = regexp.MustCompile(`<[^>]+>`)

// Offer is the normalized, storable shape of a 1688 search result card.
type Offer struct {
	OfferID            string   `json:"offer_id"`
	Title              string   `json:"title"`
	PriceCNY           float64  `json:"price_cny"`
	PriceLabel         string   `json:"price_label,omitempty"`
	MOQ                string   `json:"moq,omitempty"`
	SupplierName       string   `json:"supplier_name"`
	SupplierMemberID   string   `json:"supplier_member_id"`
	SupplierLoginID    string   `json:"supplier_login_id,omitempty"`
	Province           string   `json:"province,omitempty"`
	City               string   `json:"city,omitempty"`
	SupplierLocation   string   `json:"supplier_location,omitempty"`
	TransactionCount   int      `json:"transaction_count"`
	SalesVolume        string   `json:"sales_volume,omitempty"`
	RepurchaseRate     string   `json:"repurchase_rate,omitempty"`
	RepurchasePct      float64  `json:"repurchase_pct"`
	FactoryInspection  bool     `json:"factory_inspection"`
	SuperFactory       bool     `json:"super_factory"`
	BusinessInspection bool     `json:"business_inspection"`
	VerifiedFactory    bool     `json:"verified_factory"`
	ServiceTags        []string `json:"service_tags,omitempty"`
	TradeComposite     float64  `json:"trade_score_composite"`
	TradeLogistics     float64  `json:"trade_score_logistics"`
	TradeDispute       float64  `json:"trade_score_dispute"`
	TradeConsultation  float64  `json:"trade_score_consultation"`
	ShopURL            string   `json:"shop_url,omitempty"`
	Image              string   `json:"image,omitempty"`
	IsAd               bool     `json:"is_ad"`
	URL                string   `json:"url"`
	Keyword            string   `json:"keyword,omitempty"`
	SyncedAt           string   `json:"synced_at,omitempty"`
}

// SearchParams are the user-facing search inputs.
type SearchParams struct {
	Keyword  string
	PriceMin string
	PriceMax string
	Province string
	Sort     string // price-asc | price-desc | booked | newest | ""
	Page     int
	PageSize int
}

// SearchResult is one page of normalized offers plus the reported total.
type SearchResult struct {
	Keyword string  `json:"keyword"`
	Total   int     `json:"total_results"`
	Page    int     `json:"page"`
	HasMore bool    `json:"has_more"`
	Offers  []Offer `json:"offers"`
}

// Client holds a cookie jar (for the _m_h5_tk token) and an adaptive limiter.
type Client struct {
	http    *http.Client
	limiter *cliutil.AdaptiveLimiter
	tokenAt time.Time
}

// New returns a client with a cookie jar. ratePerSec <= 0 disables limiting.
func New(timeout time.Duration, ratePerSec float64) *Client {
	jar, _ := cookiejar.New(nil)
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &Client{
		http:    &http.Client{Timeout: timeout, Jar: jar},
		limiter: cliutil.NewAdaptiveLimiter(ratePerSec),
	}
}

func sortType(sort string) string {
	switch strings.ToLower(strings.TrimSpace(sort)) {
	case "price-asc":
		return "price-asc"
	case "price-desc":
		return "price-desc"
	case "booked", "transactions", "sales":
		return "booked"
	case "newest", "new", "newoffer":
		return "newOffer"
	default:
		return ""
	}
}

// bootstrap performs the unsigned call that makes the gateway issue the
// _m_h5_tk token cookie. The body returns FAIL_SYS_TOKEN_EMPTY; we only want
// the Set-Cookie.
func (c *Client) bootstrap(ctx context.Context) error {
	u := fmt.Sprintf("%s?jsv=2.5.1&appKey=%s&t=%d&sign=x&api=%s&v=2.0&data=%%7B%%7D",
		mtopBase, appKey, time.Now().UnixMilli(), apiName)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	c.setHeaders(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body) // drain so the connection can be reused; body content is unused during token bootstrap
	if c.token() == "" {
		return fmt.Errorf("token bootstrap failed: no _m_h5_tk cookie issued")
	}
	c.tokenAt = time.Now()
	return nil
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Referer", "https://www.1688.com/")
	req.Header.Set("Origin", "https://www.1688.com")
	req.Header.Set("Accept", "application/json, text/plain, */*")
}

// token returns the part of _m_h5_tk before the underscore, or "".
func (c *Client) token() string {
	u, _ := url.Parse(mtopBase)
	for _, ck := range c.http.Jar.Cookies(u) {
		if ck.Name == "_m_h5_tk" {
			if i := strings.IndexByte(ck.Value, '_'); i > 0 {
				return ck.Value[:i]
			}
			return ck.Value
		}
	}
	return ""
}

func (c *Client) ensureToken(ctx context.Context) error {
	if c.token() != "" && time.Since(c.tokenAt) < tokenTTL {
		return nil
	}
	return c.bootstrap(ctx)
}

// Search runs one page of signed offer search and returns normalized offers.
func (c *Client) Search(ctx context.Context, p SearchParams) (*SearchResult, error) {
	if strings.TrimSpace(p.Keyword) == "" {
		return nil, fmt.Errorf("keyword is required")
	}
	if p.Page < 1 {
		p.Page = 1
	}
	if p.PageSize < 1 {
		p.PageSize = 20
	}
	if p.PageSize > 60 {
		p.PageSize = 60
	}

	params := map[string]any{
		"keywords":            p.Keyword,
		"beginPage":           p.Page,
		"pageSize":            p.PageSize,
		"method":              "getOfferList",
		"verticalProductFlag": "pcmarket",
		"searchScene":         "pcOfferSearch",
		"charset":             "GBK",
	}
	if p.PriceMin != "" {
		params["priceStart"] = p.PriceMin
	}
	if p.PriceMax != "" {
		params["priceEnd"] = p.PriceMax
	}
	if p.Province != "" {
		params["province"] = p.Province
	}
	if st := sortType(p.Sort); st != "" {
		params["sortType"] = st
	}

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if c.limiter != nil {
			c.limiter.Wait()
		}
		if err := c.ensureToken(ctx); err != nil {
			lastErr = err
			continue
		}
		res, retryToken, rateLimited, err := c.doSearch(ctx, params, p)
		if rateLimited {
			if c.limiter != nil {
				c.limiter.OnRateLimit()
			}
			lastErr = &cliutil.RateLimitError{URL: mtopBase, Body: fmt.Sprintf("1688 mtop throttled after %d attempts", attempt+1)}
			time.Sleep(cliutil.Backoff(attempt))
			continue
		}
		if retryToken {
			// token expired mid-flight; force re-bootstrap and retry
			c.tokenAt = time.Time{}
			lastErr = err
			continue
		}
		if err != nil {
			lastErr = err
			continue
		}
		if c.limiter != nil {
			c.limiter.OnSuccess()
		}
		return res, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("search failed after %d attempts", maxRetries)
	}
	return nil, lastErr
}

func (c *Client) doSearch(ctx context.Context, params map[string]any, p SearchParams) (res *SearchResult, retryToken, rateLimited bool, err error) {
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, false, false, err
	}
	data, err := json.Marshal(map[string]string{"appId": appID, "params": string(paramsJSON)})
	if err != nil {
		return nil, false, false, err
	}
	t := time.Now().UnixMilli()
	// #nosec G401 -- mtop's sign param is md5(token&t&appKey&data) by protocol; this is a wire-format signature, not a security hash
	sum := md5.Sum([]byte(fmt.Sprintf("%s&%d&%s&%s", c.token(), t, appKey, string(data))))
	sign := hex.EncodeToString(sum[:])

	u := fmt.Sprintf("%s?jsv=2.5.1&appKey=%s&t=%d&sign=%s&api=%s&v=2.0&data=%s",
		mtopBase, appKey, t, sign, apiName, url.QueryEscape(string(data)))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, false, false, err
	}
	c.setHeaders(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, false, false, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, false, false, err
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, false, true, nil
	}
	return c.parse(body, p)
}

func (c *Client) parse(body []byte, p SearchParams) (*SearchResult, bool, bool, error) {
	var raw struct {
		Ret  []string `json:"ret"`
		Data struct {
			Data struct {
				OFFER struct {
					Found   string `json:"found"`
					HasMore string `json:"hasMore"`
					Items   []struct {
						CellType string          `json:"cellType"`
						Data     json.RawMessage `json:"data"`
					} `json:"items"`
				} `json:"OFFER"`
			} `json:"data"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, false, false, fmt.Errorf("decoding mtop response: %w", err)
	}
	ret := ""
	if len(raw.Ret) > 0 {
		ret = raw.Ret[0]
	}
	switch {
	case strings.HasPrefix(ret, "SUCCESS"):
		// fall through to parse
	case strings.Contains(ret, "TOKEN_EMPTY"), strings.Contains(ret, "TOKEN_EXPIRED"), strings.Contains(ret, "ILLEGAL_ACCESS"):
		return nil, true, false, fmt.Errorf("mtop token error: %s", ret)
	case strings.Contains(ret, "FLOWLIMIT"), strings.Contains(ret, "FREQUENCY"):
		return nil, false, true, nil
	default:
		return nil, false, false, fmt.Errorf("mtop returned %q", ret)
	}

	result := &SearchResult{
		Keyword: p.Keyword,
		Page:    p.Page,
		HasMore: raw.Data.Data.OFFER.HasMore == "true",
	}
	if n, err := strconv.Atoi(raw.Data.Data.OFFER.Found); err == nil {
		result.Total = n
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, it := range raw.Data.Data.OFFER.Items {
		if it.CellType != "smart_ui_offer" || len(it.Data) == 0 {
			continue
		}
		o, ok := decodeOffer(it.Data)
		if !ok {
			continue
		}
		o.Keyword = p.Keyword
		o.SyncedAt = now
		result.Offers = append(result.Offers, o)
	}
	return result, false, false, nil
}

func decodeOffer(data json.RawMessage) (Offer, bool) {
	var card struct {
		OfferID   string `json:"offerId"`
		Title     string `json:"title"`
		Subject   string `json:"subject"`
		PriceInfo struct {
			Price            string `json:"price"`
			PriceDescription string `json:"priceDescription"`
		} `json:"priceInfo"`
		AfterPrice struct {
			Text string `json:"text"`
		} `json:"afterPrice"`
		BookedCount         string `json:"bookedCount"`
		OfferRepurchaseRate string `json:"offerRepurchaseRate"`
		FactoryInspection   string `json:"factoryInspection"`
		SuperFactory        string `json:"superFactory"`
		BusinessInspection  string `json:"businessInspection"`
		IsP4P               string `json:"isP4P"`
		Province            string `json:"province"`
		City                string `json:"city"`
		MemberID            string `json:"memberId"`
		QuantityBegin       string `json:"quantityBegin"`
		OfferTags           struct {
			ServiceTags []string `json:"serviceTags"`
		} `json:"offerTags"`
		Shop struct {
			Text    string `json:"text"`
			LoginID string `json:"loginIdOfUtf8"`
		} `json:"shop"`
		ShopAddition struct {
			ShopLinkURL  string `json:"shopLinkUrl"`
			TradeService struct {
				CompositeNewScore string `json:"compositeNewScore"`
				LogisticsScore    string `json:"logisticsScore"`
				DisputeScore      string `json:"disputeScore"`
				ConsultationScore string `json:"consultationScore"`
			} `json:"tradeService"`
		} `json:"shopAddition"`
		List struct {
			Cover struct {
				Pic string `json:"pic"`
			} `json:"cover"`
		} `json:"list"`
	}
	if err := json.Unmarshal(data, &card); err != nil {
		return Offer{}, false
	}
	if card.OfferID == "" {
		return Offer{}, false
	}
	title := card.Title
	if title == "" {
		title = card.Subject
	}
	loginID := card.Shop.LoginID
	if dec, derr := url.QueryUnescape(loginID); derr == nil && dec != "" {
		loginID = dec
	}
	o := Offer{
		OfferID:            card.OfferID,
		Title:              cliutil.CleanText(fontTagRE.ReplaceAllString(title, "")),
		PriceCNY:           parseFloat(card.PriceInfo.Price),
		PriceLabel:         card.PriceInfo.PriceDescription,
		MOQ:                card.QuantityBegin,
		SupplierName:       cliutil.CleanText(card.Shop.Text),
		SupplierMemberID:   card.MemberID,
		SupplierLoginID:    loginID,
		Province:           card.Province,
		City:               card.City,
		TransactionCount:   parseInt(card.BookedCount),
		SalesVolume:        cliutil.CleanText(card.AfterPrice.Text),
		RepurchaseRate:     card.OfferRepurchaseRate,
		RepurchasePct:      parsePercent(card.OfferRepurchaseRate),
		FactoryInspection:  card.FactoryInspection == "true",
		SuperFactory:       card.SuperFactory == "true",
		BusinessInspection: card.BusinessInspection == "true",
		ServiceTags:        card.OfferTags.ServiceTags,
		TradeComposite:     parseFloat(card.ShopAddition.TradeService.CompositeNewScore),
		TradeLogistics:     parseFloat(card.ShopAddition.TradeService.LogisticsScore),
		TradeDispute:       parseFloat(card.ShopAddition.TradeService.DisputeScore),
		TradeConsultation:  parseFloat(card.ShopAddition.TradeService.ConsultationScore),
		ShopURL:            card.ShopAddition.ShopLinkURL,
		Image:              firstImage(card.List.Cover.Pic),
		IsAd:               card.IsP4P == "true",
		URL:                fmt.Sprintf("https://detail.1688.com/offer/%s.html", card.OfferID),
	}
	if card.Province != "" || card.City != "" {
		o.SupplierLocation = strings.TrimSpace(card.Province + " " + card.City)
	}
	o.VerifiedFactory = o.FactoryInspection || o.SuperFactory || hasServiceTag(o.ServiceTags, "深度验厂")
	return o, true
}

func hasServiceTag(tags []string, want string) bool {
	for _, t := range tags {
		if strings.Contains(t, want) {
			return true
		}
	}
	return false
}

func firstImage(pic string) string {
	if pic == "" {
		return ""
	}
	if i := strings.IndexByte(pic, ','); i > 0 {
		return pic[:i]
	}
	return pic
}

func parseFloat(s string) float64 {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return f
}

func parseInt(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	return n
}

// parsePercent turns "11%" into 11.0.
func parsePercent(s string) float64 {
	return parseFloat(strings.TrimSuffix(strings.TrimSpace(s), "%"))
}
