package haven

import (
	"fmt"
	"math"
	"sort"
	"time"
)

type Observation struct {
	SnapshotID int64     `json:"snapshot_id"`
	LocationID int64     `json:"location_id"`
	FetchedAt  time.Time `json:"fetched_at"`
	Complete   bool      `json:"complete"`
}

func observation(s Snapshot) Observation {
	return Observation{s.ID, s.LocationID, s.FetchedAt, s.Complete}
}
func observations(ss []Snapshot) ([]Observation, error) {
	out := []Observation{}
	seen := map[int64]bool{}
	if len(ss) == 0 {
		return out, fmt.Errorf("no saved observations; refresh locations first")
	}
	for _, s := range ss {
		if err := validateSnapshot(s); err != nil {
			return out, err
		}
		if seen[s.LocationID] {
			return out, fmt.Errorf("duplicate location %d; select one observation per location", s.LocationID)
		}
		seen[s.LocationID] = true
		out = append(out, observation(s))
	}
	return out, nil
}

type CompareRow struct {
	LocationID int64  `json:"location_id"`
	Items      []Item `json:"items"`
	Ambiguous  bool   `json:"ambiguous"`
	Status     string `json:"status"`
}

func Compare(ss []Snapshot, name string) (any, error) {
	obs, err := observations(ss)
	if err != nil {
		return nil, err
	}
	key := normalized(name)
	if key == "" {
		return nil, fmt.Errorf("item name must not be empty")
	}
	rows := []CompareRow{}
	for _, s := range ss {
		r := CompareRow{LocationID: s.LocationID, Items: []Item{}, Status: "missing"}
		for _, i := range s.Items {
			if normalized(i.Name) == key {
				r.Items = append(r.Items, i)
			}
		}
		r.Ambiguous = len(r.Items) > 1
		if len(r.Items) > 0 {
			r.Status = "matched"
		} else if !s.Complete {
			r.Status = "unknown_in_incomplete_snapshot"
		}
		rows = append(rows, r)
	}
	return map[string]any{"name": name, "observations": obs, "rows": rows, "note": "Exact case/whitespace-normalized name matches only; variants are preserved and not assumed equivalent. Prices are base prices in cents."}, nil
}

type CommonLocation struct {
	LocationID int64  `json:"location_id"`
	Status     string `json:"status"`
	Items      []Item `json:"items"`
	Ambiguous  bool   `json:"ambiguous"`
}
type CommonRow struct {
	Name      string           `json:"name"`
	Locations []CommonLocation `json:"locations"`
}

func Common(ss []Snapshot) (any, error) {
	obs, err := observations(ss)
	if err != nil {
		return nil, err
	}
	if len(ss) < 2 {
		return nil, fmt.Errorf("common requires at least two different locations")
	}
	names := map[string]bool{}
	groups := make([]map[string][]Item, len(ss))
	for n, s := range ss {
		groups[n] = map[string][]Item{}
		for _, i := range s.Items {
			k := normalized(i.Name)
			names[k] = true
			groups[n][k] = append(groups[n][k], i)
		}
	}
	keys := []string{}
	for k := range names {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	rows := []CommonRow{}
	excluded := []CommonRow{}
	for _, name := range keys {
		row := CommonRow{Name: name, Locations: []CommonLocation{}}
		all := true
		for n, s := range ss {
			items := groups[n][name]
			if items == nil {
				items = []Item{}
			}
			r := CommonLocation{LocationID: s.LocationID, Status: "missing", Items: items, Ambiguous: len(items) > 1}
			available, unknown := false, false
			for _, i := range items {
				if usable(i) {
					available = true
				}
				if !i.Hidden && !i.Disabled && i.Available == nil {
					unknown = true
				}
			}
			switch {
			case available:
				r.Status = "available"
			case unknown:
				r.Status = "unknown"
			case len(items) > 0:
				r.Status = "unavailable"
			case !s.Complete:
				r.Status = "unknown"
			}
			if !available {
				all = false
			}
			row.Locations = append(row.Locations, r)
		}
		if all {
			rows = append(rows, row)
		} else {
			excluded = append(excluded, row)
		}
	}
	return map[string]any{"observations": obs, "rows": rows, "excluded": excluded, "note": "Availability is from saved observations, not a live ordering guarantee. Matching names do not establish identical recipes or options."}, nil
}

type SubtotalLine struct {
	ItemID         int64  `json:"item_id"`
	Name           string `json:"name"`
	Quantity       int64  `json:"quantity"`
	UnitPriceCents int64  `json:"unit_price_cents"`
	TotalCents     int64  `json:"total_cents"`
}

func Subtotal(s Snapshot, quantities map[int64]int64) (any, error) {
	if err := validateSnapshot(s); err != nil {
		return nil, err
	}
	if len(quantities) == 0 {
		return nil, fmt.Errorf("at least one item id and quantity is required")
	}
	byID := map[int64]Item{}
	for _, i := range s.Items {
		byID[i.ID] = i
	}
	ids := []int64{}
	for id := range quantities {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	rows := []SubtotalLine{}
	var total int64
	for _, id := range ids {
		q := quantities[id]
		if q < 1 || q > 10000 {
			return nil, fmt.Errorf("item %d quantity must be between 1 and 10000", id)
		}
		i, ok := byID[id]
		if !ok {
			return nil, fmt.Errorf("item %d is absent from location %d snapshot", id, s.LocationID)
		}
		if !usable(i) {
			return nil, fmt.Errorf("item %d is hidden, disabled, unavailable or has unknown availability", id)
		}
		if i.PriceCents > math.MaxInt64/q {
			return nil, fmt.Errorf("item %d subtotal overflows integer cents", id)
		}
		line := i.PriceCents * q
		if total > math.MaxInt64-line {
			return nil, fmt.Errorf("subtotal overflows integer cents")
		}
		total += line
		rows = append(rows, SubtotalLine{id, i.Name, q, i.PriceCents, line})
	}
	return map[string]any{"observation": observation(s), "items": rows, "subtotal_cents": total, "currency": "USD", "note": "Base-price estimate only; excludes modifiers, tax, tips and fees. Does not create an order or checkout total."}, nil
}

type Change struct {
	ItemID int64  `json:"item_id"`
	Kind   string `json:"kind"`
	Before *Item  `json:"before"`
	After  *Item  `json:"after"`
}

func Changes(before, after Snapshot) (any, error) {
	if err := validateSnapshot(before); err != nil {
		return nil, err
	}
	if err := validateSnapshot(after); err != nil {
		return nil, err
	}
	if before.LocationID != after.LocationID {
		return nil, fmt.Errorf("changes requires snapshots from the same location")
	}
	if !before.Complete || !after.Complete {
		return nil, fmt.Errorf("changes requires two complete observations; partial data cannot establish removals")
	}
	if !after.FetchedAt.After(before.FetchedAt) {
		return nil, fmt.Errorf("after observation must be newer than before; refresh to establish history")
	}
	old, newItems := map[int64]Item{}, map[int64]Item{}
	ids := map[int64]bool{}
	for _, i := range before.Items {
		old[i.ID] = i
		ids[i.ID] = true
	}
	for _, i := range after.Items {
		newItems[i.ID] = i
		ids[i.ID] = true
	}
	ordered := []int64{}
	for id := range ids {
		ordered = append(ordered, id)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	rows := []Change{}
	for _, id := range ordered {
		a, aok := old[id]
		b, bok := newItems[id]
		switch {
		case !aok:
			rows = append(rows, Change{id, "added", nil, &b})
		case !bok:
			rows = append(rows, Change{id, "removed", &a, nil})
		default:
			if a.Name != b.Name {
				rows = append(rows, Change{id, "renamed", &a, &b})
			}
			if a.PriceCents != b.PriceCents {
				rows = append(rows, Change{id, "price_changed", &a, &b})
			}
			if !sameBool(a.Available, b.Available) || a.Disabled != b.Disabled || a.Hidden != b.Hidden {
				rows = append(rows, Change{id, "availability_changed", &a, &b})
			}
		}
	}
	return map[string]any{"before": observation(before), "after": observation(after), "rows": rows}, nil
}

type NearbyRow struct {
	Location      Location `json:"location"`
	DistanceMiles float64  `json:"distance_miles"`
}

func Nearby(locations []Location, lat, lon float64, limit int) (any, error) {
	if !validCoord(lat, lon) {
		return nil, fmt.Errorf("latitude must be finite within -90..90 and longitude within -180..180")
	}
	if limit < 1 || limit > 1000 {
		return nil, fmt.Errorf("limit must be between 1 and 1000")
	}
	rows := []NearbyRow{}
	skipped := []int64{}
	for _, l := range locations {
		if l.Latitude == nil || l.Longitude == nil || !validCoord(*l.Latitude, *l.Longitude) {
			skipped = append(skipped, l.ID)
			continue
		}
		r := math.Pi / 180
		dlat := (*l.Latitude - lat) * r
		dlon := (*l.Longitude - lon) * r
		a := math.Sin(dlat/2)*math.Sin(dlat/2) + math.Cos(lat*r)*math.Cos(*l.Latitude*r)*math.Sin(dlon/2)*math.Sin(dlon/2)
		a = math.Max(0, math.Min(1, a))
		distance := 3958.7613 * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
		rows = append(rows, NearbyRow{l, distance})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].DistanceMiles == rows[j].DistanceMiles {
			return rows[i].Location.ID < rows[j].Location.ID
		}
		return rows[i].DistanceMiles < rows[j].DistanceMiles
	})
	if len(rows) > limit {
		rows = rows[:limit]
	}
	var note = "Straight-line miles from supplied coordinates; not driving distance or travel time. Locations without valid coordinates are skipped."
	if len(locations) == 0 {
		note = "No saved locations. Run haven refresh first. Distances use only locally saved coordinates."
	}
	return map[string]any{"latitude": lat, "longitude": lon, "rows": rows, "skipped_location_ids": skipped, "note": note}, nil
}
