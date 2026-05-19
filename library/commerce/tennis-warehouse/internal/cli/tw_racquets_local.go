package cli

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/tennis-warehouse/internal/store"
)

// racquetRow mirrors the racquets table for output.
type racquetRow struct {
	SKU           string  `json:"sku"`
	Brand         string  `json:"brand"`
	Model         string  `json:"model"`
	Price         float64 `json:"price"`
	MSRP          float64 `json:"msrp,omitempty"`
	URL           string  `json:"url,omitempty"`
	HeadSizeIn2   float64 `json:"head_size_in2,omitempty"`
	StrungWeight  float64 `json:"strung_weight_oz,omitempty"`
	Swingweight   int     `json:"swingweight,omitempty"`
	Stiffness     int     `json:"stiffness,omitempty"`
	BeamWidth     string  `json:"beam_width,omitempty"`
	StringPattern string  `json:"string_pattern,omitempty"`
	LengthIn      float64 `json:"length_in,omitempty"`
	Composition   string  `json:"composition,omitempty"`
	PowerLevel    string  `json:"power_level,omitempty"`
	StrokeStyle   string  `json:"stroke_style,omitempty"`
	Status        string  `json:"status,omitempty"`
}

func openTennisStore(ctx context.Context) (*store.Store, error) {
	s, err := store.OpenWithContext(ctx, defaultDBPath("tennis-warehouse-pp-cli"))
	if err != nil {
		return nil, err
	}
	if err := s.EnsureTennisWarehouseSchema(ctx); err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}

func newRacquetsLocalListCmd(flags *rootFlags) *cobra.Command {
	var brand string
	var headSize float64
	var stringPattern string
	var maxStrungWt float64
	var maxPrice float64
	var minSwingweight, maxSwingweight int
	var status string
	var sortKey string
	var limit int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List current (new) racquets from the local store with rich filters",
		Long: `List current racquets stored locally (run 'crawl' first to populate).
Supports filters that the website itself does not expose at the catalog level
— head size, string pattern, max weight, max price, swingweight band, status.`,
		Example: strings.Trim(`
  tennis-warehouse-pp-cli racquets list --brand wilson --head-size 98
  tennis-warehouse-pp-cli racquets list --string-pattern 16x19 --max-strung-weight 11.5 --json
  tennis-warehouse-pp-cli racquets list --max-price 250 --sort price-asc --limit 20
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
			rows, err := queryRacquets(ctx, s, racquetFilter{
				Brand:          strings.ToLower(strings.TrimSpace(brand)),
				HeadSize:       headSize,
				StringPattern:  stringPattern,
				MaxStrungWt:    maxStrungWt,
				MaxPrice:       maxPrice,
				MinSwingweight: minSwingweight,
				MaxSwingweight: maxSwingweight,
				Status:         strings.ToLower(strings.TrimSpace(status)),
				Sort:           sortKey,
				Limit:          limit,
			})
			if err != nil {
				return err
			}
			return printRacquetRows(cmd.OutOrStdout(), flags, rows)
		},
	}
	cmd.Flags().StringVar(&brand, "brand", "", "Filter by brand slug (wilson, babolat, head, ...)")
	cmd.Flags().Float64Var(&headSize, "head-size", 0, "Filter by exact head size (in²)")
	cmd.Flags().StringVar(&stringPattern, "string-pattern", "", "Filter by string pattern (e.g. 16x19)")
	cmd.Flags().Float64Var(&maxStrungWt, "max-strung-weight", 0, "Max strung weight (oz)")
	cmd.Flags().Float64Var(&maxPrice, "max-price", 0, "Max asking price")
	cmd.Flags().IntVar(&minSwingweight, "min-swingweight", 0, "Min swingweight")
	cmd.Flags().IntVar(&maxSwingweight, "max-swingweight", 0, "Max swingweight")
	cmd.Flags().StringVar(&status, "status", "", "Filter by status: new, reduced, closeout")
	cmd.Flags().StringVar(&sortKey, "sort", "model", "Sort by: model, price-asc, price-desc, swingweight, strung-weight, head-size")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max rows to return")
	return cmd
}

type racquetFilter struct {
	Brand          string
	HeadSize       float64
	StringPattern  string
	MaxStrungWt    float64
	MaxPrice       float64
	MinSwingweight int
	MaxSwingweight int
	Status         string
	Sort           string
	Limit          int
}

func queryRacquets(ctx context.Context, s *store.Store, f racquetFilter) ([]racquetRow, error) {
	var conds []string
	var args []any
	if f.Brand != "" {
		conds = append(conds, "LOWER(brand) = ?")
		args = append(args, f.Brand)
	}
	if f.HeadSize > 0 {
		conds = append(conds, "head_size_in2 = ?")
		args = append(args, f.HeadSize)
	}
	if f.StringPattern != "" {
		conds = append(conds, "string_pattern = ?")
		args = append(args, f.StringPattern)
	}
	if f.MaxStrungWt > 0 {
		conds = append(conds, "strung_weight > 0 AND strung_weight <= ?")
		args = append(args, f.MaxStrungWt)
	}
	if f.MaxPrice > 0 {
		conds = append(conds, "price > 0 AND price <= ?")
		args = append(args, f.MaxPrice)
	}
	if f.MinSwingweight > 0 {
		conds = append(conds, "swingweight >= ?")
		args = append(args, f.MinSwingweight)
	}
	if f.MaxSwingweight > 0 {
		conds = append(conds, "swingweight > 0 AND swingweight <= ?")
		args = append(args, f.MaxSwingweight)
	}
	if f.Status != "" {
		conds = append(conds, "LOWER(status) = ?")
		args = append(args, f.Status)
	}
	q := `SELECT sku,brand,model,price,msrp,url,head_size_in2,strung_weight,
			swingweight,stiffness,beam_width,string_pattern,length_in,composition,
			power_level,stroke_style,status
		FROM racquets`
	if len(conds) > 0 {
		q += " WHERE " + strings.Join(conds, " AND ")
	}
	switch f.Sort {
	case "price-asc":
		q += " ORDER BY price ASC NULLS LAST"
	case "price-desc":
		q += " ORDER BY price DESC NULLS LAST"
	case "swingweight":
		q += " ORDER BY swingweight DESC NULLS LAST"
	case "strung-weight":
		q += " ORDER BY strung_weight ASC NULLS LAST"
	case "head-size":
		q += " ORDER BY head_size_in2 DESC NULLS LAST"
	default:
		q += " ORDER BY brand, model"
	}
	if f.Limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", f.Limit)
	}
	rows, err := s.DB().QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []racquetRow{}
	for rows.Next() {
		var r racquetRow
		var (
			price, msrp, head, strungWt, lengthIn             sql.NullFloat64
			swing, stiff                                      sql.NullInt64
			url, balance, beam, pattern, comp, pl, ss, status sql.NullString
			_                                                 = balance
		)
		if err := rows.Scan(&r.SKU, &r.Brand, &r.Model, &price, &msrp, &url,
			&head, &strungWt, &swing, &stiff, &beam, &pattern, &lengthIn, &comp,
			&pl, &ss, &status); err != nil {
			return nil, err
		}
		r.Price = price.Float64
		r.MSRP = msrp.Float64
		r.URL = url.String
		r.HeadSizeIn2 = head.Float64
		r.StrungWeight = strungWt.Float64
		r.Swingweight = int(swing.Int64)
		r.Stiffness = int(stiff.Int64)
		r.BeamWidth = beam.String
		r.StringPattern = pattern.String
		r.LengthIn = lengthIn.Float64
		r.Composition = comp.String
		r.PowerLevel = pl.String
		r.StrokeStyle = ss.String
		r.Status = status.String
		out = append(out, r)
	}
	return out, rows.Err()
}

func printRacquetRows(w io.Writer, flags *rootFlags, rows []racquetRow) error {
	if flags.asJSON || !isTerminal(w) {
		return printJSONFiltered(w, rows, flags)
	}
	if len(rows) == 0 {
		fmt.Fprintln(w, "(no racquets matched — run 'crawl' to populate, or relax filters)")
		return nil
	}
	for _, r := range rows {
		fmt.Fprintf(w, "%-10s %-10s %-50s  $%-7.2f  hs=%-5.0f  strung=%-5.1f  sw=%-3d  %s\n",
			r.SKU, r.Brand, twTruncate(r.Model, 50), r.Price, r.HeadSizeIn2, r.StrungWeight,
			r.Swingweight, r.StringPattern)
	}
	fmt.Fprintf(w, "\n%d racquet(s)\n", len(rows))
	return nil
}

func twTruncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func newRacquetsLocalGetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "get [sku]",
		Short:       "Show the full spec sheet for a racquet from the local store",
		Example:     "  tennis-warehouse-pp-cli racquets get WB9810",
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
			rows, err := queryRacquets(ctx, s, racquetFilter{Limit: 1})
			_ = rows
			if err != nil {
				return err
			}
			rows, err = querySingleRacquet(ctx, s, args[0])
			if err != nil {
				return err
			}
			if len(rows) == 0 {
				return fmt.Errorf("no racquet with SKU %q in local store — run 'crawl' first", args[0])
			}
			return printRacquetRows(cmd.OutOrStdout(), flags, rows)
		},
	}
	return cmd
}

func querySingleRacquet(ctx context.Context, s *store.Store, sku string) ([]racquetRow, error) {
	q := `SELECT sku,brand,model,price,msrp,url,head_size_in2,strung_weight,
			swingweight,stiffness,beam_width,string_pattern,length_in,composition,
			power_level,stroke_style,status
		FROM racquets WHERE sku = ?`
	rows, err := s.DB().QueryContext(ctx, q, sku)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []racquetRow{}
	for rows.Next() {
		var r racquetRow
		var (
			price, msrp, head, strungWt, lengthIn             sql.NullFloat64
			swing, stiff                                      sql.NullInt64
			url, balance, beam, pattern, comp, pl, ss, status sql.NullString
			_                                                 = balance
		)
		if err := rows.Scan(&r.SKU, &r.Brand, &r.Model, &price, &msrp, &url,
			&head, &strungWt, &swing, &stiff, &beam, &pattern, &lengthIn, &comp,
			&pl, &ss, &status); err != nil {
			return nil, err
		}
		r.Price = price.Float64
		r.MSRP = msrp.Float64
		r.URL = url.String
		r.HeadSizeIn2 = head.Float64
		r.StrungWeight = strungWt.Float64
		r.Swingweight = int(swing.Int64)
		r.Stiffness = int(stiff.Int64)
		r.BeamWidth = beam.String
		r.StringPattern = pattern.String
		r.LengthIn = lengthIn.Float64
		r.Composition = comp.String
		r.PowerLevel = pl.String
		r.StrokeStyle = ss.String
		r.Status = status.String
		out = append(out, r)
	}
	return out, rows.Err()
}

// newRacquetsSimilarCmd is the substitute-finder. Given a target SKU, find current racquets
// within tolerance bands across head_size, strung_weight, swingweight; require exact match
// on string_pattern; rank by aggregate spec distance.
func newRacquetsSimilarCmd(flags *rootFlags) *cobra.Command {
	var tolerance string
	var limit int
	cmd := &cobra.Command{
		Use:   "similar [sku]",
		Short: "Find current racquets whose specs are similar to the given SKU",
		Long: `Score every current racquet against the target SKU by aggregate
distance across head_size, strung_weight, and swingweight, requiring exact
match on string_pattern. Returns the top N candidates ranked by similarity.`,
		Example: strings.Trim(`
  tennis-warehouse-pp-cli racquets similar WB9810
  tennis-warehouse-pp-cli racquets similar WB9810 --tolerance tight --limit 5 --json
`, "\n"),
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
			target, err := querySingleRacquet(ctx, s, args[0])
			if err != nil {
				return err
			}
			if len(target) == 0 {
				return fmt.Errorf("no racquet with SKU %q in local store — run 'crawl' first", args[0])
			}
			t := target[0]
			if t.HeadSizeIn2 == 0 && t.StrungWeight == 0 {
				return fmt.Errorf("target %q is missing spec data — try 'crawl --brand <brand>' to refresh", args[0])
			}
			// Pull all current racquets with matching string pattern.
			q := `SELECT sku,brand,model,price,msrp,url,head_size_in2,strung_weight,
					swingweight,stiffness,beam_width,string_pattern,length_in,composition,
					power_level,stroke_style,status
				FROM racquets WHERE sku != ? AND string_pattern = ?`
			rows, err := s.DB().QueryContext(ctx, q, t.SKU, t.StringPattern)
			if err != nil {
				return err
			}
			defer rows.Close()
			var candidates []scoredRacquet
			for rows.Next() {
				var r racquetRow
				var (
					price, msrp, head, strungWt, lengthIn    sql.NullFloat64
					swing, stiff                             sql.NullInt64
					url, beam, pattern, comp, pl, ss, status sql.NullString
				)
				if err := rows.Scan(&r.SKU, &r.Brand, &r.Model, &price, &msrp, &url,
					&head, &strungWt, &swing, &stiff, &beam, &pattern, &lengthIn, &comp,
					&pl, &ss, &status); err != nil {
					return err
				}
				r.Price = price.Float64
				r.MSRP = msrp.Float64
				r.URL = url.String
				r.HeadSizeIn2 = head.Float64
				r.StrungWeight = strungWt.Float64
				r.Swingweight = int(swing.Int64)
				r.Stiffness = int(stiff.Int64)
				r.BeamWidth = beam.String
				r.StringPattern = pattern.String
				r.LengthIn = lengthIn.Float64
				r.Composition = comp.String
				r.PowerLevel = pl.String
				r.StrokeStyle = ss.String
				r.Status = status.String
				score := similarityScore(t, r)
				if !inTolerance(t, r, tolerance) {
					continue
				}
				candidates = append(candidates, scoredRacquet{racquetRow: r, Score: score})
			}
			if err := rows.Err(); err != nil {
				return err
			}
			sort.Slice(candidates, func(i, j int) bool {
				return candidates[i].Score < candidates[j].Score
			})
			if limit > 0 && len(candidates) > limit {
				candidates = candidates[:limit]
			}
			return printSimilarResults(cmd.OutOrStdout(), flags, t, candidates)
		},
	}
	cmd.Flags().StringVar(&tolerance, "tolerance", "loose", "Spec tolerance: tight (±2sq, ±0.3oz, ±10sw) or loose (±5sq, ±0.6oz, ±20sw)")
	cmd.Flags().IntVar(&limit, "limit", 10, "Max similar racquets to return")
	return cmd
}

type scoredRacquet struct {
	racquetRow
	Score float64 `json:"distance"`
}

func similarityScore(t, c racquetRow) float64 {
	// Normalized euclidean across head size, strung weight, swingweight.
	dHead := (t.HeadSizeIn2 - c.HeadSizeIn2) / 5.0
	dWt := (t.StrungWeight - c.StrungWeight) / 0.5
	dSw := float64(t.Swingweight-c.Swingweight) / 15.0
	return math.Sqrt(dHead*dHead + dWt*dWt + dSw*dSw)
}

func inTolerance(t, c racquetRow, tol string) bool {
	hs, wt, sw := 5.0, 0.6, 20
	if tol == "tight" {
		hs, wt, sw = 2.0, 0.3, 10
	}
	if t.HeadSizeIn2 > 0 && c.HeadSizeIn2 > 0 && math.Abs(t.HeadSizeIn2-c.HeadSizeIn2) > hs {
		return false
	}
	if t.StrungWeight > 0 && c.StrungWeight > 0 && math.Abs(t.StrungWeight-c.StrungWeight) > wt {
		return false
	}
	if t.Swingweight > 0 && c.Swingweight > 0 && intAbs(t.Swingweight-c.Swingweight) > sw {
		return false
	}
	return true
}

func intAbs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func printSimilarResults(w io.Writer, flags *rootFlags, target racquetRow, hits []scoredRacquet) error {
	if flags.asJSON || !isTerminal(w) {
		out := map[string]any{
			"target":  target,
			"similar": hits,
		}
		return printJSONFiltered(w, out, flags)
	}
	fmt.Fprintf(w, "Target: %s %s %s  hs=%.0f  strung=%.1foz  sw=%d  %s\n\n",
		target.SKU, target.Brand, target.Model, target.HeadSizeIn2, target.StrungWeight,
		target.Swingweight, target.StringPattern)
	if len(hits) == 0 {
		fmt.Fprintln(w, "(no similar racquets within tolerance — try --tolerance loose)")
		return nil
	}
	for _, h := range hits {
		fmt.Fprintf(w, "  %-10s %-10s %-50s  hs=%-5.0f  strung=%-5.1f  sw=%-3d  d=%.2f\n",
			h.SKU, h.Brand, twTruncate(h.Model, 50), h.HeadSizeIn2, h.StrungWeight, h.Swingweight, h.Score)
	}
	fmt.Fprintf(w, "\n%d similar racquet(s)\n", len(hits))
	return nil
}

// newRacquetsCompareCmd renders an aligned spec table for 2-5 SKUs.
func newRacquetsCompareCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "compare [sku] [sku] [sku...]",
		Short:       "Compare 2-5 racquets side-by-side across all spec fields",
		Example:     "  tennis-warehouse-pp-cli racquets compare WB9810 WB9818 WB9816 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				if len(args) == 0 {
					return cmd.Help()
				}
				return fmt.Errorf("compare needs at least 2 SKUs (got %d)", len(args))
			}
			if len(args) > 5 {
				return fmt.Errorf("compare supports at most 5 SKUs (got %d)", len(args))
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
			var found []racquetRow
			var missing []string
			for _, sku := range args {
				rs, err := querySingleRacquet(ctx, s, sku)
				if err != nil {
					return err
				}
				if len(rs) == 0 {
					missing = append(missing, sku)
				} else {
					found = append(found, rs[0])
				}
			}
			if len(missing) > 0 {
				return fmt.Errorf("not in local store (run 'crawl' first): %s", strings.Join(missing, ", "))
			}
			return printCompare(cmd.OutOrStdout(), flags, found)
		},
	}
	return cmd
}

func printCompare(w io.Writer, flags *rootFlags, rs []racquetRow) error {
	if flags.asJSON || !isTerminal(w) {
		return printJSONFiltered(w, rs, flags)
	}
	specs := []struct {
		Label string
		Get   func(r racquetRow) string
	}{
		{"Brand", func(r racquetRow) string { return r.Brand }},
		{"Model", func(r racquetRow) string { return twTruncate(r.Model, 30) }},
		{"Price", func(r racquetRow) string { return fmtPrice(r.Price) }},
		{"MSRP", func(r racquetRow) string { return fmtPrice(r.MSRP) }},
		{"HeadSize in²", func(r racquetRow) string { return fmtFloat(r.HeadSizeIn2, 0) }},
		{"Strung Wt", func(r racquetRow) string { return fmtFloat(r.StrungWeight, 1) }},
		{"Swingweight", func(r racquetRow) string { return strconv.Itoa(r.Swingweight) }},
		{"Stiffness", func(r racquetRow) string { return strconv.Itoa(r.Stiffness) }},
		{"String Pattern", func(r racquetRow) string { return r.StringPattern }},
		{"Length in", func(r racquetRow) string { return fmtFloat(r.LengthIn, 1) }},
		{"Beam Width", func(r racquetRow) string { return r.BeamWidth }},
		{"Composition", func(r racquetRow) string { return twTruncate(r.Composition, 25) }},
		{"Power", func(r racquetRow) string { return r.PowerLevel }},
		{"Stroke", func(r racquetRow) string { return r.StrokeStyle }},
	}
	// Header row.
	fmt.Fprintf(w, "%-16s", "Spec")
	for _, r := range rs {
		fmt.Fprintf(w, " | %-12s", r.SKU)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, strings.Repeat("-", 16+len(rs)*15))
	for _, sp := range specs {
		vals := make([]string, len(rs))
		for i, r := range rs {
			vals[i] = sp.Get(r)
		}
		fmt.Fprintf(w, "%-16s", sp.Label)
		for _, v := range vals {
			marker := " "
			if isDifferent(vals, v) {
				marker = "*"
			}
			fmt.Fprintf(w, " | %s%-11s", marker, twTruncate(v, 11))
		}
		fmt.Fprintln(w)
	}
	fmt.Fprintln(w, "\n(* marks values that differ from siblings)")
	return nil
}

func isDifferent(vals []string, v string) bool {
	for _, o := range vals {
		if o != v && o != "" {
			return true
		}
	}
	return false
}

func fmtPrice(v float64) string {
	if v <= 0 {
		return "-"
	}
	return fmt.Sprintf("$%.2f", v)
}
func fmtFloat(v float64, prec int) string {
	if v == 0 {
		return "-"
	}
	return strconv.FormatFloat(v, 'f', prec, 64)
}
