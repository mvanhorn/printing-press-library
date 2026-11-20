// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type mktMove struct {
	Term         string `json:"term"`
	ListingID    int64  `json:"listing_id"`
	Title        string `json:"title"`
	FromPosition int    `json:"from_position"`
	ToPosition   int    `json:"to_position"`
	Delta        int    `json:"delta"` // negative = moved up the results
}

type mktPriceChange struct {
	ListingID int64   `json:"listing_id"`
	Title     string  `json:"title"`
	From      float64 `json:"from"`
	To        float64 `json:"to"`
	Currency  string  `json:"currency,omitempty"`
}

type mktFavDelta struct {
	ListingID int64  `json:"listing_id"`
	Title     string `json:"title"`
	From      int64  `json:"from"`
	To        int64  `json:"to"`
	Delta     int64  `json:"delta"`
}

type mktListingRef struct {
	ListingID int64  `json:"listing_id"`
	ShopID    *int64 `json:"shop_id,omitempty"`
	Title     string `json:"title"`
	Term      string `json:"term"`
	Position  int    `json:"position"`
}

func newNovelMarketTrendCmd(flags *rootFlags) *cobra.Command {
	var flagFrom, flagTo int64

	cmd := &cobra.Command{
		Use:   "trend",
		Short: "Compare two snapshots to see which listings and shops entered or left the results, and how positions, prices and favorites moved.",
		Long: `Compare two snapshots to see which listings and shops entered or left the results, and how positions, prices and favorites moved.

Defaults to the latest two complete snapshots; pick others with --from ID --to ID. Only search terms
present in both snapshots are compared, and the output names any terms that were skipped.
With fewer than two snapshots the result is an empty list plus a note.
--csv and --plain are refused for this nested report; use --json or --agent.`,
		Example:     "  etsy-pp-cli market trend\n  etsy-pp-cli market trend --from 1 --to 3 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			explicit := cmd.Flags().Changed("from")
			if explicit != cmd.Flags().Changed("to") {
				return usageErr(fmt.Errorf("--from and --to must be given together"))
			}
			if explicit && (flagFrom <= 0 || flagTo <= 0) {
				return usageErr(fmt.Errorf("--from/--to must be positive snapshot IDs (omit both for the latest two)"))
			}
			if flags.csv || flags.plain {
				return usageErr(fmt.Errorf("market trend output is nested; use --json"))
			}
			if dryRunOK(flags) {
				return mktDryRun(cmd, flags, "market trend")
			}
			empty := func(note string) error {
				return mktEmitReport(cmd, flags, map[string]any{"results": make([]any, 0), "note": note})
			}
			db, err := mktOpenRead(cmd.Context())
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			if db == nil {
				if explicit {
					return notFoundErr(fmt.Errorf("snapshot %d or %d not found", flagFrom, flagTo))
				}
				return empty(mktNoSnapshotHint)
			}
			defer db.Close()

			var from, to *mktSnapshot
			if explicit {
				if from, err = mktGetSnapshot(db.DB(), flagFrom); err != nil {
					return err
				}
				if to, err = mktGetSnapshot(db.DB(), flagTo); err != nil {
					return err
				}
				if from == nil || to == nil {
					return notFoundErr(fmt.Errorf("snapshot %d or %d not found", flagFrom, flagTo))
				}
			} else {
				snaps, err := mktCompleteSnapshots(db.DB(), 2)
				if err != nil {
					return err
				}
				if len(snaps) < 2 {
					var count int
					if err := db.DB().QueryRow(`SELECT count(*) FROM market_snapshots`).Scan(&count); err != nil {
						return err
					}
					if count == 0 {
						return empty(mktNoSnapshotHint)
					}
					var partialID int64
					err := db.DB().QueryRow(`SELECT id FROM market_snapshots WHERE status != 'complete' ORDER BY id DESC LIMIT 1`).Scan(&partialID)
					if err != nil && err != sql.ErrNoRows {
						return err
					}
					hint := `take another snapshot with etsy-pp-cli market snapshot --term "SEARCH TERM"`
					if err == nil {
						hint = fmt.Sprintf("resume the partial snapshot with etsy-pp-cli market snapshot --resume %d", partialID)
					}
					return empty(fmt.Sprintf("need at least two complete snapshots to compare; found %d; %s", len(snaps), hint))
				}
				to, from = snaps[0], snaps[1]
			}

			common, onlyFrom, onlyTo := mktSplitTerms(from.Terms, to.Terms)
			if len(common) == 0 {
				return empty("the two snapshots share no search terms, so nothing is comparable")
			}
			fr, err := mktLoadResults(db.DB(), from.ID)
			if err != nil {
				return err
			}
			tr, err := mktLoadResults(db.DB(), to.ID)
			if err != nil {
				return err
			}
			out := mktCompareSnapshots(from, to, fr, tr, common, onlyFrom, onlyTo)
			payload := map[string]any{}
			for k, v := range out {
				if k != "from_snapshot" && k != "to_snapshot" && k != "note" {
					payload[k] = v
					delete(out, k)
				}
			}
			out["results"] = payload
			return mktEmitReport(cmd, flags, out)
		},
	}
	cmd.Flags().Int64Var(&flagFrom, "from", 0, "Positive snapshot ID; omit both for the latest two complete snapshots")
	mktHideZeroDefault(cmd, "from")
	cmd.Flags().Int64Var(&flagTo, "to", 0, "Positive snapshot ID; omit both for the latest two complete snapshots")
	mktHideZeroDefault(cmd, "to")
	return cmd
}

func mktSplitTerms(a, b []string) (common, onlyA, onlyB []string) {
	inA, inB := map[string]bool{}, map[string]bool{}
	for _, t := range a {
		inA[t] = true
	}
	for _, t := range b {
		inB[t] = true
	}
	common, onlyA, onlyB = make([]string, 0), make([]string, 0), make([]string, 0)
	for _, t := range a {
		if inB[t] {
			common = append(common, t)
		} else {
			onlyA = append(onlyA, t)
		}
	}
	for _, t := range b {
		if !inA[t] {
			onlyB = append(onlyB, t)
		}
	}
	return
}

func mktCompareSnapshots(from, to *mktSnapshot, fr, tr []mktRow, common, onlyFrom, onlyTo []string) map[string]any {
	isCommon := map[string]bool{}
	for _, t := range common {
		isCommon[t] = true
	}
	type key struct {
		term string
		id   int64
	}
	fromBy, toBy := map[key]mktRow{}, map[key]mktRow{}
	fromL, toL := map[int64]mktRow{}, map[int64]mktRow{}
	fromS, toS := map[int64]bool{}, map[int64]bool{}
	for _, r := range fr {
		if !isCommon[r.Term] {
			continue
		}
		fromBy[key{r.Term, r.ListingID}] = r
		if cur, ok := fromL[r.ListingID]; !ok || r.Position < cur.Position {
			fromL[r.ListingID] = r
		}
		if r.ShopID.Valid {
			fromS[r.ShopID.Int64] = true
		}
	}
	for _, r := range tr {
		if !isCommon[r.Term] {
			continue
		}
		toBy[key{r.Term, r.ListingID}] = r
		if cur, ok := toL[r.ListingID]; !ok || r.Position < cur.Position {
			toL[r.ListingID] = r
		}
		if r.ShopID.Valid {
			toS[r.ShopID.Int64] = true
		}
	}
	ref := func(r mktRow) mktListingRef {
		x := mktListingRef{ListingID: r.ListingID, Title: r.Title, Term: r.Term, Position: r.Position}
		if r.ShopID.Valid {
			v := r.ShopID.Int64
			x.ShopID = &v
		}
		return x
	}
	newL, exitL := make([]mktListingRef, 0), make([]mktListingRef, 0)
	for id, r := range toL {
		if _, ok := fromL[id]; !ok {
			newL = append(newL, ref(r))
		}
	}
	for id, r := range fromL {
		if _, ok := toL[id]; !ok {
			exitL = append(exitL, ref(r))
		}
	}
	byID := func(s []mktListingRef) {
		sort.Slice(s, func(i, j int) bool { return s[i].ListingID < s[j].ListingID })
	}
	byID(newL)
	byID(exitL)
	newS, exitS := make([]int64, 0), make([]int64, 0)
	for id := range toS {
		if !fromS[id] {
			newS = append(newS, id)
		}
	}
	for id := range fromS {
		if !toS[id] {
			exitS = append(exitS, id)
		}
	}
	sort.Slice(newS, func(i, j int) bool { return newS[i] < newS[j] })
	sort.Slice(exitS, func(i, j int) bool { return exitS[i] < exitS[j] })

	moves := make([]mktMove, 0)
	for k, a := range fromBy {
		b, ok := toBy[k]
		if !ok || a.Position == b.Position {
			continue
		}
		moves = append(moves, mktMove{Term: k.term, ListingID: k.id, Title: b.Title, FromPosition: a.Position, ToPosition: b.Position, Delta: b.Position - a.Position})
	}
	sort.Slice(moves, func(i, j int) bool {
		ai, aj := moves[i].Delta, moves[j].Delta
		if ai < 0 {
			ai = -ai
		}
		if aj < 0 {
			aj = -aj
		}
		if ai != aj {
			return ai > aj
		}
		if moves[i].Term != moves[j].Term {
			return moves[i].Term < moves[j].Term
		}
		return moves[i].ListingID < moves[j].ListingID
	})
	prices, favs := make([]mktPriceChange, 0), make([]mktFavDelta, 0)
	for id, a := range fromL {
		b, ok := toL[id]
		if !ok {
			continue
		}
		if a.Price.Valid && b.Price.Valid && a.Price.Float64 != b.Price.Float64 {
			prices = append(prices, mktPriceChange{ListingID: id, Title: b.Title, From: a.Price.Float64, To: b.Price.Float64, Currency: b.Currency})
		}
		// Favorites compare only when both snapshots recorded a value.
		if a.Favorers.Valid && b.Favorers.Valid && a.Favorers.Int64 != b.Favorers.Int64 {
			favs = append(favs, mktFavDelta{ListingID: id, Title: b.Title, From: a.Favorers.Int64, To: b.Favorers.Int64, Delta: b.Favorers.Int64 - a.Favorers.Int64})
		}
	}
	sort.Slice(prices, func(i, j int) bool { return prices[i].ListingID < prices[j].ListingID })
	sort.Slice(favs, func(i, j int) bool { return favs[i].ListingID < favs[j].ListingID })

	note := fmt.Sprintf("compared %d term(s) present in both snapshots", len(common))
	if len(onlyFrom)+len(onlyTo) > 0 {
		sides := make([]string, 0, 2)
		if len(onlyFrom) > 0 {
			sides = append(sides, fmt.Sprintf("only in snapshot %d: %s", from.ID, mktQuoteJoin(onlyFrom)))
		}
		if len(onlyTo) > 0 {
			sides = append(sides, fmt.Sprintf("only in snapshot %d: %s", to.ID, mktQuoteJoin(onlyTo)))
		}
		note += "; skipped terms " + strings.Join(sides, "; ")
	}
	return map[string]any{
		"from_snapshot":    from.ID,
		"to_snapshot":      to.ID,
		"common_terms":     common,
		"skipped_terms":    map[string]any{"only_in_from": onlyFrom, "only_in_to": onlyTo},
		"new_listings":     newL,
		"exited_listings":  exitL,
		"new_shops":        newS,
		"exited_shops":     exitS,
		"position_moves":   moves,
		"price_changes":    prices,
		"favorites_deltas": favs,
		"note":             note,
	}
}
