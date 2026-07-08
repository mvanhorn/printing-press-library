package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	APIVersion         = "v63.0"
	CompositeGraphPath = "/services/data/" + APIVersion + "/composite/graph"
)

type GraphRequest struct {
	Graphs []Graph `json:"graphs"`
}

type Graph struct {
	GraphID          string      `json:"graphId"`
	CompositeRequest []GraphNode `json:"compositeRequest"`
}

type GraphNode struct {
	Method      string `json:"method"`
	URL         string `json:"url"`
	ReferenceID string `json:"referenceId"`
}

type GraphResult struct {
	Records    map[string][]json.RawMessage
	PageRefs   []PageRef
	TotalSizes map[string]int
}

func FilterGraphRecords(ctx context.Context, graph *GraphResult, filter Filter) map[string][]json.RawMessage {
	out := map[string][]json.RawMessage{}
	if graph == nil {
		return out
	}
	if filter == nil {
		filter = NoopFilter()
	}
	for sobject, records := range graph.Records {
		for _, raw := range records {
			record := filter.Apply(ctx, Record{SObject: sobject, Data: raw})
			if len(record.Data) == 0 {
				continue
			}
			out[sobject] = append(out[sobject], record.Data)
		}
	}
	return out
}

type PageRef struct {
	SObject        string
	NextRecordsURL string
}

func BuildAccountGraph(accountID string, sinceUTC time.Time) (*GraphRequest, error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return nil, fmt.Errorf("account id is required")
	}

	accountLiteral := soqlQuote(accountID)
	nodes := []GraphNode{
		queryNode("Account", "SELECT Id, Name, Industry, AnnualRevenue, NumberOfEmployees, LastModifiedDate FROM Account WHERE Id = "+accountLiteral, sinceUTC),
		queryNode("Contacts", "SELECT Id, AccountId, FirstName, LastName, Name, Email, Title, LastModifiedDate FROM Contact WHERE AccountId = "+accountLiteral, sinceUTC),
		queryNode("Opportunities", "SELECT Id, AccountId, Name, StageName, Amount, CloseDate, LastModifiedDate FROM Opportunity WHERE AccountId = "+accountLiteral, sinceUTC),
		queryNode("Cases", "SELECT Id, AccountId, Subject, Status, Priority, LastModifiedDate FROM Case WHERE AccountId = "+accountLiteral, sinceUTC),
		queryNode("Tasks", "SELECT Id, WhatId, WhoId, Subject, Status, ActivityDate, LastModifiedDate FROM Task WHERE (WhatId = "+accountLiteral+" OR WhoId IN (SELECT Id FROM Contact WHERE AccountId = "+accountLiteral+"))", sinceUTC),
		queryNode("Events", "SELECT Id, WhatId, WhoId, Subject, ActivityDate, LastModifiedDate FROM Event WHERE (WhatId = "+accountLiteral+" OR WhoId IN (SELECT Id FROM Contact WHERE AccountId = "+accountLiteral+"))", sinceUTC),
		queryNode("Chatter", "SELECT Id, ParentId, Body, CreatedDate FROM FeedItem WHERE ParentId = "+accountLiteral, sinceUTC),
		queryNode("ContentDocumentLinks", "SELECT Id, LinkedEntityId, ContentDocumentId, ShareType, Visibility, SystemModstamp FROM ContentDocumentLink WHERE LinkedEntityId = "+accountLiteral, sinceUTC),
	}
	if len(nodes) > 500 {
		return nil, fmt.Errorf("composite graph node budget exceeded: %d > 500", len(nodes))
	}
	return &GraphRequest{
		Graphs: []Graph{{
			GraphID:          "acme-graph",
			CompositeRequest: nodes,
		}},
	}, nil
}

func queryNode(referenceID, q string, sinceUTC time.Time) GraphNode {
	if !sinceUTC.IsZero() {
		q += " AND LastModifiedDate >= " + sinceUTC.UTC().Format("2006-01-02T15:04:05Z")
	}
	return GraphNode{
		Method:      "GET",
		URL:         "/services/data/" + APIVersion + "/query?q=" + url.QueryEscape(q),
		ReferenceID: referenceID,
	}
}

func ParseGraphResponse(data json.RawMessage) (*GraphResult, error) {
	var wrapper struct {
		Envelope json.RawMessage `json:"envelope"`
	}
	if err := json.Unmarshal(data, &wrapper); err == nil && len(wrapper.Envelope) > 0 {
		return ParseGraphResponse(wrapper.Envelope)
	}

	var envelope struct {
		Graphs []struct {
			GraphID       string `json:"graphId"`
			IsSuccessful  bool   `json:"isSuccessful"`
			GraphResponse struct {
				CompositeResponse []struct {
					ReferenceID    string          `json:"referenceId"`
					HTTPStatusCode int             `json:"httpStatusCode"`
					Body           json.RawMessage `json:"body"`
				} `json:"compositeResponse"`
			} `json:"graphResponse"`
		} `json:"graphs"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("parse composite graph response: %w", err)
	}
	if len(envelope.Graphs) == 0 {
		return nil, fmt.Errorf("parse composite graph response: no graphs")
	}

	result := &GraphResult{
		Records:    map[string][]json.RawMessage{},
		TotalSizes: map[string]int{},
	}
	for _, graph := range envelope.Graphs {
		if !graph.IsSuccessful {
			// Surface per-subrequest Salesforce error detail (errorCode +
			// message) so a failure like a customized org dropping a queried
			// field (e.g. Account.AnnualRevenue removed) is diagnosable from
			// the CLI output instead of an opaque "was not successful".
			var details []string
			for _, item := range graph.GraphResponse.CompositeResponse {
				if item.HTTPStatusCode >= 400 {
					details = append(details, fmt.Sprintf("%s (HTTP %d): %s", item.ReferenceID, item.HTTPStatusCode, nodeErrorDetail(item.Body)))
				}
			}
			if len(details) > 0 {
				return nil, fmt.Errorf("composite graph %s was not successful: %s", graph.GraphID, strings.Join(details, "; "))
			}
			return nil, fmt.Errorf("composite graph %s was not successful", graph.GraphID)
		}
		for _, item := range graph.GraphResponse.CompositeResponse {
			if item.HTTPStatusCode >= 400 {
				return nil, fmt.Errorf("composite node %s returned HTTP %d: %s", item.ReferenceID, item.HTTPStatusCode, nodeErrorDetail(item.Body))
			}
			page, err := parseQueryPage(item.Body)
			if err != nil {
				return nil, fmt.Errorf("parse composite node %s: %w", item.ReferenceID, err)
			}
			sobject := objectName(item.ReferenceID, page.Records)
			result.Records[sobject] = append(result.Records[sobject], page.Records...)
			result.TotalSizes[sobject] += page.TotalSize
			if !page.Done && page.NextRecordsURL != "" {
				result.PageRefs = append(result.PageRefs, PageRef{SObject: sobject, NextRecordsURL: page.NextRecordsURL})
			}
		}
	}
	sort.Slice(result.PageRefs, func(i, j int) bool {
		if result.PageRefs[i].SObject == result.PageRefs[j].SObject {
			return result.PageRefs[i].NextRecordsURL < result.PageRefs[j].NextRecordsURL
		}
		return result.PageRefs[i].SObject < result.PageRefs[j].SObject
	})
	return result, nil
}

// nodeErrorDetail renders a composite subrequest's error body. Salesforce
// returns query errors as [{"message": ..., "errorCode": ...}]; anything
// that doesn't match that shape falls back to the raw body, truncated.
func nodeErrorDetail(body json.RawMessage) string {
	var errs []struct {
		Message   string `json:"message"`
		ErrorCode string `json:"errorCode"`
	}
	if json.Unmarshal(body, &errs) == nil && len(errs) > 0 {
		parts := make([]string, 0, len(errs))
		for _, e := range errs {
			switch {
			case e.ErrorCode != "" && e.Message != "":
				parts = append(parts, e.ErrorCode+": "+e.Message)
			case e.Message != "":
				parts = append(parts, e.Message)
			case e.ErrorCode != "":
				parts = append(parts, e.ErrorCode)
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, "; ")
		}
	}
	s := strings.TrimSpace(string(body))
	if len(s) > 300 {
		s = s[:300] + "..."
	}
	if s == "" {
		return "(no error body)"
	}
	return s
}

func objectName(referenceID string, records []json.RawMessage) string {
	for _, record := range records {
		var obj struct {
			Attributes struct {
				Type string `json:"type"`
			} `json:"attributes"`
		}
		if json.Unmarshal(record, &obj) == nil && obj.Attributes.Type != "" {
			return obj.Attributes.Type
		}
	}
	switch referenceID {
	case "Contacts":
		return "Contact"
	case "Opportunities":
		return "Opportunity"
	case "Cases":
		return "Case"
	case "Tasks":
		return "Task"
	case "Events":
		return "Event"
	case "Chatter":
		return "FeedItem"
	default:
		return strings.TrimSuffix(referenceID, "s")
	}
}

func soqlQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "\\'") + "'"
}
