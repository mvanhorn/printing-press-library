// `cl-sync run` is the Craigslist-specific store populator that the framework
// `sync` cannot replace because the sapi positional-array response shape is
// not the auto-mapped JSON the generator expects. Idempotent — UpsertListing
// handles "seen this pid" via INSERT ... ON CONFLICT.
//
// Sibling top-level command (kebab-cased) so it does not collide with the
// generator's reserved `sync` namespace.

package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/craigslist/internal/source/craigslist"
	"github.com/mvanhorn/printing-press-library/library/commerce/craigslist/internal/store"

	"github.com/spf13/cobra"
)

func newCLSyncCmd(flags *rootFlags) *cobra.Command {
	var site, category, since string
	var limit, page int
	var withDetail bool

	cmd := &cobra.Command{
		Use:         "cl-sync",
		Short:       "Populate the local store from a Craigslist site/category",
		Long:        "Walk sapi pages for a (site, category) pair and upsert listings into the local SQLite store. Optional --with-detail hydrates body and attributes by following each row through rapi.",
		Example:     "  craigslist-pp-cli cl-sync --site sfbay --category sss --since 24h",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			db, err := openCLStore(ctx)
			if err != nil {
				return err
			}
			defer db.Close()

			cutoff, err := parseDuration(since)
			if err != nil {
				return err
			}
			c := craigslist.New(1.0)

			summary, err := runCLSync(ctx, c, db, site, category, page, limit, withDetail, cutoff, cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "synced %d new, %d updated, %d skipped\n", summary.New, summary.Updated, summary.Skipped)
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), summary, flags)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&site, "site", "sfbay", "Craigslist site to sync from")
	cmd.Flags().StringVar(&category, "category", "sss", "Category abbreviation (e.g. sss, apa, sof)")
	cmd.Flags().StringVar(&since, "since", "", "Skip listings older than this duration (e.g. 24h, 3d)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Cap total upserts (0 = no cap; safety net for large categories)")
	cmd.Flags().IntVar(&page, "page", 1, "Starting page (1-indexed)")
	cmd.Flags().BoolVar(&withDetail, "with-detail", false, "Hydrate body and attributes via rapi (slower)")
	return cmd
}

// clSyncSummary is the typed shape returned at the end of a sync run. Stable
// because dogfood probes parse the line-form summary and downstream agents
// parse the JSON form.
type clSyncSummary struct {
	Site     string `json:"site"`
	Category string `json:"category"`
	Pages    int    `json:"pages"`
	New      int    `json:"new"`
	Updated  int    `json:"updated"`
	Skipped  int    `json:"skipped"`
}

// runCLSync walks pages until exhausted or the limit is hit. We treat any
// pid we have already seen as Updated (since UpsertListing always upserts);
// "skipped" reflects the --since cutoff.
func runCLSync(ctx context.Context, c *craigslist.Client, db *store.Store, site, category string, startPage, limit int, withDetail bool, cutoff time.Duration, stderr interface {
	Write(p []byte) (n int, err error)
}) (clSyncSummary, error) {
	if site == "" {
		return clSyncSummary{}, fmt.Errorf("site is required")
	}
	summary := clSyncSummary{Site: site, Category: category}
	thresholdUnix := int64(0)
	if cutoff > 0 {
		thresholdUnix = time.Now().Add(-cutoff).Unix()
	}
	for p := maxInt(startPage, 1); ; p++ {
		summary.Pages++
		fmt.Fprintf(os.Stderr, "fetching page %d...\n", p)
		sr, err := c.Search(ctx, site, craigslist.SearchQuery{
			SearchPath: category,
			Page:       p,
		})
		if err != nil {
			return summary, fmt.Errorf("page %d: %w", p, err)
		}
		if len(sr.Items) == 0 {
			break
		}
		for _, item := range sr.Items {
			if thresholdUnix > 0 && item.PostedAt > 0 && item.PostedAt < thresholdUnix {
				summary.Skipped++
				continue
			}
			row, err := db.GetListing(ctx, item.PostingID)
			updated := err == nil && row != nil
			cl := store.CLListing{
				PID:          item.PostingID,
				UUID:         item.UUID,
				Site:         site,
				Subarea:      item.Subarea,
				Neighborhood: item.Neighborhood,
				CategoryAbbr: category,
				CategoryID:   item.CategoryID,
				Title:        item.Title,
				Price:        item.Price,
				PriceDisplay: item.PriceDisplay,
				Lat:          item.Latitude,
				Lng:          item.Longitude,
				Images:       item.Images,
				CanonicalURL: item.CanonicalURL,
				Slug:         item.Slug,
				PostedAt:     item.PostedAt,
				UpdatedAt:    time.Now().Unix(),
			}
			if withDetail && item.UUID != "" {
				if d, err := c.GetListing(ctx, item.UUID); err == nil {
					cl.Body = d.Body
					cl.BodyText = d.BodyText
					attrs := map[string]string{}
					for _, a := range d.Attributes {
						attrs[a.PostingAttributeKey] = a.Value
					}
					cl.Attributes = attrs
				}
			}
			if err := db.UpsertListing(ctx, cl); err != nil {
				return summary, fmt.Errorf("upsert pid %d: %w", item.PostingID, err)
			}
			if updated {
				summary.Updated++
			} else {
				summary.New++
			}
			if limit > 0 && (summary.New+summary.Updated) >= limit {
				return summary, nil
			}
		}
		if len(sr.Items) < 100 { // sapi default batch is 360; small pages signal exhaustion
			break
		}
	}
	return summary, nil
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
