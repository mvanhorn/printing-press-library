// Copyright 2026 Matthew Vassallo and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/woot/internal/store"
)

type fakeWootDealsSyncClient struct {
	t          *testing.T
	seenSkips  []int
	emptySkips map[int]bool
	totalHits  int
}

func (c *fakeWootDealsSyncClient) Get(_ context.Context, _ string, params map[string]string) (json.RawMessage, error) {
	c.t.Helper()
	matches := regexp.MustCompile(`Limit:(\d+), Skip:(\d+)`).FindStringSubmatch(params["query"])
	if matches == nil {
		return nil, fmt.Errorf("query missing Limit/Skip: %s", params["query"])
	}
	limit, _ := strconv.Atoi(matches[1])
	skip, _ := strconv.Atoi(matches[2])
	c.seenSkips = append(c.seenSkips, skip)
	count := limit
	if c.emptySkips[skip] {
		count = 0
	} else if skip == 200 {
		count = 50
	}
	offers := make([]map[string]any, 0, count)
	for i := 0; i < count; i++ {
		id := fmt.Sprintf("offer-%d", skip+i)
		offers = append(offers, map[string]any{
			"Id": id, "Title": id, "Slug": id,
			"Items": []map[string]any{{"SalePrice": 9.99}},
		})
	}
	data, err := json.Marshal(map[string]any{"data": map[string]any{"searchOffers": map[string]any{
		"Offers": offers, "TotalHits": c.totalHits,
	}}})
	return data, err
}

func (*fakeWootDealsSyncClient) RateLimit() float64 { return 2 }

type rawWootDealsSyncClient struct {
	data json.RawMessage
}

func (c *rawWootDealsSyncClient) Get(context.Context, string, map[string]string) (json.RawMessage, error) {
	return c.data, nil
}

func (*rawWootDealsSyncClient) RateLimit() float64 { return 2 }

type failAtSkipWootDealsSyncClient struct {
	base     *fakeWootDealsSyncClient
	failSkip int
}

func (c *failAtSkipWootDealsSyncClient) Get(ctx context.Context, path string, params map[string]string) (json.RawMessage, error) {
	matches := regexp.MustCompile(`Skip:(\d+)`).FindStringSubmatch(params["query"])
	if len(matches) == 2 {
		skip, _ := strconv.Atoi(matches[1])
		if skip == c.failSkip {
			return nil, errors.New("forced page fetch failure")
		}
	}
	return c.base.Get(ctx, path, params)
}

func (*failAtSkipWootDealsSyncClient) RateLimit() float64 { return 2 }

type changingWootDealsSyncClient struct {
	calls int
}

func (c *changingWootDealsSyncClient) Get(context.Context, string, map[string]string) (json.RawMessage, error) {
	ids := []string{"offer-a", "offer-b"}
	if c.calls > 0 {
		ids = []string{"offer-b", "offer-c"}
	}
	c.calls++
	offers := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		offers = append(offers, map[string]any{
			"Id": id, "Title": id, "Slug": id,
			"Items": []map[string]any{{"SalePrice": 9.99}},
		})
	}
	return json.Marshal(map[string]any{"data": map[string]any{"searchOffers": map[string]any{
		"Offers": offers, "TotalHits": len(offers),
	}}})
}

func (*changingWootDealsSyncClient) RateLimit() float64 { return 2 }

type failingSyncStateStore struct {
	*store.Store
	failCursor string
}

func (s *failingSyncStateStore) SaveSyncState(resource, cursor string, count int) error {
	if cursor == s.failCursor {
		return errors.New("forced sync-state failure")
	}
	return s.Store.SaveSyncState(resource, cursor, count)
}

func (s *failingSyncStateStore) UpsertBatchWithSyncState(resource string, items []json.RawMessage, cursor string) (int, int, int, error) {
	if cursor == s.failCursor {
		return 0, 0, 0, errors.New("forced sync-state failure")
	}
	return s.Store.UpsertBatchWithSyncState(resource, items, cursor)
}

func TestSyncWootDealsPersistsPagesAcrossEmptyGapWithoutPruningStaleRows(t *testing.T) {
	t.Parallel()
	db, err := store.Open(t.TempDir() + "/data.db")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()
	if err := db.Upsert("deals", "stale", json.RawMessage(`{"id":"stale","title":"Expired offer"}`)); err != nil {
		t.Fatalf("seed stale deal: %v", err)
	}

	client := &fakeWootDealsSyncClient{t: t, emptySkips: map[int]bool{100: true}, totalHits: 250}
	var events bytes.Buffer
	result := syncWootDeals(context.Background(), client, db, "", true, 0, false, true, nil, &events)
	if result.Err != nil || result.Warn == nil {
		t.Fatalf("syncWootDeals result = %+v, want incomplete-snapshot warning", result)
	}
	if result.Count != 150 {
		t.Fatalf("synced count = %d, want 150", result.Count)
	}
	if got, want := fmt.Sprint(client.seenSkips), "[0 100 200]"; got != want {
		t.Fatalf("seen skips = %s, want %s", got, want)
	}
	count, err := db.Count("deals")
	if err != nil {
		t.Fatalf("count deals: %v", err)
	}
	if count != 151 {
		t.Fatalf("stored deals = %d, want 151 including preserved stale row", count)
	}
	if !strings.Contains(events.String(), `"reason":"incomplete-snapshot"`) {
		t.Fatalf("missing reconcile skip event: %s", events.String())
	}
	cursor, _, _, err := db.GetSyncState("deals")
	if err != nil {
		t.Fatalf("read incomplete sync state: %v", err)
	}
	if cursor != "0" || !strings.Contains(events.String(), `"reason":"snapshot_incomplete"`) {
		t.Fatalf("incomplete snapshot cursor=%q events=%s", cursor, events.String())
	}
	if _, err := db.Get("deals", "stale"); err != nil {
		t.Fatalf("stale row should be preserved after incomplete snapshot: %v", err)
	}
	results, err := db.Search("offer", 10, "deals")
	if err != nil {
		t.Fatalf("search deals: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("synced deal was not searchable")
	}
}

func TestSyncWootDealsPrunesOnlyCompleteSnapshot(t *testing.T) {
	t.Parallel()
	db, err := store.Open(t.TempDir() + "/data.db")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()
	if err := db.Upsert("deals", "stale", json.RawMessage(`{"id":"stale","title":"Expired offer"}`)); err != nil {
		t.Fatalf("seed stale deal: %v", err)
	}

	client := &fakeWootDealsSyncClient{t: t, totalHits: 250}
	result := syncWootDeals(context.Background(), client, db, "", true, 0, false, true, nil, io.Discard)
	if result.Err != nil || result.Warn != nil || result.Count != 250 {
		t.Fatalf("sync result = %+v", result)
	}
	if got, want := fmt.Sprint(client.seenSkips), "[0 100 200 0 100 200]"; got != want {
		t.Fatalf("seen skips = %s, want two complete passes %s", got, want)
	}
	count, err := db.Count("deals")
	if err != nil {
		t.Fatalf("count deals: %v", err)
	}
	if count != 250 {
		t.Fatalf("stored deals = %d, want 250", count)
	}
	if _, err := db.Get("deals", "stale"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("stale row lookup error = %v, want not found", err)
	}
	cursor, _, savedCount, err := db.GetSyncState("deals")
	if err != nil {
		t.Fatalf("read reconciled sync state: %v", err)
	}
	if cursor != "" || savedCount != 250 {
		t.Fatalf("reconciled sync state = cursor %q count %d, want empty cursor and 250", cursor, savedCount)
	}
}

func TestSyncWootDealsWithoutPruneKeepsStaleStoreIncomplete(t *testing.T) {
	t.Parallel()
	db, err := store.Open(t.TempDir() + "/data.db")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()
	if err := db.Upsert("deals", "stale", json.RawMessage(`{"id":"stale","title":"Expired offer"}`)); err != nil {
		t.Fatalf("seed stale deal: %v", err)
	}

	client := &rawWootDealsSyncClient{data: json.RawMessage(`{"data":{"searchOffers":{"Offers":[{"Id":"offer-a","Title":"A","Slug":"a"},{"Id":"offer-b","Title":"B","Slug":"b"}],"TotalHits":2}}}`)}
	var events bytes.Buffer
	result := syncWootDeals(context.Background(), client, db, "", true, 0, false, false, nil, &events)
	if result.Err != nil || result.Warn == nil {
		t.Fatalf("sync result = %+v, want stale-row warning", result)
	}
	cursor, _, savedCount, err := db.GetSyncState("deals")
	if err != nil {
		t.Fatalf("read unreconciled sync state: %v", err)
	}
	if cursor != "0" || savedCount != 3 {
		t.Fatalf("unreconciled state = cursor %q count %d, want restart marker 0 and 3 rows", cursor, savedCount)
	}
	incomplete, err := db.HasIncompleteSyncContext(context.Background())
	if err != nil {
		t.Fatalf("read no-prune incomplete context: %v", err)
	}
	if !incomplete {
		t.Fatal("stale no-prune store was published as ready")
	}
	if !strings.Contains(events.String(), `"catalog_verified":true`) || !strings.Contains(events.String(), `"store_ready":false`) {
		t.Fatalf("sync events do not distinguish verified catalog from stale local store: %s", events.String())
	}
	if _, err := db.Get("deals", "stale"); err != nil {
		t.Fatalf("--no-prune should retain stale row: %v", err)
	}
}

func TestSyncWootDealsDoesNotPruneWhenEqualSizedSnapshotsChange(t *testing.T) {
	t.Parallel()
	db, err := store.Open(t.TempDir() + "/data.db")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()
	if err := db.Upsert("deals", "offer-c", json.RawMessage(`{"id":"offer-c","title":"Existing offer C"}`)); err != nil {
		t.Fatalf("seed existing deal: %v", err)
	}

	client := &changingWootDealsSyncClient{}
	var events bytes.Buffer
	result := syncWootDeals(context.Background(), client, db, "", true, 0, false, true, nil, &events)
	if result.Err != nil || result.Warn == nil {
		t.Fatalf("sync result = %+v, want changing-snapshot warning", result)
	}
	if !strings.Contains(events.String(), `"reason":"snapshot_changed_during_sync"`) {
		t.Fatalf("missing changing-snapshot event: %s", events.String())
	}
	if _, err := db.Get("deals", "offer-c"); err != nil {
		t.Fatalf("existing row was pruned from a changing snapshot: %v", err)
	}
	cursor, _, _, err := db.GetSyncState("deals")
	if err != nil {
		t.Fatalf("read sync state: %v", err)
	}
	if cursor != "0" {
		t.Fatalf("changing snapshot cursor = %q, want restart marker 0", cursor)
	}
}

func TestSyncWootDealsResumesAfterPageCap(t *testing.T) {
	t.Parallel()
	db, err := store.Open(t.TempDir() + "/data.db")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()

	client := &fakeWootDealsSyncClient{t: t, totalHits: 250}
	var events bytes.Buffer
	first := syncWootDeals(context.Background(), client, db, "", false, 1, false, false, nil, &events)
	if first.Err != nil || first.Warn != nil || first.Count != 100 {
		t.Fatalf("first sync result = %+v, want 100 successful rows", first)
	}
	cursor, _, _, err := db.GetSyncState("deals")
	if err != nil {
		t.Fatalf("get capped sync state: %v", err)
	}
	if cursor != "100" {
		t.Fatalf("saved cursor = %q, want 100", cursor)
	}
	if !strings.Contains(events.String(), `"reason":"max_pages_cap_hit"`) {
		t.Fatalf("cap warning missing from events: %s", events.String())
	}
	for _, line := range strings.Split(strings.TrimSpace(events.String()), "\n") {
		if !json.Valid([]byte(line)) {
			t.Fatalf("sync event is not valid JSON: %s", line)
		}
	}

	second := syncWootDeals(context.Background(), client, db, "", false, 0, false, false, nil, io.Discard)
	if second.Err != nil || second.Warn == nil || second.Count != 150 {
		t.Fatalf("resumed sync result = %+v, want 150 rows plus restart warning", second)
	}
	if got, want := fmt.Sprint(client.seenSkips), "[0 100 200]"; got != want {
		t.Fatalf("seen skips = %s, want %s", got, want)
	}
	cursor, _, savedCount, err := db.GetSyncState("deals")
	if err != nil {
		t.Fatalf("get completed sync state: %v", err)
	}
	if cursor != "0" {
		t.Fatalf("resumed cursor = %q, want catalog-head restart marker 0", cursor)
	}
	if savedCount != 250 {
		t.Fatalf("completed checkpoint count = %d, want 250", savedCount)
	}
	count, err := db.Count("deals")
	if err != nil {
		t.Fatalf("count deals: %v", err)
	}
	if count != 250 {
		t.Fatalf("stored deals = %d, want 250", count)
	}

	client.seenSkips = nil
	third := syncWootDeals(context.Background(), client, db, "", false, 0, false, false, nil, io.Discard)
	if third.Err != nil || third.Warn != nil || third.Count != 250 {
		t.Fatalf("head restart result = %+v, want verified 250-row snapshot", third)
	}
	if got, want := fmt.Sprint(client.seenSkips), "[0 100 200 0 100 200]"; got != want {
		t.Fatalf("head restart skips = %s, want two complete passes %s", got, want)
	}
	cursor, _, _, err = db.GetSyncState("deals")
	if err != nil {
		t.Fatalf("get verified sync state: %v", err)
	}
	if cursor != "" {
		t.Fatalf("verified cursor = %q, want empty", cursor)
	}
}

func TestSyncWootDealsResumesAfterCappedEmptyFirstPage(t *testing.T) {
	t.Parallel()
	db, err := store.Open(t.TempDir() + "/data.db")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()

	client := &fakeWootDealsSyncClient{t: t, emptySkips: map[int]bool{0: true}, totalHits: 250}
	first := syncWootDeals(context.Background(), client, db, "", false, 1, false, false, nil, io.Discard)
	if first.Err != nil || first.Count != 0 {
		t.Fatalf("first sync result = %+v, want capped empty page", first)
	}
	cursor, _, _, err := db.GetSyncState("deals")
	if err != nil {
		t.Fatalf("read sync state: %v", err)
	}
	if cursor != "100" {
		t.Fatalf("saved cursor = %q, want 100", cursor)
	}

	second := syncWootDeals(context.Background(), client, db, "", false, 0, false, false, nil, io.Discard)
	if second.Err != nil || second.Warn == nil || second.Count != 150 {
		t.Fatalf("second sync result = %+v, want 150 rows and restart warning after resume", second)
	}
	if got, want := fmt.Sprint(client.seenSkips), "[0 100 200]"; got != want {
		t.Fatalf("seen skips = %s, want %s", got, want)
	}
	cursor, _, _, err = db.GetSyncState("deals")
	if err != nil {
		t.Fatalf("read completed gap state: %v", err)
	}
	if cursor != "0" {
		t.Fatalf("incomplete resumed cursor = %q, want restart marker 0", cursor)
	}
}

func TestSyncWootDealsStopsOnEmptyZeroTotalPage(t *testing.T) {
	t.Parallel()
	db, err := store.Open(t.TempDir() + "/data.db")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()

	client := &fakeWootDealsSyncClient{t: t, emptySkips: map[int]bool{0: true}}
	result := syncWootDeals(context.Background(), client, db, "", true, 0, false, true, nil, io.Discard)
	if result.Err != nil || result.Warn != nil || result.Count != 0 {
		t.Fatalf("sync result = %+v, want an empty success", result)
	}
	if got, want := fmt.Sprint(client.seenSkips), "[0 0]"; got != want {
		t.Fatalf("seen skips = %s, want %s", got, want)
	}
}

func TestSyncWootDealsRejectsQueryOverride(t *testing.T) {
	t.Parallel()
	db, err := store.Open(t.TempDir() + "/data.db")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()

	client := &fakeWootDealsSyncClient{t: t, totalHits: 1}
	params := &syncUserParams{flatGlobal: map[string]string{"query": "mutation Bad { nope }"}}
	result := syncWootDeals(context.Background(), client, db, "", true, 0, false, false, params, io.Discard)
	if result.Err == nil || !strings.Contains(result.Err.Error(), "query parameter is reserved") {
		t.Fatalf("sync error = %v, want reserved query rejection", result.Err)
	}
	if len(client.seenSkips) != 0 {
		t.Fatalf("client received %d request(s), want zero", len(client.seenSkips))
	}
}

func TestSyncWootDealsRejectsGraphQLErrorsWithoutPruning(t *testing.T) {
	t.Parallel()
	db, err := store.Open(t.TempDir() + "/data.db")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()
	if err := db.Upsert("deals", "existing", json.RawMessage(`{"id":"existing","title":"Existing"}`)); err != nil {
		t.Fatalf("seed existing deal: %v", err)
	}

	client := &rawWootDealsSyncClient{data: json.RawMessage(`{"errors":[{"message":"unauthorized field"}]}`)}
	result := syncWootDeals(context.Background(), client, db, "", true, 0, false, true, nil, io.Discard)
	if result.Err == nil || !strings.Contains(result.Err.Error(), "GraphQL returned errors") {
		t.Fatalf("sync error = %v, want GraphQL envelope rejection", result.Err)
	}
	count, err := db.Count("deals")
	if err != nil {
		t.Fatalf("count deals: %v", err)
	}
	if count != 1 {
		t.Fatalf("stored deals = %d, want existing row preserved", count)
	}
}

func TestSyncWootDealsLaterPageFailureLeavesIncompleteMarker(t *testing.T) {
	t.Parallel()
	db, err := store.Open(t.TempDir() + "/data.db")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()
	base := &fakeWootDealsSyncClient{t: t, totalHits: 250}
	client := &failAtSkipWootDealsSyncClient{base: base, failSkip: 100}

	result := syncWootDeals(context.Background(), client, db, "", true, 0, false, true, nil, io.Discard)
	if result.Err == nil || !strings.Contains(result.Err.Error(), "forced page fetch failure") {
		t.Fatalf("sync result = %+v, want later-page failure", result)
	}
	cursor, _, count, err := db.GetSyncState("deals")
	if err != nil {
		t.Fatalf("read sync state: %v", err)
	}
	if cursor != "100" || count != 100 {
		t.Fatalf("failed-page state = cursor %q count %d, want 100 and 100", cursor, count)
	}
	incomplete, err := db.HasIncompleteSyncContext(context.Background())
	if err != nil {
		t.Fatalf("read incomplete context: %v", err)
	}
	if !incomplete {
		t.Fatal("later-page failure published the partial store as complete")
	}
}

func TestSyncWootDealsRejectsIncompleteGraphQLEnvelopesWithoutPruning(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		body    string
		wantErr string
	}{
		{name: "missing offers", body: `{"data":{"searchOffers":{"TotalHits":0}}}`, wantErr: "missing data.searchOffers.Offers"},
		{name: "missing total hits", body: `{"data":{"searchOffers":{"Offers":[]}}}`, wantErr: "missing data.searchOffers.TotalHits"},
		{name: "missing data", body: `{}`, wantErr: "missing usable data"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			db, err := store.Open(t.TempDir() + "/data.db")
			if err != nil {
				t.Fatalf("open store: %v", err)
			}
			defer db.Close()
			if err := db.Upsert("deals", "existing", json.RawMessage(`{"id":"existing","title":"Existing"}`)); err != nil {
				t.Fatalf("seed existing deal: %v", err)
			}

			client := &rawWootDealsSyncClient{data: json.RawMessage(tc.body)}
			result := syncWootDeals(context.Background(), client, db, "", true, 0, false, true, nil, io.Discard)
			if result.Err == nil || !strings.Contains(result.Err.Error(), tc.wantErr) {
				t.Fatalf("sync error = %v, want %q", result.Err, tc.wantErr)
			}
			count, err := db.Count("deals")
			if err != nil {
				t.Fatalf("count deals: %v", err)
			}
			if count != 1 {
				t.Fatalf("stored deals = %d, want existing row preserved", count)
			}
		})
	}
}

func TestSyncWootDealsSurfacesCheckpointFailures(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		failCursor string
		want       string
	}{
		{name: "page checkpoint", failCursor: "100", want: "storing deals with sync checkpoint"},
		{name: "final state", failCursor: "", want: "saving final deals sync state"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, err := store.Open(t.TempDir() + "/data.db")
			if err != nil {
				t.Fatalf("open store: %v", err)
			}
			defer db.Close()
			wrapped := &failingSyncStateStore{Store: db, failCursor: tc.failCursor}
			client := &fakeWootDealsSyncClient{t: t, totalHits: 100}
			result := syncWootDeals(context.Background(), client, wrapped, "", true, 0, false, false, nil, io.Discard)
			if result.Err == nil || !strings.Contains(result.Err.Error(), tc.want) {
				t.Fatalf("sync error = %v, want %q", result.Err, tc.want)
			}
		})
	}
}
