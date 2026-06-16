// Copyright 2026 Abe Diaz (@abe238) and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Shared parsing/flattening helpers for the novel NHC commands (storm,
// advisory, outlook, graphics, gis, brief). These are pure functions over
// already-fetched bytes so they can be unit-tested against the real fixtures
// without any network access. No infrastructure imports beyond the stdlib.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/nhc/internal/cliutil"
)

// userAgent is the descriptive UA every NHC/NWS request must carry. The
// generated client already sends it; the tiny text/HTML fetch helper below
// sets it explicitly for products that the JSON client would not handle.
const userAgent = "nhc-pp-cli (github.com/abe238/nhc-pp-cli)"

// mapServerBase is the ArcGIS REST service root for NHC tropical layers.
// gis links to it but never ingests it.
const mapServerBase = "https://mapservices.weather.noaa.gov/tropical/rest/services/tropical/NHC_tropical_weather/MapServer"

// envelope is the small machine envelope every novel command emits:
// {"source": "<url|fixture>", "fetched_at": "<ISO8601>", "data": {...}}.
type envelope struct {
	Source    string `json:"source"`
	FetchedAt string `json:"fetched_at"`
	Data      any    `json:"data"`
}

// newEnvelope builds an envelope with a current UTC fetched_at stamp.
func newEnvelope(source string, data any) envelope {
	return envelope{
		Source:    source,
		FetchedAt: time.Now().UTC().Format(time.RFC3339),
		Data:      data,
	}
}

// ---------------------------------------------------------------------------
// CurrentStorms.json types
// ---------------------------------------------------------------------------

// product is a nested advisory-product object: {advNum, issuance, url, ...}.
// It is also reused for GIS objects (which carry kmzFile/zipFile/kmlFile/etc.)
// so a single decode captures every shape. All fields are pointers/strings
// with no semantic loss, but the storm-flattening layer projects only the
// keys the contract calls for.
type rawProduct struct {
	AdvNum           string `json:"advNum"`
	Issuance         string `json:"issuance"`
	FileUpdateTime   string `json:"fileUpdateTime"`
	URL              string `json:"url"`
	ZipFile          string `json:"zipFile"`
	KmzFile          string `json:"kmzFile"`
	KmlFile          string `json:"kmlFile"`
	PeakSurgeKMLFile string `json:"peakSurgeKMLFile"`
	// windSpeedProbabilitiesGIS carries threshold-specific kmz files instead
	// of a single kmzFile; the 34 kt file is the canonical link-out.
	KmzFile34kt string `json:"kmzFile34kt"`
	KmzFile50kt string `json:"kmzFile50kt"`
	KmzFile64kt string `json:"kmzFile64kt"`
}

// rawStorm mirrors the CurrentStorms.json storm object. Nested product/GIS
// objects are *rawProduct pointers so a JSON `null` in the feed decodes to a
// nil pointer (preserving the present-but-null contract) rather than a
// zero-value struct.
type rawStorm struct {
	ID               string  `json:"id"`
	BinNumber        string  `json:"binNumber"`
	Name             string  `json:"name"`
	Classification   string  `json:"classification"`
	Intensity        string  `json:"intensity"`
	Pressure         string  `json:"pressure"`
	Latitude         string  `json:"latitude"`
	Longitude        string  `json:"longitude"`
	LatitudeNumeric  float64 `json:"latitudeNumeric"`
	LongitudeNumeric float64 `json:"longitudeNumeric"`
	MovementDir      int     `json:"movementDir"`
	MovementSpeed    int     `json:"movementSpeed"`
	LastUpdate       string  `json:"lastUpdate"`

	PublicAdvisory         *rawProduct `json:"publicAdvisory"`
	ForecastAdvisory       *rawProduct `json:"forecastAdvisory"`
	WindSpeedProbabilities *rawProduct `json:"windSpeedProbabilities"`
	ForecastDiscussion     *rawProduct `json:"forecastDiscussion"`
	ForecastGraphics       *rawProduct `json:"forecastGraphics"`

	ForecastTrack                  *rawProduct `json:"forecastTrack"`
	TrackCone                      *rawProduct `json:"trackCone"`
	WindWatchesWarnings            *rawProduct `json:"windWatchesWarnings"`
	InitialWindExtent              *rawProduct `json:"initialWindExtent"`
	ForecastWindRadiiGIS           *rawProduct `json:"forecastWindRadiiGIS"`
	BestTrackGIS                   *rawProduct `json:"bestTrackGIS"`
	EarliestArrivalTimeTSWindsGIS  *rawProduct `json:"earliestArrivalTimeTSWindsGIS"`
	MostLikelyTimeTSWindsGIS       *rawProduct `json:"mostLikelyTimeTSWindsGIS"`
	WindSpeedProbabilitiesGIS      *rawProduct `json:"windSpeedProbabilitiesGIS"`
	StormSurgeWatchWarningGIS      *rawProduct `json:"stormSurgeWatchWarningGIS"`
	PotentialStormSurgeFloodingGIS *rawProduct `json:"potentialStormSurgeFloodingGIS"`
	PeakSurgeKML                   *rawProduct `json:"peakSurgeKML"`
}

type currentStorms struct {
	ActiveStorms []rawStorm `json:"activeStorms"`
}

// parseCurrentStorms decodes the CurrentStorms.json payload.
func parseCurrentStorms(data []byte) (*currentStorms, error) {
	var cs currentStorms
	if err := json.Unmarshal(data, &cs); err != nil {
		return nil, fmt.Errorf("parsing CurrentStorms feed: %w", err)
	}
	return &cs, nil
}

// activeIDs returns the lowercase ATCF ids of every active storm.
func (cs *currentStorms) activeIDs() []string {
	ids := make([]string, 0, len(cs.ActiveStorms))
	for _, s := range cs.ActiveStorms {
		ids = append(ids, s.ID)
	}
	return ids
}

// findStorm resolves a query to a single storm. A query matching the ATCF id
// case-insensitively wins; otherwise a case-insensitive name match is tried.
// Returns nil when nothing matches.
func (cs *currentStorms) findStorm(query string) *rawStorm {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return nil
	}
	for i := range cs.ActiveStorms {
		if strings.ToLower(cs.ActiveStorms[i].ID) == q {
			return &cs.ActiveStorms[i]
		}
	}
	for i := range cs.ActiveStorms {
		if strings.ToLower(cs.ActiveStorms[i].Name) == q {
			return &cs.ActiveStorms[i]
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// storm <id> flattened output
// ---------------------------------------------------------------------------

// textProduct is the {advNum, url} projection for a text advisory product.
type textProduct struct {
	AdvNum string `json:"advNum"`
	URL    string `json:"url"`
}

// gisLink is a per-GIS-object projection. Only the keys present on the
// underlying object are populated; unused ones are omitted so the link object
// stays tight (a CONE object has kmz+zip, a peakSurge object has kml).
type gisLink struct {
	Kmz string `json:"kmz,omitempty"`
	Zip string `json:"zip,omitempty"`
	Kml string `json:"kml,omitempty"`
}

// stormDetail is the flattened storm <id> payload. Nested product/gis blocks
// are pointer-typed WITHOUT omitempty so a nil maps to JSON `null`
// (present-but-null), satisfying the verified null contract for Isaac/John.
type stormDetail struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	BinNumber      string `json:"binNumber"`
	Classification string `json:"classification"`
	// IntensityKt/PressureMb are *int so an absent or non-numeric source value
	// serializes as JSON null (honest unknown) rather than a fabricated 0,
	// which a safety tool must never present as a real measurement (0 kt =
	// calm). The verified Helene fixture carries both, so they stay non-nil.
	IntensityKt   *int    `json:"intensity_kt"`
	PressureMb    *int    `json:"pressure_mb"`
	Lat           float64 `json:"lat"`
	Lon           float64 `json:"lon"`
	Latitude      string  `json:"latitude"`
	Longitude     string  `json:"longitude"`
	MovementDir   int     `json:"movementDir"`
	MovementSpeed int     `json:"movementSpeed_kt"`
	LastUpdate    string  `json:"lastUpdate"`

	Products struct {
		PublicAdvisory         *textProduct `json:"publicAdvisory"`
		ForecastAdvisory       *textProduct `json:"forecastAdvisory"`
		ForecastDiscussion     *textProduct `json:"forecastDiscussion"`
		WindSpeedProbabilities *textProduct `json:"windSpeedProbabilities"`
		ForecastGraphics       *textProduct `json:"forecastGraphics"`
	} `json:"products"`

	GIS struct {
		TrackCone                 *gisLink `json:"trackCone"`
		ForecastTrack             *gisLink `json:"forecastTrack"`
		ForecastWindRadii         *gisLink `json:"forecastWindRadii"`
		WindWatchesWarnings       *gisLink `json:"windWatchesWarnings"`
		StormSurgeWatchWarningGIS *gisLink `json:"stormSurgeWatchWarningGIS"`
		PotentialStormSurge       *gisLink `json:"potentialStormSurgeFloodingGIS"`
		PeakSurgeKML              *gisLink `json:"peakSurgeKML"`
		WindSpeedProbabilitiesGIS *gisLink `json:"windSpeedProbabilitiesGIS"`
		BestTrackGIS              *gisLink `json:"bestTrackGIS"`
	} `json:"gis"`
}

// atoiPtr coerces a numeric string to *int; empty or non-numeric input yields
// nil so the field serializes as JSON null (honest unknown) instead of a
// fabricated 0. Used for storm vitals where 0 is a meaningful measurement.
func atoiPtr(s string) *int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return nil
	}
	return &n
}

// textProj projects a *rawProduct into {advNum, url}, or nil when absent.
func textProj(p *rawProduct) *textProduct {
	if p == nil {
		return nil
	}
	return &textProduct{AdvNum: p.AdvNum, URL: p.URL}
}

// gisProj projects a *rawProduct into a gisLink, or nil when absent. peakSurge
// objects carry the URL under peakSurgeKMLFile rather than kmlFile.
func gisProj(p *rawProduct) *gisLink {
	if p == nil {
		return nil
	}
	g := &gisLink{Kmz: p.KmzFile, Zip: p.ZipFile, Kml: p.KmlFile}
	if g.Kml == "" && p.PeakSurgeKMLFile != "" {
		g.Kml = p.PeakSurgeKMLFile
	}
	// windSpeedProbabilitiesGIS has no kmzFile; use the 34 kt threshold file.
	if g.Kmz == "" && p.KmzFile34kt != "" {
		g.Kmz = p.KmzFile34kt
	}
	return g
}

// flattenStorm projects a rawStorm into the flattened stormDetail contract,
// preserving the present-but-null behavior for every nested object.
func flattenStorm(s *rawStorm) *stormDetail {
	d := &stormDetail{
		ID:             s.ID,
		Name:           s.Name,
		BinNumber:      s.BinNumber,
		Classification: s.Classification,
		IntensityKt:    atoiPtr(s.Intensity),
		PressureMb:     atoiPtr(s.Pressure),
		Lat:            s.LatitudeNumeric,
		Lon:            s.LongitudeNumeric,
		Latitude:       s.Latitude,
		Longitude:      s.Longitude,
		MovementDir:    s.MovementDir,
		MovementSpeed:  s.MovementSpeed,
		LastUpdate:     s.LastUpdate,
	}
	d.Products.PublicAdvisory = textProj(s.PublicAdvisory)
	d.Products.ForecastAdvisory = textProj(s.ForecastAdvisory)
	d.Products.ForecastDiscussion = textProj(s.ForecastDiscussion)
	d.Products.WindSpeedProbabilities = textProj(s.WindSpeedProbabilities)
	d.Products.ForecastGraphics = textProj(s.ForecastGraphics)

	d.GIS.TrackCone = gisProj(s.TrackCone)
	d.GIS.ForecastTrack = gisProj(s.ForecastTrack)
	d.GIS.ForecastWindRadii = gisProj(s.ForecastWindRadiiGIS)
	d.GIS.WindWatchesWarnings = gisProj(s.WindWatchesWarnings)
	d.GIS.StormSurgeWatchWarningGIS = gisProj(s.StormSurgeWatchWarningGIS)
	d.GIS.PotentialStormSurge = gisProj(s.PotentialStormSurgeFloodingGIS)
	d.GIS.PeakSurgeKML = gisProj(s.PeakSurgeKML)
	d.GIS.WindSpeedProbabilitiesGIS = gisProj(s.WindSpeedProbabilitiesGIS)
	d.GIS.BestTrackGIS = gisProj(s.BestTrackGIS)
	return d
}

// ---------------------------------------------------------------------------
// graphics <id> link-out output
// ---------------------------------------------------------------------------

// graphicsLinks is the per-kind link-out map for graphics <id>. Every field is
// a *string so an absent/null kind serializes as JSON null (the Isaac surge
// contract), never a fabricated URL.
type graphicsLinks struct {
	ConeKmz              *string `json:"cone_kmz"`
	TrackKmz             *string `json:"track_kmz"`
	ForecastTrackZip     *string `json:"forecast_track_zip"`
	WindRadiiKmz         *string `json:"wind_radii_kmz"`
	PeakSurgeKml         *string `json:"peak_surge_kml"`
	WatchWarningKmz      *string `json:"watch_warning_kmz"`
	ForecastGraphicsPage *string `json:"forecast_graphics_page"`
}

type graphicsDetail struct {
	ID    string        `json:"id"`
	Name  string        `json:"name"`
	Links graphicsLinks `json:"links"`
}

// strPtr returns &s when s is non-empty, else nil.
func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// kmzPtr/kmlPtr/zipPtr return the URL for a GIS object kind, or nil when the
// object is absent (preserving the present-but-null contract).
func kmzPtr(p *rawProduct) *string {
	if p == nil {
		return nil
	}
	return strPtr(p.KmzFile)
}

func zipPtr(p *rawProduct) *string {
	if p == nil {
		return nil
	}
	return strPtr(p.ZipFile)
}

func peakSurgePtr(p *rawProduct) *string {
	if p == nil {
		return nil
	}
	return strPtr(p.PeakSurgeKMLFile)
}

func pageURLPtr(p *rawProduct) *string {
	if p == nil {
		return nil
	}
	return strPtr(p.URL)
}

// graphicsFor builds the link-out detail for a storm, honoring a --kind
// selection. kinds is the lowercase set of requested kinds ("cone","track",
// "surge","wind"); empty or "all" means everything. A kind that is requested
// but whose source object is null still emits null (not omitted).
func graphicsFor(s *rawStorm, kinds map[string]bool) *graphicsDetail {
	all := len(kinds) == 0 || kinds["all"]
	d := &graphicsDetail{ID: s.ID, Name: s.Name}
	if all || kinds["cone"] {
		d.Links.ConeKmz = kmzPtr(s.TrackCone)
	}
	if all || kinds["track"] {
		d.Links.TrackKmz = kmzPtr(s.ForecastTrack)
		d.Links.ForecastTrackZip = zipPtr(s.ForecastTrack)
	}
	if all || kinds["wind"] {
		d.Links.WindRadiiKmz = kmzPtr(s.ForecastWindRadiiGIS)
		d.Links.WatchWarningKmz = kmzPtr(s.WindWatchesWarnings)
	}
	if all || kinds["surge"] {
		d.Links.PeakSurgeKml = peakSurgePtr(s.PeakSurgeKML)
	}
	// The forecast-graphics landing page (HTML, never an image) always rides
	// along: it is the agent's pointer to the per-storm graphics page.
	d.Links.ForecastGraphicsPage = pageURLPtr(s.ForecastGraphics)
	return d
}

// downloadableURLs returns the concrete file URLs in a graphicsDetail (skips
// the HTML landing page, which is not a downloadable artifact).
func (d *graphicsDetail) downloadableURLs() []string {
	var out []string
	add := func(p *string) {
		if p != nil && *p != "" {
			out = append(out, *p)
		}
	}
	add(d.Links.ConeKmz)
	add(d.Links.TrackKmz)
	add(d.Links.ForecastTrackZip)
	add(d.Links.WindRadiiKmz)
	add(d.Links.PeakSurgeKml)
	add(d.Links.WatchWarningKmz)
	return out
}

// validateGraphicsURL guards feed-derived URLs before they reach the HTTP/file
// layer or the OS opener. It requires https and an NHC/NWS host (or subdomain)
// so a tampered feed cannot smuggle a file://, smb://, or arbitrary http URL
// into exec.Command/download (blocking SSRF, scheme abuse, and leading-dash arg
// injection, which url.Parse rejects via an empty scheme). Returns the parsed
// URL on success.
func validateGraphicsURL(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, fmt.Errorf("invalid URL %q: %w", raw, err)
	}
	if u.Scheme != "https" {
		return nil, fmt.Errorf("refusing non-https URL %q (scheme %q)", raw, u.Scheme)
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return nil, fmt.Errorf("refusing URL with empty host: %q", raw)
	}
	allowed := host == "noaa.gov" || host == "weather.gov" ||
		strings.HasSuffix(host, ".noaa.gov") || strings.HasSuffix(host, ".weather.gov")
	if !allowed {
		return nil, fmt.Errorf("refusing non-NHC/NWS host %q in URL %q", host, raw)
	}
	return u, nil
}

// safeGraphicsFilename derives a safe destination filename from a validated URL
// path's last segment. It rejects empty/"."/".."/dotfile names, any name
// containing a path separator (so a feed-derived URL cannot write outside the
// target directory), and any name without a filename extension (a directory-style
// URL like /dir/ collapses to "dir" via path.Base; real NHC GIS files always
// carry a .kmz/.zip/.kml/.png extension).
func safeGraphicsFilename(u *url.URL) (string, error) {
	name := path.Base(u.Path)
	if name == "" || name == "." || name == "/" || name == ".." ||
		strings.HasPrefix(name, ".") ||
		strings.ContainsAny(name, "/\\") ||
		!strings.Contains(name, ".") {
		return "", fmt.Errorf("refusing unsafe filename %q derived from URL %q", name, u.String())
	}
	return name, nil
}

// ---------------------------------------------------------------------------
// gis <id> MapServer slot mapping (link-out only)
// ---------------------------------------------------------------------------

// gisLayer is a single ArcGIS REST layer reference.
type gisLayer struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

type gisDetail struct {
	ID      string     `json:"id"`
	Slot    string     `json:"slot"`
	Service string     `json:"service"`
	Layers  []gisLayer `json:"layers"`
}

// slotConeBaseID returns the ArcGIS layer id of the "Forecast Cone" layer for
// a per-storm slot (e.g. "AT4" -> 86). The MapServer pre-allocates 26-layer
// blocks per slot; within a block the group header is base, Forecast Cone is
// base+4, Watch-Warning base+5, Advisory Wind Field base+13. The first group
// header is AT1=4, EP1=134, CP1=264 (each basin offset 26 from the prior
// basin's slot 5). Cone = header+4, so AT1 cone=8, AT4 cone=86, EP5 cone=242,
// CP1 cone=268. Returns (coneID, ok); ok=false for an unmappable bin.
func slotConeBaseID(bin string) (int, bool) {
	bin = strings.ToUpper(strings.TrimSpace(bin))
	if len(bin) != 3 {
		return 0, false
	}
	basin := bin[:2]
	n, err := strconv.Atoi(bin[2:])
	if err != nil || n < 1 || n > 5 {
		return 0, false
	}
	var headerBase int
	switch basin {
	case "AT":
		headerBase = 4
	case "EP":
		headerBase = 134
	case "CP":
		headerBase = 264
	default:
		return 0, false
	}
	header := headerBase + 26*(n-1)
	return header + 4, true // Forecast Cone = header+4
}

// gisLayersFor maps a storm's binNumber to its three cite-worthy ArcGIS REST
// layers: Forecast Cone, Watch-Warning (cone+1), Advisory Wind Field
// (cone+9). Returns nil when the bin is unmappable.
func gisLayersFor(bin string) (string, []gisLayer) {
	cone, ok := slotConeBaseID(bin)
	if !ok {
		return "", nil
	}
	slot := strings.ToUpper(strings.TrimSpace(bin))
	mk := func(id int, name string) gisLayer {
		return gisLayer{ID: id, Name: name, URL: fmt.Sprintf("%s/%d", mapServerBase, id)}
	}
	return slot, []gisLayer{
		mk(cone, slot+" Forecast Cone"),
		mk(cone+1, slot+" Watch-Warning"),
		mk(cone+9, slot+" Advisory Wind Field"),
	}
}

// ---------------------------------------------------------------------------
// advisory product parsing (TCP / TCD / TCM)
// ---------------------------------------------------------------------------

var (
	reAtcfID    = regexp.MustCompile(`\b((?:AL|EP|CP|WP)\d{6})\b`)
	reAdvNumTCP = regexp.MustCompile(`(?i)Advisory\s+Number\s+(\S+)`)
	reAdvNumTCM = regexp.MustCompile(`(?i)FORECAST/ADVISORY\s+NUMBER\s+(\S+)`)
	// TCD titles read "... Discussion Number  16" (not "Advisory Number"), so
	// the discussion branch accepts either keyword.
	reAdvNumTCD = regexp.MustCompile(`(?i)(?:Advisory|Discussion)\s+Number\s+(\S+)`)
	rePreBlock  = regexp.MustCompile(`(?s)<pre>(.*?)</pre>`)

	// TCP (public): MPH-primary, dot-delimited.
	reTCPWinds    = regexp.MustCompile(`(?im)^MAXIMUM SUSTAINED WINDS\.{2,}(.+)$`)
	reTCPMovement = regexp.MustCompile(`(?im)^PRESENT MOVEMENT\.{2,}(.+)$`)
	reTCPPressure = regexp.MustCompile(`(?im)^MINIMUM CENTRAL PRESSURE\.{2,}(.+)$`)
	reTCPLocation = regexp.MustCompile(`(?im)^LOCATION\.{2,}(.+)$`)

	// TCM (forecast/marine): KT, space-delimited, ALL-CAPS, padded whitespace.
	reTCMWinds    = regexp.MustCompile(`(?im)^MAX SUSTAINED WINDS\s+(.+)$`)
	reTCMPressure = regexp.MustCompile(`(?im)^ESTIMATED MINIMUM CENTRAL PRESSURE\s+(.+)$`)
	reTCMMovement = regexp.MustCompile(`(?im)^PRESENT MOVEMENT\s+(.+)$`)
	reTCMLocation = regexp.MustCompile(`(?im)^(?:HURRICANE|TROPICAL STORM|TROPICAL DEPRESSION|POST-TROPICAL CYCLONE|POTENTIAL TROPICAL CYCLONE|SUBTROPICAL STORM|SUBTROPICAL DEPRESSION|REMNANTS OF .+?)\s+CENTER LOCATED NEAR\s+(.+)$`)

	// Name from the title line of any product type. The product-type token
	// (Advisory / Discussion / Forecast/Advisory) follows the storm name.
	reStormName = regexp.MustCompile(`(?im)^(?:HURRICANE|TROPICAL STORM|TROPICAL DEPRESSION|POST-TROPICAL CYCLONE|POTENTIAL TROPICAL CYCLONE|SUBTROPICAL STORM|SUBTROPICAL DEPRESSION|REMNANTS OF)\s+([A-Za-z][A-Za-z'\- ]*?)\s+(?:Advisory|Discussion|Forecast/Advisory|FORECAST/ADVISORY|Intermediate Advisory|Special Advisory)\b`)
)

// advisoryFields carries the parsed vitals for a TCP/TCM product. TCD has no
// vitals block so its fields stay empty.
type advisoryFields struct {
	Location           string `json:"location,omitempty"`
	MaxSustainedWinds  string `json:"max_sustained_winds,omitempty"`
	PresentMovement    string `json:"present_movement,omitempty"`
	MinCentralPressure string `json:"min_central_pressure,omitempty"`
}

// advisoryResult is the parsed advisory <id> --type ... payload.
type advisoryResult struct {
	AtcfID         string         `json:"atcf_id"`
	Storm          string         `json:"storm"`
	Type           string         `json:"type"`
	AdvisoryNumber string         `json:"advisory_number"`
	Issued         string         `json:"issued"`
	Fields         advisoryFields `json:"fields"`
	Raw            string         `json:"raw"`
}

// extractProductBody returns the cleaned product body. A fetched .shtml wraps
// the product in exactly one <pre> block; the raw-body fixtures are already
// the extracted body. Both forms are handled: extract the <pre> if present,
// then HTML-entity decode.
func extractProductBody(raw string) string {
	if m := rePreBlock.FindStringSubmatch(raw); m != nil {
		return strings.TrimRight(cliutil.CleanText(m[1]), "\n")
	}
	// Already-extracted body: still entity-decode in case of stray entities.
	return strings.TrimRight(cliutil.CleanText(raw), "\n")
}

// productIssuanceLine returns the human-readable issuance time line. The
// archive leaves line 2 (TTAA00 KNHC DDHHMM) as an unsubstituted placeholder,
// so it is never used. The issuance line is the first line after the
// "NWS National Hurricane Center ... <ATCF>" station line that contains a
// year and a time token (e.g. "1000 PM CDT Tue Oct 08 2024" /
// "0300 UTC WED OCT 09 2024").
var reIssuance = regexp.MustCompile(`(?im)^\s*(\d{3,4}\s+(?:AM|PM|UTC).*\b(?:19|20)\d{2})\s*$`)

func productIssuanceLine(body string) string {
	if m := reIssuance.FindStringSubmatch(body); m != nil {
		return strings.TrimSpace(m[1])
	}
	return ""
}

// trimTerminator drops the product terminator ($$ / Forecaster <name> / NNNN)
// and everything after it so a parsed body never includes the NNNN trailer.
func trimTerminator(body string) string {
	if i := strings.Index(body, "\n$$"); i >= 0 {
		return strings.TrimRight(body[:i], "\n")
	}
	// Fallback: drop a trailing NNNN line if present.
	lines := strings.Split(body, "\n")
	for len(lines) > 0 {
		last := strings.TrimSpace(lines[len(lines)-1])
		if last == "" || last == "NNNN" {
			lines = lines[:len(lines)-1]
			continue
		}
		break
	}
	return strings.Join(lines, "\n")
}

// firstSubmatch returns the trimmed first capture group, or "".
func firstSubmatch(re *regexp.Regexp, s string) string {
	if m := re.FindStringSubmatch(s); m != nil {
		return strings.TrimSpace(m[1])
	}
	return ""
}

// parseAdvisory parses an already-extracted-or-raw advisory body for the given
// product type ("tcp"|"tcd"|"tcm"). Storm identity comes from the body ATCF id
// (never the AWIPS PIL). Returns the structured result with the raw body
// attached. typ is normalized lowercase.
func parseAdvisory(raw, typ string) (*advisoryResult, error) {
	body := extractProductBody(raw)
	if strings.TrimSpace(body) == "" {
		return nil, fmt.Errorf("empty advisory body")
	}
	typ = strings.ToLower(strings.TrimSpace(typ))
	res := &advisoryResult{Type: typ, Raw: body}

	// ATCF id: the body station line carries it (e.g. AL142024). Skip the
	// AWIPS PIL on line 1 (MIATCPAT4) which is not a valid ATCF id.
	if m := reAtcfID.FindStringSubmatch(body); m != nil {
		res.AtcfID = m[1]
	}
	res.Storm = firstSubmatch(reStormName, body)
	res.Issued = productIssuanceLine(body)

	switch typ {
	case "tcm":
		res.AdvisoryNumber = normalizeAdvNum(firstSubmatch(reAdvNumTCM, body))
		res.Fields.MaxSustainedWinds = firstSubmatch(reTCMWinds, body)
		res.Fields.MinCentralPressure = firstSubmatch(reTCMPressure, body)
		res.Fields.PresentMovement = firstSubmatch(reTCMMovement, body)
		res.Fields.Location = firstSubmatch(reTCMLocation, body)
	case "tcd":
		// No vitals block; the discussion body carries the value. The TCD title
		// reads "Discussion Number", so use the broadened regex. Drop the
		// terminator so the raw passthrough has no NNNN trailer.
		res.AdvisoryNumber = normalizeAdvNum(firstSubmatch(reAdvNumTCD, body))
		res.Raw = trimTerminator(body)
	default: // tcp (public) is the default
		res.Type = "tcp"
		res.AdvisoryNumber = normalizeAdvNum(firstSubmatch(reAdvNumTCP, body))
		res.Fields.MaxSustainedWinds = firstSubmatch(reTCPWinds, body)
		res.Fields.MinCentralPressure = firstSubmatch(reTCPPressure, body)
		res.Fields.PresentMovement = firstSubmatch(reTCPMovement, body)
		res.Fields.Location = firstSubmatch(reTCPLocation, body)
	}
	return res, nil
}

// normalizeAdvNum strips a leading-zero pad from a numeric advisory number
// ("016" -> "16") while preserving an intermediate suffix ("016A" -> "16A").
// Non-numeric leading content is returned unchanged.
func normalizeAdvNum(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	// Split into leading digits and trailing suffix.
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == 0 {
		return s
	}
	num, err := strconv.Atoi(s[:i])
	if err != nil {
		return s
	}
	return strconv.Itoa(num) + s[i:]
}

// ---------------------------------------------------------------------------
// Tropical Weather Outlook (TWO) parsing
// ---------------------------------------------------------------------------

var (
	// The percent is sometimes prefixed with a qualifier in low-activity
	// phrasing ("...low...near 0 percent."); the qualifier is optional so the
	// quiet-season outlook still yields structured odds rather than null.
	reFormation = regexp.MustCompile(`(?im)^\*\s*Formation chance through (48 hours|7 days)\.{2,}(\w+)\.{2,}(?:near\s+)?(\d+)\s*percent`)
	reTWOArea   = regexp.MustCompile(`(?m)^([A-Z][^\n]*\([A-Z]{2}\d{2}\)):\s*$`)
	reTWOIssued = regexp.MustCompile(`(?im)^\s*(\d{3,4}\s+(?:AM|PM)\s+\w{2,4}\s+\w{3}\s+\w{3}\s+\d{1,2}\s+\d{4})\s*$`)
)

// formationChance is one {level, percent} odds reading.
type formationChance struct {
	Level   string `json:"level"`
	Percent int    `json:"percent"`
}

// outlookArea is one development area in the outlook.
type outlookArea struct {
	Name         string           `json:"name"`
	Formation48h *formationChance `json:"formation_48h"`
	Formation7d  *formationChance `json:"formation_7d"`
	Text         string           `json:"text,omitempty"`
}

// outlookGraphics is the two PNG link-outs.
type outlookGraphics struct {
	Two2D string `json:"two_2d"`
	Two7D string `json:"two_7d"`
}

// outlookResult is the parsed outlook --basin payload.
type outlookResult struct {
	Basin    string           `json:"basin"`
	Issued   string           `json:"issued"`
	Areas    []outlookArea    `json:"areas"`
	Graphics *outlookGraphics `json:"graphics,omitempty"`
}

// twoGraphicCode maps a clean basin token to the corrected TWO graphic basin
// code (atl->atl, ep->pac, cp->cpac). Returns ("", false) for an unknown token.
func twoGraphicCode(basin string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(basin)) {
	case "atl", "at":
		return "atl", true
	case "ep":
		return "pac", true
	case "cp":
		return "cpac", true
	}
	return "", false
}

// twoGraphics builds the two PNG link-outs for a basin.
func twoGraphics(basin string) (*outlookGraphics, bool) {
	code, ok := twoGraphicCode(basin)
	if !ok {
		return nil, false
	}
	return &outlookGraphics{
		Two2D: fmt.Sprintf("https://www.nhc.noaa.gov/xgtwo/two_%s_2d0.png", code),
		Two7D: fmt.Sprintf("https://www.nhc.noaa.gov/xgtwo/two_%s_7d0.png", code),
	}, true
}

// twoTextURL maps a clean basin token to the corrected TWO text URL path.
func twoTextURL(basin string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(basin)) {
	case "atl", "at":
		return "/text/MIATWOAT.shtml", true
	case "ep":
		return "/text/MIATWOEP.shtml", true
	case "cp":
		return "/text/HFOTWOCP.shtml", true
	}
	return "", false
}

// parseTWO parses a TWO product (fetched .shtml or raw body) for a basin into
// the outlook contract. The en-Espanol anchor and HTML chrome are stripped via
// the same <pre> extraction; formation lines are pulled per area.
func parseTWO(raw, basin string) *outlookResult {
	body := extractProductBody(raw)
	res := &outlookResult{Basin: strings.ToLower(strings.TrimSpace(basin))}
	res.Issued = firstSubmatch(reTWOIssued, body)

	// Detect named development areas ("Name (AL90):"). If none, synthesize a
	// single area so formation odds still surface (the always-on contract). In
	// that branch the whole body is one block, so scan it globally.
	areaMatches := reTWOArea.FindAllStringSubmatchIndex(body, -1)
	if len(areaMatches) == 0 {
		f48, f7d := formationPair(body)
		res.Areas = []outlookArea{{
			Name:         "",
			Formation48h: f48,
			Formation7d:  f7d,
		}}
		return res
	}
	for i, m := range areaMatches {
		name := strings.TrimSpace(body[m[2]:m[3]])
		// Area text runs from end of the name line to the next area (or end of
		// body for the last area). Scope the formation-line search to this
		// block so multi-area outlooks attach each pair to its own area
		// instead of dropping odds for area 2+.
		start := m[1]
		end := len(body)
		if i+1 < len(areaMatches) {
			end = areaMatches[i+1][0]
		}
		text := strings.TrimSpace(body[start:end])
		f48, f7d := formationPair(body[start:end])
		res.Areas = append(res.Areas, outlookArea{
			Name:         name,
			Text:         text,
			Formation48h: f48,
			Formation7d:  f7d,
		})
	}
	return res
}

// formationPair scans a text block for its first 48h and 7d formation chances.
// Returns (nil, nil) when the block carries no formation lines.
func formationPair(block string) (f48, f7d *formationChance) {
	for _, m := range reFormation.FindAllStringSubmatch(block, -1) {
		pct, _ := strconv.Atoi(m[3])
		fc := &formationChance{Level: strings.ToLower(m[2]), Percent: pct}
		if strings.HasPrefix(m[1], "48") && f48 == nil {
			f48 = fc
		} else if strings.HasPrefix(m[1], "7") && f7d == nil {
			f7d = fc
		}
	}
	return f48, f7d
}

// twoBasinFromHeader recognizes the basin from a TWO WMO/AWIPS header
// (ABNT20/TWOAT -> atl, ABPZ20/TWOEP -> ep, ACPN50/TWOCP -> cp). Used by the
// header-recognition acceptance test. Returns "" when unrecognized.
func twoBasinFromHeader(raw string) string {
	body := extractProductBody(raw)
	switch {
	case strings.Contains(body, "TWOAT") || strings.Contains(body, "ABNT20"):
		return "atl"
	case strings.Contains(body, "TWOEP") || strings.Contains(body, "ABPZ20"):
		return "ep"
	case strings.Contains(body, "TWOCP") || strings.Contains(body, "ACPN50"):
		return "cp"
	}
	return ""
}

// ---------------------------------------------------------------------------
// NWS active alerts parsing
// ---------------------------------------------------------------------------

// alert is the projected tropical-alert shape. instruction is *string so a
// JSON null stays null (the verified tropical contract).
type alert struct {
	ID           string  `json:"id"`
	Event        string  `json:"event"`
	Severity     string  `json:"severity"`
	AreaDesc     string  `json:"areaDesc"`
	Headline     string  `json:"headline"`
	Instruction  *string `json:"instruction"`
	GeometryType string  `json:"geometry_type"`
	Effective    string  `json:"effective"`
	Expires      string  `json:"expires"`
}

type alertsResult struct {
	Count   int            `json:"count"`
	Alerts  []alert        `json:"alerts"`
	ByEvent map[string]int `json:"by_event"`
}

// rawAlertFeature mirrors a GeoJSON Feature with the fields the projection
// needs.
type rawAlertFeature struct {
	ID       string `json:"id"`
	Geometry *struct {
		Type string `json:"type"`
	} `json:"geometry"`
	Properties struct {
		Event       string  `json:"event"`
		Severity    string  `json:"severity"`
		AreaDesc    string  `json:"areaDesc"`
		Headline    string  `json:"headline"`
		Instruction *string `json:"instruction"`
		Effective   string  `json:"effective"`
		Expires     string  `json:"expires"`
		Geocode     struct {
			// UGC zone/county codes are state-prefixed (e.g. "FLC099" =
			// Florida); the first two letters are the authoritative state
			// token. The free-form areaDesc only sometimes carries ", FL".
			UGC []string `json:"UGC"`
		} `json:"geocode"`
	} `json:"properties"`
}

// matchesState reports whether the feature belongs to the given two-letter
// state token. The authoritative signal is the UGC code prefix; areaDesc is a
// fallback for payloads without geocode.
func (f rawAlertFeature) matchesState(state string) bool {
	state = strings.ToUpper(strings.TrimSpace(state))
	if state == "" {
		return true
	}
	for _, u := range f.Properties.Geocode.UGC {
		if len(u) >= 2 && strings.EqualFold(u[:2], state) {
			return true
		}
	}
	return strings.Contains(strings.ToUpper(f.Properties.AreaDesc), state)
}

type rawAlertCollection struct {
	Features []rawAlertFeature `json:"features"`
}

// parseAlerts decodes an alerts GeoJSON payload (FeatureCollection or a single
// Feature) and projects it into the alerts contract. area, when non-empty,
// filters features whose areaDesc contains the token (case-insensitive). When
// statements is false, "Tropical Cyclone Statement" features are excluded.
func parseAlerts(data []byte, area string, statements bool) (*alertsResult, error) {
	var coll rawAlertCollection
	if err := json.Unmarshal(data, &coll); err != nil {
		return nil, fmt.Errorf("parsing alerts payload: %w", err)
	}
	// Single-Feature payload (no "features" array): decode as one feature.
	if coll.Features == nil {
		var feat rawAlertFeature
		if err := json.Unmarshal(data, &feat); err == nil && feat.Properties.Event != "" {
			coll.Features = []rawAlertFeature{feat}
		}
	}

	res := &alertsResult{Alerts: []alert{}, ByEvent: map[string]int{}}
	area = strings.TrimSpace(area)
	for _, f := range coll.Features {
		if !statements && f.Properties.Event == "Tropical Cyclone Statement" {
			continue
		}
		if area != "" && !f.matchesState(area) {
			continue
		}
		a := alert{
			ID:          f.ID,
			Event:       f.Properties.Event,
			Severity:    f.Properties.Severity,
			AreaDesc:    f.Properties.AreaDesc,
			Headline:    f.Properties.Headline,
			Instruction: f.Properties.Instruction,
			Effective:   f.Properties.Effective,
			Expires:     f.Properties.Expires,
		}
		if f.Geometry != nil {
			a.GeometryType = f.Geometry.Type
		}
		res.Alerts = append(res.Alerts, a)
		res.ByEvent[a.Event]++
	}
	res.Count = len(res.Alerts)
	return res, nil
}

// tropicalAlertEventList is the verified single-call OR filter for active
// tropical alerts. statements appends the Tropical Cyclone Statement type.
func tropicalAlertEventList(statements bool) string {
	events := []string{
		"Hurricane Warning", "Hurricane Watch",
		"Tropical Storm Warning", "Tropical Storm Watch",
		"Storm Surge Warning", "Storm Surge Watch",
	}
	if statements {
		events = append(events, "Tropical Cyclone Statement")
	}
	return strings.Join(events, ",")
}

// ---------------------------------------------------------------------------
// Input loading: --fixture / stdin / live
// ---------------------------------------------------------------------------

// loadFixture reads a fixture file or stdin (when path is "-"). Returns the raw
// bytes for parsing through the same code path as a live response.
func loadFixture(path string) ([]byte, error) {
	if path == "-" {
		return io.ReadAll(os.Stdin)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading fixture %q: %w", path, err)
	}
	return b, nil
}

// httpGetText fetches a text/HTML product with the descriptive UA. The
// generated JSON client mangles nothing for text/* responses, but for the
// .shtml advisory/outlook products a tiny GET keeps the raw bytes pristine and
// avoids the JSON-sanitizer path entirely. Timeouts ride the passed context.
func httpGetText(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,*/*")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxGraphicsBytes))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("GET %s returned HTTP %d", url, resp.StatusCode)
	}
	return body, nil
}

// maxGraphicsBytes caps a downloaded GIS artifact. KMZ/KML files are a few KB
// to low MB; 16 MB is generous headroom while still bounding a hostile or
// runaway response so the download path cannot exhaust memory/disk.
const maxGraphicsBytes = 16 << 20 // 16 MiB

// httpGetCapped fetches a URL with the descriptive UA and an enforced size cap.
// Unlike httpGetText (unbounded io.ReadAll), it reads at most maxGraphicsBytes
// and errors when the body would exceed the cap, so feed-derived download URLs
// cannot stream an unbounded payload. Timeouts ride the passed context.
func httpGetCapped(ctx context.Context, url string, max int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("GET %s returned HTTP %d", url, resp.StatusCode)
	}
	// Read one byte past the cap so an over-limit body is detectable.
	body, err := io.ReadAll(io.LimitReader(resp.Body, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > max {
		return nil, fmt.Errorf("GET %s exceeded %d-byte download cap", url, max)
	}
	return body, nil
}
