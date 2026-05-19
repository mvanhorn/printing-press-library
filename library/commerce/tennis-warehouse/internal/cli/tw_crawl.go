package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/tennis-warehouse/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/tennis-warehouse/internal/scraper"
	"github.com/mvanhorn/printing-press-library/library/commerce/tennis-warehouse/internal/store"
)

func newCrawlCmd(flags *rootFlags) *cobra.Command {
	var brand string
	var brands []string
	var only string
	var rate float64
	var maxModels int
	var dbPath string
	cmd := &cobra.Command{
		Use:   "crawl",
		Short: "Crawl Tennis Warehouse pages and populate the local store",
		Long: `Crawl Tennis Warehouse new-racquet catalogs and used-racquet inventory,
parse the HTML, and populate the local SQLite store with typed records:
racquets, used_models, used_units, and price_snapshots.

By default, crawls every supported brand. Pass --brand to scope to one.
Pass --only used or --only new to skip half the work.`,
		Example: strings.Trim(`
  # Crawl every brand (~5-15 minutes depending on network)
  tennis-warehouse-pp-cli crawl

  # Crawl only Wilson, used inventory only
  tennis-warehouse-pp-cli crawl --brand wilson --only used

  # Crawl Wilson + Babolat new catalogs
  tennis-warehouse-pp-cli crawl --brands wilson,babolat --only new
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			if dbPath == "" {
				dbPath = defaultDBPath("tennis-warehouse-pp-cli")
			}
			s, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return err
			}
			defer s.Close()
			if err := s.EnsureTennisWarehouseSchema(ctx); err != nil {
				return err
			}
			selected := pickBrands(brand, brands)
			if len(selected) == 0 {
				return fmt.Errorf("no brands selected — pass --brand or --brands, or omit to crawl all")
			}
			// In live-dogfood, curtail to one brand and one model per side.
			if cliutil.IsDogfoodEnv() {
				if len(selected) > 1 {
					selected = selected[:1]
				}
				if maxModels == 0 || maxModels > 1 {
					maxModels = 1
				}
			}
			c := scraper.NewHTTPClient(rate)
			doNew := only == "" || only == "new"
			doUsed := only == "" || only == "used"
			summary := &crawlSummary{}
			for _, b := range selected {
				if doNew {
					if err := crawlNew(ctx, c, s, b, maxModels, summary); err != nil {
						summary.errs = append(summary.errs, fmt.Sprintf("new/%s: %v", b, err))
					}
				}
				if doUsed {
					if err := crawlUsed(ctx, c, s, b, maxModels, summary); err != nil {
						summary.errs = append(summary.errs, fmt.Sprintf("used/%s: %v", b, err))
					}
				}
			}
			return printCrawlSummary(cmd.OutOrStdout(), flags, summary)
		},
	}
	cmd.Flags().StringVar(&brand, "brand", "", "Crawl just this brand (e.g. wilson, babolat, head)")
	cmd.Flags().StringSliceVar(&brands, "brands", nil, "Crawl this comma-separated list of brands")
	cmd.Flags().StringVar(&only, "only", "", "Crawl only 'used' or 'new' (omit for both)")
	cmd.Flags().Float64Var(&rate, "rate", 1.0, "Requests per second (1.0 default; site is polite)")
	cmd.Flags().IntVar(&maxModels, "max-models", 0, "Cap models crawled per brand (0 = no cap; useful for smoke tests)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.cache/tennis-warehouse-pp-cli/store.db)")
	return cmd
}

type crawlSummary struct {
	racquets       int
	usedModels     int
	usedUnits      int
	priceSnapshots int
	errs           []string
}

func pickBrands(single string, multi []string) []string {
	if single != "" {
		return []string{strings.ToLower(strings.TrimSpace(single))}
	}
	if len(multi) > 0 {
		var out []string
		for _, b := range multi {
			b = strings.ToLower(strings.TrimSpace(b))
			if b != "" {
				out = append(out, b)
			}
		}
		return out
	}
	return scraper.AllNewBrands()
}

func crawlNew(ctx context.Context, c *scraper.HTTPClient, s *store.Store, brand string, cap int, sum *crawlSummary) error {
	path, ok := scraper.NewBrandPath[brand]
	if !ok {
		return fmt.Errorf("unknown brand %q", brand)
	}
	body, err := c.Fetch(ctx, path)
	if err != nil {
		return err
	}
	cards, err := scraper.ParseRacquetCatalog(body, strings.Title(brand))
	if err != nil {
		return err
	}
	if cap > 0 && len(cards) > cap {
		cards = cards[:cap]
	}
	for _, card := range cards {
		if card.URL != "" {
			detailBody, derr := c.Fetch(ctx, card.URL)
			if derr == nil {
				r, perr := scraper.ParseRacquetDetail(detailBody, card.SKU, card.URL, card.Brand)
				if perr == nil {
					// Carry through whatever the catalog card knew that the detail page didn't.
					if r.Price == 0 && card.Price > 0 {
						r.Price = card.Price
					}
					if r.MSRP == 0 && card.MSRP > 0 {
						r.MSRP = card.MSRP
					}
					if r.ImageURL == "" && card.ImageURL != "" {
						r.ImageURL = card.ImageURL
					}
					if r.Status == "" && card.Status != "" {
						r.Status = card.Status
					}
					card = *r
				}
			}
		}
		if err := upsertRacquet(ctx, s, &card); err != nil {
			sum.errs = append(sum.errs, fmt.Sprintf("upsert racquet %s: %v", card.SKU, err))
			continue
		}
		if card.Price > 0 {
			_ = recordSnapshot(ctx, s, "racquet", card.SKU, card.Price)
			sum.priceSnapshots++
		}
		sum.racquets++
	}
	return nil
}

func crawlUsed(ctx context.Context, c *scraper.HTTPClient, s *store.Store, brand string, cap int, sum *crawlSummary) error {
	ccode, ok := scraper.BrandCodes[brand]
	if !ok {
		return fmt.Errorf("brand %q has no used-inventory code", brand)
	}
	body, err := c.Fetch(ctx, "/usedcatpage.html?ccode="+ccode)
	if err != nil {
		return err
	}
	cards, err := scraper.ParseUsedCatalog(body)
	if err != nil {
		return err
	}
	if cap > 0 && len(cards) > cap {
		cards = cards[:cap]
	}
	for _, card := range cards {
		detailBody, derr := c.Fetch(ctx, card.URL)
		if derr != nil {
			sum.errs = append(sum.errs, fmt.Sprintf("fetch %s: %v", card.URL, derr))
			continue
		}
		m, units, perr := scraper.ParseUsedDetail(detailBody, card.PCode, card.URL, card.Brand)
		if perr != nil {
			sum.errs = append(sum.errs, fmt.Sprintf("parse %s: %v", card.PCode, perr))
			continue
		}
		// Merge catalog price hints when the detail page didn't surface them.
		if m.PriceLow == 0 && card.PriceLow > 0 {
			m.PriceLow = card.PriceLow
		}
		if m.PriceHigh == 0 && card.PriceHigh > 0 {
			m.PriceHigh = card.PriceHigh
		}
		if m.MSRP == 0 && card.MSRP > 0 {
			m.MSRP = card.MSRP
		}
		if m.ImageURL == "" && card.ImageURL != "" {
			m.ImageURL = card.ImageURL
		}
		if err := upsertUsedModel(ctx, s, m); err != nil {
			sum.errs = append(sum.errs, fmt.Sprintf("upsert model %s: %v", m.PCode, err))
			continue
		}
		sum.usedModels++
		for i := range units {
			if err := upsertUsedUnit(ctx, s, &units[i]); err != nil {
				sum.errs = append(sum.errs, fmt.Sprintf("upsert unit %s: %v", units[i].StockCode, err))
				continue
			}
			sum.usedUnits++
			if units[i].Price > 0 {
				_ = recordSnapshot(ctx, s, "used_unit", units[i].StockCode, units[i].Price)
				sum.priceSnapshots++
			}
		}
	}
	return nil
}

func upsertRacquet(ctx context.Context, s *store.Store, r *scraper.Racquet) error {
	if r.LastSeenAt.IsZero() {
		r.LastSeenAt = time.Now().UTC()
	}
	_, err := s.DB().ExecContext(ctx, `
		INSERT INTO racquets (sku, brand, model, price, msrp, url, image_url,
			head_size_in2, strung_weight, unstrung_oz, balance, swingweight,
			stiffness, beam_width, string_pattern, length_in, composition,
			power_level, stroke_style, status, description, last_seen_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(sku) DO UPDATE SET
			brand=excluded.brand,
			model=excluded.model,
			price=COALESCE(NULLIF(excluded.price,0), price),
			msrp=COALESCE(NULLIF(excluded.msrp,0), msrp),
			url=excluded.url,
			image_url=COALESCE(NULLIF(excluded.image_url,''), image_url),
			head_size_in2=COALESCE(NULLIF(excluded.head_size_in2,0), head_size_in2),
			strung_weight=COALESCE(NULLIF(excluded.strung_weight,0), strung_weight),
			unstrung_oz=COALESCE(NULLIF(excluded.unstrung_oz,0), unstrung_oz),
			balance=COALESCE(NULLIF(excluded.balance,''), balance),
			swingweight=COALESCE(NULLIF(excluded.swingweight,0), swingweight),
			stiffness=COALESCE(NULLIF(excluded.stiffness,0), stiffness),
			beam_width=COALESCE(NULLIF(excluded.beam_width,''), beam_width),
			string_pattern=COALESCE(NULLIF(excluded.string_pattern,''), string_pattern),
			length_in=COALESCE(NULLIF(excluded.length_in,0), length_in),
			composition=COALESCE(NULLIF(excluded.composition,''), composition),
			power_level=COALESCE(NULLIF(excluded.power_level,''), power_level),
			stroke_style=COALESCE(NULLIF(excluded.stroke_style,''), stroke_style),
			status=COALESCE(NULLIF(excluded.status,''), status),
			description=COALESCE(NULLIF(excluded.description,''), description),
			last_seen_at=excluded.last_seen_at`,
		r.SKU, r.Brand, r.Model, r.Price, r.MSRP, r.URL, r.ImageURL,
		r.HeadSizeIn2, r.StrungWeight, r.UnstrungOz, r.Balance, r.Swingweight,
		r.Stiffness, r.BeamWidth, r.StringPattern, r.LengthIn, r.Composition,
		r.PowerLevel, r.StrokeStyle, r.Status, r.Description, r.LastSeenAt.Format(time.RFC3339))
	if err != nil {
		return err
	}
	// Mirror searchable fields into FTS.
	_, err = s.DB().ExecContext(ctx, `INSERT INTO racquets_fts (sku, brand, model, composition) VALUES (?,?,?,?)`,
		r.SKU, r.Brand, r.Model, r.Composition)
	return err
}

func upsertUsedModel(ctx context.Context, s *store.Store, m *scraper.UsedModel) error {
	if m.LastSeenAt.IsZero() {
		m.LastSeenAt = time.Now().UTC()
	}
	if m.FirstSeenAt.IsZero() {
		m.FirstSeenAt = m.LastSeenAt
	}
	_, err := s.DB().ExecContext(ctx, `
		INSERT INTO used_models (pcode, brand, model, url, image_url, price_low, price_high, msrp,
			head_size_in2, strung_weight, unstrung_oz, balance, swingweight, stiffness,
			beam_width, string_pattern, length_in, composition, power_level, stroke_style,
			unit_count, first_seen_at, last_seen_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(pcode) DO UPDATE SET
			brand=COALESCE(NULLIF(excluded.brand,''), brand),
			model=COALESCE(NULLIF(excluded.model,''), model),
			url=COALESCE(NULLIF(excluded.url,''), url),
			image_url=COALESCE(NULLIF(excluded.image_url,''), image_url),
			price_low=COALESCE(NULLIF(excluded.price_low,0), price_low),
			price_high=COALESCE(NULLIF(excluded.price_high,0), price_high),
			msrp=COALESCE(NULLIF(excluded.msrp,0), msrp),
			head_size_in2=COALESCE(NULLIF(excluded.head_size_in2,0), head_size_in2),
			strung_weight=COALESCE(NULLIF(excluded.strung_weight,0), strung_weight),
			unstrung_oz=COALESCE(NULLIF(excluded.unstrung_oz,0), unstrung_oz),
			balance=COALESCE(NULLIF(excluded.balance,''), balance),
			swingweight=COALESCE(NULLIF(excluded.swingweight,0), swingweight),
			stiffness=COALESCE(NULLIF(excluded.stiffness,0), stiffness),
			beam_width=COALESCE(NULLIF(excluded.beam_width,''), beam_width),
			string_pattern=COALESCE(NULLIF(excluded.string_pattern,''), string_pattern),
			length_in=COALESCE(NULLIF(excluded.length_in,0), length_in),
			composition=COALESCE(NULLIF(excluded.composition,''), composition),
			power_level=COALESCE(NULLIF(excluded.power_level,''), power_level),
			stroke_style=COALESCE(NULLIF(excluded.stroke_style,''), stroke_style),
			unit_count=excluded.unit_count,
			last_seen_at=excluded.last_seen_at`,
		m.PCode, m.Brand, m.Model, m.URL, m.ImageURL, m.PriceLow, m.PriceHigh, m.MSRP,
		m.HeadSizeIn2, m.StrungWeight, m.UnstrungOz, m.Balance, m.Swingweight, m.Stiffness,
		m.BeamWidth, m.StringPattern, m.LengthIn, m.Composition, m.PowerLevel, m.StrokeStyle,
		m.UnitCount, m.FirstSeenAt.Format(time.RFC3339), m.LastSeenAt.Format(time.RFC3339))
	if err != nil {
		return err
	}
	_, err = s.DB().ExecContext(ctx, `INSERT INTO used_models_fts (pcode, brand, model, composition) VALUES (?,?,?,?)`,
		m.PCode, m.Brand, m.Model, m.Composition)
	return err
}

func upsertUsedUnit(ctx context.Context, s *store.Store, u *scraper.UsedUnit) error {
	if u.LastSeenAt.IsZero() {
		u.LastSeenAt = time.Now().UTC()
	}
	if u.FirstSeenAt.IsZero() {
		u.FirstSeenAt = u.LastSeenAt
	}
	_, err := s.DB().ExecContext(ctx, `
		INSERT INTO used_units (stock_code, pcode, grade, grip_size, price, notes,
			first_seen_at, last_seen_at)
		VALUES (?,?,?,?,?,?,?,?)
		ON CONFLICT(stock_code) DO UPDATE SET
			pcode=excluded.pcode, grade=excluded.grade,
			grip_size=COALESCE(NULLIF(excluded.grip_size,''), grip_size),
			price=excluded.price,
			notes=excluded.notes,
			last_seen_at=excluded.last_seen_at`,
		u.StockCode, u.PCode, u.Grade, u.GripSize, u.Price, u.Notes,
		u.FirstSeenAt.Format(time.RFC3339), u.LastSeenAt.Format(time.RFC3339))
	return err
}

func recordSnapshot(ctx context.Context, s *store.Store, kind, ref string, price float64) error {
	_, err := s.DB().ExecContext(ctx,
		`INSERT INTO price_snapshots (kind, ref, price, captured_at) VALUES (?,?,?,?)`,
		kind, ref, price, time.Now().UTC().Format(time.RFC3339))
	return err
}

func printCrawlSummary(w io.Writer, flags *rootFlags, sum *crawlSummary) error {
	out := map[string]any{
		"racquets":        sum.racquets,
		"used_models":     sum.usedModels,
		"used_units":      sum.usedUnits,
		"price_snapshots": sum.priceSnapshots,
		"errors":          sum.errs,
	}
	if flags.asJSON || !isTerminal(w) {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}
	fmt.Fprintf(w, "racquets:         %d\n", sum.racquets)
	fmt.Fprintf(w, "used_models:      %d\n", sum.usedModels)
	fmt.Fprintf(w, "used_units:       %d\n", sum.usedUnits)
	fmt.Fprintf(w, "price_snapshots:  %d\n", sum.priceSnapshots)
	if len(sum.errs) > 0 {
		fmt.Fprintf(w, "errors (%d):\n", len(sum.errs))
		for _, e := range sum.errs {
			fmt.Fprintf(w, "  - %s\n", e)
		}
	}
	return nil
}

// Lookup helper: resolve a brand input to its canonical slug. Used by the
// novel commands when the user passes --brand wilson or --brand Wilson.
func canonicalBrand(b string) string {
	b = strings.ToLower(strings.TrimSpace(b))
	for _, valid := range scraper.AllNewBrands() {
		if valid == b {
			return b
		}
	}
	return ""
}

// Avoid "imported and not used" if a helper drops out during iteration.
var _ = sort.Strings
var _ = sync.Mutex{}
var _ = errors.Is
var _ = sql.Open
