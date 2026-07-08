package slack

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/security"
)

const (
	QueryPath      = "/services/data/v63.0/query"
	SourceLinkage  = "slack_linkage"
	sobjectLinkage = "SlackConversationRelation"
)

type SOQLClient interface {
	Get(path string, params map[string]string) (json.RawMessage, error)
}

type Availability struct {
	Source    string
	Available bool
	Reason    string
	Warning   string
}

type Linkage struct {
	EntityID    string
	ChannelID   string
	WorkspaceID string
}

type LinkageResult struct {
	Rows         []Linkage
	Availability Availability
	Provenance   *security.Provenance
}

func LinkageFetch(ctx context.Context, c SOQLClient, entityID string, filter security.Filter) (LinkageResult, error) {
	result := LinkageResult{
		Availability: Availability{Source: SourceLinkage},
		Provenance:   &security.Provenance{Redactions: map[string]int{}},
	}
	if c == nil {
		result.Availability.Reason = "no_client"
		return result, nil
	}
	q := "SELECT EntityId, SlackChannelId, WorkspaceId FROM SlackConversationRelation WHERE EntityId = " + soqlQuote(entityID)
	body, err := c.Get(QueryPath, map[string]string{"q": q})
	if err != nil {
		return result, fmt.Errorf("slack linkage query: %w", err)
	}
	var page struct {
		TotalSize int               `json:"totalSize"`
		Records   []json.RawMessage `json:"records"`
	}
	if err := unmarshalEnvelope(body, &page); err != nil {
		return result, fmt.Errorf("parse slack linkage query: %w", err)
	}
	if page.TotalSize == 0 || len(page.Records) == 0 {
		result.Availability.Reason = "no_rows"
		return result, nil
	}
	for _, raw := range page.Records {
		filtered, provenance, err := applyFilter(ctx, filter, sobjectLinkage, raw)
		if err != nil {
			return result, err
		}
		security.MergeProvenance(result.Provenance, provenance)
		if len(filtered) == 0 {
			continue
		}
		row, ok := parseLinkage(filtered)
		if ok {
			result.Rows = append(result.Rows, row)
		}
	}
	if len(result.Rows) == 0 {
		result.Availability.Reason = "filtered"
		return result, nil
	}
	result.Availability.Available = true
	return result, nil
}

func parseLinkage(raw json.RawMessage) (Linkage, bool) {
	var row struct {
		EntityID            string `json:"EntityId"`
		RelatedRecordID     string `json:"RelatedRecordId"`
		SlackChannelID      string `json:"SlackChannelId"`
		SlackConversationID string `json:"SlackConversationId"`
		WorkspaceID         string `json:"WorkspaceId"`
		SlackWorkspaceID    string `json:"SlackWorkspaceId"`
	}
	if err := json.Unmarshal(raw, &row); err != nil {
		return Linkage{}, false
	}
	channelID := firstString(row.SlackChannelID, row.SlackConversationID)
	if channelID == "" {
		return Linkage{}, false
	}
	return Linkage{
		EntityID:    firstString(row.EntityID, row.RelatedRecordID),
		ChannelID:   channelID,
		WorkspaceID: firstString(row.WorkspaceID, row.SlackWorkspaceID),
	}, true
}

func applyFilter(ctx context.Context, filter security.Filter, sobject string, raw json.RawMessage) (json.RawMessage, *security.Provenance, error) {
	provenance := &security.Provenance{Redactions: map[string]int{}}
	if filter == nil {
		return raw, provenance, nil
	}
	record, err := security.FromJSON(sobject, raw)
	if err != nil {
		return nil, provenance, err
	}
	record = filter.Apply(ctx, record)
	if record == nil {
		return nil, provenance, nil
	}
	security.MergeProvenance(provenance, record.Provenance)
	data, err := security.ToJSON(record)
	return data, provenance, err
}

func unmarshalEnvelope(data json.RawMessage, v any) error {
	var wrapper struct {
		Envelope json.RawMessage `json:"envelope"`
	}
	if err := json.Unmarshal(data, &wrapper); err == nil && len(wrapper.Envelope) > 0 {
		return json.Unmarshal(wrapper.Envelope, v)
	}
	return json.Unmarshal(data, v)
}

func soqlQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "\\'") + "'"
}

func EncodedSOQLForTest(entityID string) string {
	q := "SELECT EntityId, SlackChannelId, WorkspaceId FROM SlackConversationRelation WHERE EntityId = " + soqlQuote(entityID)
	return url.QueryEscape(q)
}

func firstString(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
