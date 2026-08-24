// Copyright 2026 Richard Gill and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command. The RunE body below is implemented; do not
// replace it with a scaffold.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/woolworths/internal/unitprice"
	"github.com/spf13/cobra"
)

// multibuyBetterPack is the cheapest single pack, larger than the offer's own
// pack and of the same measure kind, found in the same result set. It is the
// benchmark a "2 for $10" tag never shows you.
type multibuyBetterPack struct {
	Stockcode   string  `json:"stockcode"`
	Name        string  `json:"name"`
	Brand       string  `json:"brand,omitempty"`
	Price       float64 `json:"price"`
	PackageSize string  `json:"package_size,omitempty"`
	UnitPrice   float64 `json:"unit_price"`
	UnitBasis   string  `json:"unit_basis"`
	IsOnSpecial bool    `json:"is_on_special"`
}

// multibuyOffer is one multi-buy tag, costed at the quantity it demands.
type multibuyOffer struct {
	Stockcode   string `json:"stockcode"`
	Name        string `json:"name"`
	Brand       string `json:"brand,omitempty"`
	PackageSize string `json:"package_size,omitempty"`

	// The offer as advertised.
	OfferQuantity int     `json:"offer_quantity"`
	OfferTotal    float64 `json:"offer_total"`
	OfferCupTag   string  `json:"offer_cup_tag,omitempty"`

	// What the offer actually costs.
	EffectiveUnitPrice float64 `json:"effective_unit_price"`
	ShelfSinglePrice   float64 `json:"shelf_single_price"`
	SavingPerUnit      float64 `json:"saving_per_unit_vs_single"`
	SavingPct          float64 `json:"saving_pct_vs_single"`
	OutlayAtOffer      float64 `json:"outlay_at_offer_quantity"`

	// Normalised comparison against the shelf price of the same product.
	OfferUnitPrice      float64 `json:"offer_normalised_unit_price,omitempty"`
	ShelfUnitPrice      float64 `json:"shelf_normalised_unit_price,omitempty"`
	UnitBasis           string  `json:"unit_basis,omitempty"`
	MeasureKind         string  `json:"measure_kind"`
	NormalisedComparand string  `json:"normalised_comparand,omitempty"`

	// Comparison against the cheapest larger single pack in the result set.
	BetterSinglePack    *multibuyBetterPack `json:"cheapest_larger_single_pack,omitempty"`
	WorseThanSinglePack bool                `json:"worse_than_single_pack"`
	VerdictReason       string              `json:"verdict_reason"`
	IsOnSpecial         bool                `json:"is_on_special"`

	// History is how the shelf single price sits against this product's own
	// recorded low, when the local database has seen it before.
	History *wowHistoryLow `json:"history,omitempty"`
}

type multibuyEnvelope struct {
	Query               string            `json:"query"`
	QueryKind           string            `json:"query_kind"`
	Offers              []multibuyOffer   `json:"offers"`
	OffersFound         int               `json:"offers_found"`
	WorseThanPackCount  int               `json:"worse_than_single_pack_count"`
	ScannedProducts     int               `json:"scanned_products"`
	ScannedPages        int               `json:"scanned_pages"`
	ExcludedUnavailable int               `json:"excluded_unavailable"`
	ExcludedUnparseable int               `json:"excluded_unparseable_measure"`
	HistoryAvailable    bool              `json:"history_available"`
	FetchFailures       []wowFetchFailure `json:"fetch_failures"`
	Note                string            `json:"note,omitempty"`
}

func newNovelMultibuyCmd(flags *rootFlags) *cobra.Command {
	var (
		flagLimit        int
		flagDB           string
		flagMaxScanPages int
	)

	cmd := &cobra.Command{
		Use:   "multibuy <term|category>",
		Short: "Works out what a multi-buy offer actually costs per unit at the quantity it demands",
		Long: strings.Trim(`
Costs every multi-buy tag in a search or category listing at the quantity the
offer actually demands, then checks whether taking it is a good idea.

A "2 for $10" tag tells you a total, not a rate. multibuy divides it out to an
effective per-unit price, normalises that to the product's measure kind — price
per 1kg, per 1L, or per countable unit — and puts it beside two benchmarks the
tag never shows:

  1. buying one unit at the ordinary shelf price, and
  2. the cheapest LARGER single pack of the same measure kind in the same
     result set.

When the larger single pack beats the offer, the row is flagged with
worse_than_single_pack, because a discount that still costs more per kilo than
the pack next to it is not a saving.

Measures are compared only within a kind. A litre tile is never ranked against
a gram tile, and a product whose measure will not parse is excluded and counted
rather than treated as costing zero.

When a term carries no multi-buy tags at all, the offer list comes back empty
with a note saying so. Nothing is invented to fill it.
`, "\n"),
		Example: strings.Trim(`
  # Cost every multi-buy tag on the chocolate shelf
  woolworths-pp-cli multibuy chocolate --limit 10

  # Same, over a browse category id rather than a search term
  woolworths-pp-cli multibuy 1-E5BEE36E --json

  # Scan deeper when the offers are spread thin across the listing
  woolworths-pp-cli multibuy "laundry powder" --max-scan-pages 8 --agent
`, "\n"),
		Annotations: map[string]string{
			// Empty is a valid answer, not an error: a nonsense term is indistinguishable from a valid term carrying no multi-buy offers.
			// Inventing an "empty means not found" rule would break honest-empty output.
			"pp:no-error-path-probe": "true",
			"mcp:read-only":          "true",
			"pp:happy-args":          "term=chocolate",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "multibuy")
			}
			if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("<term|category> is required"))
			}
			query := strings.TrimSpace(args[0])
			limit := flagLimit
			if limit <= 0 {
				limit = 10
			}
			maxPages := wowResolveMaxScanPages(flagMaxScanPages)

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			// Akamai drops unwarmed POST connections; one GET of /shop primes the jar.
			c.WarmSession(ctx)

			// A category id carries a hyphen (e.g. "1-E5BEE36E"); a bare
			// number is a stockcode and still resolves through search.
			queryKind := "term"
			var scan wowScanResult
			if multibuyLooksLikeCategory(query) {
				queryKind = "category"
				scan = wowScanCategory(ctx, c, query, maxPages, wowMaxPageSize, "TraderRelevance")
			} else {
				scan = wowScanSearch(ctx, c, query, maxPages, wowMaxPageSize, "TraderRelevance")
			}

			env := multibuyEnvelope{
				Query:         query,
				QueryKind:     queryKind,
				Offers:        make([]multibuyOffer, 0, limit),
				FetchFailures: make([]wowFetchFailure, 0),
			}
			env.FetchFailures = append(env.FetchFailures, scan.Failures...)
			env.ScannedPages = scan.Pages
			env.ScannedProducts = len(scan.Tiles)

			// Index the whole result set once so each offer can be measured
			// against the larger packs sitting beside it on the same shelf.
			packs := multibuyIndexPacks(scan.Tiles, &env)

			offers := make([]multibuyOffer, 0, len(scan.Tiles))
			for _, tile := range scan.Tiles {
				mb := tile.multibuy()
				if mb == nil {
					continue
				}
				if !tile.available() {
					continue
				}
				offers = append(offers, multibuyCostOffer(tile, mb, packs))
			}

			multibuySortOffers(offers)
			env.OffersFound = len(offers)
			for _, o := range offers {
				if o.WorseThanSinglePack {
					env.WorseThanPackCount++
				}
			}
			if len(offers) > limit {
				offers = offers[:limit]
			}

			// History is an enrichment on the rows that survive the cap, so
			// the database is only consulted for what is actually reported.
			db, closeDB, haveDB := wowOpenHistoryDB(ctx, flagDB)
			defer closeDB()
			var histProbe wowHistoryProbe
			if haveDB {
				for i := range offers {
					hist, herr := wowRecordedLow(ctx, db, offers[i].Stockcode, offers[i].ShelfSinglePrice)
					if histProbe.record(herr) {
						offers[i].History = hist
					}
				}
			}
			env.HistoryAvailable = histProbe.available(haveDB)
			histProbe.warn("multibuy")
			env.Offers = offers

			if env.OffersFound == 0 {
				env.Note = fmt.Sprintf("no multi-buy offers found among %d product(s) scanned over %d page(s) for %s %q; this is an honest empty result, not a fetch failure - widen the scan with --max-scan-pages (currently %d) or try a term whose shelf carries multi-buy tags, e.g. chocolate",
					env.ScannedProducts, env.ScannedPages, queryKind, query, maxPages)
			}
			wowWarnFailures("multibuy", env.FetchFailures, env.ScannedPages)
			return multibuyEmit(cmd, flags, env)
		},
	}

	cmd.Flags().IntVar(&flagLimit, "limit", 10, "Maximum offers to return")
	cmd.Flags().StringVar(&flagDB, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	cmd.Flags().IntVar(&flagMaxScanPages, "max-scan-pages", wowDefaultMaxScanPages,
		"Maximum result pages to scan and filter locally; raise it to widen the search")
	return cmd
}

// multibuyLooksLikeCategory distinguishes a browse category id such as
// "1-E5BEE36E" from a search term. Category ids are hyphenated codes with no
// spaces; anything with whitespace is a term.
func multibuyLooksLikeCategory(q string) bool {
	if strings.ContainsAny(q, " \t") {
		return false
	}
	if !strings.Contains(q, "-") {
		return false
	}
	for _, part := range strings.Split(q, "-") {
		if part == "" {
			return false
		}
		for _, r := range part {
			isDigit := r >= '0' && r <= '9'
			isUpper := r >= 'A' && r <= 'Z'
			if !isDigit && !isUpper {
				return false
			}
		}
	}
	return true
}

// multibuyPack is one single-pack candidate from the result set, kept with the
// size of its pack so "larger" can be judged on the same base units.
type multibuyPack struct {
	tile       wowTile
	normalized unitprice.Normalized
	baseSize   float64
}

// multibuySortOffers ranks costed offers for display: best real saving per unit
// first.
//
// Whole shelves advertise the same headline discount — every "2 for $10"
// against a $7.50 single saves 33.33% — so saving alone would leave the
// ordering to the stockcode tie-break, letting a $39.70/kg offer outrank a
// $27.80/kg one and pushing genuinely cheaper offers outside --limit. The
// normalised unit price breaks that tie on what the shopper actually pays, and
// the stockcode still settles genuinely equal rows so runs stay stable.
func multibuySortOffers(offers []multibuyOffer) {
	sort.SliceStable(offers, func(i, j int) bool {
		a, b := offers[i], offers[j]
		if a.SavingPct != b.SavingPct {
			return a.SavingPct > b.SavingPct
		}
		ra, rb := multibuyUnitRankOf(a), multibuyUnitRankOf(b)
		if ra.basis != rb.basis {
			return ra.basis < rb.basis
		}
		if ra.price != rb.price {
			return ra.price < rb.price
		}
		return a.Stockcode < b.Stockcode
	})
}

// multibuyUnitRank is the tie-break key applied once two offers advertise the
// same headline saving. A unit price only means something inside one measure
// basis, so the basis travels with the price: grouping by it keeps $/kg from
// ever being ranked against $/ea while still ordering like against like.
type multibuyUnitRank struct {
	basis string
	price float64
}

// multibuyUnitRankOf builds that key. An offer whose measure never normalised
// has no unit price to rank on, so it sorts to the end of the tie rather than
// to the front at a phantom price of zero.
func multibuyUnitRankOf(o multibuyOffer) multibuyUnitRank {
	if o.OfferUnitPrice <= 0 || o.UnitBasis == "" {
		return multibuyUnitRank{basis: "￿", price: math.Inf(1)}
	}
	return multibuyUnitRank{basis: o.MeasureKind + "/" + o.UnitBasis, price: o.OfferUnitPrice}
}

// multibuyIndexPacks builds the pool of single packs each offer is benchmarked
// against. A tile only enters the pool when it is buyable, its pack size parses
// and its unit price normalises — anything else is counted as excluded rather
// than admitted at a zero price, which would make it the fake winner.
func multibuyIndexPacks(tiles []wowTile, env *multibuyEnvelope) []multibuyPack {
	packs := make([]multibuyPack, 0, len(tiles))
	for _, tile := range tiles {
		if !tile.available() {
			env.ExcludedUnavailable++
			continue
		}
		size, sizeOK := unitprice.ParseMeasure(tile.PackageSize)
		norm, normOK := wowNormalizeTile(tile)
		if !sizeOK || !normOK || size.Kind != norm.Kind {
			env.ExcludedUnparseable++
			continue
		}
		packs = append(packs, multibuyPack{tile: tile, normalized: norm, baseSize: size.BaseQuantity})
	}
	return packs
}

// multibuyCostOffer turns one multi-buy tag into a costed row.
func multibuyCostOffer(tile wowTile, mb *wowMultibuyData, packs []multibuyPack) multibuyOffer {
	effective := mb.Price / float64(mb.Quantity)
	offer := multibuyOffer{
		Stockcode:          tile.stockcode(),
		Name:               tile.displayName(),
		Brand:              tile.Brand,
		PackageSize:        tile.PackageSize,
		OfferQuantity:      mb.Quantity,
		OfferTotal:         roundMoney(mb.Price),
		OfferCupTag:        mb.CupTag,
		EffectiveUnitPrice: roundMoney(effective),
		ShelfSinglePrice:   roundMoney(tile.Price),
		OutlayAtOffer:      roundMoney(mb.Price),
		MeasureKind:        unitprice.KindUnknown.String(),
	}
	if tile.Price > 0 {
		offer.SavingPerUnit = roundMoney(tile.Price - effective)
		offer.SavingPct = roundMoney((tile.Price - effective) / tile.Price * 100)
	}
	offer.IsOnSpecial = tile.IsOnSpecial

	// Normalise the effective per-unit price against the product's own pack
	// size, which puts it on the same basis as the shelf cup price.
	offerNorm, offerNormOK := unitprice.Normalize(effective, tile.PackageSize)
	if !offerNormOK {
		// Fall back to the retailer's own cup tag for the offer, which is
		// already quoted per measure.
		if price, measure, ok := unitprice.ParseCupTag(mb.CupTag); ok {
			offerNorm, offerNormOK = unitprice.Normalize(price, measure)
			if offerNormOK {
				offer.NormalisedComparand = "offer cup tag"
			}
		}
	} else {
		offer.NormalisedComparand = "effective unit price over pack size"
	}
	shelfNorm, shelfNormOK := wowNormalizeTile(tile)

	switch {
	case offerNormOK:
		offer.OfferUnitPrice = offerNorm.Price
		offer.UnitBasis = offerNorm.Basis
		offer.MeasureKind = offerNorm.Kind.String()
		if shelfNormOK && unitprice.Comparable(offerNorm, shelfNorm) {
			offer.ShelfUnitPrice = shelfNorm.Price
		}
	case shelfNormOK:
		offer.ShelfUnitPrice = shelfNorm.Price
		offer.UnitBasis = shelfNorm.Basis
		offer.MeasureKind = shelfNorm.Kind.String()
	}

	if !offerNormOK {
		offer.VerdictReason = fmt.Sprintf("effective unit price is $%.2f (%d for $%.2f) versus $%.2f on the shelf; the pack measure %q does not parse, so no per-kilo/per-litre comparison against larger packs is possible",
			effective, mb.Quantity, mb.Price, tile.Price, tile.PackageSize)
		return offer
	}

	// Find the cheapest LARGER single pack of the same measure kind.
	ownSize, ownSizeOK := unitprice.ParseMeasure(tile.PackageSize)
	if !ownSizeOK {
		offer.VerdictReason = fmt.Sprintf("effective unit price $%.2f (%d for $%.2f) normalises to $%s/%s; own pack size %q does not parse, so no larger-pack benchmark was applied",
			effective, mb.Quantity, mb.Price, formatUnitPrice(offerNorm.Price), offerNorm.Basis, tile.PackageSize)
		return offer
	}

	var best *multibuyPack
	for i := range packs {
		cand := packs[i]
		if cand.tile.stockcode() == tile.stockcode() {
			continue
		}
		if !unitprice.Comparable(offerNorm, cand.normalized) {
			continue
		}
		if cand.baseSize <= ownSize.BaseQuantity {
			continue
		}
		if best == nil || cand.normalized.Price < best.normalized.Price {
			c := cand
			best = &c
		}
	}

	if best == nil {
		offer.VerdictReason = fmt.Sprintf("effective unit price $%.2f (%d for $%.2f, versus $%.2f single) normalises to $%s/%s; no larger single pack of the same measure kind appeared in this result set to compare against",
			effective, mb.Quantity, mb.Price, tile.Price, formatUnitPrice(offerNorm.Price), offerNorm.Basis)
		return offer
	}

	offer.BetterSinglePack = &multibuyBetterPack{
		Stockcode:   best.tile.stockcode(),
		Name:        best.tile.displayName(),
		Brand:       best.tile.Brand,
		Price:       roundMoney(best.tile.Price),
		PackageSize: best.tile.PackageSize,
		UnitPrice:   best.normalized.Price,
		UnitBasis:   best.normalized.Basis,
		IsOnSpecial: best.tile.IsOnSpecial,
	}
	offer.WorseThanSinglePack = best.normalized.Price < offerNorm.Price
	if offer.WorseThanSinglePack {
		offer.VerdictReason = fmt.Sprintf("taking %d for $%.2f costs $%s/%s, but the larger %s pack of %q is $%s/%s - the offer is dearer per %s despite the tag",
			mb.Quantity, mb.Price, formatUnitPrice(offerNorm.Price), offerNorm.Basis,
			best.tile.PackageSize, best.tile.displayName(),
			formatUnitPrice(best.normalized.Price), best.normalized.Basis, offerNorm.Basis)
	} else {
		offer.VerdictReason = fmt.Sprintf("taking %d for $%.2f costs $%s/%s, which beats both the $%.2f single and the cheapest larger pack (%q at $%s/%s)",
			mb.Quantity, mb.Price, formatUnitPrice(offerNorm.Price), offerNorm.Basis, tile.Price,
			best.tile.displayName(), formatUnitPrice(best.normalized.Price), best.normalized.Basis)
	}
	return offer
}

// formatUnitPrice renders a normalised unit price. It uses the shortest exact
// representation rather than %.2f: a sub-cent per-unit price (e.g. $0.004/1EA)
// would otherwise print as "0.00" and read as free. swap formats the same
// quantity the same way.
func formatUnitPrice(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

// multibuyUnitPriceCell renders the offer's normalised unit price for the human
// table. OfferUnitPrice is only populated when the OFFER normalised; in the
// shelf-only fallback it stays zero, and printing "$0.00/1kg" there reads as
// the cheapest offer on the shelf rather than as a missing figure. The unit
// basis is never printed empty either.
func multibuyUnitPriceCell(o multibuyOffer) string {
	if o.OfferUnitPrice <= 0 || strings.TrimSpace(o.UnitBasis) == "" {
		return "n/a"
	}
	return fmt.Sprintf("$%s/%s", formatUnitPrice(o.OfferUnitPrice), o.UnitBasis)
}

func multibuyEmit(cmd *cobra.Command, flags *rootFlags, env multibuyEnvelope) error {
	out := cmd.OutOrStdout()
	if !wantsHumanTable(out, flags) {
		return printJSONFiltered(out, env, flags)
	}
	if len(env.Offers) == 0 {
		fmt.Fprintf(out, "%s\n", env.Note)
		return nil
	}
	rows := make([]map[string]any, 0, len(env.Offers))
	for _, o := range env.Offers {
		row := map[string]any{
			"name":       o.Name,
			"offer":      fmt.Sprintf("%d for $%.2f", o.OfferQuantity, o.OfferTotal),
			"per_unit":   o.EffectiveUnitPrice,
			"vs_single":  fmt.Sprintf("$%.2f (%.1f%%)", o.SavingPerUnit, o.SavingPct),
			"unit_price": multibuyUnitPriceCell(o),
			"worse_pack": o.WorseThanSinglePack,
		}
		if o.BetterSinglePack != nil {
			row["larger_pack"] = fmt.Sprintf("%s $%s/%s", o.BetterSinglePack.PackageSize, formatUnitPrice(o.BetterSinglePack.UnitPrice), o.BetterSinglePack.UnitBasis)
		}
		rows = append(rows, row)
	}
	if err := printAutoTable(out, rows); err != nil {
		return err
	}
	fmt.Fprintf(out, "\n%d offer(s) among %d product(s) scanned over %d page(s); %d cost more per unit than a larger single pack on the same shelf\n",
		env.OffersFound, env.ScannedProducts, env.ScannedPages, env.WorseThanPackCount)
	return nil
}
