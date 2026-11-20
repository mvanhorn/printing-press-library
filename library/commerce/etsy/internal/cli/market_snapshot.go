// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"

	"github.com/spf13/cobra"
)

const (
	mktBatchLimit      = 100
	mktReviewPageLimit = 100
	mktMaxReviewPages  = 10
	mktPageLimit       = 100
	mktMaxPages        = 5
	mktDefaultMaxCall  = 200
)

// mktSnapshotPlan is the pre-flight plan, also printed by --dry-run.
type mktSnapshotPlan struct {
	Terms          []string `json:"terms"`
	Pages          int      `json:"pages"`
	PerPage        int      `json:"per_page"`
	SortOn         string   `json:"sort_on"`
	WithReviews    bool     `json:"with_reviews"`
	PlannedCalls   int      `json:"planned_calls"`
	PageCalls      int      `json:"page_calls"`
	ShopCallsEst   int      `json:"shop_calls_estimate"`
	ReviewCallsEst int      `json:"review_calls_estimate"`
	MaxCalls       int      `json:"max_calls"`
	Curtailed      bool     `json:"curtailed,omitempty"`
}

func mktPlanSnapshot(terms []string, pages, perPage int, withReviews bool, reviewTop, maxCalls, pagesDone int) mktSnapshotPlan {
	pageCalls := len(terms)*pages - pagesDone
	if pageCalls < 0 {
		pageCalls = 0
	}
	shops := (pageCalls*perPage + 99) / 100
	reviews := 0
	if withReviews {
		reviews = pageCalls * min(reviewTop, perPage)
	}
	return mktSnapshotPlan{Terms: terms, Pages: pages, PerPage: perPage, WithReviews: withReviews, PageCalls: pageCalls,
		ShopCallsEst: shops, ReviewCallsEst: reviews, PlannedCalls: pageCalls + shops + reviews, MaxCalls: maxCalls}
}

type mktListingPayload struct {
	Count   int64 `json:"count"`
	Results []struct {
		ListingID int64  `json:"listing_id"`
		ShopID    *int64 `json:"shop_id"`
		Title     string `json:"title"`
		Price     *struct {
			Amount       *int64 `json:"amount"`
			Divisor      *int64 `json:"divisor"`
			CurrencyCode string `json:"currency_code"`
		} `json:"price"`
		NumFavorers *int64   `json:"num_favorers"`
		Views       *int64   `json:"views"`
		Tags        []string `json:"tags"`
		TaxonomyID  *int64   `json:"taxonomy_id"`
		CreatedTS   *int64   `json:"original_creation_timestamp"`
		ListingType *string  `json:"listing_type"`
	} `json:"results"`
}

type mktShopPayload struct {
	ShopName             *string  `json:"shop_name"`
	ShopID               *int64   `json:"shop_id"`
	CreateDate           *int64   `json:"create_date"`
	TransactionSoldCount *int64   `json:"transaction_sold_count"`
	ListingActiveCount   *int64   `json:"listing_active_count"`
	ReviewAverage        *float64 `json:"review_average"`
	ReviewCount          *int64   `json:"review_count"`
	IsVacation           *bool    `json:"is_vacation"`
}

type mktReviewPayload struct {
	Count   int `json:"count"`
	Results []struct {
		CreatedTimestamp int64  `json:"created_timestamp"`
		Rating           *int64 `json:"rating"`
	} `json:"results"`
}

// mktRun holds the mutable state of one snapshot run.
type mktRun struct {
	cmd             *cobra.Command
	ctx             context.Context
	c               *client.Client
	db              *sql.DB
	snap            *mktSnapshot
	maxCalls        int
	reviewTop       int
	perPage         int
	runCalls        int
	unresolvedShops int
}

func (r *mktRun) mktSpend() bool {
	return r.runCalls < r.maxCalls
}

func (r *mktRun) chargeAttempt() error {
	if r.runCalls >= r.maxCalls {
		return errMktBudget
	}
	r.runCalls++
	r.snap.CallsUsed++
	return nil
}

func (r *mktRun) mktSave(status string) error {
	if status != "" {
		r.snap.Status = status
	}
	prog, _ := json.Marshal(struct {
		mktProg
		PerPage   int `json:"per_page"`
		ReviewTop int `json:"review_top"`
	}{r.snap.Progress, r.perPage, r.reviewTop})
	_, err := r.db.Exec(`UPDATE market_snapshots SET calls_used = ?, status = ?, max_calls = ?, progress_json = ? WHERE id = ?`,
		r.snap.CallsUsed, r.snap.Status, r.maxCalls, string(prog), r.snap.ID)
	return err
}

// mktDigitalFromListingType maps download and both to 1 (both counts as digital),
// physical to 0, and unknown values to SQL NULL, ignoring case and outer spaces.
func mktDigitalFromListingType(listingType *string) any {
	if listingType != nil {
		switch strings.ToLower(strings.TrimSpace(*listingType)) {
		case "download", "both":
			return 1
		case "physical":
			return 0
		}
	}
	return nil
}

func mktBoolInt(b *bool) any {
	if b == nil {
		return nil
	}
	if *b {
		return 1
	}
	return 0
}

func (r *mktRun) mktFetchPage(term string, page int) (int, error) {
	raw, err := r.c.Get(r.ctx, "/listings/active", map[string]string{
		"keywords": term,
		"limit":    fmt.Sprint(r.perPage),
		"offset":   fmt.Sprint((page - 1) * r.perPage),
		"sort_on":  r.snap.SortOn,
	})
	if err != nil {
		return 0, err
	}
	var p mktListingPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return 0, fmt.Errorf("decode /listings/active: %w", err)
	}
	if p.Results == nil {
		return 0, fmt.Errorf("decode /listings/active: response has no results array")
	}
	tx, err := r.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT OR REPLACE INTO market_term_pages (snapshot_id, term, page, result_count, listings) VALUES (?,?,?,?,?)`, r.snap.ID, term, page, p.Count, len(p.Results)); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(`DELETE FROM market_results WHERE snapshot_id=? AND term=? AND page=?`, r.snap.ID, term, page); err != nil {
		return 0, err
	}
	for i, l := range p.Results {
		var amt, div any
		cur := ""
		if l.Price != nil {
			if l.Price.Amount != nil {
				amt = *l.Price.Amount
			}
			if l.Price.Divisor != nil {
				div = *l.Price.Divisor
			}
			cur = l.Price.CurrencyCode
		}
		tags, _ := json.Marshal(append([]string{}, l.Tags...))
		pos := (page-1)*r.perPage + i + 1
		_, err := tx.Exec(`INSERT INTO market_results
			(snapshot_id, term, page, position, listing_id, shop_id, title, price_amount, price_divisor, currency,
			 num_favorers, views, tags_json, taxonomy_id, original_creation_ts, is_digital, result_count)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
            ON CONFLICT(snapshot_id, term, listing_id) DO UPDATE SET
            page=excluded.page, position=excluded.position, shop_id=excluded.shop_id, title=excluded.title,
            price_amount=excluded.price_amount, price_divisor=excluded.price_divisor, currency=excluded.currency,
            num_favorers=excluded.num_favorers, views=excluded.views, tags_json=excluded.tags_json,
            taxonomy_id=excluded.taxonomy_id, original_creation_ts=excluded.original_creation_ts,
            is_digital=excluded.is_digital, result_count=excluded.result_count
            WHERE excluded.position < market_results.position`,
			r.snap.ID, term, page, pos, l.ListingID, mktPtr(l.ShopID), l.Title, amt, div, cur,
			mktPtr(l.NumFavorers), mktPtr(l.Views), string(tags), mktPtr(l.TaxonomyID), mktPtr(l.CreatedTS), mktDigitalFromListingType(l.ListingType), p.Count)
		if err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(p.Results), nil
}

// mktPtr maps a nil pointer to SQL NULL (never 0).
func mktPtr(p *int64) any {
	if p == nil {
		return nil
	}
	return *p
}

func (r *mktRun) mktFetchShop(shopID int64) error {
	raw, err := r.c.Get(r.ctx, fmt.Sprintf("/shops/%d", shopID), nil)
	if err != nil {
		var ae *client.APIError
		if errors.As(err, &ae) && (ae.StatusCode == 404 || ae.StatusCode == 403) {
			// Unavailable shop: record a NULL row so it is not refetched
			// and downstream views show unknown, not zero.
			_, derr := r.db.Exec(`INSERT OR REPLACE INTO market_shops (snapshot_id, shop_id) VALUES (?,?)`, r.snap.ID, shopID)
			return derr
		}
		return err
	}
	var s mktShopPayload
	if err := json.Unmarshal(raw, &s); err != nil {
		return fmt.Errorf("decode /shops/%d: %w", shopID, err)
	}
	return r.mktStoreShop(shopID, s)
}
func (r *mktRun) mktStoreShop(shopID int64, s mktShopPayload) error {
	var name any
	if s.ShopName != nil {
		if trimmed := strings.TrimSpace(*s.ShopName); trimmed != "" {
			name = trimmed
		}
	}
	var rating any
	if s.ReviewAverage != nil {
		rating = *s.ReviewAverage
	}
	_, err := r.db.Exec(`INSERT OR REPLACE INTO market_shops
		(snapshot_id, shop_id, shop_name, transaction_sold_count, listing_active_count, create_date, review_average, review_count, is_vacation)
		VALUES (?,?,?,?,?,?,?,?,?)`,
		r.snap.ID, shopID, name, mktPtr(s.TransactionSoldCount), mktPtr(s.ListingActiveCount), mktPtr(s.CreateDate), rating, mktPtr(s.ReviewCount), mktBoolInt(s.IsVacation))
	return err
}

var errMktBudget = errors.New("snapshot call budget exhausted")

// A stop between review pages leaves the listing uncommitted for resume.
var errMktReviewPageBudget = errors.New("snapshot review page budget exhausted")

func mktIsDailyQuota(err error) bool {
	return isDailyQuotaErr(err)
}
func (r *mktRun) mktFetchReviews(listingID int64, since time.Time) error {
	var all mktReviewPayload
	capped := false
	for page := 0; page < mktMaxReviewPages; page++ {
		if !r.mktSpend() {
			return errMktReviewPageBudget
		}
		raw, err := r.c.Get(r.ctx, fmt.Sprintf("/listings/%d/reviews", listingID), map[string]string{"limit": fmt.Sprint(mktReviewPageLimit), "offset": fmt.Sprint(page * mktReviewPageLimit), "min_created": fmt.Sprint(since.Unix())})
		if err != nil {
			return err
		}
		var p mktReviewPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return fmt.Errorf("decode reviews: %w", err)
		}
		if p.Results == nil {
			return fmt.Errorf("decode reviews: response has no results array")
		}
		all.Results = append(all.Results, p.Results...)
		if len(p.Results) < 100 || (p.Count > 0 && len(all.Results) >= p.Count) {
			break
		}
		capped = page == mktMaxReviewPages-1
	}
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("DELETE FROM market_reviews WHERE snapshot_id=? AND listing_id=?", r.snap.ID, listingID); err != nil {
		return err
	}
	// Only timestamps and ratings reach storage.
	for _, rv := range all.Results {
		if _, err := tx.Exec("INSERT INTO market_reviews (snapshot_id,listing_id,created_ts,rating) VALUES (?,?,?,?)", r.snap.ID, listingID, rv.CreatedTimestamp, mktPtr(rv.Rating)); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if capped {
		found := false
		for _, id := range r.snap.Progress.ReviewsCapped {
			found = found || id == listingID
		}
		if !found {
			r.snap.Progress.ReviewsCapped = append(r.snap.Progress.ReviewsCapped, listingID)
		}
	}
	return nil
}

func newNovelMarketSnapshotCmd(flags *rootFlags) *cobra.Command {
	var flagTerm []string
	var flagTerms string
	var flagPages int
	var flagPerPage int
	var flagSortOn string
	var flagWithReviews bool
	var flagReviewWindow string
	var flagReviewTop int
	var flagMaxCalls int
	var flagIgnoreBudget bool
	var flagResume int64

	cmd := &cobra.Command{
		Use:   "snapshot",
		Short: "Capture a dated snapshot of the listings and shops that appear for your search terms.",
		Long: `Capture a dated snapshot of the listings and shops that appear for your search terms.

Runs GET /listings/active per term and page (sort_on=score), looks up each unique shop once,
and optionally collects review timestamps and ratings (never review text or buyer ids).
Shops use batches of 100 representative listings. Review estimates assume one call
per listing; up to ten pages of 100 reviews may be fetched for each listing.
Calls are checked against the daily Etsy budget first and capped by --max-calls. A capped run
is saved as partial; continue it with --resume ID (--max-calls then applies to that run).`,
		Example: "  etsy-pp-cli market snapshot --term \"linen apron\" --term \"ceramic mug\" --pages 2 --max-calls 200\n" +
			"  etsy-pp-cli market snapshot --terms terms.txt --with-reviews --review-window 12mo\n" +
			"  etsy-pp-cli market snapshot --resume 3",
		// etsy-ext: declare filesystem sinks for the MCP walker.
		Annotations: map[string]string{"mcp:read-flags": "terms", "mcp:local-write": "true", "pp:data-source": "live", "pp:happy-args": "--term=linen apron", "pp:live-happy-path": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// etsy-ext: MCP callers cannot bypass budget checks.
			if cliutil.IsMCPChildEnv() {
				if flagIgnoreBudget {
					return usageErr(fmt.Errorf("--ignore-budget is not available over MCP"))
				}
				etsyMCPClamp(cmd, "max-calls", &flagMaxCalls, etsyMCPMaxCalls, false)
				etsyMCPClamp(cmd, "pages", &flagPages, etsyMCPMaxSnapshotPages, false)
			}

			if flagPerPage < 1 || flagPerPage > mktPageLimit {
				return usageErr(fmt.Errorf("--per-page must be between 1 and 100"))
			}
			terms := make([]string, 0)
			for _, t := range flagTerm {
				if t = strings.TrimSpace(t); t != "" {
					terms = append(terms, t)
				}
			}
			if flagTerms != "" {
				more, err := mktReadLines(flagTerms)
				if err != nil {
					if dryRunOK(flags) {
						if flags.asJSON {
							return mktWriteDryRun(cmd, "market snapshot", map[string]any{
								"note":       fmt.Sprintf("terms file %s not readable (%v); plan omits its terms", flagTerms, err),
								"terms_file": flagTerms,
							})
						}
						fmt.Fprintf(cmd.OutOrStdout(), "dry-run: would snapshot terms from %s (not readable now: %v)\n", flagTerms, err)
						return nil
					}
					return usageErr(fmt.Errorf("read --terms: %w", err))
				}
				terms = append(terms, more...)
			}
			if flagPages < 1 || flagPages > mktMaxPages {
				return usageErr(fmt.Errorf("--pages must be between 1 and %d", mktMaxPages))
			}
			if flagMaxCalls < 1 {
				return usageErr(fmt.Errorf("--max-calls must be at least 1"))
			}
			// etsy-ext: Resume uses the saved horizon, ignoring the new flag.
			var window time.Duration
			var err error
			if flagResume == 0 {
				window, err = mktParseWindow(flagReviewWindow)
				if err != nil {
					return usageErr(fmt.Errorf("--review-window: %w", err))
				}
				if window <= 0 || window%(24*time.Hour) != 0 {
					return usageErr(fmt.Errorf("--review-window must be whole days (e.g. 90d, 12mo)"))
				}
			}

			if flagReviewTop < 1 {
				return usageErr(fmt.Errorf("--review-top must be at least 1"))
			}
			if flagResume == 0 && len(terms) == 0 {
				if dryRunOK(flags) || requiredInputWaived() {
					return mktDryRun(cmd, flags, "market snapshot")
				}
				return usageErr(fmt.Errorf("provide --term X (repeatable), --terms FILE, or --resume ID"))
			}

			withReviews := flagWithReviews
			curtailed := false
			if cliutil.IsDogfoodEnv() && flagResume == 0 {
				// Bound live dogfood: one term, one page, no reviews.
				if len(terms) > 1 {
					terms = terms[:1]
				}
				flagPages, withReviews, curtailed = 1, false, true
			}

			ctx := cmd.Context()
			var snap *mktSnapshot
			var rw *store.Store
			var resumeRead *store.Store
			if flagResume > 0 {
				// Read the stored snapshot (read-only) to plan the resume.
				ro, err := mktOpenRead(ctx)
				if err != nil {
					return fmt.Errorf("opening local store: %w", err)
				}
				if ro == nil {
					return notFoundErr(fmt.Errorf("snapshot %d not found (no market data yet)", flagResume))
				}
				snap, err = mktGetSnapshot(ro.DB(), flagResume)
				if err == nil && snap != nil {
					var progress string
					err = ro.DB().QueryRow("SELECT progress_json FROM market_snapshots WHERE id = ?", snap.ID).Scan(&progress)
					if err == nil {
						var saved struct {
							PerPage   int `json:"per_page"`
							ReviewTop int `json:"review_top"`
						}
						err = json.Unmarshal([]byte(progress), &saved)
						if saved.PerPage == 0 {
							saved.PerPage = mktPageLimit
						} // Legacy snapshots used 100.
						flagPerPage = saved.PerPage
						if saved.ReviewTop == 0 {
							saved.ReviewTop = 20
						}
						flagReviewTop = saved.ReviewTop
					}
				}
				resumeRead = ro
				defer func() {
					if resumeRead != nil {
						resumeRead.Close()
					}
				}()
				if err != nil {
					return err
				}
				if snap == nil {
					return notFoundErr(fmt.Errorf("snapshot %d not found", flagResume))
				}
				terms, flagPages, withReviews = snap.Terms, snap.Pages, snap.WithReviews
				// etsy-ext: saved CLI snapshots cannot restore an MCP-unsafe page ceiling.
				if cliutil.IsMCPChildEnv() {
					etsyMCPClamp(cmd, "pages", &flagPages, etsyMCPMaxSnapshotPages, false)
				}
				if snap.ReviewWindowDays > 0 {
					window = time.Duration(snap.ReviewWindowDays) * 24 * time.Hour
				} else {
					// etsy-ext: Legacy snapshots without a saved horizon still use the flag.
					window, err = mktParseWindow(flagReviewWindow)
					if err != nil {
						return usageErr(fmt.Errorf("--review-window: %w", err))
					}
					if window <= 0 || window%(24*time.Hour) != 0 {
						return usageErr(fmt.Errorf("--review-window must be whole days (e.g. 90d, 12mo)"))
					}
				}
			}

			pagesDone := 0
			if snap != nil {
				pagesDone = len(snap.Progress.Pages)
			}
			plan := mktPlanSnapshot(terms, flagPages, flagPerPage, withReviews, flagReviewTop, flagMaxCalls, pagesDone)
			plan.SortOn = flagSortOn
			plan.Curtailed = curtailed
			if snap != nil && snap.Status == "complete" {
				summary := mktSummarize(resumeRead.DB(), &mktRun{snap: snap, maxCalls: flagMaxCalls}, "complete")
				resumeRead.Close()
				resumeRead = nil
				summary["note"] = "snapshot is already complete; nothing to resume"
				return mktEmitReport(cmd, flags, summary)
			}

			if dryRunOK(flags) {
				if flags.asJSON {
					return mktWriteDryRun(cmd, "market snapshot", map[string]any{"plan": plan})
				}
				fmt.Fprintf(cmd.OutOrStdout(), "dry-run: would snapshot %d term(s) x %d page(s) = %d page calls (+ up to %d shop and %d review lookups); planned %d, cap %d\n",
					len(plan.Terms), plan.Pages, plan.PageCalls, plan.ShopCallsEst, plan.ReviewCallsEst, plan.PlannedCalls, plan.MaxCalls)
				return nil
			}

			planned := plan.PlannedCalls
			if planned > flagMaxCalls {
				planned = flagMaxCalls // the run stops at --max-calls
			}
			if err := checkBudget(cmd, planned, flagIgnoreBudget); err != nil {
				return err
			}

			rw, err = store.OpenWithContext(ctx, defaultDBPath(mktCLIName))
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer rw.Close()
			if err := store.EnsureMarketSchema(rw.DB()); err != nil {
				return err
			}
			if snap == nil {
				termsJSON, _ := json.Marshal(terms)
				wr := 0
				if withReviews {
					wr = 1
				}
				res, err := rw.DB().Exec(`INSERT INTO market_snapshots (taken_at, terms_json, pages, sort_on, max_calls, calls_used, status, with_reviews, review_window_days)
					VALUES (?,?,?,?,?,0,'running',?,?)`, time.Now().UTC().Format(time.RFC3339), string(termsJSON), flagPages, flagSortOn, flagMaxCalls, wr, int(window/(24*time.Hour)))
				if err != nil {
					return err
				}
				id, _ := res.LastInsertId()
				snap, err = mktGetSnapshot(rw.DB(), id)
				if err != nil || snap == nil {
					return fmt.Errorf("reload snapshot: %v", err)
				}
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			c.NoCache = true
			run := &mktRun{cmd: cmd, ctx: ctx, c: c, db: rw.DB(), snap: snap, maxCalls: flagMaxCalls, perPage: flagPerPage, reviewTop: flagReviewTop}
			if err := run.mktSave(""); err != nil {
				return err
			}
			c.OnAttempt = run.chargeAttempt
			status, runErr := mktExecute(run, terms, flagPages, withReviews, flagReviewTop, window)
			// etsy-ext: Retry caps have the same successful partial outcome as boundary caps.
			if errors.Is(runErr, errMktBudget) {
				status, runErr = "partial", nil
			}
			if err := run.mktSave(status); err != nil && runErr == nil {
				runErr = err
			}
			summary := mktSummarize(rw.DB(), run, status)
			if runErr != nil {
				summary["error"] = runErr.Error()
				if err := mktEmitReport(cmd, flags, summary); err != nil {
					return err
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "snapshot %d stopped early (%v); progress saved, continue with: etsy-pp-cli market snapshot --resume %d\n", snap.ID, runErr, snap.ID)
				return classifyAPIErrorOnly(runErr)
			}
			if status == "partial" {
				fmt.Fprintf(cmd.ErrOrStderr(), "snapshot %d hit --max-calls=%d and is partial; continue with: etsy-pp-cli market snapshot --resume %d\n", snap.ID, flagMaxCalls, snap.ID)
			}
			return mktEmitReport(cmd, flags, summary)
		},
	}
	cmd.Flags().StringArrayVar(&flagTerm, "term", nil, "Search term (repeatable)")
	cmd.Flags().StringVar(&flagTerms, "terms", "", "File with one search term per line (# comments allowed)")
	cmd.Flags().IntVar(&flagPages, "pages", 1, "Result pages per term (--per-page listings each, max 5)")
	cmd.Flags().IntVar(&flagPerPage, "per-page", mktPageLimit, "Listings requested per search page (1..100)")
	cmd.Flags().StringVar(&flagSortOn, "sort-on", "score", "Etsy sort_on value")
	cmd.Flags().BoolVar(&flagWithReviews, "with-reviews", false, "Also collect review timestamps and ratings for the top listings (no review text)")
	cmd.Flags().StringVar(&flagReviewWindow, "review-window", "12mo", "How far back to collect reviews (e.g. 12mo, 90d)")
	cmd.Flags().IntVar(&flagReviewTop, "review-top", 20, "With --with-reviews, collect reviews for the top N positions per term page")
	cmd.Flags().IntVar(&flagMaxCalls, "max-calls", mktDefaultMaxCall, "Stop after this many API request attempts; save partial and exit 0, including caps during retries")
	cmd.Flags().BoolVar(&flagIgnoreBudget, "ignore-budget", false, "Skip the daily Etsy budget check")
	cmd.Flags().Int64Var(&flagResume, "resume", 0, "Continue a partial snapshot by ID without refetching completed pages")
	return cmd
}

// mktExecute runs the page, shop and review phases and returns the final
// status ("complete" or "partial") plus any hard error.
func mktExecute(r *mktRun, terms []string, pages int, withReviews bool, reviewTop int, window time.Duration) (string, error) {
	for _, term := range terms {
		for page := 1; page <= pages; page++ {
			if r.snap.Progress.hasPage(term, page) {
				continue
			}
			if !r.mktSpend() {
				return "partial", nil
			}
			n, err := r.mktFetchPage(term, page)
			if err != nil {
				return "partial", err
			}
			r.snap.Progress.Pages = append(r.snap.Progress.Pages, fmt.Sprintf("%s|%d", term, page))
			if n < r.perPage {
				// Short page: later pages are empty, skip them for free.
				for p := page + 1; p <= pages; p++ {
					if !r.snap.Progress.hasPage(term, p) {
						r.snap.Progress.Pages = append(r.snap.Progress.Pages, fmt.Sprintf("%s|%d", term, p))
					}
				}
			}
			if err := r.mktSave(""); err != nil {
				return "partial", err
			}
			if n < r.perPage {
				break
			}
		}
	}

	// Shops: unique ids not yet stored for this snapshot (deduped).
	rows, err := r.db.Query(`SELECT shop_id, MIN(listing_id) FROM market_results WHERE snapshot_id = ? AND shop_id IS NOT NULL
		AND shop_id NOT IN (SELECT shop_id FROM market_shops WHERE snapshot_id = ?) GROUP BY shop_id ORDER BY shop_id`, r.snap.ID, r.snap.ID)
	if err != nil {
		return "partial", err
	}
	type representative struct{ shop, listing int64 }
	var shops []representative
	for rows.Next() {
		var id representative
		if err := rows.Scan(&id.shop, &id.listing); err != nil {
			_ = rows.Close()
			return "partial", err
		}
		shops = append(shops, id)
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return "partial", err
	}
	for start := 0; start < len(shops); start += mktBatchLimit {
		chunk := shops[start:min(start+mktBatchLimit, len(shops))]
		if !r.mktSpend() {
			return "partial", nil
		}
		ids := make([]string, len(chunk))
		for i, rep := range chunk {
			ids[i] = fmt.Sprint(rep.listing)
		}
		raw, err := r.c.Get(r.ctx, "/listings/batch", map[string]string{"listing_ids": strings.Join(ids, ","), "includes": "Shop"})
		if mktIsDailyQuota(err) {
			return "partial", err
		}
		if err != nil {
			var ae *client.APIError
			if !errors.As(err, &ae) || (ae.StatusCode != 400 && ae.StatusCode != 404) {
				return "partial", err
			}
			for _, rep := range chunk {
				if !r.mktSpend() {
					return "partial", nil
				}
				if err := r.mktFetchShop(rep.shop); err != nil {
					return "partial", err
				}
			}
		} else {
			var p struct {
				Results []struct {
					ListingID int64           `json:"listing_id"`
					ShopID    int64           `json:"shop_id"`
					Shop      *mktShopPayload `json:"shop"`
				} `json:"results"`
			}
			if err := json.Unmarshal(raw, &p); err != nil {
				return "partial", fmt.Errorf("decode /listings/batch: %w", err)
			}
			if p.Results == nil {
				return "partial", fmt.Errorf("decode /listings/batch: response has no results array")
			}
			for _, rep := range chunk {
				payload := mktShopPayload{}
				found := false
				for _, result := range p.Results {
					id := result.ShopID
					if id == 0 && result.Shop != nil && result.Shop.ShopID != nil {
						id = *result.Shop.ShopID
					}
					if result.ListingID == rep.listing && id == rep.shop && result.Shop != nil {
						payload = *result.Shop
						found = true
						break
					}
				}
				if !found {
					if !r.mktSpend() {
						return "partial", nil
					}
					if err := r.mktFetchShop(rep.shop); err != nil {
						if mktIsDailyQuota(err) {
							return "partial", err
						}
						var ae *client.APIError
						if !errors.As(err, &ae) {
							return "partial", err
						}
						r.unresolvedShops++
						continue
					}
					continue
				}
				if err := r.mktStoreShop(rep.shop, payload); err != nil {
					return "partial", err
				}
			}
		}
		if err := r.mktSave(""); err != nil {
			return "partial", err
		}
	}

	if withReviews {
		since := r.snap.takenTime.Add(-window)
		if r.snap.takenTime.IsZero() {
			since = time.Now().Add(-window)
		}
		lrows, err := r.db.Query(`SELECT listing_id, MIN(position) FROM market_results WHERE snapshot_id = ? AND position <= ?
			GROUP BY listing_id ORDER BY MIN(position), listing_id`, r.snap.ID, reviewTop)
		if err != nil {
			return "partial", err
		}
		var ids []int64
		for lrows.Next() {
			var id int64
			var pos int
			if err := lrows.Scan(&id, &pos); err != nil {
				_ = lrows.Close()
				return "partial", err
			}
			ids = append(ids, id)
		}
		_ = lrows.Close()
		for _, id := range ids {
			if r.snap.Progress.hasReview(id) {
				continue
			}
			if err := r.mktFetchReviews(id, since); err != nil {
				if errors.Is(err, errMktReviewPageBudget) {
					return "partial", nil
				}
				return "partial", err
			}
			r.snap.Progress.ReviewsDone = append(r.snap.Progress.ReviewsDone, id)
			if err := r.mktSave(""); err != nil {
				return "partial", err
			}
		}
	}
	if r.unresolvedShops > 0 {
		return "partial", nil
	}
	return "complete", nil
}

func mktSummarize(db *sql.DB, r *mktRun, status string) map[string]any {
	var listings, shops, reviews int
	_ = db.QueryRow(`SELECT count(DISTINCT listing_id) FROM market_results WHERE snapshot_id = ?`, r.snap.ID).Scan(&listings)
	_ = db.QueryRow(`SELECT count(*) FROM market_shops WHERE snapshot_id = ?`, r.snap.ID).Scan(&shops)
	_ = db.QueryRow(`SELECT count(*) FROM market_reviews WHERE snapshot_id = ?`, r.snap.ID).Scan(&reviews)
	out := map[string]any{
		"snapshot_id":    r.snap.ID,
		"status":         status,
		"terms":          r.snap.Terms,
		"pages":          r.snap.Pages,
		"calls_this_run": r.runCalls,
		"calls_used":     r.snap.CallsUsed,
		"max_calls":      r.maxCalls,
		"listings":       listings,
		"shops":          shops,
		"reviews":        reviews,
		"reviews_capped": len(r.snap.Progress.ReviewsCapped),
	}
	counts := map[string]any{}
	for _, k := range []string{"listings", "shops", "reviews", "reviews_capped"} {
		counts[k] = out[k]
	}
	if r.unresolvedShops > 0 {
		counts["unresolved_shops"] = r.unresolvedShops
	}
	out["results"] = counts
	if len(r.snap.Progress.ReviewsCapped) > 0 {
		out["note"] = "Review collection capped at 1000 reviews per listing."
		out["notes"] = []string{"Review collection capped at 1000 reviews per listing."}
	}
	if r.unresolvedShops > 0 {
		out["unresolved_shops"] = r.unresolvedShops
		note := fmt.Sprintf("%d shop(s) could not be resolved from /listings/batch and were skipped; resume the snapshot to retry.", r.unresolvedShops)
		ns, _ := out["notes"].([]string)
		out["notes"] = append(ns, note)
		if existing, ok := out["note"].(string); ok {
			note = existing + "; " + note
		}
		out["note"] = note
	}
	if status == "partial" {
		out["resume_hint"] = fmt.Sprintf("etsy-pp-cli market snapshot --resume %d", r.snap.ID)
	}
	return out
}
