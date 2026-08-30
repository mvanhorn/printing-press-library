// Copyright 2026 Richard Gill and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/woolworths/internal/pricehist"
	"github.com/mvanhorn/printing-press-library/library/commerce/woolworths/internal/store"
	"github.com/spf13/cobra"
)

// The tile wire types, the search/browse endpoint paths and the page-size cap
// live once in swap.go as the wow* family. real-special, cycle and
// specials-diff decode the SAME tiles as swap, multibuy and basket, and a
// second parallel family had already drifted (it lacked IsInStock, so
// availability was recorded on a different rule from the one the other
// commands filter on).

// ppwDBPath resolves the --db override against the canonical mirror location.
func ppwDBPath(override string) string {
	if strings.TrimSpace(override) != "" {
		return strings.TrimSpace(override)
	}
	return defaultDBPath("woolworths-pp-cli")
}

// ppwPoints converts store observations into the pure-analysis shape.
//
// Non-positive prices are dropped, the same filter basket.go and swap.go apply
// to the same table. An observation recorded while a product was unavailable
// carries price = 0, and letting one through would drag Min to $0.00 and the
// median down with it, after which every genuine half-price week on that
// product is misreported as ORDINARY.
func ppwPoints(obs []store.PriceObservation) []pricehist.Point {
	out := make([]pricehist.Point, 0, len(obs))
	for _, o := range obs {
		if o.Price <= 0 {
			continue
		}
		out = append(out, pricehist.Point{
			ObservedAt:  o.ObservedAt,
			Price:       o.Price,
			WasPrice:    o.WasPrice,
			IsOnSpecial: o.IsOnSpecial,
			IsHalfPrice: o.IsHalfPrice,
		})
	}
	return out
}

// ppwUnixString renders a unix timestamp as RFC3339, or "" for a zero/absent time.
func ppwUnixString(ts int64) string {
	if ts <= 0 {
		return ""
	}
	return time.Unix(ts, 0).UTC().Format(time.RFC3339)
}

// ppwTruncate shortens s to at most n RUNES, appending an ellipsis. It counts
// runes rather than bytes: product names carry accented characters and slicing
// bytes cuts a multi-byte rune in half, emitting invalid UTF-8 into the table.
func ppwTruncate(s string, n int) string {
	if n <= 1 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n-1]) + "..."
}

// realSpecialRow is one product's verdict. Every number the verdict rests on is
// reported alongside it, so a caller can check the arithmetic rather than take
// the label on faith.
type realSpecialRow struct {
	Stockcode        string  `json:"stockcode"`
	Name             string  `json:"name,omitempty"`
	Brand            string  `json:"brand,omitempty"`
	PackageSize      string  `json:"package_size,omitempty"`
	CurrentPrice     float64 `json:"current_price"`
	WasPrice         float64 `json:"was_price"`
	AdvertisedSaving float64 `json:"advertised_saving"`
	IsOnSpecial      bool    `json:"is_on_special"`
	IsHalfPrice      bool    `json:"is_half_price"`
	Observations     int     `json:"observations"`
	LocalMin         float64 `json:"local_min"`
	LocalMedian      float64 `json:"local_median"`
	LocalMax         float64 `json:"local_max"`
	DaysOfHistory    float64 `json:"days_of_history"`
	FirstObservedAt  string  `json:"first_observed_at,omitempty"`
	LastObservedAt   string  `json:"last_observed_at,omitempty"`
	Verdict          string  `json:"verdict"`
	Reason           string  `json:"reason"`
	PriceSource      string  `json:"price_source"`
}

func newNovelRealSpecialCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "real-special <term|stockcode>",
		Short: "Tells you whether an advertised special is genuinely cheap or just a lower number next to an inflated was-price.",
		Long: strings.Trim(`
Fetches the live tile(s) for a term or stockcode, records what it sees into the
local price_observation mirror, then joins the live price against everything
this machine has recorded before to emit ONE verdict per product:

  NO-HISTORY          fewer than 3 prior local observations - no verdict is honest yet
  GENUINE             at/near the local minimum and materially below the local median
  RECYCLED            this exact price recurs; the "special" is the normal cycle low
  WAS-PRICE-INFLATED  the advertised was-price sits above what the product actually sold at
  ORDINARY            enough history to judge, and today's price is unremarkable

Woolworths publishes no price history of any kind, so the local mirror is the
only evidence there is. A thin mirror yields NO-HISTORY, never a guess.

With --data-source local the command skips the network entirely and judges the
most recent recorded observation against everything recorded before it.
`, "\n"),
		Example: strings.Trim(`
  woolworths-pp-cli real-special 'tim tam' --agent
  woolworths-pp-cli real-special 36066 --limit 1
  woolworths-pp-cli real-special 'olive oil' --data-source local --json
`, "\n"),
		Annotations: map[string]string{
			// Empty is a valid answer, not an error: a nonsense term is indistinguishable from a valid term with no matches; Woolworths answers 200 either way.
			// Inventing an "empty means not found" rule would break honest-empty output.
			"pp:no-error-path-probe": "true",
			"mcp:read-only":          "true",
			"pp:data-source":         "auto",
			"pp:happy-args":          "tim tam;--limit=3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "real-special")
			}
			query := ""
			if len(args) > 0 {
				query = strings.TrimSpace(args[0])
			}
			if query == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("<term|stockcode> is required"))
			}
			if limit <= 0 {
				limit = 5
			}
			resolvedDB := ppwDBPath(dbPath)
			localOnly := flags != nil && flags.dataSource == "local"

			// Missing-mirror guard. Only a local-only run bails here: a live
			// run is precisely how the mirror gets created in the first place,
			// so refusing to fetch because the file does not exist yet would
			// make the command impossible to bootstrap.
			if localOnly {
				if _, statErr := os.Stat(resolvedDB); os.IsNotExist(statErr) {
					fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: woolworths-pp-cli real-special <term> --db %s without --data-source local to record the first observations\n", resolvedDB, resolvedDB)
					if !wantsHumanTable(cmd.OutOrStdout(), flags) {
						return printJSONFiltered(cmd.OutOrStdout(), make([]realSpecialRow, 0), flags)
					}
					return nil
				}
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			db, err := store.OpenWithContext(ctx, resolvedDB)
			if err != nil {
				return fmt.Errorf("opening local mirror %s: %w", resolvedDB, err)
			}
			defer db.Close()
			if err := store.EnsureWoolworthsTables(ctx, db.DB()); err != nil {
				return err
			}

			var (
				rows  []realSpecialRow
				notes []string
			)
			if localOnly {
				rows, notes, err = realSpecialFromLocal(ctx, db.DB(), query, limit)
			} else {
				rows, notes, err = realSpecialFromLive(ctx, cmd, flags, db.DB(), query, limit)
			}
			if err != nil {
				return err
			}

			// Output mode is settled before any prose: a machine caller must
			// get [] rather than an English sentence on an empty result.
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				for _, n := range notes {
					fmt.Fprintln(cmd.ErrOrStderr(), n)
				}
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}

			if len(rows) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "no products matched %q - no verdict issued\n", query)
				for _, n := range notes {
					fmt.Fprintln(cmd.OutOrStdout(), n)
				}
				return nil
			}
			headers := []string{"STOCKCODE", "PRODUCT", "NOW", "WAS", "LOCAL MIN", "LOCAL MED", "OBS", "DAYS", "VERDICT"}
			table := make([][]string, 0, len(rows))
			for _, r := range rows {
				table = append(table, []string{
					r.Stockcode,
					ppwTruncate(r.Name, 38),
					fmt.Sprintf("$%.2f", r.CurrentPrice),
					fmt.Sprintf("$%.2f", r.WasPrice),
					fmt.Sprintf("$%.2f", r.LocalMin),
					fmt.Sprintf("$%.2f", r.LocalMedian),
					fmt.Sprintf("%d", r.Observations),
					fmt.Sprintf("%.0f", r.DaysOfHistory),
					r.Verdict,
				})
			}
			if err := flags.printTable(cmd, headers, table); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout())
			for _, r := range rows {
				fmt.Fprintf(cmd.OutOrStdout(), "%s  %s: %s\n", r.Stockcode, r.Verdict, r.Reason)
			}
			for _, n := range notes {
				fmt.Fprintln(cmd.OutOrStdout(), n)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 5, "Maximum number of products to judge")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to the local SQLite mirror (default: the CLI data directory)")
	return cmd
}

// realSpecialFromLive fetches tiles, records them, and judges each against the
// history that existed before this run.
func realSpecialFromLive(ctx context.Context, cmd *cobra.Command, flags *rootFlags, db *sql.DB, query string, limit int) ([]realSpecialRow, []string, error) {
	rows := make([]realSpecialRow, 0, limit)
	notes := make([]string, 0, 2)

	c, err := flags.newClient()
	if err != nil {
		return rows, notes, err
	}
	// Akamai drops unwarmed POST connections; one GET of /shop primes the jar.
	c.WarmSession(ctx)
	// One page is fetched, so the server's hard PageSize cap bounds how many
	// products this run can possibly judge. limit is clamped to the same cap
	// and the narrowing is reported: silently returning 36 rows for --limit 100
	// would read as "that is all there is".
	requestedLimit := limit
	if limit > wowMaxPageSize {
		limit = wowMaxPageSize
		notes = append(notes, fmt.Sprintf("note: --limit %d was narrowed to %d; the search endpoint caps one page at %d products and this command judges a single page",
			requestedLimit, wowMaxPageSize, wowMaxPageSize))
	}
	pageSize := limit * 2
	if pageSize > wowMaxPageSize {
		pageSize = wowMaxPageSize
	}
	if pageSize < 1 {
		pageSize = 1
	}
	body := map[string]any{
		"SearchTerm": query,
		"PageNumber": 1,
		"PageSize":   pageSize,
		"SortType":   "TraderRelevance",
		"IsSpecial":  false,
	}
	data, _, err := c.PostQueryWithParams(ctx, wowSearchPath, map[string]string{}, body)
	if err != nil {
		return rows, notes, classifyAPIError(cmd.OutOrStdout(), err, flags)
	}
	var parsed wowSearchResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		return rows, notes, fmt.Errorf("parsing search response: %w", err)
	}

	// Relevance gate. Woolworths returns loosely related merchandise for a
	// nonsense query; issuing verdicts on those would have the CLI asserting
	// things about products the caller never asked about.
	wantStockcode := pricehist.IsStockcode(query)
	kept := make([]wowTile, 0, limit)
	dropped := 0
	for _, tile := range parsed.tiles() {
		if tile.stockcode() == "" {
			continue
		}
		if wantStockcode {
			if tile.stockcode() != strings.TrimSpace(query) {
				dropped++
				continue
			}
		} else if !pricehist.TermRelevant(query, tile.Name, tile.Brand) {
			dropped++
			continue
		}
		kept = append(kept, tile)
		if len(kept) >= limit {
			break
		}
	}
	if dropped > 0 {
		notes = append(notes, fmt.Sprintf("note: dropped %d live result(s) that did not match %q - no verdict is issued for products you did not ask about", dropped, query))
	}
	if len(kept) == 0 {
		notes = append(notes, fmt.Sprintf("note: Woolworths returned %d result(s) for %q, none of which matched the term", parsed.SearchResultsCount, query))
		return rows, notes, nil
	}

	observedAt := time.Now().Unix()
	obs := make([]store.PriceObservation, 0, len(kept))
	for _, tile := range kept {
		obs = append(obs, tile.observation(observedAt))
	}
	if err := store.RecordPriceObservations(ctx, db, obs); err != nil {
		return rows, notes, err
	}

	for _, tile := range kept {
		history, err := store.PriceHistory(ctx, db, tile.stockcode())
		if err != nil {
			return rows, notes, err
		}
		rows = append(rows, buildRealSpecialRow(tile.stockcode(), tile.Name, tile.Brand, tile.PackageSize,
			tile.Price, tile.WasPrice, tile.SavingsAmount, tile.IsOnSpecial, tile.IsHalfPrice,
			history, observedAt, "live"))
	}
	return rows, notes, nil
}

// realSpecialFromLocal judges the most recent recorded observation of each
// matching product against everything recorded before it. No network.
func realSpecialFromLocal(ctx context.Context, db *sql.DB, query string, limit int) ([]realSpecialRow, []string, error) {
	rows := make([]realSpecialRow, 0, limit)
	notes := make([]string, 0, 1)

	codes, err := ppwLookupStockcodes(ctx, db, query, limit)
	if err != nil {
		return rows, notes, err
	}
	if len(codes) == 0 {
		notes = append(notes, fmt.Sprintf("note: nothing recorded locally for %q; run this command without --data-source local to fetch and record it", query))
		return rows, notes, nil
	}
	for _, code := range codes {
		history, err := store.PriceHistory(ctx, db, code)
		if err != nil {
			return rows, notes, err
		}
		if len(history) == 0 {
			continue
		}
		latest := history[len(history)-1]
		rows = append(rows, buildRealSpecialRow(latest.Stockcode, latest.Name, latest.Brand, latest.PackageSize,
			latest.Price, latest.WasPrice, latest.Savings, latest.IsOnSpecial, latest.IsHalfPrice,
			history, latest.ObservedAt, "local"))
	}
	notes = append(notes, "note: --data-source local judged the most recent recorded observation, which may be stale")
	return rows, notes, nil
}

// buildRealSpecialRow assembles one verdict row. currentAt separates "the
// observation being judged" from "the evidence": only observations strictly
// older than currentAt count as prior history, so the current price never
// votes on its own typicality.
func buildRealSpecialRow(stockcode, name, brand, packageSize string, price, wasPrice, savings float64,
	onSpecial, halfPrice bool, history []store.PriceObservation, currentAt int64, source string) realSpecialRow {

	points := ppwPoints(history)
	prior := make([]pricehist.Point, 0, len(points))
	for _, p := range points {
		if p.ObservedAt < currentAt {
			prior = append(prior, p)
		}
	}
	full := pricehist.Summarize(points)
	priorSummary := pricehist.Summarize(prior)
	verdict := pricehist.Classify(pricehist.ClassifyInput{
		CurrentPrice: price,
		WasPrice:     wasPrice,
		Prior:        prior,
	})
	return realSpecialRow{
		Stockcode:        stockcode,
		Name:             name,
		Brand:            brand,
		PackageSize:      packageSize,
		CurrentPrice:     pricehist.Round2(price),
		WasPrice:         pricehist.Round2(wasPrice),
		AdvertisedSaving: pricehist.Round2(savings),
		IsOnSpecial:      onSpecial,
		IsHalfPrice:      halfPrice,
		Observations:     priorSummary.Count,
		LocalMin:         priorSummary.Min,
		LocalMedian:      priorSummary.Median,
		LocalMax:         priorSummary.Max,
		DaysOfHistory:    full.DaysOfHistory,
		FirstObservedAt:  ppwUnixString(full.FirstAt),
		LastObservedAt:   ppwUnixString(full.LastAt),
		Verdict:          verdict.Label,
		Reason:           verdict.Reason,
		PriceSource:      source,
	}
}

// ppwLookupStockcodes finds locally recorded stockcodes matching a stockcode or
// a name/brand fragment, most recently observed first.
//
// Drain-first: the whole result set is scanned and closed before the caller
// issues the per-stockcode history queries that follow. NULL-safe: name and
// brand are nullable columns, so they go through COALESCE, and the scanned
// values land in sql.Null* targets.
func ppwLookupStockcodes(ctx context.Context, db *sql.DB, query string, limit int) ([]string, error) {
	if db == nil {
		return nil, fmt.Errorf("lookup stockcodes: nil database handle")
	}
	if limit <= 0 {
		limit = 5
	}
	trimmed := strings.TrimSpace(query)
	like := "%" + strings.ToLower(trimmed) + "%"
	rows, err := db.QueryContext(ctx, `
		SELECT stockcode, MAX(observed_at) AS last_at
		FROM price_observation
		WHERE stockcode = ?
		   OR LOWER(COALESCE(name, '')) LIKE ?
		   OR LOWER(COALESCE(brand, '')) LIKE ?
		GROUP BY stockcode
		ORDER BY last_at DESC
		LIMIT ?`, trimmed, like, like, limit)
	if err != nil {
		return nil, fmt.Errorf("lookup stockcodes: query: %w", err)
	}
	out := make([]string, 0, limit)
	for rows.Next() {
		var (
			code   sql.NullString
			lastAt sql.NullInt64
		)
		if err := rows.Scan(&code, &lastAt); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("lookup stockcodes: scan: %w", err)
		}
		if code.Valid && code.String != "" {
			out = append(out, code.String)
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("lookup stockcodes: iterate: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("lookup stockcodes: close: %w", err)
	}
	return out, nil
}
