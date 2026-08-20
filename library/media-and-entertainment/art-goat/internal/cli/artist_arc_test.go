// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0.

package cli

import (
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/art-goat/internal/store"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuildArtistArc_EmptyCorpus is the no-data path. We still want a
// non-nil arc back so the renderer's "no works" message fires instead of
// crashing on nil.
func TestBuildArtistArc_EmptyCorpus(t *testing.T) {
	arc := buildArtistArc("hokusai", nil)
	require.NotNil(t, arc)
	assert.Equal(t, "hokusai", arc.Query)
	assert.Equal(t, 0, arc.TotalWorks)
	assert.Empty(t, arc.Buckets)
}

// TestBuildArtistArc_PeriodBucketing covers the primary path: works
// carry non-empty Period strings and get bucketed by (period, decade).
func TestBuildArtistArc_PeriodBucketing(t *testing.T) {
	works := []store.Work{
		{ID: "aic:1", DateStart: 1820, Period: "Edo period", Medium: "woodblock", CultureRegion: "Japan", Source: "aic"},
		{ID: "aic:2", DateStart: 1825, Period: "Edo period", Medium: "woodblock", CultureRegion: "Japan", Source: "aic"},
		{ID: "met:3", DateStart: 1828, Period: "Edo period", Medium: "woodblock", CultureRegion: "Japan", Source: "met"},
		{ID: "aic:4", DateStart: 1832, Period: "Edo period", Medium: "ink", CultureRegion: "Japan", Source: "aic"},
	}
	arc := buildArtistArc("hokusai", works)
	require.NotNil(t, arc)
	assert.Equal(t, 4, arc.TotalWorks)
	assert.Equal(t, 1820, arc.Earliest)
	assert.Equal(t, 1832, arc.Latest)
	require.NotEmpty(t, arc.Buckets)
	// 1820s bucket: 3 woodblocks, 1830s bucket: 1 ink.
	assert.Equal(t, "Edo period (1820s)", arc.Buckets[0].Label)
	assert.Equal(t, 3, arc.Buckets[0].Count)
	assert.Equal(t, "woodblock", arc.Buckets[0].DominantMed)
	assert.Equal(t, "Japan", arc.Buckets[0].DominantReg)
	assert.Equal(t, []string{"aic", "met"}, arc.Buckets[0].Sources)
}

// TestBuildArtistArc_DecadeBucketing covers the fallback path: no
// period strings, so works bucket by decade only.
func TestBuildArtistArc_DecadeBucketing(t *testing.T) {
	works := []store.Work{
		{ID: "harvard:1", DateStart: 1880, Period: "", Medium: "oil", CultureRegion: "Netherlands", Source: "harvard"},
		{ID: "harvard:2", DateStart: 1889, Period: "", Medium: "oil", CultureRegion: "France", Source: "harvard"},
		{ID: "met:3", DateStart: 1890, Period: "", Medium: "oil", CultureRegion: "France", Source: "met"},
	}
	arc := buildArtistArc("van gogh", works)
	require.Len(t, arc.Buckets, 2)
	assert.Equal(t, "1880s", arc.Buckets[0].Label)
	assert.Equal(t, "1890s", arc.Buckets[1].Label)
}

// TestBuildArtistArc_MergeSmallAdjacent ensures buckets cap at 6 even
// when the raw input would produce more. Smallest adjacent pair merges
// first; chronological extremes survive.
func TestBuildArtistArc_MergeSmallAdjacent(t *testing.T) {
	// Eight decades, each with 5 works except the middle four which have
	// 1 each. The middle four should merge to bring the total down to 6.
	makeWorks := func(decade, n int) []store.Work {
		out := make([]store.Work, 0, n)
		for i := 0; i < n; i++ {
			out = append(out, store.Work{
				ID:        randomID("w", decade, i),
				DateStart: decade + i,
				Medium:    "oil",
				Source:    "aic",
			})
		}
		return out
	}
	var works []store.Work
	works = append(works, makeWorks(1800, 5)...)
	works = append(works, makeWorks(1810, 1)...)
	works = append(works, makeWorks(1820, 1)...)
	works = append(works, makeWorks(1830, 1)...)
	works = append(works, makeWorks(1840, 1)...)
	works = append(works, makeWorks(1850, 5)...)
	works = append(works, makeWorks(1860, 5)...)
	works = append(works, makeWorks(1870, 5)...)
	arc := buildArtistArc("anonymous", works)
	assert.LessOrEqual(t, len(arc.Buckets), 6, "buckets must cap at 6")
	// First and last buckets must remain at the chronological extremes.
	assert.Equal(t, 1800, arc.Buckets[0].StartYear)
	last := arc.Buckets[len(arc.Buckets)-1]
	assert.GreaterOrEqual(t, last.EndYear, 1870)
}

// TestBuildArtistArc_UndatedBucket: works with DateStart=0 land in a
// trailing "undated" bucket so they're surfaced rather than dropped.
func TestBuildArtistArc_UndatedBucket(t *testing.T) {
	works := []store.Work{
		{ID: "met:1", DateStart: 1500, Medium: "oil", Source: "met"},
		{ID: "met:2", DateStart: 1505, Medium: "oil", Source: "met"},
		{ID: "met:3", DateStart: 0, Medium: "oil", Source: "met"}, // undated
	}
	arc := buildArtistArc("anon", works)
	require.NotEmpty(t, arc.Buckets)
	last := arc.Buckets[len(arc.Buckets)-1]
	assert.Equal(t, "undated", last.Label)
	assert.Equal(t, 1, last.Count)
}

// TestBuildArtistArc_RepresentativeWork: the representative is the
// median-by-date work — not an outlier at either edge of a bucket.
func TestBuildArtistArc_RepresentativeWork(t *testing.T) {
	works := []store.Work{
		{ID: "x:1", DateStart: 1820, Period: "Edo", Medium: "ink", Source: "aic"},
		{ID: "x:2", DateStart: 1822, Period: "Edo", Medium: "ink", Source: "aic"},
		{ID: "x:3", DateStart: 1825, Period: "Edo", Medium: "ink", Source: "aic"},
	}
	arc := buildArtistArc("hokusai", works)
	require.Len(t, arc.Buckets, 1)
	require.NotNil(t, arc.Buckets[0].Representative)
	// Median of 3 is index 1.
	assert.Equal(t, "x:2", arc.Buckets[0].Representative.ID)
}

// TestArtistArc_Envelope checks the JSON envelope shape — kept in sync
// with the workToEnvelope keys so agent consumers can dispatch on the
// same fields.
func TestArtistArc_Envelope(t *testing.T) {
	works := []store.Work{
		{
			ID: "aic:1", Source: "aic", SourceID: "1", Title: "Wave",
			Creator: "Hokusai", DateStart: 1830, DateText: "c.1830",
			Period: "Edo period", Medium: "woodblock", CultureRegion: "Japan",
		},
	}
	arc := buildArtistArc("hokusai", works)
	env := arc.envelope()
	assert.Equal(t, "hokusai", env["query"])
	assert.Equal(t, 1, env["total_works"])
	buckets, ok := env["buckets"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, buckets, 1)
	assert.Equal(t, "Edo period (1830s)", buckets[0]["label"])
	repr, ok := buckets[0]["representative"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "aic:1", repr["id"])
	assert.Equal(t, "woodblock", repr["medium"])
}

// randomID is a tiny deterministic helper so test inputs don't share
// IDs across decades and confuse the bucketer's stable sort.
func randomID(prefix string, decade, i int) string {
	return prefix + ":" + itoaPad(decade) + "-" + itoaPad(i)
}

func itoaPad(n int) string {
	if n < 0 {
		return "0"
	}
	if n < 10 {
		return "0" + string(rune('0'+n))
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}
