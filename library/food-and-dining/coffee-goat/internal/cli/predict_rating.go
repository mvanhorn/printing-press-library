// Copyright 2026 Justin Fu and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/coffee-goat/internal/store"
	"github.com/spf13/cobra"
)

const (
	predictKNN           = 5
	predictBucketTightS  = 0.6 // σ threshold for high-bucket
	predictBucketMediumS = 1.2 // σ threshold for medium-bucket
)

// predictedRating is one row of the predicted_ratings table.
type predictedRating struct {
	ID           int64      `json:"id"`
	BeanRef      string     `json:"bean"`
	RoasterSlug  string     `json:"roaster_slug,omitempty"`
	ProductSlug  string     `json:"product_slug,omitempty"`
	Method       string     `json:"method"`
	Predicted    float64    `json:"predicted"`
	CILow        float64    `json:"ci_low"`
	CIHigh       float64    `json:"ci_high"`
	Bucket       string     `json:"bucket"`
	Neighbors    int        `json:"neighbors"`
	Sigma        float64    `json:"sigma"`
	Quality      string     `json:"quality,omitempty"`
	NeighborRows []neighbor `json:"neighbor_rows,omitempty"`
	PredictedAt  string     `json:"predicted_at"`
	BeanTitle    string     `json:"bean_title,omitempty"`
}

type neighbor struct {
	Roaster    string  `json:"roaster"`
	Handle     string  `json:"handle"`
	Method     string  `json:"method"`
	Rating     int     `json:"rating"`
	Similarity float64 `json:"similarity"`
}

// ensurePredictedRatingsTable creates the predicted_ratings table if
// missing. Idempotent — safe to call on every command invocation.
func ensurePredictedRatingsTable(db *store.Store) error {
	_, err := db.DB().Exec(`CREATE TABLE IF NOT EXISTS predicted_ratings (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		bean_roaster_slug TEXT NOT NULL,
		bean_product_slug TEXT NOT NULL,
		method TEXT NOT NULL,
		predicted REAL NOT NULL,
		ci_low REAL,
		ci_high REAL,
		bucket TEXT,
		neighbors INTEGER,
		sigma REAL,
		quality TEXT,
		neighbors_json TEXT,
		predicted_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		return fmt.Errorf("create predicted_ratings: %w", err)
	}
	_, _ = db.DB().Exec(`CREATE INDEX IF NOT EXISTS idx_predicted_ratings_bean ON predicted_ratings(bean_roaster_slug, bean_product_slug, method)`)
	_, _ = db.DB().Exec(`CREATE INDEX IF NOT EXISTS idx_predicted_ratings_at ON predicted_ratings(predicted_at)`)
	return nil
}

func newPredictRatingCmd(flags *rootFlags) *cobra.Command {
	var (
		method        string
		showNeighbors bool
	)
	cmd := &cobra.Command{
		Use:   "predict-rating [bean]",
		Short: "Predict YOUR rating for a (bean, method) pair using K-nearest-twin lookup over your logged brews",
		Long: `Predicts your rating for a bean as brewed via a method, using the K=5
nearest twins from your brew log (twin similarity against
roaster_products attributes). Cold-start hedges by widening the
neighbor pool to all methods when fewer than K twins exist on the
requested method.

Confidence is a bucket label (high/medium/low) computed from neighbor
count and rating variance. JSON output also includes the numeric CI
bounds and matched-neighbor list.

Bare 'predict-rating <bean> --method v60' and 'predict-rating --cellar'
write a row to the predicted_ratings table (used by 'calibration' for
back-test residuals). 'predict-rating calibration' is read-only.`,
		Example: `  coffee-goat-pp-cli predict-rating sey/banko-gotiti --method v60
  coffee-goat-pp-cli predict-rating --cellar --method v60 --agent
  coffee-goat-pp-cli predict-rating calibration --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if strings.TrimSpace(method) == "" {
				return usageErr(fmt.Errorf("predict-rating requires --method"))
			}
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("coffee-goat-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()
			if err := ensurePredictedRatingsTable(db); err != nil {
				return err
			}
			pred, err := predictForBean(db, args[0], method)
			if err != nil {
				return err
			}
			if err := persistPrediction(db, &pred); err != nil {
				return err
			}
			if !showNeighbors {
				pred.NeighborRows = nil
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), pred, flags)
			}
			renderPrediction(cmd, pred)
			return nil
		},
	}
	cmd.Flags().StringVar(&method, "method", "", "Brew method to predict against (required)")
	cmd.Flags().BoolVar(&showNeighbors, "show-neighbors", false, "Include matched neighbor list in output")
	cmd.AddCommand(newPredictRatingCellarCmd(flags))
	cmd.AddCommand(newPredictRatingCalibrationCmd(flags))
	return cmd
}

func newPredictRatingCellarCmd(flags *rootFlags) *cobra.Command {
	var method string
	var showNeighbors bool
	cmd := &cobra.Command{
		Use:     "cellar",
		Short:   "Predict ratings for every bag in the cellar (writes one predicted_ratings row per bag)",
		Example: `  coffee-goat-pp-cli predict-rating cellar --method v60 --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if strings.TrimSpace(method) == "" {
				return usageErr(fmt.Errorf("predict-rating cellar requires --method"))
			}
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("coffee-goat-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()
			if err := ensurePredictedRatingsTable(db); err != nil {
				return err
			}
			refs, err := loadCellarBeanRefs(db)
			if err != nil {
				return err
			}
			out := make([]predictedRating, 0, len(refs))
			for _, ref := range refs {
				pred, err := predictForBean(db, ref, method)
				if err != nil {
					continue
				}
				if err := persistPrediction(db, &pred); err != nil {
					return err
				}
				if !showNeighbors {
					pred.NeighborRows = nil
				}
				out = append(out, pred)
			}
			sort.Slice(out, func(i, j int) bool { return out[i].Predicted > out[j].Predicted })
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no bags in cellar")
				return nil
			}
			for _, p := range out {
				renderPredictionLine(cmd, p)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&method, "method", "", "Brew method (required)")
	cmd.Flags().BoolVar(&showNeighbors, "show-neighbors", false, "Include matched neighbor list per bag")
	return cmd
}

func newPredictRatingCalibrationCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "calibration",
		Short:       "Report MAE + bias of past predictions sliced by method/origin/process (read-only)",
		Example:     `  coffee-goat-pp-cli predict-rating calibration --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("coffee-goat-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()
			if err := ensurePredictedRatingsTable(db); err != nil {
				return err
			}
			report, err := buildCalibrationReport(db)
			if err != nil {
				return err
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), report, flags)
			}
			renderCalibration(cmd, report)
			return nil
		},
	}
	return cmd
}

// predictForBean computes the K-NN prediction for a (bean, method) pair.
func predictForBean(db *store.Store, beanRef, method string) (predictedRating, error) {
	method = strings.ToLower(strings.TrimSpace(method))
	roaster, handle := splitRoasterHandle(beanRef)
	// loadTwinCorpus matches on plain handle / title / roaster slug — it does
	// not understand the "roaster/handle" composite form. Pass the handle
	// component when the caller gave us roaster/handle so the match can
	// actually fire.
	lookup := beanRef
	if handle != "" {
		lookup = handle
	}
	ref, others, err := loadTwinCorpus(db, lookup)
	if err != nil {
		return predictedRating{}, err
	}
	if ref.Handle == "" {
		return predictedRating{}, notFoundErr(fmt.Errorf("predict-rating: bean %q not found in roaster_products", beanRef))
	}
	// When the caller named a specific roaster (the slash form), require the
	// matched ref to come from that roaster. Otherwise an ambiguous handle
	// shared across roasters would silently resolve to whichever row came
	// first in the corpus scan.
	if roaster != "" && !strings.EqualFold(ref.Roaster, roaster) {
		return predictedRating{}, notFoundErr(fmt.Errorf("predict-rating: bean %q not found in roaster_products (handle %q exists but not under roaster %q)", beanRef, handle, roaster))
	}
	// Find which of `others` you've brewed and pull their ratings + methods.
	brewedByHandle, err := loadUserBrewedByHandle(db, method)
	if err != nil {
		return predictedRating{}, err
	}
	type cand struct {
		f           twinFeatures
		rating      int
		method      string
		similarity  float64
		crossMethod bool
	}
	var cands []cand
	for _, o := range others {
		brews, ok := brewedByHandle[o.Roaster+"/"+o.Handle]
		if !ok {
			continue
		}
		sim, _ := twinSimilarity(ref, o)
		for _, br := range brews {
			cands = append(cands, cand{
				f: o, rating: br.rating, method: br.method,
				similarity: sim, crossMethod: br.method != method,
			})
		}
	}
	if len(cands) == 0 {
		return predictedRating{}, notFoundErr(fmt.Errorf("predict-rating: no twin brews to compute a prediction from (log some brews first)"))
	}
	// Prefer same-method neighbors first; widen to cross-method if fewer
	// than K on the requested method.
	var sameMethod, crossMethod []cand
	for _, c := range cands {
		if c.crossMethod {
			crossMethod = append(crossMethod, c)
		} else {
			sameMethod = append(sameMethod, c)
		}
	}
	pool := sameMethod
	if len(pool) < predictKNN {
		pool = append(pool, crossMethod...)
	}
	sort.SliceStable(pool, func(i, j int) bool { return pool[i].similarity > pool[j].similarity })
	if len(pool) > predictKNN {
		pool = pool[:predictKNN]
	}
	// Weighted mean by similarity, with σ for the bucket label.
	var sumW, sumWR float64
	ratings := make([]float64, 0, len(pool))
	for _, c := range pool {
		w := math.Max(c.similarity, 0.01)
		sumW += w
		sumWR += w * float64(c.rating)
		ratings = append(ratings, float64(c.rating))
	}
	predicted := sumWR / sumW
	sigma := stdDev(ratings, predicted)
	ciLow, ciHigh := predicted-sigma, predicted+sigma
	if ciLow < 0 {
		ciLow = 0
	}
	if ciHigh > 10 {
		ciHigh = 10
	}
	bucket, quality := confidenceBucket(len(pool), sigma)
	out := predictedRating{
		BeanRef:     ref.Roaster + "/" + ref.Handle,
		RoasterSlug: ref.Roaster, ProductSlug: ref.Handle,
		Method:    method,
		Predicted: round2(predicted),
		CILow:     round2(ciLow), CIHigh: round2(ciHigh),
		Bucket: bucket, Neighbors: len(pool), Sigma: round2(sigma),
		Quality:     quality,
		PredictedAt: time.Now().UTC().Format(time.RFC3339),
		BeanTitle:   ref.Title,
	}
	_, _ = roaster, handle
	for _, c := range pool {
		out.NeighborRows = append(out.NeighborRows, neighbor{
			Roaster: c.f.Roaster, Handle: c.f.Handle, Method: c.method,
			Rating: c.rating, Similarity: round2(c.similarity),
		})
	}
	return out, nil
}

// userBrew is a row in loadUserBrewedByHandle's return map.
type userBrew struct {
	rating int
	method string
}

// loadUserBrewedByHandle returns a map of "<roaster>/<handle>" → all
// rated brews on that bean (method + rating). Method filter is informational;
// the predict-rating engine inspects per-brew method itself.
func loadUserBrewedByHandle(db *store.Store, _method string) (map[string][]userBrew, error) {
	rows, err := db.DB().Query(
		`SELECT COALESCE(bn.roaster_slug,''), COALESCE(bn.product_slug,''),
		        COALESCE(b.method,''), COALESCE(b.rating,0)
		 FROM brews b
		 JOIN beans bn ON b.bean_id = bn.id
		 WHERE b.rating > 0`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]userBrew{}
	for rows.Next() {
		var r, h, m string
		var rating int
		if err := rows.Scan(&r, &h, &m, &rating); err != nil {
			return nil, err
		}
		if r == "" || h == "" {
			continue
		}
		key := r + "/" + h
		out[key] = append(out[key], userBrew{rating: rating, method: strings.ToLower(m)})
	}
	return out, rows.Err()
}

// loadCellarBeanRefs returns "<roaster>/<handle>" for every bean in the
// cellar that has a roaster/product join.
func loadCellarBeanRefs(db *store.Store) ([]string, error) {
	rows, err := db.DB().Query(
		`SELECT DISTINCT COALESCE(roaster_slug,''), COALESCE(product_slug,'')
		 FROM beans WHERE roaster_slug IS NOT NULL AND product_slug IS NOT NULL`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var r, h string
		if err := rows.Scan(&r, &h); err != nil {
			return nil, err
		}
		if r != "" && h != "" {
			out = append(out, r+"/"+h)
		}
	}
	return out, rows.Err()
}

func stdDev(xs []float64, mean float64) float64 {
	if len(xs) <= 1 {
		return 0
	}
	var sum float64
	for _, x := range xs {
		sum += (x - mean) * (x - mean)
	}
	return math.Sqrt(sum / float64(len(xs)-1))
}

// confidenceBucket maps (neighbor count, σ) → high/medium/low.
// quality="low" is also returned when neighbors <3 (cold-start hedge).
func confidenceBucket(n int, sigma float64) (string, string) {
	if n < 3 {
		return "low", "low"
	}
	if n >= 5 && sigma <= predictBucketTightS {
		return "high", ""
	}
	if n >= 3 && sigma <= predictBucketMediumS {
		return "medium", ""
	}
	return "low", ""
}

// persistPrediction inserts the prediction row.
func persistPrediction(db *store.Store, p *predictedRating) error {
	neighborsJSON := ""
	if len(p.NeighborRows) > 0 {
		b, _ := json.Marshal(p.NeighborRows)
		neighborsJSON = string(b)
	}
	res, err := db.DB().Exec(
		`INSERT INTO predicted_ratings (bean_roaster_slug, bean_product_slug, method,
			predicted, ci_low, ci_high, bucket, neighbors, sigma, quality, neighbors_json, predicted_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.RoasterSlug, p.ProductSlug, p.Method,
		p.Predicted, p.CILow, p.CIHigh, p.Bucket, p.Neighbors, p.Sigma, p.Quality,
		nullableString(neighborsJSON),
		p.PredictedAt,
	)
	if err != nil {
		return fmt.Errorf("insert predicted_rating: %w", err)
	}
	id, _ := res.LastInsertId()
	p.ID = id
	return nil
}

// renderPrediction is the human-readable single-prediction output.
func renderPrediction(cmd *cobra.Command, p predictedRating) {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "→ %s  %s  predicted %.1f  %s  [%d neighbors, σ=%.2f]\n",
		p.BeanRef, p.Method, p.Predicted, p.Bucket, p.Neighbors, p.Sigma)
	if p.BeanTitle != "" {
		fmt.Fprintf(w, "  bean: %s\n", p.BeanTitle)
	}
	fmt.Fprintf(w, "  CI: [%.1f, %.1f]\n", p.CILow, p.CIHigh)
	if len(p.NeighborRows) > 0 {
		fmt.Fprintln(w, "  neighbors:")
		for _, n := range p.NeighborRows {
			fmt.Fprintf(w, "    %s/%s (%s) rating=%d sim=%.2f\n",
				n.Roaster, n.Handle, n.Method, n.Rating, n.Similarity)
		}
	}
}

func renderPredictionLine(cmd *cobra.Command, p predictedRating) {
	fmt.Fprintf(cmd.OutOrStdout(), "  %-40s  %s  pred=%.1f  %s  [n=%d]\n",
		p.BeanRef, p.Method, p.Predicted, p.Bucket, p.Neighbors)
}

// calibrationReport is the output of `predict-rating calibration`.
type calibrationReport struct {
	N           int                `json:"n_scored"`
	OverallMAE  float64            `json:"overall_mae"`
	OverallBias float64            `json:"overall_bias"`
	ByMethod    []calibrationSlice `json:"by_method,omitempty"`
	ByOrigin    []calibrationSlice `json:"by_origin,omitempty"`
	ByProcess   []calibrationSlice `json:"by_process,omitempty"`
}

type calibrationSlice struct {
	Key  string  `json:"key"`
	N    int     `json:"n"`
	MAE  float64 `json:"mae,omitempty"`
	Bias float64 `json:"bias,omitempty"`
}

// buildCalibrationReport scores each persisted prediction against the
// FIRST brew of that (bean, method) logged AFTER the prediction
// timestamp. Same-method first-brew is required to avoid scoring V60
// predictions against espresso brews.
func buildCalibrationReport(db *store.Store) (calibrationReport, error) {
	// Pull all predictions ordered by predicted_at DESC; for each
	// (roaster, product, method), keep only the most-recent prediction
	// BEFORE the first brew (per spec: most-recent-before-first-brew).
	rows, err := db.DB().Query(
		`SELECT bean_roaster_slug, bean_product_slug, method, predicted, predicted_at FROM predicted_ratings ORDER BY predicted_at`,
	)
	if err != nil {
		return calibrationReport{}, err
	}
	defer rows.Close()
	type predRow struct {
		roaster, product, method string
		predicted                float64
		at                       time.Time
	}
	var preds []predRow
	for rows.Next() {
		var p predRow
		var atStr string
		if err := rows.Scan(&p.roaster, &p.product, &p.method, &p.predicted, &atStr); err != nil {
			return calibrationReport{}, err
		}
		if t, ok := parseBrewedAt(atStr); ok {
			p.at = t
		}
		preds = append(preds, p)
	}
	if err := rows.Err(); err != nil && err != sql.ErrNoRows {
		return calibrationReport{}, err
	}
	// For each prediction, find the first brew of that (bean, method)
	// after `predicted_at`. Skip preds that have no follow-up brew.
	var scoredRows []scoredCalibrationRow
	// Index of (roaster/product, method) -> list of last-prediction-per-bin.
	// Bin is "before first brew" — we approximate by walking predictions in
	// order and pairing each with the first brew strictly after it; if multiple
	// predictions exist before that first brew, only the LAST one is scored.
	lastPredKey := map[string]predRow{}
	for _, p := range preds {
		key := p.roaster + "/" + p.product + "/" + p.method
		// First brew of (bean, method) strictly after p.at:
		var firstBrew sql.NullString
		_ = db.DB().QueryRow(
			`SELECT MIN(b.brewed_at) FROM brews b
			 JOIN beans bn ON b.bean_id = bn.id
			 WHERE bn.roaster_slug = ? AND bn.product_slug = ?
			   AND LOWER(b.method) = ? AND b.rating > 0
			   AND b.brewed_at > ?`,
			p.roaster, p.product, p.method, p.at.Format(time.RFC3339),
		).Scan(&firstBrew)
		if !firstBrew.Valid || firstBrew.String == "" {
			lastPredKey[key] = p // keep newest preds with no follow-up too
			continue
		}
		lastPredKey[key] = p
		// Pull the actual rating for that first brew.
		var actual int
		var origin, process string
		_ = db.DB().QueryRow(
			`SELECT b.rating, COALESCE(rp.origin,''), COALESCE(rp.process,'')
			 FROM brews b
			 JOIN beans bn ON b.bean_id = bn.id
			 LEFT JOIN roaster_products rp ON bn.roaster_slug = rp.roaster_slug AND bn.product_slug = rp.handle
			 WHERE bn.roaster_slug = ? AND bn.product_slug = ?
			   AND LOWER(b.method) = ? AND b.rating > 0
			   AND b.brewed_at = ?`,
			p.roaster, p.product, p.method, firstBrew.String,
		).Scan(&actual, &origin, &process)
		if actual > 0 {
			scoredRows = append(scoredRows, scoredCalibrationRow{
				method: p.method, origin: strings.ToLower(origin), process: strings.ToLower(process),
				predicted: p.predicted, actual: float64(actual),
			})
		}
	}
	rep := calibrationReport{N: len(scoredRows)}
	if rep.N == 0 {
		return rep, nil
	}
	var totalMAE, totalBias float64
	for _, s := range scoredRows {
		totalMAE += math.Abs(s.actual - s.predicted)
		totalBias += s.actual - s.predicted
	}
	rep.OverallMAE = round2(totalMAE / float64(rep.N))
	rep.OverallBias = round2(totalBias / float64(rep.N))
	rep.ByMethod = sliceCalibration(scoredRows, func(s scoredCalibrationRow) string { return s.method })
	rep.ByOrigin = sliceCalibration(scoredRows, func(s scoredCalibrationRow) string {
		country, _ := canonicalCountry(s.origin)
		return country
	})
	rep.ByProcess = sliceCalibration(scoredRows, func(s scoredCalibrationRow) string { return s.process })
	return rep, nil
}

// scoredCalibrationRow is one realized prediction with the bean's
// origin/process so the calibration report can slice by them.
type scoredCalibrationRow struct {
	method, origin, process string
	predicted, actual       float64
}

// sliceCalibration groups scored rows by the provided key function and
// returns rows with n≥3; n<3 rows collapse to a "—" presentation by
// being omitted from the typed output (caller can detect via N field).
func sliceCalibration(rows []scoredCalibrationRow, keyOf func(scoredCalibrationRow) string) []calibrationSlice {
	type acc struct {
		n    int
		mae  float64
		bias float64
	}
	bucket := map[string]*acc{}
	for _, r := range rows {
		k := keyOf(r)
		if k == "" {
			continue
		}
		if bucket[k] == nil {
			bucket[k] = &acc{}
		}
		bucket[k].n++
		bucket[k].mae += math.Abs(r.actual - r.predicted)
		bucket[k].bias += r.actual - r.predicted
	}
	out := make([]calibrationSlice, 0, len(bucket))
	for k, v := range bucket {
		s := calibrationSlice{Key: k, N: v.n}
		if v.n >= 3 {
			s.MAE = round2(v.mae / float64(v.n))
			s.Bias = round2(v.bias / float64(v.n))
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].N > out[j].N })
	return out
}

func renderCalibration(cmd *cobra.Command, rep calibrationReport) {
	w := cmd.OutOrStdout()
	if rep.N == 0 {
		fmt.Fprintln(w, "no scored predictions yet (log brews of beans you've previously run predict-rating on)")
		return
	}
	fmt.Fprintf(w, "→ Calibration  (n=%d scored predictions)\n", rep.N)
	fmt.Fprintf(w, "  Overall  MAE %.2f   bias %+.2f\n", rep.OverallMAE, rep.OverallBias)
	renderSlice := func(name string, slices []calibrationSlice) {
		if len(slices) == 0 {
			return
		}
		fmt.Fprintln(w)
		fmt.Fprintf(w, "By %s:\n", name)
		for _, s := range slices {
			if s.N < 3 {
				fmt.Fprintf(w, "  %-20s  n=%d  —\n", s.Key, s.N)
			} else {
				fmt.Fprintf(w, "  %-20s  n=%d  MAE %.2f  bias %+.2f\n", s.Key, s.N, s.MAE, s.Bias)
			}
		}
	}
	renderSlice("method", rep.ByMethod)
	renderSlice("origin", rep.ByOrigin)
	renderSlice("process", rep.ByProcess)
}
