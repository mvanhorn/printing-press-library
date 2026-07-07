// internal/render/render_test.go
package render

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/other/running-race-results/internal/domain"
)

func sample() domain.Result {
	return domain.Result{Provider: "mika", RaceName: "BMW Berlin Marathon", Year: 2025,
		Runner: "Sample Runner", Bib: "1234", NetTime: "02:45:10", OverallPlace: 512}
}

func TestTableContainsCoreFields(t *testing.T) {
	var b bytes.Buffer
	if err := Table(&b, sample()); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	for _, want := range []string{"Sample Runner", "1234", "02:45:10", "Berlin"} {
		if !strings.Contains(out, want) {
			t.Fatalf("table missing %q in:\n%s", want, out)
		}
	}
}

func TestJSONRoundTrips(t *testing.T) {
	var b bytes.Buffer
	if err := JSON(&b, sample()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), `"Runner": "Sample Runner"`) {
		t.Fatalf("json missing runner:\n%s", b.String())
	}
}

func TestTablePlaces(t *testing.T) {
	r := domain.Result{Provider: "athlinks", RaceName: "R", Year: 2024, Runner: "A B", Bib: "7",
		OverallPlace: 3, GenderPlace: 2, AgeGroup: "M30-39", AgeGroupPlace: 1}
	var b bytes.Buffer
	if err := Table(&b, r); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	for _, want := range []string{"Gender place", "Age group", "M30-39", "Age group place"} {
		if !strings.Contains(out, want) {
			t.Fatalf("table missing %q in:\n%s", want, out)
		}
	}
}

func TestTableOmitsZeroPlaces(t *testing.T) {
	r := domain.Result{Provider: "x", RaceName: "R", Year: 2024, Runner: "A B", Bib: "9"}
	var b bytes.Buffer
	if err := Table(&b, r); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(b.String(), "Gender place") {
		t.Fatalf("table showed a zero gender place:\n%s", b.String())
	}
}

func TestListAndAthletes(t *testing.T) {
	rows := []domain.Result{
		{RaceName: "Berlin", Date: "2024-09-29", NetTime: "02:45:10", OverallPlace: 512},
		{RaceName: "Boston", Date: "2025-04-21", NetTime: "02:50:00", OverallPlace: 700},
	}
	cols := []Column{
		{"Date", func(r domain.Result) string { return r.Date }},
		{"Race", func(r domain.Result) string { return r.RaceName }},
		{"Net time", func(r domain.Result) string { return r.NetTime }},
	}
	var b bytes.Buffer
	if err := List(&b, rows, cols); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	for _, want := range []string{"Date", "Berlin", "Boston", "02:45:10"} {
		if !strings.Contains(out, want) {
			t.Fatalf("list missing %q:\n%s", want, out)
		}
	}

	var b2 bytes.Buffer
	if err := Athletes(&b2, []domain.Athlete{{Name: "Sample Athlete", City: "amadora", Age: 41, ID: "338681853"}}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b2.String(), "Sample Athlete") || !strings.Contains(b2.String(), "338681853") {
		t.Fatalf("athletes missing fields:\n%s", b2.String())
	}
}

func TestJSONValueArray(t *testing.T) {
	var b bytes.Buffer
	if err := JSONValue(&b, []domain.Result{{Runner: "A"}}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), `"Runner": "A"`) || b.String()[0] != '[' {
		t.Fatalf("expected JSON array:\n%s", b.String())
	}
}

func TestTableOmitsZeroYear(t *testing.T) {
	r := domain.Result{Provider: "x", RaceName: "Some Race", Runner: "A B", Bib: "9"}
	var b bytes.Buffer
	if err := Table(&b, r); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(b.String(), "Some Race 0") {
		t.Fatalf("table faked a zero year:\n%s", b.String())
	}
	if !strings.Contains(b.String(), "Some Race") {
		t.Fatalf("table missing race name:\n%s", b.String())
	}
}
