// Copyright 2026 zayd. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

const defaultAccountBaseURL = "https://account.squarespace.com"

func newAccountCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "account",
		Short: "Inspect authenticated Squarespace account, site, and domain dashboard data",
		Long: `Inspect authenticated Squarespace account, site, and domain dashboard data.

These commands use Squarespace's browser-backed account API, not the official
Commerce API token surface. Provide a browser cookie header at runtime with
SQUARESPACE_ACCOUNT_COOKIE or SQUARESPACE_ACCOUNT_COOKIE_FILE. Sensitive JSON
fields such as CSRF tokens are redacted before output.`,
	}

	cmd.AddCommand(newAccountContextCmd(flags))
	cmd.AddCommand(newAccountSitesCmd(flags))
	cmd.AddCommand(newAccountBriefsCmd(flags))
	cmd.AddCommand(newAccountDomainsCmd(flags))
	cmd.AddCommand(newAccountDomainSummariesCmd(flags))
	cmd.AddCommand(newAccountDomainCmd(flags))
	cmd.AddCommand(newAccountWebsiteCmd(flags))
	return cmd
}

func newAccountContextCmd(flags *rootFlags) *cobra.Command {
	return accountGetCmd(flags, accountGetSpec{
		Use:     "context",
		Short:   "Show the authenticated account dashboard context",
		Example: "  squarespace-pp-cli account context --json",
		Path:    "/api/account/1/context/project-picker",
	})
}

func newAccountSitesCmd(flags *rootFlags) *cobra.Command {
	var page int
	var projectPicker bool

	cmd := accountGetCmd(flags, accountGetSpec{
		Use:     "sites",
		Short:   "List Squarespace website summaries visible to the account",
		Example: "  squarespace-pp-cli account sites --page 0 --json",
		PathFunc: func() (string, url.Values, error) {
			path := "/api/account/1/website-summaries"
			if projectPicker {
				path = "/api/account/1/project-picker/website-summaries"
			}
			q := url.Values{}
			q.Set("page", fmt.Sprintf("%d", page))
			return path, q, nil
		},
	})
	cmd.Flags().IntVar(&page, "page", 0, "Result page to request")
	cmd.Flags().BoolVar(&projectPicker, "project-picker", false, "Use the project-picker website summaries endpoint")
	return cmd
}

func newAccountBriefsCmd(flags *rootFlags) *cobra.Command {
	return accountGetCmd(flags, accountGetSpec{
		Use:     "briefs",
		Short:   "List compact website briefs for the account",
		Example: "  squarespace-pp-cli account briefs --json",
		Path:    "/api/account/1/website-briefs",
	})
}

func newAccountDomainsCmd(flags *rootFlags) *cobra.Command {
	return accountGetCmd(flags, accountGetSpec{
		Use:     "domains",
		Aliases: []string{"user-domains"},
		Short:   "List domains owned by the authenticated user",
		Example: "  squarespace-pp-cli account domains --json",
		Path:    "/api/account/1/user/domains",
	})
}

func newAccountDomainSummariesCmd(flags *rootFlags) *cobra.Command {
	var page int
	var pageSize int
	var query string
	var sortDirection string
	var sortField string

	cmd := accountGetCmd(flags, accountGetSpec{
		Use:     "domain-summaries",
		Short:   "List dashboard domain summaries with dashboard filters",
		Example: "  squarespace-pp-cli account domain-summaries --page-size 50 --json",
		PathFunc: func() (string, url.Values, error) {
			q := url.Values{}
			q.Set("page", fmt.Sprintf("%d", page))
			q.Set("pageSize", fmt.Sprintf("%d", pageSize))
			q.Set("sortDirection", sortDirection)
			q.Set("sortField", sortField)
			if query != "" {
				q.Set("query", query)
			}
			return "/api/account/1/domain-summaries", q, nil
		},
	})
	cmd.Flags().IntVar(&page, "page", 0, "Result page to request")
	cmd.Flags().IntVar(&pageSize, "page-size", 20, "Number of domains to request")
	cmd.Flags().StringVar(&query, "query", "", "Filter domains by name")
	cmd.Flags().StringVar(&sortDirection, "sort-direction", "ASCENDING", "Sort direction from the dashboard API")
	cmd.Flags().StringVar(&sortField, "sort-field", "NAME", "Sort field from the dashboard API")
	return cmd
}

func newAccountDomainCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "domain",
		Aliases: []string{"domains-by-name"},
		Short:   "Inspect one Squarespace domain by name",
	}
	cmd.AddCommand(newAccountDomainByNameCmd(flags))
	cmd.AddCommand(newAccountDomainDetailCmd(flags, "whois", "Show domain WHOIS contact information", "/api/account/1/domains/%s/whoisInfo"))
	cmd.AddCommand(newAccountDomainDetailCmd(flags, "registrar-info", "Show domain registrar information", "/api/account/1/domains/%s/registrar-info"))
	cmd.AddCommand(newAccountDomainDetailCmd(flags, "certificates", "Show domain SSL certificate records", "/api/account/1/domains/%s/certificates"))
	cmd.AddCommand(newAccountDomainDetailCmd(flags, "is-unparked", "Check whether a domain is unparked", "/api/account/1/domains/%s/is-unparked"))
	cmd.AddCommand(newAccountDomainDetailCmd(flags, "category", "Show the detected domain category", "/api/account/1/domains/%s/category"))
	cmd.AddCommand(newAccountDomainDetailCmd(flags, "permissions", "Show account permissions for one domain", "/api/account/1/domains/%s/user-permissions"))
	cmd.AddCommand(newAccountDomainDetailCmd(flags, "forwarding-presets", "Show forwarding presets for one domain", "/api/account/1/domains/%s/forwarding-presets"))
	cmd.AddCommand(newAccountDomainDetailCmd(flags, "custom-records", "Show custom DNS records for one domain", "/api/account/1/domains/%s/custom-record-set"))
	cmd.AddCommand(newAccountDomainDetailCmd(flags, "presets", "Show DNS provider presets for one domain", "/api/account/1/domains/%s/presets"))
	cmd.AddCommand(newAccountDomainDetailCmd(flags, "email-forwarding", "Show email forwarding rules for one domain", "/api/account/1/domains/%s/email-forwarding"))
	cmd.AddCommand(newAccountDomainDetailCmd(flags, "email-mx-conflicts", "Check email-forwarding MX conflicts for one domain", "/api/account/1/domains/%s/email-forwarding/has-conflicting-mx-records"))
	cmd.AddCommand(newAccountDomainBillingCmd(flags, "billing-eligibility", "Show domain billing renewal eligibility", false))
	cmd.AddCommand(newAccountDomainBillingCmd(flags, "billing-valid-terms", "Show valid domain renewal terms", true))
	cmd.AddCommand(newAccountGoogleWorkspacePricingCmd(flags))
	return cmd
}

func newAccountDomainByNameCmd(flags *rootFlags) *cobra.Command {
	var name string
	cmd := accountGetCmd(flags, accountGetSpec{
		Use:     "get",
		Short:   "Fetch dashboard metadata for one domain",
		Example: "  squarespace-pp-cli account domain get --name example.com --json",
		PathFunc: func() (string, url.Values, error) {
			if name == "" {
				return "", nil, usageErr(fmt.Errorf("--name is required"))
			}
			return "/api/account/1/domains/byName/" + url.PathEscape(name), nil, nil
		},
	})
	cmd.Flags().StringVar(&name, "name", "", "Domain name")
	return cmd
}

func newAccountDomainDetailCmd(flags *rootFlags, use, short, pathTemplate string) *cobra.Command {
	var name string
	var id string
	cmd := &cobra.Command{
		Use:         use,
		Short:       short,
		Example:     fmt.Sprintf("  squarespace-pp-cli account domain %s --name example.com --json", use),
		Annotations: map[string]string{"pp:method": "GET", "mcp:read-only": "true", "pp:surface": "squarespace-account-dashboard"},
		RunE: func(cmd *cobra.Command, args []string) error {
			resolvedID := id
			var resolveURL string
			if resolvedID == "" {
				if name == "" {
					return usageErr(fmt.Errorf("--name or --id is required"))
				}
				var err error
				resolvedID, resolveURL, err = resolveAccountDomainID(cmd.Context(), flags, name)
				if err != nil {
					return err
				}
			}
			path := fmt.Sprintf(pathTemplate, accountPathEscape(resolvedID))
			fullURL, err := accountURL(path, nil)
			if err != nil {
				return usageErr(err)
			}
			if flags.dryRun {
				payload, err := json.Marshal(map[string]any{
					"dry_run":     true,
					"method":      http.MethodGet,
					"url":         accountDryRunURL(fullURL),
					"resolve_url": resolveURL,
					"auth":        "SQUARESPACE_ACCOUNT_COOKIE or SQUARESPACE_ACCOUNT_COOKIE_FILE",
				})
				if err != nil {
					return err
				}
				return printOutputWithFlags(cmd.OutOrStdout(), payload, flags)
			}
			data, err := accountGet(cmd.Context(), flags, fullURL)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Domain name")
	cmd.Flags().StringVar(&id, "id", "", "Squarespace internal domain id")
	return cmd
}

func newAccountDomainBillingCmd(flags *rootFlags, use, short string, includeDomainName bool) *cobra.Command {
	var name string
	var domainID string
	var websiteID string
	var contractID string
	cmd := &cobra.Command{
		Use:         use,
		Short:       short,
		Example:     fmt.Sprintf("  squarespace-pp-cli account domain %s --name example.com --json", use),
		Annotations: map[string]string{"pp:method": "GET", "mcp:read-only": "true", "pp:surface": "squarespace-account-dashboard"},
		RunE: func(cmd *cobra.Command, args []string) error {
			meta := accountDomainMeta{
				ID:             domainID,
				Name:           name,
				WebsiteID:      websiteID,
				SubscriptionID: contractID,
			}
			resolveURL := ""
			if meta.WebsiteID == "" || meta.SubscriptionID == "" || (includeDomainName && meta.Name == "") {
				if name == "" {
					return usageErr(fmt.Errorf("--name is required unless --website-id and --contract-id are both provided"))
				}
				resolved, url, err := resolveAccountDomainMeta(cmd.Context(), flags, name)
				if err != nil {
					return err
				}
				resolveURL = url
				if meta.ID == "" {
					meta.ID = resolved.ID
				}
				if meta.Name == "" {
					meta.Name = resolved.Name
				}
				if meta.WebsiteID == "" {
					meta.WebsiteID = resolved.WebsiteID
				}
				if meta.SubscriptionID == "" {
					meta.SubscriptionID = resolved.SubscriptionID
				}
			}
			if meta.WebsiteID == "" {
				return notFoundErr(fmt.Errorf("domain %q lookup did not include websiteId", name))
			}
			if meta.SubscriptionID == "" {
				return notFoundErr(fmt.Errorf("domain %q lookup did not include subscriptionId", name))
			}
			path := "/api/account/1/billing/websites/" + accountPathEscape(meta.WebsiteID) + "/contracts/" + accountPathEscape(meta.SubscriptionID) + "/eligibility"
			var query url.Values
			if includeDomainName {
				path = "/api/account/1/billing/websites/" + accountPathEscape(meta.WebsiteID) + "/contracts/" + accountPathEscape(meta.SubscriptionID) + "/validTerms"
				query = url.Values{"domainName": []string{meta.Name}}
			}
			fullURL, err := accountURL(path, query)
			if err != nil {
				return usageErr(err)
			}
			if flags.dryRun {
				payload, err := json.Marshal(map[string]any{
					"dry_run":     true,
					"method":      http.MethodGet,
					"url":         accountDryRunURL(fullURL),
					"resolve_url": resolveURL,
					"auth":        "SQUARESPACE_ACCOUNT_COOKIE or SQUARESPACE_ACCOUNT_COOKIE_FILE",
				})
				if err != nil {
					return err
				}
				return printOutputWithFlags(cmd.OutOrStdout(), payload, flags)
			}
			data, err := accountGet(cmd.Context(), flags, fullURL)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Domain name")
	cmd.Flags().StringVar(&domainID, "id", "", "Squarespace internal domain id")
	cmd.Flags().StringVar(&websiteID, "website-id", "", "Squarespace website id")
	cmd.Flags().StringVar(&contractID, "contract-id", "", "Squarespace billing contract/subscription id")
	return cmd
}

func newAccountGoogleWorkspacePricingCmd(flags *rootFlags) *cobra.Command {
	var countryCode string
	cmd := accountGetCmd(flags, accountGetSpec{
		Use:     "google-workspace-pricing",
		Short:   "Show applicable Google Workspace starting prices for the account locale",
		Example: "  squarespace-pp-cli account domain google-workspace-pricing --country-code US --json",
		PathFunc: func() (string, url.Values, error) {
			q := url.Values{}
			q.Set("countryCode", countryCode)
			return "/api/account/1/plans/available-plans/google-apps/applicable/starting-prices", q, nil
		},
	})
	cmd.Flags().StringVar(&countryCode, "country-code", "US", "ISO country code for pricing")
	return cmd
}

func newAccountWebsiteCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "website",
		Aliases: []string{"site"},
		Short:   "Inspect one Squarespace website by dashboard id",
	}
	cmd.AddCommand(newAccountWebsiteGetCmd(flags))
	cmd.AddCommand(newAccountWebsiteDomainsCmd(flags))
	cmd.AddCommand(newAccountWebsiteContributorsCmd(flags))
	return cmd
}

func newAccountWebsiteGetCmd(flags *rootFlags) *cobra.Command {
	var id string
	cmd := accountGetCmd(flags, accountGetSpec{
		Use:     "get",
		Short:   "Fetch dashboard metadata for one website",
		Example: "  squarespace-pp-cli account website get --id <website-id> --json",
		PathFunc: func() (string, url.Values, error) {
			if id == "" {
				return "", nil, usageErr(fmt.Errorf("--id is required"))
			}
			return "/api/account/1/websites/" + url.PathEscape(id), nil, nil
		},
	})
	cmd.Flags().StringVar(&id, "id", "", "Website id")
	return cmd
}

func newAccountWebsiteDomainsCmd(flags *rootFlags) *cobra.Command {
	var websiteID string
	cmd := accountGetCmd(flags, accountGetSpec{
		Use:     "domains",
		Short:   "List domains attached to one website",
		Example: "  squarespace-pp-cli account website domains --website-id <website-id> --json",
		PathFunc: func() (string, url.Values, error) {
			if websiteID == "" {
				return "", nil, usageErr(fmt.Errorf("--website-id is required"))
			}
			return "/api/account/1/websites/" + url.PathEscape(websiteID) + "/website-domains", nil, nil
		},
	})
	cmd.Flags().StringVar(&websiteID, "website-id", "", "Website id")
	return cmd
}

func newAccountWebsiteContributorsCmd(flags *rootFlags) *cobra.Command {
	var websiteID string
	cmd := accountGetCmd(flags, accountGetSpec{
		Use:     "contributors",
		Short:   "List contributors for one website",
		Example: "  squarespace-pp-cli account website contributors --website-id <website-id> --json",
		PathFunc: func() (string, url.Values, error) {
			if websiteID == "" {
				return "", nil, usageErr(fmt.Errorf("--website-id is required"))
			}
			return "/api/account/1/websites/" + url.PathEscape(websiteID) + "/website-contributors", nil, nil
		},
	})
	cmd.Flags().StringVar(&websiteID, "website-id", "", "Website id")
	return cmd
}

type accountGetSpec struct {
	Use      string
	Aliases  []string
	Short    string
	Example  string
	Path     string
	PathFunc func() (string, url.Values, error)
}

func accountGetCmd(flags *rootFlags, spec accountGetSpec) *cobra.Command {
	cmd := &cobra.Command{
		Use:         spec.Use,
		Aliases:     spec.Aliases,
		Short:       spec.Short,
		Example:     spec.Example,
		Annotations: map[string]string{"pp:method": "GET", "mcp:read-only": "true", "pp:surface": "squarespace-account-dashboard"},
		RunE: func(cmd *cobra.Command, args []string) error {
			path, query, err := spec.resolve()
			if err != nil {
				return err
			}
			fullURL, err := accountURL(path, query)
			if err != nil {
				return usageErr(err)
			}
			if flags.dryRun {
				payload, err := json.Marshal(map[string]any{
					"dry_run": true,
					"method":  http.MethodGet,
					"url":     fullURL,
					"auth":    "SQUARESPACE_ACCOUNT_COOKIE or SQUARESPACE_ACCOUNT_COOKIE_FILE",
				})
				if err != nil {
					return err
				}
				return printOutputWithFlags(cmd.OutOrStdout(), payload, flags)
			}
			data, err := accountGet(cmd.Context(), flags, fullURL)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	return cmd
}

func (s accountGetSpec) resolve() (string, url.Values, error) {
	if s.PathFunc != nil {
		return s.PathFunc()
	}
	return s.Path, nil, nil
}

func accountURL(path string, query url.Values) (string, error) {
	base := strings.TrimRight(os.Getenv("SQUARESPACE_ACCOUNT_BASE_URL"), "/")
	if base == "" {
		base = defaultAccountBaseURL
	}
	u, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	if path == "" || !strings.HasPrefix(path, "/") {
		return "", fmt.Errorf("account API path must start with /")
	}
	u.Path = path
	u.RawQuery = query.Encode()
	return u.String(), nil
}

func accountGet(ctx context.Context, flags *rootFlags, fullURL string) (json.RawMessage, error) {
	cookie, err := accountCookieHeader()
	if err != nil {
		return nil, authErr(err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, usageErr(err)
	}
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Cookie", cookie)
	req.Header.Set("User-Agent", "squarespace-pp-cli/1.0")
	if csrf := strings.TrimSpace(os.Getenv("SQUARESPACE_ACCOUNT_CSRF_TOKEN")); csrf != "" {
		req.Header.Set("X-CSRF-Token", csrf)
	}

	client := &http.Client{Timeout: flags.timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, apiErr(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	if err != nil {
		return nil, apiErr(err)
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, authErr(fmt.Errorf("Squarespace account API returned %d", resp.StatusCode))
	}
	contentType := resp.Header.Get("Content-Type")
	finalURL := ""
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
		if strings.Contains(resp.Request.URL.Host, "login.squarespace.com") {
			return nil, authErr(fmt.Errorf("Squarespace redirected to login; refresh SQUARESPACE_ACCOUNT_COOKIE"))
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, apiErr(fmt.Errorf("Squarespace account API returned %d: %s", resp.StatusCode, firstLine(body)))
	}
	if isJSONContent(contentType, body) {
		return redactSensitiveJSON(body), nil
	}
	return accountHTMLSummary(resp.StatusCode, contentType, finalURL, body)
}

type accountDomainMeta struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	WebsiteID      string `json:"websiteId"`
	SubscriptionID string `json:"subscriptionId"`
}

func resolveAccountDomainID(ctx context.Context, flags *rootFlags, name string) (string, string, error) {
	meta, fullURL, err := resolveAccountDomainMeta(ctx, flags, name)
	return meta.ID, fullURL, err
}

func accountPathEscape(value string) string {
	if strings.HasPrefix(value, "resolved-") && strings.HasSuffix(value, "-id") {
		return value
	}
	return url.PathEscape(value)
}

func accountDryRunURL(value string) string {
	return value
}

func resolveAccountDomainMeta(ctx context.Context, flags *rootFlags, name string) (accountDomainMeta, string, error) {
	path := "/api/account/1/domains/byName/" + url.PathEscape(name)
	fullURL, err := accountURL(path, nil)
	if err != nil {
		return accountDomainMeta{}, "", usageErr(err)
	}
	if flags.dryRun {
		return accountDomainMeta{
			ID:             "resolved-domain-id",
			Name:           name,
			WebsiteID:      "resolved-website-id",
			SubscriptionID: "resolved-contract-id",
		}, fullURL, nil
	}
	data, err := accountGet(ctx, flags, fullURL)
	if err != nil {
		return accountDomainMeta{}, fullURL, err
	}
	var payload accountDomainMeta
	if err := json.Unmarshal(data, &payload); err != nil {
		return accountDomainMeta{}, fullURL, apiErr(fmt.Errorf("decode domain lookup response: %w", err))
	}
	if payload.ID == "" {
		return accountDomainMeta{}, fullURL, notFoundErr(fmt.Errorf("domain %q lookup did not include an id", name))
	}
	if payload.Name == "" {
		payload.Name = name
	}
	return payload, fullURL, nil
}

func accountCookieHeader() (string, error) {
	if cookie := strings.TrimSpace(os.Getenv("SQUARESPACE_ACCOUNT_COOKIE")); cookie != "" {
		return cookie, nil
	}
	path := strings.TrimSpace(os.Getenv("SQUARESPACE_ACCOUNT_COOKIE_FILE"))
	if path == "" {
		return "", fmt.Errorf("missing Squarespace account cookie; set SQUARESPACE_ACCOUNT_COOKIE or SQUARESPACE_ACCOUNT_COOKIE_FILE")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read SQUARESPACE_ACCOUNT_COOKIE_FILE: %w", err)
	}
	if cookie := strings.TrimSpace(string(data)); cookie != "" {
		return cookie, nil
	}
	return "", fmt.Errorf("SQUARESPACE_ACCOUNT_COOKIE_FILE is empty")
}

func isJSONContent(contentType string, body []byte) bool {
	if strings.Contains(strings.ToLower(contentType), "json") {
		return true
	}
	body = bytes.TrimSpace(body)
	return len(body) > 0 && (body[0] == '{' || body[0] == '[')
}

func redactSensitiveJSON(body []byte) json.RawMessage {
	var value any
	if err := json.Unmarshal(body, &value); err != nil {
		return body
	}
	value = redactSensitiveValue(value)
	out, err := json.Marshal(value)
	if err != nil {
		return body
	}
	return out
}

func redactSensitiveValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, val := range v {
			if isSensitiveAccountKey(key) {
				out[key] = "[redacted]"
				continue
			}
			out[key] = redactSensitiveValue(val)
		}
		return out
	case []any:
		for i := range v {
			v[i] = redactSensitiveValue(v[i])
		}
		return v
	default:
		return value
	}
}

func isSensitiveAccountKey(key string) bool {
	k := strings.ToLower(key)
	return strings.Contains(k, "token") ||
		strings.Contains(k, "secret") ||
		strings.Contains(k, "cookie") ||
		strings.Contains(k, "password") ||
		strings.Contains(k, "session") ||
		k == "crumb"
}

func accountHTMLSummary(status int, contentType, finalURL string, body []byte) (json.RawMessage, error) {
	payload := map[string]any{
		"status":       status,
		"content_type": contentType,
		"final_url":    finalURL,
		"title":        htmlTitle(body),
		"bytes":        len(body),
	}
	out, err := json.Marshal(payload)
	return out, err
}

var titleRE = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)

func htmlTitle(body []byte) string {
	match := titleRE.FindSubmatch(body)
	if len(match) < 2 {
		return ""
	}
	title := string(match[1])
	title = strings.Join(strings.Fields(title), " ")
	return title
}

func firstLine(body []byte) string {
	line := string(bytes.TrimSpace(body))
	if idx := strings.IndexByte(line, '\n'); idx >= 0 {
		line = line[:idx]
	}
	if len(line) > 240 {
		line = line[:240] + "..."
	}
	return line
}
