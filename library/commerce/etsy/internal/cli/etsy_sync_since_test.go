package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"
)

// etsy-ext: Inspect actual outgoing parameters rather than only mapping helpers.
type sinceSyncFake struct{ params map[string]string }

func (f *sinceSyncFake) RateLimit() float64 { return 100000 }
func (f *sinceSyncFake) Get(_ context.Context, _ string, p map[string]string) (json.RawMessage, error) {
	f.params = p
	return json.RawMessage(`{"results":[],"count":0}`), nil
}
func TestEtsySyncSinceRequests(t *testing.T) {
	_, db := listingStatesFixture(t)
	ts := time.Now().UTC().Add(-7 * 24 * time.Hour).Truncate(time.Second)
	for _, resource := range []string{"receipts", "reviews", "ledger_entries"} {
		f := &sinceSyncFake{}
		var events bytes.Buffer
		r := syncResource(context.Background(), f, db, resource, ts.Format(time.RFC3339Nano), true, 0, false, true, nil, &events)
		if r.Err != nil {
			t.Fatal(r.Err)
		}
		if f.params[syncResourceSinceParam(resource)] != strconv.FormatInt(ts.Unix(), 10) {
			t.Fatalf("%s: %v", resource, f.params)
		}
		if strings.Contains(events.String(), "resource_not_incremental") {
			t.Fatal(events.String())
		}
	}
	if got := formatSyncSinceValue("invalid", "unix"); got != "invalid" {
		t.Fatal(got)
	}
}
