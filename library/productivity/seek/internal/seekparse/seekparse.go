// Copyright 2026 Paul Siola and contributors. Licensed under Apache-2.0. See LICENSE.

// Package seekparse holds pure parsing and statistics helpers for the SEEK
// novel commands (salary distributions, hiring trends, saved-search replay).
// It never touches the network or the local store.
package seekparse

import (
	"math"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// SalaryRange is a parsed pay range in whole dollars per year (annualised).
// Ok is false when a listing's salary text carried no usable numbers
// ("Competitive", "Attractive package", empty).
type SalaryRange struct {
	Min      float64 `json:"min"`
	Max      float64 `json:"max"`
	Period   string  `json:"period"` // "year" | "hour" | "day" | "month"
	Ok       bool    `json:"ok"`
	Original string  `json:"original,omitempty"`
}

var (
	moneyRe    = regexp.MustCompile(`(?i)\$?\s*([0-9][0-9.,]*)\s*(k|m)?`)
	perHourRe  = regexp.MustCompile(`(?i)per\s*hour|/\s*h(ou)?r|p\.?h\.?|hourly|/hr`)
	perDayRe   = regexp.MustCompile(`(?i)per\s*day|/\s*day|daily`)
	perMonthRe = regexp.MustCompile(`(?i)per\s*month|/\s*month|monthly|p\.?m\.?`)
)

// hoursPerYear / daysPerYear / monthsPerYear annualise non-yearly rates so
// every parsed listing lands on one comparable axis.
const (
	hoursPerYear  = 38 * 52 // 1976, SEEK's standard full-time week
	daysPerYear   = 5 * 52  // 260
	monthsPerYear = 12

	minPlausibleAnnual = 15000   // below AU full-time minimum wage
	maxPlausibleAnnual = 2000000 // executive ceiling; above this is a parse artifact
)

// ParseSalary turns a SEEK salaryLabel ("$120,000 – $140,000", "$55 per hour",
// "Up to $90k + super") into an annualised range. It is deliberately lenient:
// a single figure becomes Min==Max, and a "k"/"m" suffix or a bare thousands
// figure ("120" -> 120000) is expanded.
func ParseSalary(label string) SalaryRange {
	out := SalaryRange{Period: "year", Original: strings.TrimSpace(label)}
	s := strings.ToLower(label)
	if strings.TrimSpace(s) == "" {
		return out
	}

	switch {
	case perHourRe.MatchString(s):
		out.Period = "hour"
	case perDayRe.MatchString(s):
		out.Period = "day"
	case perMonthRe.MatchString(s):
		out.Period = "month"
	}

	var nums []float64
	for _, m := range moneyRe.FindAllStringSubmatch(s, -1) {
		raw := strings.ReplaceAll(m[1], ",", "")
		raw = strings.TrimRight(raw, ".")
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil || v == 0 {
			continue
		}
		switch strings.ToLower(m[2]) {
		case "k":
			v *= 1000
		case "m":
			v *= 1_000_000
		default:
			// A bare "120" or "140" in a yearly context is thousands.
			if out.Period == "year" && v < 1000 {
				v *= 1000
			}
		}
		nums = append(nums, v)
	}
	if len(nums) == 0 {
		return out
	}
	sort.Float64s(nums)
	lo, hi := nums[0], nums[len(nums)-1]

	factor := 1.0
	switch out.Period {
	case "hour":
		factor = hoursPerYear
	case "day":
		factor = daysPerYear
	case "month":
		factor = monthsPerYear
	}
	out.Min = math.Round(lo * factor)
	out.Max = math.Round(hi * factor)
	out.Period = "year"
	// Guard against a stray superannuation percentage ("+ 11%") sneaking in
	// as a tiny number next to a real salary.
	if out.Min < 5000 {
		out.Min = out.Max
	}
	// Domain sanity clamp: a parsed figure outside a plausible annual-salary
	// band is almost always a phone number, ABN, packaging cap, or a
	// mis-multiplied hourly rate. Drop the whole listing rather than let it
	// skew a distribution.
	if out.Min < minPlausibleAnnual || out.Max > maxPlausibleAnnual || out.Max <= 0 {
		return out
	}
	out.Ok = true
	return out
}

// Midpoint is the middle of a parsed range, used as the single figure a
// listing contributes to a distribution.
func (r SalaryRange) Midpoint() float64 {
	if !r.Ok {
		return 0
	}
	return (r.Min + r.Max) / 2
}

// Percentile returns the linear-interpolated pth percentile (0-100) of a
// value set. The input is copied, not mutated.
func Percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	v := append([]float64(nil), values...)
	sort.Float64s(v)
	if p <= 0 {
		return v[0]
	}
	if p >= 100 {
		return v[len(v)-1]
	}
	rank := (p / 100) * float64(len(v)-1)
	lo := int(math.Floor(rank))
	hi := int(math.Ceil(rank))
	if lo == hi {
		return v[lo]
	}
	frac := rank - float64(lo)
	return v[lo] + frac*(v[hi]-v[lo])
}

// Winsorize returns a copy of values with everything below the loPct percentile
// and above the hiPct percentile clamped to those percentile values. It keeps
// the sample size constant while stopping a handful of extreme outliers
// (exec packages, data-entry errors) from stretching a histogram or skewing a
// high percentile. Fewer than 5 values are returned unchanged.
func Winsorize(values []float64, loPct, hiPct float64) []float64 {
	if len(values) < 5 {
		return append([]float64(nil), values...)
	}
	lo := Percentile(values, loPct)
	hi := Percentile(values, hiPct)
	out := make([]float64, len(values))
	for i, v := range values {
		switch {
		case v < lo:
			out[i] = lo
		case v > hi:
			out[i] = hi
		default:
			out[i] = v
		}
	}
	return out
}

// Histogram buckets values into `buckets` equal-width bins between the min and
// max, returning bin lower bounds and counts.
func Histogram(values []float64, buckets int) (bounds []float64, counts []int) {
	if len(values) == 0 || buckets < 1 {
		return nil, nil
	}
	v := append([]float64(nil), values...)
	sort.Float64s(v)
	lo, hi := v[0], v[len(v)-1]
	if hi <= lo {
		return []float64{lo}, []int{len(v)}
	}
	width := (hi - lo) / float64(buckets)
	bounds = make([]float64, buckets)
	counts = make([]int, buckets)
	for i := 0; i < buckets; i++ {
		bounds[i] = math.Round(lo + float64(i)*width)
	}
	for _, x := range v {
		idx := int((x - lo) / width)
		if idx >= buckets {
			idx = buckets - 1
		}
		counts[idx]++
	}
	return bounds, counts
}

// SavedSearchParams is the flattened, wire-ready query for one SEEK saved
// search, ready to hand to /api/jobsearch/v5/search.
type SavedSearchParams struct {
	Keywords          string `json:"keywords,omitempty"`
	Where             string `json:"where,omitempty"`
	Classification    string `json:"classification,omitempty"`
	Subclassification string `json:"subclassification,omitempty"`
	Worktype          string `json:"worktype,omitempty"`
	Salaryrange       string `json:"salaryrange,omitempty"`
	Salarytype        string `json:"salarytype,omitempty"`
	Workarrangement   string `json:"workarrangement,omitempty"`
	SiteKey           string `json:"siteKey,omitempty"`
}

// ParseSavedSearchQuery accepts the `query` value SEEK stores on a saved
// search. SEEK has shipped this as a leading-`?` query string
// ("?keywords=nurse&where=...") and, on newer accounts, as a path
// ("/nurse-jobs/in-Melbourne-VIC"). Both shapes are handled; unknown shapes
// return an empty struct rather than erroring so `me new-jobs` can skip one
// bad row without failing the whole run.
func ParseSavedSearchQuery(q string) SavedSearchParams {
	var p SavedSearchParams
	q = strings.TrimSpace(q)
	if q == "" {
		return p
	}
	// Query-string shape.
	if strings.Contains(q, "=") {
		raw := strings.TrimPrefix(q, "?")
		if i := strings.Index(raw, "?"); i >= 0 {
			raw = raw[i+1:]
		}
		vals, err := url.ParseQuery(raw)
		if err == nil {
			get := func(keys ...string) string {
				for _, k := range keys {
					if v := vals.Get(k); v != "" {
						return v
					}
				}
				return ""
			}
			p.Keywords = get("keywords", "q")
			p.Where = get("where", "location", "locationId")
			p.Classification = get("classification", "classificationId")
			p.Subclassification = get("subclassification", "subclassificationId")
			p.Worktype = get("worktype", "workType", "worktypeId")
			p.Salaryrange = get("salaryrange", "salaryRange")
			p.Salarytype = get("salarytype", "salaryType")
			p.Workarrangement = get("workarrangement", "workArrangement")
			p.SiteKey = get("siteKey", "sitekey")
			return p
		}
	}
	// SEO path shape: "/<keywords>-jobs/in-<Where>".
	if strings.HasPrefix(q, "/") {
		seg := strings.Trim(q, "/")
		parts := strings.SplitN(seg, "/", 2)
		if kw := strings.TrimSuffix(parts[0], "-jobs"); kw != "" && kw != parts[0] {
			p.Keywords = strings.ReplaceAll(kw, "-", " ")
		}
		if len(parts) == 2 {
			w := strings.TrimPrefix(parts[1], "in-")
			p.Where = strings.ReplaceAll(w, "-", " ")
		}
	}
	return p
}
