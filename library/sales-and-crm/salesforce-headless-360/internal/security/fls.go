package security

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

const APIVersion = "v63.0"

type SalesforceGetter interface {
	GetWithResponseHeaders(path string, params map[string]string) (json.RawMessage, http.Header, error)
}

type UnknownFieldRecorder interface {
	RecordUnknownFieldSeen(sobject, field string) error
}

type FLSFilter struct {
	Client               SalesforceGetter
	OrgAlias             string
	Sandbox              bool
	UnknownFieldRecorder UnknownFieldRecorder
	Now                  func() time.Time

	mu       sync.Mutex
	describe map[string]describeEntry
}

type describeEntry struct {
	fields              map[string]struct{}
	fieldUpdateable     map[string]bool
	objectUpdateable    bool
	hasWritePermissions bool
	expiresAt           time.Time
}

func NewFLSFilter(client SalesforceGetter) *FLSFilter {
	return &FLSFilter{Client: client, describe: map[string]describeEntry{}}
}

func (f *FLSFilter) Apply(ctx context.Context, record *Record) *Record {
	if record == nil || f == nil || f.Client == nil {
		return record
	}
	id, _ := record.Fields["Id"].(string)
	if id == "" {
		id, _ = record.Fields["id"].(string)
	}
	if id == "" {
		return record
	}
	describe := f.describeFields(ctx, record.SObject, record.Fields)
	uiFields, ok := f.uiRecordFields(ctx, record.SObject, id, requestedFields(record.SObject, record.Fields))
	if !ok {
		for key := range record.Fields {
			if !systemField(key) {
				delete(record.Fields, key)
			}
		}
		return record
	}
	for key := range uiFields {
		if _, known := describe[key]; !known && !systemField(key) {
			f.invalidate(record.SObject)
			if f.UnknownFieldRecorder != nil {
				_ = f.UnknownFieldRecorder.RecordUnknownFieldSeen(record.SObject, key)
			}
			describe = f.describeFields(ctx, record.SObject, record.Fields)
			break
		}
	}
	for key := range record.Fields {
		if systemField(key) {
			continue
		}
		if _, known := describe[key]; !known {
			delete(record.Fields, key)
			continue
		}
		value, visible := uiFields[key]
		if !visible || value == nil {
			delete(record.Fields, key)
		}
	}
	return record
}

func (f *FLSFilter) describeFields(ctx context.Context, sobject string, fallback map[string]any) map[string]struct{} {
	now := f.now()
	f.mu.Lock()
	if f.describe == nil {
		f.describe = map[string]describeEntry{}
	}
	if cached, ok := f.describe[sobject]; ok && now.Before(cached.expiresAt) {
		out := copySet(cached.fields)
		f.mu.Unlock()
		return out
	}
	f.mu.Unlock()

	fields, err := f.fetchDescribe(ctx, sobject)
	if err != nil || len(fields) == 0 {
		fields = map[string]struct{}{}
		for key := range fallback {
			if !systemField(key) {
				fields[key] = struct{}{}
			}
		}
		fields["Id"] = struct{}{}
	}
	ttl := time.Hour
	if f.Sandbox {
		ttl = 24 * time.Hour
	}
	f.mu.Lock()
	f.describe[sobject] = describeEntry{fields: copySet(fields), expiresAt: now.Add(ttl)}
	f.mu.Unlock()
	return fields
}

func (f *FLSFilter) fetchDescribe(ctx context.Context, sobject string) (map[string]struct{}, error) {
	entry, err := f.fetchDescribeEntry(ctx, sobject)
	if err != nil {
		return nil, err
	}
	return entry.fields, nil
}

func (f *FLSFilter) AllowFieldWrite(user, sobject, field string) bool {
	_ = user
	if f == nil || f.Client == nil || sobject == "" || field == "" || systemField(field) {
		return false
	}
	now := f.now()
	f.mu.Lock()
	if f.describe == nil {
		f.describe = map[string]describeEntry{}
	}
	if cached, ok := f.describe[sobject]; ok && now.Before(cached.expiresAt) && cached.hasWritePermissions {
		allowed := cached.objectUpdateable && cached.fieldUpdateable[field]
		f.mu.Unlock()
		return allowed
	}
	f.mu.Unlock()

	entry, err := f.fetchDescribeEntry(context.Background(), sobject)
	if err != nil {
		return false
	}
	ttl := time.Hour
	if f.Sandbox {
		ttl = 24 * time.Hour
	}
	entry.expiresAt = now.Add(ttl)
	f.mu.Lock()
	f.describe[sobject] = entry
	f.mu.Unlock()
	return entry.objectUpdateable && entry.fieldUpdateable[field]
}

func (f *FLSFilter) fetchDescribeEntry(ctx context.Context, sobject string) (describeEntry, error) {
	if err := ctx.Err(); err != nil {
		return describeEntry{}, err
	}
	path := "/services/data/" + APIVersion + "/sobjects/" + sobject + "/describe"
	body, _, err := f.Client.GetWithResponseHeaders(path, nil)
	if err != nil {
		return describeEntry{}, err
	}
	body = unwrapEnvelope(body)
	var payload struct {
		Updateable bool `json:"updateable"`
		Fields     []struct {
			Name       string `json:"name"`
			Updateable bool   `json:"updateable"`
		} `json:"fields"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return describeEntry{}, err
	}
	fields := map[string]struct{}{}
	fieldUpdateable := map[string]bool{}
	for _, field := range payload.Fields {
		if field.Name != "" {
			fields[field.Name] = struct{}{}
			fieldUpdateable[field.Name] = field.Updateable
		}
	}
	return describeEntry{
		fields:              fields,
		fieldUpdateable:     fieldUpdateable,
		objectUpdateable:    payload.Updateable,
		hasWritePermissions: true,
	}, nil
}

func (f *FLSFilter) uiRecordFields(ctx context.Context, sobject, id string, fields []string) (map[string]any, bool) {
	if err := ctx.Err(); err != nil {
		return nil, false
	}
	params := map[string]string{}
	if len(fields) > 0 {
		params["fields"] = strings.Join(fields, ",")
	}
	body, _, err := f.Client.GetWithResponseHeaders("/services/data/"+APIVersion+"/ui-api/records/"+id, params)
	if err != nil {
		return nil, false
	}
	body = unwrapEnvelope(body)
	var payload struct {
		Fields map[string]struct {
			Value any `json:"value"`
		} `json:"fields"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, false
	}
	out := map[string]any{}
	for key, value := range payload.Fields {
		out[key] = value.Value
	}
	return out, true
}

func (f *FLSFilter) invalidate(sobject string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.describe, sobject)
}

func (f *FLSFilter) now() time.Time {
	if f.Now != nil {
		return f.Now()
	}
	return time.Now()
}

func requestedFields(sobject string, fields map[string]any) []string {
	out := make([]string, 0, len(fields))
	for key := range fields {
		if key == "attributes" {
			continue
		}
		out = append(out, fmt.Sprintf("%s.%s", sobject, key))
	}
	sort.Strings(out)
	return out
}

func unwrapEnvelope(body json.RawMessage) json.RawMessage {
	var wrapper struct {
		Envelope json.RawMessage `json:"envelope"`
	}
	if json.Unmarshal(body, &wrapper) == nil && len(wrapper.Envelope) > 0 {
		return wrapper.Envelope
	}
	return body
}

func copySet(in map[string]struct{}) map[string]struct{} {
	out := make(map[string]struct{}, len(in))
	for key := range in {
		out[key] = struct{}{}
	}
	return out
}
