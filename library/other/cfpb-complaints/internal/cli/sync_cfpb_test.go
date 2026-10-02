// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
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

func TestSyncDataResearchStartingOffsetAdvances(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	c := &cfpbSyncTestClient{total: 1001}
	params := &syncUserParams{flatGlobal: map[string]string{"size": "500", "from": "500"}}
	result := syncResource(context.Background(), c, db, "data-research", "", true, 0, false, false, params, io.Discard)
	if result.Err != nil || result.Warn != nil || result.Count != 501 {
		t.Fatalf("offset sync did not reach the last page: %+v", result)
	}
	if len(c.calls) != 2 || c.calls[0]["from"] != "500" || c.calls[1]["from"] != "1000" {
		t.Fatalf("starting offset was repeated or did not advance: %v", c.calls)
	}
}

func TestSyncDataResearchSavedOffsetWinsWhenResuming(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	c := &cfpbSyncTestClient{total: 1501}
	params := &syncUserParams{flatGlobal: map[string]string{"size": "500", "from": "500"}}
	first := syncResource(context.Background(), c, db, "data-research", "", false, 1, false, false, params, io.Discard)
	if first.Err != nil || first.Count != 500 {
		t.Fatalf("first capped sync failed: %+v", first)
	}
	second := syncResource(context.Background(), c, db, "data-research", "", false, 0, false, false, params, io.Discard)
	if second.Err != nil || second.Count != 501 {
		t.Fatalf("resume did not complete remaining pages: %+v", second)
	}
	if len(c.calls) != 3 || c.calls[0]["from"] != "500" || c.calls[1]["from"] != "1000" || c.calls[2]["from"] != "1500" {
		t.Fatalf("saved cursor was overwritten by repeated starting offset: %v", c.calls)
	}
}

func TestSyncDataResearchRejectsInvalidPaginationBeforeFetch(t *testing.T) {
	for _, tc := range []struct {
		name, key, value string
	}{
		{"zero size", "size", "0"},
		{"negative size", "size", "-1"},
		{"non-numeric size", "size", "many"},
		{"oversized page", "size", "1001"},
		{"negative offset", "from", "-1"},
		{"non-numeric offset", "from", "later"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			c := &cfpbSyncTestClient{total: 1}
			result := syncResource(context.Background(), c, db, "data-research", "", true, 0, false, false,
				&syncUserParams{flatGlobal: map[string]string{tc.key: tc.value}}, io.Discard)
			if result.Err == nil || len(c.calls) != 0 {
				t.Fatalf("invalid pagination reached API: err=%v calls=%v", result.Err, c.calls)
			}
		})
	}
}

func TestSyncDataResearchRemovesOnlyLegacyEnvelopeAfterCompleteSync(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	const resource = "data-research"
	legacy := []byte(`{"hits":{"hits":[{"_id":"complaint-0","_source":{"complaint_id":"complaint-0"}}]}}`)
	if err := db.Upsert(resource, resource, legacy); err != nil {
		t.Fatal(err)
	}
	c := &cfpbSyncTestClient{total: 1001}
	partial := syncResource(context.Background(), c, db, resource, "", true, 1, false, false, &syncUserParams{}, io.Discard)
	if partial.Err != nil {
		t.Fatal(partial.Err)
	}
	if _, err := db.Get(resource, resource); err != nil {
		t.Fatalf("incomplete sync removed legacy envelope: %v", err)
	}
	// The CLI clears sync state before --full; this direct helper test does it here.
	if err := db.SaveSyncState(resource, "", 0); err != nil {
		t.Fatal(err)
	}
	complete := syncResource(context.Background(), c, db, resource, "", true, 0, false, false, &syncUserParams{}, io.Discard)
	if complete.Err != nil || complete.Count != 1001 {
		t.Fatalf("replacement sync failed: %+v", complete)
	}
	if _, err := db.Get(resource, resource); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("legacy envelope remains after complete sync: %v", err)
	}
	if count, err := db.Count(resource); err != nil || count != 1001 {
		t.Fatalf("stored complaint count includes legacy envelope: count=%d err=%v", count, err)
	}
	// A real complaint with the same ID must not be removed by cleanup.
	if err := db.Upsert(resource, resource, []byte(`{"_id":"data-research","_source":{"complaint_id":"data-research"}}`)); err != nil {
		t.Fatal(err)
	}
	if err := db.DeleteLegacyCFPBEnvelopeIfCovered(); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Get(resource, resource); err != nil {
		t.Fatalf("cleanup removed a non-envelope complaint: %v", err)
	}
}

func TestSyncDataResearchPartialRangeKeepsLegacyEnvelope(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	const resource = "data-research"
	legacy := []byte(`{"hits":{"hits":[{"_id":"complaint-0","_source":{"complaint_id":"complaint-0"}}]}}`)
	if err := db.Upsert(resource, resource, legacy); err != nil {
		t.Fatal(err)
	}
	c := &cfpbSyncTestClient{total: 1001}
	params := &syncUserParams{flatGlobal: map[string]string{"size": "500", "from": "500"}}
	result := syncResource(context.Background(), c, db, resource, "", true, 0, false, false, params, io.Discard)
	if result.Err != nil || result.Count != 501 {
		t.Fatalf("partial-range sync failed: %+v", result)
	}
	if _, err := db.Get(resource, resource); err != nil {
		t.Fatalf("partial-range sync removed legacy copy of earlier complaints: %v", err)
	}
}

func TestSyncDataResearchResumedFullScanRemovesCoveredLegacyEnvelope(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	const resource = "data-research"
	legacy := []byte(`{"hits":{"hits":[{"_id":"complaint-0","_source":{"complaint_id":"complaint-0"}}]}}`)
	if err := db.Upsert(resource, resource, legacy); err != nil {
		t.Fatal(err)
	}
	c := &cfpbSyncTestClient{total: 1001}
	first := syncResource(context.Background(), c, db, resource, "", false, 1, false, false, &syncUserParams{}, io.Discard)
	if first.Err != nil || first.Count != 1000 {
		t.Fatalf("first capped run failed: %+v", first)
	}
	if _, err := db.Get(resource, resource); err != nil {
		t.Fatalf("capped run removed legacy envelope: %v", err)
	}
	second := syncResource(context.Background(), c, db, resource, "", false, 0, false, false, &syncUserParams{}, io.Discard)
	if second.Err != nil || second.Count != 1 {
		t.Fatalf("resume failed: %+v", second)
	}
	if _, err := db.Get(resource, resource); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("covered legacy envelope remained after resumed scan: %v", err)
	}
}
