package security

import (
	"context"
	"strings"
	"sync"
	"time"
)

type PolymorphicFilter struct {
	Client SalesforceGetter
	Now    func() time.Time

	mu    sync.Mutex
	cache map[string]visibilityEntry
}

type visibilityEntry struct {
	visible   bool
	expiresAt time.Time
}

func NewPolymorphicFilter(client SalesforceGetter) *PolymorphicFilter {
	return &PolymorphicFilter{Client: client, cache: map[string]visibilityEntry{}}
}

func (p *PolymorphicFilter) Apply(ctx context.Context, record *Record) *Record {
	if record == nil || p == nil || p.Client == nil {
		return record
	}
	for _, field := range []string{"WhatId", "WhoId", "OwnerId"} {
		id, _ := record.Fields[field].(string)
		if id == "" {
			continue
		}
		visible := p.visible(ctx, id)
		if !visible {
			MarkRedacted(record.Fields, field, ReasonPolymorphicUnreadable)
			ProvenanceCounter(record.Provenance, ReasonPolymorphicUnreadable)
		}
	}
	return record
}

func (p *PolymorphicFilter) visible(ctx context.Context, id string) bool {
	now := p.now()
	p.mu.Lock()
	if p.cache == nil {
		p.cache = map[string]visibilityEntry{}
	}
	if cached, ok := p.cache[id]; ok && now.Before(cached.expiresAt) {
		p.mu.Unlock()
		return cached.visible
	}
	p.mu.Unlock()
	visible := true
	if err := ctx.Err(); err != nil {
		visible = false
	} else {
		_, _, err := p.Client.GetWithResponseHeaders("/services/data/"+APIVersion+"/ui-api/records/"+id, nil)
		if err != nil {
			visible = !strings.Contains(err.Error(), "HTTP 403")
		}
	}
	p.mu.Lock()
	p.cache[id] = visibilityEntry{visible: visible, expiresAt: now.Add(time.Hour)}
	p.mu.Unlock()
	return visible
}

func (p *PolymorphicFilter) now() time.Time {
	if p.Now != nil {
		return p.Now()
	}
	return time.Now()
}
