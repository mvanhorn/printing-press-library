// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/mvanhorn/printing-press-library/library/travel/iko-yo/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/travel/iko-yo/internal/store"
	"github.com/mvanhorn/printing-press-library/library/travel/iko-yo/internal/trip"
	"os"
	"strings"
	"testing"
	"time"
)

func tripRun(t *testing.T, args ...string) ([]byte, error) {
	t.Helper()
	cmd := RootCmd()
	var out, errout bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errout)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.Bytes(), err
}
func TestTripCommandsDryRunBeforeValidation(t *testing.T) {
	testenv.Isolate(t)
	for _, name := range []string{"discover", "inspect", "compare", "cached"} {
		t.Run(name, func(t *testing.T) {
			b, e := tripRun(t, "trip", name, "--dry-run", "--json")
			if e != nil || !json.Valid(b) {
				t.Fatalf("%s: %s %v", name, b, e)
			}
		})
	}
}
func TestTripInputsFailBeforeIO(t *testing.T) {
	testenv.Isolate(t)
	for _, args := range [][]string{
		{"discover", "--max-pages", "6"}, {"discover", "--from", "2026-02-30"}, {"discover", "--data-source", "local"}, {"discover", "--prefecture", "11"}, {"discover", "--max-pages", "1", "--max-scan-pages", "2"},
		{"inspect"}, {"inspect", "spots/8220", "--ref", "events/8412"}, {"inspect", "https://iko-yo.net/spots/1"},
		{"compare", "spots/8220", "spots/8220"}, {"compare", "spots/8220", "--as-of", "2026-02-30"}, {"compare", "spots/8220", "--amenities", "unicorn"},
		{"cached", "--data-source", "live"}, {"cached", "--max-scan-records", "1001"},
	} {
		_, e := tripRun(t, append([]string{"trip"}, args...)...)
		if e == nil {
			t.Errorf("accepted %v", args)
		}
	}
}
func TestTripOfflineSavedFactsAndEmpty(t *testing.T) {
	testenv.Isolate(t)
	b, e := tripRun(t, "trip", "cached", "Mooovi", "--json")
	var empty trip.Discovery
	if e != nil || json.Unmarshal(b, &empty) != nil || empty.Records == nil || empty.ScannedRecords != 0 || !strings.Contains(empty.Note, "source-wide") {
		t.Fatalf("empty local: %s %v", b, e)
	}
	db, e := store.OpenWithContext(context.Background(), defaultDBPath("iko-yo-pp-cli"))
	if e != nil {
		t.Fatal(e)
	}
	raw, e := os.ReadFile("../trip/testdata/spots-8220.html")
	if e != nil {
		t.Fatal(e)
	}
	r, e := trip.ParseDetail(raw, "spots/8220", time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC))
	if e != nil {
		t.Fatal(e)
	}
	if e = db.SaveTripRecords(context.Background(), []trip.Record{r}); e != nil {
		t.Fatal(e)
	}
	db.Close()
	b, e = tripRun(t, "trip", "inspect", "spots/8220", "--data-source", "local", "--json", "--select", "ref,data_source,observed_at")
	var selected map[string]any
	if e != nil || json.Unmarshal(b, &selected) != nil || len(selected) != 3 || selected["data_source"] != "local" || selected["observed_at"] != r.ObservedAt {
		t.Fatalf("local select %s %v", b, e)
	}
	b, e = tripRun(t, "trip", "compare", "spots/8220", "--data-source", "local", "--age-months", "24", "--amenities", "nursing,changing", "--json")
	var view tripComparison
	if e != nil || json.Unmarshal(b, &view) != nil || view.ComparedRecords != 1 || view.Records[0].Overall != "supported" {
		t.Fatalf("compare: %s %v", b, e)
	}
	b, e = tripRun(t, "trip", "compare", "spots/8220", "events/999999999", "--data-source", "local", "--json")
	if e != nil || json.Unmarshal(b, &view) != nil || view.ComparedRecords != 1 || len(view.FetchFailures) != 1 {
		t.Fatalf("partial %s %v", b, e)
	}
	for _, args := range [][]string{{"trip", "inspect", "spots/8220", "--data-source", "local", "--quiet"}, {"trip", "cached", "Mooovi", "--kind", "spots", "--quiet"}, {"trip", "compare", "spots/8220", "--data-source", "local", "--quiet"}} {
		quiet, err := tripRun(t, args...)
		if err != nil || strings.TrimSpace(string(quiet)) != "spots/8220" {
			t.Fatalf("quiet identities %s %v", quiet, err)
		}
	}
	for _, args := range [][]string{{"trip", "inspect", "spots/8220", "--data-source", "local", "--csv"}, {"trip", "cached", "Mooovi", "--kind", "spots", "--csv"}, {"trip", "compare", "spots/8220", "--data-source", "local", "--csv"}} {
		data, err := tripRun(t, args...)
		if err != nil || !strings.Contains(string(data), "ref") || !strings.Contains(string(data), "spots/8220") {
			t.Fatalf("CSV rows %s %v", data, err)
		}
	}

	b, e = tripRun(t, "trip", "cached", "unicorn-never-match", "--json")
	if e != nil || json.Unmarshal(b, &empty) != nil || len(empty.Records) != 0 || empty.ScannedRecords != 1 {
		t.Fatalf("negative %s %v", b, e)
	}
	b, e = tripRun(t, "trip", "cached", "--amenities", "stroller", "--json")
	if e != nil || json.Unmarshal(b, &empty) != nil || len(empty.Records) != 0 || empty.UnknownRequirementRecords != 1 {
		t.Fatalf("unknown %s %v", b, e)
	}
}

func TestTripProfileCannotEnableAutomaticLearning(t *testing.T) {
	testenv.Isolate(t)
	if _, e := tripRun(t, "profile", "save", "private-trip-policy", "--json", "--no-learn=false"); e != nil {
		t.Fatal(e)
	}
	for _, args := range [][]string{{"trip", "cached", "--profile", "private-trip-policy", "--age-months", "24", "--json"}, {"trip", "cached", "--no-learn=false", "--json"}} {
		cmd := RootCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs(args)
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		suppressed, e := cmd.PersistentFlags().GetBool("no-learn")
		if e != nil || !suppressed {
			t.Errorf("Trip learning suppression was overridden by %v", args)
		}
	}
}
