// Copyright 2026 cathryn-lavery. Licensed under Apache-2.0. See LICENSE.
// TikTok Shop client code only encodes endpoints confirmed in official Partner Center docs.

package client

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/tiktok-shop/internal/config"
)

const (
	SellerAPIDocsURL         = "https://partner.tiktokshop.com/docv2/page/650b1f2ff1fd3102b93c6d3d"
	AuthorizationOverviewURL = "https://partner.tiktokshop.com/docv2/page/678e3a3292b0f40314a92d75"
	AuthorizationGuideURL    = "https://partner.tiktokshop.com/docv2/page/678e3a2dbd083702fd17455c"
	SigningDocsURL           = "https://partner.tiktokshop.com/docv2/page/678e3a3d4ddec3030b238faf"
	AuthorizedShopsDocsURL   = "https://partner.tiktokshop.com/docv2/page/6507ead7b99d5302be949ba9"
	ActiveShopsDocsURL       = "https://partner.tiktokshop.com/docv2/page/650a69e24a0bb702c067291c"
	OrderListDocsURL         = "https://partner.tiktokshop.com/docv2/page/650aa8094a0bb702c06df242"
	OrderDetailDocsURL       = "https://partner.tiktokshop.com/docv2/page/650aa8ccc16ffe02b8f167a0"
	ProductSearchDocsURL     = "https://partner.tiktokshop.com/docv2/page/6503081a56e2bb0289dd6d7d"
	ProductDetailDocsURL     = "https://partner.tiktokshop.com/docv2/page/6509d85b4a0bb702c057fdda"
	InventorySearchDocsURL   = "https://partner.tiktokshop.com/docv2/page/650a9191c16ffe02b8eec161"
	InventoryUpdateDocsURL   = "https://partner.tiktokshop.com/docv2/page/6503068fc20ad60284b38858"
	WarehouseListDocsURL     = "https://partner.tiktokshop.com/docv2/page/650aa418defece02be6e66b6"
	PackageSearchDocsURL     = "https://partner.tiktokshop.com/docv2/page/650aa592bace3e02b75db748"
	PackageDetailDocsURL     = "https://partner.tiktokshop.com/docv2/page/650aa39fbace3e02b75d8617"
	ReturnsSearchDocsURL     = "https://partner.tiktokshop.com/docv2/page/69c3070c441217049711fdea"
	DefaultOpenAPIBaseURL    = "https://open-api.tiktokglobalshop.com"
	DefaultAuthBaseURL       = "https://auth.tiktok-shops.com"
	AccessTokenHeader        = "x-tts-access-token"
	ContentTypeJSON          = "application/json"
	maxRetryWait             = 60 * time.Second
)

// PartnerAPIDocsURL is retained for older scaffold call sites.
const PartnerAPIDocsURL = SellerAPIDocsURL

type Client struct {
	Config     *config.Config
	HTTPClient *http.Client
	DryRun     bool
}

type PendingConfirmationError struct {
	Operation string
	DocURL    string
}

func (e *PendingConfirmationError) Error() string {
	return fmt.Sprintf("%s not yet implemented; awaiting API confirmation from %s", e.Operation, e.DocURL)
}

type AuthRequiredError struct {
	Missing []string
}

func (e *AuthRequiredError) Error() string {
	return "missing TikTok Shop auth configuration: " + strings.Join(e.Missing, ", ")
}

type APIError struct {
	StatusCode int
	Code       int             `json:"code,omitempty"`
	Message    string          `json:"message,omitempty"`
	RequestID  string          `json:"request_id,omitempty"`
	Body       json.RawMessage `json:"body,omitempty"`
}

func (e *APIError) Error() string {
	parts := []string{fmt.Sprintf("TikTok Shop API error HTTP %d", e.StatusCode)}
	if e.Code != 0 {
		parts = append(parts, fmt.Sprintf("code %d", e.Code))
	}
	if e.Message != "" {
		parts = append(parts, e.Message)
	}
	if e.RequestID != "" {
		parts = append(parts, "request_id "+e.RequestID)
	}
	return strings.Join(parts, ": ")
}

type RateLimitError struct {
	RetryAfter time.Duration
	Err        *APIError
}

func (e *RateLimitError) Error() string {
	if e.RetryAfter > 0 {
		return fmt.Sprintf("rate limited by TikTok Shop; retry after %s", e.RetryAfter.Round(time.Second))
	}
	return "rate limited by TikTok Shop"
}

func New(cfg *config.Config, timeout time.Duration) *Client {
	return &Client{
		Config:     cfg,
		HTTPClient: &http.Client{Timeout: timeout},
	}
}

func (c *Client) ValidateTokenPath() error {
	return c.RequireOpenAPIAuth(false)
}

func (c *Client) Placeholder(operation string) error {
	operation = strings.TrimSpace(operation)
	if operation == "" {
		operation = "TikTok Shop API operation"
	}
	return &PendingConfirmationError{Operation: operation, DocURL: SellerAPIDocsURL}
}

func (c *Client) RequireOpenAPIAuth(requireShopCipher bool) error {
	missing := []string{}
	if c.Config.AppKey == "" {
		missing = append(missing, config.EnvAppKey)
	}
	if c.Config.AppSecret == "" {
		missing = append(missing, config.EnvAppSecret)
	}
	if c.Config.AccessToken == "" {
		missing = append(missing, config.EnvAccessToken)
	}
	if requireShopCipher && c.Config.ShopCipher == "" {
		missing = append(missing, config.EnvShopCipher)
	}
	if len(missing) > 0 {
		return &AuthRequiredError{Missing: missing}
	}
	return nil
}

func (c *Client) RequireTokenRefreshAuth() error {
	missing := []string{}
	if c.Config.AppKey == "" {
		missing = append(missing, config.EnvAppKey)
	}
	if c.Config.AppSecret == "" {
		missing = append(missing, config.EnvAppSecret)
	}
	if c.Config.RefreshToken == "" {
		missing = append(missing, config.EnvRefreshToken)
	}
	if len(missing) > 0 {
		return &AuthRequiredError{Missing: missing}
	}
	return nil
}

func (c *Client) GetAccessToken(ctx context.Context, authCode string) (json.RawMessage, error) {
	if strings.TrimSpace(authCode) == "" {
		return nil, errors.New("auth code is required; see " + AuthorizationOverviewURL)
	}
	if c.Config.AppKey == "" || c.Config.AppSecret == "" {
		return nil, &AuthRequiredError{Missing: []string{config.EnvAppKey, config.EnvAppSecret}}
	}
	q := url.Values{}
	q.Set("app_key", c.Config.AppKey)
	q.Set("app_secret", c.Config.AppSecret)
	q.Set("auth_code", authCode)
	q.Set("grant_type", "authorized_code")
	return c.doAuthGET(ctx, "/api/v2/token/get", q)
}

func (c *Client) RefreshToken(ctx context.Context) (json.RawMessage, error) {
	if err := c.RequireTokenRefreshAuth(); err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Set("app_key", c.Config.AppKey)
	q.Set("app_secret", c.Config.AppSecret)
	q.Set("refresh_token", c.Config.RefreshToken)
	q.Set("grant_type", "refresh_token")
	return c.doAuthGET(ctx, "/api/v2/token/refresh", q)
}

func (c *Client) AuthorizedShops(ctx context.Context) (json.RawMessage, error) {
	if err := c.RequireOpenAPIAuth(false); err != nil {
		return nil, err
	}
	return c.DoOpenAPI(ctx, http.MethodGet, "/authorization/202309/shops", nil, nil)
}

func (c *Client) ActiveShops(ctx context.Context) (json.RawMessage, error) {
	if err := c.RequireOpenAPIAuth(false); err != nil {
		return nil, err
	}
	return c.DoOpenAPI(ctx, http.MethodGet, "/seller/202309/shops", nil, nil)
}

func (c *Client) DoOpenAPI(ctx context.Context, method, path string, query url.Values, body any) (json.RawMessage, error) {
	if err := c.RequireOpenAPIAuth(strings.Contains(path, "/order/") || strings.Contains(path, "/product/") || strings.Contains(path, "/logistics/") || strings.Contains(path, "/fulfillment/")); err != nil {
		return nil, err
	}
	if query == nil {
		query = url.Values{}
	}
	query.Set("app_key", c.Config.AppKey)
	query.Set("timestamp", fmt.Sprintf("%d", time.Now().UTC().Unix()))
	if c.Config.ShopCipher != "" && query.Get("shop_cipher") == "" {
		query.Set("shop_cipher", c.Config.ShopCipher)
	}

	bodyBytes, err := encodeBody(body)
	if err != nil {
		return nil, err
	}

	baseURL := valueOr(c.Config.BaseURL, DefaultOpenAPIBaseURL)
	u, err := joinURL(baseURL, path)
	if err != nil {
		return nil, err
	}
	u.RawQuery = query.Encode()
	query.Set("sign", Sign(path, query, bodyBytes, c.Config.AppSecret, ContentTypeJSON))
	u.RawQuery = query.Encode()

	if c.DryRun {
		return dryRunResponse(method, path, query, bodyBytes, true), nil
	}

	var reader io.Reader
	if bodyBytes != nil {
		reader = bytes.NewReader(bodyBytes)
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("content-type", ContentTypeJSON)
	req.Header.Set(AccessTokenHeader, c.Config.AccessToken)
	return c.do(req)
}

func (c *Client) doAuthGET(ctx context.Context, path string, query url.Values) (json.RawMessage, error) {
	baseURL := valueOr(c.Config.AuthBaseURL, DefaultAuthBaseURL)
	u, err := joinURL(baseURL, path)
	if err != nil {
		return nil, err
	}
	u.RawQuery = query.Encode()
	if c.DryRun {
		return dryRunResponse(http.MethodGet, path, query, nil, false), nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	return c.do(req)
}

func (c *Client) do(req *http.Request) (json.RawMessage, error) {
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		apiErr := decodeAPIError(resp.StatusCode, data)
		return nil, &RateLimitError{RetryAfter: retryAfter(resp), Err: apiErr}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, decodeAPIError(resp.StatusCode, data)
	}
	apiErr := decodeAPIError(resp.StatusCode, data)
	if apiErr.Code != 0 {
		return nil, apiErr
	}
	return json.RawMessage(data), nil
}

func Sign(path string, query url.Values, body []byte, secret, contentType string) string {
	keys := make([]string, 0, len(query))
	for k := range query {
		if k == "sign" || k == "access_token" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	b.WriteString(path)
	for _, k := range keys {
		b.WriteString(k)
		b.WriteString(query.Get(k))
	}
	if !strings.EqualFold(contentType, "multipart/form-data") && len(body) > 0 {
		b.Write(body)
	}
	payload := secret + b.String() + secret
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

func encodeBody(body any) ([]byte, error) {
	if body == nil {
		return nil, nil
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encoding JSON body: %w", err)
	}
	return data, nil
}

func decodeAPIError(status int, data []byte) *APIError {
	apiErr := &APIError{StatusCode: status, Body: json.RawMessage(data)}
	var envelope struct {
		Code      int    `json:"code"`
		Message   string `json:"message"`
		RequestID string `json:"request_id"`
	}
	if err := json.Unmarshal(data, &envelope); err == nil {
		apiErr.Code = envelope.Code
		apiErr.Message = envelope.Message
		apiErr.RequestID = envelope.RequestID
	}
	return apiErr
}

func retryAfter(resp *http.Response) time.Duration {
	header := resp.Header.Get("Retry-After")
	if header == "" {
		return 0
	}
	if seconds, err := time.ParseDuration(header + "s"); err == nil {
		if seconds > maxRetryWait {
			return maxRetryWait
		}
		return seconds
	}
	if when, err := http.ParseTime(header); err == nil {
		wait := time.Until(when)
		if wait < 0 {
			return 0
		}
		if wait > maxRetryWait {
			return maxRetryWait
		}
		return wait
	}
	return 0
}

func dryRunResponse(method, path string, query url.Values, body []byte, signed bool) json.RawMessage {
	redactedQuery := map[string]string{}
	for k := range query {
		switch k {
		case "app_key", "app_secret", "auth_code", "refresh_token", "shop_cipher", "access_token", "sign":
			redactedQuery[k] = "[redacted]"
		default:
			redactedQuery[k] = query.Get(k)
		}
	}
	out := map[string]any{
		"dry_run": true,
		"method":  method,
		"path":    path,
		"query":   redactedQuery,
		"signed":  signed,
	}
	if len(body) > 0 {
		out["body"] = json.RawMessage(body)
	}
	data, _ := json.Marshal(out)
	return data
}

func joinURL(base, path string) (*url.URL, error) {
	base = strings.TrimRight(base, "/")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return url.Parse(base + path)
}

func valueOr(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}
