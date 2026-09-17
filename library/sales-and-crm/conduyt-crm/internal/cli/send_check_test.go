// Sweep findings: audience/DNC fetch and decode failures must print an inconclusive partial result and exit API-class non-zero.
package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/conduyt-crm/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/conduyt-crm/internal/store"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestNovelSendCheckHelpWires(t *testing.T) {
	testenv.Isolate(t)
	_, _, err := runNovel(t, []string{"send-check", "--help"}, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestSendCheckRejectsNonPositiveLimitBeforeIO(t *testing.T) {
	for _, tc := range []struct {
		name     string
		selector []string
	}{
		{name: "local", selector: []string{"--contact", "c"}},
		{name: "live-list", selector: []string{"--list", "list-1", "--db", t.TempDir() + "/missing.db"}},
		{name: "live-tag", selector: []string{"--tag", "lead", "--db", t.TempDir() + "/missing.db"}},
	} {
		for _, limit := range []string{"0", "-1"} {
			t.Run(tc.name+"/"+limit, func(t *testing.T) {
				testenv.Isolate(t)
				requests := 0
				args := append([]string{"send-check"}, tc.selector...)
				if tc.name == "local" {
					dbPath := t.TempDir() + "/data.db"
					db, err := store.Open(dbPath)
					if err != nil {
						t.Fatal(err)
					}
					if err := db.Upsert("contacts", "c", json.RawMessage(`{"id":"c","phone":"+15550000001"}`)); err != nil {
						t.Fatal(err)
					}
					if err := db.SaveSyncState("contacts", "", 1); err != nil {
						t.Fatal(err)
					}
					if err := db.Close(); err != nil {
						t.Fatal(err)
					}
					args = append(args, "--db", dbPath)
				}
				args = append(args, "--limit", limit, "--json")
				_, _, err := runNovel(t, args, func(w http.ResponseWriter, r *http.Request) {
					requests++
					http.NotFound(w, r)
				})
				var ce *cliError
				if !errors.As(err, &ce) || ce.code != 2 {
					t.Fatalf("err=%v", err)
				}
				if requests != 0 {
					t.Fatalf("requests=%d, want 0", requests)
				}
			})
		}
	}
}
func TestSendCheckVerdicts(t *testing.T) {
	testenv.Isolate(t)
	dbPath := t.TempDir() + "/data.db"
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	fixtures := map[string]string{"ok": `{"id":"ok","phone":"+15550000001","lineType":"mobile","phoneValid":true,"tags":["leads"]}`, "none": `{"id":"none","phone":"","tags":["leads"]}`, "unknown": `{"id":"unknown","phone":"+15550000003","tags":["leads"]}`, "bad": `{"id":"bad","phone":"+15550000004","line_type":"mobile","valid":false,"tags":["leads"]}`, "dnc": `{"id":"dnc","phone":"+15550000005","lineType":"mobile","valid":true,"tags":["leads"]}`}
	for id, raw := range fixtures {
		if err := db.Upsert("contacts", id, json.RawMessage(raw)); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.SaveSyncState("contacts", "", len(fixtures)); err != nil {
		t.Fatal(err)
	}
	db.Close()
	h := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/contacts/dnc-status" {
			t.Fatalf("path=%s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		writeDNCResponse(t, w, r, "dnc")
	}
	out, _, err := runNovel(t, []string{"send-check", "--tag", "leads", "--db", dbPath, "--json"}, h)
	if err != nil {
		t.Fatal(err)
	}
	out = compactTestJSON(out)
	for _, want := range []string{`"checked":5`, `"ok":1`, `"no_phone":1`, `"unverified":1`, `"invalid":1`, `"dnc":1`} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %s in %s", want, out)
		}
	}
}
func TestSendCheckEmptyIsSuccess(t *testing.T) {
	testenv.Isolate(t)
	dbPath := t.TempDir() + "/data.db"
	db, _ := store.Open(dbPath)
	db.Close()
	out, _, err := runNovel(t, []string{"send-check", "--tag", "absent", "--db", dbPath, "--json"}, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`{"data":[]}`)) })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(compactTestJSON(out), `"verdicts":[]`) {
		t.Fatalf("output=%s", out)
	}
}

func TestSendCheckUnsyncedMirrorFallsBackLive(t *testing.T) {
	testenv.Isolate(t)
	dbPath := t.TempDir() + "/data.db"
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	var contactsCalls int
	h := func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/contacts":
			contactsCalls++
			if r.URL.Query().Get("smartListId") != "list-1" {
				t.Fatalf("query=%s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"data":[{"id":"live","phone":"+15550000001","lineType":"mobile"}]}`))
		case "/contacts/dnc-status":
			writeDNCResponse(t, w, r)
		default:
			http.NotFound(w, r)
		}
	}
	out, _, err := runNovel(t, []string{"send-check", "--list", "list-1", "--db", dbPath, "--json"}, h)
	if err != nil {
		t.Fatal(err)
	}
	if contactsCalls != 1 || !strings.Contains(compactTestJSON(out), `"checked":1`) {
		t.Fatalf("calls=%d out=%s", contactsCalls, out)
	}
}

func TestSendCheckLiveListShortPageWithLargerTotalIsPartial(t *testing.T) {
	testenv.Isolate(t)
	out, _, err := runNovel(t, []string{"send-check", "--list", "list-1", "--limit", "200", "--db", t.TempDir() + "/missing.db", "--json"}, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/contacts":
			items := make([]map[string]any, 50)
			for i := range items {
				items[i] = map[string]any{"id": fmt.Sprintf("c%d", i), "phone": "+15550000001", "lineType": "mobile"}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": items, "meta": map[string]any{"total": 100}})
		case "/contacts/dnc-status":
			writeDNCResponse(t, w, r)
		}
	})
	var view sendCheckView
	if decodeErr := json.Unmarshal([]byte(out), &view); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	if ExitCode(err) != 5 || !view.Partial || view.Verdict != "inconclusive" || view.Checked != 50 || view.Total == nil || *view.Total != 100 || !strings.Contains(strings.Join(view.Failures, "; "), "ended after 50 rows") {
		t.Fatalf("err=%v view=%+v", err, view)
	}
}

func TestSendCheckLiveListEmptyWithLargerTotalIsPartial(t *testing.T) {
	testenv.Isolate(t)
	out, _, err := runNovel(t, []string{"send-check", "--list", "list-1", "--limit", "200", "--db", t.TempDir() + "/missing.db", "--json"}, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/contacts" {
			t.Fatalf("unexpected request %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"data":[],"meta":{"total":1}}`))
	})
	var view sendCheckView
	if decodeErr := json.Unmarshal([]byte(out), &view); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	if ExitCode(err) != 5 || !view.Partial || view.Verdict != "inconclusive" || view.Checked != 0 || view.Total == nil || *view.Total != 1 || !strings.Contains(strings.Join(view.Failures, "; "), "ended after 0 rows") {
		t.Fatalf("err=%v view=%+v", err, view)
	}
}

func TestSendCheckSyncedMirrorIsLocalAgentSource(t *testing.T) {
	testenv.Isolate(t)
	dbPath := t.TempDir() + "/data.db"
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Upsert("contacts", "local", json.RawMessage(`{"id":"local","phone":"+15550000001","lineType":"mobile","tags":["leads"]}`)); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveSyncState("contacts", "", 1); err != nil {
		t.Fatal(err)
	}
	db.Close()
	h := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/contacts" {
			t.Fatal("unexpected live contacts call")
		}
		writeDNCResponse(t, w, r)
	}
	out, _, err := runNovel(t, []string{"send-check", "--tag", "leads", "--db", dbPath, "--agent", "--json"}, h)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(compactTestJSON(out), `"source":"local"`) {
		t.Fatalf("out=%s", out)
	}
}

func TestSendCheckLocalFilterPrecedesLimit(t *testing.T) {
	testenv.Isolate(t)
	dbPath := t.TempDir() + "/data.db"
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"a", "b", "c"} {
		if err := db.Upsert("contacts", id, json.RawMessage(`{"id":"`+id+`","phone":"+15550000001","lineType":"mobile","tags":["other"]}`)); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Upsert("contacts", "z-match", json.RawMessage(`{"id":"z-match","phone":"+15550000009","lineType":"mobile","tags":["wanted"]}`)); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveSyncState("contacts", "", 4); err != nil {
		t.Fatal(err)
	}
	db.Close()
	out, _, err := runNovel(t, []string{"send-check", "--tag", "wanted", "--limit", "1", "--db", dbPath, "--json"}, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/contacts" {
			t.Fatal("unexpected live contacts call")
		}
		writeDNCResponse(t, w, r)
	})
	if err != nil {
		t.Fatal(err)
	}
	compact := compactTestJSON(out)
	if !strings.Contains(compact, `"contact_id":"z-match"`) || !strings.Contains(compact, `"checked":1`) {
		t.Fatalf("out=%s", out)
	}
}

func TestSendCheckLocalLimitIsInconclusive(t *testing.T) {
	for _, tc := range []struct {
		name        string
		limit       string
		wantCode    int
		wantDNC     int
		wantPartial bool
	}{
		{name: "capped before DNC", limit: "2", wantCode: 5, wantPartial: true},
		{name: "covers DNC", limit: "3", wantDNC: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testenv.Isolate(t)
			dbPath := t.TempDir() + "/data.db"
			db, err := store.Open(dbPath)
			if err != nil {
				t.Fatal(err)
			}
			for _, fixture := range []struct{ id, phone string }{
				{id: "a-safe", phone: "+15550000001"},
				{id: "b-safe", phone: "+15550000002"},
				{id: "z-dnc", phone: "+15550000003"},
			} {
				raw := fmt.Sprintf(`{"id":%q,"phone":%q,"lineType":"mobile","tags":["leads"]}`, fixture.id, fixture.phone)
				if err := db.Upsert("contacts", fixture.id, json.RawMessage(raw)); err != nil {
					t.Fatal(err)
				}
			}
			if err := db.SaveSyncState("contacts", "", 3); err != nil {
				t.Fatal(err)
			}
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			h := func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/contacts" {
					t.Fatal("unexpected live contacts call")
				}
				writeDNCResponse(t, w, r, "z-dnc")
			}
			out, _, err := runNovel(t, []string{"send-check", "--tag", "leads", "--limit", tc.limit, "--db", dbPath, "--json"}, h)
			gotCode := 0
			if err != nil {
				gotCode = ExitCode(err)
			}
			if gotCode != tc.wantCode {
				t.Fatalf("err=%v code=%d", err, gotCode)
			}
			var view sendCheckView
			if err := json.Unmarshal([]byte(out), &view); err != nil {
				t.Fatal(err)
			}
			if view.Partial != tc.wantPartial || view.Summary.DNC != tc.wantDNC {
				t.Fatalf("view=%+v", view)
			}
			if tc.wantPartial && (view.Total == nil || *view.Total != 3 || view.Verdict != "inconclusive") {
				t.Fatalf("partial metadata=%+v", view)
			}
			if !tc.wantPartial && view.Verdict != "no_go" {
				t.Fatalf("verdict=%q, want no_go", view.Verdict)
			}
		})
	}
}

func TestSendCheckStaleMirrorFetchesAudienceLive(t *testing.T) {
	testenv.Isolate(t)
	dbPath := t.TempDir() + "/data.db"
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Upsert("contacts", "local-safe", json.RawMessage(`{"id":"local-safe","phone":"+15550000001","lineType":"mobile","tags":["leads"]}`)); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveSyncState("contacts", "", 1); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB().Exec(`UPDATE sync_state SET last_synced_at = ? WHERE resource_type = 'contacts'`, time.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	contactsCalls := 0
	h := func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/contacts":
			contactsCalls++
			_, _ = w.Write([]byte(`{"data":[{"id":"live-dnc","phone":"+15550000009","lineType":"mobile"}]}`))
		case "/contacts/dnc-status":
			writeDNCResponse(t, w, r, "live-dnc")
		default:
			http.NotFound(w, r)
		}
	}
	out, _, err := runNovel(t, []string{"send-check", "--tag", "leads", "--db", dbPath, "--json"}, h)
	if err != nil {
		t.Fatal(err)
	}
	var view sendCheckView
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatal(err)
	}
	if contactsCalls != 1 || view.Summary.DNC != 1 || view.Verdicts[0].ContactID != "live-dnc" {
		t.Fatalf("contactsCalls=%d view=%+v", contactsCalls, view)
	}
}

func TestSendCheckLiveListPaginates(t *testing.T) {
	testenv.Isolate(t)
	var pages []string
	h := func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/contacts":
			pages = append(pages, r.URL.Query().Get("page")+":"+r.URL.Query().Get("per_page"))
			page := 1
			perPage := 0
			_, _ = fmt.Sscan(r.URL.Query().Get("page"), &page)
			_, _ = fmt.Sscan(r.URL.Query().Get("per_page"), &perPage)
			start, count := (page-1)*perPage, perPage
			if start+count > 250 {
				count = 250 - start
			}
			items := make([]map[string]any, count)
			for i := range items {
				items[i] = map[string]any{"id": fmt.Sprintf("id-%d", start+i), "phone": "+15550000001", "lineType": "mobile"}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"data": items, "meta": map[string]any{"page": page, "per_page": perPage, "total": 250}}})
		case "/contacts/dnc-status":
			writeDNCResponse(t, w, r)
		default:
			http.NotFound(w, r)
		}
	}
	out, _, err := runNovel(t, []string{"send-check", "--list", "list-1", "--limit", "250", "--db", t.TempDir() + "/missing.db", "--json"}, h)
	if err != nil {
		t.Fatal(err)
	}
	var view sendCheckView
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, verdict := range view.Verdicts {
		ids[verdict.ContactID] = true
	}
	if strings.Join(pages, ",") != "1:200,2:200" || len(view.Verdicts) != 250 || len(ids) != 250 {
		t.Fatalf("pages=%v out=%s", pages, out)
	}
}

func TestSendCheckLiveSmallLimitUsesOnePage(t *testing.T) {
	testenv.Isolate(t)
	var pages []string
	h := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/contacts/dnc-status" {
			writeDNCResponse(t, w, r)
			return
		}
		pages = append(pages, r.URL.Query().Get("page")+":"+r.URL.Query().Get("per_page"))
		items := make([]map[string]any, 30)
		for i := range items {
			items[i] = map[string]any{"id": fmt.Sprintf("id-%d", i), "phone": "+15550000001", "lineType": "mobile"}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": items})
	}
	out, _, err := runNovel(t, []string{"send-check", "--tag", "lead", "--limit", "30", "--db", t.TempDir() + "/missing.db", "--json"}, h)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(pages, ",") != "1:200" || !strings.Contains(compactTestJSON(out), `"checked":30`) {
		t.Fatalf("pages=%v out=%s", pages, out)
	}
}

func TestSendCheckLiveTagsMergeDeduplicateThenLimit(t *testing.T) {
	testenv.Isolate(t)
	var queries []string
	h := func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/contacts":
			queries = append(queries, r.URL.Query().Get("tag")+":"+r.URL.Query().Get("page")+":"+r.URL.Query().Get("per_page"))
			var data string
			switch r.URL.Query().Get("tag") {
			case "A":
				data = `[{"id":"shared","phone":"+15550000001","lineType":"mobile"}]`
			case "B":
				data = `[{"id":"shared","phone":"+15550000001","lineType":"mobile"},{"id":"blocked","phone":"+15550000002","lineType":"mobile","valid":false},{"id":"after-limit","phone":"+15550000003","lineType":"mobile"}]`
			default:
				t.Fatalf("unexpected tag query: %s", r.URL.RawQuery)
			}
			_, _ = fmt.Fprintf(w, `{"data":%s}`, data)
		case "/contacts/dnc-status":
			writeDNCResponse(t, w, r, "blocked")
		default:
			http.NotFound(w, r)
		}
	}
	out, _, err := runNovel(t, []string{"send-check", "--tag", "A", "--tag", "B", "--limit", "2", "--db", t.TempDir() + "/missing.db", "--json"}, h)
	if err == nil || ExitCode(err) != 5 {
		t.Fatalf("err=%v code=%d", err, ExitCode(err))
	}
	var view sendCheckView
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatal(err)
	}
	if strings.Join(queries, ",") != "A:1:200,B:1:200" {
		t.Fatalf("queries=%v", queries)
	}
	if view.Summary.Checked != 2 || view.Summary.DNC != 1 || len(view.Verdicts) != 2 {
		t.Fatalf("summary=%+v verdicts=%+v", view.Summary, view.Verdicts)
	}
	if !view.Partial || view.Verdict != "inconclusive" || view.Total == nil || *view.Total != 3 {
		t.Fatalf("partial metadata=%+v", view)
	}
	if view.Verdicts[0].ContactID != "shared" || view.Verdicts[1].ContactID != "blocked" {
		t.Fatalf("verdicts=%+v", view.Verdicts)
	}
}

func TestSendCheckRejectsLiveAudienceRowsWithoutIDs(t *testing.T) {
	for _, tc := range []struct {
		name     string
		args     []string
		contacts string
	}{
		{name: "contact", args: []string{"--contact", "requested-id"}, contacts: `{"data":{"phone":"+15550000001","lineType":"mobile"}}`},
		{name: "tag", args: []string{"--tag", "lead"}, contacts: `{"data":[{"phone":"+15550000001","lineType":"mobile"}]}`},
		{name: "multiple tags", args: []string{"--tag", "A", "--tag", "B"}, contacts: `{"data":[{"phone":"+15550000001","lineType":"mobile"},{"phone":"+15550000002","lineType":"mobile"}]}`},
		{name: "smart list", args: []string{"--list", "list-1"}, contacts: `{"data":[{"phone":"+15550000001","lineType":"mobile"}]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testenv.Isolate(t)
			dncCalls := 0
			out, _, err := runNovel(t, append([]string{"send-check"}, append(tc.args, "--db", t.TempDir()+"/missing.db", "--json")...), func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/contacts/dnc-status" {
					dncCalls++
					writeDNCResponse(t, w, r)
					return
				}
				_, _ = w.Write([]byte(tc.contacts))
			})
			var view sendCheckView
			if decodeErr := json.Unmarshal([]byte(out), &view); decodeErr != nil {
				t.Fatal(decodeErr)
			}
			if ExitCode(err) != 5 || !view.Partial || view.Verdict != "inconclusive" || view.Verdict == "go" {
				t.Fatalf("err=%v view=%+v", err, view)
			}
			if !strings.Contains(view.Error, "missing a stable id") || dncCalls != 0 {
				t.Fatalf("error=%q dncCalls=%d", view.Error, dncCalls)
			}
		})
	}
}

func TestSendCheckLiveListLimitIsInconclusive(t *testing.T) {
	for _, tc := range []struct {
		name        string
		limit       string
		wantCode    int
		wantDNC     int
		wantPartial bool
	}{
		{name: "capped before DNC", limit: "1", wantCode: 5, wantPartial: true},
		{name: "covers DNC", limit: "2", wantDNC: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testenv.Isolate(t)
			h := func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/contacts":
					page := r.URL.Query().Get("page")
					perPage := r.URL.Query().Get("per_page")
					if page == "1" {
						if perPage == "1" {
							_, _ = w.Write([]byte(`{"data":[{"id":"safe","phone":"+15550000001","lineType":"mobile"}]}`))
						} else {
							_, _ = w.Write([]byte(`{"data":[{"id":"safe","phone":"+15550000001","lineType":"mobile"},{"id":"blocked","phone":"+15550000002","lineType":"mobile"}]}`))
						}
					} else if page == "2" && perPage == "1" {
						_, _ = w.Write([]byte(`{"data":[{"id":"blocked","phone":"+15550000002","lineType":"mobile"}]}`))
					} else {
						_, _ = w.Write([]byte(`{"data":[]}`))
					}
				case "/contacts/dnc-status":
					writeDNCResponse(t, w, r, "blocked")
				default:
					http.NotFound(w, r)
				}
			}
			out, _, err := runNovel(t, []string{"send-check", "--list", "list-1", "--limit", tc.limit, "--db", t.TempDir() + "/missing.db", "--json"}, h)
			gotCode := 0
			if err != nil {
				gotCode = ExitCode(err)
			}
			if gotCode != tc.wantCode {
				t.Fatalf("err=%v code=%d", err, gotCode)
			}
			var view sendCheckView
			if err := json.Unmarshal([]byte(out), &view); err != nil {
				t.Fatal(err)
			}
			if view.Partial != tc.wantPartial || view.Summary.DNC != tc.wantDNC {
				t.Fatalf("view=%+v", view)
			}
		})
	}
}

func TestSendCheckRejectsInvalidDNCResponses(t *testing.T) {
	for name, body := range map[string]string{"malformed": `{`, "unexpected": `{"data":42}`, "error with data": `{"error":{"message":"down"},"data":[]}`, "nested message with data": `{"data":{"message":"down","c":false}}`} {
		t.Run(name, func(t *testing.T) {
			testenv.Isolate(t)
			h := func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/contacts/c" {
					_, _ = w.Write([]byte(`{"data":{"id":"c","phone":"+15550000001","lineType":"mobile"}}`))
					return
				}
				_, _ = w.Write([]byte(body))
			}
			out, _, err := runNovel(t, []string{"send-check", "--contact", "c", "--db", t.TempDir() + "/missing.db", "--json"}, h)
			var ce *cliError
			if !errors.As(err, &ce) || ce.code != 5 {
				t.Fatalf("err=%v", err)
			}
			if !strings.Contains(compactTestJSON(out), `"partial":true`) {
				t.Fatalf("missing partial verdict: %s", out)
			}
		})
	}
}

func TestSendCheckRejectsInvalidDNCStatuses(t *testing.T) {
	for name, body := range map[string]string{
		"missing statuses":       `{"data":{"requested":1,"checked":0,"missing":1,"truncated":false}}`,
		"statuses is array":      `{"data":{"statuses":[],"requested":1,"checked":0,"missing":1,"truncated":false}}`,
		"non-boolean smsBlocked": `{"data":{"statuses":{"c":{"smsBlocked":"true"}},"requested":1,"checked":1,"missing":0,"truncated":false}}`,
	} {
		t.Run(name, func(t *testing.T) {
			testenv.Isolate(t)
			out, _, err := runNovel(t, []string{"send-check", "--contact", "c", "--db", t.TempDir() + "/missing.db", "--json"}, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/contacts/c" {
					_, _ = w.Write([]byte(`{"data":{"id":"c","phone":"+15550000001","lineType":"mobile"}}`))
					return
				}
				_, _ = w.Write([]byte(body))
			})
			var view sendCheckView
			if decodeErr := json.Unmarshal([]byte(out), &view); decodeErr != nil {
				t.Fatal(decodeErr)
			}
			if ExitCode(err) != 5 || !view.Partial || view.Verdict != "inconclusive" || view.Verdict == "go" {
				t.Fatalf("err=%v view=%+v", err, view)
			}
		})
	}
}

func TestSendCheckCarriesVoiceBlockedAndUsesSMSBlocked(t *testing.T) {
	testenv.Isolate(t)
	out, _, err := runNovel(t, []string{"send-check", "--contact", "c", "--db", t.TempDir() + "/missing.db", "--json"}, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/contacts/c" {
			_, _ = w.Write([]byte(`{"data":{"id":"c","phone":"2125551212","lineType":"mobile"}}`))
			return
		}
		if r.URL.Query().Get("ids") != "c" {
			t.Fatalf("ids=%q", r.URL.Query().Get("ids"))
		}
		_, _ = w.Write([]byte(`{"data":{"statuses":{"c":{"dnc":true,"litigator":false,"voiceBlocked":true,"smsBlocked":true}},"requested":1,"checked":1,"missing":0,"truncated":false}}`))
	})
	if err != nil {
		t.Fatal(err)
	}
	var view sendCheckView
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatal(err)
	}
	if view.Verdict != "no_go" || view.Summary.DNC != 1 || !view.Verdicts[0].VoiceBlocked {
		t.Fatalf("view=%+v", view)
	}
}

func TestSendCheckMalformedAudiencePhoneIsInconclusive(t *testing.T) {
	testenv.Isolate(t)
	out, _, err := runNovel(t, []string{"send-check", "--contact", "c", "--db", t.TempDir() + "/missing.db", "--json"}, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/contacts/c" {
			_, _ = w.Write([]byte(`{"data":{"id":"c","phone":"123456","lineType":"mobile"}}`))
			return
		}
		writeDNCResponse(t, w, r)
	})
	var view sendCheckView
	if decodeErr := json.Unmarshal([]byte(out), &view); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	if ExitCode(err) != 5 || !view.Partial || view.Verdict != "inconclusive" {
		t.Fatalf("err=%v view=%+v", err, view)
	}
}

func TestSendCheckDNCFailuresAreTyped(t *testing.T) {
	for _, tc := range []struct{ status, code int }{{403, 4}, {500, 5}} {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			testenv.Isolate(t)
			h := func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/contacts/c" {
					_, _ = w.Write([]byte(`{"data":{"id":"c","phone":"+15550000001","lineType":"mobile"}}`))
					return
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(`{"message":"DNC denied"}`))
			}
			out, _, err := runNovel(t, []string{"send-check", "--contact", "c", "--db", t.TempDir() + "/missing.db", "--json"}, h)
			if err == nil {
				t.Fatal("expected error")
			}
			var ce *cliError
			if !errors.As(err, &ce) || ce.code != tc.code {
				t.Fatalf("err=%v", err)
			}
			if !strings.Contains(compactTestJSON(out), `"partial":true`) {
				t.Fatalf("missing partial verdict: %s", out)
			}
		})
	}
}

func TestSendCheckDNCChunksAt500(t *testing.T) {
	testenv.Isolate(t)
	dbPath := t.TempDir() + "/data.db"
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 501; i++ {
		id := fmt.Sprintf("c%03d", i)
		if err := db.Upsert("contacts", id, json.RawMessage(fmt.Sprintf(`{"id":%q,"phone":"+15550000001","lineType":"mobile","tags":["all"]}`, id))); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.SaveSyncState("contacts", "", 501); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	calls := 0
	_, _, err = runNovel(t, []string{"send-check", "--tag", "all", "--limit", "501", "--db", dbPath, "--json"}, func(w http.ResponseWriter, r *http.Request) {
		calls++
		ids := strings.Split(r.URL.Query().Get("ids"), ",")
		if len(ids) > 500 {
			t.Fatalf("chunk=%d", len(ids))
		}
		statuses := map[string]any{}
		for _, id := range ids {
			statuses[id] = map[string]bool{"dnc": false, "litigator": false, "voiceBlocked": false, "smsBlocked": false}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"statuses": statuses, "requested": len(ids), "checked": len(ids), "missing": 0, "truncated": false}})
	})
	if err != nil || calls != 2 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}

func TestSendCheckDNCMissingAndTruncatedArePartial(t *testing.T) {
	for _, tc := range []struct{ name, body, want string }{
		{"missing", `{"data":{"statuses":{},"requested":1,"checked":0,"missing":1,"truncated":false}}`, "absent from DNC statuses"},
		{"truncated", `{"data":{"statuses":{"c":{"dnc":false,"litigator":false,"voiceBlocked":false,"smsBlocked":false}},"requested":1,"checked":1,"missing":0,"truncated":true}}`, "was truncated"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, _, err := runNovel(t, []string{"send-check", "--contact", "c", "--db", t.TempDir() + "/missing.db", "--json"}, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/contacts/c" {
					_, _ = w.Write([]byte(`{"data":{"id":"c","phone":"+15550000001","lineType":"mobile"}}`))
					return
				}
				_, _ = w.Write([]byte(tc.body))
			})
			if ExitCode(err) != 5 || !strings.Contains(out, tc.want) {
				t.Fatalf("err=%v out=%s", err, out)
			}
		})
	}
}

func compactTestJSON(s string) string {
	var b bytes.Buffer
	if json.Compact(&b, []byte(s)) == nil {
		return b.String()
	}
	return s
}

func writeDNCResponse(t *testing.T, w http.ResponseWriter, r *http.Request, blockedIDs ...string) {
	t.Helper()
	blocked := map[string]bool{}
	for _, id := range blockedIDs {
		blocked[id] = true
	}
	ids := strings.Split(r.URL.Query().Get("ids"), ",")
	statuses := map[string]any{}
	for _, id := range ids {
		if id == "" {
			continue
		}
		statuses[id] = map[string]bool{"dnc": blocked[id], "litigator": false, "voiceBlocked": false, "smsBlocked": blocked[id]}
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"statuses": statuses, "requested": len(statuses), "checked": len(statuses), "missing": 0, "truncated": false}})
}

func TestSendCheckTagRepeatedFullPageIsInconclusive(t *testing.T) {
	testenv.Isolate(t)
	requests := 0
	out, _, err := runNovel(t, []string{"send-check", "--tag", "lead", "--limit", "500", "--db", t.TempDir() + "/missing.db", "--json"}, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/contacts":
			requests++
			items := make([]map[string]any, 200)
			for i := range items {
				items[i] = map[string]any{"id": fmt.Sprintf("c-%d", i), "phone": "+15550000001", "lineType": "mobile"}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": items})
		case "/contacts/dnc-status":
			writeDNCResponse(t, w, r)
		default:
			http.NotFound(w, r)
		}
	})
	if ExitCode(err) != 5 || requests != 2 {
		t.Fatalf("err=%v code=%d requests=%d", err, ExitCode(err), requests)
	}
	compact := compactTestJSON(out)
	if !strings.Contains(compact, `"partial":true`) || !strings.Contains(compact, `"verdict":"inconclusive"`) || !strings.Contains(compact, "zero new ids") {
		t.Fatalf("out=%s", out)
	}
}

func TestSendCheckTagPageCapIsInconclusive(t *testing.T) {
	testenv.Isolate(t)
	requests := 0
	out, _, err := runNovel(t, []string{"send-check", "--tag", "lead", "--limit", "25000", "--db", t.TempDir() + "/missing.db", "--json"}, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/contacts":
			requests++
			page, _ := strconv.Atoi(r.URL.Query().Get("page"))
			items := make([]map[string]any, 200)
			for i := range items {
				items[i] = map[string]any{"id": fmt.Sprintf("p%d-c%d", page, i), "phone": "+15550000001", "lineType": "mobile"}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": items})
		case "/contacts/dnc-status":
			writeDNCResponse(t, w, r)
		default:
			http.NotFound(w, r)
		}
	})
	if ExitCode(err) != 5 || requests != paginationMaxPages {
		t.Fatalf("err=%v code=%d requests=%d", err, ExitCode(err), requests)
	}
	if compact := compactTestJSON(out); !strings.Contains(compact, `"partial":true`) || !strings.Contains(compact, "100-page safety cap") {
		t.Fatalf("out=%s", out)
	}
}

func TestPaginationProgressGuardRejectsRepeatedCursor(t *testing.T) {
	guard := newPaginationProgressGuard()
	if err := guard.observe([]string{"one"}, "cursor-a", true); err != nil {
		t.Fatal(err)
	}
	if err := guard.observe([]string{"two"}, "cursor-a", true); err == nil || !strings.Contains(err.Error(), "cursor") {
		t.Fatalf("err=%v", err)
	}
}

func runNovel(t *testing.T, args []string, h http.HandlerFunc) (string, string, error) {
	t.Helper()
	if h == nil {
		h = func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) }
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	t.Setenv("CONDUYT_CRM_BASE_URL", srv.URL)
	t.Setenv("CONDUYT_CRM_BEARER_AUTH", "test-token")
	cmd := RootCmd()
	cmd.SetArgs(args)
	var out, stderr bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&stderr)
	err := cmd.Execute()
	return out.String(), stderr.String(), err
}
