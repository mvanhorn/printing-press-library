// Copyright 2026 Abe Diaz (@abe238) and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Fixture-backed unit tests for the novel NHC parsers. These load the REAL
// captured fixtures under testdata/ and assert the exact values pinned in
// docs/research/acceptance-tests.md. They run with no network access.

package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixturesRoot is relative to this package dir; `go test` sets the working
// directory to the package, so testdata/ resolves in a fresh clone with no
// machine-specific absolute path.
const fixturesRoot = "testdata"

func fixPath(parts ...string) string {
	return filepath.Join(append([]string{fixturesRoot}, parts...)...)
}

func readFix(t *testing.T, parts ...string) []byte {
	t.Helper()
	b, err := os.ReadFile(fixPath(parts...))
	if err != nil {
		t.Fatalf("read fixture %v: %v", parts, err)
	}
	return b
}

// ---- storm flattening (A/B) ----

func TestParseCurrentStorms_Counts(t *testing.T) {
	cs, err := parseCurrentStorms(readFix(t, "currentstorms", "helene-2024.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cs.ActiveStorms) != 3 { // A1
		t.Fatalf("count = %d, want 3", len(cs.ActiveStorms))
	}
	names := map[string]bool{}
	for _, s := range cs.ActiveStorms {
		names[s.Name] = true
	}
	for _, want := range []string{"Helene", "Isaac", "John"} { // A2
		if !names[want] {
			t.Errorf("missing storm name %q", want)
		}
	}
}

func TestFindStorm_ByIDAndName(t *testing.T) {
	cs, _ := parseCurrentStorms(readFix(t, "currentstorms", "helene-2024.json"))
	cases := []struct {
		query  string
		wantID string
	}{
		{"al092024", "al092024"},
		{"AL092024", "al092024"}, // case-insensitive ATCF
		{"helene", "al092024"},   // loose name match
		{"Isaac", "al102024"},
		{"al992024", ""}, // not found
	}
	for _, c := range cases {
		got := cs.findStorm(c.query)
		if c.wantID == "" {
			if got != nil {
				t.Errorf("findStorm(%q) = %v, want nil", c.query, got.ID)
			}
			continue
		}
		if got == nil || got.ID != c.wantID {
			t.Errorf("findStorm(%q) = %v, want %s", c.query, got, c.wantID)
		}
	}
}

func TestFlattenStorm_HeleneVitals(t *testing.T) {
	cs, _ := parseCurrentStorms(readFix(t, "currentstorms", "helene-2024.json"))
	d := flattenStorm(cs.findStorm("al092024"))
	// A4: Helene fixture carries both vitals, so the *int fields are non-nil.
	if d.IntensityKt == nil || *d.IntensityKt != 50 {
		t.Errorf("intensity_kt = %v, want 50", d.IntensityKt)
	}
	if d.PressureMb == nil || *d.PressureMb != 972 {
		t.Errorf("pressure_mb = %v, want 972", d.PressureMb)
	}
	if d.Lat != 34.2 || d.Lon != -83.0 {
		t.Errorf("lat/lon = %v/%v, want 34.2/-83.0", d.Lat, d.Lon)
	}
	if d.Classification != "TS" { // A5
		t.Errorf("classification = %q, want TS", d.Classification)
	}
	// B1: forecast cone URL exactly.
	wantCone := "https://www.nhc.noaa.gov/storm_graphics/api/AL092024_016Aadv_CONE.kmz"
	if d.GIS.TrackCone == nil || d.GIS.TrackCone.Kmz != wantCone {
		t.Errorf("cone kmz = %v, want %s", d.GIS.TrackCone, wantCone)
	}
	// B2: every product URL.
	wantProd := map[string]string{
		"publicAdvisory":         "https://www.nhc.noaa.gov/text/MIATCPAT4.shtml",
		"forecastAdvisory":       "https://www.nhc.noaa.gov/text/MIATCMAT4.shtml",
		"forecastDiscussion":     "https://www.nhc.noaa.gov/text/MIATCDAT4.shtml",
		"windSpeedProbabilities": "https://www.nhc.noaa.gov/text/MIAPWSAT4.shtml",
		"forecastGraphics":       "https://www.nhc.noaa.gov/graphics_at4.shtml",
	}
	got := map[string]string{
		"publicAdvisory":         ptrURL(d.Products.PublicAdvisory),
		"forecastAdvisory":       ptrURL(d.Products.ForecastAdvisory),
		"forecastDiscussion":     ptrURL(d.Products.ForecastDiscussion),
		"windSpeedProbabilities": ptrURL(d.Products.WindSpeedProbabilities),
		"forecastGraphics":       ptrURL(d.Products.ForecastGraphics),
	}
	for k, want := range wantProd {
		if got[k] != want {
			t.Errorf("product %s = %q, want %q", k, got[k], want)
		}
	}
}

func ptrURL(p *textProduct) string {
	if p == nil {
		return ""
	}
	return p.URL
}

// B3: Isaac null contract — the four fields serialize as JSON null.
func TestFlattenStorm_IsaacNullContract(t *testing.T) {
	cs, _ := parseCurrentStorms(readFix(t, "currentstorms", "helene-2024.json"))
	d := flattenStorm(cs.findStorm("al102024"))
	raw, _ := json.Marshal(d)
	var obj map[string]json.RawMessage
	json.Unmarshal(raw, &obj)
	var gis map[string]json.RawMessage
	json.Unmarshal(obj["gis"], &gis)
	for _, k := range []string{"windWatchesWarnings", "stormSurgeWatchWarningGIS", "potentialStormSurgeFloodingGIS", "peakSurgeKML"} {
		v, ok := gis[k]
		if !ok {
			t.Errorf("gis.%s omitted, want present-but-null", k)
			continue
		}
		if string(v) != "null" {
			t.Errorf("gis.%s = %s, want null", k, string(v))
		}
	}
	wantCone := "https://www.nhc.noaa.gov/storm_graphics/api/AL102024_006adv_CONE.kmz"
	if d.GIS.TrackCone == nil || d.GIS.TrackCone.Kmz != wantCone {
		t.Errorf("isaac cone = %v, want %s", d.GIS.TrackCone, wantCone)
	}
	if d.Products.ForecastGraphics == nil || d.Products.ForecastGraphics.URL != "https://www.nhc.noaa.gov/graphics_at5.shtml" {
		t.Errorf("isaac graphics page = %v, want graphics_at5.shtml", d.Products.ForecastGraphics)
	}
}

// B4: John null contract.
func TestFlattenStorm_JohnNullContract(t *testing.T) {
	cs, _ := parseCurrentStorms(readFix(t, "currentstorms", "helene-2024.json"))
	d := flattenStorm(cs.findStorm("ep102024"))
	if d.GIS.StormSurgeWatchWarningGIS != nil {
		t.Errorf("john stormSurgeWatchWarningGIS = %v, want null", d.GIS.StormSurgeWatchWarningGIS)
	}
	if d.GIS.PeakSurgeKML != nil {
		t.Errorf("john peakSurgeKML = %v, want null", d.GIS.PeakSurgeKML)
	}
	if d.GIS.WindWatchesWarnings == nil {
		t.Errorf("john windWatchesWarnings = nil, want present")
	}
}

// B5: no fabricated windHistory/keyMessages keys anywhere.
func TestFlattenStorm_NoFabricatedKeys(t *testing.T) {
	cs, _ := parseCurrentStorms(readFix(t, "currentstorms", "helene-2024.json"))
	for _, id := range []string{"al092024", "al102024", "ep102024"} {
		raw, _ := json.Marshal(flattenStorm(cs.findStorm(id)))
		s := strings.ToLower(string(raw))
		if strings.Contains(s, "windhistory") || strings.Contains(s, "keymessages") {
			t.Errorf("%s output contains fabricated key: %s", id, raw)
		}
	}
}

// ---- advisory parsing (C) ----

func TestParseAdvisory_MiltonTCP(t *testing.T) {
	res, err := parseAdvisory(string(readFix(t, "advisories", "milton.public.txt")), "tcp")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Fields.MaxSustainedWinds, "160 MPH") { // C1
		t.Errorf("max winds = %q, want contains 160 MPH", res.Fields.MaxSustainedWinds)
	}
	if res.Fields.Location != "23.4N 86.5W" { // C2
		t.Errorf("location = %q, want 23.4N 86.5W", res.Fields.Location)
	}
	if !strings.Contains(res.Fields.MinCentralPressure, "915 MB") {
		t.Errorf("pressure = %q, want contains 915 MB", res.Fields.MinCentralPressure)
	}
	if res.AtcfID != "AL142024" { // C3
		t.Errorf("atcf_id = %q, want AL142024", res.AtcfID)
	}
	if res.Storm != "Milton" {
		t.Errorf("storm = %q, want Milton", res.Storm)
	}
	if res.AdvisoryNumber != "16" {
		t.Errorf("advisory_number = %q, want 16", res.AdvisoryNumber)
	}
	if res.Issued != "1000 PM CDT Tue Oct 08 2024" { // C4
		t.Errorf("issued = %q, want 1000 PM CDT Tue Oct 08 2024", res.Issued)
	}
	if strings.Contains(res.Issued, "DDHHMM") {
		t.Errorf("issued contains placeholder DDHHMM: %q", res.Issued)
	}
	// C8: raw has no HTML tags / entities.
	if strings.Contains(res.Raw, "<pre>") || strings.Contains(res.Raw, "&amp;") {
		t.Errorf("raw contains HTML artifacts")
	}
}

func TestParseAdvisory_MiltonTCM(t *testing.T) {
	res, err := parseAdvisory(string(readFix(t, "advisories", "milton.fstadv.txt")), "tcm")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Fields.MaxSustainedWinds, "140 KT") { // C5
		t.Errorf("max winds = %q, want contains 140 KT", res.Fields.MaxSustainedWinds)
	}
	if !strings.Contains(res.Fields.MinCentralPressure, "915 MB") {
		t.Errorf("pressure = %q, want contains 915 MB", res.Fields.MinCentralPressure)
	}
	if res.AtcfID != "AL142024" {
		t.Errorf("atcf_id = %q, want AL142024", res.AtcfID)
	}
	if res.AdvisoryNumber != "16" {
		t.Errorf("advisory_number = %q, want 16", res.AdvisoryNumber)
	}
	if !strings.Contains(res.Fields.Location, "23.4N") {
		t.Errorf("location = %q, want contains 23.4N", res.Fields.Location)
	}
}

func TestParseAdvisory_MiltonTCD(t *testing.T) {
	res, err := parseAdvisory(string(readFix(t, "advisories", "milton.discus.txt")), "tcd")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(res.Raw) == "" { // C6
		t.Error("tcd body empty")
	}
	if strings.Contains(res.Raw, "NNNN") {
		t.Errorf("tcd body contains NNNN trailer")
	}
	if res.AtcfID != "AL142024" {
		t.Errorf("atcf_id = %q, want AL142024", res.AtcfID)
	}
}

func TestParseAdvisory_HelenePILReuse(t *testing.T) {
	// C7: helene TCP shares PIL MIATCPAT4 with Milton; identity must come from
	// the body ATCF id, so it resolves to AL092024 not AL142024.
	res, err := parseAdvisory(string(readFix(t, "advisories", "helene.public.txt")), "tcp")
	if err != nil {
		t.Fatal(err)
	}
	if res.AtcfID != "AL092024" {
		t.Errorf("atcf_id = %q, want AL092024", res.AtcfID)
	}
	if res.Storm != "Helene" {
		t.Errorf("storm = %q, want Helene", res.Storm)
	}
}

// ---- outlook parsing (D) ----

func TestParseTWO_Atlantic(t *testing.T) {
	res := parseTWO(string(readFix(t, "two-atl.txt")), "atl")
	if len(res.Areas) == 0 {
		t.Fatal("no areas parsed")
	}
	a := res.Areas[0]
	if a.Formation48h == nil || a.Formation48h.Percent != 60 { // D1
		t.Errorf("formation_48h = %v, want 60", a.Formation48h)
	}
	if a.Formation7d == nil || a.Formation7d.Percent != 60 {
		t.Errorf("formation_7d = %v, want 60", a.Formation7d)
	}
	if a.Formation48h == nil || a.Formation48h.Level != "medium" {
		t.Errorf("formation_48h level = %v, want medium", a.Formation48h)
	}
	if res.Issued != "200 AM EDT Tue Jun 16 2026" { // D2
		t.Errorf("issued = %q, want 200 AM EDT Tue Jun 16 2026", res.Issued)
	}
	if !strings.Contains(a.Name, "AL90") {
		t.Errorf("area name = %q, want contains AL90", a.Name)
	}
}

func TestTWOBasinFromHeader(t *testing.T) {
	cases := []struct {
		parts []string
		want  string
	}{
		{[]string{"two-atl.txt"}, "atl"},
		{[]string{"two", "two-epac.txt"}, "ep"},
		{[]string{"two", "two-cpac.txt"}, "cp"},
	}
	for _, c := range cases {
		got := twoBasinFromHeader(string(readFix(t, c.parts...)))
		if got != c.want { // D3
			t.Errorf("basin from %v = %q, want %q", c.parts, got, c.want)
		}
	}
}

func TestTWOGraphicsURLs(t *testing.T) {
	cases := []struct {
		basin  string
		want2d string
	}{
		{"atl", "https://www.nhc.noaa.gov/xgtwo/two_atl_2d0.png"},
		{"ep", "https://www.nhc.noaa.gov/xgtwo/two_pac_2d0.png"},
		{"cp", "https://www.nhc.noaa.gov/xgtwo/two_cpac_2d0.png"},
	}
	for _, c := range cases {
		g, ok := twoGraphics(c.basin)
		if !ok || g.Two2D != c.want2d { // D4
			t.Errorf("twoGraphics(%q).two_2d = %v, want %s", c.basin, g, c.want2d)
		}
		if strings.Contains(g.Two2D, "two_epac") || strings.Contains(g.Two7D, "two_epac") { // I5
			t.Errorf("twoGraphics(%q) leaked two_epac: %v", c.basin, g)
		}
	}
}

func TestTWOTextURLs(t *testing.T) {
	// D5/I5: cp must use HFOTWOCP, never MIATWOCP.
	u, _ := twoTextURL("cp")
	if u != "/text/HFOTWOCP.shtml" {
		t.Errorf("cp text url = %q, want /text/HFOTWOCP.shtml", u)
	}
	if strings.Contains(u, "MIATWOCP") {
		t.Errorf("cp text url leaked MIATWOCP")
	}
}

// ---- graphics (F) ----

func TestGraphicsFor_Helene(t *testing.T) {
	cs, _ := parseCurrentStorms(readFix(t, "currentstorms", "helene-2024.json"))
	d := graphicsFor(cs.findStorm("al092024"), nil)
	want := map[string]string{
		"cone_kmz":       "https://www.nhc.noaa.gov/storm_graphics/api/AL092024_016Aadv_CONE.kmz",
		"track_kmz":      "https://www.nhc.noaa.gov/storm_graphics/api/AL092024_016Aadv_TRACK.kmz",
		"peak_surge_kml": "https://www.nhc.noaa.gov/gis/peakSurge/AL092024_PeakStormSurge_016Aadv.kml",
		"wind_radii_kmz": "https://www.nhc.noaa.gov/storm_graphics/api/AL092024_forecastradii_016Aadv.kmz",
	}
	got := map[string]string{
		"cone_kmz":       deref(d.Links.ConeKmz),
		"track_kmz":      deref(d.Links.TrackKmz),
		"peak_surge_kml": deref(d.Links.PeakSurgeKml),
		"wind_radii_kmz": deref(d.Links.WindRadiiKmz),
	}
	for k, w := range want { // F1
		if got[k] != w {
			t.Errorf("%s = %q, want %q", k, got[k], w)
		}
	}
	// F2: landing page is HTML, not PNG.
	page := deref(d.Links.ForecastGraphicsPage)
	if page != "https://www.nhc.noaa.gov/graphics_at4.shtml" {
		t.Errorf("forecast_graphics_page = %q, want graphics_at4.shtml", page)
	}
	for _, ext := range []string{".png", ".gif", ".jpg"} {
		if strings.HasSuffix(page, ext) {
			t.Errorf("forecast_graphics_page ends in %s", ext)
		}
	}
}

func TestGraphicsFor_IsaacNullSurge(t *testing.T) {
	cs, _ := parseCurrentStorms(readFix(t, "currentstorms", "helene-2024.json"))
	d := graphicsFor(cs.findStorm("al102024"), nil)
	if d.Links.PeakSurgeKml != nil { // F3
		t.Errorf("isaac peak_surge_kml = %v, want null", *d.Links.PeakSurgeKml)
	}
	raw, _ := json.Marshal(d.Links)
	var m map[string]json.RawMessage
	json.Unmarshal(raw, &m)
	if string(m["peak_surge_kml"]) != "null" {
		t.Errorf("isaac peak_surge_kml serializes %s, want null", m["peak_surge_kml"])
	}
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// ---- gis (G) ----

func TestGISLayersFor_SlotMapping(t *testing.T) {
	// G2: AT1 slot must reference 8/9/17 with the verified layer names.
	slot, layers := gisLayersFor("AT1")
	if slot != "AT1" {
		t.Errorf("slot = %q, want AT1", slot)
	}
	wantIDs := []int{8, 9, 17}
	wantNames := []string{"AT1 Forecast Cone", "AT1 Watch-Warning", "AT1 Advisory Wind Field"}
	if len(layers) != 3 {
		t.Fatalf("layers = %d, want 3", len(layers))
	}
	for i, l := range layers {
		if l.ID != wantIDs[i] {
			t.Errorf("layer %d id = %d, want %d", i, l.ID, wantIDs[i])
		}
		if l.Name != wantNames[i] {
			t.Errorf("layer %d name = %q, want %q", i, l.Name, wantNames[i])
		}
		if !strings.HasPrefix(l.URL, mapServerBase+"/") {
			t.Errorf("layer %d url = %q, want under MapServer", i, l.URL)
		}
	}
	// Cross-check the verified layer names against the MapServer fixture.
	verifyLayerNamesAgainstFixture(t, layers)

	// AT4 (Helene's bin) maps to 86/87/95 (not remapped to AT1).
	_, at4 := gisLayersFor("AT4")
	if at4[0].ID != 86 || at4[1].ID != 87 || at4[2].ID != 95 {
		t.Errorf("AT4 ids = %d/%d/%d, want 86/87/95", at4[0].ID, at4[1].ID, at4[2].ID)
	}
	// EP5 maps to 242/243/251.
	_, ep5 := gisLayersFor("EP5")
	if ep5[0].ID != 242 || ep5[1].ID != 243 || ep5[2].ID != 251 {
		t.Errorf("EP5 ids = %d/%d/%d, want 242/243/251", ep5[0].ID, ep5[1].ID, ep5[2].ID)
	}
}

func verifyLayerNamesAgainstFixture(t *testing.T, layers []gisLayer) {
	t.Helper()
	b, err := os.ReadFile(fixPath("mapserver", "NHC_tropical_weather_MapServer.json"))
	if err != nil {
		t.Skipf("mapserver fixture unavailable: %v", err)
	}
	var ms struct {
		Layers []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"layers"`
	}
	if err := json.Unmarshal(b, &ms); err != nil {
		t.Fatal(err)
	}
	byID := map[int]string{}
	for _, l := range ms.Layers {
		byID[l.ID] = l.Name
	}
	for _, l := range layers {
		if byID[l.ID] != l.Name {
			t.Errorf("layer id %d: our name %q != fixture %q", l.ID, l.Name, byID[l.ID])
		}
	}
}

// ---- alerts (E) ----

func TestParseAlerts_Empty(t *testing.T) {
	res, err := parseAlerts(readFix(t, "alerts", "empty_hurricane_warning_2026-06-15.json"), "", false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Count != 0 || len(res.Alerts) != 0 { // E1
		t.Errorf("empty count = %d, want 0", res.Count)
	}
}

func TestParseAlerts_MiltonFeature(t *testing.T) {
	res, err := parseAlerts(readFix(t, "alerts", "milton_hurricane_warning_feature_2024-10-09.json"), "", false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Count != 1 {
		t.Fatalf("count = %d, want 1", res.Count)
	}
	a := res.Alerts[0]
	if a.Event != "Hurricane Warning" || a.Severity != "Extreme" || a.AreaDesc != "Coastal Hillsborough" { // E2
		t.Errorf("feature = %+v", a)
	}
	if a.GeometryType != "MultiPolygon" {
		t.Errorf("geometry_type = %q, want MultiPolygon", a.GeometryType)
	}
	if !strings.Contains(a.Headline, "NWS Tampa Bay Ruskin FL") {
		t.Errorf("headline = %q", a.Headline)
	}
	if a.Instruction != nil { // E3
		t.Errorf("instruction = %v, want null", *a.Instruction)
	}
}

func TestParseAlerts_Rollup(t *testing.T) {
	res, err := parseAlerts(readFix(t, "alerts", "milton_2024-10-09_FL_active.json"), "FL", false)
	if err != nil {
		t.Fatal(err)
	}
	// E4: verified counts (at least).
	want := map[string]int{
		"Hurricane Warning":      51,
		"Tropical Storm Warning": 46,
		"Storm Surge Warning":    23,
		"Hurricane Watch":        14,
	}
	for ev, n := range want {
		if res.ByEvent[ev] < n {
			t.Errorf("by_event[%q] = %d, want >= %d", ev, res.ByEvent[ev], n)
		}
	}
	// E5: default excludes Tropical Cyclone Statement.
	if res.ByEvent["Tropical Cyclone Statement"] != 0 {
		t.Errorf("default included Tropical Cyclone Statement: %d", res.ByEvent["Tropical Cyclone Statement"])
	}
	withStmts, _ := parseAlerts(readFix(t, "alerts", "milton_2024-10-09_FL_active.json"), "FL", true)
	if withStmts.ByEvent["Tropical Cyclone Statement"] == 0 {
		t.Errorf("--statements did not include Tropical Cyclone Statement")
	}
}

// ---- regression: formation odds on low-activity phrasing (finding 1) ----

// TestParseTWO_NearZeroPercent guards the quiet-season phrasing
// "...low...near 0 percent." which the original regex nulled out because the
// digit did not immediately follow the level dots. The outlook is the product's
// only value in the quiet season, so percent 0 must surface (NOT null).
func TestParseTWO_NearZeroPercent(t *testing.T) {
	body := "000\n" +
		"ABNT20 KNHC 160502\n" +
		"TWOAT \n\n" +
		"Tropical Weather Outlook\n" +
		"NWS National Hurricane Center Miami FL\n" +
		"200 AM EDT Tue Jun 16 2026\n\n" +
		"For the North Atlantic...Caribbean Sea and the Gulf of America:\n\n" +
		"Tropical cyclone formation is not expected during the next 7 days.\n\n" +
		"* Formation chance through 48 hours...low...near 0 percent.\n" +
		"* Formation chance through 7 days...low...near 10 percent.\n\n" +
		"$$\n"
	res := parseTWO(body, "atl")
	if len(res.Areas) == 0 {
		t.Fatal("no areas parsed")
	}
	a := res.Areas[0]
	if a.Formation48h == nil {
		t.Fatalf("formation_48h = nil, want {low, 0} (near 0 percent must not null out)")
	}
	if a.Formation48h.Percent != 0 || a.Formation48h.Level != "low" {
		t.Errorf("formation_48h = %+v, want {low, 0}", *a.Formation48h)
	}
	if a.Formation7d == nil {
		t.Fatalf("formation_7d = nil, want {low, 10}")
	}
	if a.Formation7d.Percent != 10 || a.Formation7d.Level != "low" {
		t.Errorf("formation_7d = %+v, want {low, 10}", *a.Formation7d)
	}
}

// ---- regression: multi-area TWO attaches odds per area (finding 8) ----

// TestParseTWO_MultiArea asserts each development area gets its own formation
// odds rather than only Areas[0]. The original code attached the first pair to
// the first area only, nulling area 2+ odds (routine in active season).
func TestParseTWO_MultiArea(t *testing.T) {
	body := "000\n" +
		"ABNT20 KNHC 160502\n" +
		"TWOAT \n\n" +
		"Tropical Weather Outlook\n" +
		"NWS National Hurricane Center Miami FL\n" +
		"200 AM EDT Tue Jun 16 2026\n\n" +
		"For the North Atlantic...Caribbean Sea and the Gulf of America:\n\n" +
		"Western Caribbean Sea (AL91):\n" +
		"A broad area of low pressure is producing showers.\n" +
		"* Formation chance through 48 hours...high...80 percent.\n" +
		"* Formation chance through 7 days...high...90 percent.\n\n" +
		"Central Tropical Atlantic (AL92):\n" +
		"A tropical wave is expected to move westward.\n" +
		"* Formation chance through 48 hours...low...near 0 percent.\n" +
		"* Formation chance through 7 days...medium...40 percent.\n\n" +
		"$$\n"
	res := parseTWO(body, "atl")
	if len(res.Areas) != 2 {
		t.Fatalf("areas = %d, want 2", len(res.Areas))
	}
	a0, a1 := res.Areas[0], res.Areas[1]
	if !strings.Contains(a0.Name, "AL91") {
		t.Errorf("area 0 name = %q, want contains AL91", a0.Name)
	}
	if a0.Formation48h == nil || a0.Formation48h.Percent != 80 || a0.Formation7d == nil || a0.Formation7d.Percent != 90 {
		t.Errorf("area 0 odds = %+v / %+v, want 80 / 90", a0.Formation48h, a0.Formation7d)
	}
	if !strings.Contains(a1.Name, "AL92") {
		t.Errorf("area 1 name = %q, want contains AL92", a1.Name)
	}
	// The regression: area 2 must carry its OWN odds, not nil or area 0's.
	if a1.Formation48h == nil || a1.Formation48h.Percent != 0 || a1.Formation48h.Level != "low" {
		t.Errorf("area 1 formation_48h = %+v, want {low, 0}", a1.Formation48h)
	}
	if a1.Formation7d == nil || a1.Formation7d.Percent != 40 || a1.Formation7d.Level != "medium" {
		t.Errorf("area 1 formation_7d = %+v, want {medium, 40}", a1.Formation7d)
	}
}

// ---- regression: single-area fixture still yields 60/60 (finding 8 guard) ----

func TestParseTWO_SingleAreaStillWorks(t *testing.T) {
	res := parseTWO(string(readFix(t, "two-atl.txt")), "atl")
	if len(res.Areas) != 1 {
		t.Fatalf("areas = %d, want 1", len(res.Areas))
	}
	a := res.Areas[0]
	if a.Formation48h == nil || a.Formation48h.Percent != 60 || a.Formation7d == nil || a.Formation7d.Percent != 60 {
		t.Errorf("single-area odds = %+v / %+v, want 60 / 60", a.Formation48h, a.Formation7d)
	}
}

// ---- regression: TCD advisory_number populated (finding 5) ----

func TestParseAdvisory_TCDAdvisoryNumber(t *testing.T) {
	res, err := parseAdvisory(string(readFix(t, "advisories", "milton.discus.txt")), "tcd")
	if err != nil {
		t.Fatal(err)
	}
	// Title reads "Hurricane Milton Discussion Number  16" — must populate.
	if res.AdvisoryNumber != "16" {
		t.Errorf("tcd advisory_number = %q, want 16", res.AdvisoryNumber)
	}
}

// ---- regression: honest absence for missing vitals (finding 7) ----

// TestFlattenStorm_AbsentVitalsNull asserts that a storm with empty/non-numeric
// intensity/pressure emits JSON null (honest unknown), not a fabricated 0 which
// a safety tool must never present as a real reading (0 kt = calm).
func TestFlattenStorm_AbsentVitalsNull(t *testing.T) {
	s := &rawStorm{ID: "al992024", Name: "Test", Intensity: "", Pressure: "xyz"}
	d := flattenStorm(s)
	if d.IntensityKt != nil {
		t.Errorf("intensity_kt = %v, want nil for empty source", *d.IntensityKt)
	}
	if d.PressureMb != nil {
		t.Errorf("pressure_mb = %v, want nil for non-numeric source", *d.PressureMb)
	}
	raw, _ := json.Marshal(d)
	var obj map[string]json.RawMessage
	json.Unmarshal(raw, &obj)
	if string(obj["intensity_kt"]) != "null" {
		t.Errorf("intensity_kt serializes %s, want null", obj["intensity_kt"])
	}
	if string(obj["pressure_mb"]) != "null" {
		t.Errorf("pressure_mb serializes %s, want null", obj["pressure_mb"])
	}
	// Renderer shows "unknown", never a fabricated number.
	out := renderStormHuman(d)
	if !strings.Contains(out, "intensity   unknown") || !strings.Contains(out, "pressure    unknown") {
		t.Errorf("renderStormHuman omitted 'unknown' for absent vitals:\n%s", out)
	}
}

// ---- regression: graphics URL validator (finding 4) ----

func TestValidateGraphicsURL(t *testing.T) {
	good := []string{
		"https://www.nhc.noaa.gov/storm_graphics/api/AL092024_016Aadv_CONE.kmz",
		"https://mapservices.weather.noaa.gov/tropical/x.kmz",
		"https://noaa.gov/x.kmz",
		"https://api.weather.gov/x.kml",
	}
	for _, u := range good {
		if _, err := validateGraphicsURL(u); err != nil {
			t.Errorf("validateGraphicsURL(%q) rejected a valid NHC/NWS https URL: %v", u, err)
		}
	}
	bad := []string{
		"file:///etc/passwd",
		"http://www.nhc.noaa.gov/x.kmz",          // not https
		"https://evil.com/x.kmz",                 // wrong host
		"https://evilnoaa.gov/x.kmz",             // suffix-spoof (no leading dot)
		"https://www.noaa.gov.evil.com/x.kmz",    // host does not end in .noaa.gov
		"smb://share/x.kmz",                      // scheme abuse
		"-rf",                                    // leading-dash arg injection (empty scheme)
	}
	for _, u := range bad {
		if _, err := validateGraphicsURL(u); err == nil {
			t.Errorf("validateGraphicsURL(%q) accepted a URL it must reject", u)
		}
	}
}

func TestSafeGraphicsFilename(t *testing.T) {
	u, err := validateGraphicsURL("https://www.nhc.noaa.gov/storm_graphics/api/AL092024_016Aadv_CONE.kmz")
	if err != nil {
		t.Fatal(err)
	}
	name, err := safeGraphicsFilename(u)
	if err != nil {
		t.Fatalf("safeGraphicsFilename rejected a valid file: %v", err)
	}
	if name != "AL092024_016Aadv_CONE.kmz" {
		t.Errorf("filename = %q, want AL092024_016Aadv_CONE.kmz", name)
	}
	// A URL whose path ends in a separator yields an unsafe (".") base.
	bad, _ := validateGraphicsURL("https://www.nhc.noaa.gov/dir/")
	if _, err := safeGraphicsFilename(bad); err == nil {
		t.Errorf("safeGraphicsFilename accepted a directory-path URL")
	}
}

// ---- envelope sanity ----

func TestEnvelope_Shape(t *testing.T) {
	env := newEnvelope("fixture:x", map[string]int{"count": 0})
	raw, _ := json.Marshal(env)
	var obj map[string]json.RawMessage
	json.Unmarshal(raw, &obj)
	for _, k := range []string{"source", "fetched_at", "data"} {
		if _, ok := obj[k]; !ok {
			t.Errorf("envelope missing key %q", k)
		}
	}
}
