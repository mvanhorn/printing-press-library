// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
	"github.com/spf13/cobra"
)

type shopxReviewDigestResult struct {
	Since     string                     `json:"since"`
	Total     int                        `json:"total"`
	Histogram map[string]int             `json:"histogram"`
	Average   *float64                   `json:"average"`
	LowRated  []shopxReviewDigestLow     `json:"low_rated"`
	ByListing []shopxReviewDigestListing `json:"by_listing"`
}

type shopxReviewDigestLow struct {
	ListingID int64   `json:"listing_id"`
	Rating    int64   `json:"rating"`
	Date      string  `json:"date"`
	Text      *string `json:"text,omitempty"`
}

type shopxReviewDigestListing struct {
	ListingID int64   `json:"listing_id"`
	Average   float64 `json:"average"`
	Count     int     `json:"count"`
}

func shopxReviewDigest(db *store.Store, now time.Time, window time.Duration, includeText bool) shopxReviewDigestResult {
	result := shopxReviewDigestResult{
		Histogram: map[string]int{"1": 0, "2": 0, "3": 0, "4": 0, "5": 0},
		LowRated:  make([]shopxReviewDigestLow, 0), ByListing: make([]shopxReviewDigestListing, 0),
	}
	seen := make(map[string]bool)
	counts := make(map[int64]int)
	sums := make(map[int64]int64)
	var sum int64
	for _, row := range shopxRows(db, shopxReviewTypes...) {
		ts, ok := shopxFirstTS(row, "created_timestamp", "create_timestamp")
		if !ok || ts.Before(now.Add(-window)) {
			continue
		}
		rating, ok := shopxInt(row["rating"])
		if !ok || rating < 1 || rating > 5 {
			continue
		}
		id, _ := shopxInt(row["listing_id"])
		key := fmt.Sprintf("%d|%d|%d", id, ts.Unix(), rating)
		if transaction, ok := shopxInt(row["transaction_id"]); ok && transaction != 0 {
			key = fmt.Sprintf("t:%d", transaction)
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		result.Total++
		result.Histogram[fmt.Sprint(rating)]++
		sum += rating
		counts[id]++
		sums[id] += rating
		if rating <= 3 {
			low := shopxReviewDigestLow{ListingID: id, Rating: rating, Date: ts.UTC().Format("2006-01-02")}
			if includeText {
				text, _ := row["review"].(string)
				low.Text = &text
			}
			result.LowRated = append(result.LowRated, low)
		}
	}
	if result.Total > 0 {
		avg := math.Round(float64(sum)/float64(result.Total)*100) / 100
		result.Average = &avg
	}
	for id, count := range counts {
		result.ByListing = append(result.ByListing, shopxReviewDigestListing{ListingID: id, Count: count, Average: math.Round(float64(sums[id])/float64(count)*100) / 100})
	}
	sort.Slice(result.LowRated, func(i, j int) bool {
		a, b := result.LowRated[i], result.LowRated[j]
		if a.Date != b.Date {
			return a.Date > b.Date
		}
		return a.ListingID < b.ListingID
	})
	sort.Slice(result.ByListing, func(i, j int) bool {
		a, b := result.ByListing[i], result.ByListing[j]
		if a.Count != b.Count {
			return a.Count > b.Count
		}
		return a.ListingID < b.ListingID
	})
	return result
}

func newNovelReviewsDigestCmd(flags *rootFlags) *cobra.Command {
	var flagSince, dbPath string
	var includeText bool
	cmd := &cobra.Command{
		Use:         "digest",
		Short:       "Rating histogram, low-rated reviews and per-listing averages for a period.",
		Long:        "Rating histogram, low-rated reviews and per-listing averages for a period.",
		Example:     "  etsy-pp-cli reviews digest --since 30d --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if includeText {
				if err := etsyRefuseBuyerDataOverMCP("reviews digest --text"); err != nil {
					return err
				}
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "reviews digest")
			}
			window, err := shopxParseWindow("since", flagSince)
			if err != nil {
				return err
			}
			db, err := shopxOpenDB(cmd, dbPath)
			if err != nil {
				return err
			}
			if db != nil {
				defer db.Close()
			}
			familyNotes := shopxCommandNotes(db, "reviews")
			result := shopxReviewDigest(db, shopxNow(), window, includeText)
			result.Since = flagSince
			return shopxEmit(cmd, flags, result, familyNotes)
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "30d", "Review period (for example 30d)")
	cmd.Flags().BoolVar(&includeText, "text", false, "Include low-rated review text")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	return cmd
}
