package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/tenki/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/tenki/internal/mcp/cobratree"
	"github.com/mvanhorn/printing-press-library/library/travel/tenki/internal/tenki"
)

type tenkiFake struct {
	search       tenki.SearchResult
	place        tenki.PlaceResult
	daily        tenki.ForecastResult
	hourly       tenki.ForecastResult
	seasonList   tenki.SeasonalListResult
	season       tenki.SeasonalResult
	mountain     tenki.MountainResult
	err          error
	placesByURL  map[string]tenki.PlaceResult
	dailyByURL   map[string]tenki.ForecastResult
	errorsByCall map[string]error
	calls        []string
	deadline     bool
}

func (f *tenkiFake) call(ctx context.Context, name string) {
	f.calls = append(f.calls, name)
	_, f.deadline = ctx.Deadline()
}
func (f *tenkiFake) Search(ctx context.Context, query, kind string, limit, pages int) (tenki.SearchResult, error) {
	f.call(ctx, "search:"+query+":"+kind)
	return f.search, f.err
}
func (f *tenkiFake) Resolve(ctx context.Context, place string) (tenki.PlaceResult, error) {
	f.call(ctx, "resolve:"+place)
	if err := f.errorsByCall["resolve:"+place]; err != nil {
		return tenki.PlaceResult{}, err
	}
	if result, ok := f.placesByURL[place]; ok {
		return result, f.err
	}
	return f.place, f.err
}
func (f *tenkiFake) Daily(ctx context.Context, place string) (tenki.ForecastResult, error) {
	f.call(ctx, "daily:"+place)
	if err := f.errorsByCall["daily:"+place]; err != nil {
		return tenki.ForecastResult{}, err
	}
	if result, ok := f.dailyByURL[place]; ok {
		return result, f.err
	}
	return f.daily, f.err
}
func (f *tenkiFake) Hourly(ctx context.Context, place string) (tenki.ForecastResult, error) {
	f.call(ctx, "hourly:"+place)
	return f.hourly, f.err
}
func (f *tenkiFake) SeasonalList(ctx context.Context, kind, query string, year, limit, pages int) (tenki.SeasonalListResult, error) {
	f.call(ctx, "season-list:"+kind+":"+query)
	return f.seasonList, f.err
}
func (f *tenkiFake) Seasonal(ctx context.Context, kind, place string, year int) (tenki.SeasonalResult, error) {
	f.call(ctx, "season:"+kind+":"+place)
	return f.season, f.err
}
func (f *tenkiFake) Mountain(ctx context.Context, place string) (tenki.MountainResult, error) {
	f.call(ctx, "mountain:"+place)
	return f.mountain, f.err
}
func (f *tenkiFake) Metrics() tenki.Metrics { return tenki.Metrics{HTTPRequests: len(f.calls)} }

func tenkiRun(t *testing.T, f tenkiSource, args ...string) (map[string]any, string, string, error, tenki.Config) {
	t.Helper()
	var flags rootFlags
	root := newRootCmd(&flags)
	var captured tenki.Config
	factory := func(config tenki.Config) tenkiSource { captured = config; return f }
	for _, command := range newTenkiCommands(&flags, factory) {
		for _, existing := range root.Commands() {
			if existing.Name() == command.Name() {
				root.RemoveCommand(existing)
			}
		}
		root.AddCommand(command)
	}
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs(args)
	err := root.Execute()
	var value map[string]any
	if json.Valid(stdout.Bytes()) {
		if decodeErr := json.Unmarshal(stdout.Bytes(), &value); decodeErr != nil {
			t.Fatal(decodeErr)
		}
	}
	return value, stdout.String(), stderr.String(), err, captured
}

func tenkiResult(t *testing.T, value map[string]any) map[string]any {
	t.Helper()
	result, ok := value["results"].(map[string]any)
	if !ok {
		t.Fatalf("missing single results envelope: %#v", value)
	}
	return result
}

func TestTenkiCoreDryRunBeforeValidationOrSource(t *testing.T) {
	for _, path := range [][]string{{"places", "search"}, {"places", "show"}, {"forecast", "daily"}, {"forecast", "hourly"}, {"mountain", "show"}, {"seasonal", "list"}, {"seasonal", "show"}, {"compare"}} {
		t.Run(strings.Join(path, "/"), func(t *testing.T) {
			f := &tenkiFake{}
			args := append(append([]string{}, path...), "--dry-run", "--json")
			value, _, _, err, _ := tenkiRun(t, f, args...)
			if err != nil || value["dry_run"] != true || len(f.calls) != 0 {
				t.Fatalf("dry run = %#v, %v; calls %v", value, err, f.calls)
			}
		})
	}
}

func TestTenkiValidationBoundsAndAbsence(t *testing.T) {
	cases := [][]string{
		{"places", "search", "--json"},
		{"places", "search", "--query", "東京", "--kind", "geocoder"},
		{"places", "search", "--query", "東京", "--limit", "51"},
		{"places", "search", "--query", "東京", "--max-scan-pages", "3"},
		{"places", "show", "--place", "https://example.com/forecast/3/16/4410/13101/"},
		{"places", "show", "--place", tenkiTokyo + "?redirect=x"},
		{"forecast", "daily", "--place", tenkiTokyo, "--days", "15"},
		{"forecast", "daily", "--place", tenkiTokyo, "--from", "2026-02-30"},
		{"forecast", "hourly", "--place", tenkiTokyo, "--limit", "73"},
		{"forecast", "hourly", "--place", tenkiTokyo, "--hours", "09:30-17:00"},
		{"forecast", "hourly", "--place", tenkiTokyo, "--hours", "17:00-09:00"},
		{"mountain", "show", "--place", tenkiTokyo},
		{"mountain", "show", "--place", tenkiFuji, "--level", "-1"},
		{"mountain", "show", "--place", tenkiFuji, "--level", "NaN"},
		{"places", "search", "--query", "金閣寺", "--kind", "leisure", "--directory", "https://example.com/leisure/6/29/"},
		{"seasonal", "list", "--kind", "summer"},
		{"seasonal", "list", "--kind", "kouyou", "--year", "0"},
		{"seasonal", "show", "--kind", "sakura", "--place", tenkiOze},
		{"places", "show", "--place", tenkiTokyo, "--data-source", "local", "--refresh"},
		{"places", "show", "stray", "--json"},
	}
	for _, args := range cases {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			f := &tenkiFake{}
			_, stdout, _, err, _ := tenkiRun(t, f, args...)
			if err == nil || ExitCode(err) != 2 || len(f.calls) != 0 || stdout != "" {
				t.Fatalf("wanted exit2 and no work/stdout, got %v, %q, calls %v", err, stdout, f.calls)
			}
		})
	}
}

func TestTenkiPlaceSearchProjectionAndBounds(t *testing.T) {
	f := &tenkiFake{search: tenki.SearchResult{Status: "ok", Source: tenki.Source{URL: "https://tenki.jp/search/", Freshness: "fresh"}, Places: []tenki.Place{{Name: "千代田区", URL: tenkiTokyo}, {Name: "新宿区", URL: "https://tenki.jp/forecast/3/16/4410/13104/"}}, Warnings: []string{"two candidates; select a URL"}}}
	value, stdout, stderr, err, _ := tenkiRun(t, f, "places", "search", "--query", "東京", "--limit", "1", "--agent", "--select", "results.places.name")
	if err != nil {
		t.Fatal(err)
	}
	if len(value) != 1 {
		t.Fatalf("projection leaked fields: %s", stdout)
	}
	result := tenkiResult(t, value)
	rows := result["places"].([]any)
	if len(rows) != 1 || rows[0].(map[string]any)["name"] != "千代田区" || len(rows[0].(map[string]any)) != 1 {
		t.Fatalf("wrong projection/limit: %s", stdout)
	}
	if !strings.Contains(stderr, "two candidates") || !f.deadline || strings.Count(stdout, "\n") != 1 {
		t.Fatalf("missing warning/deadline/compact: %q %s", stderr, stdout)
	}
	if strings.Contains(stdout, "results\":{\"results") {
		t.Fatalf("duplicate envelope: %s", stdout)
	}
}

func TestTenkiWrongProjectionIsError(t *testing.T) {
	f := &tenkiFake{place: tenki.PlaceResult{Status: "ok", Place: tenki.Place{Name: "千代田区", URL: tenkiTokyo}}}
	_, _, _, err, _ := tenkiRun(t, f, "places", "show", "--place", tenkiTokyo, "--json", "--select", "results.place.deliberately_missing")
	if err == nil {
		t.Fatal("mismatched selector unexpectedly succeeded")
	}
}

func TestTenkiDailyActualDatesAndDetail(t *testing.T) {
	f := &tenkiFake{daily: tenki.ForecastResult{Status: "ok", CoverageStart: "2026-09-27", CoverageEnd: "2026-10-10", Source: tenki.Source{URL: tenkiTokyo + "10days.html", Freshness: "fresh"}, Periods: []tenki.Period{{Date: "2026-09-27", Kind: "forecast"}, {Date: "2026-09-28", Kind: "forecast"}, {Date: "2026-09-29", Kind: "forecast"}}, Intervals: []tenki.Period{{Date: "2026-09-28", Start: "2026-09-28T00:00:00+09:00", End: "2026-09-28T06:00:00+09:00"}}, Instants: []tenki.Period{{Date: "2026-09-28", ValidAt: "2026-09-28T00:00:00+09:00"}, {Date: "2026-09-28", ValidAt: "2026-09-28T06:00:00+09:00"}}}}
	value, _, _, err, _ := tenkiRun(t, f, "forecast", "daily", "--place", tenkiTokyo, "--from", "2026-09-28", "--days", "1", "--detail", "--json")
	if err != nil {
		t.Fatal(err)
	}
	result := tenkiResult(t, value)
	rows := result["periods"].([]any)
	if len(rows) != 1 || rows[0].(map[string]any)["date"] != "2026-09-28" || rows[0].(map[string]any)["temperature_c"] != nil {
		t.Fatalf("actual dates/nulls: %#v", rows)
	}
	if len(result["intervals"].([]any)) != 1 || len(result["instants"].([]any)) != 2 {
		t.Fatalf("details aligned incorrectly: %#v", result)
	}
	if result["coverage_end"] != "2026-10-10" {
		t.Fatal("source horizon lost")
	}
	value, _, _, err, _ = tenkiRun(t, f, "forecast", "daily", "--place", tenkiTokyo, "--from", "2030-01-01", "--days", "1")
	if err != nil {
		t.Fatal(err)
	}
	result = tenkiResult(t, value)
	if result["status"] != "out_of_horizon" || len(result["periods"].([]any)) != 0 {
		t.Fatalf("unavailable fabricated rows: %#v", result)
	}
	if _, present := result["intervals"]; present {
		t.Fatal("summary should omit detail intervals")
	}
	if _, present := result["instants"]; present {
		t.Fatal("summary should omit detail instants")
	}
}

func TestTenkiHourlyEndpointsAndMidnight(t *testing.T) {
	rows := []tenki.Period{}
	for _, hour := range []int{8, 9, 10, 17, 18, 24} {
		instant := time.Date(2026, 9, 28, 0, 0, 0, 0, time.FixedZone("JST", 9*60*60)).Add(time.Duration(hour) * time.Hour)
		rows = append(rows, tenki.Period{Date: "2026-09-28", Start: instant.Add(-time.Hour).Format(time.RFC3339), End: instant.Format(time.RFC3339), ValidAt: instant.Format(time.RFC3339), Kind: "forecast"})
	}
	f := &tenkiFake{hourly: tenki.ForecastResult{Status: "ok", Periods: rows}}
	value, _, _, err, _ := tenkiRun(t, f, "forecast", "hourly", "--place", tenkiTokyo, "--date", "2026-09-28", "--hours", "09:00-17:00")
	if err != nil {
		t.Fatal(err)
	}
	periods := tenkiResult(t, value)["periods"].([]any)
	if len(periods) != 3 || periods[0].(map[string]any)["valid_at"] != "2026-09-28T09:00:00+09:00" || periods[2].(map[string]any)["valid_at"] != "2026-09-28T17:00:00+09:00" {
		t.Fatalf("endpoint filter wrong: %#v", periods)
	}
	value, _, _, err, _ = tenkiRun(t, f, "forecast", "hourly", "--place", tenkiTokyo, "--date", "2026-09-28", "--hours", "23:00-24:00")
	if err != nil {
		t.Fatal(err)
	}
	periods = tenkiResult(t, value)["periods"].([]any)
	if len(periods) != 1 || periods[0].(map[string]any)["valid_at"] != "2026-09-29T00:00:00+09:00" {
		t.Fatalf("24:00 rollover wrong: %#v", periods)
	}
}

func TestTenkiMountainExactLevelAndNoSummit(t *testing.T) {
	f := &tenkiFake{mountain: tenki.MountainResult{Status: "ok", ModelKind: "nearby_model_guidance", Levels: []tenki.ModelLevel{{ElevationM: 3000, ValidAt: "2026-09-28T09:00:00+09:00"}, {ElevationM: 4400, ValidAt: "2026-09-28T09:00:00+09:00"}}}}
	value, _, _, err, _ := tenkiRun(t, f, "mountain", "show", "--place", tenkiFuji, "--level", "3000")
	if err != nil {
		t.Fatal(err)
	}
	mountain := tenkiResult(t, value)["mountain"].(map[string]any)
	if len(mountain["levels"].([]any)) != 1 || mountain["summit_forecast_available"] != false {
		t.Fatalf("level/summit invalid: %#v", mountain)
	}
	value, _, _, err, _ = tenkiRun(t, f, "mountain", "show", "--place", tenkiFuji, "--level", "3100")
	if err != nil {
		t.Fatal(err)
	}
	mountain = tenkiResult(t, value)["mountain"].(map[string]any)
	if mountain["status"] != "level_unavailable" || len(mountain["levels"].([]any)) != 0 {
		t.Fatalf("interpolated absent level: %#v", mountain)
	}
}

func TestTenkiSeasonEvidenceAndEmptyArrays(t *testing.T) {
	f := &tenkiFake{season: tenki.SeasonalResult{Kind: "sakura", RequestedYear: 2027, Year: 2026, Status: "year_unavailable", UpdateState: "ended", Spot: tenki.SeasonalSpot{Year: 2026, NormalPeriod: "3月下旬～4月上旬"}}, seasonList: tenki.SeasonalListResult{Kind: "kouyou", Year: 2026, Status: "no_results"}}
	value, _, _, err, _ := tenkiRun(t, f, "seasonal", "show", "--kind", "sakura", "--place", "https://tenki.jp/sakura/3/16/54401.html", "--year", "2027")
	if err != nil {
		t.Fatal(err)
	}
	result := tenkiResult(t, value)
	if result["status"] != "year_unavailable" || result["requested_year"] != float64(2027) || result["year"] != float64(2026) || result["update_state"] != "ended" {
		t.Fatalf("year/state laundering: %#v", result)
	}
	value, _, _, err, _ = tenkiRun(t, f, "seasonal", "list", "--kind", "kouyou", "--query", "definitely_absent")
	if err != nil {
		t.Fatal(err)
	}
	result = tenkiResult(t, value)
	if len(result["spots"].([]any)) != 0 {
		t.Fatalf("empty spots must []: %#v", result)
	}
}

func TestTenkiSourceModesErrorsAndMetrics(t *testing.T) {
	f := &tenkiFake{place: tenki.PlaceResult{Status: "ok", Source: tenki.Source{URL: tenkiTokyo, FromCache: true, Freshness: "fresh"}}}
	value, _, _, err, config := tenkiRun(t, f, "places", "show", "--place", tenkiTokyo, "--data-source", "local", "--allow-stale", "--cache-dir", t.TempDir(), "--timeout", "1s")
	if err != nil {
		t.Fatal(err)
	}
	if !config.Local || !config.AllowStale || config.Refresh || config.Timeout != time.Second {
		t.Fatalf("source config: %#v", config)
	}
	meta := value["meta"].(map[string]any)
	if meta["source"] != "local" || meta["metrics"].(map[string]any)["http_requests"] != float64(1) {
		t.Fatalf("actual provenance/metrics: %#v", meta)
	}
	f.err = &cliutil.RateLimitError{URL: tenkiTokyo}
	_, _, _, err, _ = tenkiRun(t, f, "places", "show", "--place", tenkiTokyo, "--data-source", "live")
	if ExitCode(err) != 7 {
		t.Fatalf("throttle lost: %v", err)
	}
	f.err = errors.New("source unavailable")
	_, _, _, err, _ = tenkiRun(t, f, "places", "show", "--place", tenkiTokyo)
	if ExitCode(err) != 5 {
		t.Fatalf("source error code: %v", err)
	}
}

func TestTenkiCoreHelpIsFocused(t *testing.T) {
	root := RootCmd()
	paths := []string{"places search", "places show", "forecast daily", "forecast hourly", "mountain show", "seasonal list", "seasonal show", "compare"}
	for _, path := range paths {
		cmd, _, err := root.Find(strings.Split(path, " "))
		if err != nil || cmd.CommandPath() != "tenki-pp-cli "+path {
			t.Fatalf("missing command %s", path)
		}
		if cmd.Annotations["mcp:read-only"] != "true" || cmd.Annotations["pp:data-source"] != "auto" || !strings.Contains(cmd.Example, "tenki.jp") && path != "places search" && path != "seasonal list" {
			t.Fatalf("incomplete command contract %s", path)
		}
		if cobratree.CommandPathForInvocation(root, path) != path {
			t.Fatalf("MCP mirror cannot discover leaf %s", path)
		}
	}
	if cobratree.CommandPathForInvocation(root, "places") != "" {
		t.Fatal("help-only parent must not be an MCP tool")
	}
	for _, cmd := range root.Commands() {
		if cmd.Name() == "profile" && !cmd.Hidden {
			t.Fatal("unrelated profile should be hidden")
		}
	}
	for _, name := range []string{"audit-dir", "client-profile", "deliver", "receipt", "profile", "yes", "max-age"} {
		if flag := root.PersistentFlags().Lookup(name); flag == nil || !flag.Hidden {
			t.Errorf("unrelated flag %s should be hidden", name)
		}
	}
	if !strings.Contains(root.PersistentFlags().Lookup("data-source").Usage, "fresh cache") || !strings.Contains(root.PersistentFlags().Lookup("rate-limit").Usage, "at most 1") {
		t.Fatal("source/cache/rate help does not match runtime")
	}
	rateHelp := root.PersistentFlags().Lookup("rate-limit").Usage
	if !strings.Contains(rateHelp, "per command/client") || !strings.Contains(rateHelp, "concurrent CLI/MCP invocations pace independently") {
		t.Fatal("rate help must explain the scope of pacing")
	}
}

func TestTenkiShowCommandsRemainDistinct(t *testing.T) {
	root := RootCmd()
	cases := []struct {
		path   []string
		flags  map[string]string
		absent []string
	}{
		{[]string{"places", "show"}, map[string]string{"place": "string"}, []string{"level", "kind", "year"}},
		{[]string{"mountain", "show"}, map[string]string{"place": "string", "level": "float64", "limit": "int"}, []string{"kind", "year"}},
		{[]string{"seasonal", "show"}, map[string]string{"place": "string", "kind": "string", "year": "int"}, []string{"level", "limit"}},
	}
	for _, test := range cases {
		t.Run(strings.Join(test.path, "/"), func(t *testing.T) {
			command, remaining, err := root.Find(test.path)
			if err != nil || len(remaining) != 0 || command.Use != "show" || command.CommandPath() != "tenki-pp-cli "+strings.Join(test.path, " ") {
				t.Fatalf("real show registration changed: %v %v %v", command, remaining, err)
			}
			for name, kind := range test.flags {
				flag := command.Flags().Lookup(name)
				if flag == nil || flag.Value.Type() != kind {
					t.Errorf("--%s should have type %s", name, kind)
				}
			}
			for _, name := range test.absent {
				if command.Flags().Lookup(name) != nil {
					t.Errorf("--%s leaked from a different show command", name)
				}
			}
			if command.Annotations["mcp:read-only"] != "true" || command.Annotations["pp:data-source"] != "auto" {
				t.Fatal("read-only source contract changed")
			}
		})
	}
	profile, _, err := root.Find([]string{"profile", "show"})
	if err != nil || !strings.Contains(profile.Use, "<name>") {
		t.Fatal("fixture no longer exercises the competing profile show signature")
	}
}

func TestTenkiCanonicalPlaces(t *testing.T) {
	for _, value := range []string{tenkiTokyo, tenkiFuji, tenkiOze, "https://tenki.jp/leisure/6/29/189/7327/", "https://tenki.jp/leisure/3/15/80/100001/"} {
		if err := validateTenkiPlace(value); err != nil {
			t.Errorf("canonical URL rejected %s: %v", value, err)
		}
	}
	for _, value := range []string{"https://tenki.jp/leisure/6/29/", "https://tenki.jp/leisure/6/29/189/7327.html", "https://tenki.jp/docs/rule/", "https://tenki.jp.evil.test/forecast/3/16/4410/13101/", "https://tenki.jp/forecast/%33/16/4410/13101/", " " + tenkiTokyo} {
		if err := validateTenkiPlace(value); err == nil {
			t.Errorf("unrelated URL accepted %s", value)
		}
	}
}

func TestTenkiSeasonalDirectoryIsScopedAndMatchingKind(t *testing.T) {
	f := &tenkiFake{seasonList: tenki.SeasonalListResult{Kind: "sakura", Year: 2026, Status: "season_ended", UpdateState: "ended", Source: tenki.Source{URL: "https://tenki.jp/sakura/3/16/"}, Scanned: 12}}
	value, _, _, err, config := tenkiRun(t, f, "seasonal", "list", "--kind", "sakura", "--directory", "https://tenki.jp/sakura/3/16/", "--query", "上野", "--year", "2026")
	if err != nil {
		t.Fatal(err)
	}
	result := tenkiResult(t, value)
	if config.SearchDirectory != "https://tenki.jp/sakura/3/16/" || result["source"].(map[string]any)["url"] != "https://tenki.jp/sakura/3/16/" || result["scanned"] != float64(12) || result["update_state"] != "ended" {
		t.Fatalf("directory coverage/year state lost: %#v", result)
	}
	_, _, _, err, _ = tenkiRun(t, &tenkiFake{}, "seasonal", "list", "--kind", "sakura", "--directory", "https://tenki.jp/kouyou/3/16/", "--query", "上野")
	if ExitCode(err) != 2 {
		t.Fatalf("directory of wrong product accepted: %v", err)
	}
}

func TestTenkiGeneratedLifecycleCompare(t *testing.T) {
	root := RootCmd()
	count := 0
	for _, command := range root.Commands() {
		if command.Name() != "compare" {
			continue
		}
		count++
		if isNovelScaffoldCommand(command) || command.Annotations["mcp:read-only"] != "true" || command.Flags().Lookup("place").Value.Type() != "stringArray" || command.Flags().Lookup("days").Value.Type() != "int" {
			t.Fatal("final generated lifecycle retained a scaffold instead of real compare")
		}
	}
	if count != 1 {
		t.Fatalf("final root contains %d compare commands", count)
	}
	if cobratree.CommandPathForInvocation(root, "compare") != "compare" {
		t.Fatal("real compare missing from MCP traversal")
	}
	adapter := newNovelCompareCmd(&rootFlags{})
	if isNovelScaffoldCommand(adapter) || adapter.Flags().Lookup("max-pop").Value.Type() != "float64" {
		t.Fatal("generated constructor adapter is not the typed implementation")
	}
	for _, args := range [][]string{{"compare", "--help"}, {"compare", "--dry-run", "--json"}} {
		root := RootCmd()
		var output bytes.Buffer
		root.SetOut(&output)
		root.SetErr(&output)
		root.SetArgs(args)
		if err := root.Execute(); err != nil {
			t.Fatalf("real root lifecycle %v failed: %v", args, err)
		}
		if args[1] == "--help" {
			if !strings.Contains(output.String(), "--season-condition") || !strings.Contains(output.String(), "--max-rain") {
				t.Fatal("compare help lacks real command options")
			}
		} else {
			var dry map[string]any
			if err := json.Unmarshal(output.Bytes(), &dry); err != nil || dry["dry_run"] != true {
				t.Fatalf("input-free dry-run not JSON success: %s", output.String())
			}
		}
	}
}

func TestTenkiDailyPartialHorizonIsDistinctFromLimit(t *testing.T) {
	f := &tenkiFake{daily: tenki.ForecastResult{Status: "ok", CoverageStart: "2026-09-27", CoverageEnd: "2026-10-10", Periods: []tenki.Period{{Date: "2026-10-08", Kind: "forecast"}, {Date: "2026-10-09", Kind: "forecast"}, {Date: "2026-10-10", Kind: "forecast"}}}}
	value, _, _, err, _ := tenkiRun(t, f, "forecast", "daily", "--place", tenkiTokyo, "--from", "2026-10-10", "--days", "3", "--json")
	if err != nil {
		t.Fatal(err)
	}
	result := tenkiResult(t, value)
	unsupported := result["unsupported_dates"].([]any)
	if result["status"] != "partial_horizon" || len(result["periods"].([]any)) != 1 || len(unsupported) != 2 || unsupported[0] != "2026-10-11" || unsupported[1] != "2026-10-12" {
		t.Fatalf("partial horizon is silent: %#v", result)
	}
	value, _, _, err, _ = tenkiRun(t, f, "forecast", "daily", "--place", tenkiTokyo, "--from", "2026-10-08", "--days", "3", "--limit", "1", "--json")
	if err != nil {
		t.Fatal(err)
	}
	result = tenkiResult(t, value)
	if result["status"] != "ok" || len(result["unsupported_dates"].([]any)) != 0 || result["truncated"] != true || len(result["periods"].([]any)) != 1 {
		t.Fatalf("limit confused with source absence: %#v", result)
	}
	f.daily.Status = "unavailable"
	value, _, _, err, _ = tenkiRun(t, f, "forecast", "daily", "--place", tenkiTokyo, "--from", "2026-10-11", "--days", "1", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if tenkiResult(t, value)["status"] != "unavailable" {
		t.Fatal("source unavailable status overwritten")
	}
}
