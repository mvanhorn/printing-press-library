// `scam-score` is a rule-based 0-100 score for a listing using fresh-listing,
// below-median, payment-keyword, ship-only, dupe-cluster, and external-URL
// signals. Pure-logic — no LLM, no network. Inputs come from the local store
// only. Each rule contributes a fixed point value; we cap at 100.

package cli

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/craigslist/internal/store"

	"github.com/spf13/cobra"
)

// scoreContribution is one rule firing.
type scoreContribution struct {
	Rule   string `json:"rule"`
	Points int    `json:"points"`
	Reason string `json:"reason,omitempty"`
}

// scamScoreResult is the typed shape returned by `scam-score`.
type scamScoreResult struct {
	PID           int64               `json:"pid"`
	Score         int                 `json:"score"`
	Contributions []scoreContribution `json:"contributions"`
}

func newScamScoreCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "scam-score [pid]",
		Short:       "0-100 rule-based scam score for a stored listing",
		Long:        "Score a listing using fresh-listing, below-median, payment-keyword, ship-only, dupe-cluster, and external-URL rules. Pure local SQL aggregation.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			pid, err := strconv.ParseInt(strings.TrimSpace(args[0]), 10, 64)
			if err != nil {
				return fmt.Errorf("invalid pid %q: %w", args[0], err)
			}
			ctx := cmd.Context()
			db, err := openCLStore(ctx)
			if err != nil {
				return err
			}
			defer db.Close()
			res, err := computeScamScoreFromStore(ctx, db, pid)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), res, flags)
		},
	}
	return cmd
}

// computeScamScoreFromStore is the wired-up entry: load the listing + median
// + cluster size from the store and run the pure scorer.
func computeScamScoreFromStore(ctx context.Context, db *store.Store, pid int64) (scamScoreResult, error) {
	row, err := db.GetListing(ctx, pid)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return scamScoreResult{}, fmt.Errorf("listing %d not in local store; run `craigslist-pp-cli cl-sync --site <hostname> --category <abbr>` first", pid)
		}
		return scamScoreResult{}, fmt.Errorf("get listing: %w", err)
	}
	if row == nil {
		return scamScoreResult{}, fmt.Errorf("listing %d not in local store; run `craigslist-pp-cli cl-sync --site <hostname> --category <abbr>` first", pid)
	}
	median := categoryMedianPrice(ctx, db.DB(), row.CategoryAbbr)

	rows, _ := loadListingsForCluster(ctx, db.DB(), "", row.CategoryAbbr)
	clusters := buildDupeClusters(rows, 2)
	clusterSize := 0
	for _, c := range clusters {
		for _, m := range c.Members {
			if m == pid {
				clusterSize = c.Size
				break
			}
		}
		if clusterSize > 0 {
			break
		}
	}

	input := scamScoreInput{
		PostedAt:    row.PostedAt,
		Now:         time.Now().Unix(),
		Price:       row.Price,
		Median:      median,
		BodyText:    row.BodyText,
		ClusterSize: clusterSize,
	}
	res := computeScamScore(input)
	res.PID = pid
	return res, nil
}

// scamScoreInput collects every signal computeScamScore needs in one struct
// so tests can drive the scorer without touching the store.
type scamScoreInput struct {
	PostedAt    int64
	Now         int64
	Price       int
	Median      int
	BodyText    string
	ClusterSize int
}

var (
	wireRE       = regexp.MustCompile(`(?i)\b(wire|zelle|cashapp|moneygram|western\s*union|bitcoin|btc)\b`)
	shipRE       = regexp.MustCompile(`(?i)\b(ship|shipping)\b`)
	excuseRE     = regexp.MustCompile(`(?i)\b(no\s*viewing|no\s*meet|out\s*of\s*town|relocating|abroad|deployed)\b`)
	urlRE        = regexp.MustCompile(`https?://[^\s)]+`)
	craigslistRE = regexp.MustCompile(`(?i)craigslist\.org`)
)

// computeScamScore is the pure rule scorer. Each rule contributes a fixed
// point value; we cap the final score at 100.
func computeScamScore(in scamScoreInput) scamScoreResult {
	res := scamScoreResult{}
	if in.PostedAt > 0 && in.Now > 0 && in.Now-in.PostedAt < 24*3600 && in.Median > 0 && in.Price > 0 && in.Price < in.Median/2 {
		res.Contributions = append(res.Contributions, scoreContribution{
			Rule: "fresh_below_median", Points: 30,
			Reason: fmt.Sprintf("posted <24h ago, price $%d < 50%% of median $%d", in.Price, in.Median),
		})
	}
	if wireRE.MatchString(in.BodyText) {
		res.Contributions = append(res.Contributions, scoreContribution{
			Rule: "payment_keyword", Points: 25,
			Reason: "wire/zelle/cashapp/etc. mentioned in body",
		})
	}
	if shipRE.MatchString(in.BodyText) && excuseRE.MatchString(in.BodyText) {
		res.Contributions = append(res.Contributions, scoreContribution{
			Rule: "ship_only_excuse", Points: 20,
			Reason: "ships only + out-of-town/relocating/etc. excuse",
		})
	}
	if in.ClusterSize >= 3 {
		res.Contributions = append(res.Contributions, scoreContribution{
			Rule: "dupe_cluster", Points: 15,
			Reason: fmt.Sprintf("listing in dupe cluster of size %d", in.ClusterSize),
		})
	}
	if hasExternalURL(in.BodyText) {
		res.Contributions = append(res.Contributions, scoreContribution{
			Rule: "external_url", Points: 10,
			Reason: "body links to a non-craigslist host",
		})
	}
	for _, c := range res.Contributions {
		res.Score += c.Points
	}
	if res.Score > 100 {
		res.Score = 100
	}
	return res
}

// hasExternalURL returns true if the text contains a URL whose host is not
// craigslist.org or images.craigslist.org. Bare-text matches like "go to
// example.com" without a scheme don't count — too noisy.
func hasExternalURL(s string) bool {
	for _, m := range urlRE.FindAllString(s, -1) {
		u, err := url.Parse(m)
		if err != nil {
			continue
		}
		host := strings.ToLower(u.Host)
		if host == "" || craigslistRE.MatchString(host) {
			continue
		}
		return true
	}
	return false
}

// categoryMedianPrice computes the median (p50) of all positive prices in the
// store for the given category abbreviation. Returns 0 when the store is
// empty or every row has price <= 0; the scorer then skips the
// fresh-below-median rule rather than firing on noise.
func categoryMedianPrice(ctx context.Context, db *sql.DB, category string) int {
	rows, err := db.QueryContext(ctx, `SELECT price FROM listings WHERE price > 0 AND category_abbr = ?`, category)
	if err != nil {
		return 0
	}
	defer rows.Close()
	var prices []int
	for rows.Next() {
		var p int
		if err := rows.Scan(&p); err == nil {
			prices = append(prices, p)
		}
	}
	if len(prices) == 0 {
		return 0
	}
	sort.Ints(prices)
	return prices[len(prices)/2]
}
