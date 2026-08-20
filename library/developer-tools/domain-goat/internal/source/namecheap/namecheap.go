// Package namecheap is a thin Namecheap API client for availability and pricing.
// Auth model: API user + API key + whitelisted client IP. See
// https://www.namecheap.com/support/api/intro/ — keys must be enabled and IP
// must be whitelisted in the Namecheap dashboard.
//
// Credential exposure note: Namecheap accepts auth ONLY as URL query parameters
// (no header-based auth path exists). callAPI therefore appends ApiUser, ApiKey,
// UserName, and ClientIp to the request URL, which means those values land in
// Namecheap's server-side access logs and any intermediate proxy logs. There is
// no in-code mitigation — if an unexpected exposure is suspected, rotate the
// API key in the Namecheap dashboard.
package namecheap

// PATCH(namecheap-credentials-doc): doc-only — see package doc above. Namecheap accepts auth ONLY as URL query params (no header path), so credentials land in server-side access logs and any intermediate proxy logs; dashboard key rotation is the only mitigation for suspected exposure.

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/domain-goat/internal/cliutil"
)

// Endpoint is the production Namecheap API URL.
const Endpoint = "https://api.namecheap.com/xml.response"

// SandboxEndpoint is the sandbox URL for testing.
const SandboxEndpoint = "https://api.sandbox.namecheap.com/xml.response"

// Env var names. NAMECHEAP_USERNAME is the canonical name; NAMECHEAP_API_USER
// is accepted as a legacy alias.
const (
	EnvAPIUser    = "NAMECHEAP_USERNAME"
	EnvAPIUserAlt = "NAMECHEAP_API_USER"
	EnvAPIKey     = "NAMECHEAP_API_KEY"
	EnvClientIP   = "NAMECHEAP_CLIENT_IP"
)

// limiter paces outbound Namecheap requests. Namecheap allows 20 req/sec,
// 700/min, 8000/hour — start at a comfortable 5/sec floor.
var limiter = cliutil.NewAdaptiveLimiter(5.0)

// Creds holds the auth context required by every Namecheap API call.
type Creds struct {
	APIUser  string
	APIKey   string
	ClientIP string
}

// FromEnv loads creds from environment variables, accepting either
// NAMECHEAP_USERNAME (canonical) or NAMECHEAP_API_USER (legacy alias).
func FromEnv(get func(string) string) Creds {
	user := strings.TrimSpace(get(EnvAPIUser))
	if user == "" {
		user = strings.TrimSpace(get(EnvAPIUserAlt))
	}
	return Creds{
		APIUser:  user,
		APIKey:   strings.TrimSpace(get(EnvAPIKey)),
		ClientIP: strings.TrimSpace(get(EnvClientIP)),
	}
}

// Validate returns an error describing any missing required field.
func (c Creds) Validate() error {
	missing := []string{}
	if c.APIUser == "" {
		missing = append(missing, EnvAPIUser)
	}
	if c.APIKey == "" {
		missing = append(missing, EnvAPIKey)
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing namecheap creds: %s — set these env vars (NAMECHEAP_CLIENT_IP is optional; will auto-detect)", strings.Join(missing, ", "))
	}
	return nil
}

// commonParams builds the auth+client query string for every Namecheap call.
func (c Creds) commonParams(command string) url.Values {
	ip := c.ClientIP
	if ip == "" {
		ip = detectClientIP()
	}
	v := url.Values{}
	v.Set("ApiUser", c.APIUser)
	v.Set("ApiKey", c.APIKey)
	v.Set("UserName", c.APIUser)
	v.Set("ClientIp", ip)
	v.Set("Command", command)
	return v
}

// detectClientIP returns the local outbound interface IP. Namecheap requires
// the request IP to match a whitelisted entry; falling back to 127.0.0.1 will
// not work but produces a clear API error message instead of a hang.
func detectClientIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "127.0.0.1"
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP.String()
}

// apiResponse mirrors Namecheap's XML envelope.
type apiResponse struct {
	XMLName xml.Name   `xml:"ApiResponse"`
	Status  string     `xml:"Status,attr"`
	Errors  apiErrors  `xml:"Errors"`
	Command apiCommand `xml:"CommandResponse"`
}

type apiErrors struct {
	Errors []apiError `xml:"Error"`
}

type apiError struct {
	Number int    `xml:"Number,attr"`
	Text   string `xml:",chardata"`
}

type apiCommand struct {
	Type        string         `xml:"Type,attr"`
	CheckResult []checkResult  `xml:"DomainCheckResult"`
	Pricing     []categoryNode `xml:"UserGetPricingResult>ProductType>ProductCategory"`
}

type checkResult struct {
	Domain                   string `xml:"Domain,attr"`
	Available                bool   `xml:"Available,attr"`
	IsPremiumName            bool   `xml:"IsPremiumName,attr"`
	PremiumRegistrationPrice string `xml:"PremiumRegistrationPrice,attr"`
	PremiumRenewalPrice      string `xml:"PremiumRenewalPrice,attr"`
}

type categoryNode struct {
	Name     string        `xml:"Name,attr"`
	Products []productNode `xml:"Product"`
}

type productNode struct {
	Name   string      `xml:"Name,attr"`
	Prices []priceNode `xml:"Price"`
}

type priceNode struct {
	Duration     int    `xml:"Duration,attr"`
	DurationType string `xml:"DurationType,attr"`
	Price        string `xml:"Price,attr"`
	YourPrice    string `xml:"YourPrice,attr"`
	RegularPrice string `xml:"RegularPrice,attr"`
	Currency     string `xml:"Currency,attr"`
}

// AvailabilityResult is the per-domain availability response.
type AvailabilityResult struct {
	FQDN              string  `json:"fqdn"`
	Available         bool    `json:"available"`
	Premium           bool    `json:"premium"`
	PremiumRegPrice   float64 `json:"premium_registration_price,omitempty"`
	PremiumRenewPrice float64 `json:"premium_renewal_price,omitempty"`
}

// CheckAvailability runs `namecheap.domains.check` for up to 50 domains.
func CheckAvailability(ctx context.Context, creds Creds, fqdns []string) ([]AvailabilityResult, error) {
	if err := creds.Validate(); err != nil {
		return nil, err
	}
	if len(fqdns) == 0 {
		return nil, nil
	}
	if len(fqdns) > 50 {
		fqdns = fqdns[:50]
	}
	limiter.Wait()
	params := creds.commonParams("namecheap.domains.check")
	params.Set("DomainList", strings.Join(fqdns, ","))
	body, err := callAPI(ctx, params)
	if err != nil {
		return nil, err
	}
	var resp apiResponse
	if err := xml.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode namecheap response: %w", err)
	}
	if !strings.EqualFold(resp.Status, "OK") {
		return nil, formatErrors(resp.Errors.Errors)
	}
	limiter.OnSuccess()
	out := make([]AvailabilityResult, 0, len(resp.Command.CheckResult))
	for _, r := range resp.Command.CheckResult {
		out = append(out, AvailabilityResult{
			FQDN:              strings.ToLower(r.Domain),
			Available:         r.Available,
			Premium:           r.IsPremiumName,
			PremiumRegPrice:   parseFloat(r.PremiumRegistrationPrice),
			PremiumRenewPrice: parseFloat(r.PremiumRenewalPrice),
		})
	}
	return out, nil
}

// PriceEntry is the per-TLD pricing snapshot for Namecheap.
type PriceEntry struct {
	TLD          string  `json:"tld"`
	Registration float64 `json:"registration"`
	Renewal      float64 `json:"renewal"`
	Transfer     float64 `json:"transfer"`
}

// FetchPricing pulls the Namecheap pricing tables for one product type
// (register | renew | transfer). Use FetchAllPricing for a combined snapshot.
func FetchPricing(ctx context.Context, creds Creds, productType string) (map[string]float64, error) {
	if err := creds.Validate(); err != nil {
		return nil, err
	}
	limiter.Wait()
	params := creds.commonParams("namecheap.users.getPricing")
	params.Set("ProductType", "DOMAIN")
	params.Set("ActionName", productType) // REGISTER | RENEW | TRANSFER
	body, err := callAPI(ctx, params)
	if err != nil {
		return nil, err
	}
	var resp apiResponse
	if err := xml.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode namecheap pricing: %w", err)
	}
	if !strings.EqualFold(resp.Status, "OK") {
		return nil, formatErrors(resp.Errors.Errors)
	}
	limiter.OnSuccess()
	out := map[string]float64{}
	for _, cat := range resp.Command.Pricing {
		for _, p := range cat.Products {
			tld := strings.ToLower(strings.TrimPrefix(p.Name, "."))
			if tld == "" {
				continue
			}
			// pick the 1-year price
			for _, price := range p.Prices {
				if strings.EqualFold(price.DurationType, "YEAR") && price.Duration == 1 {
					out[tld] = parseFloat(price.Price)
					break
				}
			}
		}
	}
	return out, nil
}

// FetchAllPricing fetches REGISTER + RENEW + TRANSFER and joins them.
func FetchAllPricing(ctx context.Context, creds Creds) ([]PriceEntry, error) {
	reg, err := FetchPricing(ctx, creds, "REGISTER")
	if err != nil {
		return nil, fmt.Errorf("register: %w", err)
	}
	renew, err := FetchPricing(ctx, creds, "RENEW")
	if err != nil {
		return nil, fmt.Errorf("renew: %w", err)
	}
	trf, err := FetchPricing(ctx, creds, "TRANSFER")
	if err != nil {
		return nil, fmt.Errorf("transfer: %w", err)
	}
	seen := map[string]bool{}
	out := []PriceEntry{}
	for tld, p := range reg {
		seen[tld] = true
		out = append(out, PriceEntry{TLD: tld, Registration: p, Renewal: renew[tld], Transfer: trf[tld]})
	}
	for tld, p := range renew {
		if seen[tld] {
			continue
		}
		seen[tld] = true
		out = append(out, PriceEntry{TLD: tld, Renewal: p, Transfer: trf[tld]})
	}
	return out, nil
}

func callAPI(ctx context.Context, params url.Values) ([]byte, error) {
	endpoint := Endpoint
	if v := strings.TrimSpace(os.Getenv("NAMECHEAP_SANDBOX")); v == "1" || strings.EqualFold(v, "true") {
		endpoint = SandboxEndpoint
	}
	u := endpoint + "?" + params.Encode()
	c := &http.Client{Timeout: 25 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("namecheap http: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		limiter.OnRateLimit()
		return nil, fmt.Errorf("namecheap: rate limited (HTTP 429)")
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("namecheap: HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func formatErrors(errs []apiError) error {
	if len(errs) == 0 {
		return fmt.Errorf("namecheap: unknown error")
	}
	parts := make([]string, 0, len(errs))
	for _, e := range errs {
		// Hint common IP-whitelist failure
		hint := ""
		if e.Number == 1011102 || strings.Contains(strings.ToLower(e.Text), "whitelist") || strings.Contains(strings.ToLower(e.Text), "ip address") {
			hint = " (hint: whitelist your current IP in Namecheap dashboard at https://ap.www.namecheap.com/settings/tools/apiaccess/whitelisted-ips)"
		}
		parts = append(parts, fmt.Sprintf("[%d] %s%s", e.Number, strings.TrimSpace(e.Text), hint))
	}
	return fmt.Errorf("namecheap: %s", strings.Join(parts, "; "))
}

func parseFloat(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return f
}
