// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/appstore"
	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/cliutil"
)

func TestRootCountryLangFlags(t *testing.T) {
	root := RootCmd()
	country := root.PersistentFlags().Lookup("country")
	lang := root.PersistentFlags().Lookup("lang")
	if country == nil || country.DefValue != "us" {
		t.Fatalf("--country flag = %+v", country)
	}
	if lang == nil || lang.DefValue != "" {
		t.Fatalf("--lang flag = %+v", lang)
	}
	for _, name := range []string{"json", "select", "dry-run", "no-cache", "timeout", "rate-limit", "data-source", "agent"} {
		if root.PersistentFlags().Lookup(name) == nil {
			t.Fatalf("generated flag --%s lost", name)
		}
	}
}

func TestNewAppStoreClientFromFlags(t *testing.T) {
	c, err := newAppStoreClient(&rootFlags{country: "PK", lang: "en-GB", noCache: true})
	if err != nil {
		t.Fatal(err)
	}
	if c.Country() != "pk" || c.Lang() != "en-GB" {
		t.Fatalf("client country %s lang %s", c.Country(), c.Lang())
	}
	if _, err := newAppStoreClient(&rootFlags{country: "zz"}); ExitCode(err) != 2 {
		t.Fatalf("unknown --country exit = %d (%v), want 2", ExitCode(err), err)
	}
	// --rate-limit 0.1 lowers the Search API (0.3/s) and Lookup (1/s)
	// classes; pages already go one per 20 s and stay there.
	c, err = newAppStoreClient(&rootFlags{rateLimit: 0.1, noCache: true})
	if err != nil || c.LimiterRate(appstore.RateSearchAPI) != 0.1 || c.LimiterRate(appstore.RateITunes) != 0.1 || c.LimiterRate(appstore.RatePages) != 1.0/20 {
		t.Fatalf("--rate-limit ceiling not applied: %v", err)
	}
	c, _ = newAppStoreClient(&rootFlags{rateLimit: 50, noCache: true})
	if c.LimiterRate(appstore.RatePages) != 1.0/20 || c.LimiterRate(appstore.RateSearchAPI) != 0.3 {
		t.Fatal("--rate-limit must never raise the per-host defaults")
	}
}

func TestClassifyAppStoreErr(t *testing.T) {
	rl := &cliutil.RateLimitError{URL: "https://itunes.apple.com/lookup?id=1", Body: "HTTP 429 from Apple; the governor stopped this run (no retry)"}
	cases := []struct {
		name string
		err  error
		code int
	}{
		{"throttle", rl, 7},
		{"wrapped throttle", fmt.Errorf("lookup: %w", rl), 7},
		{"not found", &appstore.NotFoundError{URL: "https://apps.apple.com/us/app/id1"}, 3},
		{"opt-in", appstore.ErrOptInRequired, 2},
		{"http 500", &appstore.HTTPError{URL: "u", Status: 500}, 5},
		{"timeout", fmt.Errorf("x: %w", context.DeadlineExceeded), 5},
		{"already typed", usageErr(errors.New("bad")), 2},
		{"wrong-shaped 200", &appstore.EnvelopeError{Surface: "iTunes Lookup/Search", Reason: "the body is null"}, 5},
		{"refused redirect", fmt.Errorf("refusing redirect from a to b: %w", appstore.ErrOptInRequired), 2},
	}
	for _, tc := range cases {
		got := classifyAppStoreErr(tc.err)
		if ExitCode(got) != tc.code {
			t.Errorf("%s: exit %d, want %d (%v)", tc.name, ExitCode(got), tc.code, got)
		}
	}
	got := classifyAppStoreErr(rl)
	var back *cliutil.RateLimitError
	if !errors.As(got, &back) || !isRateLimited(got) {
		t.Fatal("classified throttle must keep *cliutil.RateLimitError reachable")
	}
	if !strings.Contains(got.Error(), "rate limited by Apple") || strings.Contains(got.Error(), "HTTP 429 for") {
		t.Fatalf("throttle message = %q", got.Error())
	}
	if classifyAppStoreErr(nil) != nil {
		t.Fatal("nil must stay nil")
	}
}

func TestArgumentHelpers(t *testing.T) {
	if _, err := parseAppRef("not an app"); ExitCode(err) != 2 {
		t.Fatalf("bad app ref exit = %d", ExitCode(err))
	}
	ref, err := parseAppRef("https://apps.apple.com/jp/app/id324684580")
	if err != nil || ref.ID != "324684580" || ref.Country != "jp" {
		t.Fatalf("ref = %+v %v", ref, err)
	}
	cmd := RootCmd()
	if cc, err := refCountry(cmd, &rootFlags{country: "us"}, ref); err != nil || cc != "jp" {
		t.Fatalf("URL storefront should win when --country is not set: %s %v", cc, err)
	}
	if err := cmd.PersistentFlags().Set("country", "gb"); err != nil {
		t.Fatal(err)
	}
	if cc, _ := refCountry(cmd, &rootFlags{country: "gb"}, ref); cc != "gb" {
		t.Fatalf("explicit --country should win: %s", cc)
	}
	if p, err := parsePlatform("iPad"); err != nil || p != "ipad" {
		t.Fatalf("platform = %s %v", p, err)
	}
	if _, err := parseChartPlatform("watch"); ExitCode(err) != 2 {
		t.Fatal("watch charts must be a usage error")
	}
	if g, err := resolveGenre("puzzle"); err != nil || g.ID != "7012" {
		t.Fatalf("genre = %+v %v", g, err)
	}
	if g, err := resolveGenre(""); err != nil || g.ID != "36" {
		t.Fatalf("empty genre = %+v %v", g, err)
	}
	if _, err := resolveGenre("nope"); ExitCode(err) != 2 {
		t.Fatal("unknown genre must be a usage error")
	}
}

func TestResolveAgeBandFlag(t *testing.T) {
	// Unset or empty: the overall chart (no band), never All Kids (0).
	for _, raw := range []string{"", "   "} {
		if b, err := resolveAgeBand(raw); err != nil || b.ID != "" {
			t.Fatalf("resolveAgeBand(%q) = %+v %v, want the zero band", raw, b, err)
		}
	}
	for raw, want := range map[string]string{"all": "0", "kids": "0", "0": "0", "6-8": "10001", "10002": "10002"} {
		if b, err := resolveAgeBand(raw); err != nil || b.ID != want {
			t.Fatalf("resolveAgeBand(%q) = %+v %v, want %s", raw, b, err, want)
		}
	}
	if _, err := resolveAgeBand("teens"); ExitCode(err) != 2 {
		t.Fatal("an unknown band must be a usage error")
	}
	all, _ := resolveGenre("36")
	games, _ := resolveGenre("6014")
	kids, _ := resolveAgeBand("6-8")
	if err := checkChartAgeBand(kids, all, "iphone", "top-free"); err != nil {
		t.Fatalf("Kids 6-8 on iphone/36/top-free: %v", err)
	}
	for _, c := range []struct {
		genre    appstore.Genre
		platform string
		chart    string
	}{{games, "iphone", "top-free"}, {all, "mac", "top-free"}, {all, "ipad", "top-grossing"}} {
		if err := checkChartAgeBand(kids, c.genre, c.platform, c.chart); ExitCode(err) != 2 {
			t.Fatalf("Kids band on %s/%s/%s must be a usage error, got %v", c.platform, c.genre.ID, c.chart, err)
		}
	}
	if err := checkChartAgeBand(appstore.KidsAgeBand{}, games, "mac", "top-grossing"); err != nil {
		t.Fatalf("the overall chart needs no band check: %v", err)
	}
	if ageBandLabel("") != "" || ageBandLabel("10001") != "Kids Ages 6-8" || ageBandLabel("0") != "All Kids" {
		t.Fatalf("labels: %q %q %q", ageBandLabel(""), ageBandLabel("10001"), ageBandLabel("0"))
	}
}

func TestCountriesFlagAndDogfoodCaps(t *testing.T) {
	list, err := countriesFlag(&rootFlags{country: "pk"}, "")
	if err != nil || strings.Join(list, ",") != "pk" {
		t.Fatalf("default countries = %v %v", list, err)
	}
	list, err = countriesFlag(&rootFlags{country: "us"}, "us,gb,jp")
	if err != nil || len(list) != 3 {
		t.Fatalf("countries = %v %v", list, err)
	}
	if _, err := countriesFlag(&rootFlags{}, "us,zz"); ExitCode(err) != 2 {
		t.Fatal("bad storefront must be a usage error")
	}
	if capForDogfood(200, dogfoodMaxChartLimit) != 200 {
		t.Fatal("caps must not apply outside dogfood")
	}
	t.Setenv(cliutil.DogfoodEnvVar, "1")
	list, _ = countriesFlag(&rootFlags{country: "us"}, "us,gb,jp")
	if len(list) != dogfoodMaxCountries {
		t.Fatalf("dogfood countries = %v, want %d", list, dogfoodMaxCountries)
	}
	if capForDogfood(200, dogfoodMaxChartLimit) != 50 || capForDogfood(10, dogfoodMaxChartLimit) != 10 || capForDogfood(0, dogfoodMaxReviewPages) != 1 {
		t.Fatal("dogfood caps wrong")
	}
}

func TestOpenAppStoreStoreAndPrintRows(t *testing.T) {
	cmd := RootCmd()
	dbPath := filepath.Join(t.TempDir(), "x.db")
	cmd.Flags().String("db", "", "")
	if err := cmd.Flags().Set("db", dbPath); err != nil {
		t.Fatal(err)
	}
	if got := appStoreDBPath(cmd); got != dbPath {
		t.Fatalf("db path = %s", got)
	}
	if _, ok := appStoreMirrorExists(cmd); ok {
		t.Fatal("mirror should not exist yet")
	}
	st, err := openAppStoreStore(cmd, &rootFlags{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	if _, ok := appStoreMirrorExists(cmd); !ok {
		t.Fatal("store file not created")
	}
	c, _ := newAppStoreClient(&rootFlags{noCache: true})
	attachCaptureLog(c, st)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	type row struct {
		Rank  int    `json:"rank"`
		AppID string `json:"appId"`
	}
	flags := &rootFlags{asJSON: true}
	if err := printRows(cmd, flags, make([]row, 0)); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(buf.String()) != "[]" {
		t.Fatalf("empty rows JSON = %q, want []", buf.String())
	}
	buf.Reset()
	if err := printRows(cmd, flags, []row{{1, "324684580"}}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"appId": "324684580"`) && !strings.Contains(buf.String(), `"appId":"324684580"`) {
		t.Fatalf("rows JSON = %q", buf.String())
	}
	var warn bytes.Buffer
	warnFetchFailures(&warn, 2, 10, "storefronts")
	if warn.String() != "warning: 2 of 10 fetches failed; results cover the remaining 8 storefronts\n" {
		t.Fatalf("warning = %q", warn.String())
	}
}

// s1RoundTrip answers every App Store request from a function (no network).
type s1RoundTrip func(*http.Request) (*http.Response, error)

func (f s1RoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func s1LookupClient(t *testing.T, body string) (*appstore.Client, *int) {
	t.Helper()
	requests := 0
	transport := s1RoundTrip(func(r *http.Request) (*http.Response, error) {
		requests++
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/javascript"}},
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    r,
		}, nil
	})
	c, err := appstore.New(appstore.Options{Country: "us", NoCache: true, Jitter: -1, HTTPClient: &http.Client{Transport: transport}})
	if err != nil {
		t.Fatal(err)
	}
	return c, &requests
}

// Review defect: a wrong-shaped 200 from Lookup was "verified empty", so
// `app <id>` exited 3 (not found) and prices recorded "unavailable". Only
// the documented empty envelope may mean "no such app".
func TestS1LookupAppRefNeedsTheEnvelopeForNotFound(t *testing.T) {
	ref := appstore.AppRef{ID: "324684580", Raw: "324684580"}
	for _, body := range []string{`{}`, `null`, `{"errorMessage":"Invalid value(s) for key(s): [id]"}`, `{"resultCount":3}`} {
		c, requests := s1LookupClient(t, body)
		_, err := lookupAppRef(context.Background(), c, ref, "us")
		if ExitCode(err) != 5 || !appstore.IsEnvelopeError(err) {
			t.Fatalf("body %s: exit %d (%v), want 5 with an envelope error, not 3", body, ExitCode(err), err)
		}
		if *requests != 1 {
			t.Fatalf("body %s: requests = %d", body, *requests)
		}
	}
	c, _ := s1LookupClient(t, `{"resultCount":0,"results":[]}`)
	_, err := lookupAppRef(context.Background(), c, ref, "us")
	if ExitCode(err) != 3 || !strings.Contains(err.Error(), "no App Store app 324684580 in storefront us") {
		t.Fatalf("documented empty Lookup: exit %d (%v), want 3", ExitCode(err), err)
	}
}
