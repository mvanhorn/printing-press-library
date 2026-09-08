package cli

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/keyword-planner/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/keyword-planner/internal/keywordapi"
	"github.com/mvanhorn/printing-press-library/library/marketing/keyword-planner/internal/portfolio"
)

func TestPlannerCollectorFailurePreservesCommittedPages(t *testing.T) {
	for _, test := range []struct {
		name         string
		maxPages     int
		writeFailure bool
		wantCalls    int32
		wantRows     int
		wantReceipts int
	}{
		{"later_page_error", 0, false, 2, 1, 3},
		{"explicit_page_budget", 1, false, 1, 1, 2},
		{"receipt_write_failure", 0, true, 0, 0, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			store, err := portfolio.Open(ctx, filepath.Join(t.TempDir(), "snapshots.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			if test.writeFailure {
				_, err = store.DB().Exec(`CREATE TRIGGER fixture_receipt_failure BEFORE INSERT ON response_receipts BEGIN SELECT RAISE(ABORT, 'fixture persistence failure'); END`)
				if err != nil {
					t.Fatal(err)
				}
			}
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if strings.HasSuffix(request.URL.Path, "/googleAds:search") {
					_, _ = io.WriteString(w, `{"results":[{"customer":{"id":"1234567890","currencyCode":"EUR"}}]}`)
					return
				}
				calls.Add(1)
				var body map[string]json.RawMessage
				if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
					t.Error(err)
					return
				}
				if _, continued := body["pageToken"]; continued {
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = io.WriteString(w, `{"error":{"code":500,"status":"INTERNAL","message":"fixture upstream failure"},"unknownErrorField":true}`)
					return
				}
				_, _ = io.WriteString(w, `{"results":[{"text":"beef steak","keywordIdeaMetrics":{"avgMonthlySearches":"100","competition":"LOW","monthlySearchVolumes":[{"month":"JULY","year":"2026","monthlySearches":"100"}]}}],"nextPageToken":"page-two","unknownResultField":true}`)
			}))
			defer server.Close()
			client := keywordapi.NewClient(keywordapi.Config{
				AccessToken: "test-access-fixture", DeveloperToken: "test-developer-fixture",
				CustomerID: "1234567890", BaseURL: server.URL, HTTPClient: server.Client(),
				RateLockDir: t.TempDir(), MinRequestInterval: time.Nanosecond,
				Limiter: cliutil.NewAdaptiveLimiter(1000), MaxAttempts: 1,
			})
			input := plannerIdeasInput{
				plannerTarget: plannerTarget{CustomerID: "1234567890", Language: "languageConstants/1000",
					GeoTargets: []string{"geoTargetConstants/2840"}, Network: plannerNetworkGoogleSearch,
					Window: plannerWindow{Start: "2026-07", End: "2026-07"}},
				Seeds:    []string{"beef steak", "ribeye", "brisket", "pork chops", "meat cutting", "butchery"},
				PageSize: 10000, MaxPages: test.maxPages, OutputLimit: 1,
			}
			output, err := collectPlannerIdeasWithLogin(ctx, client, store, input, "", time.Now().UTC())
			if err == nil {
				t.Fatal("partial/persistence failure returned success")
			}
			if output.Snapshot.Complete {
				t.Fatal("failure output claimed complete")
			}
			if calls.Load() != test.wantCalls {
				t.Fatalf("Planner calls=%d want%d", calls.Load(), test.wantCalls)
			}
			snapshots, err := store.ListSnapshots(ctx, portfolio.QueryOptions{})
			if err != nil || len(snapshots) != 1 {
				t.Fatalf("snapshot inventory=%v err=%v", snapshots, err)
			}
			if snapshots[0].Complete {
				t.Fatal("stored failed snapshot claimed complete")
			}
			view, err := store.ShowSnapshot(ctx, snapshots[0].ID)
			if err != nil {
				t.Fatal(err)
			}
			if len(view.Rows) != test.wantRows || len(view.Receipts) != test.wantReceipts {
				t.Fatalf("committed evidence rows=%d receipts=%d want%d/%d", len(view.Rows), len(view.Receipts), test.wantRows, test.wantReceipts)
			}
			if test.wantRows > 0 && (view.Rows[0].MonthlySearches == nil || *view.Rows[0].MonthlySearches != 100) {
				t.Fatal("earlier successful page was lost or changed")
			}
			if test.name == "later_page_error" && !strings.Contains(string(view.Receipts[len(view.Receipts)-1].Body), `"unknownErrorField":true`) {
				t.Fatal("non-2xx raw response was not preserved")
			}
		})
	}
}
