// Copyright 2026 alex-osti. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/cloud/cloudflare/internal/cliutil"
)

func newCacheCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Cache convenience commands (purge with verification)",
	}
	cmd.AddCommand(newCachePurgeCmd(flags))
	return cmd
}

func newCachePurgeCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "purge",
		Short: "Cache purge variants",
	}
	cmd.AddCommand(newCachePurgeReleaseCmd(flags))
	return cmd
}

func newCachePurgeReleaseCmd(flags *rootFlags) *cobra.Command {
	var (
		zone        string
		urls        []string
		tags        []string
		hosts       []string
		probe       string
		probeTimeout time.Duration
	)

	cmd := &cobra.Command{
		Use:   "release",
		Short: "Purge cache and verify with cf-cache-status probes",
		Long: `Purge cache by URL, tag, or host, then probe a URL twice and assert that
cf-cache-status transitions from MISS to HIT. Useful in deploy/release scripts
where downstream steps depend on cached content actually being purged.

If no purge target is provided, performs purge_everything.`,
		Example: `  cloudflare-pp-cli cache purge release --zone example.com --tags release-v1 --probe https://example.com/`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if zone == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"action":  "would_purge",
					"zone":    zone,
					"urls":    urls,
					"tags":    tags,
					"hosts":   hosts,
					"probe":   probe,
					"dry_run": true,
				}, flags)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			zoneID, err := resolveZoneID(c, zone)
			if err != nil {
				return notFoundErr(err)
			}

			body := map[string]any{}
			switch {
			case len(urls) > 0:
				body["files"] = urls
			case len(tags) > 0:
				body["tags"] = tags
			case len(hosts) > 0:
				body["hosts"] = hosts
			default:
				body["purge_everything"] = true
			}

			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"action": "would_purge",
					"zone":   zone,
					"body":   body,
					"probe":  probe,
				}, flags)
			}

			purgeResp, _, err := c.Post(fmt.Sprintf("/zones/%s/purge_cache", zoneID), body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			result := map[string]any{
				"action":  "purged",
				"zone":    zone,
				"purge_response": purgeResp,
			}

			if probe != "" && !cliutil.IsVerifyEnv() {
				ctx, cancel := context.WithTimeout(cmd.Context(), probeTimeout)
				defer cancel()
				hc := &http.Client{Timeout: 10 * time.Second}
				attempts := []map[string]string{}
				deadline := time.Now().Add(probeTimeout)
				sawMiss := false
				sawHit := false
				for time.Now().Before(deadline) {
					req, _ := http.NewRequestWithContext(ctx, "HEAD", probe, nil)
					resp, perr := hc.Do(req)
					if perr != nil {
						attempts = append(attempts, map[string]string{"error": perr.Error()})
						time.Sleep(2 * time.Second)
						continue
					}
					status := resp.Header.Get("Cf-Cache-Status")
					resp.Body.Close()
					attempts = append(attempts, map[string]string{
						"http_status":     fmt.Sprintf("%d", resp.StatusCode),
						"cf_cache_status": status,
						"at":              time.Now().UTC().Format(time.RFC3339),
					})
					switch strings.ToUpper(status) {
					case "MISS", "EXPIRED", "BYPASS":
						sawMiss = true
					case "HIT":
						if sawMiss {
							sawHit = true
						}
					}
					if sawMiss && sawHit {
						break
					}
					time.Sleep(3 * time.Second)
				}
				result["probe"] = map[string]any{
					"url":      probe,
					"attempts": attempts,
					"miss_to_hit_observed": sawMiss && sawHit,
				}
			}

			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&zone, "zone", "", "Zone name or ID (required)")
	cmd.Flags().StringSliceVar(&urls, "urls", nil, "URLs to purge (purges by file)")
	cmd.Flags().StringSliceVar(&tags, "tags", nil, "Cache tags to purge (Enterprise)")
	cmd.Flags().StringSliceVar(&hosts, "hosts", nil, "Hostnames to purge")
	cmd.Flags().StringVar(&probe, "probe", "", "URL to HEAD-probe; asserts cf-cache-status MISS→HIT after purge")
	cmd.Flags().DurationVar(&probeTimeout, "probe-timeout", 60*time.Second, "Probe verification timeout")
	return cmd
}
