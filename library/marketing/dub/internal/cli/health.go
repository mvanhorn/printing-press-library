// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/spf13/cobra"
)

type healthReport struct {
	APIReachable      bool              `json:"api_reachable"`
	RateLimit         map[string]string `json:"rate_limit_headers,omitempty"`
	ExpiredActive     int               `json:"expired_but_active_links"`
	UnverifiedDomains []string          `json:"unverified_domains,omitempty"`
	DormantTags       []string          `json:"dormant_tags,omitempty"`
	DeadDestinations  []deadDestination `json:"dead_destination_samples,omitempty"`
	StoreStaleness    string            `json:"store_staleness,omitempty"`
	Warnings          []string          `json:"warnings,omitempty"`
}

type deadDestination struct {
	LinkID    string `json:"link_id"`
	ShortLink string `json:"shortLink"`
	URL       string `json:"url"`
	Status    int    `json:"status"`
	Error     string `json:"error,omitempty"`
}

func newHealthCmd(flags *rootFlags) *cobra.Command {
	var probe int
	var skipProbe bool

	cmd := &cobra.Command{
		Use:   "health",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Workspace health doctor — rate limit, expired-but-active links, dead destinations",
		Long: `Cross-resource Monday-morning health report:
  - API reachability and current rate-limit headroom
  - links flagged as expired (expiresAt past) but not archived (cleanup work)
  - unverified custom domains
  - dormant tags (no associated links)
  - sample destination URLs HEAD-probed for 4xx/5xx (--probe N)

Combines a single live API call with the local store. Run sync first.`,
		Example: `  dub-pp-cli health --json
  dub-pp-cli health --probe 25 --agent
  dub-pp-cli health --no-probe`,
		RunE: func(cmd *cobra.Command, args []string) error {
			report := healthReport{
				RateLimit: map[string]string{},
			}
			c, err := flags.newClient()
			if err == nil {
				_, err := c.Get("/links/count", map[string]string{})
				if err == nil {
					report.APIReachable = true
				} else {
					report.APIReachable = false
					report.Warnings = append(report.Warnings, fmt.Sprintf("API unreachable: %v", err))
				}
			} else {
				report.Warnings = append(report.Warnings, fmt.Sprintf("client init failed: %v", err))
			}

			db, err := openStoreForRead("dub-pp-cli")
			if err != nil {
				report.Warnings = append(report.Warnings, fmt.Sprintf("store open failed: %v", err))
			}
			if db != nil {
				defer db.Close()

				// expired but active
				now := nowFunc()
				lRows, err := db.DB().Query(`SELECT id, data, archived, expires_at FROM links`)
				var probeTargets []struct{ id, shortLink, url string }
				if err == nil {
					for lRows.Next() {
						var id, raw, expiresAt string
						var archived int
						if err := lRows.Scan(&id, &raw, &archived, &expiresAt); err != nil {
							continue
						}
						if expiresAt != "" {
							if t, perr := parseTimestamp(expiresAt); perr == nil && t.Before(now) && archived == 0 {
								report.ExpiredActive++
							}
						}
						if !skipProbe && len(probeTargets) < probe {
							var obj map[string]any
							if json.Unmarshal([]byte(raw), &obj) == nil {
								u := stringField(obj, "url")
								if u != "" {
									probeTargets = append(probeTargets, struct{ id, shortLink, url string }{
										id:        id,
										shortLink: stringField(obj, "shortLink"),
										url:       u,
									})
								}
							}
						}
					}
					lRows.Close()
				}

				// unverified domains
				dRows, err := db.DB().Query(`SELECT data FROM domains`)
				if err == nil {
					for dRows.Next() {
						var raw string
						if err := dRows.Scan(&raw); err != nil {
							continue
						}
						var obj map[string]any
						if json.Unmarshal([]byte(raw), &obj) != nil {
							continue
						}
						verified, _ := obj["verified"].(bool)
						if !verified {
							slug := stringField(obj, "slug")
							if slug != "" {
								report.UnverifiedDomains = append(report.UnverifiedDomains, slug)
							}
						}
					}
					dRows.Close()
				}

				// dormant tags (tags not referenced by any link)
				tRows, err := db.DB().Query(`SELECT name FROM tags WHERE name NOT IN (SELECT DISTINCT json_extract(value, '$') FROM links, json_each(json_extract(data, '$.tagNames')) WHERE json_extract(data, '$.tagNames') IS NOT NULL)`)
				if err == nil {
					for tRows.Next() {
						var name string
						if err := tRows.Scan(&name); err == nil && name != "" {
							report.DormantTags = append(report.DormantTags, name)
						}
					}
					tRows.Close()
				}

				// store staleness
				oldest := time.Time{}
				resources := []string{"links", "domains", "tags", "partners"}
				for _, r := range resources {
					if _, syncedAt, _, err := db.GetSyncState(r); err == nil && !syncedAt.IsZero() {
						if oldest.IsZero() || syncedAt.Before(oldest) {
							oldest = syncedAt
						}
					}
				}
				if !oldest.IsZero() {
					report.StoreStaleness = nowFunc().Sub(oldest).Round(time.Minute).String()
				}

				// dead-destination probe
				if !skipProbe && len(probeTargets) > 0 {
					report.DeadDestinations = probeDestinations(probeTargets)
				}
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, report)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "API reachable: %v\n", report.APIReachable)
			if len(report.RateLimit) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "Rate limit:")
				for k, v := range report.RateLimit {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s: %s\n", k, v)
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Expired-but-active links: %d\n", report.ExpiredActive)
			fmt.Fprintf(cmd.OutOrStdout(), "Unverified domains: %d\n", len(report.UnverifiedDomains))
			fmt.Fprintf(cmd.OutOrStdout(), "Dormant tags: %d\n", len(report.DormantTags))
			fmt.Fprintf(cmd.OutOrStdout(), "Dead destinations (sampled %d): %d\n", probe, len(report.DeadDestinations))
			if report.StoreStaleness != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Local store staleness: %s\n", report.StoreStaleness)
			}
			if len(report.Warnings) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "Warnings:")
				for _, w := range report.Warnings {
					fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", w)
				}
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&probe, "probe", 10, "How many destination URLs to HEAD-probe for 4xx/5xx")
	cmd.Flags().BoolVar(&skipProbe, "no-probe", false, "Skip destination URL probing (faster, no outbound HTTP)")
	return cmd
}

// probeDestinations HEAD-checks a small sample of URLs in parallel.
func probeDestinations(targets []struct{ id, shortLink, url string }) []deadDestination {
	out := make([]deadDestination, 0)
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 5) // 5 concurrent probes
	client := &http.Client{Timeout: 5 * time.Second}

	for _, t := range targets {
		wg.Add(1)
		sem <- struct{}{}
		go func(t struct{ id, shortLink, url string }) {
			defer wg.Done()
			defer func() { <-sem }()
			req, err := http.NewRequest(http.MethodHead, t.url, nil)
			if err != nil {
				mu.Lock()
				out = append(out, deadDestination{LinkID: t.id, ShortLink: t.shortLink, URL: t.url, Error: err.Error()})
				mu.Unlock()
				return
			}
			req.Header.Set("User-Agent", "dub-pp-cli/health")
			resp, err := client.Do(req)
			if err != nil {
				mu.Lock()
				out = append(out, deadDestination{LinkID: t.id, ShortLink: t.shortLink, URL: t.url, Error: err.Error()})
				mu.Unlock()
				return
			}
			resp.Body.Close()
			if resp.StatusCode >= 400 {
				mu.Lock()
				out = append(out, deadDestination{LinkID: t.id, ShortLink: t.shortLink, URL: t.url, Status: resp.StatusCode})
				mu.Unlock()
			}
		}(t)
	}
	wg.Wait()
	return out
}
