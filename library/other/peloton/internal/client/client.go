// Copyright 2026 Todd Dailey. Licensed under Apache-2.0. See LICENSE.

// Package client wraps the unofficial Peloton API. There is no public API;
// these endpoints are reverse-engineered from members.onepeloton.com network
// traffic and may change without notice.
package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const (
	defaultBaseURL = "https://api.onepeloton.com/api"
	defaultTimeout = 30 * time.Second
)

// Client holds the bearer token + HTTP client. Cheap to construct; safe to
// share across goroutines (http.Client is goroutine-safe).
type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

// New returns a Client with the default 30s timeout. Pass an empty token only
// for unauthenticated probes; every documented endpoint requires auth.
func New(token string) *Client {
	return &Client{
		BaseURL: defaultBaseURL,
		Token:   token,
		HTTP:    &http.Client{Timeout: defaultTimeout},
	}
}

// Workout is the trimmed shape we surface — full Peloton response has 60+
// fields, most of which are SPA implementation noise (image_url variants,
// canonical permalinks, etc.). Keep the fields agents actually use.
type Workout struct {
	ID                string  `json:"id"`
	RideID            string  `json:"ride_id"`      // links to /api/ride/{id}/details for playlist
	WorkoutDate       string  `json:"workout_date"` // YYYY-MM-DD (computed from start_time)
	WorkoutTime       string  `json:"workout_time"` // HH:MM:SS (computed from start_time)
	FitnessDiscipline string  `json:"fitness_discipline"`
	Title             string  `json:"title"`
	Instructor        string  `json:"instructor"`
	DurationSeconds   int     `json:"duration_seconds"`
	TotalOutputKJ     float64 `json:"total_output_kj"`
	Calories          int     `json:"calories"`
	AvgHeartRate      int     `json:"avg_heart_rate"`
	MaxHeartRate      int     `json:"max_heart_rate"`
}

// rawWorkout is the on-the-wire envelope. Peloton returns start_time as a
// Unix epoch int (seconds), total_work in joules (we convert to kJ), and
// nests the title/instructor under .ride / .ride.instructor.
type rawWorkout struct {
	ID                string  `json:"id"`
	StartTime         int64   `json:"start_time"`
	FitnessDiscipline string  `json:"fitness_discipline"`
	TotalWork         float64 `json:"total_work"`
	Calories          int     `json:"calories"`
	AvgHeartRate      int     `json:"avg_heart_rate"`
	MaxHeartRate      int     `json:"max_heart_rate"`
	Ride struct {
		ID         string `json:"id"`
		Title      string `json:"title"`
		Duration   int    `json:"duration"`
		Instructor struct {
			Name string `json:"name"`
		} `json:"instructor"`
	} `json:"ride"`
}

// rawList is the pagination envelope every list-style Peloton endpoint uses.
type rawList struct {
	Data []json.RawMessage `json:"data"`
}

// Me returns the current user's id + username — every workout endpoint is
// scoped to user_id, so callers usually fetch this once per session.
func (c *Client) Me() (id, username string, err error) {
	var resp struct {
		ID       string `json:"id"`
		Username string `json:"username"`
	}
	if err := c.get("/me", nil, &resp); err != nil {
		return "", "", err
	}
	return resp.ID, resp.Username, nil
}

// GetWorkout fetches a single workout by id with the same ride/instructor
// joins ListWorkouts uses, so the returned Workout is shape-identical to a
// list element. Returns an AuthError on 401/403 and a generic API error
// (which the CLI maps to CodeNotFound) on 404.
func (c *Client) GetWorkout(id string) (Workout, error) {
	if id == "" {
		return Workout{}, fmt.Errorf("GetWorkout: id required")
	}
	params := url.Values{"joins": {"ride,ride.instructor"}}
	var rw rawWorkout
	if err := c.get("/workout/"+id, params, &rw); err != nil {
		return Workout{}, err
	}
	return fromRaw(rw), nil
}

// Song is the trimmed playlist-entry shape. The on-the-wire object also
// carries image_urls, popularity, and a small marketing payload; agents
// looking for "what songs played in this ride" only need title + artist
// + liked + the index/offset for ordering.
type Song struct {
	ID              string   `json:"id"`
	Title           string   `json:"title"`
	Artists         []string `json:"artists"`
	Album           string   `json:"album"`
	Genres          []string `json:"genres"`
	Liked           bool     `json:"liked"`
	Index           int      `json:"index"`
	StartTimeOffset int      `json:"start_time_offset"` // seconds into the ride
}

// RideDetails is the trimmed /api/ride/{id}/details response — playlist +
// the ride's id/title/duration so callers don't have to round-trip a second
// time to label the songs. Full upstream payload has 20+ top-level keys
// covering segments, target metrics, instructor cues, etc.; we surface
// only what the discoveries / sync flows need.
type RideDetails struct {
	RideID   string `json:"ride_id"`
	Title    string `json:"title"`
	Duration int    `json:"duration"`
	Songs    []Song `json:"songs"`
}

// rawRideDetails matches the upstream envelope.
type rawRideDetails struct {
	Ride struct {
		ID       string `json:"id"`
		Title    string `json:"title"`
		Duration int    `json:"duration"`
	} `json:"ride"`
	Playlist struct {
		Songs []struct {
			ID      string `json:"id"`
			Title   string `json:"title"`
			Artists []struct {
				ArtistName string `json:"artist_name"`
			} `json:"artists"`
			Album struct {
				Name string `json:"name"`
			} `json:"album"`
			Genres []struct {
				Name string `json:"name"`
			} `json:"genres"`
			Liked           bool `json:"liked"`
			Index           int  `json:"index"`
			StartTimeOffset int  `json:"start_time_offset"`
		} `json:"songs"`
	} `json:"playlist"`
}

// GetRideDetails fetches /api/ride/{id}/details and projects the playlist
// to the trimmed Song shape. Some on-demand rides have empty playlists
// (instructor talk-only); callers should treat zero songs as normal, not
// an error.
func (c *Client) GetRideDetails(rideID string) (RideDetails, error) {
	if rideID == "" {
		return RideDetails{}, fmt.Errorf("GetRideDetails: rideID required")
	}
	var raw rawRideDetails
	if err := c.get("/ride/"+rideID+"/details", nil, &raw); err != nil {
		return RideDetails{}, err
	}
	out := RideDetails{
		RideID:   raw.Ride.ID,
		Title:    raw.Ride.Title,
		Duration: raw.Ride.Duration,
		Songs:    make([]Song, 0, len(raw.Playlist.Songs)),
	}
	for _, s := range raw.Playlist.Songs {
		artists := make([]string, 0, len(s.Artists))
		for _, a := range s.Artists {
			if a.ArtistName != "" {
				artists = append(artists, a.ArtistName)
			}
		}
		genres := make([]string, 0, len(s.Genres))
		for _, g := range s.Genres {
			if g.Name != "" {
				genres = append(genres, g.Name)
			}
		}
		out.Songs = append(out.Songs, Song{
			ID:              s.ID,
			Title:           s.Title,
			Artists:         artists,
			Album:           s.Album.Name,
			Genres:          genres,
			Liked:           s.Liked,
			Index:           s.Index,
			StartTimeOffset: s.StartTimeOffset,
		})
	}
	return out, nil
}

// NotFoundError signals a 404 — used by GetWorkout and other id-scoped lookups.
type NotFoundError struct {
	Path string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("Peloton 404 on %s — no such resource", e.Path)
}

// ListWorkouts pulls up to `limit` workouts for `userID`, newest-first. Stops
// paginating once a page is fully contained in `knownIDs` — Peloton returns
// reverse-chronological, so the moment we've seen every ID on a page, the
// rest of the feed is older still and there's nothing new to fetch.
//
// Pass `knownIDs=nil` to fetch unconditionally (e.g. first sync).
func (c *Client) ListWorkouts(userID string, limit int, knownIDs map[string]struct{}) ([]Workout, error) {
	if userID == "" {
		return nil, fmt.Errorf("ListWorkouts: userID required")
	}
	if limit <= 0 {
		limit = 50
	}
	const pageSize = 20

	var out []Workout
	for page := 0; len(out) < limit; page++ {
		params := url.Values{
			"page":  {strconv.Itoa(page)},
			"limit": {strconv.Itoa(pageSize)},
			"joins": {"ride,ride.instructor"},
		}
		var raw rawList
		if err := c.get("/user/"+userID+"/workouts", params, &raw); err != nil {
			return out, err
		}
		if len(raw.Data) == 0 {
			break
		}
		allKnown := knownIDs != nil
		for _, b := range raw.Data {
			var rw rawWorkout
			if err := json.Unmarshal(b, &rw); err != nil {
				continue
			}
			if knownIDs != nil {
				if _, seen := knownIDs[rw.ID]; !seen {
					allKnown = false
				}
			}
			out = append(out, fromRaw(rw))
			if len(out) >= limit {
				break
			}
		}
		if allKnown {
			// Every workout on this page was already known — the rest of the
			// feed is older still.
			break
		}
		if len(raw.Data) < pageSize {
			break
		}
	}
	return out, nil
}

func fromRaw(rw rawWorkout) Workout {
	t := time.Unix(rw.StartTime, 0)
	dur := rw.Ride.Duration
	w := Workout{
		ID:                rw.ID,
		RideID:            rw.Ride.ID,
		WorkoutDate:       t.Format("2006-01-02"),
		WorkoutTime:       t.Format("15:04:05"),
		FitnessDiscipline: rw.FitnessDiscipline,
		Title:             rw.Ride.Title,
		Instructor:        rw.Ride.Instructor.Name,
		DurationSeconds:   dur,
		TotalOutputKJ:     rw.TotalWork / 1000.0, // Peloton ships joules
		Calories:          rw.Calories,
		AvgHeartRate:      rw.AvgHeartRate,
		MaxHeartRate:      rw.MaxHeartRate,
	}
	return w
}

// get is the shared GET helper — adds the bearer header, decodes JSON,
// distinguishes auth failures (401/403) so callers can return CodeAuth.
func (c *Client) get(path string, params url.Values, out any) error {
	u := c.BaseURL + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return fmt.Errorf("building request for %s: %w", path, err)
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("GET %s: %w", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return &AuthError{Status: resp.StatusCode, Path: path}
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return &RateLimitError{Path: path}
	}
	if resp.StatusCode == http.StatusNotFound {
		return &NotFoundError{Path: path}
	}
	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(resp.Body)
		snip := string(body)
		if len(snip) > 300 {
			snip = snip[:300] + "…"
		}
		return fmt.Errorf("GET %s: status %d: %s", path, resp.StatusCode, snip)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decoding %s response: %w", path, err)
	}
	return nil
}

// AuthError signals a 401/403 — caller should re-run `auth login`.
type AuthError struct {
	Status int
	Path   string
}

func (e *AuthError) Error() string {
	return fmt.Sprintf("Peloton auth failed (HTTP %d on %s) — re-run `peloton-pp-cli auth login`", e.Status, e.Path)
}

// RateLimitError signals a 429.
type RateLimitError struct{ Path string }

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("Peloton rate-limited on %s — back off and retry", e.Path)
}
