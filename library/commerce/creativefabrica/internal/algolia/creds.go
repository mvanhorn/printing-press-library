package algolia

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/creativefabrica/internal/cliutil"
)

// Creds is the public Algolia application id + search-only key used for catalog
// queries. Neither value is a write credential; the key is the same one the
// public website ships in its JavaScript. Values are never compiled into the
// binary.
type Creds struct {
	AppID  string `json:"app_id"`
	APIKey string `json:"api_key"`
	Source string `json:"-"` // "env" | "cache" | "discovered"
}

const (
	envAppID  = "CREATIVEFABRICA_ALGOLIA_APP_ID"
	envAPIKey = "CREATIVEFABRICA_ALGOLIA_API_KEY"
)

// CredsPath returns the on-disk cache location for discovered credentials.
func CredsPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "creativefabrica-pp-cli", "algolia-creds.json")
}

// ResolveCreds resolves catalog credentials in order: env vars, the local cache
// file, then best-effort live discovery from the site's JavaScript. The
// returned Creds.Source records which path produced them.
func ResolveCreds(ctx context.Context, httpClient *http.Client) (Creds, error) {
	// 1. Environment.
	if k := strings.TrimSpace(os.Getenv(envAPIKey)); k != "" {
		appID := strings.TrimSpace(os.Getenv(envAppID))
		if appID == "" {
			appID = DefaultAppID
		}
		return Creds{AppID: appID, APIKey: k, Source: "env"}, nil
	}

	// 2. Cache file.
	if c, ok := readCachedCreds(); ok {
		return c, nil
	}

	// 3. Best-effort live discovery.
	if c, err := discoverCreds(ctx, httpClient); err == nil {
		_ = writeCachedCreds(c)
		c.Source = "discovered"
		return c, nil
	}

	return Creds{}, fmt.Errorf(
		"no Creative Fabrica catalog key available: set %s to the public search key "+
			"(find it in any creativefabrica.com search request's x-algolia-api-key header), "+
			"or run 'creativefabrica-pp-cli auth set-key <key>'", envAPIKey)
}

// SaveCreds writes credentials to the local cache, e.g. from `auth set-key`.
func SaveCreds(appID, apiKey string) error {
	if appID == "" {
		appID = DefaultAppID
	}
	return writeCachedCreds(Creds{AppID: appID, APIKey: apiKey})
}

func readCachedCreds() (Creds, bool) {
	data, err := os.ReadFile(CredsPath())
	if err != nil {
		return Creds{}, false
	}
	var c Creds
	if err := json.Unmarshal(data, &c); err != nil || c.APIKey == "" {
		return Creds{}, false
	}
	if c.AppID == "" {
		c.AppID = DefaultAppID
	}
	c.Source = "cache"
	return c, true
}

func writeCachedCreds(c Creds) error {
	p := CredsPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	// #nosec G117 -- APIKey is Algolia's public, search-only, referer-restricted
	// key (the same value the public website ships in its JavaScript), not a write
	// credential; persisting it to a 0o600 cache file is safe by design.
	data, _ := json.Marshal(struct {
		AppID  string `json:"app_id"`
		APIKey string `json:"api_key"`
	}{c.AppID, c.APIKey})
	return os.WriteFile(p, data, 0o600)
}

var (
	scriptSrcRe = regexp.MustCompile(`<script[^>]+src="(https://cf-web-assets\.creativefabrica\.com/[^"]+\.js)"`)
	prodKeyRe   = regexp.MustCompile(`"productsAlgoliaApiKey",0,"([0-9a-f]{32})"`)
	prodAppRe   = regexp.MustCompile(`"productsAlgoliaAppId",0,"([A-Z0-9]{6,})"`)
	envKeyRe    = regexp.MustCompile(`"([0-9a-f]{32})","NEXT_PUBLIC_ALGOLIA_API_KEY`)
	envAppRe    = regexp.MustCompile(`"([A-Z0-9]{6,})","NEXT_PUBLIC_ALGOLIA_APP_ID`)
)

// discoverCreds fetches the live site, finds the JS bundle that configures
// Algolia, and extracts the public app id + search key. Best-effort: the
// homepage may be behind a bot challenge for plain HTTP, in which case this
// returns an error and the caller falls back to env/cache or guidance.
func discoverCreds(ctx context.Context, httpClient *http.Client) (Creds, error) {
	lim := cliutil.NewAdaptiveLimiter(4)
	html, err := getText(ctx, httpClient, lim, "https://www.creativefabrica.com/")
	if err != nil {
		return Creds{}, err
	}
	srcs := uniqueStrings(scriptSrcRe.FindAllStringSubmatch(html, -1))
	if len(srcs) == 0 {
		return Creds{}, fmt.Errorf("no JS bundle references found on homepage")
	}
	// Probe up to a bounded number of chunks; the config chunk is small.
	sort.Strings(srcs)
	if len(srcs) > 60 {
		srcs = srcs[:60]
	}
	for _, src := range srcs {
		body, err := getText(ctx, httpClient, lim, src)
		if err != nil {
			continue
		}
		if k := prodKeyRe.FindStringSubmatch(body); k != nil {
			appID := DefaultAppID
			if a := prodAppRe.FindStringSubmatch(body); a != nil {
				appID = a[1]
			}
			return Creds{AppID: appID, APIKey: k[1]}, nil
		}
		if k := envKeyRe.FindStringSubmatch(body); k != nil {
			appID := DefaultAppID
			if a := envAppRe.FindStringSubmatch(body); a != nil {
				appID = a[1]
			}
			return Creds{AppID: appID, APIKey: k[1]}, nil
		}
	}
	return Creds{}, fmt.Errorf("Algolia config not found in site bundle")
}

func getText(ctx context.Context, httpClient *http.Client, lim *cliutil.AdaptiveLimiter, rawurl string) (string, error) {
	if lim != nil {
		lim.Wait()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawurl, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/javascript,*/*")
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		if lim != nil {
			lim.OnRateLimit()
		}
		return "", &cliutil.RateLimitError{URL: rawurl, RetryAfter: 0, Body: "rate limited during credential discovery"}
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GET %s: HTTP %d", rawurl, resp.StatusCode)
	}
	if lim != nil {
		lim.OnSuccess()
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func uniqueStrings(matches [][]string) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range matches {
		if len(m) > 1 && !seen[m[1]] {
			seen[m[1]] = true
			out = append(out, m[1])
		}
	}
	return out
}
