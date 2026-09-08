// Copyright 2026 Max Michel and contributors. Licensed under Apache-2.0. See LICENSE.

package portfolio

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestReceiptEndpointMustMatchSnapshot(t *testing.T) {
	for _, endpoint := range []string{EndpointIdeas, EndpointHistorical} {
		t.Run(endpoint, func(t *testing.T) {
			store := newTestStore(t)
			ctx := context.Background()
			snapshot, err := store.StartSnapshot(ctx, testSnapshotInput(endpoint), time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC))
			if err != nil {
				t.Fatal(err)
			}
			other := EndpointIdeas
			if endpoint == EndpointIdeas {
				other = EndpointHistorical
			}
			if _, err := store.AppendReceipt(ctx, ReceiptInput{SnapshotID: snapshot.ID, Endpoint: other, HTTPStatus: 200, Body: []byte(`{"results":[]}`)}); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("mismatched receipt error = %v, want invalid input", err)
			}
			var count int
			if err := store.db.QueryRowContext(ctx, `SELECT count(*) FROM response_receipts WHERE snapshot_id = ?`, snapshot.ID).Scan(&count); err != nil {
				t.Fatal(err)
			}
			if count != 0 {
				t.Fatalf("rejected receipt persisted: %d rows", count)
			}
			for attempt, supplied := range []string{"", endpoint, EndpointAccount} {
				ids, err := store.AppendReceipt(ctx, ReceiptInput{SnapshotID: snapshot.ID, Endpoint: supplied, Attempt: attempt, HTTPStatus: 200, Body: []byte(`{"results":[]}`)})
				if err != nil {
					t.Fatalf("append %q: %v", supplied, err)
				}
				var actual string
				if err := store.db.QueryRowContext(ctx, `SELECT endpoint FROM response_receipts WHERE id = ?`, ids.ReceiptID).Scan(&actual); err != nil {
					t.Fatal(err)
				}
				want := supplied
				if want == "" {
					want = endpoint
				}
				if actual != want {
					t.Fatalf("receipt endpoint = %q, want %q", actual, want)
				}
			}
		})
	}
}
