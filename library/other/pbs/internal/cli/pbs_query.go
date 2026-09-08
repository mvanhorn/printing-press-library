// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"sort"
	"strings"
)

// cityObs is one city's observation of an item in one release.
type cityObs struct {
	City  string  `json:"city"`
	Code  string  `json:"city_code,omitempty"`
	Value float64 `json:"value"`
}

// cellCensus reports how many cells were excluded from an aggregate and why.
//
// Every panel command prints this. An average over 14 of 17 cities is a
// different number from an average over 17, and a caller cannot tell which one
// they were handed unless the exclusions are stated.
type cellCensus struct {
	Present     int `json:"present"`
	Zero        int `json:"excluded_zero"`
	Blank       int `json:"excluded_blank"`
	NA          int `json:"excluded_na"`
	Unparseable int `json:"excluded_unparseable"`
}

// Excluded totals the cells that carried no usable number.
func (c cellCensus) Excluded() int { return c.Zero + c.Blank + c.NA + c.Unparseable }

// resolveItem maps a user-supplied item name to the stored description.
//
// Matching is exact first, then a case-insensitive contains. A query that
// matches several items is an error rather than a silent pick, because
// silently choosing one of "Vegetable Ghee ... 2.5 kg Tin" and
// "Vegetable Ghee ... 1 kg Pouch" would answer a different question than asked.
func resolveItem(ctx context.Context, db *sql.DB, q string) (string, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return "", fmt.Errorf("item name is required")
	}
	var exact string
	err := db.QueryRowContext(ctx,
		`SELECT item_desc FROM pbs_price WHERE item_desc = ? LIMIT 1`, q).Scan(&exact)
	if err == nil {
		return exact, nil
	}
	if err != sql.ErrNoRows {
		return "", fmt.Errorf("resolve item: %w", err)
	}
	rows, err := db.QueryContext(ctx,
		`SELECT DISTINCT item_desc FROM pbs_price WHERE item_desc LIKE ? ESCAPE '\' ORDER BY item_desc`,
		"%"+likeEscape(q)+"%")
	if err != nil {
		return "", fmt.Errorf("resolve item: %w", err)
	}
	var cands []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			_ = rows.Close()
			return "", err
		}
		cands = append(cands, d)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return "", err
	}
	if err := rows.Close(); err != nil {
		return "", err
	}
	switch len(cands) {
	case 0:
		// Name what IS available rather than pointing at a command that does not
		// exist. An empty panel is handled by the caller before reaching here.
		avail, _ := storedItemSample(ctx, db, 8)
		if len(avail) == 0 {
			return "", fmt.Errorf("no item matches %q and the local panel holds no items; run: pbs-pp-cli sync --max-releases 4", q)
		}
		return "", fmt.Errorf("no item matches %q. stored items include:\n  %s",
			q, strings.Join(avail, "\n  "))
	case 1:
		return cands[0], nil
	default:
		return "", fmt.Errorf("%q matches %d items, which would answer a different question than asked; be more specific:\n  %s",
			q, len(cands), strings.Join(cands, "\n  "))
	}
}

// storedItemSample returns up to n stored item descriptions, for error messages
// that tell a caller what they can actually ask for.
func storedItemSample(ctx context.Context, db *sql.DB, n int) ([]string, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT DISTINCT item_desc FROM pbs_price ORDER BY item_desc LIMIT ?`, n)
	if err != nil {
		return nil, err
	}
	var out []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			_ = rows.Close()
			return out, err
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return out, err
	}
	if err := rows.Close(); err != nil {
		return out, err
	}
	return out, nil
}

func likeEscape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "%", `\%`)
	return strings.ReplaceAll(s, "_", `\_`)
}

// resolveCity maps a user-supplied city name to the stored key.
//
// It accepts the bare name, the soft-hyphenated spelling PBS uses in
// Appendix-B, and the two-digit city code.
func resolveCity(ctx context.Context, db *sql.DB, q string) (string, error) {
	q = strings.TrimSpace(strings.ToLower(q))
	if q == "" {
		return "", fmt.Errorf("city name is required")
	}
	norm := strings.ReplaceAll(q, "-", "")
	rows, err := db.QueryContext(ctx, `SELECT DISTINCT city, city_code FROM pbs_price`)
	if err != nil {
		return "", fmt.Errorf("resolve city: %w", err)
	}
	var match []string
	for rows.Next() {
		var city string
		var code sql.NullString
		if err := rows.Scan(&city, &code); err != nil {
			_ = rows.Close()
			return "", err
		}
		if city == q || city == norm || (code.Valid && code.String == q) {
			_ = rows.Close()
			return city, nil
		}
		if strings.Contains(city, norm) {
			match = append(match, city)
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return "", err
	}
	if err := rows.Close(); err != nil {
		return "", err
	}
	if len(match) == 1 {
		return match[0], nil
	}
	if len(match) > 1 {
		sort.Strings(match)
		return "", fmt.Errorf("%q matches %d cities: %s", q, len(match), strings.Join(match, ", "))
	}
	return "", fmt.Errorf("no city matches %q", q)
}

// normStat validates a statistic name.
func normStat(s string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "avg", "average", "mean":
		return "avg", nil
	case "min", "minimum":
		return "min", nil
	case "max", "maximum":
		return "max", nil
	}
	return "", fmt.Errorf("--stat must be min, avg or max, got %q", s)
}

// latestAsOf returns the newest stored release date for a surface.
func latestAsOf(ctx context.Context, db *sql.DB) (string, error) {
	var d sql.NullString
	if err := db.QueryRowContext(ctx,
		`SELECT MAX(as_of) FROM pbs_price WHERE surface = 'appendix-a'`).Scan(&d); err != nil {
		return "", fmt.Errorf("find latest release: %w", err)
	}
	if !d.Valid || d.String == "" {
		return "", nil
	}
	return d.String, nil
}

// cityObsFor loads every city's observation of one item, statistic and release,
// separating usable values from each kind of exclusion.
func cityObsFor(ctx context.Context, db *sql.DB, item, stat, asOf string) ([]cityObs, cellCensus, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT city, COALESCE(city_code, ''), value, value_state
		FROM pbs_price
		WHERE surface = 'appendix-a' AND item_desc = ? AND stat = ? AND as_of = ?
		ORDER BY city`, item, stat, asOf)
	if err != nil {
		return nil, cellCensus{}, fmt.Errorf("query prices: %w", err)
	}
	var obs []cityObs
	var cen cellCensus
	for rows.Next() {
		var city, code, state string
		var val sql.NullFloat64
		if err := rows.Scan(&city, &code, &val, &state); err != nil {
			_ = rows.Close()
			return nil, cen, fmt.Errorf("scan price: %w", err)
		}
		switch state {
		case "present":
			if val.Valid {
				obs = append(obs, cityObs{City: city, Code: code, Value: val.Float64})
				cen.Present++
			} else {
				cen.Unparseable++
			}
		case "zero":
			cen.Zero++
		case "blank":
			cen.Blank++
		case "na":
			cen.NA++
		default:
			cen.Unparseable++
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, cen, fmt.Errorf("iterate prices: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, cen, err
	}
	return obs, cen, nil
}

// dispersion summarises a cross-sectional distribution.
type dispersion struct {
	Cities   int      `json:"contributing_cities"`
	Min      float64  `json:"min"`
	MinCity  string   `json:"min_city"`
	Max      float64  `json:"max"`
	MaxCity  string   `json:"max_city"`
	Range    float64  `json:"range"`
	Mean     float64  `json:"mean"`
	Median   float64  `json:"median"`
	IQR      float64  `json:"iqr"`
	StdDev   float64  `json:"std_dev"`
	CV       float64  `json:"cv_pct"`
	Outliers []string `json:"tukey_outlier_cities,omitempty"`
}

// summarise computes the dispersion of a set of observations.
//
// Outliers are FLAGGED with Tukey fences and never removed: on this data an
// extreme city price is usually a real regional dislocation, which is the
// signal rather than the noise.
func summarise(obs []cityObs) dispersion {
	d := dispersion{Cities: len(obs)}
	if len(obs) == 0 {
		return d
	}
	vals := make([]float64, 0, len(obs))
	d.Min, d.Max = obs[0].Value, obs[0].Value
	d.MinCity, d.MaxCity = obs[0].City, obs[0].City
	var sum float64
	for _, o := range obs {
		vals = append(vals, o.Value)
		sum += o.Value
		if o.Value < d.Min {
			d.Min, d.MinCity = o.Value, o.City
		}
		if o.Value > d.Max {
			d.Max, d.MaxCity = o.Value, o.City
		}
	}
	sort.Float64s(vals)
	d.Range = d.Max - d.Min
	d.Mean = sum / float64(len(vals))
	d.Median = quantile(vals, 0.5)
	q1, q3 := quantile(vals, 0.25), quantile(vals, 0.75)
	d.IQR = q3 - q1
	var ss float64
	for _, v := range vals {
		ss += (v - d.Mean) * (v - d.Mean)
	}
	if len(vals) > 1 {
		d.StdDev = math.Sqrt(ss / float64(len(vals)-1))
	}
	if d.Mean != 0 {
		d.CV = d.StdDev / d.Mean * 100
	}
	lo, hi := q1-1.5*d.IQR, q3+1.5*d.IQR
	for _, o := range obs {
		if o.Value < lo || o.Value > hi {
			d.Outliers = append(d.Outliers, o.City)
		}
	}
	sort.Strings(d.Outliers)
	return d
}

// quantile returns a linear-interpolated quantile of sorted data.
func quantile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	pos := p * float64(len(sorted)-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	if lo == hi {
		return sorted[lo]
	}
	frac := pos - float64(lo)
	return sorted[lo]*(1-frac) + sorted[hi]*frac
}

// storedAsOfRange lists the stored release dates in a window, oldest first.
func storedAsOfRange(ctx context.Context, db *sql.DB, item, stat, from, to string) ([]string, error) {
	q := `SELECT DISTINCT as_of FROM pbs_price
	      WHERE surface = 'appendix-a' AND item_desc = ? AND stat = ?`
	args := []any{item, stat}
	if from != "" {
		q += ` AND as_of >= ?`
		args = append(args, from)
	}
	if to != "" {
		q += ` AND as_of <= ?`
		args = append(args, to)
	}
	q += ` ORDER BY as_of`
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query release dates: %w", err)
	}
	var out []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			_ = rows.Close()
			return nil, err
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	return out, nil
}

// spearmanFromValues returns Spearman's rho between two observation sets.
//
// Ranks are recomputed over the INTERSECTION of the two city sets. Ranking each
// release independently over only its present cities and then differencing those
// ranks is wrong whenever the sets differ: comparing ranks drawn from 1..17
// against ranks drawn from 1..3 put rho far outside [-1, 1] (a measured -146),
// which is not a correlation at all.
//
// Tied values share the midpoint rank, so a week in which several cities report
// the same price does not manufacture an ordering.
func spearmanFromValues(a, b map[string]float64) (rho float64, n int) {
	var keys []string
	for k := range a {
		if _, ok := b[k]; ok {
			keys = append(keys, k)
		}
	}
	n = len(keys)
	if n < 3 {
		return 0, n
	}
	ra := tiedRanks(a, keys)
	rb := tiedRanks(b, keys)
	var d2 float64
	for _, k := range keys {
		d := ra[k] - rb[k]
		d2 += d * d
	}
	nf := float64(n)
	return 1 - (6*d2)/(nf*(nf*nf-1)), n
}

// tiedRanks ranks the named keys by their value, averaging ranks across ties.
func tiedRanks(vals map[string]float64, keys []string) map[string]float64 {
	ordered := append([]string{}, keys...)
	sort.Slice(ordered, func(i, j int) bool { return vals[ordered[i]] < vals[ordered[j]] })
	out := make(map[string]float64, len(ordered))
	for i := 0; i < len(ordered); {
		j := i
		for j+1 < len(ordered) && vals[ordered[j+1]] == vals[ordered[i]] {
			j++
		}
		// Ranks are 1-based; a tied run from i..j shares their average.
		avg := (float64(i+1) + float64(j+1)) / 2
		for k := i; k <= j; k++ {
			out[ordered[k]] = avg
		}
		i = j + 1
	}
	return out
}
