// Copyright 2026 Elizabeth Stone and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/marketing/pinterest/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/pinterest/internal/store"
)

var trendTypes = []string{"growing", "monthly", "yearly", "seasonal"}

type trendGap struct {
	Keyword      string   `json:"keyword"`
	PctGrowthWoW any      `json:"pct_growth_wow"`
	PctGrowthMoM any      `json:"pct_growth_mom"`
	PctGrowthYoY any      `json:"pct_growth_yoy"`
	MatchingPins int      `json:"matching_pins"`
	SamplePinIDs []string `json:"sample_pin_ids"`
}

// parseTrendingKeywords decodes the TrendingKeywordsResponse {"trends":[...]}.
func parseTrendingKeywords(data json.RawMessage) ([]map[string]any, error) {
	var resp struct {
		Trends []map[string]any `json:"trends"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing trending keywords: %w", err)
	}
	if resp.Trends == nil {
		return make([]map[string]any, 0), nil
	}
	return resp.Trends, nil
}

// rankTrendGaps puts uncovered keywords first and otherwise keeps Pinterest's
// trend order (the input order is the #1..#N ranking).
func rankTrendGaps(gaps []trendGap) []trendGap {
	sort.SliceStable(gaps, func(i, j int) bool {
		return gaps[i].MatchingPins == 0 && gaps[j].MatchingPins != 0
	})
	return gaps
}

func newNovelTrendsGapsCmd(flags *rootFlags) *cobra.Command {
	var region, trendType, interests, dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:   "gaps",
		Short: "List trending keywords with how many of your synced Pins cover each",
		Long: "Fetches Pinterest's top trending keywords for a region and counts matching synced Pins\n" +
			"(every keyword word appears in the title, description, or alt text) in the local store.\n" +
			"Keywords with zero coverage come first. Run 'pinterest-pp-cli sync' first for coverage counts.",
		Example: strings.Trim(`
  pinterest-pp-cli trends gaps --region US --agent
  pinterest-pp-cli trends gaps --region US --trend-type seasonal --interests womens_fashion,beauty --limit 30 --json`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "live",
			"pp:happy-args":  "--region=US",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "trends gaps")
			}
			if strings.TrimSpace(region) == "" {
				return usageErr(fmt.Errorf("--region is required, e.g. --region US"))
			}
			if !containsString(trendTypes, trendType) {
				return usageErr(fmt.Errorf("--trend-type must be one of %s, got %q", strings.Join(trendTypes, ", "), trendType))
			}
			if limit < 1 || limit > 50 {
				return usageErr(fmt.Errorf("--limit must be between 1 and 50, got %d", limit))
			}
			if cliutil.IsDogfoodEnv() && limit > 10 {
				limit = 10
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			path := fmt.Sprintf("/trends/keywords/%s/top/%s", url.PathEscape(strings.ToUpper(region)), url.PathEscape(trendType))
			if interests != "" {
				path = appendArrayQueryParam(path, "interests", interests, "form", true)
			}
			data, err := c.Get(ctx, path, map[string]string{"limit": fmt.Sprint(limit)})
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			trends, err := parseTrendingKeywords(data)
			if err != nil {
				return err
			}

			var db *store.Store
			if dbPath == "" {
				dbPath = defaultDBPath("pinterest-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); statErr == nil {
				if db, err = store.OpenReadOnlyContext(ctx, dbPath); err != nil {
					return fmt.Errorf("opening database: %w", err)
				}
				defer db.Close()
				hintIfUnsynced(cmd, db, "pins")
			} else {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s; coverage counts are 0\nrun: pinterest-pp-cli sync --resources pins --db %s\n", dbPath, dbPath)
			}

			// Owned Pins and board Pins are stored separately; match against both,
			// deduplicated by Pin id, so existing content is not reported as a gap.
			pinTexts := make([]pinText, 0)
			if db != nil {
				pins, err := loadSyncedPins(ctx, db)
				if err != nil {
					return err
				}
				for _, pin := range pins {
					pinTexts = append(pinTexts, newPinText(fieldString(pin, "id"),
						fieldString(pin, "title")+" "+fieldString(pin, "description")+" "+fieldString(pin, "alt_text")))
				}
			}

			gaps := make([]trendGap, 0, len(trends))
			for _, t := range trends {
				g := trendGap{
					Keyword:      fieldString(t, "keyword"),
					PctGrowthWoW: t["pct_growth_wow"],
					PctGrowthMoM: t["pct_growth_mom"],
					PctGrowthYoY: t["pct_growth_yoy"],
					SamplePinIDs: make([]string, 0, 3),
				}
				if g.Keyword != "" {
					g.MatchingPins, g.SamplePinIDs = keywordCoverage(g.Keyword, pinTexts)
				}
				gaps = append(gaps, g)
			}
			gaps = rankTrendGaps(gaps)

			w := cmd.OutOrStdout()
			if !wantsHumanTable(w, flags) {
				return printJSONFiltered(w, gaps, flags)
			}
			if len(gaps) == 0 {
				fmt.Fprintln(w, "Pinterest returned no trending keywords for that region and type.")
				return nil
			}
			fmt.Fprintf(w, "%-40s %8s %s\n", "KEYWORD", "PINS", "WOW%")
			for _, g := range gaps {
				fmt.Fprintf(w, "%-40s %8d %v\n", truncateRunes(g.Keyword, 40), g.MatchingPins, g.PctGrowthWoW)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&region, "region", "US", "Two-letter Pinterest trends region, e.g. US, GB, CA")
	cmd.Flags().StringVar(&trendType, "trend-type", "growing", "Trend ranking: growing, monthly, yearly, or seasonal")
	cmd.Flags().StringVar(&interests, "interests", "", "Comma-separated Pinterest interest filters, e.g. womens_fashion,beauty")
	cmd.Flags().IntVar(&limit, "limit", 20, "Number of trending keywords to fetch (1-50)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to the local SQLite store (defaults to the CLI data dir)")
	return cmd
}

type pinText struct {
	ID    string
	Words map[string]bool
}

func newPinText(id, text string) pinText {
	words := map[string]bool{}
	for _, w := range keywordTokenRE.FindAllString(strings.ToLower(text), -1) {
		words[w] = true
	}
	return pinText{ID: id, Words: words}
}

var keywordTokenRE = regexp.MustCompile(`[\pL\pN]+`)

// keywordCoverage counts Pins whose text contains every word of keyword as a
// whole word (so "art" does not match "party"), and returns up to three
// sample Pin ids.
func keywordCoverage(keyword string, pins []pinText) (int, []string) {
	samples := make([]string, 0, 3)
	tokens := keywordTokenRE.FindAllString(strings.ToLower(keyword), -1)
	if len(tokens) == 0 {
		return 0, samples
	}
	count := 0
	for _, p := range pins {
		matched := true
		for _, tok := range tokens {
			if !p.Words[tok] {
				matched = false
				break
			}
		}
		if matched {
			count++
			if len(samples) < 3 {
				samples = append(samples, p.ID)
			}
		}
	}
	return count, samples
}
