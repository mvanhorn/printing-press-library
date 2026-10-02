// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/other/cfpb-complaints/internal/store"
)

type cfpbSyncTestClient struct {
	calls []map[string]string
	total int
}

func (c *cfpbSyncTestClient) Get(_ context.Context, _ string, params map[string]string) (json.RawMessage, error) {
	call := make(map[string]string, len(params))
	for key, value := range params {
		call[key] = value
	}
	c.calls = append(c.calls, call)

	pageSize, err := strconv.Atoi(params["size"])
	if err != nil || pageSize <= 0 {
		return nil, fmt.Errorf("invalid requested size %q", params["size"])
	}
	start, _ := strconv.Atoi(params["from"])
	total := c.total
	if total == 0 {
		total = 1001
	}
	count := pageSize
	if remaining := total - start; remaining < count {
		count = remaining
	}
	if count < 0 {
		count = 0
	}
	hits := make([]map[string]any, count)
	for i := range hits {
		id := fmt.Sprintf("complaint-%d", start+i)
		hits[i] = map[string]any{
			"_id": id,
			"_source": map[string]any{
				"complaint_id": id,
				"product":      "Credit card",
			},
		}
	}
	data, err := json.Marshal(map[string]any{
		"hits": map[string]any{
			"total": map[string]any{"value": total, "relation": "eq"},
			"hits":  hits,
		},
	})
	return data, err
}

func TestSyncDataResearchHonorsCustomPageSize(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()

	client := &cfpbSyncTestClient{total: 501}
	params := &syncUserParams{flatGlobal: map[string]string{"size": "500"}}
	result := syncResource(
		context.Background(), client, db, "data-research", "", true,
		0, false, false, params, io.Discard,
	)
	if result.Err != nil || result.Warn != nil {
		t.Fatalf("sync result: err=%v warn=%v", result.Err, result.Warn)
	}
	if result.Count != 501 {
		t.Fatalf("stored count = %d, want 501", result.Count)
	}
	if len(client.calls) != 2 {
		t.Fatalf("request count = %d, want 2", len(client.calls))
	}
	if got := client.calls[0]["size"]; got != "500" {
		t.Fatalf("first size = %q, want 500", got)
	}
	if got := client.calls[1]["from"]; got != "500" {
		t.Fatalf("second from = %q, want 500", got)
	}
}

func (*cfpbSyncTestClient) RateLimit() float64 { return 0 }

func TestSyncDataResearchExtractsHitsAndPaginatesByOffset(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()

	client := &cfpbSyncTestClient{}
	result := syncResource(
		context.Background(), client, db, "data-research", "", true,
		0, false, false, &syncUserParams{}, io.Discard,
	)
	if result.Err != nil || result.Warn != nil {
		t.Fatalf("sync result: err=%v warn=%v", result.Err, result.Warn)
	}
	if result.Count != 1001 {
		t.Fatalf("stored count = %d, want 1001", result.Count)
	}
	if len(client.calls) != 2 {
		t.Fatalf("request count = %d, want 2", len(client.calls))
	}
	if got := client.calls[0]["size"]; got != "1000" {
		t.Fatalf("first size = %q, want 1000", got)
	}
	if _, ok := client.calls[0]["from"]; ok {
		t.Fatalf("first request unexpectedly set from=%q", client.calls[0]["from"])
	}
	if got := client.calls[1]["from"]; got != "1000" {
		t.Fatalf("second from = %q, want 1000", got)
	}

	stored, err := db.Count("data-research")
	if err != nil {
		t.Fatalf("count stored resources: %v", err)
	}
	if stored != 1001 {
		t.Fatalf("database count = %d, want 1001 complaint hits", stored)
	}
	var envelopes int
	if err := db.DB().QueryRow(
		`SELECT COUNT(*) FROM resources WHERE resource_type = ? AND id = ?`,
		"data-research", "data-research",
	).Scan(&envelopes); err != nil {
		t.Fatalf("count envelope rows: %v", err)
	}
	if envelopes != 0 {
		t.Fatalf("stored %d outer response envelopes, want 0", envelopes)
	}
}
