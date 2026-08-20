// Copyright 2026 zaydiscold. Licensed under Apache-2.0. See LICENSE.

package brokeragemap

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/payments/robinhood/internal/cliutil"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
)

const AllowWritesEnv = "ROBINHOOD_PP_ALLOW_WRITES"

//go:embed brokerage-routes.json browser-cdp-routes-2026-05-27.json robinhood-routes.json
var routeFS embed.FS

type Route struct {
	URL          string   `json:"url"`
	Host         string   `json:"host"`
	Categories   []string `json:"categories"`
	Risk         string   `json:"risk"`
	Methods      []string `json:"methods,omitempty"`
	Source       string   `json:"source,omitempty"`
	SeenOn       []string `json:"seenOn,omitempty"`
	QueryKeys    []string `json:"queryKeys,omitempty"`
	RequestTypes []string `json:"requestTypes,omitempty"`
}

type Filters struct {
	Risk     string
	Category string
	Host     string
	Query    string
	Limit    int
}

type Summary struct {
	RouteCount          int            `json:"route_count"`
	UnifiedRouteCount   int            `json:"unified_route_count"`
	OfficialCryptoCount int            `json:"official_crypto_route_count"`
	BrowserRouteCount   int            `json:"browser_route_count"`
	Hosts               map[string]int `json:"hosts"`
	Risks               map[string]int `json:"risks"`
	Categories          map[string]int `json:"categories"`
}

type Plan struct {
	URL            string   `json:"url"`
	Method         string   `json:"method"`
	Risk           string   `json:"risk"`
	Host           string   `json:"host"`
	Categories     []string `json:"categories"`
	MissingParams  []string `json:"missing_params"`
	Warnings       []string `json:"warnings"`
	Command        string   `json:"command"`
	Mode           string   `json:"mode"`
	MutatesAccount bool     `json:"mutates_account"`
	RequiresAuth   bool     `json:"requires_auth"`
	Body           any      `json:"body,omitempty"`
}

type ExecuteOptions struct {
	DryRun       bool
	Body         []byte
	FullBody     bool
	MaxBodyBytes int
	RateLimit    float64
	// Timeout is the per-request HTTP timeout. When zero, Execute falls back
	// to the historical 30s default so callers that don't set it are unchanged.
	Timeout time.Duration
}

type ExecuteResult struct {
	OK             bool   `json:"ok"`
	Status         int    `json:"status"`
	StatusText     string `json:"status_text"`
	Method         string `json:"method"`
	URL            string `json:"url"`
	Risk           string `json:"risk"`
	MutatesAccount bool   `json:"mutates_account"`
	RequiresAuth   bool   `json:"requires_auth"`
	ContentType    string `json:"content_type,omitempty"`
	Body           string `json:"body"`
	Truncated      bool   `json:"truncated"`
}

func Routes() ([]Route, error) {
	return readRoutes("brokerage-routes.json")
}

func UnifiedRoutes() ([]Route, error) {
	return readRoutes("robinhood-routes.json")
}

func BrowserRoutes() ([]Route, error) {
	return readRoutes("browser-cdp-routes-2026-05-27.json")
}

func readRoutes(name string) ([]Route, error) {
	data, err := routeFS.ReadFile(name)
	if err != nil {
		return nil, err
	}
	var routes []Route
	if err := json.Unmarshal(data, &routes); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", name, err)
	}
	return routes, nil
}

func Filter(routes []Route, filters Filters) []Route {
	query := strings.ToLower(strings.TrimSpace(filters.Query))
	out := make([]Route, 0, len(routes))
	for _, route := range routes {
		if filters.Risk != "" && route.Risk != filters.Risk {
			continue
		}
		if filters.Host != "" && route.Host != filters.Host {
			continue
		}
		if filters.Category != "" && !contains(route.Categories, filters.Category) {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(route.URL), query) {
			continue
		}
		out = append(out, route)
		if filters.Limit > 0 && len(out) >= filters.Limit {
			break
		}
	}
	return out
}

func Find(routes []Route, query string) (Route, error) {
	matches := Filter(routes, Filters{Query: query})
	for _, route := range matches {
		if route.URL == query {
			return route, nil
		}
	}
	if len(matches) == 0 {
		return Route{}, fmt.Errorf("no Robinhood brokerage route matched: %s", query)
	}
	return matches[0], nil
}

func Summarize() (Summary, error) {
	routes, err := Routes()
	if err != nil {
		return Summary{}, err
	}
	unifiedRoutes, err := UnifiedRoutes()
	if err != nil {
		return Summary{}, err
	}
	browserRoutes, err := BrowserRoutes()
	if err != nil {
		return Summary{}, err
	}
	summary := Summary{
		RouteCount:          len(routes),
		UnifiedRouteCount:   len(unifiedRoutes),
		OfficialCryptoCount: countHost(unifiedRoutes, "trading.robinhood.com"),
		BrowserRouteCount:   len(browserRoutes),
		Hosts:               map[string]int{},
		Risks:               map[string]int{},
		Categories:          map[string]int{},
	}
	for _, route := range unifiedRoutes {
		summary.Hosts[route.Host]++
		summary.Risks[route.Risk]++
		for _, category := range route.Categories {
			summary.Categories[category]++
		}
	}
	return summary, nil
}

func countHost(routes []Route, host string) int {
	count := 0
	for _, route := range routes {
		if route.Host == host {
			count++
		}
	}
	return count
}

func BuildPlan(route Route, method string, params map[string]string, body any, dryRun bool) Plan {
	missing := []string{}
	resolvedURL := placeholderRE.ReplaceAllStringFunc(route.URL, func(match string) string {
		name := strings.Trim(match, "{}")
		if value := params[name]; value != "" {
			// Percent-encode user-supplied path values so reserved characters
			// in an --param value (e.g. an order-id containing "/" or "..")
			// cannot collapse into extra path segments and retarget the request.
			return url.PathEscape(value)
		}
		missing = append(missing, name)
		return match
	})
	if method == "" {
		method = InferMethod(route)
	}
	mode := "execute"
	if dryRun {
		mode = "dry_run"
	}
	return Plan{
		URL:            resolvedURL,
		Method:         strings.ToUpper(method),
		Risk:           route.Risk,
		Host:           route.Host,
		Categories:     route.Categories,
		MissingParams:  uniqueSorted(missing),
		Warnings:       RiskWarnings(route.Risk),
		Command:        fmt.Sprintf("curl -sS -X %s %q -H \"Authorization: Bearer $ROBINHOOD_BROKERAGE_TOKEN\"", strings.ToUpper(method), resolvedURL),
		Mode:           mode,
		MutatesAccount: Mutates(route.Risk),
		RequiresAuth:   RequiresAuth(route),
		Body:           body,
	}
}

// BuildDirectPlan constructs a Plan for an explicit Robinhood host + path that
// is not (or need not be) looked up from the embedded route list. It is the
// foundation for the typed brokerage read/write commands (accounts, options,
// performance, transfers, dividends, orders): each captured endpoint from the
// live API surface is expressed as a host + path template here, then executed
// through the same Bearer-token, multi-host, rate-limited, write-gated Execute
// path the generic `brokerage execute` command uses. No new auth path is
// invented — typed commands and the route-map executor share one transport.
//
// pathTemplate may contain {placeholder} segments; supplying params resolves
// them and any unresolved placeholder is reported in Plan.MissingParams, exactly
// like the route-list BuildPlan. query carries already-resolved query-string
// values appended to the URL (empty values are dropped).
func BuildDirectPlan(host, pathTemplate, method, risk string, params, query map[string]string) Plan {
	route := Route{
		URL:        "https://" + host + pathTemplate,
		Host:       host,
		Risk:       risk,
		Categories: nil,
		Methods:    []string{strings.ToUpper(method)},
	}
	plan := BuildPlan(route, method, params, nil, false)
	if len(query) > 0 {
		keys := make([]string, 0, len(query))
		for k := range query {
			if query[k] != "" {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		sep := "?"
		if strings.Contains(plan.URL, "?") {
			sep = "&"
		}
		for _, k := range keys {
			// Percent-encode user-supplied query keys and values so reserved
			// characters in a --query value cannot inject extra parameters or
			// corrupt the request URL.
			plan.URL += sep + url.QueryEscape(k) + "=" + url.QueryEscape(query[k])
			sep = "&"
		}
	}
	return plan
}

func InferMethod(route Route) string {
	if len(route.Methods) > 0 && route.Methods[0] != "" {
		return route.Methods[0]
	}
	if route.Risk == "write-safe" || route.Risk == "write-mutate" || route.Risk == "write-or-sensitive" || route.Risk == "destructive" {
		return http.MethodPost
	}
	return http.MethodGet
}

func Mutates(risk string) bool {
	return risk == "write-safe" || risk == "write-mutate" || risk == "write-or-sensitive" || risk == "destructive"
}

func RequiresAuth(route Route) bool {
	return route.Risk != "read" || route.Host == "api.robinhood.com"
}

func RiskWarnings(risk string) []string {
	switch risk {
	case "destructive":
		return []string{"Destructive route. PP execution defaults to dry-run and live execution requires ROBINHOOD_PP_ALLOW_WRITES=1."}
	case "write-mutate", "write-or-sensitive":
		return []string{"Write route. PP execution defaults to dry-run and live execution requires ROBINHOOD_PP_ALLOW_WRITES=1."}
	case "write-safe":
		return []string{"Non-account-state write route such as telemetry or acknowledgement. PP execution defaults to dry-run."}
	case "sensitive-read":
		return []string{"Sensitive read route. Redact account identifiers, positions, documents, and tax data in shared artifacts."}
	default:
		return nil
	}
}

func ParseParams(values []string) (map[string]string, error) {
	params := map[string]string{}
	for _, value := range values {
		name, val, ok := strings.Cut(value, "=")
		if !ok || strings.TrimSpace(name) == "" {
			return nil, fmt.Errorf("invalid --param value %q: use name=value", value)
		}
		params[name] = val
	}
	return params, nil
}

func Execute(ctx context.Context, plan Plan, options ExecuteOptions) (ExecuteResult, error) {
	if options.DryRun || plan.Mode == "dry_run" {
		body, _ := json.MarshalIndent(plan, "", "  ")
		return ExecuteResult{
			OK:             true,
			Status:         0,
			StatusText:     "DRY_RUN",
			Method:         plan.Method,
			URL:            plan.URL,
			Risk:           plan.Risk,
			MutatesAccount: plan.MutatesAccount,
			RequiresAuth:   plan.RequiresAuth,
			ContentType:    "application/json",
			Body:           string(body),
		}, nil
	}
	if plan.MutatesAccount && os.Getenv(AllowWritesEnv) != "1" {
		return ExecuteResult{}, fmt.Errorf("live Robinhood write requires %s=1; rerun with --dry-run to preview without sending", AllowWritesEnv)
	}
	token := os.Getenv("ROBINHOOD_BROKERAGE_TOKEN")
	cookie := os.Getenv("ROBINHOOD_COOKIE")
	csrf := os.Getenv("ROBINHOOD_CSRF_TOKEN")
	if plan.RequiresAuth && token == "" && cookie == "" {
		return ExecuteResult{}, fmt.Errorf("missing auth: set ROBINHOOD_BROKERAGE_TOKEN or ROBINHOOD_COOKIE outside the repo")
	}
	req, err := http.NewRequestWithContext(ctx, plan.Method, plan.URL, bodyReader(plan.Method, options.Body))
	if err != nil {
		return ExecuteResult{}, err
	}
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("User-Agent", "robinhood-pp-cli/1.0")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	if csrf != "" {
		req.Header.Set("X-CSRFToken", csrf)
	}
	if len(options.Body) > 0 && plan.Method != http.MethodGet {
		req.Header.Set("Content-Type", "application/json")
	}
	limiter := cliutil.NewAdaptiveLimiter(options.RateLimit)
	limiter.Wait()
	timeout := options.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return ExecuteResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		limiter.OnRateLimit()
	} else if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		limiter.OnSuccess()
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return ExecuteResult{}, fmt.Errorf("reading response: %w", err)
	}
	max := options.MaxBodyBytes
	if max <= 0 {
		max = 4000
	}
	truncated := false
	if !options.FullBody && len(data) > max {
		data = data[:max]
		truncated = true
	}
	return ExecuteResult{
		OK:             resp.StatusCode >= 200 && resp.StatusCode < 300,
		Status:         resp.StatusCode,
		StatusText:     resp.Status,
		Method:         plan.Method,
		URL:            plan.URL,
		Risk:           plan.Risk,
		MutatesAccount: plan.MutatesAccount,
		RequiresAuth:   plan.RequiresAuth,
		ContentType:    resp.Header.Get("Content-Type"),
		Body:           string(data),
		Truncated:      truncated,
	}, nil
}

var placeholderRE = regexp.MustCompile(`\{([^}]+)\}`)

func bodyReader(method string, body []byte) io.Reader {
	if method == http.MethodGet || len(body) == 0 {
		return nil
	}
	return bytes.NewReader(body)
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func uniqueSorted(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		if seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
