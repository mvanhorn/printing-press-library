package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/navitime/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/navitime/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/travel/navitime/internal/navitime"
)

type commandProvider struct {
	places          navitime.PlacesResult
	routes          navitime.RouteResult
	passes          navitime.PassResult
	detail          navitime.DetailResult
	failure         error
	options         navitime.Options
	calls           int
	kind, query, id string
	limit           int
	deadline        time.Time
	metrics         navitime.Metrics
}

func (c *commandProvider) record(ctx context.Context) {
	c.calls++
	c.deadline, _ = ctx.Deadline()
}
func (c *commandProvider) Places(ctx context.Context, query, kind string, limit int) (navitime.PlacesResult, error) {
	c.record(ctx)
	c.query = query
	c.kind = kind
	c.limit = limit
	result := c.places
	if len(result.Places) > limit {
		result.Places = result.Places[:limit]
	}
	return result, c.failure
}
func (c *commandProvider) Routes(ctx context.Context, query navitime.Query) (navitime.RouteResult, error) {
	c.record(ctx)
	c.routes.Query = query
	return c.routes, c.failure
}
func (c *commandProvider) Passes(ctx context.Context) (navitime.PassResult, error) {
	c.record(ctx)
	return c.passes, c.failure
}
func (c *commandProvider) Show(ctx context.Context, id string) (navitime.DetailResult, error) {
	c.record(ctx)
	c.id = id
	return c.detail, c.failure
}
func (c *commandProvider) Metrics() navitime.Metrics { return c.metrics }

func commandFixture() *commandProvider {
	amount := 13320
	cheaper := 12500
	minutes := 135
	seconds := 8100
	longer := 160
	walking := 450
	transfers := 0
	date := "2026-10-01T09:00:00+09:00"
	meta := navitime.Metadata{SourceURL: "https://japantravel.navitime.com/en/area/jp/route/result/?start=00006668", FetchedAt: time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC), DataKind: "route_options"}
	routes := []navitime.Route{
		{ID: "known", SourceIndex: 1, IDProvenance: "local_content_hash", SourceURL: meta.SourceURL, DepartureAt: &date, DurationMinutes: &minutes, DurationSeconds: &seconds, DurationMinutesBasis: "whole_minutes_floor_from_calendar_seconds", TransportKinds: []string{"rail"}, TimingBasis: "scheduled_timetable", WalkingMeters: &walking, Transfers: &transfers, Fare: navitime.Fare{TotalJPY: &amount, Currency: "JPY", Basis: "published source fare"}, Legs: []navitime.Leg{{Index: 0, Kind: "rail", DepartureAt: &date}}, FareGroups: []navitime.FareGroup{{ID: "group-1", DisplayedTotalJPY: &amount}}},
		{ID: "unknown", SourceIndex: 2, SourceURL: meta.SourceURL, TransportKinds: []string{"car_taxi"}, TimingBasis: "estimated_road_travel", DurationMinutes: &longer, WalkingMeters: nil, Fare: navitime.Fare{Currency: "JPY", Basis: "unknown"}, Legs: []navitime.Leg{}, FareGroups: []navitime.FareGroup{}},
		{ID: "cheaper", SourceIndex: 3, SourceURL: meta.SourceURL, DurationMinutes: &longer, WalkingMeters: &walking, Transfers: &transfers, Fare: navitime.Fare{TotalJPY: &cheaper, Currency: "JPY"}, Legs: []navitime.Leg{}, FareGroups: []navitime.FareGroup{}},
	}
	return &commandProvider{
		places:  navitime.PlacesResult{Meta: meta, Query: "大久保", Kind: "station", Places: []navitime.Place{{ID: "00006668", Ref: "station:00006668", Kind: "station", Name: map[string]string{"en": "Tokyo", "ja": "東京"}, SourceURL: meta.SourceURL}, {ID: "00001756", Ref: "station:00001756", Kind: "station", Name: map[string]string{"en": "Kyoto", "ja": "京都"}, SourceURL: meta.SourceURL}}, Ambiguous: true, Notes: []string{"Source candidates only."}},
		routes:  navitime.RouteResult{Meta: meta, Routes: routes, Notes: []string{"Published cash fares remain source fares."}},
		passes:  navitime.PassResult{Meta: meta, Passes: []navitime.Pass{{ID: "japan_rail_pass", Name: "Japan Rail Pass", SupportStatus: "advertised", LiveTested: true, AnonymousMaxSelected: 1}, {ID: "regional", Name: "Regional pass", SupportStatus: "advertised", LiveTested: false, AnonymousMaxSelected: 1}}, Notes: []string{"Advertised catalogue."}},
		detail:  navitime.DetailResult{Meta: meta, Route: &routes[0], StoredSnapshot: true, Notes: []string{"Stored source response."}},
		metrics: navitime.Metrics{Requests: 1, CacheHits: 0, CacheWrites: 2, BytesReceived: 4321},
	}
}

func installCommandProvider(t *testing.T, c *commandProvider) {
	t.Helper()
	old := newNavitimeClient
	newNavitimeClient = func(options navitime.Options) (navitimeService, error) { c.options = options; return c, nil }
	t.Cleanup(func() { newNavitimeClient = old })
}
func runCommand(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	root := RootCmd()
	root.SetArgs(args)
	var out, diagnostics bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&diagnostics)
	err := root.Execute()
	return out.String(), diagnostics.String(), err
}
func oneJSON(t *testing.T, s string) map[string]any {
	t.Helper()
	decoder := json.NewDecoder(strings.NewReader(s))
	var value map[string]any
	if err := decoder.Decode(&value); err != nil {
		t.Fatalf("invalid JSON %q: %v", s, err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		t.Fatalf("expected exactly one JSON value, got %v (%q)", err, s)
	}
	return value
}
func routeArgs(command string, extra ...string) []string {
	return append([]string{"routes", command, "--from", "station:00006668", "--to", "station:00001756", "--depart-at", "2026-10-01T09:00"}, extra...)
}

func TestNavitimeDryRunBeforeValidationOrIO(t *testing.T) {
	testenv.Isolate(t)
	old := newNavitimeClient
	newNavitimeClient = func(navitime.Options) (navitimeService, error) {
		t.Fatal("dry run created a provider")
		return nil, nil
	}
	t.Cleanup(func() { newNavitimeClient = old })
	for _, path := range [][]string{{"places", "search"}, {"routes", "search"}, {"routes", "compare"}, {"routes", "show"}, {"passes", "list"}, {"capabilities"}} {
		t.Run(strings.Join(path, "/"), func(t *testing.T) {
			args := append(append([]string{}, path...), "--dry-run", "--json", "--home", "relative-invalid", "--cache-dir", "relative-invalid", "--timeout", "0s")
			out, _, err := runCommand(t, args...)
			if err != nil {
				t.Fatal(err)
			}
			if got := oneJSON(t, out)["dry_run"]; got != true {
				t.Fatalf("dry_run=%v", got)
			}
		})
	}
}
func TestNavitimeDryRunIgnoresDomainProjectionWithoutIO(t *testing.T) {
	testenv.Isolate(t)
	old := newNavitimeClient
	newNavitimeClient = func(navitime.Options) (navitimeService, error) {
		t.Fatal("projected dry-run created provider")
		return nil, nil
	}
	t.Cleanup(func() { newNavitimeClient = old })
	for _, leaf := range []string{"search", "compare", "show"} {
		out, _, err := runCommand(t, "routes", leaf, "--dry-run", "--agent", "--select", "routes.id,routes.fare.total_jpy")
		if err != nil {
			t.Fatal(err)
		}
		if oneJSON(t, out)["dry_run"] != true {
			t.Fatalf("projected dry-run did not retain preview: %q", out)
		}
	}
}

func TestNavitimeInvalidInputsExitTwoBeforeProvider(t *testing.T) {
	testenv.Isolate(t)
	c := commandFixture()
	installCommandProvider(t, c)
	cases := [][]string{
		{"places", "search", "--json"}, {"places", "search", "Tokyo", "extra"}, {"places", "search", "Tokyo", "--type", "airport"}, {"places", "search", "Tokyo", "--limit", "11"},
		{"routes", "search", "--json"}, {"routes", "search", "--from", "station:00006668"},
		{"routes", "search", "--from", "Tokyo", "--to", "station:00001756", "--depart-at", "2026-10-01T09:00"},
		routeArgs("search", "--arrive-by", "2026-10-01T12:00"),
		{"routes", "search", "--from", "station:00006668", "--to", "station:00001756", "--depart-at", "09:00"},
		{"routes", "search", "--from", "station:00006668", "--to", "station:00001756", "--first-on", "2026-02-30"},
		routeArgs("search", "--pass", "japan_rail_pass", "--pass", "regional"),
		routeArgs("search", "--pass", "japan_rail_pass,regional"),
		routeArgs("search", "--limit", "0"), routeArgs("compare", "--sort", "price"), routeArgs("compare", "--max-fare", "-1"),
		{"routes", "show", "--json"}, {"routes", "show", "known", "--latest"}, {"passes", "list", "--offset", "-1"}, {"passes", "list", "--limit", "0"},
	}
	for i, args := range cases {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			out, _, err := runCommand(t, args...)
			if err == nil || ExitCode(err) != 2 {
				t.Fatalf("args=%v err=%v code=%d", args, err, ExitCode(err))
			}
			if out != "" {
				t.Fatalf("invalid command emitted stdout %q", out)
			}
		})
	}
	if c.calls != 0 {
		t.Fatalf("invalid inputs reached provider %d times", c.calls)
	}
}
func TestNavitimeCompactJSONProjectionAndFieldsAlias(t *testing.T) {
	testenv.Isolate(t)
	c := commandFixture()
	installCommandProvider(t, c)
	out, stderr, err := runCommand(t, routeArgs("search")...)
	if err != nil {
		t.Fatal(err)
	}
	if stderr != "" || strings.Count(out, "\n") != 1 {
		t.Fatalf("default output must be one compact JSON line; stdout=%q stderr=%q", out, stderr)
	}
	value := oneJSON(t, out)
	routes := value["routes"].([]any)
	if len(routes) != 3 {
		t.Fatalf("route count=%d", len(routes))
	}
	row := routes[0].(map[string]any)
	if _, exists := row["legs"]; exists {
		t.Fatal("summary includes full detail legs")
	}
	if row["source_id"] != nil || row["fare"].(map[string]any)["pass_holder_cost_jpy"] != nil {
		t.Fatal("missing source IDs or pass-holder costs were invented")
	}
	selectArgs := routeArgs("search", "--select", "routes.id,routes.fare.total_jpy")
	selected, _, err := runCommand(t, selectArgs...)
	if err != nil {
		t.Fatal(err)
	}
	projected := oneJSON(t, selected)
	if len(projected) != 1 {
		t.Fatalf("projection retained unrelated keys: %v", projected)
	}
	first := projected["routes"].([]any)[0].(map[string]any)
	if first["id"] != "known" || first["fare"].(map[string]any)["total_jpy"] != float64(13320) {
		t.Fatalf("nested projection lost values: %v", first)
	}
	aliased, _, err := runCommand(t, routeArgs("search", "--fields", "routes.id,routes.fare.total_jpy")...)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(oneJSON(t, aliased), projected) {
		t.Fatal("--fields differs from --select")
	}
	pretty, _, err := runCommand(t, routeArgs("search", "--pretty")...)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(pretty, "\n  ") {
		t.Fatal("--pretty did not indent JSON")
	}
	agent, _, err := runCommand(t, routeArgs("search", "--agent")...)
	if err != nil {
		t.Fatal(err)
	}
	agentResult := oneJSON(t, agent)
	agentRoute := agentResult["routes"].([]any)[0].(map[string]any)
	if agentRoute["duration_minutes"] != float64(135) || agentRoute["walking_meters"] != float64(450) || agentRoute["fare"].(map[string]any)["total_jpy"] != float64(13320) {
		t.Fatalf("--agent compacted away domain fields: %v", agentRoute)
	}
	if agentRoute["duration_seconds"] != float64(8100) || agentRoute["timing_basis"] != "scheduled_timetable" || agentRoute["transport_kinds"].([]any)[0] != "rail" {
		t.Fatal("additive timing/transport fields were lost under --agent")
	}
	road := agentResult["routes"].([]any)[1].(map[string]any)
	if road["transport_kinds"].([]any)[0] != "car_taxi" || road["timing_basis"] != "estimated_road_travel" || road["fare"].(map[string]any)["total_jpy"] != nil {
		t.Fatal("road estimate lost its distinct basis or unknown fare")
	}
	agentSelected, _, err := runCommand(t, routeArgs("search", "--agent", "--select", "routes.id,routes.fare.total_jpy")...)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(oneJSON(t, agentSelected), projected) {
		t.Fatal("--agent changes projection paths")
	}
}
func TestNavitimeMetricsAreSeparateAndMeasureOutput(t *testing.T) {
	testenv.Isolate(t)
	c := commandFixture()
	installCommandProvider(t, c)
	out, stderr, err := runCommand(t, routeArgs("search", "--metrics", "--limit", "1")...)
	if err != nil {
		t.Fatal(err)
	}
	oneJSON(t, out)
	metrics := oneJSON(t, stderr)
	if metrics["request_count"] != float64(1) || metrics["output_bytes"] != float64(len([]byte(out))) || metrics["bytes_received"] != float64(4321) {
		t.Fatalf("wrong metrics: %v", metrics)
	}
	if metrics["latency_ms"].(float64) < 0 {
		t.Fatal("negative latency")
	}
}
func TestNavitimeOptionsAndTimeModesPropagate(t *testing.T) {
	testenv.Isolate(t)
	t.Cleanup(func() { _, _ = cliutil.SetHomeOverride("") })
	c := commandFixture()
	installCommandProvider(t, c)
	envDir := t.TempDir()
	t.Setenv("NAVITIME_CACHE_DIR", envDir)
	for _, mode := range []struct{ flag, value string }{{"depart-at", "2026-10-01T09:00"}, {"arrive-by", "2026-10-01T12:00"}, {"first-on", "2026-10-01"}, {"last-on", "2026-10-01"}} {
		args := []string{"routes", "search", "--from", "station:00006668", "--to", "station:00001756", "--" + mode.flag, mode.value, "--pass", "japan_rail_pass", "--no-cache", "--refresh", "--timeout", "2s"}
		_, _, err := runCommand(t, args...)
		if err != nil {
			t.Fatal(err)
		}
		if !c.options.NoCache || !c.options.Refresh || c.options.Timeout != 2*time.Second || c.options.CacheDir != envDir {
			t.Fatalf("options not propagated: %+v", c.options)
		}
		if c.routes.Query.Pass != "japan_rail_pass" {
			t.Fatalf("pass not propagated: %+v", c.routes.Query)
		}
		actual := map[string]string{"depart-at": c.routes.Query.DepartAt, "arrive-by": c.routes.Query.ArriveBy, "first-on": c.routes.Query.FirstOn, "last-on": c.routes.Query.LastOn}[mode.flag]
		if actual != mode.value {
			t.Fatalf("%s lost input %q", mode.flag, actual)
		}
		if c.deadline.IsZero() || time.Until(c.deadline) > 2*time.Second {
			t.Fatal("command lacks bounded context")
		}
	}
	explicit := t.TempDir()
	_, _, err := runCommand(t, "passes", "list", "--cache-dir", explicit)
	if err != nil {
		t.Fatal(err)
	}
	if c.options.CacheDir != explicit {
		t.Fatal("explicit --cache-dir did not override env")
	}
	_, _, err = runCommand(t, "places", "search", "Tokyo", "--data-source", "live")
	if err != nil {
		t.Fatal(err)
	}
	if !c.options.Refresh {
		t.Fatal("--data-source live did not bypass read cache")
	}
	_, _, err = runCommand(t, "routes", "show", "--latest", "--data-source", "live")
	if err == nil || ExitCode(err) != 2 {
		t.Fatal("stored-only show accepted --data-source live")
	}
	t.Setenv("NAVITIME_CACHE_DIR", "")
	home := t.TempDir()
	_, _, err = runCommand(t, "places", "search", "Tokyo", "--home", home, "--type", "station", "--limit", "1")
	if err != nil {
		t.Fatal(err)
	}
	if c.options.CacheDir != filepath.Join(home, "cache", "navitime") || c.kind != "station" || c.limit != 1 {
		t.Fatalf("home/place options: %+v kind=%s limit=%d", c.options, c.kind, c.limit)
	}
	_, _ = cliutil.SetHomeOverride("")
}
func TestNavitimeComparisonUnknownsCapsAndZeroResults(t *testing.T) {
	testenv.Isolate(t)
	c := commandFixture()
	installCommandProvider(t, c)
	out, _, err := runCommand(t, routeArgs("compare", "--sort", "fare")...)
	if err != nil {
		t.Fatal(err)
	}
	routes := oneJSON(t, out)["routes"].([]any)
	if routes[0].(map[string]any)["id"] != "cheaper" || routes[2].(map[string]any)["id"] != "unknown" {
		t.Fatalf("unknown fare ranked as zero or best: %v", routes)
	}
	out, _, err = runCommand(t, routeArgs("compare", "--sort", "fare", "--max-fare", "13000", "--limit", "1")...)
	if err != nil {
		t.Fatal(err)
	}
	routes = oneJSON(t, out)["routes"].([]any)
	if len(routes) != 1 || routes[0].(map[string]any)["id"] != "cheaper" {
		t.Fatalf("caps or limit applied before sorting: %v", routes)
	}
	out, _, err = runCommand(t, routeArgs("compare", "--max-fare", "0")...)
	if err != nil {
		t.Fatal(err)
	}
	value := oneJSON(t, out)
	if got := value["routes"].([]any); len(got) != 0 {
		t.Fatalf("no-match filter returned unrelated routes: %v", got)
	}
	if len(value["notes"].([]any)) == 0 {
		t.Fatal("zero results lack explanatory note")
	}
	out, _, err = runCommand(t, routeArgs("compare", "--sort", "walking", "--max-walk", "450")...)
	if err != nil {
		t.Fatal(err)
	}
	if len(oneJSON(t, out)["routes"].([]any)) != 2 {
		t.Fatal("unknown walk matched local cap")
	}
	if oneJSON(t, out)["sort"] != "walking" {
		t.Fatal("comparison leaked its internal sort enum")
	}
}
func TestNavitimeCatalogueLocalPaginationAndBounds(t *testing.T) {
	testenv.Isolate(t)
	c := commandFixture()
	installCommandProvider(t, c)
	out, _, err := runCommand(t, "passes", "list", "--offset", "1", "--limit", "10")
	if err != nil {
		t.Fatal(err)
	}
	value := oneJSON(t, out)
	passes := value["passes"].([]any)
	if value["total"] != float64(2) || len(passes) != 1 || passes[0].(map[string]any)["id"] != "regional" {
		t.Fatalf("bad local page: %v", value)
	}
	if len(c.passes.Passes) != 2 {
		t.Fatal("pagination modified the full provider catalogue")
	}
	out, _, err = runCommand(t, "passes", "list", "--offset", "999")
	if err != nil {
		t.Fatal(err)
	}
	value = oneJSON(t, out)
	if len(value["passes"].([]any)) != 0 || len(value["notes"].([]any)) < 2 {
		t.Fatalf("empty page should be [] with note: %v", value)
	}
	out, _, err = runCommand(t, routeArgs("search", "--limit", "10")...)
	if err != nil {
		t.Fatal(err)
	}
	if len(oneJSON(t, out)["routes"].([]any)) != 3 {
		t.Fatal("limit invented source alternatives")
	}
}
func TestNavitimeDetailIDsAndNotFound(t *testing.T) {
	testenv.Isolate(t)
	c := commandFixture()
	installCommandProvider(t, c)
	summary, _, err := runCommand(t, routeArgs("search", "--limit", "1")...)
	if err != nil {
		t.Fatal(err)
	}
	id := oneJSON(t, summary)["routes"].([]any)[0].(map[string]any)["id"].(string)
	out, _, err := runCommand(t, "routes", "show", id, "--select", "route.id,route.legs,route.fare_groups")
	if err != nil {
		t.Fatal(err)
	}
	route := oneJSON(t, out)["route"].(map[string]any)
	if c.id != id || route["id"] != id || len(route["legs"].([]any)) == 0 || len(route["fare_groups"].([]any)) == 0 {
		t.Fatalf("detail mismatch: %v", route)
	}
	c.detail = navitime.DetailResult{Route: nil, Notes: []string{"No stored routes yet."}, StoredSnapshot: true}
	out, _, err = runCommand(t, "routes", "show", "--latest")
	if err != nil {
		t.Fatal(err)
	}
	if oneJSON(t, out)["route"] != nil {
		t.Fatal("empty latest fabricated a route")
	}
	c.failure = &navitime.NotFoundError{Message: "stored route snapshot was not found"}
	out, _, err = runCommand(t, "routes", "show", "unknown")
	if err == nil || ExitCode(err) != 3 || out != "" {
		t.Fatalf("unknown snapshot: stdout=%q err=%v code=%d", out, err, ExitCode(err))
	}
}
func TestNavitimeTypedErrorsAndCapabilityProvenance(t *testing.T) {
	testenv.Isolate(t)
	for _, tc := range []struct {
		err  error
		code int
	}{{&navitime.ArgumentError{Message: "bad date"}, 2}, {&navitime.NotFoundError{Message: "missing"}, 3}, {&navitime.SourceError{Message: "challenge", Status: 403}, 5}, {&cliutil.RateLimitError{URL: "https://example.com"}, 7}, {errors.New("network failed"), 5}} {
		if got := ExitCode(navitimeError(tc.err)); got != tc.code {
			t.Fatalf("error=%v code=%d want=%d", tc.err, got, tc.code)
		}
	}
	out, _, err := runCommand(t, "capabilities")
	if err != nil {
		t.Fatal(err)
	}
	value := oneJSON(t, out)
	if value["meta"].(map[string]any)["research_date"] != "2026-09-27" {
		t.Fatal("capabilities lack research provenance")
	}
	statuses := map[string]bool{}
	for _, item := range value["capabilities"].([]any) {
		statuses[item.(map[string]any)["status"].(string)] = true
	}
	for _, status := range []string{"verified", "advertised", "credential_gated", "unavailable"} {
		if !statuses[status] {
			t.Fatalf("missing status %s", status)
		}
	}
	for _, path := range [][]string{{"places", "search"}, {"routes", "search"}, {"routes", "compare"}, {"routes", "show"}} {
		help, _, err := runCommand(t, path...)
		if err != nil || !strings.Contains(help, "Usage:") {
			t.Fatalf("bare leaf %v did not show help: %v", path, err)
		}
	}
	for _, group := range []string{"places", "routes", "passes"} {
		help, _, err := runCommand(t, group, "--json")
		if err != nil || !strings.Contains(help, "Usage:") {
			t.Fatalf("parent %s did not show help: err=%v output=%q", group, err, help)
		}
	}
}

func TestNavitimeRootHelpStaysFocused(t *testing.T) {
	testenv.Isolate(t)
	help, _, err := runCommand(t, "--help")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"--cache-dir", "--metrics", "--select", "--fields", "--timeout", "--pretty", "--no-cache"} {
		if !strings.Contains(help, name) {
			t.Fatalf("missing public flag %s", name)
		}
	}
	for _, name := range []string{"--rate-limit", "--max-age", "--compact", "--receipt", "--client-profile", "--deliver", "--audit-dir"} {
		if strings.Contains(help, name) {
			t.Fatalf("inactive framework flag advertised: %s", name)
		}
	}
	if strings.Count(help, "• places search") != 1 || strings.Count(help, "• routes show") != 1 {
		t.Fatal("main help repeats workflow highlights")
	}
}

func TestNavitimeLiveFixturesUseValidSingleModeAndLatestAlias(t *testing.T) {
	testenv.Isolate(t)
	c := commandFixture()
	installCommandProvider(t, c)
	_, _, err := runCommand(t, "routes", "compare", "--from", "station:00006668", "--to", "station:00001756", "--arrive-by", "2026-10-01T12:00", "--sort", "fare")
	if err != nil {
		t.Fatal(err)
	}
	if c.routes.Query.ArriveBy != "2026-10-01T12:00" || c.routes.Query.DepartAt != "" {
		t.Fatalf("fixture mixes modes: %+v", c.routes.Query)
	}
	out, _, err := runCommand(t, "routes", "show", "latest")
	if err != nil {
		t.Fatal(err)
	}
	if c.id != "latest" || oneJSON(t, out)["route"].(map[string]any)["id"] != "known" {
		t.Fatal("literal latest did not use actual stored snapshot path")
	}
}
