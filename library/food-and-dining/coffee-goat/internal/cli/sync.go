// Copyright 2026 Justin Fu and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/coffee-goat/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/coffee-goat/internal/roasters"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/coffee-goat/internal/source/coffeereview"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/coffee-goat/internal/source/shopify"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/coffee-goat/internal/source/youtube"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/coffee-goat/internal/store"
	"github.com/spf13/cobra"
)

// Per-source concurrency bounds. Shopify accepts moderate parallelism
// because every roaster is a different origin; Coffee Review and
// YouTube target single origins each so concurrency=1 keeps us nice.
const (
	shopifyConcurrency      = 4
	coffeeReviewConcurrency = 1
	youtubeConcurrency      = 1
)

// sourceResult is the per-source verdict emitted in the final summary.
type sourceResult struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Items      int    `json:"items"`
	DurationMS int64  `json:"duration_ms"`
	Message    string `json:"message,omitempty"`
}

func newSyncCmd(flags *rootFlags) *cobra.Command {
	var sourceFlag string
	var roasterFlag string
	var full bool

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync from coffee sources (Shopify, Coffee Review, YouTube) into the local store",
		Long: `Pull catalog data from Shopify-shaped roasters, editorial signal
from Coffee Review, and creator transcripts from tracked YouTube
channels (Hoffmann, Hedrick) into the local SQLite store.

Sources run in parallel, bounded by per-source concurrency limits.
YouTube transcript fetching requires the optional 'youtube-pp-cli'
binary on PATH; when missing, the YouTube source emits a warning
and the rest of the sync still completes.`,
		Example: `  # Sync everything
  coffee-goat-pp-cli sync

  # Sync only Shopify (the fastest path to a populated catalog)
  coffee-goat-pp-cli sync --source shopify

  # Re-sync one roaster's full catalog
  coffee-goat-pp-cli sync --source shopify --roaster onyx --full`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			if cliutil.IsDogfoodEnv() {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 25*time.Second)
				defer cancel()
			}

			db, err := store.OpenWithContext(ctx, defaultDBPath("coffee-goat-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			selected := normaliseSourceFlag(sourceFlag)
			results := make(chan sourceResult, len(selected))
			var wg sync.WaitGroup

			for _, src := range selected {
				wg.Add(1)
				go func(src string) {
					defer wg.Done()
					results <- runSource(ctx, db, src, roasterFlag, full)
				}(src)
			}

			go func() {
				wg.Wait()
				close(results)
			}()

			summary := struct {
				Sources []sourceResult `json:"sources"`
			}{}
			for r := range results {
				summary.Sources = append(summary.Sources, r)
				if humanFriendly {
					fmt.Fprintf(os.Stderr, "  %s: %s (%d items, %dms)\n", r.Name, r.Status, r.Items, r.DurationMS)
				}
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), summary, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "synced %d sources\n", len(summary.Sources))
			for _, r := range summary.Sources {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s: %s (%d items)\n", r.Name, r.Status, r.Items)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&sourceFlag, "source", "all", "Source(s) to sync: shopify, coffee-review, youtube, all")
	cmd.Flags().StringVar(&roasterFlag, "roaster", "", "When --source=shopify, limit to one roaster slug")
	cmd.Flags().BoolVar(&full, "full", false, "Full resync (ignore cursors)")
	cmd.Annotations = map[string]string{"mcp:read-only": "false"}
	return cmd
}

// normaliseSourceFlag canonicalises the --source value into a slice
// of source names. Unknown values fall through to "all" so a typo
// doesn't silently sync nothing.
func normaliseSourceFlag(v string) []string {
	switch v {
	case "", "all":
		return []string{"shopify", "coffee-review", "youtube"}
	case "shopify", "coffee-review", "youtube":
		return []string{v}
	default:
		return []string{"shopify", "coffee-review", "youtube"}
	}
}

// syncSourceForResource is the canonical resource_type → source mapping.
// One row per syncable resource type. sync.go writes SaveSyncState(rtype, ...)
// after each source completes; auto_refresh.go reverses this map to dispatch
// the right source when a resource is stale; the test surface enumerates the
// resource_type set for freshness assertions.
var syncSourceForResource = map[string]string{
	"products": "shopify",
	"reviews":  "coffee-review",
	"videos":   "youtube",
}

// syncResources is the ordered list of resource types tracked in sync_state.
// Stable order is important so that diagnostic output (e.g. doctor's cache
// report) is deterministic for testing.
var syncResources = []string{"products", "reviews", "videos"}

func runSource(ctx context.Context, db *store.Store, src, roasterFlag string, full bool) sourceResult {
	started := time.Now()
	switch src {
	case "shopify":
		items, msg, status := syncShopify(ctx, db, roasterFlag, full)
		return sourceResult{Name: "shopify", Status: status, Items: items, Message: msg, DurationMS: time.Since(started).Milliseconds()}
	case "coffee-review":
		items, msg, status := syncCoffeeReview(ctx, db)
		return sourceResult{Name: "coffee-review", Status: status, Items: items, Message: msg, DurationMS: time.Since(started).Milliseconds()}
	case "youtube":
		items, msg, status := syncYouTube(ctx, db, full)
		return sourceResult{Name: "youtube", Status: status, Items: items, Message: msg, DurationMS: time.Since(started).Milliseconds()}
	default:
		return sourceResult{Name: src, Status: "skipped", Message: "unknown source", DurationMS: time.Since(started).Milliseconds()}
	}
}

func syncShopify(ctx context.Context, db *store.Store, roasterFlag string, full bool) (int, string, string) {
	var targets []roasters.Roaster
	if roasterFlag != "" {
		r, ok := roasters.BySlug(roasterFlag)
		if !ok {
			return 0, fmt.Sprintf("unknown roaster slug %q", roasterFlag), "error"
		}
		if r.Transport != roasters.TransportShopify {
			return 0, fmt.Sprintf("roaster %q is not a Shopify storefront", roasterFlag), "error"
		}
		targets = []roasters.Roaster{r}
	} else {
		targets = roasters.ShopifyOnly()
	}
	_ = full // cursors not used in this implementation; full is a no-op for now

	fetcher := shopify.New()
	sem := make(chan struct{}, shopifyConcurrency)
	var (
		mu      sync.Mutex
		stored  int
		errs    []string
		wg      sync.WaitGroup
		anyDone bool
	)
	for _, r := range targets {
		wg.Add(1)
		go func(r roasters.Roaster) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			products, err := fetcher.Fetch(ctx, r)
			if err != nil {
				var rl *cliutil.RateLimitError
				if errors.As(err, &rl) {
					mu.Lock()
					errs = append(errs, fmt.Sprintf("%s rate-limited", r.Slug))
					mu.Unlock()
				} else {
					mu.Lock()
					errs = append(errs, fmt.Sprintf("%s: %v", r.Slug, err))
					mu.Unlock()
				}
				return
			}
			for _, p := range products {
				if uerr := db.UpsertRoasterProduct(p.RoasterSlug, p.Handle, map[string]any{
					"title":        p.Title,
					"vendor":       p.Vendor,
					"body_text":    p.BodyText,
					"origin":       p.Origin,
					"producer":     p.Producer,
					"process":      p.Process,
					"varietal":     p.Varietal,
					"altitude":     p.Altitude,
					"tags_json":    p.TagsJSON,
					"price_cents":  p.PriceCents,
					"currency":     p.Currency,
					"weight_g":     p.WeightG,
					"url":          p.URL,
					"image_url":    p.ImageURL,
					"in_stock":     boolToInt(p.InStock),
					"published_at": p.PublishedAt,
					"updated_at":   p.UpdatedAt,
				}); uerr != nil {
					mu.Lock()
					errs = append(errs, fmt.Sprintf("%s upsert: %v", r.Slug, uerr))
					mu.Unlock()
					continue
				}
				mu.Lock()
				stored++
				anyDone = true
				mu.Unlock()
			}
		}(r)
	}
	wg.Wait()

	status := "ok"
	msg := ""
	if len(errs) > 0 {
		status = "partial"
		msg = fmt.Sprintf("%d roasters errored", len(errs))
	}
	if !anyDone && len(errs) > 0 {
		status = "error"
	}
	_ = db.SaveCoffeeSyncState("shopify", status, stored)
	_ = db.SaveSyncState("products", "", stored)
	return stored, msg, status
}

func syncCoffeeReview(ctx context.Context, db *store.Store) (int, string, string) {
	fetcher := coffeereview.New()
	pages := 2
	if cliutil.IsDogfoodEnv() {
		pages = 1
	}
	reviews, err := fetcher.Fetch(ctx, 10, pages)
	if err != nil {
		_ = db.SaveCoffeeSyncState("coffee-review", "error", 0)
		return 0, err.Error(), "error"
	}
	stored := 0
	failed := 0
	for _, rv := range reviews {
		_, ierr := db.DB().ExecContext(ctx,
			`INSERT INTO reviews (id, source, source_url, roaster_name, bean_name, score, descriptors_json, published_at, reviewer, raw_json)
			 VALUES (?, 'coffeereview', ?, ?, ?, ?, ?, ?, ?, ?)
			 ON CONFLICT(id) DO UPDATE SET
			   source_url=excluded.source_url,
			   roaster_name=excluded.roaster_name,
			   bean_name=excluded.bean_name,
			   score=excluded.score,
			   raw_json=excluded.raw_json,
			   last_seen_at=CURRENT_TIMESTAMP`,
			rv.ID, rv.SourceURL, rv.RoasterName, rv.BeanName, rv.Score, "", rv.PublishedAt, rv.Reviewer, rv.RawJSON,
		)
		if ierr != nil {
			failed++
			continue
		}
		stored++
	}
	status := "ok"
	msg := ""
	if failed > 0 {
		status = "partial"
		msg = fmt.Sprintf("%d reviews failed to persist", failed)
		if stored == 0 {
			status = "error"
		}
	}
	_ = db.SaveCoffeeSyncState("coffee-review", status, stored)
	_ = db.SaveSyncState("reviews", "", stored)
	return stored, msg, status
}

func syncYouTube(ctx context.Context, db *store.Store, full bool) (int, string, string) {
	fetcher := youtube.New()
	stored := 0
	failed := 0
	for _, creator := range youtube.TrackedCreators {
		lastSyncedAt := time.Time{}
		if !full {
			if ts, err := db.LastCoffeeSyncAt("youtube:" + creator.Slug); err == nil && ts.Valid {
				lastSyncedAt, _ = time.Parse(time.RFC3339, ts.String)
			}
		}
		videos, err := fetcher.Fetch(ctx, creator, lastSyncedAt)
		if err != nil {
			if errors.Is(err, youtube.ErrYoutubeCliMissing) {
				_ = db.SaveCoffeeSyncState("youtube", "skipped", 0)
				return 0, "youtube-pp-cli not installed; skipping creator transcript sync", "warn"
			}
			_ = db.SaveCoffeeSyncState("youtube", "error", stored)
			return stored, err.Error(), "error"
		}
		for _, v := range videos {
			tx, txErr := db.DB().BeginTx(ctx, nil)
			if txErr != nil {
				failed++
				continue
			}
			_, ierr := tx.ExecContext(ctx,
				`INSERT INTO youtube_reviews (video_id, creator, channel_id, video_title, video_published_at, transcript_text, mentioned_roaster_slugs_json, mentioned_bean_handles_json, last_synced_at)
				 VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
				 ON CONFLICT(video_id) DO UPDATE SET
				   video_title=excluded.video_title,
				   transcript_text=excluded.transcript_text,
				   mentioned_roaster_slugs_json=excluded.mentioned_roaster_slugs_json,
				   mentioned_bean_handles_json=excluded.mentioned_bean_handles_json,
				   last_synced_at=CURRENT_TIMESTAMP`,
				v.VideoID, v.Creator, v.ChannelID, v.VideoTitle, v.VideoPublishedAt,
				v.TranscriptText, v.MentionedRoasterSlugsJSON, v.MentionedBeanHandlesJSON,
			)
			if ierr != nil {
				_ = tx.Rollback()
				failed++
				continue
			}
			// Mirror into FTS5 atomically with the main row.
			if _, ftsErr := tx.ExecContext(ctx,
				`INSERT OR REPLACE INTO youtube_reviews_fts (rowid, video_title, transcript_text)
				 SELECT rowid, video_title, transcript_text FROM youtube_reviews WHERE video_id=?`,
				v.VideoID,
			); ftsErr != nil {
				_ = tx.Rollback()
				failed++
				continue
			}
			if commitErr := tx.Commit(); commitErr != nil {
				failed++
				continue
			}
			stored++
		}
		_ = db.SaveCoffeeSyncState("youtube:"+creator.Slug, "ok", len(videos))
	}
	status := "ok"
	msg := ""
	if failed > 0 {
		status = "partial"
		msg = fmt.Sprintf("%d videos failed to persist", failed)
		if stored == 0 {
			status = "error"
		}
	}
	_ = db.SaveCoffeeSyncState("youtube", status, stored)
	_ = db.SaveSyncState("videos", "", stored)
	return stored, msg, status
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// unused but kept so unused-import lint doesn't fire on json
var _ = json.Marshal
