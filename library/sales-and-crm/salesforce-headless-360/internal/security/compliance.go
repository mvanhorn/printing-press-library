package security

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"

	safelog "github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/log"
)

type ComplianceField struct {
	SObject         string
	FieldAPIName    string
	ComplianceGroup string
	IsEncrypted     bool
}

type ComplianceStore interface {
	SaveComplianceFields([]ComplianceField) error
	ComplianceFields(sobject string) ([]ComplianceField, error)
}

type ComplianceFilter struct {
	Client                 SalesforceGetter
	Store                  ComplianceStore
	IncludePII             bool
	IncludeShieldEncrypted bool
	RedactGroups           []string

	mu         sync.Mutex
	loaded     map[string]bool
	fields     map[string]map[string]ComplianceField
	failClosed map[string]bool
}

func NewComplianceFilter(client SalesforceGetter, store ComplianceStore) *ComplianceFilter {
	return &ComplianceFilter{
		Client: client, Store: store,
		loaded: map[string]bool{}, fields: map[string]map[string]ComplianceField{}, failClosed: map[string]bool{},
	}
}

func (c *ComplianceFilter) Apply(ctx context.Context, record *Record) *Record {
	if record == nil || c == nil {
		return record
	}
	c.ensureLoaded(ctx, record.SObject)
	c.mu.Lock()
	failClosed := c.failClosed[record.SObject]
	fields := map[string]ComplianceField{}
	for key, value := range c.fields[record.SObject] {
		fields[key] = value
	}
	c.mu.Unlock()
	if failClosed {
		for key := range record.Fields {
			if systemField(key) {
				continue
			}
			MarkRedacted(record.Fields, key, ReasonPII)
			ProvenanceCounter(record.Provenance, ReasonPII)
		}
		ProvenanceCounter(record.Provenance, ReasonComplianceFailClosed)
		return record
	}
	for key := range record.Fields {
		if systemField(key) {
			continue
		}
		field, ok := fields[key]
		if !ok {
			continue
		}
		groups := splitGroups(field.ComplianceGroup)
		redacted := false
		if field.IsEncrypted && !c.IncludeShieldEncrypted {
			MarkRedacted(record.Fields, key, ReasonShieldEncrypted)
			ProvenanceCounter(record.Provenance, ReasonShieldEncrypted)
			redacted = true
		}
		for _, group := range groups {
			if c.shouldRedactGroup(group) {
				if !redacted {
					MarkRedacted(record.Fields, key, group)
					redacted = true
				}
				ProvenanceCounter(record.Provenance, group)
			}
		}
	}
	return record
}

func (c *ComplianceFilter) ensureLoaded(ctx context.Context, sobject string) {
	c.mu.Lock()
	if c.loaded[sobject] {
		c.mu.Unlock()
		return
	}
	c.loaded[sobject] = true
	c.mu.Unlock()

	fields, err := c.loadFromStore(sobject)
	if err == nil && len(fields) > 0 {
		c.setFields(sobject, fields, false)
		return
	}
	fields, err = c.loadFromTooling(ctx, []string{sobject})
	if err != nil {
		c.mu.Lock()
		c.failClosed[sobject] = true
		c.mu.Unlock()
		return
	}
	if c.Store != nil {
		_ = c.Store.SaveComplianceFields(fields)
	}
	c.setFields(sobject, fields, false)
	registerComplianceFields(fields)
}

func (c *ComplianceFilter) loadFromStore(sobject string) ([]ComplianceField, error) {
	if c.Store == nil {
		return nil, nil
	}
	fields, err := c.Store.ComplianceFields(sobject)
	if err != nil {
		return nil, err
	}
	registerComplianceFields(fields)
	return fields, nil
}

func (c *ComplianceFilter) loadFromTooling(ctx context.Context, sobjects []string) ([]ComplianceField, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if c.Client == nil {
		return nil, fmt.Errorf("compliance client not configured")
	}
	q := "SELECT QualifiedApiName, ComplianceGroup, IsEncrypted, EntityDefinition.QualifiedApiName FROM FieldDefinition WHERE EntityDefinitionId IN (" + soqlStringList(sobjects) + ")"
	body, _, err := c.Client.GetWithResponseHeaders("/services/data/"+APIVersion+"/tooling/query", map[string]string{"q": q})
	if err != nil {
		return nil, err
	}
	body = unwrapEnvelope(body)
	var payload struct {
		Records []struct {
			QualifiedAPIName string `json:"QualifiedApiName"`
			ComplianceGroup  any    `json:"ComplianceGroup"`
			IsEncrypted      bool   `json:"IsEncrypted"`
			EntityDefinition struct {
				QualifiedAPIName string `json:"QualifiedApiName"`
			} `json:"EntityDefinition"`
			DurableID string `json:"DurableId"`
		} `json:"records"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	out := make([]ComplianceField, 0, len(payload.Records))
	for _, record := range payload.Records {
		sobject := record.EntityDefinition.QualifiedAPIName
		if sobject == "" && strings.Contains(record.DurableID, ".") {
			sobject = strings.SplitN(record.DurableID, ".", 2)[0]
		}
		group := ""
		if record.ComplianceGroup != nil {
			group = fmt.Sprint(record.ComplianceGroup)
			if group == "<nil>" {
				group = ""
			}
		}
		out = append(out, ComplianceField{
			SObject: sobject, FieldAPIName: record.QualifiedAPIName, ComplianceGroup: group, IsEncrypted: record.IsEncrypted,
		})
	}
	return out, nil
}

func (c *ComplianceFilter) setFields(sobject string, fields []ComplianceField, failClosed bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.fields == nil {
		c.fields = map[string]map[string]ComplianceField{}
	}
	if c.failClosed == nil {
		c.failClosed = map[string]bool{}
	}
	c.failClosed[sobject] = failClosed
	c.fields[sobject] = map[string]ComplianceField{}
	for _, field := range fields {
		if field.SObject == sobject && field.FieldAPIName != "" {
			c.fields[sobject][field.FieldAPIName] = field
		}
	}
}

func (c *ComplianceFilter) shouldRedactGroup(group string) bool {
	group = strings.TrimSpace(group)
	if group == "" {
		return false
	}
	for _, redact := range c.RedactGroups {
		if strings.EqualFold(strings.TrimSpace(redact), group) {
			return true
		}
	}
	if c.IncludePII {
		return false
	}
	switch strings.ToUpper(group) {
	case "PII", "HIPAA", "GLBA", "PCI":
		return true
	default:
		return false
	}
}

func splitGroups(group string) []string {
	parts := strings.FieldsFunc(group, func(r rune) bool {
		return r == ',' || r == ';' || r == '|' || r == ' '
	})
	out := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || strings.EqualFold(part, "null") || seen[part] {
			continue
		}
		seen[part] = true
		out = append(out, part)
	}
	return out
}

func registerComplianceFields(fields []ComplianceField) {
	names := make([]string, 0, len(fields)*2)
	for _, field := range fields {
		if field.ComplianceGroup == "" && !field.IsEncrypted {
			continue
		}
		names = append(names, field.FieldAPIName)
		if field.SObject != "" && field.FieldAPIName != "" {
			names = append(names, field.SObject+"."+field.FieldAPIName)
		}
	}
	safelog.RegisterComplianceFields(names)
}

func soqlStringList(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ReplaceAll(value, "'", "\\'")
		quoted = append(quoted, "'"+value+"'")
	}
	sort.Strings(quoted)
	return strings.Join(quoted, ",")
}

func ValidateSensitiveOverride(flagName string, include bool, yes bool, isTTY bool) error {
	if !include || yes {
		return nil
	}
	if !isTTY {
		return fmt.Errorf("%s requires --yes in non-interactive mode", flagName)
	}
	fmt.Fprintf(os.Stderr, "%s exposes restricted data. Re-run with --yes to confirm.\n", flagName)
	return fmt.Errorf("%s requires explicit confirmation", flagName)
}
