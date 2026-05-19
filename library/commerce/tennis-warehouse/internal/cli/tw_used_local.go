package cli

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/tennis-warehouse/internal/scraper"
	"github.com/mvanhorn/printing-press-library/library/commerce/tennis-warehouse/internal/store"
)

// ====================================================================
// used units <pcode>
// ====================================================================

type unitRow struct {
	StockCode   string  `json:"stock_code"`
	PCode       string  `json:"pcode"`
	Brand       string  `json:"brand,omitempty"`
	Model       string  `json:"model,omitempty"`
	Grade       string  `json:"grade"`
	GripSize    string  `json:"grip_size,omitempty"`
	Price       float64 `json:"price"`
	Notes       string  `json:"notes,omitempty"`
	FirstSeenAt string  `json:"first_seen_at,omitempty"`
}

func newUsedUnitsCmd(flags *rootFlags) *cobra.Command {
	var grade string
	cmd := &cobra.Command{
		Use:         "units [pcode]",
		Short:       "List individual used units for a model (per-stock-code rows with grade and price)",
		Example:     "  tennis-warehouse-pp-cli used units WB9816 --grade A --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			s, err := openTennisStore(ctx)
			if err != nil {
				return err
			}
			defer s.Close()
			rows, err := queryUnits(ctx, s, args[0], grade)
			if err != nil {
				return err
			}
			if len(rows) == 0 {
				return fmt.Errorf("no units for pcode %q in local store (run 'crawl' to populate, or check the SKU spelling)", args[0])
			}
			return printUnitRows(cmd.OutOrStdout(), flags, rows)
		},
	}
	cmd.Flags().StringVar(&grade, "grade", "", "Filter by grade: A, B, C, Unused (matches 'Grade A' etc.)")
	return cmd
}

func queryUnits(ctx context.Context, s *store.Store, pcode, grade string) ([]unitRow, error) {
	var conds []string
	var args []any
	conds = append(conds, "u.pcode = ?")
	args = append(args, pcode)
	if grade != "" {
		g := strings.TrimSpace(grade)
		if !strings.HasPrefix(strings.ToLower(g), "grade") && !strings.EqualFold(g, "unused") {
			g = "Grade " + strings.ToUpper(g)
		}
		conds = append(conds, "u.grade = ?")
		args = append(args, g)
	}
	q := `SELECT u.stock_code, u.pcode, COALESCE(m.brand,''), COALESCE(m.model,''),
			u.grade, COALESCE(u.grip_size,''), u.price, COALESCE(u.notes,''),
			u.first_seen_at
		FROM used_units u LEFT JOIN used_models m ON u.pcode = m.pcode
		WHERE ` + strings.Join(conds, " AND ") + `
		ORDER BY u.grade, u.price ASC`
	rows, err := s.DB().QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []unitRow{}
	for rows.Next() {
		var r unitRow
		if err := rows.Scan(&r.StockCode, &r.PCode, &r.Brand, &r.Model, &r.Grade,
			&r.GripSize, &r.Price, &r.Notes, &r.FirstSeenAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func printUnitRows(w io.Writer, flags *rootFlags, rows []unitRow) error {
	if flags.asJSON || !isTerminal(w) {
		return printJSONFiltered(w, rows, flags)
	}
	if len(rows) == 0 {
		fmt.Fprintln(w, "(no units found — run 'crawl' to populate, or adjust --grade)")
		return nil
	}
	for _, r := range rows {
		notes := r.Notes
		if notes != "" {
			notes = " (" + notes + ")"
		}
		fmt.Fprintf(w, "  %-10s %-9s grip=%-6s  $%-7.2f  %s%s\n",
			r.StockCode, r.Grade, r.GripSize, r.Price, twTruncate(r.Model, 40), notes)
	}
	fmt.Fprintf(w, "\n%d unit(s)\n", len(rows))
	return nil
}

// ====================================================================
// used new --since <window>
// ====================================================================

func newUsedNewCmd(flags *rootFlags) *cobra.Command {
	var since string
	var brand string
	var grade string
	cmd := &cobra.Command{
		Use:   "new",
		Short: "List used listings whose first_seen_at falls within a recent window",
		Example: strings.Trim(`
  tennis-warehouse-pp-cli used new --since 7d
  tennis-warehouse-pp-cli used new --since 30d --brand wilson --grade A --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			d, err := parseSince(since)
			if err != nil {
				return err
			}
			cutoff := time.Now().UTC().Add(-d).Format(time.RFC3339)
			ctx := cmd.Context()
			s, err := openTennisStore(ctx)
			if err != nil {
				return err
			}
			defer s.Close()
			var conds = []string{"u.first_seen_at >= ?"}
			var argsSQL = []any{cutoff}
			if brand != "" {
				conds = append(conds, "LOWER(m.brand) = ?")
				argsSQL = append(argsSQL, strings.ToLower(brand))
			}
			if grade != "" {
				g := normalizeGrade(grade)
				conds = append(conds, "u.grade = ?")
				argsSQL = append(argsSQL, g)
			}
			q := `SELECT u.stock_code, u.pcode, COALESCE(m.brand,''), COALESCE(m.model,''),
					u.grade, COALESCE(u.grip_size,''), u.price, COALESCE(u.notes,''),
					u.first_seen_at
				FROM used_units u LEFT JOIN used_models m ON u.pcode = m.pcode
				WHERE ` + strings.Join(conds, " AND ") + `
				ORDER BY u.first_seen_at DESC, u.grade, u.price ASC`
			rows, err := s.DB().QueryContext(ctx, q, argsSQL...)
			if err != nil {
				return err
			}
			defer rows.Close()
			out := []unitRow{}
			for rows.Next() {
				var r unitRow
				if err := rows.Scan(&r.StockCode, &r.PCode, &r.Brand, &r.Model, &r.Grade,
					&r.GripSize, &r.Price, &r.Notes, &r.FirstSeenAt); err != nil {
					return err
				}
				out = append(out, r)
			}
			return printUnitRows(cmd.OutOrStdout(), flags, out)
		},
	}
	cmd.Flags().StringVar(&since, "since", "7d", "Window: e.g. 24h, 7d, 30d")
	cmd.Flags().StringVar(&brand, "brand", "", "Filter by brand")
	cmd.Flags().StringVar(&grade, "grade", "", "Filter by grade (A, B, C, Unused)")
	return cmd
}

func parseSince(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 7 * 24 * time.Hour, nil
	}
	if strings.HasSuffix(s, "d") {
		n := strings.TrimSuffix(s, "d")
		var days int
		if _, err := fmt.Sscanf(n, "%d", &days); err == nil && days > 0 {
			return time.Duration(days) * 24 * time.Hour, nil
		}
	}
	return time.ParseDuration(s)
}

func normalizeGrade(g string) string {
	g = strings.TrimSpace(g)
	if g == "" {
		return ""
	}
	if strings.EqualFold(g, "unused") {
		return "Unused"
	}
	if strings.HasPrefix(strings.ToLower(g), "grade") {
		return "Grade " + strings.ToUpper(strings.TrimSpace(strings.TrimPrefix(strings.ToLower(g), "grade")))
	}
	return "Grade " + strings.ToUpper(g)
}

// ====================================================================
// used deals --min-discount-pct N [--grade A] [--brand x]
// ====================================================================

type dealRow struct {
	StockCode   string  `json:"stock_code"`
	PCode       string  `json:"pcode"`
	Brand       string  `json:"brand"`
	Model       string  `json:"model"`
	Grade       string  `json:"grade"`
	GripSize    string  `json:"grip_size,omitempty"`
	Price       float64 `json:"price"`
	MSRP        float64 `json:"msrp"`
	DiscountPct float64 `json:"discount_pct"`
}

func newUsedDealsCmd(flags *rootFlags) *cobra.Command {
	var minDiscount float64
	var brand string
	var grade string
	cmd := &cobra.Command{
		Use:   "deals",
		Short: "Find used listings whose price is a steep discount versus the new-racquet MSRP",
		Long: `LEFT JOINs used_units against the racquets catalog (by brand + model
substring match) to compute a discount percentage per listing. Surfaces
the bargain-hunt path the website cannot show.`,
		Example:     "  tennis-warehouse-pp-cli used deals --min-discount-pct 30 --grade A --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			s, err := openTennisStore(ctx)
			if err != nil {
				return err
			}
			defer s.Close()
			// Match used_models to racquets by (brand, model). The model strings
			// usually share a common prefix (e.g. "Wilson Blade 98 16x19 v9 Racquet"
			// vs "Wilson Blade 98 16x19 v9"). Compare on lowercased brand + first
			// 25 chars of model for a fuzzy-but-cheap match.
			var conds = []string{"r.price > 0", "u.price > 0"}
			var argsSQL []any
			if brand != "" {
				conds = append(conds, "LOWER(m.brand) = ?")
				argsSQL = append(argsSQL, strings.ToLower(brand))
			}
			if grade != "" {
				conds = append(conds, "u.grade = ?")
				argsSQL = append(argsSQL, normalizeGrade(grade))
			}
			q := `SELECT u.stock_code, u.pcode, m.brand, m.model, u.grade,
					COALESCE(u.grip_size,''), u.price,
					COALESCE(r.msrp, r.price) AS reference_price,
					(COALESCE(r.msrp, r.price) - u.price) * 100.0 / COALESCE(r.msrp, r.price) AS discount_pct
				FROM used_units u
				JOIN used_models m ON u.pcode = m.pcode
				JOIN racquets r ON LOWER(r.brand) = LOWER(m.brand)
					AND substr(LOWER(r.model), 1, 25) = substr(LOWER(m.model), 1, 25)
				WHERE ` + strings.Join(conds, " AND ") + `
				ORDER BY discount_pct DESC`
			rows, err := s.DB().QueryContext(ctx, q, argsSQL...)
			if err != nil {
				return err
			}
			defer rows.Close()
			out := []dealRow{}
			for rows.Next() {
				var d dealRow
				var msrp, disc sql.NullFloat64
				if err := rows.Scan(&d.StockCode, &d.PCode, &d.Brand, &d.Model, &d.Grade,
					&d.GripSize, &d.Price, &msrp, &disc); err != nil {
					return err
				}
				d.MSRP = msrp.Float64
				d.DiscountPct = disc.Float64
				if !disc.Valid {
					continue
				}
				if d.DiscountPct < minDiscount {
					continue
				}
				out = append(out, d)
			}
			return printDealRows(cmd.OutOrStdout(), flags, out)
		},
	}
	cmd.Flags().Float64Var(&minDiscount, "min-discount-pct", 30, "Minimum discount percentage versus MSRP")
	cmd.Flags().StringVar(&brand, "brand", "", "Filter by brand")
	cmd.Flags().StringVar(&grade, "grade", "", "Filter by grade (A, B, C, Unused)")
	return cmd
}

func printDealRows(w io.Writer, flags *rootFlags, rows []dealRow) error {
	if flags.asJSON || !isTerminal(w) {
		return printJSONFiltered(w, rows, flags)
	}
	if len(rows) == 0 {
		fmt.Fprintln(w, "(no deals matched — run 'crawl' to populate both used and new sides, or lower --min-discount-pct)")
		return nil
	}
	for _, d := range rows {
		fmt.Fprintf(w, "  %-10s %-9s grip=%-6s  $%-7.2f vs MSRP $%-7.2f  -%.1f%%  %s\n",
			d.StockCode, d.Grade, d.GripSize, d.Price, d.MSRP, d.DiscountPct,
			twTruncate(d.Model, 40))
	}
	fmt.Fprintf(w, "\n%d deal(s)\n", len(rows))
	return nil
}

// ====================================================================
// used drops --since <window> --min-drop-pct N [--watchlist-only]
// ====================================================================

type dropRow struct {
	StockCode  string  `json:"stock_code"`
	PCode      string  `json:"pcode"`
	Brand      string  `json:"brand"`
	Model      string  `json:"model"`
	Grade      string  `json:"grade"`
	Price      float64 `json:"price"`
	PriorPrice float64 `json:"prior_price"`
	DropPct    float64 `json:"drop_pct"`
	CapturedAt string  `json:"captured_at"`
}

func newUsedDropsCmd(flags *rootFlags) *cobra.Command {
	var since string
	var minDrop float64
	var brand string
	var watchOnly bool
	cmd := &cobra.Command{
		Use:   "drops",
		Short: "List used listings whose latest price dropped versus a prior snapshot",
		Long: `Joins each used_unit to its two latest price_snapshot rows and computes a drop
percentage. The website does not store any price history.`,
		Example:     "  tennis-warehouse-pp-cli used drops --since 7d --min-drop-pct 10 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			d, err := parseSince(since)
			if err != nil {
				return err
			}
			cutoff := time.Now().UTC().Add(-d).Format(time.RFC3339)
			ctx := cmd.Context()
			s, err := openTennisStore(ctx)
			if err != nil {
				return err
			}
			defer s.Close()
			// Build the snapshot pair per stock_code via a window function:
			// LAG(price) over (partition by ref order by captured_at). SQLite has window functions.
			q := `WITH paired AS (
					SELECT ps.ref AS stock_code,
						ps.price AS price,
						ps.captured_at AS captured_at,
						LAG(ps.price) OVER (PARTITION BY ps.ref ORDER BY ps.captured_at) AS prior_price
					FROM price_snapshots ps
					WHERE ps.kind = 'used_unit'
				)
				SELECT p.stock_code, u.pcode, m.brand, m.model, u.grade,
					p.price, p.prior_price, p.captured_at
				FROM paired p
				JOIN used_units u ON p.stock_code = u.stock_code
				LEFT JOIN used_models m ON u.pcode = m.pcode
				WHERE p.prior_price > 0
				  AND p.price < p.prior_price
				  AND p.captured_at >= ?
				ORDER BY (p.prior_price - p.price) * 100.0 / p.prior_price DESC`
			rows, err := s.DB().QueryContext(ctx, q, cutoff)
			if err != nil {
				return err
			}
			defer rows.Close()
			out := []dropRow{}
			var watchSet map[string]struct{}
			if watchOnly {
				watchSet, err = loadWatchlistSet(ctx, s)
				if err != nil {
					return err
				}
			}
			for rows.Next() {
				var r dropRow
				var pcode, model, gbrand, grade, captured string
				var price, prior float64
				var stockCode string
				if err := rows.Scan(&stockCode, &pcode, &gbrand, &model, &grade, &price, &prior, &captured); err != nil {
					return err
				}
				r.StockCode = stockCode
				r.PCode = pcode
				r.Brand = gbrand
				r.Model = model
				r.Grade = grade
				r.Price = price
				r.PriorPrice = prior
				r.CapturedAt = captured
				r.DropPct = (prior - price) * 100.0 / prior
				if r.DropPct < minDrop {
					continue
				}
				if brand != "" && !strings.EqualFold(brand, gbrand) {
					continue
				}
				if watchOnly {
					if _, ok := watchSet[pcode]; !ok {
						continue
					}
				}
				out = append(out, r)
			}
			return printDropRows(cmd.OutOrStdout(), flags, out)
		},
	}
	cmd.Flags().StringVar(&since, "since", "7d", "Time window: e.g. 24h, 7d, 30d")
	cmd.Flags().Float64Var(&minDrop, "min-drop-pct", 5, "Minimum drop percentage")
	cmd.Flags().StringVar(&brand, "brand", "", "Filter by brand")
	cmd.Flags().BoolVar(&watchOnly, "watchlist-only", false, "Only return drops for watchlisted pcodes")
	return cmd
}

func printDropRows(w io.Writer, flags *rootFlags, rows []dropRow) error {
	if flags.asJSON || !isTerminal(w) {
		return printJSONFiltered(w, rows, flags)
	}
	if len(rows) == 0 {
		fmt.Fprintln(w, "(no drops detected — need at least two crawl runs with different prices)")
		return nil
	}
	for _, r := range rows {
		fmt.Fprintf(w, "  %-10s %-9s  $%-7.2f -> $%-7.2f  -%.1f%%  %s\n",
			r.StockCode, r.Grade, r.PriorPrice, r.Price, r.DropPct, twTruncate(r.Model, 40))
	}
	fmt.Fprintf(w, "\n%d drop(s)\n", len(rows))
	return nil
}

func loadWatchlistSet(ctx context.Context, s *store.Store) (map[string]struct{}, error) {
	rows, err := s.DB().QueryContext(ctx, "SELECT pcode FROM watchlist")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]struct{})
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		out[p] = struct{}{}
	}
	return out, rows.Err()
}

// ====================================================================
// used depth --min-units N [--grade] [--brand]
// ====================================================================

type depthRow struct {
	PCode    string  `json:"pcode"`
	Brand    string  `json:"brand"`
	Model    string  `json:"model"`
	GradeA   int     `json:"grade_a"`
	GradeB   int     `json:"grade_b"`
	GradeC   int     `json:"grade_c"`
	Unused   int     `json:"unused"`
	Total    int     `json:"total"`
	MinPrice float64 `json:"min_price"`
	MaxPrice float64 `json:"max_price"`
}

func newUsedDepthCmd(flags *rootFlags) *cobra.Command {
	var minUnits int
	var brand string
	var grade string
	cmd := &cobra.Command{
		Use:   "depth",
		Short: "Per-model used-inventory unit counts grouped by condition grade",
		Long: `Aggregates physical unit counts per model. The website only exposes
per-model unit lists on the order page — there's no aggregate view.`,
		Example:     "  tennis-warehouse-pp-cli used depth --min-units 3 --grade A --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			s, err := openTennisStore(ctx)
			if err != nil {
				return err
			}
			defer s.Close()
			q := `SELECT m.pcode, m.brand, m.model,
					SUM(CASE WHEN u.grade = 'Grade A' THEN 1 ELSE 0 END) AS grade_a,
					SUM(CASE WHEN u.grade = 'Grade B' THEN 1 ELSE 0 END) AS grade_b,
					SUM(CASE WHEN u.grade = 'Grade C' THEN 1 ELSE 0 END) AS grade_c,
					SUM(CASE WHEN u.grade = 'Unused' THEN 1 ELSE 0 END) AS unused,
					COUNT(*) AS total,
					MIN(u.price) AS min_price, MAX(u.price) AS max_price
				FROM used_models m JOIN used_units u ON u.pcode = m.pcode
				WHERE COALESCE(u.notes,'') != 'sold_out'`
			if brand != "" {
				q += " AND LOWER(m.brand) = '" + strings.ReplaceAll(strings.ToLower(brand), "'", "''") + "'"
			}
			q += " GROUP BY m.pcode, m.brand, m.model ORDER BY total DESC"
			rows, err := s.DB().QueryContext(ctx, q)
			if err != nil {
				return err
			}
			defer rows.Close()
			out := []depthRow{}
			for rows.Next() {
				var d depthRow
				var minP, maxP sql.NullFloat64
				if err := rows.Scan(&d.PCode, &d.Brand, &d.Model, &d.GradeA, &d.GradeB,
					&d.GradeC, &d.Unused, &d.Total, &minP, &maxP); err != nil {
					return err
				}
				d.MinPrice = minP.Float64
				d.MaxPrice = maxP.Float64
				if d.Total < minUnits {
					continue
				}
				if grade != "" {
					ng := normalizeGrade(grade)
					switch ng {
					case "Grade A":
						if d.GradeA == 0 {
							continue
						}
					case "Grade B":
						if d.GradeB == 0 {
							continue
						}
					case "Grade C":
						if d.GradeC == 0 {
							continue
						}
					case "Unused":
						if d.Unused == 0 {
							continue
						}
					}
				}
				out = append(out, d)
			}
			return printDepthRows(cmd.OutOrStdout(), flags, out)
		},
	}
	cmd.Flags().IntVar(&minUnits, "min-units", 1, "Only return models with at least this many units in stock")
	cmd.Flags().StringVar(&brand, "brand", "", "Filter by brand")
	cmd.Flags().StringVar(&grade, "grade", "", "Only return models that have units in this grade (A, B, C, Unused)")
	return cmd
}

func printDepthRows(w io.Writer, flags *rootFlags, rows []depthRow) error {
	if flags.asJSON || !isTerminal(w) {
		return printJSONFiltered(w, rows, flags)
	}
	if len(rows) == 0 {
		fmt.Fprintln(w, "(no models matched — run 'crawl' to populate, or relax filters)")
		return nil
	}
	for _, r := range rows {
		fmt.Fprintf(w, "  %-10s A:%-2d B:%-2d C:%-2d U:%-2d total:%-3d  $%.0f-$%.0f  %s\n",
			r.PCode, r.GradeA, r.GradeB, r.GradeC, r.Unused, r.Total,
			r.MinPrice, r.MaxPrice, twTruncate(r.Model, 40))
	}
	fmt.Fprintf(w, "\n%d model(s)\n", len(rows))
	return nil
}

// ====================================================================
// used watch <pcode> / used watchlist [drops]
// ====================================================================

func newUsedWatchCmd(flags *rootFlags) *cobra.Command {
	var label string
	cmd := &cobra.Command{
		Use:         "watch [pcode]",
		Short:       "Add a model pcode to the watchlist",
		Example:     "  tennis-warehouse-pp-cli used watch WB9810 --label 'Blade 98 v10 candidate'",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			s, err := openTennisStore(ctx)
			if err != nil {
				return err
			}
			defer s.Close()
			// Validate the pcode exists in used_models so we don't pollute the
			// watchlist with bogus SKUs.
			var exists int
			if err := s.DB().QueryRowContext(ctx,
				"SELECT COUNT(*) FROM used_models WHERE pcode = ?", args[0]).Scan(&exists); err != nil {
				return err
			}
			if exists == 0 {
				return fmt.Errorf("pcode %q is not in the local used_models table — run 'crawl' first or verify the SKU", args[0])
			}
			_, err = s.DB().ExecContext(ctx, `
				INSERT INTO watchlist (pcode, label, added_at) VALUES (?,?,?)
				ON CONFLICT(pcode) DO UPDATE SET label=excluded.label`,
				args[0], label, time.Now().UTC().Format(time.RFC3339))
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Added %s to watchlist.\n", args[0])
			return nil
		},
	}
	cmd.Flags().StringVar(&label, "label", "", "Optional human-readable note")
	return cmd
}

type watchEntry struct {
	PCode   string `json:"pcode"`
	Label   string `json:"label,omitempty"`
	AddedAt string `json:"added_at"`
	Brand   string `json:"brand,omitempty"`
	Model   string `json:"model,omitempty"`
}

func newUsedWatchlistCmd(flags *rootFlags) *cobra.Command {
	var since string
	var minDrop float64
	cmd := &cobra.Command{
		Use:   "watchlist",
		Short: "Show current watchlist; with 'drops' subcommand, list price drops on watched models",
		Example: strings.Trim(`
  tennis-warehouse-pp-cli used watchlist
  tennis-warehouse-pp-cli used watchlist drops --since 7d --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			s, err := openTennisStore(ctx)
			if err != nil {
				return err
			}
			defer s.Close()
			if len(args) > 0 && args[0] == "drops" {
				// Delegate to drops subcommand-style filter.
				return runWatchlistDrops(ctx, s, cmd.OutOrStdout(), flags, since, minDrop)
			}
			rows, err := s.DB().QueryContext(ctx, `
				SELECT w.pcode, COALESCE(w.label,''), w.added_at,
					COALESCE(m.brand,''), COALESCE(m.model,'')
				FROM watchlist w LEFT JOIN used_models m ON w.pcode = m.pcode
				ORDER BY w.added_at DESC`)
			if err != nil {
				return err
			}
			defer rows.Close()
			out := []watchEntry{}
			for rows.Next() {
				var e watchEntry
				if err := rows.Scan(&e.PCode, &e.Label, &e.AddedAt, &e.Brand, &e.Model); err != nil {
					return err
				}
				out = append(out, e)
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "(watchlist empty — use 'used watch <pcode>' to add)")
				return nil
			}
			for _, e := range out {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-10s  %s  %s\n", e.PCode, twTruncate(e.Model, 40), e.Label)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n%d watched model(s)\n", len(out))
			return nil
		},
	}
	cmd.Flags().StringVar(&since, "since", "30d", "(For 'watchlist drops') window")
	cmd.Flags().Float64Var(&minDrop, "min-drop-pct", 5, "(For 'watchlist drops') minimum drop pct")
	return cmd
}

func runWatchlistDrops(ctx context.Context, s *store.Store, w io.Writer, flags *rootFlags, since string, minDrop float64) error {
	d, err := parseSince(since)
	if err != nil {
		return err
	}
	cutoff := time.Now().UTC().Add(-d).Format(time.RFC3339)
	q := `WITH paired AS (
			SELECT ps.ref AS stock_code, ps.price, ps.captured_at,
				LAG(ps.price) OVER (PARTITION BY ps.ref ORDER BY ps.captured_at) AS prior_price
			FROM price_snapshots ps
			WHERE ps.kind = 'used_unit'
		)
		SELECT p.stock_code, u.pcode, m.brand, m.model, u.grade,
			p.price, p.prior_price, p.captured_at
		FROM paired p
		JOIN used_units u ON p.stock_code = u.stock_code
		LEFT JOIN used_models m ON u.pcode = m.pcode
		JOIN watchlist wl ON wl.pcode = u.pcode
		WHERE p.prior_price > 0 AND p.price < p.prior_price AND p.captured_at >= ?
		ORDER BY (p.prior_price - p.price) * 100.0 / p.prior_price DESC`
	rows, err := s.DB().QueryContext(ctx, q, cutoff)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []dropRow{}
	for rows.Next() {
		var r dropRow
		if err := rows.Scan(&r.StockCode, &r.PCode, &r.Brand, &r.Model, &r.Grade,
			&r.Price, &r.PriorPrice, &r.CapturedAt); err != nil {
			return err
		}
		r.DropPct = (r.PriorPrice - r.Price) * 100.0 / r.PriorPrice
		if r.DropPct < minDrop {
			continue
		}
		out = append(out, r)
	}
	return printDropRows(w, flags, out)
}

// ====================================================================
// used grip-availability --size <grip>
// ====================================================================

type gripRow struct {
	Brand    string  `json:"brand"`
	Model    string  `json:"model"`
	PCode    string  `json:"pcode"`
	Grade    string  `json:"grade,omitempty"`
	GripSize string  `json:"grip_size"`
	Count    int     `json:"count"`
	MinPrice float64 `json:"min_price"`
}

func newUsedGripAvailabilityCmd(flags *rootFlags) *cobra.Command {
	var size string
	var grade string
	var brand string
	cmd := &cobra.Command{
		Use:   "grip-availability",
		Short: "Per-model count of used units in a specific grip size, optionally filtered by grade/brand",
		Long: `Wrong grip = unplayable. This command answers "which models have units in MY grip size right now?",
aggregated across the whole used inventory.`,
		Example: strings.Trim(`
  tennis-warehouse-pp-cli used grip-availability --size 4_3/8
  tennis-warehouse-pp-cli used grip-availability --size 4_3/8 --grade A --brand wilson --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if size == "" {
				if len(args) == 0 {
					return cmd.Help()
				}
				size = args[0]
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			s, err := openTennisStore(ctx)
			if err != nil {
				return err
			}
			defer s.Close()
			// Normalize grip: accept "4_3/8" or "4 3/8" or "4-3/8" → match
			// any of the stored variants by substituting underscores/spaces/hyphens to spaces.
			normalized := strings.NewReplacer("_", " ", "-", " ").Replace(strings.TrimSpace(size))
			normalized = strings.TrimSuffix(normalized, `"`)
			var conds = []string{"REPLACE(REPLACE(u.grip_size, '_', ' '), '-', ' ') = ?"}
			var argsSQL = []any{normalized}
			if grade != "" {
				conds = append(conds, "u.grade = ?")
				argsSQL = append(argsSQL, normalizeGrade(grade))
			}
			if brand != "" {
				conds = append(conds, "LOWER(m.brand) = ?")
				argsSQL = append(argsSQL, strings.ToLower(brand))
			}
			conds = append(conds, "COALESCE(u.notes,'') != 'sold_out'")
			q := `SELECT m.brand, m.model, m.pcode, u.grade, u.grip_size,
					COUNT(*) AS cnt, MIN(u.price) AS min_price
				FROM used_units u JOIN used_models m ON u.pcode = m.pcode
				WHERE ` + strings.Join(conds, " AND ") + `
				GROUP BY m.pcode, u.grade
				ORDER BY m.brand, m.model, u.grade`
			rows, err := s.DB().QueryContext(ctx, q, argsSQL...)
			if err != nil {
				return err
			}
			defer rows.Close()
			out := []gripRow{}
			for rows.Next() {
				var r gripRow
				if err := rows.Scan(&r.Brand, &r.Model, &r.PCode, &r.Grade, &r.GripSize, &r.Count, &r.MinPrice); err != nil {
					return err
				}
				out = append(out, r)
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "(no units in that grip — try a different size or 'crawl' to refresh)")
				return nil
			}
			for _, r := range out {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-10s %-9s grip=%-6s  x%-2d  from $%.2f  %s\n",
					r.PCode, r.Grade, r.GripSize, r.Count, r.MinPrice, twTruncate(r.Model, 40))
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n%d (model, grade) combinations\n", len(out))
			return nil
		},
	}
	cmd.Flags().StringVar(&size, "size", "", "Grip size (e.g. 4_3/8, 4 3/8)")
	cmd.Flags().StringVar(&grade, "grade", "", "Filter by grade")
	cmd.Flags().StringVar(&brand, "brand", "", "Filter by brand")
	return cmd
}

// ====================================================================
// used grades  (static reference)
// ====================================================================

func newUsedGradesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "grades",
		Short: "Show the Tennis Warehouse condition-grade legend",
		Long: `Display the official condition-grade definitions Tennis Warehouse uses to
classify used inventory. Curated static reference (pp:novel-static-reference) —
this content is published by Tennis Warehouse and does not change run-to-run.`,
		Example:     "  tennis-warehouse-pp-cli used grades --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), scraper.GradeLegend, flags)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Tennis Warehouse condition grades:")
			for _, g := range scraper.GradeLegend {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-8s  %s\n", g.Grade, g.Description)
			}
			return nil
		},
	}
	return cmd
}
