package cli

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/tenki/internal/tenki"
)

const yearFixtureSpot = "https://tenki.jp/kouyou/4/19/30314.html"

func seasonalYearFixtureClient(t *testing.T) *tenki.Client {
	t.Helper()
	bodies := map[string]string{}
	for url, name := range map[string]string{
		yearFixtureSpot:            "foliage-murodo-20260927.html",
		"https://tenki.jp/kouyou/": "foliage-index-20260927.html",
		"https://tenki.jp/forecast/4/19/5510/16323/10days.html": "daily-20260927.html",
	} {
		body, err := os.ReadFile("../tenki/testdata/" + name)
		if err != nil {
			t.Fatal(err)
		}
		bodies[url] = string(body)
	}
	// Weather values are replayed solely to exercise comparison acquisition;
	// replace the municipality label so it matches the seasonal source link.
	bodies["https://tenki.jp/forecast/4/19/5510/16323/10days.html"] = strings.ReplaceAll(bodies["https://tenki.jp/forecast/4/19/5510/16323/10days.html"], "千代田区", "立山町")
	return tenki.NewClient(tenki.Config{
		Now: func() time.Time { return time.Date(2026, 9, 27, 23, 30, 0, 0, tenki.JST) },
		Transport: tenkiOutputRoundTripper(func(req *http.Request) (*http.Response, error) {
			body, ok := bodies[req.URL.String()]
			if !ok {
				return nil, fmt.Errorf("unexpected fixture URL %s", req.URL)
			}
			return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"text/html"}}, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
		}),
	})
}

func TestTenkiSeasonalYearBoundsThroughRealProvider(t *testing.T) {
	client := seasonalYearFixtureClient(t)
	for _, year := range []int{1900, 1999, 2000, 2100, 2101, 2200} {
		for _, path := range []string{"list", "show", "compare"} {
			t.Run(path+"/"+strconv.Itoa(year), func(t *testing.T) {
				var args []string
				switch path {
				case "list":
					args = []string{"seasonal", "list", "--kind", "kouyou"}
				case "show":
					args = []string{"seasonal", "show", "--kind", "kouyou", "--place", yearFixtureSpot}
				case "compare":
					args = []string{"compare", "--place", yearFixtureSpot, "--season", "kouyou", "--from", "2026-09-28", "--days", "1", "--max-pop", "100"}
				}
				args = append(args, "--year", strconv.Itoa(year), "--agent")
				value, stdout, _, err, _ := tenkiRun(t, client, args...)
				if err != nil {
					t.Fatalf("accepted year returned exit%d: %v", ExitCode(err), err)
				}
				result := tenkiResult(t, value)
				if path == "compare" {
					if result["partial"] != false || len(result["fetch_failures"].([]any)) != 0 {
						t.Fatalf("unavailable source year became fetch failure: %s", stdout)
					}
					cell := result["comparison"].(map[string]any)["cells"].([]any)[0].(map[string]any)
					seasonal := cell["seasonal"].(map[string]any)
					if cell["status"] != "insufficient_data" || seasonal["verdict"] != "unknown" || seasonal["requested_year"] != float64(year) {
						t.Fatalf("unsupported season earned suitability: %s", stdout)
					}
					result = seasonal["evidence"].(map[string]any)
				}
				if result["status"] != "year_unavailable" || result["requested_year"] != float64(year) || result["year"] != float64(2026) {
					t.Fatalf("requested/source year lost: %s", stdout)
				}
			})
		}
	}
	for _, year := range []int{1899, 2201} {
		for _, args := range [][]string{
			{"seasonal", "list", "--kind", "kouyou"},
			{"seasonal", "show", "--kind", "kouyou", "--place", yearFixtureSpot},
			{"compare", "--place", yearFixtureSpot, "--season", "kouyou", "--max-pop", "100"},
		} {
			t.Run(strings.Join(args[:2], "/")+"/invalid/"+strconv.Itoa(year), func(t *testing.T) {
				before := client.Metrics().HTTPRequests
				_, stdout, _, err, _ := tenkiRun(t, client, append(args, "--year", strconv.Itoa(year), "--json")...)
				if ExitCode(err) != 2 || stdout != "" || client.Metrics().HTTPRequests != before {
					t.Fatalf("invalid year did not fail before source calls: %v %s", err, stdout)
				}
			})
		}
	}
}
