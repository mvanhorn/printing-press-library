package cli

import (
	"bytes"
	"encoding/json"
	"github.com/spf13/cobra"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/snowjapan/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/travel/snowjapan/internal/snowjapan"
)

func TestSnowFrontierPreservesTiesAndMissingness(t *testing.T) {
	cases := []struct {
		name          string
		rows          []snowjapan.Fact
		want, missing int
	}{
		{"dominance", []snowjapan.Fact{{"id": "a", "vertical_m": 100.0, "courses": 2.0}, {"id": "b", "vertical_m": 90.0, "courses": 1.0}}, 1, 0},
		{"tradeoff", []snowjapan.Fact{{"id": "a", "vertical_m": 100.0, "courses": 1.0}, {"id": "b", "vertical_m": 90.0, "courses": 2.0}}, 2, 0},
		{"equal", []snowjapan.Fact{{"id": "a", "vertical_m": 0.0, "courses": 0.0}, {"id": "b", "vertical_m": 0.0, "courses": 0.0}}, 2, 0},
		{"unknown", []snowjapan.Fact{{"id": "a", "vertical_m": nil, "courses": 2.0}}, 0, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v, m := snowFrontier(tc.rows, []string{"vertical_m", "courses"})
			if len(v) != tc.want || m != tc.missing {
				t.Fatalf("frontier=%v missing=%d", v, m)
			}
		})
	}
}

func TestSnowWindowIsARecordedSpanNotOperation(t *testing.T) {
	parse := func(s string) time.Time { v, _ := time.Parse("2006-01-02", s); return v }
	f := snowjapan.Fact{"id": "goryu", "name": "Goryu", "source_url": "source"}
	h := []snowjapan.Fact{{"first_recorded_day": "2025-12-04", "last_recorded_day": "2026-05-03"}}
	cases := []struct {
		name, from, to, state string
		history               []snowjapan.Fact
		days                  int
	}{
		{"inside", "2026-03-28", "2026-04-05", "within_recorded_span", h, 9},
		{"partial", "2025-12-01", "2025-12-05", "partial_overlap", h, 2},
		{"outside", "2026-05-04", "2026-05-05", "outside_recorded_span", h, 0},
		{"missing", "2026-03-01", "2026-03-02", "unknown", nil, 0},
		{"multiple access bases", "2026-03-01", "2026-03-02", "unknown", append(h, h...), 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v, e := snowWindow(f, tc.history, parse(tc.from), parse(tc.to))
			if e != nil {
				t.Fatal(e)
			}
			if v["status"] != tc.state || v["overlap_calendar_days"] != tc.days || v["continuous_operation"] != "unknown" {
				t.Fatalf("%#v", v)
			}
		})
	}
}

func TestSnowChangesIgnoreObservationOnlyUpdates(t *testing.T) {
	cases := []struct {
		name     string
		new, old snowjapan.Fact
		want     int
	}{
		{"unchanged observations", snowjapan.Fact{"peak_m": 100.0, "observed_at": "new", "catalog_observed_at": "new"}, snowjapan.Fact{"peak_m": 100.0, "observed_at": "old", "catalog_observed_at": "old"}, 0},
		{"changed zero", snowjapan.Fact{"beginner_percent": 0.0}, snowjapan.Fact{"beginner_percent": 20.0}, 1},
		{"missing differs from zero", snowjapan.Fact{"courses": nil}, snowjapan.Fact{"courses": 0.0}, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := snowChanges(tc.new, tc.old)
			if len(v) != tc.want {
				t.Fatalf("%#v", v)
			}
		})
	}
}

func TestSnowTownPortfoliosKeepMunicipalitiesSeparate(t *testing.T) {
	rows := []snowjapan.Fact{{"id": "a", "name": "A", "town": "Same Town", "prefecture": "Nagano", "vertical_m": 100.0}, {"id": "b", "name": "B", "town": "Same Town", "prefecture": "Nagano", "vertical_m": nil}, {"id": "c", "name": "C", "town": "Same Town", "prefecture": "Niigata", "vertical_m": 200.0}}
	v := snowTownView(rows, map[string][]snowjapan.Fact{"a": {{"resort_id": "a"}}, "b": {{"resort_id": "b"}, {"resort_id": "b"}}})
	if len(v) != 2 || v[0]["listed_areas"] != 2 || v[0]["confirmed_endpoint_areas"] != 1 || v[0]["unknown_vertical_areas"] != 1 {
		t.Fatalf("%#v", v)
	}
	if _, ok := v[0]["summed_lifts"]; ok {
		t.Fatal("connected terrain was summed")
	}
	if v[0]["missing_endpoint_areas"] != 0 || v[0]["ambiguous_endpoint_areas"] != 1 || v[1]["missing_endpoint_areas"] != 1 {
		t.Fatal("ambiguous access-base records were counted as absent")
	}
}

func TestSnowExactCoverageScope(t *testing.T) {
	rows := []snowjapan.Fact{{"id": "nagano-prefecture/hakuba-village/goryu", "name": "Goryu"}, {"id": "nagano-prefecture/hakuba-village/happo", "name": "Happo"}}
	for _, tc := range []struct {
		csv   string
		fail  bool
		count int
	}{
		{"goryu,happo", false, 2},
		{"goryu", false, 1},
		{"goryu,goryu", true, 0},
		{"absent", true, 0},
		{"goryu,", true, 0},
		{"goryu,happo,goryu,happo,goryu", true, 0},
	} {
		got, e := snowExactScope(rows, tc.csv)
		if (e != nil) != tc.fail || (!tc.fail && len(got) != tc.count) {
			t.Fatalf("%s got=%v error=%v", tc.csv, got, e)
		}
	}
}

func TestInconsistentSeasonEvidenceCannotConfirmAWindow(t *testing.T) {
	from, _ := time.Parse("2006-01-02", "2025-02-01")
	to, _ := time.Parse("2006-01-02", "2025-02-07")
	r := snowjapan.Fact{"id": "a", "name": "A"}
	evidence := []snowjapan.Fact{{"resort_id": "a", "first_recorded_day": "2025-12-20", "last_recorded_day": "2026-03-01", "endpoint_evidence_state": "dates_outside_requested_winter"}}
	w, e := snowWindow(r, evidence, from, to)
	if e != nil || w["status"] != "unknown" || w["endpoint_evidence_state"] != "dates_outside_requested_winter" {
		t.Fatalf("inconsistent source window=%v error=%v", w, e)
	}
	towns := snowTownView([]snowjapan.Fact{r}, map[string][]snowjapan.Fact{"a": evidence})
	if towns[0]["inconsistent_endpoint_areas"] != 1 || towns[0]["confirmed_endpoint_areas"] != 0 {
		t.Fatal("inconsistent evidence counted as confirmed")
	}
}

func TestSnowDomainDryRunAndLocalStrategy(t *testing.T) {
	for _, args := range [][]string{{"resorts", "get"}, {"seasons", "list"}, {"reports", "get"}, {"sync"}, {"search"}, {"plan", "windows"}, {"plan", "coverage"}, {"plan", "frontier"}, {"plan", "towns"}, {"plan", "changes"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			testenv.Isolate(t)
			cmd := RootCmd()
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&bytes.Buffer{})
			cmd.SetArgs(append(args, "--dry-run", "--json", "--no-learn"))
			if e := cmd.Execute(); e != nil {
				t.Fatal(e)
			}
			var v any
			if e := json.Unmarshal(out.Bytes(), &v); e != nil {
				t.Fatalf("not JSON: %s", out.String())
			}
		})
	}
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"plan", "frontier", "--data-source", "live", "--agent", "--no-learn"})
	if e := cmd.Execute(); e == nil || !strings.Contains(e.Error(), "no live equivalent") {
		t.Fatalf("strategy error=%v", e)
	}
}

func TestSnowAgentProjectionRetainsFactsAndOneEnvelope(t *testing.T) {
	for _, source := range []string{"live", "local"} {
		cmd := &cobra.Command{}
		var out bytes.Buffer
		cmd.SetOut(&out)
		flags := &rootFlags{agent: true, asJSON: true, compact: true}
		view := map[string]any{"results": []snowjapan.Fact{{"id": "a", "vertical_m": 926.0, "source_url": "https://www.snowjapan.com/a"}}, "scanned_records": 478}
		if e := snowPrint(cmd, flags, view, source); e != nil {
			t.Fatal(e)
		}
		var result map[string]any
		if e := json.Unmarshal(out.Bytes(), &result); e != nil {
			t.Fatal(e)
		}
		rows, ok := result["results"].([]any)
		if !ok || len(rows) != 1 {
			t.Fatalf("nested envelope: %s", out.String())
		}
		if rows[0].(map[string]any)["vertical_m"] != 926.0 {
			t.Fatal("agent mode lost terrain facts")
		}
		out.Reset()
		flags.selectFields = "results.vertical_m"
		if e := snowPrint(cmd, flags, view, source); e != nil {
			t.Fatal(e)
		}
		if e := json.Unmarshal(out.Bytes(), &result); e != nil {
			t.Fatal(e)
		}
		rows, ok = result["results"].([]any)
		if !ok || len(rows[0].(map[string]any)) != 1 {
			t.Fatalf("projection was not narrow: %s", out.String())
		}
	}
}
