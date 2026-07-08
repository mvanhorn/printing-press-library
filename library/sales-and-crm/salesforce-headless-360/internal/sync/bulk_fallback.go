package sync

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const BulkUnsafeWarning = "WARNING: Bulk API fallback is FLS-unsafe until the Unit D Apex companion is installed"

type BulkClient interface {
	PostWithResponseHeaders(path string, body any) (json.RawMessage, int, http.Header, error)
	GetWithResponseHeaders(path string, params map[string]string) (json.RawMessage, http.Header, error)
}

type BulkResult struct {
	Records    []json.RawMessage
	Provenance map[string]any
}

func RunBulkFallback(c BulkClient, objectName, soql string, allowUnsafe bool, log io.Writer) (*BulkResult, error) {
	if !allowUnsafe {
		return nil, fmt.Errorf("single-object pull for %s exceeds Composite Graph threshold; rerun with --allow-bulk-fls-unsafe only if you accept Bulk API 2.0 FLS risk, or install the Unit D Apex companion", objectName)
	}
	if log != nil {
		fmt.Fprintln(log, BulkUnsafeWarning)
	}

	body, _, _, err := c.PostWithResponseHeaders("/services/data/"+APIVersion+"/jobs/query", map[string]any{
		"operation":   "query",
		"query":       soql,
		"contentType": "JSON",
	})
	if err != nil {
		return nil, fmt.Errorf("create bulk query job: %w", err)
	}
	var job struct {
		ID    string `json:"id"`
		State string `json:"state"`
	}
	if err := json.Unmarshal(body, &job); err != nil {
		return nil, fmt.Errorf("parse bulk query job: %w", err)
	}
	if job.ID == "" {
		return nil, fmt.Errorf("bulk query job response missing id")
	}

	wait := time.Second
	for job.State != "JobComplete" {
		if job.State == "Failed" || job.State == "Aborted" {
			return nil, fmt.Errorf("bulk query job %s ended in state %s", job.ID, job.State)
		}
		time.Sleep(wait)
		if wait < 15*time.Second {
			wait *= 2
			if wait > 15*time.Second {
				wait = 15 * time.Second
			}
		}
		body, _, err = c.GetWithResponseHeaders("/services/data/"+APIVersion+"/jobs/query/"+job.ID, nil)
		if err != nil {
			return nil, fmt.Errorf("poll bulk query job %s: %w", job.ID, err)
		}
		if err := json.Unmarshal(body, &job); err != nil {
			return nil, fmt.Errorf("parse bulk query job poll: %w", err)
		}
	}

	body, _, err = c.GetWithResponseHeaders("/services/data/"+APIVersion+"/jobs/query/"+job.ID+"/results", nil)
	if err != nil {
		return nil, fmt.Errorf("fetch bulk query job results: %w", err)
	}
	var records []json.RawMessage
	if err := json.Unmarshal(body, &records); err != nil {
		records = []json.RawMessage{body}
	}
	return &BulkResult{
		Records: records,
		Provenance: map[string]any{
			"source":             "bulk_api_2_0",
			"fls_unsafe":         true,
			"warning":            BulkUnsafeWarning,
			"unit_d_apex_needed": true,
		},
	}, nil
}
