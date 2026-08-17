package regimen

import (
	"encoding/json"
	"strings"
	"testing"
)

func syntheticObservations() (Observation, Observation) {
	return Observation{Provider: "suppco", Path: "/api/users/me_compact/", ObservedAt: "2026-07-19T12:00:01Z"},
		Observation{Provider: "suppco", Path: "/api/schedules/2026-07-19", ObservedAt: "2026-07-19T12:00:02Z"}
}

func syntheticProviderSchedule() ProviderSchedule {
	return ProviderSchedule{Date: "2026-07-19", Activities: []ScheduleActivity{
		{
			Activity: "morning",
			Products: []ScheduledProduct{{
				ProductID: "p1", ProductName: "Synthetic One", ProductBrand: "Synthetic Brand",
				ServingSizeAmount: 1, ServingSizeAmountTaken: 1, ServingUnit: "unit",
				ScheduleType: "daily", ScheduleDays: []string{},
			}},
			Reminder: ScheduleReminder{Enabled: true},
		},
	}}
}

func TestBuildIsDeterministicAndPreservesOverlap(t *testing.T) {
	products := []Product{{ID: "p2", Name: "Synthetic Two"}, {ID: "p1", Name: "Synthetic One"}}
	nutrients := []Nutrient{
		{ID: "child-b", ProductID: "p1", Name: "Child B", ParentID: "parent", Unit: "mg"},
		{ID: "parent", ProductID: "p1", Name: "Parent", Amount: 10, Unit: "mg"},
		{ID: "child-a", ProductID: "p1", Name: "Child A", ParentID: "parent", Amount: 4, Unit: "mg"},
	}
	stackObs, scheduleObs := syntheticObservations()
	schedule := syntheticProviderSchedule()
	first, err := Build("2026-07-19T12:00:03Z", stackObs, scheduleObs, products, nutrients, schedule)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Build("2026-07-19T12:00:03Z", stackObs, scheduleObs, []Product{{ID: "p1", Name: "Synthetic One"}, {ID: "p2", Name: "Synthetic Two"}}, nutrients, schedule)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(first)
	b, _ := json.Marshal(second)
	if string(a) != string(b) {
		t.Fatalf("snapshot is not deterministic:\n%s\n%s", a, b)
	}
	if first.EffectiveSource != "provider_schedule" || first.UserOverride != nil || len(first.Components) != 3 {
		t.Fatal("provider schedule semantics or component rows lost")
	}
	if got := strings.Join(first.Components[2].ComponentIDs, ","); got != "child-a,child-b" {
		t.Fatalf("children = %q", got)
	}
	if first.EffectiveRegimen.Date != first.ProviderSchedule.Date {
		t.Fatal("effective regimen must be the provider schedule without inference")
	}
}

func TestBuildRejectsMalformedDuplicateAndDanglingInputs(t *testing.T) {
	cases := []struct {
		name      string
		products  []Product
		nutrients []Nutrient
	}{
		{"duplicate product", []Product{{ID: "p", Name: "P"}, {ID: "p", Name: "P"}}, nil},
		{"duplicate nutrient", []Product{{ID: "p", Name: "P"}}, []Nutrient{{ID: "n", ProductID: "p", Name: "N", Unit: "mg"}, {ID: "n", ProductID: "p", Name: "N", Unit: "mg"}}},
		{"dangling", []Product{{ID: "p", Name: "P"}}, []Nutrient{{ID: "n", ProductID: "p", Name: "N", Unit: "mg", ParentID: "missing"}}},
		{"unknown product", []Product{{ID: "p", Name: "P"}}, []Nutrient{{ID: "n", ProductID: "missing", Name: "N", Unit: "mg"}}},
		{"partial", []Product{{ID: "p"}}, nil},
		{"cross product", []Product{{ID: "p1", Name: "P1"}, {ID: "p2", Name: "P2"}}, []Nutrient{{ID: "parent", ProductID: "p1", Name: "Parent", Unit: "mg"}, {ID: "child", ProductID: "p2", Name: "Child", Unit: "mg", ParentID: "parent"}}},
		{"cycle", []Product{{ID: "p", Name: "P"}}, []Nutrient{{ID: "a", ProductID: "p", Name: "A", Unit: "mg", ParentID: "b"}, {ID: "b", ProductID: "p", Name: "B", Unit: "mg", ParentID: "a"}}},
	}
	stackObs, scheduleObs := syntheticObservations()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Build("2026-07-19T12:00:03Z", stackObs, scheduleObs, tc.products, tc.nutrients, ProviderSchedule{Date: "2026-07-19", Activities: []ScheduleActivity{}}); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestBuildRequiresCompleteProvenanceAndSchedule(t *testing.T) {
	stackObs, scheduleObs := syntheticObservations()
	validProducts := []Product{{ID: "p", Name: "P"}}
	validSchedule := ProviderSchedule{Date: "2026-07-19", Activities: []ScheduleActivity{}}
	for _, tc := range []struct {
		name        string
		asOf        string
		stack       Observation
		scheduleObs Observation
		schedule    ProviderSchedule
	}{
		{"missing as of", "", stackObs, scheduleObs, validSchedule},
		{"wrong provider", "2026-07-19T12:00:03Z", Observation{Provider: "other", Path: stackObs.Path, ObservedAt: stackObs.ObservedAt}, scheduleObs, validSchedule},
		{"missing schedule activities", "2026-07-19T12:00:03Z", stackObs, scheduleObs, ProviderSchedule{Date: "2026-07-19"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Build(tc.asOf, tc.stack, tc.scheduleObs, validProducts, nil, tc.schedule); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestNormalizeProviderScheduleUsesStableKeysAndPreservesDayOrder(t *testing.T) {
	schedule := ProviderSchedule{Date: "2026-07-19", Activities: []ScheduleActivity{
		{Activity: "morning", Products: []ScheduledProduct{
			{ProductID: "p2", ProductName: "Synthetic Two", ServingUnit: "unit", ScheduleType: "selected_days", ScheduleDays: []string{"wednesday", "monday"}},
			{ProductID: "p1", ProductName: "Synthetic One", ServingUnit: "unit", ScheduleType: "daily", ScheduleDays: []string{}},
		}},
		{Activity: "evening", Products: []ScheduledProduct{}},
	}}
	normalized, err := NormalizeProviderSchedule(schedule)
	if err != nil {
		t.Fatal(err)
	}
	if got := []string{normalized.Activities[0].Activity, normalized.Activities[1].Activity}; strings.Join(got, ",") != "evening,morning" {
		t.Fatalf("activity order = %#v", got)
	}
	if got := []string{normalized.Activities[1].Products[0].ProductID, normalized.Activities[1].Products[1].ProductID}; strings.Join(got, ",") != "p1,p2" {
		t.Fatalf("product order = %#v", got)
	}
	if got := strings.Join(normalized.Activities[1].Products[1].ScheduleDays, ","); got != "wednesday,monday" {
		t.Fatalf("provider schedule_days order changed: %q", got)
	}
}

func TestNormalizeProviderScheduleRejectsPartialAndDuplicateRows(t *testing.T) {
	validProduct := ScheduledProduct{ProductID: "p", ProductName: "P", ServingUnit: "unit", ScheduleType: "daily", ScheduleDays: []string{}}
	for _, tc := range []struct {
		name     string
		schedule ProviderSchedule
	}{
		{"missing activities", ProviderSchedule{Date: "2026-07-19"}},
		{"duplicate activities", ProviderSchedule{Date: "2026-07-19", Activities: []ScheduleActivity{{Activity: "morning", Products: []ScheduledProduct{}}, {Activity: "morning", Products: []ScheduledProduct{}}}}},
		{"missing activity products", ProviderSchedule{Date: "2026-07-19", Activities: []ScheduleActivity{{Activity: "morning"}}}},
		{"partial product", ProviderSchedule{Date: "2026-07-19", Activities: []ScheduleActivity{{Activity: "morning", Products: []ScheduledProduct{{ProductID: "p"}}}}}},
		{"duplicate product", ProviderSchedule{Date: "2026-07-19", Activities: []ScheduleActivity{{Activity: "morning", Products: []ScheduledProduct{validProduct, validProduct}}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NormalizeProviderSchedule(tc.schedule); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestNormalizeStackEncodesEmptyProductsAsArray(t *testing.T) {
	products, components, err := NormalizeStack([]Product{}, []Nutrient{})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(struct {
		Products   []Product   `json:"products"`
		Components []Component `json:"components"`
	}{Products: products, Components: components})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(encoded), `{"products":[],"components":[]}`; got != want {
		t.Fatalf("empty stack JSON = %s, want %s", got, want)
	}
}

func TestNormalizeStackRejectsBlankNutrientUnit(t *testing.T) {
	products := []Product{{ID: "p", Name: "Synthetic Product"}}
	for _, unit := range []string{"", "   "} {
		if _, _, err := NormalizeStack(products, []Nutrient{{ID: "n", ProductID: "p", Name: "Synthetic Nutrient", Unit: unit}}); err == nil || !strings.Contains(err.Error(), "unit") {
			t.Fatalf("unit %q error = %v", unit, err)
		}
	}
}

func TestNormalizeStackEncodesLeafComponentIDsAsArray(t *testing.T) {
	products, components, err := NormalizeStack(
		[]Product{{ID: "p", Name: "Synthetic Product"}},
		[]Nutrient{{ID: "leaf", ProductID: "p", Name: "Synthetic Leaf", Unit: "mg"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(struct {
		Products   []Product   `json:"products"`
		Components []Component `json:"components"`
	}{Products: products, Components: components})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"component_ids":[]`) {
		t.Fatalf("leaf component_ids is not an array: %s", encoded)
	}
}

func TestBuildKeepsEffectiveScheduleIndependentFromProviderFact(t *testing.T) {
	schedule := syntheticProviderSchedule()
	note := "synthetic note"
	schedule.Activities[0].Products[0].ScheduleDays = []string{"monday"}
	schedule.Activities[0].Products[0].ScheduleNotes = &note
	stackObservation, scheduleObservation := syntheticObservations()
	snapshot, err := Build("2026-07-19T12:00:03Z", stackObservation, scheduleObservation, []Product{{ID: "p1", Name: "Synthetic One"}}, nil, schedule)
	if err != nil {
		t.Fatal(err)
	}
	snapshot.EffectiveRegimen.Activities[0].Products[0].ScheduleDays[0] = "changed"
	*snapshot.EffectiveRegimen.Activities[0].Products[0].ScheduleNotes = "changed"
	if snapshot.ProviderSchedule.Activities[0].Products[0].ScheduleDays[0] == "changed" || *snapshot.ProviderSchedule.Activities[0].Products[0].ScheduleNotes == "changed" {
		t.Fatal("effective regimen mutation changed provider_schedule")
	}
}
