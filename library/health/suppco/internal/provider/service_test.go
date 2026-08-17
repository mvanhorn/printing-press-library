package provider

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/health/suppco/internal/client"
)

type fakeReader struct {
	responses map[string]json.RawMessage
	errors    map[string]error
	calls     []string
}

func (f *fakeReader) Get(_ context.Context, path string, _ map[string]string) (json.RawMessage, error) {
	f.calls = append(f.calls, path)
	if err := f.errors[path]; err != nil {
		return nil, err
	}
	return f.responses[path], nil
}

func fixture(t *testing.T, name string) json.RawMessage {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func sequenceClock(values ...string) func() time.Time {
	index := 0
	return func() time.Time {
		value := values[index]
		index++
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			panic(err)
		}
		return parsed
	}
}

func TestSnapshotUsesTwoBoundedReadsAndInternalObservationTimes(t *testing.T) {
	reader := &fakeReader{responses: map[string]json.RawMessage{
		compactPath:                 fixture(t, "me_compact.json"),
		"/api/schedules/2026-07-19": fixture(t, "schedule.json"),
	}}
	service := NewWithClock(reader, sequenceClock(
		"2026-07-19T12:00:01Z",
		"2026-07-19T12:00:02Z",
		"2026-07-19T12:00:03Z",
	))

	snapshot, err := service.Snapshot(context.Background(), "2026-07-19")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(reader.calls, []string{compactPath, "/api/schedules/2026-07-19"}) {
		t.Fatalf("calls = %#v", reader.calls)
	}
	if snapshot.Provenance.Stack.ObservedAt != "2026-07-19T12:00:01Z" || snapshot.Provenance.Schedule.ObservedAt != "2026-07-19T12:00:02Z" || snapshot.AsOf != "2026-07-19T12:00:03Z" {
		t.Fatalf("observation times = %#v, as_of = %q", snapshot.Provenance, snapshot.AsOf)
	}
	if snapshot.UserOverride != nil || snapshot.EffectiveSource != "provider_schedule" {
		t.Fatal("snapshot crossed the Trainer Core override boundary")
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "discard-me") || strings.Contains(string(encoded), "unrelated") {
		t.Fatalf("snapshot retained unprojected fields: %s", encoded)
	}
}

func TestProductsProjectsMinimumFieldsAndStableOrder(t *testing.T) {
	reader := &fakeReader{responses: map[string]json.RawMessage{compactPath: fixture(t, "me_compact.json")}}
	service := NewWithClock(reader, sequenceClock("2026-07-19T12:00:01Z"))

	projection, err := service.Products(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := []string{projection.Data[0].ID, projection.Data[1].ID}; !reflect.DeepEqual(got, []string{"product-a", "product-b"}) {
		t.Fatalf("product order = %#v", got)
	}
	if projection.Provenance.Path != compactPath || projection.Provenance.Provider != "suppco" {
		t.Fatalf("provenance = %#v", projection.Provenance)
	}
}

func TestNutrientsPreservesParentAndChildRelationships(t *testing.T) {
	reader := &fakeReader{responses: map[string]json.RawMessage{compactPath: fixture(t, "me_compact.json")}}
	service := NewWithClock(reader, sequenceClock("2026-07-19T12:00:01Z"))

	projection, err := service.Nutrients(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if projection.Data[0].ParentID != "component-parent" || !reflect.DeepEqual(projection.Data[1].ComponentIDs, []string{"component-child"}) {
		t.Fatalf("relationships = %#v", projection.Data)
	}
	if projection.Data[1].Amount != 10 || projection.Data[0].Amount != 4 {
		t.Fatal("provider rows were aggregated or altered")
	}
}

func TestScheduleRejectsInvalidDateBeforeReading(t *testing.T) {
	reader := &fakeReader{}
	service := New(reader)
	if _, err := service.Schedule(context.Background(), "07/19/2026"); err == nil || !strings.Contains(err.Error(), "YYYY-MM-DD") {
		t.Fatalf("date error = %v", err)
	}
	if len(reader.calls) != 0 {
		t.Fatalf("invalid date triggered reads: %#v", reader.calls)
	}
}

func TestScheduleRejectsMissingReaderWithoutPanic(t *testing.T) {
	if _, err := New(nil).Schedule(context.Background(), "2026-07-19"); err == nil || !strings.Contains(err.Error(), "reader is required") {
		t.Fatalf("missing reader error = %v", err)
	}
}

func TestScheduleProjectsActivitiesAndStableOrder(t *testing.T) {
	reader := &fakeReader{responses: map[string]json.RawMessage{"/api/schedules/2026-07-19": fixture(t, "schedule.json")}}
	projection, err := NewWithClock(reader, sequenceClock("2026-07-19T12:00:01Z")).Schedule(context.Background(), "2026-07-19")
	if err != nil {
		t.Fatal(err)
	}
	if got := []string{projection.Data.Activities[0].Activity, projection.Data.Activities[1].Activity}; !reflect.DeepEqual(got, []string{"evening", "morning"}) {
		t.Fatalf("activity order = %#v", got)
	}
	encoded, err := json.Marshal(projection)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "product_image_url") || strings.Contains(string(encoded), "discard-me") {
		t.Fatalf("schedule retained unprojected fields: %s", encoded)
	}
}

func TestScheduleRejectsMissingProductBrand(t *testing.T) {
	reader := &fakeReader{responses: map[string]json.RawMessage{
		"/api/schedules/2026-07-19": json.RawMessage(`{"date":"2026-07-19","activities":[{"activity":"morning","products":[{"product_id":"p","product_name":"Synthetic Product","serving_size_amount":1,"serving_size_amount_taken":1,"serving_unit":"unit","schedule_type":"daily","schedule_days":[],"as_needed":false,"scheduled_not_today":false,"with_food":false}],"reminder":{"enabled":true}}]}`),
	}}
	if _, err := New(reader).Schedule(context.Background(), "2026-07-19"); err == nil || !strings.Contains(err.Error(), "missing required fields") {
		t.Fatalf("missing brand error = %v", err)
	}
}

func TestEmptyProductIngredientsProduceAnEmptyNutrientProjection(t *testing.T) {
	reader := &fakeReader{responses: map[string]json.RawMessage{compactPath: json.RawMessage(`{"products":[{"id":"p","name":"P","ingredients":[]}]}`)}}
	projection, err := NewWithClock(reader, sequenceClock("2026-07-19T12:00:01Z")).Nutrients(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if projection.Data == nil || len(projection.Data) != 0 {
		t.Fatalf("empty nutrient projection = %#v", projection.Data)
	}
}

func TestSnapshotFailsClosedWhenEitherReadFails(t *testing.T) {
	providerErr := errors.New("synthetic provider failure")
	reader := &fakeReader{
		responses: map[string]json.RawMessage{compactPath: fixture(t, "me_compact.json")},
		errors:    map[string]error{"/api/schedules/2026-07-19": providerErr},
	}
	service := NewWithClock(reader, sequenceClock("2026-07-19T12:00:01Z"))
	if _, err := service.Snapshot(context.Background(), "2026-07-19"); !errors.Is(err, providerErr) {
		t.Fatalf("snapshot error = %v", err)
	}
}

func TestReaderErrorsDoNotExposeProviderControlledText(t *testing.T) {
	const privateSentinel = "private-provider-body-sentinel"
	reader := &fakeReader{errors: map[string]error{
		compactPath: &client.APIError{
			Method: "GET", Path: compactPath, StatusCode: 401, Body: privateSentinel,
		},
	}}
	_, err := New(reader).Products(context.Background())
	if err == nil {
		t.Fatal("expected provider error")
	}
	if strings.Contains(err.Error(), privateSentinel) {
		t.Fatalf("provider response text escaped into error: %v", err)
	}
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 401 || apiErr.Body != "" {
		t.Fatalf("sanitized typed error = %#v (%v)", apiErr, err)
	}
}

func TestNonHTTPReaderErrorsRemainClassifiableButDisplaySafely(t *testing.T) {
	privateErr := errors.New("private-network-sentinel")
	reader := &fakeReader{errors: map[string]error{compactPath: privateErr}}
	_, err := New(reader).Products(context.Background())
	if !errors.Is(err, privateErr) {
		t.Fatalf("reader cause not preserved: %v", err)
	}
	if strings.Contains(err.Error(), privateErr.Error()) {
		t.Fatalf("reader cause escaped into display text: %v", err)
	}
}

func TestMalformedPartialAndDuplicatePayloadsFail(t *testing.T) {
	cases := []struct {
		name    string
		payload string
		call    func(*Service) error
	}{
		{"missing products", `{}`, func(s *Service) error { _, err := s.Products(context.Background()); return err }},
		{"products not array", `{"products":{}}`, func(s *Service) error { _, err := s.Products(context.Background()); return err }},
		{"duplicate products", `{"products":[{"id":"p","name":"P"},{"id":"p","name":"P"}]}`, func(s *Service) error { _, err := s.Products(context.Background()); return err }},
		{"missing ingredients", `{"products":[{"id":"p","name":"P"}]}`, func(s *Service) error { _, err := s.Nutrients(context.Background()); return err }},
		{"ingredients not array", `{"products":[{"id":"p","name":"P","ingredients":{}}]}`, func(s *Service) error { _, err := s.Nutrients(context.Background()); return err }},
		{"partial ingredient", `{"products":[{"id":"p","name":"P","ingredients":[{"id":"n","name":"N","amount":1,"units":"mg"}]}]}`, func(s *Service) error { _, err := s.Nutrients(context.Background()); return err }},
		{"duplicate ingredients", `{"products":[{"id":"p","name":"P","ingredients":[{"id":"n","name":"N","amount":1,"units":"mg","ancestry":null},{"id":"n","name":"N","amount":1,"units":"mg","ancestry":null}]}]}`, func(s *Service) error { _, err := s.Nutrients(context.Background()); return err }},
		{"missing parent", `{"products":[{"id":"p","name":"P","ingredients":[{"id":"n","name":"N","amount":1,"units":"mg","ancestry":"missing"}]}]}`, func(s *Service) error { _, err := s.Nutrients(context.Background()); return err }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reader := &fakeReader{responses: map[string]json.RawMessage{compactPath: json.RawMessage(tc.payload)}}
			service := NewWithClock(reader, sequenceClock("2026-07-19T12:00:01Z"))
			if err := tc.call(service); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestScheduleRejectsMismatchedOrPartialPayload(t *testing.T) {
	for _, payload := range []string{
		`{"date":"2026-07-18","activities":[]}`,
		`{"date":"2026-07-19"}`,
		`{"activities":[]}`,
		`{"date":"2026-07-19","activities":{}}`,
		`{"date":"2026-07-19","activities":[{"activity":"morning","products":[]}]}`,
		`{"date":"2026-07-19","activities":[{"activity":"morning","products":[{"product_id":"p"}],"reminder":{"enabled":false}}]}`,
	} {
		reader := &fakeReader{responses: map[string]json.RawMessage{"/api/schedules/2026-07-19": json.RawMessage(payload)}}
		service := NewWithClock(reader, sequenceClock("2026-07-19T12:00:01Z"))
		if _, err := service.Schedule(context.Background(), "2026-07-19"); err == nil {
			t.Fatalf("payload unexpectedly accepted: %s", payload)
		}
	}
}
