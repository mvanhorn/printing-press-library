// Copyright 2026 justinwfu. Licensed under Apache-2.0.

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/art-goat/internal/store"

	"github.com/spf13/cobra"
)

// artistArc captures the period-bucketed view of a creator's works for
// the `artist --arc` renderer. Buckets are ordered chronologically and
// each one carries enough metadata to render one line of narrative
// (period label, span, representative work, dominant medium/region).
type artistArc struct {
	Query      string
	TotalWorks int
	Earliest   int
	Latest     int
	Buckets    []artistArcBucket
}

type artistArcBucket struct {
	Label          string      // period name OR century string (e.g. "Edo period" / "1830s" / "1820–1840")
	StartYear      int         // representative start
	EndYear        int         // representative end
	Count          int         // works in this bucket
	DominantMed    string      // most-common medium
	DominantReg    string      // most-common region
	Sources        []string    // distinct sources in this bucket, sorted
	Representative *store.Work // most "central" work (median by date_start)
}

// buildArtistArc partitions a creator's chronologically-sorted works
// into stylistic periods and computes per-bucket dominant medium,
// region, sources, and representative work. Returns an empty arc with
// TotalWorks=0 when no works are provided so the renderer can show the
// "no works found" message rather than crashing on nil slices.
//
// Bucketing strategy:
//  1. If the corpus carries a non-empty `period` field, bucket by
//     (period, decade). The (period, decade) pair keeps the buckets
//     dense enough to be meaningful even when a creator's "period"
//     label covers a 200-year span (common in the Met's data).
//  2. If no period strings are present, bucket by decade only.
//  3. Works with date_start == 0 land in a single trailing "undated"
//     bucket so they're still surfaced rather than dropped.
//
// At most 6 buckets are emitted; smaller adjacent buckets are merged
// from the middle outward so the chronological extremes stay visible.
func buildArtistArc(query string, works []store.Work) *artistArc {
	arc := &artistArc{Query: query, TotalWorks: len(works)}
	if len(works) == 0 {
		return arc
	}

	dated := make([]store.Work, 0, len(works))
	undated := make([]store.Work, 0)
	for _, w := range works {
		if w.DateStart > 0 {
			dated = append(dated, w)
		} else {
			undated = append(undated, w)
		}
	}
	// WorksByCreator orders by date_start ASC, id ASC. Reaffirm so callers
	// don't have to depend on that ordering invariant.
	sort.SliceStable(dated, func(i, j int) bool {
		if dated[i].DateStart != dated[j].DateStart {
			return dated[i].DateStart < dated[j].DateStart
		}
		return dated[i].ID < dated[j].ID
	})
	if len(dated) > 0 {
		arc.Earliest = dated[0].DateStart
		arc.Latest = dated[len(dated)-1].DateStart
	}

	usePeriod := worksCarryPeriod(dated)
	type key struct {
		period string
		decade int
	}
	buckets := make(map[key][]store.Work)
	var order []key
	for _, w := range dated {
		k := key{decade: (w.DateStart / 10) * 10}
		if usePeriod {
			k.period = strings.TrimSpace(w.Period)
		}
		if _, seen := buckets[k]; !seen {
			order = append(order, k)
		}
		buckets[k] = append(buckets[k], w)
	}

	bucketed := make([]artistArcBucket, 0, len(order))
	for _, k := range order {
		bs := buckets[k]
		bucketed = append(bucketed, summarizeBucket(k.period, k.decade, bs))
	}
	// Merge to at most 6 buckets so the narrative stays readable.
	bucketed = mergeBuckets(bucketed, 6)

	if len(undated) > 0 {
		bucketed = append(bucketed, artistArcBucket{
			Label:          "undated",
			Count:          len(undated),
			DominantMed:    dominant(undated, func(w store.Work) string { return w.Medium }),
			DominantReg:    dominant(undated, func(w store.Work) string { return w.CultureRegion }),
			Sources:        distinctSources(undated),
			Representative: representative(undated),
		})
	}

	arc.Buckets = bucketed
	return arc
}

func worksCarryPeriod(works []store.Work) bool {
	for _, w := range works {
		if strings.TrimSpace(w.Period) != "" {
			return true
		}
	}
	return false
}

func summarizeBucket(period string, decade int, works []store.Work) artistArcBucket {
	bucket := artistArcBucket{
		Count:          len(works),
		DominantMed:    dominant(works, func(w store.Work) string { return w.Medium }),
		DominantReg:    dominant(works, func(w store.Work) string { return w.CultureRegion }),
		Sources:        distinctSources(works),
		Representative: representative(works),
	}
	if len(works) > 0 {
		bucket.StartYear = works[0].DateStart
		bucket.EndYear = works[len(works)-1].DateStart
	}
	switch {
	case period != "" && bucket.StartYear > 0:
		bucket.Label = fmt.Sprintf("%s (%ds)", period, decade)
	case period != "":
		bucket.Label = period
	case bucket.StartYear > 0:
		bucket.Label = fmt.Sprintf("%ds", decade)
	default:
		bucket.Label = "undated"
	}
	return bucket
}

// mergeBuckets collapses the smallest adjacent pair until len <= max.
// "Smallest" is measured by combined work count; collapsing low-mass
// adjacent buckets preserves the chronological story while keeping the
// chronological extremes (which are most likely to anchor a narrative)
// untouched.
func mergeBuckets(buckets []artistArcBucket, max int) []artistArcBucket {
	for len(buckets) > max {
		// Find the adjacent pair with the smallest combined count.
		smallestIdx := 0
		smallestSum := buckets[0].Count + buckets[1].Count
		for i := 1; i < len(buckets)-1; i++ {
			s := buckets[i].Count + buckets[i+1].Count
			if s < smallestSum {
				smallestSum = s
				smallestIdx = i
			}
		}
		a, b := buckets[smallestIdx], buckets[smallestIdx+1]
		merged := mergePair(a, b)
		out := make([]artistArcBucket, 0, len(buckets)-1)
		out = append(out, buckets[:smallestIdx]...)
		out = append(out, merged)
		out = append(out, buckets[smallestIdx+2:]...)
		buckets = out
	}
	return buckets
}

func mergePair(a, b artistArcBucket) artistArcBucket {
	combined := artistArcBucket{
		Count: a.Count + b.Count,
	}
	switch {
	case a.StartYear > 0 && b.EndYear > 0:
		combined.StartYear = a.StartYear
		combined.EndYear = b.EndYear
		combined.Label = fmt.Sprintf("%d–%d", a.StartYear, b.EndYear)
	case a.StartYear > 0:
		combined.StartYear = a.StartYear
		combined.EndYear = a.EndYear
		combined.Label = a.Label
	default:
		combined.Label = b.Label
		combined.StartYear = b.StartYear
		combined.EndYear = b.EndYear
	}
	combined.DominantMed = pickNonEmpty(a.DominantMed, b.DominantMed)
	combined.DominantReg = pickNonEmpty(a.DominantReg, b.DominantReg)
	combined.Sources = mergeSorted(a.Sources, b.Sources)
	// Representative: prefer the one with the longer description (more
	// likely to be a flagship record). Falls back to a's if both are nil.
	switch {
	case a.Representative == nil:
		combined.Representative = b.Representative
	case b.Representative == nil:
		combined.Representative = a.Representative
	case len(b.Representative.Description) > len(a.Representative.Description):
		combined.Representative = b.Representative
	default:
		combined.Representative = a.Representative
	}
	return combined
}

func pickNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

func mergeSorted(a, b []string) []string {
	seen := make(map[string]bool, len(a)+len(b))
	for _, s := range a {
		seen[s] = true
	}
	for _, s := range b {
		seen[s] = true
	}
	out := make([]string, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func dominant(works []store.Work, pick func(store.Work) string) string {
	counts := make(map[string]int)
	for _, w := range works {
		v := strings.TrimSpace(pick(w))
		if v == "" {
			continue
		}
		counts[v]++
	}
	bestKey := ""
	bestN := 0
	for k, n := range counts {
		if n > bestN || (n == bestN && k < bestKey) {
			bestKey = k
			bestN = n
		}
	}
	return bestKey
}

func distinctSources(works []store.Work) []string {
	seen := make(map[string]bool)
	for _, w := range works {
		if w.Source == "" {
			continue
		}
		seen[w.Source] = true
	}
	out := make([]string, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// representative picks the middle work by date_start as the canonical
// "this is the one to cite" — taking from the middle keeps a long-tail
// bucket from being represented by an outlier at either edge.
func representative(works []store.Work) *store.Work {
	if len(works) == 0 {
		return nil
	}
	mid := len(works) / 2
	w := works[mid]
	return &w
}

func (a *artistArc) envelope() map[string]any {
	buckets := make([]map[string]any, 0, len(a.Buckets))
	for _, b := range a.Buckets {
		bucket := map[string]any{
			"label":   b.Label,
			"count":   b.Count,
			"start":   b.StartYear,
			"end":     b.EndYear,
			"medium":  b.DominantMed,
			"region":  b.DominantReg,
			"sources": b.Sources,
		}
		if b.Representative != nil {
			bucket["representative"] = workToEnvelope(*b.Representative)
		}
		buckets = append(buckets, bucket)
	}
	return map[string]any{
		"query":       a.Query,
		"total_works": a.TotalWorks,
		"earliest":    a.Earliest,
		"latest":      a.Latest,
		"buckets":     buckets,
	}
}

func renderArtistArc(cmd *cobra.Command, arc *artistArc) {
	out := cmd.OutOrStdout()
	fmt.Fprintln(out, "")
	fmt.Fprintf(out, "Arc: %s\n", arc.Query)
	if arc.TotalWorks == 0 {
		fmt.Fprintln(out, "")
		fmt.Fprintln(out, "No works found for that artist in the local corpus.")
		fmt.Fprintln(out, "Try `sync` first, or widen the query (e.g. just the surname).")
		fmt.Fprintln(out, "")
		return
	}
	if arc.Earliest > 0 && arc.Latest > arc.Earliest {
		fmt.Fprintf(out, "Span: %d–%d  ·  %d works in the local corpus\n", arc.Earliest, arc.Latest, arc.TotalWorks)
	} else {
		fmt.Fprintf(out, "Works: %d in the local corpus\n", arc.TotalWorks)
	}
	fmt.Fprintln(out, "")
	for i, b := range arc.Buckets {
		fmt.Fprintf(out, "  %d. %s — %s\n", i+1, b.Label, narrativeFor(b))
		if b.Representative != nil {
			repr := b.Representative
			fmt.Fprintf(out, "       %s · %s · %s\n",
				coalesce(repr.Title, "(untitled)"),
				coalesce(repr.Creator, "(unknown)"),
				repr.ID,
			)
		}
	}
	fmt.Fprintln(out, "")
}

// narrativeFor composes a single descriptive sentence from a bucket's
// dominant medium, region, and source spread. Kept short so the full
// arc reads as a 5-line career summary rather than a wall of metadata.
func narrativeFor(b artistArcBucket) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("%d work%s", b.Count, pluralS(b.Count)))
	if b.DominantMed != "" {
		parts = append(parts, fmt.Sprintf("mostly %s", strings.ToLower(b.DominantMed)))
	}
	if b.DominantReg != "" {
		parts = append(parts, b.DominantReg)
	}
	if len(b.Sources) > 0 {
		parts = append(parts, fmt.Sprintf("from %s", strings.Join(b.Sources, ", ")))
	}
	return strings.Join(parts, "; ")
}
