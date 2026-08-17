// Copyright 2026 nspage and contributors. Licensed under Apache-2.0. See LICENSE.

package venuex

import (
	"math"
	"sort"
)

// Band is a histogram bucket.
type Band struct {
	Min       float64  `json:"min"`
	Max       float64  `json:"max"`
	Count     int      `json:"count"`
	SampleIDs []string `json:"sample_ids"`
}

// BandPrices groups listings by hourly price into fixed-width bands.
// Listings with non-positive price are skipped. bandWidth defaults to 50 when <= 0.
func BandPrices(listings []Listing, bandWidth float64) []Band {
	if bandWidth <= 0 {
		bandWidth = 50
	}
	type acc struct {
		count int
		ids   []string
	}
	buckets := map[int]*acc{}
	for _, l := range listings {
		if l.PriceHourly <= 0 {
			continue
		}
		idx := int(math.Floor(l.PriceHourly / bandWidth))
		a := buckets[idx]
		if a == nil {
			a = &acc{ids: make([]string, 0, 3)}
			buckets[idx] = a
		}
		a.count++
		if len(a.ids) < 5 && l.ID != "" {
			a.ids = append(a.ids, l.ID)
		}
	}
	keys := make([]int, 0, len(buckets))
	for k := range buckets {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	out := make([]Band, 0, len(keys))
	for _, k := range keys {
		a := buckets[k]
		ids := a.ids
		if ids == nil {
			ids = make([]string, 0)
		}
		out = append(out, Band{
			Min:       float64(k) * bandWidth,
			Max:       float64(k+1) * bandWidth,
			Count:     a.count,
			SampleIDs: ids,
		})
	}
	return out
}

// BandCapacity groups listings by guest capacity into fixed-width bands.
// bandWidth defaults to 10 when <= 0. Zero-capacity rows are skipped.
func BandCapacity(listings []Listing, bandWidth int) []Band {
	if bandWidth <= 0 {
		bandWidth = 10
	}
	type acc struct {
		count int
		ids   []string
	}
	buckets := map[int]*acc{}
	for _, l := range listings {
		if l.Guests <= 0 {
			continue
		}
		idx := l.Guests / bandWidth
		a := buckets[idx]
		if a == nil {
			a = &acc{ids: make([]string, 0, 3)}
			buckets[idx] = a
		}
		a.count++
		if len(a.ids) < 5 && l.ID != "" {
			a.ids = append(a.ids, l.ID)
		}
	}
	keys := make([]int, 0, len(buckets))
	for k := range buckets {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	out := make([]Band, 0, len(keys))
	for _, k := range keys {
		a := buckets[k]
		ids := a.ids
		if ids == nil {
			ids = make([]string, 0)
		}
		out = append(out, Band{
			Min:       float64(k * bandWidth),
			Max:       float64((k + 1) * bandWidth),
			Count:     a.count,
			SampleIDs: ids,
		})
	}
	return out
}
