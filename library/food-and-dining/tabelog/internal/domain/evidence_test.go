package domain_test

import (
	"testing"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/tabelog/internal/domain"
)

func TestEvidenceStateDistinguishesUnfetchedFromSourceUnknown(t *testing.T) {
	cases := []struct {
		name     string
		snapshot domain.Restaurant
		field    string
		want     string
	}{
		{"explicit known detail", domain.Restaurant{Surface: "detail", Evidence: map[string]string{"hours": "known"}}, "hours", "known"},
		{"explicit unknown detail", domain.Restaurant{Surface: "detail", Evidence: map[string]string{"payment": "source_unknown"}}, "payment", "source_unknown"},
		{"listing has not fetched hours", domain.Restaurant{Surface: "listing"}, "hours", "detail_not_fetched"},
		{"listing has not fetched reservation", domain.Restaurant{Surface: "listing"}, "reservation", "detail_not_fetched"},
		{"detail missing hours", domain.Restaurant{Surface: "detail"}, "hours", "source_unknown"},
		{"listing missing dinner price", domain.Restaurant{Surface: "listing"}, "dinner_budget", "source_unknown"},
		{"explicit evidence overrides surface default", domain.Restaurant{Surface: "listing", Evidence: map[string]string{"hours": "known"}}, "hours", "known"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := test.snapshot.FieldState(test.field); got != test.want {
				t.Fatalf("evidence state got %s, want %s", got, test.want)
			}
		})
	}
}
