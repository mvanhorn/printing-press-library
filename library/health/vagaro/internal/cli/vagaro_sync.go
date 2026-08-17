// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored Phase 3 foundation: local-store sync for Vagaro businesses.

package cli

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/health/vagaro/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/health/vagaro/internal/store"
	"github.com/mvanhorn/printing-press-library/library/health/vagaro/internal/vagaro"
	"github.com/spf13/cobra"
)

// nowSnapshotStamp returns a nanosecond-precision UTC timestamp so two syncs
// in the same second still produce distinct menu snapshots (menu-diff needs
// at least two distinct snapshot times to compare).
func nowSnapshotStamp() string { return time.Now().UTC().Format(time.RFC3339Nano) }

// vagaroResources are the sync-able resource names.
var vagaroResources = []string{"businesses", "services", "providers", "reviews"}

// cachedBusinessID returns a slug's businessID from the local store, or empty
// string when the store, table, or row is missing. Best-effort: any error
// falls back to a live resolve by the caller.
func cachedBusinessID(ctx context.Context, slug string) string {
	dbPath := defaultDBPath("vagaro-pp-cli")
	if _, err := os.Stat(dbPath); err != nil {
		return ""
	}
	db, err := store.OpenReadOnlyContext(ctx, dbPath)
	if err != nil {
		return ""
	}
	defer db.Close()
	id, err := db.GetBusinessIDBySlug(ctx, slug)
	if err != nil {
		return ""
	}
	return id
}

func parseResourceSet(csv string) (map[string]bool, error) {
	set := map[string]bool{}
	if strings.TrimSpace(csv) == "" {
		for _, r := range vagaroResources {
			set[r] = true
		}
		return set, nil
	}
	valid := map[string]bool{}
	for _, r := range vagaroResources {
		valid[r] = true
	}
	for _, part := range strings.Split(csv, ",") {
		r := strings.TrimSpace(strings.ToLower(part))
		if r == "" {
			continue
		}
		if !valid[r] {
			return nil, fmt.Errorf("unknown resource %q (valid: %s)", r, strings.Join(vagaroResources, ", "))
		}
		set[r] = true
	}
	if len(set) == 0 {
		return nil, fmt.Errorf("no valid resources in %q", csv)
	}
	return set, nil
}

func newVagaroSyncCmd(flags *rootFlags) *cobra.Command {
	var resources string
	var slugs []string
	var city string

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync Vagaro businesses and their services/providers/reviews into the local store",
		Long: `Populate the local SQLite store from the live Vagaro endpoints so 'sql',
'search', and the cross-business commands have data to work with.

Seed with one or more --slug flags. With no --slug, re-syncs every business
already known in the store.`,
		Example: "  vagaro-pp-cli sync --slug centralbarber --resources businesses,services,providers,reviews",
		// pp:data-source live
		Annotations: map[string]string{"pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			res, err := parseResourceSet(resources)
			if err != nil {
				return usageErr(err)
			}
			if len(slugs) == 0 && strings.TrimSpace(city) != "" {
				return usageErr(fmt.Errorf("city-seeded discovery is not available yet; pass --slug <slug> " +
					"(use 'vagaro-pp-cli listings <service> <city--state>' to find slugs)"))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			db, err := store.OpenWithContext(ctx, defaultDBPath("vagaro-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()
			if err := db.EnsureVagaroTables(ctx); err != nil {
				return err
			}

			seeds := normalizeSlugs(slugs)
			if len(seeds) == 0 {
				seeds, err = db.ListBusinessSlugs(ctx)
				if err != nil {
					return err
				}
			}
			if len(seeds) == 0 {
				return usageErr(fmt.Errorf("no businesses to sync: pass --slug <slug> (repeatable) to seed"))
			}
			// Dogfood bound: one business, small review page, so the flat
			// per-command timeout is not tripped.
			dogfood := cliutil.IsDogfoodEnv()
			if dogfood && len(seeds) > 1 {
				seeds = seeds[:1]
			}
			reviewPageSize := 20
			if dogfood {
				reviewPageSize = 5
			}

			c := newVagaroClient(flags)
			summary := syncSummary{Resources: sortedResourceList(res)}
			for _, slug := range seeds {
				prof, err := c.FetchProfile(ctx, slug)
				if err != nil {
					summary.addError(slug, classifyVagaroError(err, flags))
					continue
				}
				bid := prof.BusinessID
				if res["businesses"] {
					if err := db.UpsertBusiness(ctx, businessRecordFromProfile(prof)); err != nil {
						summary.addError(slug, err)
					} else {
						summary.Businesses++
					}
				} else {
					// Always cache the slug->id mapping so later reads skip
					// the HTML fetch, even when only child resources are synced.
					_ = db.UpsertBusiness(ctx, store.BusinessRecord{Slug: prof.Slug, BusinessID: bid})
				}
				if res["services"] {
					rows, err := c.Services(ctx, bid)
					if err != nil {
						summary.addError(slug, err)
					} else {
						recs := serviceRecords(rows)
						if err := db.UpsertServices(ctx, bid, recs); err != nil {
							summary.addError(slug, err)
						} else {
							summary.Services += len(rows)
						}
						// Append a timestamped snapshot so menu-diff can compare
						// this sync against a prior one. Best-effort: a snapshot
						// failure must not fail the current-state sync above.
						if err := db.InsertServiceSnapshot(ctx, bid, nowSnapshotStamp(), recs); err != nil {
							summary.addError(slug, err)
						}
					}
				}
				if res["providers"] {
					rows, err := c.Staff(ctx, bid)
					if err != nil {
						summary.addError(slug, err)
					} else if err := db.UpsertProviders(ctx, bid, providerRecords(rows)); err != nil {
						summary.addError(slug, err)
					} else {
						summary.Providers += len(rows)
					}
				}
				if res["reviews"] {
					rows, err := c.Reviews(ctx, bid, "", reviewPageSize)
					if err != nil {
						summary.addError(slug, err)
					} else if err := db.UpsertReviews(ctx, bid, reviewRecords(rows)); err != nil {
						summary.addError(slug, err)
					} else {
						summary.Reviews += len(rows)
					}
				}
				summary.Synced = append(summary.Synced, slug)
			}
			if len(summary.Errors) > 0 && len(summary.Synced) == 0 {
				return apiErr(fmt.Errorf("sync failed for all %d business(es); first error: %s", len(seeds), summary.Errors[0].Error))
			}
			return emitVagaro(cmd, flags, summary)
		},
	}
	cmd.Flags().StringVar(&resources, "resources", "", "Comma-separated resources to sync: "+strings.Join(vagaroResources, ", ")+" (default all)")
	cmd.Flags().StringArrayVar(&slugs, "slug", nil, "Business slug to sync (repeatable)")
	cmd.Flags().StringVar(&city, "city", "", "City seed for discovery (reserved; use --slug for now)")
	return cmd
}

type syncError struct {
	Slug  string `json:"slug"`
	Error string `json:"error"`
}

type syncSummary struct {
	Resources  []string    `json:"resources"`
	Synced     []string    `json:"synced"`
	Businesses int         `json:"businesses"`
	Services   int         `json:"services"`
	Providers  int         `json:"providers"`
	Reviews    int         `json:"reviews"`
	Errors     []syncError `json:"errors,omitempty"`
}

func (s *syncSummary) addError(slug string, err error) {
	s.Errors = append(s.Errors, syncError{Slug: slug, Error: err.Error()})
	fmt.Fprintf(os.Stderr, "warning: sync %s: %v\n", slug, err)
}

func normalizeSlugs(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.Trim(strings.TrimSpace(s), "/")
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func sortedResourceList(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for _, r := range vagaroResources {
		if set[r] {
			out = append(out, r)
		}
	}
	return out
}

func businessRecordFromProfile(p vagaro.BusinessProfile) store.BusinessRecord {
	return store.BusinessRecord{
		Slug:        p.Slug,
		BusinessID:  p.BusinessID,
		Name:        p.Name,
		Rating:      p.Rating,
		ReviewCount: p.ReviewCount,
		PriceRange:  p.PriceRange,
		City:        p.City,
		State:       p.State,
		Address:     p.Address,
		Phone:       p.Phone,
		Category:    p.Category,
	}
}

func serviceRecords(rows []vagaro.ServiceRow) []store.ServiceRecord {
	out := make([]store.ServiceRecord, 0, len(rows))
	for _, r := range rows {
		out = append(out, store.ServiceRecord{
			ServiceID:  strconv.FormatInt(r.ServiceID, 10),
			Title:      r.ServiceTitle,
			PriceText:  r.PriceText,
			PriceCents: r.PriceCents,
			Category:   r.Category,
		})
	}
	return out
}

func providerRecords(rows []vagaro.Provider) []store.ProviderRecord {
	out := make([]store.ProviderRecord, 0, len(rows))
	for _, r := range rows {
		out = append(out, store.ProviderRecord{
			ProviderID: strconv.FormatInt(r.ServiceProviderID, 10),
			Name:       r.Name,
		})
	}
	return out
}

func reviewRecords(rows []vagaro.Review) []store.ReviewRecord {
	out := make([]store.ReviewRecord, 0, len(rows))
	for _, r := range rows {
		out = append(out, store.ReviewRecord{
			ReviewID: strconv.FormatInt(r.ReviewID, 10),
			Rating:   r.Rating,
			Text:     r.Text,
			Author:   r.Author,
			Date:     r.Date,
		})
	}
	return out
}
