package security

import (
	"context"
	"errors"
	"testing"
)

func TestPolymorphicRedactsUnreadableTarget(t *testing.T) {
	client := &fakeGetter{errs: map[string]error{"/services/data/v63.0/ui-api/records/500NOPE": errors.New("GET returned HTTP 403")}}
	filter := NewPolymorphicFilter(client)
	record := &Record{SObject: "Task", Fields: map[string]any{"Id": "00T", "WhatId": "500NOPE"}, Provenance: &Provenance{Redactions: map[string]int{}}}

	got := filter.Apply(context.Background(), record)
	marker, ok := got.Fields["WhatId"].(RedactionMarker)
	if !ok || marker["redacted"] != ReasonPolymorphicUnreadable {
		t.Fatalf("WhatId marker = %#v, want polymorphic redaction", got.Fields["WhatId"])
	}
	if got.Provenance.Counters.Polymorphic != 1 {
		t.Fatalf("polymorphic counter = %d, want 1", got.Provenance.Counters.Polymorphic)
	}
}
