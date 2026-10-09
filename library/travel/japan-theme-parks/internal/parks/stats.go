// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package parks

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// Sample is one stored wait observation.
type Sample struct {
	RideID      int
	RideName    string
	Land        string
	IsOpen      bool
	WaitMinutes int
	At          time.Time // source last_updated
}

// Typical summarizes open-ride waits for one ride in one weekday x hour cell.
type Typical struct {
	RideID        int      `json:"ride_id"`
	RideName      string   `json:"ride_name"`
	Land          string   `json:"land,omitempty"`
	Weekday       string   `json:"weekday"`
	Hour          int      `json:"hour"`
	Median        *float64 `json:"median_minutes"`
	P75           *float64 `json:"p75_minutes"`
	Samples       int      `json:"samples"`
	ClosedSamples int      `json:"closed_samples"`
	DistinctDays  int      `json:"distinct_days"`
	FirstDate     *string  `json:"first_date"` // null when no open samples
	LastDate      *string  `json:"last_date"`
	Status        string   `json:"status"`
}

// Weekdays in display order.
var Weekdays = []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"}

// WeekdayName returns the short lowercase weekday for t in Tokyo.
func WeekdayName(t time.Time) string {
	return strings.ToLower(t.In(Tokyo).Weekday().String()[:3])
}

// ParseWeekdays parses "sat,sun" or "weekend"/"weekday" into a set.
func ParseWeekdays(csv string) (map[string]bool, error) {
	out := map[string]bool{}
	for _, p := range SplitCSV(strings.ToLower(csv)) {
		switch p {
		case "weekend":
			out["sat"], out["sun"] = true, true
			continue
		case "weekday", "weekdays":
			for _, d := range Weekdays[:5] {
				out[d] = true
			}
			continue
		}
		if len(p) >= 3 {
			p = p[:3]
		}
		ok := false
		for _, d := range Weekdays {
			if p == d {
				ok = true
			}
		}
		if !ok {
			return nil, fmt.Errorf("unknown weekday %q (use mon..sun, weekday or weekend)", p)
		}
		out[p] = true
	}
	return out, nil
}

// Quantile returns the linear-interpolated quantile (type 7) of values.
func Quantile(values []float64, q float64) float64 {
	if len(values) == 0 {
		return math.NaN()
	}
	s := append([]float64(nil), values...)
	sort.Float64s(s)
	return quantileSorted(s, q)
}

// quantileSorted is Quantile for an already sorted, non-empty slice.
func quantileSorted(s []float64, q float64) float64 {
	if len(s) == 1 {
		return s[0]
	}
	pos := q * float64(len(s)-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	frac := pos - float64(lo)
	return s[lo] + (s[hi]-s[lo])*frac
}

// Summarize groups samples by ride x weekday x hour (Tokyo time). Cells with
// fewer than minSamples open samples get status "insufficient" and null stats.
func Summarize(samples []Sample, minSamples int) []Typical {
	type key struct {
		ride    int
		weekday string
		hour    int
	}
	type acc struct {
		name, land  string
		waits       []float64
		closed      int
		days        map[string]bool
		first, last string
	}
	cells := map[key]*acc{}
	for _, s := range samples {
		t := s.At.In(Tokyo)
		k := key{s.RideID, WeekdayName(t), t.Hour()}
		a := cells[k]
		if a == nil {
			a = &acc{days: map[string]bool{}}
			cells[k] = a
		}
		a.name, a.land = s.RideName, s.Land
		if !s.IsOpen {
			a.closed++
			continue
		}
		day := t.Format("2006-01-02")
		if a.first == "" || day < a.first {
			a.first = day
		}
		if day > a.last {
			a.last = day
		}
		a.waits = append(a.waits, float64(s.WaitMinutes))
		a.days[day] = true
	}
	out := make([]Typical, 0, len(cells))
	for k, a := range cells {
		t := Typical{
			RideID: k.ride, RideName: a.name, Land: a.land, Weekday: k.weekday, Hour: k.hour,
			Samples: len(a.waits), ClosedSamples: a.closed, DistinctDays: len(a.days),
			Status: "ok",
		}
		if a.first != "" {
			first, last := a.first, a.last
			t.FirstDate, t.LastDate = &first, &last
		}
		if len(a.waits) < minSamples || len(a.waits) == 0 {
			t.Status = "insufficient"
		} else {
			sort.Float64s(a.waits)
			med := round1(quantileSorted(a.waits, 0.5))
			p75 := round1(quantileSorted(a.waits, 0.75))
			t.Median, t.P75 = &med, &p75
		}
		out = append(out, t)
	}
	wd := map[string]int{}
	for i, d := range Weekdays {
		wd[d] = i
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].RideName != out[j].RideName {
			return out[i].RideName < out[j].RideName
		}
		if out[i].Weekday != out[j].Weekday {
			return wd[out[i].Weekday] < wd[out[j].Weekday]
		}
		if out[i].Hour != out[j].Hour {
			return out[i].Hour < out[j].Hour
		}
		return out[i].RideID < out[j].RideID
	})
	return out
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }

// WeekdayNumber maps "mon".."sun" to time.Weekday numbers (Sunday = 0).
func WeekdayNumber(name string) (int, bool) {
	for i, d := range Weekdays {
		if d == name {
			return (i + 1) % 7, true
		}
	}
	return 0, false
}
