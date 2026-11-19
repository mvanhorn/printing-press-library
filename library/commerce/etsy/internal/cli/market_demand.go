// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

const mktMinListingsWithReviews = 3

type mktDemandListing struct {
	ListingID       int64  `json:"listing_id"`
	Title           string `json:"title"`
	ShopID          any    `json:"shop_id"`
	Position        int    `json:"best_position"`
	State           string `json:"state"` // shown | absent
	ReviewsInWindow any    `json:"reviews_in_window"`
	ShopSoldCount   any    `json:"shop_transaction_sold_count"`
}

type mktDemandTerm struct {
	OrderBasis           string             `json:"order_basis"`
	Term                 string             `json:"term"`
	State                string             `json:"state"` // shown | cant_tell | absent | no_results | not_fetched
	ReviewsInWindow      any                `json:"reviews_in_window"`
	ListingsWithReviews  int                `json:"listings_with_reviews"`
	ListingsWithData     int                `json:"listings_with_review_data"`
	ListingsTotal        any                `json:"listings_total"`
	ShopSoldCountTotal   any                `json:"shop_transaction_sold_count_total"`
	Note                 string             `json:"note,omitempty"`
	Listings             []mktDemandListing `json:"listings"`
	rank                 int
	pooled, sold, listed int
}

// mktDemandTerms computes the demand view per term. absent means
// no review data was collected (never 0); cant_tell means the pooled recent
// reviews fall under the floor or too few listings carry any.
func mktDemandTerms(snapshotTerms []string, termPages map[string]int, rows []mktRow, shops map[int64]mktShop, dem map[int64]mktListingDemand, minReviews, top int, termAlt func(term string) int64, progress ...mktProg) []mktDemandTerm {
	byTerm := map[string][]mktRow{}
	order := make([]string, 0)
	for _, r := range rows {
		if _, ok := byTerm[r.Term]; !ok {
			order = append(order, r.Term)
		}
		byTerm[r.Term] = append(byTerm[r.Term], r)
	}
	for _, term := range snapshotTerms {
		if _, ok := byTerm[term]; ok {
			continue
		}
		_, recorded := termPages[term]
		fetched := false
		if len(progress) > 0 {
			for _, key := range progress[0].Pages {
				if i := strings.LastIndex(key, "|"); i >= 0 && key[:i] == term {
					fetched = true
					break
				}
			}
		}
		if recorded || !fetched {
			order = append(order, term)
			byTerm[term] = nil
		}
	}
	out := make([]mktDemandTerm, 0, len(order))
	for _, term := range order {
		t := mktDemandTerm{Term: term, OrderBasis: "search_position", Listings: make([]mktDemandListing, 0), ListingsTotal: len(byTerm[term])}
		seenShop := map[int64]bool{}
		soldKnown := false
		sold := 0
		for _, r := range byTerm[term] {
			l := mktDemandListing{ListingID: r.ListingID, Title: r.Title, ShopID: mktNullInt(r.ShopID), Position: r.Position, State: "absent"}
			if d, ok := dem[r.ListingID]; ok && d.Collected {
				l.State = "shown"
				l.ReviewsInWindow = d.Reviews
				t.ListingsWithData++
				t.OrderBasis = "reviews_in_window"
				t.pooled += d.Reviews
				if d.Reviews > 0 {
					t.ListingsWithReviews++
				}
			}
			if r.ShopID.Valid {
				if s, ok := shops[r.ShopID.Int64]; ok && s.Sold.Valid {
					l.ShopSoldCount = s.Sold.Int64
					if !seenShop[r.ShopID.Int64] {
						seenShop[r.ShopID.Int64] = true
						sold += int(s.Sold.Int64)
						soldKnown = true
					}
				}
			}
			t.Listings = append(t.Listings, l)
		}
		if soldKnown {
			t.ShopSoldCountTotal = sold
			t.sold = sold
		}
		_, hasOutcome := termPages[term]
		switch {
		case len(byTerm[term]) == 0 && !hasOutcome:
			t.State = "not_fetched"
			t.ListingsTotal = nil
			t.rank = 4
		case t.ListingsTotal == 0:
			t.State = "no_results"
			t.Note = "Etsy's search returned no results for this term"
			t.rank = 3
		case t.ListingsWithData == 0:
			t.State = "absent"
			t.Note = "no review data was collected for this term (absent is not zero); re-run market snapshot with --with-reviews"
			if termAlt != nil {
				altSnapshotID := termAlt(term)
				if altSnapshotID != 0 {
					t.Note = "no review data was collected for this term (absent is not zero); " + mktReviewSnapshotHint(altSnapshotID)
				}
			}
			t.rank = 2
		case t.pooled < minReviews || t.ListingsWithReviews < mktMinListingsWithReviews:
			t.State = "cant_tell"
			t.ReviewsInWindow = t.pooled
			t.Note = fmt.Sprintf("can't tell: %d pooled recent reviews across %d listing(s) with any; need at least %d reviews from at least %d listings",
				t.pooled, t.ListingsWithReviews, minReviews, mktMinListingsWithReviews)
			t.rank = 1
		default:
			t.State = "shown"
			t.ReviewsInWindow = t.pooled
			t.rank = 0
		}
		sort.SliceStable(t.Listings, func(i, j int) bool {
			a, b := t.Listings[i], t.Listings[j]
			ai, bi := a.State == "shown", b.State == "shown"
			if ai != bi {
				return ai
			}
			if ai {
				x, y := a.ReviewsInWindow.(int), b.ReviewsInWindow.(int)
				if x != y {
					return x > y
				}
			}
			return a.Position < b.Position
		})
		if top > 0 && len(t.Listings) > top {
			t.Listings = t.Listings[:top]
		}
		out = append(out, t)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].rank != out[j].rank {
			return out[i].rank < out[j].rank
		}
		if out[i].pooled != out[j].pooled {
			return out[i].pooled > out[j].pooled
		}
		return out[i].sold > out[j].sold
	})
	return out
}

func newNovelMarketDemandCmd(flags *rootFlags) *cobra.Command {
	var flagWindow string
	var flagMinReviews, flagTop int
	var flagSnapshot int64

	cmd := &cobra.Command{
		Use:   "demand",
		Short: "Rank listings and search terms by recent review volume and shop sales, with an explicit can't-tell result for thin data.",
		Long: `Rank listings and search terms by recent review volume and shop sales, with an explicit can't-tell result for thin data.

Each term is reported as shown, cant_tell, absent, no_results or not_fetched. not_fetched means the search has not run yet. no_results means Etsy returned no listings. absent means no review data was collected, which is
not the same as zero. cant_tell means fewer than --min-reviews pooled recent reviews, or fewer than 3
listings with any. Review data comes from 'market snapshot --with-reviews'.`,
		Example:     "  etsy-pp-cli market demand --window 12mo --agent\n  etsy-pp-cli market demand --min-reviews 10 --snapshot 2",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := mktValidateSnapshotFlag(cmd, flagSnapshot); err != nil {
				return err
			}
			if dryRunOK(flags) {
				return mktDryRun(cmd, flags, "market demand")
			}
			window, err := mktParseWindow(flagWindow)
			if err != nil {
				return usageErr(fmt.Errorf("--window: %w", err))
			}
			if flagMinReviews < 1 {
				return usageErr(fmt.Errorf("--min-reviews must be at least 1"))
			}
			db, err := mktOpenRead(cmd.Context())
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			if db != nil {
				defer db.Close()
			}
			snap, note, err := mktPickSnapshot(db, flagSnapshot)
			if err != nil {
				return err
			}
			if snap == nil {
				return mktEmitReport(cmd, flags, map[string]any{"results": make([]any, 0), "note": note})
			}
			rows, err := mktLoadResults(db.DB(), snap.ID)
			if err != nil {
				return err
			}
			shops, err := mktLoadShops(db.DB(), snap.ID)
			if err != nil {
				return err
			}
			dem, err := mktLoadDemand(db.DB(), snap, window)
			if err != nil {
				return err
			}
			termPages, err := mktLoadTermPages(db.DB(), snap.ID)
			if err != nil {
				return err
			}
			hasReviewData, err := mktHasReviewData(db.DB(), snap)
			if err != nil {
				return err
			}
			var altSnapshotID int64
			if !hasReviewData {
				alt, err := mktOlderReviewSnapshot(db.DB(), snap)
				if err != nil {
					return err
				}
				if alt != nil {
					altSnapshotID = alt.ID
				}
			}
			var alternatives map[string]int64
			var altErr error
			loaded := false
			termAlt := func(term string) int64 {
				if !loaded {
					loaded = true
					alternatives, altErr = mktTermReviewAlternatives(db.DB(), snap)
				}
				return alternatives[term]
			}
			terms := mktDemandTerms(snap.Terms, termPages, rows, shops, dem, flagMinReviews, flagTop, termAlt, snap.Progress)
			if altErr != nil {
				return altErr
			}
			result := map[string]any{
				"snapshot_id": snap.ID, "window": flagWindow, "min_reviews": flagMinReviews, "results": terms,
			}
			hasReviews := false
			for _, term := range terms {
				if term.ListingsWithData > 0 {
					hasReviews = true
					break
				}
			}
			if !hasReviews {
				result["note"] = "no review data collected; listings are ordered by search position, not demand — re-run market snapshot with --with-reviews"
			}
			if !hasReviewData && altSnapshotID != 0 {
				result["note"] = "no review data collected; listings are ordered by search position, not demand — " + mktReviewSnapshotHint(altSnapshotID)
			}
			notes := make([]string, 0)
			if note, ok := result["note"].(string); ok {
				notes = append(notes, note)
			}
			empty := 0
			for _, term := range terms {
				if term.State == "no_results" {
					empty++
				}
			}
			if empty > 0 {
				notes = append(notes, fmt.Sprintf("%d term(s) returned no results from Etsy's search API", empty))
			}
			notFetched := 0
			for _, term := range terms {
				if term.State == "not_fetched" {
					notFetched++
				}
			}
			// Missing terms were fetched before outcomes were recorded; count per term.
			shown := make(map[string]bool)
			for _, term := range terms {
				shown[term.Term] = true
			}
			legacy := 0
			for _, term := range snap.Terms {
				if !shown[term] {
					legacy++
					shown[term] = true
				}
			}
			if legacy > 0 {
				notes = append(notes, fmt.Sprintf("%d term(s) were fetched before per-term outcome recording; any with no results are not shown", legacy))
			}
			if notFetched > 0 {
				notes = append(notes, fmt.Sprintf("%d term(s) not yet fetched; resume with etsy-pp-cli market snapshot --resume %d", notFetched, snap.ID))
			}
			result["notes"] = notes
			if len(notes) > 0 {
				result["note"] = strings.Join(notes, "; ")
			}
			keys, texts := []string{}, []string{}
			for _, term := range terms {
				keys = append(keys, term.Term)
				texts = append(texts, term.Note)
			}
			result["row_notes"] = mktRowNotes(keys, texts)
			return mktEmitReport(cmd, flags, mktReviewCoverage(result, snap, window, len(dem) > 0))
		},
	}
	cmd.Flags().StringVar(&flagWindow, "window", "12mo", "Review window counted back from the snapshot date (e.g. 12mo, 90d)")
	cmd.Flags().IntVar(&flagMinReviews, "min-reviews", 10, "Minimum pooled recent reviews before a term is anything but cant_tell")
	cmd.Flags().IntVar(&flagTop, "top", 10, "Listings to show per term (0 = all)")
	cmd.Flags().Int64Var(&flagSnapshot, "snapshot", 0, mktSnapshotUsage)
	mktHideZeroDefault(cmd, "snapshot")
	return cmd
}
