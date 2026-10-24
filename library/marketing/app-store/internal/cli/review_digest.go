// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/store"

	"github.com/spf13/cobra"
)

// reviewDigestVersion is the star breakdown of one app version.
type reviewDigestVersion struct {
	Version       string         `json:"version"`
	Reviews       int            `json:"reviews"`
	AverageRating float64        `json:"averageRating"`
	Stars         map[string]int `json:"stars"`
}

// reviewDigestStorefront is the review count and mean stars of one storefront.
type reviewDigestStorefront struct {
	Storefront    string  `json:"storefront"`
	Reviews       int     `json:"reviews"`
	AverageRating float64 `json:"averageRating"`
}

// reviewDigestTerm is a word frequent in 1-2 star reviews: Reviews counts
// the low-star reviews that mention it, Share is that over all of them.
type reviewDigestTerm struct {
	Term    string  `json:"term"`
	Reviews int     `json:"reviews"`
	Share   float64 `json:"share"`
}

type reviewDigestView struct {
	AppID                  string                   `json:"appId"`
	Storefronts            []string                 `json:"storefronts"`
	SinceVersion           string                   `json:"sinceVersion,omitempty"`
	Total                  int                      `json:"total"`
	AverageRating          *float64                 `json:"averageRating"`
	Stars                  map[string]int           `json:"stars"`
	ByVersion              []reviewDigestVersion    `json:"byVersion"`
	ByStorefront           []reviewDigestStorefront `json:"byStorefront"`
	LowStarReviews         int                      `json:"lowStarReviews"`
	ComplaintTerms         []reviewDigestTerm       `json:"complaintTerms"`
	OldestReviewAt         string                   `json:"oldestReviewAt,omitempty"`
	NewestReviewAt         string                   `json:"newestReviewAt,omitempty"`
	ExcludedUnknownVersion int                      `json:"excludedUnknownVersion,omitempty"`
	Note                   string                   `json:"note,omitempty"`
}

// s3aUnknownVersion labels reviews stored without an app version.
const s3aUnknownVersion = "unknown"

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newReviewDigestCmd(flags))
	})
}

func newReviewDigestCmd(flags *rootFlags) *cobra.Command {
	var sinceVersion, countries string
	var terms int
	cmd := &cobra.Command{
		Use:   "review-digest <app>",
		Short: "Summarize stored reviews: star and per-version histograms plus complaint terms",
		Long: strings.TrimSpace(`
Mechanical digest of one app's stored reviews (no LLM): the star histogram,
the mean, a star histogram per app version (newest version first), review
counts per storefront, and the most frequent terms in 1-2 star reviews.
Complaint terms are lower-cased words of three or more letters from the
title and body, counted once per review, minus English stopwords and store
boilerplate (app, game, please ...); a term needs two mentioning reviews.

<app> is the numeric App Store id, id123, a store URL or a bundle id (resolved
from the stored listing snapshots, no network). --countries picks
storefronts (default: a store URL's storefront, else --country; "all" =
every stored storefront). --since-version keeps reviews written for that
version or a newer one (versions compare numerically, 9.1.10 > 9.1.9);
reviews stored without a version are then left out and counted.

The input is the reviews 'reviews' captured: a sample of written reviews
(the RSS feeds reach at most 500 per sort and storefront), not every
rating. For exact star counts use 'ratings'; for individual reviews use
'reviews' or 'search --type reviews'.`),
		Example: strings.Trim(`
  app-store-pp-cli review-digest 6448311069 --countries us,gb --agent
  app-store-pp-cli review-digest 324684580 --since-version 9.1 --terms 10
  app-store-pp-cli review-digest 553834731 --countries all --json --select stars,complaintTerms`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "local",
			"pp:happy-args":  "app=6448311069",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "review-digest")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("<app> is required: a numeric App Store id such as 6448311069 (or id123, a store URL or a stored bundle id)"))
			}
			if len(args) > 1 {
				return usageErr(fmt.Errorf("review-digest takes one app, got %d arguments", len(args)))
			}
			if terms < 0 {
				return usageErr(fmt.Errorf("--terms must be 0 (none) or more, got %d", terms))
			}
			ref, appID, cc, err := s3aLocalApp(cmd, flags, args[0])
			if err != nil {
				return err
			}
			// Without --countries: the store URL's storefront, else --country.
			filter := []string{cc}
			if strings.TrimSpace(countries) != "" {
				if filter, err = s3aStorefrontFilter(flags, countries); err != nil {
					return err
				}
			}
			sinceVersion = strings.TrimSpace(sinceVersion)
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			hintCountries := strings.Join(filter, ",")
			if filter == nil {
				hintCountries = "us,gb"
			}
			hint := fmt.Sprintf("%s reviews %s --countries %s", appStoreCLIName, appID, hintCountries)
			if s3aMissingMirror(cmd, hint) {
				view := s3aDigestReviews(appID, filter, sinceVersion, nil, terms)
				view.Note = "no local reviews yet; run: " + hint
				return emitReviewDigest(cmd, flags, view)
			}
			st, err := openAppStoreStoreRead(cmd, flags)
			if err != nil {
				return err
			}
			defer s3aCloseStore(st)
			resolved, note, err := s3aResolveLocalApp(ctx, st, ref)
			if err != nil {
				return err
			}
			if resolved == "" {
				view := s3aDigestReviews(appID, filter, sinceVersion, nil, terms)
				view.Note = note
				return emitReviewDigest(cmd, flags, view)
			}
			appID = resolved
			reviews, err := store.Reviews(ctx, st, store.ReviewFilter{AppID: appID, Storefronts: filter})
			if err != nil {
				return apiErr(fmt.Errorf("reading stored reviews: %w", err))
			}
			var lastSeen time.Time
			for _, r := range reviews {
				if r.LastSeenAt.After(lastSeen) {
					lastSeen = r.LastSeenAt
				}
			}
			warnStale(cmd, flags, lastSeen, time.Now(), "review capture of app "+appID, fmt.Sprintf("%s reviews %s --countries %s", appStoreCLIName, appID, hintCountries))
			view := s3aDigestReviews(appID, filter, sinceVersion, reviews, terms)
			switch {
			case len(reviews) == 0:
				where := "any storefront"
				if filter != nil {
					where = strings.Join(filter, ",")
				}
				view.Note = fmt.Sprintf("no stored reviews of app %s in %s; run: %s", appID, where, hint)
			case view.Total == 0:
				view.Note = fmt.Sprintf("none of the %d stored reviews is for version %s or newer", len(reviews), sinceVersion)
			case view.LowStarReviews == 0:
				view.Note = "no 1-2 star reviews in the selection, so there are no complaint terms"
			}
			if view.ExcludedUnknownVersion > 0 {
				extra := fmt.Sprintf("%d stored review(s) without a version were left out by --since-version", view.ExcludedUnknownVersion)
				view.Note = strings.TrimPrefix(view.Note+"; "+extra, "; ")
			}
			return emitReviewDigest(cmd, flags, view)
		},
	}
	cmd.Flags().StringVar(&sinceVersion, "since-version", "", "Only reviews written for this app version or a newer one (e.g. 9.1)")
	cmd.Flags().StringVar(&countries, "countries", "", "Storefronts to include: us,gb,... or all (default: --country)")
	cmd.Flags().IntVar(&terms, "terms", 20, "Maximum complaint terms to list (0 = none)")
	cmd.Flags().String("db", "", "Local snapshot database path (default: the CLI data directory)")
	return cmd
}

// s3aStarMap returns a histogram with all five star keys present.
func s3aStarMap() map[string]int {
	return map[string]int{"1": 0, "2": 0, "3": 0, "4": 0, "5": 0}
}

// s3aDigestReviews aggregates stored reviews. filter nil means every
// storefront; the view then lists the storefronts present in the data.
func s3aDigestReviews(appID string, filter []string, sinceVersion string, reviews []store.StoredReview, maxTerms int) reviewDigestView {
	v := reviewDigestView{
		AppID: appID, SinceVersion: sinceVersion, Stars: s3aStarMap(),
		Storefronts:    append(make([]string, 0, len(filter)), filter...),
		ByVersion:      make([]reviewDigestVersion, 0),
		ByStorefront:   make([]reviewDigestStorefront, 0),
		ComplaintTerms: make([]reviewDigestTerm, 0),
	}
	type agg struct {
		n, sum int
		stars  map[string]int
	}
	byVersion := map[string]*agg{}
	byStorefront := map[string]*agg{}
	termCounts := map[string]int{}
	sum := 0
	for _, r := range reviews {
		if r.Rating < 1 || r.Rating > 5 {
			continue
		}
		version := strings.TrimSpace(r.Version)
		if sinceVersion != "" {
			if version == "" {
				v.ExcludedUnknownVersion++
				continue
			}
			if s3aCompareVersions(version, sinceVersion) < 0 {
				continue
			}
		}
		if version == "" {
			version = s3aUnknownVersion
		}
		star := strconv.Itoa(r.Rating)
		v.Total++
		sum += r.Rating
		v.Stars[star]++
		a := byVersion[version]
		if a == nil {
			a = &agg{stars: s3aStarMap()}
			byVersion[version] = a
		}
		a.n, a.sum = a.n+1, a.sum+r.Rating
		a.stars[star]++
		s := byStorefront[r.Storefront]
		if s == nil {
			s = &agg{}
			byStorefront[r.Storefront] = s
		}
		s.n, s.sum = s.n+1, s.sum+r.Rating
		if v.OldestReviewAt == "" || (r.ReviewedAt != "" && r.ReviewedAt < v.OldestReviewAt) {
			v.OldestReviewAt = r.ReviewedAt
		}
		if r.ReviewedAt > v.NewestReviewAt {
			v.NewestReviewAt = r.ReviewedAt
		}
		if r.Rating <= 2 {
			v.LowStarReviews++
			for _, t := range s3aTerms(r.Title + " " + r.Body) {
				termCounts[t]++
			}
		}
	}
	if v.Total > 0 {
		avg := s3aRound2(float64(sum) / float64(v.Total))
		v.AverageRating = &avg
	}
	for version, a := range byVersion {
		v.ByVersion = append(v.ByVersion, reviewDigestVersion{Version: version, Reviews: a.n, AverageRating: s3aRound2(float64(a.sum) / float64(a.n)), Stars: a.stars})
	}
	sort.Slice(v.ByVersion, func(i, j int) bool {
		a, b := v.ByVersion[i].Version, v.ByVersion[j].Version
		if (a == s3aUnknownVersion) != (b == s3aUnknownVersion) {
			return b == s3aUnknownVersion
		}
		if c := s3aCompareVersions(a, b); c != 0 {
			return c > 0
		}
		return a < b
	})
	for sf, a := range byStorefront {
		v.ByStorefront = append(v.ByStorefront, reviewDigestStorefront{Storefront: sf, Reviews: a.n, AverageRating: s3aRound2(float64(a.sum) / float64(a.n))})
	}
	sort.Slice(v.ByStorefront, func(i, j int) bool {
		if v.ByStorefront[i].Reviews != v.ByStorefront[j].Reviews {
			return v.ByStorefront[i].Reviews > v.ByStorefront[j].Reviews
		}
		return v.ByStorefront[i].Storefront < v.ByStorefront[j].Storefront
	})
	if filter == nil {
		for _, s := range v.ByStorefront {
			v.Storefronts = append(v.Storefronts, s.Storefront)
		}
		sort.Strings(v.Storefronts)
	}
	for t, n := range termCounts {
		if n >= 2 {
			v.ComplaintTerms = append(v.ComplaintTerms, reviewDigestTerm{Term: t, Reviews: n, Share: s3aRound2(float64(n) / float64(v.LowStarReviews))})
		}
	}
	sort.Slice(v.ComplaintTerms, func(i, j int) bool {
		if v.ComplaintTerms[i].Reviews != v.ComplaintTerms[j].Reviews {
			return v.ComplaintTerms[i].Reviews > v.ComplaintTerms[j].Reviews
		}
		return v.ComplaintTerms[i].Term < v.ComplaintTerms[j].Term
	})
	if len(v.ComplaintTerms) > maxTerms {
		v.ComplaintTerms = v.ComplaintTerms[:maxTerms]
	}
	return v
}

func emitReviewDigest(cmd *cobra.Command, flags *rootFlags, v reviewDigestView) error {
	return s3aEmit(cmd, flags, v, func(b *bytes.Buffer) {
		where := strings.Join(v.Storefronts, ",")
		if where == "" {
			where = "all storefronts"
		}
		fmt.Fprintf(b, "app %s, %s: %d stored review(s)", v.AppID, where, v.Total)
		if v.AverageRating != nil {
			fmt.Fprintf(b, ", mean %.2f stars", *v.AverageRating)
		}
		if v.SinceVersion != "" {
			fmt.Fprintf(b, " (version %s or newer)", v.SinceVersion)
		}
		b.WriteByte('\n')
		if v.Total > 0 {
			for star := 5; star >= 1; star-- {
				n := v.Stars[strconv.Itoa(star)]
				bar := strings.Repeat("#", (n*40+v.Total-1)/v.Total)
				fmt.Fprintf(b, "  %d star  %6d  %s\n", star, n, bar)
			}
			rows := make([][]string, 0, len(v.ByVersion))
			for i, ver := range v.ByVersion {
				if i == 15 {
					rows = append(rows, []string{fmt.Sprintf("... %d more version(s)", len(v.ByVersion)-15)})
					break
				}
				rows = append(rows, []string{s3aCell(ver.Version, 20), strconv.Itoa(ver.Reviews), fmt.Sprintf("%.2f", ver.AverageRating),
					strconv.Itoa(ver.Stars["1"]), strconv.Itoa(ver.Stars["2"]), strconv.Itoa(ver.Stars["3"]), strconv.Itoa(ver.Stars["4"]), strconv.Itoa(ver.Stars["5"])})
			}
			s3aTable(b, []string{"VERSION", "REVIEWS", "MEAN", "1", "2", "3", "4", "5"}, rows)
		}
		if len(v.ComplaintTerms) > 0 {
			fmt.Fprintf(b, "complaint terms in %d low-star review(s):\n", v.LowStarReviews)
			for _, t := range v.ComplaintTerms {
				fmt.Fprintf(b, "  %-20s %4d  %3.0f%%\n", s3aCell(t.Term, 20), t.Reviews, t.Share*100)
			}
		}
		if v.Note != "" {
			fmt.Fprintln(b, v.Note)
		}
	})
}
