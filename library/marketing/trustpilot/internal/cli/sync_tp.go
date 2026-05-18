package cli

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/spf13/cobra"

	tpkg "github.com/mvanhorn/printing-press-library/library/marketing/trustpilot/internal/trustpilot"
)

func newTPSyncCmd(flags *rootFlags) *cobra.Command {
	var maxPages, stars int
	var resume, noResume, bustCutoff bool
	cmd := &cobra.Command{
		Use:     "sync-trustpilot <domain>",
		Aliases: []string{"synctp"},
		Short:   "Sync reviews for a company into the local SQLite store",
		Long:    "Syncs reviews for a company into the local SQLite store. Trustpilot caps each filter at roughly 10 pages (200 reviews); when that cap is detected, the command stops without advancing the cursor and suggests --bust-cutoff to iterate stars=1..5.",
		Example: `  trustpilot-pp-cli sync-trustpilot www.thriftbooks.com --max-pages 25 --json
  trustpilot-pp-cli synctp bookshop.org --stars 1 --no-resume`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if noResume {
				resume = false
			}
			domain := normalizeDomain(args[0])
			ctx := cmd.Context()
			db, err := openTPStore(ctx)
			if err != nil {
				return err
			}
			defer db.Close()
			sess, _, err := loadOrHarvestSession(ctx, db, true)
			if err != nil {
				return err
			}
			start := 1
			if resume {
				last, _, err := tpkg.GetCursor(ctx, db, domain)
				if err != nil {
					return err
				}
				if last > 0 {
					start = last + 1
				}
			}
			stats, err := syncTPPages(cmd, flags, ctx, db, &sess, domain, stars, start, maxPages)
			if err != nil {
				return err
			}
			if bustCutoff && stars == 0 && stats.PagesFetched < maxPages {
				for s := 1; s <= 5; s++ {
					extra, err := syncTPPages(cmd, flags, ctx, db, &sess, domain, s, 1, maxPages-stats.PagesFetched)
					if err != nil {
						return err
					}
					stats.PagesFetched += extra.PagesFetched
					stats.ReviewsInserted += extra.ReviewsInserted
					stats.ReviewsUpdated += extra.ReviewsUpdated
					if extra.TotalAvailable > stats.TotalAvailable {
						stats.TotalAvailable = extra.TotalAvailable
					}
					stats.Cursor = extra.Cursor
				}
			}
			payload := map[string]any{
				"domain":          domain,
				"pagesFetched":    stats.PagesFetched,
				"reviewsInserted": stats.ReviewsInserted,
				"reviewsUpdated":  stats.ReviewsUpdated,
				"totalAvailable":  stats.TotalAvailable,
				"cursor":          stats.Cursor,
			}
			meta := NewMeta("live")
			for _, notice := range stats.Notices {
				meta.AddNotice(notice)
			}
			attachMeta(payload, meta)
			return flags.printJSON(cmd, payload)
		},
	}
	cmd.Flags().IntVar(&maxPages, "max-pages", 25, "Maximum pages to fetch")
	cmd.Flags().IntVar(&stars, "stars", 0, "Filter to a specific star rating (0 = all)")
	cmd.Flags().BoolVar(&resume, "resume", true, "Resume from the saved sync cursor")
	cmd.Flags().BoolVar(&noResume, "no-resume", false, "Start at page 1 instead of the saved cursor")
	cmd.Flags().BoolVar(&bustCutoff, "bust-cutoff", false, "Iterate stars=1..5 if the default fetch runs out of pages")
	return cmd
}

type tpSyncStats struct {
	PagesFetched    int
	ReviewsInserted int
	ReviewsUpdated  int
	TotalAvailable  int
	Cursor          map[string]any
	Notices         []string
}

func syncTPPages(cmd *cobra.Command, flags *rootFlags, ctx context.Context, db *sql.DB, sess *tpkg.Session, domain string, stars, start, maxPages int) (tpSyncStats, error) {
	_ = flags
	var stats tpSyncStats
	if maxPages <= 0 {
		return stats, nil
	}
	for pageNo := start; stats.PagesFetched < maxPages; pageNo++ {
		pp, err := fetchPageWithRetry(ctx, db, sess, domain, tpkg.PageFilters{Page: pageNo, Stars: stars, Sort: "recency"})
		if err != nil {
			return stats, err
		}
		// PATCH: Stop before writes when Trustpilot signals its per-filter pagination cap.
		if len(pp.Reviews) == 0 && pp.Pagination.TotalPages == 0 {
			notice := fmt.Sprintf("%s_at_page_%d", NoticeTrustpilotCapHit, pageNo)
			stats.Notices = append(stats.Notices, notice)
			fmt.Fprintln(cmd.ErrOrStderr(), "Trustpilot caps each filter at ~10 pages (200 reviews). Run with --bust-cutoff to iterate stars=1..5 and reach more.")
			break
		}
		if err := tpkg.UpsertCompany(ctx, db, pp.BusinessUnit); err != nil {
			return stats, err
		}
		inserted, updated, err := tpkg.UpsertReviews(ctx, db, pp.Reviews)
		if err != nil {
			return stats, err
		}
		stats.PagesFetched++
		stats.ReviewsInserted += inserted
		stats.ReviewsUpdated += updated
		stats.TotalAvailable = pp.Pagination.TotalCount
		if err := tpkg.SaveCursor(ctx, db, domain, pageNo, pp.Pagination.TotalPages, pp.Pagination.TotalCount, sess.ReviewsBuildID); err != nil {
			return stats, err
		}
		stats.Cursor = map[string]any{
			"lastPage":   pageNo,
			"totalPages": pp.Pagination.TotalPages,
			"totalCount": pp.Pagination.TotalCount,
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "synced page %d/%d (%d inserted, %d updated)\n", pageNo, pp.Pagination.TotalPages, inserted, updated)
		if pp.Pagination.TotalPages > 0 && pageNo >= pp.Pagination.TotalPages {
			break
		}
		if len(pp.Reviews) == 0 {
			break
		}
	}
	return stats, nil
}
