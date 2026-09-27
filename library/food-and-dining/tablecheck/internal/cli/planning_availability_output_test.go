package cli

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/tablecheck/internal/planner"
	"github.com/spf13/cobra"
)

func availabilityOutputFixture() planner.Result {
	return planner.Result{
		"venues": []map[string]any{{"id": "venue-a-id", "slug": "venue-a", "name_ja": "会場A"}, {"id": nil, "slug": "venue-b", "name_ja": nil}},
		"checks": []map[string]any{
			{"venue_id": "venue-a-id", "slug": "venue-a", "date": "2026-09-30", "party": 2, "status": "available", "scope": "venue", "available_times": []string{"17:30"}, "error": nil},
			{"venue_id": "venue-a-id", "slug": "venue-a", "date": "2026-10-01", "party": 2, "status": "unknown", "scope": "venue", "available_times": []string{}, "error": "date missing from returned calendar"},
			{"venue_id": nil, "slug": "venue-b", "date": "2026-09-30", "party": 2, "status": "failed", "scope": "venue", "available_times": []string{}, "error": "upstream HTTP 503"},
			{"venue_id": nil, "slug": "venue-b", "date": "2026-10-01", "party": 2, "status": "failed", "scope": "venue", "available_times": []string{}, "error": "upstream HTTP 503"},
		},
		"fetch_failures": []map[string]any{{"slug": "venue-b", "error": "upstream HTTP 503"}},
		"window":         map[string]any{"from": "2026-09-30", "to": "2026-10-01", "days": 2, "venues": 2},
		"meta":           map[string]any{"source": "live", "requests": 3, "transport": "http", "freshness": map[string]any{"observed_at": "2026-09-27T14:00:00Z", "served_at": "2026-09-27T14:00:01Z"}},
	}
}

func renderAvailabilityOutput(t *testing.T, flags rootFlags) (string, error) {
	t.Helper()
	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)
	err := planningPrint(cmd, &flags, availabilityOutputFixture())
	return out.String(), err
}

func TestPlanningAvailabilityNativeFormatsRenderEveryCheckRow(t *testing.T) {
	for _, tc := range []struct {
		name      string
		flags     rootFlags
		separator rune
	}{
		{"csv", rootFlags{csv: true}, ','},
		{"plain", rootFlags{plain: true}, '\t'},
		{"selected csv", rootFlags{csv: true, selectFields: "checks.slug,checks.date,checks.status"}, ','},
		{"selected plain", rootFlags{plain: true, selectFields: "checks.slug,checks.date,checks.status"}, '\t'},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := renderAvailabilityOutput(t, tc.flags)
			if err != nil {
				t.Fatal(err)
			}
			reader := csv.NewReader(strings.NewReader(out))
			reader.Comma = tc.separator
			rows, err := reader.ReadAll()
			if err != nil || len(rows) != 5 {
				t.Fatalf("expected header plus four check rows, including failures: rows=%v err=%v output=%q", rows, err, out)
			}
			columns := map[string]int{}
			for i, name := range rows[0] {
				columns[name] = i
			}
			for _, name := range []string{"slug", "date", "status"} {
				if _, ok := columns[name]; !ok {
					t.Fatalf("missing check column %s: %v", name, rows[0])
				}
			}
			for _, name := range []string{"venues", "checks", "fetch_failures", "meta", "window"} {
				if _, ok := columns[name]; ok {
					t.Fatalf("outer envelope rendered as a row: %v", rows[0])
				}
			}
			if tc.flags.selectFields != "" && len(rows[0]) != 3 {
				t.Fatalf("selection included extra columns: %v", rows[0])
			}
			want := [][]string{{"venue-a", "2026-09-30", "available"}, {"venue-a", "2026-10-01", "unknown"}, {"venue-b", "2026-09-30", "failed"}, {"venue-b", "2026-10-01", "failed"}}
			for i, row := range rows[1:] {
				got := []string{row[columns["slug"]], row[columns["date"]], row[columns["status"]]}
				if !reflect.DeepEqual(got, want[i]) {
					t.Fatalf("check row lost or changed: got=%v want=%v", got, want[i])
				}
			}
		})
	}
}

func TestPlanningAvailabilityQuietReturnsSlugPerCheckIncludingFailures(t *testing.T) {
	for _, selection := range []string{"", "checks.slug", "checks.slug,checks.date,checks.status", "checks", "checks.*", "CHECKS.SLUG"} {
		t.Run(selection, func(t *testing.T) {
			out, err := renderAvailabilityOutput(t, rootFlags{quiet: true, selectFields: selection})
			if err != nil {
				t.Fatal(err)
			}
			if want := "venue-a\nvenue-a\nvenue-b\nvenue-b\n"; out != want {
				t.Fatalf("quiet scan must retain every date/failure identity: got=%q want=%q", out, want)
			}
		})
	}
}

func TestPlanningAvailabilityQuietRejectsNonIdentitySelectionBeforeOutput(t *testing.T) {
	for _, selection := range []string{"checks.date", "checks.date,checks.status", "checks.venue_id"} {
		out, err := renderAvailabilityOutput(t, rootFlags{quiet: true, selectFields: selection})
		if err == nil || ExitCode(err) != 2 || !strings.Contains(err.Error(), "checks.slug") || out != "" {
			t.Fatalf("nonidentity quiet selection must fail deterministically before output: selection=%q err=%v output=%q", selection, err, out)
		}
	}
}

func TestPlanningAvailabilityDefaultAndAgentJSONRetainCompleteScanEnvelope(t *testing.T) {
	for _, agent := range []bool{false, true} {
		out, err := renderAvailabilityOutput(t, rootFlags{agent: agent, asJSON: agent, compact: agent})
		if err != nil {
			t.Fatal(err)
		}
		var got, want map[string]any
		if err := json.Unmarshal([]byte(out), &got); err != nil {
			t.Fatal(err)
		}
		encoded, _ := json.Marshal(availabilityOutputFixture())
		_ = json.Unmarshal(encoded, &want)
		if !reflect.DeepEqual(got, want) || strings.Count(strings.TrimSpace(out), "\n") != 0 {
			t.Fatalf("native projection leaked into complete JSON schema/freshness: %s", out)
		}
	}
}

func TestPlanningAvailabilityNativeWriterFailuresStillPropagate(t *testing.T) {
	for _, flags := range []rootFlags{{csv: true}, {plain: true}, {quiet: true}} {
		failure := errors.New("availability output writer failed")
		cmd := &cobra.Command{}
		cmd.SetOut(publicationFailingWriter{failure})
		if err := planningPrint(cmd, &flags, availabilityOutputFixture()); !errors.Is(err, failure) {
			t.Fatalf("native check rendering swallowed writer error: %v", err)
		}
	}
}
