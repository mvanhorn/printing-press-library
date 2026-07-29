// Copyright 2026 alon-auto and contributors. Licensed under Apache-2.0. See LICENSE.

// Package priorityx holds Priority-specific pure logic for the novel commands:
// EDMX ($metadata) parsing, schema snapshot diffing, and invoice-age bucketing.
package priorityx

import (
	"encoding/xml"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Form is one Priority form (OData EntityType) parsed from $metadata.
type Form struct {
	Name     string    `json:"name"`
	Fields   []Field   `json:"fields"`
	Subforms []Subform `json:"subforms"`
}

// Field is one form column (OData Property).
type Field struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Mandatory   bool   `json:"mandatory,omitempty"`
	Description string `json:"description,omitempty"`
}

// Subform is a navigation property (related lines or linked record).
type Subform struct {
	Name       string `json:"name"`
	Target     string `json:"target,omitempty"`
	Collection bool   `json:"collection"`
}

type edmxDocument struct {
	XMLName      xml.Name       `xml:"Edmx"`
	DataServices []edmxServices `xml:"DataServices"`
}

type edmxServices struct {
	Schemas []edmxSchema `xml:"Schema"`
}

type edmxSchema struct {
	Namespace string           `xml:"Namespace,attr"`
	Entities  []edmxEntityType `xml:"EntityType"`
}

type edmxEntityType struct {
	Name       string           `xml:"Name,attr"`
	Properties []edmxProperty   `xml:"Property"`
	NavProps   []edmxNavigation `xml:"NavigationProperty"`
}

type edmxProperty struct {
	Name        string           `xml:"Name,attr"`
	Type        string           `xml:"Type,attr"`
	Annotations []edmxAnnotation `xml:"Annotation"`
}

type edmxNavigation struct {
	Name string `xml:"Name,attr"`
	Type string `xml:"Type,attr"`
}

type edmxAnnotation struct {
	Term string `xml:"Term,attr"`
	Bool string `xml:"Bool,attr"`
	Str  string `xml:"String,attr"`
}

// ParseEDMX parses a Priority $metadata EDMX document into forms with fields
// and subforms. Priority emits one Schema (namespace Priority.OData) whose
// EntityTypes are forms; NavigationProperties are subforms — Collection(...)
// types are related-line subforms, plain types are single linked records.
func ParseEDMX(raw []byte) ([]Form, error) {
	var doc edmxDocument
	if err := xml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("parsing EDMX: %w", err)
	}
	var forms []Form
	for _, svc := range doc.DataServices {
		for _, schema := range svc.Schemas {
			for _, et := range schema.Entities {
				f := Form{Name: et.Name}
				for _, p := range et.Properties {
					fld := Field{Name: p.Name, Type: strings.TrimPrefix(p.Type, "Edm.")}
					for _, a := range p.Annotations {
						switch {
						case strings.HasSuffix(a.Term, "Mandatory") && strings.EqualFold(a.Bool, "true"):
							fld.Mandatory = true
						case strings.HasSuffix(a.Term, "Description") && a.Str != "":
							fld.Description = a.Str
						}
					}
					f.Fields = append(f.Fields, fld)
				}
				for _, n := range et.NavProps {
					sub := Subform{Name: n.Name}
					t := n.Type
					if strings.HasPrefix(t, "Collection(") {
						sub.Collection = true
						t = strings.TrimSuffix(strings.TrimPrefix(t, "Collection("), ")")
					} else {
						sub.Collection = false
					}
					if idx := strings.LastIndex(t, "."); idx >= 0 {
						t = t[idx+1:]
					}
					sub.Target = t
					f.Subforms = append(f.Subforms, sub)
				}
				forms = append(forms, f)
			}
		}
	}
	sort.Slice(forms, func(i, j int) bool { return forms[i].Name < forms[j].Name })
	if len(forms) == 0 {
		return nil, fmt.Errorf("EDMX contained no EntityType definitions")
	}
	return forms, nil
}

// SchemaDiff is the result of comparing two schema snapshots.
type SchemaDiff struct {
	AddedForms    []string            `json:"added_forms"`
	RemovedForms  []string            `json:"removed_forms"`
	AddedFields   map[string][]string `json:"added_fields,omitempty"`
	RemovedFields map[string][]string `json:"removed_fields,omitempty"`
	ChangedFields map[string][]string `json:"changed_fields,omitempty"`
}

// Empty reports whether the diff contains no changes.
func (d SchemaDiff) Empty() bool {
	return len(d.AddedForms) == 0 && len(d.RemovedForms) == 0 &&
		len(d.AddedFields) == 0 && len(d.RemovedFields) == 0 && len(d.ChangedFields) == 0
}

// DiffSchemas compares a baseline snapshot against a current one. Both inputs
// are keyed by form name.
func DiffSchemas(baseline, current []Form) SchemaDiff {
	diff := SchemaDiff{
		AddedFields:   map[string][]string{},
		RemovedFields: map[string][]string{},
		ChangedFields: map[string][]string{},
	}
	base := map[string]Form{}
	for _, f := range baseline {
		base[f.Name] = f
	}
	cur := map[string]Form{}
	for _, f := range current {
		cur[f.Name] = f
	}
	for name := range cur {
		if _, ok := base[name]; !ok {
			diff.AddedForms = append(diff.AddedForms, name)
		}
	}
	for name := range base {
		if _, ok := cur[name]; !ok {
			diff.RemovedForms = append(diff.RemovedForms, name)
		}
	}
	for name, cf := range cur {
		bf, ok := base[name]
		if !ok {
			continue
		}
		bFields := map[string]Field{}
		for _, fl := range bf.Fields {
			bFields[fl.Name] = fl
		}
		cFields := map[string]Field{}
		for _, fl := range cf.Fields {
			cFields[fl.Name] = fl
		}
		for fn, cfl := range cFields {
			bfl, ok := bFields[fn]
			if !ok {
				diff.AddedFields[name] = append(diff.AddedFields[name], fn)
				continue
			}
			if bfl.Type != cfl.Type || bfl.Mandatory != cfl.Mandatory {
				diff.ChangedFields[name] = append(diff.ChangedFields[name], fn)
			}
		}
		for fn := range bFields {
			if _, ok := cFields[fn]; !ok {
				diff.RemovedFields[name] = append(diff.RemovedFields[name], fn)
			}
		}
	}
	sort.Strings(diff.AddedForms)
	sort.Strings(diff.RemovedForms)
	for _, m := range []map[string][]string{diff.AddedFields, diff.RemovedFields, diff.ChangedFields} {
		for k := range m {
			sort.Strings(m[k])
		}
	}
	if len(diff.AddedFields) == 0 {
		diff.AddedFields = nil
	}
	if len(diff.RemovedFields) == 0 {
		diff.RemovedFields = nil
	}
	if len(diff.ChangedFields) == 0 {
		diff.ChangedFields = nil
	}
	return diff
}

// AgeBucket labels for invoice-date aging.
var AgeBuckets = []string{"0-30", "31-60", "61-90", "90+"}

// BucketFor returns the aging bucket index for an invoice date relative to
// now: 0 for 0-30 days old, 1 for 31-60, 2 for 61-90, 3 for older. Returns -1
// when the invoice date is in the future or unparseable.
func BucketFor(ivDate string, now time.Time) int {
	t, err := time.Parse(time.RFC3339, ivDate)
	if err != nil {
		// Priority also returns bare dates in some locales.
		t, err = time.Parse("2006-01-02", ivDate)
		if err != nil {
			return -1
		}
	}
	days := int(now.Sub(t).Hours() / 24)
	switch {
	case days < 0:
		return -1
	case days <= 30:
		return 0
	case days <= 60:
		return 1
	case days <= 90:
		return 2
	default:
		return 3
	}
}
