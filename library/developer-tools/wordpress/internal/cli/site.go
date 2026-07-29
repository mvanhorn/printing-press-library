package cli

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/wordpress/internal/client"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/wordpress/internal/config"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/wordpress/internal/store"
	"github.com/spf13/cobra"
)

func init() {
	registerClientHook(func(c *client.Client) error {
		if c == nil || c.Config == nil {
			return nil
		}
		if err := config.ApplyActiveWordPressSite(c.Config); err != nil {
			return configErr(err)
		}
		c.BaseURL = strings.TrimRight(c.Config.BaseURL, "/")
		return nil
	})
}

const maxSiteDiscoveryBody = 8 << 20

var (
	htmlHeadRE     = regexp.MustCompile(`(?is)<head\b[^>]*>(.*?)</head\s*>`)
	htmlLinkRE     = regexp.MustCompile(`(?is)<link\b[^>]*>`)
	htmlAttrRE     = regexp.MustCompile("(?is)([a-zA-Z_:][-a-zA-Z0-9_:.]*)\\s*=\\s*(?:\"([^\"]*)\"|'([^']*)'|([^\\s\"'=<>]+))")
	siteNameCharRE = regexp.MustCompile(`[^a-z0-9-]`)
)

type wordpressRootDocument struct {
	Name           string          `json:"name"`
	Description    string          `json:"description"`
	URL            string          `json:"url"`
	Home           string          `json:"home"`
	Namespaces     []string        `json:"namespaces"`
	Authentication json.RawMessage `json:"authentication"`
}

type discoveredWordPressSite struct {
	RootURL string
	Root    wordpressRootDocument
}

func newSiteCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "site",
		Short:   "Register, list, switch, and remove the WordPress sites this CLI targets",
		Example: "  wordpress-pp-cli site add https://example.org",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			_ = cmd.Usage()
			return usageErr(fmt.Errorf("a site subcommand is required"))
		},
	}
	cmd.AddCommand(newSiteAddCmd(flags))
	cmd.AddCommand(newSiteListCmd(flags))
	cmd.AddCommand(newSiteUseCmd(flags))
	cmd.AddCommand(newSiteRemoveCmd(flags))
	cmd.AddCommand(newSiteCurrentCmd(flags))
	return cmd
}

func newSiteAddCmd(flags *rootFlags) *cobra.Command {
	var requestedName string
	var user string
	var appPassword string

	cmd := &cobra.Command{
		Use:     "add <url>",
		Short:   "Discover and register a WordPress site",
		Example: "  wordpress-pp-cli site add https://example.org\n  wordpress-pp-cli site add example.org --name client-blog --user editor --app-password secret",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				target := "the supplied WordPress site"
				if len(args) > 0 {
					target = args[0]
				}
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"action": "site_add", "url": target, "dry_run": true}, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Would discover and add %s.\n", target)
				return nil
			}
			if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("site URL is required"))
			}
			if len(args) > 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("site add accepts exactly one URL"))
			}
			if (user == "") != (appPassword == "") {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--user and --app-password must be provided together"))
			}

			normalized, err := normalizeSiteURL(args[0])
			if err != nil {
				return usageErr(err)
			}
			name := requestedName
			if name == "" {
				name = siteNameFromURL(normalized)
			}
			name = sanitizeWordPressSiteName(name)
			if name == "" {
				return usageErr(fmt.Errorf("site name is empty after normalization"))
			}

			registry, err := config.LoadSites(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			if _, exists := registry.Get(name); exists {
				return configErr(fmt.Errorf("site %q already exists; choose another --name or remove it first", name))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			httpClient := &http.Client{Timeout: flags.timeout}
			discovered, err := discoverWordPressSite(ctx, httpClient, normalized, user, appPassword)
			if err != nil {
				return apiErr(err)
			}

			authorizeURL := wordpressAuthorizeURL(discovered.Root.Authentication)
			siteURL := discovered.Root.URL
			if siteURL == "" {
				siteURL = normalized
			}
			site := config.Site{
				Name: name, BaseURL: strings.TrimRight(discovered.RootURL, "/"),
				SiteURL: siteURL, DisplayName: discovered.Root.Name,
				User: user, AppPassword: appPassword,
				RestRouteFallback: usesRestRouteFallback(discovered.RootURL),
				Namespaces:        append(make([]string, 0, len(discovered.Root.Namespaces)), discovered.Root.Namespaces...),
				AuthorizeURL:      authorizeURL, AddedAt: time.Now().UTC(),
			}
			registry.Sites[name] = site
			if registry.Active == "" {
				registry.Active = name
			}
			if err := registry.Save(); err != nil {
				return configErr(err)
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"added": true, "active": registry.Active == name,
					"credentials_set": user != "" && appPassword != "", "site": site,
				}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Added %s (%s)\n", name, site.SiteURL)
			fmt.Fprintf(cmd.OutOrStdout(), "REST root: %s\n", site.BaseURL)
			if authorizeURL != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "\nAPPLICATION PASSWORD AUTHORIZATION URL:\n%s\n", authorizeURL)
			} else {
				fmt.Fprintln(cmd.ErrOrStderr(), "warning: this site did not advertise an Application Password authorization URL")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&requestedName, "name", "", "Short registry name (defaults to the site's host)")
	cmd.Flags().StringVar(&user, "user", "", "WordPress username for this site")
	cmd.Flags().StringVar(&appPassword, "app-password", "", "WordPress Application Password for this site")
	return cmd
}

func newSiteListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "list",
		Short:       "List registered WordPress sites",
		Example:     "  wordpress-pp-cli site list --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// No bare-invocation help branch: `site list` takes no required
			// input, so running it bare must list, matching the framework's
			// own zero-arg list commands (e.g. `profile list`).
			if dryRunOK(flags) {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"action": "site_list", "dry_run": true}, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "Would list registered WordPress sites.")
				return nil
			}
			if len(args) > 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("site list does not accept arguments"))
			}
			registry, err := config.LoadSites(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			names := make([]string, 0, len(registry.Sites))
			for name := range registry.Sites {
				names = append(names, name)
			}
			sort.Strings(names)
			results := make([]map[string]any, 0, len(names))
			for _, name := range names {
				site := registry.Sites[name]
				row := map[string]any{
					"name": name, "site_url": site.SiteURL,
					"credentials_set": site.User != "" && site.AppPassword != "",
					"active":          registry.Active == name,
				}
				if lastSync := wordpressSiteLastSync(cmd.Context(), name); lastSync != "" {
					row["last_sync"] = lastSync
				}
				results = append(results, row)
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}
			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No WordPress sites configured.")
				return nil
			}
			return printAutoTable(cmd.OutOrStdout(), results)
		},
	}
}

func newSiteUseCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "use <name>",
		Short:   "Set the active WordPress site",
		Example: "  wordpress-pp-cli site use example-com",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				name := "the supplied site"
				if len(args) > 0 {
					name = args[0]
				}
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"action": "site_use", "name": name, "dry_run": true}, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Would make %s active.\n", name)
				return nil
			}
			if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("site name is required"))
			}
			if len(args) > 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("site use accepts exactly one name"))
			}
			registry, err := config.LoadSites(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			name := args[0]
			if _, ok := registry.Get(name); !ok {
				return notFoundErr(fmt.Errorf("site %q is not registered", name))
			}
			registry.Active = name
			if err := registry.Save(); err != nil {
				return configErr(err)
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"active_site": name}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Active WordPress site: %s\n", name)
			return nil
		},
	}
}

func newSiteCurrentCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "current",
		Short:       "Show the active WordPress site",
		Example:     "  wordpress-pp-cli site current --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// No bare-invocation help branch: `site current` takes no required
			// input, so running it bare must report the active site.
			if dryRunOK(flags) {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"action": "site_current", "dry_run": true}, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "Would show the active WordPress site.")
				return nil
			}
			if len(args) > 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("site current does not accept arguments"))
			}
			registry, err := config.LoadSites(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			site, ok := registry.ActiveSite()
			if !ok {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"active": false}, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "No active WordPress site configured.")
				return nil
			}
			result := map[string]any{
				"active": true, "name": site.Name, "site_url": site.SiteURL,
				"base_url":            site.BaseURL,
				"credentials_set":     site.User != "" && site.AppPassword != "",
				"rest_route_fallback": site.RestRouteFallback,
				"namespaces":          site.Namespaces, "authorize_url": site.AuthorizeURL,
			}
			if lastSync := wordpressSiteLastSync(cmd.Context(), site.Name); lastSync != "" {
				result["last_sync"] = lastSync
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			return printAutoTable(cmd.OutOrStdout(), []map[string]any{result})
		},
	}
}

func newSiteRemoveCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "remove <name>",
		Short:   "Remove a WordPress site from the registry",
		Example: "  wordpress-pp-cli site remove example-com --yes",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				name := "the supplied site"
				if len(args) > 0 {
					name = args[0]
				}
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"action": "site_remove", "name": name, "dry_run": true}, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Would remove %s.\n", name)
				return nil
			}
			if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("site name is required"))
			}
			if len(args) > 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("site remove accepts exactly one name"))
			}
			registry, err := config.LoadSites(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			name := args[0]
			if _, ok := registry.Get(name); !ok {
				return notFoundErr(fmt.Errorf("site %q is not registered", name))
			}
			if !flags.yes {
				if flags.noInput {
					return usageErr(fmt.Errorf("confirmation required: pass --yes to remove site %q", name))
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "Remove WordPress site %q? [y/N] ", name)
				answer, readErr := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
				if readErr != nil && readErr != io.EOF {
					return usageErr(fmt.Errorf("reading confirmation: %w", readErr))
				}
				answer = strings.ToLower(strings.TrimSpace(answer))
				if answer != "y" && answer != "yes" {
					return usageErr(fmt.Errorf("site removal cancelled; pass --yes to confirm non-interactively"))
				}
			}
			delete(registry.Sites, name)
			if registry.Active == name {
				registry.Active = ""
			}
			if err := registry.Save(); err != nil {
				return configErr(err)
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"removed": name}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed WordPress site %s.\n", name)
			return nil
		},
	}
}

func normalizeSiteURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("site URL is required")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid site URL %q: %w", raw, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("invalid site URL %q: scheme must be http or https", raw)
	}
	if u.Host == "" || u.Hostname() == "" {
		return "", fmt.Errorf("invalid site URL %q: host is required", raw)
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("invalid site URL %q: credentials, query strings, and fragments are not supported", raw)
	}
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	u.Path = strings.TrimRight(u.Path, "/")
	return u.String(), nil
}

func sanitizeWordPressSiteName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = siteNameCharRE.ReplaceAllString(name, "-")
	return strings.Trim(name, "-")
}

func siteNameFromURL(siteURL string) string {
	u, err := url.Parse(siteURL)
	if err != nil {
		return ""
	}
	return sanitizeWordPressSiteName(u.Hostname())
}

func wordpressRESTRootFromLinkHeaders(values []string) string {
	for _, value := range values {
		start := 0
		inAngle := false
		inQuote := byte(0)
		for i := 0; i <= len(value); i++ {
			atEnd := i == len(value)
			if !atEnd {
				switch value[i] {
				case '<':
					if inQuote == 0 {
						inAngle = true
					}
				case '>':
					if inQuote == 0 {
						inAngle = false
					}
				case '\'', '"':
					if !inAngle {
						if inQuote == value[i] {
							inQuote = 0
						} else if inQuote == 0 {
							inQuote = value[i]
						}
					}
				}
			}
			if !atEnd && (value[i] != ',' || inAngle || inQuote != 0) {
				continue
			}
			entry := strings.TrimSpace(value[start:i])
			start = i + 1
			left := strings.IndexByte(entry, '<')
			right := strings.IndexByte(entry, '>')
			if left < 0 || right <= left {
				continue
			}
			target := strings.TrimSpace(entry[left+1 : right])
			for _, parameter := range strings.Split(entry[right+1:], ";") {
				key, val, ok := strings.Cut(strings.TrimSpace(parameter), "=")
				if !ok || !strings.EqualFold(strings.TrimSpace(key), "rel") {
					continue
				}
				val = strings.Trim(strings.TrimSpace(val), "\"'")
				for _, relation := range strings.Fields(val) {
					if relation == "https://api.w.org/" {
						return target
					}
				}
			}
		}
	}
	return ""
}

func wordpressRESTRootFromHTML(document, baseURL string) string {
	head := document
	if match := htmlHeadRE.FindStringSubmatch(document); len(match) == 2 {
		head = match[1]
	}
	for _, tag := range htmlLinkRE.FindAllString(head, -1) {
		attributes := make(map[string]string)
		for _, match := range htmlAttrRE.FindAllStringSubmatch(tag, -1) {
			value := match[2]
			if value == "" {
				value = match[3]
			}
			if value == "" {
				value = match[4]
			}
			attributes[strings.ToLower(match[1])] = html.UnescapeString(value)
		}
		matched := false
		for _, relation := range strings.Fields(attributes["rel"]) {
			if relation == "https://api.w.org/" {
				matched = true
				break
			}
		}
		if !matched || attributes["href"] == "" {
			continue
		}
		base, baseErr := url.Parse(baseURL)
		href, hrefErr := url.Parse(attributes["href"])
		if baseErr == nil && hrefErr == nil {
			return base.ResolveReference(href).String()
		}
		return attributes["href"]
	}
	return ""
}

func wordpressAuthorizeURL(authentication json.RawMessage) string {
	var document struct {
		ApplicationPasswords struct {
			Endpoints struct {
				Authorization string `json:"authorization"`
			} `json:"endpoints"`
		} `json:"application-passwords"`
	}
	if json.Unmarshal(authentication, &document) != nil {
		return ""
	}
	return document.ApplicationPasswords.Endpoints.Authorization
}

func usesRestRouteFallback(root string) bool {
	u, err := url.Parse(root)
	if err != nil {
		return false
	}
	_, ok := u.Query()["rest_route"]
	return ok
}

func discoverWordPressSite(ctx context.Context, httpClient *http.Client, siteURL, user, appPassword string) (discoveredWordPressSite, error) {
	tried := make(map[string]struct{})
	var lastErr error
	tryRoot := func(candidate string) (discoveredWordPressSite, bool) {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			return discoveredWordPressSite{}, false
		}
		if _, seen := tried[candidate]; seen {
			return discoveredWordPressSite{}, false
		}
		tried[candidate] = struct{}{}
		root, err := fetchWordPressRoot(ctx, httpClient, candidate, user, appPassword)
		if err != nil {
			lastErr = err
			return discoveredWordPressSite{}, false
		}
		return discoveredWordPressSite{RootURL: candidate, Root: root}, true
	}

	headResp, headErr := doSiteDiscoveryRequest(ctx, httpClient, http.MethodHead, siteURL, user, appPassword)
	if headErr == nil {
		root := wordpressRESTRootFromLinkHeaders(headResp.Header.Values("Link"))
		root = resolveDiscoveryURL(headResp.Request.URL, root)
		_ = headResp.Body.Close()
		if discovered, ok := tryRoot(root); ok {
			return discovered, nil
		}
	} else {
		lastErr = headErr
	}

	homeResp, homeErr := doSiteDiscoveryRequest(ctx, httpClient, http.MethodGet, siteURL, user, appPassword)
	if homeErr == nil {
		body, readErr := io.ReadAll(io.LimitReader(homeResp.Body, maxSiteDiscoveryBody))
		_ = homeResp.Body.Close()
		if readErr == nil {
			root := wordpressRESTRootFromHTML(string(body), homeResp.Request.URL.String())
			if discovered, ok := tryRoot(root); ok {
				return discovered, nil
			}
		} else {
			lastErr = readErr
		}
	} else {
		lastErr = homeErr
	}

	base := strings.TrimRight(siteURL, "/")
	fallbacks := []string{
		base + "/wp-json/",
		base + "/?rest_route=/",
		base + "/index.php?rest_route=/",
	}
	for _, candidate := range fallbacks {
		if discovered, ok := tryRoot(candidate); ok {
			return discovered, nil
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no WordPress REST root found")
	}
	return discoveredWordPressSite{}, fmt.Errorf("discovering WordPress REST API at %s: %w", siteURL, lastErr)
}

func doSiteDiscoveryRequest(ctx context.Context, httpClient *http.Client, method, target, user, appPassword string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, target, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "wordpress-pp-cli/0.1.0")
	if user != "" && appPassword != "" {
		req.SetBasicAuth(user, appPassword)
	}
	return httpClient.Do(req)
}

func fetchWordPressRoot(ctx context.Context, httpClient *http.Client, rootURL, user, appPassword string) (wordpressRootDocument, error) {
	resp, err := doSiteDiscoveryRequest(ctx, httpClient, http.MethodGet, rootURL, user, appPassword)
	if err != nil {
		return wordpressRootDocument{}, fmt.Errorf("GET %s: %w", rootURL, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxSiteDiscoveryBody))
	if err != nil {
		return wordpressRootDocument{}, fmt.Errorf("reading REST root %s: %w", rootURL, err)
	}
	if resp.StatusCode != http.StatusOK {
		return wordpressRootDocument{}, fmt.Errorf("GET %s returned HTTP %d", rootURL, resp.StatusCode)
	}
	if !json.Valid(body) {
		return wordpressRootDocument{}, fmt.Errorf("GET %s returned invalid JSON", rootURL)
	}
	var root wordpressRootDocument
	if err := json.Unmarshal(body, &root); err != nil {
		return wordpressRootDocument{}, fmt.Errorf("decoding REST root %s: %w", rootURL, err)
	}
	hasCore := false
	for _, namespace := range root.Namespaces {
		if namespace == "wp/v2" {
			hasCore = true
			break
		}
	}
	if !hasCore {
		return wordpressRootDocument{}, fmt.Errorf("GET %s returned a WordPress API without the wp/v2 namespace", rootURL)
	}
	return root, nil
}

func resolveDiscoveryURL(base *url.URL, discovered string) string {
	if base == nil || discovered == "" {
		return discovered
	}
	reference, err := url.Parse(discovered)
	if err != nil {
		return discovered
	}
	return base.ResolveReference(reference).String()
}

func wordpressSiteLastSync(ctx context.Context, siteName string) string {
	path := wordpressSiteDBPath(siteName)
	if _, err := os.Stat(path); err != nil {
		return ""
	}
	db, err := store.OpenReadOnlyContext(ctx, path)
	if err != nil {
		return ""
	}
	defer db.Close()
	var last sql.NullString
	if err := db.DB().QueryRowContext(ctx, `SELECT MAX(last_synced_at) FROM sync_state`).Scan(&last); err != nil || !last.Valid {
		return ""
	}
	return last.String
}
