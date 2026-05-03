// `since <duration>` walks the date+cat sitemap for the given window and
// returns listing URLs not yet in the local store. Useful for "what is new
// in this category in this city in the last N hours" without setting up a
// saved search first.

package cli

import (
	"context"
	"database/sql"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/craigslist/internal/source/craigslist"
	"github.com/mvanhorn/printing-press-library/library/commerce/craigslist/internal/store"

	"github.com/spf13/cobra"
)

// sinceHit is the typed shape per fresh URL.
type sinceHit struct {
	URL string `json:"url"`
	PID int64  `json:"pid,omitempty"`
}

func newSinceCmd(flags *rootFlags) *cobra.Command {
	var site, category, query string
	var withDetail bool
	cmd := &cobra.Command{
		Use:         "since [duration]",
		Short:       "URLs of listings posted within the last duration",
		Long:        "Walk the per-day Craigslist sitemap for the given site and meta-category and return listing URLs we have not stored yet.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			d, err := parseDuration(args[0])
			if err != nil {
				return err
			}
			meta := craigslist.MetaCategoryFromAbbr(category)
			c := craigslist.New(1.0)
			urls, err := c.FreshListingsWindow(cmd.Context(), site, meta, d)
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			db, err := openCLStore(ctx)
			if err != nil {
				return err
			}
			defer db.Close()
			out := filterFreshUnseen(ctx, db, urls)
			if strings.TrimSpace(query) != "" {
				q := strings.ToLower(query)
				kept := out[:0]
				for _, h := range out {
					if strings.Contains(strings.ToLower(h.URL), q) {
						kept = append(kept, h)
					}
				}
				out = kept
			}
			_ = withDetail // reserved; --with-detail would hydrate via rapi
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&site, "site", "sfbay", "Craigslist site")
	cmd.Flags().StringVar(&category, "category", "sss", "Category abbreviation (rolled up to its meta-cat)")
	cmd.Flags().StringVar(&query, "query", "", "Substring filter on URL")
	cmd.Flags().BoolVar(&withDetail, "with-detail", false, "Hydrate detail via rapi (slower; reserved)")
	return cmd
}

// filterFreshUnseen drops URLs whose embedded PID is already in the local
// store and parses out PIDs for the rest.
func filterFreshUnseen(ctx context.Context, db *store.Store, urls []craigslist.SitemapURL) []sinceHit {
	out := make([]sinceHit, 0, len(urls))
	for _, u := range urls {
		pid := pidFromURL(u.Loc)
		if pid > 0 {
			if existsListing(ctx, db.DB(), pid) {
				continue
			}
		}
		out = append(out, sinceHit{URL: u.Loc, PID: pid})
	}
	return out
}

// pidFromURL extracts the trailing numeric PID from a craigslist listing URL
// like "https://sfbay.craigslist.org/sfc/apa/d/some-slug/7915891289.html".
// Returns 0 when no PID is found.
func pidFromURL(u string) int64 {
	if u == "" {
		return 0
	}
	end := strings.LastIndex(u, ".html")
	if end < 0 {
		end = len(u)
	}
	start := strings.LastIndex(u[:end], "/")
	if start < 0 {
		return 0
	}
	tok := u[start+1 : end]
	pid, err := strconv.ParseInt(tok, 10, 64)
	if err != nil {
		return 0
	}
	return pid
}

func existsListing(ctx context.Context, db *sql.DB, pid int64) bool {
	var n int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM listings WHERE pid = ?`, pid).Scan(&n); err != nil {
		return false
	}
	return n > 0
}
