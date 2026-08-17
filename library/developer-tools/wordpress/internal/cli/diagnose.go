// pp:data-source live

package cli

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
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/wordpress/internal/config"
)

const maxProbeBody = 4 << 20

type wordpressRuntime struct {
	Origin            string
	AuthHeaders       map[string]string
	HasAuth           bool
	RestRouteFallback bool
}

type diagnoseProbe struct {
	Name    string         `json:"name"`
	Form    string         `json:"form,omitempty"`
	Status  int            `json:"status"`
	Code    string         `json:"code,omitempty"`
	OK      bool           `json:"ok"`
	Error   string         `json:"error,omitempty"`
	Details map[string]any `json:"details,omitempty"`

	body        []byte
	headers     http.Header
	contentType string
}

type diagnoseEvidence struct {
	PrettyRoot     diagnoseProbe
	RouteRoot      diagnoseProbe
	PrettyPosts    diagnoseProbe
	RoutePosts     diagnoseProbe
	AnonSettings   diagnoseProbe
	AuthSettings   diagnoseProbe
	AuthHeaderTest diagnoseProbe
	HasAuth        bool
}

type diagnoseClassification struct {
	Verdict string
	Remedy  string
	Plugin  string
	Details map[string]any
}

type diagnoseOutput struct {
	Target  string          `json:"target"`
	Verdict string          `json:"verdict"`
	Probes  []diagnoseProbe `json:"probes"`
	Remedy  string          `json:"remedy"`
	Details map[string]any  `json:"details"`
}

var wordpressAPILinkPattern = regexp.MustCompile(`(?i)<([^>]+)>\s*;[^,]*\brel\s*=\s*(?:"https://api\.w\.org/"|https://api\.w\.org/)`)

var knownRESTBlockers = map[string]string{
	"rest_cannot_access":               "Disable REST API",
	"rest_not_logged_in":               "WordPress handbook login-required filter",
	"rest_login_required":              "Disable WP REST API",
	"itsec_rest_api_access_restricted": "Solid/Kadence Security Restricted Access",
	"aios_user_lists_forbidden":        "All-In-One Security",
	"aios_user_details_forbidden":      "All-In-One Security",
	"rest_authentication_error":        "Perfmatters",
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		removeRootCommand(root, "diagnose")
		removeRootCommand(root, "caps")
		removeRootCommand(root, "schema")
		root.AddCommand(newDiagnoseCmd(flags))
		root.AddCommand(newCapsCmd(flags))
		root.AddCommand(newSchemaCmd(flags))
	})
}

func removeRootCommand(root *cobra.Command, name string) {
	for _, existing := range root.Commands() {
		if existing.Name() == name {
			root.RemoveCommand(existing)
		}
	}
}

func newDiagnoseCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diagnose [url]",
		Short: "Name the cause of a WordPress REST API failure",
		Example: "  wordpress-pp-cli diagnose https://example.com\n" +
			"  wordpress-pp-cli diagnose https://example.com --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would probe WordPress REST API availability, authentication, and edge behavior")
				return nil
			}
			if len(args) > 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("diagnose accepts at most one URL"))
			}

			explicitTarget := ""
			if len(args) == 1 {
				explicitTarget = args[0]
			}
			runtime, err := resolveWordPressRuntime(flags, explicitTarget)
			if err != nil {
				return configErr(err)
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			httpClient := &http.Client{Timeout: flags.timeout}
			out, classification := runDiagnose(ctx, httpClient, runtime)

			if flags.asJSON {
				if err := printJSONFiltered(cmd.OutOrStdout(), out, flags); err != nil {
					return err
				}
			} else if err := printDiagnoseHuman(cmd, out); err != nil {
				return err
			}

			switch classification.Verdict {
			case "ok", "ok-auth-required":
				return nil
			case "auth-header-stripped", "credentials-rejected", "app-layer-block":
				return authErr(fmt.Errorf("diagnose verdict: %s", classification.Verdict))
			default:
				return apiErr(fmt.Errorf("diagnose verdict: %s", classification.Verdict))
			}
		},
	}
	return cmd
}

// newNovelDiagnoseCmd keeps the generated root's scaffold hook buildable while
// init replaces that scaffold with the hand-authored command above.
func newNovelDiagnoseCmd(flags *rootFlags) *cobra.Command {
	return newDiagnoseCmd(flags)
}

func runDiagnose(ctx context.Context, httpClient *http.Client, runtime wordpressRuntime) (diagnoseOutput, diagnoseClassification) {
	probes := make([]diagnoseProbe, 0, 8)
	head := executeProbe(ctx, httpClient, runtime, "site-head", "", http.MethodHead, siteURL(runtime.Origin), false)
	probes = append(probes, head)

	postParams := url.Values{"per_page": []string{"1"}, "_fields": []string{"id"}}
	prettyPosts := executeProbe(ctx, httpClient, runtime, "posts", "pretty", http.MethodGet, prettyRESTURL(runtime.Origin, "/wp/v2/posts", postParams), false)
	probes = append(probes, prettyPosts)
	routePosts := executeProbe(ctx, httpClient, runtime, "posts", "rest_route", http.MethodGet, restRouteURL(runtime.Origin, "/wp/v2/posts", postParams), false)
	probes = append(probes, routePosts)

	anonSettings := executeProbe(ctx, httpClient, runtime, "settings-anonymous", "pretty", http.MethodGet, prettyRESTURL(runtime.Origin, "/wp/v2/settings", nil), false)
	probes = append(probes, anonSettings)
	authSettings := diagnoseProbe{}
	authHeaderTest := diagnoseProbe{}
	if runtime.HasAuth {
		authSettings = executeProbe(ctx, httpClient, runtime, "settings-authenticated", "pretty", http.MethodGet, prettyRESTURL(runtime.Origin, "/wp/v2/settings", nil), true)
		probes = append(probes, authSettings)
		authHeaderTest = executeProbe(ctx, httpClient, runtime, "authorization-header", "pretty", http.MethodGet, prettyRESTURL(runtime.Origin, "/wp-site-health/v1/tests/authorization-header", nil), true)
		probes = append(probes, authHeaderTest)
	}

	// The route index is auxiliary evidence needed for route-missing and the
	// endpoint summary. Keep it after the six ordered diagnostic probes above.
	prettyRoot := executeProbe(ctx, httpClient, runtime, "rest-root", "pretty", http.MethodGet, prettyRESTURL(runtime.Origin, "/", nil), false)
	probes = append(probes, prettyRoot)
	routeRoot := executeProbe(ctx, httpClient, runtime, "rest-root", "rest_route", http.MethodGet, restRouteURL(runtime.Origin, "/", nil), false)
	probes = append(probes, routeRoot)

	evidence := diagnoseEvidence{
		PrettyRoot: prettyRoot, RouteRoot: routeRoot,
		PrettyPosts: prettyPosts, RoutePosts: routePosts,
		AnonSettings: anonSettings, AuthSettings: authSettings,
		AuthHeaderTest: authHeaderTest, HasAuth: runtime.HasAuth,
	}
	classification := classifyDiagnose(evidence)
	details := classification.Details
	if details == nil {
		details = make(map[string]any)
	}
	if advertised := parseWordPressAPILink(head.headers); advertised != "" {
		details["advertised_root"] = advertised
		details["link_header_present"] = true
		if !isDefaultRESTRoot(runtime.Origin, advertised) && classification.Verdict == "no-rest-api" {
			classification.Remedy = "WordPress advertises its real REST root at " + advertised + "; use that root because /wp-json/ has been renamed."
		}
	} else {
		details["link_header_present"] = false
	}
	if root := firstSuccessfulRoot(prettyRoot, routeRoot); root.Status == http.StatusOK {
		var index struct {
			Routes     map[string]json.RawMessage `json:"routes"`
			Namespaces []string                   `json:"namespaces"`
		}
		if json.Unmarshal(root.body, &index) == nil {
			namespaces := append([]string(nil), index.Namespaces...)
			if namespaces == nil {
				namespaces = make([]string, 0)
			}
			details["endpoint_count"] = len(index.Routes)
			details["namespaces"] = namespaces
		}
	}
	if retryAfter := firstHeader(prettyPosts, routePosts, "Retry-After"); retryAfter != "" {
		details["retry_after"] = retryAfter
	}
	classification.Details = details

	return diagnoseOutput{
		Target: runtime.Origin, Verdict: classification.Verdict,
		Probes: probes, Remedy: classification.Remedy, Details: details,
	}, classification
}

func classifyDiagnose(e diagnoseEvidence) diagnoseClassification {
	result := diagnoseClassification{Details: make(map[string]any)}
	publicWorks := e.PrettyPosts.Status == http.StatusOK && e.RoutePosts.Status == http.StatusOK
	if publicWorks {
		if e.HasAuth && e.AnonSettings.Status == http.StatusUnauthorized && e.AuthSettings.Status == http.StatusOK {
			result.Verdict = "ok-auth-required"
			result.Remedy = "No action needed: public reads work and protected settings accept the configured credentials."
			return result
		}
		if e.HasAuth && !e.AnonSettings.OK && !e.AuthSettings.OK && sameWordPressFailure(e.AnonSettings, e.AuthSettings) {
			if authorizationHeaderRejected(e.AuthHeaderTest) {
				result.Verdict = "auth-header-stripped"
				result.Remedy = "The host strips Authorization before PHP sees it. Apache: <IfModule mod_setenvif>SetEnvIf Authorization \"(.*)\" HTTP_AUTHORIZATION=$1</IfModule> Nginx: fastcgi_pass_header Authorization;"
				return result
			}
			result.Verdict = "credentials-rejected"
			result.Remedy = "The credentials were rejected. Wordfence disables Application Passwords by default on its 5,000,000+ installs (loginSec_disableApplicationPasswords), so the credential may be valid while the feature is switched off site-side."
			return result
		}
		result.Verdict = "ok"
		result.Remedy = "No action needed: both REST request forms work."
		return result
	}

	if e.PrettyPosts.Status == http.StatusTooManyRequests || e.RoutePosts.Status == http.StatusTooManyRequests {
		result.Verdict = "rate-limited"
		result.Remedy = "The edge or application is rate-limiting REST requests. Honor Retry-After when present; it is often absent and the response may be HTML."
		return result
	}
	if e.PrettyPosts.Status != http.StatusOK && e.RoutePosts.Status == http.StatusOK {
		result.Verdict = "path-blocked"
		result.Remedy = "An nginx, Apache, WAF, or CDN rule matches the literal wp-json path. The CLI will use the ?rest_route= form permanently for this site."
		return result
	}
	if collectionRouteMissing(e) {
		result.Verdict = "route-missing"
		result.Remedy = "The REST API is healthy, but this collection is absent. Check for Plain permalinks or a rest_endpoints filter that unregisters the route."
		return result
	}
	if rootsAbsent(e) {
		result.Verdict = "no-rest-api"
		result.Remedy = "No REST root was found. The target may not be WordPress, or rest_url_prefix was renamed by code such as WP Ghost or WP Hide."
		return result
	}
	if e.PrettyPosts.Code != "" && e.RoutePosts.Code != "" {
		code := e.PrettyPosts.Code
		if code == "" {
			code = e.RoutePosts.Code
		}
		plugin := knownRESTBlocker(code)
		result.Verdict = "app-layer-block"
		result.Plugin = plugin
		result.Details["observed_code"] = code
		if plugin != "" {
			result.Details["likely_source"] = plugin
		}
		if code == "rest_cannot_access" {
			result.Details["message_signature"] = "DRA:"
		}
		if message := firstProbeMessage(e.PrettyPosts, e.RoutePosts); message != "" {
			result.Details["message"] = message
		}
		if parameterDetails := firstParameterDetails(e.PrettyPosts, e.RoutePosts); len(parameterDetails) > 0 {
			result.Details["parameter_details"] = parameterDetails
		}
		result.Remedy = "A rest_authentication_errors filter or security plugin is refusing the request. These restrictions are usually anonymous-only, so supplying credentials may fix it."
		return result
	}
	if isBotChallenge(e.PrettyPosts) || isBotChallenge(e.RoutePosts) {
		result.Verdict = "bot-challenge"
		result.Remedy = "A bot-mitigation layer above WordPress, commonly Cloudflare, is challenging the request. This is not fixable client-side; the site owner must allowlist the REST path."
		return result
	}

	result.Verdict = "no-rest-api"
	result.Remedy = "The REST API could not be reached on either request form. Verify the site URL and any renamed REST prefix."
	return result
}

func knownRESTBlocker(code string) string {
	return knownRESTBlockers[code]
}

func collectionRouteMissing(e diagnoseEvidence) bool {
	rootWorks := e.PrettyRoot.Status == http.StatusOK || e.RouteRoot.Status == http.StatusOK
	return rootWorks && e.PrettyPosts.Code == "rest_no_route" && e.RoutePosts.Code == "rest_no_route"
}

func rootsAbsent(e diagnoseEvidence) bool {
	return e.PrettyRoot.Status == http.StatusNotFound && e.RouteRoot.Status == http.StatusNotFound &&
		e.PrettyPosts.Status == http.StatusNotFound && e.RoutePosts.Status == http.StatusNotFound
}

func sameWordPressFailure(a, b diagnoseProbe) bool {
	if a.Code != "" || b.Code != "" {
		return a.Code != "" && a.Code == b.Code
	}
	return a.Status != 0 && a.Status == b.Status && bytes.Equal(bytes.TrimSpace(a.body), bytes.TrimSpace(b.body))
}

func authorizationHeaderRejected(probe diagnoseProbe) bool {
	text := strings.ToLower(string(probe.body))
	if (strings.Contains(text, "authorization") || strings.Contains(text, "header")) &&
		(strings.Contains(text, "missing") || strings.Contains(text, "invalid") || strings.Contains(text, "not found") || strings.Contains(text, "stripped")) {
		return true
	}
	var value map[string]any
	if json.Unmarshal(probe.body, &value) != nil {
		return false
	}
	if success, ok := value["success"].(bool); ok && !success {
		return true
	}
	status, _ := value["status"].(string)
	switch strings.ToLower(status) {
	case "critical", "failed", "failure", "missing", "invalid", "error":
		return true
	default:
		return false
	}
}

func isBotChallenge(probe diagnoseProbe) bool {
	if probe.OK {
		return false
	}
	if probe.headers.Get("cf-mitigated") != "" {
		return true
	}
	body := strings.ToLower(string(probe.body))
	if strings.Contains(body, "just a moment") || strings.Contains(body, "attention required") {
		return true
	}
	return strings.Contains(strings.ToLower(probe.contentType), "text/html") && probe.Status != http.StatusNotFound
}

func executeProbe(ctx context.Context, httpClient *http.Client, runtime wordpressRuntime, name, form, method, target string, authenticated bool) diagnoseProbe {
	probe := diagnoseProbe{Name: name, Form: form, headers: make(http.Header), Details: make(map[string]any)}
	req, err := http.NewRequestWithContext(ctx, method, target, nil)
	if err != nil {
		probe.Error = err.Error()
		return probe
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "wordpress-pp-cli/live-probe")
	if authenticated {
		for key, value := range runtime.AuthHeaders {
			req.Header.Set(key, value)
		}
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		probe.Error = err.Error()
		return probe
	}
	defer resp.Body.Close()
	probe.Status = resp.StatusCode
	probe.OK = resp.StatusCode >= 200 && resp.StatusCode < 300
	probe.headers = resp.Header.Clone()
	probe.contentType = resp.Header.Get("Content-Type")
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxProbeBody))
	if readErr != nil {
		probe.Error = readErr.Error()
		probe.OK = false
		return probe
	}
	probe.body = body
	probe.Code, probe.Details = parseWordPressError(body)
	if len(probe.Details) == 0 {
		probe.Details = nil
	}
	return probe
}

func parseWordPressError(body []byte) (string, map[string]any) {
	var payload struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Details map[string]any `json:"details"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &payload) != nil || payload.Code == "" {
		return "", nil
	}
	details := make(map[string]any)
	if payload.Message != "" {
		details["message"] = payload.Message
	}
	if len(payload.Data.Details) > 0 {
		details["parameter_details"] = payload.Data.Details
	}
	return payload.Code, details
}

func parseWordPressAPILink(header http.Header) string {
	for _, value := range header.Values("Link") {
		match := wordpressAPILinkPattern.FindStringSubmatch(value)
		if len(match) == 2 {
			return strings.TrimSpace(match[1])
		}
	}
	return ""
}

func resolveWordPressRuntime(flags *rootFlags, explicitTarget string) (wordpressRuntime, error) {
	runtime := wordpressRuntime{AuthHeaders: make(map[string]string)}
	configuredClient, clientErr := flags.newClient()
	configuredOrigin := ""
	if configuredClient != nil && configuredClient.Config != nil {
		configuredOrigin, _ = wordpressSiteOrigin(configuredClient.RequestBaseURL())
		for key, value := range configuredClient.Config.Headers {
			runtime.AuthHeaders[key] = value
			if strings.EqualFold(key, "Authorization") || strings.EqualFold(key, "X-WP-Nonce") {
				runtime.HasAuth = value != ""
			}
		}
		if authHeader := configuredClient.Config.AuthHeader(); authHeader != "" {
			runtime.AuthHeaders["Authorization"] = authHeader
			runtime.HasAuth = true
		}
	}

	if explicitTarget != "" {
		runtime.Origin = explicitTarget
	} else {
		registry, registryErr := config.LoadWordPressSites(flags.configPath)
		if registryErr == nil {
			if active, ok := registry.ActiveSite(); ok {
				runtime.Origin = active.SiteURL
				if runtime.Origin == "" {
					runtime.Origin = active.BaseURL
				}
				runtime.RestRouteFallback = active.RestRouteFallback
			}
		}
		if runtime.Origin == "" && configuredClient != nil {
			runtime.Origin = configuredClient.RequestBaseURL()
		}
		if runtime.Origin == "" && clientErr != nil {
			return runtime, fmt.Errorf("resolve configured WordPress site: %w", clientErr)
		}
	}
	if runtime.Origin == "" {
		return runtime, errors.New("no active WordPress site or configured base URL")
	}

	origin, err := wordpressSiteOrigin(runtime.Origin)
	if err != nil {
		return runtime, err
	}
	runtime.Origin = origin
	if explicitTarget != "" && !sameWordPressSite(runtime.Origin, configuredOrigin) {
		runtime.AuthHeaders = make(map[string]string)
		runtime.HasAuth = false
	}
	return runtime, nil
}

func wordpressSiteOrigin(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid WordPress site URL %q", raw)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("unsupported WordPress site URL scheme %q", parsed.Scheme)
	}
	if parsed.User != nil {
		return "", fmt.Errorf("WordPress site URL must not contain credentials")
	}
	if index := strings.Index(parsed.Path, "/wp-json"); index >= 0 {
		parsed.Path = parsed.Path[:index]
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return strings.TrimRight(parsed.String(), "/"), nil
}

func sameWordPressSite(left, right string) bool {
	if left == "" || right == "" {
		return false
	}
	leftURL, leftErr := url.Parse(left)
	rightURL, rightErr := url.Parse(right)
	if leftErr != nil || rightErr != nil {
		return false
	}
	return strings.EqualFold(leftURL.Scheme, rightURL.Scheme) &&
		strings.EqualFold(leftURL.Host, rightURL.Host) &&
		strings.TrimRight(leftURL.EscapedPath(), "/") == strings.TrimRight(rightURL.EscapedPath(), "/")
}

func siteURL(origin string) string {
	return strings.TrimRight(origin, "/") + "/"
}

func prettyRESTURL(origin, route string, query url.Values) string {
	result := strings.TrimRight(origin, "/") + "/wp-json"
	if route == "/" {
		result += "/"
	} else {
		result += "/" + strings.TrimLeft(route, "/")
	}
	if len(query) > 0 {
		result += "?" + query.Encode()
	}
	return result
}

func restRouteURL(origin, route string, query url.Values) string {
	values := make(url.Values)
	for key, items := range query {
		values[key] = append([]string(nil), items...)
	}
	values.Set("rest_route", route)
	return siteURL(origin) + "?" + values.Encode()
}

func runtimeRESTURL(runtime wordpressRuntime, route string, query url.Values) string {
	if runtime.RestRouteFallback {
		return restRouteURL(runtime.Origin, route, query)
	}
	return prettyRESTURL(runtime.Origin, route, query)
}

func isDefaultRESTRoot(origin, advertised string) bool {
	return strings.TrimRight(advertised, "/") == strings.TrimRight(prettyRESTURL(origin, "/", nil), "/")
}

func firstSuccessfulRoot(probes ...diagnoseProbe) diagnoseProbe {
	for _, probe := range probes {
		if probe.Status == http.StatusOK {
			return probe
		}
	}
	return diagnoseProbe{}
}

func firstHeader(a, b diagnoseProbe, key string) string {
	if value := a.headers.Get(key); value != "" {
		return value
	}
	return b.headers.Get(key)
}

func firstProbeMessage(probes ...diagnoseProbe) string {
	for _, probe := range probes {
		if message, ok := probe.Details["message"].(string); ok && message != "" {
			return message
		}
	}
	return ""
}

func firstParameterDetails(probes ...diagnoseProbe) map[string]any {
	for _, probe := range probes {
		if details, ok := probe.Details["parameter_details"].(map[string]any); ok && len(details) > 0 {
			return details
		}
	}
	return nil
}

func printDiagnoseHuman(cmd *cobra.Command, out diagnoseOutput) error {
	fmt.Fprintf(cmd.OutOrStdout(), "diagnose: %s — %s\n", out.Verdict, out.Target)
	rows := make([]map[string]any, 0, len(out.Probes))
	for _, probe := range out.Probes {
		rows = append(rows, map[string]any{
			"name": probe.Name, "form": probe.Form, "status": probe.Status,
			"code": probe.Code, "ok": probe.OK, "error": probe.Error,
		})
	}
	if err := printAutoTable(cmd.OutOrStdout(), rows); err != nil {
		return err
	}
	if details, ok := out.Details["parameter_details"]; ok {
		encoded, _ := json.Marshal(details)
		fmt.Fprintf(cmd.OutOrStdout(), "parameter details: %s\n", encoded)
	}
	fmt.Fprintln(cmd.OutOrStdout(), "remedy:", out.Remedy)
	return nil
}

func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
