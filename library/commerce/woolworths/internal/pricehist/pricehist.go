// Copyright 2026 Richard Gill and contributors. Licensed under Apache-2.0. See LICENSE.

// Package pricehist holds the pure arithmetic behind the Woolworths
// price-history commands (real-special, cycle). Woolworths publishes only
// "now" — every judgement this CLI makes about whether a special is real is
// derived from locally recorded observations, so the derivation must be
// separately testable without a database or a network.
//
// Nothing in this package touches SQLite, the network, or cobra.
package pricehist

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode"
)

// Point is one recorded sighting of a product's price, reduced to the fields
// the analysis actually uses.
type Point struct {
	ObservedAt  int64
	Price       float64
	WasPrice    float64
	IsOnSpecial bool
	IsHalfPrice bool
}

// Verdict labels emitted by Classify. They are deliberately shouty and
// hyphenated so a grep or an agent's string match cannot confuse two of them.
const (
	// VerdictNoHistory means the local mirror is too thin to say anything.
	// It is the honest answer, never a fallback for "we could not decide".
	VerdictNoHistory = "NO-HISTORY"
	// VerdictGenuine means the current price is at/near the local minimum and
	// materially below the local median.
	VerdictGenuine = "GENUINE"
	// VerdictRecycled means this exact price has shown up repeatedly before:
	// the "special" is just the normal cycle low coming round again.
	VerdictRecycled = "RECYCLED"
	// VerdictWasInflated means the advertised was-price sits above what the
	// product actually sold at, so the advertised saving is measured from a
	// number the shelf rarely showed.
	VerdictWasInflated = "WAS-PRICE-INFLATED"
	// VerdictOrdinary means there is enough history to judge and the current
	// price is simply not remarkable. Emitting a real label here keeps the
	// four loaded verdicts meaningful instead of turning one into a dustbin.
	VerdictOrdinary = "ORDINARY"
)

// MinObservationsForVerdict is the number of PRIOR observations required
// before Classify will commit to anything other than VerdictNoHistory.
const MinObservationsForVerdict = 3

const (
	// priceEpsilon is half a cent: prices arrive as float64 dollars, so exact
	// equality is not safe.
	priceEpsilon = 0.005
	// inflationRatio: a was-price must exceed the local median by at least
	// this factor before it counts as inflated.
	inflationRatio = 1.10
	// inflationShare: ...and at most this share of observations may ever have
	// reached that was-price. A was-price the product genuinely sold at for
	// months is not inflated, however big the gap to today's median.
	inflationShare = 0.25
	// nearMinRatio: "at or near the local minimum" tolerance.
	nearMinRatio = 1.02
	// genuineMedianRatio: "materially below the median" threshold.
	genuineMedianRatio = 0.90
	// recycledRepeats: how many prior observations must sit at this exact
	// price before the low counts as recycled rather than new.
	recycledRepeats = 2
)

const secondsPerDay = 86400.0

// Prices projects the price column out of a slice of points.
func Prices(points []Point) []float64 {
	out := make([]float64, 0, len(points))
	for _, p := range points {
		out = append(out, p.Price)
	}
	return out
}

// Median returns the median of vals. Returns 0 for an empty slice; it does not
// mutate the caller's slice.
func Median(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sorted := append([]float64(nil), vals...)
	sort.Float64s(sorted)
	mid := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[mid]
	}
	return (sorted[mid-1] + sorted[mid]) / 2
}

// Min returns the smallest value, or 0 for an empty slice.
func Min(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	m := vals[0]
	for _, v := range vals[1:] {
		if v < m {
			m = v
		}
	}
	return m
}

// Max returns the largest value, or 0 for an empty slice.
func Max(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	m := vals[0]
	for _, v := range vals[1:] {
		if v > m {
			m = v
		}
	}
	return m
}

// Round2 rounds to cents. Reported figures go through this so a float64
// artefact never shows up as $3.0000000000000004 in JSON.
func Round2(v float64) float64 { return math.Round(v*100) / 100 }

// Round1 rounds to one decimal place, used for day counts.
func Round1(v float64) float64 { return math.Round(v*10) / 10 }

// Summary describes a product's recorded price history.
type Summary struct {
	Count         int     `json:"count"`
	Min           float64 `json:"min"`
	Median        float64 `json:"median"`
	Max           float64 `json:"max"`
	FirstAt       int64   `json:"first_at"`
	LastAt        int64   `json:"last_at"`
	DaysOfHistory float64 `json:"days_of_history"`
}

// Summarize reduces a history to the figures real-special reports. Points need
// not be sorted.
func Summarize(points []Point) Summary {
	s := Summary{Count: len(points)}
	if len(points) == 0 {
		return s
	}
	prices := Prices(points)
	s.Min, s.Max, s.Median = Round2(Min(prices)), Round2(Max(prices)), Round2(Median(prices))
	s.FirstAt, s.LastAt = points[0].ObservedAt, points[0].ObservedAt
	for _, p := range points[1:] {
		if p.ObservedAt < s.FirstAt {
			s.FirstAt = p.ObservedAt
		}
		if p.ObservedAt > s.LastAt {
			s.LastAt = p.ObservedAt
		}
	}
	s.DaysOfHistory = Round1(float64(s.LastAt-s.FirstAt) / secondsPerDay)
	return s
}

// ClassifyInput carries everything Classify needs. Prior must contain only
// observations STRICTLY BEFORE the one being judged — otherwise the current
// price votes on its own typicality.
type ClassifyInput struct {
	CurrentPrice float64
	WasPrice     float64
	Prior        []Point
}

// Verdict is one classification plus the arithmetic that produced it. The
// reason is part of the contract: a verdict a user cannot audit is a guess.
type Verdict struct {
	Label  string `json:"label"`
	Reason string `json:"reason"`
}

// Classify decides whether an advertised special is real, in this order:
//
//  1. NO-HISTORY   — fewer than MinObservationsForVerdict prior observations.
//  2. WAS-PRICE-INFLATED — the advertised saving is measured from a number the
//     product rarely sold at. Checked first because it is the strongest
//     consumer warning: a product parked at one price for a year with a
//     doubled was-price would otherwise read as a harmless RECYCLED low.
//  3. RECYCLED     — this exact price is the product's habitual cycle low.
//  4. GENUINE      — at/near the local minimum AND materially below the median.
//  5. ORDINARY     — enough history to judge; nothing notable about today.
func Classify(in ClassifyInput) Verdict {
	n := len(in.Prior)
	if n < MinObservationsForVerdict {
		return Verdict{
			Label: VerdictNoHistory,
			Reason: fmt.Sprintf("only %d prior local observation(s); %d are needed before any verdict is honest — re-run 'real-special <term>' over several weeks to record more",
				n, MinObservationsForVerdict),
		}
	}
	prices := Prices(in.Prior)
	med := Median(prices)
	mn := Min(prices)

	if in.WasPrice > 0 && med > 0 {
		atOrAbove := 0
		for _, p := range prices {
			if p >= in.WasPrice-priceEpsilon {
				atOrAbove++
			}
		}
		share := float64(atOrAbove) / float64(n)
		if in.WasPrice >= med*inflationRatio && share <= inflationShare {
			return Verdict{
				Label: VerdictWasInflated,
				Reason: fmt.Sprintf("advertised was-price $%.2f is %.0f%% above the local median $%.2f, and only %d of %d observations (%.0f%%) ever reached it",
					in.WasPrice, (in.WasPrice/med-1)*100, med, atOrAbove, n, share*100),
			}
		}
	}

	nearMin := in.CurrentPrice <= mn*nearMinRatio+priceEpsilon

	repeats := 0
	for _, p := range prices {
		if math.Abs(p-in.CurrentPrice) <= priceEpsilon {
			repeats++
		}
	}
	if repeats >= recycledRepeats && nearMin {
		return Verdict{
			Label: VerdictRecycled,
			Reason: fmt.Sprintf("$%.2f has already appeared %d of %d times locally and is at the local minimum $%.2f — this is the normal cycle low, not a new one",
				in.CurrentPrice, repeats, n, mn),
		}
	}

	if nearMin && in.CurrentPrice <= med*genuineMedianRatio {
		return Verdict{
			Label: VerdictGenuine,
			Reason: fmt.Sprintf("$%.2f is at/near the local minimum $%.2f and %.0f%% below the local median $%.2f across %d observations",
				in.CurrentPrice, mn, (1-in.CurrentPrice/med)*100, med, n),
		}
	}

	return Verdict{
		Label: VerdictOrdinary,
		Reason: fmt.Sprintf("$%.2f is not a notable low: local minimum $%.2f, local median $%.2f across %d observations",
			in.CurrentPrice, mn, med, n),
	}
}

// Episode is a contiguous run of observations where IsHalfPrice was true.
type Episode struct {
	StartAt      int64   `json:"start_at"`
	EndAt        int64   `json:"end_at"`
	Observations int     `json:"observations"`
	RunDays      float64 `json:"run_days"`
	LowPrice     float64 `json:"low_price"`
}

// HalfPriceEpisodes segments a history into contiguous half-price runs. Points
// are sorted by time first, so callers may pass an unsorted history.
//
// "Contiguous" is defined over the observation sequence, not the calendar: two
// half-price sightings with a full-price sighting between them are two
// episodes; two half-price sightings with nothing recorded between them are
// one. That is the only segmentation the data can honestly support — a gap in
// syncing is a gap in knowledge, not evidence the special ended.
func HalfPriceEpisodes(points []Point) []Episode {
	sorted := append([]Point(nil), points...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ObservedAt < sorted[j].ObservedAt })

	out := make([]Episode, 0)
	var cur *Episode
	for _, p := range sorted {
		if !p.IsHalfPrice {
			if cur != nil {
				out = append(out, *cur)
				cur = nil
			}
			continue
		}
		if cur == nil {
			cur = &Episode{StartAt: p.ObservedAt, EndAt: p.ObservedAt, LowPrice: p.Price}
		}
		cur.EndAt = p.ObservedAt
		cur.Observations++
		if p.Price < cur.LowPrice {
			cur.LowPrice = p.Price
		}
	}
	if cur != nil {
		out = append(out, *cur)
	}
	for i := range out {
		out[i].RunDays = Round1(float64(out[i].EndAt-out[i].StartAt) / secondsPerDay)
		out[i].LowPrice = Round2(out[i].LowPrice)
	}
	return out
}

// Confidence levels for a cycle forecast.
const (
	ConfidenceNone   = "none"
	ConfidenceLow    = "low"
	ConfidenceMedium = "medium"
	ConfidenceHigh   = "high"
)

// Cycle is the half-price rhythm derived from one product's history.
type Cycle struct {
	Episodes          int       `json:"episodes"`
	MedianGapDays     float64   `json:"median_gap_days"`
	MedianRunDays     float64   `json:"median_run_days"`
	DaysSinceLastEnd  float64   `json:"days_since_last_episode_ended"`
	NextWindowStartAt int64     `json:"next_window_start_at,omitempty"`
	NextWindowEndAt   int64     `json:"next_window_end_at,omitempty"`
	Confidence        string    `json:"confidence"`
	Forecast          bool      `json:"forecast"`
	Note              string    `json:"note"`
	EpisodeList       []Episode `json:"episode_list"`
}

// AnalyzeCycle derives the half-price rhythm and, only when the evidence
// supports one, a next-window estimate.
//
// With zero or one recorded episode there is no gap to measure, so Forecast
// stays false and NextWindow* stay zero. Inventing a window from a single
// sighting would be the exact failure mode this command exists to avoid.
func AnalyzeCycle(points []Point, now int64) Cycle {
	eps := HalfPriceEpisodes(points)
	c := Cycle{Episodes: len(eps), EpisodeList: eps, Confidence: ConfidenceNone}

	switch len(eps) {
	case 0:
		c.Note = "no half-price episodes recorded locally; nothing to forecast from — run 'real-special' on this product over several weeks first"
		return c
	case 1:
		c.MedianRunDays = eps[0].RunDays
		c.DaysSinceLastEnd = Round1(float64(now-eps[0].EndAt) / secondsPerDay)
		c.Confidence = ConfidenceLow
		c.Note = "only 1 recorded half-price episode; a cycle needs at least 2 to measure a gap, so no window is estimated"
		return c
	}

	gaps := make([]float64, 0, len(eps)-1)
	for i := 1; i < len(eps); i++ {
		gaps = append(gaps, float64(eps[i].StartAt-eps[i-1].StartAt)/secondsPerDay)
	}
	runs := make([]float64, 0, len(eps))
	for _, e := range eps {
		runs = append(runs, e.RunDays)
	}
	c.MedianGapDays = Round1(Median(gaps))
	c.MedianRunDays = Round1(Median(runs))
	last := eps[len(eps)-1]
	c.DaysSinceLastEnd = Round1(float64(now-last.EndAt) / secondsPerDay)

	switch {
	case len(eps) >= 5:
		c.Confidence = ConfidenceHigh
	case len(eps) >= 3:
		c.Confidence = ConfidenceMedium
	default:
		c.Confidence = ConfidenceLow
	}
	// An erratic gap is weak evidence however many episodes there are.
	if lo, hi := Min(gaps), Max(gaps); lo > 0 && hi/lo > 2 {
		c.Confidence = downgrade(c.Confidence)
		c.Note = fmt.Sprintf("gaps between episodes range %.0f-%.0f days, so the rhythm is irregular; ", lo, hi)
	}

	if c.MedianGapDays <= 0 {
		c.Note += "recorded episode starts are not separated in time, so no window can be estimated"
		return c
	}
	step := int64(c.MedianGapDays * secondsPerDay)
	next := last.StartAt + step
	for next <= now && step > 0 {
		next += step
	}
	c.NextWindowStartAt = next
	runSpan := int64(c.MedianRunDays * secondsPerDay)
	if runSpan < int64(secondsPerDay) {
		runSpan = int64(secondsPerDay)
	}
	c.NextWindowEndAt = next + runSpan
	c.Forecast = true
	c.Note += fmt.Sprintf("estimate projects the median %.1f-day gap forward from the last episode start; confidence %s from %d recorded episode(s)",
		c.MedianGapDays, c.Confidence, len(eps))
	return c
}

func downgrade(level string) string {
	switch level {
	case ConfidenceHigh:
		return ConfidenceMedium
	case ConfidenceMedium:
		return ConfidenceLow
	default:
		return ConfidenceLow
	}
}

// TermRelevant reports whether a product tile plausibly answers the search
// term. Woolworths' search happily returns loosely related merchandise for a
// nonsense query; emitting verdicts on those would be the CLI asserting
// something about a product the user never asked about.
//
// A tile is relevant when every alphanumeric token of length >= 3 in the term
// appears somewhere in the product's name or brand. Short tokens ("og", "2%")
// and pure punctuation are ignored; a term made only of such tokens matches
// everything rather than nothing.
func TermRelevant(term, name, brand string) bool {
	tokens := Tokenize(term)
	if len(tokens) == 0 {
		return true
	}
	hay := strings.ToLower(name + " " + brand)
	for _, tok := range tokens {
		if !strings.Contains(hay, tok) {
			return false
		}
	}
	return true
}

// Tokenize lowercases a phrase and splits it into alphanumeric tokens of
// length >= 3.
func Tokenize(s string) []string {
	fields := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if len(f) >= 3 {
			out = append(out, f)
		}
	}
	return out
}

// IsStockcode reports whether s looks like a bare Woolworths stockcode.
func IsStockcode(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}
