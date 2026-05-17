// Copyright 2026 vinny-pasceri. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/other/numista/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/other/numista/internal/store"
)

// PATCH: hand-written issuer crawler with monthly quota forecast promised by README Highlights.
func newCrawlIssuerCmd(flags *rootFlags) *cobra.Command {
	var years string
	var category string
	var maxPages int

	cmd := &cobra.Command{
		Use:     "issuer [issuer_code]",
		Short:   "Crawl every type from one issuer (e.g., 'australia') matching a year range. Forecasts call cost as %-of-monthly-quota and requires confirmation in interactive mode.",
		Example: "  numista-pp-cli crawl issuer australia --years 1900-1950 --dry-run --json\n  numista-pp-cli crawl issuer france --category coin --max-pages 5 --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if maxPages <= 0 {
				return usageErr(fmt.Errorf("--max-pages must be positive"))
			}
			if category != "" && category != "coin" && category != "banknote" && category != "exonumia" {
				return usageErr(fmt.Errorf("--category must be one of coin, banknote, exonumia; got %q", category))
			}
			issuer := args[0]
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			s, err := store.OpenWithContext(cmd.Context(), defaultDBPath("numista-pp-cli"))
			if err != nil {
				return err
			}
			defer s.Close()
			if flags.dryRun {
				// PATCH: crawl dry-run forecasts from the live first page, not
				// the generated client's generic no-request dry-run preview.
				c.DryRun = false
				c.SetLogHook(makeLookupLogHook(s))
			}
			params := issuerParams(issuer, years, category, 1)
			first, firstLive, err := quotaTrackedGet(cmd.Context(), c, s, "/types", params)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			items, total := extractTypesAndTotal(first)
			pagesNeeded := int(math.Ceil(float64(total) / 50.0))
			if pagesNeeded == 0 && len(items) > 0 {
				pagesNeeded = 1
				total = len(items)
			}
			q, err := readQuota(cmd.Context(), s)
			if err != nil {
				return err
			}
			forecast := map[string]any{
				"issuer":               issuer,
				"total_types":          total,
				"pages_needed":         pagesNeeded,
				"live_calls_estimated": minInt(pagesNeeded, maxPages),
				"quota_now":            q,
				"quota_after":          forecastQuotaAfter(q, minInt(pagesNeeded, maxPages)),
				"fits":                 minInt(pagesNeeded, maxPages) <= q.Remaining,
			}
			if flags.dryRun {
				return printJSONFiltered(cmd.OutOrStdout(), forecast, flags)
			}
			if err := confirmCrawlIfInteractive(forecast, flags); err != nil {
				return err
			}
			fetched := len(items)
			liveCalls := boolToCount(firstLive)
			cacheHits := 1 - liveCalls
			stored, _ := persistTypeItems(s, items)
			for page := 2; page <= maxPages; page++ {
				if page > pagesNeeded && pagesNeeded > 0 {
					break
				}
				params := issuerParams(issuer, years, category, page)
				data, live, err := quotaTrackedGet(cmd.Context(), c, s, "/types", params)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				pageItems, _ := extractTypesAndTotal(data)
				fetched += len(pageItems)
				pageStored, _ := persistTypeItems(s, pageItems)
				stored += pageStored
				if live {
					liveCalls++
				} else {
					cacheHits++
				}
				if len(pageItems) < 50 {
					break
				}
			}
			after, err := readQuota(cmd.Context(), s)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"issuer":        issuer,
				"fetched_types": fetched,
				"stored_types":  stored,
				"live_calls":    liveCalls,
				"cache_hits":    cacheHits,
				"quota_after":   after,
			}, flags)
		},
	}
	cmd.Flags().StringVar(&years, "years", "", "Year or year range, passed as the Numista date query parameter")
	cmd.Flags().StringVar(&category, "category", "", "Catalogue category (coin, banknote, exonumia)")
	cmd.Flags().IntVar(&maxPages, "max-pages", 10, "Maximum pages to crawl")
	return cmd
}

func issuerParams(issuer, years, category string, page int) map[string]string {
	params := map[string]string{
		"issuer": issuer,
		"page":   strconv.Itoa(page),
		"count":  "50",
		"lang":   "en",
	}
	if years != "" {
		params["date"] = years
	}
	if category != "" {
		params["category"] = category
	}
	return params
}

// persistTypeItems writes each type object returned by /types into the local
// store so subsequent search/audit commands can find them offline. Skips
// items that fail to re-marshal (they were already-decoded map[string]any).
// Returns (stored, skipped). Errors from individual rows are folded into
// skipped rather than aborting — a partial crawl is still useful.
func persistTypeItems(s *store.Store, items []map[string]any) (int, int) {
	if len(items) == 0 {
		return 0, 0
	}
	raws := make([]json.RawMessage, 0, len(items))
	skipped := 0
	for _, item := range items {
		b, err := json.Marshal(item)
		if err != nil {
			skipped++
			continue
		}
		raws = append(raws, b)
	}
	if len(raws) == 0 {
		return 0, skipped
	}
	stored, batchSkipped, err := s.UpsertBatch("types", raws)
	if err != nil {
		return stored, skipped + batchSkipped + (len(raws) - stored - batchSkipped)
	}
	return stored, skipped + batchSkipped
}

func extractTypesAndTotal(raw json.RawMessage) ([]map[string]any, int) {
	items, _ := extractObjectList(raw)
	total := len(items)
	var obj map[string]any
	if json.Unmarshal(raw, &obj) == nil {
		for _, key := range []string{"count", "total", "total_count"} {
			if n, ok := anyToInt(obj[key]); ok {
				total = n
				break
			}
		}
	}
	return items, total
}

func anyToInt(v any) (int, bool) {
	switch x := v.(type) {
	case float64:
		return int(x), true
	case int:
		return x, true
	case string:
		n, err := strconv.Atoi(x)
		return n, err == nil
	default:
		return 0, false
	}
}

func forecastQuotaAfter(q cliutil.QuotaSnapshot, add int) cliutil.QuotaSnapshot {
	q.Used += add
	q.Remaining = q.Limit - q.Used
	return q
}

func confirmCrawlIfInteractive(forecast map[string]any, flags *rootFlags) error {
	if flags.yes || flags.noInput || flags.agent {
		return nil
	}
	if !isTerminal(os.Stdin) {
		return nil
	}
	fmt.Fprintf(os.Stderr, "crawl may consume up to %v live calls. Continue? [y/N] ", forecast["live_calls_estimated"])
	answer, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))
	if answer != "y" && answer != "yes" {
		return usageErr(fmt.Errorf("crawl cancelled"))
	}
	return nil
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
