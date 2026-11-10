// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/platform"
)

func TestTargetingMessageJoinAndRequestCounts(t *testing.T) {
	for _, compound := range []bool{true, false} {
		t.Run(fmt.Sprintf("included_variations=%t", compound), func(t *testing.T) {
			counts := map[string]int{}
			root, flags := reviewFollowupRecordingRoot(t, func(w http.ResponseWriter, r *http.Request) {
				counts[r.Method+" "+r.URL.Path]++
				if r.URL.Query().Has("additional-fields[segment]") {
					t.Fatal("targeting does not need profile counts")
				}
				switch r.URL.Path {
				case "/api/metrics":
					fmt.Fprint(w, `{"data":[{"id":"orders","attributes":{"name":"Placed Order","integration":{"name":"Shopify"}}}],"links":{"next":null}}`)
				case "/api/campaign-values-reports/":
					start, end := 0, 1
					var next any = "/api/campaign-values-reports/?page_cursor=last"
					if r.URL.Query().Get("page_cursor") == "last" {
						start, end, next = 1, 39, nil
					}
					rows := []map[string]any{}
					for i := start; i < end; i++ {
						rows = append(rows, map[string]any{
							"groupings":  map[string]any{"campaign_id": modelContainerID, "campaign_message_id": fmt.Sprintf("message-%02d", i)},
							"statistics": map[string]any{"recipients": 100, "conversion_value": 25},
						})
					}
					json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"attributes": map[string]any{"results": rows}}, "links": map[string]any{"next": next}})
				case "/api/campaign-messages":
					if counts["POST /api/campaign-values-reports/"] != 2 {
						t.Fatal("message lookup before report pagination finished")
					}
					var ids []string
					filter := r.URL.Query().Get("filter")
					if err := json.Unmarshal([]byte(strings.TrimSuffix(strings.TrimPrefix(filter, "any(id,"), ")")), &ids); err != nil || len(ids) != 39 || r.URL.Query().Get("page[size]") != "100" {
						t.Fatalf("unbatched message lookup: %s", r.URL)
					}
					messages, included := []map[string]any{}, []map[string]any{}
					for _, id := range ids {
						m := modelMessageFixture(id, "sent")
						anyPath(m, "attributes.definition.send_options").(map[string]any)["send_time"] = time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)
						m["relationships"].(map[string]any)["campaign-variations"] = map[string]any{"data": []any{map[string]any{"type": "campaign-variation", "id": "variation-" + id}}}
						messages = append(messages, m)
						included = append(included, modelVariationFixture("variation-"+id, "template", "email"))
					}
					if !compound || r.URL.Query().Get("include") != "campaign-variations" {
						included = nil
					}
					// Duplicate collection rows must not repeat audience lookups or attribution.
					messages = append(messages, messages[0])
					json.NewEncoder(w).Encode(map[string]any{"data": messages, "included": included, "links": map[string]any{"next": nil}})
				case "/api/segments":
					w.Write(modelCollectionFixture(
						map[string]any{"id": "segment-a", "attributes": map[string]any{"name": "VIP"}},
						map[string]any{"id": "segment-b", "attributes": map[string]any{"name": "Buyers"}},
						map[string]any{"id": "segment-never", "attributes": map[string]any{"name": "Never mailed"}},
					))
				case "/api/lists":
					w.Write(modelCollectionFixture(map[string]any{"id": "list-a", "attributes": map[string]any{"name": "Newsletter"}}))
				default:
					if strings.HasSuffix(r.URL.Path, "/campaign-audience") {
						id := strings.Split(r.URL.Path, "/")[3]
						i, err := strconv.Atoi(strings.TrimPrefix(id, "message-"))
						if err != nil {
							t.Fatal(err)
						}
						included, excluded := []string{"segment-a"}, []string{}
						if i < 4 {
							included = append(included, "segment-b", "list-a")
						}
						if i == 4 || i == 5 {
							excluded = append(excluded, "segment-b", "list-a")
						}
						w.Write(modelAudienceFixture(included, excluded))
					} else if strings.HasSuffix(r.URL.Path, "/campaign-variations") {
						w.Write(campaignVariationTestPage("variation", "template", "Subject", "email"))
					} else {
						t.Fatalf("unexpected request: %s %s", r.Method, r.URL)
					}
				}
			})
			var output bytes.Buffer
			root.SetOut(&output)
			root.SetErr(io.Discard)
			root.SetArgs([]string{"report", "targeting", "--timeframe", "last_30_days", "--json"})
			err := root.Execute()
			finalizePlatformInvocation(flags, &err)
			if err != nil {
				t.Fatalf("targeting rejected joined message rows: %v (requests=%v)", err, counts)
			}
			var result map[string]any
			if err := json.Unmarshal(output.Bytes(), &result); err != nil {
				t.Fatalf("output: %s: %v", &output, err)
			}
			result = result["data"].(map[string]any)
			declaration := flags.platformAnalytics
			if declaration == nil || declaration.MatchedRows != 3 || *declaration.BusinessMetrics["total_sends"] != 4700 || *declaration.BusinessMetrics["attributed_revenue"] != 1175 {
				t.Fatalf("joined analytics declaration: %+v", declaration)
			}
			broken := *declaration
			broken.ClientProfile = flags.platformSession.ProfileName
			broken.TenantIdentity = flags.platformSession.ObservedIdentity
			broken.MatchedRows = 0
			if err := platform.ValidateAnalytics(broken); err == nil || !strings.Contains(err.Error(), "nonempty_inputs_with_zero_join_matches") {
				t.Fatalf("zero-match validation was lost: %v", err)
			}
			if result["campaign_report_rows"] != float64(39) || result["campaign_audience_assignments_rows"] != float64(51) {
				t.Fatalf("input rows: %s", &output)
			}
			segments, lists := mapSlice(result["segments"]), mapSlice(result["lists"])
			if len(segments) != 2 || segments[0]["segment_id"] != "segment-a" || segments[0]["total_sends"] != float64(3900) || segments[0]["attributed_revenue"] != float64(975) || segments[1]["total_sends"] != float64(400) || segments[1]["attributed_revenue"] != float64(100) || segments[1]["campaigns_excluded"] != float64(2) {
				t.Fatalf("segment attribution/exclusions: %v", segments)
			}
			if len(lists) != 1 || lists[0]["total_sends"] != float64(400) || lists[0]["attributed_revenue"] != float64(100) || lists[0]["campaigns_excluded"] != float64(2) || len(mapSlice(result["never_mailed_segments"])) != 1 {
				t.Fatalf("list attribution/exclusions: %v", result)
			}
			wantVariations := 39
			if compound {
				wantVariations = 0
			}
			messages, audiences, variations, total := 0, 0, 0, 0
			for endpoint, n := range counts {
				total += n
				switch {
				case endpoint == "GET /api/campaign-messages":
					messages += n
				case strings.HasSuffix(endpoint, "/campaign-audience"):
					audiences += n
				case strings.HasSuffix(endpoint, "/campaign-variations"):
					variations += n
				}
				if endpoint != "POST /api/campaign-values-reports/" && n != 1 {
					t.Errorf("repeated request: %s x%d", endpoint, n)
				}
			}
			if !reflect.DeepEqual([]int{messages, audiences, variations, total}, []int{1, 39, wantVariations, 45 + wantVariations}) {
				t.Fatalf("requests: messages=%d audiences=%d variations=%d total=%d", messages, audiences, variations, total)
			}
			t.Logf("39 report rows / 51 assignments: report=2 metric=1 messages=%d audiences=%d variations=%d segments=1 lists=1 total=%d", messages, audiences, variations, total)
		})
	}
}

func TestTargetingCompoundVariationsFollowPagesAndRequireCompleteRelationships(t *testing.T) {
	for _, complete := range []bool{true, false} {
		t.Run(fmt.Sprintf("complete=%t", complete), func(t *testing.T) {
			m := modelMessageFixture("message-1", "sent")
			m["relationships"].(map[string]any)["campaign-variations"] = map[string]any{"data": []any{
				map[string]any{"id": "sms", "type": "campaign-variation"},
				map[string]any{"id": "email", "type": "campaign-variation"},
			}}
			sms, email := modelVariationFixture("sms", "", "sms"), modelVariationFixture("email", "template", "email")
			page, err := json.Marshal(map[string]any{"data": []any{m}, "included": []any{sms}, "links": map[string]any{"next": "/api/campaign-messages?page%5Bcursor%5D=last"}})
			if err != nil {
				t.Fatal(err)
			}
			included := []any{}
			if complete {
				included = append(included, email)
			}
			last, err := json.Marshal(map[string]any{"data": []any{}, "included": included, "links": map[string]any{"next": nil}})
			if err != nil {
				t.Fatal(err)
			}
			c := &reportTestClient{gets: []json.RawMessage{page, last, modelAudienceFixture([]string{"segment-a"}, nil)}}
			if !complete {
				c.gets = append(c.gets, modelCollectionFixture(sms, email))
			}
			rows, err := fetchTargetingCampaigns(c, []map[string]any{{"groupings": map[string]any{"campaign_message_id": "message-1", "campaign_id": modelContainerID}}})
			if err != nil || len(rows) != 1 || rows[0]["id"] != "message-1" || len(rows[0]["variations"].([]campaignModelVariation)) != 2 {
				t.Fatalf("message/variations = %v, error = %v", rows, err)
			}
			want := 3
			if !complete {
				want++
			}
			if len(c.requests) != want || !strings.Contains(c.requests[1].path, "include=campaign-variations") || !strings.Contains(c.requests[1].path, "filter=") || c.requests[2].path != "/api/campaign-messages/message-1/campaign-audience" {
				t.Fatalf("pagination/enrichment requests = %+v", c.requests)
			}
		})
	}
}
