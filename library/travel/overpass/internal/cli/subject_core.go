// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/overpass/internal/subjects"

	"github.com/spf13/cobra"
)

// nominatimURL is the OSM geocoder. Keyless, but it requires a descriptive
// User-Agent and permits roughly one request per second.
const nominatimURL = "https://nominatim.openstreetmap.org/search"

// resolved is a geocoded place.
type resolved struct {
	Label     string  `json:"label"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// geocode resolves a place name to a single point.
//
// Nominatim orders by relevance worldwide, so the resolved label is always
// reported back: a query for a common place name can land in another country,
// and the user needs to see that rather than silently searching the wrong
// continent.
func geocode(ctx context.Context, query, country string, timeout time.Duration) (resolved, error) {
	if strings.TrimSpace(query) == "" {
		return resolved{}, usageErr(fmt.Errorf("give a place name"))
	}
	q := url.Values{
		"q": {query}, "format": {"json"}, "limit": {"5"},
	}
	if country != "" {
		q.Set("countrycodes", country)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, nominatimURL+"?"+q.Encode(), nil)
	if err != nil {
		return resolved{}, err
	}
	req.Header.Set("User-Agent", subjects.UserAgent)

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return resolved{}, apiErr(fmt.Errorf("geocoding %q: %w", query, err))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return resolved{}, apiErr(fmt.Errorf("geocoding %q: HTTP %d", query, resp.StatusCode))
	}

	var hits []struct {
		DisplayName string `json:"display_name"`
		Lat         string `json:"lat"`
		Lon         string `json:"lon"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&hits); err != nil {
		return resolved{}, apiErr(fmt.Errorf("parsing geocoding response: %w", err))
	}
	if len(hits) == 0 {
		return resolved{}, notFoundErr(fmt.Errorf(
			"no place matched %q — add a region (\"San Pedro, CA\") or pass --country", query))
	}

	var lat, lon float64
	if _, err := fmt.Sscanf(hits[0].Lat, "%g", &lat); err != nil {
		return resolved{}, apiErr(fmt.Errorf("unreadable latitude %q", hits[0].Lat))
	}
	if _, err := fmt.Sscanf(hits[0].Lon, "%g", &lon); err != nil {
		return resolved{}, apiErr(fmt.Errorf("unreadable longitude %q", hits[0].Lon))
	}
	label := hits[0].DisplayName
	if len(label) > 80 {
		label = label[:80] + "..."
	}
	return resolved{Label: label, Latitude: lat, Longitude: lon}, nil
}

// searchOpts are the flags shared by the commands that run an Overpass query.
type searchOpts struct {
	at      string
	lat     float64
	lon     float64
	country string
	typ     string
	limit   int
	timeout int
}

// resolveOrigin turns --at or explicit coordinates into a point.
//
// Presence is tested with Flag.Changed rather than a 0,0 sentinel. With the
// sentinel, omitting --latitude while passing --longitude silently searched
// around 0,-118.24 — the South Pacific — and reported "0 found" as if the
// place were simply empty.
func (o *searchOpts) resolveOrigin(ctx context.Context, cmd *cobra.Command, flags *rootFlags) (resolved, error) {
	hasLat := cmd.Flags().Changed("latitude")
	hasLon := cmd.Flags().Changed("longitude")
	if o.at != "" {
		r, err := geocode(ctx, o.at, o.country, flags.timeout)
		if err != nil {
			return resolved{}, err
		}
		if !flags.asJSON && !flags.quiet {
			fmt.Fprintf(cmd.ErrOrStderr(), "resolved %q to %s\n", o.at, r.Label)
		}
		return r, nil
	}
	if !hasLat && !hasLon {
		return resolved{}, usageErr(fmt.Errorf("name a place with --at, or give --latitude and --longitude"))
	}
	if hasLat != hasLon {
		return resolved{}, usageErr(fmt.Errorf(
			"--latitude and --longitude must be given together; one alone would search a point on the equator or prime meridian"))
	}
	if o.lat < -90 || o.lat > 90 || math.IsNaN(o.lat) {
		return resolved{}, usageErr(fmt.Errorf("--latitude %v is out of range (-90..90)", o.lat))
	}
	if o.lon < -180 || o.lon > 180 || math.IsNaN(o.lon) {
		return resolved{}, usageErr(fmt.Errorf("--longitude %v is out of range (-180..180)", o.lon))
	}
	return resolved{
		Label:    fmt.Sprintf("%.4f,%.4f", o.lat, o.lon),
		Latitude: o.lat, Longitude: o.lon,
	}, nil
}

// runQuery executes a built query with mirror failover and parses the result.
//
// The failover loop gets its own deadline rather than inheriting the caller's.
// --timeout is a per-request budget; sharing one deadline across every mirror
// means a single slow host consumes the whole allowance and the remaining
// mirrors are never tried, which defeats the point of having them.
//
// The third return value is Overpass's own truncation remark, empty when the
// answer is complete. It is returned rather than printed: `near --json`,
// `route --json` and `geojson` all write a machine-readable document to stdout,
// and prose written ahead of it makes that document unparseable.
func runQuery(ctx context.Context, cmd *cobra.Command, flags *rootFlags, query string, origin *subjects.Area) ([]subjects.Subject, []subjects.Attempt, string, error) {
	perMirror := flags.timeout
	if perMirror <= 0 || perMirror > 45*time.Second {
		// Cap the per-mirror wait. An Overpass instance that has not answered
		// in 45 seconds is overloaded, and waiting the full --timeout on each
		// of three mirrors turns one slow query into a four-minute hang.
		perMirror = 45 * time.Second
	}
	overall := time.Duration(len(subjects.Mirrors)) * perMirror
	if overall > 150*time.Second {
		overall = 150 * time.Second
	}
	// WithoutCancel drops the caller's per-request DEADLINE, which is the point
	// of the paragraph above — but it drops cancellation with it, so a Ctrl-C
	// or an MCP client hanging up would leave the loop working its way through
	// every mirror for up to `overall`. Re-attach the command's root context,
	// which carries the interrupt but not the per-request deadline.
	runCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), overall)
	defer cancel()
	// Cobra leaves Context() nil until ExecuteC runs, and RunE is reachable
	// directly from tests and from the MCP walker.
	if root := cmd.Context(); root != nil {
		stop := context.AfterFunc(root, cancel)
		defer stop()
	}

	runner := subjects.NewRunner(perMirror)
	body, attempts, err := runner.Run(runCtx, query)
	if err != nil {
		// Report what each mirror actually said; a bare failure invites the
		// user to blame their query when the hosts were simply overloaded.
		for _, a := range attempts {
			fmt.Fprintf(cmd.ErrOrStderr(), "  %s: %s\n", a.Mirror, a.Err)
		}
		return nil, attempts, "", apiErr(err)
	}
	if len(attempts) > 1 && !flags.quiet {
		fmt.Fprintf(cmd.ErrOrStderr(), "note: %d mirror(s) were unavailable; served by %s\n",
			len(attempts)-1, attempts[len(attempts)-1].Mirror)
	}
	subs, remark, err := subjects.ParseElements(body, origin)
	if err != nil {
		return nil, attempts, "", apiErr(err)
	}
	if remark != "" {
		// Overpass truncated server-side. stderr only — every caller renders
		// the remark itself, into whichever shape its stdout is carrying.
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: Overpass returned partial results — %s\n", remark)
	}
	sort.SliceStable(subs, func(i, j int) bool {
		if subs[i].DistanceKM != subs[j].DistanceKM {
			return subs[i].DistanceKM < subs[j].DistanceKM
		}
		return subs[i].Name < subs[j].Name
	})
	return subs, attempts, remark, nil
}

// partialNote is the human-readable line for an Overpass truncation remark.
// Callers writing prose to stdout print it; callers writing JSON or GeoJSON
// carry the raw remark in the document instead.
func partialNote(remark string) string {
	return fmt.Sprintf("note: these results are INCOMPLETE (%s); narrow the radius or raise --query-timeout", remark)
}

// renderSubjects prints a result table. remark carries Overpass's truncation
// note, printed above the table so the count is never read as complete.
func renderSubjects(cmd *cobra.Command, flags *rootFlags, subs []subjects.Subject, ty subjects.Type, where, remark string) error {
	out := cmd.OutOrStdout()
	if remark != "" {
		fmt.Fprintln(out, partialNote(remark))
	}
	fmt.Fprintln(out, bold(fmt.Sprintf("%d %s near %s", len(subs), pluralizeType(ty.Name, len(subs)), where)))
	if len(subs) == 0 {
		fmt.Fprintln(out, "  nothing found. OpenStreetMap coverage is volunteer-contributed and uneven;")
		fmt.Fprintf(out, "  try a wider radius, or check the tags with: overpass-pp-cli types --type %s\n", ty.Name)
		return nil
	}
	rows := make([][]string, 0, len(subs))
	for _, s := range subs {
		d := ""
		if s.DistanceKM > 0 {
			d = fmt.Sprintf("%.1f km", s.DistanceKM)
		}
		rows = append(rows, []string{
			s.Name, d, fmt.Sprintf("%.5f,%.5f", s.Latitude, s.Longitude), s.URL,
		})
	}
	if err := flags.printTable(cmd, []string{"NAME", "DISTANCE", "COORDS", "OSM"}, rows); err != nil {
		return err
	}
	if ty.Note != "" {
		fmt.Fprintf(out, "\nnote: %s\n", ty.Note)
	}
	return nil
}

// pluralizeType makes a count line read as English. The type name is a
// taxonomy key (water_tower, viewpoint), so "15 mural near Los Angeles" is
// what a naive format produces. Handles the endings the catalogue actually
// uses; anything unrecognised just takes an s.
func pluralizeType(name string, n int) string {
	if n == 1 {
		return name
	}
	switch {
	case strings.HasSuffix(name, "s"), strings.HasSuffix(name, "x"),
		strings.HasSuffix(name, "ch"), strings.HasSuffix(name, "sh"):
		return name + "es"
	case strings.HasSuffix(name, "y") && !strings.HasSuffix(name, "ay") &&
		!strings.HasSuffix(name, "ey") && !strings.HasSuffix(name, "oy"):
		return strings.TrimSuffix(name, "y") + "ies"
	default:
		return name + "s"
	}
}

// typeFlagHelp is shared so every command describes --type identically.
var typeFlagHelp = "Subject type to find; run `types` for the full list (" +
	strings.Join(firstFew(subjects.Names(), 5), ", ") + ", ...)"

func firstFew(v []string, n int) []string {
	if len(v) <= n {
		return v
	}
	return v[:n]
}
