package sync

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type HeaderGetClient interface {
	GetWithResponseHeaders(path string, params map[string]string) (json.RawMessage, http.Header, error)
}

type queryPage struct {
	TotalSize      int
	Done           bool
	NextRecordsURL string
	Records        []json.RawMessage
}

func FetchRemainingPages(client HeaderGetClient, first PageRef, gate *Gate) ([]json.RawMessage, error) {
	var records []json.RawMessage
	next := first.NextRecordsURL
	for next != "" {
		if err := gate.Check(); err != nil {
			return records, &PartialSyncError{Stage: "sync.pagination", Err: err}
		}
		body, headers, err := client.GetWithResponseHeaders(normalizeNextRecordsPath(next), nil)
		if err != nil {
			return records, fmt.Errorf("fetch next records for %s: %w", first.SObject, err)
		}
		if err := gate.UpdateFromHeaders(headers); err != nil {
			return records, err
		}
		if err := gate.Check(); err != nil {
			return records, &PartialSyncError{Stage: "sync.pagination", Err: err}
		}
		page, err := parseQueryPage(body)
		if err != nil {
			return records, fmt.Errorf("parse next records for %s: %w", first.SObject, err)
		}
		records = append(records, page.Records...)
		if page.Done {
			break
		}
		next = page.NextRecordsURL
	}
	return records, nil
}

func normalizeNextRecordsPath(next string) string {
	if next == "" {
		return next
	}
	if strings.HasPrefix(next, "/services/data/") {
		return next
	}
	if strings.HasPrefix(next, "/") {
		return "/services/data/" + APIVersion + "/query" + next
	}
	return "/services/data/" + APIVersion + "/query/" + next
}

func parseQueryPage(data json.RawMessage) (queryPage, error) {
	var page queryPage
	var raw struct {
		TotalSize      int               `json:"totalSize"`
		Done           bool              `json:"done"`
		NextRecordsURL string            `json:"nextRecordsUrl"`
		Records        []json.RawMessage `json:"records"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return page, err
	}
	page.TotalSize = raw.TotalSize
	page.Done = raw.Done
	page.NextRecordsURL = raw.NextRecordsURL
	page.Records = raw.Records
	return page, nil
}
