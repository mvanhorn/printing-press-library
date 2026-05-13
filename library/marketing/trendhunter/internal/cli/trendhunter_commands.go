package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/marketing/trendhunter/internal/client"
	"github.com/mvanhorn/printing-press-library/library/marketing/trendhunter/internal/store"
	"github.com/mvanhorn/printing-press-library/library/marketing/trendhunter/internal/thparse"
	"github.com/mvanhorn/printing-press-library/library/marketing/trendhunter/internal/thstore"
)

// registerTrendhunterCommands mirrors the Digg hand-edit model used by
// Printing Press generated CLIs: all novel TrendHunter commands live in this
// file, and root.go contains only a single call to this function immediately
// before returning the generated root command. The root.go call is intentionally
// small so the generator's --force AST merge can preserve it across regens.
func registerTrendhunterCommands(root *cobra.Command, flags *rootFlags) {
	root.AddCommand(newTHLatestCmd(flags))
	root.AddCommand(newTHBrowseCmd(flags))
	root.AddCommand(newTHTrendCmd(flags))
	root.AddCommand(newTHBoardCmd(flags))
	root.AddCommand(newTHHotCmd(flags))
	root.AddCommand(newTHDigestCmd(flags))
	root.AddCommand(newTHWatchCmd(flags))
	root.AddCommand(newTHFAQShortcutCmd(flags))
	root.AddCommand(newTHClusterCmd(flags))
	root.AddCommand(newTHAuthorsCmd(flags))
	root.AddCommand(newTHMegatrendMapCmd(flags))
	root.AddCommand(newTHBriefCmd(flags))
	root.AddCommand(newTHInboxCmd(flags))
	root.AddCommand(newTHScoutCmd(flags))
	root.AddCommand(newTHPullCmd(flags))
}

func thReadOnly() map[string]string {
	return map[string]string{"mcp:read-only": "true"}
}

// thBrowserHeaders is the Chrome-imitating header set we attach to every
// outbound TrendHunter request. The site sits behind Akamai bot protection
// which fingerprints request shape; the minimum that passes is Chrome UA +
// browser-style Accept + Accept-Language. curl-default headers get 403.
var thBrowserHeaders = map[string]string{
	"User-Agent":                "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
	"Accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
	"Accept-Language":           "en-US,en;q=0.9",
	"Sec-Fetch-Dest":            "document",
	"Sec-Fetch-Mode":            "navigate",
	"Sec-Fetch-Site":            "none",
	"Sec-Fetch-User":            "?1",
	"Upgrade-Insecure-Requests": "1",
}

func openTHStore(ctx context.Context) (*store.Store, *sql.DB, func() error, error) {
	dbPath := defaultDBPath("trendhunter-pp-cli")
	s, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		if wd, wdErr := os.Getwd(); wdErr == nil {
			dbPath = filepath.Join(wd, ".trendhunter-pp-cli.db")
			s, err = store.OpenWithContext(ctx, dbPath)
		}
	}
	if err != nil {
		return nil, nil, func() error { return nil }, fmt.Errorf("opening local database: %w", err)
	}
	if err := thstore.EnsureSchema(s.DB()); err != nil {
		ensureErr := err
		_ = s.Close()
		if wd, wdErr := os.Getwd(); wdErr == nil {
			dbPath = filepath.Join(wd, ".trendhunter-pp-cli.db")
			s, err = store.OpenWithContext(ctx, dbPath)
			if err == nil {
				if ensureErr := thstore.EnsureSchema(s.DB()); ensureErr != nil {
					_ = s.Close()
					return nil, nil, func() error { return nil }, ensureErr
				}
				return s, s.DB(), s.Close, nil
			}
		}
		return nil, nil, func() error { return nil }, ensureErr
	}
	return s, s.DB(), s.Close, nil
}

func thClient(flags *rootFlags) (*client.Client, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	return c, nil
}

func fetchTH(c *client.Client, path string, params map[string]string) ([]byte, error) {
	data, err := c.GetWithHeaders(path, params, thBrowserHeaders)
	if err != nil {
		return nil, err
	}
	return []byte(data), nil
}

func outputTH(cmd *cobra.Command, flags *rootFlags, v any) error {
	return printJSONFiltered(cmd.OutOrStdout(), v, flags)
}

func parseSince(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	unit := s[len(s)-1]
	if unit == 'd' || unit == 'w' {
		n, err := strconv.Atoi(s[:len(s)-1])
		if err != nil {
			return 0, err
		}
		if unit == 'w' {
			n *= 7
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

func newTHLatestCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "latest",
		Short:       "Fetch and parse the global TrendHunter RSS feed",
		Example:     `  trendhunter-pp-cli latest --json`,
		Annotations: thReadOnly(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := thClient(flags)
			if err != nil {
				return err
			}
			body, err := fetchTH(c, "/rss", nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			trends, err := thparse.ParseRSS(body)
			if err != nil {
				return err
			}
			return outputTH(cmd, flags, trends)
		},
	}
	return cmd
}

func newTHBrowseCmd(flags *rootFlags) *cobra.Command {
	var syncStore bool
	cmd := &cobra.Command{
		Use:         "browse [category]",
		Short:       "Fetch and parse a TrendHunter category page",
		Example:     `  trendhunter-pp-cli browse tech --json`,
		Annotations: thReadOnly(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			trends, err := fetchCategory(cmd.Context(), flags, args[0])
			if err != nil {
				return err
			}
			if syncStore {
				_, db, closeFn, err := openTHStore(cmd.Context())
				if err != nil {
					return err
				}
				defer closeFn()
				for _, t := range trends {
					if t.Category == "" {
						t.Category = args[0]
					}
					if err := thstore.UpsertTrend(cmd.Context(), db, t); err != nil {
						return err
					}
				}
			}
			return outputTH(cmd, flags, trends)
		},
	}
	cmd.Flags().BoolVar(&syncStore, "sync", false, "Write parsed trends into the local store")
	return cmd
}

func newTHTrendCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trend",
		Short: "Trend detail, FAQ, and related-trend commands",
		Example: `  trendhunter-pp-cli trend show ai-clone --json
  trendhunter-pp-cli trend faq ai-clone --json`,
		Annotations: thReadOnly(),
	}
	cmd.AddCommand(newTHTrendShowCmd(flags))
	cmd.AddCommand(newTHTrendFAQCmd(flags))
	cmd.AddCommand(newTHTrendRelatedCmd(flags))
	return cmd
}

func newTHTrendShowCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "show [slug]",
		Short:       "Fetch and parse one TrendHunter trend page",
		Example:     `  trendhunter-pp-cli trend show ai-clone --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			t, err := fetchTrendDetail(cmd.Context(), flags, args[0])
			if err != nil {
				return err
			}
			// Persist into the parsed-trends store so subsequent corpus
			// commands (authors, cluster, digest) can index richer fields
			// like author, keywords, faq, category, body_text that the
			// RSS-only `pull` cannot supply.
			if t != nil {
				if _, db, closeFn, openErr := openTHStore(cmd.Context()); openErr == nil {
					_ = thstore.UpsertTrend(cmd.Context(), db, *t)
					_ = closeFn()
				}
			}
			return outputTH(cmd, flags, t)
		},
	}
	return cmd
}

func newTHTrendFAQCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "faq [slug]",
		Short:       "Extract FAQPage Q&A from a trend page",
		Example:     `  trendhunter-pp-cli trend faq ai-clone --json`,
		Annotations: thReadOnly(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			t, err := fetchTrendDetail(cmd.Context(), flags, args[0])
			if err != nil {
				return err
			}
			return outputTH(cmd, flags, t.FAQ)
		},
	}
	return cmd
}

func newTHTrendRelatedCmd(flags *rootFlags) *cobra.Command {
	var titles bool
	cmd := &cobra.Command{
		Use:         "related [slug]",
		Short:       "List related trend slugs for one trend",
		Example:     `  trendhunter-pp-cli trend related ai-clone --json`,
		Annotations: thReadOnly(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			t, err := fetchTrendDetail(cmd.Context(), flags, args[0])
			if err != nil {
				return err
			}
			if !titles {
				return outputTH(cmd, flags, t.RelatedSlugs)
			}
			rows := make([]map[string]string, 0, len(t.RelatedSlugs))
			for _, slug := range t.RelatedSlugs {
				row := map[string]string{"slug": slug}
				if rt, err := fetchTrendDetail(cmd.Context(), flags, slug); err == nil {
					row["title"] = rt.Title
				}
				rows = append(rows, row)
			}
			return outputTH(cmd, flags, rows)
		},
	}
	cmd.Flags().BoolVar(&titles, "titles", false, "Fetch titles for each related trend")
	return cmd
}

func newTHFAQShortcutCmd(flags *rootFlags) *cobra.Command {
	cmd := newTHTrendFAQCmd(flags)
	cmd.Use = "faq [slug]"
	cmd.Short = "Shortcut for trend faq"
	return cmd
}

func newTHBoardCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "board",
		Short:       "Parse the TrendHunter scoreboard page",
		Example:     `  trendhunter-pp-cli board --json`,
		Annotations: thReadOnly(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			return fetchCardCommand(cmd, flags, "/scoreboard", "scoreboard")
		},
	}
	return cmd
}

func newTHHotCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "hot",
		Short:       "Parse the TrendHunter popular page",
		Example:     `  trendhunter-pp-cli hot --json`,
		Annotations: thReadOnly(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			return fetchCardCommand(cmd, flags, "/popular", "popular")
		},
	}
	return cmd
}

func newTHDigestCmd(flags *rootFlags) *cobra.Command {
	var sinceRaw, category string
	cmd := &cobra.Command{
		Use:         "digest",
		Short:       "Summarize new stored trends and top keywords",
		Example:     `  trendhunter-pp-cli digest --since 7d --category eco --json`,
		Annotations: thReadOnly(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			since, err := parseSince(sinceRaw)
			if err != nil {
				return err
			}
			_, db, closeFn, err := openTHStore(cmd.Context())
			if err != nil {
				return err
			}
			defer closeFn()
			newTrends, err := thstore.ListTrendsByCategory(cmd.Context(), db, category, since, 500)
			if err != nil {
				return err
			}
			repeatCount, err := repeatCount(cmd.Context(), db, category, since)
			if err != nil {
				return err
			}
			topKeywords, err := keywordCounts(cmd.Context(), db, category, time.Now().Add(-since), time.Time{}, 20)
			if err != nil {
				return err
			}
			return outputTH(cmd, flags, map[string]any{
				"new_count":    len(newTrends),
				"repeat_count": repeatCount,
				"new_trends":   newTrends,
				"top_keywords": topKeywords,
			})
		},
	}
	cmd.Flags().StringVar(&sinceRaw, "since", "7d", "Lookback window")
	cmd.Flags().StringVar(&category, "category", "", "Filter by category")
	return cmd
}

func newTHWatchCmd(flags *rootFlags) *cobra.Command {
	var category, sinceRaw string
	cmd := &cobra.Command{
		Use:         "watch",
		Short:       "Fetch a category and return trends not already in the local store",
		Example:     `  trendhunter-pp-cli watch --category gadgets --since 24h --json`,
		Annotations: thReadOnly(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if category == "" {
				return fmt.Errorf("--category is required")
			}
			if _, err := parseSince(sinceRaw); err != nil {
				return err
			}
			trends, err := fetchCategory(cmd.Context(), flags, category)
			if err != nil {
				return err
			}
			_, db, closeFn, err := openTHStore(cmd.Context())
			if err != nil {
				return err
			}
			defer closeFn()
			var fresh []thparse.Trend
			for _, t := range trends {
				t.Category = category
				_, exists, err := thstore.GetTrend(cmd.Context(), db, t.Slug)
				if err != nil {
					return err
				}
				if !exists {
					fresh = append(fresh, t)
				}
				if err := thstore.UpsertTrend(cmd.Context(), db, t); err != nil {
					return err
				}
			}
			return outputTH(cmd, flags, map[string]any{
				"category":   category,
				"new_count":  len(fresh),
				"new_trends": fresh,
			})
		},
	}
	cmd.Flags().StringVar(&category, "category", "", "Category slug")
	cmd.Flags().StringVar(&sinceRaw, "since", "24h", "Lookback window")
	return cmd
}

func newTHClusterCmd(flags *rootFlags) *cobra.Command {
	var windowRaw string
	var minCount int
	cmd := &cobra.Command{
		Use:         "cluster",
		Short:       "Show rising keyword clusters from the local corpus",
		Example:     `  trendhunter-pp-cli cluster --window 30d --min-count 3 --json`,
		Annotations: thReadOnly(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			window, err := parseSince(windowRaw)
			if err != nil {
				return err
			}
			_, db, closeFn, err := openTHStore(cmd.Context())
			if err != nil {
				return err
			}
			defer closeFn()
			now := time.Now()
			current, err := keywordCounts(cmd.Context(), db, "", now.Add(-window), now, 1000)
			if err != nil {
				return err
			}
			prior, err := keywordCounts(cmd.Context(), db, "", now.Add(-2*window), now.Add(-window), 1000)
			if err != nil {
				return err
			}
			priorMap := map[string]int{}
			for _, row := range prior {
				priorMap[row.Keyword] = row.Count
			}
			type clusterRow struct {
				Keyword    string `json:"keyword"`
				Count      int    `json:"count"`
				PriorCount int    `json:"prior_count"`
				Delta      int    `json:"delta"`
			}
			var out []clusterRow
			for _, row := range current {
				if row.Count < minCount {
					continue
				}
				p := priorMap[row.Keyword]
				out = append(out, clusterRow{Keyword: row.Keyword, Count: row.Count, PriorCount: p, Delta: row.Count - p})
			}
			sort.Slice(out, func(i, j int) bool {
				if out[i].Delta != out[j].Delta {
					return out[i].Delta > out[j].Delta
				}
				return out[i].Count > out[j].Count
			})
			return outputTH(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&windowRaw, "window", "30d", "Comparison window")
	cmd.Flags().IntVar(&minCount, "min-count", 3, "Minimum current-window keyword count")
	return cmd
}

func newTHAuthorsCmd(flags *rootFlags) *cobra.Command {
	var top int
	var sinceRaw string
	cmd := &cobra.Command{
		Use:         "authors",
		Short:       "Rank authors by local trend velocity",
		Example:     `  trendhunter-pp-cli authors --top 20 --since 30d --json`,
		Annotations: thReadOnly(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			since, err := parseSince(sinceRaw)
			if err != nil {
				return err
			}
			_, db, closeFn, err := openTHStore(cmd.Context())
			if err != nil {
				return err
			}
			defer closeFn()
			rows, err := thstore.ListAuthorVelocity(cmd.Context(), db, since, top)
			if err != nil {
				return err
			}
			return outputTH(cmd, flags, rows)
		},
	}
	cmd.Flags().IntVar(&top, "top", 20, "Maximum authors")
	cmd.Flags().StringVar(&sinceRaw, "since", "30d", "Lookback window")
	return cmd
}

func newTHMegatrendMapCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "megatrend-map [slug]",
		Short:       "Walk related trends two levels deep",
		Example:     `  trendhunter-pp-cli megatrend-map ai-clone --json`,
		Annotations: thReadOnly(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			t, err := loadOrFetchTrend(cmd.Context(), flags, args[0])
			if err != nil {
				return err
			}
			depth1 := t.RelatedSlugs
			depth2Set := map[string]struct{}{}
			for _, slug := range depth1 {
				rt, err := loadOrFetchTrend(cmd.Context(), flags, slug)
				if err != nil {
					continue
				}
				for _, child := range rt.RelatedSlugs {
					if child != t.Slug {
						depth2Set[child] = struct{}{}
					}
				}
			}
			depth2 := make([]string, 0, len(depth2Set))
			for slug := range depth2Set {
				depth2 = append(depth2, slug)
			}
			sort.Strings(depth2)
			return outputTH(cmd, flags, map[string]any{
				"trend":                           t,
				"related_at_depth_1":              depth1,
				"related_at_depth_2":              depth2,
				"keyword_overlap_with_megatrends": []any{},
			})
		},
	}
	return cmd
}

func newTHBriefCmd(flags *rootFlags) *cobra.Command {
	var category, format string
	var top int
	cmd := &cobra.Command{
		Use:         "brief",
		Short:       "Build an agent-ready category brief",
		Example:     `  trendhunter-pp-cli brief --category ai --top 10 --format markdown`,
		Annotations: thReadOnly(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if category == "" {
				return fmt.Errorf("--category is required")
			}
			if format != "json" && format != "markdown" {
				return fmt.Errorf("--format must be json or markdown")
			}
			trends, err := fetchCategory(cmd.Context(), flags, category)
			if err != nil {
				trends, err = storedCategory(cmd.Context(), category, 30*24*time.Hour, top)
			}
			if err != nil {
				return err
			}
			trends = limitTrends(trends, top)
			for i := range trends {
				if detail, err := fetchTrendDetail(cmd.Context(), flags, trends[i].Slug); err == nil {
					trends[i] = *detail
				}
			}
			if format == "markdown" && !flags.asJSON {
				return renderBriefMarkdown(cmd.OutOrStdout(), category, trends)
			}
			return outputTH(cmd, flags, map[string]any{"category": category, "trends": trends})
		},
	}
	cmd.Flags().StringVar(&category, "category", "", "Category slug")
	cmd.Flags().IntVar(&top, "top", 10, "Maximum trends")
	cmd.Flags().StringVar(&format, "format", "json", "Output format: json or markdown")
	return cmd
}

func newTHInboxCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "inbox",
		Short:       "Show locally stored trends new since the last inbox read",
		Example:     `  trendhunter-pp-cli inbox --json`,
		Annotations: thReadOnly(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			_, db, closeFn, err := openTHStore(cmd.Context())
			if err != nil {
				return err
			}
			defer closeFn()
			cursor, ok, err := thstore.LookupCursor(cmd.Context(), db)
			if err != nil {
				return err
			}
			trends, err := trendsSince(cmd.Context(), db, cursor, ok)
			if err != nil {
				return err
			}
			now := time.Now().UTC()
			if err := thstore.UpdateCursor(cmd.Context(), db, now); err != nil {
				return err
			}
			return outputTH(cmd, flags, map[string]any{
				"cursor":     cursor.Format(time.RFC3339),
				"new_count":  len(trends),
				"new_trends": trends,
			})
		},
	}
	return cmd
}

func newTHScoutCmd(flags *rootFlags) *cobra.Command {
	var category, business, format string
	var top int
	var llm bool
	cmd := &cobra.Command{
		Use:         "scout",
		Short:       "Score category trends against a business profile",
		Example:     `  trendhunter-pp-cli scout --category kitchen --business "Smart ovens for home cooks" --top 10 --json`,
		Annotations: thReadOnly(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if category == "" {
				return fmt.Errorf("--category is required")
			}
			if business == "" {
				return fmt.Errorf("--business is required")
			}
			if format != "json" && format != "pipe" {
				return fmt.Errorf("--format must be json or pipe")
			}
			trends, err := fetchCategory(cmd.Context(), flags, category)
			if err != nil {
				trends, err = storedCategory(cmd.Context(), category, 30*24*time.Hour, top)
			}
			if err != nil {
				return err
			}
			rows := scoreTrends(cmd.Context(), trends, business, top, llm)
			if format == "pipe" && !flags.asJSON {
				for _, row := range rows {
					fmt.Fprintf(cmd.OutOrStdout(), "%s\t%.2f\t%s\n", row.Slug, row.Score, strings.Join(row.Keywords, ","))
				}
				return nil
			}
			return outputTH(cmd, flags, rows)
		},
	}
	cmd.Flags().StringVar(&category, "category", "", "Category slug")
	cmd.Flags().StringVar(&business, "business", "", "Business profile or product description")
	cmd.Flags().IntVar(&top, "top", 10, "Maximum trends")
	cmd.Flags().BoolVar(&llm, "llm", false, "Try local codex or claude scoring")
	cmd.Flags().StringVar(&format, "format", "json", "Output format: json or pipe")
	return cmd
}

func newTHPullCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "pull",
		Short:       "Fetch RSS and sitemap into the local parsed store",
		Example:     `  trendhunter-pp-cli pull`,
		Annotations: thReadOnly(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := thClient(flags)
			if err != nil {
				return err
			}
			_, db, closeFn, err := openTHStore(cmd.Context())
			if err != nil {
				return err
			}
			defer closeFn()
			body, err := fetchTH(c, "/rss", nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			trends, err := thparse.ParseRSS(body)
			if err != nil {
				return err
			}
			for _, t := range trends {
				if err := thstore.UpsertTrend(cmd.Context(), db, t); err != nil {
					return err
				}
			}
			sitemapBody, err := fetchTH(c, "/sitemap.xml", nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			entries, err := thparse.ParseSitemap(sitemapBody)
			if err != nil {
				return err
			}
			for _, e := range entries {
				if err := thstore.UpsertSitemap(cmd.Context(), db, e); err != nil {
					return err
				}
			}
			return outputTH(cmd, flags, map[string]any{
				"trends_upserted":  len(trends),
				"sitemap_upserted": len(entries),
			})
		},
	}
	return cmd
}

func fetchCardCommand(cmd *cobra.Command, flags *rootFlags, path, source string) error {
	c, err := thClient(flags)
	if err != nil {
		return err
	}
	body, err := fetchTH(c, path, nil)
	if err != nil {
		return classifyAPIError(err, flags)
	}
	trends, err := thparse.ParseCardList(body, source)
	if err != nil {
		return err
	}
	return outputTH(cmd, flags, trends)
}

func fetchCategory(ctx context.Context, flags *rootFlags, category string) ([]thparse.Trend, error) {
	c, err := thClient(flags)
	if err != nil {
		return nil, err
	}
	body, err := fetchTH(c, "/"+strings.Trim(category, "/"), nil)
	if err != nil {
		return nil, classifyAPIError(err, flags)
	}
	trends, err := thparse.ParseCardList(body, "category")
	if err != nil {
		return nil, err
	}
	for i := range trends {
		trends[i].Category = category
	}
	_ = ctx
	return trends, nil
}

func fetchTrendDetail(ctx context.Context, flags *rootFlags, slug string) (*thparse.Trend, error) {
	c, err := thClient(flags)
	if err != nil {
		return nil, err
	}
	path := "/trends/" + strings.Trim(slug, "/")
	body, err := fetchTH(c, path, nil)
	if err != nil {
		return nil, classifyAPIError(err, flags)
	}
	_ = ctx
	return thparse.ParseTrendPage(body, "https://www.trendhunter.com"+path)
}

func loadOrFetchTrend(ctx context.Context, flags *rootFlags, slug string) (*thparse.Trend, error) {
	_, db, closeFn, err := openTHStore(ctx)
	if err == nil {
		defer closeFn()
		if t, ok, err := thstore.GetTrend(ctx, db, slug); err == nil && ok && len(t.RelatedSlugs) > 0 {
			return t, nil
		}
	}
	return fetchTrendDetail(ctx, flags, slug)
}

func storedCategory(ctx context.Context, category string, since time.Duration, limit int) ([]thparse.Trend, error) {
	_, db, closeFn, err := openTHStore(ctx)
	if err != nil {
		return nil, err
	}
	defer closeFn()
	return thstore.ListTrendsByCategory(ctx, db, category, since, limit)
}

func limitTrends(trends []thparse.Trend, top int) []thparse.Trend {
	if top > 0 && len(trends) > top {
		return trends[:top]
	}
	return trends
}

func renderBriefMarkdown(w io.Writer, category string, trends []thparse.Trend) error {
	fmt.Fprintf(w, "# %s Trend Brief\n\n", strings.Title(category))
	for _, t := range trends {
		fmt.Fprintf(w, "## %s\n\n", t.Title)
		if t.Description != "" {
			fmt.Fprintf(w, "%s\n\n", t.Description)
		}
		if len(t.Keywords) > 0 {
			fmt.Fprintf(w, "Keywords: %s\n\n", strings.Join(t.Keywords, ", "))
		}
		for _, qa := range t.FAQ {
			fmt.Fprintf(w, "- Q: %s\n  A: %s\n", qa.Question, qa.Answer)
		}
		if len(t.FAQ) > 0 {
			fmt.Fprintln(w)
		}
	}
	return nil
}

func repeatCount(ctx context.Context, db *sql.DB, category string, since time.Duration) (int, error) {
	cutoff := time.Now().Add(-since).UTC().Format(time.RFC3339)
	where := `last_seen >= ? AND first_seen < ?`
	args := []any{cutoff, cutoff}
	if category != "" {
		where += ` AND category = ?`
		args = append(args, category)
	}
	var n int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM parsed_trends WHERE `+where, args...).Scan(&n)
	return n, err
}

func keywordCounts(ctx context.Context, db *sql.DB, category string, start, end time.Time, limit int) ([]thstore.KeywordRow, error) {
	where := `first_seen >= ?`
	args := []any{start.UTC().Format(time.RFC3339)}
	if !end.IsZero() {
		where += ` AND first_seen < ?`
		args = append(args, end.UTC().Format(time.RFC3339))
	}
	if category != "" {
		where += ` AND category = ?`
		args = append(args, category)
	}
	rows, err := db.QueryContext(ctx, `SELECT keywords FROM parsed_trends WHERE `+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	counts := map[string]int{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		for _, kw := range strings.Split(raw, ",") {
			kw = strings.ToLower(strings.TrimSpace(kw))
			if kw != "" {
				counts[kw]++
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]thstore.KeywordRow, 0, len(counts))
	for kw, n := range counts {
		out = append(out, thstore.KeywordRow{Keyword: kw, Count: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Keyword < out[j].Keyword
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func trendsSince(ctx context.Context, db *sql.DB, cursor time.Time, hasCursor bool) ([]thparse.Trend, error) {
	query := `SELECT slug, title, description, image_url, keywords, author, category, trend_id, pub_date, body_text, related_slugs, faq, source_url, source FROM parsed_trends`
	var args []any
	if hasCursor {
		query += ` WHERE first_seen > ?`
		args = append(args, cursor.UTC().Format(time.RFC3339))
	}
	query += ` ORDER BY first_seen DESC`
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []thparse.Trend
	for rows.Next() {
		var t thparse.Trend
		var keywords, related, faq string
		if err := rows.Scan(&t.Slug, &t.Title, &t.Description, &t.ImageURL, &keywords, &t.Author, &t.Category, &t.TrendID, &t.PubDate, &t.BodyText, &related, &faq, &t.SourceURL, &t.Source); err != nil {
			return nil, err
		}
		t.Keywords = splitCSV(keywords)
		_ = json.Unmarshal([]byte(related), &t.RelatedSlugs)
		_ = json.Unmarshal([]byte(faq), &t.FAQ)
		out = append(out, t)
	}
	return out, rows.Err()
}

type scoutRow struct {
	Slug     string   `json:"slug"`
	Title    string   `json:"title"`
	Score    float64  `json:"score"`
	Reason   string   `json:"reason"`
	Keywords []string `json:"keywords,omitempty"`
}

func scoreTrends(ctx context.Context, trends []thparse.Trend, business string, top int, useLLM bool) []scoutRow {
	terms := businessTerms(business)
	rows := make([]scoutRow, 0, len(trends))
	for _, t := range trends {
		score, reason := deterministicScore(t, terms)
		if useLLM {
			if llmScore, ok := llmTrendScore(ctx, t); ok {
				score = llmScore
				reason = "llm"
			}
		}
		rows = append(rows, scoutRow{Slug: t.Slug, Title: t.Title, Score: score, Reason: reason, Keywords: t.Keywords})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Score != rows[j].Score {
			return rows[i].Score > rows[j].Score
		}
		return rows[i].Title < rows[j].Title
	})
	if top > 0 && len(rows) > top {
		rows = rows[:top]
	}
	return rows
}

var stopwords = map[string]struct{}{
	"a": {}, "an": {}, "and": {}, "are": {}, "as": {}, "for": {}, "i": {}, "in": {}, "is": {}, "of": {}, "on": {}, "or": {}, "our": {}, "the": {}, "to": {}, "we": {}, "with": {},
}

var wordRE = regexp.MustCompile(`[a-z0-9]+`)

func businessTerms(s string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, term := range wordRE.FindAllString(strings.ToLower(s), -1) {
		if _, skip := stopwords[term]; skip {
			continue
		}
		if _, ok := seen[term]; ok {
			continue
		}
		seen[term] = struct{}{}
		out = append(out, term)
	}
	return out
}

func deterministicScore(t thparse.Trend, terms []string) (float64, string) {
	title := strings.ToLower(t.Title)
	desc := strings.ToLower(t.Description)
	keywords := strings.ToLower(strings.Join(t.Keywords, " "))
	var score float64
	var hits []string
	for _, term := range terms {
		exact := title == term || desc == term || keywordExact(t.Keywords, term)
		if exact || strings.Contains(" "+title+" ", " "+term+" ") || strings.Contains(" "+desc+" ", " "+term+" ") || strings.Contains(" "+keywords+" ", " "+term+" ") {
			score += 1
			hits = append(hits, term)
			continue
		}
		if strings.Contains(title, term) || strings.Contains(desc, term) || strings.Contains(keywords, term) {
			score += 0.5
			hits = append(hits, term+"~")
		}
	}
	if len(hits) == 0 {
		return 0, "no business term overlap"
	}
	return score, "matched " + strings.Join(hits, ",")
}

func keywordExact(keywords []string, term string) bool {
	for _, kw := range keywords {
		if strings.EqualFold(strings.TrimSpace(kw), term) {
			return true
		}
	}
	return false
}

var numberRE = regexp.MustCompile(`\d+(?:\.\d+)?`)

func llmTrendScore(ctx context.Context, t thparse.Trend) (float64, bool) {
	bin := ""
	if p, err := exec.LookPath("codex"); err == nil {
		bin = p
	} else if p, err := exec.LookPath("claude"); err == nil {
		bin = p
	}
	if bin == "" {
		return 0, false
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	prompt := fmt.Sprintf("Score relevance 0-10. Trend: %s. %s. Keywords: %s.", t.Title, t.Description, strings.Join(t.Keywords, ", "))
	var cmd *exec.Cmd
	if strings.Contains(bin, "claude") {
		cmd = exec.CommandContext(ctx, bin, "--print", prompt)
	} else {
		cmd = exec.CommandContext(ctx, bin, "exec", prompt)
	}
	out, err := cmd.Output()
	if err != nil {
		return 0, false
	}
	m := numberRE.FindString(string(out))
	if m == "" {
		return 0, false
	}
	score, err := strconv.ParseFloat(m, 64)
	if err != nil {
		return 0, false
	}
	return score, true
}

func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
