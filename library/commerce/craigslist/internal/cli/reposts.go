// `reposts <query>` finds listings whose body has been republished at least
// N times across distinct calendar days within the window. Reads from
// listing_snapshots so the cadence of reposts is captured accurately.

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// repostHit is the typed shape per cluster of reposted bodies.
type repostHit struct {
	BodyHash     string  `json:"bodyHash"`
	PostCount    int     `json:"postCount"`
	DistinctDays int     `json:"distinctDays"`
	MemberPids   []int64 `json:"memberPids"`
	SampleTitle  string  `json:"sampleTitle"`
}

func newRepostsCmd(flags *rootFlags) *cobra.Command {
	var minReposts int
	var window string
	cmd := &cobra.Command{
		Use:         "reposts [query]",
		Short:       "Listings whose body has been reposted N+ times in a window",
		Long:        "Group listing_snapshots by body_hash and surface clusters where the same body has appeared on multiple calendar days within the window.",
		Example:     "  craigslist-pp-cli reposts \"eames lounge\" --min-reposts 3 --window 30d --json\n  craigslist-pp-cli reposts \"1BR\" --min-reposts 2 --window 7d",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			query := strings.Join(args, " ")
			d, err := parseDuration(window)
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			db, err := openCLStore(ctx)
			if err != nil {
				return err
			}
			defer db.Close()
			hits, err := findReposts(ctx, db.DB(), query, minReposts, d)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), hits, flags)
		},
	}
	cmd.Flags().IntVar(&minReposts, "min-reposts", 3, "Minimum number of distinct calendar days to flag")
	cmd.Flags().StringVar(&window, "window", "30d", "Window size (e.g. 30d, 7d)")
	return cmd
}

func findReposts(ctx context.Context, db *sql.DB, query string, minReposts int, window time.Duration) ([]repostHit, error) {
	if minReposts < 2 {
		minReposts = 2
	}
	q := `SELECT s.pid, s.observed_at, s.body_hash, s.title FROM listing_snapshots s`
	args := []any{}
	conds := []string{"s.body_hash != ''"}
	if fts := quoteFTS(query); fts != "" {
		// SQLite FTS5 requires the unaliased table name in MATCH; an alias
		// like "f" is interpreted as a missing column. Wrap the query as a
		// phrase via quoteFTS so embedded apostrophes (e.g. "men's") and
		// other reserved characters don't blow up the parser.
		q += ` JOIN listings_fts ON listings_fts.rowid = s.pid`
		conds = append(conds, "listings_fts MATCH ?")
		args = append(args, fts)
	}
	if window > 0 {
		conds = append(conds, "s.observed_at >= ?")
		args = append(args, time.Now().Add(-window).Unix())
	}
	if len(conds) > 0 {
		q += " WHERE " + strings.Join(conds, " AND ")
	}
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("repost query: %w", err)
	}
	defer rows.Close()
	type bucket struct {
		hash    string
		title   string
		pidSet  map[int64]bool
		daySet  map[string]bool
		sampleT string
	}
	groups := map[string]*bucket{}
	for rows.Next() {
		var pid int64
		var obs int64
		var hash, title string
		if err := rows.Scan(&pid, &obs, &hash, &title); err != nil {
			return nil, err
		}
		b, ok := groups[hash]
		if !ok {
			b = &bucket{hash: hash, pidSet: map[int64]bool{}, daySet: map[string]bool{}, sampleT: title}
			groups[hash] = b
		}
		b.pidSet[pid] = true
		b.daySet[time.Unix(obs, 0).UTC().Format("2006-01-02")] = true
	}
	var out []repostHit
	for _, b := range groups {
		if len(b.daySet) < minReposts {
			continue
		}
		pids := make([]int64, 0, len(b.pidSet))
		for p := range b.pidSet {
			pids = append(pids, p)
		}
		out = append(out, repostHit{
			BodyHash:     b.hash,
			PostCount:    len(b.pidSet),
			DistinctDays: len(b.daySet),
			MemberPids:   pids,
			SampleTitle:  b.sampleT,
		})
	}
	return out, rows.Err()
}
