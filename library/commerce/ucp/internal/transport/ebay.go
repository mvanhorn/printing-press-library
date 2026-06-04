package transport

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/ucp/internal/ucp"
)

const (
	ebayOAuthURL  = "https://api.ebay.com/identity/v1/oauth2/token"
	ebayBrowseURL = "https://api.ebay.com/buy/browse/v1/item_summary/search"
)

// ebayTokenCache holds the most recently fetched client-credentials token.
// Single-process cache — fine for a CLI; the token TTL is typically 2 hours.
var (
	ebayTokenCache struct {
		sync.Mutex
		token string
		exp   time.Time
	}
)

// EbaySearch queries the eBay Browse API for item summaries.
// Requires EBAY_APP_ID + EBAY_CERT_ID env vars (OAuth client-credentials flow).
func EbaySearch(ctx context.Context, query string, limit int) ([]ucp.SearchHit, error) {
	appID := os.Getenv("EBAY_APP_ID")
	certID := os.Getenv("EBAY_CERT_ID")
	if appID == "" || certID == "" {
		return nil, fmt.Errorf("eBay adapter requires EBAY_APP_ID + EBAY_CERT_ID env vars — get them at https://developer.ebay.com/my/keys")
	}
	if limit <= 0 {
		limit = 10
	}
	if limit > 200 {
		limit = 200
	}

	token, err := getEbayToken(ctx, appID, certID)
	if err != nil {
		return nil, err
	}

	u, _ := url.Parse(ebayBrowseURL)
	q := u.Query()
	q.Set("q", query)
	q.Set("limit", strconv.Itoa(limit))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-EBAY-C-MARKETPLACE-ID", "EBAY_US")
	req.Header.Set("User-Agent", "ucp-pp-cli/1.3")
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", u.String(), err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("eBay Browse API returned HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}

	var parsed struct {
		ItemSummaries []struct {
			ItemID  string `json:"itemId"`
			Title   string `json:"title"`
			ItemURL string `json:"itemWebUrl"`
			Price   struct {
				Value    string `json:"value"`
				Currency string `json:"currency"`
			} `json:"price"`
		} `json:"itemSummaries"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("parse eBay response: %w", err)
	}
	hits := make([]ucp.SearchHit, 0, len(parsed.ItemSummaries))
	for _, r := range parsed.ItemSummaries {
		hit := ucp.SearchHit{
			Merchant: "ebay.com",
			Title:    r.Title,
			URL:      r.ItemURL,
			SKU:      r.ItemID,
			Currency: r.Price.Currency,
		}
		if f, err := strconv.ParseFloat(r.Price.Value, 64); err == nil {
			hit.Price = int(f * 100)
		}
		if hit.Currency == "" {
			hit.Currency = "USD"
		}
		hits = append(hits, hit)
	}
	return hits, nil
}

// getEbayToken fetches (or returns cached) OAuth client-credentials token.
// Token is process-cached for 90% of its TTL.
func getEbayToken(ctx context.Context, appID, certID string) (string, error) {
	ebayTokenCache.Lock()
	if ebayTokenCache.token != "" && time.Now().Before(ebayTokenCache.exp) {
		t := ebayTokenCache.token
		ebayTokenCache.Unlock()
		return t, nil
	}
	ebayTokenCache.Unlock()

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("scope", "https://api.ebay.com/oauth/api_scope")
	auth := base64.StdEncoding.EncodeToString([]byte(appID + ":" + certID))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ebayOAuthURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("eBay OAuth token request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 32<<10))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("eBay OAuth returned HTTP %d — EBAY_APP_ID/EBAY_CERT_ID may be invalid: %s", resp.StatusCode, truncate(string(body), 200))
	}
	var tk struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &tk); err != nil {
		return "", fmt.Errorf("parse eBay OAuth response: %w", err)
	}
	ebayTokenCache.Lock()
	ebayTokenCache.token = tk.AccessToken
	ebayTokenCache.exp = time.Now().Add(time.Duration(tk.ExpiresIn) * time.Second * 9 / 10)
	ebayTokenCache.Unlock()
	return tk.AccessToken, nil
}
