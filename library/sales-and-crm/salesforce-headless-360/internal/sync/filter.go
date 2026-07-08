package sync

import (
	"context"
	"encoding/json"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/security"
)

// Record is the minimal Unit C sync record shape. Unit D will replace the
// default filter with the FLS-aware implementation.
type Record struct {
	SObject string
	Data    json.RawMessage
}

// Filter applies a record-level transformation before persistence.
type Filter interface {
	Apply(ctx context.Context, record Record) Record
}

type passThroughFilter struct{}

func (passThroughFilter) Apply(_ context.Context, record Record) Record {
	return record
}

// NoopFilter returns the Unit C pass-through filter.
func NoopFilter() Filter {
	return passThroughFilter{}
}

type securityFilter struct {
	filter     security.Filter
	provenance *security.Provenance
}

func NewSecurityFilter(filter security.Filter, provenance *security.Provenance) Filter {
	if filter == nil {
		return NoopFilter()
	}
	return securityFilter{filter: filter, provenance: provenance}
}

func (s securityFilter) Apply(ctx context.Context, record Record) Record {
	secRecord, err := security.FromJSON(record.SObject, record.Data)
	if err != nil {
		return Record{SObject: record.SObject}
	}
	secRecord = s.filter.Apply(ctx, secRecord)
	if secRecord == nil {
		return Record{SObject: record.SObject}
	}
	if s.provenance != nil {
		security.MergeProvenance(s.provenance, secRecord.Provenance)
	}
	data, err := security.ToJSON(secRecord)
	if err != nil {
		return Record{SObject: record.SObject}
	}
	return Record{SObject: record.SObject, Data: data}
}
