package cli

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	tpkg "github.com/mvanhorn/printing-press-library/library/marketing/trustpilot/internal/trustpilot"
)

func newTPTopRecentCmd(flags *rootFlags) *cobra.Command {
	var (
		window string
		good   int
		bad    int
		lang   string
	)
	cmd := &cobra.Command{
		Use:   "top-recent <domain>",
		Short: "Balanced sample of recent good and bad reviews for an entity",
		Long:  "Pulls N freshest 4-5 star reviews and N freshest 1-2 star reviews for a company in one call. Designed for agent integrations like last30days that need a balanced citable sample of recent customer sentiment.",
		Example: `  trustpilot-pp-cli top-recent www.thriftbooks.com --window 30d --good 5 --bad 5 --json
  trustpilot-pp-cli top-recent www.thriftbooks.com --window 7d --good 3 --bad 3 --lang en`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			domain := normalizeDomain(args[0])
			ctx := cmd.Context()
			db, err := openTPStore(ctx)
			if err != nil {
				return err
			}
			defer db.Close()

			minDate := windowMinDate(window)
			dateWindowParam := dateWindowFromCLI(window)
			hasLocalData := false
			if flags.dataSource != "live" {
				localProbe, err := tpkg.QueryReviews(ctx, db, tpkg.QueryFilters{Domain: domain, MinDate: minDate, Limit: 1})
				if err != nil {
					return err
				}
				hasLocalData = len(localProbe) > 0
			}
			source, fallback := resolveDataSource(flags, hasLocalData)
			// PATCH: Source top-recent from local data first and mark live fallback.
			meta := NewMeta(source)
			if !hasLocalData && flags.dataSource != "live" {
				meta.AddNotice(NoticeLocalStoreEmpty)
			}
			if fallback {
				meta.AddNotice(NoticeLiveFallback)
			}
			var goodReviews, badReviews []tpkg.Review
			var bu tpkg.BusinessUnit

			if source == "local" {
				goodReviews, err = fetchLocalBucket(ctx, db, domain, []int{5, 4}, lang, minDate, good)
				if err != nil {
					return err
				}
				badReviews, err = fetchLocalBucket(ctx, db, domain, []int{1, 2}, lang, minDate, bad)
				if err != nil {
					return err
				}
				meta.LocalCount = intPtr(len(goodReviews) + len(badReviews))
				if loaded, lerr := tpkg.LoadCompany(ctx, db, domain); lerr == nil {
					bu = loaded
					addBusinessUnitMeta(&meta, bu)
				}
			} else {
				sess, _, err := loadOrHarvestSession(ctx, db, true)
				if err != nil {
					return err
				}
				page, err := fetchPageWithRetry(ctx, db, &sess, domain, tpkg.PageFilters{Page: 1, Sort: "recency"})
				if err != nil {
					return err
				}
				bu = page.BusinessUnit
				addBusinessUnitMeta(&meta, bu)

				goodReviews, _ = fetchBucket(ctx, db, &sess, domain, []int{5, 4}, lang, dateWindowParam, minDate, good*2)
				badReviews, _ = fetchBucket(ctx, db, &sess, domain, []int{1, 2}, lang, dateWindowParam, minDate, bad*2)

				goodReviews = goodReviews[:minInt(len(goodReviews), good)]
				badReviews = badReviews[:minInt(len(badReviews), bad)]
			}
			newest := newestReviewAt(goodReviews, badReviews)
			addReviewFreshnessMeta(&meta, newest, minDate)

			payload := map[string]any{
				"domain":              domain,
				"window":              window,
				"good":                goodReviews,
				"bad":                 badReviews,
				"counts":              map[string]int{"good": len(goodReviews), "bad": len(badReviews)},
				"minDate":             minDate.UTC().Format(time.RFC3339),
				"isCollectingReviews": meta.IsCollectingReviews,
				"newestReviewAt":      newest,
			}
			attachMeta(payload, meta)
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, payload)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s - last %s\n", domain, window)
			fmt.Fprintln(cmd.OutOrStdout(), "GOOD:")
			for _, r := range goodReviews {
				fmt.Fprintf(cmd.OutOrStdout(), "  %d* %s - %s\n    %s\n", r.Rating, r.PublishedDate.Format("2006-01-02"), r.Title, truncateText(r.Text, 160))
			}
			fmt.Fprintln(cmd.OutOrStdout(), "BAD:")
			for _, r := range badReviews {
				fmt.Fprintf(cmd.OutOrStdout(), "  %d* %s - %s\n    %s\n", r.Rating, r.PublishedDate.Format("2006-01-02"), r.Title, truncateText(r.Text, 160))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&window, "window", "30d", "Time window: 7d, 30d, 90d, 180d, 365d")
	cmd.Flags().IntVar(&good, "good", 5, "Number of fresh 4-5 star reviews to include")
	cmd.Flags().IntVar(&bad, "bad", 5, "Number of fresh 1-2 star reviews to include")
	cmd.Flags().StringVar(&lang, "lang", "", "Filter to a single language (ISO 639-1, e.g. en)")
	return cmd
}

// fetchBucket walks star bins until it has minCount reviews newer than minDate.
func fetchBucket(ctx context.Context, db *sql.DB, sess *tpkg.Session, domain string, stars []int, lang, dateWin string, minDate time.Time, minCount int) ([]tpkg.Review, error) {
	var out []tpkg.Review
	for _, s := range stars {
		page := 1
		for page <= 5 && len(out) < minCount {
			pp, err := fetchPageWithRetry(ctx, db, sess, domain, tpkg.PageFilters{
				Page: page, Stars: s, Language: lang, Sort: "recency", DateWindow: dateWin,
			})
			if err != nil {
				return out, err
			}
			stopped := false
			for _, r := range pp.Reviews {
				if !minDate.IsZero() && !r.PublishedDate.IsZero() && r.PublishedDate.Before(minDate) {
					stopped = true
					break
				}
				out = append(out, r)
				if len(out) >= minCount {
					return out, nil
				}
			}
			if stopped {
				break
			}
			if pp.Pagination.TotalPages > 0 && page >= pp.Pagination.TotalPages {
				break
			}
			page++
		}
	}
	return out, nil
}

// PATCH: Reuse the balanced bucket shape for local-store reads.
func fetchLocalBucket(ctx context.Context, db *sql.DB, domain string, stars []int, lang string, minDate time.Time, limit int) ([]tpkg.Review, error) {
	return tpkg.QueryReviews(ctx, db, tpkg.QueryFilters{
		Domain:   domain,
		Stars:    stars,
		Language: lang,
		MinDate:  minDate,
		Limit:    limit,
	})
}

// windowMinDate parses a "Nd"/"Nw"/"Nm" or "Ny" window into an absolute time.
func windowMinDate(window string) time.Time {
	w := strings.TrimSpace(strings.ToLower(window))
	if w == "" {
		return time.Time{}
	}
	if len(w) < 2 {
		return time.Time{}
	}
	unit := w[len(w)-1]
	nStr := w[:len(w)-1]
	n, err := strconv.Atoi(nStr)
	if err != nil || n <= 0 {
		return time.Time{}
	}
	now := time.Now().UTC()
	switch unit {
	case 'd':
		return now.AddDate(0, 0, -n)
	case 'w':
		return now.AddDate(0, 0, -7*n)
	case 'm':
		return now.AddDate(0, -n, 0)
	case 'y':
		return now.AddDate(-n, 0, 0)
	}
	return time.Time{}
}

// dateWindowFromCLI maps user-friendly windows into Trustpilot's allowed query
// values. Anything outside Trustpilot's enum returns "" (no filter at the URL
// level — we still date-clamp locally via windowMinDate).
func dateWindowFromCLI(window string) string {
	switch strings.TrimSpace(strings.ToLower(window)) {
	case "30d", "1m":
		return "last30days"
	case "90d", "3m":
		return "last3months"
	case "180d", "6m":
		return "last6months"
	case "365d", "1y", "12m":
		return "last12months"
	}
	return ""
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func truncateText(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return strings.TrimSpace(s[:n]) + "..."
}
