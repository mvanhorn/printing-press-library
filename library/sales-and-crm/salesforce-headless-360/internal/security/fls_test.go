package security

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

type fakeGetter struct {
	responses map[string]json.RawMessage
	errs      map[string]error
	calls     []string
}

func (f *fakeGetter) GetWithResponseHeaders(path string, params map[string]string) (json.RawMessage, http.Header, error) {
	key := path
	if params != nil && params["fields"] != "" {
		key += "?fields=" + params["fields"]
	}
	f.calls = append(f.calls, key)
	if err := f.errs[path]; err != nil {
		return nil, nil, err
	}
	if body, ok := f.responses[key]; ok {
		return body, nil, nil
	}
	if body, ok := f.responses[path]; ok {
		return body, nil, nil
	}
	return nil, nil, fmt.Errorf("missing fake response for %s", key)
}

type unknownRecorder struct {
	events []string
}

func (r *unknownRecorder) RecordUnknownFieldSeen(sobject, field string) error {
	r.events = append(r.events, sobject+"."+field)
	return nil
}

func TestFLSRemovesFieldMissingFromUIAPI(t *testing.T) {
	client := &fakeGetter{responses: map[string]json.RawMessage{
		"/services/data/v63.0/sobjects/Contact/describe": json.RawMessage(`{"fields":[{"name":"Id"},{"name":"Name"},{"name":"Salary__c"}]}`),
		"/services/data/v63.0/ui-api/records/003HIDDEN":  json.RawMessage(`{"fields":{"Id":{"value":"003HIDDEN"},"Name":{"value":"Avery Morgan"}}}`),
	}}
	record := &Record{SObject: "Contact", Fields: map[string]any{
		"Id": "003HIDDEN", "Name": "Avery Morgan", "Salary__c": 142000,
	}, Provenance: &Provenance{Redactions: map[string]int{}}}

	got := NewFLSFilter(client).Apply(context.Background(), record)
	if _, ok := got.Fields["Salary__c"]; ok {
		t.Fatalf("Salary__c leaked through FLS filter: %#v", got.Fields)
	}
	if got.Fields["Name"] != "Avery Morgan" {
		t.Fatalf("Name was removed unexpectedly: %#v", got.Fields)
	}
}

func TestFLSUnknownUIFieldInvalidatesCacheAndRecordsEvent(t *testing.T) {
	recorder := &unknownRecorder{}
	client := &fakeGetter{responses: map[string]json.RawMessage{
		"/services/data/v63.0/sobjects/Contact/describe": json.RawMessage(`{"fields":[{"name":"Id"},{"name":"Name"}]}`),
		"/services/data/v63.0/ui-api/records/003":        json.RawMessage(`{"fields":{"Id":{"value":"003"},"Name":{"value":"Avery"},"New__c":{"value":"x"}}}`),
	}}
	filter := NewFLSFilter(client)
	filter.UnknownFieldRecorder = recorder
	record := &Record{SObject: "Contact", Fields: map[string]any{
		"Id": "003", "Name": "Avery", "New__c": "x",
	}, Provenance: &Provenance{Redactions: map[string]int{}}}

	got := filter.Apply(context.Background(), record)
	if _, ok := got.Fields["New__c"]; ok {
		t.Fatalf("unknown field survived describe intersection: %#v", got.Fields)
	}
	if len(recorder.events) != 1 || recorder.events[0] != "Contact.New__c" {
		t.Fatalf("unknown events = %#v, want Contact.New__c", recorder.events)
	}
}

func TestFLSRequestIncludesQualifiedFields(t *testing.T) {
	client := &fakeGetter{responses: map[string]json.RawMessage{
		"/services/data/v63.0/sobjects/Contact/describe": json.RawMessage(`{"fields":[{"name":"Id"},{"name":"Name"}]}`),
		"/services/data/v63.0/ui-api/records/003":        json.RawMessage(`{"fields":{"Id":{"value":"003"},"Name":{"value":"Avery"}}}`),
	}}
	record := &Record{SObject: "Contact", Fields: map[string]any{"Id": "003", "Name": "Avery"}}
	NewFLSFilter(client).Apply(context.Background(), record)
	joined := strings.Join(client.calls, "\n")
	if !strings.Contains(joined, "Contact.Id") || !strings.Contains(joined, "Contact.Name") {
		t.Fatalf("UI API fields were not qualified: %s", joined)
	}
}

func TestFLSAllowFieldWriteRequiresObjectEditAndFieldEdit(t *testing.T) {
	client := &fakeGetter{responses: map[string]json.RawMessage{
		"/services/data/v63.0/sobjects/Contact/describe": json.RawMessage(`{
			"updateable": true,
			"fields": [
				{"name":"Name","updateable":true},
				{"name":"Salary__c","updateable":false}
			]
		}`),
	}}
	filter := NewFLSFilter(client)

	if !filter.AllowFieldWrite("005USER", "Contact", "Name") {
		t.Fatal("Name should be writeable")
	}
	if filter.AllowFieldWrite("005USER", "Contact", "Salary__c") {
		t.Fatal("Salary__c should be rejected when visible but not editable")
	}
}

func TestFLSAllowFieldWriteRequiresObjectEdit(t *testing.T) {
	client := &fakeGetter{responses: map[string]json.RawMessage{
		"/services/data/v63.0/sobjects/Contact/describe": json.RawMessage(`{
			"updateable": false,
			"fields": [
				{"name":"Name","updateable":true}
			]
		}`),
	}}

	if NewFLSFilter(client).AllowFieldWrite("005USER", "Contact", "Name") {
		t.Fatal("Name should be rejected when object CRUD edit is denied")
	}
}
