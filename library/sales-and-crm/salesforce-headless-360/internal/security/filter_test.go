package security

import (
	"context"
	"testing"
)

type addFilter struct {
	key string
	val any
}

func (f addFilter) Apply(_ context.Context, record *Record) *Record {
	record.Fields[f.key] = f.val
	ProvenanceCounter(record.Provenance, f.key)
	return record
}

func TestComposedRunsFiltersInOrderAndKeepsProvenance(t *testing.T) {
	record := &Record{SObject: "Contact", Fields: map[string]any{"Id": "003"}}
	filter := Composed{
		FLS:         addFilter{key: "fls", val: 1},
		Compliance:  addFilter{key: "compliance", val: 2},
		Polymorphic: addFilter{key: "polymorphic", val: 3},
		ContentScan: addFilter{key: "content", val: 4},
	}
	got := filter.Apply(context.Background(), record)
	for _, key := range []string{"fls", "compliance", "polymorphic", "content"} {
		if got.Fields[key] == nil {
			t.Fatalf("missing field %s after composed apply: %#v", key, got.Fields)
		}
		if got.Provenance.Redactions[key] != 1 {
			t.Fatalf("redaction count %s = %d, want 1", key, got.Provenance.Redactions[key])
		}
	}
}
