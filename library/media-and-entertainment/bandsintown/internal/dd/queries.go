package dd

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"
	"time"
)

// AddTracked inserts a row into the watchlist; existing rows are left alone.
func AddTracked(ctx context.Context, db *sql.DB, name, tier string) error {
	_, err := db.ExecContext(ctx, `
		INSERT OR IGNORE INTO dd_tracked_artists (name, tier, added_at)
		VALUES (?,?,CURRENT_TIMESTAMP)
	`, name, tier)
	return err
}

// RemoveTracked deletes a row by name (case-sensitive).
func RemoveTracked(ctx context.Context, db *sql.DB, name string) (bool, error) {
	res, err := db.ExecContext(ctx, `DELETE FROM dd_tracked_artists WHERE name=?`, name)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// ListTracked returns every watchlist row, ordered by name.
func ListTracked(ctx context.Context, db *sql.DB) ([]TrackedArtist, error) {
	rows, err := db.QueryContext(ctx, `SELECT name, COALESCE(mbid,''), COALESCE(tier,''), added_at FROM dd_tracked_artists ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []TrackedArtist{}
	for rows.Next() {
		var t TrackedArtist
		if err := rows.Scan(&t.Name, &t.MBID, &t.Tier, &t.AddedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// RouteCandidate is a single row of the route query: a tracked artist with a
// nearby show within the date window, ranked by feasibility.
type RouteCandidate struct {
	Artist       string  `json:"artist"`
	EventID      string  `json:"event_id"`
	Datetime     string  `json:"datetime"`
	DaysGap      int     `json:"days_gap"`
	VenueCity    string  `json:"venue_city"`
	VenueCountry string  `json:"venue_country"`
	VenueName    string  `json:"venue_name"`
	DistanceKM   float64 `json:"distance_km"`
	TrackerCount int     `json:"tracker_count"`
	Score        float64 `json:"score,omitempty"`
	EventURL     string  `json:"event_url,omitempty"`
}

// haversine returns the great-circle distance between two (lat,lng) points in km.
func haversine(lat1, lng1, lat2, lng2 float64) float64 {
	const R = 6371.0 // earth radius km
	dLat := (lat2 - lat1) * math.Pi / 180
	dLng := (lng2 - lng1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLng/2)*math.Sin(dLng/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return R * c
}

// Route finds, for every tracked artist (or all artists if `trackedOnly` is
// false), events near (toLat, toLng) within `windowDays` of `targetDate`.
// If `score` is true, computes a composite feasibility score where lower
// distance + lower gap + higher tracker count is better; otherwise leaves
// Score = 0.
func Route(ctx context.Context, db *sql.DB, toLat, toLng float64, target time.Time, windowDays int, trackedOnly, score bool) ([]RouteCandidate, error) {
	var where []string
	args := []any{}
	if trackedOnly {
		where = append(where, `e.artist_name IN (SELECT name FROM dd_tracked_artists)`)
	}
	where = append(where, `e.venue_lat <> 0 AND e.venue_lng <> 0`)
	// Pre-filter by date range in SQL using julianday math.
	where = append(where, `ABS(julianday(SUBSTR(e.datetime,1,10)) - julianday(?)) <= ?`)
	args = append(args, target.Format("2006-01-02"), windowDays)

	q := `
		SELECT e.artist_name, e.id, e.datetime, e.venue_city, e.venue_country, e.venue_name,
		       e.venue_lat, e.venue_lng, e.url,
		       COALESCE(a.tracker_count, 0)
		FROM dd_events e
		LEFT JOIN dd_artists a ON a.name = e.artist_name
		WHERE ` + strings.Join(where, " AND ") + `
		ORDER BY e.datetime ASC
	`
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []RouteCandidate{}
	for rows.Next() {
		var rc RouteCandidate
		var lat, lng float64
		if err := rows.Scan(&rc.Artist, &rc.EventID, &rc.Datetime, &rc.VenueCity, &rc.VenueCountry, &rc.VenueName, &lat, &lng, &rc.EventURL, &rc.TrackerCount); err != nil {
			return nil, err
		}
		rc.DistanceKM = haversine(toLat, toLng, lat, lng)
		if t, err := time.Parse("2006-01-02T15:04:05", rc.Datetime[:min(len(rc.Datetime), 19)]); err == nil {
			diff := t.Sub(target).Hours() / 24
			if diff < 0 {
				diff = -diff
			}
			rc.DaysGap = int(diff + 0.5)
		}
		out = append(out, rc)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if score {
		// Score normalises each dimension to [0,1]; lower distance + lower gap +
		// higher tracker is better. Equal weights, surface the math so callers can
		// re-rank if they disagree.
		var maxDist, maxTrack float64
		for _, r := range out {
			if r.DistanceKM > maxDist {
				maxDist = r.DistanceKM
			}
			if float64(r.TrackerCount) > maxTrack {
				maxTrack = float64(r.TrackerCount)
			}
		}
		for i := range out {
			distNorm := 0.0
			if maxDist > 0 {
				distNorm = 1.0 - (out[i].DistanceKM / maxDist)
			}
			gapNorm := 1.0
			if windowDays > 0 {
				gapNorm = 1.0 - float64(out[i].DaysGap)/float64(windowDays)
				if gapNorm < 0 {
					gapNorm = 0
				}
			}
			trackNorm := 0.0
			if maxTrack > 0 {
				trackNorm = float64(out[i].TrackerCount) / maxTrack
			}
			out[i].Score = 0.4*gapNorm + 0.3*distNorm + 0.3*trackNorm
		}
	}
	return out, nil
}

// min is a Go 1.21+ built-in; no local definition needed.

// Gap is an empty interval between two consecutive events of a single artist.
type Gap struct {
	Artist        string `json:"artist"`
	GapStart      string `json:"gap_start"`
	GapEnd        string `json:"gap_end"`
	GapDays       int    `json:"gap_days"`
	BeforeCity    string `json:"before_city,omitempty"`
	BeforeCountry string `json:"before_country,omitempty"`
	AfterCity     string `json:"after_city,omitempty"`
	AfterCountry  string `json:"after_country,omitempty"`
}

// Gaps returns intervals between consecutive synced events for an artist where
// gapDays is in [minDays, maxDays]. If regionISO is non-empty, both bounding
// events must be in a country whose ISO-3166-1 country code (or country name)
// matches one of the comma-separated values.
func Gaps(ctx context.Context, db *sql.DB, artist string, minDays, maxDays int, regionISO string) ([]Gap, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT datetime, venue_city, venue_country
		FROM dd_events
		WHERE artist_name = ?
		ORDER BY datetime ASC
	`, artist)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type r struct {
		dt, city, country string
	}
	var rs []r
	for rows.Next() {
		var x r
		if err := rows.Scan(&x.dt, &x.city, &x.country); err != nil {
			return nil, err
		}
		rs = append(rs, x)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	allowed := parseRegionFilter(regionISO)
	out := []Gap{}
	for i := 1; i < len(rs); i++ {
		t1, err := time.Parse("2006-01-02T15:04:05", rs[i-1].dt[:min(len(rs[i-1].dt), 19)])
		if err != nil {
			continue
		}
		t2, err := time.Parse("2006-01-02T15:04:05", rs[i].dt[:min(len(rs[i].dt), 19)])
		if err != nil {
			continue
		}
		days := int(t2.Sub(t1).Hours()/24 + 0.5)
		if days < minDays || days > maxDays {
			continue
		}
		if allowed != nil && !(allowed[rs[i-1].country] && allowed[rs[i].country]) {
			continue
		}
		out = append(out, Gap{
			Artist:        artist,
			GapStart:      t1.Format("2006-01-02"),
			GapEnd:        t2.Format("2006-01-02"),
			GapDays:       days,
			BeforeCity:    rs[i-1].city,
			BeforeCountry: rs[i-1].country,
			AfterCity:     rs[i].city,
			AfterCountry:  rs[i].country,
		})
	}
	return out, nil
}

// parseRegionFilter accepts SEA (built-in shorthand) or a comma-separated list
// of country names / ISO codes; returns nil for "no filter".
func parseRegionFilter(region string) map[string]bool {
	region = strings.TrimSpace(region)
	if region == "" {
		return nil
	}
	if strings.EqualFold(region, "SEA") {
		return map[string]bool{
			"Indonesia": true, "ID": true,
			"Singapore": true, "SG": true,
			"Malaysia": true, "MY": true,
			"Thailand": true, "TH": true,
			"Philippines": true, "PH": true,
			"Vietnam": true, "VN": true,
			"Cambodia": true, "KH": true,
			"Laos": true, "LA": true,
			"Myanmar": true, "MM": true,
			"Brunei": true, "BN": true,
			"East Timor": true, "TL": true,
			"Japan": true, "JP": true,
			"South Korea": true, "KR": true,
			"Taiwan": true, "TW": true,
			"Hong Kong": true, "HK": true,
		}
	}
	out := map[string]bool{}
	for _, s := range strings.Split(region, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			out[s] = true
		}
	}
	return out
}

// CoBill is a single row of the co-bill aggregation.
type CoBill struct {
	Collaborator string `json:"collaborator"`
	Shared       int    `json:"shared"`
	FirstSeen    string `json:"first_seen"`
	LastSeen     string `json:"last_seen"`
}

// LineupCoBill returns artists who appear on lineups of the given artist's
// events, ranked by shared appearances. since (YYYY-MM-DD) trims the event set;
// minShared discards collaborators below the threshold.
func LineupCoBill(ctx context.Context, db *sql.DB, artist, since string, minShared int) ([]CoBill, error) {
	q := `
		SELECT lm.artist_name AS collaborator,
		       COUNT(*) AS shared,
		       MIN(SUBSTR(e.datetime,1,10)) AS first_seen,
		       MAX(SUBSTR(e.datetime,1,10)) AS last_seen
		FROM dd_lineup_members lm
		JOIN dd_events e ON e.id = lm.event_id
		WHERE e.artist_name = ?
		  AND lm.artist_name <> ?
		  AND (? = '' OR SUBSTR(e.datetime,1,10) >= ?)
		GROUP BY lm.artist_name
		HAVING shared >= ?
		ORDER BY shared DESC, last_seen DESC
	`
	rows, err := db.QueryContext(ctx, q, artist, artist, since, since, minShared)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []CoBill{}
	for rows.Next() {
		var c CoBill
		if err := rows.Scan(&c.Collaborator, &c.Shared, &c.FirstSeen, &c.LastSeen); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// TrendRow is one artist's tracker-count delta across a period.
type TrendRow struct {
	Artist           string  `json:"artist"`
	TrackerNow       int     `json:"tracker_now"`
	TrackerThen      int     `json:"tracker_then"`
	Delta            int     `json:"delta"`
	PercentChange    float64 `json:"percent_change"`
	UpcomingEventNow int     `json:"upcoming_event_now"`
	FirstSnapAt      string  `json:"first_snap_at"`
	LastSnapAt       string  `json:"last_snap_at"`
}

// Trend computes tracker_count deltas over the past periodDays for every
// artist that has ≥2 snapshots in the window. Top N rows by delta are returned.
func Trend(ctx context.Context, db *sql.DB, periodDays, topN int) ([]TrendRow, error) {
	if topN <= 0 {
		topN = 20
	}
	cutoff := time.Now().UTC().Add(-time.Duration(periodDays) * 24 * time.Hour).Format(time.RFC3339)
	q := `
		WITH ranked AS (
			SELECT artist_name, tracker_count, upcoming_event_count, snapped_at,
			       ROW_NUMBER() OVER (PARTITION BY artist_name ORDER BY snapped_at ASC)  AS rn_asc,
			       ROW_NUMBER() OVER (PARTITION BY artist_name ORDER BY snapped_at DESC) AS rn_desc
			FROM dd_artist_snapshots
			WHERE snapped_at >= ?
		),
		first AS (SELECT artist_name, tracker_count AS t_then, snapped_at AS first_at FROM ranked WHERE rn_asc=1),
		last  AS (SELECT artist_name, tracker_count AS t_now,  upcoming_event_count AS up_now, snapped_at AS last_at FROM ranked WHERE rn_desc=1)
		SELECT f.artist_name, l.t_now, f.t_then, (l.t_now - f.t_then) AS delta, l.up_now, f.first_at, l.last_at
		FROM first f JOIN last l ON l.artist_name = f.artist_name
		WHERE f.first_at <> l.last_at
		ORDER BY delta DESC
		LIMIT ?
	`
	rows, err := db.QueryContext(ctx, q, cutoff, topN)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []TrendRow{}
	for rows.Next() {
		var r TrendRow
		if err := rows.Scan(&r.Artist, &r.TrackerNow, &r.TrackerThen, &r.Delta, &r.UpcomingEventNow, &r.FirstSnapAt, &r.LastSnapAt); err != nil {
			return nil, err
		}
		if r.TrackerThen > 0 {
			r.PercentChange = float64(r.Delta) / float64(r.TrackerThen) * 100
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// SeaRadarRow groups upcoming SEA events under cities, with tracker tier.
type SeaRadarRow struct {
	City         string `json:"city"`
	Country      string `json:"country"`
	Artist       string `json:"artist"`
	Datetime     string `json:"datetime"`
	VenueName    string `json:"venue_name,omitempty"`
	TrackerCount int    `json:"tracker_count"`
	Tier         string `json:"tier,omitempty"`
	EventURL     string `json:"event_url,omitempty"`
}

// SeaRadar returns every event from tracked artists in a SEA country within
// dateStart..dateEnd. Tier is the watchlist tier the user assigned (free-form);
// if `requireTier` is non-empty, only matching tiers are returned.
func SeaRadar(ctx context.Context, db *sql.DB, dateStart, dateEnd, requireTier string) ([]SeaRadarRow, error) {
	sea := parseRegionFilter("SEA")
	args := []any{}
	placeholders := []string{}
	for c := range sea {
		placeholders = append(placeholders, "?")
		args = append(args, c)
	}

	q := `
		SELECT e.venue_city, e.venue_country, e.artist_name, e.datetime, e.venue_name,
		       COALESCE(a.tracker_count, 0), COALESCE(ta.tier, ''), e.url
		FROM dd_events e
		JOIN dd_tracked_artists ta ON ta.name = e.artist_name
		LEFT JOIN dd_artists a ON a.name = e.artist_name
		WHERE e.venue_country IN (` + strings.Join(placeholders, ",") + `)
		  AND SUBSTR(e.datetime,1,10) >= ?
		  AND SUBSTR(e.datetime,1,10) <= ?
	`
	args = append(args, dateStart, dateEnd)
	if requireTier != "" {
		q += ` AND ta.tier = ?`
		args = append(args, requireTier)
	}
	q += ` ORDER BY e.datetime ASC, e.venue_city`

	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []SeaRadarRow{}
	for rows.Next() {
		var r SeaRadarRow
		if err := rows.Scan(&r.City, &r.Country, &r.Artist, &r.Datetime, &r.VenueName, &r.TrackerCount, &r.Tier, &r.EventURL); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// CountTracked returns the count of watchlist rows for diagnostic display.
func CountTracked(ctx context.Context, db *sql.DB) (int, error) {
	var n int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM dd_tracked_artists`).Scan(&n)
	return n, err
}

// EmptyHint formats a helpful message when a query returns zero rows.
func EmptyHint(reason string) string {
	return fmt.Sprintf("no rows returned: %s. Run `bandsintown-pp-cli track add ...` then `bandsintown-pp-cli pull --tracked` to populate the local store.", reason)
}
