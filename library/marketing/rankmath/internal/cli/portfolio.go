// Copyright 2026 ElizabethcStone and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source live
package cli

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/mvanhorn/printing-press-library/library/marketing/rankmath/internal/client"
	"github.com/spf13/cobra"
)

type portfolioSite struct {
	Site              string  `json:"site"`
	Clicks            float64 `json:"clicks"`
	ClicksChange      float64 `json:"clicks_change"`
	Impressions       float64 `json:"impressions"`
	ImpressionsChange float64 `json:"impressions_change"`
	Position          float64 `json:"position"`
	PositionChange    float64 `json:"position_change"`
	CTR               float64 `json:"ctr"`
	Status            string  `json:"status"`
	Error             string  `json:"error"`
}
type portfolioFailure struct {
	Site  string `json:"site"`
	Error string `json:"error"`
}
type portfolioResult struct {
	Sites         []portfolioSite    `json:"sites"`
	FetchFailures []portfolioFailure `json:"fetch_failures"`
	Totals        struct {
		Clicks       float64 `json:"clicks"`
		Impressions  float64 `json:"impressions"`
		SitesCounted int     `json:"sites_counted"`
	} `json:"totals"`
}

func siteEnvSlug(host string) string {
	if u, err := url.Parse("//" + host); err == nil && u.Host != "" {
		host = u.Host
	}
	return strings.Map(func(r rune) rune {
		if r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			return r
		}
		return '_'
	}, strings.ToUpper(host))
}
func decodeSummary(raw json.RawMessage) (portfolioSite, error) {
	s := portfolioSite{Status: "ok"}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return s, err
	}
	for name, pair := range map[string][2]*float64{"clicks": {&s.Clicks, &s.ClicksChange}, "impressions": {&s.Impressions, &s.ImpressionsChange}, "position": {&s.Position, &s.PositionChange}, "ctr": {&s.CTR, nil}} {
		b := obj[name]
		if len(b) == 0 || string(b) == "null" || string(b) == `"n/a"` {
			s.Status = "not_connected"
			continue
		}
		var m map[string]json.RawMessage
		if err := json.Unmarshal(b, &m); err != nil {
			return s, fmt.Errorf("%s: %w", name, err)
		}
		for i, key := range []string{"total", "difference"} {
			if pair[i] == nil {
				continue
			}
			v := m[key]
			if len(v) == 0 || string(v) == "null" || string(v) == `"n/a"` {
				if i == 0 {
					s.Status = "not_connected"
				}
				continue
			}
			if err := json.Unmarshal(v, pair[i]); err != nil {
				return s, fmt.Errorf("%s.%s: %w", name, key, err)
			}
		}
	}
	return s, nil
}
func newNovelPortfolioCmd(flags *rootFlags) *cobra.Command {
	sites := strings.Join(siteListFromFlagOrEnv(""), ",")
	cmd := &cobra.Command{
		Use:         "portfolio",
		Short:       "Compare Search Console metrics across sites.",
		Example:     "  rankmath-pp-cli portfolio --sites example.com,example.org --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
	}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 && cmd.Flags().NFlag() == 0 && len(siteListFromFlagOrEnv(sites)) == 0 {
			return cmd.Help()
		}
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "portfolio")
		}
		hosts := siteListFromFlagOrEnv(sites)
		if len(args) > 0 || len(hosts) == 0 {
			return usageErr(fmt.Errorf("--sites or RANKMATH_SITES/RANKMATH_SITE is required"))
		}
		slugs := make(map[string]string, len(hosts))
		for _, host := range hosts {
			u, err := normalizeSiteURL(host)
			if err != nil {
				continue
			}
			slug := siteEnvSlug(u.Host)
			if previous, ok := slugs[slug]; ok && !strings.EqualFold(previous, u.Host) {
				return usageErr(fmt.Errorf("sites %q and %q produce the same credential environment slug %q", previous, u.Host, slug))
			}
			slugs[slug] = u.Host
		}
		c, err := flags.newClient()
		if err != nil {
			return err
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		defaultAuthHeader, _ := novelAuthHeader(flags)
		defaultSite, defaultSiteErr := normalizeSiteURL(os.Getenv("RANKMATH_SITE"))
		result := portfolioResult{Sites: make([]portfolioSite, len(hosts)), FetchFailures: make([]portfolioFailure, 0)}
		sem := make(chan struct{}, 4)
		var wg sync.WaitGroup
		for i, host := range hosts {
			wg.Add(1)
			go func(i int, host string) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				row := portfolioSite{Site: host, Status: "error"}
				cfg := *c.Config
				cfg.TemplateVars = make(map[string]string, len(c.Config.TemplateVars))
				for k, v := range c.Config.TemplateVars {
					cfg.TemplateVars[k] = v
				}
				u, e := normalizeSiteURL(host)
				if e != nil {
					row.Error = e.Error()
					result.Sites[i] = row
					return
				}
				cfg.TemplateVars["site"] = u.Host
				cfg.BaseURL = u.Scheme + "://{site}" + strings.TrimRight(u.EscapedPath(), "/") + "/wp-json"
				slug := siteEnvSlug(u.Host)
				user, pass := os.Getenv("RANKMATH_USERNAME_"+slug), os.Getenv("RANKMATH_APP_PASSWORD_"+slug)
				authHeader := ""
				if user != "" && pass != "" {
					authHeader = "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+pass))
				} else if user == "" && pass == "" && defaultAuthHeader != "" && defaultSiteErr == nil && strings.EqualFold(u.Host, defaultSite.Host) {
					authHeader = defaultAuthHeader
				}
				if authHeader == "" {
					row.Status = "not_configured"
					row.Error = "credentials are not configured for this site"
					result.Sites[i] = row
					return
				}
				if u.Scheme != "https" {
					row.Status = "error"
					row.Error = "credentials require https; refusing to send auth over http"
					result.Sites[i] = row
					return
				}
				cfg.AuthHeaderVal = authHeader
				sc := client.New(&cfg, flags.timeout, flags.rateLimit)
				// Share only the transport: sc's own CheckRedirect is bound to this site's
				// credentials and keeps the downgrade, private-address and cross-origin guards.
				sc.HTTPClient.Transport = c.HTTPClient.Transport
				sc.NoCache = c.NoCache
				sc.SetTimeoutExplicit(flags.timeoutExplicit)
				raw, e := sc.Get(ctx, "/rankmath/v1/an/analyticsSummary", nil)
				if e == nil {
					row, e = decodeSummary(raw)
					row.Site = host
				}
				if e != nil {
					row.Status = "error"
					row.Error = e.Error()
				}
				result.Sites[i] = row
			}(i, host)
		}
		wg.Wait()
		for _, s := range result.Sites {
			if s.Status == "ok" {
				result.Totals.Clicks += s.Clicks
				result.Totals.Impressions += s.Impressions
				result.Totals.SitesCounted++
			} else if s.Status == "error" {
				result.FetchFailures = append(result.FetchFailures, portfolioFailure{Site: s.Site, Error: s.Error})
			}
		}
		failed, notConfigured := len(result.FetchFailures), 0
		for _, s := range result.Sites {
			if s.Status == "not_configured" {
				notConfigured++
			}
		}
		if failed > 0 || notConfigured > 0 {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d sites failed, %d not configured; totals computed over %d sites\n", failed, len(result.Sites), notConfigured, result.Totals.SitesCounted)
		}
		if !wantsHumanTable(cmd.OutOrStdout(), flags) {
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		}
		rows := make([][]string, 0, len(result.Sites))
		for _, s := range result.Sites {
			rows = append(rows, []string{s.Site, fmt.Sprint(s.Clicks), fmt.Sprint(s.Impressions), fmt.Sprint(s.Position), s.Status})
		}
		return flags.printTable(cmd, []string{"Site", "Clicks", "Impressions", "Position", "Status"}, rows)
	}
	cmd.Flags().StringVar(&sites, "sites", sites, "Comma-separated site hosts")
	return cmd
}
