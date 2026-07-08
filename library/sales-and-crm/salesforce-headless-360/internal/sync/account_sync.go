package sync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/store"
)

type AccountSyncClient interface {
	PostWithResponseHeaders(path string, body any) (json.RawMessage, int, http.Header, error)
	GetWithResponseHeaders(path string, params map[string]string) (json.RawMessage, http.Header, error)
}

type PartialSyncError struct {
	Stage string
	Err   error
}

func (e *PartialSyncError) Error() string {
	return fmt.Sprintf("partial sync halted at %s: %v", e.Stage, e.Err)
}

func (e *PartialSyncError) Unwrap() error {
	return e.Err
}

func IsPartialSync(err error) bool {
	var partial *PartialSyncError
	return errors.As(err, &partial)
}

func SyncAccount(c AccountSyncClient, db *store.Store, accountID string, since time.Time, filter Filter, gate *Gate) (int, error) {
	if filter == nil {
		filter = NoopFilter()
	}
	if gate == nil {
		gate = NewGate()
	}
	request, err := BuildAccountGraph(accountID, since)
	if err != nil {
		return 0, err
	}

	body, _, headers, err := c.PostWithResponseHeaders(CompositeGraphPath, request)
	if err != nil {
		return 0, fmt.Errorf("stage: sync.composite_graph: %w", err)
	}
	if err := gate.UpdateFromHeaders(headers); err != nil {
		return 0, err
	}
	if err := gate.Check(); err != nil {
		return 0, &PartialSyncError{Stage: "sync.composite_graph", Err: err}
	}

	graph, err := ParseGraphResponse(body)
	if err != nil {
		return 0, err
	}
	for _, pageRef := range graph.PageRefs {
		rows, err := FetchRemainingPages(c, pageRef, gate)
		if err != nil {
			return 0, err
		}
		graph.Records[pageRef.SObject] = append(graph.Records[pageRef.SObject], rows...)
	}

	var total int
	for _, sobject := range orderedSObjects(graph.Records) {
		records := graph.Records[sobject]
		visible, err := FilterVisibleRecords(c, db, accountID, sobject, records, gate)
		if err != nil {
			return total, err
		}
		filteredGraph := &GraphResult{Records: map[string][]json.RawMessage{sobject: visible}}
		filteredRecords := FilterGraphRecords(context.Background(), filteredGraph, filter)[sobject]
		resource := ResourceType(sobject)
		if len(filteredRecords) == 0 {
			continue
		}
		if err := db.UpsertBatch(resource, filteredRecords); err != nil {
			return total, fmt.Errorf("upsert %s: %w", resource, err)
		}
		total += len(filteredRecords)
		if err := db.SaveSyncState(resource, "", len(filteredRecords)); err != nil {
			return total, fmt.Errorf("save sync state for %s: %w", resource, err)
		}
	}
	return total, nil
}

func ResourceType(sobject string) string {
	switch sobject {
	case "Account":
		return "accounts"
	case "Contact":
		return "contacts"
	case "Opportunity":
		return "opportunities"
	case "Case":
		return "cases"
	case "Task":
		return "tasks"
	case "Event":
		return "events"
	case "FeedItem":
		return "feed_items"
	case "ContentDocumentLink":
		return "content_document_links"
	default:
		return sobject
	}
}

func orderedSObjects(records map[string][]json.RawMessage) []string {
	preferred := []string{"Account", "Contact", "Opportunity", "Case", "Task", "Event", "FeedItem", "ContentDocumentLink"}
	seen := map[string]bool{}
	var out []string
	for _, key := range preferred {
		if _, ok := records[key]; ok {
			out = append(out, key)
			seen[key] = true
		}
	}
	for key := range records {
		if !seen[key] {
			out = append(out, key)
		}
	}
	return out
}
