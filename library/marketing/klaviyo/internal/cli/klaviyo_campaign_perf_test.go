// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/store"
	"github.com/spf13/cobra"
)

type campaignPerfFixture struct {
	t             *testing.T
	messages      []map[string]any
	counts        map[string]int
	lookups       [][]string
	pageSizes     []string
	reportPages   int
	reportFirst   bool
	containerIDs  bool
	duplicatePage bool
}

func newCampaignPerfFixture(t *testing.T, count int) *campaignPerfFixture {
	f := &campaignPerfFixture{t: t, counts: map[string]int{}}
	for i := 0; i < count; i++ {
		m := modelMessageFixture(fmt.Sprintf("message-%03d", i), "sent")
		anyPath(m, "attributes.definition.send_options").(map[string]any)["send_time"] = time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
		f.messages = append(f.messages, m)
	}
	return f
}

func (f *campaignPerfFixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.counts[r.URL.Path]++
	w.Header().Set("Content-Type", "application/json")
	switch r.URL.Path {
	case "/api/metrics":
		fmt.Fprint(w, `{"data":[{"id":"orders","attributes":{"name":"Placed Order","integration":{"name":"Shopify"}}},{"id":"received","attributes":{"name":"Received Email"}},{"id":"opened","attributes":{"name":"Opened Email"}},{"id":"clicked","attributes":{"name":"Clicked Email"}}],"links":{"next":null}}`)
	case "/api/metric-aggregates":
		fmt.Fprint(w, `{"data":{"attributes":{"data":[]}}}`)
	case "/api/flows":
		fmt.Fprint(w, `{"data":[],"included":[],"links":{"next":null}}`)
	case "/api/campaign-values-reports/":
		f.reportPages++
		var rows []map[string]any
		start, end := 0, 1
		if r.URL.Query().Get("page_cursor") != "" {
			start, end = 1, 3
		}
		for i := start; i < end; i++ {
			id := fmt.Sprintf("message-%03d", i)
			containerID := id
			if f.containerIDs {
				containerID = modelContainerID
			}
			rows = append(rows, map[string]any{"groupings": map[string]any{"campaign_id": containerID, "campaign_message_id": id}, "statistics": map[string]any{"recipients": 100, "conversion_value": 500, "delivered": 100, "opens_unique": 50, "clicks_unique": 10}})
		}
		next := ""
		if end == 1 {
			next = "/api/campaign-values-reports/?page_cursor=last"
		}
		json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"attributes": map[string]any{"results": rows}}, "links": map[string]any{"next": next}})
	case "/api/campaigns":
		f.pageSizes = append(f.pageSizes, r.URL.Query().Get("page[size]"))
		w.Write(modelCollectionFixture(map[string]any{"id": modelContainerID, "attributes": map[string]any{"updated_at": "2026-11-09T00:00:00Z"}}))
	case "/api/campaign-messages", "/api/campaigns/" + modelContainerID + "/campaign-messages":
		if f.reportFirst && f.reportPages != 2 {
			f.t.Errorf("message lookup before report completed: %d pages", f.reportPages)
		}
		messages := f.messages
		if r.URL.Path == "/api/campaign-messages" {
			f.pageSizes = append(f.pageSizes, r.URL.Query().Get("page[size]"))
			filter := r.URL.Query().Get("filter")
			if strings.HasPrefix(filter, "any(id,") {
				var ids []string
				if err := json.Unmarshal([]byte(strings.TrimSuffix(strings.TrimPrefix(filter, "any(id,"), ")")), &ids); err != nil {
					f.t.Fatal(err)
				}
				f.lookups = append(f.lookups, ids)
				messages = nil
				for _, m := range f.messages {
					for _, id := range ids {
						if m["id"] == id {
							messages = append(messages, m)
							break
						}
					}
				}
			} else if filter == "equals(status,'scheduled')" {
				messages = nil
				for _, m := range f.messages {
					if stringFromMapPath(m, "attributes.definition.status") == "scheduled" {
						messages = append(messages, m)
					}
				}
			}
		}
		size, _ := strconv.Atoi(r.URL.Query().Get("page[size]"))
		if size == 0 {
			size = 20
		}
		start, _ := strconv.Atoi(r.URL.Query().Get("page[cursor]"))
		end := min(start+size, len(messages))
		var next any
		if end < len(messages) {
			query := r.URL.Query()
			query.Set("page[cursor]", strconv.Itoa(end))
			next = r.URL.Path + "?" + query.Encode()
		}
		page := append([]map[string]any{}, messages[start:end]...)
		if f.duplicatePage && len(page) > 0 {
			page = append(page, page[0])
		}
		json.NewEncoder(w).Encode(map[string]any{"data": page, "links": map[string]any{"next": next}})
	case "/api/segments":
		w.Write(modelCollectionFixture(map[string]any{"id": "segment", "attributes": map[string]any{"name": "VIP"}}))
	case "/api/lists", "/api/templates":
		w.Write(modelCollectionFixture())
	default:
		if strings.HasSuffix(r.URL.Path, "/campaign-audience") {
			w.Write(modelAudienceFixture([]string{"segment"}, nil))
		} else if strings.HasSuffix(r.URL.Path, "/campaign-variations") {
			id := strings.Split(r.URL.Path, "/")[3]
			w.Write(campaignVariationTestPage("variation-"+id, "template", "A useful subject", "email"))
		} else {
			f.t.Errorf("unexpected request: %s %s", r.Method, r.URL)
			http.NotFound(w, r)
		}
	}
}

func (f *campaignPerfFixture) messageRequests() (pages, details int) {
	for endpoint, count := range f.counts {
		if endpoint == "/api/campaign-messages" {
			pages += count
		} else if strings.HasSuffix(endpoint, "/campaign-audience") || strings.HasSuffix(endpoint, "/campaign-variations") {
			details += count
		}
	}
	return
}

func TestCampaignPerfReportsResolveOnlySentMessages(t *testing.T) {
	for _, tc := range []struct {
		name  string
		build func(*rootFlags) *cobra.Command
		args  []string
	}{
		{"targeting", newReportTargetingCmd, []string{"--conversion-metric-id", "orders"}},
		{"revenue campaign", newRevenuePerEmailCmd, []string{"--by", "campaign"}},
		{"revenue all", newRevenuePerEmailCmd, []string{"--by", "all"}},
		{"subject campaign", newSubjectLineAnalysisCmd, []string{"--source", "campaign"}},
		{"subject all", newSubjectLineAnalysisCmd, []string{"--source", "all"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newCampaignPerfFixture(t, 500)
			f.reportFirst, f.containerIDs = true, true
			campaignFixtureClient(t, f)
			cmd := tc.build(&rootFlags{asJSON: true, noCache: true, rateLimit: 10000})
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(io.Discard)
			cmd.SetArgs(tc.args)
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			pages, details := f.messageRequests()
			t.Logf("500 messages, 3 reported: collection=%d enrichment=%d total=%d", pages, details, pages+details)
			if pages != 1 || details != 6 || len(f.lookups) != 1 || len(f.lookups[0]) != 3 {
				t.Errorf("message requests: pages=%d details=%d lookups=%v", pages, details, f.lookups)
			}
			for _, size := range f.pageSizes {
				if size != "100" {
					t.Errorf("page size=%q, want 100", size)
				}
			}
			if !strings.Contains(out.String(), "VIP") && !strings.Contains(out.String(), "A useful subject") && !strings.Contains(out.String(), "Launch message-000") {
				t.Fatalf("lost message names or audiences: %s", &out)
			}
			var result map[string]any
			if err := json.Unmarshal(out.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			switch {
			case tc.name == "targeting":
				segments := mapSlice(result["segments"])
				if len(segments) != 1 || anyInt(segments[0]["total_sends"]) != 300 || anyInt(segments[0]["campaigns_sent"]) != 3 {
					t.Fatalf("targeting collapsed messages: %s", &out)
				}
			case strings.HasPrefix(tc.name, "revenue"):
				rows := mapSlice(result["campaigns"])
				if len(rows) != 3 || anyFloat(rows[0]["revenue_per_email"]) != 5 {
					t.Fatalf("revenue collapsed messages: %s", &out)
				}
			default:
				if anyInt(result["campaigns_analyzed"]) != 3 || anyInt(result["total_emails_analyzed"]) != 3 {
					t.Fatalf("subjects collapsed messages: %s", &out)
				}
			}
		})
	}
}

func TestCampaignPerfSyncSkipsUnchanged(t *testing.T) {
	for _, resource := range []string{"campaigns", "campaign-messages"} {
		t.Run(resource, func(t *testing.T) {
			f := newCampaignPerfFixture(t, 3)
			c := campaignFixtureClient(t, f)
			db, err := store.Open(filepath.Join(t.TempDir(), "sync.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			for run, want := range []int{6, 0, 2} {
				if run == 2 {
					f.messages[0]["attributes"].(map[string]any)["updated"] = "2026-11-09T01:00:00Z"
				}
				f.counts = map[string]int{}
				res := syncResource(context.Background(), c, db, resource, "", true, 0, false, false, nil, io.Discard)
				if res.Err != nil || res.Warn != nil {
					t.Fatalf("sync=%+v", res)
				}
				pages, details := f.messageRequests()
				total := 0
				for _, count := range f.counts {
					total += count
				}
				t.Logf("%s run %d: message pages=%d enrichment=%d total=%d", resource, run+1, pages, details, total)
				if details != want {
					t.Errorf("run %d: enrichment=%d, want %d", run+1, details, want)
				}
				rows, err := localCampaignAnalyticsRows(context.Background(), db)
				if err != nil || len(rows) != 3 {
					t.Fatalf("stored rows=%d error=%v", len(rows), err)
				}
			}
			for _, size := range f.pageSizes {
				if size != "100" {
					t.Errorf("page size=%q, want 100", size)
				}
			}
		})
	}
}

func TestCampaignPerfBatchesAndMemoizes(t *testing.T) {
	f := newCampaignPerfFixture(t, 205)
	f.duplicatePage = true
	c := campaignFixtureClient(t, f)
	ids := []string{"", "message-000"}
	for _, m := range f.messages {
		ids = append(ids, m["id"].(string))
	}
	rows, err := fetchSubjectCampaigns(c, ids)
	if err != nil {
		t.Fatal(err)
	}
	pages, details := f.messageRequests()
	t.Logf("205 unique IDs plus duplicates: collection=%d enrichment=%d total=%d", pages, details, pages+details)
	if pages != 3 || details != 410 || len(f.lookups) != 3 || len(f.lookups[0]) != 100 || len(f.lookups[1]) != 100 || len(f.lookups[2]) != 5 {
		t.Fatalf("205 messages: pages=%d enrichment=%d batches=%v", pages, details, f.lookups)
	}
	seen := map[string]bool{}
	for _, row := range rows {
		seen[row["id"].(string)] = true
	}
	if len(seen) != 205 {
		t.Fatalf("only read %d of 205 messages", len(seen))
	}
	for endpoint, count := range f.counts {
		if endpoint != "/api/campaign-messages" && count != 1 {
			t.Errorf("fetched %s %d times", endpoint, count)
		}
	}
}

func TestCampaignPerfEmptyReportDoesNotListMessages(t *testing.T) {
	f := newCampaignPerfFixture(t, 500)
	c := campaignFixtureClient(t, f)
	if _, err := campaignTargetingReport(c, nil); err != nil {
		t.Fatal(err)
	}
	if pages, details := f.messageRequests(); pages != 0 || details != 0 {
		t.Fatalf("empty report fetched %d pages and %d details", pages, details)
	}
}

func TestCampaignPerfListings(t *testing.T) {
	for _, name := range []string{"scheduled", "template audit", "raw messages", "send budget"} {
		t.Run(name, func(t *testing.T) {
			f := newCampaignPerfFixture(t, 500)
			for _, m := range f.messages[:120] {
				anyPath(m, "attributes.definition").(map[string]any)["status"] = "scheduled"
			}
			c := campaignFixtureClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if (name == "scheduled" || name == "send budget") && r.URL.Path == "/api/campaign-messages" && r.URL.Query().Get("filter") != "equals(status,'scheduled')" {
					t.Errorf("scheduled filter missing: %s", r.URL)
				}
				f.ServeHTTP(w, r)
			}))
			wantPages, wantDetails := 5, 1000
			switch name {
			case "scheduled":
				wantPages, wantDetails = 2, 240
				cmd := newCampaignsScheduleConflictsCmd(&rootFlags{asJSON: true, noCache: true, rateLimit: 10000})
				cmd.SetOut(io.Discard)
				if err := cmd.Execute(); err != nil {
					t.Fatal(err)
				}
			case "template audit":
				refs, err := collectReferencedTemplateIDs(c)
				if err != nil || !refs["template"] {
					t.Fatalf("refs=%v error=%v", refs, err)
				}
			case "raw messages":
				wantDetails = 0
				cmd := newCampaignMessagesGetCmd(&rootFlags{asJSON: true, noCache: true, rateLimit: 10000})
				cmd.SetArgs([]string{"--all"})
				cmd.SetOut(io.Discard)
				if err := cmd.Execute(); err != nil {
					t.Fatal(err)
				}
			case "send budget":
				wantPages, wantDetails = 2, 0
				rows, err := sendBudgetCollection(context.Background(), c, "/api/campaign-messages", map[string]string{"filter": "equals(status,'scheduled')"})
				if err != nil || len(rows) != 120 {
					t.Fatalf("rows=%d error=%v", len(rows), err)
				}
			}
			pages, details := f.messageRequests()
			if pages != wantPages || details != wantDetails {
				t.Errorf("pages=%d enrichment=%d; want %d/%d", pages, details, wantPages, wantDetails)
			}
			for _, size := range f.pageSizes {
				if size != "100" {
					t.Errorf("page size=%q, want 100", size)
				}
			}
			t.Logf("%s: collection=%d enrichment=%d total=%d", name, pages, details, pages+details)
		})
	}
}

func TestCampaignPerfSyncWindowFilters(t *testing.T) {
	for _, resource := range []string{"campaigns", "campaign-messages"} {
		t.Run(resource, func(t *testing.T) {
			f := newCampaignPerfFixture(t, 3)
			since := "2026-11-01T00:00:00Z"
			field := "updated"
			if resource == "campaigns" {
				field = "updated_at"
			}
			c := campaignFixtureClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/"+resource && r.URL.Query().Get("filter") != "greater-or-equal("+field+","+since+")" {
					t.Errorf("missing incremental window: %s", r.URL)
				}
				f.ServeHTTP(w, r)
			}))
			db, err := store.Open(filepath.Join(t.TempDir(), "sync.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			res := syncResource(context.Background(), c, db, resource, since, false, 0, false, false, nil, io.Discard)
			if res.Err != nil || res.Warn != nil {
				t.Fatalf("sync=%+v", res)
			}
		})
	}
}

func TestCampaignPerfSyncSharesMemoAcrossWorkers(t *testing.T) {
	f := newCampaignPerfFixture(t, 3)
	var mu sync.Mutex
	c := campaignFixtureClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		f.ServeHTTP(w, r)
	}))
	db, err := store.Open(filepath.Join(t.TempDir(), "sync.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.WithValue(context.Background(), campaignSyncMessagesKey{}, &campaignMessageReader{client: c})
	results := make(chan syncResult, 2)
	for _, resource := range []string{"campaigns", "campaign-messages"} {
		go func() {
			results <- syncResource(ctx, c, db, resource, "", true, 0, false, false, nil, io.Discard)
		}()
	}
	for range 2 {
		if res := <-results; res.Err != nil || res.Warn != nil {
			t.Errorf("sync=%+v", res)
		}
	}
	if _, details := f.messageRequests(); details != 6 {
		t.Fatalf("concurrent sync fetched %d details, want 6", details)
	}
}

func TestCampaignPerfSubjectWindowIncludesLaterEdits(t *testing.T) {
	f := newCampaignPerfFixture(t, 500)
	f.containerIDs, f.reportFirst = true, true
	since := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	until := since.AddDate(0, 0, 7)
	for _, m := range f.messages {
		attrs := m["attributes"].(map[string]any)
		attrs["created"] = "2020-01-01T00:00:00Z"
		attrs["updated"] = until.AddDate(0, 0, 1).Format(time.RFC3339)
		anyPath(m, "attributes.definition.send_options").(map[string]any)["send_time"] = since.Add(time.Hour).Format(time.RFC3339)
	}
	c := campaignFixtureClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/campaign-values-reports/" {
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if stringFromMapPath(body, "data.attributes.timeframe.start") != since.Format(time.RFC3339) || stringFromMapPath(body, "data.attributes.timeframe.end") != until.Format(time.RFC3339) {
				t.Errorf("report lost requested window: %v", body)
			}
		}
		f.ServeHTTP(w, r)
	}))
	result, err := subjectLineAnalysis(c, since, until, "campaign")
	if err != nil || anyInt(result["total_emails_analyzed"]) != 3 {
		t.Fatalf("subjects=%v error=%v", result, err)
	}
	if pages, details := f.messageRequests(); pages != 1 || details != 6 {
		t.Fatalf("window fetched %d pages and %d details", pages, details)
	}
}

func TestCampaignPerfListingPreservesWindowOnEveryPage(t *testing.T) {
	filter := "and(greater-or-equal(updated,2026-11-01T00:00:00Z),less-than(updated,2026-11-08T00:00:00Z))"
	pages := 0
	c := campaignFixtureClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pages++
		if r.URL.Query().Get("filter") != filter || r.URL.Query().Get("page[size]") != "100" {
			t.Errorf("lost window or page size: %s", r.URL)
		}
		if pages == 1 {
			fmt.Fprint(w, `{"data":[{"id":"one"}],"links":{"next":"/api/campaign-messages?page%5Bcursor%5D=last&page%5Bsize%5D=20"}}`)
		} else {
			w.Write(modelCollectionFixture(map[string]any{"id": "two"}))
		}
	}))
	rows, err := campaignCollection(c, "/api/campaign-messages", map[string]string{"filter": filter, "page[size]": "20"})
	if err != nil || len(rows) != 2 || pages != 2 {
		t.Fatalf("rows=%v pages=%d error=%v", rows, pages, err)
	}
}

func TestCampaignPerfMemoizesEnrichmentFailures(t *testing.T) {
	f := newCampaignPerfFixture(t, 1)
	c := campaignFixtureClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/campaign-variations") {
			f.counts[r.URL.Path]++
			w.Write(modelCollectionFixture())
			return
		}
		f.ServeHTTP(w, r)
	}))
	reader := campaignMessageReader{client: c}
	for range 2 {
		if _, err := reader.resolve(f.messages[0]); err == nil {
			t.Fatal("accepted missing variations")
		}
	}
	if _, details := f.messageRequests(); details != 2 {
		t.Fatalf("failed enrichment repeated %d requests, want 2", details)
	}
}

func TestCampaignPerfSyncEnrichesIncompleteStoredRows(t *testing.T) {
	for _, name := range []string{"raw row", "missing updated"} {
		t.Run(name, func(t *testing.T) {
			f := newCampaignPerfFixture(t, 1)
			c := campaignFixtureClient(t, f)
			db, err := store.Open(filepath.Join(t.TempDir(), "sync.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			stored := f.messages[0]
			if name == "missing updated" {
				m, err := resolveCampaignMessage(c, stored)
				if err != nil {
					t.Fatal(err)
				}
				stored = campaignAnalyticsRow(m)
				delete(f.messages[0]["attributes"].(map[string]any), "updated")
			}
			raw, err := json.Marshal(stored)
			if err != nil {
				t.Fatal(err)
			}
			if err := db.UpsertCampaignMessages(raw); err != nil {
				t.Fatal(err)
			}
			f.counts = map[string]int{}
			res := syncResource(context.Background(), c, db, "campaign-messages", "", true, 0, false, false, nil, io.Discard)
			if res.Err != nil || res.Warn != nil {
				t.Fatalf("sync=%+v", res)
			}
			if _, details := f.messageRequests(); details != 2 {
				t.Fatalf("incomplete row made %d enrichment requests, want 2", details)
			}
		})
	}
}

func TestCampaignPerfSyncFirstRunPagesAt100(t *testing.T) {
	f := newCampaignPerfFixture(t, 205)
	c := campaignFixtureClient(t, f)
	db, err := store.Open(filepath.Join(t.TempDir(), "sync.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for run, wantDetails := range []int{410, 0} {
		f.counts = map[string]int{}
		res := syncResource(context.Background(), c, db, "campaign-messages", "", true, 0, false, false, nil, io.Discard)
		if res.Err != nil || res.Warn != nil || res.Count != 205 {
			t.Fatalf("sync=%+v", res)
		}
		pages, details := f.messageRequests()
		t.Logf("205 messages sync run %d: collection=%d enrichment=%d total=%d", run+1, pages, details, pages+details)
		if pages != 3 || details != wantDetails {
			t.Fatalf("sync fetched %d pages and %d details, want 3/%d", pages, details, wantDetails)
		}
		rows, err := db.List("campaign-messages", 0)
		if err != nil || len(rows) != 205 {
			t.Fatalf("stored=%d error=%v", len(rows), err)
		}
	}
}

func TestCampaignPerfMemoAllowsDifferentMessagesInParallel(t *testing.T) {
	f := newCampaignPerfFixture(t, 2)
	started, release := make(chan struct{}, 2), make(chan struct{})
	var mu sync.Mutex
	c := campaignFixtureClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/campaign-audience") {
			started <- struct{}{}
			<-release
		}
		mu.Lock()
		defer mu.Unlock()
		f.ServeHTTP(w, r)
	}))
	reader := campaignMessageReader{client: c}
	results := make(chan error, 2)
	for _, message := range f.messages {
		go func() {
			_, err := reader.resolve(message)
			results <- err
		}()
	}
	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()
waitForRequests:
	for range 2 {
		select {
		case <-started:
		case <-deadline.C:
			t.Error("memo blocked an unrelated message behind another message's request")
			break waitForRequests
		}
	}
	close(release)
	for range 2 {
		if err := <-results; err != nil {
			t.Error(err)
		}
	}
}
