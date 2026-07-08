package security

import (
	"context"
	"encoding/json"
)

type Filter interface {
	Apply(ctx context.Context, record *Record) *Record
}

type WriteFilter interface {
	AllowFieldWrite(user, sobject, field string) bool
}

type Record struct {
	SObject    string
	Fields     map[string]any
	Provenance *Provenance
}

type Provenance struct {
	Redactions map[string]int
	Counters   struct {
		Shield          int
		Polymorphic     int
		ContentScan     int
		ApexInvocations int
	}
}

type Composed struct {
	FLS         Filter
	Compliance  Filter
	Polymorphic Filter
	ContentScan Filter
}

type Options struct {
	Client                 SalesforceGetter
	ComplianceStore        ComplianceStore
	UnknownFieldRecorder   UnknownFieldRecorder
	IncludePII             bool
	IncludeShieldEncrypted bool
	RedactGroups           []string
	OrgAlias               string
	Sandbox                bool
}

func NewDefaultFilter(opts Options) *Composed {
	fls := NewFLSFilter(opts.Client)
	fls.OrgAlias = opts.OrgAlias
	fls.Sandbox = opts.Sandbox
	fls.UnknownFieldRecorder = opts.UnknownFieldRecorder
	compliance := NewComplianceFilter(opts.Client, opts.ComplianceStore)
	compliance.IncludePII = opts.IncludePII
	compliance.IncludeShieldEncrypted = opts.IncludeShieldEncrypted
	compliance.RedactGroups = opts.RedactGroups
	return &Composed{
		FLS:         fls,
		Compliance:  compliance,
		Polymorphic: NewPolymorphicFilter(opts.Client),
		ContentScan: ContentScanFilter{},
	}
}

func (c Composed) Apply(ctx context.Context, record *Record) *Record {
	if record == nil {
		return nil
	}
	ensureProvenance(record)
	for _, filter := range []Filter{c.FLS, c.Compliance, c.Polymorphic, c.ContentScan} {
		if filter != nil {
			record = filter.Apply(ctx, record)
			if record == nil {
				return nil
			}
			ensureProvenance(record)
		}
	}
	return record
}

func (c Composed) AllowFieldWrite(user, sobject, field string) bool {
	if writeFilter, ok := c.FLS.(WriteFilter); ok {
		return writeFilter.AllowFieldWrite(user, sobject, field)
	}
	return false
}

func ensureProvenance(record *Record) {
	if record.Provenance == nil {
		record.Provenance = &Provenance{}
	}
	if record.Provenance.Redactions == nil {
		record.Provenance.Redactions = map[string]int{}
	}
}

func FromJSON(sobject string, raw json.RawMessage) (*Record, error) {
	fields := map[string]any{}
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, err
	}
	return &Record{SObject: sobject, Fields: fields, Provenance: &Provenance{Redactions: map[string]int{}}}, nil
}

func ToJSON(record *Record) (json.RawMessage, error) {
	if record == nil {
		return nil, nil
	}
	data, err := json.Marshal(record.Fields)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}

func MergeProvenance(dst *Provenance, src *Provenance) {
	if dst == nil || src == nil {
		return
	}
	if dst.Redactions == nil {
		dst.Redactions = map[string]int{}
	}
	for reason, count := range src.Redactions {
		dst.Redactions[reason] += count
	}
	dst.Counters.Shield += src.Counters.Shield
	dst.Counters.Polymorphic += src.Counters.Polymorphic
	dst.Counters.ContentScan += src.Counters.ContentScan
	dst.Counters.ApexInvocations += src.Counters.ApexInvocations
}

func systemField(field string) bool {
	switch field {
	case "attributes", "Id", "id":
		return true
	default:
		return false
	}
}
