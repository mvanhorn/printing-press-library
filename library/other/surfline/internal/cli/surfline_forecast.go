// Copyright 2026 Shoffner and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored shared helpers for Surfline novel commands (now, rank, windows,
// raw, buoy-check, journal, alert). Not generator-emitted: this file fetches and
// parses Surfline KBYG forecast resources and joins them on the shared unix
// timestamp, which no single upstream endpoint returns.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/surfline/internal/client"
)

// ratingScore maps Surfline's rating key enum to a numeric score so the
// rank/now/windows commands can sort and compare. The API also returns a
// numeric `value`; callers prefer that when present and fall back to this map.
var ratingScore = map[string]float64{
	"FLAT":         0,
	"VERY_POOR":    1,
	"POOR":         2,
	"POOR_TO_FAIR": 3,
	"FAIR":         4,
	"FAIR_TO_GOOD": 5,
	"GOOD":         6,
	"EPIC":         7,
}

type swellComponent struct {
	Height       float64 `json:"height"`
	Period       float64 `json:"period"`
	Direction    float64 `json:"direction"`
	DirectionMin float64 `json:"directionMin"`
	OptimalScore int     `json:"optimalScore"`
}

type wavePoint struct {
	Timestamp int64 `json:"timestamp"`
	UTCOffset int   `json:"utcOffset"`
	Surf      struct {
		Min           float64 `json:"min"`
		Max           float64 `json:"max"`
		OptimalScore  int     `json:"optimalScore"`
		Plus          bool    `json:"plus"`
		HumanRelation string  `json:"humanRelation"`
	} `json:"surf"`
	Swells []swellComponent `json:"swells"`
}

type windPoint struct {
	Timestamp     int64   `json:"timestamp"`
	UTCOffset     int     `json:"utcOffset"`
	Speed         float64 `json:"speed"`
	Direction     float64 `json:"direction"`
	DirectionType string  `json:"directionType"`
	Gust          float64 `json:"gust"`
	OptimalScore  int     `json:"optimalScore"`
}

type ratingPoint struct {
	Timestamp int64 `json:"timestamp"`
	UTCOffset int   `json:"utcOffset"`
	Rating    struct {
		Key   string  `json:"key"`
		Value float64 `json:"value"`
	} `json:"rating"`
}

type tidePoint struct {
	Timestamp int64   `json:"timestamp"`
	UTCOffset int     `json:"utcOffset"`
	Type      string  `json:"type"`
	Height    float64 `json:"height"`
}

// topSwell returns the highest-energy swell component (by height*period^2, a
// rough proxy for swell energy), or false when none are present.
func (w wavePoint) topSwell() (swellComponent, bool) {
	best := swellComponent{}
	found := false
	bestEnergy := -1.0
	for _, s := range w.Swells {
		if s.Height <= 0 || s.Period <= 0 {
			continue
		}
		e := s.Height * s.Period * s.Period
		if e > bestEnergy {
			bestEnergy = e
			best = s
			found = true
		}
	}
	return best, found
}

// localTime renders a unix timestamp plus Surfline's utcOffset (whole hours) as
// the spot's wall-clock time. Surfline timestamps are UTC epoch seconds.
func localTime(ts int64, utcOffset int, layout string) string {
	if ts == 0 {
		return "-"
	}
	t := time.Unix(ts, 0).UTC().Add(time.Duration(utcOffset) * time.Hour)
	return t.Format(layout)
}

func fetchWave(ctx context.Context, c *client.Client, spotID string, days, interval int) ([]wavePoint, error) {
	params := map[string]string{"spotId": spotID}
	if days > 0 {
		params["days"] = fmt.Sprintf("%d", days)
	}
	if interval > 0 {
		params["intervalHours"] = fmt.Sprintf("%d", interval)
	}
	data, err := c.Get(ctx, "/kbyg/spots/forecasts/wave", params)
	if err != nil {
		return nil, err
	}
	var env struct {
		Data struct {
			Wave []wavePoint `json:"wave"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("parsing wave forecast: %w", err)
	}
	return env.Data.Wave, nil
}

func fetchWind(ctx context.Context, c *client.Client, spotID string, days, interval int) ([]windPoint, error) {
	params := map[string]string{"spotId": spotID}
	if days > 0 {
		params["days"] = fmt.Sprintf("%d", days)
	}
	if interval > 0 {
		params["intervalHours"] = fmt.Sprintf("%d", interval)
	}
	data, err := c.Get(ctx, "/kbyg/spots/forecasts/wind", params)
	if err != nil {
		return nil, err
	}
	var env struct {
		Data struct {
			Wind []windPoint `json:"wind"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("parsing wind forecast: %w", err)
	}
	return env.Data.Wind, nil
}

func fetchRating(ctx context.Context, c *client.Client, spotID string, days int) ([]ratingPoint, error) {
	params := map[string]string{"spotId": spotID}
	if days > 0 {
		params["days"] = fmt.Sprintf("%d", days)
	}
	data, err := c.Get(ctx, "/kbyg/spots/forecasts/rating", params)
	if err != nil {
		return nil, err
	}
	var env struct {
		Data struct {
			Rating []ratingPoint `json:"rating"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("parsing rating forecast: %w", err)
	}
	return env.Data.Rating, nil
}

func fetchTides(ctx context.Context, c *client.Client, spotID string, days int) ([]tidePoint, error) {
	params := map[string]string{"spotId": spotID}
	if days > 0 {
		params["days"] = fmt.Sprintf("%d", days)
	}
	data, err := c.Get(ctx, "/kbyg/spots/forecasts/tides", params)
	if err != nil {
		return nil, err
	}
	var env struct {
		Data struct {
			Tides []tidePoint `json:"tides"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("parsing tide forecast: %w", err)
	}
	return env.Data.Tides, nil
}

// fetchSpotName resolves a spotId to its display name via /kbyg/spots/details.
// Returns the spotId itself on any failure so callers always have a label.
func fetchSpotName(ctx context.Context, c *client.Client, spotID string) string {
	data, err := c.Get(ctx, "/kbyg/spots/details", map[string]string{"spotId": spotID})
	if err != nil {
		return spotID
	}
	var env struct {
		Spot struct {
			Name string `json:"name"`
		} `json:"spot"`
	}
	if err := json.Unmarshal(data, &env); err != nil || env.Spot.Name == "" {
		return spotID
	}
	return env.Spot.Name
}

// ratingValue returns a comparable numeric rating, preferring the API's numeric
// value and falling back to the key enum map.
func ratingValue(r ratingPoint) float64 {
	if r.Rating.Value > 0 {
		return r.Rating.Value
	}
	if v, ok := ratingScore[r.Rating.Key]; ok {
		return v
	}
	return 0
}

// ratingByTimestamp indexes rating points for fast join against wave/wind.
func ratingByTimestamp(ratings []ratingPoint) map[int64]ratingPoint {
	m := make(map[int64]ratingPoint, len(ratings))
	for _, r := range ratings {
		m[r.Timestamp] = r
	}
	return m
}

func windByTimestamp(winds []windPoint) map[int64]windPoint {
	m := make(map[int64]windPoint, len(winds))
	for _, w := range winds {
		m[w.Timestamp] = w
	}
	return m
}

// fetchSpotLatLon resolves a spot's coordinates from the wave forecast's
// associated.location block (the spots/details endpoint returns only the name).
func fetchSpotLatLon(ctx context.Context, c *client.Client, spotID string) (lat, lon float64, ok bool) {
	data, err := c.Get(ctx, "/kbyg/spots/forecasts/wave", map[string]string{"spotId": spotID, "days": "1"})
	if err != nil {
		return 0, 0, false
	}
	var env struct {
		Associated struct {
			Location struct {
				Lat float64 `json:"lat"`
				Lon float64 `json:"lon"`
			} `json:"location"`
		} `json:"associated"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return 0, 0, false
	}
	if env.Associated.Location.Lat != 0 || env.Associated.Location.Lon != 0 {
		return env.Associated.Location.Lat, env.Associated.Location.Lon, true
	}
	return 0, 0, false
}

type sunlightPoint struct {
	Sunrise int64 `json:"sunrise"`
	Sunset  int64 `json:"sunset"`
}

func fetchSunlight(ctx context.Context, c *client.Client, spotID string, days int) ([]sunlightPoint, error) {
	params := map[string]string{"spotId": spotID}
	if days > 0 {
		params["days"] = fmt.Sprintf("%d", days)
	}
	data, err := c.Get(ctx, "/kbyg/spots/forecasts/sunlight", params)
	if err != nil {
		return nil, err
	}
	var env struct {
		Data struct {
			Sunlight []sunlightPoint `json:"sunlight"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("parsing sunlight forecast: %w", err)
	}
	return env.Data.Sunlight, nil
}

// isDaylight reports whether ts falls within any sunrise..sunset window. It
// fails OPEN: when there is no usable sunlight data — empty slice, or every
// entry has a zero sunrise/sunset — it returns true so the caller does not
// silently drop every point. Only when at least one valid window exists is ts
// actually tested against it.
func isDaylight(ts int64, sun []sunlightPoint) bool {
	sawValid := false
	for _, s := range sun {
		if s.Sunrise == 0 || s.Sunset == 0 {
			continue
		}
		sawValid = true
		if ts >= s.Sunrise && ts <= s.Sunset {
			return true
		}
	}
	// No usable sunlight windows → fail open rather than hiding every window.
	return !sawValid
}

// nextTides returns up to n tide extremes (HIGH/LOW) at or after `from`.
func nextTides(tides []tidePoint, from int64, n int) []tidePoint {
	var out []tidePoint
	sort.Slice(tides, func(i, j int) bool { return tides[i].Timestamp < tides[j].Timestamp })
	for _, t := range tides {
		if t.Timestamp < from {
			continue
		}
		if t.Type == "NORMAL" {
			continue
		}
		out = append(out, t)
		if len(out) >= n {
			break
		}
	}
	return out
}
