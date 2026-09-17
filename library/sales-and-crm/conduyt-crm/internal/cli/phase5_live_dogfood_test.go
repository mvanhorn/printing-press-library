package cli

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/conduyt-crm/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/conduyt-crm/internal/store"
)

func TestHoistPaginatedEnvelope(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		hoisted bool
	}{
		{"paginated", `{"data":{"data":[{"id":"1"}],"meta":{"page":1,"per_page":50,"total":1}}}`, true},
		{"flat list", `{"data":[{"id":"1"}]}`, false},
		{"single object", `{"data":{"id":"1"}}`, false},
		{"array", `[{"id":"1"}]`, false},
		{"scalar", `42`, false},
		{"inner data object", `{"data":{"data":{},"meta":{"total":1}}}`, false},
		{"missing meta", `{"data":{"data":[]}}`, false},
		{"extra inner key", `{"data":{"data":[],"meta":{},"other":true}}`, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := hoistPaginatedEnvelope(json.RawMessage(tc.input))
			if ok != tc.hoisted {
				t.Fatalf("hoisted=%t, want %t: %s", ok, tc.hoisted, got)
			}
			if !ok && string(got) != tc.input {
				t.Fatalf("changed untouched input: %s", got)
			}
			if ok && compactTestJSON(string(got)) != `{"data":[{"id":"1"}],"meta":{"page":1,"per_page":50,"total":1}}` {
				t.Fatalf("got %s", got)
			}
		})
	}
}

func TestPaginatedLiveReadHoistsRowsMetadataAndCachesRows(t *testing.T) {
	testenv.Isolate(t)
	out, _, err := runNovel(t, []string{"contacts", "list", "--json"}, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/contacts" {
			t.Fatalf("path=%s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"data":{"data":[{"id":"c1","firstName":"Ada"}],"meta":{"page":1,"per_page":50,"total":1}}}`))
	})
	if err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Results []map[string]any `json:"results"`
		Meta    struct {
			Pagination map[string]any `json:"pagination"`
		} `json:"meta"`
	}
	if err := json.Unmarshal([]byte(out), &envelope); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if len(envelope.Results) != 1 || envelope.Results[0]["id"] != "c1" || envelope.Meta.Pagination["total"] != float64(1) || envelope.Meta.Pagination["page"] != float64(1) || envelope.Meta.Pagination["per_page"] != float64(50) {
		t.Fatalf("envelope=%+v", envelope)
	}
	db, err := store.OpenReadOnly(defaultDBPath("conduyt-crm-pp-cli"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows, err := db.List("contacts", 10)
	if err != nil || len(rows) != 1 || !strings.Contains(string(rows[0]), `"id":"c1"`) {
		t.Fatalf("rows=%s err=%v", rows, err)
	}
}

func TestGeneratedQueryFlagsValidateBeforeRequestAndMapParams(t *testing.T) {
	tests := []struct {
		name      string
		good, bad []string
		keys      map[string]string
	}{
		{"automation failures", []string{"automations", "get-failures-drilldown", "--kind", "stale_drip_batches", "--limit", "3", "--offset", "2", "--json"}, []string{"automations", "get-failures-drilldown", "--json"}, map[string]string{"kind": "stale_drip_batches", "limit": "3", "offset": "2"}},
		{"custom fields", []string{"reports", "get-custom-fields", "--entity", "contacts", "--json"}, []string{"reports", "get-custom-fields", "--json"}, map[string]string{"entity": "contacts"}},
		{"funnel", []string{"reports", "get-funnel", "--pipeline-id", "11111111-1111-4111-8111-111111111111", "--json"}, []string{"reports", "get-funnel", "--pipeline-id", "not-a-uuid", "--json"}, map[string]string{"pipelineId": "11111111-1111-4111-8111-111111111111"}},
		{"local presence", []string{"dialer", "get-local-presence-resolve", "--to", "+15555550100", "--json"}, []string{"dialer", "get-local-presence-resolve", "--json"}, map[string]string{"to": "+15555550100"}},
		{"inbound lookup", []string{"dialer", "get-inbound-lookup", "--contact-id", "c1", "--phone", "+15555550100", "--call-sid", "CA1", "--json"}, []string{"dialer", "get-inbound-lookup", "--json"}, map[string]string{"contactId": "c1", "phone": "+15555550100", "callSid": "CA1"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			testenv.Isolate(t)
			var calls atomic.Int32
			_, _, err := runNovel(t, tc.good, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				for key, want := range tc.keys {
					if got := r.URL.Query().Get(key); got != want {
						t.Errorf("%s=%q, want %q", key, got, want)
					}
				}
				_, _ = w.Write([]byte(`{"data":[]}`))
			})
			if err != nil || calls.Load() != 1 {
				t.Fatalf("err=%v calls=%d", err, calls.Load())
			}
			calls.Store(0)
			_, _, err = runNovel(t, tc.bad, func(w http.ResponseWriter, r *http.Request) { calls.Add(1) })
			if ExitCode(err) != 2 || calls.Load() != 0 {
				t.Fatalf("err=%v calls=%d", err, calls.Load())
			}
		})
	}
}
