// Copyright 2026 Matt and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written: Phase 3 transcendence layer.

package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/google-search-console/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/google-search-console/internal/store"

	"github.com/spf13/cobra"
)

func newUrlInspectionInspectBatchCmd(flags *rootFlags) *cobra.Command {
	var (
		filePath     string
		siteURL      string
		maxPerDay    int
		languageCode string
	)

	cmd := &cobra.Command{
		Use:         "inspect-batch",
		Short:       "Inspect a list of URLs in one pass — streams NDJSON and snapshots into the local store",
		Long:        "Reads URLs (one per line) from --file and calls /v1/urlInspection/index:inspect for each, streaming NDJSON to stdout and snapshotting each result into the local store's url_inspections table for coverage-drift to read later. Honors --max-per-day quota guards. Honors PRINTING_PRESS_VERIFY=1 (prints \"would inspect: <url>\" without dialing).",
		Example:     "  google-search-console-pp-cli url-inspection inspect-batch --file urls.txt --site sc-domain:example.com --max-per-day 1500",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if filePath == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if siteURL == "" {
				return usageErr(fmt.Errorf("--site is required"))
			}
			f, err := os.Open(filePath)
			if err != nil {
				return configErr(err)
			}
			defer f.Close()

			urls := []string{}
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				u := strings.TrimSpace(scanner.Text())
				if u == "" || strings.HasPrefix(u, "#") {
					continue
				}
				urls = append(urls, u)
			}
			if err := scanner.Err(); err != nil {
				return configErr(err)
			}
			if maxPerDay > 0 && len(urls) > maxPerDay {
				urls = urls[:maxPerDay]
			}

			ctx := context.Background()
			s, err := openStore(ctx)
			if err != nil {
				return configErr(err)
			}
			defer s.Close()

			if cliutil.IsVerifyEnv() {
				for _, u := range urls {
					fmt.Fprintf(cmd.OutOrStdout(), "would inspect: %s\n", u)
				}
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := "/v1/urlInspection/index:inspect"
			ok := 0
			fail := 0
			for _, u := range urls {
				body := map[string]any{
					"inspectionUrl": u,
					"siteUrl":       siteURL,
				}
				if languageCode != "" {
					body["languageCode"] = languageCode
				}
				data, _, err := c.Post(path, body)
				if err != nil {
					fail++
					line := map[string]any{"inspection_url": u, "error": err.Error()}
					b, _ := json.Marshal(line)
					fmt.Fprintln(cmd.OutOrStdout(), string(b))
					continue
				}
				ok++
				// Stream raw NDJSON so an agent can parse line-by-line.
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
				// Snapshot into the store for coverage-drift.
				row := flattenInspection(siteURL, u, data)
				_ = s.UpsertUrlInspection(ctx, row)
			}
			if !flags.asJSON {
				fmt.Fprintf(os.Stderr, "inspected %d urls (%d ok, %d fail)\n", len(urls), ok, fail)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&filePath, "file", "", "Path to a text file with one URL per line")
	cmd.Flags().StringVar(&siteURL, "site", "", "GSC property the URLs belong to")
	cmd.Flags().IntVar(&maxPerDay, "max-per-day", 2000, "Cap requests per invocation (URL Inspection API quota is ~2000/day per property)")
	cmd.Flags().StringVar(&languageCode, "language-code", "en-US", "BCP-47 language code for the response")

	return cmd
}

func flattenInspection(site, inspectionURL string, raw json.RawMessage) store.URLInspectionRow {
	row := store.URLInspectionRow{
		SiteURL:       site,
		InspectionURL: inspectionURL,
		SnapshotAt:    time.Now().UTC(),
		RawJSON:       string(raw),
	}
	var parsed struct {
		InspectionResult struct {
			IndexStatusResult struct {
				Verdict         string `json:"verdict"`
				CoverageState   string `json:"coverageState"`
				RobotsTxtState  string `json:"robotsTxtState"`
				IndexingState   string `json:"indexingState"`
				PageFetchState  string `json:"pageFetchState"`
				LastCrawlTime   string `json:"lastCrawlTime"`
				GoogleCanonical string `json:"googleCanonical"`
				UserCanonical   string `json:"userCanonical"`
				CrawledAs       string `json:"crawledAs"`
			} `json:"indexStatusResult"`
		} `json:"inspectionResult"`
	}
	if json.Unmarshal(raw, &parsed) == nil {
		r := parsed.InspectionResult.IndexStatusResult
		row.Verdict = r.Verdict
		row.CoverageState = r.CoverageState
		row.RobotsTxtState = r.RobotsTxtState
		row.IndexingState = r.IndexingState
		row.PageFetchState = r.PageFetchState
		row.LastCrawlTime = r.LastCrawlTime
		row.GoogleCanonical = r.GoogleCanonical
		row.UserCanonical = r.UserCanonical
		row.CrawledAs = r.CrawledAs
	}
	return row
}
