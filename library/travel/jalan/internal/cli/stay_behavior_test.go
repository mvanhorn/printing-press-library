package cli

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/jalan/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/jalan/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/travel/jalan/internal/jalan"
)

type fakeStayService struct {
	response             jalan.Response
	err                  error
	method               string
	property, plan, room string
	query                jalan.Query
	dates                []string
	plans                []jalan.PlanRef
	deadline             time.Time
}

func (f *fakeStayService) record(ctx context.Context, method, id string, q jalan.Query) (jalan.Response, error) {
	f.method, f.property, f.query = method, id, q
	f.deadline, _ = ctx.Deadline()
	return f.response, f.err
}
func (f *fakeStayService) Search(ctx context.Context, q jalan.Query) (jalan.Response, error) {
	return f.record(ctx, "search", "", q)
}
func (f *fakeStayService) Property(ctx context.Context, id string) (jalan.Response, error) {
	return f.record(ctx, "property", id, jalan.Query{})
}
func (f *fakeStayService) Offers(ctx context.Context, id string, q jalan.Query) (jalan.Response, error) {
	return f.record(ctx, "offers", id, q)
}
func (f *fakeStayService) Plan(ctx context.Context, id, plan, room string, q jalan.Query) (jalan.Response, error) {
	f.plan, f.room = plan, room
	return f.record(ctx, "plan", id, q)
}
func (f *fakeStayService) Compare(ctx context.Context, id string, q jalan.Query, dates []string, plans []jalan.PlanRef) (jalan.Response, error) {
	f.dates, f.plans = dates, plans
	return f.record(ctx, "compare", id, q)
}

func installStayFake(t *testing.T, f *fakeStayService) *jalan.Options {
	t.Helper()
	testenv.Isolate(t)
	previous := stayClientFactory
	options := new(jalan.Options)
	stayClientFactory = func(o jalan.Options) stayService { *options = o; return f }
	t.Cleanup(func() { stayClientFactory = previous })
	return options
}
func executeStay(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	cmd := RootCmd()
	var out, diagnostic bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&diagnostic)
	cmd.SetArgs(append([]string{"--no-learn", "stay"}, args...))
	err := cmd.Execute()
	return out.String(), diagnostic.String(), err
}
func futureStayDate() string {
	return time.Now().In(time.FixedZone("Asia/Tokyo", 9*3600)).AddDate(0, 0, 10).Format("2006-01-02")
}
func stayFixture() jalan.Response {
	return jalan.Response{
		Meta:       map[string]any{"source": "live", "observed_at": "2026-09-27T23:00:00+09:00", "timezone": "Asia/Tokyo", "query": map[string]any{"adults_per_room": 2}, "upstream_requests": 1, "status": "observed"},
		Results:    []any{map[string]any{"id": "385995", "name_ja": "箱根の宿", "url": "https://www.jalan.net/yad385995/?a=1&b=2", "baths": map[string]any{"in_room": nil, "hot_spring": true}, "price": map[string]any{"amount": 46200, "basis": "whole_stay", "extra_fees": "入湯税は別"}}},
		Pagination: map[string]any{"page": 1, "has_more": false}, FetchFailures: []map[string]any{},
	}
}
func decodeStay(t *testing.T, out string) map[string]any {
	t.Helper()
	var value map[string]any
	if err := json.Unmarshal([]byte(out), &value); err != nil {
		t.Fatalf("stdout is not one JSON object: %v: %s", err, out)
	}
	return value
}

func TestStayJSONAndAgentRetainFactsInOneEnvelope(t *testing.T) {
	fake := &fakeStayService{response: stayFixture()}
	installStayFake(t, fake)
	for _, mode := range [][]string{{}, {"--json"}, {"--agent"}, {"--compact"}} {
		out, diagnostic, err := executeStay(t, append([]string{"property", "385995"}, mode...)...)
		if err != nil || diagnostic != "" {
			t.Fatalf("mode %v: %v stderr=%s", mode, err, diagnostic)
		}
		value := decodeStay(t, out)
		if len(value) != 4 || value["data"] != nil {
			t.Fatalf("double/lost envelope: %s", out)
		}
		result := value["results"].([]any)[0].(map[string]any)
		if result["price"].(map[string]any)["extra_fees"] != "入湯税は別" || result["baths"].(map[string]any)["in_room"] != nil {
			t.Fatalf("agent dropped decision facts: %s", out)
		}
		if strings.Count(out, "\n") != 1 || strings.Contains(out, `\u0026`) {
			t.Fatalf("JSON must be compact with readable source URLs: %s", out)
		}
	}
}

func TestStaySelectProjectsItemsAndKeepsCorrectnessMetadata(t *testing.T) {
	fake := &fakeStayService{response: stayFixture()}
	installStayFake(t, fake)
	out, diagnostic, err := executeStay(t, "property", "385995", "--agent", "--select", "id,price.amount,baths.in_room")
	if err != nil || diagnostic != "" {
		t.Fatalf("%v stderr=%s", err, diagnostic)
	}
	value := decodeStay(t, out)
	meta := value["meta"].(map[string]any)
	if meta["observed_at"] == nil || meta["query"] == nil || value["pagination"] == nil {
		t.Fatalf("metadata lost: %s", out)
	}
	result := value["results"].([]any)[0].(map[string]any)
	if len(result) != 3 || result["name_ja"] != nil || len(result["price"].(map[string]any)) != 1 {
		t.Fatalf("projection wrong: %s", out)
	}
	out, diagnostic, err = executeStay(t, "property", "385995", "--select", "nonexistent.path")
	if err == nil || ExitCode(err) != 2 || out != "" || !json.Valid([]byte(diagnostic)) {
		t.Fatalf("select miss: out=%s err=%v stderr=%s", out, err, diagnostic)
	}
}

func TestStayDryRunDoesNotConstructClientOrRequireLiveArguments(t *testing.T) {
	fake := &fakeStayService{}
	installStayFake(t, fake)
	constructed := false
	stayClientFactory = func(jalan.Options) stayService { constructed = true; return fake }
	for _, command := range []string{"search", "offers", "property", "plan", "compare", "locations", "capabilities"} {
		out, diagnostic, err := executeStay(t, command, "--dry-run", "--agent", "--cache-dir", t.TempDir())
		// Catalogue commands intentionally have no cache flag.
		if command == "locations" || command == "capabilities" {
			out, diagnostic, err = executeStay(t, command, "--dry-run", "--agent")
		}
		if err != nil || diagnostic != "" {
			t.Fatalf("%s dry-run: %v stderr=%s", command, err, diagnostic)
		}
		value := decodeStay(t, out)
		if len(value) != 6 || value["dry_run"] != true || value["action"] != "jalan-pp-cli stay "+command {
			t.Fatalf("dry-run must retain the envelope and expose compatible markers: %s", out)
		}
		meta := value["meta"].(map[string]any)
		if meta["dry_run"] != true || meta["upstream_requests"] != float64(0) {
			t.Fatalf("missing dry-run observation: %s", out)
		}
	}
	if constructed {
		t.Fatal("dry-run constructed live client")
	}
}

func TestStayPartyFiltersPagingAndCacheOptionsReachService(t *testing.T) {
	fake := &fakeStayService{response: stayFixture()}
	options := installStayFake(t, fake)
	out, diagnostic, err := executeStay(t, "search", "--destination", "Hakone", "--check-in", futureStayDate(), "--rooms", "2", "--adults", "2", "--children-elementary", "1", "--children-meals-bed", "2", "--children-meals", "3", "--children-bed", "4", "--children-neither", "5", "--nights", "2", "--limit", "7", "--page", "3", "--meals", "breakfast_dinner", "--lodging-type", "ryokan", "--onsen", "--outdoor-bath", "--private-bath", "--room-outdoor-bath", "--non-smoking", "--max-age", "2m", "--refresh", "--cache-dir", "/tmp/jalan-test-only", "--timeout", "12s")
	if err != nil || diagnostic != "" {
		t.Fatalf("%v stdout=%s stderr=%s", err, out, diagnostic)
	}
	q := fake.query
	if fake.method != "search" || q.Destination != "Hakone" || q.Rooms != 2 || q.Children != [5]int{1, 2, 3, 4, 5} || q.Page != 3 || q.Limit != 7 || !q.NonSmoking || !q.RoomOutdoorBath || !q.Onsen || !q.OutdoorBath || !q.PrivateBath {
		t.Fatalf("query altered: %#v", q)
	}
	if options.MaxAge != 0 || !options.Refresh || options.CacheDir != "/tmp/jalan-test-only" || options.Timeout != 12*time.Second {
		t.Fatalf("options lost: %#v", options)
	}
	if remaining := time.Until(fake.deadline); remaining <= 0 || remaining > 12*time.Second {
		t.Fatalf("root timeout missing: %v", remaining)
	}
}

func TestStayInvalidInputHasEmptyStdoutAndNoSourceCalls(t *testing.T) {
	fake := &fakeStayService{response: stayFixture()}
	installStayFake(t, fake)
	checks := [][]string{
		{"search"}, {"search", "--destination", "NowhereTown", "--check-in", futureStayDate()},
		{"search", "--destination", "Hakone", "--check-in", futureStayDate(), "--adults", "9"},
		{"search", "--destination", "Hakone", "--check-in", futureStayDate(), "--children-elementary", "6"},
		{"search", "--destination", "Hakone", "--check-in", "2026-02-30"},
		{"property"}, {"property", "nonsense"},
		{"plan", "385995", "--check-in", futureStayDate()},
		{"compare", "385995", "--dates", futureStayDate(), "--plans", "03912759:0576806"},
		{"compare", "385995", "--dates", futureStayDate(), "--check-in", futureStayDate()},
		{"compare", "385995", "--check-in", futureStayDate(), "--plans", "03912759:0576806,03912759:0576806"},
		{"property", "385995", "--max-age", "6m"}, {"property", "385995", "--timeout", "0"}, {"property", "385995", "--timeout", "61s"},
		{"property", "385995", "--data-source", "local"},
	}
	for _, name := range []string{"adults", "rooms", "nights", "page", "limit"} {
		checks = append(checks, []string{"search", "--destination", "Hakone", "--check-in", futureStayDate(), "--" + name, "0"})
	}
	for _, args := range checks {
		fake.method = ""
		out, diagnostic, err := executeStay(t, args...)
		if err == nil || ExitCode(err) != 2 || out != "" || !json.Valid([]byte(diagnostic)) || fake.method != "" {
			t.Fatalf("%v: out=%q stderr=%q err=%v call=%s", args, out, diagnostic, err, fake.method)
		}
	}
}

func TestStayCompareModesAndPartialFailure(t *testing.T) {
	fake := &fakeStayService{response: stayFixture()}
	installStayFake(t, fake)
	date := futureStayDate()
	_, _, err := executeStay(t, "compare", "385995", "--check-in", date, "--plans", "03912759:0576806,03806855:0546600")
	if err != nil || len(fake.plans) != 2 || fake.plans[1].RoomID != "0546600" || len(fake.dates) != 0 {
		t.Fatalf("exact plans not preserved: %#v err=%v", fake, err)
	}
	fake.response.FetchFailures = []map[string]any{{"date": date, "code": "parse_failure", "message": "source format changed"}}
	fake.err = &jalan.PartialError{Failures: fake.response.FetchFailures, Cause: &jalan.Error{Code: "parse_failure", Message: "source format changed"}}
	out, diagnostic, err := executeStay(t, "compare", "385995", "--dates", date, "--limit", "3")
	if err == nil || ExitCode(err) != 8 || !json.Valid([]byte(out)) || !json.Valid([]byte(diagnostic)) {
		t.Fatalf("partial: out=%s stderr=%s err=%v", out, diagnostic, err)
	}
	value := decodeStay(t, out)
	if len(value["results"].([]any)) != 1 || len(value["fetch_failures"].([]any)) != 1 {
		t.Fatalf("partial dropped observations: %s", out)
	}
}

func TestStaySourceErrorsRemainDistinctAndActionable(t *testing.T) {
	fake := &fakeStayService{}
	installStayFake(t, fake)
	for code, exit := range map[string]int{"access_failure": 4, "rate_limit": 7, "parse_failure": 9, "unavailable": 3, "upstream": 5} {
		fake.err = &jalan.Error{Code: code, Message: "source observation failed", Hint: "inspect source", URL: "https://www.jalan.net/"}
		out, diagnostic, err := executeStay(t, "property", "385995")
		if out != "" || err == nil || ExitCode(err) != exit {
			t.Fatalf("%s: out=%s err=%v", code, out, err)
		}
		value := decodeStay(t, diagnostic)["error"].(map[string]any)
		if value["code"] != code || value["hint"] != "inspect source" {
			t.Fatalf("actionable source error lost: %s", diagnostic)
		}
		var source *jalan.Error
		if !errors.As(err, &source) {
			t.Fatalf("typed service error lost: %v", err)
		}
	}
}

func TestStayCSVUsesFrameworkFormat(t *testing.T) {
	fake := &fakeStayService{response: stayFixture()}
	installStayFake(t, fake)
	out, diagnostic, err := executeStay(t, "property", "385995", "--csv", "--select", "id,name_ja")
	if err != nil || diagnostic != "" {
		t.Fatalf("%v stderr=%s", err, diagnostic)
	}
	records, err := csv.NewReader(strings.NewReader(out)).ReadAll()
	if err != nil || len(records) != 2 {
		t.Fatalf("invalid CSV: %v %s", err, out)
	}
	if !strings.Contains(out, "385995") {
		t.Fatalf("CSV lost identity: %s", out)
	}
}

func TestStayCommandsAreReadOnlyImplementedAndUnique(t *testing.T) {
	testenv.Isolate(t)
	root := RootCmd()
	stay, _, err := root.Find([]string{"stay"})
	if err != nil {
		t.Fatal(err)
	}
	if len(stay.Commands()) != 7 {
		t.Fatalf("stay child count=%d", len(stay.Commands()))
	}
	seen := map[string]bool{}
	for _, cmd := range stay.Commands() {
		if seen[cmd.Name()] || cmd.Annotations["mcp:read-only"] != "true" || isNovelScaffoldCommand(cmd) {
			t.Fatalf("unimplemented/duplicate/nonreadonly command %s", cmd.Name())
		}
		seen[cmd.Name()] = true
		if cmd.RunE == nil {
			t.Fatal(fmt.Sprintf("%s has no implementation", cmd.Name()))
		}
	}
}

func TestStayEvidenceSharesURLsAndRetainsSourceMapWithSelection(t *testing.T) {
	fixture := stayFixture()
	item := fixture.Results[0].(map[string]any)
	sourceURL := "https://www.jalan.net/uw/uwp3200/uww3201init.do?stayYear=2026&roomCrack=200000"
	item["evidence"] = []map[string]any{{"field": "name_ja", "text": "箱根の宿", "url": sourceURL}}
	item["price"].(map[string]any)["evidence"] = []map[string]any{{"field": "price.amount", "text": "46,200円", "url": sourceURL}}
	fake := &fakeStayService{response: fixture}
	installStayFake(t, fake)
	out, diagnostic, err := executeStay(t, "property", "385995", "--agent")
	if err != nil || diagnostic != "" {
		t.Fatalf("%v stderr=%s", err, diagnostic)
	}
	value := decodeStay(t, out)
	sources := value["meta"].(map[string]any)["sources"].(map[string]any)
	if len(sources) != 1 || sources["s1"] != sourceURL {
		t.Fatalf("source map wrong: %s", out)
	}
	result := value["results"].([]any)[0].(map[string]any)
	evidence := result["evidence"].([]any)[0].(map[string]any)
	if evidence["source_ref"] != "s1" || evidence["url"] != nil || result["url"] != item["url"] {
		t.Fatalf("evidence/canonical URL wrong: %s", out)
	}
	out, _, err = executeStay(t, "property", "385995", "--select", "id,price.amount")
	if err != nil || decodeStay(t, out)["meta"].(map[string]any)["sources"] == nil {
		t.Fatalf("selection lost source map: %v %s", err, out)
	}
}

func TestStayCacheHonorsHomeEnvironmentAndExplicitOverride(t *testing.T) {
	fake := &fakeStayService{response: stayFixture()}
	options := installStayFake(t, fake)
	home := t.TempDir()
	t.Setenv("JALAN_HOME", home)
	t.Setenv("JALAN_CACHE_DIR", "")
	_, _, err := executeStay(t, "property", "385995", "--max-age", "1m")
	if err != nil || options.CacheDir != filepath.Join(home, "cache", "observations") || options.MaxAge != time.Minute {
		t.Fatalf("env home lost: options=%#v err=%v", options, err)
	}
	explicit := t.TempDir()
	_, _, err = executeStay(t, "property", "385995", "--home", t.TempDir(), "--cache-dir", explicit, "--max-age", "1m", "--no-cache")
	if err != nil || options.CacheDir != explicit || options.MaxAge != 0 || !options.Refresh || !options.DisableCache {
		t.Fatalf("override/no-cache lost: options=%#v err=%v", options, err)
	}
}

func TestStayAllFailedCompareRetainsFailuresOnStderr(t *testing.T) {
	failures := []map[string]any{{"check_in": futureStayDate(), "code": "parse_failure", "message": "changed source markup"}}
	fake := &fakeStayService{err: &jalan.Error{Code: "parse_failure", Message: "all source alternatives failed", Hint: "inspect failures", FetchFailures: failures}}
	installStayFake(t, fake)
	out, diagnostic, err := executeStay(t, "compare", "385995", "--dates", futureStayDate())
	if out != "" || err == nil || ExitCode(err) != 9 {
		t.Fatalf("total failure produced observations: out=%s err=%v", out, err)
	}
	if len(decodeStay(t, diagnostic)["fetch_failures"].([]any)) != 1 {
		t.Fatalf("all failed cells dropped: %s", diagnostic)
	}
}

func TestStayExecuteReportsCobraFlagFailuresAsOneJSONDiagnostic(t *testing.T) {
	testenv.Isolate(t)
	savedArgs, savedOut, savedErr := os.Args, os.Stdout, os.Stderr
	outRead, outWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	errRead, errWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer outRead.Close()
	defer errRead.Close()
	t.Cleanup(func() { os.Args, os.Stdout, os.Stderr = savedArgs, savedOut, savedErr })
	os.Args = []string{"jalan-pp-cli", "--no-learn", "stay", "property", "385995", "--not-a-real-flag"}
	os.Stdout, os.Stderr = outWrite, errWrite
	runErr := Execute()
	outWrite.Close()
	errWrite.Close()
	os.Args, os.Stdout, os.Stderr = savedArgs, savedOut, savedErr
	out, _ := io.ReadAll(outRead)
	diagnostic, _ := io.ReadAll(errRead)
	if runErr == nil || ExitCode(runErr) != 2 || len(out) != 0 || !json.Valid(diagnostic) {
		t.Fatalf("flag failure: out=%q stderr=%q err=%v", out, diagnostic, runErr)
	}
	if strings.Count(string(diagnostic), "\n") != 1 {
		t.Fatalf("multiple diagnostics: %s", diagnostic)
	}
}

func TestStayTypedRateLimitErrorKeepsSourceAndRetryAfter(t *testing.T) {
	rate := &cliutil.RateLimitError{URL: "https://www.jalan.net/yad385995/", RetryAfter: 2 * time.Second, Cause: context.DeadlineExceeded}
	fake := &fakeStayService{err: rate}
	installStayFake(t, fake)
	out, diagnostic, err := executeStay(t, "property", "385995")
	if out != "" || err == nil || ExitCode(err) != 7 {
		t.Fatalf("429 collapsed into timeout/empty: out=%s stderr=%s err=%v", out, diagnostic, err)
	}
	value := decodeStay(t, diagnostic)
	if value["retry_after_ms"] != float64(2000) || value["error"].(map[string]any)["url"] != rate.URL {
		t.Fatalf("429 source/retry missing: %s", diagnostic)
	}
	var retained *cliutil.RateLimitError
	if !errors.As(err, &retained) {
		t.Fatalf("typed 429 cause lost: %v", err)
	}
}

func TestStayDefaultExecutionCreatesNoLearningState(t *testing.T) {
	fake := &fakeStayService{response: stayFixture()}
	installStayFake(t, fake)
	for _, name := range []string{"JALAN_NO_LEARN", "JALAN_LEARN_NO_CAPTURE", "PRINTING_PRESS_VERIFY", "PRINTING_PRESS_DOGFOOD"} {
		t.Setenv(name, "")
	}
	home := t.TempDir()
	for _, args := range [][]string{{"property", "385995"}, {"search", "--dry-run"}, {"plan"}} {
		savedArgs, savedOut, savedErr := os.Args, os.Stdout, os.Stderr
		outRead, outWrite, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		errRead, errWrite, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		os.Args = append([]string{"jalan-pp-cli", "--home", home, "stay"}, args...)
		os.Stdout, os.Stderr = outWrite, errWrite
		runErr := Execute()
		outWrite.Close()
		errWrite.Close()
		os.Args, os.Stdout, os.Stderr = savedArgs, savedOut, savedErr
		output, _ := io.ReadAll(outRead)
		diagnostic, _ := io.ReadAll(errRead)
		outRead.Close()
		errRead.Close()
		if args[0] != "plan" && runErr != nil {
			t.Fatalf("%v: %v stderr=%s", args, runErr, diagnostic)
		}
		if args[0] != "plan" && !json.Valid(output) {
			t.Fatalf("default output invalid: %s", output)
		}
		entries, err := os.ReadDir(home)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 0 {
			t.Fatalf("stay %v created non-cache learning state in private home: %v", args, entries)
		}
	}
}

func TestStayMetadataFixturesExecuteWithRealFlagAndPositionalGrammar(t *testing.T) {
	fake := &fakeStayService{response: stayFixture()}
	options := installStayFake(t, fake)
	root := RootCmd()
	nextDate, err := time.Parse("2006-01-02", futureStayDate())
	if err != nil {
		t.Fatal(err)
	}
	nextDate = nextDate.AddDate(0, 0, 1)
	for _, name := range []string{"", "search", "locations", "property", "offers", "plan", "compare", "capabilities"} {
		path := []string{"stay"}
		if name != "" {
			path = append(path, name)
		}
		cmd, _, err := root.Find(path)
		if err != nil {
			t.Fatal(err)
		}
		if strings.TrimSpace(cmd.Example) == "" {
			t.Fatalf("%v has no runnable Examples section", path)
		}
		arguments := []string{}
		if name != "" {
			arguments = append(arguments, name)
		}
		var fixturePositionals int
		annotation := cmd.Annotations["pp:happy-args"]
		if annotation != "" {
			for _, token := range strings.Split(annotation, ";") {
				label, value, ok := strings.Cut(strings.TrimSpace(token), "=")
				if !ok || label == "" || value == "" {
					t.Fatalf("%v fixture must use name=value tokens: %q", path, annotation)
				}
				value = strings.NewReplacer("2026-11-10", futureStayDate(), "2026-11-11", nextDate.Format("2006-01-02")).Replace(value)
				if strings.HasPrefix(label, "--") {
					if strings.ContainsAny(label, " \t") || cmd.Flags().Lookup(strings.TrimPrefix(label, "--")) == nil {
						t.Fatalf("%v fixture names a bogus flag: %q", path, label)
					}
					arguments = append(arguments, label, value)
				} else {
					if !strings.HasPrefix(label, "<") || !strings.HasSuffix(label, ">") {
						t.Fatalf("%v fixture positional must name its placeholder: %q", path, label)
					}
					fixturePositionals++
					arguments = append(arguments, value)
				}
			}
		}
		if strings.Contains(cmd.Use, "<property-id>") && fixturePositionals != 1 {
			t.Fatalf("%v has no authoritative real property fixture", path)
		}
		out, diagnostic, err := executeStay(t, arguments...)
		if err != nil || diagnostic != "" || !json.Valid([]byte(out)) {
			t.Fatalf("fixture %v failed: out=%s stderr=%s err=%v", arguments, out, diagnostic, err)
		}
		if len(decodeStay(t, out)) != 4 {
			t.Fatalf("live fixture changed envelope: %s", out)
		}
		switch name {
		case "search", "property", "offers", "plan", "compare":
			if options.MaxAge != 5*time.Minute {
				t.Fatalf("%v fixture must opt in to 5m reuse for formatting checks: %#v", path, options)
			}
			if cmd.Flags().Lookup("max-age").DefValue != "0s" {
				t.Fatalf("%v fixture changed the fresh runtime default", path)
			}
			if strings.Contains(cmd.Example, "--max-age") {
				t.Fatalf("%v fixture changed the fresh help example", path)
			}
		}
	}
}
