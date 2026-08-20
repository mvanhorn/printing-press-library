// Copyright 2026 markvandeven and contributors. Licensed under Apache-2.0. See LICENSE.

package rechtspraak

import (
	"fmt"
	"strings"
)

// CourtIndex provides bidirectional lookup between court codes, names, and
// PSI URIs. Built from the Instanties vocabulary once at startup, then
// queried entirely offline.
type CourtIndex struct {
	all         []Court
	byCode      map[string]Court
	byURI       map[string]Court
	byNameLower map[string]Court
}

func NewCourtIndex(courts []Court) *CourtIndex {
	idx := &CourtIndex{
		all:         courts,
		byCode:      make(map[string]Court, len(courts)),
		byURI:       make(map[string]Court, len(courts)),
		byNameLower: make(map[string]Court, len(courts)),
	}
	for _, c := range courts {
		if c.Afkorting != "" && c.Afkorting != "XX" {
			idx.byCode[c.Afkorting] = c
		}
		if c.Identifier != "" {
			idx.byURI[c.Identifier] = c
		}
		if c.Name != "" {
			idx.byNameLower[strings.ToLower(c.Name)] = c
		}
	}
	return idx
}

// Resolve accepts a code (HR), PSI URI, or fuzzy name and returns the Court.
func (i *CourtIndex) Resolve(q string) (Court, bool) {
	q = strings.TrimSpace(q)
	if q == "" {
		return Court{}, false
	}
	if c, ok := i.byURI[q]; ok {
		return c, true
	}
	if c, ok := i.byCode[q]; ok {
		return c, true
	}
	if c, ok := i.byCode[strings.ToUpper(q)]; ok {
		return c, true
	}
	low := strings.ToLower(q)
	if c, ok := i.byNameLower[low]; ok {
		return c, true
	}
	// Fuzzy partial-name match: prefer exact case-insensitive substring.
	for name, c := range i.byNameLower {
		if strings.Contains(name, low) {
			return c, true
		}
	}
	return Court{}, false
}

// URI resolves to the PSI URI or returns the input unchanged if it already
// looks like a URI.
func (i *CourtIndex) URI(q string) string {
	if strings.HasPrefix(q, "http://") || strings.HasPrefix(q, "https://") {
		return q
	}
	if c, ok := i.Resolve(q); ok {
		return c.Identifier
	}
	return ""
}

// Successors returns courts whose BeginDate is on/after the given court's
// EndDate — useful for resolving the Wet Herziening Gerechtelijke Kaart
// court mergers. The match is heuristic (same Type prefix or geographic
// overlap is left to the caller).
func (i *CourtIndex) Successors(of Court) []Court {
	if of.EndDate == "" {
		return nil
	}
	var out []Court
	for _, c := range i.all {
		if c.BeginDate == "" {
			continue
		}
		if c.BeginDate >= of.EndDate && c.Type == of.Type {
			out = append(out, c)
		}
	}
	return out
}

// Predecessors returns courts whose EndDate is on/before the given court's
// BeginDate.
func (i *CourtIndex) Predecessors(of Court) []Court {
	if of.BeginDate == "" {
		return nil
	}
	var out []Court
	for _, c := range i.all {
		if c.EndDate == "" {
			continue
		}
		if c.EndDate <= of.BeginDate && c.Type == of.Type {
			out = append(out, c)
		}
	}
	return out
}

// All returns the underlying court list.
func (i *CourtIndex) All() []Court {
	return i.all
}

// SubjectIndex provides resolution from friendly subject names/slugs to PSI URIs.
type SubjectIndex struct {
	all         []Subject
	byURI       map[string]Subject
	bySlug      map[string]Subject
	byNameLower map[string]Subject
}

func NewSubjectIndex(subjects []Subject) *SubjectIndex {
	idx := &SubjectIndex{
		all:         subjects,
		byURI:       make(map[string]Subject, len(subjects)),
		bySlug:      make(map[string]Subject, len(subjects)),
		byNameLower: make(map[string]Subject, len(subjects)),
	}
	for _, s := range subjects {
		if s.Identifier != "" {
			idx.byURI[s.Identifier] = s
		}
		if s.Slug != "" {
			idx.bySlug[s.Slug] = s
			idx.bySlug[strings.ToLower(s.Slug)] = s
		}
		if s.Name != "" {
			idx.byNameLower[strings.ToLower(s.Name)] = s
		}
	}
	return idx
}

func (i *SubjectIndex) Resolve(q string) (Subject, bool) {
	q = strings.TrimSpace(q)
	if q == "" {
		return Subject{}, false
	}
	if s, ok := i.byURI[q]; ok {
		return s, true
	}
	if s, ok := i.bySlug[q]; ok {
		return s, true
	}
	if s, ok := i.bySlug[strings.ToLower(q)]; ok {
		return s, true
	}
	low := strings.ToLower(q)
	if s, ok := i.byNameLower[low]; ok {
		return s, true
	}
	for name, s := range i.byNameLower {
		if strings.Contains(name, low) {
			return s, true
		}
	}
	return Subject{}, false
}

func (i *SubjectIndex) URI(q string) string {
	if strings.HasPrefix(q, "http://") || strings.HasPrefix(q, "https://") {
		return q
	}
	if s, ok := i.Resolve(q); ok {
		return s.Identifier
	}
	return ""
}

// All returns the underlying subject list.
func (i *SubjectIndex) All() []Subject {
	return i.all
}

// ProcedureIndex resolves procedure names to PSI URIs and back.
type ProcedureIndex struct {
	all         []Procedure
	byURI       map[string]Procedure
	bySlug      map[string]Procedure
	byNameLower map[string]Procedure
}

func NewProcedureIndex(ps []Procedure) *ProcedureIndex {
	idx := &ProcedureIndex{
		all:         ps,
		byURI:       make(map[string]Procedure, len(ps)),
		bySlug:      make(map[string]Procedure, len(ps)),
		byNameLower: make(map[string]Procedure, len(ps)),
	}
	for _, p := range ps {
		if p.Identifier != "" {
			idx.byURI[p.Identifier] = p
		}
		if p.Slug != "" {
			idx.bySlug[p.Slug] = p
			idx.bySlug[strings.ToLower(p.Slug)] = p
		}
		if p.Name != "" {
			idx.byNameLower[strings.ToLower(p.Name)] = p
		}
	}
	return idx
}

func (i *ProcedureIndex) Resolve(q string) (Procedure, bool) {
	q = strings.TrimSpace(q)
	if q == "" {
		return Procedure{}, false
	}
	if p, ok := i.byURI[q]; ok {
		return p, true
	}
	if p, ok := i.bySlug[q]; ok {
		return p, true
	}
	if p, ok := i.bySlug[strings.ToLower(q)]; ok {
		return p, true
	}
	low := strings.ToLower(q)
	if p, ok := i.byNameLower[low]; ok {
		return p, true
	}
	for name, p := range i.byNameLower {
		if strings.Contains(name, low) {
			return p, true
		}
	}
	return Procedure{}, false
}

// MatchProcedure reports whether the named procedure matches a Decision's
// procedure URI (used by local --procedure filter, since the API ignores
// procedure= as a server-side filter per IVO 1.15).
func (i *ProcedureIndex) Matches(q string, decisionProcURI string) bool {
	if decisionProcURI == "" {
		return false
	}
	p, ok := i.Resolve(q)
	if !ok {
		return false
	}
	return p.Identifier == decisionProcURI
}

// All returns the underlying procedure list.
func (i *ProcedureIndex) All() []Procedure {
	return i.all
}

// Stringifyer for human output

func (c Court) Display() string {
	if c.Afkorting != "" && c.Afkorting != "XX" {
		return fmt.Sprintf("%s (%s)", c.Name, c.Afkorting)
	}
	return c.Name
}
