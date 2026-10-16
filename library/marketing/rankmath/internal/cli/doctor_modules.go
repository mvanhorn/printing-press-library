// Copyright 2026 ElizabethcStone and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source live
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const maxIndexBytes = 8 << 20

type doctorModuleAvailability struct {
	Analytics       bool `json:"analytics"`
	RankTracking    bool `json:"rank_tracking"`
	URLInspection   bool `json:"url_inspection"`
	InstantIndexing bool `json:"instant_indexing"`
	LinkGenius      bool `json:"link_genius"`
	AIVisibility    bool `json:"ai_visibility"`
	HeadlessGetHead bool `json:"headless_get_head"`
	StatusTools     bool `json:"status_tools"`
}

type siteModules struct {
	RankMathInstalled bool                     `json:"rankmath_installed"`
	Modules           doctorModuleAvailability `json:"modules"`
	RankMathRoutes    int                      `json:"rankmath_routes"`
}

type doctorModulesSite struct {
	Site      string `json:"site"`
	Reachable bool   `json:"reachable"`
	siteModules
	Error string `json:"error"`
}

func modulesFromIndex(raw json.RawMessage) (siteModules, error) {
	var index struct {
		Namespaces []string                   `json:"namespaces"`
		Routes     map[string]json.RawMessage `json:"routes"`
	}
	if err := json.Unmarshal(raw, &index); err != nil {
		return siteModules{}, fmt.Errorf("decode WordPress route index: %w", err)
	}
	result := siteModules{Modules: doctorModuleAvailability{}}
	for _, namespace := range index.Namespaces {
		if namespace == "rankmath/v1" {
			result.RankMathInstalled = true
			break
		}
	}
	for route := range index.Routes {
		if strings.HasPrefix(route, "/rankmath/v1/") {
			result.RankMathRoutes++
		}
	}
	_, result.Modules.Analytics = index.Routes["/rankmath/v1/an/dashboard"]
	_, result.Modules.RankTracking = index.Routes["/rankmath/v1/an/getTrackedKeywords"]
	_, result.Modules.URLInspection = index.Routes["/rankmath/v1/an/inspectionResults"]
	_, result.Modules.InstantIndexing = index.Routes["/rankmath/v1/in/submitUrls"]
	_, result.Modules.LinkGenius = index.Routes["/rankmath/v1/link-genius/links"]
	_, result.Modules.AIVisibility = index.Routes["/rankmath/v1/ai-visibility/overview"]
	_, result.Modules.HeadlessGetHead = index.Routes["/rankmath/v1/getHead"]
	_, result.Modules.StatusTools = index.Routes["/rankmath/v1/status/exportSettings"]
	return result, nil
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		d, _, err := root.Find([]string{"doctor"})
		if err == nil {
			addNovelCommandIfAbsent(d, newDoctorModulesCmd(flags))
		}
	})
}

func newDoctorModulesCmd(flags *rootFlags) *cobra.Command {
	sites := strings.Join(siteListFromFlagOrEnv(""), ",")
	cmd := &cobra.Command{
		Use:     "modules",
		Short:   "Check installed Rank Math modules across WordPress sites",
		Example: "  rankmath-pp-cli doctor modules --sites example.com,example.org --json",
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "live",
			"pp:happy-args":  "--sites=example.com",
		},
	}
	cmd.Flags().StringVar(&sites, "sites", sites, "Comma-separated WordPress site hosts")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 && cmd.Flags().NFlag() == 0 && sites == "" {
			return cmd.Help()
		}
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "doctor modules")
		}
		hosts := siteListFromFlagOrEnv(sites)
		if len(args) != 0 || len(hosts) == 0 {
			return usageErr(fmt.Errorf("--sites or RANKMATH_SITE is required (RANKMATH_SITES is also supported)"))
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		timeout := flags.timeout
		if timeout == 0 {
			timeout = 30 * time.Second
		}
		client := &http.Client{Timeout: timeout}
		result := struct {
			Sites []doctorModulesSite `json:"sites"`
		}{Sites: make([]doctorModulesSite, 0, len(hosts))}
		for _, host := range hosts {
			row := doctorModulesSite{Site: host}
			target, err := doctorModulesIndexURL(host)
			if err != nil {
				row.Error = err.Error()
				result.Sites = append(result.Sites, row)
				continue
			}
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
			if err == nil {
				req.Header.Set("User-Agent", "rankmath-pp-cli")
				var resp *http.Response
				resp, err = client.Do(req)
				if err == nil {
					body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxIndexBytes+1))
					resp.Body.Close()
					if len(body) > maxIndexBytes {
						err = fmt.Errorf("WordPress route index exceeds 8 MiB")
					} else if readErr != nil {
						err = readErr
					} else if resp.StatusCode < 200 || resp.StatusCode >= 300 {
						err = fmt.Errorf("WordPress route index returned HTTP %d", resp.StatusCode)
					} else {
						row.Reachable = true
						modules, decodeErr := modulesFromIndex(body)
						row.siteModules = modules
						err = decodeErr
					}
				}
			}
			if err != nil {
				row.Error = err.Error()
			}
			result.Sites = append(result.Sites, row)
		}
		if wantsHumanTable(cmd.OutOrStdout(), flags) {
			rows := make([]map[string]any, 0, len(result.Sites))
			for _, site := range result.Sites {
				rows = append(rows, map[string]any{"site": site.Site, "reachable": site.Reachable, "rankmath_installed": site.RankMathInstalled, "rankmath_routes": site.RankMathRoutes, "modules": site.Modules, "error": site.Error})
			}
			return printAutoTable(cmd.OutOrStdout(), rows)
		}
		return printJSONFiltered(cmd.OutOrStdout(), result, flags)
	}
	return cmd
}

func doctorModulesIndexURL(site string) (string, error) {
	u, err := normalizeSiteURL(site)
	if err != nil {
		return "", err
	}
	if base := strings.TrimSpace(os.Getenv("RANKMATH_BASE_URL")); base != "" {
		baseURL, baseErr := normalizeSiteURL(base)
		if baseErr != nil {
			return "", fmt.Errorf("invalid RANKMATH_BASE_URL")
		}
		if strings.EqualFold(baseURL.Host, u.Host) {
			baseURL.Path = strings.TrimRight(baseURL.Path, "/") + "/"
			return baseURL.String(), nil
		}
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/wp-json/"
	return u.String(), nil
}
