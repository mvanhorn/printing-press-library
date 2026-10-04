// Copyright 2026 zjsng. Licensed under Apache-2.0.
package michi

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/michi-no-eki/internal/cliutil"
	"io"
	"math"
	"net/url"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

// Source uses the generated authenticated/configured HTTP client at its outer seam.
// Runtime observations deliberately bypass its response cache to preserve freshness.
type Source struct {
	BaseURL string
	Fetch   func(context.Context, string) ([]byte, error)
}

func (s *Source) read(ctx context.Context, path string) ([]byte, string, string, error) {
	body, e := s.Fetch(ctx, path)
	u := strings.TrimRight(s.BaseURL, "/") + path
	if e != nil {
		return nil, u, "", e
	}
	if len(body) > 5*1024*1024 {
		return nil, u, "", fmt.Errorf("provider response exceeds5 MiB bound")
	}
	return body, u, observedNow(), nil
}
func (s *Source) Station(ctx context.Context, id string) (Station, error) {
	if e := validateID(id); e != nil {
		return Station{}, e
	}
	b, u, t, e := s.read(ctx, "/stations/views/"+id)
	if e != nil {
		return Station{}, e
	}
	return ParseStation(b, id, u, t)
}
func (s *Source) Notice(ctx context.Context, id string) (Notice, error) {
	if e := validateID(id); e != nil {
		return Notice{}, e
	}
	b, u, t, e := s.read(ctx, "/notices/views/"+id)
	if e != nil {
		return Notice{}, e
	}
	return ParseNotice(b, id, u, t)
}
func pathPart(ids []string) string {
	if len(ids) == 0 {
		return "all"
	}
	return strings.Join(ids, "+")
}
func matchesFacilities(st Station, ids []string, match string) bool {
	if len(ids) == 0 {
		return true
	}
	c := CatalogData().Facilities
	matched := 0
	for _, id := range ids {
		i, _ := strconv.Atoi(id)
		if i >= 100 && i < 118 && st.Facilities[c[i-100].Slug] == "present" {
			matched++
		}
	}
	if match == "any" {
		return matched > 0
	}
	return matched == len(ids)
}
func (s *Source) search(ctx context.Context, q Query) (Search, error) {
	scope, e := resolveQuery(q)
	if e != nil {
		return Search{}, e
	}
	keyword := scope.Keyword
	if keyword == "" {
		keyword = "all"
	}
	path := "/stations/search/" + pathPart(scope.PrefectureIDs) + "/" + pathPart(scope.FacilityIDs) + "/" + url.PathEscape(keyword)
	b, u, t, e := s.read(ctx, path)
	if e != nil {
		return Search{}, e
	}
	out, e := ParseSearch(b, u, t)
	if e != nil {
		return Search{}, e
	}
	out.Scope = scope
	out.MaxCandidates = q.MaxCandidates
	out.Limit = q.Limit
	if len(out.Stations) > q.MaxCandidates {
		out.Stations = out.Stations[:q.MaxCandidates]
		out.CoverageComplete = false
		out.Note = "Candidate scan capped; increase --max-candidates for broader coverage."
	}
	out.ScannedCandidates = len(out.Stations)
	matched := []Station{}
	for _, st := range out.Stations {
		if matchesFacilities(st, scope.FacilityIDs, scope.FacilityMatch) {
			matched = append(matched, st)
		}
	}
	out.Stations = matched
	out.MatchedCount = len(matched)
	return out, nil
}
func (s *Source) Find(ctx context.Context, q Query) (Search, error) {
	out, e := s.search(ctx, q)
	if e != nil {
		return Search{}, e
	}
	if len(out.Stations) > q.Limit {
		out.Stations = out.Stations[:q.Limit]
	}
	out.ReturnedCount = len(out.Stations)
	if len(out.Stations) == 0 && out.Note == "" {
		out.Note = "No matches within the observed source query and candidate cap; this says nothing about overnight permission or live service."
	}
	return out, nil
}
func (s *Source) Nearby(ctx context.Context, q Query, lat, lon, radius float64) (Search, error) {
	if coordinate(strconv.FormatFloat(lat, 'f', -1, 64), strconv.FormatFloat(lon, 'f', -1, 64)) == nil {
		return Search{}, fmt.Errorf("--lat must be within -90..90 and --lon within -180..180, both finite")
	}
	if math.IsNaN(radius) || math.IsInf(radius, 0) || radius < 0 || radius > 2000 {
		return Search{}, fmt.Errorf("--radius-km must be finite and between 0 and2000")
	}
	out, e := s.search(ctx, q)
	if e != nil {
		return Search{}, e
	}
	ranked := []Station{}
	missing := 0
	for _, st := range out.Stations {
		if st.Coordinates == nil {
			missing++
			continue
		}
		d := distanceKM(lat, lon, st.Coordinates.Latitude, st.Coordinates.Longitude)
		if radius > 0 && d > radius {
			continue
		}
		st.DistanceKM = &d
		ranked = append(ranked, st)
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		a, b := *ranked[i].DistanceKM, *ranked[j].DistanceKM
		if a == b {
			return ranked[i].ID < ranked[j].ID
		}
		return a < b
	})
	out.MatchedCount = len(ranked)
	if len(ranked) > q.Limit {
		ranked = ranked[:q.Limit]
	}
	out.Stations = ranked
	out.ReturnedCount = len(ranked)
	out.Warnings = append(out.Warnings, fmt.Sprintf("Distances are straight-line kilometers, not driving routes or vehicle-clearance evidence. %d candidates lack valid source coordinates.", missing))
	if len(ranked) == 0 {
		out.Note = "No nearby matches in the scoped candidate scan; widen --radius-km, filters or --max-candidates."
	}
	return out, nil
}
func distanceKM(a, b, c, d float64) float64 {
	r := math.Pi / 180
	da, db := (c-a)*r, (d-b)*r
	x := math.Sin(da/2)*math.Sin(da/2) + math.Cos(a*r)*math.Cos(c*r)*math.Sin(db/2)*math.Sin(db/2)
	x = math.Min(1, math.Max(0, x))
	return 6371.0088 * 2 * math.Atan2(math.Sqrt(x), math.Sqrt(1-x))
}
func (s *Source) Compare(ctx context.Context, csv, kind string) (Comparison, error) {
	ids, e := idsFromCSV(csv, 6)
	if e != nil {
		return Comparison{}, e
	}
	out := Comparison{SchemaVersion: 1, Kind: kind, ObservedAt: observedNow(), RequestedIDs: ids, Stations: []Station{}, FetchFailures: []FetchFailure{}, RequestedCount: len(ids), Warnings: append([]string{}, evidenceWarnings...)}
	results, errs := cliutil.FanoutRun(ctx, ids, func(id string) string { return id }, func(ctx context.Context, id string) (Station, error) { return s.Station(ctx, id) }, cliutil.WithConcurrency(3))
	for _, r := range results {
		out.Stations = append(out.Stations, r.Value)
	}
	for _, e := range errs {
		out.FetchFailures = append(out.FetchFailures, FetchFailure{e.Source, strings.TrimRight(s.BaseURL, "/") + "/stations/views/" + e.Source, e.Err.Error()})
	}
	out.SuccessfulCount = len(out.Stations)
	if len(out.Stations) == 0 {
		return out, fmt.Errorf("all %d station fetches failed", len(ids))
	}
	return out, nil
}
func GeneralGuidance() map[string]any {
	return map[string]any{"source_url": PolicyURL, "source_verified_date": "2026-10-03", "date_timezone": "Asia/Tokyo", "scope": "general MLIT roadside-station parking guidance; station-specific rules require confirmation", "rest_napping": "Fatigue-recovery rest/napping during a drive is allowed under general MLIT guidance.", "overnight_lodging": "Lodging use of public parking areas is generally discouraged; some stations provide separately designated overnight spaces.", "outdoor_camping": "General rest/napping guidance does not establish permission for tents, awnings, cooking or outdoor camping.", "station_specific_permissions": "unknown"}
}
func (s *Source) Readiness(ctx context.Context, id string) (map[string]any, error) {
	st, e := s.Station(ctx, id)
	if e != nil {
		return nil, e
	}
	return map[string]any{"station": st, "general_guidance": GeneralGuidance(), "unresolved_questions": []string{"Confirm designated overnight space and permitted dates with the station operator.", "Confirm outdoor-camping rules separately.", "Confirm today's bath/food/shop hours, fees and charger operation.", "Confirm vehicle dimensions/access and current parking availability."}}, nil
}
func noticeIndexPath(page int) string {
	if page > 0 {
		return "/notices?page=" + strconv.Itoa(page)
	}
	return "/notices"
}

type scannedNotices struct {
	records          []Notice
	sourceURLs       []string
	scannedPages     int
	next             *int
	publicationStart string
	publicationEnd   string
	observedAt       string
}

func (s *Source) scanNoticePages(ctx context.Context, pages int) (scannedNotices, error) {
	out := scannedNotices{records: []Notice{}, sourceURLs: []string{}, observedAt: observedNow()}
	page := 0
	seen := map[string]bool{}
	for i := 0; i < pages; i++ {
		b, u, t, e := s.read(ctx, noticeIndexPath(page))
		if e != nil {
			return out, e
		}
		records, next, e := ParseNotices(b, page, u, t)
		if e != nil {
			return out, e
		}
		out.sourceURLs = append(out.sourceURLs, u)
		out.scannedPages++
		out.next = next
		for _, n := range records {
			if seen[n.ID] {
				continue
			}
			seen[n.ID] = true
			out.records = append(out.records, n)
			if n.PublishedDate != "unknown" {
				if out.publicationStart == "" || n.PublishedDate < out.publicationStart {
					out.publicationStart = n.PublishedDate
				}
				if n.PublishedDate > out.publicationEnd {
					out.publicationEnd = n.PublishedDate
				}
			}
		}
		if next == nil {
			break
		}
		page = *next
	}
	return out, nil
}

func noticesFromScan(scanned scannedNotices, pages int) Notices {
	start, end := scanned.publicationStart, scanned.publicationEnd
	if start == "" {
		start = "unknown"
	}
	if end == "" {
		end = "unknown"
	}
	urls := scanned.sourceURLs
	if urls == nil {
		urls = []string{}
	}
	return Notices{Notices: []Notice{}, SourceURLs: urls, ObservedAt: scanned.observedAt, ScannedPages: scanned.scannedPages, ScannedRecords: len(scanned.records), MaxScanPages: pages, NextPage: scanned.next, PublicationStart: start, PublicationEnd: end, FetchFailures: []FetchFailure{}, MatchCoverage: "index metadata within scanned pages only", Note: "No matching notices means no match within scanned coverage; it does not establish live opening or absence of advisories."}
}

// maxBulletinExportPages refuses an incomplete bulletin export. --limit 0
// follows the notice index until it ends; a positive limit stops once that
// many records are collected. Crossing this bound while another page remains
// is an error, not a truncated file. Tests may lower it.
var maxBulletinExportPages = 500

func (s *Source) ExportNotices(ctx context.Context, limit int) ([]Notice, error) {
	if limit < 0 {
		return nil, fmt.Errorf("--limit must be 0 (all available notices) or a positive maximum")
	}
	var out []Notice
	seen := map[string]bool{}
	seenPages := map[int]bool{}
	page := 0
	for scanned := 0; ; scanned++ {
		if scanned >= maxBulletinExportPages {
			return nil, fmt.Errorf("bulletin export stopped after %d index pages with more notices available; the result would be incomplete", maxBulletinExportPages)
		}
		if seenPages[page] {
			return nil, fmt.Errorf("bulletin export repeated notice index page %d", page)
		}
		seenPages[page] = true
		b, u, t, e := s.read(ctx, noticeIndexPath(page))
		if e != nil {
			return nil, e
		}
		records, next, e := ParseNotices(b, page, u, t)
		if e != nil {
			return nil, e
		}
		for _, n := range records {
			if seen[n.ID] {
				continue
			}
			seen[n.ID] = true
			out = append(out, n)
			if limit > 0 && len(out) >= limit {
				return out, nil
			}
		}
		if next == nil {
			if out == nil {
				out = []Notice{}
			}
			return out, nil
		}
		page = *next
	}
}

func (s *Source) Notices(ctx context.Context, pages, limit int, prefecture, keyword string) (Notices, error) {
	if pages < 1 || pages > 5 {
		return Notices{}, fmt.Errorf("--max-scan-pages must be 1–5")
	}
	if limit < 1 || limit > 50 {
		return Notices{}, fmt.Errorf("--limit must be 1–50")
	}
	prefIDs, e := resolveCSV(prefecture, CatalogData().Prefectures)
	if e != nil {
		return Notices{}, e
	}
	prefNames := map[string]bool{}
	for _, id := range prefIDs {
		for _, p := range CatalogData().Prefectures {
			if id == p.ID {
				prefNames[normalize(p.Name)] = true
			}
		}
	}
	scanned, e := s.scanNoticePages(ctx, pages)
	out := noticesFromScan(scanned, pages)
	if e != nil {
		return out, e
	}
	for _, n := range scanned.records {
		if len(prefNames) > 0 && !prefNames[normalize(n.Prefecture)] {
			continue
		}
		if keyword != "" && !strings.Contains(strings.ToLower(n.Title), strings.ToLower(keyword)) {
			continue
		}
		if len(out.Notices) < limit {
			out.Notices = append(out.Notices, n)
		}
	}
	out.ReturnedCount = len(out.Notices)
	return out, nil
}
func (s *Source) StationNotices(ctx context.Context, id string, pages, detailCap, limit int) (Notices, error) {
	if e := validateID(id); e != nil {
		return Notices{}, e
	}
	if detailCap < 1 || detailCap > 50 {
		return Notices{}, fmt.Errorf("--max-detail-records must be 1–50")
	}
	if _, e := s.Station(ctx, id); e != nil {
		return Notices{}, e
	}
	if pages < 1 || pages > 5 {
		return Notices{}, fmt.Errorf("--max-scan-pages must be 1–5")
	}
	scanned, e := s.scanNoticePages(ctx, pages)
	if e != nil {
		return noticesFromScan(scanned, pages), e
	}
	index := scanned.records
	examined := index
	if len(examined) > detailCap {
		examined = examined[:detailCap]
	}
	out := noticesFromScan(scanned, pages)
	out.Notices = []Notice{}
	out.StationID = id
	out.MaxDetailRecords = detailCap
	if len(index) > len(examined) {
		out.MatchCoverage = "exact explicit station-ID links in the opened detail prefix of the scanned index; later index rows were not opened"
		out.Note = fmt.Sprintf("Scanned %d index notices across %d pages; detail fetches opened the first %d only. Later index rows were not fetched, so a station link after that window is not a match. Publication date is distinct from dates discussed in notice text.", len(index), out.ScannedPages, len(examined))
	} else {
		out.MatchCoverage = "exact explicit station-ID links in examined detail pages; no name-based inference"
		out.Note = "No match means no exact station link within the bounded page/detail scan; increase --max-scan-pages and --max-detail-records to inspect older coverage. Publication date is distinct from dates discussed in notice text."
	}
	results, errs := cliutil.FanoutRun(ctx, examined, func(n Notice) string { return n.ID }, func(ctx context.Context, n Notice) (Notice, error) {
		detail, e := s.Notice(ctx, n.ID)
		if e != nil {
			return Notice{}, e
		}
		detail.Prefecture = n.Prefecture
		return detail, nil
	}, cliutil.WithConcurrency(3))
	out.DetailRecords = len(examined)
	for _, e := range errs {
		out.FetchFailures = append(out.FetchFailures, FetchFailure{e.Source, strings.TrimRight(s.BaseURL, "/") + "/notices/views/" + e.Source, e.Err.Error()})
	}
	for _, r := range results {
		n := r.Value
		for _, sid := range n.StationIDs {
			if sid == id {
				n.MatchReason = "explicit_station_id_link"
				if len(out.Notices) < limit {
					out.Notices = append(out.Notices, n)
				}
				break
			}
		}
	}
	out.ReturnedCount = len(out.Notices)
	if len(examined) > 0 && len(results) == 0 {
		return out, fmt.Errorf("all %d notice-detail fetches failed", len(examined))
	}
	return out, nil
}

type FieldChange struct {
	Field  string `json:"field"`
	Before any    `json:"before"`
	After  any    `json:"after"`
}
type StationChange struct {
	ID     string        `json:"id"`
	Kind   string        `json:"kind"`
	Fields []FieldChange `json:"fields"`
	Note   string        `json:"note,omitempty"`
}
type Changes struct {
	Changes          []StationChange `json:"changes"`
	BeforeObservedAt string          `json:"before_observed_at"`
	AfterObservedAt  string          `json:"after_observed_at"`
	Source           string          `json:"data_source"`
	Demo             bool            `json:"demo"`
	UnresolvedCount  int             `json:"unresolved_count"`
	Note             string          `json:"note"`
}

func stationMap(s Station) map[string]any {
	s.ObservedAt = ""
	b, _ := json.Marshal(s)
	out := map[string]any{}
	_ = json.Unmarshal(b, &out)
	delete(out, "observed_at")
	return out
}
func flatten(prefix string, v any, out map[string]any) {
	if m, ok := v.(map[string]any); ok {
		for k, x := range m {
			p := k
			if prefix != "" {
				p = prefix + "." + k
			}
			flatten(p, x, out)
		}
		return
	}
	out[prefix] = v
}
func Diff(before, after Comparison) (Changes, error) {
	if before.SchemaVersion != 1 || after.SchemaVersion != 1 || before.Kind != "snapshot" || after.Kind != "snapshot" {
		return Changes{}, fmt.Errorf("both inputs must be schema_version 1, kind snapshot observations from snapshot --ids")
	}
	out := Changes{Changes: []StationChange{}, BeforeObservedAt: before.ObservedAt, AfterObservedAt: after.ObservedAt, Source: "local snapshots", Note: "Only observed factual field/membership changes; timestamps are excluded. A source-field change does not establish current availability or new camping permission."}
	a, b := map[string]Station{}, map[string]Station{}
	all := map[string]bool{}
	failed := map[string]bool{}
	for _, set := range []Comparison{before, after} {
		seen := map[string]bool{}
		for _, s := range set.Stations {
			if e := validateID(s.ID); e != nil {
				return Changes{}, e
			}
			if seen[s.ID] {
				return Changes{}, fmt.Errorf("duplicate station ID %s in snapshot", s.ID)
			}
			seen[s.ID] = true
			all[s.ID] = true
		}
		for _, f := range set.FetchFailures {
			all[f.ID] = true
			failed[f.ID] = true
		}
	}
	for _, s := range before.Stations {
		a[s.ID] = s
	}
	for _, s := range after.Stations {
		b[s.ID] = s
	}
	ids := []string{}
	for id := range all {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		old, okA := a[id]
		new, okB := b[id]
		if failed[id] {
			out.Changes = append(out.Changes, StationChange{id, "unresolved", []FieldChange{}, "A fetch failure prevents a complete before/after comparison."})
			out.UnresolvedCount++
			continue
		}
		if !okA || !okB {
			kind := "added"
			if !okB {
				kind = "removed"
			}
			out.Changes = append(out.Changes, StationChange{id, kind, []FieldChange{}, "Membership in the selected snapshot, not station opening/closure."})
			continue
		}
		fa, fb := map[string]any{}, map[string]any{}
		flatten("", stationMap(old), fa)
		flatten("", stationMap(new), fb)
		keys := map[string]bool{}
		for k := range fa {
			keys[k] = true
		}
		for k := range fb {
			keys[k] = true
		}
		ordered := []string{}
		for k := range keys {
			ordered = append(ordered, k)
		}
		sort.Strings(ordered)
		fields := []FieldChange{}
		for _, k := range ordered {
			if !reflect.DeepEqual(fa[k], fb[k]) {
				fields = append(fields, FieldChange{k, fa[k], fb[k]})
			}
		}
		if len(fields) > 0 {
			out.Changes = append(out.Changes, StationChange{id, "changed", fields, ""})
		}
	}
	return out, nil
}

const maxSnapshotBytes = 2 * 1024 * 1024

// requireObject checks presence before decoding: omitted factual fields in a
// --select projection must never become zero values that look like changes.
func requireObject(raw json.RawMessage, keys []string, nullable map[string]bool) (map[string]json.RawMessage, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil || object == nil {
		return nil, fmt.Errorf("snapshot requires a complete JSON object")
	}
	for _, key := range keys {
		value, ok := object[key]
		if !ok || (strings.TrimSpace(string(value)) == "null" && !nullable[key]) {
			return nil, fmt.Errorf("snapshot is incomplete: missing or null %s; save snapshot --json without --select", key)
		}
	}
	return object, nil
}
func validateSnapshot(raw []byte, c Comparison) error {
	top, err := requireObject(raw, []string{"schema_version", "kind", "observed_at", "requested_ids", "stations", "fetch_failures", "requested_count", "successful_count", "warnings"}, nil)
	if err != nil {
		return err
	}
	if c.SchemaVersion != 1 || c.Kind != "snapshot" {
		return fmt.Errorf("input is not a schema_version 1 snapshot observation")
	}
	ids, err := idsFromCSV(strings.Join(c.RequestedIDs, ","), 6)
	if err != nil || len(ids) != len(c.RequestedIDs) {
		return fmt.Errorf("snapshot requires 1–6 unique valid requested IDs")
	}
	if c.RequestedCount != len(ids) || c.SuccessfulCount != len(c.Stations) || len(c.Stations)+len(c.FetchFailures) != len(ids) {
		return fmt.Errorf("snapshot count/accounting fields are inconsistent")
	}
	requested := map[string]bool{}
	seen := map[string]bool{}
	for _, id := range ids {
		requested[id] = true
	}
	var records []json.RawMessage
	if err := json.Unmarshal(top["stations"], &records); err != nil {
		return err
	}
	stationKeys := []string{"id", "name", "prefecture", "municipality", "url", "source_url", "observed_at", "address", "phone", "published_hours", "registration_source_text", "map_code", "coordinates", "facilities", "parking", "operator_urls", "permissions", "live_opening", "live_charger_availability", "fees"}
	for i, st := range c.Stations {
		if !requested[st.ID] || seen[st.ID] {
			return fmt.Errorf("snapshot station ID %q is duplicated or was not requested", st.ID)
		}
		seen[st.ID] = true
		fields, e := requireObject(records[i], stationKeys, map[string]bool{"coordinates": true})
		if e != nil {
			return e
		}
		if _, e := requireObject(fields["parking"], []string{"source_text", "large_vehicles", "ordinary_cars", "accessible_spaces", "live_space_availability", "vehicle_fit"}, map[string]bool{"large_vehicles": true, "ordinary_cars": true, "accessible_spaces": true}); e != nil {
			return e
		}
		if _, e := requireObject(fields["permissions"], []string{"overnight_lodging", "outdoor_camping", "designated_overnight_space"}, nil); e != nil {
			return e
		}
		if strings.TrimSpace(string(fields["coordinates"])) != "null" {
			if _, e := requireObject(fields["coordinates"], []string{"latitude", "longitude"}, nil); e != nil {
				return e
			}
			if st.Coordinates == nil || coordinate(strconv.FormatFloat(st.Coordinates.Latitude, 'f', -1, 64), strconv.FormatFloat(st.Coordinates.Longitude, 'f', -1, 64)) == nil {
				return fmt.Errorf("snapshot contains invalid station coordinates")
			}
		}
		for _, facility := range CatalogData().Facilities {
			status, ok := st.Facilities[facility.Slug]
			if !ok || (status != "unknown" && status != "present" && status != "not_listed") {
				return fmt.Errorf("snapshot lacks a valid facility observation for %s", facility.Slug)
			}
		}
	}
	if err := json.Unmarshal(top["fetch_failures"], &records); err != nil {
		return err
	}
	for i, failure := range c.FetchFailures {
		if !requested[failure.ID] || seen[failure.ID] {
			return fmt.Errorf("snapshot failure ID %q is duplicated or was not requested", failure.ID)
		}
		seen[failure.ID] = true
		if _, e := requireObject(records[i], []string{"id", "source_url", "error"}, nil); e != nil {
			return e
		}
		if failure.Error == "" {
			return fmt.Errorf("snapshot failure requires an error description")
		}
	}
	return nil
}
func ReadSnapshot(path string) (Comparison, error) {
	// Check before open so a named pipe/device cannot block the normal file path.
	info, err := os.Stat(path)
	if err != nil {
		return Comparison{}, fmt.Errorf("inspect snapshot %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return Comparison{}, fmt.Errorf("snapshot input must be a regular file")
	}
	if info.Size() > maxSnapshotBytes {
		return Comparison{}, fmt.Errorf("snapshot input exceeds 2 MiB bound")
	}
	// #nosec G304 -- The user intentionally selects this read-only snapshot path; regular-file checks and the hard 2 MiB reader limit apply.
	f, err := os.Open(path)
	if err != nil {
		return Comparison{}, fmt.Errorf("read snapshot %s: %w", path, err)
	}
	defer f.Close()
	info, err = f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return Comparison{}, fmt.Errorf("snapshot input must remain a regular file")
	}
	// A hard reader limit also guards against growth after the stat.
	raw, err := io.ReadAll(io.LimitReader(f, maxSnapshotBytes+1))
	if err != nil {
		return Comparison{}, fmt.Errorf("read snapshot %s: %w", path, err)
	}
	if len(raw) > maxSnapshotBytes {
		return Comparison{}, fmt.Errorf("snapshot input exceeds 2 MiB bound")
	}
	var c Comparison
	// Unmarshal accepts exactly one JSON value followed only by whitespace.
	if err := json.Unmarshal(raw, &c); err != nil {
		return Comparison{}, fmt.Errorf("parse snapshot %s: %w", path, err)
	}
	if err := validateSnapshot(raw, c); err != nil {
		return Comparison{}, fmt.Errorf("invalid snapshot %s: %w", path, err)
	}
	return c, nil
}
func DemoSnapshots() (Comparison, Comparison) {
	st := emptyStation("10001", Origin+"/stations/views/10001", "2026-10-01T12:00:00+09:00")
	st.Name = "架空の道の駅（デモ）"
	st.PublishedHours = "09:00–17:00"
	a := Comparison{SchemaVersion: 1, Kind: "snapshot", ObservedAt: st.ObservedAt, RequestedIDs: []string{st.ID}, Stations: []Station{st}, FetchFailures: []FetchFailure{}, RequestedCount: 1, SuccessfulCount: 1, Warnings: []string{"Synthetic demonstration; this is not a real station observation."}}
	b := a
	b.Stations = append([]Station{}, a.Stations...)
	b.Stations[0].PublishedHours = "09:00–18:00"
	b.Stations[0].ObservedAt = "2026-10-02T12:00:00+09:00"
	b.ObservedAt = b.Stations[0].ObservedAt
	return a, b
}
