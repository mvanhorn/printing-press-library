package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/walkerplus/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/walkerplus/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/travel/walkerplus/internal/walkerplus"
)

type walkerFake struct {
	query                                   walkerplus.Query
	searchCalls, shortlistCalls, eventCalls int
	err                                     error
	wait                                    bool
}

func fixtureWalkerEvent() walkerplus.Event {
	date := "2026-10-11"
	prefecture := "ar0727"
	return walkerplus.Event{
		ID: "ar0727e612159", TitleJA: "大阪の屋内イベント", SourceURL: "https://www.walkerplus.com/event/ar0727e612159/",
		StartDate: &date, EndDate: &date,
		Location:   walkerplus.Location{PrefectureCode: &prefecture},
		Categories: []walkerplus.CatalogItem{}, OrganizerURLs: []string{}, Sources: []walkerplus.Source{}, Evidence: []walkerplus.Evidence{},
		Schedule:  walkerplus.Schedule{OccurrenceDates: []string{}, ExcludedDates: []string{}, ClosedWeekdays: []string{}, Unresolved: []string{}},
		Admission: walkerplus.Admission{Status: "unknown"},
	}
}

func (f *walkerFake) Search(ctx context.Context, q walkerplus.Query) (walkerplus.Result, error) {
	f.searchCalls++
	f.query = q
	if _, ok := ctx.Deadline(); !ok {
		return walkerplus.Result{}, errors.New("missing command deadline")
	}
	if f.wait {
		<-ctx.Done()
		return walkerplus.Result{}, ctx.Err()
	}
	if f.err != nil {
		return walkerplus.Result{}, f.err
	}
	if q.From == "2100-10-01" {
		return walkerplus.Result{Query: q, Events: []walkerplus.Event{}, Coverage: walkerplus.Coverage{ScannedPages: 1}}, nil
	}
	return walkerplus.Result{Query: q, Events: []walkerplus.Event{fixtureWalkerEvent()}, Coverage: walkerplus.Coverage{RequestCount: 1, ScannedPages: 1}}, nil
}
func (f *walkerFake) Shortlist(ctx context.Context, q walkerplus.Query) (walkerplus.Result, error) {
	f.shortlistCalls++
	f.query = q
	return walkerplus.Result{Query: q, Events: []walkerplus.Event{fixtureWalkerEvent()}, Coverage: walkerplus.Coverage{DetailCount: 1}}, f.err
}
func (f *walkerFake) Event(ctx context.Context, input string) (walkerplus.Event, error) {
	f.eventCalls++
	return fixtureWalkerEvent(), f.err
}
func (f *walkerFake) Areas(ctx context.Context, p string) (walkerplus.CatalogResult, error) {
	return walkerplus.CatalogResult{Items: []walkerplus.CatalogItem{}, SourceURL: "https://www.walkerplus.com/event_list/"}, f.err
}
func (f *walkerFake) Categories() walkerplus.CatalogResult {
	return walkerplus.CatalogResult{Items: []walkerplus.CatalogItem{{Code: "eg0055", NameJA: "祭り", Aliases: []string{"festival"}}}}
}
func (f *walkerFake) Stats() walkerplus.Coverage { return walkerplus.Coverage{RequestCount: 3} }

func useWalkerFake(t *testing.T, fake *walkerFake) *walkerplus.Options {
	t.Helper()
	original := makeWalkerClient
	options := new(walkerplus.Options)
	makeWalkerClient = func(value walkerplus.Options) (walkerService, error) { *options = value; return fake, nil }
	t.Cleanup(func() { makeWalkerClient = original })
	return options
}

func runWalkerCLI(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	testenv.Isolate(t)
	root := RootCmd()
	root.SetArgs(args)
	var out, diagnostic bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&diagnostic)
	err := root.Execute()
	return out.String(), diagnostic.String(), err
}

func decodeWalkerOutput(t *testing.T, text string) map[string]any {
	t.Helper()
	var value map[string]any
	if err := json.Unmarshal([]byte(text), &value); err != nil {
		t.Fatalf("invalid JSON %q: %v", text, err)
	}
	if strings.Count(text, "\n") != 1 {
		t.Fatalf("default JSON must be one line, got %d lines", strings.Count(text, "\n"))
	}
	return value
}

func TestWalkerDefaultCompactSearchAndProjection(t *testing.T) {
	fake := &walkerFake{}
	useWalkerFake(t, fake)
	out, _, err := runWalkerCLI(t, "search", "--prefecture", "osaka", "--from", "2026-10-10", "--to", "2026-10-12")
	if err != nil {
		t.Fatal(err)
	}
	result := decodeWalkerOutput(t, out)
	event := result["events"].([]any)[0].(map[string]any)
	if event["title_ja"] != "大阪の屋内イベント" || event["description"] != nil {
		t.Fatalf("source title/null lost: %#v", event)
	}
	if fake.searchCalls != 1 || fake.shortlistCalls != 0 || fake.eventCalls != 0 {
		t.Fatalf("search must remain listing-only: %#v", fake)
	}
	if fake.query.Prefecture != "ar0727" || fake.query.Limit != 10 || fake.query.MaxPages != 3 {
		t.Fatalf("defaults/aliases: %#v", fake.query)
	}
	out, _, err = runWalkerCLI(t, "event", "ar0727e612159", "--agent", "--fields", "id,location.venue,admission.price,organizer_urls")
	if err != nil {
		t.Fatal(err)
	}
	result = decodeWalkerOutput(t, out)
	event = result["event"].(map[string]any)
	if len(event) != 4 || event["id"] != "ar0727e612159" {
		t.Fatalf("projection fields: %#v", event)
	}
	if event["location"].(map[string]any)["venue"] != nil || event["admission"].(map[string]any)["price"] != nil {
		t.Fatal("unknowns must stay explicit null")
	}
	if len(event["organizer_urls"].([]any)) != 0 || result["coverage"] == nil || result["meta"] == nil {
		t.Fatal("empty arrays/metadata must be retained")
	}
	if result["meta"].(map[string]any)["source"] != "live" || result["meta"].(map[string]any)["timezone"] != "Asia/Tokyo" {
		t.Fatal("source/date provenance missing")
	}
}

func TestWalkerShortlistFlagsAndRuntime(t *testing.T) {
	fake := &walkerFake{}
	opts := useWalkerFake(t, fake)
	out, _, err := runWalkerCLI(t, "shortlist", "--prefecture", "osaka", "--from", "2026-10-10", "--to", "2026-10-12", "--timing", "starts", "--sort", "start", "--free", "--indoor", "--max-details", "2", "--max-pages", "1", "--limit", "2", "--no-cache", "--refresh", "--cache-dir", "/private/tmp/walker-test-cache", "--cache-ttl", "1d", "--request-timeout", "2s", "--concurrency", "4", "--retries", "0")
	if err != nil {
		t.Fatal(err)
	}
	decodeWalkerOutput(t, out)
	if fake.shortlistCalls != 1 || !fake.query.Free || !fake.query.Indoor || fake.query.Timing != "starts" || fake.query.MaxDetails != 2 || fake.query.MaxPages != 1 {
		t.Fatalf("shortlist flags not honored: %#v", fake.query)
	}
	if opts.CacheTTL != 24*time.Hour || opts.Timeout != 2*time.Second || !opts.Refresh || !opts.NoCache || opts.Concurrency != 4 || opts.Retries != 0 {
		t.Fatalf("runtime flags not honored: %#v", opts)
	}
}

func TestWalkerOfflineDryRunAllCommands(t *testing.T) {
	original := makeWalkerClient
	makeWalkerClient = func(walkerplus.Options) (walkerService, error) {
		t.Fatal("dry-run constructed source client")
		return nil, nil
	}
	t.Cleanup(func() { makeWalkerClient = original })
	for _, command := range []string{"search", "shortlist", "event", "areas", "categories", "schema", "doctor"} {
		t.Run(command, func(t *testing.T) {
			out, _, err := runWalkerCLI(t, command, "--dry-run", "--json", "--cache-dir", "/private/tmp/nonexistent-walker-dryrun")
			if err != nil {
				t.Fatal(err)
			}
			var result map[string]any
			if err = json.Unmarshal([]byte(out), &result); err != nil || result["dry_run"] != true {
				t.Fatalf("dry-run JSON: %q %v", out, err)
			}
		})
	}
}

func TestWalkerInvalidInputsFailBeforeClient(t *testing.T) {
	original := makeWalkerClient
	makeWalkerClient = func(walkerplus.Options) (walkerService, error) {
		t.Fatal("invalid input constructed source client")
		return nil, nil
	}
	t.Cleanup(func() { makeWalkerClient = original })
	cases := [][]string{
		{"search", "--limit", "0"}, {"search", "--limit", "101"}, {"search", "--max-pages", "0"}, {"search", "--max-pages", "21"}, {"search", "--page", "0"},
		{"search", "--prefecture", "imaginary"}, {"search", "--category", "imaginary"},
		{"search", "--from", "2026-02-30", "--to", "2026-03-01"}, {"search", "--from", "2026-10-12", "--to", "2026-10-10"},
		{"search", "--select", "imaginary"}, {"search", "--request-timeout", "16s"}, {"search", "--cache-ttl", "2d"}, {"search", "--concurrency", "5"}, {"search", "--retries", "4"},
		{"search", "--timeout", "61s"}, {"search", "--page", "1001"}, {"search", "--city", "imaginary"}, {"search", "--prefecture", "kyoto", "--city", "ar0313104"},
		{"shortlist", "--json"}, {"shortlist", "--from", "2026-10-11", "--to", "2026-10-11", "--max-details", "31"},
		{"event", "--json"}, {"event", "--agent"}, {"event", "https://evil.example/event/ar0313e603640/"}, {"event", "https://www.walkerplus.com/event/ar0313e603640/data.html"},
		{"event", "ar0313e603640", "ar0727e612159"},
	}
	for _, args := range cases {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			_, _, err := runWalkerCLI(t, args...)
			if err == nil || ExitCode(err) != 2 {
				t.Fatalf("expected usage exit2: %v", err)
			}
		})
	}
}

func TestWalkerEmptyProjectionPreservesCoverage(t *testing.T) {
	useWalkerFake(t, &walkerFake{})
	out, _, err := runWalkerCLI(t, "search", "--prefecture", "osaka", "--from", "2100-10-01", "--to", "2100-10-31", "--select", "id,title_ja")
	if err != nil {
		t.Fatal(err)
	}
	result := decodeWalkerOutput(t, out)
	if len(result["events"].([]any)) != 0 || result["query"] == nil || result["coverage"] == nil || !strings.Contains(result["note"].(string), "--max-pages") {
		t.Fatalf("empty projection contract: %#v", result)
	}
}

func TestWalkerSourceErrorsAndDeadline(t *testing.T) {
	fake := &walkerFake{err: walkerplus.ErrNotFound}
	useWalkerFake(t, fake)
	out, _, err := runWalkerCLI(t, "event", "ar0313e603640")
	if ExitCode(err) != 3 {
		t.Fatalf("404 exit: %v", err)
	}
	result := decodeWalkerOutput(t, out)
	if result["error"].(map[string]any)["code"] != float64(3) || result["events"] != nil {
		t.Fatalf("failure disguised as result: %#v", result)
	}
	fake.err = &cliutil.RateLimitError{URL: "https://www.walkerplus.com/event_list/", RetryAfter: time.Second}
	out, _, err = runWalkerCLI(t, "search")
	if ExitCode(err) != 7 {
		t.Fatalf("rate limit must exit7: %v", err)
	}
	result = decodeWalkerOutput(t, out)
	if result["error"].(map[string]any)["code"] != float64(7) || result["events"] != nil {
		t.Fatal("throttling became empty success")
	}
	fake.err = walkerplus.ErrInvalidQuery
	out, _, err = runWalkerCLI(t, "search", "--prefecture", "tokyo", "--city", "shinjuku")
	if ExitCode(err) != 2 || !errors.Is(err, walkerplus.ErrInvalidQuery) {
		t.Fatalf("live city validation must exit2: %v", err)
	}
	result = decodeWalkerOutput(t, out)
	if result["error"].(map[string]any)["code"] != float64(2) || result["events"] != nil {
		t.Fatal("unknown source city became empty success")
	}
	fake.err = nil
	fake.wait = true
	out, _, err = runWalkerCLI(t, "search", "--timeout", "5ms")
	if ExitCode(err) != 5 || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("deadline not enforced: %v", err)
	}
	decodeWalkerOutput(t, out)
}

func TestWalkerRootAdvertisesFocusedSurface(t *testing.T) {
	out, _, err := runWalkerCLI(t, "--help")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"search", "shortlist", "event", "areas", "categories"} {
		if !strings.Contains(out, name) {
			t.Fatalf("missing workflow %s", name)
		}
	}
	if strings.Contains(out, "event-list") || strings.Contains(out, "\n  tail ") || strings.Contains(out, "\n  workflow ") {
		t.Fatal("unrelated framework exposed")
	}
	root := RootCmd()
	for _, child := range root.Commands() {
		if child.Name() == "tail" {
			t.Fatal("unsupported background tail command remains registered")
		}
	}
	cmd, _, err := root.Find([]string{"event"})
	if err != nil || cmd.HasSubCommands() {
		t.Fatal("sample endpoint event command remains")
	}
}
