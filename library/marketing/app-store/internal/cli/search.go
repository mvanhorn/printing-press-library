// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"bytes"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/appstore"
	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/store"

	"github.com/spf13/cobra"
)

// searchHit is one stored app listing or review matching the query.
type searchHit struct {
	Type          string     `json:"type"` // app | review
	AppID         string     `json:"appId"`
	Storefront    string     `json:"storefront"`
	Title         string     `json:"title"`
	Subtitle      string     `json:"subtitle,omitempty"`
	Version       string     `json:"version,omitempty"`
	Rating        *int       `json:"rating,omitempty"`        // review stars
	AverageRating *float64   `json:"averageRating,omitempty"` // app listing mean
	RatingCount   *int64     `json:"ratingCount,omitempty"`   // app listing count
	ReviewID      string     `json:"reviewId,omitempty"`
	Snippet       string     `json:"snippet,omitempty"`
	Body          string     `json:"body,omitempty"`
	ReviewedAt    string     `json:"reviewedAt,omitempty"`
	CapturedAt    *time.Time `json:"capturedAt,omitempty"` // app listing snapshot time
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newSearchCmd(flags))
	})
}

func newSearchCmd(flags *rootFlags) *cobra.Command {
	var kind, app, storefront string
	var limit int
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search stored reviews and app listings by full text (offline)",
		Long: strings.TrimSpace(`
Search the local store without touching the network: reviews by title and
body (the reviews_fts full-text index; every word must match, words of two
letters or fewer fall back to a substring scan) and app listings by the
title and subtitle of each app's latest stored snapshot. Each table is
searched on its own and the hits are returned apps first, then reviews by
relevance; --limit caps each type.

Only what 'reviews', 'app', 'prices' and 'compare' have stored is
searchable. To search the live App Store use 'search-store'.`),
		Example: strings.Trim(`
  app-store-pp-cli search crash --type reviews --app 6448311069 --limit 10 --agent
  app-store-pp-cli search "music streaming" --type apps --storefront us
  app-store-pp-cli search subscription --json --select type,appId,title,snippet`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "local",
			"pp:happy-args":  "query=crash",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "search")
			}
			query := strings.TrimSpace(strings.Join(args, " "))
			if query == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("<query> is required: the words to find in stored reviews and listings"))
			}
			if limit < 1 {
				return usageErr(fmt.Errorf("--limit must be at least 1, got %d", limit))
			}
			wantApps, wantReviews := true, true
			switch strings.ToLower(strings.TrimSpace(kind)) {
			case "", "all", "both":
			case "apps", "app", "listings":
				wantReviews = false
			case "reviews", "review":
				wantApps = false
			default:
				return usageErr(fmt.Errorf("--type: unknown type %q; use all, apps or reviews", kind))
			}
			appID := ""
			var appRef *appstore.AppRef
			if strings.TrimSpace(app) != "" {
				ref, err := parseAppRef(app)
				if err != nil {
					return usageErr(fmt.Errorf("--app: %w", err))
				}
				// A bundle id is resolved against the store once it is open.
				appRef, appID = &ref, ref.ID
			}
			sf := ""
			if strings.TrimSpace(storefront) != "" {
				cc, err := appstore.NormalizeCountry(storefront)
				if err != nil {
					return usageErr(fmt.Errorf("--storefront: %w", err))
				}
				sf = cc
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			hits := make([]searchHit, 0)
			if s3aMissingMirror(cmd, appStoreCLIName+" reviews <app> or "+appStoreCLIName+" app <app>   (to store reviews and listings first)") {
				return emitSearch(cmd, flags, hits, "nothing stored yet, so nothing to search")
			}
			st, err := openAppStoreStoreRead(cmd, flags)
			if err != nil {
				return err
			}
			defer s3aCloseStore(st)
			if appRef != nil {
				resolved, note, err := s3aResolveLocalApp(ctx, st, *appRef)
				if err != nil {
					return err
				}
				if resolved == "" {
					return emitSearch(cmd, flags, hits, note)
				}
				appID = resolved
			}

			if flags.maxAge > 0 {
				var ids []string
				if appID != "" {
					ids = []string{appID}
				}
				var sfs []string
				if sf != "" {
					sfs = []string{sf}
				}
				if newest, err := store.NewestAppObservation(ctx, st, ids, sfs); err == nil {
					warnStale(cmd, flags, newest, time.Now(), "capture", appStoreCLIName+" app <app> and "+appStoreCLIName+" reviews <app>")
				}
			}
			if wantApps {
				appLimit := limit
				if appID != "" {
					appLimit = 0 // filter by app below, then cap
				}
				apps, err := store.SearchApps(ctx, st, query, sf, appLimit)
				if err != nil {
					return apiErr(fmt.Errorf("searching stored listings: %w", err))
				}
				n := 0
				for _, a := range apps {
					if appID != "" && a.AppID != appID {
						continue
					}
					if n == limit {
						break
					}
					n++
					hits = append(hits, s3aAppHit(a))
				}
			}
			if wantReviews {
				filter := store.ReviewFilter{AppID: appID, Limit: limit}
				if sf != "" {
					filter.Storefronts = []string{sf}
				}
				reviews, err := store.SearchReviews(ctx, st, query, filter)
				if err != nil {
					return apiErr(fmt.Errorf("searching stored reviews: %w", err))
				}
				for _, r := range reviews {
					hits = append(hits, s3aReviewHit(r, query))
				}
			}
			note := ""
			if len(hits) == 0 {
				what := "stored app listings or reviews"
				switch {
				case !wantApps:
					what = "stored reviews"
				case !wantReviews:
					what = "stored app listings"
				}
				note = fmt.Sprintf("no %s match %q", what, query)
				if appID != "" {
					note += " for app " + appID
				}
				if sf != "" {
					note += " in " + sf
				}
			}
			return emitSearch(cmd, flags, hits, note)
		},
	}
	cmd.Flags().StringVar(&kind, "type", "all", "What to search: all, apps (listing titles and subtitles) or reviews (titles and bodies)")
	cmd.Flags().StringVar(&app, "app", "", "Only this app (numeric App Store id, id123, store URL or a stored bundle id)")
	cmd.Flags().StringVar(&storefront, "storefront", "", "Only this storefront (two-letter code); default: every stored storefront")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum hits per type")
	cmd.Flags().String("db", "", "Local snapshot database path (default: the CLI data directory)")
	return cmd
}

func s3aAppHit(a store.AppSnapshot) searchHit {
	avg, count := a.AverageRating, a.RatingCount
	at := a.CapturedAt.UTC()
	return searchHit{
		Type: "app", AppID: a.AppID, Storefront: a.Storefront, Title: a.Title, Subtitle: a.Subtitle,
		Version: a.Version, AverageRating: &avg, RatingCount: &count, CapturedAt: &at,
	}
}

func s3aReviewHit(r store.StoredReview, query string) searchHit {
	rating := r.Rating
	return searchHit{
		Type: "review", AppID: r.AppID, Storefront: r.Storefront, Title: r.Title, Version: r.Version,
		Rating: &rating, ReviewID: r.ReviewID, Snippet: s3aSnippet(r.Body, query, 160), Body: r.Body, ReviewedAt: r.ReviewedAt,
	}
}

// s3aSnippet returns up to width runes of text around the first query word
// found in it (case-insensitive), or the start of the text.
func s3aSnippet(text, query string, width int) string {
	text = strings.Join(strings.Fields(text), " ")
	runes := []rune(text)
	if len(runes) <= width {
		return text
	}
	lower := strings.Map(unicode.ToLower, text)
	start := 0
	for _, word := range strings.Fields(strings.Map(unicode.ToLower, query)) {
		if i := strings.Index(lower, word); i >= 0 {
			start = utf8.RuneCountInString(lower[:i]) - width/3
			break
		}
	}
	if start < 0 {
		start = 0
	}
	if start+width > len(runes) {
		start = len(runes) - width
	}
	out := string(runes[start : start+width])
	if start > 0 {
		out = "…" + out
	}
	if start+width < len(runes) {
		out += "…"
	}
	return out
}

func emitSearch(cmd *cobra.Command, flags *rootFlags, hits []searchHit, note string) error {
	return s3aEmitList(cmd, flags, hits, note, func(b *bytes.Buffer) {
		if len(hits) == 0 {
			fmt.Fprintln(b, note)
			return
		}
		rows := make([][]string, 0, len(hits))
		for _, h := range hits {
			stars, text := "", h.Snippet
			if h.Rating != nil {
				stars = fmt.Sprint(*h.Rating)
			} else if h.AverageRating != nil {
				stars = fmt.Sprintf("%.1f", *h.AverageRating)
				text = h.Subtitle
			}
			rows = append(rows, []string{h.Type, h.AppID, h.Storefront, stars, s3aCell(h.Title, 40), s3aCell(text, 70)})
		}
		s3aTable(b, []string{"TYPE", "APP ID", "SF", "STARS", "TITLE", "TEXT"}, rows)
	})
}
