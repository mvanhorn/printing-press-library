package cli

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/tenki/internal/tenki"
)

type tenkiOutputRoundTripper func(*http.Request) (*http.Response, error)

func (f tenkiOutputRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func tenkiOutputSnapshot(t *testing.T) tenki.ForecastResult {
	t.Helper()
	body, err := os.ReadFile("../tenki/testdata/daily-20260927.html")
	if err != nil {
		t.Fatal(err)
	}
	client := tenki.NewClient(tenki.Config{
		Now: func() time.Time { return time.Date(2026, 9, 27, 23, 30, 0, 0, tenki.JST) },
		Transport: tenkiOutputRoundTripper(func(req *http.Request) (*http.Response, error) {
			if req.URL.String() != tenkiTokyo+"10days.html" {
				t.Fatalf("unexpected source request: %s", req.URL)
			}
			return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"text/html"}},
				Body: io.NopCloser(strings.NewReader(string(body))), Request: req}, nil
		}),
	})
	result, err := client.Daily(context.Background(), tenkiTokyo)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func tenkiOutputJSONValue(t *testing.T, value any) any {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	return decoded
}

func TestTenkiDailyOutputRetainsEvidenceUnderAgentAndCompact(t *testing.T) {
	// Use the real dated SSR response rather than sparse hand-built records.
	// These fields occur on fewer than 80% of rows and must survive formatting.
	snapshot := tenkiOutputSnapshot(t)
	for _, test := range []struct {
		name  string
		flags []string
	}{{"agent", []string{"--agent"}}, {"compact", []string{"--compact"}}, {"agent_and_compact", []string{"--agent", "--compact"}}} {
		t.Run(test.name, func(t *testing.T) {
			args := []string{"forecast", "daily", "--place", tenkiTokyo, "--from", "2026-09-27", "--days", "14", "--detail", "--cache-dir", t.TempDir(), "--home", t.TempDir()}
			value, stdout, _, err, _ := tenkiRun(t, &tenkiFake{daily: snapshot}, append(args, test.flags...)...)
			if err != nil {
				t.Fatal(err)
			}
			result := tenkiResult(t, value)
			periods := result["periods"].([]any)
			if got := periods[0].(map[string]any)["weather_probability_from"]; got != "2026-09-27T23:00:00+09:00" {
				t.Fatalf("day-zero remaining-weather onset lost: %v", got)
			}
			if periods[11].(map[string]any)["confidence"] != "D" || periods[13].(map[string]any)["confidence"] != "E" {
				t.Fatal("late outlook confidence was removed by output formatting")
			}
			for key, expected := range map[string]any{"source": snapshot.Source, "periods": snapshot.Periods, "intervals": snapshot.Intervals, "instants": snapshot.Instants} {
				if !reflect.DeepEqual(result[key], tenkiOutputJSONValue(t, expected)) {
					t.Fatalf("%s source evidence changed during output formatting", key)
				}
			}
			if strings.Count(stdout, "\n") != 1 || !json.Valid([]byte(stdout)) {
				t.Fatal("JSON minification or valid single-line output changed")
			}
		})
	}
}

func TestTenkiDailyOutputSelectStillNarrowsEvidence(t *testing.T) {
	snapshot := tenkiOutputSnapshot(t)
	for _, flag := range []string{"--agent", "--compact"} {
		t.Run(strings.TrimPrefix(flag, "--"), func(t *testing.T) {
			args := []string{"forecast", "daily", "--place", tenkiTokyo, "--from", "2026-09-27", "--days", "14", "--detail", "--cache-dir", t.TempDir(), "--home", t.TempDir(), flag}
			_, full, _, err, _ := tenkiRun(t, &tenkiFake{daily: snapshot}, args...)
			if err != nil {
				t.Fatal(err)
			}
			projection := "results.periods.weather_probability_from,results.periods.confidence,results.source.issue_at"
			value, narrowed, _, err, _ := tenkiRun(t, &tenkiFake{daily: snapshot}, append(args, "--select", projection)...)
			if err != nil {
				t.Fatal(err)
			}
			if len(narrowed) >= len(full) || strings.Contains(narrowed, "max_temperature_c") || strings.Contains(narrowed, "intervals") {
				t.Fatal("explicit field selection no longer narrows the response")
			}
			result := tenkiResult(t, value)
			if result["source"].(map[string]any)["issue_at"] != snapshot.Source.IssueAt {
				t.Fatal("selected source issue time lost")
			}
			foundOnset, foundConfidence := false, false
			for _, item := range result["periods"].([]any) {
				row := item.(map[string]any)
				foundOnset = foundOnset || row["weather_probability_from"] == snapshot.Source.IssueAt
				foundConfidence = foundConfidence || row["confidence"] == "E"
				for key := range row {
					if key != "weather_probability_from" && key != "confidence" {
						t.Fatalf("unselected period field retained: %s", key)
					}
				}
			}
			if !foundOnset || !foundConfidence {
				t.Fatal("selected sparse onset/confidence fields lost")
			}
		})
	}
}
