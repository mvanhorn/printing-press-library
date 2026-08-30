package cli

import (
	"math"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist/pb"
)

func TestResolveMealRecipeAndLabelByName(t *testing.T) {
	t.Parallel()

	data := &pb.PBUserDataResponse{
		RecipeDataResponse: &pb.PBRecipeDataResponse{
			Recipes: []*pb.PBRecipe{{Identifier: "recipe-1", Name: "Chicken Dinner"}},
		},
		MealPlanningCalendarResponse: &pb.PBCalendarResponse{
			CalendarId: "calendar-1",
			Labels:     []*pb.PBCalendarLabel{{Identifier: "label-1", Name: "Dinner"}},
		},
	}
	recipeID, err := resolveMealRecipeID(data, " chicken dinner ")
	if err != nil {
		t.Fatalf("resolveMealRecipeID returned error: %v", err)
	}
	if recipeID != "recipe-1" {
		t.Fatalf("recipe ID = %q, want recipe-1", recipeID)
	}
	labelID, err := resolveMealLabelID(data, "dInNeR")
	if err != nil {
		t.Fatalf("resolveMealLabelID returned error: %v", err)
	}
	if labelID != "label-1" {
		t.Fatalf("label ID = %q, want label-1", labelID)
	}
}

func TestResolveMealNamesRejectsAmbiguousMatches(t *testing.T) {
	t.Parallel()

	data := &pb.PBUserDataResponse{
		RecipeDataResponse: &pb.PBRecipeDataResponse{
			Recipes: []*pb.PBRecipe{
				{Identifier: "recipe-1", Name: "Dinner"},
				{Identifier: "recipe-2", Name: "dinner"},
			},
		},
		MealPlanningCalendarResponse: &pb.PBCalendarResponse{
			Labels: []*pb.PBCalendarLabel{
				{Identifier: "label-1", Name: "Dinner"},
				{Identifier: "label-2", Name: "dinner"},
			},
		},
	}
	if _, err := resolveMealRecipeID(data, "Dinner"); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("recipe ambiguity error = %v, want ambiguous error", err)
	}
	if _, err := resolveMealLabelID(data, "Dinner"); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("label ambiguity error = %v, want ambiguous error", err)
	}
}

func TestValidateMealEventReadBackChecksMutableFields(t *testing.T) {
	t.Parallel()

	expected := &pb.PBCalendarEvent{
		Identifier: "event-1", CalendarId: "calendar-1", Date: "2026-08-16",
		Title: "Dinner", Details: "Notes", RecipeId: "recipe-1", LabelId: "label-1",
	}
	actual := &pb.PBCalendarEvent{
		Identifier: "event-1", CalendarId: "calendar-1", Date: "2026-08-16",
		Title: "Dinner", Details: "Notes", RecipeId: "recipe-1", LabelId: "label-1",
		LogicalTimestamp: 99,
	}
	if err := validateMealEventReadBack(expected, actual); err != nil {
		t.Fatalf("matching event returned error: %v", err)
	}
	actual.Title = "Different"
	if err := validateMealEventReadBack(expected, actual); err == nil || !strings.Contains(err.Error(), "title") {
		t.Fatalf("title mismatch error = %v, want title verification error", err)
	}
}

func TestFindMealEventByIDUsesFreshCalendarPayload(t *testing.T) {
	t.Parallel()

	event := &pb.PBCalendarEvent{Identifier: "event-1", Date: "2026-08-16"}
	data := &pb.PBUserDataResponse{MealPlanningCalendarResponse: &pb.PBCalendarResponse{Events: []*pb.PBCalendarEvent{event}}}
	got, found := findMealEventByID(data, "event-1")
	if !found || got != event {
		t.Fatalf("findMealEventByID = %#v, %v; want original event, true", got, found)
	}
	if _, found := findMealEventByID(data, "missing"); found {
		t.Fatal("findMealEventByID found missing event")
	}
}

func TestValidateMealDateRejectsMalformedDates(t *testing.T) {
	t.Parallel()

	if err := validateMealDate("2026-08-16"); err != nil {
		t.Fatalf("valid date returned error: %v", err)
	}
	if err := validateMealDate("not-a-date"); err == nil {
		t.Fatal("malformed date returned nil error")
	}
}

func TestValidateScaleFactorAcceptsFinitePositive(t *testing.T) {
	t.Parallel()
	for _, v := range []float64{0.5, 1.0, 1.5, 2.0, 1e6} {
		if err := validateScaleFactor(v); err != nil {
			t.Errorf("validateScaleFactor(%v) returned error: %v", v, err)
		}
	}
}

func TestValidateScaleFactorRejectsInvalid(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		v    float64
	}{
		{"zero", 0},
		{"negative", -1.0},
		{"NaN", math.NaN()},
		{"positive infinity", math.Inf(1)},
		{"negative infinity", math.Inf(-1)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if err := validateScaleFactor(tt.v); err == nil {
				t.Errorf("validateScaleFactor(%v) returned nil error", tt.v)
			}
		})
	}
}

func TestValidateScaleFactorReadBack(t *testing.T) {
	t.Parallel()
	expected := &pb.PBCalendarEvent{Identifier: "event-1", CalendarId: "calendar-1", Date: "2026-08-16", Title: "Dinner", RecipeScaleFactor: 1.5}
	actual := &pb.PBCalendarEvent{Identifier: "event-1", CalendarId: "calendar-1", Date: "2026-08-16", Title: "Dinner", RecipeScaleFactor: 1.5}
	if err := validateMealEventReadBack(expected, actual); err != nil {
		t.Fatalf("matching scale factor returned error: %v", err)
	}
	actual.RecipeScaleFactor = 2
	if err := validateMealEventReadBack(expected, actual); err == nil {
		t.Fatal("scale factor mismatch did not return error")
	}
}

func TestValidateMealEventReadBackAllowsOmittedScaleFactor(t *testing.T) {
	t.Parallel()
	expected := &pb.PBCalendarEvent{Identifier: "event-1", CalendarId: "calendar-1", Date: "2026-08-16", Title: "Dinner"}
	actual := &pb.PBCalendarEvent{Identifier: "event-1", CalendarId: "calendar-1", Date: "2026-08-16", Title: "Dinner", RecipeScaleFactor: 1}
	if err := validateMealEventReadBack(expected, actual); err != nil {
		t.Fatalf("omitted expected scale factor should allow existing value: %v", err)
	}
}
